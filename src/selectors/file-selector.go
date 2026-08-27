package selectors

import (
	"fmt"
	"path/filepath"

	"github.com/wescc-dev/gogo/src/core"
	"github.com/wescc-dev/gogo/src/security"
	"github.com/wescc-dev/gogo/src/utility"
)

type FileSelector struct {
	svrInfoProvider core.IServerInfoProvider
}

func NewFileSelector(svrInfoProvider core.IServerInfoProvider) Selector {
	return &FileSelector{svrInfoProvider: svrInfoProvider}
}

func (s *FileSelector) Select(ctx *core.RequestContext) (*SelectResult, error) {
	result := &SelectResult{false}
	filePath := filepath.Join(ctx.Request.RootDir, ctx.Request.Selector)
	// If it's a file
	if utility.FileExists(filePath) {
		if err := security.AssertFileSystemAccess(filePath); err != nil {
			core.ContextLog(ctx, "Access denied for file:", filePath)
			return nil, err
		}
		core.ContextLog(ctx, "Serving file:", filePath)
		if err := writeFileToConn(ctx.Request.Conn, filePath, ctx.Request.Timeout); err != nil {
			return nil, fmt.Errorf("cannot serve file: %w", err)
		}
		result.Handled = true
	}
	return result, nil
}
