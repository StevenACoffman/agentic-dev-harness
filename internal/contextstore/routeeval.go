package contextstore

import "sort"

// RoutingCase is one routing-eval fixture (§10): the labels and paths an arc
// declares and the unit ids that should route for them. An empty Want asserts that
// nothing should route (the correct NONE case).
type RoutingCase struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	Want   []string `json:"want,omitempty"`
}

// RoutingReport aggregates a routing eval: how many cases routed exactly their Want
// set, and the precision/recall of the routed ids against the expected ones summed
// across cases — so a routing regression is a measured drop, not a vibe. Failures
// names the cases that did not route exactly.
type RoutingReport struct {
	Cases     int      `json:"cases"`
	Passed    int      `json:"passed"`
	Precision float64  `json:"precision"`
	Recall    float64  `json:"recall"`
	Failures  []string `json:"failures,omitempty"`
}

// EvaluateRouting runs each case through Route and scores it (§10): a case passes
// when the routed id set equals its Want set (an empty Want must route nothing).
// Precision and recall aggregate the true/false positives and false negatives over
// all cases; an empty denominator is vacuously 1.0 (nothing predicted or nothing
// wanted is not a miss). It is pure — the caller supplies the units and cases.
func EvaluateRouting(units []Unit, cases []RoutingCase, maxUnits int) RoutingReport {
	report := RoutingReport{Cases: len(cases), Failures: make([]string, 0)}
	var tp, fp, fn int
	for i := range cases {
		routed := routedIDs(units, &cases[i], maxUnits)
		want := idSet(cases[i].Want)
		casePass := len(routed) == len(want)
		for id := range routed {
			if want[id] {
				tp++
			} else {
				fp++
				casePass = false
			}
		}
		for id := range want {
			if !routed[id] {
				fn++
				casePass = false
			}
		}
		if casePass {
			report.Passed++
		} else {
			report.Failures = append(report.Failures, cases[i].Name)
		}
	}
	report.Precision = ratio(tp, tp+fp)
	report.Recall = ratio(tp, tp+fn)
	sort.Strings(report.Failures)
	return report
}

// routedIDs is the set of unit ids Route returns for a case.
func routedIDs(units []Unit, c *RoutingCase, maxUnits int) map[string]bool {
	routed := Route(units, c.Labels, c.Paths, maxUnits)
	ids := make(map[string]bool, len(routed))
	for i := range routed {
		ids[routed[i].ID] = true
	}
	return ids
}

// idSet turns a slice of ids into a set.
func idSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// ratio is num/denom, or 1.0 when denom is zero (vacuously correct — nothing to get
// wrong), so an all-NONE eval scores a clean 1.0 rather than 0.
func ratio(num, denom int) float64 {
	if denom == 0 {
		return 1.0
	}
	return float64(num) / float64(denom)
}
