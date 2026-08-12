package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"gogopher/configuration"
	"gogopher/utility"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type HandlerFunc func(conn net.Conn, rootDir string, selector string, timeout time.Duration) error

type Middleware func(HandlerFunc) HandlerFunc

var cfg = configuration.GetConfiguration()

// server is a simple gopher server
type server struct {
	Hostname         string
	BindAddr         string
	Port             string
	Handler          HandlerFunc
	Middlewares      []Middleware
	ReadWriteTimeout time.Duration
	IdleTimeout      time.Duration
	GopherRoot       string
	startTime        time.Time
	stopTime         time.Time

	mu              sync.Mutex
	listener        net.Listener
	conns           map[net.Conn]struct{}
	clientWaitGroup sync.WaitGroup
	acceptDone      chan struct{}
	stopOnce        sync.Once
}

type IServer interface {
	Start() error
	Stop(ctx context.Context) error
	ConnectionCount() int
	UpTime() time.Duration
	AddMiddleware(middleware Middleware)
	IsStarted() bool
}

func NewServer(
	hostname string,
	bindAddr string,
	port string,
	gopherRoot string,
	idleTimeout time.Duration,
	readWriteTimeout time.Duration) (IServer, error) {

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

	return &server{
		Hostname:         hostname,
		BindAddr:         bindAddr,
		Port:             port,
		GopherRoot:       gopherRoot,
		IdleTimeout:      idleTimeout,
		ReadWriteTimeout: readWriteTimeout,
		Middlewares:      []Middleware{},
		clientWaitGroup:  sync.WaitGroup{},
		acceptDone:       make(chan struct{}),
	}, nil
}

func (s *server) Start() error {
	if err := s.generateGopherMap(); err != nil {
		log.Println("Error generating gophermap:", err)
	}
	ln, err := net.Listen("tcp", s.BindAddr+":"+s.Port)
	if err != nil {
		return err
	}
	log.Println("gopher: listening on ", s.BindAddr+":"+s.Port)

	s.Handler = s.useMiddleware(serveSelector)
	s.mu.Lock()
	s.listener = ln
	s.conns = make(map[net.Conn]struct{})
	s.startTime = time.Now()
	s.mu.Unlock()
	go s.acceptLoop()
	return nil
}

func (s *server) AddMiddleware(middleware Middleware) {
	s.Middlewares = append(s.Middlewares, middleware)
}
func (s *server) IsStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener != nil
}
func (s *server) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() {
		log.Println("Shutting down server...")
		s.mu.Lock()
		ln := s.listener
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

func (s *server) ConnectionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

func (s *server) UpTime() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.stopTime.Sub(s.startTime)
	return t
}

func (s *server) acceptLoop() {
	defer close(s.acceptDone)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			return
		}
		s.clientWaitGroup.Add(1)
		s.trackConn(conn)
		go s.handleClientConnection(conn, s.Handler)
	}
}
func (s *server) handleClientConnection(conn net.Conn, handler HandlerFunc) {
	defer s.clientWaitGroup.Done()
	defer s.untrackConn(conn)
	defer conn.Close()
	if err := s.processRequest(handler, conn, s.GopherRoot); err != nil {
		log.Println("Error handling connection:", err)
	}
}

func (s *server) processRequest(handler HandlerFunc, conn net.Conn, gopherRoot string) error {
	if s.ReadWriteTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(s.ReadWriteTimeout))
	}

	ctx := context.Background()
	if s.IdleTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.IdleTimeout)
		defer cancel()
		// Apply the timeout to the connection
		_ = conn.SetReadDeadline(time.Now().Add(s.IdleTimeout))
	}
	reader := bufio.NewReader(conn)
	req, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	var selector = strings.TrimSpace(req)
	log.Println("Selector:", selector)
	return handler(conn, gopherRoot, selector, s.ReadWriteTimeout)
}

func (s *server) trackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[c] = struct{}{}
	var cons = len(s.conns)
	log.Println("New connection:", c.RemoteAddr(), "total:", cons)

}

func (s *server) untrackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, c)
	var cons = len(s.conns)
	log.Println("Connection closed:", c.RemoteAddr(), "remaining:", cons)
}

func (s *server) useMiddleware(h HandlerFunc) HandlerFunc {
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		h = s.Middlewares[i](h)
	}
	return h
}

func (s *server) generateGopherMap() error {
	srcPath := filepath.Join(s.GopherRoot, ".gophermap")
	outPath := filepath.Join(s.GopherRoot, "gophermap")
	log.Println("Generating gophermap:", srcPath, "->", outPath)

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("cannot open gophermap: %w", err)
	}

	content := string(data)

	// Replace simple tokens first
	tokens := map[string]string{
		"TITLE":   cfg.Title,
		"VERSION": configuration.Version,
		"HOST":    s.Hostname,
		"BIND":    s.BindAddr,
		"=":       "======================================================================",
		"*":       "**********************************************************************",
		"-":       "----------------------------------------------------------------------",
	}

	for key, val := range tokens {
		content = strings.ReplaceAll(content, "{{"+key+"}}", val)
	}

	// Now handle {{ENTRIES}}
	entries, err := s.generateEntries()
	if err != nil {
		return err
	}

	content = strings.ReplaceAll(content, "{{ENTRIES}}", entries)

	// Write output
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write generated gophermap: %w", err)
	}

	return nil
}

func (s *server) generateEntries() (string, error) {
	dir, err := os.ReadDir(s.GopherRoot)
	if err != nil {
		return "", fmt.Errorf("cannot read gopher root: %w", err)
	}

	var entries []string

	for _, e := range dir {
		if s, err := buildGopherEntry(e, "", s.Hostname, s.Port); err != nil {
			log.Println(err)
			continue
		} else {
			entries = append(entries, s)
		}
	}
	var entriesBuf bytes.Buffer
	w := bufio.NewWriter(&entriesBuf)
	sort.Strings(entries)
	for _, entry := range entries {
		_, _ = w.WriteString(entry)
	}
	_ = w.Flush()
	return entriesBuf.String(), nil
}

func serveSelector(conn net.Conn, rootDir string, selector string, timeOut time.Duration) error {
	log.Println("Serving selector:", selector)
	// Empty selector → serve root directory
	if selector == "" {
		serveDirectory(conn, rootDir, selector, timeOut)
		return nil
	}
	clean := filepath.Clean("/" + selector) // force selector to be relative
	path := filepath.Join(rootDir, clean)

	realRoot, _ := filepath.Abs(rootDir)
	realPath, _ := filepath.Abs(path)

	if !strings.HasPrefix(realPath, realRoot) {
		_, err := io.WriteString(conn, "3Access denied.\r\n")
		if err != nil {
			return err
		}
		return nil
	}

	// If it's a directory
	if utility.IsDirectory(path) {
		serveDirectory(conn, path, selector, timeOut)
		return nil
	}

	// If it's a file
	if utility.FileExists(path) {
		serveFile(conn, path, timeOut)
		return nil
	}

	// Not found
	fmt.Fprintf(conn, "3Not found.\r\n.\r\n")
	return nil
}

func serveDirectory(conn net.Conn, dir string, selector string, timeOut time.Duration) {
	// If gophermap exists, serve it
	mapPath := filepath.Join(dir, "gophermap")
	if utility.FileExists(mapPath) {
		serveFile(conn, mapPath, timeOut)
		return
	}

	// Otherwise list directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		_, err := fmt.Fprintf(conn,
			"3Error reading directory\t%s/\t%s\t%s\r\n",
			dir, cfg.Host, cfg.Port)
		if err != nil {
			log.Println(err)
		}
		return
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	var entires []string
	for _, e := range entries {
		if gopherEntry, err := buildGopherEntry(e, selector, cfg.Host, cfg.Port); err != nil {
			log.Println(err)
			continue
		} else {
			entires = append(entires, gopherEntry)

		}
	}
	sort.Strings(entires)
	for entry := range entires {
		w.WriteString(entires[entry])
	}

	if _, err := w.WriteString(".\r\n"); err != nil {
		log.Println(err)
		return
	}
	if err := w.Flush(); err != nil {
		log.Println("Error writing directory contents:", err)
		return
	}
	if _, err := conn.Write(buf.Bytes()); err != nil {
		log.Println("connection write error:", err)
		return
	}

}

func buildGopherEntry(e fs.DirEntry, selector string, host string, port string) (string, error) {
	name := e.Name()

	// Skip hidden files
	if strings.HasPrefix(name, ".") {
		return "", nil
	}
	// Skip gophermap files
	if name == "gophermap" {
		return "", nil
	}

	// Always use path.Join for gopher selectors
	fullSelector := path.Join("/"+selector, name)

	// Directories
	if e.IsDir() {
		return "1" + "📁 " + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n", nil
	}

	// Files
	ext := strings.ToLower(filepath.Ext(name))

	var itemType string
	var pictogram string
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif":
		itemType = "I"
		pictogram = "🖼️ "
	case ".txt", ".md", ".log":
		itemType = "0"
		pictogram = "📄 "
	default:
		itemType = "9"
		pictogram = "⚙︎ "
	}

	return itemType + pictogram + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n", nil
}

func serveFile(conn net.Conn, path string, timeout time.Duration) {
	if strings.HasSuffix(path, ".gophermap") ||
		strings.HasSuffix(path, ".gophermap") {
		io.WriteString(conn, "3Access denied.\r\n")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		_, err := io.WriteString(conn, "3Error reading file.\r\n")
		if err != nil {
			return
		}
		return
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
		}
	}(f)

	buf := make([]byte, 4096) // 4 KB chunks

	for {
		// Read next chunk
		n, err := f.Read(buf)
		if n > 0 {
			// Apply timeout for each write
			err := conn.SetWriteDeadline(time.Now().Add(timeout))
			if err != nil {
				return
			}

			if _, err := conn.Write(buf[:n]); err != nil {
				log.Println("Write aborted:", err)
				return
			}

			// Clear deadline after successful write
			if err := conn.SetWriteDeadline(time.Time{}); err != nil {
				// safe to ignore
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("File read error:", err)
			return
		}
	}
	log.Println("File read:", path)
	if _, err := io.WriteString(conn, ".\r\n"); err != nil {
		log.Println("Write aborted:", err)
	}
}
