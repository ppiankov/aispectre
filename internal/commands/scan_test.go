package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/aispectre/internal/anthropic"
	"github.com/ppiankov/aispectre/internal/bedrock"
	"github.com/ppiankov/aispectre/internal/openai"
)

func TestScanHelp(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"scan", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	for _, flag := range []string{"--platform", "--all", "--window", "--idle-days", "--min-waste", "--format", "--token"} {
		if !strings.Contains(help, flag) {
			t.Errorf("scan help missing flag %q", flag)
		}
	}
}

func TestScanDefaultFlags(t *testing.T) {
	cmd := newScanCmd()
	flags := cmd.Flags()

	window, _ := flags.GetInt("window")
	if window != 30 {
		t.Errorf("window default = %d, want 30", window)
	}

	idleDays, _ := flags.GetInt("idle-days")
	if idleDays != 7 {
		t.Errorf("idle-days default = %d, want 7", idleDays)
	}

	minWaste, _ := flags.GetFloat64("min-waste")
	if minWaste != 1.0 {
		t.Errorf("min-waste default = %f, want 1.0", minWaste)
	}

	format, _ := flags.GetString("format")
	if format != "text" {
		t.Errorf("format default = %q, want text", format)
	}
}

func TestScanRequiresPlatform(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"scan"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --platform or --all")
	}
	if !strings.Contains(err.Error(), "specify --platform or --all") {
		t.Errorf("error = %q, want mention of --platform or --all", err.Error())
	}
}

func TestScanMutuallyExclusive(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"scan", "--platform", "openai", "--all"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error with both --platform and --all")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want 'mutually exclusive'", err.Error())
	}
}

func TestScanInvalidPlatform(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"scan", "--platform", "unknown"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
	if !strings.Contains(err.Error(), "unknown platform") {
		t.Errorf("error = %q, want 'unknown platform'", err.Error())
	}
}

func TestScanInvalidFormat(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"scan", "--platform", "openai", "--format", "xml"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "unknown output format") {
		t.Errorf("error = %q, want 'unknown output format'", err.Error())
	}
}

// --- Normalization tests ---

func TestNormalizeOpenAIUsage(t *testing.T) {
	model := "gpt-4"
	buckets := []openai.UsageBucket{
		{
			StartTime: 1700000000,
			EndTime:   1700003600,
			Results: []openai.UsageResult{
				{
					Model:             &model,
					InputTokens:       1000,
					OutputTokens:      500,
					InputCachedTokens: 200,
					NumModelRequests:  10,
				},
			},
		},
	}

	got := normalizeOpenAIUsage(buckets)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	u := got[0]
	if u.Platform != "openai" {
		t.Errorf("Platform = %s, want openai", u.Platform)
	}
	if u.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", u.Model)
	}
	if u.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", u.InputTokens)
	}
	if u.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", u.OutputTokens)
	}
	if u.CachedTokens != 200 {
		t.Errorf("CachedTokens = %d, want 200", u.CachedTokens)
	}
	if u.Requests != 10 {
		t.Errorf("Requests = %d, want 10", u.Requests)
	}
	if u.StartTime != time.Unix(1700000000, 0) {
		t.Errorf("StartTime = %v, want Unix(1700000000)", u.StartTime)
	}
}

func TestNormalizeOpenAIUsageNilModel(t *testing.T) {
	buckets := []openai.UsageBucket{
		{
			StartTime: 1700000000,
			EndTime:   1700003600,
			Results: []openai.UsageResult{
				{InputTokens: 100},
			},
		},
	}
	got := normalizeOpenAIUsage(buckets)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Model != "unknown" {
		t.Errorf("Model = %s, want unknown", got[0].Model)
	}
}

func TestNormalizeOpenAICosts(t *testing.T) {
	buckets := []openai.CostBucket{
		{
			StartTime: 1700000000,
			EndTime:   1700086400,
			Results: []openai.CostResult{
				{Amount: openai.CostAmount{Value: 12.50, Currency: "usd"}},
				{Amount: openai.CostAmount{Value: 3.25, Currency: "usd"}},
			},
		},
	}

	got := normalizeOpenAICosts(buckets)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Amount != 12.50 {
		t.Errorf("Amount[0] = %f, want 12.50", got[0].Amount)
	}
	if got[1].Amount != 3.25 {
		t.Errorf("Amount[1] = %f, want 3.25", got[1].Amount)
	}
	if got[0].Currency != "usd" {
		t.Errorf("Currency = %s, want usd", got[0].Currency)
	}
	if got[0].Date != time.Unix(1700000000, 0) {
		t.Errorf("Date = %v, want Unix(1700000000)", got[0].Date)
	}
}

func TestNormalizeOpenAIKeys(t *testing.T) {
	lastUsed := int64(1700000000)
	keys := []openai.APIKey{
		{
			ID:         "key-1",
			Name:       "prod-key",
			CreatedAt:  1699000000,
			LastUsedAt: &lastUsed,
		},
		{
			ID:        "key-2",
			Name:      "unused-key",
			CreatedAt: 1698000000,
		},
	}

	got := normalizeOpenAIKeys(keys)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Platform != "openai" {
		t.Errorf("Platform = %s, want openai", got[0].Platform)
	}
	if got[0].ID != "key-1" {
		t.Errorf("ID = %s, want key-1", got[0].ID)
	}
	if got[0].LastUsedAt == nil {
		t.Fatal("LastUsedAt should not be nil")
	}
	if *got[0].LastUsedAt != time.Unix(1700000000, 0) {
		t.Errorf("LastUsedAt = %v, want Unix(1700000000)", *got[0].LastUsedAt)
	}
	if got[1].LastUsedAt != nil {
		t.Errorf("LastUsedAt should be nil for unused key")
	}
}

func TestNormalizeOpenAIModels(t *testing.T) {
	models := []openai.Model{
		{ID: "gpt-4", OwnedBy: "openai", Created: 1690000000},
	}

	got := normalizeOpenAIModels(models)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Platform != "openai" {
		t.Errorf("Platform = %s, want openai", got[0].Platform)
	}
	if got[0].ID != "gpt-4" {
		t.Errorf("ID = %s, want gpt-4", got[0].ID)
	}
	if got[0].OwnedBy != "openai" {
		t.Errorf("OwnedBy = %s, want openai", got[0].OwnedBy)
	}
}

func TestNormalizeAnthropicUsage(t *testing.T) {
	buckets := []anthropic.UsageBucket{
		{
			BucketStartTime: "2024-01-15T00:00:00Z",
			BucketEndTime:   "2024-01-16T00:00:00Z",
			Usage: []anthropic.UsageEntry{
				{
					Model:                "claude-3-opus-20240229",
					InputTokens:          5000,
					OutputTokens:         2000,
					CacheReadInputTokens: 1000,
					Requests:             50,
				},
			},
		},
	}

	got := normalizeAnthropicUsage(buckets)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	u := got[0]
	if u.Platform != "anthropic" {
		t.Errorf("Platform = %s, want anthropic", u.Platform)
	}
	if u.Model != "claude-3-opus-20240229" {
		t.Errorf("Model = %s, want claude-3-opus-20240229", u.Model)
	}
	if u.InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000", u.InputTokens)
	}
	if u.CachedTokens != 1000 {
		t.Errorf("CachedTokens = %d, want 1000", u.CachedTokens)
	}
	if u.Requests != 50 {
		t.Errorf("Requests = %d, want 50", u.Requests)
	}
	wantStart, _ := time.Parse(time.RFC3339, "2024-01-15T00:00:00Z")
	if u.StartTime != wantStart {
		t.Errorf("StartTime = %v, want %v", u.StartTime, wantStart)
	}
}

func TestNormalizeBedrockUsage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	buckets := []bedrock.UsageBucket{
		{
			StartTime:    now.Add(-24 * time.Hour),
			EndTime:      now,
			ModelID:      "anthropic.claude-3-sonnet-20240229-v1:0",
			InputTokens:  3000,
			OutputTokens: 1500,
			Invocations:  25,
		},
	}

	got := normalizeBedrockUsage(buckets)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	u := got[0]
	if u.Platform != "bedrock" {
		t.Errorf("Platform = %s, want bedrock", u.Platform)
	}
	if u.Model != "anthropic.claude-3-sonnet-20240229-v1:0" {
		t.Errorf("Model = %s, want anthropic.claude-3-sonnet-20240229-v1:0", u.Model)
	}
	if u.InputTokens != 3000 {
		t.Errorf("InputTokens = %d, want 3000", u.InputTokens)
	}
	if u.Requests != 25 {
		t.Errorf("Requests = %d, want 25", u.Requests)
	}
}

func TestNormalizeEmptySlices(t *testing.T) {
	if got := normalizeOpenAIUsage(nil); len(got) != 0 {
		t.Errorf("normalizeOpenAIUsage(nil) = %d items, want 0", len(got))
	}
	if got := normalizeOpenAICosts(nil); len(got) != 0 {
		t.Errorf("normalizeOpenAICosts(nil) = %d items, want 0", len(got))
	}
	if got := normalizeAnthropicUsage(nil); len(got) != 0 {
		t.Errorf("normalizeAnthropicUsage(nil) = %d items, want 0", len(got))
	}
	if got := normalizeBedrockUsage(nil); len(got) != 0 {
		t.Errorf("normalizeBedrockUsage(nil) = %d items, want 0", len(got))
	}
}
