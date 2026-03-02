package cohere

import "errors"

// Config holds Cohere client settings.
type Config struct{}

// UsageBucket is a placeholder for Cohere usage data.
// Cohere does not expose a historical usage API.
type UsageBucket struct{}

// ErrUnsupported is returned because Cohere does not expose a historical usage API.
var ErrUnsupported = errors.New("cohere: historical usage API not available")
