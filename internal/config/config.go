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
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
)

const (
	// RepoConfigFile is the repo-level config path (SPEC §3): the loader's repo
	// layer and the path `adh init` writes the starter config to.
	RepoConfigFile = ".adh/config.toml"
	autonomyFile   = ".adh/autonomy"
	defaultLevel   = "L2"
	classReasoning = "reasoning"
	classFast      = "fast"
	// UnconfirmedLesson is the default disposition for a critic finding no artifact
	// confirmed (§19.2, §19.4): keep it as a §11 lesson candidate.
	UnconfirmedLesson = "lesson"
)

// StarterTOML is the documented starter config `adh init` writes to
// .adh/config.toml (SPEC §3.1). It resolves to the built-in defaults with the
// knobs surfaced and commented so an operator can tailor a deployment; unknown
// keys (illustrative sections like [oracle]) are ignored by the loader.
const StarterTOML = `autonomy = "L2"                 # current autonomy level (L0-L4)

[models]
# The model-gate enforces that "judgment" roles run on a "reasoning" class.
reasoning = "strong-cold-model" # Strategy, Critic, Evaluation
fast      = "fast-model"        # Execution, Ops

[models.gate]
# Roles that MUST run on a reasoning-class model.
judgment_roles = ["strategy", "critic", "eval"]

[gates]
approval_phrase_required = true

[critic]
# What an unconfirmed critic finding becomes (§19.2): a §11 lesson candidate.
unconfirmed = "lesson"

[evaluation]
# How many times a confirmed finding may return an arc to Execution before it
# fails terminally (§4.1). 0 uses the built-in default (2).
max_reworks = 2

[proof.contract]
# The acceptance bar each resolution's proof must meet to close (§SPEC 5.4, §12).
# Generic defaults; a mobile port might set change = "oracle, invariant, and
# on-device proof".
change        = "the change's tests pass and its review/CI checks are green"
investigation = "the sources inspected and the reproducible finding"
experiment    = "the instrumentation and the readout that answers the product question"
decision      = "the evidence and the rationale behind the call"
`

// Config is the resolved harness configuration (SPEC §3.1).
type Config struct {
	Autonomy   string     `toml:"autonomy"`
	Models     Models     `toml:"models"`
	Gates      Gates      `toml:"gates"`
	Critic     Critic     `toml:"critic"`
	Proof      Proof      `toml:"proof"`
	Evaluation Evaluation `toml:"evaluation"`
}

// Evaluation is the Evaluation-stage policy (SPEC §4.1). MaxReworks bounds the
// rework loop: the times a confirmed finding may return an arc to Execution before
// it fails terminally. Zero means unset — MaxReworks() then falls back to the
// evaluation package's default, so that default keeps a single owner.
type Evaluation struct {
	MaxReworks int `toml:"max_reworks"`
}

// Proof is the proof policy (SPEC §3.1, §5.4). Contract maps each resolution to
// the acceptance bar its proof must satisfy to close (§12); a deployment tailors
// it per key, and any key it omits falls back to the generic built-in default.
type Proof struct {
	Contract map[string]string `toml:"contract"`
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

// MaxReworks is the configured Evaluation rework budget (§4.1), falling back to
// evaluation.DefaultMaxReworks when unset so the default has a single owner.
func (c *Config) MaxReworks() int {
	if c.Evaluation.MaxReworks <= 0 {
		return evaluation.DefaultMaxReworks
	}
	return c.Evaluation.MaxReworks
}

// CriticUnconfirmed is the configured disposition for an unconfirmed critic
// finding (§19.2), defaulting to the §11 lesson candidate when unset.
func (c *Config) CriticUnconfirmed() string {
	if c.Critic.Unconfirmed == "" {
		return UnconfirmedLesson
	}
	return c.Critic.Unconfirmed
}

// ProofContract returns the acceptance bar a resolution's proof must satisfy to
// close (SPEC §3.1 `[proof.contract]`, §12): the configured text when set, else
// the resolution's generic built-in default (adh.Resolution.ProofKind), so the
// default text has a single owner. The harness enforces that matching proof
// exists (NO-PROOF-NO-CLOSE); this text is the bar the critic and close hold it to.
func (c *Config) ProofContract(res adh.Resolution) string {
	if text := c.Proof.Contract[string(res)]; text != "" {
		return text
	}
	return res.ProofKind()
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

// ProfileConfigFile is the repo-local profile config path (SPEC §3 tier 3): the
// layer `--profile <name>` (or ADH_PROFILE) selects, overlaying the repo config.
func ProfileConfigFile(profile string) string {
	return ".adh/config." + profile + ".toml"
}

// configDocs reads the config file layers lowest precedence first. ADH_CONFIG,
// when set, is the single explicit config file and replaces the search. Otherwise
// the layers are user config, repo config, then — when ADH_PROFILE is set — the
// profile config (SPEC §3 tier 3), which overlays the repo config and is itself
// overridden by env then flags. A missing file is skipped; any other read error
// propagates.
func configDocs(getenv func(string) string) ([][]byte, error) {
	if explicit := getenv("ADH_CONFIG"); explicit != "" {
		return readLayers(explicit)
	}
	paths := []string{userConfigPath(getenv), RepoConfigFile}
	if profile := getenv("ADH_PROFILE"); profile != "" {
		paths = append(paths, ProfileConfigFile(profile))
	}
	return readLayers(paths...)
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
