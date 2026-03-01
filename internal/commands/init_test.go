package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, ".aispectre.yaml") {
		t.Error("output should mention .aispectre.yaml")
	}

	data, err := os.ReadFile(filepath.Join(dir, ".aispectre.yaml"))
	if err != nil {
		t.Fatalf("expected .aispectre.yaml to exist: %v", err)
	}
	if len(data) == 0 {
		t.Error(".aispectre.yaml is empty")
	}
	if !strings.Contains(string(data), "openai") {
		t.Error("sample config should mention openai")
	}
}

func TestInitSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	existing := "custom: true\n"
	_ = os.WriteFile(filepath.Join(dir, ".aispectre.yaml"), []byte(existing), 0o644)

	cmd := newRootCmd(testBuildInfo)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(errBuf.String(), "skip") {
		t.Error("should report skipping existing file")
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".aispectre.yaml"))
	if string(data) != existing {
		t.Errorf("existing file was overwritten: %q", string(data))
	}
}

func TestInitAllExist(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(origDir) }()

	_ = os.WriteFile(filepath.Join(dir, ".aispectre.yaml"), []byte("# existing\n"), 0o644)

	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "Nothing to do") {
		t.Error("should report nothing to do when all files exist")
	}
}

func TestInitHelp(t *testing.T) {
	cmd := newRootCmd(testBuildInfo)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "starter") {
		t.Error("init help should mention starter config")
	}
}
