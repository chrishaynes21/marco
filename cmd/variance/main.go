// variance reads a set of cold-establishment sandboxes and reports what each run made durable.
//
// Read-only. It opens each run's semantic memory and asks what composition was canonized, so the
// question "why did 14/19/30 win one run and 15/20/32 another" has a distribution behind it
// instead of two anecdotes.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

func key(roles map[string]int) string {
	ks := make([]string, 0, len(roles))
	for r, n := range roles {
		ks = append(ks, fmt.Sprintf("%s=%d", r, n))
	}
	sort.Strings(ks)
	return strings.Join(ks, " ")
}

func main() {
	root, app := os.Args[1], os.Args[2]
	dirs, _ := filepath.Glob(filepath.Join(root, "run-*"))
	sort.Strings(dirs)

	variants := map[string]int{}
	perRole := map[string][]int{}
	runs, empty := 0, 0
	for _, d := range dirs {
		store, un := semanticmemory.Open(filepath.Join(d, "semantic-memory.json"))
		if un != "" {
			continue
		}
		var places []observe.RememberedSubject
		for _, s := range store.Subjects() {
			if s.Application == app && s.Structure.Subject == observe.SubjectState {
				places = append(places, s)
			}
		}
		if len(places) == 0 {
			empty++
			continue
		}
		if len(places) > 1 {
			fmt.Printf("%s: %d places in one run (expected 1)\n", filepath.Base(d), len(places))
		}
		runs++
		p := places[0]
		variants[key(p.Structure.Roles)]++
		for r, n := range p.Structure.Roles {
			perRole[r] = append(perRole[r], n)
		}
	}
	fmt.Printf("runs with an established place: %d   runs with none: %d\n\n", runs, empty)

	type vc struct {
		sig string
		n   int
	}
	var vs []vc
	for s, n := range variants {
		vs = append(vs, vc{s, n})
	}
	sort.Slice(vs, func(i, j int) bool {
		if vs[i].n != vs[j].n {
			return vs[i].n > vs[j].n
		}
		return vs[i].sig < vs[j].sig
	})
	fmt.Printf("DISTINCT SIGNATURE VARIANTS: %d\n", len(vs))
	for i, v := range vs {
		fmt.Printf("  variant %c  %2d/%d  %s\n", 'A'+rune(i), v.n, runs, v.sig)
	}

	fmt.Printf("\nPER-ROLE VARIANCE\n  %-12s %5s %5s %5s %6s %s\n",
		"role", "min", "max", "mode", "mode/n", "values")
	roles := make([]string, 0, len(perRole))
	for r := range perRole {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	for _, r := range roles {
		vals := perRole[r]
		sort.Ints(vals)
		counts := map[int]int{}
		for _, v := range vals {
			counts[v]++
		}
		mode, modeN := 0, -1
		for v, c := range counts {
			if c > modeN || (c == modeN && v > mode) {
				mode, modeN = v, c
			}
		}
		spread := ""
		if vals[0] != vals[len(vals)-1] {
			spread = "   <-- VARIES"
		}
		fmt.Printf("  %-12s %5d %5d %5d %5d/%d %v%s\n",
			r, vals[0], vals[len(vals)-1], mode, modeN, len(vals), vals, spread)
	}
}
