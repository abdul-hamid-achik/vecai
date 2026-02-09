package tui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// copyToClipboard copies text to system clipboard
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = stdin.Close()
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}

// getLastAssistantResponse returns the content of the last assistant block
func getLastAssistantResponse(blocks []ContentBlock) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].Type == BlockAssistant {
			return blocks[i].Content
		}
	}
	return ""
}

// codeBlockRegex matches fenced code blocks
var codeBlockRegex = regexp.MustCompile("(?s)```[^\n]*\n(.*?)```")

// getLastCodeBlock returns the content of the last code block in conversation
func getLastCodeBlock(blocks []ContentBlock) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].Type == BlockAssistant {
			matches := codeBlockRegex.FindAllStringSubmatch(blocks[i].Content, -1)
			if len(matches) > 0 {
				// Return the last code block's content
				return strings.TrimSpace(matches[len(matches)-1][1])
			}
		}
	}
	return ""
}
