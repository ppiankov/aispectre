package groq

// Client is a stub for Groq usage data.
type Client struct{}

// NewClient constructs a Groq client stub.
func NewClient(_ Config) (*Client, error) {
	return &Client{}, nil
}
