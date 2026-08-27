package core

import (
	"context"
	"net"
	"time"
)

type Request struct {
	Conn      net.Conn
	RequestId string
	RootDir   string
	Selector  string
	Timeout   time.Duration
}

type RequestContext struct {
	context.Context
	Request *Request
}

func NewRequestContext(ctx context.Context, request *Request) *RequestContext {
	return &RequestContext{
		Context: ctx,
		Request: request,
	}
}

type HandlerFunc func(ctx *RequestContext) error

type Middleware func(HandlerFunc) HandlerFunc

type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
	AddMiddleware(middleware Middleware)
	IsStarted() bool
	GetCurrentServerInfo() ServerInfo
}
