package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{
		Token:       "sk-ant-admin-test",
		BaseURL:     server.URL,
		RateLimitMS: 1,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestNewClientRequiresToken(t *testing.T) {
	_, err := NewClient(Config{})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestNewClientDefaults(t *testing.T) {
	c, err := NewClient(Config{Token: "sk-ant-admin-test"})
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL.String() != defaultBaseURL {
		t.Errorf("baseURL = %s, want %s", c.baseURL.String(), defaultBaseURL)
	}
	if c.minInterval != time.Duration(defaultRateLimitMS)*time.Millisecond {
		t.Errorf("minInterval = %v, want %v", c.minInterval, time.Duration(defaultRateLimitMS)*time.Millisecond)
	}
}

func TestFetchUsage(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/organizations/usage_report/messages" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-api-key") != "sk-ant-admin-test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("anthropic-version") != apiVersion {
			http.Error(w, "missing anthropic-version", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("starting_at") == "" {
			http.Error(w, "missing starting_at", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("group_by") != "model" {
			t.Error("missing group_by=model")
		}
		resp := usageResponse{
			Data: []UsageBucket{
				{
					BucketStartTime: "2025-01-01T00:00:00Z",
					BucketEndTime:   "2025-01-02T00:00:00Z",
					Usage: []UsageEntry{
						{
							Model:                    "claude-sonnet-4-20250514",
							InputTokens:              15000,
							OutputTokens:             3000,
							CacheCreationInputTokens: 5000,
							CacheReadInputTokens:     2000,
							Requests:                 100,
						},
					},
				},
			},
			HasMore: false,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	c := newTestClient(t, handler)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 {
		t.Fatalf("got %d buckets, want 1", len(buckets))
	}
	if len(buckets[0].Usage) != 1 {
		t.Fatalf("got %d usage entries, want 1", len(buckets[0].Usage))
	}
	u := buckets[0].Usage[0]
	if u.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %s, want claude-sonnet-4-20250514", u.Model)
	}
	if u.InputTokens != 15000 {
		t.Errorf("input_tokens = %d, want 15000", u.InputTokens)
	}
	if u.OutputTokens != 3000 {
		t.Errorf("output_tokens = %d, want 3000", u.OutputTokens)
	}
	if u.CacheCreationInputTokens != 5000 {
		t.Errorf("cache_creation_input_tokens = %d, want 5000", u.CacheCreationInputTokens)
	}
	if u.CacheReadInputTokens != 2000 {
		t.Errorf("cache_read_input_tokens = %d, want 2000", u.CacheReadInputTokens)
	}
	if u.Requests != 100 {
		t.Errorf("requests = %d, want 100", u.Requests)
	}
}

func TestFetchUsagePagination(t *testing.T) {
	page2 := "page2token"
	calls := 0

	handler := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "page2token" {
			resp := usageResponse{
				Data:    []UsageBucket{{BucketStartTime: "2025-01-02T00:00:00Z"}},
				HasMore: false,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := usageResponse{
			Data:     []UsageBucket{{BucketStartTime: "2025-01-01T00:00:00Z"}},
			HasMore:  true,
			NextPage: &page2,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	c := newTestClient(t, handler)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), start, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 {
		t.Fatalf("got %d buckets, want 2", len(buckets))
	}
	if calls != 2 {
		t.Errorf("got %d API calls, want 2", calls)
	}
}

func TestFetchUsageEmpty(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(usageResponse{Data: []UsageBucket{}})
	}

	c := newTestClient(t, handler)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), start, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 0 {
		t.Errorf("got %d buckets, want 0", len(buckets))
	}
}

func TestAPIError(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		})
	}

	c := newTestClient(t, handler)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), start, time.Time{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsStatus(err, http.StatusUnauthorized) {
		t.Errorf("expected 401, got: %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("expected APIError")
	}
	if apiErr.Type != "authentication_error" {
		t.Errorf("type = %s, want authentication_error", apiErr.Type)
	}
	if apiErr.Message != "Invalid API key" {
		t.Errorf("message = %s, want Invalid API key", apiErr.Message)
	}
}

func TestAPIErrorNonJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}

	c := newTestClient(t, handler)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), start, time.Time{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsStatus(err, http.StatusInternalServerError) {
		t.Errorf("expected 500, got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(time.Second)
		w.WriteHeader(http.StatusOK)
	}

	c := newTestClient(t, handler)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(ctx, start, time.Time{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestIsStatus(t *testing.T) {
	if IsStatus(nil, 401) {
		t.Error("IsStatus(nil) should be false")
	}
	if IsStatus(fmt.Errorf("other"), 401) {
		t.Error("IsStatus(non-APIError) should be false")
	}
}
