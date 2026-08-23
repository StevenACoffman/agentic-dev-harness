package contextstore

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// IntegrityFile is the conventional repo path the integrity-verification log lives
// under. Kept distinct from the miss log and the evidence trail: a verification is
// neither a learning signal nor a gate decision, it is an observation about whether a
// unit still describes what it claims to.
const IntegrityFile = ".adh/context-integrity.jsonl"

// Integrity result values, as recorded by `context verify`.
const (
	ResultOK         = "ok"
	ResultDrift      = "drift"
	ResultUnverified = "unverified"
)

// Freshness states for a context unit, four of them, and the fourth is the point.
//
// `unknown` (never evaluated) and `not_applicable` (declares no integrity tool) are
// distinct from `drifted`, and collapsing them would make a hand-written unit that
// nothing could check look the same as one nobody has checked yet. Absence of a
// comparison is not a passing comparison.
const (
	// FreshnessUnknown is the zero value on purpose: a unit nothing has verified must
	// read as unchecked, never as clean. §12's applicability rule in a constant.
	FreshnessUnknown       Freshness = ""
	FreshnessFresh         Freshness = "fresh"
	FreshnessDrifted       Freshness = "drifted"
	FreshnessNotApplicable Freshness = "not_applicable"
)

// Freshness is whether a unit still matches what its integrity tool attests.
type Freshness string

// IntegrityRecord is one verification event: at some point, this unit's integrity
// tool was run and produced this result.
//
// An event, not a conclusion. The log is append-only and the newest record for a unit
// wins, so the file's own order carries the recency and nothing here needs a clock —
// which is what keeps the fold below pure.
type IntegrityRecord struct {
	Unit   string `json:"unit"`
	Tool   string `json:"tool,omitempty"`
	Result string `json:"result"`
}

// IntegrityLogFor returns the verification log that pairs with a store directory.
//
// The pairing is the naming convention -- ".adh/context" pairs with
// ".adh/context-integrity.jsonl" -- rather than a second path a caller has to know and
// could get wrong. It holds for a temp store in a test as readily as for the
// conventional one, which is why LoadFresh needs no repo root.
func IntegrityLogFor(storeDir string) string {
	return storeDir + "-integrity.jsonl"
}

// LoadFresh reads the context units under storeDir with their derived freshness
// already applied from the paired verification log.
//
// This is what routing callers want, and Load is what everything else wants. Splitting
// them rather than adding freshness to Load keeps the choice visible: a caller that
// ranks units by trust must account for recorded drift, and one that merely lists or
// counts them has nothing to account for.
func LoadFresh(storeDir string) ([]Unit, error) {
	units, err := Load(storeDir)
	if err != nil {
		return nil, err
	}
	records, err := LoadIntegrity(IntegrityLogFor(storeDir))
	if err != nil {
		return nil, err
	}
	ApplyFreshness(units, records)
	return units, nil
}

// AppendIntegrity records verification events to the append-only log at path,
// creating the parent directory and file if needed. It is the I/O shell to
// ApplyFreshness' pure core.
func AppendIntegrity(path string, records ...IntegrityRecord) error {
	const op = "contextstore.AppendIntegrity"
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	defer func() { _ = file.Close() }()
	for i := range records {
		line, marshalErr := json.Marshal(&records[i])
		if marshalErr != nil {
			return &adh.Error{Op: op, Err: marshalErr}
		}
		if _, writeErr := file.Write(append(line, '\n')); writeErr != nil {
			return &adh.Error{Op: op, Err: writeErr}
		}
	}
	return nil
}

// LoadIntegrity reads the verification log at path. An absent file is no records, not
// an error — a repository that has never run `context verify` is a valid one. A
// corrupt line is a hard error, matching the miss log and the evidence trail: the
// log's integrity is its whole value.
func LoadIntegrity(path string) ([]IntegrityRecord, error) {
	const op = "contextstore.LoadIntegrity"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []IntegrityRecord{}, nil
	}
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	records := make([]IntegrityRecord, 0)
	for _, line := range splitLines(data) {
		var rec IntegrityRecord
		if unmarshalErr := json.Unmarshal(line, &rec); unmarshalErr != nil {
			return nil, &adh.Error{Op: op, Err: unmarshalErr}
		}
		records = append(records, rec)
	}
	return records, nil
}

// ApplyFreshness fills each unit's derived Fresh field from the verification log.
// Pure: it mutates only the slice the caller owns and reads no clock or filesystem.
//
// The newest record for a unit wins, which is the append-only log's own order. A unit
// declaring no integrity tool is not applicable rather than unknown — nothing could
// have checked it, so nothing is missing.
func ApplyFreshness(units []Unit, records []IntegrityRecord) {
	latest := make(map[string]string, len(records))
	for i := range records {
		latest[records[i].Unit] = records[i].Result
	}
	for i := range units {
		units[i].Fresh = freshnessOf(&units[i], latest)
	}
}

// EffectiveTier is the trust tier a unit routes at, after recorded drift is taken
// into account.
//
// Recorded drift suppresses the authored tier to Unverified rather than overwriting
// it. The authored value is a human's judgement about the unit and is not adh's to
// destroy; drift is an observation that the thing judged has since moved. Keeping both
// means re-verifying a unit restores its standing with no edit, and it is why Fresh is
// derived and never serialised — a suppressed unit written back to disk cannot silently
// lose the authored tier, because there is nothing to write back.
func (u *Unit) EffectiveTier() TrustTier {
	if u.Fresh == FreshnessDrifted {
		return Unverified
	}
	return u.Verified
}

// freshnessOf maps one unit's newest recorded result to a freshness state.
func freshnessOf(unit *Unit, latest map[string]string) Freshness {
	if unit.Integrity == "" {
		return FreshnessNotApplicable
	}
	switch latest[unit.ID] {
	case ResultOK:
		return FreshnessFresh
	case ResultDrift:
		return FreshnessDrifted
	default:
		// Includes ResultUnverified: a tool that could not run has not cleared the
		// unit and has not condemned it either.
		return FreshnessUnknown
	}
}
