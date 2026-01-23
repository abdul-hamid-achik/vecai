package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui/highlight"
)

// ANSI color codes
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Dim        = "\033[2m"
	Italic     = "\033[3m"
	Underline  = "\033[4m"

	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

// ANSI cursor control codes
const (
	CursorStart = "\r"       // Move cursor to start of line
	ClearLine   = "\033[2K"  // Clear entire line
)

// OutputHandler handles console output with colors
type OutputHandler struct {
	useColors   bool
	highlighter *highlight.Highlighter
}

// NewOutputHandler creates a new output handler
func NewOutputHandler() *OutputHandler {
	// Check if output is a terminal
	useColors := true
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		useColors = false
	}

	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		useColors = false
	}

	return &OutputHandler{
		useColors:   useColors,
		highlighter: highlight.New(useColors),
	}
}

// color applies color if colors are enabled
func (o *OutputHandler) color(color, text string) string {
	if !o.useColors {
		return text
	}
	return color + text + Reset
}

// IsTTY returns true if the output is a terminal (not piped/redirected)
func (o *OutputHandler) IsTTY() bool {
	return o.useColors
}

// UseColors returns true if colors are enabled
func (o *OutputHandler) UseColors() bool {
	return o.useColors
}

// Text outputs regular text
func (o *OutputHandler) Text(text string) {
	fmt.Print(text)
}

// TextLn outputs regular text with newline
func (o *OutputHandler) TextLn(text string) {
	fmt.Println(text)
}

// Thinking outputs thinking text (dimmed)
func (o *OutputHandler) Thinking(text string) {
	fmt.Print(o.color(Dim+Italic, text))
}

// ThinkingLn outputs thinking text with newline
func (o *OutputHandler) ThinkingLn(text string) {
	fmt.Println(o.color(Dim+Italic, text))
}

// ToolCall outputs a tool call notification
func (o *OutputHandler) ToolCall(name string, description string) {
	prefix := o.color(Cyan+Bold, "⚡ ")
	toolName := o.color(Cyan, name)
	desc := o.color(Dim, " - "+description)
	fmt.Println(prefix + toolName + desc)
}

// ToolResult outputs a tool result
func (o *OutputHandler) ToolResult(name string, result string, isError bool) {
	if isError {
		prefix := o.color(Red+Bold, "✗ ")
		fmt.Println(prefix + o.color(Red, name+": ") + result)
	} else {
		// Truncate long results
		const maxLen = 500
		display := result
		if len(result) > maxLen {
			display = result[:maxLen] + "..."
		}

		// Apply syntax highlighting to code blocks
		display = o.highlighter.HighlightMarkdownCodeBlocks(display)

		prefix := o.color(Green, "✓ ")
		fmt.Println(prefix + o.color(Green, name))
		if display != "" && display != "(no output)" {
			// Indent result
			lines := strings.Split(display, "\n")
			if len(lines) > 10 {
				lines = append(lines[:10], "... (truncated)")
			}
			for _, line := range lines {
				fmt.Println(o.color(Dim, "  │ ") + line)
			}
		}
	}
}

// Error outputs an error message
func (o *OutputHandler) Error(err error) {
	prefix := o.color(Red+Bold, "Error: ")
	fmt.Fprintln(os.Stderr, prefix+err.Error())
}

// ErrorStr outputs an error string
func (o *OutputHandler) ErrorStr(msg string) {
	prefix := o.color(Red+Bold, "Error: ")
	fmt.Fprintln(os.Stderr, prefix+msg)
}

// Warning outputs a warning message
func (o *OutputHandler) Warning(msg string) {
	prefix := o.color(Yellow+Bold, "Warning: ")
	fmt.Fprintln(os.Stderr, prefix+msg)
}

// Success outputs a success message
func (o *OutputHandler) Success(msg string) {
	prefix := o.color(Green+Bold, "✓ ")
	fmt.Println(prefix + msg)
}

// Info outputs an info message
func (o *OutputHandler) Info(msg string) {
	prefix := o.color(Blue, "ℹ ")
	fmt.Println(prefix + msg)
}

// Done outputs a completion message
func (o *OutputHandler) Done() {
	fmt.Println()
}

// Prompt outputs a prompt
func (o *OutputHandler) Prompt(prompt string) {
	fmt.Print(o.color(Bold+Green, prompt))
}

// PermissionPrompt outputs a permission prompt
func (o *OutputHandler) PermissionPrompt(toolName string, level tools.PermissionLevel, description string) {
	fmt.Println()

	var levelColor string
	var levelIcon string
	switch level {
	case tools.PermissionRead:
		levelColor = Blue
		levelIcon = "👁"
	case tools.PermissionWrite:
		levelColor = Yellow
		levelIcon = "✏️"
	case tools.PermissionExecute:
		levelColor = Red
		levelIcon = "⚠️"
	}

	fmt.Println(o.color(levelColor+Bold, fmt.Sprintf("%s Permission Required: %s", levelIcon, toolName)))
	fmt.Println(o.color(Dim, "   Level: ") + o.color(levelColor, level.String()))
	if description != "" {
		fmt.Println(o.color(Dim, "   Action: ") + description)
	}
	fmt.Println()
}

// Header outputs a header
func (o *OutputHandler) Header(text string) {
	fmt.Println()
	fmt.Println(o.color(Bold+Underline, text))
	fmt.Println()
}

// Separator outputs a horizontal line
func (o *OutputHandler) Separator() {
	fmt.Println(o.color(Dim, strings.Repeat("─", 40)))
}

// ModelInfo outputs the current model info
func (o *OutputHandler) ModelInfo(model string) {
	fmt.Println(o.color(Dim, "Using model: ") + o.color(Cyan, model))
}

// Question outputs a question for the user
func (o *OutputHandler) Question(question string, options []string) {
	fmt.Println()
	fmt.Println(o.color(Bold+Yellow, "? ") + question)
	for i, opt := range options {
		fmt.Printf("  %s %s\n", o.color(Cyan, fmt.Sprintf("[%d]", i+1)), opt)
	}
}

// StreamText outputs streaming text without newline
func (o *OutputHandler) StreamText(text string) {
	fmt.Print(text)
}

// StreamThinking outputs streaming thinking text without newline
func (o *OutputHandler) StreamThinking(text string) {
	fmt.Print(o.color(Dim+Italic, text))
}

// StreamDone signals end of streaming
func (o *OutputHandler) StreamDone() {
	fmt.Println()
}
