package ignore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestConformanceWithGit checks tre's matcher against git's own implementation.
//
// Each case writes a .gitignore (plus a nested one), creates the referenced
// paths, then asks git whether each path is ignored and compares the verdict.
// This is the test that keeps the hand-rolled matcher honest, since gitignore
// semantics are easy to get subtly wrong.
//
// The test is skipped when git is unavailable.
func TestConformanceWithGit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git conformance in -short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tests := []struct {
		name       string
		rootRules  string
		nestedPath string // directory holding the nested .gitignore, "" for none
		nestedRule string
		files      []string
	}{
		{
			name:      "extension glob",
			rootRules: "*.log\n",
			files:     []string{"a.log", "a.txt", "src/b.log", "src/b.txt"},
		},
		{
			name:      "directory only",
			rootRules: "build/\n",
			files:     []string{"build/out.bin", "build/deep/out.bin", "builder/x.go", "src/build/y.go"},
		},
		{
			name:      "anchored to root",
			rootRules: "/only-root.txt\n",
			files:     []string{"only-root.txt", "src/only-root.txt"},
		},
		{
			name:      "leading doublestar",
			rootRules: "**/generated\n",
			files:     []string{"generated/a.go", "src/generated/b.go", "a/b/generated/c.go"},
		},
		{
			name:      "trailing doublestar",
			rootRules: "vendor/**\n",
			files:     []string{"vendor/a.go", "vendor/deep/b.go", "vendorish/c.go"},
		},
		{
			name:      "middle doublestar",
			rootRules: "a/**/b.txt\n",
			files:     []string{"a/b.txt", "a/x/b.txt", "a/x/y/b.txt", "ab.txt"},
		},
		{
			name:      "character class",
			rootRules: "[abc].txt\n",
			files:     []string{"a.txt", "b.txt", "d.txt"},
		},
		{
			name:      "negated class",
			rootRules: "[!abc].txt\n",
			files:     []string{"a.txt", "d.txt"},
		},
		{
			name:      "single character",
			rootRules: "?.log\n",
			files:     []string{"a.log", "ab.log"},
		},
		{
			name:      "comments and blanks",
			rootRules: "# a comment\n\na.log\n",
			files:     []string{"a.log", "# a comment"},
		},
		{
			name:      "multiple rules last wins",
			rootRules: "*.log\n!keep.log\n",
			files:     []string{"a.log", "keep.log"},
		},
		{
			name:       "nested override re-includes",
			rootRules:  "*.log\n",
			nestedPath: "src",
			nestedRule: "!important.log\n*.tmp\n",
			files: []string{
				"src/important.log", "src/debug.log", "src/scratch.tmp",
				"other/important.log", "other/scratch.tmp",
			},
		},
		{
			name:       "nested directory rule",
			rootRules:  "*.log\n",
			nestedPath: "src",
			nestedRule: "cache/\n",
			files:      []string{"src/cache/x.bin", "src/a.log", "cache/y.bin"},
		},
		{
			name:      "node_modules style",
			rootRules: "node_modules\n",
			files:     []string{"node_modules/pkg/index.js", "src/node_modules/a.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// git init is required for check-ignore to load .gitignore rules.
			runGit(t, dir, "init", "-q", ".")

			writeFile(t, filepath.Join(dir, ".gitignore"), tt.rootRules)
			if tt.nestedPath != "" {
				writeFile(t, filepath.Join(dir, tt.nestedPath, ".gitignore"), tt.nestedRule)
			}
			for _, f := range tt.files {
				full := filepath.Join(dir, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, full, "x")
			}

			for _, f := range tt.files {
				slashed := filepath.ToSlash(f)

				want := gitIgnores(t, dir, slashed)
				got := chainFor(t, dir, slashed).Match(slashed, false)
				if got != want {
					t.Errorf("path %q: tre says ignored=%v, git says ignored=%v", slashed, got, want)
				}
			}
		})
	}
}

// chainFor builds the ignore chain for a path the same way the walker does,
// pushing a .gitignore for the root and for every ancestor directory of rel.
func chainFor(t *testing.T, root, rel string) Chain {
	t.Helper()

	chain := Chain{}
	push := func(dir string) {
		set, ok, err := Load(dir)
		if ok && err == nil {
			chain = chain.Push(set)
		}
	}

	push(root)
	parts := strings.Split(filepath.ToSlash(rel), "/")
	cur := root
	// Every ancestor except the final component, which is the entry itself.
	for _, p := range parts[:len(parts)-1] {
		cur = filepath.Join(cur, p)
		push(cur)
	}
	return chain
}

// gitIgnores asks git whether a path is ignored. --no-index makes the check
// apply to untracked files too.
func gitIgnores(t *testing.T, dir, path string) bool {
	t.Helper()
	cmd := exec.Command("git", "check-ignore", "-q", "--no-index", "--", path)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); !ok {
		t.Fatalf("git check-ignore %q: %v", path, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("git check-ignore %q: unexpected exit %d", path, exitErr.ExitCode())
	}
	return false
}

func asExitError(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
