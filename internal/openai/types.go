package openai

import (
	"errors"
	"fmt"
)

// Config holds OpenAI Admin API client settings.
type Config struct {
	Token       string
	BaseURL     string
	RateLimitMS int
}

// CostBucket is a single time bucket from the costs endpoint.
type CostBucket struct {
	StartTime int64        `json:"start_time"`
	EndTime   int64        `json:"end_time"`
	Results   []CostResult `json:"results"`
}

// CostResult is a single cost line item within a bucket.
type CostResult struct {
	Amount         CostAmount `json:"amount"`
	LineItem       *string    `json:"line_item"`
	ProjectID      *string    `json:"project_id"`
	OrganizationID *string    `json:"organization_id"`
}

// CostAmount holds a monetary value with currency.
type CostAmount struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

// UsageBucket is a single time bucket from the usage endpoint.
type UsageBucket struct {
	StartTime int64         `json:"start_time"`
	EndTime   int64         `json:"end_time"`
	Results   []UsageResult `json:"results"`
}

// UsageResult is a single usage line item within a bucket.
type UsageResult struct {
	InputTokens       int     `json:"input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	InputCachedTokens int     `json:"input_cached_tokens"`
	NumModelRequests  int     `json:"num_model_requests"`
	ProjectID         *string `json:"project_id"`
	Model             *string `json:"model"`
}

// APIKey represents an OpenAI organization API key.
type APIKey struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	CreatedAt     int64   `json:"created_at"`
	LastUsedAt    *int64  `json:"last_used_at"`
	RedactedValue string  `json:"redacted_value"`
	Owner         *string `json:"-"`
}

// Model represents an OpenAI model.
type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
	Created int64  `json:"created"`
}

// APIError is a structured non-2xx OpenAI API response.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("openai api %d (%s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("openai api %d: %s", e.StatusCode, e.Message)
}

// IsStatus reports whether err is an APIError with the given status code.
func IsStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}

// costsResponse is the paginated envelope for the costs endpoint.
type costsResponse struct {
	Data     []CostBucket `json:"data"`
	HasMore  bool         `json:"has_more"`
	NextPage *string      `json:"next_page"`
}

// usageResponse is the paginated envelope for the usage endpoint.
type usageResponse struct {
	Data     []UsageBucket `json:"data"`
	HasMore  bool          `json:"has_more"`
	NextPage *string       `json:"next_page"`
}

// apiKeysResponse is the paginated envelope for the admin API keys endpoint.
type apiKeysResponse struct {
	Data    []APIKey `json:"data"`
	HasMore bool     `json:"has_more"`
	LastID  *string  `json:"last_id"`
}

// modelsResponse is the envelope for the models endpoint.
type modelsResponse struct {
	Data []Model `json:"data"`
}
