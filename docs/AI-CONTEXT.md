---
type: guide
status: active
updated: 2026-08-10
---

# How to use this knowledge base

This directory is an Obsidian-compatible vault. It is plain Markdown first: every note is
readable with `cat`, greppable with `rg`, and diffable in Git. Obsidian is an optional
interface for viewing the graph, never a dependency.

## Retrieval order

0. If you cannot read a Marco program yet, read [[Reading-Marco]] first. It is one page, and
   most of this vault is about what generated Marco is allowed to say.
1. Read [[Director]] for the system map.
2. Read the relevant note in `subsystems/`.
3. Follow its linked ADRs in `decisions/`. Those are the constraints you must not break.
4. Read only the **latest** linked experiment for current evidence. Older experiment
   records are kept for history and are frequently superseded.
5. Treat `HANDOFF.md` as **navigation, not architectural truth**. It is a chronological
   narrative that grows append-only; where it disagrees with a subsystem note or an ADR,
   the note and the ADR win.
6. **Verify claims against code and tests before modifying implementation.** Every note
   carries `source_paths` in its frontmatter for exactly this. A note describes intent;
   only the code is the system.
7. After work: update the subsystem note, the relevant ADR, and the experiment record.
8. Do not duplicate documentation already represented by a canonical note. Link to it.

## Auditing rather than building

If the task is to **review** the system rather than extend it, read [[Audit]] in place of step 2.
It states each invariant as something that must be false to break it, names where it is enforced,
and lists — deliberately — where the evidence is thinnest. [[Wiring-Tests]] explains why a claim
here is only believed once somebody has broken it on purpose and watched the right test fail.

## Do not load the whole vault

A session should read the system map, one or two subsystem notes, and their ADRs. That is
usually under 2,000 lines. Reading every milestone document is both unnecessary and
actively misleading, because milestone documents describe the state at the time they were
written.

## Note types

| type | lives in | answers |
|---|---|---|
| `map` | `Director.md` | what the parts are and how they connect |
| `subsystem` | `subsystems/` | what one part is responsible for, and what it may not do |
| `decision` | `decisions/` | why a constraint exists, and what enforces it |
| `experiment` | `experiments/` | what was measured, under what conditions, and what it means |
| `milestone` | `director-*.md` | what was built in one sitting, with its Known gaps |

Milestone documents are the existing `docs/director-*.md` files. They were not moved: they carry
the real detail and the reasoning, and subsystem notes are thin canonical indexes **over** them.

Every one of them now declares `status: historical` or `status: complete` and opens with a banner
saying so. **They are history, not spec.** Where a milestone disagrees with a subsystem note or an
ADR, the note and the ADR win — the same rule that applies to `HANDOFF.md`. Read one when you want
to know *why* a thing is the way it is, or to reuse a method; never to learn the current state.

## Relationship vocabulary

Links are deliberate. The vault uses a small, fixed set of relationships, and a link is
expected to mean one of them:

- **depends on** — cannot function without
- **produces** — emits something another subsystem consumes
- **consumes** — reads something another subsystem produced
- **supersedes** — replaces an earlier decision or experiment
- **validated by** — the test or evidence that holds a claim up
- **blocked by** — cannot progress until something else lands
- **implemented in** — the code path
- **related experiment** — evidence bearing on this note

A link that means none of these is probably decoration. Decorative links make the graph
larger and less useful; adding one costs everybody who later trusts it.

## Validating the vault

```sh
go run ./cmd/docscheck            # broken links, duplicate notes, stale source_paths
go run ./cmd/docscheck --json     # machine-readable
```

`docscheck` is read-only. It parses Markdown and reports; it never rewrites a note. The
Markdown files remain the single source of truth — there is no database, and no process
that edits its own beliefs.

## What this vault is not

It is not a memory system and it is not a "brain". It adds structure, links, metadata and
navigation around documentation that already existed. A graph is only worth having when
the notes contain deliberate relationships, so the number of links is kept small on
purpose.

## Before you call a subsystem done

Implementation correctness and integration correctness are separate gates. This vault
records two occasions where a complete, well-tested mechanism was never invoked by the
running Director, and every gate stayed green both times. Read [[Wiring-Tests]] before
writing "implemented and tested" anywhere — and before believing such a claim you find.
