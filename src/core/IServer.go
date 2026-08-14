package core

import (
	"context"
	"net"
	"time"
)

type HandlerFunc func(conn net.Conn, rootDir string, selector string, timeout time.Duration) error

type Middleware func(HandlerFunc) HandlerFunc

type IServer interface {
	Start() error
	Stop(ctx context.Context) error
	ConnectionCount() int
	UpTime() time.Duration
	AddMiddleware(middleware Middleware)
	IsStarted() bool
}
