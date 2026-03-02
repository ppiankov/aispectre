package report

import (
	"encoding/json"
	"io"
)

type jsonReporter struct{}

// jsonReport is the JSON-serializable form of a Report.
type jsonReport struct {
	GeneratedAt string        `json:"generated_at"`
	Platform    string        `json:"platform"`
	Window      int           `json:"window_days"`
	TotalWaste  float64       `json:"estimated_monthly_waste"`
	Findings    []jsonFinding `json:"findings"`
}

type jsonFinding struct {
	Kind            string         `json:"kind"`
	Severity        string         `json:"severity"`
	Platform        string         `json:"platform"`
	Model           string         `json:"model,omitempty"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	EstMonthlyWaste float64        `json:"estimated_monthly_waste"`
	Evidence        map[string]any `json:"evidence,omitempty"`
}

func (r *jsonReporter) Render(w io.Writer, report Report) error {
	findings := make([]jsonFinding, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, jsonFinding{
			Kind:            string(f.Kind),
			Severity:        f.Severity.String(),
			Platform:        f.Platform,
			Model:           f.Model,
			Title:           f.Title,
			Description:     f.Description,
			EstMonthlyWaste: f.MonthlyWaste,
			Evidence:        f.Evidence,
		})
	}

	jr := jsonReport{
		GeneratedAt: report.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Platform:    report.Platform,
		Window:      report.Window,
		TotalWaste:  report.TotalWaste(),
		Findings:    findings,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}
