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

// whyDiffer explains a mismatch, and asks the MATCHER rather than re-deriving one.
//
// It used to compute its own role-set and term comparison. Two problems, both of which showed up
// the moment this was pointed at the responsive breakpoint: it compared RAW roles where the
// matcher compares `identityRoles`, so it reported layout roles as decisive when the matcher had
// already dropped them; and it printed two term lists without saying which terms had been LOST
// and which GAINED, which is precisely the distinction between an interface that reflowed and a
// person who navigated.
//
// So the verdict and its reasons now come from `ExplainStructure` — the one implementation —
// and what this adds is the lost/gained breakdown, which is a MEASUREMENT and not a rule.
func whyDiffer(a, b observe.StructureSignature) []string {
	var out []string
	cmp := observe.ExplainStructure(a, b)
	for _, d := range cmp.Why {
		mark := " "
		if d.Decisive {
			mark = "*"
		}
		out = append(out, fmt.Sprintf("%s %-10s current=%s remembered=%s",
			mark, d.Field, d.Current, d.Remembered))
	}
	// LOST and GAINED, kept apart. An interface that reflows away its navigation loses
	// structures and contradicts nothing; a person who navigated brings different ones.
	if lost, gained := termDelta(a.Terms, b.Terms); len(lost) > 0 || len(gained) > 0 {
		out = append(out, fmt.Sprintf("  terms      lost=%v gained=%v", lost, gained))
	}
	onlyA, onlyB := missingRoles(a.Roles, b.Roles), missingRoles(b.Roles, a.Roles)
	if len(onlyA) > 0 || len(onlyB) > 0 {
		out = append(out, fmt.Sprintf("  roles      only-in-first=%v only-in-second=%v",
			onlyA, onlyB))
	}
	out = append(out, fmt.Sprintf("  totals     %d vs %d over %d vs %d roles",
		total(a.Roles), total(b.Roles), len(a.Roles), len(b.Roles)))
	return out
}

// termDelta is what the FIRST signature has that the second does not, and the reverse.
//
// Named from the second's point of view, because the question this exists for is asked that way:
// "what did the remembered place have that this reading does not" is a different fact from "what
// does this reading have that the remembered place did not".
func termDelta(current, remembered []observe.InterfaceTerm) (lost, gained []string) {
	has := func(list []observe.InterfaceTerm, t observe.InterfaceTerm) bool {
		for _, got := range list {
			if got == t {
				return true
			}
		}
		return false
	}
	for _, t := range remembered {
		if !has(current, t) {
			lost = append(lost, string(t))
		}
	}
	for _, t := range current {
		if !has(remembered, t) {
			gained = append(gained, string(t))
		}
	}
	sort.Strings(lost)
	sort.Strings(gained)
	return lost, gained
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
