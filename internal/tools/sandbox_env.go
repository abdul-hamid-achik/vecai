package tools

import "os"

// allowedEnvVars is the allowlist of environment variables passed to sandboxed commands.
var allowedEnvVars = []string{
	"PATH",
	"HOME",
	"TERM",
	"GOPATH",
	"GOROOT",
	"TMPDIR",
}

// SanitizedEnv returns an environment slice containing only allowlisted variables.
func SanitizedEnv() []string {
	var env []string
	for _, key := range allowedEnvVars {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+val)
		}
	}
	return env
}
