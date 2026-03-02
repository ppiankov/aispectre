package analyzer

import "time"

// FindingKind identifies the category of a waste finding.
type FindingKind string

const (
	KindModelOverkill      FindingKind = "MODEL_OVERKILL"
	KindNoCaching          FindingKind = "NO_CACHING"
	KindKeyUnused          FindingKind = "KEY_UNUSED"
	KindFinetunedIdle      FindingKind = "FINETUNED_IDLE"
	KindCostSpike          FindingKind = "COST_SPIKE"
	KindBatchEligible      FindingKind = "BATCH_ELIGIBLE"
	KindTokenInefficiency  FindingKind = "TOKEN_INEFFICIENCY"
	KindDowngradeAvailable FindingKind = "DOWNGRADE_AVAILABLE"
)

// Severity ranks how critical a finding is.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
)

func (s Severity) String() string {
	switch s {
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	default:
		return "info"
	}
}

// TokenUsage is normalized token usage from any platform.
type TokenUsage struct {
	StartTime    time.Time
	EndTime      time.Time
	Platform     string
	Model        string
	InputTokens  int
	OutputTokens int
	CachedTokens int // input tokens served from cache
	Requests     int
}

// DailyCost is a daily cost entry (e.g. from OpenAI costs API).
type DailyCost struct {
	Date     time.Time
	Amount   float64
	Currency string
}

// APIKeyInfo is metadata about an API key.
type APIKeyInfo struct {
	Platform   string
	ID         string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// ModelInfo is metadata about a deployed or fine-tuned model.
type ModelInfo struct {
	Platform string
	ID       string
	OwnedBy  string
	Created  time.Time
}

// Input holds all normalized data for analysis.
type Input struct {
	TokenUsage []TokenUsage
	DailyCosts []DailyCost
	APIKeys    []APIKeyInfo
	Models     []ModelInfo
}

// Config holds analyzer thresholds. Now is injectable for deterministic tests.
type Config struct {
	IdleDays int
	MinWaste float64
	Now      time.Time
}

// Finding is a single waste or hygiene issue detected by the analyzer.
type Finding struct {
	Kind         FindingKind
	Severity     Severity
	Platform     string
	Model        string
	Title        string
	Description  string
	MonthlyWaste float64
	Evidence     map[string]any
}
