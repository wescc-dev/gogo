package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"gogopher/src/configuration"
	"gogopher/src/core"
	"gogopher/src/security"
	"gogopher/src/selectorHandler"
	"io"
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
	Title                  string
	Hostname               string
	BindAddr               string
	Port                   string
	Handler                core.HandlerFunc
	Middlewares            []core.Middleware
	RequestTimeoutDuration time.Duration
	RequestMaximumBytes    int
	GopherRoot             string
	TLSCertFile            string
	TLSKeyFile             string

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

func NewServer(
	Title string,
	hostname string,
	bindAddr string,
	port string,
	gopherRoot string,
	requestTimeoutDuration time.Duration,
	requestMaximumBytea int,
	tlsCertFile string,
	tlsKeyFile string) (core.IServer, error) {

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
	if (strings.TrimSpace(tlsCertFile) == "") != (strings.TrimSpace(tlsKeyFile) == "") {
		return nil, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be configured together")
	}

	return &gopherServer{
		Title:                  Title,
		Hostname:               hostname,
		BindAddr:               bindAddr,
		Port:                   port,
		GopherRoot:             gopherRoot,
		TLSCertFile:            tlsCertFile,
		TLSKeyFile:             tlsKeyFile,
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
	address := s.BindAddr + ":" + s.Port
	var ln net.Listener
	var err error
	if strings.TrimSpace(s.TLSCertFile) != "" {
		certificate, loadErr := tls.LoadX509KeyPair(s.TLSCertFile, s.TLSKeyFile)
		if loadErr != nil {
			return fmt.Errorf("load TLS certificate and key: %w", loadErr)
		}
		ln, err = tls.Listen("tcp", address, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
		if err == nil {
			core.SystemLog("TLS ENABLED: ", s.TLSCertFile, " ", s.TLSKeyFile, "")
		}
	} else {
		ln, err = net.Listen("tcp", address)
		core.SystemLog("TLS DISABLED")
	}
	if err != nil {
		return err
	}
	core.SystemLog("GoGopher: listening on ", address)

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
		core.SystemLog("Shutting down server...")
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
func (s *gopherServer) GetCurrentServerInfo() core.ServerInfoView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return core.ServerInfoView{
		Title:                   s.Title,
		HostName:                s.Hostname,
		Port:                    s.Port,
		TLSEnabled:              s.TLSCertFile != "",
		StartTime:               s.startTime,
		Uptime:                  time.Now().Sub(s.startTime),
		TotalConnections:        s.totalConnections,
		CurrentConnections:      len(s.conns),
		OS:                      runtime.GOOS,
		Architecture:            runtime.GOARCH,
		NumCpus:                 runtime.NumCPU(),
		GopherRoot:              s.GopherRoot,
		GophermapTemplateName:   cfg.GophermapTemplateName,
		ServerSoftwareName:      cfg.Metadata.AppName,
		ServerSoftwareVersion:   cfg.Metadata.Version,
		ServerSoftwareCopyright: cfg.Metadata.Copyright,
		ServerSoftwareLicense:   cfg.Metadata.License,
	}
}

func (s *gopherServer) acceptLoop() {
	defer close(s.acceptDone)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.stopRequested {
				core.SystemLog("Server stopped.")
			} else {
				core.SystemLog("Error accepting connection:", err)
			}
			return
		}
		s.clientWaitGroup.Add(1)
		go s.handleClientConnection(conn, s.Handler)
	}
}

func (s *gopherServer) trackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[c] = struct{}{}
	var cons = len(s.conns)
	s.totalConnections += cons
	host, _, _ := net.SplitHostPort(c.RemoteAddr().String())
	core.SystemLog("New connection:", host, "total:", cons)

}

func (s *gopherServer) untrackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, c)
	var cons = len(s.conns)
	core.SystemLog("Connection closed:", c.RemoteAddr(), "remaining:", cons)
}

func (s *gopherServer) useMiddleware(h core.HandlerFunc) core.HandlerFunc {
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		h = s.Middlewares[i](h)
	}
	return h
}

func (s *gopherServer) handleClientConnection(conn net.Conn, handler core.HandlerFunc) {
	s.trackConn(conn)
	defer s.clientWaitGroup.Done()
	defer s.untrackConn(conn)
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			core.SystemLog("Error closing connection:", err)
		}
	}(conn)
	if err := s.processRequest(handler, conn, s.GopherRoot); err != nil {
		core.SystemLog("Error handling request:", err)
	}
	core.SystemLog("Request processed:", conn.RemoteAddr())
}

func (s *gopherServer) processRequest(handler core.HandlerFunc, conn net.Conn, gopherRoot string) error {
	if s.RequestTimeoutDuration > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(s.RequestTimeoutDuration))
	}
	maxBytes := int64(s.RequestMaximumBytes)
	selector, err := s.readSelector(conn, maxBytes, s.RequestTimeoutDuration)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if s.RequestTimeoutDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.RequestTimeoutDuration)
		defer cancel()
		// Apply the timeout to the connection
		_ = conn.SetReadDeadline(time.Now().Add(s.RequestTimeoutDuration))
	}
	requestContext := core.NewRequestContext(ctx, &core.Request{
		Conn:     conn,
		RootDir:  gopherRoot,
		Selector: selector,
		Timeout:  s.RequestTimeoutDuration,
	})
	return handler(requestContext)
}

func (s *gopherServer) readSelector(conn net.Conn, maxBytes int64, timeOut time.Duration) (string, error) {
	reader := bufio.NewReader(io.LimitReader(conn, maxBytes))
	req, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			_ = selectorHandler.WriteErrorToConn(conn, timeOut, "Access denied")

			return "", fmt.Errorf("request size exceeded maximum %d bytes", maxBytes)
		}
		return "", fmt.Errorf("cannot read request: %w", err)
	}
	var selector = strings.TrimSpace(req)
	return selector, nil
}

func (s *gopherServer) serveSelector(ctx *core.RequestContext) error {

	cleanSelector, realPath, err := resolveSelectorPath(ctx.Request.RootDir, ctx.Request.Selector)
	if err != nil {
		_ = selectorHandler.WriteErrorToConn(ctx.Request.Conn, ctx.Request.Timeout, "Access denied")
		return fmt.Errorf("cannot write error message: %w", err)
	}

	// Handlers use the normalized relative selector that passed validation.
	// RootDir remains in its configured form so it cannot leak into selectors
	// generated for clients. Lexical validation intentionally permits symlinks.
	ctx.Request.Selector = cleanSelector
	core.ContextLog(ctx, "Serving selector:", ctx.Request.Selector)

	if err := security.AssertFileSystemAccess(realPath); err != nil {
		_ = selectorHandler.WriteErrorToConn(ctx.Request.Conn, ctx.Request.Timeout, "Not found")
		return err
	}

	for _, sh := range s.selectors {
		res, err := sh.Select(ctx)
		if err != nil {
			return err
		}
		if res.Handled {
			return nil
		}
	}

	// Not found
	_ = selectorHandler.WriteErrorToConn(ctx.Request.Conn, ctx.Request.Timeout, "Not found")
	return fmt.Errorf("file not found: %s", realPath)
}

func resolveSelectorPath(rootDir string, selector string) (string, string, error) {
	realRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", "", err
	}

	// A leading slash is conventional for Gopher selectors; it does not mean
	// the host filesystem root.
	cleanSelector := filepath.Clean(filepath.FromSlash(strings.TrimLeft(selector, "/")))
	realPath := filepath.Join(realRoot, cleanSelector)
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("selector escapes gopher root")
	}

	return rel, realPath, nil
}
