package bedrock

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	bedrockNamespace   = "AWS/Bedrock"
	metricInvocations  = "Invocations"
	metricInputTokens  = "InputTokenCount"
	metricOutputTokens = "OutputTokenCount"
	metricPeriod       = int32(86400) // 1 day in seconds
)

// FetchUsage retrieves Bedrock model usage from CloudWatch metrics.
func (c *Client) FetchUsage(ctx context.Context, startTime, endTime time.Time) ([]UsageBucket, error) {
	modelIDs, err := c.listModelIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(modelIDs) == 0 {
		return nil, nil
	}

	queries := buildQueries(modelIDs)

	results, err := c.getMetricData(ctx, queries, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return aggregateResults(modelIDs, results), nil
}

func (c *Client) listModelIDs(ctx context.Context) ([]string, error) {
	var modelIDs []string
	seen := make(map[string]bool)

	input := &cloudwatch.ListMetricsInput{
		Namespace:  aws.String(bedrockNamespace),
		MetricName: aws.String(metricInvocations),
	}

	for {
		out, err := c.cw.ListMetrics(ctx, input)
		if err != nil {
			return nil, &Error{Op: "ListMetrics", Message: "list bedrock metrics", Err: err}
		}
		for _, m := range out.Metrics {
			for _, d := range m.Dimensions {
				if aws.ToString(d.Name) == "ModelId" {
					id := aws.ToString(d.Value)
					if !seen[id] {
						seen[id] = true
						modelIDs = append(modelIDs, id)
					}
				}
			}
		}
		if out.NextToken == nil {
			break
		}
		input.NextToken = out.NextToken
	}
	return modelIDs, nil
}

func buildQueries(modelIDs []string) []cwtypes.MetricDataQuery {
	var queries []cwtypes.MetricDataQuery
	for i, modelID := range modelIDs {
		dim := []cwtypes.Dimension{{Name: aws.String("ModelId"), Value: aws.String(modelID)}}
		base := i * 3

		for j, name := range []string{metricInvocations, metricInputTokens, metricOutputTokens} {
			prefix := []string{"inv", "inp", "out"}[j]
			queries = append(queries, cwtypes.MetricDataQuery{
				Id: aws.String(fmt.Sprintf("%s_%d", prefix, base+j)),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  aws.String(bedrockNamespace),
						MetricName: aws.String(name),
						Dimensions: dim,
					},
					Period: aws.Int32(metricPeriod),
					Stat:   aws.String("Sum"),
				},
			})
		}
	}
	return queries
}

func (c *Client) getMetricData(ctx context.Context, queries []cwtypes.MetricDataQuery, startTime, endTime time.Time) ([]cwtypes.MetricDataResult, error) {
	var all []cwtypes.MetricDataResult

	input := &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
		StartTime:         aws.Time(startTime),
		EndTime:           aws.Time(endTime),
	}

	for {
		out, err := c.cw.GetMetricData(ctx, input)
		if err != nil {
			return nil, &Error{Op: "GetMetricData", Message: "fetch bedrock metrics", Err: err}
		}
		all = append(all, out.MetricDataResults...)

		if out.NextToken == nil {
			break
		}
		input.NextToken = out.NextToken
	}
	return all, nil
}

// aggregateResults groups metric data results by model and timestamp into UsageBuckets.
func aggregateResults(modelIDs []string, results []cwtypes.MetricDataResult) []UsageBucket {
	// Build lookup: query ID → MetricDataResult
	lookup := make(map[string]*cwtypes.MetricDataResult, len(results))
	for i := range results {
		if results[i].Id != nil {
			lookup[*results[i].Id] = &results[i]
		}
	}

	var buckets []UsageBucket
	for i, modelID := range modelIDs {
		base := i * 3
		invID := fmt.Sprintf("inv_%d", base)
		inpID := fmt.Sprintf("inp_%d", base+1)
		outID := fmt.Sprintf("out_%d", base+2)

		invResult := lookup[invID]
		if invResult == nil || len(invResult.Timestamps) == 0 {
			continue
		}

		for j, ts := range invResult.Timestamps {
			b := UsageBucket{
				StartTime: ts,
				EndTime:   ts.Add(time.Duration(metricPeriod) * time.Second),
				ModelID:   modelID,
			}
			if j < len(invResult.Values) {
				b.Invocations = int(invResult.Values[j])
			}
			if r := lookup[inpID]; r != nil && j < len(r.Values) {
				b.InputTokens = int(r.Values[j])
			}
			if r := lookup[outID]; r != nil && j < len(r.Values) {
				b.OutputTokens = int(r.Values[j])
			}
			buckets = append(buckets, b)
		}
	}
	return buckets
}
