// Package metricscmd implements the "metrics" CLI command: report effectiveness
// accounting — attention per accepted arc and compute (SPEC-ADDITIONS §16).
package metricscmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	metricslib "github.com/StevenACoffman/agentic-dev-harness/internal/metrics"
)

const metricsFile = ".adh/metrics.json"

// Config holds the configuration for the metrics command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the metrics command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("metrics").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "metrics",
		Usage:     "agentic-dev-harness metrics",
		ShortHelp: "report effectiveness accounting",
		LongHelp:  "Summarize effectiveness: attention per accepted arc and compute (SPEC-ADDITIONS §16).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	records, err := load()
	if err != nil {
		return err
	}
	s := metricslib.Summarize(records)
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"arcs:                 %d\naccepted:             %d\nattention (min):      %d\ncompute (tokens):     %d\nattention/accept:     %.1f\n",
		s.Arcs,
		s.Accepted,
		s.AttentionMinutes,
		s.ComputeTokens,
		s.AttentionPerAccept,
	)
	return nil
}

func load() ([]metricslib.Record, error) {
	data, err := os.ReadFile(metricsFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}
	var records []metricslib.Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}
	return records, nil
}
