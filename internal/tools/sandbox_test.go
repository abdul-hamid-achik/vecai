package tools

import (
	"os"
	"testing"
)

func TestDetectSandbox(t *testing.T) {
	s := DetectSandbox()
	if s == nil {
		t.Fatal("DetectSandbox returned nil")
	}
	// Should always return something (at minimum NoopSandbox)
	if s.Name() == "" {
		t.Error("sandbox Name() should not be empty")
	}
	if !s.Available() {
		t.Error("detected sandbox should report Available() == true")
	}
}

func TestNoopSandbox(t *testing.T) {
	s := &NoopSandbox{}

	if s.Name() != "noop" {
		t.Errorf("expected name 'noop', got %q", s.Name())
	}
	if !s.Available() {
		t.Error("NoopSandbox should always be available")
	}

	exe, args, err := s.Wrap("echo hello", "/some/dir")
	if err != nil {
		t.Fatalf("Wrap returned error: %v", err)
	}
	if exe != "bash" {
		t.Errorf("expected exe 'bash', got %q", exe)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "echo hello" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestNoopSandbox_PreservesCommand(t *testing.T) {
	s := &NoopSandbox{}
	cmds := []string{
		"ls -la",
		"go build ./...",
		"echo 'multi word' && cd /tmp",
	}
	for _, cmd := range cmds {
		exe, args, err := s.Wrap(cmd, "/project")
		if err != nil {
			t.Fatalf("Wrap(%q) error: %v", cmd, err)
		}
		if exe != "bash" {
			t.Errorf("expected bash, got %q", exe)
		}
		if args[1] != cmd {
			t.Errorf("expected command %q preserved, got %q", cmd, args[1])
		}
	}
}

func TestSanitizedEnv(t *testing.T) {
	// Set some known env vars
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("HOME", "/home/test")
	t.Setenv("TERM", "xterm")
	t.Setenv("SECRET_KEY", "should-not-appear")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "should-not-appear")

	env := SanitizedEnv()

	allowed := map[string]bool{
		"PATH": false, "HOME": false, "TERM": false,
		"GOPATH": false, "GOROOT": false, "TMPDIR": false,
	}

	for _, e := range env {
		parts := splitEnvVar(e)
		if _, ok := allowed[parts[0]]; ok {
			allowed[parts[0]] = true
		} else {
			t.Errorf("unexpected env var in sanitized env: %s", parts[0])
		}
	}

	// PATH, HOME, TERM were set, so they should be present
	for _, key := range []string{"PATH", "HOME", "TERM"} {
		if !allowed[key] {
			t.Errorf("expected %s in sanitized env", key)
		}
	}
}

func TestSanitizedEnv_ExcludesSecrets(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("DATABASE_URL", "postgres://...")
	t.Setenv("API_KEY", "key123")

	env := SanitizedEnv()
	for _, e := range env {
		parts := splitEnvVar(e)
		switch parts[0] {
		case "AWS_SECRET_ACCESS_KEY", "DATABASE_URL", "API_KEY":
			t.Errorf("secret env var %s should not be in sanitized env", parts[0])
		}
	}
}

func TestSanitizedEnv_MissingVarsOmitted(t *testing.T) {
	// Unset optional vars
	os.Unsetenv("GOPATH")
	os.Unsetenv("GOROOT")
	os.Unsetenv("TMPDIR")

	env := SanitizedEnv()
	for _, e := range env {
		parts := splitEnvVar(e)
		if parts[0] == "GOPATH" || parts[0] == "GOROOT" || parts[0] == "TMPDIR" {
			t.Errorf("unset var %s should not appear in sanitized env", parts[0])
		}
	}
}

// splitEnvVar splits "KEY=VALUE" into ["KEY", "VALUE"]
func splitEnvVar(e string) []string {
	for i, c := range e {
		if c == '=' {
			return []string{e[:i], e[i+1:]}
		}
	}
	return []string{e}
}
