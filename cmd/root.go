// Package cmd wires the command-line interface to the internal packages.
package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ngbcoder/tre/internal/cli"
	"github.com/ngbcoder/tre/internal/render"
	"github.com/ngbcoder/tre/internal/walk"
)

// ExecuteContext runs the tre command.
func ExecuteContext(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	var (
		depth     int
		all       bool
		gitIgnore bool
		noGit     bool
		dirsOnly  bool
		format    string
		size      bool
	)

	cmd := &cobra.Command{
		Use:   "tre [path]",
		Short: "Display a directory hierarchy as a tree",
		Long: strings.TrimSpace(`
tre prints a directory hierarchy using box-drawing characters.

It is gitignore-aware by default, so build output and other ignored paths do
not clutter the output, and it can cap traversal depth for a quick overview.

Examples:
  tre                       show the current directory
  tre -s 2                  show only two levels below the root
  tre -s 3 -a --size        include hidden entries, three levels, with sizes
  tre -d                    directories only
  tre --format json -s 4    emit JSON for further processing
  tre --no-git              ignore .gitignore rules and show everything`),
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}

			opts := cli.Options{
				Root:     root,
				Depth:    depth,
				All:      all,
				UseGit:   gitIgnore && !noGit,
				DirsOnly: dirsOnly,
				Size:     size,
				Format:   format,
				Out:      cmd.OutOrStdout(),
				Warnings: cmd.ErrOrStderr(),
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Run(cmd.Context())
		},
	}

	f := cmd.Flags()
	f.IntVarP(&depth, "shallow", "s", walk.DepthUnlimited,
		"limit traversal to N levels below the root (0 shows only the root, -1 is unlimited)")
	f.BoolVarP(&all, "all", "a", false, "include hidden files and directories")
	// -g is a no-op: gitignore filtering is already on by default. It is kept
	// so scripts can state the intent explicitly.
	f.BoolVarP(&gitIgnore, "git-ignore", "g", true, "respect .gitignore rules (the default)")
	f.BoolVar(&noGit, "no-git", false, "disable .gitignore filtering and show ignored paths")
	f.BoolVarP(&dirsOnly, "dirs-only", "d", false, "list directories only, omitting files")
	f.StringVarP(&format, "format", "f", render.FormatText, "output format: text, json or markdown")
	f.BoolVar(&size, "size", false, "display human-readable file sizes alongside names")

	cmd.SetVersionTemplate("{{.Version}}\n")
	return cmd
}
