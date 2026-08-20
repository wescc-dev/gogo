package selectorHandler

import (
	"gogo/src/core"
)

type SelectResult struct {
	Handled bool
}

type ISelectorHandler interface {
	//Select processes the connection. If it handles the connection, it returns nil.
	Select(ctx *core.RequestContext) (*SelectResult, error)
}
