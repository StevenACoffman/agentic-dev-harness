package evidence_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
)

func TestAppendReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	in := []evidence.Record{
		{
			Timestamp: "2026-07-25T00:00:00Z",
			Arc:       "arc-0001",
			Stage:     "critic",
			Status:    evidence.StatusBaseline,
		},
		{
			Timestamp:  "2026-07-25T00:01:00Z",
			Arc:        "arc-0001",
			GateAction: "reject",
			NewScore:   0.6,
			Status:     evidence.StatusRevert,
		},
	}
	if err := evidence.Append(&buf, in...); err != nil {
		t.Fatalf("Append: %v", err)
	}
	out, err := evidence.Read(&buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(out) != 2 || out[1].GateAction != "reject" || out[1].Status != evidence.StatusRevert {
		t.Errorf("round trip = %+v", out)
	}
}

func TestAppendRejectsUnknownStatus(t *testing.T) {
	var buf bytes.Buffer
	err := evidence.Append(&buf, evidence.Record{Status: evidence.Status("bogus")})
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Append unknown status code = %q, want invalid", adh.ErrorCode(err))
	}
	if buf.Len() != 0 {
		t.Errorf("nothing should be written for an invalid record, got %q", buf.String())
	}
}

func TestReadRejectsMalformedLine(t *testing.T) {
	_, err := evidence.Read(strings.NewReader("{\"status\":\"keep\"}\nnot json\n"))
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Read malformed code = %q, want invalid", adh.ErrorCode(err))
	}
}

func TestReadSkipsBlankLines(t *testing.T) {
	out, err := evidence.Read(strings.NewReader("\n{\"status\":\"keep\"}\n\n"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("blank lines should be skipped, got %d records", len(out))
	}
}
