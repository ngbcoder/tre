package ignore

import (
	"path/filepath"
	"strings"
	"testing"
)

// matchIn reports whether path would be ignored by the given .gitignore
// contents, evaluated as if the file sat at the walk root.
func matchIn(gitignore, path string, isDir bool) bool {
	return Chain{}.Push(Parse(gitignore)).Match(filepath.ToSlash(path), isDir)
}

func TestPatternMatching(t *testing.T) {
	tests := []struct {
		name      string
		gitignore string
		path      string
		isDir     bool
		want      bool
	}{
		// Plain globs.
		{"log match", "*.log", "a.log", false, true},
		{"log miss", "*.log", "a.txt", false, false},
		{"question mark", "?at", "cat", false, true},
		{"question mark needs a char", "?at", "at", false, false},

		// Anchoring: a separator in the middle binds the pattern to the
		// .gitignore's own directory.
		{"unanchored matches at any depth", "*.log", "src/a.log", false, true},
		{"anchored root only", "/*.log", "src/a.log", false, false},
		{"anchored matches at root", "/*.log", "a.log", false, true},
		{"anchored middle segment", "doc/*.txt", "doc/a.txt", false, true},
		{"anchored does not recurse", "doc/*.txt", "doc/sub/a.txt", false, false},

		// Directory-only patterns.
		{"dir pattern matches dir", "build/", "build", true, true},
		{"dir pattern ignores same-named file", "build/", "build", false, false},
		{"dir pattern covers contents", "build/", "build/out.bin", false, true},
		{"dir pattern covers deep contents", "build/", "build/a/b/c.bin", false, true},
		{"dir pattern is not a prefix match", "build/", "buildx", true, false},
		{"dir pattern covers nested dir", "build/", "build/sub", true, true},

		// Comments and escapes.
		{"comment ignored", "# a.log", "a.log", false, false},
		{"escaped hash is a literal", `\#a.log`, "#a.log", false, true},
		{"escaped bang is a literal", `\!a.log`, "!a.log", false, true},
		{"blank lines ignored", "\n\n*.log\n\n", "a.log", false, true},
		{"trailing spaces stripped", "a.log   ", "a.log", false, true},
		{"trailing spaces significant mid-pattern", "a b", "a b", false, true},

		// Double star.
		{"leading doublestar zero dirs", "**/foo", "foo", false, true},
		{"leading doublestar one dir", "**/foo", "a/foo", false, true},
		{"leading doublestar many dirs", "**/foo", "a/b/foo", false, true},
		{"trailing doublestar", "abc/**", "abc/x", false, true},
		{"trailing doublestar deep", "abc/**", "abc/x/y", false, true},
		{"trailing doublestar not the parent", "abc/**", "abc", true, false},
		{"middle doublestar zero dirs", "a/**/b", "a/b", false, true},
		{"middle doublestar one dir", "a/**/b", "a/x/b", false, true},
		{"middle doublestar many dirs", "a/**/b", "a/x/y/b", false, true},
		{"middle doublestar no match", "a/**/b", "ab", false, false},
		{"doublestar inside segment is a star", "a**b", "axxb", false, true},
		{"doublestar inside segment is a star, no separator", "a**b", "ax/xb", false, false},

		// Character classes.
		{"class match", "[abc].txt", "b.txt", false, true},
		{"class miss", "[abc].txt", "d.txt", false, false},
		{"negated class match", "[!abc].txt", "d.txt", false, true},
		{"negated class miss", "[!abc].txt", "a.txt", false, false},
		{"unterminated class is a literal", "[abc.txt", "[abc.txt", false, true},

		// Negation, with last matching pattern winning.
		{"negation re-includes", "*.log\n!important.log", "important.log", false, false},
		{"order matters", "!important.log\n*.log", "important.log", false, true},
		{"negation of a directory pattern", "build/\n!build", "build", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchIn(tt.gitignore, tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("Match(%q, %q, isDir=%v) = %v, want %v",
					tt.gitignore, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

// TestChainPrecedence covers the rule that .gitignore files nest: a deeper
// file's rules override a shallower file's, which is what the lightweight
// third-party matchers get wrong.
func TestChainPrecedence(t *testing.T) {
	const debugLog = "src/debug.log"

	rootRules := Parse("*.log\nbuild/\n")
	shallow := Chain{}.Push(rootRules)
	deep := shallow.Push(Parse("!debug.log\n*.tmp"))

	if !shallow.Match(debugLog, false) {
		t.Error("root rules should ignore src/debug.log")
	}
	if deep.Match(debugLog, false) {
		t.Error("nested .gitignore should re-include src/debug.log")
	}
	if !deep.Match("src/a.tmp", false) {
		t.Error("nested .gitignore should ignore src/a.tmp")
	}
	if shallow.Match("src/a.tmp", false) {
		t.Error("root rules must not know about the nested *.tmp rule")
	}
}

// TestChainPushIsImmutable guards the invariant the concurrent walk relies on:
// Push must never mutate the receiver, or sibling goroutines would observe each
// other's rules.
func TestChainPushIsImmutable(t *testing.T) {
	base := Chain{}.Push(Parse("*.log"))
	before := base.Len()

	child := base.Push(Parse("*.tmp"))

	if base.Len() != before {
		t.Errorf("Push mutated the receiver: Len = %d, want %d", base.Len(), before)
	}
	if child.Len() != before+1 {
		t.Errorf("child Len = %d, want %d", child.Len(), before+1)
	}
}

func TestParseSkipsMalformedLines(t *testing.T) {
	// A bare "!" reduces to nothing, but an unterminated "[" is a valid
	// literal pattern (git's wildmatch agrees), so it must survive.
	set := Parse("*.log\n[\n" + strings.Repeat("!\n", 3) + "*.tmp\n")
	if set.empty() {
		t.Fatal("expected patterns to survive")
	}
	if got := len(set.patterns); got != 3 {
		t.Errorf("len(patterns) = %d, want 3", got)
	}
}
