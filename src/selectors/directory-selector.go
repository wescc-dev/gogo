package selectors

import (
	"fmt"
	"gogo/src/configuration"
	"gogo/src/core"
	"gogo/src/utility"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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

	if tmpl, err := template.ParseFiles(gophermapTemplatePath); err != nil {
		return fmt.Errorf("cannot parse gophermap template: %w", err)
	} else {
		entries, err := readDirFiltered(dirPath)
		if err != nil {
			return fmt.Errorf("cannot read gopher root: %w", err)
		}
		sortDirectoryEntries(entries)
		var selectors []string
		for _, e := range entries {
			if s, err := buildGopherEntry(e, strings.TrimPrefix(dirPath, svrInfo.GopherRoot), svrInfo.HostName, svrInfo.Port); err != nil {
				core.ContextLog(ctx, err)
				continue
			} else {
				if s != "" {
					selectors = append(selectors, s)
				}
			}
		}
		data := &struct {
			SvrInfo core.ServerInfoView
			Entries []string
		}{
			SvrInfo: svrInfo,
			Entries: selectors,
		}
		if err := tmpl.Execute(ctx.Request.Conn, data); err != nil {
			return fmt.Errorf("cannot execute gophermap template: %w", err)
		}
	}

	// Write the gophermap
	m, _ := configuration.GetMetadata()
	if _, err := ctx.Request.Conn.Write([]byte(m.Footer)); err != nil {
		return fmt.Errorf("cannot write gophermap: %w", err)
	}
	return nil
}

func (d *DirectorySelector) addFooter(templateContent string) string {
	m, _ := configuration.GetMetadata()
	templateContent += m.Footer
	return templateContent
}
