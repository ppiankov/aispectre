package bedrock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type mockCloudWatch struct {
	getMetricDataFn func(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
	listMetricsFn   func(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
}

func (m *mockCloudWatch) GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	return m.getMetricDataFn(ctx, params, optFns...)
}

func (m *mockCloudWatch) ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
	return m.listMetricsFn(ctx, params, optFns...)
}

func TestNewClientRequiresRegion(t *testing.T) {
	_, err := NewClient(context.Background(), Config{})
	if err == nil {
		t.Fatal("expected error for empty region")
	}
}

func TestFetchUsage(t *testing.T) {
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	mock := &mockCloudWatch{
		listMetricsFn: func(_ context.Context, params *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			if aws.ToString(params.Namespace) != bedrockNamespace {
				t.Errorf("namespace = %s, want %s", aws.ToString(params.Namespace), bedrockNamespace)
			}
			return &cloudwatch.ListMetricsOutput{
				Metrics: []cwtypes.Metric{
					{
						Dimensions: []cwtypes.Dimension{
							{Name: aws.String("ModelId"), Value: aws.String("anthropic.claude-3-sonnet")},
						},
					},
				},
			}, nil
		},
		getMetricDataFn: func(_ context.Context, _ *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
			return &cloudwatch.GetMetricDataOutput{
				MetricDataResults: []cwtypes.MetricDataResult{
					{Id: aws.String("inv_0"), Timestamps: []time.Time{ts}, Values: []float64{200}},
					{Id: aws.String("inp_1"), Timestamps: []time.Time{ts}, Values: []float64{50000}},
					{Id: aws.String("out_2"), Timestamps: []time.Time{ts}, Values: []float64{10000}},
				},
			}, nil
		},
	}

	c := newClientWithAPI(mock, "us-east-1")
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
	if b.ModelID != "anthropic.claude-3-sonnet" {
		t.Errorf("ModelID = %s, want anthropic.claude-3-sonnet", b.ModelID)
	}
	if b.Invocations != 200 {
		t.Errorf("Invocations = %d, want 200", b.Invocations)
	}
	if b.InputTokens != 50000 {
		t.Errorf("InputTokens = %d, want 50000", b.InputTokens)
	}
	if b.OutputTokens != 10000 {
		t.Errorf("OutputTokens = %d, want 10000", b.OutputTokens)
	}
}

func TestFetchUsageMultipleModels(t *testing.T) {
	ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	mock := &mockCloudWatch{
		listMetricsFn: func(_ context.Context, _ *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			return &cloudwatch.ListMetricsOutput{
				Metrics: []cwtypes.Metric{
					{Dimensions: []cwtypes.Dimension{{Name: aws.String("ModelId"), Value: aws.String("model-a")}}},
					{Dimensions: []cwtypes.Dimension{{Name: aws.String("ModelId"), Value: aws.String("model-b")}}},
				},
			}, nil
		},
		getMetricDataFn: func(_ context.Context, params *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
			if len(params.MetricDataQueries) != 6 {
				t.Errorf("got %d queries, want 6 (3 per model × 2 models)", len(params.MetricDataQueries))
			}
			return &cloudwatch.GetMetricDataOutput{
				MetricDataResults: []cwtypes.MetricDataResult{
					{Id: aws.String("inv_0"), Timestamps: []time.Time{ts}, Values: []float64{100}},
					{Id: aws.String("inp_1"), Timestamps: []time.Time{ts}, Values: []float64{20000}},
					{Id: aws.String("out_2"), Timestamps: []time.Time{ts}, Values: []float64{5000}},
					{Id: aws.String("inv_3"), Timestamps: []time.Time{ts}, Values: []float64{50}},
					{Id: aws.String("inp_4"), Timestamps: []time.Time{ts}, Values: []float64{10000}},
					{Id: aws.String("out_5"), Timestamps: []time.Time{ts}, Values: []float64{2000}},
				},
			}, nil
		},
	}

	c := newClientWithAPI(mock, "us-east-1")
	buckets, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 {
		t.Fatalf("got %d buckets, want 2", len(buckets))
	}
	if buckets[0].ModelID != "model-a" || buckets[1].ModelID != "model-b" {
		t.Errorf("models = [%s, %s], want [model-a, model-b]", buckets[0].ModelID, buckets[1].ModelID)
	}
}

func TestFetchUsageListMetricsPagination(t *testing.T) {
	calls := 0

	mock := &mockCloudWatch{
		listMetricsFn: func(_ context.Context, params *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			calls++
			if calls == 1 {
				return &cloudwatch.ListMetricsOutput{
					Metrics: []cwtypes.Metric{
						{Dimensions: []cwtypes.Dimension{{Name: aws.String("ModelId"), Value: aws.String("model-a")}}},
					},
					NextToken: aws.String("token1"),
				}, nil
			}
			return &cloudwatch.ListMetricsOutput{
				Metrics: []cwtypes.Metric{
					{Dimensions: []cwtypes.Dimension{{Name: aws.String("ModelId"), Value: aws.String("model-b")}}},
				},
			}, nil
		},
		getMetricDataFn: func(_ context.Context, _ *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
			return &cloudwatch.GetMetricDataOutput{
				MetricDataResults: []cwtypes.MetricDataResult{},
			}, nil
		},
	}

	c := newClientWithAPI(mock, "us-east-1")
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("ListMetrics calls = %d, want 2", calls)
	}
}

func TestFetchUsageEmpty(t *testing.T) {
	mock := &mockCloudWatch{
		listMetricsFn: func(_ context.Context, _ *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			return &cloudwatch.ListMetricsOutput{Metrics: []cwtypes.Metric{}}, nil
		},
	}

	c := newClientWithAPI(mock, "us-east-1")
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if buckets != nil {
		t.Errorf("got %d buckets, want nil", len(buckets))
	}
}

func TestFetchUsageListMetricsError(t *testing.T) {
	mock := &mockCloudWatch{
		listMetricsFn: func(_ context.Context, _ *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	c := newClientWithAPI(mock, "us-east-1")
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	var bErr *Error
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if bErr.Op != "ListMetrics" {
		t.Errorf("Op = %s, want ListMetrics", bErr.Op)
	}
}

func TestFetchUsageGetMetricDataError(t *testing.T) {
	mock := &mockCloudWatch{
		listMetricsFn: func(_ context.Context, _ *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			return &cloudwatch.ListMetricsOutput{
				Metrics: []cwtypes.Metric{
					{Dimensions: []cwtypes.Dimension{{Name: aws.String("ModelId"), Value: aws.String("model-a")}}},
				},
			}, nil
		},
		getMetricDataFn: func(_ context.Context, _ *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
			return nil, errors.New("throttled")
		},
	}

	c := newClientWithAPI(mock, "us-east-1")
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	var bErr *Error
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if bErr.Op != "GetMetricData" {
		t.Errorf("Op = %s, want GetMetricData", bErr.Op)
	}
}

func TestContextCancellation(t *testing.T) {
	mock := &mockCloudWatch{
		listMetricsFn: func(ctx context.Context, _ *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			return nil, ctx.Err()
		},
	}

	c := newClientWithAPI(mock, "us-east-1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchUsage(ctx, ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestErrorFormatting(t *testing.T) {
	e := &Error{Op: "GetMetricData", Message: "fetch failed", Err: errors.New("throttled")}
	got := e.Error()
	want := "bedrock GetMetricData: fetch failed: throttled"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if e.Unwrap() == nil {
		t.Error("Unwrap() should return non-nil")
	}

	e2 := &Error{Op: "ListMetrics", Message: "no permissions"}
	got2 := e2.Error()
	want2 := "bedrock ListMetrics: no permissions"
	if got2 != want2 {
		t.Errorf("Error() = %q, want %q", got2, want2)
	}
	if e2.Unwrap() != nil {
		t.Error("Unwrap() should return nil when Err is nil")
	}
}
