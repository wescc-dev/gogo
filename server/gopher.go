package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"gopher/configuration"
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

type HandlerFunc func(ctx context.Context, c net.Conn, req string) error

type Middleware func(HandlerFunc) HandlerFunc

var cfg = configuration.GetConfiguration()

// server is a simple gopher server
type server struct {
	Hostname     string
	BindAddr     string
	Port         string
	Handler      HandlerFunc
	Middlewares  []Middleware
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	GopherRoot   string
	mu           sync.Mutex
	listener     net.Listener
	conns        map[net.Conn]struct{}
	shutting     bool
}

// Wrap middleware chain
func (s *server) wrap(h HandlerFunc) HandlerFunc {
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		h = s.Middlewares[i](h)
	}
	return h
}

type IServer interface {
	ListenAndServe() error
	Close() error
	Shutdown(ctx context.Context) error
	ConnectionCount() int
}

func NewServer(hostname string, bindAddr string, port string, gopherRoot string) (IServer, error) {
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
		Hostname:   hostname,
		BindAddr:   bindAddr,
		Port:       port,
		GopherRoot: gopherRoot,
	}, nil
}

func (s *server) ListenAndServe() error {
	if err := s.generateGopherMap(); err != nil {
		log.Println("Error generating gophermap:", err)
	}
	ln, err := net.Listen("tcp", s.BindAddr+":"+s.Port)
	if err != nil {
		return err
	}
	log.Println("gopher: listening on ", s.BindAddr+":"+s.Port)
	s.mu.Lock()
	s.listener = ln
	s.conns = make(map[net.Conn]struct{})
	s.mu.Unlock()
	handler := s.wrap(s.Handler)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.shutting {
				return nil
			}
			return err
		}
		s.trackConn(conn)
		go func() {
			err := s.handleConn(handler, conn, s.GopherRoot)
			if err != nil {

			}
		}()
	}
}

func (s *server) Close() error {
	s.mu.Lock()
	s.shutting = true
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}

	s.mu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()

	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.shutting = true
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}

	done := make(chan struct{})

	go func() {
		s.mu.Lock()
		for c := range s.conns {
			_ = c.Close()
		}
		s.mu.Unlock()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *server) ConnectionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
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
		w.WriteString(entry)
	}
	w.Flush()
	return entriesBuf.String(), nil
}

func (s *server) handleConn(handler HandlerFunc, conn net.Conn, gopherRoot string) error {
	defer s.untrackConn(conn)
	defer conn.Close()

	if s.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(s.ReadTimeout))
	}
	if s.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(s.WriteTimeout))
	}

	ctx := context.Background()
	if s.IdleTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.IdleTimeout)
		defer cancel()
	}
	reader := bufio.NewReader(conn)
	req, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	var selector = strings.TrimSpace(req)
	log.Println("Selector:", selector)
	serveSelector(conn, gopherRoot, selector)
	return nil

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

func serveSelector(conn net.Conn, rootDir string, selector string) {
	log.Println("Serving selector:", selector)
	// Empty selector → serve root directory
	if selector == "" {
		serveDirectory(conn, rootDir, selector)
		return
	}
	clean := filepath.Clean("/" + selector) // force selector to be relative
	path := filepath.Join(rootDir, clean)

	realRoot, _ := filepath.Abs(rootDir)
	realPath, _ := filepath.Abs(path)

	if !strings.HasPrefix(realPath, realRoot) {
		_, err := io.WriteString(conn, "3Access denied.\r\n")
		if err != nil {
			return
		}
		return
	}

	// If it's a directory
	if isDir(path) {
		serveDirectory(conn, path, selector)
		return
	}

	// If it's a file
	if fileExists(path) {
		serveFile(conn, path)
		return
	}

	// Not found
	fmt.Fprintf(conn, "3Not found.\r\n.\r\n")
}

func serveDirectory(conn net.Conn, dir string, selector string) {
	// If gophermap exists, serve it
	mapPath := filepath.Join(dir, "gophermap")
	if fileExists(mapPath) {
		serveFile(conn, mapPath)
		return
	}

	// Otherwise list directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(conn, "3Error reading directory"+"\t"+dir+"/\t"+cfg.Host+"\t"+cfg.Port+"\r\n")
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
		return "1" + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n", nil
	}

	// Files
	ext := strings.ToLower(filepath.Ext(name))

	var itemType string

	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif":
		itemType = "I"
	case ".txt", ".md", ".log":
		itemType = "0"
	default:
		itemType = "9"
	}

	return itemType + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n", nil
}

func serveFile(conn net.Conn, path string) {
	if strings.HasSuffix(path, ".gophermap") ||
		strings.HasSuffix(path, ".gophermap") {
		io.WriteString(conn, "3Access denied.\r\n")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		io.WriteString(conn, "3Error reading file.\r\n")
		return
	}
	defer f.Close()

	buf := make([]byte, 4096) // 4 KB chunks

	for {
		// Read next chunk
		n, err := f.Read(buf)
		if n > 0 {
			// Apply timeout for each write
			err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
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

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
