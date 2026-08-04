package skillsaw_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/skillsaw"
)

func TestDecode(t *testing.T) {
	// The real skillsaw eval --json shape (rubric.Evaluation / DimScore).
	full := `{"skill":"s","hash":"abc","bytes":1200,"deterministic_score":72.5,
	  "has_full_score":false,"dims":[
	  {"num":1,"name":"outcome-clarity","weight":25,"final":8,"needs_judge":true},
	  {"num":2,"name":"failure-handling","weight":20,"final":10,"needs_judge":false}
	]}`
	eval, err := skillsaw.Decode([]byte(full))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if eval.Score() != 72.5 {
		t.Errorf("Score() = %v, want the deterministic score 72.5", eval.Score())
	}
	nj := eval.NeedsJudge()
	if len(nj) != 1 || nj[0] != "outcome-clarity" {
		t.Errorf("NeedsJudge = %v, want [outcome-clarity]", nj)
	}

	// Once a judge has scored the judge dimensions, the full score is the gate score.
	judged, err := skillsaw.Decode(
		[]byte(`{"deterministic_score":72.5,"full_score":88,"has_full_score":true}`),
	)
	if err != nil || judged.Score() != 88 {
		t.Errorf("judged Score() = (%v, %v), want the full score 88", judged.Score(), err)
	}

	// Malformed JSON is EINVALID at the boundary.
	if _, err := skillsaw.Decode([]byte(`not json`)); adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("malformed decode = %v, want EINVALID", err)
	}
}
