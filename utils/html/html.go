package html

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"html/template"
	"io"
)

// GetTemplate converts a Markdown string to HTML and returns it as template.HTML.
// It uses a custom renderer to remove paragraph tags from the output.
//
// Parameters:
//   - c: The Markdown content as a string.
//
// Returns:
//   - template.HTML: The converted HTML content.
func GetTemplate(c string) template.HTML {
	// Create a new renderer with custom options
	renderer := html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags,
		// Custom hook to handle paragraph nodes
		RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			// Check if the node is a paragraph
			if _, ok := node.(*ast.Paragraph); ok {
				if entering {
					// Write an empty string when entering a paragraph node
					w.Write([]byte(""))
				} else {
					// Write an empty string when exiting a paragraph node
					w.Write([]byte(""))
				}
				return ast.GoToNext, true
			}
			return ast.GoToNext, false
		},
	})
	// Convert the Markdown content to HTML
	htmlContent := markdown.ToHTML([]byte(c), nil, renderer)
	// Return the HTML content as template.HTML
	return template.HTML(htmlContent)
}
