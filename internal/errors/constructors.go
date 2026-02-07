package errors

import "fmt"

// LLMUnavailable creates an error for when the LLM backend is unreachable.
func LLMUnavailable(cause error) *VecaiError {
	return &VecaiError{
		Category:  CategoryLLM,
		Code:      "llm_unavailable",
		Message:   "LLM service is unavailable",
		Retryable: true,
		Cause:     cause,
	}
}

// LLMModelNotFound creates an error for when a requested model does not exist.
func LLMModelNotFound(model string) *VecaiError {
	return &VecaiError{
		Category:  CategoryLLM,
		Code:      "llm_model_not_found",
		Message:   fmt.Sprintf("model %q not found - run 'ollama pull %s'", model, model),
		Retryable: false,
	}
}

// LLMRequestFailed creates an error for when an LLM request fails.
func LLMRequestFailed(cause error) *VecaiError {
	return &VecaiError{
		Category:  CategoryLLM,
		Code:      "llm_request_failed",
		Message:   "LLM request failed",
		Retryable: true,
		Cause:     cause,
	}
}

// LLMTimeout creates an error for when an LLM request times out.
func LLMTimeout(cause error) *VecaiError {
	return &VecaiError{
		Category:  CategoryLLM,
		Code:      "llm_timeout",
		Message:   "LLM request timed out",
		Retryable: true,
		Cause:     cause,
	}
}

// ToolNotFound creates an error for when a requested tool does not exist.
func ToolNotFound(name string) *VecaiError {
	return &VecaiError{
		Category:  CategoryTool,
		Code:      "tool_not_found",
		Message:   fmt.Sprintf("tool %q not found", name),
		Retryable: false,
	}
}

// ToolExecutionFailed creates an error for when a tool execution fails.
// Retryability depends on the underlying cause.
func ToolExecutionFailed(name string, cause error) *VecaiError {
	return &VecaiError{
		Category:  CategoryTool,
		Code:      "tool_execution_failed",
		Message:   fmt.Sprintf("tool %q execution failed", name),
		Retryable: IsRetryable(cause),
		Cause:     cause,
	}
}

// ToolPermissionDenied creates an error for when tool access is denied.
func ToolPermissionDenied(name string) *VecaiError {
	return &VecaiError{
		Category:  CategoryPermission,
		Code:      "tool_permission_denied",
		Message:   fmt.Sprintf("permission denied for tool %q", name),
		Retryable: false,
	}
}

// MaxIterationsReached creates an error for when the agent loop exceeds its iteration limit.
func MaxIterationsReached(iterations int) *VecaiError {
	return &VecaiError{
		Category:  CategoryAgent,
		Code:      "max_iterations_reached",
		Message:   fmt.Sprintf("agent loop exceeded %d iterations", iterations),
		Retryable: false,
	}
}

// PipelineStepFailed creates an error for when a pipeline step fails.
// Retryability depends on the underlying cause.
func PipelineStepFailed(stepID string, cause error) *VecaiError {
	return &VecaiError{
		Category:  CategoryAgent,
		Code:      "pipeline_step_failed",
		Message:   fmt.Sprintf("pipeline step %q failed", stepID),
		Retryable: IsRetryable(cause),
		Cause:     cause,
	}
}

// ConfigLoadFailed creates an error for when configuration loading fails.
func ConfigLoadFailed(path string, cause error) *VecaiError {
	return &VecaiError{
		Category:  CategoryConfig,
		Code:      "config_load_failed",
		Message:   fmt.Sprintf("failed to load config from %q", path),
		Retryable: false,
		Cause:     cause,
	}
}

// ContextWindowExceeded creates an error for when the context window is exceeded.
func ContextWindowExceeded(used, max int) *VecaiError {
	return &VecaiError{
		Category:  CategoryContext,
		Code:      "context_window_exceeded",
		Message:   fmt.Sprintf("context window exceeded: %d/%d tokens used", used, max),
		Retryable: false,
	}
}
