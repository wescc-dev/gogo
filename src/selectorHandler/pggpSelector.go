package selectorHandler

import (
	"io"
	"net"
	"os"
	"strings"
	"time"
)

type pggpSelector struct {
	serverPubKeyFile string
}

func NewSelector() ISelector {
	return &pggpSelector{
		serverPubKeyFile: "server.public.asc",
	}
}

func (s *pggpSelector) Select(conn net.Conn, dir string, selector string, timeOut time.Duration) error {
	if strings.ToLower(selector) == "/pggp-key" {
		if f, eerr := os.Open(s.serverPubKeyFile); eerr != nil {
			return eerr
		} else {
			defer func(f *os.File) {
				_ = f.Close()
			}(f)
			_, eerr = io.Copy(conn, f)
			conn.Write([]byte(".\r\n"))
			return eerr
		}
	}
	return nil
}
