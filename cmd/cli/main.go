package main

import (
	"context"
	"fmt"
	"os"
)

var (
	version     = "dev"
	commit      = "none"
	date        = "unknown"
	environment = "local"
)

func main() {
	ctx := context.Background()

	metadata := BuildMetadata{
		Version:     version,
		Commit:      commit,
		Date:        date,
		Release:     fmt.Sprintf("ship@%s:%s", version, commit),
		Environment: environment,
	}

	if err := run(ctx, os.Args, os.Getenv, os.Stdout, metadata); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
