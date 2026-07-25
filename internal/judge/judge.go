// Package judge is the deterministic rule-judge for scoring an output against a
// set of checks (SPEC-ADDITIONS §11 test-checks; ported from skillsaw's judge,
// itself a port of SkillOpt-Sleep's judges.py). hard = 1.0 iff every check
// passes, soft = passed/total. It is pure — values in, values out, no I/O and no
// model — so it carries the deterministic first line of behavioral scoring, with
// a model reserved only for what a rule cannot decide.
package judge

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Rule-check operators (a closed set).
const (
	OpSectionPresent Op = "section_present" // a heading line contains arg
	OpRegex          Op = "regex"           // arg is an RE2 pattern that matches
	OpContains       Op = "contains"        // arg is a substring of the output
	OpMaxChars       Op = "max_chars"       // rune count <= atoi(arg)
	OpMinChars       Op = "min_chars"       // rune count >= atoi(arg)
	OpToolCalled     Op = "tool_called"     // output names the tool (contains arg)
)

// Op is a rule-check operator.
type Op string

// Check is one rule: an operator applied to an argument.
type Check struct {
	Op  Op     `json:"op"`
	Arg string `json:"arg"`
}

// Result is the outcome of scoring an output against a set of checks.
type Result struct {
	Hard float64  `json:"hard"` // 1.0 iff every check passes, else 0.0
	Soft float64  `json:"soft"` // passed / total
	Why  []string `json:"why"`  // one line per check: pass/fail + reason
}

// Score evaluates output against checks. It rejects an empty check set with
// EINVALID rather than returning a vacuous perfect score: judging nothing is a
// caller error, not a pass.
func Score(output string, checks []Check) (Result, error) {
	if len(checks) == 0 {
		return Result{}, &adh.Error{Code: adh.EINVALID, Message: "judge: no checks provided"}
	}
	res := Result{Why: make([]string, 0, len(checks))}
	passed := 0
	for _, check := range checks {
		ok, why := check.eval(output)
		res.Why = append(res.Why, why)
		if ok {
			passed++
		}
	}
	res.Soft = float64(passed) / float64(len(checks))
	if passed == len(checks) {
		res.Hard = 1.0
	}
	return res, nil
}

// eval applies one check to the output, returning pass/fail and a reason. A
// malformed check (bad regex, non-numeric length) fails with an explanatory
// reason rather than panicking — an authoring error is surfaced, not fatal.
func (c Check) eval(output string) (bool, string) {
	switch c.Op {
	case OpSectionPresent:
		return evalSectionPresent(output, c.Arg)
	case OpRegex:
		return evalRegex(output, c.Arg)
	case OpContains:
		return boolWhy(strings.Contains(output, c.Arg), "contains", c.Arg)
	case OpToolCalled:
		return boolWhy(strings.Contains(output, c.Arg), "tool_called", c.Arg)
	case OpMaxChars:
		return evalLen(output, c.Arg, true)
	case OpMinChars:
		return evalLen(output, c.Arg, false)
	default:
		return false, "unknown op " + strconv.Quote(string(c.Op))
	}
}

func evalSectionPresent(output, arg string) (bool, string) {
	want := strings.ToLower(arg)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && strings.Contains(strings.ToLower(trimmed), want) {
			return true, "section_present: found heading containing " + strconv.Quote(arg)
		}
	}
	return false, "section_present: no heading contains " + strconv.Quote(arg)
}

func evalRegex(output, arg string) (bool, string) {
	re, err := regexp.Compile(arg)
	if err != nil {
		return false, "regex: invalid pattern " + strconv.Quote(arg) + ": " + err.Error()
	}
	if re.MatchString(output) {
		return true, "regex: matched " + strconv.Quote(arg)
	}
	return false, "regex: no match for " + strconv.Quote(arg)
}

func evalLen(output, arg string, atMost bool) (bool, string) {
	limit, err := strconv.Atoi(arg)
	if err != nil {
		return false, "length: invalid arg " + strconv.Quote(arg)
	}
	got := utf8.RuneCountInString(output)
	if atMost {
		return boolWhy(
			got <= limit,
			"max_chars",
			strconv.Itoa(limit)+" (got "+strconv.Itoa(got)+")",
		)
	}
	return boolWhy(got >= limit, "min_chars", strconv.Itoa(limit)+" (got "+strconv.Itoa(got)+")")
}

func boolWhy(ok bool, op, detail string) (bool, string) {
	verdict := "pass"
	if !ok {
		verdict = "fail"
	}
	return ok, op + ": " + verdict + " " + detail
}
