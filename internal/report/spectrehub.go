package report

import (
	"encoding/json"
	"io"

	"github.com/ppiankov/aispectre/internal/analyzer"
)

type spectreHubReporter struct{}

// spectreEnvelope is the spectre/v1 JSON envelope.
type spectreEnvelope struct {
	Schema      string           `json:"$schema"`
	GeneratedAt string           `json:"generated_at"`
	Target      spectreTarget    `json:"target"`
	Summary     spectreSummary   `json:"summary"`
	Findings    []spectreFinding `json:"findings"`
}

type spectreTarget struct {
	Type     string `json:"type"`
	Platform string `json:"platform"`
}

type spectreSummary struct {
	TotalFindings   int            `json:"total_findings"`
	EstMonthlyWaste float64        `json:"estimated_monthly_waste"`
	BySeverity      map[string]int `json:"by_severity"`
}

type spectreFinding struct {
	Kind            string         `json:"kind"`
	Severity        string         `json:"severity"`
	Platform        string         `json:"platform"`
	Model           string         `json:"model,omitempty"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	EstMonthlyWaste float64        `json:"estimated_monthly_waste"`
	Evidence        map[string]any `json:"evidence,omitempty"`
}

func (r *spectreHubReporter) Render(w io.Writer, report Report) error {
	bySeverity := map[string]int{
		"high":   0,
		"medium": 0,
		"low":    0,
		"info":   0,
	}
	findings := make([]spectreFinding, 0, len(report.Findings))
	for _, f := range report.Findings {
		sev := f.Severity.String()
		bySeverity[sev]++
		findings = append(findings, spectreFinding{
			Kind:            string(f.Kind),
			Severity:        sev,
			Platform:        f.Platform,
			Model:           f.Model,
			Title:           f.Title,
			Description:     f.Description,
			EstMonthlyWaste: f.MonthlyWaste,
			Evidence:        f.Evidence,
		})
	}

	envelope := spectreEnvelope{
		Schema:      "spectre/v1",
		GeneratedAt: report.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Target: spectreTarget{
			Type:     targetType(report),
			Platform: report.Platform,
		},
		Summary: spectreSummary{
			TotalFindings:   len(report.Findings),
			EstMonthlyWaste: report.TotalWaste(),
			BySeverity:      bySeverity,
		},
		Findings: findings,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func targetType(_ Report) string {
	return "ai-org"
}

// severityCounts returns finding counts by severity for a Report.
func severityCounts(report Report) map[analyzer.Severity]int {
	counts := make(map[analyzer.Severity]int)
	for _, f := range report.Findings {
		counts[f.Severity]++
	}
	return counts
}
