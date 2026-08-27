package selectors

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wescc-dev/gogo/src/configuration"
	"github.com/wescc-dev/gogo/src/core"
	"github.com/wescc-dev/gogo/src/security"
)

func WriteErrorToConn(conn net.Conn, timeOut time.Duration, errMessage string, v ...any) error {
	msg := fmt.Sprint(append([]any{errMessage}, v...)...)
	if timeOut > 0 {
		defer func() {
			_ = conn.SetWriteDeadline(time.Time{})
		}()
		_ = conn.SetWriteDeadline(time.Now().Add(timeOut))
	}
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
	if _, err := conn.Write([]byte(m.Footer)); err != nil {
		return err
	}
	return nil
}

func writeTerminationMarker(conn net.Conn) {
	_, _ = conn.Write([]byte(".\r\n"))
}

func readDirFiltered(dirPath string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	filtered := make([]fs.DirEntry, 0, len(entries))

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())
		if err := security.AssertFileSystemAccess(fullPath); err == nil {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

func buildGopherEntry(e fs.DirEntry, selector string, host string, port string) (string, error) {
	name := e.Name()
	// Skip hidden files
	if strings.HasPrefix(name, ".") {
		return "", nil
	}
	// Skip secret files and directories
	if strings.HasPrefix(name, "$") {
		return "", nil
	}
	// Skip gophermap files
	if name == "gophermap" {
		return "", nil
	}

	// Always use path.Join for gopher selectors
	fullSelector := path.Join("/"+selector, name)

	// Directories
	itemType, pictogram := getGopherItemTypeByExtension("/")
	if e.IsDir() {
		return buildSelectorWithPictogram(itemType, pictogram+" ", name, fullSelector, host, port), nil
	}

	return buildSelector(name, fullSelector, host, port), nil
}

func buildSelector(name string, fullSelector string, host string, port string) string {
	ext := strings.ToLower(filepath.Ext(name))
	itemType, pictogram := getGopherItemTypeByExtension(ext)
	return buildSelectorWithPictogram(itemType, pictogram+" ", name, fullSelector, host, port)
}

func getGopherItemTypeByExtension(ext string) (string, string) {
	itemType, pictogram := core.GetItemTypeByExtension(ext)
	return itemType, pictogram
}

func buildSelectorWithPictogram(itemType string, pictogram string, name string, fullSelector string, host string, port string) string {
	return itemType + pictogram + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n"
}

func sortDirectoryEntries(entries []os.DirEntry) {
	// Sort entries by name, case-insensitive
	sort.Slice(entries, func(i, j int) bool {
		iIsDir := entries[i].IsDir()
		jIsDir := entries[j].IsDir()

		// Directories last
		if iIsDir != jIsDir {
			return !iIsDir
		}

		// Same type → sort by name
		return entries[i].Name() < entries[j].Name()
	})
}
