package server

import (
	"bufio"
	"context"
	"fmt"
	"gogopher/src/configuration"
	"gogopher/src/core"
	"gogopher/src/security"
	"gogopher/src/selectorHandler"
	"io"
	"log"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const GophermapTemplateName = ".gophermap"

var cfg = configuration.GetConfiguration()

// gopherServer is a simple gopherServer
type gopherServer struct {
	Hostname               string
	BindAddr               string
	Port                   string
	Handler                core.HandlerFunc
	Middlewares            []core.Middleware
	RequestTimeoutDuration time.Duration
	RequestMaximumBytes    int
	GopherRoot             string

	startTime        time.Time
	stopTime         time.Time
	totalConnections int

	selectors       []selectorHandler.ISelectorHandler
	mu              sync.Mutex
	listener        net.Listener
	conns           map[net.Conn]struct{}
	clientWaitGroup sync.WaitGroup
	acceptDone      chan struct{}
	stopOnce        sync.Once
	stopRequested   bool
}

func (s *gopherServer) GetCurrentServerInfo() core.ServerInfoView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return core.ServerInfoView{
		HostName:                s.Hostname,
		Port:                    s.Port,
		StartTime:               s.startTime,
		Uptime:                  time.Now().Sub(s.startTime),
		TotalConnections:        s.totalConnections,
		CurrentConnections:      len(s.conns),
		OS:                      runtime.GOOS,
		Architecture:            runtime.GOARCH,
		NumCpus:                 runtime.NumCPU(),
		GopherRoot:              s.GopherRoot,
		GophermapTemplateName:   cfg.GophermapTemplateName,
		ServerSoftwareName:      cfg.ServerSoftwareName,
		ServerSoftwareVersion:   cfg.ServerSoftwareVersion,
		ServerSoftwareCopyright: cfg.ServerSoftwareCopyright,
		ServerSoftwareLicense:   cfg.ServerSoftwareLicense,
	}
}

func NewServer(
	hostname string,
	bindAddr string,
	port string,
	gopherRoot string,
	requestTimeoutDuration time.Duration,
	requestMaximumBytea int) (core.IServer, error) {

	if strings.TrimSpace(hostname) == "" {
		hostname = "localhost"
	}
	if strings.TrimSpace(bindAddr) == "" {
		bindAddr = "0.0.0.0"
	}
	if strings.TrimSpace(port) == "" {
		port = "70"
	}
	if strings.TrimSpace(gopherRoot) == "" {
		return nil, fmt.Errorf("gopherRoot cannot be empty")
	}

	return &gopherServer{
		Hostname:               hostname,
		BindAddr:               bindAddr,
		Port:                   port,
		GopherRoot:             gopherRoot,
		RequestTimeoutDuration: requestTimeoutDuration,
		RequestMaximumBytes:    requestMaximumBytea,
		Middlewares:            []core.Middleware{},
		selectors:              []selectorHandler.ISelectorHandler{},
		clientWaitGroup:        sync.WaitGroup{},
		acceptDone:             make(chan struct{}),
		stopRequested:          false,
	}, nil
}

func (s *gopherServer) Start() error {
	ln, err := net.Listen("tcp", s.BindAddr+":"+s.Port)
	if err != nil {
		return err
	}
	log.Println("GoGopher: listening on ", s.BindAddr+":"+s.Port)

	s.mu.Lock()
	s.Handler = s.useMiddleware(s.serveSelector)
	s.selectors = []selectorHandler.ISelectorHandler{
		selectorHandler.NewDirectorySelectorHandler(s),
		selectorHandler.NewFileSelectorHandler(s)}
	s.listener = ln
	s.conns = make(map[net.Conn]struct{})
	s.startTime = time.Now()
	s.stopRequested = false
	s.mu.Unlock()
	go s.acceptLoop()
	return nil
}

func (s *gopherServer) AddMiddleware(middleware core.Middleware) {
	s.Middlewares = append(s.Middlewares, middleware)
}

func (s *gopherServer) IsStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener != nil
}

func (s *gopherServer) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() {
		log.Println("Shutting down server...")
		s.mu.Lock()
		ln := s.listener
		s.stopRequested = true
		s.mu.Unlock()
		if ln != nil {
			_ = ln.Close()
		}
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.acceptDone:
	}
	s.mu.Lock()
	s.listener = nil
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
	done := make(chan struct{})
	go func() {
		s.clientWaitGroup.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		s.mu.Lock()
		s.stopTime = time.Now()
		s.mu.Unlock()
		return nil
	}
}

func (s *gopherServer) acceptLoop() {
	defer close(s.acceptDone)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.stopRequested {
				log.Println("Server stopped")
			} else {
				log.Println("Error accepting connection:", err)
			}
			return
		}
		s.clientWaitGroup.Add(1)
		s.trackConn(conn)
		go s.handleClientConnection(conn, s.Handler)
	}
}

func (s *gopherServer) trackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[c] = struct{}{}
	var cons = len(s.conns)
	s.totalConnections += cons
	log.Println("New connection:", c.RemoteAddr(), "total:", cons)

}

func (s *gopherServer) untrackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, c)
	var cons = len(s.conns)
	log.Println("Connection closed:", c.RemoteAddr(), "remaining:", cons)
}

func (s *gopherServer) useMiddleware(h core.HandlerFunc) core.HandlerFunc {
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		h = s.Middlewares[i](h)
	}
	return h
}

func (s *gopherServer) handleClientConnection(conn net.Conn, handler core.HandlerFunc) {
	defer s.clientWaitGroup.Done()
	defer s.untrackConn(conn)
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Println("Error closing connection:", err)
		}
	}(conn)
	if err := s.processRequest(handler, conn, s.GopherRoot); err != nil {
		log.Println("Error handling request:", err)
	}
	log.Println("Request processed:", conn.RemoteAddr())
}

func (s *gopherServer) processRequest(handler core.HandlerFunc, conn net.Conn, gopherRoot string) error {
	if s.RequestTimeoutDuration > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(s.RequestTimeoutDuration))
	}
	ctx := context.Background()
	if s.RequestTimeoutDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.RequestTimeoutDuration)
		defer cancel()
		// Apply the timeout to the connection
		_ = conn.SetReadDeadline(time.Now().Add(s.RequestTimeoutDuration))
	}

	reader := bufio.NewReader(io.LimitReader(conn, int64(s.RequestMaximumBytes)))
	req, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("request size exceeded maximum %d bytes", s.RequestMaximumBytes)
		}
		return fmt.Errorf("cannot read request: %w", err)
	}
	var selector = strings.TrimSpace(req)
	log.Println("Selector:", selector)
	return handler(conn, gopherRoot, selector, s.RequestTimeoutDuration)
}

func (s *gopherServer) serveSelector(conn net.Conn, rootDir string, selector string, timeOut time.Duration) error {
	clean := filepath.Clean("/" + selector) // force selector to be relative
	cleanPath := filepath.Join(rootDir, clean)
	realRoot, _ := filepath.Abs(rootDir)
	realPath, _ := filepath.Abs(cleanPath)
	if !strings.HasPrefix(realPath, realRoot) {
		if _, err := io.WriteString(conn, "3Access denied.\r\n"); err != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
		return nil
	}
	if err := security.AssertFileSystemAccess(realPath); err != nil {
		if _, err := io.WriteString(conn, "3Not found.\t\terror.host\t1\r\n"); err != nil {
		}
		return fmt.Errorf("cannot access file system: %w", err)
	}

	for _, sh := range s.selectors {
		res, err := sh.Select(conn, s.GopherRoot, selector, timeOut)
		if err != nil {
			return fmt.Errorf("cannot select: %w", err)
		}
		if res.Handled {
			return nil
		}
	}

	// Not found
	if _, err := fmt.Fprintf(conn, "3Not found.\r\n.\r\n"); err != nil {
		return fmt.Errorf("cannot write error message: %w", err)
	}
	return fmt.Errorf("file not found: %s", cleanPath)
}
