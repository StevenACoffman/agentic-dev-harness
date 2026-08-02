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

// LooksLikePath reports whether a provenance source is a repo-relative path — a
// candidate for existence checking — rather than a URL or a prose citation: it has
// no URL scheme, no whitespace, and contains a path separator or a file extension.
func LooksLikePath(source string) bool {
	if source == "" || strings.Contains(source, "://") || strings.ContainsAny(source, " \t") {
		return false
	}
	return strings.Contains(source, "/") || strings.Contains(source, ".")
}

// DanglingSources returns "id: source" for each unit provenance source that looks
// like a repo path but does not resolve, per the injected exists predicate (§10.4
// receipt verification): a promoted unit citing a source that is not there is a
// provenance defect. URL and prose sources are skipped. It is pure — the caller
// injects the filesystem check so the core stays testable. Sorted.
func DanglingSources(units []Unit, exists func(string) bool) []string {
	dangling := make([]string, 0)
	for i := range units {
		for _, src := range units[i].Sources {
			if LooksLikePath(src) && !exists(src) {
				dangling = append(dangling, units[i].ID+": "+src)
			}
		}
	}
	sort.Strings(dangling)
	return dangling
}

// UnverifiedClaims returns "id: quote" for each unit claim whose source looks like a
// repo path but whose file cannot be read or does not contain the quoted text, per
// the injected read function (§10.4 receipt verification): the receipt half of
// provenance, beyond DanglingSources' check that the path resolves. A claim citing a
// URL or prose source is skipped — its quote cannot be traced deterministically. It
// is pure — the caller injects the file read so the core stays testable. Sorted.
func UnverifiedClaims(units []Unit, read func(string) (string, error)) []string {
	unverified := make([]string, 0)
	for i := range units {
		for _, claim := range units[i].Claims {
			if !LooksLikePath(claim.Source) {
				continue
			}
			text, err := read(claim.Source)
			if err != nil || !strings.Contains(text, claim.Quote) {
				unverified = append(unverified, units[i].ID+": "+claim.Quote)
			}
		}
	}
	sort.Strings(unverified)
	return unverified
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

// InvalidKPIs returns the ids of units declaring a malformed KPI (§16/§18) — an empty
// metric or an unknown direction — so a silently-ignored indicator is caught rather
// than never firing. A unit is listed once however many KPIs are malformed. Sorted.
// Pure.
func InvalidKPIs(units []Unit) []string {
	bad := make([]string, 0)
	for i := range units {
		for j := range units[i].KPIs {
			if !units[i].KPIs[j].Valid() {
				bad = append(bad, units[i].ID)
				break
			}
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
