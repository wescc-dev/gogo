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

func writeFileToConn(conn net.Conn, path string, timeout time.Duration) error {
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

func writeBannerToConn(conn net.Conn, timeOut time.Duration) error {
	defer func(conn net.Conn, t time.Time) error {
		err := conn.SetReadDeadline(t)
		if err != nil {
			return err
		}
		return nil
	}(conn, time.Time{})
	conn.SetWriteDeadline(time.Now().Add(timeOut))
	if _, err := conn.Write([]byte(configuration.Footer)); err != nil {
		log.Println(err)
		return err
	}
	return nil
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
