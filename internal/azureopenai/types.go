package azureopenai

import (
	"fmt"
	"time"
)

// Config holds Azure OpenAI client settings.
type Config struct {
	SubscriptionID string
	ResourceGroup  string
	AccountName    string
}

// UsageBucket is a single time bucket of Azure OpenAI model usage from Azure Monitor.
type UsageBucket struct {
	StartTime    time.Time
	EndTime      time.Time
	Deployment   string
	Model        string
	InputTokens  int
	OutputTokens int
	Requests     int
}

// Error wraps SDK errors with context.
type Error struct {
	Op      string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("azureopenai %s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("azureopenai %s: %s", e.Op, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}
