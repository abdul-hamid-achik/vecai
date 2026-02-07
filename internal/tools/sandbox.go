package tools

// Sandbox wraps command execution with OS-level sandboxing
type Sandbox interface {
	// Wrap takes a command and returns the sandboxed executable, args, and error
	Wrap(command string, projectDir string) (string, []string, error)
	// Available returns true if this sandbox is usable on the current system
	Available() bool
	// Name returns the sandbox implementation name
	Name() string
}

// platformSandboxes holds sandboxes registered by platform-specific init() functions.
var platformSandboxes []Sandbox

// registerPlatformSandbox is called from platform-specific init() to register a sandbox.
func registerPlatformSandbox(s Sandbox) {
	platformSandboxes = append(platformSandboxes, s)
}

// DetectSandbox returns the best available sandbox for the current OS.
// It checks platform-specific sandboxes first, falling back to NoopSandbox.
func DetectSandbox() Sandbox {
	for _, s := range platformSandboxes {
		if s.Available() {
			return s
		}
	}
	return &NoopSandbox{}
}
