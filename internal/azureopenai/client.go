package azureopenai

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
)

// MetricsAPI is the subset of the Azure Monitor MetricsClient used by this package.
type MetricsAPI interface {
	QueryResource(ctx context.Context, resourceURI string, options *azquery.MetricsClientQueryResourceOptions) (azquery.MetricsClientQueryResourceResponse, error)
}

// Client wraps Azure Monitor API calls for Azure OpenAI usage metrics.
type Client struct {
	metrics     MetricsAPI
	resourceURI string
}

// NewClient constructs an Azure OpenAI usage client using the Azure default credential chain.
func NewClient(cfg Config) (*Client, error) {
	if cfg.SubscriptionID == "" {
		return nil, fmt.Errorf("azureopenai: subscription_id is required")
	}
	if cfg.ResourceGroup == "" {
		return nil, fmt.Errorf("azureopenai: resource_group is required")
	}
	if cfg.AccountName == "" {
		return nil, fmt.Errorf("azureopenai: account_name is required")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azureopenai: azure credentials: %w", err)
	}

	metricsClient, err := azquery.NewMetricsClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azureopenai: metrics client: %w", err)
	}

	resourceURI := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.CognitiveServices/accounts/%s",
		cfg.SubscriptionID, cfg.ResourceGroup, cfg.AccountName,
	)

	return &Client{
		metrics:     metricsClient,
		resourceURI: resourceURI,
	}, nil
}

// newClientWithAPI constructs a Client with a custom MetricsAPI (for testing).
func newClientWithAPI(api MetricsAPI, resourceURI string) *Client {
	return &Client{metrics: api, resourceURI: resourceURI}
}

// ResourceURI returns the Azure resource URI for this client.
func (c *Client) ResourceURI() string {
	return c.resourceURI
}
