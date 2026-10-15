package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/melis/tre/internal/humanize"
	"github.com/melis/tre/internal/tree"
)

// Text writes root as a classic box-drawing tree.
//
// The size column is measured across the whole tree in a first pass so that
// every annotation lines up, regardless of traversal order.
func Text(w io.Writer, root *tree.Node, showSize bool) error {
	width := 0
	if showSize {
		width = sizeWidth(root)
	}

	buf := bufio.NewWriter(w)

	// The root is labelled with the path the user asked for, not its base name.
	label := root.Path
	if showSize && root.Type == tree.TypeFile {
		label = fmt.Sprintf("%s  [%*s]", root.Path, width, humanize.Size(root.Size))
	}
	fmt.Fprintln(buf, label)

	writeChildren(buf, root.Children, "", showSize, width)
	return buf.Flush()
}

func writeChildren(buf *bufio.Writer, nodes []*tree.Node, prefix string, showSize bool, width int) {
	for i, n := range nodes {
		connector, childPrefix := "├── ", prefix+"│   "
		if i == len(nodes)-1 {
			connector, childPrefix = "└── ", prefix+"    "
		}

		label := n.Name
		if showSize && n.Type == tree.TypeFile {
			label = fmt.Sprintf("%s  [%*s]", n.Name, width, humanize.Size(n.Size))
		}
		fmt.Fprintf(buf, "%s%s%s\n", prefix, connector, label)

		if len(n.Children) > 0 {
			writeChildren(buf, n.Children, childPrefix, showSize, width)
		}
	}
}

// sizeWidth is the rendered width of the widest size annotation in the tree.
func sizeWidth(n *tree.Node) int {
	width := 0
	for _, c := range n.Children {
		if c.Type == tree.TypeFile {
			if l := len(humanize.Size(c.Size)); l > width {
				width = l
			}
		}
		if w := sizeWidth(c); w > width {
			width = w
		}
	}
	return width
}
