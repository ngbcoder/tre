package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/melis/tre/internal/tree"
)

// sample builds a deterministic tree for the renderers.
//
//	root
//	├── a          (dir)
//	│   ├── deep   (dir, empty)
//	│   └── a1.txt
//	└── b.txt
func sample() *tree.Node {
	deep := &tree.Node{Name: "deep", Path: "a/deep", Type: tree.TypeDir}
	a1 := &tree.Node{Name: "a1.txt", Path: "a/a1.txt", Type: tree.TypeFile, Size: 512}
	a := &tree.Node{
		Name:     "a",
		Path:     "a",
		Type:     tree.TypeDir,
		Children: []*tree.Node{deep, a1},
	}
	b := &tree.Node{Name: "b.txt", Path: "b.txt", Type: tree.TypeFile, Size: 1024 * 1024}
	return &tree.Node{
		Name:     "root",
		Path:     "root",
		Type:     tree.TypeDir,
		Children: []*tree.Node{a, b},
	}
}

func renderToString(t *testing.T, format string, showSize bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Render(&buf, sample(), format, showSize); err != nil {
		t.Fatalf("Render(%s): %v", format, err)
	}
	return buf.String()
}

func TestText(t *testing.T) {
	want := strings.Join([]string{
		"root",
		"├── a",
		"│   ├── deep",
		"│   └── a1.txt",
		"└── b.txt",
		"",
	}, "\n")

	if got := renderToString(t, FormatText, false); got != want {
		t.Errorf("Text() =\n%s\nwant\n%s", got, want)
	}
}

func TestTextWithSizes(t *testing.T) {
	// The size column is measured across the whole tree, so both annotations
	// share a width and line up.
	want := strings.Join([]string{
		"root",
		"├── a",
		"│   ├── deep",
		"│   └── a1.txt  [512B]",
		"└── b.txt  [1.0M]",
		"",
	}, "\n")

	if got := renderToString(t, FormatText, true); got != want {
		t.Errorf("Text(--size) =\n%s\nwant\n%s", got, want)
	}
}

func TestTextRootOnly(t *testing.T) {
	var buf bytes.Buffer
	root := &tree.Node{Name: "empty", Path: "empty", Type: tree.TypeDir}
	if err := Text(&buf, root, false); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "empty\n" {
		t.Errorf("Text() = %q, want %q", got, "empty\n")
	}
}

func TestJSON(t *testing.T) {
	got := renderToString(t, FormatJSON, false)

	// children must be absent on leaves so consumers can tell an empty
	// directory from a file.
	if strings.Contains(got, `"deep": {`+"\n"+`      "name": "deep",`) {
		t.Error("unexpected formatting")
	}
	for _, want := range []string{
		`"name": "root"`,
		`"type": "directory"`,
		`"name": "a1.txt"`,
		`"type": "file"`,
		`"size": 1048576`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON output missing %s:\n%s", want, got)
		}
	}

	deep := extract(t, got, "deep")
	if strings.Contains(deep, "children") {
		t.Errorf("an empty directory should not carry a children key:\n%s", deep)
	}
	a1 := extract(t, got, "a1.txt")
	if strings.Contains(a1, "children") {
		t.Errorf("a file must not carry a children key:\n%s", a1)
	}
}

func TestMarkdown(t *testing.T) {
	want := strings.Join([]string{
		"- root",
		"  - a",
		"    - deep",
		"    - a1.txt",
		"  - b.txt",
		"",
	}, "\n")

	if got := renderToString(t, FormatMarkdown, false); got != want {
		t.Errorf("Markdown() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sample(), "xml", false); err == nil {
		t.Error("expected an error for an unknown format")
	}
}

// extract returns the JSON object whose "name" field equals name.
func extract(t *testing.T, doc, name string) string {
	t.Helper()
	marker := `"name": "` + name + `"`
	i := strings.Index(doc, marker)
	if i < 0 {
		t.Fatalf("no entry named %q in:\n%s", name, doc)
	}
	start := strings.LastIndex(doc[:i], "{")
	// Walk braces to find the matching close, ignoring braces inside strings.
	depth, inStr, esc := 0, false, false
	for j := start; j < len(doc); j++ {
		c := doc[j]
		switch {
		case esc:
			esc = false
		case inStr && c == '\\':
			esc = true
		case c == '"':
			inStr = !inStr
		case inStr:
			// skip
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return doc[start : j+1]
			}
		}
	}
	t.Fatalf("unterminated object for %q", name)
	return ""
}
