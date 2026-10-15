// Package walk traverses a directory tree and builds an in-memory
// representation of it, applying depth, hidden-file and .gitignore filters.
package walk

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/ngbcoder/tre/internal/ignore"
	"github.com/ngbcoder/tre/internal/tree"
)

// DepthUnlimited traverses every level.
const DepthUnlimited = -1

// AccessError reports a failure to read the root path. It is distinct from
// per-directory warnings, which never abort the walk.
type AccessError struct {
	Path string
	Err  error
}

func (e *AccessError) Error() string {
	return fmt.Sprintf("cannot access %s: %v", e.Path, e.Err)
}

func (e *AccessError) Unwrap() error { return e.Err }

// Options configures a traversal.
type Options struct {
	// Root is the directory (or file) to start from.
	Root string
	// Depth limits how many levels below Root are listed. DepthUnlimited
	// traverses everything; 0 lists only the root itself.
	//
	// Note that the zero value of this field means "root only", so callers
	// that want a full walk must set it to DepthUnlimited explicitly.
	Depth int
	// All includes entries whose name begins with a dot.
	All bool
	// UseGit applies .gitignore rules found during the walk.
	UseGit bool
	// DirsOnly omits files and symlinks from the result.
	DirsOnly bool
	// NeedSize controls whether file sizes are collected. Stat-ing every entry
	// is by far the most expensive part of a walk, so it is skipped entirely
	// when the caller will not render sizes.
	NeedSize bool
	// Warnings receives non-fatal problems, e.g. unreadable directories.
	Warnings io.Writer
	// Parallelism caps the number of directories walked concurrently.
	// Zero selects GOMAXPROCS * 4.
	Parallelism int

	// Injectable for tests; defaults to the os package.
	readDir func(string) ([]os.DirEntry, error)
	lstat   func(string) (os.FileInfo, error)
}

type walker struct {
	opts        Options
	readDir     func(string) ([]os.DirEntry, error)
	lstat       func(string) (os.FileInfo, error)
	warnMu      sync.Mutex
	parallelism int
}

type subdir struct {
	idx  int
	path string
	rel  string
	info os.FileInfo
}

// Walk builds the tree rooted at opts.Root.
func (o Options) Walk(ctx context.Context) (*tree.Node, error) {
	w := &walker{
		opts:        o,
		readDir:     o.readDir,
		lstat:       o.lstat,
		parallelism: o.Parallelism,
	}
	if w.readDir == nil {
		w.readDir = os.ReadDir
	}
	if w.lstat == nil {
		w.lstat = os.Lstat
	}
	if w.parallelism <= 0 {
		w.parallelism = runtime.GOMAXPROCS(0) * 4
	}

	root := filepath.Clean(o.Root)
	info, err := w.lstat(root)
	if err != nil {
		return nil, &AccessError{Path: root, Err: err}
	}

	// A symlinked root is reported as a leaf; we never traverse through links.
	if info.Mode()&os.ModeSymlink != 0 {
		return &tree.Node{
			Name: displayName(root),
			Path: root,
			Type: tree.TypeSymlink,
			Size: info.Size(),
		}, nil
	}
	if !info.IsDir() {
		return &tree.Node{
			Name: displayName(root),
			Path: root,
			Type: tree.TypeFile,
			Size: info.Size(),
		}, nil
	}

	return w.dir(ctx, root, info, 0, "", ignore.Chain{}), nil
}

// dir walks a single directory. info is the already-known stat for the
// directory (nil when sizes are not being collected), depth is 0 for the root,
// rel is the slash-separated path from the walk root, and chain carries the
// .gitignore rules in effect. chain is treated as immutable.
func (w *walker) dir(ctx context.Context, path string, info os.FileInfo, depth int, rel string, chain ignore.Chain) *tree.Node {
	node := &tree.Node{
		Name: displayName(path),
		Path: path,
		Type: tree.TypeDir,
	}
	if info != nil {
		node.Size = info.Size()
	}

	// Depth limit reached: report the directory, but do not enumerate it.
	if w.opts.Depth != DepthUnlimited && depth >= w.opts.Depth {
		return node
	}

	entries, err := w.readDir(path)
	if err != nil {
		w.warnf("permission denied: %s", path)
		return node
	}

	// A directory's own .gitignore governs its contents, not the directory
	// itself, so it is loaded only after the parent has already decided to
	// keep it. Push returns a new Chain, so sibling goroutines never observe
	// each other's rules.
	if w.opts.UseGit {
		if set, ok, err := ignore.Load(path); ok {
			if err != nil {
				w.warnf("cannot read .gitignore in %s: %v", path, err)
			} else {
				chain = chain.Push(set)
			}
		}
	}

	children := make([]*tree.Node, 0, len(entries))
	subdirs := make([]subdir, 0, len(entries)/4)

	for _, e := range entries {
		name := e.Name()
		if !w.opts.All && strings.HasPrefix(name, ".") {
			continue
		}

		isSymlink := e.Type()&os.ModeSymlink != 0
		isDir := e.IsDir() && !isSymlink

		// The slash-relative path, built once and reused for the ignore
		// check and for the recursive call.
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}

		if w.opts.UseGit && chain.Len() > 0 && chain.MatchEntry(childRel, isDir) {
			// Pruned here, which is what keeps tre from walking dead trees.
			continue
		}

		childPath := filepath.Join(path, name)

		if isSymlink {
			if w.opts.DirsOnly {
				continue
			}
			children = append(children, &tree.Node{
				Name: name,
				Path: childPath,
				Type: tree.TypeSymlink,
				Size: sizeOf(w.infoOf(e)),
			})
			continue
		}

		if isDir {
			// Placeholder keeps children indices aligned with subdirs.
			subdirs = append(subdirs, subdir{
				idx:  len(children),
				path: childPath,
				rel:  childRel,
				info: w.infoOf(e),
			})
			children = append(children, nil)
			continue
		}

		if w.opts.DirsOnly {
			continue
		}
		children = append(children, &tree.Node{
			Name: name,
			Path: childPath,
			Type: tree.TypeFile,
			Size: sizeOf(w.infoOf(e)),
		})
	}

	if len(subdirs) > 0 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, w.parallelism)

		// The semaphore is acquired before spawning so that at most
		// `parallelism` goroutines exist at once, even on very wide trees.
		for _, sd := range subdirs {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				w.warnf("interrupted while walking %s", path)
			}
			if ctx.Err() != nil {
				break
			}

			wg.Add(1)
			go func(sd subdir) {
				defer wg.Done()
				defer func() { <-sem }()
				children[sd.idx] = w.dir(ctx, sd.path, sd.info, depth+1, sd.rel, chain)
			}(sd)
		}
		wg.Wait()
	}

	node.Children = compact(children)
	tree.Sort(node.Children)
	return node
}

// infoOf returns the stat for a directory entry, or nil when sizes are not
// wanted. os.ReadDir already carries the size from the directory enumeration,
// so this costs no additional syscall.
func (w *walker) infoOf(e os.DirEntry) os.FileInfo {
	if !w.opts.NeedSize {
		return nil
	}
	info, err := e.Info()
	if err != nil {
		// The entry vanished between listing and stat, which is routine while
		// a build rewrites the tree.
		return nil
	}
	return info
}

func sizeOf(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

// warnf writes a non-fatal diagnostic. Safe for concurrent use.
func (w *walker) warnf(format string, args ...any) {
	if w.opts.Warnings == nil {
		return
	}
	w.warnMu.Lock()
	defer w.warnMu.Unlock()
	fmt.Fprintf(w.opts.Warnings, format+"\n", args...)
}

// compact drops the nil placeholders left by pruned or failed branches.
func compact(nodes []*tree.Node) []*tree.Node {
	out := nodes[:0]
	for _, n := range nodes {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

// displayName picks a readable label for a path, resolving "." and ".." to the
// base name of the absolute path.
func displayName(p string) string {
	base := filepath.Base(p)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		if abs, err := filepath.Abs(p); err == nil {
			return filepath.Base(abs)
		}
	}
	return base
}
