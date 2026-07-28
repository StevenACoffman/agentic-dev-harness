// Package config resolves the harness configuration by SPEC §3 precedence:
// built-in defaults, then the user config, then the repo .adh/config.toml, then
// the .adh/autonomy runtime override, then ADH_* environment variables (each
// overriding the last); command-line flags override the result at the call site.
// resolve is a pure overlay of decoded TOML documents onto the defaults; Load is
// the thin shell that reads the files and env.
//
// Security (SPEC §3.2, §5.2): the approval phrase is NEVER sourced from env or
// file — ADH_APPROVAL_PHRASE is ignored — so the agent has no self-grant route.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
)

const (
	repoConfigFile = ".adh/config.toml"
	autonomyFile   = ".adh/autonomy"
	defaultLevel   = "L2"
	classReasoning = "reasoning"
	classFast      = "fast"
	// UnconfirmedLesson is the default disposition for a critic finding no artifact
	// confirmed (§19.2, §19.4): keep it as a §11 lesson candidate.
	UnconfirmedLesson = "lesson"
)

// Config is the resolved harness configuration (SPEC §3.1).
type Config struct {
	Autonomy string `toml:"autonomy"`
	Models   Models `toml:"models"`
	Gates    Gates  `toml:"gates"`
	Critic   Critic `toml:"critic"`
}

// Critic is the cold-critic policy (SPEC-ADDITIONS §19.4). GroundFrom and Deny
// declare the working set and the one denied input; both are enforced
// structurally today (the grounding assembly and the renderer), so they are
// recorded for documentation and future wiring. Unconfirmed is the disposition
// of a finding no artifact confirmed and is acted on by the eval command.
type Critic struct {
	GroundFrom  []string `toml:"ground_from"`
	Deny        []string `toml:"deny"`
	Unconfirmed string   `toml:"unconfirmed"`
}

// Models is the per-class model binding and the model-gate's judgment set.
type Models struct {
	Reasoning string        `toml:"reasoning"`
	Fast      string        `toml:"fast"`
	Gate      ModelGateConf `toml:"gate"`
}

// ModelGateConf lists the roles that must run on a reasoning-class model.
type ModelGateConf struct {
	JudgmentRoles []string `toml:"judgment_roles"`
}

// Gates holds the human-gate policy.
type Gates struct {
	ApprovalPhraseRequired bool `toml:"approval_phrase_required"`
}

// Defaults returns the built-in configuration: autonomy L2, the placeholder
// reasoning/fast model identifiers, the SPEC §5.1 judgment set, and an approval
// phrase required.
func Defaults() Config {
	return Config{
		Autonomy: defaultLevel,
		Models: Models{
			Reasoning: classReasoning,
			Fast:      classFast,
			Gate: ModelGateConf{
				JudgmentRoles: []string{
					string(adh.StageStrategy),
					string(adh.StageCritic),
					string(adh.StageEvaluation),
				},
			},
		},
		Gates: Gates{ApprovalPhraseRequired: true},
		Critic: Critic{
			GroundFrom:  []string{"diff", "proof", "acceptance_bar", "context"},
			Deny:        []string{"transcript"},
			Unconfirmed: UnconfirmedLesson,
		},
	}
}

// CriticUnconfirmed is the configured disposition for an unconfirmed critic
// finding (§19.2), defaulting to the §11 lesson candidate when unset.
func (c *Config) CriticUnconfirmed() string {
	if c.Critic.Unconfirmed == "" {
		return UnconfirmedLesson
	}
	return c.Critic.Unconfirmed
}

// Load resolves the configuration by precedence. getenv is injected so the
// precedence is testable without touching the process environment.
func Load(getenv func(string) string) (Config, error) {
	docs, err := configDocs(getenv)
	if err != nil {
		return Config{}, err
	}
	cfg, err := resolve(docs)
	if err != nil {
		return Config{}, err
	}
	if lvl := readAutonomyOverride(); lvl != "" {
		cfg.Autonomy = lvl
	}
	applyEnv(&cfg, getenv)
	return cfg, nil
}

// AutonomyLevel parses the resolved autonomy string, defaulting to L2 when it is
// unset or unparseable.
func (c *Config) AutonomyLevel() authority.Level {
	lvl, err := authority.ParseLevel(c.Autonomy)
	if err != nil {
		return authority.L2
	}
	return lvl
}

// JudgmentRoles is the model-gate's judgment set as typed stages, normalizing
// the SPEC aliases "eval" and "exec".
func (c *Config) JudgmentRoles() authority.JudgmentRoles {
	roles := make(authority.JudgmentRoles, len(c.Models.Gate.JudgmentRoles))
	for _, name := range c.Models.Gate.JudgmentRoles {
		roles[normalizeRole(name)] = true
	}
	return roles
}

// BaselineModels is the per-role model binding for a requalification epoch
// (§14): each judgment role binds to the reasoning-class identifier, the rest to
// the fast-class one.
func (c *Config) BaselineModels() map[string]string {
	judgment := c.JudgmentRoles()
	out := make(map[string]string, len(allRoles()))
	for _, role := range allRoles() {
		if judgment.Requires(role) {
			out[string(role)] = c.Models.Reasoning
			continue
		}
		out[string(role)] = c.Models.Fast
	}
	return out
}

// resolve overlays the TOML documents (lowest precedence first) onto the
// defaults. It is pure: value-in, value-out, no I/O. A malformed document is an
// EINVALID error, never a silent skip.
func resolve(docs [][]byte) (Config, error) {
	cfg := Defaults()
	for _, doc := range docs {
		if err := toml.Unmarshal(doc, &cfg); err != nil {
			return Config{}, &adh.Error{Op: "config.resolve", Err: err}
		}
	}
	return cfg, nil
}

// configDocs reads the config file layers lowest precedence first. ADH_CONFIG,
// when set, is the single explicit config file and replaces the search. A
// missing file is skipped; any other read error propagates.
func configDocs(getenv func(string) string) ([][]byte, error) {
	if explicit := getenv("ADH_CONFIG"); explicit != "" {
		return readLayers(explicit)
	}
	return readLayers(userConfigPath(getenv), repoConfigFile)
}

func readLayers(paths ...string) ([][]byte, error) {
	var docs [][]byte
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			return nil, &adh.Error{Op: "config.readLayers", Err: err}
		}
		docs = append(docs, data)
	}
	return docs, nil
}

// userConfigPath is $XDG_CONFIG_HOME/adh/config.toml, or empty when XDG is unset
// (the user layer is then skipped).
func userConfigPath(getenv func(string) string) string {
	xdg := getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		return ""
	}
	return filepath.Join(xdg, "adh", "config.toml")
}

// readAutonomyOverride returns the trimmed .adh/autonomy runtime level (what
// `autonomy set` writes), or empty when absent.
func readAutonomyOverride() string {
	data, err := os.ReadFile(autonomyFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// applyEnv overlays the ADH_* environment. Only ADH_AUTONOMY is honored;
// ADH_APPROVAL_PHRASE is deliberately ignored (SPEC §5.2 — no self-grant).
func applyEnv(cfg *Config, getenv func(string) string) {
	if lvl := getenv("ADH_AUTONOMY"); lvl != "" {
		cfg.Autonomy = lvl
	}
}

func normalizeRole(name string) adh.Stage {
	switch name {
	case "eval":
		return adh.StageEvaluation
	case "exec":
		return adh.StageExecution
	default:
		return adh.Stage(name)
	}
}

func allRoles() []adh.Stage {
	return []adh.Stage{
		adh.StageStrategy, adh.StageExecution, adh.StageCritic,
		adh.StageEvaluation, adh.StageOps,
	}
}
