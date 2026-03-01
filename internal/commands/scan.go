package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var scanFlags struct {
	platform string
	all      bool
	window   int
	idleDays int
	minWaste float64
	format   string
	token    string
}

func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan AI/LLM platforms for spend waste",
		Long: `Scan AI/LLM platform usage data to find idle API keys, underused models,
and wasted compute. Reports estimated monthly waste for each finding.

Supported platforms: openai, anthropic, bedrock, azureopenai, vertexai, cohere`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "scan: not yet implemented (see WO-9)")
			return nil
		},
	}

	cmd.Flags().StringVar(&scanFlags.platform, "platform", "", "platform to scan (openai, anthropic, bedrock, azureopenai, vertexai, cohere)")
	cmd.Flags().BoolVar(&scanFlags.all, "all", false, "scan all configured platforms")
	cmd.Flags().IntVar(&scanFlags.window, "window", 30, "lookback window for usage data (days)")
	cmd.Flags().IntVar(&scanFlags.idleDays, "idle-days", 7, "days of inactivity before flagging as idle")
	cmd.Flags().Float64Var(&scanFlags.minWaste, "min-waste", 1.0, "minimum monthly waste to report ($)")
	cmd.Flags().StringVar(&scanFlags.format, "format", "text", "output format: text, json, sarif, spectrehub")
	cmd.Flags().StringVar(&scanFlags.token, "token", "", "API token (overrides config file and env)")

	return cmd
}
