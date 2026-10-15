# tre

A fast, gitignore-aware directory tree printer.

Classic `tree` will happily dump 50,000 paths including your build output.
`tre` respects `.gitignore` out of the box, walks directories concurrently, and
can cap depth when you just want a quick overview.

```
$ tre -s 2
.
├── cmd
│   └── root.go
├── internal
│   ├── cli
│   ├── humanize
│   ├── ignore
│   ├── render
│   ├── tree
│   └── walk
├── go.mod
├── go.sum
├── main.go
├── Makefile
└── README.md
```

## Install

```bash
go install github.com/ngbcoder/tre@latest
```

Or build from source:

```bash
make build      # produces ./tre
make install    # installs into $GOPATH/bin
```

## Usage

```
tre [path] [flags]
```

`path` defaults to the current directory and may be a file, in which case only
that file is reported.

| Flag | Long flag | Description | Default |
| --- | --- | --- | --- |
| `-s <N>` | `--shallow <N>` | Limit traversal to `N` levels below the root. `0` shows only the root, `-1` is unlimited. | `-1` |
| `-a` | `--all` | Include hidden files and directories (names starting with `.`). | `false` |
| `-g` | `--git-ignore` | Respect `.gitignore` rules. Already the default; provided for explicitness. | `true` |
| | `--no-git` | Disable `.gitignore` filtering and show ignored paths. | `false` |
| `-d` | `--dirs-only` | List directories only, omitting files. | `false` |
| `-f <fmt>` | `--format <fmt>` | Output format: `text`, `json` or `markdown`. | `text` |
| | `--size` | Show human-readable file sizes alongside names. | `false` |
| `-h` | `--help` | Show help and exit. | |

### Examples

```bash
# A quick two-level overview
tre -s 2

# Hidden entries, three levels deep, with sizes
tre -s 3 -a --size

# Directories only
tre -d

# Export the structure for further processing
tre --format json -s 4 > structure.json

# Pasted straight into a README
tre --format markdown -s 3

# Everything, including ignored paths
tre --no-git
```

## Output formats

**text** — the default, with box-drawing characters:

```
.
├── src
│   ├── nested
│   └── main.go  [512B]
└── README.md  [1.0M]
```

**json** — nested objects, so the structure survives round-tripping:

```json
{
  "name": "tre",
  "path": ".",
  "type": "directory",
  "size": 0,
  "children": [
    { "name": "go.mod", "path": "go.mod", "type": "file", "size": 196 }
  ]
}
```

`type` is one of `directory`, `file` or `symlink`. `children` is omitted for
leaves, so an empty directory is distinguishable from a file.

**markdown** — a nested bullet list:

```
- .
  - src
    - main.go
  - README.md
```

## Behaviour

**`.gitignore` is honoured by default**, including nested files and correct
precedence: a `.gitignore` deeper in the tree overrides a shallower one, and
negations work.

```gitignore
# root .gitignore
*.log
!important.log
```

```gitignore
# src/.gitignore
!important.log
```

Ignored *directories* are pruned, so `tre` never walks into a dead subtree.
One consequence, shared with git itself: if a directory is excluded, a
negation inside it cannot re-include a file, because the walk never descends
far enough to see it.

The matcher is verified against `git check-ignore` by a conformance test that
covers globs, anchors, `**`, character classes, directory-only rules and
nested precedence. Run `go test -short ./...` to skip it.

**Symlinks are listed but never followed**, so a circular link cannot hang the
walk. There is no `--follow-links` flag yet.

**Hidden entries** are those whose name begins with `.`. The Windows hidden
file *attribute* is not consulted; `--all` covers dotfiles only.

**Ordering** is directories first, then files and symlinks, each group sorted
case-insensitively.

**Errors** during the walk are non-fatal. An unreadable directory produces
`permission denied: <path>` on stderr and the rest of the tree is still
printed. Only a root that cannot be read at all is fatal, which exits `2`;
usage errors exit `1`.

## Architecture

```
cmd/            cobra wiring, flags -> Options
internal/cli    option validation
internal/walk   concurrent traversal, pruning, filters
internal/ignore gitignore parser and hierarchical matcher
internal/tree   node model and ordering
internal/render text, json and markdown output
internal/humanize  byte-size formatting
```

The walk is concurrent, one goroutine per directory, bounded by a semaphore of
`GOMAXPROCS * 4`. Each goroutine carries an **immutable** chain of the
`.gitignore` rules governing its path; `Chain.Push` returns a new value rather
than mutating, so sibling goroutines never observe each other's rules. A test
asserts the resulting tree is identical at parallelism 1, 2, 4, 8 and 64.

### Performance notes

Three things dominated the cost of a walk and are deliberately avoided:

* **Stat-ing entries.** `os.ReadDir` already carries each entry's size from the
  directory enumeration, so sizes come from `DirEntry.Info()` rather than a
  separate `os.Lstat`, which on Windows means opening the file. When nothing
  will render a size, no stat happens at all. Dropping a redundant stat per
  file cut a full walk of a 140k-file tree from 8.3s to 1.4s.
* **Re-joining path components per ignore level.** `Chain.Match` takes a
  single slash-relative string and addresses each level by scanning for
  separators, so matching allocates nothing per entry.
* **Matching whole paths for simple patterns.** A pattern with no separator,
  such as `*.log` or `node_modules`, is matched against the basename only,
  which is what git itself does.

Measured on the Go module cache (~140k files, ~300 nested `.gitignore` files,
Windows/amd64): the walk runs in roughly 1.4s with `--no-git` and 3s with
gitignore filtering, down from 8.3s and 12s before these changes. JSON output
and `--size` pay a stat cost by design.

## Development

```bash
make help       # list targets
make test       # run the suite
make race       # run under the race detector (needs a C toolchain)
make cover      # coverage report
make bench      # benchmarks
make dist       # cross-compile into ./dist
```

## Not implemented

From the original design's "future enhancements": colour coding, an
interactive TUI, and `--follow-links`.
