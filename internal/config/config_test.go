package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTier != TierSmart {
		t.Errorf("expected default tier %s, got %s", TierSmart, cfg.DefaultTier)
	}

	if cfg.MaxTokens != 8192 {
		t.Errorf("expected max tokens 8192, got %d", cfg.MaxTokens)
	}

	if cfg.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", cfg.Temperature)
	}

	if cfg.SkillsDir != "skills" {
		t.Errorf("expected skills dir 'skills', got %s", cfg.SkillsDir)
	}

	if cfg.VecgrepPath != "vecgrep" {
		t.Errorf("expected vecgrep path 'vecgrep', got %s", cfg.VecgrepPath)
	}
}

func TestGetModel(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		tier     ModelTier
		expected string
	}{
		{TierFast, "claude-haiku-4-5-20251015"},
		{TierSmart, "claude-sonnet-4-5-20250929"},
		{TierGenius, "claude-opus-4-5-20251101"},
		{ModelTier("unknown"), "claude-sonnet-4-5-20250929"}, // default fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			model := cfg.GetModel(tt.tier)
			if model != tt.expected {
				t.Errorf("GetModel(%s) = %s, want %s", tt.tier, model, tt.expected)
			}
		})
	}
}

func TestGetDefaultModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultTier = TierFast

	expected := "claude-haiku-4-5-20251015"
	if got := cfg.GetDefaultModel(); got != expected {
		t.Errorf("GetDefaultModel() = %s, want %s", got, expected)
	}
}

func TestSetTier(t *testing.T) {
	cfg := DefaultConfig()

	cfg.SetTier(TierGenius)
	if cfg.DefaultTier != TierGenius {
		t.Errorf("SetTier(TierGenius) failed, got %s", cfg.DefaultTier)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := `default_tier: fast
max_tokens: 4096
temperature: 0.5
skills_dir: custom_skills
vecgrep_path: /usr/local/bin/vecgrep
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := cfg.loadFromFile(configPath); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}

	if cfg.DefaultTier != TierFast {
		t.Errorf("expected tier fast, got %s", cfg.DefaultTier)
	}

	if cfg.MaxTokens != 4096 {
		t.Errorf("expected max tokens 4096, got %d", cfg.MaxTokens)
	}

	if cfg.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", cfg.Temperature)
	}

	if cfg.SkillsDir != "custom_skills" {
		t.Errorf("expected skills dir 'custom_skills', got %s", cfg.SkillsDir)
	}
}

func TestLoadRequiresAPIKey(t *testing.T) {
	// Unset API key
	original := os.Getenv("ANTHROPIC_API_KEY")
	_ = os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() { _ = os.Setenv("ANTHROPIC_API_KEY", original) }()

	_, err := Load()
	if err == nil {
		t.Error("expected error when ANTHROPIC_API_KEY is not set")
	}
}

func TestLoadWithAPIKey(t *testing.T) {
	// Set API key
	original := os.Getenv("ANTHROPIC_API_KEY")
	_ = os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer func() { _ = os.Setenv("ANTHROPIC_API_KEY", original) }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %s", cfg.APIKey)
	}
}

func TestConfigPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.configPath = "/test/path/config.yaml"

	if got := cfg.ConfigPath(); got != "/test/path/config.yaml" {
		t.Errorf("ConfigPath() = %s, want /test/path/config.yaml", got)
	}
}
