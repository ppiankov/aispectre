package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/ppiankov/aispectre/internal/analyzer"
)

type textReporter struct{}

func (r *textReporter) Render(w io.Writer, report Report) error {
	p := &printer{w: w}

	p.println("aispectre scan report")
	p.printf("Generated: %s\n", report.GeneratedAt.UTC().Format("2006-01-02"))
	p.printf("Platform: %s | Window: %d days\n", report.Platform, report.Window)

	counts := severityCounts(report)
	p.printf("Findings: %d | Estimated monthly waste: $%.2f\n",
		len(report.Findings), report.TotalWaste())

	if len(report.Findings) == 0 {
		p.println("\nNo findings.")
		return p.err
	}

	// Group findings by severity, output in descending order.
	for _, sev := range []analyzer.Severity{
		analyzer.SeverityHigh,
		analyzer.SeverityMedium,
		analyzer.SeverityLow,
		analyzer.SeverityInfo,
	} {
		count := counts[sev]
		if count == 0 {
			continue
		}
		p.printf("\n--- %s (%d) ---\n", sevLabel(sev), count)
		for _, f := range report.Findings {
			if f.Severity != sev {
				continue
			}
			p.printf("\n[%s] %s\n", f.Kind, f.Title)
			p.printf("  %s\n", f.Description)
			meta := fmt.Sprintf("  Platform: %s", f.Platform)
			if f.Model != "" {
				meta += fmt.Sprintf(" | Model: %s", f.Model)
			}
			if f.MonthlyWaste > 0 {
				meta += fmt.Sprintf(" | Est. monthly waste: $%.2f", f.MonthlyWaste)
			}
			p.println(meta)
		}
	}

	// Savings opportunities table (DOWNGRADE_AVAILABLE findings only).
	var downgrades []analyzer.Finding
	for _, f := range report.Findings {
		if f.Kind == analyzer.KindDowngradeAvailable {
			downgrades = append(downgrades, f)
		}
	}
	if len(downgrades) > 0 {
		p.print("\n--- Savings Opportunities ---\n\n")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "MODEL\tDOWNGRADE TO\tEST. SAVINGS/MO"); err != nil {
			return err
		}
		for _, f := range downgrades {
			downTo, _ := f.Evidence["downgrade_to"].(string)
			if _, err := fmt.Fprintf(tw, "%s\t%s\t$%.2f\n", f.Model, downTo, f.MonthlyWaste); err != nil {
				return err
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return p.err
}

func sevLabel(s analyzer.Severity) string {
	switch s {
	case analyzer.SeverityHigh:
		return "HIGH"
	case analyzer.SeverityMedium:
		return "MEDIUM"
	case analyzer.SeverityLow:
		return "LOW"
	default:
		return "INFO"
	}
}

// printer captures the first write error to simplify text rendering.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

func (p *printer) println(s string) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintln(p.w, s)
}

func (p *printer) print(s string) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprint(p.w, s)
}
