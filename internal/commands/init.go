package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create starter .aispectre.yaml in the current directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			wrote := 0
			for _, f := range initFiles {
				path := filepath.Join(cwd, f.name)
				if _, err := os.Stat(path); err == nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s already exists\n", f.name)
					continue
				}
				if err := os.WriteFile(path, []byte(f.content), 0o600); err != nil {
					return fmt.Errorf("write %s: %w", f.name, err)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", f.name)
				wrote++
			}

			if wrote == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Nothing to do — all config files already exist.")
			}
			return nil
		},
	}
	return cmd
}

type initFile struct {
	name    string
	content string
}

var initFiles = []initFile{
	{
		name:    ".aispectre.yaml",
		content: sampleConfig,
	},
}
