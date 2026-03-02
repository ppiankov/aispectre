package vertexai

import (
	"context"
	"fmt"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// MonitoringAPI abstracts Cloud Monitoring for testing.
type MonitoringAPI interface {
	ListTimeSeries(ctx context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error)
}

// Client wraps Cloud Monitoring API calls for Vertex AI usage metrics.
type Client struct {
	mon       MonitoringAPI
	projectID string
}

// NewClient constructs a Vertex AI usage client using Application Default Credentials.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("vertexai: project_id is required")
	}

	var opts []option.ClientOption
	if cfg.Region != "" {
		opts = append(opts, option.WithEndpoint(
			fmt.Sprintf("monitoring.%s.rep.googleapis.com:443", cfg.Region),
		))
	}

	mc, err := monitoring.NewMetricClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("vertexai: monitoring client: %w", err)
	}

	return &Client{
		mon:       &metricClientAdapter{mc: mc},
		projectID: cfg.ProjectID,
	}, nil
}

// newClientWithAPI constructs a Client with a custom MonitoringAPI (for testing).
func newClientWithAPI(api MonitoringAPI, projectID string) *Client {
	return &Client{mon: api, projectID: projectID}
}

// metricClientAdapter adapts the GCP MetricClient iterator API to MonitoringAPI.
type metricClientAdapter struct {
	mc *monitoring.MetricClient
}

func (a *metricClientAdapter) ListTimeSeries(ctx context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
	var results []*monitoringpb.TimeSeries
	it := a.mc.ListTimeSeries(ctx, req)
	for {
		ts, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		results = append(results, ts)
	}
	return results, nil
}
