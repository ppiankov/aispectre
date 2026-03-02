package analyzer

import "strings"

// Tier classifies model cost/capability level.
type Tier int

const (
	TierSmall  Tier = 1 // cheap, fast inference
	TierMedium Tier = 2 // balanced
	TierLarge  Tier = 3 // expensive, frontier
)

// ModelPricing holds pricing and capability data for a known model.
type ModelPricing struct {
	Name          string
	Tier          Tier
	InputPerMTok  float64 // USD per 1M input tokens
	OutputPerMTok float64 // USD per 1M output tokens
	SupportsBatch bool
	SupportsCache bool
	DowngradeTo   string // cheaper alternative (empty if none)
}

// modelDB is the static pricing database. Prices are approximate and used
// for relative waste estimation, not billing.
var modelDB = map[string]ModelPricing{
	// OpenAI
	"gpt-4o":        {Name: "gpt-4o", Tier: TierMedium, InputPerMTok: 2.50, OutputPerMTok: 10.00, SupportsBatch: true, SupportsCache: true, DowngradeTo: "gpt-4o-mini"},
	"gpt-4o-mini":   {Name: "gpt-4o-mini", Tier: TierSmall, InputPerMTok: 0.15, OutputPerMTok: 0.60, SupportsBatch: true, SupportsCache: true},
	"gpt-4-turbo":   {Name: "gpt-4-turbo", Tier: TierLarge, InputPerMTok: 10.00, OutputPerMTok: 30.00, SupportsBatch: true, DowngradeTo: "gpt-4o"},
	"gpt-4":         {Name: "gpt-4", Tier: TierLarge, InputPerMTok: 30.00, OutputPerMTok: 60.00, DowngradeTo: "gpt-4o"},
	"gpt-3.5-turbo": {Name: "gpt-3.5-turbo", Tier: TierSmall, InputPerMTok: 0.50, OutputPerMTok: 1.50, SupportsBatch: true},
	"o1":            {Name: "o1", Tier: TierLarge, InputPerMTok: 15.00, OutputPerMTok: 60.00, DowngradeTo: "o3-mini"},
	"o1-mini":       {Name: "o1-mini", Tier: TierMedium, InputPerMTok: 3.00, OutputPerMTok: 12.00, DowngradeTo: "o3-mini"},
	"o3-mini":       {Name: "o3-mini", Tier: TierSmall, InputPerMTok: 1.10, OutputPerMTok: 4.40, SupportsBatch: true},

	// Anthropic
	"claude-3-opus-20240229":     {Name: "claude-3-opus-20240229", Tier: TierLarge, InputPerMTok: 15.00, OutputPerMTok: 75.00, SupportsCache: true, DowngradeTo: "claude-3.5-sonnet-20241022"},
	"claude-3-sonnet-20240229":   {Name: "claude-3-sonnet-20240229", Tier: TierMedium, InputPerMTok: 3.00, OutputPerMTok: 15.00, SupportsCache: true, DowngradeTo: "claude-3.5-haiku-20241022"},
	"claude-3-haiku-20240307":    {Name: "claude-3-haiku-20240307", Tier: TierSmall, InputPerMTok: 0.25, OutputPerMTok: 1.25, SupportsCache: true},
	"claude-3.5-sonnet-20241022": {Name: "claude-3.5-sonnet-20241022", Tier: TierMedium, InputPerMTok: 3.00, OutputPerMTok: 15.00, SupportsCache: true, DowngradeTo: "claude-3.5-haiku-20241022"},
	"claude-3.5-haiku-20241022":  {Name: "claude-3.5-haiku-20241022", Tier: TierSmall, InputPerMTok: 0.80, OutputPerMTok: 4.00, SupportsCache: true},

	// Google
	"gemini-1.5-pro":   {Name: "gemini-1.5-pro", Tier: TierMedium, InputPerMTok: 1.25, OutputPerMTok: 5.00, DowngradeTo: "gemini-1.5-flash"},
	"gemini-1.5-flash": {Name: "gemini-1.5-flash", Tier: TierSmall, InputPerMTok: 0.075, OutputPerMTok: 0.30},
	"gemini-2.0-flash": {Name: "gemini-2.0-flash", Tier: TierSmall, InputPerMTok: 0.10, OutputPerMTok: 0.40},

	// Google (Vertex AI publisher/model_user_id format)
	"google/gemini-1.5-pro":   {Name: "google/gemini-1.5-pro", Tier: TierMedium, InputPerMTok: 1.25, OutputPerMTok: 5.00, DowngradeTo: "google/gemini-1.5-flash"},
	"google/gemini-1.5-flash": {Name: "google/gemini-1.5-flash", Tier: TierSmall, InputPerMTok: 0.075, OutputPerMTok: 0.30},
	"google/gemini-2.0-flash": {Name: "google/gemini-2.0-flash", Tier: TierSmall, InputPerMTok: 0.10, OutputPerMTok: 0.40},

	// AWS Bedrock (prefix-matched)
	"anthropic.claude-3-sonnet": {Name: "anthropic.claude-3-sonnet", Tier: TierMedium, InputPerMTok: 3.00, OutputPerMTok: 15.00, DowngradeTo: "anthropic.claude-3-haiku"},
	"anthropic.claude-3-haiku":  {Name: "anthropic.claude-3-haiku", Tier: TierSmall, InputPerMTok: 0.25, OutputPerMTok: 1.25},
	"amazon.titan-text-express": {Name: "amazon.titan-text-express", Tier: TierSmall, InputPerMTok: 0.20, OutputPerMTok: 0.60},
	"meta.llama3-8b-instruct":   {Name: "meta.llama3-8b-instruct", Tier: TierSmall, InputPerMTok: 0.30, OutputPerMTok: 0.60},
	"meta.llama3-70b-instruct":  {Name: "meta.llama3-70b-instruct", Tier: TierMedium, InputPerMTok: 2.65, OutputPerMTok: 3.50, DowngradeTo: "meta.llama3-8b-instruct"},
}

// LookupModel finds pricing data for a model ID.
// It tries exact match first, then prefix match for Bedrock-style IDs.
func LookupModel(modelID string) (ModelPricing, bool) {
	if p, ok := modelDB[modelID]; ok {
		return p, true
	}
	// Prefix match: find the longest matching key.
	var best ModelPricing
	bestLen := 0
	for key, p := range modelDB {
		if strings.HasPrefix(modelID, key) && len(key) > bestLen {
			best = p
			bestLen = len(key)
		}
	}
	if bestLen > 0 {
		return best, true
	}
	return ModelPricing{}, false
}

// TokenCost estimates the USD cost for given token counts on a model.
// Returns 0 for unknown models.
func TokenCost(model string, inputTokens, outputTokens int) float64 {
	p, ok := LookupModel(model)
	if !ok {
		return 0
	}
	return float64(inputTokens)*p.InputPerMTok/1_000_000 + float64(outputTokens)*p.OutputPerMTok/1_000_000
}
