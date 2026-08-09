package main

import (
	"bufio"
	"bytes"
	"fmt"
	"gopher/configuration"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	port    = 70
	rootDir = "./gopherroot"
)

var (
	wg       sync.WaitGroup
	shutdown = make(chan struct{}) // closed on shutdown
)

var cfg = configuration.GetConfiguration()

func main() {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%s", cfg.HostBindIp, cfg.Port))
	if err != nil {
		log.Fatal(err)
	}

	log.Println(cfg.Title, "listening on", cfg.Host, "port", cfg.Port)

	// Capture Ctrl‑C and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	listenerDone := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		<-sig
		log.Println("Shutdown signal received")
		close(shutdown)
		ln.Close()
		<-listenerDone
		log.Println("Shutdown complete")
		shutdownDone <- struct{}{}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Listener closed, waiting for active connections...")
			wg.Wait()
			log.Println("All connections closed.")
			listenerDone <- struct{}{}
			break
		}
		wg.Add(1)
		go handle(conn)

	}
	<-shutdownDone
	log.Println("Done.")
}

func handle(conn net.Conn) {
	defer wg.Done()
	defer conn.Close()

	// If shutdown triggered, abort immediately
	select {
	case <-shutdown:
		log.Println("Closing connection due to shutdown:", conn.RemoteAddr())
		return
	default:
		log.Println("Handling connection:", conn.RemoteAddr())
	}
	type inputResult struct {
		result string
		err    error
	}
	var result = make(chan inputResult)

	go func() {
		reader := bufio.NewReader(conn)
		err := conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		if err != nil {
			result <- inputResult{err: err}
			return
		}
		selector, err := reader.ReadString('\n')
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			// safe to ignore
		}
		result <- inputResult{selector, err}
	}()

	select {
	case <-shutdown:
		log.Println("Closing connection due to shutdown:", conn.RemoteAddr())
		return
	case r := <-result:
		if r.err != nil {
			log.Println(r.err)
			return
		}
		var selector = strings.TrimSpace(r.result)
		log.Println("Selector:", selector)
		serveSelector(conn, selector)
	}
}

func serveSelector(conn net.Conn, selector string) {
	// Empty selector → serve root directory
	if selector == "" {
		serveDirectory(conn, rootDir, selector)
		return
	}
	clean := filepath.Clean("/" + selector) // force selector to be relative
	path := filepath.Join(rootDir, clean)

	realRoot, _ := filepath.Abs(rootDir)
	realPath, _ := filepath.Abs(path)

	if !strings.HasPrefix(realPath, realRoot) {
		_, err := io.WriteString(conn, "3Access denied\tfake\tlocalhost\t70\r\n.\r\n")
		if err != nil {
			return
		}
		return
	}

	// If it's a directory
	if isDir(path) {
		serveDirectory(conn, path, selector)
		return
	}

	// If it's a file
	if fileExists(path) {
		serveFile(conn, path)
		return
	}

	// Not found
	fmt.Fprintf(conn, "3Not found\tfake\t"+""+"\t70\r\n.\r\n")
}

func serveDirectory(conn net.Conn, dir string, selector string) {
	// If gophermap exists, serve it
	mapPath := filepath.Join(dir, "gophermap")
	if fileExists(mapPath) {
		serveFile(conn, mapPath)
		return
	}

	// Otherwise list directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(conn, "3Error reading directory\tfake\tlocalhost\t70\r\n.\r\n")
		return
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	for _, e := range entries {
		name := e.Name()
		fullSelector := filepath.Join(selector, name)

		if e.IsDir() {
			w.WriteString("1" + name + "\t" + fullSelector + "/\tlocalhost\t70\r\n")
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))

		if strings.HasSuffix(fullSelector, ".gophermap") ||
			strings.HasSuffix(fullSelector, "gophermap") {
			continue
		}

		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif":
			w.WriteString("I" + name + "\t" + fullSelector + "\tlocalhost\t70\r\n")
		case ".txt", ".md", ".log":
			w.WriteString("0" + name + "\t" + fullSelector + "\tlocalhost\t70\r\n")
		default:
			w.WriteString("9" + name + "\t" + fullSelector + "\tlocalhost\t70\r\n")
		}
	}

	if _, err := w.WriteString(".\r\n"); err != nil {
		log.Println(err)
		return
	}
	if err := w.Flush(); err != nil {
		log.Println("Error writing directory contents:", err)
		return
	}
	if _, err := conn.Write(buf.Bytes()); err != nil {
		log.Println("connection write error:", err)
	}

}

func serveFile(conn net.Conn, path string) {
	if strings.HasSuffix(path, ".gophermap") ||
		strings.HasSuffix(path, ".gophermap") {
		io.WriteString(conn, "3Access denied\tfake\tlocalhost\t70\r\n.\r\n")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		io.WriteString(conn, "3Error reading file\tfake\tlocalhost\t70\r\n.\r\n")
		return
	}
	defer f.Close()

	buf := make([]byte, 4096) // 4 KB chunks

	for {
		// Abort immediately if shutdown triggered
		select {
		case <-shutdown:
			conn.SetWriteDeadline(time.Now()) // force unblock
			return
		default:
		}

		// Read next chunk
		n, err := f.Read(buf)
		if n > 0 {
			// Apply timeout for each write
			err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err != nil {
				return
			}

			if _, err := conn.Write(buf[:n]); err != nil {
				log.Println("Write aborted:", err)
				return
			}

			// Clear deadline after successful write
			if err := conn.SetWriteDeadline(time.Time{}); err != nil {
				// safe to ignore
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("File read error:", err)
			return
		}
	}
	log.Println("File read:", path)
	if _, err := io.WriteString(conn, ".\r\n"); err != nil {
		log.Println("Write aborted:", err)
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
