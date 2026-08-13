package selectorHandler

import (
	"net"
	"time"
)

type ISelector interface {
	Select(conn net.Conn, dir string, selector string, timeOut time.Duration) error
}
