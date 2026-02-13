//go:build linux

package tools

import "os/exec"

// LinuxSandbox uses bubblewrap (bwrap) to sandbox commands on Linux.
type LinuxSandbox struct{}

func (s *LinuxSandbox) Wrap(command string, projectDir string) (string, []string, error) {
	args := []string{
		"--unshare-pid",
		"--die-with-parent",

		// Bind-mount essential read-only paths
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/etc", "/etc",
		"--symlink", "usr/lib64", "/lib64",

		// Proc and dev
		"--proc", "/proc",
		"--dev", "/dev",

		// Temp directory
		"--tmpfs", "/tmp",

		// Bind-mount project directory read-write
		"--bind", projectDir, projectDir,

		// Set working directory
		"--chdir", projectDir,

		// Run bash with the command
		"bash", "-c", command,
	}
	return "bwrap", args, nil
}

func (s *LinuxSandbox) Available() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

func (s *LinuxSandbox) Name() string {
	return "linux-bwrap"
}

func init() {
	registerPlatformSandbox(&LinuxSandbox{})
}
