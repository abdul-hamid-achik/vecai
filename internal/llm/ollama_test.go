package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
)

// newTestClient creates an OllamaClient pointing at the given base URL.
// If baseURL is empty it defaults to a bogus address (useful when the
// test does not need a real server).
func newTestClient(baseURL string) *OllamaClient {
	cfg := config.DefaultConfig()
	if baseURL != "" {
		cfg.Ollama.BaseURL = baseURL
	}
	return NewOllamaClient(cfg)
}

// ---------------------------------------------------------------------------
// buildMessages
// ---------------------------------------------------------------------------

func TestBuildMessages_SystemPrompt(t *testing.T) {
	c := newTestClient("")

	msgs := c.buildMessages(nil, "You are helpful.")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected role system, got %q", msgs[0].Role)
	}
	if msgs[0].Content != "You are helpful." {
		t.Errorf("expected system content, got %q", msgs[0].Content)
	}
}

func TestBuildMessages_NoSystemPrompt(t *testing.T) {
	c := newTestClient("")

	input := []Message{{Role: "user", Content: "hi"}}
	msgs := c.buildMessages(input, "")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("expected role user, got %q", msgs[0].Role)
	}
}

func TestBuildMessages_ToolCallIDPreserved(t *testing.T) {
	c := newTestClient("")

	input := []Message{
		{Role: "tool", Content: `{"result":"ok"}`, ToolCallID: "call_abc123"},
	}
	msgs := c.buildMessages(input, "")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ToolCallID != "call_abc123" {
		t.Errorf("expected ToolCallID call_abc123, got %q", msgs[0].ToolCallID)
	}
}

func TestBuildMessages_ToolCallIDNotSetForNonTool(t *testing.T) {
	c := newTestClient("")

	input := []Message{
		{Role: "user", Content: "hello", ToolCallID: "ignored"},
	}
	msgs := c.buildMessages(input, "")
	if msgs[0].ToolCallID != "" {
		t.Errorf("expected empty ToolCallID for user role, got %q", msgs[0].ToolCallID)
	}
}

func TestBuildMessages_ToolCallConversion(t *testing.T) {
	c := newTestClient("")

	input := []Message{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{ID: "tc_1", Name: "read_file", Input: map[string]any{"path": "/tmp/x"}},
				{ID: "tc_2", Name: "list_files", Input: map[string]any{"dir": "."}},
			},
		},
	}
	msgs := c.buildMessages(input, "")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(msgs[0].ToolCalls))
	}

	tc := msgs[0].ToolCalls[0]
	if tc.ID != "tc_1" {
		t.Errorf("expected ID tc_1, got %q", tc.ID)
	}
	if tc.Type != "function" {
		t.Errorf("expected type function, got %q", tc.Type)
	}
	if tc.Function.Name != "read_file" {
		t.Errorf("expected name read_file, got %q", tc.Function.Name)
	}

	// Verify arguments are valid JSON containing the path key
	var args map[string]any
	if err := json.Unmarshal(tc.Function.Arguments, &args); err != nil {
		t.Fatalf("failed to unmarshal arguments: %v", err)
	}
	if args["path"] != "/tmp/x" {
		t.Errorf("expected path /tmp/x, got %v", args["path"])
	}
}

func TestBuildMessages_SystemPromptAndMessages(t *testing.T) {
	c := newTestClient("")

	input := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	msgs := c.buildMessages(input, "Be concise.")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (system + 2 input), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message should be system, got %q", msgs[0].Role)
	}
	if msgs[1].Role != "user" {
		t.Errorf("second message should be user, got %q", msgs[1].Role)
	}
	if msgs[2].Role != "assistant" {
		t.Errorf("third message should be assistant, got %q", msgs[2].Role)
	}
}

// ---------------------------------------------------------------------------
// buildTools
// ---------------------------------------------------------------------------

func TestBuildTools_EmptyTools(t *testing.T) {
	c := newTestClient("")

	result := c.buildTools(nil)
	if result != nil {
		t.Errorf("expected nil for empty tools, got %v", result)
	}

	result = c.buildTools([]ToolDefinition{})
	if result != nil {
		t.Errorf("expected nil for zero-length tools, got %v", result)
	}
}

func TestBuildTools_Conversion(t *testing.T) {
	c := newTestClient("")

	tools := []ToolDefinition{
		{
			Name:        "read_file",
			Description: "Reads a file",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path",
					},
				},
				"required": []any{"path"},
			},
		},
		{
			Name:        "list_files",
			Description: "Lists files in a directory",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"dir": map[string]any{
						"type": "string",
					},
				},
			},
		},
	}

	result := c.buildTools(tools)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}

	// Check first tool
	if result[0].Type != "function" {
		t.Errorf("expected type function, got %q", result[0].Type)
	}
	if result[0].Function.Name != "read_file" {
		t.Errorf("expected name read_file, got %q", result[0].Function.Name)
	}
	if result[0].Function.Description != "Reads a file" {
		t.Errorf("expected description 'Reads a file', got %q", result[0].Function.Description)
	}
	if result[0].Function.Parameters == nil {
		t.Error("expected non-nil parameters")
	}

	// Check second tool
	if result[1].Function.Name != "list_files" {
		t.Errorf("expected name list_files, got %q", result[1].Function.Name)
	}
}

// ---------------------------------------------------------------------------
// parseResponse
// ---------------------------------------------------------------------------

func TestParseResponse_ContentOnly(t *testing.T) {
	c := newTestClient("")

	resp := &OllamaChatResponse{
		Message:    OllamaMessage{Role: "assistant", Content: "Hello world"},
		DoneReason: "stop",
	}

	result := c.parseResponse(resp)
	if result.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got %q", result.Content)
	}
	if result.StopReason != "stop" {
		t.Errorf("expected stop reason 'stop', got %q", result.StopReason)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(result.ToolCalls))
	}
}

func TestParseResponse_WithToolCalls(t *testing.T) {
	c := newTestClient("")

	args, _ := json.Marshal(map[string]any{"path": "/etc/hosts"})
	resp := &OllamaChatResponse{
		Message: OllamaMessage{
			Role: "assistant",
			ToolCalls: []OllamaToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					}{
						Name:      "read_file",
						Arguments: args,
					},
				},
			},
		},
		DoneReason: "tool_calls",
	}

	result := c.parseResponse(resp)
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("expected ID call_1, got %q", tc.ID)
	}
	if tc.Name != "read_file" {
		t.Errorf("expected name read_file, got %q", tc.Name)
	}
	if tc.Input["path"] != "/etc/hosts" {
		t.Errorf("expected path /etc/hosts, got %v", tc.Input["path"])
	}
}

func TestParseResponse_InvalidToolArgs(t *testing.T) {
	c := newTestClient("")

	resp := &OllamaChatResponse{
		Message: OllamaMessage{
			Role: "assistant",
			ToolCalls: []OllamaToolCall{
				{
					ID:   "call_bad",
					Type: "function",
					Function: struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					}{
						Name:      "some_tool",
						Arguments: json.RawMessage(`not valid json`),
					},
				},
			},
		},
	}

	result := c.parseResponse(resp)
	// Should not panic; should produce an empty input map
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Input == nil {
		t.Error("expected non-nil (empty) input map for invalid args")
	}
}

// ---------------------------------------------------------------------------
// parseToolArguments
// ---------------------------------------------------------------------------

func TestParseToolArguments_ObjectArgs(t *testing.T) {
	args := json.RawMessage(`{"path":"/tmp/file","line":42}`)
	result, err := parseToolArguments(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["path"] != "/tmp/file" {
		t.Errorf("expected path /tmp/file, got %v", result["path"])
	}
	// JSON numbers decode as float64
	if result["line"] != float64(42) {
		t.Errorf("expected line 42, got %v", result["line"])
	}
}

func TestParseToolArguments_StringArgs(t *testing.T) {
	// A JSON string containing a JSON object (OpenAI format)
	inner := `{"key":"value"}`
	raw, _ := json.Marshal(inner) // produces `"{\"key\":\"value\"}"`
	result, err := parseToolArguments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestParseToolArguments_EmptyArgs(t *testing.T) {
	cases := []json.RawMessage{
		nil,
		json.RawMessage(``),
		json.RawMessage(`{}`),
		json.RawMessage(`null`),
	}

	for _, args := range cases {
		result, err := parseToolArguments(args)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", string(args), err)
		}
		if result == nil {
			t.Errorf("expected non-nil map for %q", string(args))
		}
	}
}

func TestParseToolArguments_EmptyStringArg(t *testing.T) {
	// A JSON string that is empty: `""`
	raw, _ := json.Marshal("")
	result, err := parseToolArguments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseToolArguments_EmptyObjectStringArg(t *testing.T) {
	// A JSON string containing "{}"
	raw, _ := json.Marshal("{}")
	result, err := parseToolArguments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseToolArguments_InvalidJSON(t *testing.T) {
	args := json.RawMessage(`not json at all`)
	_, err := parseToolArguments(args)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// CheckHealth (using httptest)
// ---------------------------------------------------------------------------

func TestCheckHealth_HealthyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"0.6.0"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCheckHealth_UnhealthyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.CheckHealth(context.Background())
	if err == nil {
		t.Fatal("expected error for unhealthy server")
	}
}

func TestCheckHealth_ConnectionError(t *testing.T) {
	// Point at a port nothing listens on
	c := newTestClient("http://127.0.0.1:1")
	err := c.CheckHealth(context.Background())
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestCheckHealthWithVersion_ReturnsVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"0.7.1"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	ver, err := c.CheckHealthWithVersion(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "0.7.1" {
		t.Errorf("expected version 0.7.1, got %q", ver)
	}
}

// ---------------------------------------------------------------------------
// SetModel / GetModel / SetTier
// ---------------------------------------------------------------------------

func TestSetModelGetModel(t *testing.T) {
	c := newTestClient("")
	c.SetModel("llama3.2:3b")
	if got := c.GetModel(); got != "llama3.2:3b" {
		t.Errorf("expected llama3.2:3b, got %q", got)
	}
}

func TestSetTier(t *testing.T) {
	c := newTestClient("")
	c.SetTier(config.TierGenius)
	if got := c.GetModel(); got != c.config.Ollama.ModelGenius {
		t.Errorf("expected %q, got %q", c.config.Ollama.ModelGenius, got)
	}
}

func TestClose(t *testing.T) {
	c := newTestClient("")
	if err := c.Close(); err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Chat request keep_alive field
// ---------------------------------------------------------------------------

func TestChatRequestIncludesKeepAlive(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(OllamaChatResponse{
			Message:    OllamaMessage{Role: "assistant", Content: "hi"},
			Done:       true,
			DoneReason: "stop",
		})
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Ollama.BaseURL = srv.URL
	cfg.Ollama.KeepAlive = "30m"
	c := NewOllamaClient(cfg)

	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}}, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req OllamaChatRequest
	if err := json.Unmarshal(captured, &req); err != nil {
		t.Fatalf("failed to unmarshal captured request: %v", err)
	}
	if req.KeepAlive != "30m" {
		t.Errorf("expected keep_alive \"30m\", got %q", req.KeepAlive)
	}
}

func TestChatRequestOmitsEmptyKeepAlive(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(OllamaChatResponse{
			Message:    OllamaMessage{Role: "assistant", Content: "hi"},
			Done:       true,
			DoneReason: "stop",
		})
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Ollama.BaseURL = srv.URL
	cfg.Ollama.KeepAlive = ""
	c := NewOllamaClient(cfg)

	_, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}}, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The raw JSON should not contain the keep_alive key at all (omitempty)
	if bytes.Contains(captured, []byte(`"keep_alive"`)) {
		t.Error("expected keep_alive to be omitted from request when empty, but it was present")
	}
}

// ---------------------------------------------------------------------------
// WarmModel
// ---------------------------------------------------------------------------

func TestWarmModel_Success(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(OllamaChatResponse{
			Message:    OllamaMessage{Role: "assistant", Content: ""},
			Done:       true,
			DoneReason: "stop",
		})
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Ollama.BaseURL = srv.URL
	cfg.Ollama.KeepAlive = "15m"
	c := NewOllamaClient(cfg)

	err := c.WarmModel(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/api/chat" {
		t.Errorf("expected /api/chat, got %s", capturedPath)
	}

	var req OllamaChatRequest
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if len(req.Messages) != 0 {
		t.Errorf("expected empty messages array, got %d messages", len(req.Messages))
	}
	if req.KeepAlive != "15m" {
		t.Errorf("expected keep_alive \"15m\", got %q", req.KeepAlive)
	}
}

func TestWarmModel_OllamaUnavailable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Ollama.BaseURL = "http://127.0.0.1:1" // nothing listening
	c := NewOllamaClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.WarmModel(ctx)
	if err == nil {
		t.Fatal("expected error for unavailable server, got nil")
	}
}

func TestWarmModel_ModelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Ollama.BaseURL = srv.URL
	c := NewOllamaClient(cfg)

	err := c.WarmModel(context.Background())
	if err == nil {
		t.Fatal("expected error for model not found, got nil")
	}
}

func TestWarmModel_RespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Delay long enough that a cancelled context should win
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Ollama.BaseURL = srv.URL
	c := NewOllamaClient(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := c.WarmModel(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
