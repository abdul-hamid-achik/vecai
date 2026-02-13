package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// dangerousPatterns contains shell patterns that are blocked for safety.
// These prevent the LLM from executing destructive or exfiltration commands.
var dangerousPatterns = []string{
	"rm -rf /",
	"rm -rf /*",
	"mkfs.",
	"dd if=/dev/",
	":(){:|:&};:", // fork bomb
	"> /dev/sd",
	"chmod -r 777 /",
	"shutdown",
	"reboot",
	"init 0",
	"init 6",
	"find / -delete",
	"find / -exec rm",
}

// networkExfilPatterns are blocked when commands appear to exfiltrate data
var networkExfilPatterns = []string{
	"/dev/tcp/",
	"/dev/udp/",
}

// obfuscationPatterns detect attempts to bypass the blocklist via encoding or evasion.
var obfuscationPatterns = []*regexp.Regexp{
	// base64 decode piped to execution: echo ... | base64 -d | bash (or sh)
	regexp.MustCompile(`base64\s+(-d|--decode)\s*\|\s*(bash|sh|zsh|exec)`),
	regexp.MustCompile(`base64\s+(-d|--decode)`),

	// hex-encoded execution via xxd, printf with \x escapes piped to shell
	regexp.MustCompile(`xxd\s+-r.*\|\s*(bash|sh|zsh|exec)`),
	regexp.MustCompile(`printf\s+.*\\x[0-9a-fA-F].*\|\s*(bash|sh|zsh|exec)`),

	// python/perl/ruby one-liner evasion
	regexp.MustCompile(`python[23]?\s+-c\s+.*__(import|eval|exec)__`),
	regexp.MustCompile(`perl\s+-e\s+.*system\s*\(`),

	// curl/wget piped to shell execution
	regexp.MustCompile(`(curl|wget)\s+.*\|\s*(bash|sh|zsh|exec)`),
}

// evasionPatterns detect attempts to smuggle dangerous commands through
// variable interpolation, backslash insertion, or command substitution.
var evasionPatterns = []*regexp.Regexp{
	// Backslash-inserted commands: r\m, s\hutdown, re\boot
	regexp.MustCompile(`r\\m\s`),
	regexp.MustCompile(`s\\hutdown`),
	regexp.MustCompile(`re\\boot`),
	regexp.MustCompile(`mk\\fs`),

	// Variable-based evasion: $'\x72\x6d' or ${cmd} tricks
	regexp.MustCompile(`\$'\\x[0-9a-fA-F]{2}`),

	// eval with variable expansion
	regexp.MustCompile(`eval\s+.*\$`),

	// Nested command substitution executing dangerous commands
	regexp.MustCompile(`\$\(.*\brm\b.*-rf\b`),
	regexp.MustCompile("`.*(\\brm\\b.*-rf\\b)"),
}

// CheckCommandSafety checks a command against all blocklist layers.
// Returns nil if the command is safe, or an error describing why it was blocked.
func CheckCommandSafety(command string) error {
	lowerCmd := strings.ToLower(strings.TrimSpace(command))

	// Layer 1: Simple dangerous pattern matching
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerCmd, pattern) {
			return fmt.Errorf("command blocked: contains dangerous pattern %q", pattern)
		}
	}

	// Layer 2: Network exfiltration patterns (case-sensitive)
	for _, pattern := range networkExfilPatterns {
		if strings.Contains(command, pattern) {
			return fmt.Errorf("command blocked: contains network exfiltration pattern %q", pattern)
		}
	}

	// Layer 3: Obfuscation detection
	for _, re := range obfuscationPatterns {
		if re.MatchString(command) {
			return fmt.Errorf("command blocked: contains obfuscated/encoded command execution pattern")
		}
	}

	// Layer 4: Evasion pattern detection
	for _, re := range evasionPatterns {
		if re.MatchString(command) {
			return fmt.Errorf("command blocked: contains command evasion pattern")
		}
	}

	return nil
}
