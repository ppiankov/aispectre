package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const configFileName = ".aispectre.yaml"

// Config is the top-level aispectre configuration.
type Config struct {
	Platforms PlatformConfigs `yaml:"platforms"`
	Window    int             `yaml:"window"`
	IdleDays  int             `yaml:"idle_days"`
	MinWaste  float64         `yaml:"min_waste"`
	Format    string          `yaml:"format"`
}

// PlatformConfigs holds per-platform configuration. Nil pointer = not configured.
type PlatformConfigs struct {
	OpenAI      *OpenAIConfig      `yaml:"openai"`
	Anthropic   *AnthropicConfig   `yaml:"anthropic"`
	Bedrock     *BedrockConfig     `yaml:"bedrock"`
	AzureOpenAI *AzureOpenAIConfig `yaml:"azureopenai"`
	VertexAI    *VertexAIConfig    `yaml:"vertexai"`
	Cohere      *CohereConfig      `yaml:"cohere"`
}

// OpenAIConfig holds OpenAI platform settings.
type OpenAIConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"`
	Enabled bool   `yaml:"enabled"`
}

// AnthropicConfig holds Anthropic platform settings.
type AnthropicConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"`
	Enabled bool   `yaml:"enabled"`
}

// BedrockConfig holds AWS Bedrock platform settings.
type BedrockConfig struct {
	Profile string `yaml:"profile"`
	Region  string `yaml:"region"`
	Enabled bool   `yaml:"enabled"`
}

// AzureOpenAIConfig holds Azure OpenAI platform settings.
type AzureOpenAIConfig struct {
	SubscriptionID string `yaml:"subscription_id"`
	ResourceGroup  string `yaml:"resource_group"`
	AccountName    string `yaml:"account_name"`
	Enabled        bool   `yaml:"enabled"`
}

// VertexAIConfig holds Google Vertex AI platform settings.
type VertexAIConfig struct {
	Project string `yaml:"project"`
	Region  string `yaml:"region"`
	Enabled bool   `yaml:"enabled"`
}

// CohereConfig holds Cohere platform settings.
type CohereConfig struct {
	Token   string `yaml:"token"`
	Enabled bool   `yaml:"enabled"`
}

// defaults returns a Config with default values.
func defaults() *Config {
	return &Config{
		Window:   30,
		IdleDays: 7,
		MinWaste: 1.0,
		Format:   "text",
	}
}

// Load reads configuration from .aispectre.yaml (cwd then home dir),
// then auto-detects platforms from environment variables.
// Missing config file is not an error.
func Load() (*Config, error) {
	return load(os.Getenv)
}

// load is the internal implementation with injectable env lookup.
func load(getenv func(string) string) (*Config, error) {
	cfg := defaults()

	// Try cwd first, then home dir.
	cwdPath := configFileName
	homePath := ""
	if home, err := os.UserHomeDir(); err == nil {
		homePath = filepath.Join(home, configFileName)
	}

	loaded := false
	for _, path := range []string{cwdPath, homePath} {
		if path == "" {
			continue
		}
		err := loadFromFile(cfg, path)
		if err == nil {
			loaded = true
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("config: %w", err)
		}
	}
	_ = loaded // no-op: missing file is fine

	detectPlatforms(cfg, getenv)
	return cfg, nil
}

// loadFromFile reads and parses a YAML config file into cfg.
func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// detectPlatforms auto-detects available platforms from environment variables.
// It only creates configs for platforms not already configured in YAML.
func detectPlatforms(cfg *Config, getenv func(string) string) {
	if cfg.Platforms.OpenAI == nil {
		if token := getenv("OPENAI_API_KEY"); token != "" {
			cfg.Platforms.OpenAI = &OpenAIConfig{Token: token, Enabled: true}
		}
	}

	if cfg.Platforms.Anthropic == nil {
		if token := getenv("ANTHROPIC_API_KEY"); token != "" {
			cfg.Platforms.Anthropic = &AnthropicConfig{Token: token, Enabled: true}
		}
	}

	if cfg.Platforms.Bedrock == nil {
		profile := getenv("AWS_PROFILE")
		accessKey := getenv("AWS_ACCESS_KEY_ID")
		if profile != "" || accessKey != "" {
			cfg.Platforms.Bedrock = &BedrockConfig{
				Profile: profile,
				Region:  getenv("AWS_DEFAULT_REGION"),
				Enabled: true,
			}
		}
	}

	if cfg.Platforms.AzureOpenAI == nil {
		if sub := getenv("AZURE_SUBSCRIPTION_ID"); sub != "" {
			cfg.Platforms.AzureOpenAI = &AzureOpenAIConfig{
				SubscriptionID: sub,
				ResourceGroup:  getenv("AZURE_RESOURCE_GROUP"),
				AccountName:    getenv("AZURE_OPENAI_ACCOUNT"),
				Enabled:        true,
			}
		}
	}

	if cfg.Platforms.VertexAI == nil {
		project := getenv("GOOGLE_CLOUD_PROJECT")
		if project == "" {
			project = getenv("GCLOUD_PROJECT")
		}
		if project != "" {
			cfg.Platforms.VertexAI = &VertexAIConfig{
				Project: project,
				Region:  getenv("GOOGLE_CLOUD_REGION"),
				Enabled: true,
			}
		}
	}

	if cfg.Platforms.Cohere == nil {
		if token := getenv("COHERE_API_KEY"); token != "" {
			cfg.Platforms.Cohere = &CohereConfig{Token: token, Enabled: true}
		}
	}
}

// EnabledPlatforms returns a sorted list of platform names that are configured and enabled.
func (c *Config) EnabledPlatforms() []string {
	var platforms []string
	if c.Platforms.OpenAI != nil && c.Platforms.OpenAI.Enabled {
		platforms = append(platforms, "openai")
	}
	if c.Platforms.Anthropic != nil && c.Platforms.Anthropic.Enabled {
		platforms = append(platforms, "anthropic")
	}
	if c.Platforms.Bedrock != nil && c.Platforms.Bedrock.Enabled {
		platforms = append(platforms, "bedrock")
	}
	if c.Platforms.AzureOpenAI != nil && c.Platforms.AzureOpenAI.Enabled {
		platforms = append(platforms, "azureopenai")
	}
	if c.Platforms.VertexAI != nil && c.Platforms.VertexAI.Enabled {
		platforms = append(platforms, "vertexai")
	}
	if c.Platforms.Cohere != nil && c.Platforms.Cohere.Enabled {
		platforms = append(platforms, "cohere")
	}
	sort.Strings(platforms)
	return platforms
}
