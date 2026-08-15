package selectorHandler

import (
	"fmt"
	"gogopher/src/core"
	"gogopher/src/security"
	"gogopher/src/utility"
	"log"
	"net"
	"path/filepath"
	"time"
)

type FileSelectorHandler struct {
	svrInfoProvider core.IServerInfoViewProvider
}

func NewFileSelectorHandler(svrInfoProvider core.IServerInfoViewProvider) ISelectorHandler {
	return &FileSelectorHandler{svrInfoProvider: svrInfoProvider}
}

func (s *FileSelectorHandler) Select(conn net.Conn, gopherRootDir string, selector string, timeOut time.Duration) (*SelectResult, error) {
	result := &SelectResult{false}
	filePath := filepath.Join(gopherRootDir, selector)
	// If it's a file
	if utility.FileExists(filePath) {
		if err := security.AssertFileSystemAccess(filePath); err != nil {
			log.Println("REJECT:", filePath, "ERR:", err)
			return nil, err
		}
		log.Println("ALLOW:", filePath)
		if err := WriteFileToConn(conn, filePath, timeOut); err != nil {
			return nil, fmt.Errorf("cannot serve file: %w", err)
		}
		result.Handled = true
	}
	return result, nil
}
