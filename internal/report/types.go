package report

import (
	"fmt"
	"io"
	"time"

	"github.com/ppiankov/aispectre/internal/analyzer"
)

// Report holds all data needed by reporters.
type Report struct {
	Findings    []analyzer.Finding
	Platform    string
	Window      int
	GeneratedAt time.Time
}

// TotalWaste sums MonthlyWaste across all findings.
func (r Report) TotalWaste() float64 {
	var total float64
	for _, f := range r.Findings {
		total += f.MonthlyWaste
	}
	return total
}

// Reporter formats a Report for output.
type Reporter interface {
	Render(w io.Writer, report Report) error
}

// NewReporter returns a Reporter for the given format string.
func NewReporter(format string) (Reporter, error) {
	switch format {
	case "text":
		return &textReporter{}, nil
	case "json":
		return &jsonReporter{}, nil
	case "spectrehub":
		return &spectreHubReporter{}, nil
	case "sarif":
		return &sarifReporter{}, nil
	default:
		return nil, fmt.Errorf("unknown output format: %q", format)
	}
}
