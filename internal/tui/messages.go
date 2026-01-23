package tui

import "github.com/abdul-hamid-achik/vecai/internal/tools"

// StreamMsg represents a streaming message from the LLM
type StreamMsg struct {
	Type     string // "text", "thinking", "done", "tool_call", "tool_result", "error", "info", "warning", "success", "permission", "clear", "model_info"
	Text     string
	ToolName string
	ToolDesc string
	IsError  bool
	Level    tools.PermissionLevel
}

// TickMsg is sent for spinner animation
type TickMsg struct{}

// PermissionResponseMsg is sent when user responds to a permission prompt
type PermissionResponseMsg struct {
	Decision string
}

// QuitMsg signals the TUI to quit
type QuitMsg struct{}

// ClearMsg signals to clear the conversation
type ClearMsg struct{}

// Message constructors for the adapter

// NewTextMsg creates a text stream message
func NewTextMsg(text string) StreamMsg {
	return StreamMsg{Type: "text", Text: text}
}

// NewThinkingMsg creates a thinking stream message
func NewThinkingMsg(text string) StreamMsg {
	return StreamMsg{Type: "thinking", Text: text}
}

// NewDoneMsg creates a done stream message
func NewDoneMsg() StreamMsg {
	return StreamMsg{Type: "done"}
}

// NewToolCallMsg creates a tool call message
func NewToolCallMsg(name, description string) StreamMsg {
	return StreamMsg{Type: "tool_call", ToolName: name, ToolDesc: description}
}

// NewToolResultMsg creates a tool result message
func NewToolResultMsg(name, result string, isError bool) StreamMsg {
	return StreamMsg{Type: "tool_result", ToolName: name, Text: result, IsError: isError}
}

// NewErrorMsg creates an error message
func NewErrorMsg(text string) StreamMsg {
	return StreamMsg{Type: "error", Text: text}
}

// NewInfoMsg creates an info message
func NewInfoMsg(text string) StreamMsg {
	return StreamMsg{Type: "info", Text: text}
}

// NewWarningMsg creates a warning message
func NewWarningMsg(text string) StreamMsg {
	return StreamMsg{Type: "warning", Text: text}
}

// NewSuccessMsg creates a success message
func NewSuccessMsg(text string) StreamMsg {
	return StreamMsg{Type: "success", Text: text}
}

// NewPermissionMsg creates a permission request message
func NewPermissionMsg(toolName string, level tools.PermissionLevel, description string) StreamMsg {
	return StreamMsg{
		Type:     "permission",
		ToolName: toolName,
		Level:    level,
		ToolDesc: description,
	}
}

// NewClearMsg creates a clear message
func NewClearMsg() StreamMsg {
	return StreamMsg{Type: "clear"}
}

// NewModelInfoMsg creates a model info message
func NewModelInfoMsg(model string) StreamMsg {
	return StreamMsg{Type: "model_info", Text: model}
}

// NewUserMsg creates a user message
func NewUserMsg(text string) StreamMsg {
	return StreamMsg{Type: "user", Text: text}
}

// NewActivityMsg creates an activity status message
func NewActivityMsg(message string) StreamMsg {
	return StreamMsg{Type: "activity", Text: message}
}
