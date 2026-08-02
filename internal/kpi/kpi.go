// Package kpi turns declared per-unit KPIs (SPEC-ADDITIONS §16/§18) into gated
// improvement proposals: it measures a unit against the failure-record log and, when a
// KPI's threshold is breached across enough independent time strata to trust it
// (§18.2), proposes a change to that unit. It never adopts one — the proposal is a
// human's to act on. Every function is pure; the command supplies the loaded inputs.
package kpi

import (
	"sort"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
)

// MinStrata is how many distinct time strata a breach must span before it is proposed
// (§18.2): the same "never adopt on one signal" replication axis as the routing-miss
// and lesson-promotion gates.
const MinStrata = 2

// MetricGroundedMiss is the per-unit error KPI ObserveUnits measures: how many times an
// arc failed *despite* the unit's scope being routed (a grounded miss, §19.2) — the
// unit's guidance was present yet did not prevent the failure.
const MetricGroundedMiss = "grounded_miss"

// Observation is a subject's measured value for one metric and the count of distinct
// time strata it was seen across — the replication axis a proposal gates on (§18.2).
type Observation struct {
	Metric string
	Value  float64
	Strata int
}

// Subject bundles a tool or unit id, the KPIs it declares, and the observations
// measured for it, so Propose can match each KPI to its metric's observation.
type Subject struct {
	ID           string
	KPIs         []adh.KPI
	Observations []Observation
}

// Proposal is a degradation a crossed KPI earned (§16/§18): the subject and metric, the
// observed value against the threshold, and the strata it replicated across. It is a
// proposed change, never an applied one.
type Proposal struct {
	Subject   string  `json:"subject"`
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
			KPIs: units[i].KPIs,
			Observations: []Observation{
				{Metric: MetricGroundedMiss, Value: float64(count), Strata: strata},
			},
		})
	}
	return subjects
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
