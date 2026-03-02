package anthropic

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// FetchUsage retrieves organization token usage grouped by model per day.
func (c *Client) FetchUsage(ctx context.Context, startTime, endTime time.Time) ([]UsageBucket, error) {
	var all []UsageBucket

	query := url.Values{
		"starting_at":  {startTime.UTC().Format(time.RFC3339)},
		"bucket_width": {"1d"},
		"group_by":     {"model"},
		"limit":        {strconv.Itoa(maxPageSize)},
	}
	if !endTime.IsZero() {
		query.Set("ending_at", endTime.UTC().Format(time.RFC3339))
	}

	for {
		var resp usageResponse
		if err := c.get(ctx, "/v1/organizations/usage_report/messages", query, &resp); err != nil {
			return nil, fmt.Errorf("fetch usage: %w", err)
		}
		all = append(all, resp.Data...)

		if !resp.HasMore || resp.NextPage == nil {
			break
		}
		query.Set("page", *resp.NextPage)
	}
	return all, nil
}
