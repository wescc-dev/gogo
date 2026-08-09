package server

import (
	"bufio"
	"context"
	"errors"
	"log"
	"net"
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
	if s.Handler == nil {
		return errors.New("gopher: no handler")
	}

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

func (s *Server) handleConn(handler HandlerFunc, conn net.Conn, gopherRoot string) {
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
		ctx = context.WithValue(ctx, "gopherRoot", gopherRoot)
	}

	reader := bufio.NewReader(conn)
	req, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	_ = handler(ctx, conn, req)

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
