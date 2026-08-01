// Package lessoncmd implements the "lesson" CLI command: distill the failure
// registry into candidate lessons and promote one to a durable owner
// (SPEC-ADDITIONS §11). Promotion to an executable owner is human-gated.
package lessoncmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/atomicfile"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	lessonlib "github.com/StevenACoffman/agentic-dev-harness/internal/lesson"
)

// Config holds the configuration for the lesson command.
type Config struct {
	*root.Config
	To      string
	Approve bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the lesson command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("lesson").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.To,
		0,
		"to",
		"",
		"durable owner: context|doc|decision|skill|check|invariant|type",
	)
	cfg.Flags.BoolVar(&cfg.Approve, 0, "approve", "approve promotion to an executable owner")
	cfg.Command = &ff.Command{
		Name:      "lesson",
		Usage:     "agentic-dev-harness lesson [--to owner [--approve]] <list|promote> [class]",
		ShortHelp: "distill failures into lessons and promote them",
		LongHelp: "List candidate lessons distilled from the failure registry, or promote one " +
			"to a durable owner (SPEC-ADDITIONS §11). Flags precede the verb: " +
			"`lesson --to decision promote <class>`. Promoting to context/doc/decision " +
			"writes a routable §10 context unit; an executable owner (check/invariant/type) " +
			"needs --approve and is authored separately.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("lesson: expected a verb: list or promote")
	}
	switch args[0] {
	case "list":
		return cfg.list()
	case "promote":
		return cfg.promote(args[1:])
	default:
		return fmt.Errorf("lesson: unknown verb %q; want list or promote", args[0])
	}
}

// lessons distills the candidate classes from both the confirmed failure registry
// (§4.1) and the lesson-candidate file the Evaluation loop writes for unconfirmed
// critic findings (§19.2), so a class the loop surfaced is visible and promotable.
func (cfg *Config) lessons() ([]lessonlib.Lesson, error) {
	registry, err := failures.Load(failures.RegistryFile)
	if err != nil {
		return nil, fmt.Errorf("lesson: %w", err)
	}
	candidates, err := failures.Load(failures.CandidatesFile)
	if err != nil {
		return nil, fmt.Errorf("lesson: %w", err)
	}
	return lessonlib.Distill(append(registry, candidates...)), nil
}

func (cfg *Config) list() error {
	lessons, err := cfg.lessons()
	if err != nil {
		return err
	}
	for i := range lessons {
		_, _ = fmt.Fprintf(
			cfg.Stdout,
			"%s\t(%d instances)\n",
			lessons[i].Class,
			len(lessons[i].Instances),
		)
	}
	return nil
}

func (cfg *Config) promote(args []string) error {
	if len(args) == 0 {
		return errors.New("lesson: promote requires a class")
	}
	owner := lessonlib.Owner(cfg.To)
	if cfg.To == "" || !owner.Valid() {
		return errors.New(
			"lesson: promote requires --to context|doc|decision|skill|check|invariant|type",
		)
	}
	lessons, err := cfg.lessons()
	if err != nil {
		return err
	}
	class := args[0]
	lesson, ok := findLesson(lessons, class)
	if !ok {
		return fmt.Errorf("lesson: no candidate class %q; see `adh lesson list`", class)
	}
	records, err := failures.LoadRecords(failures.RecordsFile)
	if err != nil {
		return fmt.Errorf("lesson: %w", err)
	}
	// Temporal gate: a class promotes only after recurring across ≥2 strata (§11).
	if strata := failures.StrataCount(records)[class]; strata < lessonlib.MinStrata {
		_, _ = fmt.Fprintf(cfg.Stderr,
			"promotion of %q blocked: recurred across %d time stratum(s), needs %d "+
				"— a lesson promotes only on a pattern sustained across time (§11)\n",
			class, strata, lessonlib.MinStrata)
		return root.ExitError(19)
	}
	// An executable owner changes a gate and needs human approval (§11.2).
	if owner.RequiresApproval() && !cfg.Approve {
		_, _ = fmt.Fprintf(cfg.Stderr,
			"promotion of %q to executable owner %q requires approval; re-run with --approve\n",
			class, owner)
		return root.ExitError(13)
	}
	if owner.Materializes() {
		return cfg.materialize(&lesson, owner, records)
	}
	// Skill and the executable owners are authored separately; record the intent.
	_, _ = fmt.Fprintf(cfg.Stdout,
		"promoted %q to %s — author and register the %s (materialization deferred)\n",
		class, owner, owner)
	return nil
}

// findLesson returns the distilled lesson for a class, if present.
func findLesson(lessons []lessonlib.Lesson, class string) (lessonlib.Lesson, bool) {
	for i := range lessons {
		if lessons[i].Class == class {
			return lessons[i], true
		}
	}
	return lessonlib.Lesson{}, false
}

// materialize writes the lesson as a routable §10 context unit (a content file plus
// its unit JSON) under the context store, so the next arc inherits the correction
// (§11.2). The unit routes by the class label and the scope the class recurred under
// (§19.2 scope-tagging), so it lands where it was learned; it carries its lesson
// provenance and the root-cause triage from the failure records.
func (cfg *Config) materialize(
	l *lessonlib.Lesson,
	owner lessonlib.Owner,
	records []failures.Record,
) error {
	id := lessonlib.Slug(l.Class) + "-" + string(owner)
	contentFile := id + ".md"
	dir := contextstore.DefaultStoreDir
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("lesson: %w", err)
	}
	if err := atomicfile.WriteFile(
		filepath.Join(dir, contentFile), []byte(l.Render(owner)), 0o600,
	); err != nil {
		return fmt.Errorf("lesson: write content: %w", err)
	}
	labels, paths := failures.ScopeFor(records, l.Class)
	unit := contextstore.Unit{
		ID: id, Kind: owner.Kind(),
		Labels:      append([]string{l.Class}, labels...),
		Paths:       paths,
		ContentPath: contentFile,
		Provenance:  fmt.Sprintf("lesson: %s (%d instances)", l.Class, len(l.Instances)),
	}
	data, err := json.MarshalIndent(unit, "", "  ")
	if err != nil {
		return fmt.Errorf("lesson: %w", err)
	}
	if err := atomicfile.WriteFile(
		filepath.Join(dir, id+".json"), append(data, '\n'), 0o600,
	); err != nil {
		return fmt.Errorf("lesson: write unit: %w", err)
	}
	return cfg.reportPromotion(l, owner, id, failures.RootCauseCounts(records, l.Class))
}

// reportPromotion reports a materialized lesson: one JSONL outcome carrying the unit
// and its root-cause triage, or a human line naming the routing scope and root cause.
func (cfg *Config) reportPromotion(
	l *lessonlib.Lesson,
	owner lessonlib.Owner,
	id string,
	rootCauses map[string]int,
) error {
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"promoted": l.Class, "owner": string(owner), "unit": id, "kind": owner.Kind(),
			"root_cause": rootCauses,
		}); err != nil {
			return fmt.Errorf("lesson: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"promoted %q to %s: wrote context unit %s (routes by label %q; root cause %s)\n",
		l.Class, owner, id, l.Class, renderCounts(rootCauses))
	return nil
}

// renderCounts formats a root-cause tally deterministically (sorted by cause), or
// "unrecorded" when the class carries no root cause.
func renderCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "unrecorded"
	}
	causes := make([]string, 0, len(counts))
	for cause := range counts {
		causes = append(causes, cause)
	}
	sort.Strings(causes)
	parts := make([]string, len(causes))
	for i, cause := range causes {
		parts[i] = fmt.Sprintf("%s×%d", cause, counts[cause])
	}
	return strings.Join(parts, ", ")
}
