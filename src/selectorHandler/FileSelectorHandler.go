package selectorHandler

import (
	"fmt"
	"gogopher/src/configuration"
	"gogopher/src/utility"
	"net"
	"path/filepath"
	"time"
)

type FileSelectorHandler struct {
	cfg *configuration.Configuration
}

func NewFileSelectorHandler(cfg *configuration.Configuration) ISelectorHandler {
	return &FileSelectorHandler{cfg: cfg}
}

func (s *FileSelectorHandler) Select(conn net.Conn, gopherRootDir string, selector string, timeOut time.Duration) (*SelectResult, error) {
	result := &SelectResult{false}
	filePath := filepath.Join(gopherRootDir, selector)
	// If it's a file
	if utility.FileExists(filePath) {
		if err := WriteFileToConn(conn, filePath, timeOut); err != nil {
			return nil, fmt.Errorf("cannot serve file: %w", err)
		}
		result.Handled = true
	}
	return result, nil
}
