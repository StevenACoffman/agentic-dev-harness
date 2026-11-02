package toolreg

import (
	"regexp"
	"strings"
)

// commandHazard is one command shape and what it means.
type commandHazard struct {
	re  *regexp.Regexp
	why string
}

// Hazards reports the shapes in a tool's shell command that `tool doctor` warns about:
// code fetched and executed in one step, an eval of an expansion, decoded text piped to
// a shell, and environment variables that change what every child process loads or
// trusts. Each result names the shape in words a maintainer can act on.
//
// These are warnings routed to a human, never failures: a registry may legitimately
// bootstrap a toolchain with `curl … | sh`, and only its maintainer can say so. The
// detector is deliberately small. It names a handful of exact shapes rather than porting
// a large regex corpus whose misses would read as a clean bill of health.
//
// Requires: nothing; run may be empty.
// Ensures:  one entry per shape present, in a fixed order; it is pure.
func Hazards(run string) []string {
	var out []string
	for _, h := range commandHazards() {
		if h.re.MatchString(run) {
			out = append(out, h.why)
		}
	}
	for _, name := range assignedVariables(run) {
		if why, ok := dangerousVariable(name); ok {
			out = append(out, why)
		}
	}
	return out
}

// commandHazards builds the command shapes. Built per call: adh keeps no package state,
// and doctor runs once per invocation.
func commandHazards() []commandHazard {
	const shell = `(?:ba|z|da|k)?sh`
	return []commandHazard{
		{
			regexp.MustCompile(`\b(?:curl|wget)\b[^|;&]*\|\s*(?:sudo\s+)?` + shell + `\b`),
			"downloads code and pipes it straight into a shell",
		},
		{
			regexp.MustCompile(`\$\(\s*(?:curl|wget)\b`),
			"executes the output of a download via $(curl …)",
		},
		{
			regexp.MustCompile(`\beval\s+["']?\$`),
			"evals an expansion, so whatever the variable holds runs",
		},
		{
			regexp.MustCompile(
				`\bbase64\s+(?:-d|--decode|-D)\b[^|;&]*\|\s*(?:sudo\s+)?` + shell + `\b`,
			),
			"decodes base64 and pipes it into a shell, hiding what runs",
		},
	}
}

// assignedVariables returns the names assigned in run as `NAME=…`, either as a command
// prefix or after `export`, in order of appearance.
func assignedVariables(run string) []string {
	re := regexp.MustCompile(`(?:^|[\s;&|])(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=`)
	var names []string
	for _, m := range re.FindAllStringSubmatch(run, -1) {
		names = append(names, m[1])
	}
	return names
}

// dangerousVariable reports why setting name for a tool's command is a hazard: each one
// makes every process the command starts load code, or trust a network path, that the
// command itself does not name.
func dangerousVariable(name string) (string, bool) {
	upper := strings.ToUpper(name)
	switch upper {
	case "LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES", "DYLD_LIBRARY_PATH":
		return "sets " + name + ", which injects a library into every child process", true
	case "NODE_OPTIONS", "BASH_ENV", "ENV", "PYTHONSTARTUP", "PERL5OPT":
		return "sets " + name + ", which runs extra code in every interpreter it starts", true
	case "GIT_SSH_COMMAND", "GIT_SSH", "GIT_EXEC_PATH":
		return "sets " + name + ", which replaces the program git runs", true
	case "SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS", "REQUESTS_CA_BUNDLE",
		"CURL_CA_BUNDLE", "GIT_SSL_NO_VERIFY", "NODE_TLS_REJECT_UNAUTHORIZED":
		return "sets " + name + ", which changes which TLS certificates are trusted", true
	}
	if strings.HasSuffix(upper, "_PROXY") && upper != "NO_PROXY" {
		return "sets " + name + ", which routes traffic through a proxy", true
	}
	return "", false
}
