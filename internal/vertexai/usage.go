package vertexai

import (
	"context"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	metricTokenCount      = "aiplatform.googleapis.com/publisher/online_serving/token_count"
	metricInvocationCount = "aiplatform.googleapis.com/publisher/online_serving/model_invocation_count"
	metricPeriodSeconds   = int64(86400) // 1 day
)

// FetchUsage retrieves Vertex AI model usage from Cloud Monitoring metrics.
func (c *Client) FetchUsage(ctx context.Context, startTime, endTime time.Time) ([]UsageBucket, error) {
	interval := &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(startTime),
		EndTime:   timestamppb.New(endTime),
	}
	agg := &monitoringpb.Aggregation{
		AlignmentPeriod:  &durationpb.Duration{Seconds: metricPeriodSeconds},
		PerSeriesAligner: monitoringpb.Aggregation_ALIGN_SUM,
	}

	tokenSeries, err := c.mon.ListTimeSeries(ctx, &monitoringpb.ListTimeSeriesRequest{
		Name:        "projects/" + c.projectID,
		Filter:      `metric.type = "` + metricTokenCount + `"`,
		Interval:    interval,
		Aggregation: agg,
		View:        monitoringpb.ListTimeSeriesRequest_FULL,
	})
	if err != nil {
		return nil, &Error{Op: "ListTimeSeries", Message: "fetch token counts", Err: err}
	}

	invocationSeries, err := c.mon.ListTimeSeries(ctx, &monitoringpb.ListTimeSeriesRequest{
		Name:        "projects/" + c.projectID,
		Filter:      `metric.type = "` + metricInvocationCount + `"`,
		Interval:    interval,
		Aggregation: agg,
		View:        monitoringpb.ListTimeSeriesRequest_FULL,
	})
	if err != nil {
		return nil, &Error{Op: "ListTimeSeries", Message: "fetch invocation counts", Err: err}
	}

	return aggregateResults(tokenSeries, invocationSeries), nil
}

// aggregateResults merges token and invocation time series into UsageBuckets.
func aggregateResults(tokenSeries, invocationSeries []*monitoringpb.TimeSeries) []UsageBucket {
	type bucketKey struct {
		model     string
		timestamp time.Time
	}
	type bucketData struct {
		inputTokens  int
		outputTokens int
		requests     int
	}

	agg := make(map[bucketKey]*bucketData)

	for _, ts := range tokenSeries {
		model := extractModel(ts)
		tokenType := ""
		if ts.Metric != nil {
			tokenType = ts.Metric.Labels["type"]
		}

		for _, pt := range ts.Points {
			if pt.Interval == nil || pt.Interval.EndTime == nil {
				continue
			}
			key := bucketKey{
				model:     model,
				timestamp: pt.Interval.EndTime.AsTime(),
			}
			d, ok := agg[key]
			if !ok {
				d = &bucketData{}
				agg[key] = d
			}
			val := int(pt.Value.GetInt64Value())
			switch tokenType {
			case "input":
				d.inputTokens += val
			case "output":
				d.outputTokens += val
			}
		}
	}

	for _, ts := range invocationSeries {
		model := extractModel(ts)
		for _, pt := range ts.Points {
			if pt.Interval == nil || pt.Interval.EndTime == nil {
				continue
			}
			key := bucketKey{
				model:     model,
				timestamp: pt.Interval.EndTime.AsTime(),
			}
			d, ok := agg[key]
			if !ok {
				d = &bucketData{}
				agg[key] = d
			}
			d.requests += int(pt.Value.GetInt64Value())
		}
	}

	buckets := make([]UsageBucket, 0, len(agg))
	for key, data := range agg {
		buckets = append(buckets, UsageBucket{
			StartTime:    key.timestamp.Add(-time.Duration(metricPeriodSeconds) * time.Second),
			EndTime:      key.timestamp,
			Model:        key.model,
			InputTokens:  data.inputTokens,
			OutputTokens: data.outputTokens,
			Requests:     data.requests,
		})
	}
	return buckets
}

// extractModel builds a model identifier from resource labels.
func extractModel(ts *monitoringpb.TimeSeries) string {
	if ts.Resource == nil {
		return ""
	}
	publisher := ts.Resource.Labels["publisher"]
	modelID := ts.Resource.Labels["model_user_id"]
	if publisher != "" && modelID != "" {
		return publisher + "/" + modelID
	}
	if modelID != "" {
		return modelID
	}
	return publisher
}
