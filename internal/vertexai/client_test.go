package vertexai

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockMonitoring struct {
	listTimeSeriesFn func(ctx context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error)
}

func (m *mockMonitoring) ListTimeSeries(ctx context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
	return m.listTimeSeriesFn(ctx, req)
}

func makeTokenSeries(publisher, modelID, tokenType string, ts time.Time, value int64) *monitoringpb.TimeSeries {
	return &monitoringpb.TimeSeries{
		Metric: &metricpb.Metric{
			Type:   metricTokenCount,
			Labels: map[string]string{"type": tokenType},
		},
		Resource: &monitoredrespb.MonitoredResource{
			Type:   "aiplatform.googleapis.com/PublisherModel",
			Labels: map[string]string{"publisher": publisher, "model_user_id": modelID},
		},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					EndTime: timestamppb.New(ts),
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{Int64Value: value},
				},
			},
		},
	}
}

func makeInvocationSeries(publisher, modelID string, ts time.Time, value int64) *monitoringpb.TimeSeries {
	return &monitoringpb.TimeSeries{
		Metric: &metricpb.Metric{
			Type: metricInvocationCount,
		},
		Resource: &monitoredrespb.MonitoredResource{
			Type:   "aiplatform.googleapis.com/PublisherModel",
			Labels: map[string]string{"publisher": publisher, "model_user_id": modelID},
		},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					EndTime: timestamppb.New(ts),
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{Int64Value: value},
				},
			},
		},
	}
}

func TestNewClientRequiresProjectID(t *testing.T) {
	_, err := NewClient(context.Background(), Config{})
	if err == nil {
		t.Fatal("expected error for empty project_id")
	}
}

func TestFetchUsage(t *testing.T) {
	ts := time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)
	calls := 0

	mock := &mockMonitoring{
		listTimeSeriesFn: func(_ context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			calls++
			if req.Name != "projects/my-project" {
				t.Errorf("name = %s, want projects/my-project", req.Name)
			}
			if calls == 1 {
				return []*monitoringpb.TimeSeries{
					makeTokenSeries("google", "gemini-1.5-pro", "input", ts, 15000),
					makeTokenSeries("google", "gemini-1.5-pro", "output", ts, 3000),
				}, nil
			}
			return []*monitoringpb.TimeSeries{
				makeInvocationSeries("google", "gemini-1.5-pro", ts, 100),
			}, nil
		},
	}

	c := newClientWithAPI(mock, "my-project")
	start := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	end := ts
	buckets, err := c.FetchUsage(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 {
		t.Fatalf("got %d buckets, want 1", len(buckets))
	}
	b := buckets[0]
	if b.Model != "google/gemini-1.5-pro" {
		t.Errorf("Model = %s, want google/gemini-1.5-pro", b.Model)
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
	if calls != 2 {
		t.Errorf("ListTimeSeries calls = %d, want 2", calls)
	}
}

func TestFetchUsageMultipleModels(t *testing.T) {
	ts := time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)
	calls := 0

	mock := &mockMonitoring{
		listTimeSeriesFn: func(_ context.Context, _ *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			calls++
			if calls == 1 {
				return []*monitoringpb.TimeSeries{
					makeTokenSeries("google", "gemini-1.5-pro", "input", ts, 10000),
					makeTokenSeries("google", "gemini-1.5-pro", "output", ts, 2000),
					makeTokenSeries("google", "gemini-1.5-flash", "input", ts, 30000),
					makeTokenSeries("google", "gemini-1.5-flash", "output", ts, 8000),
				}, nil
			}
			return []*monitoringpb.TimeSeries{
				makeInvocationSeries("google", "gemini-1.5-pro", ts, 50),
				makeInvocationSeries("google", "gemini-1.5-flash", ts, 200),
			}, nil
		},
	}

	c := newClientWithAPI(mock, "my-project")
	start := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), start, ts)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 {
		t.Fatalf("got %d buckets, want 2", len(buckets))
	}

	byModel := make(map[string]UsageBucket)
	for _, b := range buckets {
		byModel[b.Model] = b
	}
	pro := byModel["google/gemini-1.5-pro"]
	if pro.InputTokens != 10000 {
		t.Errorf("pro InputTokens = %d, want 10000", pro.InputTokens)
	}
	flash := byModel["google/gemini-1.5-flash"]
	if flash.InputTokens != 30000 {
		t.Errorf("flash InputTokens = %d, want 30000", flash.InputTokens)
	}
}

func TestFetchUsageEmpty(t *testing.T) {
	mock := &mockMonitoring{
		listTimeSeriesFn: func(_ context.Context, _ *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			return nil, nil
		},
	}

	c := newClientWithAPI(mock, "my-project")
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 0 {
		t.Errorf("got %d buckets, want 0", len(buckets))
	}
}

func TestFetchUsageTokenError(t *testing.T) {
	mock := &mockMonitoring{
		listTimeSeriesFn: func(_ context.Context, _ *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			return nil, errors.New("permission denied")
		},
	}

	c := newClientWithAPI(mock, "my-project")
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	var vErr *Error
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if vErr.Op != "ListTimeSeries" {
		t.Errorf("Op = %s, want ListTimeSeries", vErr.Op)
	}
}

func TestFetchUsageInvocationError(t *testing.T) {
	calls := 0
	mock := &mockMonitoring{
		listTimeSeriesFn: func(_ context.Context, _ *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			calls++
			if calls == 1 {
				return nil, nil // tokens succeed
			}
			return nil, errors.New("quota exceeded") // invocations fail
		},
	}

	c := newClientWithAPI(mock, "my-project")
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	var vErr *Error
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
}

func TestContextCancellation(t *testing.T) {
	mock := &mockMonitoring{
		listTimeSeriesFn: func(ctx context.Context, _ *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			return nil, ctx.Err()
		},
	}

	c := newClientWithAPI(mock, "my-project")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(ctx, ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestErrorFormatting(t *testing.T) {
	e := &Error{Op: "ListTimeSeries", Message: "fetch failed", Err: errors.New("timeout")}
	got := e.Error()
	want := "vertexai ListTimeSeries: fetch failed: timeout"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if e.Unwrap() == nil {
		t.Error("Unwrap() should return non-nil")
	}

	e2 := &Error{Op: "ListTimeSeries", Message: "no permissions"}
	got2 := e2.Error()
	want2 := "vertexai ListTimeSeries: no permissions"
	if got2 != want2 {
		t.Errorf("Error() = %q, want %q", got2, want2)
	}
	if e2.Unwrap() != nil {
		t.Error("Unwrap() should return nil when Err is nil")
	}
}
