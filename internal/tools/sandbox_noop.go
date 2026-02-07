package tools

// NoopSandbox is a fallback sandbox that passes commands through unchanged.
type NoopSandbox struct{}

func (s *NoopSandbox) Wrap(command string, projectDir string) (string, []string, error) {
	return "bash", []string{"-c", command}, nil
}

func (s *NoopSandbox) Available() bool {
	return true
}

func (s *NoopSandbox) Name() string {
	return "noop"
}
