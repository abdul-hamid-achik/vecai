package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/logger"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Message represents a conversation message
type Message struct {
	Role    string
	Content string
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
	Thinking   string
	StopReason string
}

// StreamChunk represents a chunk of streamed response
type StreamChunk struct {
	Type      string // "text", "thinking", "tool_call", "done"
	Text      string
	ToolCall  *ToolCall
	Error     error
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
}

// Client wraps the Anthropic SDK
type Client struct {
	client *anthropic.Client
	config *config.Config
	model  string
}

// NewClient creates a new LLM client
func NewClient(cfg *config.Config) *Client {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithMaxRetries(cfg.RateLimit.MaxRetries),
	}
	client := anthropic.NewClient(opts...)
	return &Client{
		client: &client,
		config: cfg,
		model:  cfg.GetDefaultModel(),
	}
}

// SetModel changes the current model
func (c *Client) SetModel(model string) {
	c.model = model
}

// SetTier changes the model tier
func (c *Client) SetTier(tier config.ModelTier) {
	c.model = c.config.GetModel(tier)
}

// GetModel returns the current model
func (c *Client) GetModel() string {
	return c.model
}

// Chat sends a message and returns the response
func (c *Client) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
	logger.Debug("Chat: sending request with %d messages, %d tools", len(messages), len(tools))

	params := c.buildParams(messages, tools, systemPrompt)

	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		logger.Error("Chat: API error: %v", err)
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	logger.Debug("Chat: received response with stop_reason=%s", msg.StopReason)
	return c.parseResponse(msg), nil
}

// ChatStream sends a message and streams the response
func (c *Client) ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 100)

	go func() {
		defer close(ch)

		params := c.buildParams(messages, tools, systemPrompt)
		stream := c.client.Messages.NewStreaming(ctx, params)

		var currentToolCall *ToolCall
		var toolInputJSON string

		for stream.Next() {
			event := stream.Current()

			switch e := event.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				switch block := e.ContentBlock.AsAny().(type) {
				case anthropic.TextBlock:
					// Text block started
				case anthropic.ToolUseBlock:
					currentToolCall = &ToolCall{
						ID:   block.ID,
						Name: block.Name,
					}
					toolInputJSON = ""
				case anthropic.ThinkingBlock:
					// Thinking block started
				}

			case anthropic.ContentBlockDeltaEvent:
				switch delta := e.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					ch <- StreamChunk{Type: "text", Text: delta.Text}
				case anthropic.InputJSONDelta:
					toolInputJSON += delta.PartialJSON
				case anthropic.ThinkingDelta:
					ch <- StreamChunk{Type: "thinking", Text: delta.Thinking}
				}

			case anthropic.ContentBlockStopEvent:
				if currentToolCall != nil {
					// Parse the tool input JSON
					input, err := parseToolInput(toolInputJSON)
					if err != nil {
						ch <- StreamChunk{Type: "error", Error: fmt.Errorf("failed to parse tool input: %w", err)}
					} else {
						currentToolCall.Input = input
						ch <- StreamChunk{Type: "tool_call", ToolCall: currentToolCall}
					}
					currentToolCall = nil
					toolInputJSON = ""
				}

			case anthropic.MessageStopEvent:
				ch <- StreamChunk{Type: "done"}
			}
		}

		if err := stream.Err(); err != nil {
			logger.Error("ChatStream: stream error: %v", err)
			ch <- StreamChunk{Type: "error", Error: err}
		}
	}()

	return ch
}

func (c *Client) buildParams(messages []Message, tools []ToolDefinition, systemPrompt string) anthropic.MessageNewParams {

	// Convert messages
	var apiMessages []anthropic.MessageParam
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			apiMessages = append(apiMessages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		case "assistant":
			apiMessages = append(apiMessages, anthropic.NewAssistantMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: int64(c.config.MaxTokens),
		Messages:  apiMessages,
	}

	// Add system prompt
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: systemPrompt,
			},
		}
	}

	// Add tools
	if len(tools) > 0 {
		var apiTools []anthropic.ToolUnionParam
		for _, tool := range tools {
			schema := buildInputSchema(tool.InputSchema)
			toolParam := anthropic.ToolUnionParamOfTool(schema, tool.Name)
			// Set description
			toolParam.OfTool.Description = anthropic.String(tool.Description)

			apiTools = append(apiTools, toolParam)
		}
		params.Tools = apiTools
	}

	return params
}

func (c *Client) parseResponse(msg *anthropic.Message) *Response {
	resp := &Response{
		StopReason: string(msg.StopReason),
	}

	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			resp.Content += b.Text
		case anthropic.ToolUseBlock:
			var input map[string]any
			if err := json.Unmarshal(b.Input, &input); err != nil {
				logger.Warn("failed to parse tool input for %s: %v", b.Name, err)
				input = make(map[string]any) // Use empty map on error
			}
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    b.ID,
				Name:  b.Name,
				Input: input,
			})
		case anthropic.ThinkingBlock:
			resp.Thinking += b.Thinking
		}
	}

	return resp
}

func parseToolInput(jsonStr string) (map[string]any, error) {
	if jsonStr == "" || jsonStr == "{}" {
		return map[string]any{}, nil
	}

	// Use the SDK's JSON handling
	var result map[string]any
	// Simple JSON parsing for tool inputs
	if err := unmarshalJSON([]byte(jsonStr), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// buildInputSchema converts a tool's schema map to the SDK's ToolInputSchemaParam
func buildInputSchema(schema map[string]any) anthropic.ToolInputSchemaParam {
	result := anthropic.ToolInputSchemaParam{}

	// Extract properties
	if props, ok := schema["properties"].(map[string]any); ok {
		result.Properties = props
	}

	// Extract required fields via ExtraFields
	if req, ok := schema["required"]; ok {
		result.ExtraFields = map[string]interface{}{
			"required": req,
		}
	}

	return result
}
