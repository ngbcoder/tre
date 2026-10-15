package ignore

import (
	"os"
	"path/filepath"
	"strings"
)

// PatternSet is the ordered rule list parsed from a single .gitignore file.
type PatternSet struct {
	patterns []*Pattern
}

// Parse compiles the contents of a .gitignore file.
// Malformed lines are skipped rather than failing the whole walk.
func Parse(contents string) *PatternSet {
	set := &PatternSet{}
	for _, line := range strings.Split(contents, "\n") {
		p, err := compile(line)
		if err != nil || p == nil {
			continue
		}
		set.patterns = append(set.patterns, p)
	}
	return set
}

func (s *PatternSet) empty() bool { return s == nil || len(s.patterns) == 0 }

// Match applies the rules to a path relative to this .gitignore's directory.
// Within a single file the last matching pattern wins.
func (s *PatternSet) Match(rel, base string, isDir bool) (ignored, matched bool) {
	return s.match(rel, base, isDir, true)
}

// MatchEntry is Match without the check for entries nested inside a matched
// directory. It is valid only when the caller has already pruned ignored
// directories, so that no reachable entry can have an ignored ancestor.
func (s *PatternSet) MatchEntry(rel, base string, isDir bool) (ignored, matched bool) {
	return s.match(rel, base, isDir, false)
}

func (s *PatternSet) match(rel, base string, isDir, nested bool) (ignored, matched bool) {
	if s.empty() {
		return false, false
	}
	for _, p := range s.patterns {
		var hit bool
		if nested {
			hit = p.match(rel, base, isDir)
		} else {
			hit = p.matchSelf(rel, base, isDir)
		}
		if hit {
			ignored = !p.negate
			matched = true
		}
	}
	return ignored, matched
}

// Load reads <dir>/.gitignore. It reports ok=false when no such file exists.
func Load(dir string) (set *PatternSet, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, true, err
	}
	return Parse(string(data)), true, nil
}

// Chain is an immutable stack of the PatternSets governing a directory,
// ordered from the walk root down to the directory itself.
//
// Immutability is what makes the concurrent walk race-free: each goroutine
// carries its own Chain and derives children with Push, so no goroutine ever
// mutates state another one is reading.
type Chain struct {
	levels []level
}

type level struct {
	patterns *PatternSet
}

// Push returns a new Chain that also applies the given rules. The receiver is
// left untouched.
func (c Chain) Push(set *PatternSet) Chain {
	if set.empty() {
		return c
	}
	levels := make([]level, len(c.levels), len(c.levels)+1)
	copy(levels, c.levels)
	levels = append(levels, level{patterns: set})
	return Chain{levels: levels}
}

// Len reports how many .gitignore files are in effect.
func (c Chain) Len() int { return len(c.levels) }

// Match reports whether a path is ignored, given as a slash-separated path
// relative to the walk root. Rules from deeper .gitignore files override
// shallower ones.
//
// Each level only needs the suffix of rel below its own directory, so the
// levels are addressed by scanning for separators rather than re-joining path
// components. That keeps the walk allocation-free per entry, which matters
// because this runs once for every file in the tree.
func (c Chain) Match(rel string, isDir bool) bool {
	return c.match(rel, isDir, true)
}

// MatchEntry is Match for a walk that has already pruned ignored directories.
// See PatternSet.MatchEntry.
func (c Chain) MatchEntry(rel string, isDir bool) bool {
	return c.match(rel, isDir, false)
}

func (c Chain) match(rel string, isDir, nested bool) bool {
	if len(c.levels) == 0 {
		return false
	}

	// Single-segment patterns only ever compare against the final component,
	// which is the same at every level.
	base := rel
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		base = rel[i+1:]
	}

	ignored := false
	matched := false

	for _, lvl := range c.levels {
		if rel == "" {
			break
		}
		var ig, ok bool
		if nested {
			ig, ok = lvl.patterns.Match(rel, base, isDir)
		} else {
			ig, ok = lvl.patterns.MatchEntry(rel, base, isDir)
		}
		if ok {
			ignored = ig
			matched = true
		}
		// Descend one directory level for the next set of rules.
		i := strings.IndexByte(rel, '/')
		if i < 0 {
			break
		}
		rel = rel[i+1:]
	}
	return matched && ignored
}
