package docgen

import (
	"fmt"
	"os"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

func MarkdownToHTML(fin string) error {
	md, err := os.ReadFile(fin)
	if err != nil {
		return err
	}

	htmlb := markdownToHTML(md)
	s := "<!DOCTYPE html>\n<html>\n<head>\n    <meta charset=\"utf-8\"> \n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n    <title>GoGo</title>\n    <style>\n        body {\n            font-family: -apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, Helvetica, Arial, sans-serif, \"Apple Color Emoji\", \"Segoe UI Emoji\", \"Segoe UI Symbol\";\n            font-size: 16px;\n            line-height: 1.5;\n            word-wrap: break-word;\n        }\n        img {\n            max-width: 100%;\n        }\n        \n    </style>\n</head>\n<body>\n</body>\n"
	fmt.Print(s)
	fmt.Print(string(htmlb))
	fmt.Println("</body>\n</html>\n")
	return err
}
func markdownToHTML(md []byte) []byte {
	// enable common extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	// parse markdown
	doc := p.Parse(md)
	// HTML renderer
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	renderer := html.NewRenderer(html.RendererOptions{Flags: htmlFlags})

	return markdown.Render(doc, renderer)
}
