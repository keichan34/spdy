// Copyright 2013 Jamie Hall. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spdy

import (
	"errors"
	"sync"
)

// Objects conforming to the FlowControl interface can be
// used to provide the flow control mechanism for a
// connection using SPDY version 3 and above.
//
// InitialWindowSize is called whenever a new stream is
// created, and the returned value used as the initial
// flow control window size. Note that a values smaller
// than the default (65535) will likely result in poor
// network utilisation.
//
// ReceiveData is called whenever a stream's window is
// consumed by inbound data. The stream's ID is provided,
// along with the stream's initial window size and the
// current window size after receiving the data that
// caused the call. If the window is to be regrown,
// ReceiveData should return the increase in size. A value
// of 0 does not change the window. Note that in SPDY/3.1
// and later, the streamID may be 0 to represent the
// connection-level flow control window.
type FlowControl interface {
	InitialWindowSize() uint32
	ReceiveData(streamID StreamID, initialWindowSize uint32, newWindowSize int64) (deltaSize uint32)
}

type DefaultFlowControl uint32

func (f DefaultFlowControl) InitialWindowSize() uint32 {
	return uint32(f)
}

func (f DefaultFlowControl) ReceiveData(_ StreamID, initialWindowSize uint32, newWindowSize int64) uint32 {
	if newWindowSize < (int64(initialWindowSize) / 2) {
		return uint32(int64(initialWindowSize) - newWindowSize)
	}

	return 0
}

// flowControl is used by Streams to ensure that
// they abide by SPDY's flow control rules. For
// versions of SPDY before 3, this has no effect.
type flowControl struct {
	sync.Mutex
	stream              Stream
	streamID            StreamID
	output              chan<- Frame
	initialWindow       uint32
	transferWindow      int64
	sent                uint32
	buffer              [][]byte
	constrained         bool
	initialWindowThere  uint32
	transferWindowThere int64
	flowControl         FlowControl
}

// AddFlowControl initialises flow control for
// the Stream. If the Stream is running at an
// older SPDY version than SPDY/3, the flow
// control has no effect. Multiple calls to
// AddFlowControl are safe.
func (s *serverStreamV3) AddFlowControl(f FlowControl) {
	if s.flow != nil {
		return
	}

	s.flow = new(flowControl)
	initialWindow, err := s.conn.InitialWindowSize()
	if err != nil {
		log.Println(err)
		return
	}
	s.flow.streamID = s.streamID
	s.flow.output = s.output
	s.flow.buffer = make([][]byte, 0, 10)
	s.flow.initialWindow = initialWindow
	s.flow.transferWindow = int64(initialWindow)
	s.flow.stream = s
	s.flow.flowControl = f
	s.flow.initialWindowThere = f.InitialWindowSize()
	s.flow.transferWindowThere = int64(s.flow.initialWindowThere)
}

// AddFlowControl initialises flow control for
// the Stream. If the Stream is running at an
// older SPDY version than SPDY/3, the flow
// control has no effect. Multiple calls to
// AddFlowControl are safe.
func (p *pushStreamV3) AddFlowControl(f FlowControl) {
	if p.flow != nil {
		return
	}

	p.flow = new(flowControl)
	initialWindow, err := p.conn.InitialWindowSize()
	if err != nil {
		log.Println(err)
		return
	}
	p.flow.streamID = p.streamID
	p.flow.output = p.output
	p.flow.buffer = make([][]byte, 0, 10)
	p.flow.initialWindow = initialWindow
	p.flow.transferWindow = int64(initialWindow)
	p.flow.stream = p
	p.flow.flowControl = f
	p.flow.initialWindowThere = f.InitialWindowSize()
	p.flow.transferWindowThere = int64(p.flow.transferWindowThere)
}

// AddFlowControl initialises flow control for
// the Stream. If the Stream is running at an
// older SPDY version than SPDY/3, the flow
// control has no effect. Multiple calls to
// AddFlowControl are safe.
func (r *clientStreamV3) AddFlowControl(f FlowControl) {
	if r.flow != nil {
		return
	}

	r.flow = new(flowControl)
	initialWindow, err := r.conn.InitialWindowSize()
	if err != nil {
		log.Println(err)
		return
	}
	r.flow.streamID = r.streamID
	r.flow.output = r.output
	r.flow.buffer = make([][]byte, 0, 10)
	r.flow.initialWindow = initialWindow
	r.flow.transferWindow = int64(initialWindow)
	r.flow.stream = r
	r.flow.flowControl = f
	r.flow.initialWindowThere = f.InitialWindowSize()
	r.flow.transferWindowThere = int64(r.flow.initialWindowThere)
}

// CheckInitialWindow is used to handle the race
// condition where the flow control is initialised
// before the server has received any updates to
// the initial tranfer window sent by the client.
//
// The transfer window is updated retroactively,
// if necessary.
func (f *flowControl) CheckInitialWindow() {
	if f.stream == nil || f.stream.Conn() == nil {
		return
	}

	newWindow, err := f.stream.Conn().InitialWindowSize()
	if err != nil {
		log.Println(err)
		return
	}

	if f.initialWindow != newWindow {
		if f.initialWindow > newWindow {
			f.transferWindow = int64(newWindow - f.sent)
		} else if f.initialWindow < newWindow {
			f.transferWindow += int64(newWindow - f.initialWindow)
		}
		if f.transferWindow <= 0 {
			f.constrained = true
		}
		f.initialWindow = newWindow
	}
}

// Close nils any references held by the flowControl.
func (f *flowControl) Close() {
	f.buffer = nil
	f.stream = nil
}

// Flush is used to send buffered data to
// the connection, if the transfer window
// will allow. Flush does not guarantee
// that any or all buffered data will be
// sent with a single flush.
func (f *flowControl) Flush() {
	f.CheckInitialWindow()
	if !f.constrained || f.transferWindow == 0 {
		return
	}

	out := make([]byte, 0, f.transferWindow)
	left := f.transferWindow
	for i := 0; i < len(f.buffer); i++ {
		if l := int64(len(f.buffer[i])); l <= left {
			out = append(out, f.buffer[i]...)
			left -= l
			f.buffer = f.buffer[1:]
		} else {
			out = append(out, f.buffer[i][:left]...)
			f.buffer[i] = f.buffer[i][left:]
			left = 0
		}

		if left == 0 {
			break
		}
	}

	f.transferWindow -= int64(len(out))

	if f.transferWindow > 0 {
		f.constrained = false
		log.Printf("Stream %d is no longer constrained.\n", f.streamID)
	}

	dataFrame := new(dataFrameV3)
	dataFrame.StreamID = f.streamID
	dataFrame.Data = out

	f.output <- dataFrame
}

// Paused indicates whether there is data buffered.
// A Stream should not be closed until after the
// last data has been sent and then Paused returns
// false.
func (f *flowControl) Paused() bool {
	f.CheckInitialWindow()
	return f.constrained
}

// Receive is called when data is received from
// the other endpoint. This ensures that they
// conform to the transfer window, regrows the
// window, and sends errors if necessary.
func (f *flowControl) Receive(data []byte) {
	// The transfer window shouldn't already be negative.
	if f.transferWindowThere < 0 {
		rst := new(rstStreamFrameV3)
		rst.StreamID = f.streamID
		rst.Status = RST_STREAM_FLOW_CONTROL_ERROR
		f.output <- rst
	}

	// Update the window.
	f.transferWindowThere -= int64(len(data))

	// Regrow the window if it's half-empty.
	delta := f.flowControl.ReceiveData(f.streamID, f.initialWindowThere, f.transferWindowThere)
	if delta != 0 {
		grow := new(windowUpdateFrameV3)
		grow.StreamID = f.streamID
		grow.DeltaWindowSize = delta
		f.output <- grow
		f.transferWindowThere += int64(grow.DeltaWindowSize)
	}
}

// UpdateWindow is called when an UPDATE_WINDOW frame is received,
// and performs the growing of the transfer window.
func (f *flowControl) UpdateWindow(deltaWindowSize uint32) error {
	f.Lock()
	defer f.Unlock()

	if int64(deltaWindowSize)+f.transferWindow > MAX_TRANSFER_WINDOW_SIZE {
		return errors.New("Error: WINDOW_UPDATE delta window size overflows transfer window size.")
	}

	// Grow window and flush queue.
	debug.Printf("Flow: Growing window in stream %d by %d bytes.\n", f.streamID, deltaWindowSize)
	f.transferWindow += int64(deltaWindowSize)

	f.Flush()
	return nil
}

// Write is used to send data to the connection. This
// takes care of the windowing. Although data may be
// buffered, rather than actually sent, this is not
// visible to the caller.
func (f *flowControl) Write(data []byte) (int, error) {
	l := len(data)
	if l == 0 {
		return 0, nil
	}

	if f.buffer == nil || f.stream == nil {
		return 0, errors.New("Error: Stream closed.")
	}

	// Transfer window processing.
	f.CheckInitialWindow()
	if f.constrained {
		f.Flush()
	}
	f.Lock()
	var window uint32
	if f.transferWindow < 0 {
		window = 0
	} else {
		window = uint32(f.transferWindow)
	}
	f.Unlock()

	if uint32(len(data)) > window {
		f.buffer = append(f.buffer, data[window:])
		data = data[:window]
		f.sent += window
		f.transferWindow -= int64(window)
		f.constrained = true
		log.Printf("Stream %d is now constrained.\n", f.streamID)
	}

	if len(data) == 0 {
		return l, nil
	}

	dataFrame := new(dataFrameV3)
	dataFrame.StreamID = f.streamID
	dataFrame.Data = data

	f.output <- dataFrame
	return l, nil
}
