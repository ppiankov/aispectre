package groq

import "errors"

// Config holds Groq client settings.
type Config struct{}

// UsageBucket is a placeholder for Groq usage data.
// Groq does not expose a historical usage API.
type UsageBucket struct{}

// ErrUnsupported is returned because Groq does not expose a historical usage API.
var ErrUnsupported = errors.New("groq: historical usage API not available")
