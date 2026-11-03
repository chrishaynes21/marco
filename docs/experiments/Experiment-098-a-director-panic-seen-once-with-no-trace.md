---
type: experiment
status: partial
date: 2026-11-03
backend:
  - suite
fixture: "Full `go build ./... && go test ./...` from the repository root, in the working tree of the scope-vocabulary/default and in-place-admission slice (ADR-296), uncommitted on top of `6c0498b`. One machine. The output was piped through a filter that kept only `ok`/`FAIL`/`panic` lines and `_test.go:N:` lines."
result: "`cmd/director` FAILED ONCE, after 31.266s, with `panic: runtime error: slice bounds out of range [5444:5392]`. The stack trace WAS NOT RETAINED — the filter discarded every line after the panic line — so the panicking function, the test and the goroutine are unknown. Afterwards: three isolated `go test -count=1 ./cmd/director/` runs passed (182.8s, 175.2s, 159.1s), and a second full uncached `go test -count=1 ./...` with complete output passed (115 packages ok, no FAIL). CAUSE UNKNOWN; RELATIONSHIP TO THE CHANGE UNKNOWN."
supersedes: []
affects:
  - execution
source_paths:
  - cmd/director
---

# Experiment-098 — a Director panic seen once, with no trace

## Why this is its own record

The slice it was seen in ([[ADR-296-one-scope-vocabulary-one-default-and-in-place-is-checked-at-the-door]])
was committed on a final suite run that passed. That sentence is true of the run it describes. It
is not a statement that the suite is reliably green, and a panic folded into a green tally is a
panic nobody looks for again — the reason [[Experiment-094-four-failures-no-mechanism-and-a-window-that-has-to-hold-still]]
was written apart from its gate too.

## What was seen

```
panic: runtime error: slice bounds out of range [5444:5392]
FAIL	github.com/chaynes-simpleclouds/marco/cmd/director	31.266s
```

That is the whole of the evidence. The run's output had been filtered to summary lines before it
was saved, so everything the runtime printed after the panic line — the goroutine, the stack, the
test that was running — was discarded at the moment it was produced. **That was a mistake in how
the run was captured**, and it is why nothing below is a diagnosis.

## What came after

| run | command | result |
|---|---|---|
| 1 (the failure) | `go build ./... && go test ./...`, filtered | `cmd/director` panic at 31.266s; every other package ok except a docs-vault check, since fixed and unrelated |
| 2 | `go test ./cmd/director/` | ok (cached — the cache held a passing result, so this was not a new run) |
| 3–5 | `go test -count=1 ./cmd/director/`, alone | ok, ok, ok |
| 6 | `go test -count=1 ./...`, complete output kept | 115 ok, no FAIL |

## What can and cannot be said

- **Can:** the message, the package, the elapsed time, and that the shape `[5444:5392]` is a slice
  whose start is past its end — typically an index computed against one buffer and applied to
  another, or two searches over text that changed between them.
- **Cannot:** which code panicked; whether it was a test or production code under test; whether
  load from the parallel full run mattered (the failure was under a full run, and the full run
  that passed was also under load); and **whether the uncommitted change interacted with it**.
  No `cmd/director` test reads the files that change touched, and that does **not** rule out an
  interaction: `cmd/director` imports `internal/orchestrator`, which the change edited.
- The passes after it are not evidence of absence. They were not run to establish a rate, and
  this record does not claim one.

## What to do if it recurs

Keep the **complete, unfiltered** output of that run — stdout and stderr — under the
scratchpad or `.tmp/`, and add it here. Do not rerun to collect green results; one trace is worth
more than any number of passes.
