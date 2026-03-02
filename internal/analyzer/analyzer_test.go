package analyzer

import (
	"math"
	"testing"
	"time"
)

// --- Model DB tests ---

func TestLookupModelExact(t *testing.T) {
	p, ok := LookupModel("gpt-4o")
	if !ok {
		t.Fatal("expected to find gpt-4o")
	}
	if p.Tier != TierMedium {
		t.Errorf("Tier = %d, want %d", p.Tier, TierMedium)
	}
	if p.DowngradeTo != "gpt-4o-mini" {
		t.Errorf("DowngradeTo = %s, want gpt-4o-mini", p.DowngradeTo)
	}
}

func TestLookupModelPrefix(t *testing.T) {
	p, ok := LookupModel("anthropic.claude-3-sonnet-20240229-v1:0")
	if !ok {
		t.Fatal("expected prefix match for Bedrock model ID")
	}
	if p.Name != "anthropic.claude-3-sonnet" {
		t.Errorf("Name = %s, want anthropic.claude-3-sonnet", p.Name)
	}
}

func TestLookupModelUnknown(t *testing.T) {
	_, ok := LookupModel("some-unknown-model-xyz")
	if ok {
		t.Error("expected false for unknown model")
	}
}

func TestTokenCost(t *testing.T) {
	// gpt-4: $30/M input, $60/M output
	cost := TokenCost("gpt-4", 1_000_000, 1_000_000)
	want := 90.0 // 30 + 60
	if math.Abs(cost-want) > 0.01 {
		t.Errorf("TokenCost = %f, want %f", cost, want)
	}
}

func TestTokenCostUnknown(t *testing.T) {
	cost := TokenCost("unknown-model", 1_000_000, 1_000_000)
	if cost != 0 {
		t.Errorf("TokenCost for unknown = %f, want 0", cost)
	}
}

// --- Cost aggregation tests ---

func TestDailySpend(t *testing.T) {
	day1 := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)
	usage := []TokenUsage{
		{StartTime: day1, Model: "gpt-4", InputTokens: 1_000_000, OutputTokens: 500_000, Requests: 10},
		{StartTime: day1, Model: "gpt-4", InputTokens: 1_000_000, OutputTokens: 500_000, Requests: 10},
		{StartTime: day2, Model: "gpt-4", InputTokens: 500_000, OutputTokens: 250_000, Requests: 5},
	}
	days := DailySpend(usage)
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2", len(days))
	}
	// Day1: 2 * (1M*30/1M + 500K*60/1M) = 2 * (30 + 30) = 120
	if math.Abs(days[0].Amount-120.0) > 0.01 {
		t.Errorf("day1 amount = %f, want 120", days[0].Amount)
	}
}

func TestDailySpendFromCosts(t *testing.T) {
	day1 := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	costs := []DailyCost{
		{Date: day1, Amount: 50.0, Currency: "USD"},
		{Date: day1, Amount: 30.0, Currency: "USD"},
	}
	days := DailySpendFromCosts(costs)
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	if math.Abs(days[0].Amount-80.0) > 0.01 {
		t.Errorf("amount = %f, want 80", days[0].Amount)
	}
}

func TestModelSpend(t *testing.T) {
	usage := []TokenUsage{
		{Model: "gpt-4", InputTokens: 1_000_000, OutputTokens: 500_000, Requests: 10},
		{Model: "gpt-4o-mini", InputTokens: 2_000_000, OutputTokens: 1_000_000, Requests: 100},
	}
	ms := ModelSpend(usage)
	if len(ms) != 2 {
		t.Fatalf("got %d models, want 2", len(ms))
	}
	// Sorted by cost descending: gpt-4 should be first (more expensive).
	if ms[0].Model != "gpt-4" {
		t.Errorf("first model = %s, want gpt-4 (most expensive)", ms[0].Model)
	}
}

func TestMonthlyProjection(t *testing.T) {
	// $100 over 10 days → $300/month
	got := MonthlyProjection(100, 10)
	if math.Abs(got-300) > 0.01 {
		t.Errorf("MonthlyProjection = %f, want 300", got)
	}
}

func TestMonthlyProjectionZeroDays(t *testing.T) {
	got := MonthlyProjection(100, 0)
	if got != 0 {
		t.Errorf("MonthlyProjection(0 days) = %f, want 0", got)
	}
}

// --- Finding tests ---

func TestModelOverkill(t *testing.T) {
	usage := makeUsage("gpt-4", 100, 200, 50, 100) // avg input=2, avg output=1 (very small)
	findings := checkModelOverkill(usage)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindModelOverkill {
		t.Errorf("Kind = %s, want MODEL_OVERKILL", findings[0].Kind)
	}
}

func TestModelOverkillHighUsage(t *testing.T) {
	// gpt-4 with high avg tokens → not overkill
	usage := makeUsage("gpt-4", 100_000, 200_000, 100, 100)
	findings := checkModelOverkill(usage)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (high usage is not overkill)", len(findings))
	}
}

func TestModelOverkillTier1(t *testing.T) {
	// gpt-4o-mini (tier 1) with low tokens → no finding (already cheap)
	usage := makeUsage("gpt-4o-mini", 100, 50, 25, 100)
	findings := checkModelOverkill(usage)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (tier 1 model)", len(findings))
	}
}

func TestNoCaching(t *testing.T) {
	// gpt-4o supports caching, 2000 requests, 0 cached tokens
	usage := []TokenUsage{
		{
			Platform:     "openai",
			Model:        "gpt-4o",
			InputTokens:  2_000_000,
			CachedTokens: 0,
			Requests:     2000,
			StartTime:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			EndTime:      time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
		},
	}
	findings := checkNoCaching(usage)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindNoCaching {
		t.Errorf("Kind = %s, want NO_CACHING", findings[0].Kind)
	}
}

func TestNoCachingHighRate(t *testing.T) {
	usage := []TokenUsage{
		{
			Platform:     "openai",
			Model:        "gpt-4o",
			InputTokens:  1_000_000,
			CachedTokens: 200_000, // 20% hit rate — above threshold
			Requests:     2000,
			StartTime:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			EndTime:      time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
		},
	}
	findings := checkNoCaching(usage)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (cache rate above threshold)", len(findings))
	}
}

func TestKeyUnused(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	lastUsed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // 31 days ago
	keys := []APIKeyInfo{
		{Platform: "openai", ID: "key-1", Name: "old-key", LastUsedAt: &lastUsed},
	}
	cfg := Config{IdleDays: 7, Now: now}
	findings := checkKeyUnused(keys, cfg)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindKeyUnused {
		t.Errorf("Kind = %s, want KEY_UNUSED", findings[0].Kind)
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("Severity = %v, want high", findings[0].Severity)
	}
}

func TestKeyUnusedRecent(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	lastUsed := time.Date(2025, 1, 28, 0, 0, 0, 0, time.UTC) // 4 days ago
	keys := []APIKeyInfo{
		{Platform: "openai", ID: "key-1", Name: "active-key", LastUsedAt: &lastUsed},
	}
	cfg := Config{IdleDays: 7, Now: now}
	findings := checkKeyUnused(keys, cfg)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (recently used)", len(findings))
	}
}

func TestKeyUnusedNeverUsed(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	keys := []APIKeyInfo{
		{Platform: "openai", ID: "key-2", Name: "never-used-key", LastUsedAt: nil},
	}
	cfg := Config{IdleDays: 7, Now: now}
	findings := checkKeyUnused(keys, cfg)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
}

func TestFinetunedIdle(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	models := []ModelInfo{
		{Platform: "openai", ID: "ft:gpt-4o-mini:my-org:suffix:abc123", OwnedBy: "user-abc", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	cfg := Config{IdleDays: 7, Now: now}
	findings := checkFinetunedIdle(models, nil, cfg) // no usage
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindFinetunedIdle {
		t.Errorf("Kind = %s, want FINETUNED_IDLE", findings[0].Kind)
	}
}

func TestFinetunedIdleWithUsage(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	modelID := "ft:gpt-4o-mini:my-org:suffix:abc123"
	models := []ModelInfo{
		{Platform: "openai", ID: modelID, OwnedBy: "user-abc", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	usage := []TokenUsage{
		{Model: modelID, Requests: 10},
	}
	cfg := Config{IdleDays: 7, Now: now}
	findings := checkFinetunedIdle(models, usage, cfg)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (model has usage)", len(findings))
	}
}

func TestCostSpike(t *testing.T) {
	// 8 days: first 7 days ~$10/day, day 8 = $50 (5x spike)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var costs []DailyCost
	for i := 0; i < 7; i++ {
		costs = append(costs, DailyCost{
			Date:   base.AddDate(0, 0, i),
			Amount: 10.0,
		})
	}
	costs = append(costs, DailyCost{
		Date:   base.AddDate(0, 0, 7),
		Amount: 50.0,
	})
	findings := checkCostSpike(nil, costs)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindCostSpike {
		t.Errorf("Kind = %s, want COST_SPIKE", findings[0].Kind)
	}
}

func TestCostSpikeNoSpike(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var costs []DailyCost
	for i := 0; i < 10; i++ {
		costs = append(costs, DailyCost{
			Date:   base.AddDate(0, 0, i),
			Amount: 10.0,
		})
	}
	findings := checkCostSpike(nil, costs)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (no spike)", len(findings))
	}
}

func TestBatchEligible(t *testing.T) {
	// gpt-4o supports batch, 200 req/day over 5 days
	base := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	var usage []TokenUsage
	for i := 0; i < 5; i++ {
		day := base.AddDate(0, 0, i)
		usage = append(usage, TokenUsage{
			Platform:     "openai",
			Model:        "gpt-4o",
			InputTokens:  100_000,
			OutputTokens: 50_000,
			Requests:     200,
			StartTime:    day,
			EndTime:      day.Add(24 * time.Hour),
		})
	}
	findings := checkBatchEligible(usage)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindBatchEligible {
		t.Errorf("Kind = %s, want BATCH_ELIGIBLE", findings[0].Kind)
	}
}

func TestTokenInefficiency(t *testing.T) {
	// output/input ratio > 10x
	usage := makeUsage("gpt-4o", 10_000, 200_000, 0, 100) // ratio = 20x
	findings := checkTokenInefficiency(usage)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindTokenInefficiency {
		t.Errorf("Kind = %s, want TOKEN_INEFFICIENCY", findings[0].Kind)
	}
}

func TestTokenInefficiencyNormal(t *testing.T) {
	// output/input ratio ~2x — normal
	usage := makeUsage("gpt-4o", 100_000, 200_000, 0, 100)
	findings := checkTokenInefficiency(usage)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (normal ratio)", len(findings))
	}
}

func TestDowngradeAvailable(t *testing.T) {
	usage := makeUsage("gpt-4", 1_000_000, 500_000, 0, 100)
	findings := checkDowngradeAvailable(usage)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Kind != KindDowngradeAvailable {
		t.Errorf("Kind = %s, want DOWNGRADE_AVAILABLE", findings[0].Kind)
	}
	if findings[0].Evidence["downgrade_to"] != "gpt-4o" {
		t.Errorf("downgrade_to = %v, want gpt-4o", findings[0].Evidence["downgrade_to"])
	}
}

func TestDowngradeAvailableNoAlternative(t *testing.T) {
	// gpt-3.5-turbo has no downgrade path
	usage := makeUsage("gpt-3.5-turbo", 1_000_000, 500_000, 0, 100)
	findings := checkDowngradeAvailable(usage)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 (no downgrade)", len(findings))
	}
}

// --- Integration tests ---

func TestAnalyzeEmpty(t *testing.T) {
	findings := Analyze(Input{}, Config{IdleDays: 7, MinWaste: 1.0, Now: time.Now()})
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0 for empty input", len(findings))
	}
}

func TestAnalyzeMinWasteFilter(t *testing.T) {
	// Create usage that generates findings with tiny waste
	usage := makeUsage("gpt-4o", 100, 50, 0, 100) // small amounts
	findings := Analyze(Input{TokenUsage: usage}, Config{
		IdleDays: 7,
		MinWaste: 1000.0, // very high threshold
		Now:      time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	// All non-security findings should be filtered out
	for _, f := range findings {
		if !isSecurityFinding(f.Kind) {
			t.Errorf("non-security finding %s with waste $%.2f passed filter", f.Kind, f.MonthlyWaste)
		}
	}
}

func TestAnalyzeSecurityBypassesMinWaste(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	input := Input{
		APIKeys: []APIKeyInfo{
			{Platform: "openai", ID: "key-1", Name: "stale", LastUsedAt: nil},
		},
	}
	findings := Analyze(input, Config{
		IdleDays: 7,
		MinWaste: 1000.0, // high threshold
		Now:      now,
	})
	found := false
	for _, f := range findings {
		if f.Kind == KindKeyUnused {
			found = true
		}
	}
	if !found {
		t.Error("KEY_UNUSED finding should bypass MinWaste filter")
	}
}

func TestAnalyzeSortOrder(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	input := Input{
		TokenUsage: makeUsage("gpt-4", 100, 200, 50, 100), // will generate MODEL_OVERKILL (medium) + DOWNGRADE_AVAILABLE (medium)
		APIKeys: []APIKeyInfo{
			{Platform: "openai", ID: "key-1", Name: "stale", LastUsedAt: nil}, // KEY_UNUSED (high)
		},
	}
	findings := Analyze(input, Config{IdleDays: 7, MinWaste: 0, Now: now})
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	// First finding should be highest severity
	if findings[0].Severity < findings[len(findings)-1].Severity {
		t.Errorf("findings not sorted by severity: first=%s last=%s",
			findings[0].Severity, findings[len(findings)-1].Severity)
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityHigh, "high"},
		{SeverityMedium, "medium"},
		{SeverityLow, "low"},
		{SeverityInfo, "info"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %s, want %s", tt.s, got, tt.want)
		}
	}
}

func TestWindowDays(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	usage := []TokenUsage{
		{StartTime: base, EndTime: base.Add(24 * time.Hour)},
		{StartTime: base.AddDate(0, 0, 9), EndTime: base.AddDate(0, 0, 10)},
	}
	got := windowDays(usage)
	if got != 11 {
		t.Errorf("windowDays = %d, want 11", got)
	}
}

func TestWindowDaysEmpty(t *testing.T) {
	got := windowDays(nil)
	if got != 1 {
		t.Errorf("windowDays(nil) = %d, want 1", got)
	}
}

// --- helpers ---

func makeUsage(model string, inputTokens, outputTokens, cachedTokens, requests int) []TokenUsage {
	base := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	return []TokenUsage{
		{
			Platform:     "openai",
			Model:        model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			CachedTokens: cachedTokens,
			Requests:     requests,
			StartTime:    base,
			EndTime:      base.Add(24 * time.Hour),
		},
	}
}
