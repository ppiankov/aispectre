package anthropic

import (
	"errors"
	"fmt"
)

// Config holds Anthropic Admin API client settings.
type Config struct {
	Token       string
	BaseURL     string
	RateLimitMS int
}

// UsageBucket is a single time bucket from the usage report endpoint.
type UsageBucket struct {
	BucketStartTime string       `json:"bucket_start_time"`
	BucketEndTime   string       `json:"bucket_end_time"`
	Usage           []UsageEntry `json:"usage"`
}

// UsageEntry is a single usage line item within a bucket.
type UsageEntry struct {
	Model                    string `json:"model"`
	InputTokens              int    `json:"input_tokens"`
	OutputTokens             int    `json:"output_tokens"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
	Requests                 int    `json:"requests"`
}

// APIError is a structured non-2xx Anthropic API response.
type APIError struct {
	StatusCode int
	Type       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("anthropic api %d (%s): %s", e.StatusCode, e.Type, e.Message)
	}
	return fmt.Sprintf("anthropic api %d: %s", e.StatusCode, e.Message)
}

// IsStatus reports whether err is an APIError with the given status code.
func IsStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}

// usageResponse is the paginated envelope for the usage report endpoint.
type usageResponse struct {
	Data     []UsageBucket `json:"data"`
	HasMore  bool          `json:"has_more"`
	NextPage *string       `json:"next_page"`
}
