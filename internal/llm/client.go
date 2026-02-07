package llm

import (
	"context"

	"github.com/abdul-hamid-achik/vecai/internal/config"
)

// Message represents a conversation message
type Message struct {
	Role       string
	Content    string
	ToolCallID string     // For tool result messages (role="tool")
	ToolCalls  []ToolCall // For assistant messages with tool calls
}

// ToolCall represents a tool call from the LLM
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// Response represents an LLM response
type Response struct {
	Content    string
	ToolCalls  []ToolCall
	Thinking   string // Note: not used by Ollama, kept for interface compatibility
	StopReason string
}

// Usage represents token usage from API responses
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// StreamChunk represents a chunk of streamed response
type StreamChunk struct {
	Type      string // "text", "thinking", "tool_call", "done", "error"
	Text      string
	ToolCall  *ToolCall
	Error     error
	Usage     *Usage // Token usage (only set on "done" chunks)
}

// ToolDefinition defines a tool for the LLM
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// LLMClient is the interface for LLM clients
type LLMClient interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error)
	ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk
	SetModel(model string)
	SetTier(tier config.ModelTier)
	GetModel() string
	Close() error
}

// Client is an alias for OllamaClient for backward compatibility
type Client = OllamaClient

// NewClient creates a new LLM client (Ollama)
func NewClient(cfg *config.Config) *Client {
	return NewOllamaClient(cfg)
}
