package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ngbcoder/tre/internal/render"
	"github.com/ngbcoder/tre/internal/walk"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		depth   int
		wantErr bool
	}{
		{"defaults", render.FormatText, walk.DepthUnlimited, false},
		{"json", render.FormatJSON, 4, false},
		{"markdown", render.FormatMarkdown, 0, false},
		{"bad format", "xml", -1, true},
		{"empty format", "", -1, true},
		{"negative depth below -1", render.FormatText, -2, true},
		{"depth of -1 is unlimited", render.FormatText, -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := Options{Format: tt.format, Depth: tt.depth}
			err := o.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunEndToEnd(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "src", "nested"))
	writeFile(t, filepath.Join(root, "src", "main.go"), "x")
	writeFile(t, filepath.Join(root, "src", "nested", "util.go"), "x")
	writeFile(t, filepath.Join(root, "README.md"), "x")
	writeFile(t, filepath.Join(root, ".gitignore"), "*.log\n")
	writeFile(t, filepath.Join(root, "debug.log"), "x")

	var out strings.Builder
	o := Options{
		Root:     root,
		Depth:    3,
		UseGit:   true,
		Format:   render.FormatText,
		Size:     true,
		Out:      &out,
		Warnings: &strings.Builder{},
	}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := o.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "main.go") {
		t.Errorf("output missing main.go:\n%s", got)
	}
	if !strings.Contains(got, "util.go") {
		t.Errorf("output missing nested/util.go at depth 3:\n%s", got)
	}
	// .gitignore filters debug.log, and the leading dot hides the file itself.
	if strings.Contains(got, "debug.log") {
		t.Errorf("ignored file was listed:\n%s", got)
	}
	if strings.Contains(got, ".gitignore") {
		t.Errorf("hidden entries are filtered by default, so .gitignore must be absent:\n%s", got)
	}
}

func TestRunMissingRoot(t *testing.T) {
	o := Options{
		Root:   filepath.Join(t.TempDir(), "nope"),
		Format: render.FormatText,
		Out:    &strings.Builder{},
	}
	err := o.Run(context.Background())
	if err == nil {
		t.Fatal("expected an error for a missing root")
	}
	if _, ok := err.(*walk.AccessError); !ok {
		t.Errorf("error %v should be a *walk.AccessError", err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
