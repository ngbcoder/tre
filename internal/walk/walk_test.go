package walk

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melis/tre/internal/tree"
)

// fixture builds a small tree and returns its root. The layout exercises
// directories, files, hidden entries and a .gitignore:
//
//	root/{a/{a1.txt, deep/d.txt}, b/b.txt, c.txt, ignoreddir/x.txt, ignored.txt}
//	root/{.hidden, .hiddendir/h.txt, .gitignore}
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	files := map[string]string{
		"a/a1.txt":         "a",
		"a/deep/d.txt":     "d",
		"b/b.txt":          "b",
		"c.txt":            "c",
		"ignoreddir/x.txt": "x",
		"ignored.txt":      "i",
		".hidden":          "h",
		".hiddendir/h.txt": "h",
		".gitignore":       "ignored.txt\nignoreddir/\n",
	}
	for name, contents := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// paths flattens the tree into sorted-looking slash-separated relative paths,
// depth first in the order the nodes were sorted.
func paths(n *tree.Node) []string {
	var out []string
	var walk func(prefix string, nodes []*tree.Node)
	walk = func(prefix string, nodes []*tree.Node) {
		for _, c := range nodes {
			p := c.Name
			if prefix != "" {
				p = prefix + "/" + c.Name
			}
			out = append(out, p)
			walk(p, c.Children)
		}
	}
	walk("", n.Children)
	return out
}

func names(nodes []*tree.Node) []string {
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

func TestDepth(t *testing.T) {
	root := fixture(t)

	tests := []struct {
		name  string
		depth int
		want  []string
	}{
		{"root only", 0, nil},
		{"one level", 1, []string{"a", "b", "c.txt"}},
		{"two levels", 2, []string{"a", "a/deep", "a/a1.txt", "b", "b/b.txt", "c.txt"}},
		{"three levels", 3, []string{"a", "a/deep", "a/deep/d.txt", "a/a1.txt", "b", "b/b.txt", "c.txt"}},
		{"unlimited", DepthUnlimited, []string{"a", "a/deep", "a/deep/d.txt", "a/a1.txt", "b", "b/b.txt", "c.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Options{Root: root, Depth: tt.depth, UseGit: true}.Walk(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if p := paths(got); !equal(p, tt.want) {
				t.Errorf("paths = %v\nwant     %v", p, tt.want)
			}
		})
	}
}

func TestDepthKeepsDirectoryAtTheLimit(t *testing.T) {
	root := fixture(t)

	// -s 2 should still show a/deep as a directory, just not its contents.
	got, err := Options{Root: root, Depth: 2}.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got.Children {
		if c.Name != "a" {
			continue
		}
		for _, gc := range c.Children {
			if gc.Name == "deep" && len(gc.Children) != 0 {
				t.Errorf("a/deep was expanded past the depth limit: %v", paths(gc))
			}
		}
	}
}

func TestHiddenEntries(t *testing.T) {
	root := fixture(t)

	got, err := Options{Root: root, Depth: 1, All: false}.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n := names(got.Children); !equal(n, []string{"a", "b", "ignoreddir", "c.txt", "ignored.txt"}) {
		t.Errorf("hidden entries leaked: %v", n)
	}

	got, err = Options{Root: root, Depth: 1, All: true}.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n := names(got.Children); !equal(n, []string{".hiddendir", "a", "b", "ignoreddir", ".gitignore", ".hidden", "c.txt", "ignored.txt"}) {
		t.Errorf("--all did not include hidden entries: %v", n)
	}
}

func TestGitIgnore(t *testing.T) {
	root := fixture(t)

	t.Run("on by default in the caller", func(t *testing.T) {
		got, err := Options{Root: root, Depth: 1, UseGit: true}.Walk(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if n := names(got.Children); !equal(n, []string{"a", "b", "c.txt"}) {
			t.Errorf("ignored paths not filtered: %v", n)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		got, err := Options{Root: root, Depth: 1, UseGit: false}.Walk(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if n := names(got.Children); !equal(n, []string{"a", "b", "ignoreddir", "c.txt", "ignored.txt"}) {
			t.Errorf("--no-git should show everything: %v", n)
		}
	})
}

// TestGitIgnorePrunesSubtree confirms ignored directories are never descended
// into, which is what keeps tre fast on large repositories.
func TestGitIgnorePrunesSubtree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, ".gitignore"), "node_modules\n")
	write(t, filepath.Join(root, "node_modules", "pkg", "deep", "index.js"), "x")

	got, err := Options{Root: root, Depth: DepthUnlimited, UseGit: true}.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p := paths(got); len(p) != 0 {
		// The only surviving file is .gitignore, which is hidden, so once the
		// ignored subtree has been pruned the root has no visible children.
		t.Errorf("ignored subtree was walked: %v", p)
	}
}

func TestDirsOnly(t *testing.T) {
	root := fixture(t)

	got, err := Options{Root: root, Depth: 2, DirsOnly: true}.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p := paths(got); !equal(p, []string{"a", "a/deep", "b", "ignoreddir"}) {
		t.Errorf("paths = %v, want directories only", p)
	}
	for _, c := range got.Children {
		if c.Type != tree.TypeDir {
			t.Errorf("-d returned a non-directory: %s (%s)", c.Name, c.Type)
		}
	}
}

// TestSymlinksAreNotFollowed covers the cycle-safety guarantee: links appear in
// the listing but are never descended into.
func TestSymlinksAreNotFollowed(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "real", "inner.txt"), "x")

	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "real"), link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	// A self-referential link would loop forever if we followed it.
	if err := os.Symlink(root, filepath.Join(root, "real", "loop")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	// Depth must be set explicitly: the zero value means "root only".
	got, err := Options{Root: root, Depth: DepthUnlimited, All: true}.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var linkNode *tree.Node
	for _, c := range got.Children {
		if c.Name == "link" {
			linkNode = c
		}
	}
	if linkNode == nil {
		t.Fatalf("symlink not listed; got %v", names(got.Children))
	}
	if linkNode.Type != tree.TypeSymlink {
		t.Errorf("link type = %s, want %s", linkNode.Type, tree.TypeSymlink)
	}
	if len(linkNode.Children) != 0 {
		t.Errorf("symlink was traversed: %v", paths(linkNode))
	}
	// "real" sorts before "link" because directories are grouped first and a
	// symlink is grouped with files.
	if p := paths(got); !equal(p, []string{"real", "real/inner.txt", "real/loop", "link"}) {
		t.Errorf("paths = %v", p)
	}
}

func TestPermissionDeniedIsAWarning(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "locked"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "root.txt"), "x")
	write(t, filepath.Join(root, "locked", "secret.txt"), "x")
	write(t, filepath.Join(root, "ok", "fine.txt"), "x")

	var warnings strings.Builder
	// Injecting readDir is the only portable way to exercise this on Windows,
	// where chmod does not deny access to the owner.
	opts := Options{
		Root:     root,
		Depth:    2,
		All:      true,
		Warnings: &warnings,
		readDir: func(path string) ([]os.DirEntry, error) {
			if filepath.Base(path) == "locked" {
				return nil, fs.ErrPermission
			}
			return os.ReadDir(path)
		},
	}

	got, err := opts.Walk(context.Background())
	if err != nil {
		t.Fatalf("an unreadable directory must not abort the walk: %v", err)
	}
	if !strings.Contains(warnings.String(), "permission denied") {
		t.Errorf("warnings = %q, want a permission denied message", warnings.String())
	}
	if !strings.Contains(warnings.String(), "locked") {
		t.Errorf("warnings = %q, want the offending path", warnings.String())
	}
	// The locked directory is still listed; only its contents are missing.
	if p := paths(got); !equal(p, []string{"locked", "ok", "ok/fine.txt", "root.txt"}) {
		t.Errorf("paths = %v", p)
	}
}

func TestRootIsFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "solo.txt")
	write(t, file, "hello")

	got, err := Options{Root: file}.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != tree.TypeFile || got.Name != "solo.txt" {
		t.Errorf("got %s %q, want file solo.txt", got.Type, got.Name)
	}
	if len(got.Children) != 0 {
		t.Errorf("a file root should have no children: %v", paths(got))
	}
}

func TestMissingRoot(t *testing.T) {
	_, err := Options{Root: filepath.Join(t.TempDir(), "nope")}.Walk(context.Background())
	if err == nil {
		t.Fatal("expected an error for a missing root")
	}
	var access *AccessError
	if !asAccessError(err, &access) {
		t.Fatalf("error %v is not an *AccessError, so main would not exit 2", err)
	}
}

func asAccessError(err error, target **AccessError) bool {
	e, ok := err.(*AccessError)
	if ok {
		*target = e
	}
	return ok
}

// TestParallelismIsDeterministic is the regression test for the concurrent
// walk: whatever the worker count, the tree must come out identical.
func TestParallelismIsDeterministic(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 40; i++ {
		dir := filepath.Join(root, "d"+strings.Repeat("x", i%5), "s"+string(rune('a'+i%7)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 5; j++ {
			write(t, filepath.Join(dir, "f"+string(rune('a'+j))+".txt"), "x")
		}
	}
	write(t, filepath.Join(root, ".gitignore"), "dxxx*\n")

	var want []string
	for _, p := range []int{1, 2, 4, 8, 64} {
		got, err := Options{Root: root, Depth: DepthUnlimited, All: true, UseGit: true, Parallelism: p}.Walk(context.Background())
		if err != nil {
			t.Fatalf("parallelism %d: %v", p, err)
		}
		cur := paths(got)
		if want == nil {
			want = cur
			// Guard against a vacuous test: a fixture that walked to nothing
			// would compare equal at every parallelism.
			if len(want) < 40 {
				t.Fatalf("fixture too small to be meaningful: %d nodes", len(want))
			}
			continue
		}
		if !equal(cur, want) {
			t.Fatalf("parallelism %d produced a different tree:\n got %v\nwant %v", p, cur, want)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	root := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := Options{Root: root, Depth: DepthUnlimited, Parallelism: 1}.Walk(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Cancellation degrades to a partial tree rather than a crash.
	if got == nil {
		t.Fatal("expected a root node even when cancelled")
	}
}

func write(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkWalkFixture(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 200; i++ {
		dir := filepath.Join(root, "pkg", string(rune('a'+i%26)), string(rune('a'+(i/26)%26)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		for j := 0; j < 10; j++ {
			if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := (Options{Root: root, Depth: DepthUnlimited, All: true}).Walk(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}
