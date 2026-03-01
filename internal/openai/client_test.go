package openai

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
		Token:       "test-token",
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
	c, err := NewClient(Config{Token: "sk-test"})
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

func TestFetchCosts(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/organization/costs" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("start_time") == "" {
			http.Error(w, "missing start_time", http.StatusBadRequest)
			return
		}
		resp := costsResponse{
			Data: []CostBucket{
				{
					StartTime: 1709251200,
					EndTime:   1709337600,
					Results: []CostResult{
						{
							Amount: CostAmount{Value: 12.50, Currency: "usd"},
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
	start := time.Unix(1709251200, 0)
	buckets, err := c.FetchCosts(context.Background(), start, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 {
		t.Fatalf("got %d buckets, want 1", len(buckets))
	}
	if len(buckets[0].Results) != 1 {
		t.Fatalf("got %d results, want 1", len(buckets[0].Results))
	}
	if buckets[0].Results[0].Amount.Value != 12.50 {
		t.Errorf("cost = %f, want 12.50", buckets[0].Results[0].Amount.Value)
	}
}

func TestFetchCostsPagination(t *testing.T) {
	page2 := "page2"
	calls := 0

	handler := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "page2" {
			resp := costsResponse{
				Data:    []CostBucket{{StartTime: 2}},
				HasMore: false,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := costsResponse{
			Data:     []CostBucket{{StartTime: 1}},
			HasMore:  true,
			NextPage: &page2,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	c := newTestClient(t, handler)
	buckets, err := c.FetchCosts(context.Background(), time.Unix(1, 0), time.Time{})
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

func TestFetchCompletionUsage(t *testing.T) {
	model := "gpt-4o"

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/organization/usage/completions" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("group_by[]") != "model" {
			t.Error("missing group_by[]=model")
		}
		resp := usageResponse{
			Data: []UsageBucket{
				{
					StartTime: 1709251200,
					EndTime:   1709337600,
					Results: []UsageResult{
						{
							InputTokens:      10000,
							OutputTokens:     500,
							NumModelRequests: 50,
							Model:            &model,
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
	start := time.Unix(1709251200, 0)
	end := time.Unix(1709337600, 0)
	buckets, err := c.FetchCompletionUsage(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 {
		t.Fatalf("got %d buckets, want 1", len(buckets))
	}
	r := buckets[0].Results[0]
	if r.InputTokens != 10000 {
		t.Errorf("input_tokens = %d, want 10000", r.InputTokens)
	}
	if r.OutputTokens != 500 {
		t.Errorf("output_tokens = %d, want 500", r.OutputTokens)
	}
	if *r.Model != "gpt-4o" {
		t.Errorf("model = %s, want gpt-4o", *r.Model)
	}
}

func TestFetchAPIKeys(t *testing.T) {
	lastUsed := int64(1709337600)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/organization/admin_api_keys" {
			http.NotFound(w, r)
			return
		}
		resp := apiKeysResponse{
			Data: []APIKey{
				{
					ID:            "key_abc123",
					Name:          "Production",
					CreatedAt:     1709251200,
					LastUsedAt:    &lastUsed,
					RedactedValue: "sk-org-*****",
				},
				{
					ID:            "key_xyz789",
					Name:          "Unused",
					CreatedAt:     1700000000,
					LastUsedAt:    nil,
					RedactedValue: "sk-org-*****",
				},
			},
			HasMore: false,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	c := newTestClient(t, handler)
	keys, err := c.FetchAPIKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2", len(keys))
	}
	if keys[0].Name != "Production" {
		t.Errorf("key[0].Name = %s, want Production", keys[0].Name)
	}
	if keys[1].LastUsedAt != nil {
		t.Errorf("key[1].LastUsedAt should be nil")
	}
}

func TestFetchAPIKeysPagination(t *testing.T) {
	lastID := "key_abc123"
	calls := 0

	handler := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("after") == "key_abc123" {
			resp := apiKeysResponse{
				Data:    []APIKey{{ID: "key_page2"}},
				HasMore: false,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := apiKeysResponse{
			Data:    []APIKey{{ID: "key_page1"}},
			HasMore: true,
			LastID:  &lastID,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	c := newTestClient(t, handler)
	keys, err := c.FetchAPIKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2", len(keys))
	}
	if calls != 2 {
		t.Errorf("got %d API calls, want 2", calls)
	}
}

func TestFetchModels(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		resp := modelsResponse{
			Data: []Model{
				{ID: "gpt-4o", OwnedBy: "openai", Created: 1686935002},
				{ID: "ft:gpt-4o:my-org:custom:id", OwnedBy: "user-abc123", Created: 1709251200},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	c := newTestClient(t, handler)
	models, err := c.FetchModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].ID != "gpt-4o" {
		t.Errorf("model[0].ID = %s, want gpt-4o", models[0].ID)
	}
	if models[1].OwnedBy != "user-abc123" {
		t.Errorf("model[1].OwnedBy = %s, want user-abc123", models[1].OwnedBy)
	}
}

func TestAPIError(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
	}

	c := newTestClient(t, handler)
	_, err := c.FetchModels(context.Background())
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
	if apiErr.Code != "invalid_api_key" {
		t.Errorf("code = %s, want invalid_api_key", apiErr.Code)
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
	_, err := c.FetchModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsStatus(err, http.StatusInternalServerError) {
		t.Errorf("expected 500, got: %v", err)
	}
}

func TestFetchCostsEmpty(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(costsResponse{Data: []CostBucket{}})
	}

	c := newTestClient(t, handler)
	buckets, err := c.FetchCosts(context.Background(), time.Unix(1, 0), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 0 {
		t.Errorf("got %d buckets, want 0", len(buckets))
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

	_, err := c.FetchModels(ctx)
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
