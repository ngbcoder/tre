package tree

import "testing"

func dir(name string, children ...*Node) *Node {
	return &Node{Name: name, Type: TypeDir, Children: children}
}

func file(name string) *Node { return &Node{Name: name, Type: TypeFile} }

func names(nodes []*Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSort(t *testing.T) {
	tests := []struct {
		name  string
		input []*Node
		want  []string
	}{
		{
			name:  "directories first",
			input: []*Node{file("a.txt"), dir("z")},
			want:  []string{"z", "a.txt"},
		},
		{
			name:  "case insensitive within a group",
			input: []*Node{file("README.md"), file("main.go"), file("zebra.txt")},
			want:  []string{"main.go", "README.md", "zebra.txt"},
		},
		{
			name:  "directories sort among themselves",
			input: []*Node{dir("internal"), dir("cmd"), dir("assets")},
			want:  []string{"assets", "cmd", "internal"},
		},
		{
			name:  "names differing only by case keep a stable order",
			input: []*Node{file("Foo.txt"), file("foo.txt")},
			want:  []string{"Foo.txt", "foo.txt"},
		},
		{
			name:  "symlinks group with files",
			input: []*Node{file("a.txt"), {Name: "link", Type: TypeSymlink}, dir("d")},
			want:  []string{"d", "a.txt", "link"},
		},
		{
			name:  "empty",
			input: nil,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := append([]*Node(nil), tt.input...)
			Sort(got)
			if n := names(got); !equal(n, tt.want) {
				t.Errorf("Sort() = %v, want %v", n, tt.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	root := dir("root",
		dir("a", file("a1.txt"), dir("deep", file("d.txt"))),
		file("b.txt"),
	)
	// Two nodes under a, one under a/deep, one at the top level.
	if got := root.Count(); got != 5 {
		t.Errorf("Count() = %d, want 5", got)
	}
	if got := (&Node{}).Count(); got != 0 {
		t.Errorf("Count() on an empty node = %d, want 0", got)
	}
}

func TestAddChildSkipsNil(t *testing.T) {
	root := dir("root")
	root.AddChild(nil)
	root.AddChild(file("a.txt"))
	if got := len(root.Children); got != 1 {
		t.Errorf("len(Children) = %d, want 1", got)
	}
}
