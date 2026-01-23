package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ModelTier represents the model capability level
type ModelTier string

const (
	TierFast   ModelTier = "fast"   // Claude Haiku
	TierSmart  ModelTier = "smart"  // Claude Sonnet
	TierGenius ModelTier = "genius" // Claude Opus
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	MaxRetries         int           `yaml:"max_retries"`          // Maximum retries on 429
	BaseDelay          time.Duration `yaml:"base_delay"`           // Base delay for exponential backoff
	MaxDelay           time.Duration `yaml:"max_delay"`            // Maximum delay between retries
	TokensPerMinute    int           `yaml:"tokens_per_minute"`    // Rate limit (tokens/minute)
	EnableRateLimiting bool          `yaml:"enable_rate_limiting"` // Enable proactive rate limiting
}

// ContextConfig holds context management configuration
type ContextConfig struct {
	AutoCompactThreshold float64 `yaml:"auto_compact_threshold"` // Trigger auto-compact at this % (default: 0.95)
	WarnThreshold        float64 `yaml:"warn_threshold"`         // Show warning at this % (default: 0.80)
	PreserveLast         int     `yaml:"preserve_last"`          // Messages to preserve during compact (default: 4)
	EnableAutoCompact    bool    `yaml:"enable_auto_compact"`    // Enable auto-compaction (default: true)
	ContextWindow        int     `yaml:"context_window"`         // Context window size in tokens (default: 200000)
}

// Config holds the application configuration
type Config struct {
	APIKey      string          `yaml:"-"` // From environment only
	DefaultTier ModelTier       `yaml:"default_tier"`
	MaxTokens   int             `yaml:"max_tokens"`
	Temperature float64         `yaml:"temperature"`
	SkillsDir   string          `yaml:"skills_dir"`
	VecgrepPath string          `yaml:"vecgrep_path"`
	RateLimit   RateLimitConfig `yaml:"rate_limit"`
	Context     ContextConfig   `yaml:"context"`

	// Internal: where config was loaded from
	configPath string
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		DefaultTier: TierFast,
		MaxTokens:   8192,
		Temperature: 0.7,
		SkillsDir:   "skills",
		VecgrepPath: "vecgrep",
		RateLimit: RateLimitConfig{
			MaxRetries:         5,
			BaseDelay:          1 * time.Second,
			MaxDelay:           60 * time.Second,
			TokensPerMinute:    30000,
			EnableRateLimiting: true,
		},
		Context: ContextConfig{
			AutoCompactThreshold: 0.95,
			WarnThreshold:        0.80,
			PreserveLast:         4,
			EnableAutoCompact:    true,
			ContextWindow:        200000,
		},
	}
}

// Load loads configuration from files and environment
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from config files in priority order
	configPaths := getConfigPaths()
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			if err := cfg.loadFromFile(path); err != nil {
				return nil, fmt.Errorf("error loading config from %s: %w", path, err)
			}
			cfg.configPath = path
			break
		}
	}

	// If no config found, create default
	if cfg.configPath == "" {
		if err := cfg.createDefault(); err != nil {
			// Non-fatal: just use defaults
			fmt.Fprintf(os.Stderr, "Warning: could not create default config: %v\n", err)
		}
	}

	// Load API key from environment
	cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
	}

	return cfg, nil
}

// getConfigPaths returns config file paths in priority order
func getConfigPaths() []string {
	paths := []string{
		"vecai.yaml",
		".vecai/config.yaml",
	}

	// Add user config directory
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "vecai", "config.yaml"))
	}

	return paths
}

// loadFromFile loads config from a YAML file
func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

// createDefault creates a default config file
func (c *Config) createDefault() error {
	// Prefer .vecai/config.yaml in current directory
	dir := ".vecai"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "config.yaml")
	c.configPath = path

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	content := "# vecai configuration\n# See: https://github.com/abdul-hamid-achik/vecai\n\n" + string(data)
	return os.WriteFile(path, []byte(content), 0644)
}

// GetModel returns the Anthropic model ID for a tier
func (c *Config) GetModel(tier ModelTier) string {
	switch tier {
	case TierFast:
		return "claude-haiku-4-5-20251015"
	case TierSmart:
		return "claude-sonnet-4-5-20250929"
	case TierGenius:
		return "claude-opus-4-5-20251101"
	default:
		return "claude-sonnet-4-5-20250929"
	}
}

// GetDefaultModel returns the model ID for the default tier
func (c *Config) GetDefaultModel() string {
	return c.GetModel(c.DefaultTier)
}

// SetTier updates the default tier
func (c *Config) SetTier(tier ModelTier) {
	c.DefaultTier = tier
}

// ConfigPath returns where the config was loaded from
func (c *Config) ConfigPath() string {
	return c.configPath
}
