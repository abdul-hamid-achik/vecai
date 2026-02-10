package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	vecerr "github.com/abdul-hamid-achik/vecai/internal/errors"
	"github.com/abdul-hamid-achik/vecai/internal/logging"
)

// Ollama-specific errors
var (
	ErrOllamaUnavailable = errors.New("ollama unavailable - run 'ollama serve'")
	ErrModelNotFound     = errors.New("model not found - run 'ollama pull <model>'")
)

// OllamaClient implements LLMClient for Ollama's HTTP API
type OllamaClient struct {
	baseURL    string
	model      string
	modelMu    sync.RWMutex // Protects model field from concurrent access
	config     *config.Config
	httpClient *http.Client
}

// OllamaMessage represents a message in Ollama's format
type OllamaMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []OllamaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OllamaToolCall represents a tool call in Ollama's format (OpenAI-compatible)
type OllamaToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"` // Can be string or object
	} `json:"function"`
}

// OllamaTool represents a tool definition for Ollama (OpenAI-compatible)
type OllamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

// OllamaChatRequest represents a chat request to Ollama
type OllamaChatRequest struct {
	Model     string          `json:"model"`
	Messages  []OllamaMessage `json:"messages"`
	Tools     []OllamaTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
	Options   *OllamaOptions  `json:"options,omitempty"`
	KeepAlive string          `json:"keep_alive,omitempty"`
}

// OllamaOptions represents model options
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
	NumThread   int     `json:"num_thread,omitempty"`
}

// OllamaChatResponse represents a chat response from Ollama
type OllamaChatResponse struct {
	Model      string        `json:"model"`
	Message    OllamaMessage `json:"message"`
	Done       bool          `json:"done"`
	DoneReason string        `json:"done_reason,omitempty"`
	Error      string        `json:"error,omitempty"`

	// Usage info (only set when done=true)
	PromptEvalCount int `json:"prompt_eval_count,omitempty"`
	EvalCount       int `json:"eval_count,omitempty"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(cfg *config.Config) *OllamaClient {
	baseURL := cfg.Ollama.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaClient{
		baseURL: baseURL,
		model:   cfg.GetDefaultModel(),
		config:  cfg,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// SetModel changes the current model (thread-safe)
func (c *OllamaClient) SetModel(model string) {
	c.modelMu.Lock()
	defer c.modelMu.Unlock()
	c.model = model
}

// SetTier changes the model tier (thread-safe)
func (c *OllamaClient) SetTier(tier config.ModelTier) {
	c.modelMu.Lock()
	defer c.modelMu.Unlock()
	c.model = c.config.GetModel(tier)
}

// GetModel returns the current model (thread-safe)
func (c *OllamaClient) GetModel() string {
	c.modelMu.RLock()
	defer c.modelMu.RUnlock()
	return c.model
}

// Close cleans up the OllamaClient, unloading the model and closing idle HTTP connections.
func (c *OllamaClient) Close() error {
	// Best-effort model unload to free VRAM
	c.unloadModel()
	c.httpClient.CloseIdleConnections()
	return nil
}

// unloadModel sends a keep_alive=0 request to Ollama to free VRAM.
func (c *OllamaClient) unloadModel() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	currentModel := c.GetModel()
	request := OllamaChatRequest{
		Model:     currentModel,
		Messages:  []OllamaMessage{},
		Stream:    false,
		KeepAlive: "0",
	}

	body, err := json.Marshal(request)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// CheckHealthWithVersion verifies Ollama is running and returns the version string
func (c *OllamaClient) CheckHealthWithVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/version", nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", ErrOllamaUnavailable
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse version response: %w", err)
	}
	return result.Version, nil
}

// CheckHealth verifies Ollama is running
func (c *OllamaClient) CheckHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/version", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return vecerr.LLMUnavailable(ErrOllamaUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return vecerr.LLMUnavailable(ErrOllamaUnavailable)
	}

	return nil
}

// Chat sends a message and returns the response
func (c *OllamaClient) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
	// Per-request timeout for non-streaming calls
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Snapshot model name under lock for consistent use throughout this call
	currentModel := c.GetModel()

	log := logging.Global()
	if log != nil {
		log.Debug("sending LLM request",
			logging.Model(currentModel),
			logging.MessageCount(len(messages)),
			logging.F("tools", len(tools)),
		)
	}

	// Generate request ID for tracing
	requestID := debug.GenerateRequestID()
	debug.LLMRequest(requestID, currentModel, len(messages), len(tools))
	startTime := time.Now()

	// Log event to new tracer
	if log != nil {
		log.SetRequestID(requestID)
		log.Event(logging.EventLLMRequest,
			logging.RequestID(requestID),
			logging.Model(currentModel),
			logging.MessageCount(len(messages)),
			logging.F("tools", len(tools)),
		)
	}

	// Build request
	ollamaMessages := c.buildMessages(messages, systemPrompt)
	ollamaTools := c.buildTools(tools)

	numCtx := c.config.GetContextWindowForModel(currentModel)

	opts := &OllamaOptions{
		Temperature: c.config.Temperature,
		NumPredict:  c.config.MaxTokens,
		NumCtx:      numCtx,
		NumThread:   c.config.Ollama.NumThread,
	}

	request := OllamaChatRequest{
		Model:     currentModel,
		Messages:  ollamaMessages,
		Tools:     ollamaTools,
		Stream:    false,
		KeepAlive: c.config.Ollama.KeepAlive,
		Options:   opts,
	}

	// Log full request payload if enabled
	debug.LLMRequestFull(requestID, map[string]any{
		"model":       currentModel,
		"messages":    len(ollamaMessages),
		"tools":       len(ollamaTools),
		"temperature": c.config.Temperature,
		"max_tokens":  c.config.MaxTokens,
	})

	// Send request
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("ollama request failed: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("failed to read response: %w", err))
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, vecerr.LLMModelNotFound(currentModel)
		}
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody)))
	}

	// Parse response
	var ollamaResp OllamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("failed to parse response: %w", err))
	}

	if ollamaResp.Error != "" {
		var err error
		if ollamaResp.Error == "model not found" {
			err = vecerr.LLMModelNotFound(currentModel)
		} else {
			err = vecerr.LLMRequestFailed(fmt.Errorf("ollama error: %s", ollamaResp.Error))
		}
		debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), 0, err)
		return nil, err
	}

	// Log successful response
	totalTokens := ollamaResp.PromptEvalCount + ollamaResp.EvalCount
	debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), totalTokens, nil)

	// Log full payload if enabled
	debug.LLMResponseFull(requestID, map[string]any{
		"model":             ollamaResp.Model,
		"content":           ollamaResp.Message.Content,
		"tool_calls":        len(ollamaResp.Message.ToolCalls),
		"done_reason":       ollamaResp.DoneReason,
		"prompt_eval_count": ollamaResp.PromptEvalCount,
		"eval_count":        ollamaResp.EvalCount,
	})

	if log != nil {
		log.Debug("received LLM response",
			logging.RequestID(requestID),
			logging.F("done_reason", ollamaResp.DoneReason),
			logging.DurationSince(startTime),
		)
		log.Event(logging.EventLLMResponse,
			logging.RequestID(requestID),
			logging.DurationSince(startTime),
			logging.InputTokens(ollamaResp.PromptEvalCount),
			logging.OutputTokens(ollamaResp.EvalCount),
			logging.Success(true),
		)
		log.ClearRequestID()
	}
	return c.parseResponse(&ollamaResp), nil
}

// ChatStream sends a message and streams the response
func (c *OllamaClient) ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 100)

	// Snapshot model name under lock for consistent use throughout this call
	currentModel := c.GetModel()

	// Generate request ID for tracing (before goroutine to ensure consistent ID)
	requestID := debug.GenerateRequestID()

	go func() {
		defer close(ch)

		debug.LLMRequest(requestID, currentModel, len(messages), len(tools))
		startTime := time.Now()

		// Build request
		ollamaMessages := c.buildMessages(messages, systemPrompt)
		ollamaTools := c.buildTools(tools)

		numCtx := c.config.GetContextWindowForModel(currentModel)

		request := OllamaChatRequest{
			Model:     currentModel,
			Messages:  ollamaMessages,
			Tools:     ollamaTools,
			Stream:    true,
			KeepAlive: c.config.Ollama.KeepAlive,
			Options: &OllamaOptions{
				Temperature: c.config.Temperature,
				NumPredict:  c.config.MaxTokens,
				NumCtx:      numCtx,
				NumThread:   c.config.Ollama.NumThread,
			},
		}

		// Send request
		body, err := json.Marshal(request)
		if err != nil {
			ch <- StreamChunk{Type: "error", Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
		if err != nil {
			ch <- StreamChunk{Type: "error", Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return // Clean exit on cancellation
			}
			ch <- StreamChunk{Type: "error", Error: vecerr.LLMRequestFailed(fmt.Errorf("ollama request failed: %w", err))}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		// Check for HTTP errors
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusNotFound {
				ch <- StreamChunk{Type: "error", Error: vecerr.LLMModelNotFound(currentModel)}
			} else {
				ch <- StreamChunk{Type: "error", Error: vecerr.LLMRequestFailed(fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body)))}
			}
			return
		}

		// Process NDJSON stream
		c.processStream(ctx, resp.Body, ch, requestID, startTime, currentModel)
	}()

	return ch
}

// processStream processes the NDJSON stream from Ollama
func (c *OllamaClient) processStream(ctx context.Context, reader io.Reader, ch chan<- StreamChunk, requestID string, startTime time.Time, currentModel string) {
	decoder := json.NewDecoder(reader)

	var usage *Usage

	for {
		select {
		case <-ctx.Done():
			debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), 0, ctx.Err())
			return
		default:
		}

		var chunk OllamaChatResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			if errors.Is(err, context.Canceled) {
				debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), 0, err)
				return
			}
			if log := logging.Global(); log != nil {
				log.Error("stream decode error", logging.Error(err))
			}
			debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), 0, err)
			ch <- StreamChunk{Type: "error", Error: err}
			return
		}

		// Handle errors in chunk
		if chunk.Error != "" {
			var err error
			if chunk.Error == "model not found" {
				err = vecerr.LLMModelNotFound(currentModel)
			} else {
				err = vecerr.LLMRequestFailed(fmt.Errorf("ollama error: %s", chunk.Error))
			}
			debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), 0, err)
			ch <- StreamChunk{Type: "error", Error: err}
			return
		}

		// Process content
		if chunk.Message.Content != "" {
			ch <- StreamChunk{Type: "text", Text: chunk.Message.Content}
		}

		// Process tool calls
		for _, tc := range chunk.Message.ToolCalls {
			input, err := parseToolArguments(tc.Function.Arguments)
			if err != nil {
				if log := logging.Global(); log != nil {
					log.Warn("failed to parse tool arguments",
						logging.ToolName(tc.Function.Name),
						logging.Error(err),
					)
				}
				input = make(map[string]any)
			}

			toolCall := ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			}
			ch <- StreamChunk{Type: "tool_call", ToolCall: &toolCall}
		}

		// Check if done
		if chunk.Done {
			usage = &Usage{
				InputTokens:  int64(chunk.PromptEvalCount),
				OutputTokens: int64(chunk.EvalCount),
			}
			// Log successful response
			totalTokens := chunk.PromptEvalCount + chunk.EvalCount
			debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), totalTokens, nil)
			ch <- StreamChunk{Type: "done", Usage: usage}
			return
		}
	}
}

// buildMessages converts internal messages to Ollama format
func (c *OllamaClient) buildMessages(messages []Message, systemPrompt string) []OllamaMessage {
	var ollamaMessages []OllamaMessage

	// Add system prompt as first message
	if systemPrompt != "" {
		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Convert messages
	for _, msg := range messages {
		om := OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		// Set ToolCallID for tool result messages
		if msg.Role == "tool" && msg.ToolCallID != "" {
			om.ToolCallID = msg.ToolCallID
		}
		// Convert ToolCalls for assistant messages
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				args, _ := json.Marshal(tc.Input)
				otc := OllamaToolCall{
					ID:   tc.ID,
					Type: "function",
				}
				otc.Function.Name = tc.Name
				otc.Function.Arguments = args
				om.ToolCalls = append(om.ToolCalls, otc)
			}
		}
		ollamaMessages = append(ollamaMessages, om)
	}

	return ollamaMessages
}

// buildTools converts internal tool definitions to Ollama format
func (c *OllamaClient) buildTools(tools []ToolDefinition) []OllamaTool {
	if len(tools) == 0 {
		return nil
	}

	ollamaTools := make([]OllamaTool, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = OllamaTool{
			Type: "function",
		}
		ollamaTools[i].Function.Name = tool.Name
		ollamaTools[i].Function.Description = tool.Description
		ollamaTools[i].Function.Parameters = tool.InputSchema
	}

	return ollamaTools
}

// parseResponse converts Ollama response to internal format
func (c *OllamaClient) parseResponse(resp *OllamaChatResponse) *Response {
	result := &Response{
		Content:    resp.Message.Content,
		StopReason: resp.DoneReason,
	}

	// Parse tool calls
	for _, tc := range resp.Message.ToolCalls {
		input, err := parseToolArguments(tc.Function.Arguments)
		if err != nil {
			if log := logging.Global(); log != nil {
				log.Warn("failed to parse tool arguments",
					logging.ToolName(tc.Function.Name),
					logging.Error(err),
				)
			}
			input = make(map[string]any)
		}

		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	return result
}

// parseToolArguments parses JSON arguments (can be object or string) to map
func parseToolArguments(args json.RawMessage) (map[string]any, error) {
	if len(args) == 0 || string(args) == "{}" || string(args) == "null" {
		return map[string]any{}, nil
	}

	var result map[string]any

	// First try to unmarshal directly as an object
	if err := json.Unmarshal(args, &result); err == nil {
		return result, nil
	}

	// If that fails, it might be a JSON string containing JSON (OpenAI format)
	var argsStr string
	if err := json.Unmarshal(args, &argsStr); err == nil {
		if argsStr == "" || argsStr == "{}" {
			return map[string]any{}, nil
		}
		if err := json.Unmarshal([]byte(argsStr), &result); err != nil {
			return nil, fmt.Errorf("failed to parse arguments string: %w", err)
		}
		return result, nil
	}

	return nil, fmt.Errorf("failed to parse arguments: %s", string(args))
}

// WarmModel preloads the current model into memory by sending a no-op chat request.
// This is useful when Ollama has unloaded the model due to keep_alive timeout.
// The method uses /api/chat with an empty messages array which triggers model loading
// without generating any tokens.
func (c *OllamaClient) WarmModel(ctx context.Context) error {
	currentModel := c.GetModel()

	request := OllamaChatRequest{
		Model:     currentModel,
		Messages:  []OllamaMessage{},
		Stream:    false,
		KeepAlive: c.config.Ollama.KeepAlive,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal warm request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create warm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return vecerr.LLMUnavailable(fmt.Errorf("model warm failed: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return vecerr.LLMModelNotFound(currentModel)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return vecerr.LLMRequestFailed(fmt.Errorf("model warm returned status %d: %s", resp.StatusCode, string(respBody)))
	}

	// Drain response body
	_, _ = io.ReadAll(resp.Body)

	if log := logging.Global(); log != nil {
		log.Debug("model warmed successfully", logging.Model(currentModel))
	}

	return nil
}
