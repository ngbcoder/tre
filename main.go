// Command tre displays a directory hierarchy as a tree.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/ngbcoder/tre/cmd"
	"github.com/ngbcoder/tre/internal/walk"
)

func main() {
	// Honour Ctrl+C so a walk over a huge tree can be abandoned cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "tre: %v\n", err)
		var access *walk.AccessError
		if errors.As(err, &access) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
