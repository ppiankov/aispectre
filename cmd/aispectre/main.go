package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ppiankov/aispectre/internal/commands"
)

// Set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := commands.Execute(version, commit, date); err != nil {
		var exitErr *commands.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
