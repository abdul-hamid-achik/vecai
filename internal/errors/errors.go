package errors

import (
	"errors"
	"fmt"
)

// Category groups errors by subsystem
type Category string

const (
	CategoryLLM        Category = "llm"
	CategoryTool       Category = "tool"
	CategoryAgent      Category = "agent"
	CategoryConfig     Category = "config"
	CategoryPermission Category = "permission"
	CategoryContext    Category = "context"
)

// VecaiError is the structured error type for the project
type VecaiError struct {
	Category  Category
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *VecaiError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Category, e.Code, e.Message)
}

func (e *VecaiError) Unwrap() error {
	return e.Cause
}

func (e *VecaiError) Is(target error) bool {
	t, ok := target.(*VecaiError)
	if !ok {
		return false
	}
	return e.Code == t.Code && e.Category == t.Category
}

// IsRetryable checks whether an error is retryable.
// Returns false for nil errors or non-VecaiError types.
func IsRetryable(err error) bool {
	var ve *VecaiError
	if errors.As(err, &ve) {
		return ve.Retryable
	}
	return false
}

// GetCategory extracts the error category from a VecaiError.
// Returns an empty Category for nil errors or non-VecaiError types.
func GetCategory(err error) Category {
	var ve *VecaiError
	if errors.As(err, &ve) {
		return ve.Category
	}
	return ""
}

// GetUserMessage returns a user-friendly message for the error.
// For VecaiError it returns the Message field; for other errors it returns Error().
func GetUserMessage(err error) string {
	if err == nil {
		return ""
	}
	var ve *VecaiError
	if errors.As(err, &ve) {
		return ve.Message
	}
	return err.Error()
}
