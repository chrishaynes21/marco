// Command docscheck validates the Markdown knowledge base in docs/.
//
// It is read-only. It parses notes, builds a link graph, and reports; it never rewrites a
// note and it writes no index to disk. See docs/AI-CONTEXT.md for what the vault is.
//
//	go run ./cmd/docscheck             # human-readable, exit 1 on any problem
//	go run ./cmd/docscheck --json      # machine-readable
//	go run ./cmd/docscheck --root docs --code .
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/docsindex"
)

func main() {
	root := flag.String("root", "docs", "vault directory to validate")
	code := flag.String("code", ".", "repository root that source_paths resolve against")
	asJSON := flag.Bool("json", false, "emit JSON")
	quiet := flag.Bool("quiet", false, "suppress the summary; print problems only")
	flag.Parse()

	g, err := docsindex.Build(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docscheck: %v\n", err)
		os.Exit(2)
	}
	problems := g.Check(*code)
	summary := g.Describe()

	if *asJSON {
		out := struct {
			Summary  docsindex.Summary   `json:"summary"`
			Problems []docsindex.Problem `json:"problems"`
		}{summary, problems}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "docscheck: %v\n", err)
			os.Exit(2)
		}
		if len(problems) > 0 {
			os.Exit(1)
		}
		return
	}

	if !*quiet {
		fmt.Printf("%d notes, %d links\n", summary.Notes, summary.Links)
		for _, t := range sortedKeys(summary.ByType) {
			fmt.Printf("  %-12s %d\n", t, summary.ByType[t])
		}
		if len(summary.Orphans) > 0 {
			fmt.Printf("  %d notes nothing links to (not an error): %v\n",
				len(summary.Orphans), summary.Orphans)
		}
		fmt.Println()
	}

	if len(problems) == 0 {
		fmt.Println("no problems")
		return
	}
	for _, p := range problems {
		where := p.Path
		if p.Line > 0 {
			where = fmt.Sprintf("%s:%d", p.Path, p.Line)
		}
		fmt.Printf("%-20s %s\n    %s\n", p.Kind, where, p.Detail)
		if p.Remedy != "" {
			fmt.Printf("    → %s\n", p.Remedy)
		}
	}
	fmt.Printf("\n%d problems\n", len(problems))
	os.Exit(1)
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
