package vertexai

import (
	"fmt"
	"time"
)

// Config holds GCP Vertex AI client settings.
type Config struct {
	ProjectID string
	Region    string
}

// UsageBucket is a single time bucket of Vertex AI model usage from Cloud Monitoring.
type UsageBucket struct {
	StartTime    time.Time
	EndTime      time.Time
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
		return fmt.Sprintf("vertexai %s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("vertexai %s: %s", e.Op, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}
