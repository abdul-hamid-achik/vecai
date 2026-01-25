package permissions

import (
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// mockInput implements InputHandler for testing
type mockInput struct {
	response string
}

func (m *mockInput) ReadLine(prompt string) (string, error) {
	return m.response, nil
}

// mockOutput implements OutputHandler for testing
type mockOutput struct {
	lastPrompt string
}

func (m *mockOutput) PermissionPrompt(toolName string, level tools.PermissionLevel, description string) {
	m.lastPrompt = toolName
}

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeAsk, "ask"},
		{ModeAuto, "auto"},
		{ModeStrict, "strict"},
		{ModeAnalysis, "analysis"},
		{Mode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("Mode.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPolicy_ModeAuto(t *testing.T) {
	policy := NewPolicy(ModeAuto, &mockInput{}, &mockOutput{})

	// Auto mode should allow everything
	tests := []struct {
		name  string
		level tools.PermissionLevel
	}{
		{"read_file", tools.PermissionRead},
		{"write_file", tools.PermissionWrite},
		{"bash", tools.PermissionExecute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := policy.Check(tt.name, tt.level, "test")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !allowed {
				t.Errorf("ModeAuto should allow %s", tt.name)
			}
		})
	}
}

func TestPolicy_ModeAnalysis(t *testing.T) {
	policy := NewPolicy(ModeAnalysis, &mockInput{}, &mockOutput{})

	tests := []struct {
		name     string
		level    tools.PermissionLevel
		expected bool
	}{
		{"read_file", tools.PermissionRead, true},
		{"vecgrep_search", tools.PermissionRead, true},
		{"list_files", tools.PermissionRead, true},
		{"write_file", tools.PermissionWrite, false},
		{"edit_file", tools.PermissionWrite, false},
		{"bash", tools.PermissionExecute, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := policy.Check(tt.name, tt.level, "test")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if allowed != tt.expected {
				t.Errorf("ModeAnalysis Check(%s, %s) = %v, want %v",
					tt.name, tt.level, allowed, tt.expected)
			}
		})
	}
}

func TestPolicy_ModeAsk_AutoApproveReads(t *testing.T) {
	policy := NewPolicy(ModeAsk, &mockInput{response: "n"}, &mockOutput{})

	// Read operations should be auto-approved (no prompt)
	allowed, err := policy.Check("read_file", tools.PermissionRead, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("ModeAsk should auto-approve reads")
	}
}

func TestPolicy_ModeAsk_PromptForWrite(t *testing.T) {
	output := &mockOutput{}
	policy := NewPolicy(ModeAsk, &mockInput{response: "y"}, output)

	allowed, err := policy.Check("write_file", tools.PermissionWrite, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected write to be allowed when user says yes")
	}
	if output.lastPrompt != "write_file" {
		t.Error("expected permission prompt for write")
	}
}

func TestPolicy_ModeAsk_DenyOnNo(t *testing.T) {
	policy := NewPolicy(ModeAsk, &mockInput{response: "n"}, &mockOutput{})

	allowed, err := policy.Check("write_file", tools.PermissionWrite, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected write to be denied when user says no")
	}
}

func TestPolicy_CacheAlwaysAllow(t *testing.T) {
	policy := NewPolicy(ModeAsk, &mockInput{response: "a"}, &mockOutput{})

	// First call should prompt and cache "always"
	allowed1, _ := policy.Check("bash", tools.PermissionExecute, "test")
	if !allowed1 {
		t.Error("expected first call to allow")
	}

	// Second call should use cache (even with "n" response)
	policy2 := NewPolicy(ModeAsk, &mockInput{response: "n"}, &mockOutput{})
	policy2.cache = policy.cache // Share cache

	decision, ok := policy2.GetCachedDecision("bash")
	if !ok {
		t.Error("expected cached decision")
	}
	if decision != DecisionAlwaysAllow {
		t.Errorf("expected DecisionAlwaysAllow, got %v", decision)
	}
}

func TestPolicy_CacheNeverAllow(t *testing.T) {
	policy := NewPolicy(ModeAsk, &mockInput{response: "v"}, &mockOutput{})

	// First call should prompt and cache "never"
	allowed1, _ := policy.Check("bash", tools.PermissionExecute, "test")
	if allowed1 {
		t.Error("expected first call to deny")
	}

	decision, ok := policy.GetCachedDecision("bash")
	if !ok {
		t.Error("expected cached decision")
	}
	if decision != DecisionNeverAllow {
		t.Errorf("expected DecisionNeverAllow, got %v", decision)
	}
}

func TestPolicy_ClearCache(t *testing.T) {
	policy := NewPolicy(ModeAsk, &mockInput{response: "a"}, &mockOutput{})

	_, _ = policy.Check("bash", tools.PermissionExecute, "test")

	_, ok := policy.GetCachedDecision("bash")
	if !ok {
		t.Error("expected cached decision before clear")
	}

	policy.ClearCache()

	_, ok = policy.GetCachedDecision("bash")
	if ok {
		t.Error("expected no cached decision after clear")
	}
}

func TestPolicy_GetMode(t *testing.T) {
	policy := NewPolicy(ModeAnalysis, &mockInput{}, &mockOutput{})
	if policy.GetMode() != ModeAnalysis {
		t.Errorf("GetMode() = %v, want %v", policy.GetMode(), ModeAnalysis)
	}
}

func TestPolicy_SetMode(t *testing.T) {
	policy := NewPolicy(ModeAsk, &mockInput{}, &mockOutput{})
	policy.SetMode(ModeAuto)
	if policy.GetMode() != ModeAuto {
		t.Errorf("after SetMode(ModeAuto), GetMode() = %v", policy.GetMode())
	}
}
