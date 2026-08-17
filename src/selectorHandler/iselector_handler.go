package selectorHandler

import (
	"gogopher/src/core"
	"net"
	"time"
)

type SelectResult struct {
	Handled bool
}
type SelectorFunc func(ctx *core.RequestContext, conn net.Conn, gopherRootDir string, selector string, timeOut time.Duration) (*SelectResult, error)

type ISelectorHandler interface {
	//Select processes the connection. If it handles the connection, it returns nil.
	Select(ctx *core.RequestContext) (*SelectResult, error)
}
