package kpi_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/kpi"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolrun"
)

func TestProposeGatesOnBreachAndStrata(t *testing.T) {
	errorKPI := adh.KPI{Metric: "grounded_miss", Threshold: 2, Direction: adh.WorseWhenAbove}
	tests := []struct {
		name     string
		subjects []kpi.Subject
		want     int
	}{
		{
			"breached across two strata proposes",
			[]kpi.Subject{{
				ID: "u", KPIs: []adh.KPI{errorKPI},
				Observations: []kpi.Observation{{Metric: "grounded_miss", Value: 3, Strata: 2}},
			}},
			1,
		},
		{
			"breached in one stratum does not propose",
			[]kpi.Subject{{
				ID: "u", KPIs: []adh.KPI{errorKPI},
				Observations: []kpi.Observation{{Metric: "grounded_miss", Value: 9, Strata: 1}},
			}},
			0,
		},
		{
			"at threshold does not propose",
			[]kpi.Subject{{
				ID: "u", KPIs: []adh.KPI{errorKPI},
				Observations: []kpi.Observation{{Metric: "grounded_miss", Value: 2, Strata: 5}},
			}},
			0,
		},
		{
			"unmeasured metric is skipped",
			[]kpi.Subject{
				{
					ID: "u",
					KPIs: []adh.KPI{
						{Metric: "duration", Threshold: 1, Direction: adh.WorseWhenAbove},
					},
					Observations: []kpi.Observation{{Metric: "grounded_miss", Value: 9, Strata: 9}},
				},
			},
			0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := kpi.Propose(tc.subjects, kpi.MinStrata)
			if len(got) != tc.want {
				t.Fatalf("Propose = %v, want %d proposal(s)", got, tc.want)
			}
			if tc.want == 1 && (got[0].Subject != "u" || got[0].Observed != 3) {
				t.Errorf("proposal = %+v, want subject u observed 3", got[0])
			}
		})
	}
}

func TestObserveUnits(t *testing.T) {
	units := []contextstore.Unit{
		{
			ID: "ui-rule", Labels: []string{"ui"},
			KPIs: []adh.KPI{{Metric: "grounded_miss", Threshold: 1, Direction: adh.WorseWhenAbove}},
		},
		{ID: "unmonitored", Labels: []string{"ui"}}, // no KPI → no subject
	}
	records := []failures.Record{
		// In ui-rule's scope, grounded-miss, two distinct strata.
		{
			Class:     "oracle",
			Stratum:   "2026-06",
			Labels:    []string{"ui"},
			RootCause: failures.RootGroundedMiss,
		},
		{
			Class:     "oracle",
			Stratum:   "2026-07",
			Labels:    []string{"ui"},
			RootCause: failures.RootGroundedMiss,
		},
		// Ungrounded — not counted.
		{
			Class:     "oracle",
			Stratum:   "2026-08",
			Labels:    []string{"ui"},
			RootCause: failures.RootUngrounded,
		},
		// Out of scope — not counted.
		{
			Class:     "oracle",
			Stratum:   "2026-06",
			Labels:    []string{"api"},
			RootCause: failures.RootGroundedMiss,
		},
	}
	subjects := kpi.ObserveUnits(units, records)
	if len(subjects) != 1 || subjects[0].ID != "ui-rule" {
		t.Fatalf("subjects = %+v, want just ui-rule", subjects)
	}
	obs := subjects[0].Observations
	if len(obs) != 1 || obs[0].Metric != kpi.MetricGroundedMiss {
		t.Fatalf("observations = %+v, want one grounded_miss", obs)
	}
	if obs[0].Value != 2 || obs[0].Strata != 2 {
		t.Errorf("observation = %+v, want value 2 across 2 strata", obs[0])
	}
}

func TestObserveTools(t *testing.T) {
	tools := []toolreg.Tool{
		{
			ID: "skillsaw-eval", Verifies: "rubric floor",
			KPIs: []adh.KPI{{Metric: "run_failure", Threshold: 1, Direction: adh.WorseWhenAbove}},
		},
		{ID: "unmonitored", Verifies: "x"}, // no KPI → no subject
	}
	records := []toolrun.Record{
		{Tool: "skillsaw-eval", Stratum: "2026-06", Ran: true, Failed: true},
		{Tool: "skillsaw-eval", Stratum: "2026-07", Ran: true, Failed: true},
		{
			Tool:    "skillsaw-eval",
			Stratum: "2026-07",
			Ran:     true,
			Failed:  false,
		}, // passed, not counted
		{Tool: "other", Stratum: "2026-06", Ran: true, Failed: true}, // different tool
	}
	subjects := kpi.ObserveTools(tools, records)
	if len(subjects) != 1 || subjects[0].ID != "skillsaw-eval" ||
		subjects[0].Kind != kpi.SubjectTool {
		t.Fatalf("subjects = %+v, want just the skillsaw tool", subjects)
	}
	obs := subjects[0].Observations
	if len(obs) != 1 || obs[0].Metric != kpi.MetricRunFailure || obs[0].Value != 2 ||
		obs[0].Strata != 2 {
		t.Errorf("observation = %+v, want run_failure value 2 across 2 strata", obs)
	}
}

func TestObserveUnitsMatchesByPath(t *testing.T) {
	units := []contextstore.Unit{{
		ID: "board", Paths: []string{"board.go"},
		KPIs: []adh.KPI{{Metric: "grounded_miss", Threshold: 0, Direction: adh.WorseWhenAbove}},
	}}
	records := []failures.Record{
		{
			Class:     "oracle",
			Stratum:   "2026-06",
			Paths:     []string{"board.go"},
			RootCause: failures.RootGroundedMiss,
		},
	}
	subjects := kpi.ObserveUnits(units, records)
	if len(subjects) != 1 || subjects[0].Observations[0].Value != 1 {
		t.Errorf("path-scoped observation = %+v, want value 1", subjects)
	}
}
