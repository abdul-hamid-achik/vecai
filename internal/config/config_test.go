package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTier != TierFast {
		t.Errorf("expected default tier %s, got %s", TierFast, cfg.DefaultTier)
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

	if cfg.Provider != ProviderOllama {
		t.Errorf("expected provider 'ollama', got %s", cfg.Provider)
	}

	if cfg.Ollama.BaseURL != "http://localhost:11434" {
		t.Errorf("expected ollama base URL 'http://localhost:11434', got %s", cfg.Ollama.BaseURL)
	}
}

func TestGetModel(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		tier     ModelTier
		expected string
	}{
		{TierFast, "qwen2.5-coder:3b"},
		{TierSmart, "qwen2.5-coder:7b"},
		{TierGenius, "qwen2.5-coder:14b"},
		{ModelTier("unknown"), "qwen2.5-coder:7b"}, // default fallback
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

	expected := "qwen2.5-coder:3b"
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
ollama:
  base_url: "http://localhost:11434"
  model_fast: "llama3:8b"
  model_smart: "codellama:13b"
  model_genius: "mixtral:8x7b"
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

	if cfg.Ollama.ModelFast != "llama3:8b" {
		t.Errorf("expected ollama model_fast 'llama3:8b', got %s", cfg.Ollama.ModelFast)
	}
}

func TestLoadWithOllamaHostEnv(t *testing.T) {
	// Set Ollama host env
	original := os.Getenv("OLLAMA_HOST")
	_ = os.Setenv("OLLAMA_HOST", "http://custom-ollama:11434")
	defer func() {
		if original != "" {
			_ = os.Setenv("OLLAMA_HOST", original)
		} else {
			_ = os.Unsetenv("OLLAMA_HOST")
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Ollama.BaseURL != "http://custom-ollama:11434" {
		t.Errorf("expected Ollama base URL from env, got %s", cfg.Ollama.BaseURL)
	}
}

func TestLoadWithOverrides(t *testing.T) {
	cfg, err := LoadWithOptions(LoadOptions{
		BaseURLOverride: "http://override:11434",
		ModelOverride:   "custom-model:7b",
	})
	if err != nil {
		t.Fatalf("LoadWithOptions failed: %v", err)
	}

	if cfg.Ollama.BaseURL != "http://override:11434" {
		t.Errorf("expected base URL override, got %s", cfg.Ollama.BaseURL)
	}

	if cfg.Ollama.ModelFast != "custom-model:7b" {
		t.Errorf("expected model override, got %s", cfg.Ollama.ModelFast)
	}
}

func TestConfigPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.configPath = "/test/path/config.yaml"

	if got := cfg.ConfigPath(); got != "/test/path/config.yaml" {
		t.Errorf("ConfigPath() = %s, want /test/path/config.yaml", got)
	}
}
