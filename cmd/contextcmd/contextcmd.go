// Package contextcmd implements the "context" CLI command: inspect the
// just-in-time context store (SPEC-ADDITIONS §10) — list units, show one unit's
// text and provenance, route a working set by labels, lint the store, or verify
// that routed units have not drifted from their canonical source.
package contextcmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/shell"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// Exit codes for the context command (SPEC §7), kept distinct from the arc gates.
const (
	lintCode  = 12 // an invalid store (missing fields, unresolved content, duplicate ids)
	driftCode = 18 // a routed unit drifted from its canonical source (§10.4 anti-drift)
)

// checkInstruction leads the human consistency-review packet, naming the judgment
// the relayed agent makes over the assembled units (§10.4).
const checkInstruction = "review these routed context units for contradictions " +
	"(a skill vs a base rule vs a domain invariant) before one silently governs an arc:"

// defaultMissThreshold is how many recorded misses a label or path must accumulate
// before the router proposes a deterministic route for it (§10.3). A sensible
// default beats a knob no one tunes; the miss log makes the signal auditable.
const defaultMissThreshold = 2

// Config holds the configuration for the context command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// integrityResult is one unit's anti-drift verdict: ok (the check passed), drift
// (it ran and failed), or unverified (its tool is not installed, so integrity could
// not be proven — reported, but not a gate failure).
type integrityResult struct {
	Unit   string `json:"unit"`
	Tool   string `json:"tool"`
	Status string `json:"status"`
}

// reviewUnit is one unit in a consistency-review packet: its identity, provenance,
// and text, assembled so the relayed agent can judge cross-unit contradictions.
type reviewUnit struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Provenance string `json:"provenance,omitempty"`
	Content    string `json:"content,omitempty"`
}

// New creates and registers the context command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("context").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "context",
		Usage:     "agentic-dev-harness context <list|show|route|lint|verify|check|misses> [id|arc|labels...]",
		ShortHelp: "list, show, route, lint, verify, check, and learn from context units",
		LongHelp: "Inspect the just-in-time context store (SPEC-ADDITIONS §10): list units, " +
			"show one unit's text and provenance, route a working set by labels, lint the " +
			"store, verify that routed units have not drifted from their canonical source, " +
			"check a routed set for cross-unit contradictions, or list routing misses and the " +
			"route proposals they have earned.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(
			"context: expected a verb: list, show, route, lint, verify, check, or misses",
		)
	}
	storeDir := cfg.storeDir()
	units, err := contextstore.Load(storeDir)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}
	switch args[0] {
	case "list":
		return cfg.list(units)
	case "show":
		return cfg.show(storeDir, units, args[1:])
	case "route":
		routed := contextstore.Route(units, args[1:], nil, contextstore.DefaultWorkingSet)
		for i := range routed {
			_, _ = fmt.Fprintln(cfg.Stdout, routed[i].ID)
		}
		return nil
	case "lint":
		return cfg.lint(storeDir, units)
	case "verify":
		return cfg.verify(ctx, units, args[1:])
	case "check":
		return cfg.check(storeDir, units, args[1:])
	case "misses":
		return cfg.misses()
	default:
		return fmt.Errorf(
			"context: unknown verb %q; want list, show, route, lint, verify, check, or misses",
			args[0],
		)
	}
}

// list prints each unit's id, kind, and labels.
func (cfg *Config) list(units []contextstore.Unit) error {
	for i := range units {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t%s\n",
			units[i].ID, units[i].Kind, strings.Join(units[i].Labels, ","))
	}
	return nil
}

// show prints one unit's text and provenance — the content the routing preview
// points a worker at, pulled just in time (§10.4). Under --jsonl it emits one OK
// outcome carrying the metadata, provenance, and content.
func (cfg *Config) show(storeDir string, units []contextstore.Unit, args []string) error {
	if len(args) == 0 {
		return errors.New("context: show requires a unit id")
	}
	id := args[0]
	for i := range units {
		unit := &units[i]
		if unit.ID != id {
			continue
		}
		content, err := contextstore.Content(storeDir, unit)
		if err != nil {
			return fmt.Errorf("context: %w", err)
		}
		return cfg.reportUnit(unit, content)
	}
	return fmt.Errorf("context: no such unit %q", id)
}

// reportUnit emits a unit and its content, as one outcome under --jsonl else text.
func (cfg *Config) reportUnit(unit *contextstore.Unit, content string) error {
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"id": unit.ID, "kind": unit.Kind, "owner": unit.Owner,
			"provenance": unit.Provenance, "content": content,
		}); err != nil {
			return fmt.Errorf("context: %w", err)
		}
		return nil
	}
	if unit.Provenance != "" {
		_, _ = fmt.Fprintf(cfg.Stdout, "# %s (%s) — %s\n", unit.ID, unit.Kind, unit.Provenance)
	} else {
		_, _ = fmt.Fprintf(cfg.Stdout, "# %s (%s)\n", unit.ID, unit.Kind)
	}
	if content != "" {
		_, _ = fmt.Fprintln(cfg.Stdout, content)
	}
	return nil
}

// lint checks the store's structural validity: every unit has an id and kind, its
// promised content resolves, and ids are unique across the store (a duplicate id
// makes routing ambiguous, §10.4). It exits lintCode when any check fails.
func (cfg *Config) lint(storeDir string, units []contextstore.Unit) error {
	bad := 0
	for i := range units {
		unit := &units[i]
		if unit.ID == "" || unit.Kind == "" {
			bad++
			_, _ = fmt.Fprintf(cfg.Stderr, "unit missing id or kind: %+v\n", unit)
			continue
		}
		// The content the routing preview promises must exist and stay in the store.
		if _, err := contextstore.Content(storeDir, unit); err != nil {
			bad++
			_, _ = fmt.Fprintf(
				cfg.Stderr,
				"unit %s: content_path does not resolve: %v\n",
				unit.ID,
				err,
			)
		}
	}
	for _, id := range contextstore.DuplicateIDs(units) {
		bad++
		_, _ = fmt.Fprintf(cfg.Stderr, "duplicate unit id: %s\n", id)
	}
	if bad > 0 {
		return root.ExitError(lintCode)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d context units, all valid\n", len(units))
	return nil
}

// verify runs each unit's declared integrity check (§10.4 anti-drift): the §13 tool
// that proves the unit's content still matches its canonical source. With an arc id
// it verifies only the units routed to that arc; otherwise the whole store. A check
// that ran and failed is drift (exit driftCode); a check whose tool is not installed
// is unverified — reported, but not a gate failure (best-effort, matching the
// unrunnable=unconfirmed rule the Evaluation stage uses). A unit whose integrity
// names a tool the registry does not declare is a store misconfiguration (EINVALID).
func (cfg *Config) verify(ctx context.Context, units []contextstore.Unit, args []string) error {
	reg, err := toolreg.LoadRepo(cfg.repoDir())
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}
	targets, err := cfg.targets(units, args)
	if err != nil {
		return err
	}
	results := make([]integrityResult, 0, len(targets))
	drift := false
	for i := range targets {
		unit := &targets[i]
		if unit.Integrity == "" {
			continue
		}
		tool, ok := reg.FindByID(unit.Integrity)
		if !ok {
			return &adh.Error{Code: adh.EINVALID, Message: fmt.Sprintf(
				"unit %s: integrity check %q is not a declared tool (§13)",
				unit.ID,
				unit.Integrity,
			)}
		}
		code, ran := shell.Runner{}.Run(ctx, tool.Run, cfg.repoDir())
		result := integrityResult{Unit: unit.ID, Tool: tool.ID, Status: "ok"}
		switch {
		case shell.NotRun(code, ran):
			result.Status = "unverified"
		case code != 0:
			result.Status = "drift"
			drift = true
		}
		results = append(results, result)
	}
	return cfg.reportVerify(results, drift)
}

// targets is the unit set verify acts on: the units routed to an arc when an arc id
// is given, else the whole store.
func (cfg *Config) targets(units []contextstore.Unit, args []string) ([]contextstore.Unit, error) {
	if len(args) == 0 {
		return units, nil
	}
	store := state.NewStore(filepath.Join(cfg.repoDir(), state.DefaultArcsDir))
	arc, err := store.Get(args[0])
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}
	return contextstore.Route(units, arc.Labels, arc.Paths, 0), nil
}

// reportVerify emits the anti-drift results: under --jsonl one outcome carrying the
// per-unit verdicts (an error outcome when any unit drifted), else a line per unit.
// It exits driftCode when a routed unit drifted from its source.
func (cfg *Config) reportVerify(results []integrityResult, drift bool) error {
	if cfg.JSONL {
		if err := cfg.emitVerify(results, drift); err != nil {
			return err
		}
	} else {
		cfg.printVerify(results)
	}
	if drift {
		return root.ExitError(driftCode)
	}
	return nil
}

// emitVerify writes the anti-drift outcome under --jsonl: an error outcome carrying
// the per-unit verdicts when any unit drifted, an ok one otherwise.
func (cfg *Config) emitVerify(results []integrityResult, drift bool) error {
	status, reason, code := root.StatusOK, "", 0
	if drift {
		status, reason, code = root.StatusError, "context_drift", driftCode
	}
	if err := cfg.EmitJSONL(root.Outcome{
		Status: status, Code: code, Reason: reason,
		Data: map[string]any{"results": results, "drift": drift},
	}); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	return nil
}

// printVerify prints the anti-drift verdicts for a human, a line per unit.
func (cfg *Config) printVerify(results []integrityResult) {
	if len(results) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "no routed units declare an integrity check")
		return
	}
	for i := range results {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t(%s)\n",
			results[i].Status, results[i].Unit, results[i].Tool)
	}
}

// check assembles the routed unit set and each unit's content into one consistency-
// review packet (§10.4): adh gathers the context deterministically and the relayed
// agent judges whether any units contradict each other — a skill vs a base rule vs a
// domain invariant. With an arc id it reviews that arc's routed set; otherwise the
// whole store. It surfaces the set for judgment (the agent then promotes a lesson or
// opens an arc); it is not itself a gate — modelith reference-integrity (deterministic
// conflicts) is `context verify`/`tool run modelith-lint`, this is the semantic half.
func (cfg *Config) check(storeDir string, units []contextstore.Unit, args []string) error {
	targets, err := cfg.targets(units, args)
	if err != nil {
		return err
	}
	reviewed := make([]reviewUnit, 0, len(targets))
	for i := range targets {
		content, cerr := contextstore.Content(storeDir, &targets[i])
		if cerr != nil {
			return fmt.Errorf("context: %w", cerr)
		}
		reviewed = append(reviewed, reviewUnit{
			ID:         targets[i].ID,
			Kind:       targets[i].Kind,
			Provenance: targets[i].Provenance,
			Content:    content,
		})
	}
	return cfg.reportCheck(reviewed)
}

// reportCheck emits the consistency-review packet: under --jsonl one outcome
// carrying the units and their content, else the human-readable assembled packet.
func (cfg *Config) reportCheck(reviewed []reviewUnit) error {
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{"units": reviewed}); err != nil {
			return fmt.Errorf("context: %w", err)
		}
		return nil
	}
	if len(reviewed) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "no units routed; nothing to check")
		return nil
	}
	_, _ = fmt.Fprintln(cfg.Stdout, checkInstruction)
	for i := range reviewed {
		unit := &reviewed[i]
		if unit.Provenance != "" {
			_, _ = fmt.Fprintf(
				cfg.Stdout,
				"\n## %s (%s) — %s\n",
				unit.ID,
				unit.Kind,
				unit.Provenance,
			)
		} else {
			_, _ = fmt.Fprintf(cfg.Stdout, "\n## %s (%s)\n", unit.ID, unit.Kind)
		}
		if unit.Content != "" {
			_, _ = fmt.Fprintln(cfg.Stdout, unit.Content)
		}
	}
	return nil
}

// misses lists the recorded routing misses and the route proposals they have
// earned (§10.3): a label or path arcs missed on at least defaultMissThreshold
// times, so authoring a unit for it converts the recurring miss into a deterministic
// route. It only proposes — applying a route (authoring a unit, gated at §11) stays
// a human/agent decision; nothing is auto-routed.
func (cfg *Config) misses() error {
	misses, err := contextstore.LoadMisses(filepath.Join(cfg.repoDir(), contextstore.MissFile))
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}
	proposals := contextstore.ProposeRoutes(misses, defaultMissThreshold)
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"misses": len(misses), "proposals": proposals,
		}); err != nil {
			return fmt.Errorf("context: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d routing miss(es) recorded\n", len(misses))
	for i := range proposals {
		proposal := &proposals[i]
		_, _ = fmt.Fprintf(
			cfg.Stdout,
			"propose: route %s %q (missed %d×) — author a unit for it (adh lesson promote / context)\n",
			proposal.Kind,
			proposal.Key,
			proposal.Count,
		)
	}
	return nil
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}

// storeDir is the context store under the repo root.
func (cfg *Config) storeDir() string {
	return filepath.Join(cfg.repoDir(), contextstore.DefaultStoreDir)
}
