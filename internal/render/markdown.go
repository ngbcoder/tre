package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/melis/tre/internal/tree"
)

// Markdown writes root as a nested bullet list, suitable for pasting into
// READMEs and issue templates.
func Markdown(w io.Writer, root *tree.Node) error {
	buf := bufio.NewWriter(w)
	fmt.Fprintf(buf, "- %s\n", root.Path)
	writeMarkdownChildren(buf, root.Children, "  ")
	return buf.Flush()
}

func writeMarkdownChildren(buf *bufio.Writer, nodes []*tree.Node, indent string) {
	for _, n := range nodes {
		fmt.Fprintf(buf, "%s- %s\n", indent, n.Name)
		if len(n.Children) > 0 {
			writeMarkdownChildren(buf, n.Children, indent+"  ")
		}
	}
}
