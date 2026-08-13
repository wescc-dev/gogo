package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"gogopher/src/configuration"
	"gogopher/src/core"
	"gogopher/src/security"
	"gogopher/src/selectorHandler"
	"gogopher/src/utility"
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

const GophermapTemplateName = ".gophermap"

var cfg = configuration.GetConfiguration()

// server is a simple gopher server
type server struct {
	Hostname               string
	BindAddr               string
	Port                   string
	Handler                core.HandlerFunc
	Middlewares            []core.Middleware
	RequestTimeoutDuration time.Duration
	GopherRoot             string
	startTime              time.Time
	stopTime               time.Time

	mu              sync.Mutex
	listener        net.Listener
	conns           map[net.Conn]struct{}
	clientWaitGroup sync.WaitGroup
	acceptDone      chan struct{}
	stopOnce        sync.Once
	stopRequested   bool
}

func NewServer(
	hostname string,
	bindAddr string,
	port string,
	gopherRoot string,
	requestTimeoutDuration time.Duration) (core.IServer, error) {

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
		Hostname:               hostname,
		BindAddr:               bindAddr,
		Port:                   port,
		GopherRoot:             gopherRoot,
		RequestTimeoutDuration: requestTimeoutDuration,
		Middlewares:            []core.Middleware{},
		clientWaitGroup:        sync.WaitGroup{},
		acceptDone:             make(chan struct{}),
		stopRequested:          false,
	}, nil
}

func (s *server) Start() error {
	ln, err := net.Listen("tcp", s.BindAddr+":"+s.Port)
	if err != nil {
		return err
	}
	log.Println("GoGopher: listening on ", s.BindAddr+":"+s.Port)

	s.mu.Lock()
	s.Handler = s.useMiddleware(s.serveSelector)
	s.listener = ln
	s.conns = make(map[net.Conn]struct{})
	s.startTime = time.Now()
	s.stopRequested = false
	s.mu.Unlock()
	go s.acceptLoop()
	return nil
}

func (s *server) AddMiddleware(middleware core.Middleware) {
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
func (s *server) handleClientConnection(conn net.Conn, handler core.HandlerFunc) {
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

func (s *server) processRequest(handler core.HandlerFunc, conn net.Conn, gopherRoot string) error {
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
	reader := bufio.NewReader(conn)
	req, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("cannot read request: %w", err)
	}

	var selector = strings.TrimSpace(req)
	log.Println("Selector:", selector)
	return handler(conn, gopherRoot, selector, s.RequestTimeoutDuration)
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

func (s *server) useMiddleware(h core.HandlerFunc) core.HandlerFunc {
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		h = s.Middlewares[i](h)
	}
	return h
}

func (s *server) serveSelector(conn net.Conn, rootDir string, selector string, timeOut time.Duration) error {
	// Empty selector → serve root directory
	if selector == "" {
		log.Println("Serving Selector:", s.GopherRoot)

		if err := s.serveDirectory(conn, rootDir, selector, timeOut); err != nil {
			return fmt.Errorf("cannot serve root directory: %w", err)
		}
		return nil
	} else {
		log.Println("Serving Selector:", selector)
	}

	clean := filepath.Clean("/" + selector) // force selector to be relative
	cleanPath := filepath.Join(rootDir, clean)
	if clean == "/pggp-key" {
		s := selectorHandler.NewSelector()
		s.Select(conn, "", selector, timeOut)
	}
	realRoot, _ := filepath.Abs(rootDir)
	realPath, _ := filepath.Abs(cleanPath)
	if err := security.AssertFileSystemAccess(realPath); err != nil {
		if _, err := io.WriteString(conn, "3Not found.\t\terror.host\t1\r\n"); err != nil {
		}
		return fmt.Errorf("cannot access file system: %w", err)
	}
	if !strings.HasPrefix(realPath, realRoot) {
		if _, err := io.WriteString(conn, "3Access denied.\r\n"); err != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
		return nil
	}

	// If it's a directory
	if utility.IsDirectory(cleanPath) {
		err := s.serveDirectory(conn, cleanPath, selector, timeOut)
		if err != nil {
			return fmt.Errorf("cannot serve directory: %w", err)
		}
		return nil
	}

	// If it's a file
	if utility.FileExists(cleanPath) {
		if err := s.serveFile(conn, cleanPath, timeOut); err != nil {
			return fmt.Errorf("cannot serve file: %w", err)
		}
		return nil
	}

	// Not found
	if _, err := fmt.Fprintf(conn, "3Not found.\r\n.\r\n"); err != nil {
		return fmt.Errorf("cannot write error message: %w", err)
	}
	return fmt.Errorf("file not found: %s", cleanPath)
}

func (s *server) serveDirectory(conn net.Conn, dir string, selector string, timeOut time.Duration) error {
	defer func(conn net.Conn, t time.Time) {
		_ = conn.SetReadDeadline(t)
	}(conn, time.Time{})

	// Regenerate the gophermap if it's outdated
	gophermapPath := filepath.Join(dir, "gophermap")
	gophermapTemplatePath := filepath.Join(dir, GophermapTemplateName)
	gopherMapExists := utility.FileExists(gophermapPath)
	gopherMapTemplateExists := utility.FileExists(gophermapTemplatePath)

	// Gophermap exists, but template doesn't': serve it
	if gopherMapExists && !gopherMapTemplateExists {
		err := s.serveFile(conn, gophermapPath, timeOut)
		if err != nil {
			return fmt.Errorf("cannot serve gophermap: %w", err)
		}
		return nil
	}

	// Template exists, but gophermao doesn't: generate it and serve it
	if gopherMapTemplateExists && !gopherMapExists {
		err := s.generateGopherMap(conn, dir)
		if err != nil {
			return fmt.Errorf("cannot generate gophermap: %w", err)
		}
		log.Println("Generated gophermap for:", dir)
		return nil
	}

	// Gophermap exists and newer template exists: generate it and serve it
	if gopherMapExists {
		var gopherMapInfo, _ = os.Stat(gophermapPath)
		var gopherMapTemplateInfo, _ = os.Stat(gophermapTemplatePath)
		if gopherMapInfo.ModTime().Before(gopherMapTemplateInfo.ModTime()) {
			err := s.generateGopherMap(conn, gophermapTemplatePath)
			if err != nil {
				return err
			}
			log.Println("Generated gophermap for:", dir)
			return nil
		}

		if err := s.serveFile(conn, gophermapPath, timeOut); err != nil {
			return fmt.Errorf("cannot serve gophermap: %w", err)
		}
		return nil
	}

	// Otherwise list directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		if _, eerr := fmt.Fprintf(conn,
			"3Error reading directory\t%s/\t%s\t%s\r\n",
			dir, cfg.HostName, cfg.Port); eerr != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
		return fmt.Errorf("cannot read directory: %w", err)
	}
	if len(entries) == 0 {
		if _, err := conn.Write([]byte("3Directory is empty\t\t.\r\n")); err != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
	} else {
		sortDirectoryEntries(entries)
		for _, e := range entries {
			if gopherEntry, err := buildGopherEntry(e, selector, cfg.HostName, cfg.Port); err != nil {
				log.Println(err)
				continue
			} else {
				if err = conn.SetWriteDeadline(time.Now().Add(timeOut)); err != nil {
					return err
				}
				if _, err := conn.Write([]byte(gopherEntry)); err != nil {
					log.Println(err)
					return err
				}
			}
		}
	}
	writeBanner(conn, timeOut)
	writeFooter(conn, timeOut)
	return nil
}

func (s *server) serveFile(conn net.Conn, path string, timeout time.Duration) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	buf := make([]byte, 4*1024) // 4 KB chunks

	for {
		// Read next chunk
		n, err := f.Read(buf)
		if n > 0 {
			// Apply timeout for each write operation
			err := conn.SetWriteDeadline(time.Now().Add(timeout))
			if err != nil {
				return fmt.Errorf("cannot set write deadline: %w", err)
			}

			if _, err := conn.Write(buf[:n]); err != nil {
				return fmt.Errorf("cannot write file: %w", err)
			}

			// Clear deadline after successful write operation
			if err := conn.SetWriteDeadline(time.Time{}); err != nil {
				// safe to ignore
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("cannot read file: %w", err)
		}
	}
	log.Println("File read:", path)
	writeFooter(conn, timeout)
	return nil
}

func (s *server) generateGopherMap(conn net.Conn, dirPath string) error {
	if info, err := os.Stat(s.GopherRoot); !(err == nil && info.IsDir()) {
		if err := os.MkdirAll(s.GopherRoot, os.ModePerm); err != nil {
			return fmt.Errorf("GopherRoot %s does not exist and could not be created. No files will be served", s.GopherRoot)
		}
	}
	gophermapPath := filepath.Join(dirPath, "gophermap")
	gophermapTemplatePath := filepath.Join(dirPath, GophermapTemplateName)
	data, err := os.ReadFile(gophermapTemplatePath)
	if err != nil {
		return fmt.Errorf("cannot open .gophermap: %w", err)
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
	entries, err := s.generateEntries(dirPath)
	if err != nil {
		return err
	}
	// Replace tokens
	content = strings.ReplaceAll(content, "{{ENTRIES}}", entries)
	content += configuration.Footer
	if _, err := conn.Write([]byte(content)); err != nil {
		return fmt.Errorf("cannot write gophermap: %w", err)
	}

	// Write output
	if err := os.WriteFile(gophermapPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write generated gophermap: %w", err)
	}
	return nil
}

func (s *server) generateEntries(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("cannot read gopher root: %w", err)
	}

	sortDirectoryEntries(entries)

	var entriesBuf bytes.Buffer
	w := bufio.NewWriter(&entriesBuf)
	for _, e := range entries {
		if s, err := buildGopherEntry(e, strings.TrimPrefix(dirPath, s.GopherRoot), s.Hostname, s.Port); err != nil {
			log.Println(err)
			continue
		} else {
			_, _ = w.WriteString(s)
		}
	}
	_ = w.Flush()
	return entriesBuf.String(), nil
}

func sortDirectoryEntries(entries []os.DirEntry) {
	// Sort entries by name, case-insensitive
	sort.Slice(entries, func(i, j int) bool {
		iIsDir := entries[i].IsDir()
		jIsDir := entries[j].IsDir()

		// Directories first
		if iIsDir != jIsDir {
			return !iIsDir
		}

		// Same type → sort by name
		return entries[i].Name() < entries[j].Name()
	})
}

func writeBanner(conn net.Conn, timeOut time.Duration) {
	defer conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Now().Add(timeOut))
	if _, err := conn.Write([]byte(configuration.Footer)); err != nil {
		log.Println(err)
		return
	}
}

func writeFooter(conn net.Conn, timeOut time.Duration) {
	defer func(conn net.Conn, t time.Time) {
		_ = conn.SetReadDeadline(t)
	}(conn, time.Time{})
	_ = conn.SetWriteDeadline(time.Now().Add(timeOut))
	if _, err := conn.Write([]byte(".\r\n")); err != nil {
		log.Println(err)
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
