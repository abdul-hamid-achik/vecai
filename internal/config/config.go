package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Provider represents the LLM provider
type Provider string

const (
	ProviderOllama Provider = "ollama"
)

// ModelTier represents the model capability level
type ModelTier string

const (
	TierFast   ModelTier = "fast"   // Fast model (llama3.2:3b)
	TierSmart  ModelTier = "smart"  // Smart model (qwen3:8b)
	TierGenius ModelTier = "genius" // Genius model (qwen3:14b)
)

// OllamaConfig holds Ollama-specific configuration
type OllamaConfig struct {
	BaseURL     string `yaml:"base_url"`     // Default: "http://localhost:11434"
	ModelFast   string `yaml:"model_fast"`   // Default: "qwen3:8b"
	ModelSmart  string `yaml:"model_smart"`  // Default: "qwen2.5-coder:7b"
	ModelGenius string `yaml:"model_genius"` // Default: "cogito:14b"
	KeepAlive   string `yaml:"keep_alive"`   // Default: "5m"
}

// AgentConfig holds multi-agent configuration
type AgentConfig struct {
	MaxRetries          int  `yaml:"max_retries"`          // Max retries per step (default: 3)
	VerificationEnabled bool `yaml:"verification_enabled"` // Enable verification agent (default: true)
}

// MemoryConfig holds memory layer configuration
type MemoryConfig struct {
	Enabled    bool   `yaml:"enabled"`     // Enable memory layer (default: true)
	ProjectDir string `yaml:"project_dir"` // Per-project memory (default: ".vecai/memory")
	GlobalDir  string `yaml:"global_dir"`  // Global memory (default: "~/.config/vecai/memory")
}

// RateLimitConfig holds rate limiting configuration (kept for backward compatibility)
type RateLimitConfig struct {
	MaxRetries         int           `yaml:"max_retries"`          // Maximum retries on error
	BaseDelay          time.Duration `yaml:"base_delay"`           // Base delay for exponential backoff
	MaxDelay           time.Duration `yaml:"max_delay"`            // Maximum delay between retries
	TokensPerMinute    int           `yaml:"tokens_per_minute"`    // Rate limit (unused for Ollama)
	EnableRateLimiting bool          `yaml:"enable_rate_limiting"` // Enable rate limiting (unused for Ollama)
}

// ContextConfig holds context management configuration
type ContextConfig struct {
	AutoCompactThreshold float64 `yaml:"auto_compact_threshold"` // Trigger auto-compact at this % (default: 0.95)
	WarnThreshold        float64 `yaml:"warn_threshold"`         // Show warning at this % (default: 0.80)
	PreserveLast         int     `yaml:"preserve_last"`          // Messages to preserve during compact (default: 4)
	EnableAutoCompact    bool    `yaml:"enable_auto_compact"`    // Enable auto-compaction (default: true)
	ContextWindow        int     `yaml:"context_window"`         // Context window size in tokens (qwen3:8b=32K, cogito:14b=128K)
}

// AnalysisConfig holds configuration for token-efficient analysis mode
type AnalysisConfig struct {
	Enabled            bool `yaml:"enabled"`              // Enable analysis mode by default
	MaxFileTokens      int  `yaml:"max_file_tokens"`      // Max tokens per file read (default: 2000)
	AggressiveCompact  bool `yaml:"aggressive_compaction"` // Use aggressive compaction thresholds
	SmartToolSelection bool `yaml:"smart_tool_selection"` // Enable on-demand tool loading
}

// ToolsConfig holds configuration for all tools
type ToolsConfig struct {
	Vecgrep VecgrepToolConfig `yaml:"vecgrep"`
	Noted   NotedToolConfig   `yaml:"noted"`
	Gpeek   GpeekToolConfig   `yaml:"gpeek"`
}

// VecgrepToolConfig holds vecgrep-specific configuration
type VecgrepToolConfig struct {
	Enabled      bool   `yaml:"enabled"`       // Enable vecgrep tools (default: true)
	DefaultMode  string `yaml:"default_mode"`  // Default search mode: "hybrid", "semantic", "keyword"
	DefaultLimit int    `yaml:"default_limit"` // Default result limit (default: 10)
}

// NotedToolConfig holds noted-specific configuration
type NotedToolConfig struct {
	Enabled          bool `yaml:"enabled"`            // Enable noted tools (default: true if installed)
	IncludeInContext bool `yaml:"include_in_context"` // Include notes in context enrichment (default: true)
	MaxContextNotes  int  `yaml:"max_context_notes"`  // Max notes to include in context (default: 5)
}

// GpeekToolConfig holds gpeek-specific configuration
type GpeekToolConfig struct {
	Enabled bool `yaml:"enabled"` // Enable gpeek tools (default: true)
}

// Config holds the application configuration
type Config struct {
	Provider    Provider        `yaml:"provider"`    // LLM provider (only "ollama" supported)
	Ollama      OllamaConfig    `yaml:"ollama"`      // Ollama configuration
	Agent       AgentConfig     `yaml:"agent"`       // Multi-agent configuration
	Memory      MemoryConfig    `yaml:"memory"`      // Memory layer configuration
	Tools       ToolsConfig     `yaml:"tools"`       // Tool-specific configuration
	DefaultTier ModelTier       `yaml:"default_tier"`
	MaxTokens   int             `yaml:"max_tokens"`
	Temperature float64         `yaml:"temperature"`
	SkillsDir   string          `yaml:"skills_dir"`
	VecgrepPath string          `yaml:"vecgrep_path"`
	RateLimit   RateLimitConfig `yaml:"rate_limit"` // Kept for backward compat
	Context     ContextConfig   `yaml:"context"`
	Analysis    AnalysisConfig  `yaml:"analysis"`

	// Internal: where config was loaded from
	configPath string
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Provider: ProviderOllama,
		Ollama: OllamaConfig{
			BaseURL:     "http://localhost:11434",
			ModelFast:   "llama3.2:3b",
			ModelSmart:  "qwen3:8b",
			ModelGenius: "qwen3:14b",
			KeepAlive:   "30m",
		},
		Agent: AgentConfig{
			MaxRetries:          3,
			VerificationEnabled: true,
		},
		Memory: MemoryConfig{
			Enabled:    true,
			ProjectDir: ".vecai/memory",
			GlobalDir:  "~/.config/vecai/memory",
		},
		Tools: ToolsConfig{
			Vecgrep: VecgrepToolConfig{
				Enabled:      true,
				DefaultMode:  "hybrid",
				DefaultLimit: 10,
			},
			Noted: NotedToolConfig{
				Enabled:          true,
				IncludeInContext: true,
				MaxContextNotes:  5,
			},
			Gpeek: GpeekToolConfig{
				Enabled: true,
			},
		},
		DefaultTier: TierFast,
		MaxTokens:   8192,
		Temperature: 0.7,
		SkillsDir:   "skills",
		VecgrepPath: "vecgrep",
		RateLimit: RateLimitConfig{
			MaxRetries:         3,
			BaseDelay:          1 * time.Second,
			MaxDelay:           30 * time.Second,
			TokensPerMinute:    0, // Not used for Ollama
			EnableRateLimiting: false,
		},
		Context: ContextConfig{
			AutoCompactThreshold: 0.70,
			WarnThreshold:        0.50,
			PreserveLast:         2,
			EnableAutoCompact:    true,
			ContextWindow:        8192, // Optimized for speed with llama3.2:3b
		},
		Analysis: AnalysisConfig{
			Enabled:            false,
			MaxFileTokens:      2000,
			AggressiveCompact:  true,
			SmartToolSelection: true,
		},
	}
}

// LoadOptions contains options for loading configuration
type LoadOptions struct {
	BaseURLOverride string // Override Ollama base URL
	ModelOverride   string // Override default model
}

// Load loads configuration from files and environment
func Load() (*Config, error) {
	return LoadWithOptions(LoadOptions{})
}

// LoadWithOptions loads configuration with the given options
func LoadWithOptions(opts LoadOptions) (*Config, error) {
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

	// Apply overrides from environment
	// OLLAMA_HOST can be: "0.0.0.0", "0.0.0.0:11434", or "http://localhost:11434"
	if hostEnv := os.Getenv("OLLAMA_HOST"); hostEnv != "" {
		cfg.Ollama.BaseURL = normalizeOllamaHost(hostEnv)
	}

	// Apply CLI overrides
	if opts.BaseURLOverride != "" {
		cfg.Ollama.BaseURL = opts.BaseURLOverride
	}
	if opts.ModelOverride != "" {
		cfg.Ollama.ModelFast = opts.ModelOverride
		cfg.Ollama.ModelSmart = opts.ModelOverride
		cfg.Ollama.ModelGenius = opts.ModelOverride
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

// GetModel returns the Ollama model ID for a tier
func (c *Config) GetModel(tier ModelTier) string {
	switch tier {
	case TierFast:
		return c.Ollama.ModelFast
	case TierSmart:
		return c.Ollama.ModelSmart
	case TierGenius:
		return c.Ollama.ModelGenius
	default:
		return c.Ollama.ModelSmart
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

// normalizeOllamaHost converts OLLAMA_HOST env var to a proper URL
// Handles: "0.0.0.0", "0.0.0.0:11434", "localhost:11434", "http://localhost:11434"
func normalizeOllamaHost(host string) string {
	// Already a URL
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}

	// If it's 0.0.0.0 (bind all interfaces), use localhost for client connections
	if host == "0.0.0.0" || strings.HasPrefix(host, "0.0.0.0:") {
		if strings.Contains(host, ":") {
			parts := strings.SplitN(host, ":", 2)
			return "http://localhost:" + parts[1]
		}
		return "http://localhost:11434"
	}

	// Add default port if not specified
	if !strings.Contains(host, ":") {
		host = host + ":11434"
	}

	return "http://" + host
}
