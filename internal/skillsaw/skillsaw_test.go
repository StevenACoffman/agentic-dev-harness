package skillsaw_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/skillsaw"
)

func TestDecode(t *testing.T) {
	full := `{"score":72.5,"dimensions":[
	  {"name":"outcome-clarity","score":0.8,"needs_judge":true},
	  {"name":"failure-handling","score":1.0,"needs_judge":false}
	],"extra":"ignored"}`
	eval, err := skillsaw.Decode([]byte(full))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if eval.Score != 72.5 {
		t.Errorf("score = %v, want 72.5", eval.Score)
	}
	nj := eval.NeedsJudge()
	if len(nj) != 1 || nj[0] != "outcome-clarity" {
		t.Errorf("NeedsJudge = %v, want [outcome-clarity]", nj)
	}

	// A minimal envelope (just the score) decodes; unknown fields are tolerated.
	bare, err := skillsaw.Decode([]byte(`{"score":50}`))
	if err != nil || bare.Score != 50 {
		t.Errorf("minimal decode = (%+v, %v), want score 50", bare, err)
	}

	// Malformed JSON is EINVALID at the boundary.
	if _, err := skillsaw.Decode([]byte(`not json`)); adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("malformed decode = %v, want EINVALID", err)
	}
}
