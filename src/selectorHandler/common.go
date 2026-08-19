package selectorHandler

import (
	"fmt"
	"gogopher/src/configuration"
	"io"
	"net"
	"os"
	"time"
)

func WriteErrorToConn(conn net.Conn, timeOut time.Duration, errMessage string, v ...any) error {
	if timeOut > 0 {
		defer func() {
			_ = conn.SetWriteDeadline(time.Time{})
		}()
		_ = conn.SetWriteDeadline(time.Now().Add(timeOut))
	}
	msg := fmt.Sprint(append([]any{errMessage}, v...)...)
	_, err := conn.Write([]byte("3" + msg + ".\t\terror.host\t1\n"))
	writeTerminationMarker(conn)
	return err
}

func writeFileToConn(conn net.Conn, path string, timeout time.Duration) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	if timeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(timeout))
		defer func(conn net.Conn) {
			_ = conn.SetWriteDeadline(time.Time{})
		}(conn)
	}
	_, err = io.Copy(conn, f)
	if err != nil {
		return err
	}
	writeTerminationMarker(conn)
	return nil
}

func writeBannerToConn(conn net.Conn, timeOut time.Duration) error {
	defer func(conn net.Conn, t time.Time) {
		_ = conn.SetWriteDeadline(t)
	}(conn, time.Time{}) // Clear the deadline
	m, _ := configuration.GetMetadata()
	err := conn.SetWriteDeadline(time.Now().Add(timeOut))
	if err != nil {
		return err
	}
	if _, err := conn.Write([]byte(m.AppName)); err != nil {
		return err
	}
	return nil
}

func writeTerminationMarker(conn net.Conn) {
	_, _ = conn.Write([]byte(".\r\n"))
}
