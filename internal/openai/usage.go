package openai

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// FetchCosts retrieves organization cost data grouped by model per day.
func (c *Client) FetchCosts(ctx context.Context, startTime, endTime time.Time) ([]CostBucket, error) {
	var all []CostBucket

	query := url.Values{
		"start_time":   {strconv.FormatInt(startTime.Unix(), 10)},
		"bucket_width": {"1d"},
		"limit":        {strconv.Itoa(maxDailyBuckets)},
	}
	if !endTime.IsZero() {
		query.Set("end_time", strconv.FormatInt(endTime.Unix(), 10))
	}

	for {
		var resp costsResponse
		if err := c.get(ctx, "/v1/organization/costs", query, &resp); err != nil {
			return nil, fmt.Errorf("fetch costs: %w", err)
		}
		all = append(all, resp.Data...)

		if !resp.HasMore || resp.NextPage == nil {
			break
		}
		query.Set("page", *resp.NextPage)
	}
	return all, nil
}

// FetchCompletionUsage retrieves token usage for completions grouped by model per day.
func (c *Client) FetchCompletionUsage(ctx context.Context, startTime, endTime time.Time) ([]UsageBucket, error) {
	var all []UsageBucket

	query := url.Values{
		"start_time":   {strconv.FormatInt(startTime.Unix(), 10)},
		"bucket_width": {"1d"},
		"group_by[]":   {"model"},
		"limit":        {strconv.Itoa(maxDailyBuckets)},
	}
	if !endTime.IsZero() {
		query.Set("end_time", strconv.FormatInt(endTime.Unix(), 10))
	}

	for {
		var resp usageResponse
		if err := c.get(ctx, "/v1/organization/usage/completions", query, &resp); err != nil {
			return nil, fmt.Errorf("fetch completion usage: %w", err)
		}
		all = append(all, resp.Data...)

		if !resp.HasMore || resp.NextPage == nil {
			break
		}
		query.Set("page", *resp.NextPage)
	}
	return all, nil
}

// FetchAPIKeys retrieves the list of organization admin API keys.
func (c *Client) FetchAPIKeys(ctx context.Context) ([]APIKey, error) {
	var all []APIKey

	query := url.Values{
		"limit": {strconv.Itoa(maxPageSize)},
	}

	for {
		var resp apiKeysResponse
		if err := c.get(ctx, "/v1/organization/admin_api_keys", query, &resp); err != nil {
			return nil, fmt.Errorf("fetch api keys: %w", err)
		}
		all = append(all, resp.Data...)

		if !resp.HasMore || resp.LastID == nil {
			break
		}
		query.Set("after", *resp.LastID)
	}
	return all, nil
}

// FetchModels retrieves the list of models available to the organization.
func (c *Client) FetchModels(ctx context.Context) ([]Model, error) {
	var resp modelsResponse
	if err := c.get(ctx, "/v1/models", nil, &resp); err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	return resp.Data, nil
}
