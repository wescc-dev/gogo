package selectorHandler

import (
	"bufio"
	"bytes"
	"fmt"
	"gogopher/src/configuration"
	"gogopher/src/utility"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DirectorySelectorHandler handles directory selectors
type DirectorySelectorHandler struct {
	cfg *configuration.Configuration
}

func NewDirectorySelectorHandler(cfg *configuration.Configuration) ISelectorHandler {
	return &DirectorySelectorHandler{cfg: cfg}
}

func (d *DirectorySelectorHandler) Select(conn net.Conn, gopherRootDir string, selector string, timeOut time.Duration) (*SelectResult, error) {
	result := &SelectResult{false}
	selectorPath := filepath.Join(gopherRootDir, selector)
	// If it's a directory
	if utility.IsDirectory(selectorPath) {
		err := d.serveDirectory(conn, selectorPath, selector, timeOut)
		if err != nil {
			return nil, fmt.Errorf("cannot serve directory: %w", err)
		}
		result.Handled = true
	}
	return result, nil
}
func (d *DirectorySelectorHandler) serveDirectory(conn net.Conn, selectorPath string, selector string, timeOut time.Duration) error {
	defer func(conn net.Conn, t time.Time) {
		_ = conn.SetReadDeadline(t)
	}(conn, time.Time{})

	done, err := d.serveGopherMap(conn, selectorPath, timeOut)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	// Otherwise list directory
	entries, err := os.ReadDir(selectorPath)
	if err != nil {
		if _, eerr := fmt.Fprintf(conn,
			"3Error reading directory\t%s/\t%s\t%s\r\n",
			selectorPath, d.cfg.HostName, d.cfg.Port); eerr != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
		return fmt.Errorf("cannot read directory: %w", err)
	}
	if len(entries) == 0 {
		if _, err := conn.Write([]byte("3Directory is empty\t\t.\r\n")); err != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
	} else {
		sortDirectoryEntries(entries)
		for _, e := range entries {
			if gopherEntry, err := buildGopherEntry(e, selector, d.cfg.HostName, d.cfg.Port); err != nil {
				log.Println(err)
				continue
			} else {
				if err = conn.SetWriteDeadline(time.Now().Add(timeOut)); err != nil {
					return err
				}
				if _, err := conn.Write([]byte(gopherEntry)); err != nil {
					log.Println(err)
					return err
				}
			}
		}
	}
	WriteBannerToConn(conn, timeOut)
	writeTerminationMarker(conn, timeOut)
	return nil
}

func (d *DirectorySelectorHandler) serveGopherMap(conn net.Conn, selectorPath string, timeOut time.Duration) (bool, error) {
	// Regenerate the gophermap if it's outdated
	gophermapPath := filepath.Join(selectorPath, "gophermap")
	gophermapTemplatePath := filepath.Join(selectorPath, d.cfg.GophermapTemplateName)
	gopherMapExists := utility.FileExists(gophermapPath)
	gopherMapTemplateExists := utility.FileExists(gophermapTemplatePath)

	// Gophermap exists then serve it
	if gopherMapExists {
		err := WriteFileToConn(conn, gophermapPath, timeOut)
		if err != nil {
			return false, fmt.Errorf("cannot serve gophermap: %w", err)
		}
		return true, nil
	} else if gopherMapTemplateExists {
		err := d.generateGopherMap(conn, selectorPath)
		if err != nil {
			return false, fmt.Errorf("cannot generate gophermap: %w", err)
		}
		log.Println("Generated gophermap for:", selectorPath)
		return true, nil
	}
	return false, nil
}
func (d *DirectorySelectorHandler) generateGopherMap(conn net.Conn, dirPath string) error {
	if info, err := os.Stat(d.cfg.GopherRoot); !(err == nil && info.IsDir()) {
		if err := os.MkdirAll(d.cfg.GopherRoot, os.ModePerm); err != nil {
			return fmt.Errorf("GopherRoot %s does not exist and could not be created. No files will be served", d.cfg.GopherRoot)
		}
	}
	gophermapTemplatePath := filepath.Join(dirPath, d.cfg.GophermapTemplateName)
	data, err := os.ReadFile(gophermapTemplatePath)
	if err != nil {
		return fmt.Errorf("cannot open .gophermap: %w", err)
	}

	content := string(data)

	// Replace simple tokens first
	tokens := map[string]string{
		"TITLE":   d.cfg.Title,
		"VERSION": configuration.Version,
		"HOST":    d.cfg.HostName,
		"BIND":    d.cfg.BindAddress,
		"=":       "======================================================================",
		"*":       "**********************************************************************",
		"-":       "----------------------------------------------------------------------",
	}

	for key, val := range tokens {
		content = strings.ReplaceAll(content, "{{"+key+"}}", val)
	}

	// Now handle {{ENTRIES}}
	entries, err := d.generateEntries(dirPath)
	if err != nil {
		return err
	}
	// Replace tokens
	content = strings.ReplaceAll(content, "{{ENTRIES}}", entries)
	content += configuration.Footer
	if _, err := conn.Write([]byte(content)); err != nil {
		return fmt.Errorf("cannot write gophermap: %w", err)
	}
	return nil
}
func (d *DirectorySelectorHandler) generateEntries(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("cannot read gopher root: %w", err)
	}

	sortDirectoryEntries(entries)

	var entriesBuf bytes.Buffer
	w := bufio.NewWriter(&entriesBuf)
	for _, e := range entries {
		if s, err := buildGopherEntry(e, strings.TrimPrefix(dirPath, d.cfg.GopherRoot), d.cfg.HostName, d.cfg.Port); err != nil {
			log.Println(err)
			continue
		} else {
			_, _ = w.WriteString(s)
		}
	}
	_ = w.Flush()
	return entriesBuf.String(), nil
}

func buildGopherEntry(e fs.DirEntry, selector string, host string, port string) (string, error) {
	name := e.Name()

	// Skip hidden files
	if strings.HasPrefix(name, ".") {
		return "", nil
	}
	// Skip gophermap files
	if name == "gophermap" {
		return "", nil
	}

	// Always use path.Join for gopher selectors
	fullSelector := path.Join("/"+selector, name)

	// Directories
	if e.IsDir() {
		return "1" + "📁 " + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n", nil
	}

	// Files
	ext := strings.ToLower(filepath.Ext(name))

	var itemType string
	var pictogram string
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif":
		itemType = "I"
		pictogram = "🖼️ "
	case ".txt", ".md", ".log":
		itemType = "0"
		pictogram = "📄 "
	default:
		itemType = "9"
		pictogram = "⚙︎ "
	}

	return itemType + pictogram + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n", nil
}

func sortDirectoryEntries(entries []os.DirEntry) {
	// Sort entries by name, case-insensitive
	sort.Slice(entries, func(i, j int) bool {
		iIsDir := entries[i].IsDir()
		jIsDir := entries[j].IsDir()

		// Directories first
		if iIsDir != jIsDir {
			return !iIsDir
		}

		// Same type → sort by name
		return entries[i].Name() < entries[j].Name()
	})
}
