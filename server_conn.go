// Copyright 2013 Jamie Hall. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spdy

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

// NewServerConn is used to create a SPDY connection, using the given
// net.Conn for the underlying connection, and the given http.Server to
// configure the request serving.
func NewServerConn(conn net.Conn, server *http.Server, version float64) (spdyConn Conn, err error) {
	if conn == nil {
		return nil, errors.New("Error: Connection initialised with nil net.conn.")
	}
	if server == nil {
		return nil, errors.New("Error: Connection initialised with nil server.")
	}

	switch version {
	case 3:
		out := new(connV3)
		out.remoteAddr = conn.RemoteAddr().String()
		out.server = server
		out.conn = conn
		out.buf = bufio.NewReader(conn)
		if tlsConn, ok := conn.(*tls.Conn); ok {
			out.tlsState = new(tls.ConnectionState)
			*out.tlsState = tlsConn.ConnectionState()
		}
		out.streams = make(map[StreamID]Stream)
		out.output = [8]chan Frame{}
		out.output[0] = make(chan Frame)
		out.output[1] = make(chan Frame)
		out.output[2] = make(chan Frame)
		out.output[3] = make(chan Frame)
		out.output[4] = make(chan Frame)
		out.output[5] = make(chan Frame)
		out.output[6] = make(chan Frame)
		out.output[7] = make(chan Frame)
		out.pings = make(map[uint32]chan<- Ping)
		out.nextPingID = 2
		out.compressor = NewCompressor(3)
		out.decompressor = NewDecompressor(3)
		out.receivedSettings = make(Settings)
		out.lastPushStreamID = 0
		out.lastRequestStreamID = 0
		out.oddity = 0
		out.initialWindowSize = DEFAULT_INITIAL_WINDOW_SIZE
		out.requestStreamLimit = newStreamLimit(DEFAULT_STREAM_LIMIT)
		out.pushStreamLimit = newStreamLimit(NO_STREAM_LIMIT)
		out.vectorIndex = 8
		out.certificates = make(map[uint16][]*x509.Certificate, 8)
		if out.tlsState != nil && out.tlsState.PeerCertificates != nil {
			out.certificates[1] = out.tlsState.PeerCertificates
		}
		out.stop = make(chan bool)
		out.init = func() {
			// Initialise the connection by sending the connection settings.
			settings := new(settingsFrameV3)
			settings.Settings = defaultSPDYServerSettings(3, DEFAULT_STREAM_LIMIT)
			out.output[0] <- settings
		}
		if d := server.ReadTimeout; d != 0 {
			out.SetReadTimeout(d)
		}
		if d := server.WriteTimeout; d != 0 {
			out.SetWriteTimeout(d)
		}
		out.flowControl = DefaultFlowControl(DEFAULT_INITIAL_WINDOW_SIZE)
		out.pushedResources = make(map[Stream]map[string]struct{})

		return out, nil

	case 3.1:
		out := new(connV3)
		out.subversion = 1
		out.remoteAddr = conn.RemoteAddr().String()
		out.server = server
		out.conn = conn
		out.buf = bufio.NewReader(conn)
		if tlsConn, ok := conn.(*tls.Conn); ok {
			out.tlsState = new(tls.ConnectionState)
			*out.tlsState = tlsConn.ConnectionState()
		}
		out.streams = make(map[StreamID]Stream)
		out.output = [8]chan Frame{}
		out.output[0] = make(chan Frame)
		out.output[1] = make(chan Frame)
		out.output[2] = make(chan Frame)
		out.output[3] = make(chan Frame)
		out.output[4] = make(chan Frame)
		out.output[5] = make(chan Frame)
		out.output[6] = make(chan Frame)
		out.output[7] = make(chan Frame)
		out.pings = make(map[uint32]chan<- Ping)
		out.nextPingID = 2
		out.compressor = NewCompressor(3)
		out.decompressor = NewDecompressor(3)
		out.receivedSettings = make(Settings)
		out.lastPushStreamID = 0
		out.lastRequestStreamID = 0
		out.oddity = 0
		out.initialWindowSize = DEFAULT_INITIAL_WINDOW_SIZE
		out.connectionWindowSize = DEFAULT_INITIAL_WINDOW_SIZE
		out.requestStreamLimit = newStreamLimit(DEFAULT_STREAM_LIMIT)
		out.pushStreamLimit = newStreamLimit(NO_STREAM_LIMIT)
		out.vectorIndex = 8
		out.stop = make(chan bool)
		out.init = func() {
			// Initialise the connection by sending the connection settings.
			settings := new(settingsFrameV3)
			settings.Settings = defaultSPDYServerSettings(3, DEFAULT_STREAM_LIMIT)
			out.output[0] <- settings
		}
		if d := server.ReadTimeout; d != 0 {
			out.SetReadTimeout(d)
		}
		if d := server.WriteTimeout; d != 0 {
			out.SetWriteTimeout(d)
		}
		out.flowControl = DefaultFlowControl(DEFAULT_INITIAL_WINDOW_SIZE)
		out.pushedResources = make(map[Stream]map[string]struct{})
		out.initialWindowSizeThere = out.flowControl.InitialWindowSize()
		out.connectionWindowSizeThere = int64(out.initialWindowSizeThere)

		return out, nil

	case 2:
		out := new(connV2)
		out.remoteAddr = conn.RemoteAddr().String()
		out.server = server
		out.conn = conn
		out.buf = bufio.NewReader(conn)
		if tlsConn, ok := conn.(*tls.Conn); ok {
			out.tlsState = new(tls.ConnectionState)
			*out.tlsState = tlsConn.ConnectionState()
		}
		out.streams = make(map[StreamID]Stream)
		out.output = [8]chan Frame{}
		out.output[0] = make(chan Frame)
		out.output[1] = make(chan Frame)
		out.output[2] = make(chan Frame)
		out.output[3] = make(chan Frame)
		out.output[4] = make(chan Frame)
		out.output[5] = make(chan Frame)
		out.output[6] = make(chan Frame)
		out.output[7] = make(chan Frame)
		out.pings = make(map[uint32]chan<- Ping)
		out.nextPingID = 2
		out.compressor = NewCompressor(2)
		out.decompressor = NewDecompressor(2)
		out.receivedSettings = make(Settings)
		out.lastPushStreamID = 0
		out.lastRequestStreamID = 0
		out.oddity = 0
		out.initialWindowSize = DEFAULT_INITIAL_WINDOW_SIZE
		out.requestStreamLimit = newStreamLimit(DEFAULT_STREAM_LIMIT)
		out.pushStreamLimit = newStreamLimit(NO_STREAM_LIMIT)
		out.stop = make(chan bool)
		out.init = func() {
			// Initialise the connection by sending the connection settings.
			settings := new(settingsFrameV2)
			settings.Settings = defaultSPDYServerSettings(2, DEFAULT_STREAM_LIMIT)
			out.output[0] <- settings
		}
		if d := server.ReadTimeout; d != 0 {
			out.SetReadTimeout(d)
		}
		if d := server.WriteTimeout; d != 0 {
			out.SetWriteTimeout(d)
		}
		out.pushedResources = make(map[Stream]map[string]struct{})

		return out, nil

	default:
		return nil, errors.New("Error: Unsupported SPDY version.")
	}
}

// AddSPDY adds SPDY support to srv, and must be called before srv begins serving.
func AddSPDY(srv *http.Server) {
	if srv == nil {
		return
	}

	npnStrings := npn()
	if len(npnStrings) <= 1 {
		return
	}
	if srv.TLSConfig == nil {
		srv.TLSConfig = new(tls.Config)
	}
	if srv.TLSConfig.NextProtos == nil {
		srv.TLSConfig.NextProtos = npnStrings
	} else {
		// Collect compatible alternative protocols.
		others := make([]string, 0, len(srv.TLSConfig.NextProtos))
		for _, other := range srv.TLSConfig.NextProtos {
			if !strings.Contains(other, "spdy/") && !strings.Contains(other, "http/") {
				others = append(others, other)
			}
		}

		// Start with spdy.
		srv.TLSConfig.NextProtos = make([]string, 0, len(others)+len(npnStrings))
		srv.TLSConfig.NextProtos = append(srv.TLSConfig.NextProtos, npnStrings[:len(npnStrings)-1]...)

		// Add the others.
		srv.TLSConfig.NextProtos = append(srv.TLSConfig.NextProtos, others...)
		srv.TLSConfig.NextProtos = append(srv.TLSConfig.NextProtos, "http/1.1")
	}
	if srv.TLSNextProto == nil {
		srv.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	}
	for _, str := range npnStrings {
		switch str {
		case "spdy/2":
			srv.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 2)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		case "spdy/3":
			srv.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 3)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		case "spdy/3.1":
			srv.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 3.1)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		}
	}
}

// ErrNotSPDY indicates that a SPDY-specific feature was attempted
// with a ResponseWriter using a non-SPDY connection.
var ErrNotSPDY = errors.New("Error: Not a SPDY connection.")

// ErrNotConnected indicates that a SPDY-specific feature was
// attempted with a Client not connected to the given server.
var ErrNotConnected = errors.New("Error: Not connected to given server.")

// GetPriority is used to identify the request priority of the
// given stream. This can be used to manually enforce stream
// priority, although this is already performed by the
// library.
// If the underlying connection is using HTTP, and not SPDY,
// GetPriority will return the ErrNotSPDY error.
//
// A simple example of finding a stream's priority is:
//
//      import (
//              "github.com/SlyMarbo/spdy"
//              "log"
//              "net/http"
//      )
//
//      func httpHandler(w http.ResponseWriter, r *http.Request) {
//							priority, err := spdy.GetPriority(w)
//              if err != nil {
//                      // Non-SPDY connection.
//              } else {
//                      log.Println(priority)
//              }
//      }
//
//      func main() {
//              http.HandleFunc("/", httpHandler)
//              log.Printf("About to listen on 10443. Go to https://127.0.0.1:10443/")
//              err := spdy.ListenAndServeTLS(":10443", "cert.pem", "key.pem", nil)
//              if err != nil {
//                      log.Fatal(err)
//              }
//      }
func GetPriority(w http.ResponseWriter) (int, error) {
	if stream, ok := w.(*serverStreamV3); ok {
		return int(stream.priority), nil
	}
	if stream, ok := w.(*serverStreamV2); ok {
		return int(stream.priority), nil
	}
	return 0, ErrNotSPDY
}

// ListenAndServeTLS listens on the TCP network address addr
// and then calls Serve with handler to handle requests on
// incoming connections.  Handler is typically nil, in which
// case the DefaultServeMux is used. Additionally, files
// containing a certificate and matching private key for the
// server must be provided. If the certificate is signed by
// a certificate authority, the certFile should be the
// concatenation of the server's certificate followed by the
// CA's certificate.
//
// A trivial example server is:
//
//      import (
//              "github.com/SlyMarbo/spdy"
//              "log"
//              "net/http"
//      )
//
//      func httpHandler(w http.ResponseWriter, req *http.Request) {
//              w.Header().Set("Content-Type", "text/plain")
//              w.Write([]byte("This is an example server.\n"))
//      }
//
//      func main() {
//              http.HandleFunc("/", httpHandler)
//              log.Printf("About to listen on 10443. Go to https://127.0.0.1:10443/")
//              err := spdy.ListenAndServeTLS(":10443", "cert.pem", "key.pem", nil)
//              if err != nil {
//                      log.Fatal(err)
//              }
//      }
//
// One can use generate_cert.go in crypto/tls to generate cert.pem and key.pem.
func ListenAndServeTLS(addr string, certFile string, keyFile string, handler http.Handler) error {
	npnStrings := npn()
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		TLSConfig: &tls.Config{
			NextProtos: npnStrings,
		},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	for _, str := range npnStrings {
		switch str {
		case "spdy/2":
			server.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 2)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		case "spdy/3":
			server.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 3)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		case "spdy/3.1":
			server.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 3.1)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		}
	}

	return server.ListenAndServeTLS(certFile, keyFile)
}

// ListenAndServeTLS listens on the TCP network address addr
// and then calls Serve with handler to handle requests on
// incoming connections.  Handler is typically nil, in which
// case the DefaultServeMux is used. Additionally, files
// containing a certificate and matching private key for the
// server must be provided. If the certificate is signed by
// a certificate authority, the certFile should be the
// concatenation of the server's certificate followed by the
// CA's certificate.
//
// IMPORTANT NOTE: Unlike spdy.ListenAndServeTLS, this function
// will ONLY serve SPDY. HTTPS requests are refused.
//
// A trivial example server is:
//
//      import (
//              "github.com/SlyMarbo/spdy"
//              "log"
//              "net/http"
//      )
//
//      func httpHandler(w http.ResponseWriter, req *http.Request) {
//              w.Header().Set("Content-Type", "text/plain")
//              w.Write([]byte("This is an example server.\n"))
//      }
//
//      func main() {
//              http.HandleFunc("/", httpHandler)
//              log.Printf("About to listen on 10443. Go to https://127.0.0.1:10443/")
//              err := spdy.ListenAndServeSPDY(":10443", "cert.pem", "key.pem", nil)
//              if err != nil {
//                      log.Fatal(err)
//              }
//      }
//
// One can use generate_cert.go in crypto/tls to generate cert.pem and key.pem.
func ListenAndServeSPDY(addr string, certFile string, keyFile string, handler http.Handler) error {
	npnStrings := npn()
	if addr == "" {
		addr = ":https"
	}
	if handler == nil {
		handler = http.DefaultServeMux
	}
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		TLSConfig: &tls.Config{
			NextProtos:   npnStrings,
			Certificates: make([]tls.Certificate, 1),
		},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	for _, str := range npnStrings {
		switch str {
		case "spdy/2":
			server.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 2)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		case "spdy/3":
			server.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 3)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		case "spdy/3.1":
			server.TLSNextProto[str] = func(s *http.Server, tlsConn *tls.Conn, handler http.Handler) {
				conn, err := NewServerConn(tlsConn, s, 3.1)
				if err != nil {
					log.Println(err)
					return
				}
				conn.Run()
				conn = nil
				runtime.GC()
			}
		}
	}

	var err error
	server.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(conn, server.TLSConfig)
	defer tlsListener.Close()

	// Main loop
	var tempDelay time.Duration
	for {
		rw, e := tlsListener.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("Accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0
		go serveSPDY(rw, server)
	}
}

func ListenAndServeSPDYNoNPN(addr string, certFile string, keyFile string, handler http.Handler, version float64) error {
	if addr == "" {
		addr = ":https"
	}
	if handler == nil {
		handler = http.DefaultServeMux
	}
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: make([]tls.Certificate, 1),
		},
	}

	var err error
	server.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(conn, server.TLSConfig)
	defer tlsListener.Close()

	// Main loop
	var tempDelay time.Duration
	for {
		rw, e := tlsListener.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("Accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0
		go serveSPDYNoNPN(rw, server, version)
	}
}

func serveSPDY(conn net.Conn, srv *http.Server) {
	defer func() {
		if v := recover(); v != nil {
			const size = 4096
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("panic serving %v: %v\n%s", conn.RemoteAddr(), v, buf)
		}
	}()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok { // Only allow TLS connections.
		return
	}

	if d := srv.ReadTimeout; d != 0 {
		conn.SetReadDeadline(time.Now().Add(d))
	}
	if d := srv.WriteTimeout; d != 0 {
		conn.SetWriteDeadline(time.Now().Add(d))
	}
	if err := tlsConn.Handshake(); err != nil {
		return
	}

	tlsState := new(tls.ConnectionState)
	*tlsState = tlsConn.ConnectionState()
	proto := tlsState.NegotiatedProtocol
	if fn := srv.TLSNextProto[proto]; fn != nil {
		fn(srv, tlsConn, nil)
	}
	return
}

func serveSPDYNoNPN(conn net.Conn, srv *http.Server, version float64) {
	defer func() {
		if v := recover(); v != nil {
			const size = 4096
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("panic serving %v: %v\n%s", conn.RemoteAddr(), v, buf)
		}
	}()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok { // Only allow TLS connections.
		return
	}

	if d := srv.ReadTimeout; d != 0 {
		conn.SetReadDeadline(time.Now().Add(d))
	}
	if d := srv.WriteTimeout; d != 0 {
		conn.SetWriteDeadline(time.Now().Add(d))
	}
	if err := tlsConn.Handshake(); err != nil {
		return
	}

	serverConn, err := NewServerConn(tlsConn, srv, version)
	if err != nil {
		log.Println(err)
		return
	}
	serverConn.Run()
	serverConn = nil
	runtime.GC()

	return
}

// PingClient is used to send PINGs with SPDY servers.
// PingClient takes a ResponseWriter and returns a channel on
// which a spdy.Ping will be sent when the PING response is
// received. If the channel is closed before a spdy.Ping has
// been sent, this indicates that the PING was unsuccessful.
//
// If the underlying connection is using HTTP, and not SPDY,
// PingClient will return the ErrNotSPDY error.
//
// A simple example of sending a ping is:
//
//      import (
//              "github.com/SlyMarbo/spdy"
//              "log"
//              "net/http"
//      )
//
//      func httpHandler(w http.ResponseWriter, req *http.Request) {
//              ping, err := spdy.PingClient(w)
//              if err != nil {
//                      // Non-SPDY connection.
//              } else {
//                      resp, ok <- ping
//                      if ok {
//                              // Ping was successful.
//                      }
//              }
//
//      }
//
//      func main() {
//              http.HandleFunc("/", httpHandler)
//              log.Printf("About to listen on 10443. Go to https://127.0.0.1:10443/")
//              err := spdy.ListenAndServeTLS(":10443", "cert.pem", "key.pem", nil)
//              if err != nil {
//                      log.Fatal(err)
//              }
//      }
func PingClient(w http.ResponseWriter) (<-chan Ping, error) {
	if stream, ok := w.(Stream); !ok {
		return nil, ErrNotSPDY
	} else {
		return stream.Conn().Ping()
	}
}

// PingServer is used to send PINGs with http.Clients using.
// SPDY. PingServer takes a ResponseWriter and returns a
// channel onwhich a spdy.Ping will be sent when the PING
// response is received. If the channel is closed before a
// spdy.Ping has been sent, this indicates that the PING was
// unsuccessful.
//
// If the underlying connection is using HTTP, and not SPDY,
// PingServer will return the ErrNotSPDY error.
//
// If an underlying connection has not been made to the given
// server, PingServer will return the ErrNotConnected error.
//
// A simple example of sending a ping is:
//
//      import (
//              "github.com/SlyMarbo/spdy"
//              "net/http"
//      )
//
//      func main() {
//              resp, err := http.Get("https://example.com/")
//
//              // ...
//
//              ping, err := spdy.PingServer(http.DefaultClient, "https://example.com")
//              if err != nil {
//                      // No SPDY connection.
//              } else {
//                      resp, ok <- ping
//                      if ok {
//                              // Ping was successful.
//                      }
//              }
//      }
func PingServer(c http.Client, server string) (<-chan Ping, error) {
	if transport, ok := c.Transport.(*Transport); !ok {
		return nil, ErrNotSPDY
	} else {
		u, err := url.Parse(server)
		if err != nil {
			return nil, err
		}
		// Make sure the URL host contains the port.
		if !strings.Contains(u.Host, ":") {
			switch u.Scheme {
			case "http":
				u.Host += ":80"

			case "https":
				u.Host += ":443"
			}
		}
		conn, ok := transport.spdyConns[u.Host]
		if !ok || conn == nil {
			return nil, ErrNotConnected
		}
		return conn.Ping()
	}
}

// Push is used to send server pushes with SPDY servers.
// Push takes a ResponseWriter and the url of the resource
// being pushed, and returns a ResponseWriter to which the
// push should be written.
//
// If the underlying connection is using HTTP, and not SPDY,
// Push will return the ErrNotSPDY error.
//
// A simple example of pushing a file is:
//
//      import (
//              "github.com/SlyMarbo/spdy"
//              "log"
//              "net/http"
//      )
//
//      func httpHandler(w http.ResponseWriter, r *http.Request) {
//              path := r.URL.Scheme + "://" + r.URL.Host + "/javascript.js"
//              push, err := spdy.Push(w, path)
//              if err != nil {
//                      // Non-SPDY connection.
//              } else {
//                      http.ServeFile(push, r, "./javascript.js") // Push the given file.
//											push.Finish()                              // Finish the stream once used.
//              }
//
//      }
//
//      func main() {
//              http.HandleFunc("/", httpHandler)
//              log.Printf("About to listen on 10443. Go to https://127.0.0.1:10443/")
//              err := spdy.ListenAndServeTLS(":10443", "cert.pem", "key.pem", nil)
//              if err != nil {
//                      log.Fatal(err)
//              }
//      }
func Push(w http.ResponseWriter, url string) (PushStream, error) {
	if stream, ok := w.(Stream); !ok {
		return nil, ErrNotSPDY
	} else {
		return stream.Conn().Push(url, stream)
	}
}

// SetFlowControl can be used to set the flow control mechanism on
// the underlying SPDY connection.
func SetFlowControl(w http.ResponseWriter, f FlowControl) error {
	if stream, ok := w.(Stream); !ok {
		return ErrNotSPDY
	} else {
		return stream.Conn().SetFlowControl(f)
	}
}

// SPDYversion returns the SPDY version being used in the underlying
// connection used by the given http.ResponseWriter. This is 0 for
// connections not using SPDY.
func SPDYversion(w http.ResponseWriter) float64 {
	if stream, ok := w.(Stream); ok {
		switch stream.Conn().(type) {
		case *connV3:
			switch stream.Conn().(*connV3).subversion {
			case 0:
				return 3
			case 1:
				return 3.1
			default:
				return 0
			}

		case *connV2:
			return 2

		default:
			return 0
		}
	}
	return 0
}

// UsingSPDY indicates whether a given ResponseWriter is using SPDY.
func UsingSPDY(w http.ResponseWriter) bool {
	_, ok := w.(Stream)
	return ok
}
