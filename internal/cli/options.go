// Package cli validates user-supplied options and turns them into a walk.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/melis/tre/internal/render"
	"github.com/melis/tre/internal/tree"
	"github.com/melis/tre/internal/walk"
)

// Options is the fully resolved configuration for one invocation.
type Options struct {
	// Root is the path to display; defaults to the working directory.
	Root string
	// Depth is the maximum number of levels below Root to list.
	Depth int
	// All includes entries whose name begins with a dot.
	All bool
	// UseGit applies .gitignore rules.
	UseGit bool
	// DirsOnly omits files and symlinks.
	DirsOnly bool
	// Size annotates output with human-readable file sizes.
	Size bool
	// Format is one of render.FormatText, render.FormatJSON or FormatMarkdown.
	Format string
	// Out receives the rendered tree.
	Out io.Writer
	// Warnings receives non-fatal diagnostics.
	Warnings io.Writer
}

// Validate reports whether the options describe a runnable command.
func (o *Options) Validate() error {
	switch o.Format {
	case render.FormatText, render.FormatJSON, render.FormatMarkdown:
	default:
		return fmt.Errorf("unknown format %q (want %s, %s or %s)",
			o.Format, render.FormatText, render.FormatJSON, render.FormatMarkdown)
	}
	if o.Depth < walk.DepthUnlimited {
		return fmt.Errorf("invalid depth %d: use -1 for unlimited, 0 for the root only, or a positive number", o.Depth)
	}
	return nil
}

// Tree walks the filesystem and returns the resulting tree.
func (o *Options) Tree(ctx context.Context) (*tree.Node, error) {
	return walk.Options{
		Root:     o.Root,
		Depth:    o.Depth,
		All:      o.All,
		UseGit:   o.UseGit,
		DirsOnly: o.DirsOnly,
		// Sizes are collected only when something will actually show them.
		// The JSON format always reports size, regardless of --size.
		NeedSize: o.Size || o.Format == render.FormatJSON,
		Warnings: o.Warnings,
	}.Walk(ctx)
}

// Run walks the filesystem and writes the rendered tree to o.Out.
func (o *Options) Run(ctx context.Context) error {
	t, err := o.Tree(ctx)
	if err != nil {
		return err
	}
	return render.Render(o.Out, t, o.Format, o.Size)
}
