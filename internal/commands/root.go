package commands

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var verbose bool

// BuildInfo holds version and build metadata.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"goVersion"`
}

func newRootCmd(info BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:   "aispectre",
		Short: "AI/LLM spend waste auditor",
		Long: `aispectre audits AI and LLM platform spending to find idle API keys,
underused models, and wasted compute across OpenAI, Anthropic, AWS Bedrock,
Azure OpenAI, Vertex AI, and Cohere.

Each finding includes estimated waste and remediation guidance.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	root.AddCommand(newVersionCmd(info))
	root.AddCommand(newScanCmd())
	root.AddCommand(newInitCmd())

	return root
}

func newVersionCmd(info BuildInfo) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				_ = enc.Encode(info)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "aispectre %s (commit: %s, built: %s, go: %s)\n",
					info.Version, info.Commit, info.Date, info.GoVersion)
			}
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output version as JSON")

	return cmd
}

// ExitError signals a non-zero exit code without calling os.Exit directly.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// Execute runs the root command with injected build info.
func Execute(v, commit, date string) error {
	info := BuildInfo{
		Version:   v,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
	}
	return newRootCmd(info).Execute()
}
