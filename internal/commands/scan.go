package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ppiankov/aispectre/internal/analyzer"
	"github.com/ppiankov/aispectre/internal/anthropic"
	"github.com/ppiankov/aispectre/internal/azureopenai"
	"github.com/ppiankov/aispectre/internal/bedrock"
	"github.com/ppiankov/aispectre/internal/cohere"
	"github.com/ppiankov/aispectre/internal/config"
	"github.com/ppiankov/aispectre/internal/groq"
	"github.com/ppiankov/aispectre/internal/openai"
	"github.com/ppiankov/aispectre/internal/report"
	"github.com/ppiankov/aispectre/internal/vertexai"
)

var validPlatforms = map[string]bool{
	"openai": true, "anthropic": true, "bedrock": true,
	"azureopenai": true, "vertexai": true, "cohere": true, "groq": true,
}

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

Supported platforms: openai, anthropic, bedrock, azureopenai, vertexai, cohere, groq`,
		RunE: runScan,
	}

	cmd.Flags().StringVar(&scanFlags.platform, "platform", "", "platform to scan (openai, anthropic, bedrock, azureopenai, vertexai, cohere, groq)")
	cmd.Flags().BoolVar(&scanFlags.all, "all", false, "scan all configured platforms")
	cmd.Flags().IntVar(&scanFlags.window, "window", 30, "lookback window for usage data (days)")
	cmd.Flags().IntVar(&scanFlags.idleDays, "idle-days", 7, "days of inactivity before flagging as idle")
	cmd.Flags().Float64Var(&scanFlags.minWaste, "min-waste", 1.0, "minimum monthly waste to report ($)")
	cmd.Flags().StringVar(&scanFlags.format, "format", "text", "output format: text, json, sarif, spectrehub")
	cmd.Flags().StringVar(&scanFlags.token, "token", "", "API token (overrides config file and env)")

	return cmd
}

// scanResult holds normalized data from a single platform scan.
type scanResult struct {
	tokenUsage []analyzer.TokenUsage
	dailyCosts []analyzer.DailyCost
	apiKeys    []analyzer.APIKeyInfo
	models     []analyzer.ModelInfo
}

func runScan(cmd *cobra.Command, _ []string) error {
	// Validate flags.
	if scanFlags.platform == "" && !scanFlags.all {
		return fmt.Errorf("specify --platform or --all")
	}
	if scanFlags.platform != "" && scanFlags.all {
		return fmt.Errorf("--platform and --all are mutually exclusive")
	}
	if scanFlags.platform != "" && !validPlatforms[scanFlags.platform] {
		return fmt.Errorf("unknown platform: %q", scanFlags.platform)
	}

	// Validate reporter early.
	reporter, err := report.NewReporter(scanFlags.format)
	if err != nil {
		return err
	}

	// Load config.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply CLI flag overrides.
	if cmd.Flags().Changed("window") {
		cfg.Window = scanFlags.window
	}
	if cmd.Flags().Changed("idle-days") {
		cfg.IdleDays = scanFlags.idleDays
	}
	if cmd.Flags().Changed("min-waste") {
		cfg.MinWaste = scanFlags.minWaste
	}
	if cmd.Flags().Changed("format") {
		cfg.Format = scanFlags.format
	}

	// Determine target platforms.
	var platforms []string
	if scanFlags.all {
		platforms = cfg.EnabledPlatforms()
		if len(platforms) == 0 {
			return fmt.Errorf("no platforms configured; set env vars or create .aispectre.yaml")
		}
	} else {
		platforms = []string{scanFlags.platform}
		ensurePlatformConfig(cfg, scanFlags.platform, scanFlags.token)
	}

	// Scan each platform.
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -cfg.Window)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var combined scanResult
	platformLabel := ""
	for _, p := range platforms {
		result, scanErr := scanPlatform(ctx, cfg, p, start, now)
		if scanErr != nil {
			return fmt.Errorf("scan %s: %w", p, scanErr)
		}
		combined.tokenUsage = append(combined.tokenUsage, result.tokenUsage...)
		combined.dailyCosts = append(combined.dailyCosts, result.dailyCosts...)
		combined.apiKeys = append(combined.apiKeys, result.apiKeys...)
		combined.models = append(combined.models, result.models...)
		if platformLabel == "" {
			platformLabel = p
		} else {
			platformLabel = "multiple"
		}
	}

	// Analyze.
	input := analyzer.Input{
		TokenUsage: combined.tokenUsage,
		DailyCosts: combined.dailyCosts,
		APIKeys:    combined.apiKeys,
		Models:     combined.models,
	}
	analyzerCfg := analyzer.Config{
		IdleDays: cfg.IdleDays,
		MinWaste: cfg.MinWaste,
		Now:      now,
	}
	findings := analyzer.Analyze(input, analyzerCfg)

	// Report.
	r := report.Report{
		Findings:    findings,
		Platform:    platformLabel,
		Window:      cfg.Window,
		GeneratedAt: now,
	}
	return reporter.Render(cmd.OutOrStdout(), r)
}

// ensurePlatformConfig creates a minimal config for the target platform
// if it doesn't exist yet, applying the --token override.
func ensurePlatformConfig(cfg *config.Config, platform, token string) {
	switch platform {
	case "openai":
		if cfg.Platforms.OpenAI == nil {
			cfg.Platforms.OpenAI = &config.OpenAIConfig{Enabled: true}
		}
		if token != "" {
			cfg.Platforms.OpenAI.Token = token
		}
	case "anthropic":
		if cfg.Platforms.Anthropic == nil {
			cfg.Platforms.Anthropic = &config.AnthropicConfig{Enabled: true}
		}
		if token != "" {
			cfg.Platforms.Anthropic.Token = token
		}
	case "bedrock":
		if cfg.Platforms.Bedrock == nil {
			cfg.Platforms.Bedrock = &config.BedrockConfig{Enabled: true}
		}
	case "azureopenai":
		if cfg.Platforms.AzureOpenAI == nil {
			cfg.Platforms.AzureOpenAI = &config.AzureOpenAIConfig{Enabled: true}
		}
	case "vertexai":
		if cfg.Platforms.VertexAI == nil {
			cfg.Platforms.VertexAI = &config.VertexAIConfig{Enabled: true}
		}
	case "cohere":
		if cfg.Platforms.Cohere == nil {
			cfg.Platforms.Cohere = &config.CohereConfig{Enabled: true}
		}
		if token != "" {
			cfg.Platforms.Cohere.Token = token
		}
	case "groq":
		if cfg.Platforms.Groq == nil {
			cfg.Platforms.Groq = &config.GroqConfig{Enabled: true}
		}
		if token != "" {
			cfg.Platforms.Groq.Token = token
		}
	}
}

func scanPlatform(ctx context.Context, cfg *config.Config, platform string, start, end time.Time) (scanResult, error) {
	switch platform {
	case "openai":
		return scanOpenAI(ctx, cfg.Platforms.OpenAI, start, end)
	case "anthropic":
		return scanAnthropic(ctx, cfg.Platforms.Anthropic, start, end)
	case "bedrock":
		return scanBedrock(ctx, cfg.Platforms.Bedrock, start, end)
	case "azureopenai":
		return scanAzureOpenAI(ctx, cfg.Platforms.AzureOpenAI, start, end)
	case "vertexai":
		return scanVertexAI(ctx, cfg.Platforms.VertexAI, start, end)
	case "cohere":
		return scanCohere(ctx, cfg.Platforms.Cohere, start, end)
	case "groq":
		return scanGroq(ctx, cfg.Platforms.Groq, start, end)
	default:
		return scanResult{}, fmt.Errorf("unknown platform: %s", platform)
	}
}

func scanOpenAI(_ context.Context, pcfg *config.OpenAIConfig, start, end time.Time) (scanResult, error) {
	if pcfg == nil {
		return scanResult{}, fmt.Errorf("openai not configured")
	}
	client, err := openai.NewClient(openai.Config{
		Token:   pcfg.Token,
		BaseURL: pcfg.BaseURL,
	})
	if err != nil {
		return scanResult{}, err
	}
	ctx := context.Background()

	var result scanResult

	usage, err := client.FetchCompletionUsage(ctx, start, end)
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch usage: %w", err)
	}
	result.tokenUsage = normalizeOpenAIUsage(usage)

	costs, err := client.FetchCosts(ctx, start, end)
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch costs: %w", err)
	}
	result.dailyCosts = normalizeOpenAICosts(costs)

	keys, err := client.FetchAPIKeys(ctx)
	if err != nil {
		// API keys endpoint may not be available for all orgs — warn, don't fail.
		_ = err
	} else {
		result.apiKeys = normalizeOpenAIKeys(keys)
	}

	models, err := client.FetchModels(ctx)
	if err != nil {
		_ = err
	} else {
		result.models = normalizeOpenAIModels(models)
	}

	return result, nil
}

func scanAnthropic(_ context.Context, pcfg *config.AnthropicConfig, start, end time.Time) (scanResult, error) {
	if pcfg == nil {
		return scanResult{}, fmt.Errorf("anthropic not configured")
	}
	client, err := anthropic.NewClient(anthropic.Config{
		Token:   pcfg.Token,
		BaseURL: pcfg.BaseURL,
	})
	if err != nil {
		return scanResult{}, err
	}

	usage, err := client.FetchUsage(context.Background(), start, end)
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch usage: %w", err)
	}

	return scanResult{tokenUsage: normalizeAnthropicUsage(usage)}, nil
}

func scanBedrock(ctx context.Context, pcfg *config.BedrockConfig, start, end time.Time) (scanResult, error) {
	if pcfg == nil {
		return scanResult{}, fmt.Errorf("bedrock not configured")
	}
	client, err := bedrock.NewClient(ctx, bedrock.Config{
		Region:  pcfg.Region,
		Profile: pcfg.Profile,
	})
	if err != nil {
		return scanResult{}, err
	}

	usage, err := client.FetchUsage(ctx, start, end)
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch usage: %w", err)
	}

	return scanResult{tokenUsage: normalizeBedrockUsage(usage)}, nil
}

func scanAzureOpenAI(_ context.Context, pcfg *config.AzureOpenAIConfig, start, end time.Time) (scanResult, error) {
	if pcfg == nil {
		return scanResult{}, fmt.Errorf("azureopenai not configured")
	}
	client, err := azureopenai.NewClient(azureopenai.Config{
		SubscriptionID: pcfg.SubscriptionID,
		ResourceGroup:  pcfg.ResourceGroup,
		AccountName:    pcfg.AccountName,
	})
	if err != nil {
		return scanResult{}, err
	}

	usage, err := client.FetchUsage(context.Background(), start, end)
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch usage: %w", err)
	}

	return scanResult{tokenUsage: normalizeAzureUsage(usage)}, nil
}

func scanVertexAI(ctx context.Context, pcfg *config.VertexAIConfig, start, end time.Time) (scanResult, error) {
	if pcfg == nil {
		return scanResult{}, fmt.Errorf("vertexai not configured")
	}
	client, err := vertexai.NewClient(ctx, vertexai.Config{
		ProjectID: pcfg.Project,
		Region:    pcfg.Region,
	})
	if err != nil {
		return scanResult{}, err
	}

	usage, err := client.FetchUsage(ctx, start, end)
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch usage: %w", err)
	}

	return scanResult{tokenUsage: normalizeVertexAIUsage(usage)}, nil
}

func scanCohere(_ context.Context, pcfg *config.CohereConfig, start, end time.Time) (scanResult, error) {
	if pcfg == nil {
		return scanResult{}, fmt.Errorf("cohere not configured")
	}
	client, err := cohere.NewClient(cohere.Config{})
	if err != nil {
		return scanResult{}, err
	}

	_, err = client.FetchUsage(context.Background(), start, end)
	if err != nil && errors.Is(err, cohere.ErrUnsupported) {
		// Cohere has no usage API — not an error.
		return scanResult{}, nil
	}
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch usage: %w", err)
	}

	return scanResult{}, nil
}

func scanGroq(_ context.Context, pcfg *config.GroqConfig, start, end time.Time) (scanResult, error) {
	if pcfg == nil {
		return scanResult{}, fmt.Errorf("groq not configured")
	}
	client, err := groq.NewClient(groq.Config{})
	if err != nil {
		return scanResult{}, err
	}

	_, err = client.FetchUsage(context.Background(), start, end)
	if err != nil && errors.Is(err, groq.ErrUnsupported) {
		return scanResult{}, nil
	}
	if err != nil {
		return scanResult{}, fmt.Errorf("fetch usage: %w", err)
	}

	return scanResult{}, nil
}

// --- Normalization functions ---

func normalizeOpenAIUsage(buckets []openai.UsageBucket) []analyzer.TokenUsage {
	var out []analyzer.TokenUsage
	for _, b := range buckets {
		for _, r := range b.Results {
			model := "unknown"
			if r.Model != nil {
				model = *r.Model
			}
			out = append(out, analyzer.TokenUsage{
				StartTime:    time.Unix(b.StartTime, 0),
				EndTime:      time.Unix(b.EndTime, 0),
				Platform:     "openai",
				Model:        model,
				InputTokens:  r.InputTokens,
				OutputTokens: r.OutputTokens,
				CachedTokens: r.InputCachedTokens,
				Requests:     r.NumModelRequests,
			})
		}
	}
	return out
}

func normalizeOpenAICosts(buckets []openai.CostBucket) []analyzer.DailyCost {
	var out []analyzer.DailyCost
	for _, b := range buckets {
		for _, r := range b.Results {
			out = append(out, analyzer.DailyCost{
				Date:     time.Unix(b.StartTime, 0),
				Amount:   r.Amount.Value,
				Currency: r.Amount.Currency,
			})
		}
	}
	return out
}

func normalizeOpenAIKeys(keys []openai.APIKey) []analyzer.APIKeyInfo {
	out := make([]analyzer.APIKeyInfo, 0, len(keys))
	for _, k := range keys {
		info := analyzer.APIKeyInfo{
			Platform:  "openai",
			ID:        k.ID,
			Name:      k.Name,
			CreatedAt: time.Unix(k.CreatedAt, 0),
		}
		if k.LastUsedAt != nil {
			t := time.Unix(*k.LastUsedAt, 0)
			info.LastUsedAt = &t
		}
		out = append(out, info)
	}
	return out
}

func normalizeOpenAIModels(models []openai.Model) []analyzer.ModelInfo {
	out := make([]analyzer.ModelInfo, 0, len(models))
	for _, m := range models {
		out = append(out, analyzer.ModelInfo{
			Platform: "openai",
			ID:       m.ID,
			OwnedBy:  m.OwnedBy,
			Created:  time.Unix(m.Created, 0),
		})
	}
	return out
}

func normalizeAnthropicUsage(buckets []anthropic.UsageBucket) []analyzer.TokenUsage {
	var out []analyzer.TokenUsage
	for _, b := range buckets {
		startTime, _ := time.Parse(time.RFC3339, b.BucketStartTime)
		endTime, _ := time.Parse(time.RFC3339, b.BucketEndTime)
		for _, e := range b.Usage {
			out = append(out, analyzer.TokenUsage{
				StartTime:    startTime,
				EndTime:      endTime,
				Platform:     "anthropic",
				Model:        e.Model,
				InputTokens:  e.InputTokens,
				OutputTokens: e.OutputTokens,
				CachedTokens: e.CacheReadInputTokens,
				Requests:     e.Requests,
			})
		}
	}
	return out
}

func normalizeBedrockUsage(buckets []bedrock.UsageBucket) []analyzer.TokenUsage {
	out := make([]analyzer.TokenUsage, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, analyzer.TokenUsage{
			StartTime:    b.StartTime,
			EndTime:      b.EndTime,
			Platform:     "bedrock",
			Model:        b.ModelID,
			InputTokens:  b.InputTokens,
			OutputTokens: b.OutputTokens,
			Requests:     b.Invocations,
		})
	}
	return out
}

func normalizeAzureUsage(buckets []azureopenai.UsageBucket) []analyzer.TokenUsage {
	out := make([]analyzer.TokenUsage, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, analyzer.TokenUsage{
			StartTime:    b.StartTime,
			EndTime:      b.EndTime,
			Platform:     "azureopenai",
			Model:        b.Model,
			InputTokens:  b.InputTokens,
			OutputTokens: b.OutputTokens,
			Requests:     b.Requests,
		})
	}
	return out
}

func normalizeVertexAIUsage(buckets []vertexai.UsageBucket) []analyzer.TokenUsage {
	out := make([]analyzer.TokenUsage, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, analyzer.TokenUsage{
			StartTime:    b.StartTime,
			EndTime:      b.EndTime,
			Platform:     "vertexai",
			Model:        b.Model,
			InputTokens:  b.InputTokens,
			OutputTokens: b.OutputTokens,
			Requests:     b.Requests,
		})
	}
	return out
}
