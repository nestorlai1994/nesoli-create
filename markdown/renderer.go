package markdown

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// wikilink pattern: [[target]] or [[target|display]]
var wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|([^\]]+?))?\]\]`)

// RenderResult holds the rendered HTML and extracted frontmatter.
type RenderResult struct {
	HTML        string                 `json:"html"`
	Frontmatter map[string]interface{} `json:"frontmatter"`
}

// Render converts Obsidian-flavored Markdown to HTML.
func Render(source []byte) (RenderResult, error) {
	// Pre-process: convert wikilinks to standard markdown links
	processed := wikilinkRe.ReplaceAllFunc(source, func(match []byte) []byte {
		parts := wikilinkRe.FindSubmatch(match)
		target := string(parts[1])
		display := target
		if len(parts) > 2 && len(parts[2]) > 0 {
			display = string(parts[2])
		}
		return []byte("[" + display + "](/notes/" + target + ")")
	})

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	ctx := parser.NewContext()
	var buf bytes.Buffer
	if err := md.Convert(processed, &buf, parser.WithContext(ctx)); err != nil {
		return RenderResult{}, err
	}

	frontmatter := meta.Get(ctx)
	if frontmatter == nil {
		frontmatter = make(map[string]interface{})
	}

	return RenderResult{
		HTML:        buf.String(),
		Frontmatter: frontmatter,
	}, nil
}
