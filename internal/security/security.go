// Package security provides command validation and environment sanitization.
package security

import (
	"net/url"
	"os"
	"regexp"
	"strings"
)

// dangerousPatterns are compiled regexes for commands that must never execute.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-rf\s+/`),
	regexp.MustCompile(`mkfs\.`),
	regexp.MustCompile(`dd\s+if=`),
	regexp.MustCompile(`:\(\)\{\s*:\|:&\s*\};:`),
	regexp.MustCompile(`curl.*\|\s*sh`),
	regexp.MustCompile(`wget.*\|\s*sh`),
	regexp.MustCompile(`chmod\s+-R\s+777\s+/`),
	regexp.MustCompile(`>\s*/dev/sd`),
}

// sanitizedEnvVars are environment variable names stripped from subprocess environments.
var sanitizedEnvVars = []string{
	"AWS_SECRET_ACCESS_KEY", "AWS_ACCESS_KEY_ID", "AWS_SESSION_TOKEN",
	"GITHUB_TOKEN", "GH_TOKEN",
	"OPENAI_API_KEY", "ANTHROPIC_API_KEY",
	"DATABASE_URL", "SECRET_KEY", "DJANGO_SECRET_KEY",
	"ENCRYPTION_KEY", "PRIVATE_KEY",
	"STRIPE_SECRET_KEY", "SENDGRID_API_KEY",
	"TWILIO_AUTH_TOKEN",
	"SLACK_TOKEN", "SLACK_BOT_TOKEN",
	"NPM_TOKEN", "PYPI_TOKEN",
}

var localhostHosts = map[string]struct{}{
	"localhost": {},
	"127.0.0.1": {},
	"::1":       {},
	"[::1]":     {},
	"0.0.0.0":   {},
}

var modelNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]*$`)

const maxModelNameLength = 256

var whitespaceRE = regexp.MustCompile(`\s+`)

// IsCommandDangerous checks if a shell command matches any dangerous pattern.
// Normalizes whitespace before matching to prevent evasion via extra spaces.
func IsCommandDangerous(cmd string) bool {
	normalized := whitespaceRE.ReplaceAllString(strings.TrimSpace(cmd), " ")
	for _, pat := range dangerousPatterns {
		if pat.MatchString(cmd) || pat.MatchString(normalized) {
			return true
		}
	}
	return false
}

// SanitizedEnv returns a copy of the current environment with sensitive
// variables removed.
func SanitizedEnv() []string {
	blocked := make(map[string]struct{}, len(sanitizedEnvVars))
	for _, k := range sanitizedEnvVars {
		blocked[k] = struct{}{}
	}

	env := os.Environ()
	result := make([]string, 0, len(env))
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if _, skip := blocked[key]; !skip {
			result = append(result, e)
		}
	}
	return result
}

// ValidateOllamaHost checks that a URL points to a localhost address.
func ValidateOllamaHost(rawURL string) bool {
	if strings.Contains(rawURL, "@") {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	_, ok := localhostHosts[host]
	return ok
}

// ValidateModelName checks that a model name is safe and well-formed.
func ValidateModelName(name string) bool {
	if name == "" || len(name) > maxModelNameLength {
		return false
	}
	return modelNameRE.MatchString(name)
}
