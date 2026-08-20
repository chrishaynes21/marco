// bakeoff evaluates the whole-composition producer against real recorded sessions.
//
// It changes nothing. For every ScreenState in every session it reports what the CURRENT producer
// emitted (independent per-role modes) and what a WHOLE-COMPOSITION producer would emit, and
// whether each is a composition that was actually observed.
//
// The question it exists to answer is not "is the blend possible" — that is proven in isolation —
// but "would requiring a real recurring composition make legitimate places impossible to
// establish". That is a fact about real sessions, so it is read off real sessions.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

type session struct {
	ID          string `json:"id"`
	Application string `json:"application"`
	Stats       struct {
		Shadow observe.ShadowTotals `json:"shadow"`
	} `json:"stats"`
}

// key renders a role composition the same way the state's tally does.
func key(roles map[string]int) string {
	parts := make([]string, 0, len(roles))
	for r, n := range roles {
		if n <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d", r, n))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

type ranked struct {
	sig string
	n   int
}

func rank(m map[string]int) []ranked {
	out := make([]ranked, 0, len(m))
	for s, n := range m {
		out = append(out, ranked{s, n})
	}
	// Most frequent first; ties broken by the composition string so the answer is stable
	// whatever order the map iterates in.
	sort.Slice(out, func(i, j int) bool {
		if out[i].n != out[j].n {
			return out[i].n > out[j].n
		}
		return out[i].sig < out[j].sig
	})
	return out
}

func main() {
	var files []string
	for _, arg := range os.Args[1:] {
		g, _ := filepath.Glob(arg)
		files = append(files, g...)
	}
	sort.Strings(files)

	var states, curEst, wholeEst, lost, synth, wholeSynth, multi int
	fmt.Printf("%-14s %-9s %5s %5s %6s %6s %-8s %-8s %s\n",
		"session", "state", "obs", "comps", "top", "2nd", "current", "whole", "note")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s session
		if json.Unmarshal(b, &s) != nil {
			continue
		}
		for _, st := range s.Stats.Shadow.States {
			if len(st.Compositions) == 0 {
				continue
			}
			states++
			r := rank(st.Compositions)
			top, second := r[0], ranked{}
			if len(r) > 1 {
				second = r[1]
			}
			if len(r) > 1 {
				multi++
			}
			emitted := key(st.Roles)
			observed := st.Compositions[emitted] > 0
			cur := st.Settled
			whole := top.n >= observe.StatePromotionCount
			if cur {
				curEst++
			}
			if whole {
				wholeEst++
			}
			if cur && !whole {
				lost++
			}
			if cur && !observed {
				synth++
			}
			note := ""
			if cur && !observed {
				note = "SYNTHETIC"
			}
			if cur && !whole {
				note += " LOST-BY-WHOLE"
			}
			fmt.Printf("%-14s %-9s %5d %5d %6d %6d %-8v %-8v %s\n",
				strings.TrimSuffix(filepath.Base(f), ".json"), st.ID,
				st.Inferences, len(r), top.n, second.n, cur, whole, note)
		}
	}
	fmt.Printf("\nSTATES_ANALYZED: %d   with >1 distinct composition: %d\n", states, multi)
	fmt.Printf("CURRENT_RULE_ESTABLISHES: %d\nWHOLE_RULE_ESTABLISHES:   %d\n", curEst, wholeEst)
	fmt.Printf("STATES_LOST_BY_WHOLE_RULE: %d\n", lost)
	fmt.Printf("CURRENT_SYNTHETIC_SIGNATURES: %d\nWHOLE_RULE_SYNTHETIC_SIGNATURES: %d\n",
		synth, wholeSynth)
}
