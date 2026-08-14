package selectorHandler

import (
	"fmt"
	"gogopher/src/configuration"
	"io"
	"log"
	"net"
	"os"
	"time"
)

type SelectResult struct {
	Handled bool
}
type SelectorFunc func(conn net.Conn, gopherRootDir string, selector string, timeOut time.Duration) (*SelectResult, error)

type ISelectorHandler interface {
	//Select processes the connection. If it handles the connection, it returns nil.
	Select(conn net.Conn, gopherRootDir string, selector string, timeOut time.Duration) (*SelectResult, error)
}

func WriteFileToConn(conn net.Conn, path string, timeout time.Duration) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	buf := make([]byte, 4*1024) // 4 KB chunks

	for {
		// Read next chunk
		n, err := f.Read(buf)
		if n > 0 {
			// Apply timeout for each write operation
			err := conn.SetWriteDeadline(time.Now().Add(timeout))
			if err != nil {
				return fmt.Errorf("cannot set write deadline: %w", err)
			}

			if _, err := conn.Write(buf[:n]); err != nil {
				return fmt.Errorf("cannot write file: %w", err)
			}

			// Clear deadline after successful write operation
			if err := conn.SetWriteDeadline(time.Time{}); err != nil {
				// safe to ignore
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("cannot read file: %w", err)
		}
	}
	log.Println("File read:", path)
	writeTerminationMarker(conn, timeout)
	return nil
}

func WriteBannerToConn(conn net.Conn, timeOut time.Duration) {
	defer conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Now().Add(timeOut))
	if _, err := conn.Write([]byte(configuration.Footer)); err != nil {
		log.Println(err)
		return
	}
}

func writeTerminationMarker(conn net.Conn, timeOut time.Duration) {
	defer func(conn net.Conn, t time.Time) {
		_ = conn.SetReadDeadline(t)
	}(conn, time.Time{})
	_ = conn.SetWriteDeadline(time.Now().Add(timeOut))
	if _, err := conn.Write([]byte(".\r\n")); err != nil {
		log.Println(err)
		return
	}
}
