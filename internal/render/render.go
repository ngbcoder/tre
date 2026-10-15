// Package render converts a tree into text, JSON or Markdown.
package render

import (
	"fmt"
	"io"

	"github.com/melis/tre/internal/tree"
)

// Supported output formats.
const (
	FormatText     = "text"
	FormatJSON     = "json"
	FormatMarkdown = "markdown"
)

// Render writes root in the requested format. The tree package's own type is
// used directly so the renderers stay free of conversion boilerplate.
func Render(w io.Writer, root *tree.Node, format string, showSize bool) error {
	switch format {
	case FormatText:
		return Text(w, root, showSize)
	case FormatJSON:
		return JSON(w, root)
	case FormatMarkdown:
		return Markdown(w, root)
	default:
		return fmt.Errorf("unknown format %q (want %s, %s or %s)",
			format, FormatText, FormatJSON, FormatMarkdown)
	}
}
