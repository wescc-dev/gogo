package selectors

import (
	"bufio"
	"bytes"
	"fmt"
	"gogo/src/configuration"
	"gogo/src/core"
	"gogo/src/utility"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DirectorySelector handles directory selectors
type DirectorySelector struct {
	svrInfoViewProvider core.IServerInfoViewProvider
}

func NewDirectorySelector(svrInfoViewProvider core.IServerInfoViewProvider) ISelector {
	return &DirectorySelector{
		svrInfoViewProvider: svrInfoViewProvider,
	}
}

func (d *DirectorySelector) Select(ctx *core.RequestContext) (*SelectResult, error) {
	result := &SelectResult{false}
	selectorPath := filepath.Join(ctx.Request.RootDir, ctx.Request.Selector)
	// If it's a directory
	if utility.IsDirectory(selectorPath) {
		err := d.serveDirectory(ctx, selectorPath, ctx.Request.Selector, ctx.Request.Timeout)
		if err != nil {
			return nil, fmt.Errorf("cannot serve directory: %w", err)
		}
		result.Handled = true
	}
	return result, nil
}

func (d *DirectorySelector) serveDirectory(ctx *core.RequestContext, selectorPath string, selector string, timeOut time.Duration) error {
	defer func(ctx *core.RequestContext, t time.Time) {
		_ = ctx.Request.Conn.SetReadDeadline(t)
	}(ctx, time.Time{})

	done, err := d.serveGopherMap(ctx, selectorPath)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	// Otherwise list directory
	entries, err := readDirFiltered(selectorPath)
	svrInfo := d.svrInfoViewProvider.GetCurrentServerInfo()
	if err != nil {
		err := WriteErrorToConn(ctx.Request.Conn, ctx.Request.Timeout, "Error reading directory", selectorPath)
		if err != nil {
			return err
		}
		return fmt.Errorf("cannot read directory: %w", err)
	}
	if len(entries) == 0 {
		empty := buildSelectorWithPictogram("3", "", "Directory is empty", "", svrInfo.HostName, svrInfo.Port)
		if _, err := ctx.Request.Conn.Write([]byte(empty)); err != nil {
			return fmt.Errorf("cannot write error message: %w", err)
		}
	} else {
		sortDirectoryEntries(entries)
		for _, e := range entries {
			if gopherEntry, err := buildGopherEntry(e, selector, svrInfo.HostName, svrInfo.Port); err != nil {
				core.ContextLog(ctx, err)
				continue
			} else {
				if err = ctx.Request.Conn.SetWriteDeadline(time.Now().Add(timeOut)); err != nil {
					return err
				}
				if _, err := ctx.Request.Conn.Write([]byte(gopherEntry)); err != nil {
					core.ContextLog(ctx, err)
					return err
				}
			}
		}
	}
	err = writeBannerToConn(ctx.Request.Conn, timeOut)
	if err != nil {
		return err
	}
	writeTerminationMarker(ctx.Request.Conn)
	return nil
}

func (d *DirectorySelector) serveGopherMap(ctx *core.RequestContext, selectorPath string) (bool, error) {
	svrInfo := d.svrInfoViewProvider.GetCurrentServerInfo()
	gophermapPath := filepath.Join(selectorPath, "gophermap")
	gophermapTemplatePath := filepath.Join(selectorPath, svrInfo.GophermapTemplateName)
	gopherMapExists := utility.FileExists(gophermapPath)
	gopherMapTemplateExists := utility.FileExists(gophermapTemplatePath)

	// Gophermap exists then serve it
	if gopherMapExists {
		err := writeFileToConn(ctx.Request.Conn, gophermapPath, ctx.Request.Timeout)
		if err != nil {
			return false, fmt.Errorf("cannot serve gophermap: %w", err)
		}
		return true, nil
	} else if gopherMapTemplateExists {
		err := d.generateGopherMap(ctx, svrInfo, selectorPath)
		if err != nil {
			return false, fmt.Errorf("cannot generate gophermap: %w", err)
		}
		core.ContextLog(ctx, "Generated gophermap for:", selectorPath)
		return true, nil
	}
	return false, nil
}

func (d *DirectorySelector) generateGopherMap(ctx *core.RequestContext, svrInfo core.ServerInfoView, dirPath string) error {
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
	if content, err := d.replaceSingleTokens(ctx.Request.Conn, svrInfo, templateContent); err != nil {
		return fmt.Errorf("cannot replace tokens: %w", err)
	} else {
		templateContent = content
	}

	// Replace {{ENTRIES}} with directory entries
	if content, err := d.replaceDirectoryEntriesToken(ctx, dirPath, templateContent); err != nil {
		return fmt.Errorf("cannot replace directory entries token: %w", err)
	} else {
		templateContent = content
	}

	// Add the footer
	templateContent = d.addFooter(templateContent)

	// Write the gophermap
	if _, err := ctx.Request.Conn.Write([]byte(templateContent)); err != nil {
		return fmt.Errorf("cannot write gophermap: %w", err)
	}
	return nil
}

func (d *DirectorySelector) addFooter(templateContent string) string {
	m, _ := configuration.GetMetadata()
	templateContent += m.Footer
	return templateContent
}

func (d *DirectorySelector) replaceDirectoryEntriesToken(ctx *core.RequestContext, dirPath string, content string) (string, error) {
	// Now handle {{ENTRIES}}
	entries, err := d.generateEntries(ctx, dirPath)
	if err != nil {
		return content, err
	}
	// Replace tokens
	content = strings.ReplaceAll(content, "{{ENTRIES}}", entries)
	return content, nil
}

func (d *DirectorySelector) replaceSingleTokens(conn net.Conn, svrInfo core.ServerInfoView, content string) (string, error) {
	clientIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return content, err
	}
	tls := "No"
	if svrInfo.TLSEnabled {
		tls = "Yes (TLS certificate is valid)"
	}

	// Replace simple tokens first
	tokens := map[string]string{
		"TITLE":               svrInfo.Title,
		"HOST":                svrInfo.HostName,
		"PORT":                svrInfo.Port,
		"TLS_ENABLED":         fmt.Sprintf("%s", tls),
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

func (d *DirectorySelector) generateEntries(ctx *core.RequestContext, dirPath string) (string, error) {
	entries, err := readDirFiltered(dirPath)
	if err != nil {
		return "", fmt.Errorf("cannot read gopher root: %w", err)
	}
	sortDirectoryEntries(entries)
	var entriesBuf bytes.Buffer
	w := bufio.NewWriter(&entriesBuf)
	svrInfo := d.svrInfoViewProvider.GetCurrentServerInfo()
	for _, e := range entries {
		if s, err := buildGopherEntry(e, strings.TrimPrefix(dirPath, svrInfo.GopherRoot), svrInfo.HostName, svrInfo.Port); err != nil {
			core.ContextLog(ctx, err)
			continue
		} else {
			_, _ = w.WriteString(s)
		}
	}
	_ = w.Flush()
	return entriesBuf.String(), nil
}
