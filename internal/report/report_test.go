package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/aispectre/internal/analyzer"
)

var update = flag.Bool("update", false, "update golden files")

var testTime = time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

func testReport() Report {
	return Report{
		Findings: []analyzer.Finding{
			{
				Kind:         analyzer.KindKeyUnused,
				Severity:     analyzer.SeverityHigh,
				Platform:     "openai",
				Title:        "Unused API key: old-key",
				Description:  `API key "old-key" has never been used`,
				MonthlyWaste: 0,
			},
			{
				Kind:         analyzer.KindDowngradeAvailable,
				Severity:     analyzer.SeverityMedium,
				Platform:     "openai",
				Model:        "gpt-4",
				Title:        "Cheaper alternative available for gpt-4",
				Description:  "Consider gpt-4o",
				MonthlyWaste: 80,
				Evidence: map[string]any{
					"downgrade_to": "gpt-4o",
					"savings":      float64(80),
				},
			},
		},
		Platform:    "openai",
		Window:      30,
		GeneratedAt: testTime,
	}
}

// --- Reporter factory tests ---

func TestNewReporter(t *testing.T) {
	for _, format := range []string{"text", "json", "spectrehub", "sarif"} {
		r, err := NewReporter(format)
		if err != nil {
			t.Errorf("NewReporter(%q) error: %v", format, err)
		}
		if r == nil {
			t.Errorf("NewReporter(%q) returned nil", format)
		}
	}
}

func TestNewReporterUnknown(t *testing.T) {
	_, err := NewReporter("xml")
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

// --- spectre/v1 tests ---

func TestSpectreHubGoldenFile(t *testing.T) {
	report := testReport()
	var buf bytes.Buffer
	r := &spectreHubReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}

	goldenPath := "testdata/spectre_v1.golden.json"
	if *update {
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("updated golden file")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}

	// Normalize: compare parsed JSON to avoid whitespace differences.
	var gotJSON, wantJSON any
	if err := json.Unmarshal(buf.Bytes(), &gotJSON); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if err := json.Unmarshal(golden, &wantJSON); err != nil {
		t.Fatalf("invalid golden file JSON: %v", err)
	}

	gotNorm, _ := json.MarshalIndent(gotJSON, "", "  ")
	wantNorm, _ := json.MarshalIndent(wantJSON, "", "  ")
	if string(gotNorm) != string(wantNorm) {
		t.Errorf("spectre/v1 output does not match golden file.\ngot:\n%s\nwant:\n%s", gotNorm, wantNorm)
	}
}

func TestSpectreHubSchema(t *testing.T) {
	report := testReport()
	var buf bytes.Buffer
	r := &spectreHubReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		Schema string `json:"$schema"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != "spectre/v1" {
		t.Errorf("$schema = %q, want spectre/v1", envelope.Schema)
	}
}

func TestSpectreHubSummary(t *testing.T) {
	report := testReport()
	var buf bytes.Buffer
	r := &spectreHubReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		Summary struct {
			TotalFindings   int            `json:"total_findings"`
			EstMonthlyWaste float64        `json:"estimated_monthly_waste"`
			BySeverity      map[string]int `json:"by_severity"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Summary.TotalFindings != 2 {
		t.Errorf("total_findings = %d, want 2", envelope.Summary.TotalFindings)
	}
	if envelope.Summary.EstMonthlyWaste != 80 {
		t.Errorf("estimated_monthly_waste = %f, want 80", envelope.Summary.EstMonthlyWaste)
	}
	if envelope.Summary.BySeverity["high"] != 1 {
		t.Errorf("by_severity.high = %d, want 1", envelope.Summary.BySeverity["high"])
	}
}

func TestSpectreHubEmpty(t *testing.T) {
	report := Report{
		Platform:    "openai",
		Window:      30,
		GeneratedAt: testTime,
	}
	var buf bytes.Buffer
	r := &spectreHubReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		Summary struct {
			TotalFindings   int     `json:"total_findings"`
			EstMonthlyWaste float64 `json:"estimated_monthly_waste"`
		} `json:"summary"`
		Findings []json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if envelope.Summary.TotalFindings != 0 {
		t.Errorf("total_findings = %d, want 0", envelope.Summary.TotalFindings)
	}
	if len(envelope.Findings) != 0 {
		t.Errorf("findings count = %d, want 0", len(envelope.Findings))
	}
}

// --- Text tests ---

func TestTextReport(t *testing.T) {
	report := testReport()
	var buf bytes.Buffer
	r := &textReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Verify header.
	if !strings.Contains(out, "aispectre scan report") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "Platform: openai") {
		t.Error("missing platform")
	}
	if !strings.Contains(out, "Window: 30 days") {
		t.Error("missing window")
	}

	// Verify severity sections.
	if !strings.Contains(out, "--- HIGH (1) ---") {
		t.Error("missing HIGH section")
	}
	if !strings.Contains(out, "--- MEDIUM (1) ---") {
		t.Error("missing MEDIUM section")
	}

	// Verify finding details.
	if !strings.Contains(out, "[KEY_UNUSED]") {
		t.Error("missing KEY_UNUSED finding")
	}
	if !strings.Contains(out, "[DOWNGRADE_AVAILABLE]") {
		t.Error("missing DOWNGRADE_AVAILABLE finding")
	}
}

func TestTextReportEmpty(t *testing.T) {
	report := Report{
		Platform:    "openai",
		Window:      30,
		GeneratedAt: testTime,
	}
	var buf bytes.Buffer
	r := &textReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No findings.") {
		t.Error("expected 'No findings.' for empty report")
	}
}

func TestTextSavingsTable(t *testing.T) {
	report := testReport()
	var buf bytes.Buffer
	r := &textReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Savings Opportunities") {
		t.Error("missing savings table header")
	}
	if !strings.Contains(out, "gpt-4o") {
		t.Error("missing downgrade target in savings table")
	}
}

func TestTextNoSavingsTable(t *testing.T) {
	report := Report{
		Findings: []analyzer.Finding{
			{
				Kind:     analyzer.KindKeyUnused,
				Severity: analyzer.SeverityHigh,
				Platform: "openai",
				Title:    "Unused key",
			},
		},
		Platform:    "openai",
		Window:      30,
		GeneratedAt: testTime,
	}
	var buf bytes.Buffer
	r := &textReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Savings Opportunities") {
		t.Error("savings table should not appear without DOWNGRADE_AVAILABLE findings")
	}
}

// --- JSON tests ---

func TestJSONReport(t *testing.T) {
	report := testReport()
	var buf bytes.Buffer
	r := &jsonReporter{}
	if err := r.Render(&buf, report); err != nil {
		t.Fatal(err)
	}

	// Round-trip: unmarshal and verify.
	var jr jsonReport
	if err := json.Unmarshal(buf.Bytes(), &jr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if jr.Platform != "openai" {
		t.Errorf("platform = %s, want openai", jr.Platform)
	}
	if jr.Window != 30 {
		t.Errorf("window_days = %d, want 30", jr.Window)
	}
	if len(jr.Findings) != 2 {
		t.Errorf("findings count = %d, want 2", len(jr.Findings))
	}
	if jr.TotalWaste != 80 {
		t.Errorf("estimated_monthly_waste = %f, want 80", jr.TotalWaste)
	}
}

// --- SARIF tests ---

func TestSARIFUnsupported(t *testing.T) {
	r := &sarifReporter{}
	err := r.Render(nil, Report{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got: %v", err)
	}
}

// --- TotalWaste tests ---

func TestTotalWaste(t *testing.T) {
	report := testReport()
	got := report.TotalWaste()
	if got != 80 {
		t.Errorf("TotalWaste() = %f, want 80", got)
	}
}

func TestTotalWasteEmpty(t *testing.T) {
	report := Report{}
	got := report.TotalWaste()
	if got != 0 {
		t.Errorf("TotalWaste() = %f, want 0", got)
	}
}
