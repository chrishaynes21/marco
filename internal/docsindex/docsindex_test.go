package docsindex

import (
	"os"
	"path/filepath"
	"testing"
)

// write creates a note in a temporary vault.
func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func kinds(problems []Problem) map[string]int {
	out := map[string]int{}
	for _, p := range problems {
		out[p.Kind]++
	}
	return out
}

func TestParsesFrontmatterAndLinks(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "A.md", `---
type: subsystem
status: active
source_paths:
  - internal/docsindex
tags:
  - one
  - two
---

# Alpha

Links to [[B]] and to [[B|an alias]] and to [[B#a heading]].
`)
	write(t, dir, "B.md", "# Beta\n")

	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("want 2 notes, got %d", len(g.Nodes))
	}
	a := g.Nodes["A"]
	if a.Title != "Alpha" {
		t.Errorf("title = %q, want Alpha", a.Title)
	}
	if a.Type != "subsystem" || a.Status != "active" {
		t.Errorf("frontmatter scalars not parsed: %+v", a)
	}
	if len(a.Tags) != 2 || a.Tags[0] != "one" {
		t.Errorf("block sequence not parsed: %v", a.Tags)
	}
	if len(g.Edges) != 3 {
		t.Errorf("want 3 links (plain, alias, heading), got %d", len(g.Edges))
	}
	for _, e := range g.Edges {
		if e.To != "B" {
			t.Errorf("link target = %q, want B — alias and heading must not change the target", e.To)
		}
	}
}

// A link inside a fenced block is an example of a link. Counting it would make any note that
// documents the convention fail the check that enforces it.
func TestLinksInsideCodeFencesAreIgnored(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "A.md", "# A\n\n```\n[[NoSuchNote]]\n```\n\nReal link: [[A]]\n")

	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("want 1 link, got %d: %+v", len(g.Edges), g.Edges)
	}
	if got := kinds(g.Check(dir))["broken-link"]; got != 0 {
		t.Errorf("a fenced link was reported broken (%d)", got)
	}
}

func TestBrokenLinkIsReported(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "A.md", "# A\n\nSee [[Missing]].\n")

	g, _ := Build(dir)
	problems := g.Check(dir)
	if kinds(problems)["broken-link"] != 1 {
		t.Fatalf("want 1 broken link, got %+v", problems)
	}
	if problems[0].Line != 3 {
		t.Errorf("line = %d, want 3", problems[0].Line)
	}
}

// Obsidian resolves a wiki link by basename from anywhere in the vault, so two notes sharing
// a name make every link to that name ambiguous. Neither can be addressed.
func TestDuplicateNoteNamesAreReported(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "one/Vision.md", "# Vision\n")
	write(t, dir, "two/Vision.md", "# Vision\n")

	g, _ := Build(dir)
	problems := g.Check(dir)
	if kinds(problems)["duplicate-note"] != 1 {
		t.Fatalf("want 1 duplicate, got %+v", problems)
	}
	if len(g.Nodes) != 1 {
		t.Errorf("a duplicate must not create a second node: %d", len(g.Nodes))
	}
}

func TestStaleCodePathIsReported(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "A.md", `---
type: subsystem
status: active
source_paths:
  - definitely/not/here
---

# A
`)
	g, _ := Build(dir)
	problems := g.Check(dir)
	if kinds(problems)["stale-code-path"] != 1 {
		t.Fatalf("want 1 stale path, got %+v", problems)
	}
}

func TestLiveCodePathIsNotReported(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "sub/real.txt", "x")
	write(t, dir, "A.md", `---
type: subsystem
status: active
source_paths:
  - sub/real.txt
---

# A
`)
	g, _ := Build(dir)
	if got := kinds(g.Check(dir))["stale-code-path"]; got != 0 {
		t.Errorf("an existing path was called stale (%d)", got)
	}
}

// A note that claims a type must carry that type's metadata. A note with NO type is ordinary
// documentation and is left alone — milestone records predate the vault and are still good.
func TestRequiredFrontmatterIsEnforcedOnlyForTypedNotes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "typed.md", "---\ntype: decision\n---\n\n# Typed\n")
	write(t, dir, "untyped.md", "# Untyped, and that is fine\n")

	g, _ := Build(dir)
	problems := g.Check(dir)
	// decision requires status and date; both are absent.
	if got := kinds(problems)["missing-frontmatter"]; got != 2 {
		t.Fatalf("want 2 missing fields, got %d: %+v", got, problems)
	}
	for _, p := range problems {
		if p.Note == "untyped" {
			t.Errorf("an untyped note was held to a schema: %+v", p)
		}
	}
}

// An empty list is a declared value, not a missing one — `supersedes: []` means "nothing",
// which is different from having forgotten the field.
func TestEmptyListIsDistinctFromAbsent(t *testing.T) {
	fm := parseFrontmatter("supersedes: []\nstatus: accepted\n")
	if _, declared := fm["supersedes"]; !declared {
		t.Error("an explicitly empty list should be recorded as declared")
	}
	if len(fm.list("supersedes")) != 0 {
		t.Error("an empty list should carry no items")
	}
	if fm.scalar("status") != "accepted" {
		t.Error("a scalar beside an empty list was lost")
	}
}

func TestDescribeCountsAndFindsOrphans(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "A.md", "---\ntype: subsystem\n---\n\n# A\n\n[[B]]\n")
	write(t, dir, "B.md", "---\ntype: subsystem\n---\n\n# B\n")
	write(t, dir, "C.md", "# C\n")

	g, _ := Build(dir)
	s := g.Describe()
	if s.Notes != 3 || s.Links != 1 {
		t.Fatalf("notes=%d links=%d", s.Notes, s.Links)
	}
	if s.ByType["subsystem"] != 2 || s.ByType["(untyped)"] != 1 {
		t.Errorf("by-type = %v", s.ByType)
	}
	// A and C are linked from nothing.
	if len(s.Orphans) != 2 || s.Orphans[0] != "A" || s.Orphans[1] != "C" {
		t.Errorf("orphans = %v, want [A C] sorted", s.Orphans)
	}
}
