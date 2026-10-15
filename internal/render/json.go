package render

import (
	"encoding/json"
	"io"

	"github.com/ngbcoder/tre/internal/tree"
)

// jsonNode is the wire format for --format json.
type jsonNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"`
	Size     int64       `json:"size"`
	Children []*jsonNode `json:"children,omitempty"`
}

// JSON writes root as an indented JSON document. children is omitted for
// leaves, so consumers can distinguish files from empty directories.
func JSON(w io.Writer, root *tree.Node) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Encode adds the trailing newline.
	return enc.Encode(toJSON(root))
}

func toJSON(n *tree.Node) *jsonNode {
	out := &jsonNode{
		Name: n.Name,
		Path: n.Path,
		Type: string(n.Type),
		Size: n.Size,
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, toJSON(c))
	}
	return out
}
