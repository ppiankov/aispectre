package bedrock

import (
	"fmt"
	"time"
)

// Config holds AWS Bedrock client settings.
type Config struct {
	Region  string
	Profile string
}

// UsageBucket is a single time bucket of Bedrock model usage from CloudWatch.
type UsageBucket struct {
	StartTime    time.Time
	EndTime      time.Time
	ModelID      string
	Invocations  int
	InputTokens  int
	OutputTokens int
}

// Error wraps SDK errors with context.
type Error struct {
	Op      string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("bedrock %s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("bedrock %s: %s", e.Op, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}
