package agent

import (
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

// CLIOutput wraps ui.OutputHandler and ui.InputHandler to satisfy AgentOutput and AgentInput.
type CLIOutput struct {
	Out *ui.OutputHandler
	In  *ui.InputHandler
}

// Verify interface compliance at compile time.
var _ AgentOutput = (*CLIOutput)(nil)
var _ AgentInput = (*CLIOutput)(nil)

// --- AgentOutput: Streaming ---

func (c *CLIOutput) StreamText(text string)      { c.Out.StreamText(text) }
func (c *CLIOutput) StreamThinking(text string)   { c.Out.StreamThinking(text) }
func (c *CLIOutput) StreamDone()                  { c.Out.StreamDone() }
func (c *CLIOutput) StreamDoneWithUsage(_, _ int64) { c.Out.StreamDone() }

// --- AgentOutput: Messages ---

func (c *CLIOutput) Text(text string)     { c.Out.Text(text) }
func (c *CLIOutput) TextLn(text string)   { c.Out.TextLn(text) }
func (c *CLIOutput) Error(err error)      { c.Out.Error(err) }
func (c *CLIOutput) ErrorStr(msg string)  { c.Out.ErrorStr(msg) }
func (c *CLIOutput) Warning(msg string)   { c.Out.Warning(msg) }
func (c *CLIOutput) Success(msg string)   { c.Out.Success(msg) }
func (c *CLIOutput) Info(msg string)      { c.Out.Info(msg) }

// --- AgentOutput: Tools ---

func (c *CLIOutput) ToolCall(name, description string)            { c.Out.ToolCall(name, description) }
func (c *CLIOutput) ToolResult(name, result string, isError bool) { c.Out.ToolResult(name, result, isError) }

// --- AgentOutput: Prompts ---

func (c *CLIOutput) PermissionPrompt(toolName string, level tools.PermissionLevel, description string) {
	c.Out.PermissionPrompt(toolName, level, description)
}

// --- AgentOutput: Formatting ---

func (c *CLIOutput) Header(text string)     { c.Out.Header(text) }
func (c *CLIOutput) Separator()             { c.Out.Separator() }
func (c *CLIOutput) Thinking(text string)   { c.Out.Thinking(text) }
func (c *CLIOutput) ThinkingLn(text string) { c.Out.ThinkingLn(text) }

// --- AgentOutput: Model & Activity ---

func (c *CLIOutput) ModelInfo(model string) { c.Out.ModelInfo(model) }
func (c *CLIOutput) Activity(_ string)      {} // No-op in CLI mode
func (c *CLIOutput) Done()                  { c.Out.Done() }

// --- AgentInput ---

func (c *CLIOutput) ReadLine(prompt string) (string, error) { return c.In.ReadLine(prompt) }
func (c *CLIOutput) Confirm(prompt string, defaultYes bool) (bool, error) {
	return c.In.Confirm(prompt, defaultYes)
}
