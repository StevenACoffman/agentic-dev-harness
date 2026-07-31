package contextstore

import (
	"fmt"
	"sort"
	"strings"
)

// Orphans returns the ids of units that can never route — no labels and no paths —
// so they are unreachable pages in the store (§10.4 wiki-lint): adh routes only by
// label or path, so a unit with neither is dead weight. Sorted. Pure.
func Orphans(units []Unit) []string {
	orphans := make([]string, 0)
	for i := range units {
		if len(units[i].Labels) == 0 && len(units[i].Paths) == 0 {
			orphans = append(orphans, units[i].ID)
		}
	}
	sort.Strings(orphans)
	return orphans
}

// DanglingSupersessions returns the ids of units whose SupersededBy names a unit that
// does not exist — a broken lifecycle reference (§10.4). Sorted. Pure.
func DanglingSupersessions(units []Unit) []string {
	exists := make(map[string]bool, len(units))
	for i := range units {
		exists[units[i].ID] = true
	}
	dangling := make([]string, 0)
	for i := range units {
		if units[i].SupersededBy != "" && !exists[units[i].SupersededBy] {
			dangling = append(dangling, units[i].ID)
		}
	}
	sort.Strings(dangling)
	return dangling
}

// InvalidTrust returns the ids of units whose trust tier is outside the taxonomy
// (§10.4). Sorted. Pure.
func InvalidTrust(units []Unit) []string {
	bad := make([]string, 0)
	for i := range units {
		if !units[i].Verified.Valid() {
			bad = append(bad, units[i].ID)
		}
	}
	sort.Strings(bad)
	return bad
}

// Index renders the read-first routing catalog (§10 compounding-wiki): one line per
// routable (non-superseded) unit — id, kind, trust tier, labels, and the one-line
// provenance — the JIT grounding preview a worker reads before pulling any unit's
// text. Sorted by id. Pure.
func Index(units []Unit) string {
	rows := make([]Unit, 0, len(units))
	for i := range units {
		if units[i].SupersededBy == "" {
			rows = append(rows, units[i])
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	var b strings.Builder
	b.WriteString("# Context index\n\n")
	for i := range rows {
		writeIndexRow(&b, &rows[i])
	}
	return b.String()
}

// writeIndexRow renders one catalog line, defaulting an unset trust tier to unverified.
func writeIndexRow(b *strings.Builder, unit *Unit) {
	trust := unit.Verified
	if trust == "" {
		trust = Unverified
	}
	_, _ = fmt.Fprintf(b, "- %s (%s) [%s]", unit.ID, unit.Kind, trust)
	if len(unit.Labels) > 0 {
		_, _ = fmt.Fprintf(b, " %s", strings.Join(unit.Labels, ","))
	}
	if unit.Provenance != "" {
		_, _ = fmt.Fprintf(b, " — %s", unit.Provenance)
	}
	b.WriteString("\n")
}
