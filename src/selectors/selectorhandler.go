package selectors

import (
	"github.com/wescc-dev/gogo/src/core"
)

type SelectResult struct {
	Handled bool
}

type Selector interface {
	//Select processes the connection. If it handles the connection, it returns nil.
	Select(ctx *core.RequestContext) (*SelectResult, error)
}
