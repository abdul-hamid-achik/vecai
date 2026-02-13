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

// TierClient is a lightweight LLM client that shares the HTTP transport with
// a parent OllamaClient but maintains its own independent model field.
// Close is a no-op because TierClient does not own the connection pool.
type TierClient struct {
	baseURL    string
	model      string
	modelMu    sync.RWMutex
	config     *config.Config
	httpClient *http.Client // shared, not owned
}

// NewTierClient creates a TierClient that shares the HTTP transport of the
// given OllamaClient but has its own mutable model field.
func NewTierClient(parent *OllamaClient) *TierClient {
	return &TierClient{
		baseURL:    parent.baseURL,
		model:      parent.GetModel(),
		config:     parent.config,
		httpClient: parent.httpClient,
	}
}

// SetModel changes the current model (thread-safe).
func (tc *TierClient) SetModel(model string) {
	tc.modelMu.Lock()
	defer tc.modelMu.Unlock()
	tc.model = model
}

// SetTier changes the model tier (thread-safe).
func (tc *TierClient) SetTier(tier config.ModelTier) {
	tc.modelMu.Lock()
	defer tc.modelMu.Unlock()
	tc.model = tc.config.GetModel(tier)
}

// GetModel returns the current model (thread-safe).
func (tc *TierClient) GetModel() string {
	tc.modelMu.RLock()
	defer tc.modelMu.RUnlock()
	return tc.model
}

// Fork returns another TierClient sharing the same HTTP transport.
func (tc *TierClient) Fork() LLMClient {
	return &TierClient{
		baseURL:    tc.baseURL,
		model:      tc.GetModel(),
		config:     tc.config,
		httpClient: tc.httpClient,
	}
}

// Close is a no-op; TierClient does not own the HTTP transport.
func (tc *TierClient) Close() error {
	return nil
}

// Chat sends a message and returns the response.
func (tc *TierClient) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	currentModel := tc.GetModel()

	requestID := debug.GenerateRequestID()
	debug.LLMRequest(requestID, currentModel, len(messages), len(tools))
	startTime := time.Now()

	ollamaMessages := buildMessagesStatic(messages, systemPrompt)
	ollamaTools := buildToolsStatic(tools)

	numCtx := tc.config.GetContextWindowForModel(currentModel)

	temperature := tc.config.Temperature
	if override, ok := GetTemperature(ctx); ok {
		temperature = override
	}

	request := OllamaChatRequest{
		Model:    currentModel,
		Messages: ollamaMessages,
		Tools:    ollamaTools,
		Stream:   false,
		KeepAlive: tc.config.Ollama.KeepAlive,
		Options: &OllamaOptions{
			Temperature: temperature,
			NumPredict:  tc.config.MaxTokens,
			NumCtx:      numCtx,
			NumThread:   tc.config.Ollama.NumThread,
		},
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tc.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("ollama request failed: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("failed to read response: %w", err))
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, vecerr.LLMModelNotFound(currentModel)
		}
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody)))
	}

	var ollamaResp OllamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, vecerr.LLMRequestFailed(fmt.Errorf("failed to parse response: %w", err))
	}

	if ollamaResp.Error != "" {
		var respErr error
		if ollamaResp.Error == "model not found" {
			respErr = vecerr.LLMModelNotFound(currentModel)
		} else {
			respErr = vecerr.LLMRequestFailed(fmt.Errorf("ollama error: %s", ollamaResp.Error))
		}
		debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), 0, respErr)
		return nil, respErr
	}

	totalTokens := ollamaResp.PromptEvalCount + ollamaResp.EvalCount
	debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), totalTokens, nil)

	return parseResponseStatic(&ollamaResp), nil
}

// ChatStream sends a message and streams the response.
func (tc *TierClient) ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 100)

	currentModel := tc.GetModel()
	requestID := debug.GenerateRequestID()

	go func() {
		defer close(ch)

		// Add a default timeout if context has no deadline
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
		}

		debug.LLMRequest(requestID, currentModel, len(messages), len(tools))
		startTime := time.Now()

		ollamaMessages := buildMessagesStatic(messages, systemPrompt)
		ollamaTools := buildToolsStatic(tools)

		numCtx := tc.config.GetContextWindowForModel(currentModel)

		temperature := tc.config.Temperature
		if override, ok := GetTemperature(ctx); ok {
			temperature = override
		}

		request := OllamaChatRequest{
			Model:    currentModel,
			Messages: ollamaMessages,
			Tools:    ollamaTools,
			Stream:   true,
			KeepAlive: tc.config.Ollama.KeepAlive,
			Options: &OllamaOptions{
				Temperature: temperature,
				NumPredict:  tc.config.MaxTokens,
				NumCtx:      numCtx,
				NumThread:   tc.config.Ollama.NumThread,
			},
		}

		body, err := json.Marshal(request)
		if err != nil {
			ch <- StreamChunk{Type: "error", Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", tc.baseURL+"/api/chat", bytes.NewReader(body))
		if err != nil {
			ch <- StreamChunk{Type: "error", Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := tc.httpClient.Do(req)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			ch <- StreamChunk{Type: "error", Error: vecerr.LLMRequestFailed(fmt.Errorf("ollama request failed: %w", err))}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusNotFound {
				ch <- StreamChunk{Type: "error", Error: vecerr.LLMModelNotFound(currentModel)}
			} else {
				ch <- StreamChunk{Type: "error", Error: vecerr.LLMRequestFailed(fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body)))}
			}
			return
		}

		processStreamStatic(ctx, resp.Body, ch, requestID, startTime, currentModel)
	}()

	return ch
}

// buildMessagesStatic converts internal messages to Ollama format (static, no receiver needed).
func buildMessagesStatic(messages []Message, systemPrompt string) []OllamaMessage {
	var ollamaMessages []OllamaMessage

	if systemPrompt != "" {
		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	for _, msg := range messages {
		om := OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "tool" && msg.ToolCallID != "" {
			om.ToolCallID = msg.ToolCallID
		}
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

// buildToolsStatic converts internal tool definitions to Ollama format (static).
func buildToolsStatic(tools []ToolDefinition) []OllamaTool {
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

// parseResponseStatic converts Ollama response to internal format (static).
func parseResponseStatic(resp *OllamaChatResponse) *Response {
	result := &Response{
		Content:    resp.Message.Content,
		StopReason: resp.DoneReason,
	}

	for _, tc := range resp.Message.ToolCalls {
		input, parseErr := parseToolArguments(tc.Function.Arguments)
		var parseErrStr string
		if parseErr != nil {
			if log := logging.Global(); log != nil {
				log.Warn("failed to parse tool arguments",
					logging.ToolName(tc.Function.Name),
					logging.Error(parseErr),
				)
			}
			input = make(map[string]any)
			parseErrStr = parseErr.Error()
		}

		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:         tc.ID,
			Name:       tc.Function.Name,
			Input:      input,
			ParseError: parseErrStr,
		})
	}

	return result
}

// processStreamStatic processes the NDJSON stream from Ollama (static).
func processStreamStatic(ctx context.Context, reader io.Reader, ch chan<- StreamChunk, requestID string, startTime time.Time, currentModel string) {
	decoder := json.NewDecoder(reader)

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

		if chunk.Message.Content != "" {
			ch <- StreamChunk{Type: "text", Text: chunk.Message.Content}
		}

		for _, tc := range chunk.Message.ToolCalls {
			input, parseErr := parseToolArguments(tc.Function.Arguments)
			var parseErrStr string
			if parseErr != nil {
				if log := logging.Global(); log != nil {
					log.Warn("failed to parse tool arguments",
						logging.ToolName(tc.Function.Name),
						logging.Error(parseErr),
					)
				}
				input = make(map[string]any)
				parseErrStr = parseErr.Error()
			}

			toolCall := ToolCall{
				ID:         tc.ID,
				Name:       tc.Function.Name,
				Input:      input,
				ParseError: parseErrStr,
			}
			ch <- StreamChunk{Type: "tool_call", ToolCall: &toolCall}
		}

		if chunk.Done {
			usage := &Usage{
				InputTokens:  int64(chunk.PromptEvalCount),
				OutputTokens: int64(chunk.EvalCount),
			}
			totalTokens := chunk.PromptEvalCount + chunk.EvalCount
			debug.LLMResponse(requestID, time.Since(startTime).Milliseconds(), totalTokens, nil)
			ch <- StreamChunk{Type: "done", Usage: usage}
			return
		}
	}
}
