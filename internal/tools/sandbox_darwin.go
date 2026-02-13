//go:build darwin

package tools

import (
	"fmt"
	"os/exec"
)

// DarwinSandbox uses macOS sandbox-exec with a Seatbelt profile.
type DarwinSandbox struct{}

func (s *DarwinSandbox) Wrap(command string, projectDir string) (string, []string, error) {
	profile := seatbeltProfile(projectDir)
	return "sandbox-exec", []string{"-p", profile, "bash", "-c", command}, nil
}

func (s *DarwinSandbox) Available() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func (s *DarwinSandbox) Name() string {
	return "darwin-seatbelt"
}

// seatbeltProfile generates a Seatbelt profile that restricts filesystem access
// to the project directory, temp dirs, and essential system paths.
func seatbeltProfile(projectDir string) string {
	return fmt.Sprintf(`(version 1)
(deny default)
(allow process-exec)
(allow process-fork)
(allow sysctl-read)
(allow signal)
(allow mach-lookup)
(allow system-socket)

; Allow read access to system essentials
(allow file-read*
  (subpath "/usr")
  (subpath "/bin")
  (subpath "/sbin")
  (subpath "/Library")
  (subpath "/System")
  (subpath "/private/var")
  (subpath "/private/etc")
  (subpath "/dev")
  (subpath "/opt/homebrew")
  (subpath "/Applications")
  (literal "/etc")
  (literal "/tmp")
  (literal "/var"))

; Allow read-write to project directory
(allow file-read* (subpath %q))
(allow file-write* (subpath %q))

; Allow read-write to temp directories
(allow file-read* (subpath "/tmp"))
(allow file-write* (subpath "/tmp"))
(allow file-read* (subpath "/private/tmp"))
(allow file-write* (subpath "/private/tmp"))

; Allow read access to Go toolchain and specific home config dirs
(allow file-read* (subpath (string-append (param "HOME") "/go")))
(allow file-read* (subpath (string-append (param "HOME") "/.cache")))
(allow file-read* (subpath (string-append (param "HOME") "/.config/go")))
(allow file-read* (subpath (string-append (param "HOME") "/Library/Caches")))

; Deny access to sensitive credential directories
(deny file-read* (subpath (string-append (param "HOME") "/.ssh")))
(deny file-read* (subpath (string-append (param "HOME") "/.aws")))
(deny file-read* (subpath (string-append (param "HOME") "/.gnupg")))
`, projectDir, projectDir)
}

func init() {
	registerPlatformSandbox(&DarwinSandbox{})
}
