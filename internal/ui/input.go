package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// InputHandler handles user input
type InputHandler struct {
	reader *bufio.Reader
}

// NewInputHandler creates a new input handler
func NewInputHandler() *InputHandler {
	return &InputHandler{
		reader: bufio.NewReader(os.Stdin),
	}
}

// ReadLine reads a single line of input
func (h *InputHandler) ReadLine(prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := h.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ReadMultiLine reads multiple lines until an empty line or Ctrl+D
func (h *InputHandler) ReadMultiLine(prompt string) (string, error) {
	fmt.Println(prompt)
	fmt.Println("(Enter an empty line or Ctrl+D to finish)")

	var lines []string
	for {
		line, err := h.reader.ReadString('\n')
		if err != nil {
			// EOF - return what we have
			break
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// ReadInput reads input, handling both single and multi-line cases
// Multi-line mode is triggered when pasting (multiple lines detected quickly)
func (h *InputHandler) ReadInput(prompt string) (string, error) {
	fmt.Print(prompt)

	// Read first line
	firstLine, err := h.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	firstLine = strings.TrimRight(firstLine, "\r\n")

	// Check if there's more data buffered (indicates paste)
	if h.reader.Buffered() > 0 {
		// Multi-line paste detected
		var lines []string
		lines = append(lines, firstLine)

		for h.reader.Buffered() > 0 {
			line, err := h.reader.ReadString('\n')
			if err != nil {
				break
			}
			lines = append(lines, strings.TrimRight(line, "\r\n"))
		}

		return strings.Join(lines, "\n"), nil
	}

	return firstLine, nil
}

// Confirm asks for a yes/no confirmation
func (h *InputHandler) Confirm(prompt string, defaultYes bool) (bool, error) {
	suffix := " [y/N]: "
	if defaultYes {
		suffix = " [Y/n]: "
	}

	response, err := h.ReadLine(prompt + suffix)
	if err != nil {
		return false, err
	}

	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" {
		return defaultYes, nil
	}

	return response == "y" || response == "yes", nil
}

// Select presents options and returns the selected index
func (h *InputHandler) Select(prompt string, options []string) (int, error) {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  [%d] %s\n", i+1, opt)
	}

	for {
		response, err := h.ReadLine("Enter choice: ")
		if err != nil {
			return -1, err
		}

		// Parse number
		var choice int
		if _, err := fmt.Sscanf(response, "%d", &choice); err != nil {
			fmt.Println("Please enter a number.")
			continue
		}

		if choice < 1 || choice > len(options) {
			fmt.Printf("Please enter a number between 1 and %d.\n", len(options))
			continue
		}

		return choice - 1, nil
	}
}

// ReadPassword reads a password without echoing
func (h *InputHandler) ReadPassword(prompt string) (string, error) {
	// Note: For real password reading, use golang.org/x/term
	// This is a simple version that just reads normally
	return h.ReadLine(prompt)
}

// Clear clears the terminal screen
func (h *InputHandler) Clear() {
	fmt.Print("\033[H\033[2J")
}
