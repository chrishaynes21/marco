// Package docsindex builds a read-only knowledge graph from a directory of Markdown notes.
//
// The Markdown files are the source of truth. This package parses them and reports; it never
// writes a note, and it stores nothing durable. There is deliberately no database: a second
// store would immediately begin to disagree with the files, and the whole point of keeping
// documentation in Git is that there is one copy of it.
//
// What it models is small on purpose — nodes, typed edges, and the frontmatter needed to
// filter them. A richer model would invite the index to become a system of record rather than
// a view over one.
package docsindex

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Node is one note.
//
// ID is the filename without its extension, because that is how Obsidian resolves a
// [[wiki link]] — by basename, from anywhere in the vault. Two notes sharing a basename
// therefore make every link to that name ambiguous, which is why DuplicateNotes is a check
// rather than a curiosity.
type Node struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Path      string   `json:"path"`
	Tags      []string `json:"tags,omitempty"`
	CodePaths []string `json:"code_paths,omitempty"`

	// meta is the note's whole frontmatter, kept so a check can ask about a field
	// without the struct growing one member per rule.
	meta frontmatter
}

// Field returns a frontmatter scalar and whether it carried a value.
func (n *Node) Field(key string) (string, bool) {
	v := n.meta.scalar(key)
	return v, v != ""
}

// Edge is a link from one note to another.
//
// Kind is currently always "links_to". The vault documents a richer relationship vocabulary
// (depends on, produces, supersedes, validated by...), but those are expressed in prose today
// and inferring them from surrounding words would be guessing. When relationships become
// explicit in frontmatter, this is where they land.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
}

// Graph is the parsed vault.
type Graph struct {
	Nodes map[string]*Node `json:"nodes"`
	Edges []Edge           `json:"edges"`

	// duplicates records IDs that appeared more than once, with every path.
	duplicates map[string][]string
	// order preserves discovery order so output is deterministic.
	order []string
}

// wikiLink matches [[Target]], [[Target|alias]] and [[Target#heading]].
//
// The target is everything before the first | or #. An alias changes display text only, and a
// heading anchor addresses a section of the same note; neither changes which note is meant.
var wikiLink = regexp.MustCompile(`\[\[([^\]\[|#]+)(?:#[^\]\[|]*)?(?:\|[^\]\[]*)?\]\]`)

// Build parses every .md file under root into a graph.
func Build(root string) (*Graph, error) {
	g := &Graph{
		Nodes:      map[string]*Node{},
		duplicates: map[string][]string{},
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// .obsidian holds editor state, not knowledge.
			if name := d.Name(); strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		return g.add(root, path)
	})
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Graph) add(root, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)

	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fm, body := splitFrontmatter(string(data))
	meta := parseFrontmatter(fm)

	n := &Node{
		ID:        id,
		Title:     firstHeading(body, id),
		Type:      meta.scalar("type"),
		Status:    meta.scalar("status"),
		Path:      rel,
		Tags:      meta.list("tags"),
		CodePaths: meta.list("source_paths"),
		meta:      meta,
	}

	if prev, exists := g.Nodes[id]; exists {
		if len(g.duplicates[id]) == 0 {
			g.duplicates[id] = []string{prev.Path}
		}
		g.duplicates[id] = append(g.duplicates[id], rel)
		// Keep the first. Which one wins does not matter, because a duplicate is
		// reported as an error rather than silently resolved.
		return nil
	}
	g.Nodes[id] = n
	g.order = append(g.order, id)

	for _, l := range findLinks(body) {
		g.Edges = append(g.Edges, Edge{From: id, To: l.target, Kind: "links_to", Line: l.line})
	}
	return nil
}

type link struct {
	target string
	line   int
}

// findLinks extracts wiki links, ignoring fenced code blocks.
//
// A link inside a fence is an example of a link, not a link. Counting those would make every
// document that explains the convention fail the check that enforces it.
func findLinks(body string) []link {
	var out []link
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	fenced := false
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if trimmed := strings.TrimSpace(text); strings.HasPrefix(trimmed, "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, m := range wikiLink.FindAllStringSubmatch(text, -1) {
			target := strings.TrimSpace(m[1])
			if target != "" {
				out = append(out, link{target: target, line: line})
			}
		}
	}
	return out
}

// frontmatter is a parsed YAML header.
//
// A deliberately small subset: scalars, inline empty lists, and block sequences. That covers
// every field the vault actually uses, and refusing to grow beyond it keeps a YAML dependency
// out of a stdlib-only module. Anything more elaborate belongs in the note body.
type frontmatter map[string][]string

func (f frontmatter) scalar(key string) string {
	if v := f[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (f frontmatter) list(key string) []string {
	if v := f[key]; len(v) > 0 {
		out := make([]string, len(v))
		copy(out, v)
		return out
	}
	return nil
}

// splitFrontmatter separates a leading --- block from the body.
func splitFrontmatter(s string) (string, string) {
	s = strings.TrimPrefix(s, string(rune(0xFEFF))) // tolerate a UTF-8 BOM
	if !strings.HasPrefix(s, "---") {
		return "", s
	}
	rest := s[3:]
	if !strings.HasPrefix(rest, "\n") && !strings.HasPrefix(rest, "\r\n") {
		return "", s
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", s
	}
	head := rest[:idx]
	tail := rest[idx+4:]
	return head, tail
}

func parseFrontmatter(s string) frontmatter {
	fm := frontmatter{}
	if s == "" {
		return fm
	}
	var key string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		raw := sc.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// A block-sequence item belongs to the key above it.
		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			if key != "" {
				if item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-")); item != "" {
					fm[key] = append(fm[key], unquote(item))
				}
			}
			continue
		}
		colon := strings.Index(trimmed, ":")
		if colon < 0 {
			continue
		}
		key = strings.TrimSpace(trimmed[:colon])
		value := strings.TrimSpace(trimmed[colon+1:])
		switch value {
		case "", "[]":
			// A key with no inline value either opens a block sequence or is an
			// explicitly empty list. Register it so "declared but empty" is
			// distinguishable from "absent".
			if _, seen := fm[key]; !seen {
				fm[key] = nil
			}
		default:
			fm[key] = append(fm[key], unquote(value))
		}
	}
	return fm
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func firstHeading(body, fallback string) string {
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return fallback
}

// Nodes returns every node in discovery order.
func (g *Graph) OrderedNodes() []*Node {
	out := make([]*Node, 0, len(g.order))
	for _, id := range g.order {
		out = append(out, g.Nodes[id])
	}
	return out
}

// Problem is one thing wrong with the vault.
type Problem struct {
	Kind    string `json:"kind"`
	Note    string `json:"note"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Detail  string `json:"detail"`
	Remedy  string `json:"remedy,omitempty"`
	Warning bool   `json:"warning,omitempty"`
}

// requiredFields is the frontmatter each note type must carry.
//
// Not every note needs metadata — a milestone record written before the vault existed is
// still perfectly good documentation — so an absent `type` is not an error. The rule only
// binds notes that claim a type.
var requiredFields = map[string][]string{
	"subsystem":  {"status", "source_paths"},
	"decision":   {"status", "date"},
	"experiment": {"status", "result"},
}

// Check validates the graph and returns every problem, deterministically ordered.
//
// codeRoot is the repository root that source_paths are resolved against.
func (g *Graph) Check(codeRoot string) []Problem {
	var problems []Problem

	// Duplicate notes first: they make every other finding about those IDs unreliable,
	// because a link to the name cannot be resolved to one file.
	dupIDs := make([]string, 0, len(g.duplicates))
	for id := range g.duplicates {
		dupIDs = append(dupIDs, id)
	}
	sort.Strings(dupIDs)
	for _, id := range dupIDs {
		problems = append(problems, Problem{
			Kind:   "duplicate-note",
			Note:   id,
			Path:   g.duplicates[id][0],
			Detail: fmt.Sprintf("%d notes share the name %q: %s", len(g.duplicates[id]), id, strings.Join(g.duplicates[id], ", ")),
			Remedy: "rename one; a wiki link resolves by basename and cannot address either",
		})
	}

	for _, e := range g.Edges {
		if _, ok := g.Nodes[e.To]; ok {
			continue
		}
		problems = append(problems, Problem{
			Kind:   "broken-link",
			Note:   e.From,
			Path:   g.Nodes[e.From].Path,
			Line:   e.Line,
			Detail: fmt.Sprintf("[[%s]] resolves to no note", e.To),
			Remedy: "create the note or correct the link",
		})
	}

	for _, n := range g.OrderedNodes() {
		for _, field := range requiredFields[n.Type] {
			if _, present := n.Field(field); !present {
				problems = append(problems, Problem{
					Kind:   "missing-frontmatter",
					Note:   n.ID,
					Path:   n.Path,
					Detail: fmt.Sprintf("type %q requires frontmatter field %q", n.Type, field),
					Remedy: "add the field, or drop the type if the note is not canonical",
				})
			}
		}
		for _, cp := range n.CodePaths {
			full := filepath.Join(codeRoot, filepath.FromSlash(cp))
			if _, err := os.Stat(full); err != nil {
				problems = append(problems, Problem{
					Kind:   "stale-code-path",
					Note:   n.ID,
					Path:   n.Path,
					Detail: fmt.Sprintf("source_paths entry %q does not exist", cp),
					Remedy: "update source_paths — the code moved or was deleted",
				})
			}
		}
	}
	return problems
}

// Summary counts a graph for reporting.
type Summary struct {
	Notes   int            `json:"notes"`
	Links   int            `json:"links"`
	ByType  map[string]int `json:"by_type"`
	Orphans []string       `json:"orphans,omitempty"`
}

// Describe summarises the graph.
//
// Orphans — notes nothing links to — are reported but are NOT a problem. A milestone record
// that predates the vault is orphaned and perfectly valid; the count is there so somebody can
// notice a canonical note that never got wired in.
func (g *Graph) Describe() Summary {
	s := Summary{Notes: len(g.Nodes), Links: len(g.Edges), ByType: map[string]int{}}
	linked := map[string]bool{}
	for _, e := range g.Edges {
		linked[e.To] = true
	}
	for _, n := range g.OrderedNodes() {
		t := n.Type
		if t == "" {
			t = "(untyped)"
		}
		s.ByType[t]++
		if !linked[n.ID] {
			s.Orphans = append(s.Orphans, n.ID)
		}
	}
	sort.Strings(s.Orphans)
	return s
}
