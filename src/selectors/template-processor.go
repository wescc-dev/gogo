package selectors

import (
	"fmt"
	"gogo/src/core"
	"strings"
	"text/template"
)

func ProcessTemplate(ctx *core.RequestContext, svrInfo core.ServerInfo, dirPath string, templatePath string) error {
	if tmpl, err := template.ParseFiles(templatePath); err != nil {
		return fmt.Errorf("cannot parse template: %w", err)
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
			SvrInfo core.ServerInfo
			Entries []string
		}{
			SvrInfo: svrInfo,
			Entries: selectors,
		}

		if err := tmpl.Execute(ctx.Request.Conn, data); err != nil {
			return fmt.Errorf("cannot execute template: %w", err)
		}
	}
	return nil
}
