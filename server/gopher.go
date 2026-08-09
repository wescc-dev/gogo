package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type HandlerFunc func(ctx context.Context, c net.Conn, req string) error

type Middleware func(HandlerFunc) HandlerFunc

type Server struct {
	Addr         string
	Handler      HandlerFunc
	Middlewares  []Middleware
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]struct{}
	shutting bool
}

// Wrap middleware chain
func (s *Server) wrap(h HandlerFunc) HandlerFunc {
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		h = s.Middlewares[i](h)
	}
	return h
}

func (s *Server) ListenAndServe(gopherRoot string) error {

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	log.Printf("gopher: listening on %s", s.Addr)
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
		go s.handleConn(handler, conn, gopherRoot)
	}
}

func (s *Server) handleConn(handler HandlerFunc, conn net.Conn, gopherRoot string) error {
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
	ctx = context.WithValue(ctx, "gopherRoot", gopherRoot)

	reader := bufio.NewReader(conn)
	req, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	var selector = strings.TrimSpace(req)
	log.Println("Selector:", selector)
	var _rootDir = ctx.Value("gopherRoot").(string)
	serveSelector(conn, _rootDir, selector)
	return nil

}

func (s *Server) trackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[c] = struct{}{}
}

func (s *Server) untrackConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, c)
}

func (s *Server) Close() error {
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

func (s *Server) Shutdown(ctx context.Context) error {
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

func serveSelector(conn net.Conn, rootDir string, selector string) {
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
		_, err := io.WriteString(conn, "3Access denied\tfake\tlocalhost\t70\r\n.\r\n")
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
	fmt.Fprintf(conn, "3Not found\tfake\t"+""+"\t70\r\n.\r\n")
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
		fmt.Fprintf(conn, "3Error reading directory\tfake\tlocalhost\t70\r\n.\r\n")
		return
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	for _, e := range entries {
		name := e.Name()
		fullSelector := filepath.Join(selector, name)

		if e.IsDir() {
			w.WriteString("1" + name + "\t" + fullSelector + "/\tlocalhost\t70\r\n")
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))

		if strings.HasSuffix(fullSelector, ".gophermap") ||
			strings.HasSuffix(fullSelector, "gophermap") {
			continue
		}

		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif":
			w.WriteString("I" + name + "\t" + fullSelector + "\tlocalhost\t70\r\n")
		case ".txt", ".md", ".log":
			w.WriteString("0" + name + "\t" + fullSelector + "\tlocalhost\t70\r\n")
		default:
			w.WriteString("9" + name + "\t" + fullSelector + "\tlocalhost\t70\r\n")
		}
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
	}

}

func serveFile(conn net.Conn, path string) {
	if strings.HasSuffix(path, ".gophermap") ||
		strings.HasSuffix(path, ".gophermap") {
		io.WriteString(conn, "3Access denied\tfake\tlocalhost\t70\r\n.\r\n")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		io.WriteString(conn, "3Error reading file\tfake\tlocalhost\t70\r\n.\r\n")
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
