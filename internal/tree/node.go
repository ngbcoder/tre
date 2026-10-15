package tree

import "strings"

// Type describes what a node points at.
type Type string

const (
	TypeDir     Type = "directory"
	TypeFile    Type = "file"
	TypeSymlink Type = "symlink"
)

// Node is one entry in the rendered tree. A node with children is always a
// directory; symlinks are listed but never populated.
type Node struct {
	Name     string
	Path     string
	Type     Type
	Size     int64
	Children []*Node
}

// AddChild appends a child, skipping nils so pruned branches disappear cleanly.
func (n *Node) AddChild(c *Node) {
	if c != nil {
		n.Children = append(n.Children, c)
	}
}

// Sort orders children directories-first, then files and symlinks, with each
// group ordered case-insensitively by name.
func Sort(nodes []*Node) {
	// Insertion sort: sibling lists are small and this avoids pulling in
	// sort.Interface boilerplate for a two-key comparison.
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && less(nodes[j], nodes[j-1]); j-- {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
		}
	}
}

func less(a, b *Node) bool {
	ad, bd := a.IsDir(), b.IsDir()
	if ad != bd {
		return ad // directories first
	}
	al, bl := strings.ToLower(a.Name), strings.ToLower(b.Name)
	if al != bl {
		return al < bl
	}
	return a.Name < b.Name // stable tie-break for names differing only by case
}

// IsDir reports whether the node should be grouped with directories.
func (n *Node) IsDir() bool { return n.Type == TypeDir }

// Count returns the number of nodes in the subtree rooted at n, excluding n.
func (n *Node) Count() int {
	total := 0
	for _, c := range n.Children {
		total += 1 + c.Count()
	}
	return total
}
