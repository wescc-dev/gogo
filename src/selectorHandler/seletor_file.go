package selectorHandler

import (
	"fmt"
	"gogo/src/core"
	"gogo/src/security"
	"gogo/src/utility"
	"path/filepath"
)

type FileSelectorHandler struct {
	svrInfoProvider core.IServerInfoViewProvider
}

func NewFileSelectorHandler(svrInfoProvider core.IServerInfoViewProvider) ISelectorHandler {
	return &FileSelectorHandler{svrInfoProvider: svrInfoProvider}
}

func (s *FileSelectorHandler) Select(ctx *core.RequestContext) (*SelectResult, error) {
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
