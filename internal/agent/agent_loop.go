package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ctxmgr "github.com/abdul-hamid-achik/vecai/internal/context"
	vecerr "github.com/abdul-hamid-achik/vecai/internal/errors"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

// ErrInterrupted is returned when the user interrupts the loop with ESC
var ErrInterrupted = errors.New("interrupted by user")

// runAgentLoop executes the agent loop with unified output.
// It handles both CLI and TUI modes via the AgentOutput/AgentInput interfaces.
// If output implements InterruptSupport, the loop supports ESC-based interruption.
// If output implements StatsSupport, context stats are displayed/updated.
func (a *Agent) runAgentLoop(ctx context.Context, output AgentOutput, input AgentInput) error {
	maxIterations := a.config.Agent.MaxIterations
	if maxIterations == 0 {
		maxIterations = 20
	}

	loopStartTime := time.Now()

	// Check if output supports interrupt and stats
	interruptible, hasInterrupt := output.(InterruptSupport)
	forceInterruptible, hasForceInterrupt := output.(ForceInterruptSupport)
	statsOut, hasStats := output.(StatsSupport)

	// Create cancellable context for interrupt support
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	// forceStop is closed when the user double-presses ESC — the loop should
	// bail out immediately without waiting for in-flight operations.
	forceStop := make(chan struct{})

	// Monitor interrupt channels if available
	if hasInterrupt {
		interruptChan := interruptible.GetInterruptChan()
		var forceChan <-chan struct{}
		if hasForceInterrupt {
			forceChan = forceInterruptible.GetForceInterruptChan()
		}
		go func() {
			// Wait for graceful interrupt first
			select {
			case <-interruptChan:
				cancelRun()
			case <-runCtx.Done():
				return
			}
			// After graceful interrupt, wait for force interrupt or exit
			if forceChan != nil {
				select {
				case <-forceChan:
					close(forceStop)
				case <-runCtx.Done():
				}
			}
		}()
	}

	// Start a checkpoint for /rewind support
	if a.checkpointMgr != nil {
		a.checkpointMgr.StartCheckpoint(a.currentQuery)
		defer a.checkpointMgr.CommitCheckpoint()
	}

	// Auto RAG: inject relevant code context before first LLM call
	if a.currentQuery != "" && !a.analysisMode {
		if ragCtx := a.autoRAGSearch(runCtx, a.currentQuery); ragCtx != "" {
			a.contextMgr.AddMessage(llm.Message{
				Role:    "user",
				Content: "[Relevant code context from codebase search]\n" + ragCtx,
			})
		}
	}

	consecutiveParseErrors := 0

	for i := 0; i < maxIterations; i++ {
		// Check for interrupt/cancellation before starting iteration
		if hasInterrupt {
			select {
			case <-runCtx.Done():
				output.Warning("Interrupted by user")
				output.StreamDone()
				return nil
			default:
			}
		}

		// Send stats update at start of each iteration
		if hasStats {
			statsOut.UpdateStats(tui.SessionStats{
				LoopIteration: i + 1,
				MaxIterations: maxIterations,
				LoopStartTime: loopStartTime,
			})
		}

		// Get tool definitions and call LLM
		toolDefs := a.getToolDefinitions()

		// Use lower temperature when tools are available (more deterministic tool calls)
		llmCtx := runCtx
		if len(toolDefs) > 0 {
			llmCtx = llm.WithTemperature(runCtx, 0.1)
		}

		stream := a.llm.ChatStream(llmCtx, a.contextMgr.GetMessagesWithMasking(), toolDefs, a.getSystemPrompt())

		var response llm.Response
		var textContent strings.Builder
		var toolCalls []llm.ToolCall
		interrupted := false

		// Process stream with interrupt-aware select when available
		if hasInterrupt {
			// TUI-style: use select to race between chunks, cancel, and force-stop
		streamLoopInterruptible:
			for {
				select {
				case <-forceStop:
					interrupted = true
					break streamLoopInterruptible

				case <-runCtx.Done():
					interrupted = true
					break streamLoopInterruptible

				case chunk, ok := <-stream:
					if !ok {
						break streamLoopInterruptible
					}
					if err := a.processStreamChunk(chunk, output, &textContent, &toolCalls); err != nil {
						if runCtx.Err() != nil {
							interrupted = true
							break streamLoopInterruptible
						}
						output.StreamDone()
						return err
					}
				}
			}
		} else {
			// CLI-style: simple range over channel (no interrupt support)
			for chunk := range stream {
				if err := a.processStreamChunk(chunk, output, &textContent, &toolCalls); err != nil {
					return err
				}
			}
		}

		// Handle interruption
		if interrupted {
			output.Warning("Interrupted by user")
			output.StreamDone()
			return nil
		}

		response.Content = textContent.String()
		response.ToolCalls = toolCalls

		// Add assistant message with tool calls
		if response.Content != "" || len(response.ToolCalls) > 0 {
			a.contextMgr.AddMessage(llm.Message{
				Role:      "assistant",
				Content:   response.Content,
				ToolCalls: response.ToolCalls,
			})
		}

		// Update context stats
		a.updateContextStats(runCtx, output)

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			return nil
		}

		// Architect/Editor split: on the first iteration, if the model wants to write files,
		// engage the architect (genius model) to plan, then editor (smart model) to execute.
		if i == 0 && a.config.Agent.ArchitectEditorMode && hasWriteToolCalls(response.ToolCalls) && a.agentMode != tui.ModeAsk {
			plan, archErr := a.architectPhase(runCtx, output)
			if archErr != nil {
				// Fall through to normal execution on failure
				logWarn("Architect phase failed, falling back to normal: %v", archErr)
			} else {
				if edErr := a.editorPhase(runCtx, plan, output, input); edErr != nil {
					return edErr
				}
				return nil
			}
		}

		// Check for force-stop before executing tools
		select {
		case <-forceStop:
			output.Warning("Force stopped by user")
			output.StreamDone()
			return nil
		default:
		}

		// Execute tool calls
		toolResults := a.toolExecutor.ExecuteToolCalls(runCtx, response.ToolCalls, output, input)

		// Track consecutive iterations where ALL tool calls had parse errors
		allParseErrors := true
		for _, result := range toolResults {
			if !result.Error || !strings.Contains(result.Result, "could not parse arguments") {
				allParseErrors = false
			}
		}
		if allParseErrors && len(toolResults) > 0 {
			consecutiveParseErrors++
		} else {
			consecutiveParseErrors = 0
		}
		if consecutiveParseErrors >= 3 {
			output.Warning("Too many consecutive tool call parse errors — stopping")
			output.StreamDone()
			return nil
		}

		// Add individual tool result messages
		for _, result := range toolResults {
			a.contextMgr.AddMessage(llm.Message{
				Role:       "tool",
				Content:    result.Result,
				ToolCallID: result.ToolCallID,
			})
		}
	}

	return vecerr.MaxIterationsReached(maxIterations)
}

// processStreamChunk handles a single stream chunk for both CLI and TUI modes.
// Returns an error only for error chunks; nil means continue processing.
func (a *Agent) processStreamChunk(chunk llm.StreamChunk, output AgentOutput, textContent *strings.Builder, toolCalls *[]llm.ToolCall) error {
	switch chunk.Type {
	case "text":
		output.StreamText(chunk.Text)
		textContent.WriteString(chunk.Text)

	case "thinking":
		output.StreamThinking(chunk.Text)

	case "tool_call":
		if chunk.ToolCall != nil {
			*toolCalls = append(*toolCalls, *chunk.ToolCall)
		}

	case "done":
		// Feed calibrator with actual vs estimated token counts
		if chunk.Usage != nil && a.calibrator != nil {
			estimated := a.contextMgr.GetStats().UsedTokens
			actual := int(chunk.Usage.InputTokens)
			a.calibrator.Record(estimated, actual)
		}
		if chunk.Usage != nil {
			if su, ok := output.(StreamUsageSupport); ok {
				su.StreamDoneWithUsage(chunk.Usage.InputTokens, chunk.Usage.OutputTokens)
			} else {
				output.StreamDone()
			}
		} else {
			output.StreamDone()
		}

	case "error":
		if chunk.Error != nil {
			return chunk.Error
		}
	}

	return nil
}

// StreamUsageSupport is an optional interface for outputs that can accept usage data with StreamDone.
type StreamUsageSupport interface {
	StreamDoneWithUsage(inputTokens, outputTokens int64)
}

// updateContextStats checks context usage and handles auto-compact/warnings.
// Works for both CLI and TUI modes via the AgentOutput interface.
// If output implements StatsSupport, also updates the UI stats display.
func (a *Agent) updateContextStats(ctx context.Context, output AgentOutput) {
	stats := a.contextMgr.GetStats()

	// Adjust token estimate using calibrator if available
	if a.calibrator != nil {
		adjusted := a.calibrator.Adjust(stats.UsedTokens)
		if adjusted != stats.UsedTokens {
			stats.UsedTokens = adjusted
			stats.UsagePercent = float64(adjusted) / float64(stats.ContextWindow)
			stats.NeedsCompaction = stats.UsagePercent >= a.config.Context.AutoCompactThreshold
			stats.NeedsWarning = stats.UsagePercent >= a.config.Context.WarnThreshold
		}
	}

	// Update TUI stats display if supported
	if statsOut, ok := output.(StatsSupport); ok {
		statsOut.UpdateContextStats(
			stats.UsagePercent,
			stats.UsedTokens,
			stats.ContextWindow,
			stats.NeedsWarning,
		)
	}

	// Handle auto-compact at threshold
	if stats.NeedsCompaction && a.config.Context.EnableAutoCompact {
		output.Warning(fmt.Sprintf("Context at %.0f%% - auto-compacting...", stats.UsagePercent*100))
		if err := a.compactConversation(ctx, "", output); err != nil {
			output.Warning("Auto-compact failed: " + err.Error())
		} else if _, ok := output.(StatsSupport); !ok {
			// CLI mode: show success message since compactConversation only updates TUI stats
			newStats := a.contextMgr.GetStats()
			output.Success(fmt.Sprintf("Compacted to %.0f%%", newStats.UsagePercent*100))
		}
		return
	}

	// Show warning at threshold (once per high-usage session)
	if stats.NeedsWarning && !a.shownContextWarning {
		output.Warning(fmt.Sprintf("Context at %.0f%% - consider using /compact", stats.UsagePercent*100))
		a.shownContextWarning = true
	}
}

// compactConversation compacts the conversation history.
// If output implements StatsSupport, updates the stats display after compaction.
func (a *Agent) compactConversation(ctx context.Context, focusPrompt string, output AgentOutput) error {
	messages := a.contextMgr.GetMessages()
	if len(messages) == 0 {
		if output != nil {
			output.Info("No conversation to compact")
		}
		return nil
	}

	preserveLast := a.contextMgr.GetPreserveLast()

	result, err := a.compactor.Compact(ctx, ctxmgr.CompactRequest{
		Messages:     messages,
		FocusPrompt:  focusPrompt,
		PreserveLast: preserveLast,
	})
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}

	// Replace history with summary
	a.contextMgr.ReplaceWithSummary(result.Summary, result.PreservedMsgs)

	// Reset warning flag after compaction
	a.shownContextWarning = false

	// Update stats display if supported
	if output != nil {
		if statsOut, ok := output.(StatsSupport); ok {
			newStats := a.contextMgr.GetStats()
			statsOut.UpdateContextStats(
				newStats.UsagePercent,
				newStats.UsedTokens,
				newStats.ContextWindow,
				newStats.NeedsWarning,
			)
			output.Success(fmt.Sprintf("Compacted: %d msgs summarized, saved ~%d tokens (now at %.0f%%)",
				result.MessagesSummarized, result.TokensSaved, newStats.UsagePercent*100))
		}
	}

	return nil
}
