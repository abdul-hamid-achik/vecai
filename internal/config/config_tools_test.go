package config

import (
	"testing"
)

func TestToolsConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// Verify Tools config exists
	if cfg.Tools.Vecgrep.Enabled != true {
		t.Error("expected Vecgrep.Enabled to default to true")
	}
	if cfg.Tools.Vecgrep.DefaultMode != "hybrid" {
		t.Errorf("expected Vecgrep.DefaultMode to be 'hybrid', got %q", cfg.Tools.Vecgrep.DefaultMode)
	}
	if cfg.Tools.Vecgrep.DefaultLimit != 10 {
		t.Errorf("expected Vecgrep.DefaultLimit to be 10, got %d", cfg.Tools.Vecgrep.DefaultLimit)
	}

	if cfg.Tools.Noted.Enabled != true {
		t.Error("expected Noted.Enabled to default to true")
	}
	if cfg.Tools.Noted.IncludeInContext != true {
		t.Error("expected Noted.IncludeInContext to default to true")
	}
	if cfg.Tools.Noted.MaxContextNotes != 5 {
		t.Errorf("expected Noted.MaxContextNotes to be 5, got %d", cfg.Tools.Noted.MaxContextNotes)
	}

	if cfg.Tools.Gpeek.Enabled != true {
		t.Error("expected Gpeek.Enabled to default to true")
	}
}

func TestToolsConfigStructure(t *testing.T) {
	// Verify the struct fields exist and are the right types
	cfg := ToolsConfig{
		Vecgrep: VecgrepToolConfig{
			Enabled:      false,
			DefaultMode:  "semantic",
			DefaultLimit: 5,
		},
		Noted: NotedToolConfig{
			Enabled:          false,
			IncludeInContext: false,
			MaxContextNotes:  3,
		},
		Gpeek: GpeekToolConfig{
			Enabled: false,
		},
	}

	if cfg.Vecgrep.Enabled != false {
		t.Error("failed to set Vecgrep.Enabled")
	}
	if cfg.Vecgrep.DefaultMode != "semantic" {
		t.Error("failed to set Vecgrep.DefaultMode")
	}
	if cfg.Vecgrep.DefaultLimit != 5 {
		t.Error("failed to set Vecgrep.DefaultLimit")
	}
	if cfg.Noted.Enabled != false {
		t.Error("failed to set Noted.Enabled")
	}
	if cfg.Noted.IncludeInContext != false {
		t.Error("failed to set Noted.IncludeInContext")
	}
	if cfg.Noted.MaxContextNotes != 3 {
		t.Error("failed to set Noted.MaxContextNotes")
	}
	if cfg.Gpeek.Enabled != false {
		t.Error("failed to set Gpeek.Enabled")
	}
}
