package permissions

import (
	"fmt"
	"strings"
	"sync"

	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// Mode defines the permission checking mode
type Mode int

const (
	ModeAsk    Mode = iota // Prompt for write/execute, auto-approve reads
	ModeAuto               // Approve everything automatically
	ModeStrict             // Prompt for everything including reads
)

func (m Mode) String() string {
	switch m {
	case ModeAsk:
		return "ask"
	case ModeAuto:
		return "auto"
	case ModeStrict:
		return "strict"
	default:
		return "unknown"
	}
}

// Decision represents a permission decision
type Decision int

const (
	DecisionAllow      Decision = iota // Allow this time
	DecisionAlwaysAllow                // Always allow this tool
	DecisionDeny                       // Deny this time
	DecisionNeverAllow                 // Never allow this tool
)

// InputHandler interface for getting user input
type InputHandler interface {
	ReadLine(prompt string) (string, error)
}

// OutputHandler interface for displaying output
type OutputHandler interface {
	PermissionPrompt(toolName string, level tools.PermissionLevel, description string)
}

// Policy manages permission checking
type Policy struct {
	mode    Mode
	input   InputHandler
	output  OutputHandler
	cache   map[string]Decision
	cacheMu sync.RWMutex
}

// NewPolicy creates a new permission policy
func NewPolicy(mode Mode, input InputHandler, output OutputHandler) *Policy {
	return &Policy{
		mode:   mode,
		input:  input,
		output: output,
		cache:  make(map[string]Decision),
	}
}

// Check checks if a tool execution is allowed
func (p *Policy) Check(toolName string, level tools.PermissionLevel, description string) (bool, error) {
	// Auto mode always allows
	if p.mode == ModeAuto {
		return true, nil
	}

	// Check cache first
	p.cacheMu.RLock()
	if decision, ok := p.cache[toolName]; ok {
		p.cacheMu.RUnlock()
		switch decision {
		case DecisionAlwaysAllow:
			return true, nil
		case DecisionNeverAllow:
			return false, nil
		}
	} else {
		p.cacheMu.RUnlock()
	}

	// In ask mode, auto-approve reads
	if p.mode == ModeAsk && level == tools.PermissionRead {
		return true, nil
	}

	// Need to prompt user
	return p.promptUser(toolName, level, description)
}

// promptUser asks the user for permission
func (p *Policy) promptUser(toolName string, level tools.PermissionLevel, description string) (bool, error) {
	p.output.PermissionPrompt(toolName, level, description)

	prompt := "[y]es / [n]o / [a]lways / ne[v]er: "
	response, err := p.input.ReadLine(prompt)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		return true, nil

	case "n", "no":
		return false, nil

	case "a", "always":
		p.cacheMu.Lock()
		p.cache[toolName] = DecisionAlwaysAllow
		p.cacheMu.Unlock()
		return true, nil

	case "v", "never":
		p.cacheMu.Lock()
		p.cache[toolName] = DecisionNeverAllow
		p.cacheMu.Unlock()
		return false, nil

	default:
		// Default to deny for unrecognized input
		fmt.Println("Unrecognized response, denying.")
		return false, nil
	}
}

// GetMode returns the current permission mode
func (p *Policy) GetMode() Mode {
	return p.mode
}

// SetMode changes the permission mode
func (p *Policy) SetMode(mode Mode) {
	p.mode = mode
}

// ClearCache clears all cached decisions
func (p *Policy) ClearCache() {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	p.cache = make(map[string]Decision)
}

// GetCachedDecision returns a cached decision if it exists
func (p *Policy) GetCachedDecision(toolName string) (Decision, bool) {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()
	decision, ok := p.cache[toolName]
	return decision, ok
}
