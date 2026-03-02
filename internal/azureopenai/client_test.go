package azureopenai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
)

type mockMetrics struct {
	queryResourceFn func(ctx context.Context, resourceURI string, options *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error)
}

func (m *mockMetrics) QueryResource(ctx context.Context, resourceURI string, options *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error) {
	return m.queryResourceFn(ctx, resourceURI, options)
}

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"empty subscription", Config{ResourceGroup: "rg", AccountName: "acct"}},
		{"empty resource group", Config{SubscriptionID: "sub", AccountName: "acct"}},
		{"empty account name", Config{SubscriptionID: "sub", ResourceGroup: "rg"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestResourceURI(t *testing.T) {
	c := newClientWithAPI(&mockMetrics{}, "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.CognitiveServices/accounts/my-openai")
	want := "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.CognitiveServices/accounts/my-openai"
	if c.ResourceURI() != want {
		t.Errorf("ResourceURI = %s, want %s", c.ResourceURI(), want)
	}
}

func TestFetchUsage(t *testing.T) {
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	mock := &mockMetrics{
		queryResourceFn: func(_ context.Context, resourceURI string, opts *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error) {
			if resourceURI == "" {
				t.Error("empty resourceURI")
			}
			if opts.MetricNames == nil {
				t.Error("missing MetricNames")
			}

			return azquery.MetricsClientQueryResourceResponse{
				Response: azquery.Response{
					Value: []*azquery.Metric{
						{
							Name: &azquery.LocalizableString{Value: ptrTo(metricPromptTokens)},
							TimeSeries: []*azquery.TimeSeriesElement{
								{
									MetadataValues: []*azquery.MetadataValue{
										{Name: &azquery.LocalizableString{Value: ptrTo("ModelDeploymentName")}, Value: ptrTo("gpt-4o-deploy")},
										{Name: &azquery.LocalizableString{Value: ptrTo("ModelName")}, Value: ptrTo("gpt-4o")},
									},
									Data: []*azquery.MetricValue{
										{TimeStamp: &ts, Total: ptrTo(15000.0)},
									},
								},
							},
						},
						{
							Name: &azquery.LocalizableString{Value: ptrTo(metricCompletionTokens)},
							TimeSeries: []*azquery.TimeSeriesElement{
								{
									MetadataValues: []*azquery.MetadataValue{
										{Name: &azquery.LocalizableString{Value: ptrTo("ModelDeploymentName")}, Value: ptrTo("gpt-4o-deploy")},
										{Name: &azquery.LocalizableString{Value: ptrTo("ModelName")}, Value: ptrTo("gpt-4o")},
									},
									Data: []*azquery.MetricValue{
										{TimeStamp: &ts, Total: ptrTo(3000.0)},
									},
								},
							},
						},
						{
							Name: &azquery.LocalizableString{Value: ptrTo(metricRequests)},
							TimeSeries: []*azquery.TimeSeriesElement{
								{
									MetadataValues: []*azquery.MetadataValue{
										{Name: &azquery.LocalizableString{Value: ptrTo("ModelDeploymentName")}, Value: ptrTo("gpt-4o-deploy")},
										{Name: &azquery.LocalizableString{Value: ptrTo("ModelName")}, Value: ptrTo("gpt-4o")},
									},
									Data: []*azquery.MetricValue{
										{TimeStamp: &ts, Total: ptrTo(100.0)},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	c := newClientWithAPI(mock, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/acct")
	start := ts
	end := ts.Add(24 * time.Hour)
	buckets, err := c.FetchUsage(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 {
		t.Fatalf("got %d buckets, want 1", len(buckets))
	}
	b := buckets[0]
	if b.Deployment != "gpt-4o-deploy" {
		t.Errorf("Deployment = %s, want gpt-4o-deploy", b.Deployment)
	}
	if b.Model != "gpt-4o" {
		t.Errorf("Model = %s, want gpt-4o", b.Model)
	}
	if b.InputTokens != 15000 {
		t.Errorf("InputTokens = %d, want 15000", b.InputTokens)
	}
	if b.OutputTokens != 3000 {
		t.Errorf("OutputTokens = %d, want 3000", b.OutputTokens)
	}
	if b.Requests != 100 {
		t.Errorf("Requests = %d, want 100", b.Requests)
	}
}

func TestFetchUsageMultipleDeployments(t *testing.T) {
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	makeSeries := func(deployment, model string, total float64) *azquery.TimeSeriesElement {
		return &azquery.TimeSeriesElement{
			MetadataValues: []*azquery.MetadataValue{
				{Name: &azquery.LocalizableString{Value: ptrTo("ModelDeploymentName")}, Value: ptrTo(deployment)},
				{Name: &azquery.LocalizableString{Value: ptrTo("ModelName")}, Value: ptrTo(model)},
			},
			Data: []*azquery.MetricValue{
				{TimeStamp: &ts, Total: &total},
			},
		}
	}

	mock := &mockMetrics{
		queryResourceFn: func(_ context.Context, _ string, _ *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error) {
			return azquery.MetricsClientQueryResourceResponse{
				Response: azquery.Response{
					Value: []*azquery.Metric{
						{
							Name: &azquery.LocalizableString{Value: ptrTo(metricPromptTokens)},
							TimeSeries: []*azquery.TimeSeriesElement{
								makeSeries("deploy-a", "gpt-4o", 10000),
								makeSeries("deploy-b", "gpt-4o-mini", 20000),
							},
						},
						{
							Name: &azquery.LocalizableString{Value: ptrTo(metricCompletionTokens)},
							TimeSeries: []*azquery.TimeSeriesElement{
								makeSeries("deploy-a", "gpt-4o", 2000),
								makeSeries("deploy-b", "gpt-4o-mini", 5000),
							},
						},
					},
				},
			}, nil
		},
	}

	c := newClientWithAPI(mock, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/acct")
	buckets, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 {
		t.Fatalf("got %d buckets, want 2", len(buckets))
	}

	// Map by deployment for order-independent assertion
	byDeploy := make(map[string]UsageBucket)
	for _, b := range buckets {
		byDeploy[b.Deployment] = b
	}
	a := byDeploy["deploy-a"]
	if a.InputTokens != 10000 {
		t.Errorf("deploy-a InputTokens = %d, want 10000", a.InputTokens)
	}
	b := byDeploy["deploy-b"]
	if b.InputTokens != 20000 {
		t.Errorf("deploy-b InputTokens = %d, want 20000", b.InputTokens)
	}
}

func TestFetchUsageEmpty(t *testing.T) {
	mock := &mockMetrics{
		queryResourceFn: func(_ context.Context, _ string, _ *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error) {
			return azquery.MetricsClientQueryResourceResponse{
				Response: azquery.Response{
					Value: []*azquery.Metric{},
				},
			}, nil
		},
	}

	c := newClientWithAPI(mock, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/acct")
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 0 {
		t.Errorf("got %d buckets, want 0", len(buckets))
	}
}

func TestFetchUsageError(t *testing.T) {
	mock := &mockMetrics{
		queryResourceFn: func(_ context.Context, _ string, _ *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error) {
			return azquery.MetricsClientQueryResourceResponse{}, errors.New("unauthorized")
		},
	}

	c := newClientWithAPI(mock, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/acct")
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	var aErr *Error
	if !errors.As(err, &aErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if aErr.Op != "QueryResource" {
		t.Errorf("Op = %s, want QueryResource", aErr.Op)
	}
}

func TestErrorFormatting(t *testing.T) {
	e := &Error{Op: "QueryResource", Message: "query failed", Err: errors.New("timeout")}
	got := e.Error()
	want := "azureopenai QueryResource: query failed: timeout"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if e.Unwrap() == nil {
		t.Error("Unwrap() should return non-nil")
	}

	e2 := &Error{Op: "QueryResource", Message: "no credentials"}
	got2 := e2.Error()
	want2 := "azureopenai QueryResource: no credentials"
	if got2 != want2 {
		t.Errorf("Error() = %q, want %q", got2, want2)
	}
	if e2.Unwrap() != nil {
		t.Error("Unwrap() should return nil when Err is nil")
	}
}

func TestFetchUsageNilMetricFields(t *testing.T) {
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	mock := &mockMetrics{
		queryResourceFn: func(_ context.Context, _ string, _ *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error) {
			return azquery.MetricsClientQueryResourceResponse{
				Response: azquery.Response{
					Value: []*azquery.Metric{
						{Name: nil}, // nil Name should be skipped
						{
							Name: &azquery.LocalizableString{Value: ptrTo(metricPromptTokens)},
							TimeSeries: []*azquery.TimeSeriesElement{
								{
									MetadataValues: nil, // nil metadata
									Data: []*azquery.MetricValue{
										{TimeStamp: &ts, Total: ptrTo(500.0)},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	c := newClientWithAPI(mock, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/acct")
	buckets, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 {
		t.Fatalf("got %d buckets, want 1", len(buckets))
	}
	if buckets[0].InputTokens != 500 {
		t.Errorf("InputTokens = %d, want 500", buckets[0].InputTokens)
	}
}

func TestContextCancellation(t *testing.T) {
	mock := &mockMetrics{
		queryResourceFn: func(ctx context.Context, _ string, _ *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error) {
			return azquery.MetricsClientQueryResourceResponse{}, ctx.Err()
		},
	}

	c := newClientWithAPI(mock, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.CognitiveServices/accounts/acct")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(ctx, ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
