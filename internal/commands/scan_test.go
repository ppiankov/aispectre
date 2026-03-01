package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestScanHelp(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"scan", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	for _, flag := range []string{"--platform", "--all", "--window", "--idle-days", "--min-waste", "--format", "--token"} {
		if !strings.Contains(help, flag) {
			t.Errorf("scan help missing flag %q", flag)
		}
	}
}

func TestScanDefaultFlags(t *testing.T) {
	cmd := newScanCmd()
	flags := cmd.Flags()

	window, _ := flags.GetInt("window")
	if window != 30 {
		t.Errorf("window default = %d, want 30", window)
	}

	idleDays, _ := flags.GetInt("idle-days")
	if idleDays != 7 {
		t.Errorf("idle-days default = %d, want 7", idleDays)
	}

	minWaste, _ := flags.GetFloat64("min-waste")
	if minWaste != 1.0 {
		t.Errorf("min-waste default = %f, want 1.0", minWaste)
	}

	format, _ := flags.GetString("format")
	if format != "text" {
		t.Errorf("format default = %q, want text", format)
	}
}

func TestScanStubOutput(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"scan"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not yet implemented") {
		t.Errorf("scan output = %q, want stub message", out.String())
	}
}
