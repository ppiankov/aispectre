package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL     = "https://api.anthropic.com"
	defaultTimeout     = 30 * time.Second
	defaultRateLimitMS = 250
	maxPageSize        = 100
	apiVersion         = "2023-06-01"
)

// Client wraps Anthropic Admin API calls for usage data.
type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	token       string
	minInterval time.Duration

	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient constructs an Anthropic API client.
func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("anthropic API token is required")
	}

	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = defaultBaseURL
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid anthropic base url: %w", err)
	}

	intervalMS := cfg.RateLimitMS
	if intervalMS <= 0 {
		intervalMS = defaultRateLimitMS
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		token:       cfg.Token,
		minInterval: time.Duration(intervalMS) * time.Millisecond,
	}, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	if err := c.waitRateLimit(ctx); err != nil {
		return err
	}

	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + path
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.token)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("User-Agent", "aispectre")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		apiErr := &APIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if len(body) > 0 {
			var payload struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(body, &payload) == nil && payload.Error.Message != "" {
				apiErr.Type = payload.Error.Type
				apiErr.Message = payload.Error.Message
			}
		}
		return apiErr
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode anthropic response: %w", err)
	}
	return nil
}

func (c *Client) waitRateLimit(ctx context.Context) error {
	if c.minInterval <= 0 {
		return nil
	}

	for {
		c.mu.Lock()
		wait := c.minInterval - time.Since(c.lastRequest)
		if wait <= 0 {
			c.lastRequest = time.Now()
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
