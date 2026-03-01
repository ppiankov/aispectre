package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

var testBuildInfo = BuildInfo{
	Version:   "1.2.3",
	Commit:    "abc1234",
	Date:      "2026-03-01T00:00:00Z",
	GoVersion: "go1.24.0",
}

func TestVersionCommand(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "aispectre 1.2.3") {
		t.Errorf("version output = %q", output)
	}
	if !strings.Contains(output, "abc1234") {
		t.Errorf("missing commit in version output: %q", output)
	}
	if !strings.Contains(output, "2026-03-01") {
		t.Errorf("missing date in version output: %q", output)
	}
}

func TestVersionCommandJSON(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var info BuildInfo
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if info.Version != "1.2.3" {
		t.Errorf("version = %s, want 1.2.3", info.Version)
	}
	if info.Commit != "abc1234" {
		t.Errorf("commit = %s, want abc1234", info.Commit)
	}
}

func TestRootHelp(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	for _, sub := range []string{"version", "scan", "init"} {
		if !strings.Contains(help, sub) {
			t.Errorf("help missing subcommand %q", sub)
		}
	}
}

func TestRootUnknownCommand(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"bogus"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestExitErrorFormat(t *testing.T) {
	err := &ExitError{Code: 2}
	if err.Error() != "exit status 2" {
		t.Errorf("ExitError.Error() = %q", err.Error())
	}
}
