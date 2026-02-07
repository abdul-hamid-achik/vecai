package tools

import (
	"testing"
)

func TestDarwinSandbox_Available(t *testing.T) {
	s := &DarwinSandbox{}
	// On macOS, sandbox-exec should be available
	if !s.Available() {
		t.Log("sandbox-exec not found on this macOS system")
	}
	if s.Name() != "darwin-seatbelt" {
		t.Errorf("expected name 'darwin-seatbelt', got %q", s.Name())
	}
}

func TestDarwinSandbox_Wrap(t *testing.T) {
	s := &DarwinSandbox{}
	exe, args, err := s.Wrap("echo hello", "/tmp/project")
	if err != nil {
		t.Fatalf("Wrap returned error: %v", err)
	}
	if exe != "sandbox-exec" {
		t.Errorf("expected exe 'sandbox-exec', got %q", exe)
	}
	// Args should be: -p <profile> bash -c <command>
	if len(args) < 4 {
		t.Fatalf("expected at least 4 args, got %d: %v", len(args), args)
	}
	if args[0] != "-p" {
		t.Errorf("expected first arg '-p', got %q", args[0])
	}
	// profile is args[1]
	if args[2] != "bash" {
		t.Errorf("expected 'bash' in args, got %q", args[2])
	}
	if args[3] != "-c" {
		t.Errorf("expected '-c' in args, got %q", args[3])
	}
	if args[4] != "echo hello" {
		t.Errorf("expected command in args, got %q", args[4])
	}
}
