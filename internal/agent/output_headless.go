package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// HeadlessOutput implements AgentOutput for pipe/headless mode.
// Writes text to stdout, errors to stderr, suppresses tool progress.
type HeadlessOutput struct {
	textBuf strings.Builder
}

func (h *HeadlessOutput) StreamText(text string)         { fmt.Print(text); h.textBuf.WriteString(text) }
func (h *HeadlessOutput) StreamThinking(_ string)        {}
func (h *HeadlessOutput) StreamDone()                    { fmt.Println() }
func (h *HeadlessOutput) StreamDoneWithUsage(_, _ int64) { fmt.Println() }

func (h *HeadlessOutput) Text(text string)   { fmt.Print(text) }
func (h *HeadlessOutput) TextLn(text string) { fmt.Println(text) }
func (h *HeadlessOutput) Error(err error)    { fmt.Fprintln(os.Stderr, "Error:", err) }
func (h *HeadlessOutput) ErrorStr(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
}
func (h *HeadlessOutput) Warning(msg string)  { fmt.Fprintln(os.Stderr, "Warning:", msg) }
func (h *HeadlessOutput) Success(msg string)  {} // Suppress in headless
func (h *HeadlessOutput) Info(msg string)     {} // Suppress in headless
func (h *HeadlessOutput) ToolCall(_, _ string) {}
func (h *HeadlessOutput) ToolResult(_, _ string, _ bool) {
}
func (h *HeadlessOutput) PermissionPrompt(_ string, _ tools.PermissionLevel, _ string) {
}
func (h *HeadlessOutput) Header(_ string)     {}
func (h *HeadlessOutput) Separator()          {}
func (h *HeadlessOutput) Thinking(_ string)   {}
func (h *HeadlessOutput) ThinkingLn(_ string) {}
func (h *HeadlessOutput) ModelInfo(_ string)  {}
func (h *HeadlessOutput) Activity(_ string)   {}
func (h *HeadlessOutput) Done()               {}

// HeadlessInput implements AgentInput for headless mode.
// Always denies interactive prompts.
type HeadlessInput struct{}

func (h *HeadlessInput) ReadLine(_ string) (string, error) {
	return "n", nil // Auto-deny permission prompts
}

func (h *HeadlessInput) Confirm(_ string, _ bool) (bool, error) {
	return false, nil
}

// JSONOutput captures all output and emits a single JSON object at the end.
type JSONOutput struct {
	textBuf   strings.Builder
	toolCalls []jsonToolCall
	errors    []string
}

type jsonToolCall struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Result      string `json:"result,omitempty"`
	IsError     bool   `json:"is_error,omitempty"`
}

type jsonResult struct {
	Text      string         `json:"text"`
	ToolCalls []jsonToolCall `json:"tool_calls,omitempty"`
	Errors    []string       `json:"errors,omitempty"`
}

func (j *JSONOutput) StreamText(text string)         { j.textBuf.WriteString(text) }
func (j *JSONOutput) StreamThinking(_ string)        {}
func (j *JSONOutput) StreamDone()                    {}
func (j *JSONOutput) StreamDoneWithUsage(_, _ int64) {}

func (j *JSONOutput) Text(text string)    { j.textBuf.WriteString(text) }
func (j *JSONOutput) TextLn(text string)  { j.textBuf.WriteString(text + "\n") }
func (j *JSONOutput) Error(err error)     { j.errors = append(j.errors, err.Error()) }
func (j *JSONOutput) ErrorStr(msg string) { j.errors = append(j.errors, msg) }
func (j *JSONOutput) Warning(_ string)    {}
func (j *JSONOutput) Success(_ string)    {}
func (j *JSONOutput) Info(_ string)       {}
func (j *JSONOutput) ToolCall(name, desc string) {
	j.toolCalls = append(j.toolCalls, jsonToolCall{Name: name, Description: desc})
}
func (j *JSONOutput) ToolResult(name, result string, isError bool) {
	// Update the last matching tool call with its result
	for i := len(j.toolCalls) - 1; i >= 0; i-- {
		if j.toolCalls[i].Name == name && j.toolCalls[i].Result == "" {
			j.toolCalls[i].Result = result
			j.toolCalls[i].IsError = isError
			break
		}
	}
}
func (j *JSONOutput) PermissionPrompt(_ string, _ tools.PermissionLevel, _ string) {}
func (j *JSONOutput) Header(_ string)                                               {}
func (j *JSONOutput) Separator()                                                    {}
func (j *JSONOutput) Thinking(_ string)                                             {}
func (j *JSONOutput) ThinkingLn(_ string)                                           {}
func (j *JSONOutput) ModelInfo(_ string)                                            {}
func (j *JSONOutput) Activity(_ string)                                             {}
func (j *JSONOutput) Done()                                                         {}

// Emit writes the JSON result to stdout.
func (j *JSONOutput) Emit() error {
	result := jsonResult{
		Text:      j.textBuf.String(),
		ToolCalls: j.toolCalls,
		Errors:    j.errors,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
