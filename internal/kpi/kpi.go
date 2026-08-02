// Package kpi turns declared KPIs on §10 units and §13 tools (SPEC-ADDITIONS §16/§18)
// into gated improvement proposals: it measures a subject against the log of how it
// actually behaved — the failure-record log for units, the tool-run log for tools — and
// when a KPI's threshold is breached across enough independent time strata to trust it
// (§18.2), proposes a change to that subject. It never adopts one — the proposal is a
// human's to act on. Every function is pure; the command supplies the loaded inputs.
package kpi

import (
	"sort"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolrun"
)

// MinStrata is how many distinct time strata a breach must span before it is proposed
// (§18.2): the same "never adopt on one signal" replication axis as the routing-miss
// and lesson-promotion gates.
const MinStrata = 2

// Subject kinds, so a proposal names what it concerns (§16/§18).
const (
	SubjectUnit = "unit"
	SubjectTool = "tool"
)

// Metric names the observation sources measure.
const (
	// MetricGroundedMiss is the per-unit error KPI: how many times an arc failed
	// *despite* the unit's scope being routed (a grounded miss, §19.2) — the unit's
	// guidance was present yet did not prevent the failure.
	MetricGroundedMiss = "grounded_miss"
	// MetricRunFailure is the per-tool error KPI: how many times a declared tool ran
	// and exited non-zero (§16/§18).
	MetricRunFailure = "run_failure"
	// MetricRunDuration is the per-tool latency KPI: a tool's mean run duration in
	// milliseconds, over the runs that started (§16/§18).
	MetricRunDuration = "run_duration_ms"
)

// Observation is a subject's measured value for one metric and the count of distinct
// time strata it was seen across — the replication axis a proposal gates on (§18.2).
type Observation struct {
	Metric string
	Value  float64
	Strata int
}

// Subject bundles a tool or unit id, its Kind, the KPIs it declares, and the
// observations measured for it, so Propose can match each KPI to its metric's
// observation.
type Subject struct {
	ID           string
	Kind         string
	KPIs         []adh.KPI
	Observations []Observation
}

// Proposal is a degradation a crossed KPI earned (§16/§18): the subject (its id and
// kind) and metric, the observed value against the threshold, and the strata it
// replicated across. It is a proposed change, never an applied one.
type Proposal struct {
	Subject   string  `json:"subject"`
	Kind      string  `json:"kind"`
	Metric    string  `json:"metric"`
	Observed  float64 `json:"observed"`
	Threshold float64 `json:"threshold"`
	Strata    int     `json:"strata"`
}

// Propose returns a proposal for each subject KPI whose matching observation breached
// its threshold across at least minStrata distinct strata (§18.2). A KPI with no
// matching observation is skipped — a declared indicator no source measures never
// fires. Pure; sorted by subject then metric.
func Propose(subjects []Subject, minStrata int) []Proposal {
	proposals := make([]Proposal, 0)
	for i := range subjects {
		for j := range subjects[i].KPIs {
			kpi := subjects[i].KPIs[j]
			obs, ok := findObservation(subjects[i].Observations, kpi.Metric)
			if !ok || !kpi.Breached(obs.Value) || obs.Strata < minStrata {
				continue
			}
			proposals = append(proposals, Proposal{
				Subject:   subjects[i].ID,
				Kind:      subjects[i].Kind,
				Metric:    kpi.Metric,
				Observed:  obs.Value,
				Threshold: kpi.Threshold,
				Strata:    obs.Strata,
			})
		}
	}
	sort.Slice(proposals, func(a, b int) bool {
		if proposals[a].Subject != proposals[b].Subject {
			return proposals[a].Subject < proposals[b].Subject
		}
		return proposals[a].Metric < proposals[b].Metric
	})
	return proposals
}

// ObserveUnits measures each KPI-declaring unit's grounded_miss value from the
// failure-record log: the count of grounded-miss records whose scope (a label or path)
// overlaps the unit's, and the distinct strata among them. A unit that declares no KPI
// is skipped. Pure — the caller loads the units and records.
func ObserveUnits(units []contextstore.Unit, records []failures.Record) []Subject {
	subjects := make([]Subject, 0)
	for i := range units {
		if len(units[i].KPIs) == 0 {
			continue
		}
		count, strata := groundedMissesInScope(&units[i], records)
		subjects = append(subjects, Subject{
			ID:   units[i].ID,
			Kind: SubjectUnit,
			KPIs: units[i].KPIs,
			Observations: []Observation{
				{Metric: MetricGroundedMiss, Value: float64(count), Strata: strata},
			},
		})
	}
	return subjects
}

// ObserveTools measures each KPI-declaring tool from the tool-run log: a run_failure
// observation (the count of runs that ran and exited non-zero) and a run_duration_ms
// observation (the mean duration of runs that started), each with the distinct strata
// it spanned. A tool that declares no KPI is skipped, and a KPI naming neither metric
// simply never fires. Pure — the caller loads the registry and the run log.
func ObserveTools(tools []toolreg.Tool, records []toolrun.Record) []Subject {
	subjects := make([]Subject, 0)
	for i := range tools {
		if len(tools[i].KPIs) == 0 {
			continue
		}
		failCount, failStrata := toolFailures(tools[i].ID, records)
		meanMS, runStrata := toolDuration(tools[i].ID, records)
		subjects = append(subjects, Subject{
			ID:   tools[i].ID,
			Kind: SubjectTool,
			KPIs: tools[i].KPIs,
			Observations: []Observation{
				{Metric: MetricRunFailure, Value: float64(failCount), Strata: failStrata},
				{Metric: MetricRunDuration, Value: meanMS, Strata: runStrata},
			},
		})
	}
	return subjects
}

// toolFailures counts a tool's failed runs and the distinct strata among them.
func toolFailures(id string, records []toolrun.Record) (count, strata int) {
	seen := make(map[string]bool)
	for i := range records {
		if records[i].Tool != id || !records[i].Failed {
			continue
		}
		count++
		if records[i].Stratum != "" {
			seen[records[i].Stratum] = true
		}
	}
	return count, len(seen)
}

// toolDuration returns a tool's mean run duration in milliseconds over the runs that
// started, and the distinct strata those runs spanned. A tool with no started run has
// no duration (0, 0), so its KPI never fires.
func toolDuration(id string, records []toolrun.Record) (meanMS float64, strata int) {
	var sum, count int
	seen := make(map[string]bool)
	for i := range records {
		if records[i].Tool != id || !records[i].Ran {
			continue
		}
		sum += records[i].DurationMS
		count++
		if records[i].Stratum != "" {
			seen[records[i].Stratum] = true
		}
	}
	if count == 0 {
		return 0, 0
	}
	return float64(sum) / float64(count), len(seen)
}

// groundedMissesInScope counts the grounded-miss records whose scope overlaps the
// unit's labels or paths, and the distinct strata among them.
func groundedMissesInScope(unit *contextstore.Unit, records []failures.Record) (count, strata int) {
	labels := toSet(unit.Labels)
	paths := toSet(unit.Paths)
	seen := make(map[string]bool)
	for i := range records {
		if records[i].RootCause != failures.RootGroundedMiss {
			continue
		}
		if !overlaps(labels, records[i].Labels) && !overlaps(paths, records[i].Paths) {
			continue
		}
		count++
		if records[i].Stratum != "" {
			seen[records[i].Stratum] = true
		}
	}
	return count, len(seen)
}

// findObservation returns the observation for a metric, if measured.
func findObservation(obs []Observation, metric string) (Observation, bool) {
	for i := range obs {
		if obs[i].Metric == metric {
			return obs[i], true
		}
	}
	return Observation{}, false
}

// overlaps reports whether any key is in the set.
func overlaps(set map[string]bool, keys []string) bool {
	for _, key := range keys {
		if set[key] {
			return true
		}
	}
	return false
}

// toSet collects keys into a set.
func toSet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	return set
}
