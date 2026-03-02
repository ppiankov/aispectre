package bedrock

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

// CloudWatchAPI is the subset of the CloudWatch client used by this package.
type CloudWatchAPI interface {
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
	ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
}

// Client wraps CloudWatch API calls for Bedrock usage metrics.
type Client struct {
	cw     CloudWatchAPI
	region string
}

// NewClient constructs a Bedrock usage client using the AWS default credential chain.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("bedrock: region is required")
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}
	if cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(cfg.Profile))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load aws config: %w", err)
	}

	return &Client{
		cw:     cloudwatch.NewFromConfig(awsCfg),
		region: cfg.Region,
	}, nil
}

// newClientWithAPI constructs a Client with a custom CloudWatchAPI (for testing).
func newClientWithAPI(api CloudWatchAPI, region string) *Client {
	return &Client{cw: api, region: region}
}
