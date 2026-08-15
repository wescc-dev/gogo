package selectorHandler

import (
	"bufio"
	"bytes"
	"fmt"
	"gogopher/src/configuration"
	"gogopher/src/core"
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
	svrInfoViewProvider core.IServerInfoViewProvider
}

func NewDirectorySelectorHandler(
	svrInfoViewProvider core.IServerInfoViewProvider) ISelectorHandler {

	return &DirectorySelectorHandler{
		svrInfoViewProvider: svrInfoViewProvider,
	}
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
	svrInfo := d.svrInfoViewProvider.GetCurrentServerInfo()
	if err != nil {
		if _, eerr := fmt.Fprintf(conn,
			"3Error reading directory\t%s/\t%s\t%s\r\n",
			selectorPath, svrInfo.HostName, svrInfo.Port); eerr != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
		return fmt.Errorf("cannot read directory: %w", err)
	}
	if len(entries) == 0 {
		emptymsg := buildSelectorWithPictogram("3", "", "Directory is empty", "", svrInfo.HostName, svrInfo.Port)
		if _, err := conn.Write([]byte(emptymsg)); err != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
	} else {
		sortDirectoryEntries(entries)
		for _, e := range entries {
			if gopherEntry, err := buildGopherEntry(e, selector, svrInfo.HostName, svrInfo.Port); err != nil {
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
	svrInfo := d.svrInfoViewProvider.GetCurrentServerInfo()
	gophermapPath := filepath.Join(selectorPath, "gophermap")
	gophermapTemplatePath := filepath.Join(selectorPath, svrInfo.GophermapTemplateName)
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
		err := d.generateGopherMap(conn, svrInfo, selectorPath)
		if err != nil {
			return false, fmt.Errorf("cannot generate gophermap: %w", err)
		}
		log.Println("Generated gophermap for:", selectorPath)
		return true, nil
	}
	return false, nil
}

func (d *DirectorySelectorHandler) generateGopherMap(conn net.Conn, svrInfo core.ServerInfoView, dirPath string) error {
	if info, err := os.Stat(svrInfo.GopherRoot); !(err == nil && info.IsDir()) {
		if err := os.MkdirAll(svrInfo.GopherRoot, os.ModePerm); err != nil {
			return fmt.Errorf("GopherRoot %s does not exist and could not be created. No files will be served", svrInfo.GopherRoot)
		}
	}
	gophermapTemplatePath := filepath.Join(dirPath, svrInfo.GophermapTemplateName)
	templateBytes, templateErr := os.ReadFile(gophermapTemplatePath)
	if templateErr != nil {
		return fmt.Errorf("cannot open .gophermap: %w", templateErr)
	}
	templateContent := string(templateBytes)

	// Replace Tokens
	if content, err := d.replaceSingleTokens(conn, svrInfo, templateContent); err != nil {
		return fmt.Errorf("cannot replace tokens: %w", err)
	} else {
		templateContent = content
	}

	// Replace {{ENTRIES}} with directory entries
	if content, err := d.replaceDirectoryEntriesToken(dirPath, templateContent); err != nil {
		return fmt.Errorf("cannot replace directory entries token: %w", err)
	} else {
		templateContent = content
	}

	// Add the footer
	templateContent = d.addFooter(templateContent)

	// Write the gophermap
	if _, err := conn.Write([]byte(templateContent)); err != nil {
		return fmt.Errorf("cannot write gophermap: %w", err)
	}
	return nil
}

func (d *DirectorySelectorHandler) addFooter(templateContent string) string {
	templateContent += configuration.Footer
	return templateContent
}

func (d *DirectorySelectorHandler) replaceDirectoryEntriesToken(dirPath string, content string) (string, error) {
	// Now handle {{ENTRIES}}
	entries, err := d.generateEntries(dirPath)
	if err != nil {
		return content, err
	}
	// Replace tokens
	content = strings.ReplaceAll(content, "{{ENTRIES}}", entries)
	return content, nil
}

func (d *DirectorySelectorHandler) replaceSingleTokens(conn net.Conn, svrInfo core.ServerInfoView, content string) (string, error) {
	clientIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return content, err
	}

	// Replace simple tokens first
	tokens := map[string]string{
		"TITLE":               svrInfo.GopherHoleName,
		"HOST":                svrInfo.HostName,
		"PORT":                svrInfo.Port,
		"CLIENT_IP_ADDRESS":   clientIP,
		"SERVER":              svrInfo.ServerSoftwareName + " (" + svrInfo.ServerSoftwareVersion + ") " + svrInfo.ServerSoftwareLicense,
		"START_TIME":          svrInfo.StartTime.Format(time.RFC3339),
		"UPTIME":              utility.FormatDuration(svrInfo.Uptime),
		"CURRENT_CONNECTIONS": fmt.Sprintf("%d", svrInfo.CurrentConnections),
		"TOTAL_CONNECTIONS":   fmt.Sprintf("%d", svrInfo.TotalConnections),
		"OS":                  svrInfo.OS,
		"ARCH":                svrInfo.Architecture,
		"CPUS":                fmt.Sprintf("%d", svrInfo.NumCpus),
		"=":                   "======================================================================",
		"*":                   "**********************************************************************",
		"-":                   "----------------------------------------------------------------------",
	}

	for key, val := range tokens {
		content = strings.ReplaceAll(content, "{{"+key+"}}", val)
	}
	return content, nil
}

func (d *DirectorySelectorHandler) generateEntries(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("cannot read gopher root: %w", err)
	}
	sortDirectoryEntries(entries)
	var entriesBuf bytes.Buffer
	w := bufio.NewWriter(&entriesBuf)
	svrInfo := d.svrInfoViewProvider.GetCurrentServerInfo()
	for _, e := range entries {
		if s, err := buildGopherEntry(e, strings.TrimPrefix(dirPath, svrInfo.GopherRoot), svrInfo.HostName, svrInfo.Port); err != nil {
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
		return buildSelectorWithPictogram("1", "📁 ", name, fullSelector, host, port), nil
	}

	return buildSelector(name, fullSelector, host, port), nil
}

func buildSelector(name string, fullSelector string, host string, port string) string {
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
	return buildSelectorWithPictogram(itemType, pictogram, name, fullSelector, host, port)
}

func buildSelectorWithPictogram(itemType string, pictogram string, name string, fullSelector string, host string, port string) string {
	return itemType + pictogram + name + "\t" + fullSelector + "\t" + host + "\t" + port + "\r\n"
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
