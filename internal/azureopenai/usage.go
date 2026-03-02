package azureopenai

import (
	"context"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
)

const (
	metricPromptTokens     = "ProcessedPromptTokens"
	metricCompletionTokens = "GeneratedTokens"
	metricRequests         = "AzureOpenAIRequests"
	aggregationInterval    = "P1D"
)

// FetchUsage retrieves Azure OpenAI token usage from Azure Monitor metrics.
func (c *Client) FetchUsage(ctx context.Context, startTime, endTime time.Time) ([]UsageBucket, error) {
	timespan := azquery.NewTimeInterval(startTime, endTime)
	metricNames := strings.Join([]string{metricPromptTokens, metricCompletionTokens, metricRequests}, ",")
	totalAgg := azquery.AggregationTypeTotal

	resp, err := c.metrics.QueryResource(ctx, c.resourceURI, &azquery.MetricsClientQueryResourceOptions{
		Timespan:    &timespan,
		Interval:    ptrTo(aggregationInterval),
		MetricNames: &metricNames,
		Aggregation: []*azquery.AggregationType{&totalAgg},
	})
	if err != nil {
		return nil, &Error{Op: "QueryResource", Message: "query azure openai metrics", Err: err}
	}

	return parseResponse(resp), nil
}

// parseResponse converts Azure Monitor metric response into UsageBuckets.
func parseResponse(resp azquery.MetricsClientQueryResourceResponse) []UsageBucket {
	// Build per-metric time series data keyed by deployment name.
	// Each metric (prompt tokens, completion tokens, requests) may have
	// multiple time series elements, one per dimension combination.
	type tsKey struct {
		deployment string
		model      string
		timestamp  time.Time
	}
	type tsData struct {
		inputTokens  int
		outputTokens int
		requests     int
	}

	agg := make(map[tsKey]*tsData)

	for _, metric := range resp.Value {
		if metric.Name == nil || metric.Name.Value == nil {
			continue
		}
		metricName := *metric.Name.Value

		for _, ts := range metric.TimeSeries {
			deployment, model := extractDimensions(ts.MetadataValues)

			for _, val := range ts.Data {
				if val.TimeStamp == nil {
					continue
				}
				key := tsKey{
					deployment: deployment,
					model:      model,
					timestamp:  *val.TimeStamp,
				}
				d, ok := agg[key]
				if !ok {
					d = &tsData{}
					agg[key] = d
				}

				total := 0
				if val.Total != nil {
					total = int(*val.Total)
				}

				switch metricName {
				case metricPromptTokens:
					d.inputTokens = total
				case metricCompletionTokens:
					d.outputTokens = total
				case metricRequests:
					d.requests = total
				}
			}
		}
	}

	buckets := make([]UsageBucket, 0, len(agg))
	for key, data := range agg {
		buckets = append(buckets, UsageBucket{
			StartTime:    key.timestamp,
			EndTime:      key.timestamp.Add(24 * time.Hour),
			Deployment:   key.deployment,
			Model:        key.model,
			InputTokens:  data.inputTokens,
			OutputTokens: data.outputTokens,
			Requests:     data.requests,
		})
	}
	return buckets
}

// extractDimensions reads ModelDeploymentName and ModelName from metadata values.
func extractDimensions(metadata []*azquery.MetadataValue) (deployment, model string) {
	for _, md := range metadata {
		if md.Name == nil || md.Name.Value == nil || md.Value == nil {
			continue
		}
		switch *md.Name.Value {
		case "ModelDeploymentName":
			deployment = *md.Value
		case "ModelName":
			model = *md.Value
		}
	}
	return deployment, model
}

func ptrTo[T any](v T) *T {
	return &v
}
