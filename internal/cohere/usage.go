package cohere

import (
	"context"
	"time"
)

// FetchUsage returns ErrUnsupported because Cohere does not expose a historical usage API.
func (c *Client) FetchUsage(_ context.Context, _, _ time.Time) ([]UsageBucket, error) {
	return nil, ErrUnsupported
}
