package config

import (
	"os"
	"path/filepath"
	"testing"
)

func noEnv(_ string) string { return "" }

func TestLoadDefaults(t *testing.T) {
	// Load from a temp dir with no config file and no env vars.
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg, err := load(noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Window != 30 {
		t.Errorf("Window = %d, want 30", cfg.Window)
	}
	if cfg.IdleDays != 7 {
		t.Errorf("IdleDays = %d, want 7", cfg.IdleDays)
	}
	if cfg.MinWaste != 1.0 {
		t.Errorf("MinWaste = %f, want 1.0", cfg.MinWaste)
	}
	if cfg.Format != "text" {
		t.Errorf("Format = %s, want text", cfg.Format)
	}
	if len(cfg.EnabledPlatforms()) != 0 {
		t.Errorf("expected no enabled platforms, got %v", cfg.EnabledPlatforms())
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	yamlContent := `
window: 14
idle_days: 3
min_waste: 5.0
format: json
platforms:
  openai:
    token: sk-test-123
    enabled: true
  bedrock:
    region: us-west-2
    profile: dev
    enabled: true
`
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := load(noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Window != 14 {
		t.Errorf("Window = %d, want 14", cfg.Window)
	}
	if cfg.IdleDays != 3 {
		t.Errorf("IdleDays = %d, want 3", cfg.IdleDays)
	}
	if cfg.MinWaste != 5.0 {
		t.Errorf("MinWaste = %f, want 5.0", cfg.MinWaste)
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %s, want json", cfg.Format)
	}
	if cfg.Platforms.OpenAI == nil {
		t.Fatal("expected OpenAI config")
	}
	if cfg.Platforms.OpenAI.Token != "sk-test-123" {
		t.Errorf("OpenAI.Token = %s, want sk-test-123", cfg.Platforms.OpenAI.Token)
	}
	if cfg.Platforms.Bedrock == nil {
		t.Fatal("expected Bedrock config")
	}
	if cfg.Platforms.Bedrock.Region != "us-west-2" {
		t.Errorf("Bedrock.Region = %s, want us-west-2", cfg.Platforms.Bedrock.Region)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg, err := load(noEnv)
	if err != nil {
		t.Fatalf("missing config should not be an error: %v", err)
	}
	if cfg.Window != 30 {
		t.Errorf("Window = %d, want 30 (default)", cfg.Window)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := load(noEnv)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestDetectOpenAI(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		if key == "OPENAI_API_KEY" {
			return "sk-from-env"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.OpenAI == nil {
		t.Fatal("expected OpenAI config from env")
	}
	if cfg.Platforms.OpenAI.Token != "sk-from-env" {
		t.Errorf("Token = %s, want sk-from-env", cfg.Platforms.OpenAI.Token)
	}
	if !cfg.Platforms.OpenAI.Enabled {
		t.Error("expected Enabled = true")
	}
}

func TestDetectAnthropic(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-ant-from-env"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.Anthropic == nil {
		t.Fatal("expected Anthropic config from env")
	}
	if cfg.Platforms.Anthropic.Token != "sk-ant-from-env" {
		t.Errorf("Token = %s, want sk-ant-from-env", cfg.Platforms.Anthropic.Token)
	}
}

func TestDetectBedrock(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		switch key {
		case "AWS_PROFILE":
			return "dev"
		case "AWS_DEFAULT_REGION":
			return "us-east-1"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.Bedrock == nil {
		t.Fatal("expected Bedrock config from env")
	}
	if cfg.Platforms.Bedrock.Profile != "dev" {
		t.Errorf("Profile = %s, want dev", cfg.Platforms.Bedrock.Profile)
	}
	if cfg.Platforms.Bedrock.Region != "us-east-1" {
		t.Errorf("Region = %s, want us-east-1", cfg.Platforms.Bedrock.Region)
	}
}

func TestDetectBedrockAccessKey(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		if key == "AWS_ACCESS_KEY_ID" {
			return "test-access-key"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.Bedrock == nil {
		t.Fatal("expected Bedrock config from AWS_ACCESS_KEY_ID")
	}
	if !cfg.Platforms.Bedrock.Enabled {
		t.Error("expected Enabled = true")
	}
}

func TestDetectVertexAI(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		switch key {
		case "GOOGLE_CLOUD_PROJECT":
			return "my-project"
		case "GOOGLE_CLOUD_REGION":
			return "us-central1"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.VertexAI == nil {
		t.Fatal("expected VertexAI config from env")
	}
	if cfg.Platforms.VertexAI.Project != "my-project" {
		t.Errorf("Project = %s, want my-project", cfg.Platforms.VertexAI.Project)
	}
	if cfg.Platforms.VertexAI.Region != "us-central1" {
		t.Errorf("Region = %s, want us-central1", cfg.Platforms.VertexAI.Region)
	}
}

func TestDetectVertexAIFallbackEnv(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		if key == "GCLOUD_PROJECT" {
			return "fallback-project"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.VertexAI == nil {
		t.Fatal("expected VertexAI config from GCLOUD_PROJECT")
	}
	if cfg.Platforms.VertexAI.Project != "fallback-project" {
		t.Errorf("Project = %s, want fallback-project", cfg.Platforms.VertexAI.Project)
	}
}

func TestDetectCohere(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		if key == "COHERE_API_KEY" {
			return "cohere-key"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.Cohere == nil {
		t.Fatal("expected Cohere config from env")
	}
	if cfg.Platforms.Cohere.Token != "cohere-key" {
		t.Errorf("Token = %s, want cohere-key", cfg.Platforms.Cohere.Token)
	}
}

func TestDetectNoOverrideYAML(t *testing.T) {
	cfg := defaults()
	// Simulate YAML setting OpenAI as disabled.
	cfg.Platforms.OpenAI = &OpenAIConfig{Token: "from-yaml", Enabled: false}

	getenv := func(key string) string {
		if key == "OPENAI_API_KEY" {
			return "from-env"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	// Env should NOT override YAML config.
	if cfg.Platforms.OpenAI.Token != "from-yaml" {
		t.Errorf("Token = %s, want from-yaml (YAML should not be overridden)", cfg.Platforms.OpenAI.Token)
	}
	if cfg.Platforms.OpenAI.Enabled {
		t.Error("Enabled should remain false (YAML takes precedence)")
	}
}

func TestEnabledPlatforms(t *testing.T) {
	cfg := &Config{
		Platforms: PlatformConfigs{
			OpenAI:    &OpenAIConfig{Enabled: true},
			Anthropic: &AnthropicConfig{Enabled: false},
			Bedrock:   &BedrockConfig{Enabled: true},
			VertexAI:  &VertexAIConfig{Enabled: true},
		},
	}
	got := cfg.EnabledPlatforms()
	want := []string{"bedrock", "openai", "vertexai"}
	if len(got) != len(want) {
		t.Fatalf("EnabledPlatforms() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("EnabledPlatforms()[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestEnabledPlatformsEmpty(t *testing.T) {
	cfg := defaults()
	got := cfg.EnabledPlatforms()
	if len(got) != 0 {
		t.Errorf("EnabledPlatforms() = %v, want empty", got)
	}
}

func TestDetectAzureOpenAI(t *testing.T) {
	cfg := defaults()
	getenv := func(key string) string {
		switch key {
		case "AZURE_SUBSCRIPTION_ID":
			return "sub-123"
		case "AZURE_RESOURCE_GROUP":
			return "rg-test"
		case "AZURE_OPENAI_ACCOUNT":
			return "my-account"
		}
		return ""
	}
	detectPlatforms(cfg, getenv)

	if cfg.Platforms.AzureOpenAI == nil {
		t.Fatal("expected AzureOpenAI config from env")
	}
	if cfg.Platforms.AzureOpenAI.SubscriptionID != "sub-123" {
		t.Errorf("SubscriptionID = %s, want sub-123", cfg.Platforms.AzureOpenAI.SubscriptionID)
	}
	if cfg.Platforms.AzureOpenAI.ResourceGroup != "rg-test" {
		t.Errorf("ResourceGroup = %s, want rg-test", cfg.Platforms.AzureOpenAI.ResourceGroup)
	}
	if cfg.Platforms.AzureOpenAI.AccountName != "my-account" {
		t.Errorf("AccountName = %s, want my-account", cfg.Platforms.AzureOpenAI.AccountName)
	}
}
