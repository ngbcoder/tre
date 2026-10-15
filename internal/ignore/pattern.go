package ignore

import (
	"regexp"
	"strings"
)

// Pattern is a single compiled line from a .gitignore file.
//
// Two regular expressions are produced for every pattern:
//   - self    matches the entry itself
//   - descend matches entries nested beneath a directory the pattern matched
//
// The trailing slash only decides whether self may match a *file* of that
// name; it does not affect descend.
type Pattern struct {
	negate  bool
	dirOnly bool
	// baseOnly marks a pattern that is a single path segment and is not
	// anchored, such as "*.log" or "node_modules". Those only ever need to be
	// compared against the final path component, which is far cheaper than
	// scanning the whole path.
	baseOnly bool
	raw      string

	self    *regexp.Regexp
	descend *regexp.Regexp
}

// compile turns one raw .gitignore line into a Pattern.
// It returns (nil, nil) for blank lines, comments and lines that reduce to nothing.
func compile(line string) (*Pattern, error) {
	pat := stripTrailingSpace(line)
	if pat == "" || strings.HasPrefix(pat, "#") {
		return nil, nil
	}

	p := &Pattern{raw: line}

	if strings.HasPrefix(pat, "!") {
		p.negate = true
		pat = pat[1:]
	}
	if strings.HasSuffix(pat, "/") {
		p.dirOnly = true
		pat = strings.TrimSuffix(pat, "/")
	}
	// A leading backslash escapes a literal '#' or '!'.
	if strings.HasPrefix(pat, `\`) {
		pat = pat[1:]
	}
	if pat == "" {
		return nil, nil
	}

	// Per git: a separator at the beginning or middle of the pattern makes it
	// relative to the .gitignore's own directory. Otherwise it matches at any
	// level below, which for a single segment means "compare the basename".
	anchored := strings.Contains(pat, "/")
	pat = strings.TrimPrefix(pat, "/")
	if pat == "" {
		return nil, nil
	}
	p.baseOnly = !anchored

	body, err := globToRegexp(pat)
	if err != nil {
		return nil, err
	}

	prefix := `^(?:.*/)?`
	if p.baseOnly {
		// The level prefix is supplied by the caller as the basename.
		prefix = "^"
	} else if anchored {
		prefix = "^"
	}

	p.self, err = regexp.Compile(prefix + body + `$`)
	if err != nil {
		return nil, err
	}
	// Once a pattern matches a *directory*, git ignores everything beneath it,
	// whether or not the pattern was written with a trailing slash. So
	// "node_modules" covers "node_modules/pkg/index.js" just as "build/" does.
	// The trailing "/.*" requires at least one further segment, which keeps a
	// plain file named "build" from being matched by the pattern "build/".
	//
	// The nested check is only needed when matching a path directly. During a
	// walk the caller prunes ignored directories, so no reachable entry can
	// have an ignored ancestor and this never fires.
	descendPrefix := prefix
	if p.baseOnly {
		descendPrefix = `^(?:.*/)?`
	}
	p.descend, err = regexp.Compile(descendPrefix + body + `/.*$`)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// selfTarget picks the string a pattern's `self` expression should see: the
// basename for simple patterns, the whole relative path otherwise.
func (p *Pattern) selfTarget(rel, base string) string {
	if p.baseOnly {
		return base
	}
	return rel
}

// matchSelf reports whether the pattern matches the entry itself, without
// considering entries nested beneath a directory it matched.
func (p *Pattern) matchSelf(rel, base string, isDir bool) bool {
	return p.self.MatchString(p.selfTarget(rel, base)) && (!p.dirOnly || isDir)
}

// match reports whether the pattern covers the entry, either directly or
// because the entry sits inside a directory the pattern matched.
func (p *Pattern) match(rel, base string, isDir bool) bool {
	if p.matchSelf(rel, base, isDir) {
		return true
	}
	return p.descend.MatchString(rel)
}

// globToRegexp converts a slash-separated gitignore pattern into a regular
// expression body, honouring git's `**` rules.
func globToRegexp(pat string) (string, error) {
	segs := strings.Split(pat, "/")
	var b strings.Builder

	for i, seg := range segs {
		isLast := i == len(segs)-1

		if seg == "**" {
			switch {
			case len(segs) == 1:
				// Bare "**" matches everything.
				b.WriteString(`.*`)
			case i == 0:
				// "**/foo" == "foo" at any depth.
				b.WriteString(`(?:[^/]+/)*`)
			case isLast:
				// "abc/**" == everything inside abc, at any depth.
				b.WriteString(`.*`)
			default:
				// "a/**/b" == zero or more directories between.
				b.WriteString(`(?:[^/]+/)*`)
			}
			continue
		}

		s, err := segmentToRegexp(seg)
		if err != nil {
			return "", err
		}
		b.WriteString(s)

		if !isLast {
			b.WriteString(`/`)
		}
	}
	return b.String(), nil
}

// segmentToRegexp converts a single path segment's glob to a regular expression.
func segmentToRegexp(seg string) (string, error) {
	var b strings.Builder
	runes := []rune(seg)

	for i := 0; i < len(runes); i++ {
		switch c := runes[i]; c {
		case '\\':
			if i+1 < len(runes) {
				i++
				b.WriteString(regexp.QuoteMeta(string(runes[i])))
			} else {
				b.WriteString(regexp.QuoteMeta(`\`))
			}
		case '*':
			// "*" never crosses a path separator.
			b.WriteString(`[^/]*`)
		case '?':
			b.WriteString(`[^/]`)
		case '[':
			j := i + 1
			if j < len(runes) && (runes[j] == '!' || runes[j] == '^') {
				j++
			}
			if j < len(runes) && runes[j] == ']' {
				j++ // a literal ']' may appear first in the class
			}
			for j < len(runes) && runes[j] != ']' {
				j++
			}
			if j >= len(runes) {
				// Unterminated class: treat '[' as a literal.
				b.WriteString(regexp.QuoteMeta(`[`))
				continue
			}
			class := string(runes[i+1 : j])
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + class + "]")
			i = j
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String(), nil
}

// stripTrailingSpace removes trailing whitespace, honouring backslash escapes.
func stripTrailingSpace(s string) string {
	i := len(s)
	for i > 0 {
		c := s[i-1]
		if c != ' ' && c != '\t' && c != '\r' {
			break
		}
		backslashes := 0
		for j := i - 2; j >= 0 && s[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 1 {
			// The whitespace is escaped, so it is significant.
			break
		}
		i--
	}
	return s[:i]
}
