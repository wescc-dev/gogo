package selectors

import (
	"gogo/src/core"
	"gogo/src/utility"
	"path/filepath"
	"strings"
)

type TemplateSelector struct {
	svrInfoProvider core.IServerInfoProvider
}

func NewTemplateSelector(svrInfoProvider core.IServerInfoProvider) Selector {
	return &TemplateSelector{svrInfoProvider: svrInfoProvider}
}
func (s *TemplateSelector) Select(ctx *core.RequestContext) (*SelectResult, error) {
	result := &SelectResult{Handled: false}
	filePath := filepath.Join(ctx.Request.RootDir, ctx.Request.Selector)
	if !utility.FileExists(filePath) {
		return result, nil
	}
	if strings.HasSuffix(strings.ToLower(ctx.Request.Selector), ".tmpl") {
		err := ProcessTemplate(ctx, s.svrInfoProvider.GetCurrentServerInfo(), ctx.Request.RootDir, filePath)
		if err != nil {
			return result, err
		}
	}
	result.Handled = true
	return result, nil
}
