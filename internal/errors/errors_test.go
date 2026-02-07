package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestVecaiError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *VecaiError
		contains []string
	}{
		{
			name: "with cause",
			err: &VecaiError{
				Category: CategoryLLM,
				Code:     "llm_unavailable",
				Message:  "LLM service is unavailable",
				Cause:    fmt.Errorf("connection refused"),
			},
			contains: []string{"[llm]", "llm_unavailable", "LLM service is unavailable", "connection refused"},
		},
		{
			name: "without cause",
			err: &VecaiError{
				Category: CategoryTool,
				Code:     "tool_not_found",
				Message:  "tool \"foo\" not found",
			},
			contains: []string{"[tool]", "tool_not_found", "tool \"foo\" not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			for _, s := range tt.contains {
				if !strings.Contains(msg, s) {
					t.Errorf("Error() = %q, want it to contain %q", msg, s)
				}
			}
		})
	}
}

func TestVecaiError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := &VecaiError{
		Category: CategoryLLM,
		Code:     "test",
		Message:  "test error",
		Cause:    cause,
	}

	if err.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
	}

	// Nil cause
	errNoCause := &VecaiError{
		Category: CategoryLLM,
		Code:     "test",
		Message:  "test error",
	}
	if errNoCause.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", errNoCause.Unwrap())
	}
}

func TestVecaiError_UnwrapChain(t *testing.T) {
	root := fmt.Errorf("disk full")
	mid := &VecaiError{
		Category: CategoryConfig,
		Code:     "config_load_failed",
		Message:  "failed to load config",
		Cause:    root,
	}
	outer := fmt.Errorf("startup failed: %w", mid)

	if !errors.Is(outer, root) {
		t.Error("expected errors.Is to find root cause through chain")
	}

	var ve *VecaiError
	if !errors.As(outer, &ve) {
		t.Error("expected errors.As to find VecaiError in chain")
	}
	if ve.Code != "config_load_failed" {
		t.Errorf("got code %q, want %q", ve.Code, "config_load_failed")
	}
}

func TestVecaiError_Is(t *testing.T) {
	err1 := &VecaiError{Category: CategoryLLM, Code: "llm_unavailable", Message: "a"}
	err2 := &VecaiError{Category: CategoryLLM, Code: "llm_unavailable", Message: "b"}
	err3 := &VecaiError{Category: CategoryLLM, Code: "llm_timeout", Message: "c"}
	err4 := &VecaiError{Category: CategoryTool, Code: "llm_unavailable", Message: "d"}

	if !errors.Is(err1, err2) {
		t.Error("expected Is() to match same category+code regardless of message")
	}
	if errors.Is(err1, err3) {
		t.Error("expected Is() to not match different codes")
	}
	if errors.Is(err1, err4) {
		t.Error("expected Is() to not match different categories")
	}

	// Non-VecaiError target
	if errors.Is(err1, fmt.Errorf("not a vecai error")) {
		t.Error("expected Is() to return false for non-VecaiError target")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable VecaiError",
			err:  LLMUnavailable(nil),
			want: true,
		},
		{
			name: "non-retryable VecaiError",
			err:  LLMModelNotFound("test-model"),
			want: false,
		},
		{
			name: "wrapped retryable",
			err:  fmt.Errorf("outer: %w", LLMRequestFailed(nil)),
			want: true,
		},
		{
			name: "non-VecaiError",
			err:  fmt.Errorf("plain error"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCategory(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Category
	}{
		{
			name: "LLM error",
			err:  LLMUnavailable(nil),
			want: CategoryLLM,
		},
		{
			name: "tool error",
			err:  ToolNotFound("bash"),
			want: CategoryTool,
		},
		{
			name: "agent error",
			err:  MaxIterationsReached(20),
			want: CategoryAgent,
		},
		{
			name: "wrapped error",
			err:  fmt.Errorf("wrap: %w", ConfigLoadFailed("config.yaml", nil)),
			want: CategoryConfig,
		},
		{
			name: "non-VecaiError",
			err:  fmt.Errorf("plain"),
			want: "",
		},
		{
			name: "nil",
			err:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCategory(tt.err); got != tt.want {
				t.Errorf("GetCategory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetUserMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "VecaiError returns Message field",
			err:  LLMModelNotFound("llama3"),
			want: "model \"llama3\" not found - run 'ollama pull llama3'",
		},
		{
			name: "wrapped VecaiError",
			err:  fmt.Errorf("wrap: %w", ToolNotFound("missing")),
			want: "tool \"missing\" not found",
		},
		{
			name: "plain error",
			err:  fmt.Errorf("something broke"),
			want: "something broke",
		},
		{
			name: "nil",
			err:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUserMessage(tt.err); got != tt.want {
				t.Errorf("GetUserMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConstructors(t *testing.T) {
	t.Run("LLMUnavailable", func(t *testing.T) {
		cause := fmt.Errorf("connection refused")
		err := LLMUnavailable(cause)
		assertError(t, err, CategoryLLM, "llm_unavailable", true, cause)
	})

	t.Run("LLMModelNotFound", func(t *testing.T) {
		err := LLMModelNotFound("llama3")
		assertError(t, err, CategoryLLM, "llm_model_not_found", false, nil)
		if !strings.Contains(err.Message, "llama3") {
			t.Errorf("Message should contain model name, got %q", err.Message)
		}
	})

	t.Run("LLMRequestFailed", func(t *testing.T) {
		cause := fmt.Errorf("500")
		err := LLMRequestFailed(cause)
		assertError(t, err, CategoryLLM, "llm_request_failed", true, cause)
	})

	t.Run("LLMTimeout", func(t *testing.T) {
		cause := fmt.Errorf("deadline exceeded")
		err := LLMTimeout(cause)
		assertError(t, err, CategoryLLM, "llm_timeout", true, cause)
	})

	t.Run("ToolNotFound", func(t *testing.T) {
		err := ToolNotFound("missing_tool")
		assertError(t, err, CategoryTool, "tool_not_found", false, nil)
		if !strings.Contains(err.Message, "missing_tool") {
			t.Errorf("Message should contain tool name, got %q", err.Message)
		}
	})

	t.Run("ToolExecutionFailed_with_retryable_cause", func(t *testing.T) {
		cause := LLMUnavailable(nil) // retryable
		err := ToolExecutionFailed("bash", cause)
		assertError(t, err, CategoryTool, "tool_execution_failed", true, cause)
	})

	t.Run("ToolExecutionFailed_with_non_retryable_cause", func(t *testing.T) {
		cause := fmt.Errorf("syntax error")
		err := ToolExecutionFailed("bash", cause)
		assertError(t, err, CategoryTool, "tool_execution_failed", false, cause)
	})

	t.Run("ToolPermissionDenied", func(t *testing.T) {
		err := ToolPermissionDenied("write_file")
		assertError(t, err, CategoryPermission, "tool_permission_denied", false, nil)
		if !strings.Contains(err.Message, "write_file") {
			t.Errorf("Message should contain tool name, got %q", err.Message)
		}
	})

	t.Run("MaxIterationsReached", func(t *testing.T) {
		err := MaxIterationsReached(20)
		assertError(t, err, CategoryAgent, "max_iterations_reached", false, nil)
		if !strings.Contains(err.Message, "20") {
			t.Errorf("Message should contain iteration count, got %q", err.Message)
		}
	})

	t.Run("PipelineStepFailed_with_retryable_cause", func(t *testing.T) {
		cause := LLMRequestFailed(nil) // retryable
		err := PipelineStepFailed("step-1", cause)
		assertError(t, err, CategoryAgent, "pipeline_step_failed", true, cause)
	})

	t.Run("PipelineStepFailed_with_non_retryable_cause", func(t *testing.T) {
		cause := fmt.Errorf("parse error")
		err := PipelineStepFailed("step-1", cause)
		assertError(t, err, CategoryAgent, "pipeline_step_failed", false, cause)
	})

	t.Run("ConfigLoadFailed", func(t *testing.T) {
		cause := fmt.Errorf("file not found")
		err := ConfigLoadFailed("/etc/vecai.yaml", cause)
		assertError(t, err, CategoryConfig, "config_load_failed", false, cause)
		if !strings.Contains(err.Message, "/etc/vecai.yaml") {
			t.Errorf("Message should contain path, got %q", err.Message)
		}
	})

	t.Run("ContextWindowExceeded", func(t *testing.T) {
		err := ContextWindowExceeded(130000, 128000)
		assertError(t, err, CategoryContext, "context_window_exceeded", false, nil)
		if !strings.Contains(err.Message, "130000") || !strings.Contains(err.Message, "128000") {
			t.Errorf("Message should contain token counts, got %q", err.Message)
		}
	})
}

func assertError(t *testing.T, err *VecaiError, category Category, code string, retryable bool, cause error) {
	t.Helper()
	if err.Category != category {
		t.Errorf("Category = %q, want %q", err.Category, category)
	}
	if err.Code != code {
		t.Errorf("Code = %q, want %q", err.Code, code)
	}
	if err.Retryable != retryable {
		t.Errorf("Retryable = %v, want %v", err.Retryable, retryable)
	}
	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
	if err.Message == "" {
		t.Error("Message should not be empty")
	}
}
