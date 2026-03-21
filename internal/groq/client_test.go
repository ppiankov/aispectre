package groq

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c, err := NewClient(Config{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestFetchUsageReturnsUnsupported(t *testing.T) {
	c, _ := NewClient(Config{})
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	buckets, err := c.FetchUsage(context.Background(), ts, ts.Add(24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("error = %v, want ErrUnsupported", err)
	}
	if buckets != nil {
		t.Errorf("got %d buckets, want nil", len(buckets))
	}
}
