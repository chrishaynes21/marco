// identityprobe measures how stable a durable PLACE identity is across repeated visits.
//
// # Why this exists
//
// Roadmap 34's remaining blocker is that the same real screen sometimes resolves to a
// different durable subject, or to none. Widening the matcher to get past it would be
// guessing; this measures which identity-bearing dimension actually varies, across several
// real applications, before anything is changed.
//
// # What it is, and what it deliberately is not
//
// It reads finished observation sessions and re-runs the PRODUCTION identity path over them:
// `observe.SignatureOfState` — the same function `PlaceNow`, `PlaceToEstablish` and the
// relationship layer all use — and `observe.CompareStructure`, the one matcher. It computes
// nothing of its own and applies no tolerance of its own, so a verdict here is the verdict
// the Director would reach.
//
// It observes; it never establishes, never writes to semantic memory, and never drives input.
// The sessions themselves are produced by ordinary `director observe-game` runs.
//
// Usage:
//
//	identityprobe <session-json-file>...
//
// Each file is one visit. Files are grouped by application, and every pair of visits to the
// same application is compared.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// sessionFile is the part of a session's JSON this needs. Deliberately narrow: the shadow
// totals decode into the REAL type, so the producer runs over the real evidence.
type sessionFile struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	Application string `json:"application"`
	Samples     int    `json:"samples"`
	Stats       struct {
		Shadow observe.ShadowTotals `json:"shadow"`
	} `json:"stats"`
}

// visit is one identity opportunity, reduced to what durable matching actually reads.
type visit struct {
	file        string
	app         string
	samples     int
	state       observe.ScreenStateID
	settled     bool
	inferences  int
	episodes    int
	sig         observe.StructureSignature
	describable bool
}

func main() {
	args := os.Args[1:]
	// -replay answers a different question from the pairwise comparison: not "is this the
	// same place twice" but "what would a licensed pass over this session have made durable,
	// and would its transitions then have resolved". See replay.go.
	if len(args) > 0 && args[0] == "-replay" {
		args = args[1:]
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "usage: identityprobe -replay <session-json>...")
			os.Exit(2)
		}
		for _, path := range args {
			if err := replay(path); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			}
		}
		return
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: identityprobe [-replay] <session-json>...")
		os.Exit(2)
	}
	th := observe.DefaultHypothesisThresholds()

	byApp := map[string][]visit{}
	for _, path := range args {
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			continue
		}
		var s sessionFile
		if err := json.Unmarshal(raw, &s); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			continue
		}
		v := visit{
			file: shortName(path), app: s.Application, samples: s.Samples,
			state: s.Stats.Shadow.CurrentState,
		}
		for _, st := range s.Stats.Shadow.States {
			if st.ID == v.state {
				v.settled, v.inferences, v.episodes = st.Settled, st.Inferences, st.Episodes
			}
		}
		// THE production projection. Not a reimplementation.
		v.sig, v.describable = observe.SignatureOfState(s.Stats.Shadow, v.state, th)
		byApp[s.Application] = append(byApp[s.Application], v)
	}

	apps := make([]string, 0, len(byApp))
	for a := range byApp {
		apps = append(apps, a)
	}
	sort.Strings(apps)

	for _, app := range apps {
		report(app, byApp[app])
	}
}

func report(app string, visits []visit) {
	fmt.Printf("\n=== %s — %d visit(s) ===\n", app, len(visits))
	for _, v := range visits {
		fmt.Printf("  %-22s state=%-8s settled=%-5v inf=%-3d ep=%-2d describable=%v\n",
			v.file, v.state, v.settled, v.inferences, v.episodes, v.describable)
		if !v.describable {
			continue
		}
		fmt.Printf("      roles=%s\n", roleString(v.sig.Roles))
		fmt.Printf("      terms=%v termsKnown=%v discriminating=%v members=%d envelope=%v\n",
			v.sig.Terms, v.sig.TermsKnown, v.sig.Discriminating(), v.sig.Members,
			v.sig.Envelope != nil)
	}

	// Every pair, through the ONE matcher. This is the question Learn actually asks.
	fmt.Printf("  --- pairwise, via observe.CompareStructure ---\n")
	same, other := 0, 0
	for i := range visits {
		for j := i + 1; j < len(visits); j++ {
			a, b := visits[i], visits[j]
			if !a.describable || !b.describable {
				continue
			}
			verdict := observe.CompareStructure(a.sig, b.sig)
			if verdict == observe.MatchSame {
				same++
				continue
			}
			other++
			fmt.Printf("  %s vs %s -> %s\n", a.file, b.file, verdict)
			for _, line := range whyDiffer(a.sig, b.sig) {
				fmt.Printf("      %s\n", line)
			}
		}
	}
	fmt.Printf("  SUMMARY %s: same=%d not-same=%d\n", app, same, other)
}

// whyDiffer names the identity-bearing fields that disagree, in the matcher's own order.
//
// It reports what CompareStructure would have looked at rather than a general diff, so a
// reader is told which rule fired rather than which numbers happen to be unequal.
func whyDiffer(a, b observe.StructureSignature) []string {
	var out []string
	if a.Subject != b.Subject {
		out = append(out, fmt.Sprintf("kind: %s vs %s", a.Subject, b.Subject))
	}
	// Role SET, then per-role counts — the two rules, kept apart.
	onlyA, onlyB := missingRoles(a.Roles, b.Roles), missingRoles(b.Roles, a.Roles)
	if len(onlyA) > 0 || len(onlyB) > 0 {
		out = append(out, fmt.Sprintf("role SET differs: only-in-first=%v only-in-second=%v",
			onlyA, onlyB))
	}
	var over []string
	for role, n := range a.Roles {
		m, ok := b.Roles[role]
		if !ok {
			continue
		}
		if d := n - m; d > observe.RoleCountTolerance || -d > observe.RoleCountTolerance {
			over = append(over, fmt.Sprintf("%s %d vs %d", role, n, m))
		}
	}
	sort.Strings(over)
	if len(over) > 0 {
		out = append(out, "role COUNTS past tolerance: "+strings.Join(over, ", "))
	}
	if a.TermsKnown && b.TermsKnown && !sameTerms(a.Terms, b.Terms) {
		out = append(out, fmt.Sprintf("terms differ: %v vs %v", a.Terms, b.Terms))
	}
	if !a.TermsKnown || !b.TermsKnown {
		out = append(out, fmt.Sprintf("terms not comparable: known=%v/%v",
			a.TermsKnown, b.TermsKnown))
	}
	if !a.Discriminating() || !b.Discriminating() {
		out = append(out, fmt.Sprintf("no discriminator: %v/%v",
			a.Discriminating(), b.Discriminating()))
	}
	// The totals, so a reader can see the SHAPE of the difference at a glance.
	out = append(out, fmt.Sprintf("total members: %d vs %d over %d vs %d roles",
		total(a.Roles), total(b.Roles), len(a.Roles), len(b.Roles)))
	return out
}

func missingRoles(from, in map[string]int) []string {
	var out []string
	for role := range from {
		if _, ok := in[role]; !ok {
			out = append(out, fmt.Sprintf("%s=%d", role, from[role]))
		}
	}
	sort.Strings(out)
	return out
}

func sameTerms(a, b []observe.InterfaceTerm) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]observe.InterfaceTerm{}, a...), append([]observe.InterfaceTerm{}, b...)
	sort.Slice(x, func(i, j int) bool { return x[i] < x[j] })
	sort.Slice(y, func(i, j int) bool { return y[i] < y[j] })
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func total(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func roleString(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, " ")
}

func shortName(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	return p[i+1:]
}
