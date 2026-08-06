package consolidate_test

import (
	"reflect"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
)

// TestCommonClassesArcCoverageNotCount is the property that justifies the miner
// existing beside Reflect: coverage counts arcs, not raw instances. "spam" appears
// 10 times but in one arc (high count, low coverage) and is not common, while
// "shared" appears once in 3 of 4 arcs (0.75) and is.
func TestCommonClassesArcCoverageNotCount(t *testing.T) {
	t.Parallel()
	spam := make([]string, 0, 11)
	for range 10 {
		spam = append(spam, "spam: instance")
	}
	spam = append(spam, "shared: a")
	signals := []consolidate.Signal{
		{Failures: spam},
		{Failures: []string{"shared: b"}},
		{Failures: []string{"shared: c"}},
		{Failures: []string{"rare: d"}},
	}
	got := consolidate.CommonClasses(signals, consolidate.DefaultCommonCoverage)
	want := []consolidate.CommonClass{
		{Class: "shared", Kind: consolidate.KindFailure, Coverage: 0.75},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CommonClasses = %+v, want %+v (arc-coverage, not raw count)", got, want)
	}
}

// TestCommonClassesSuccessAndStrictThreshold: success signals are mined too, and the
// threshold is strict — a class in exactly half the arcs is not common.
func TestCommonClassesSuccessAndStrictThreshold(t *testing.T) {
	t.Parallel()
	signals := []consolidate.Signal{
		{Successes: []string{"half: a", "most: a"}},
		{Successes: []string{"half: b", "most: b"}},
		{Successes: []string{"most: c"}},
		{Successes: []string{"other: d"}},
	}
	got := consolidate.CommonClasses(signals, consolidate.DefaultCommonCoverage)
	want := []consolidate.CommonClass{
		{Class: "most", Kind: consolidate.KindSuccess, Coverage: 0.75},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CommonClasses = %+v, want %+v (half at 0.5 excluded by strict >)", got, want)
	}
}

func TestCommonClassesEmpty(t *testing.T) {
	t.Parallel()
	if got := consolidate.CommonClasses(nil, consolidate.DefaultCommonCoverage); got != nil {
		t.Errorf("CommonClasses(nil) = %v, want nil", got)
	}
}
