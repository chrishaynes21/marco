---
description: Run, check, or test a Marco (.marco) program through the engine CLI
argument-hint: "[run|check|test|contracts] <file.marco>"
---

Drive the Marco engine CLI against a `.marco` program. Use this when the user
wants to execute, validate, or test a Marco source file.

Arguments: `$ARGUMENTS` — an optional subcommand followed by a file path.

## How to run

The CLI lives at `./cmd/marco`. Invoke it without a separate build step:

```
go run ./cmd/marco <subcommand> <file.marco>
```

On Windows use the PowerShell tool; `go run` works the same. The four
subcommands (see `cmd/marco/main.go`):

- `run <file>` — execute the program; prints `log` output to stdout. A runtime
  error or a failed `expect` exits non-zero with the diagnostic on stderr.
- `check <file>` — static validation only (no execution). Add `--json` before
  the path (`check --json <file>`) for machine-readable diagnostics with stable
  `code` fields. Empty output / `[]` means the program is well-formed.
- `test <file>` — run the `test ...` blocks in the file; reports `PASS`/`FAIL`
  per test and exits non-zero on any failure.
- `contracts <file>` — print the inferred/declared contracts for each action.

## What to do

1. Parse `$ARGUMENTS`. If it names a subcommand, use it; otherwise default to
   `check` first (cheap, catches errors), then `run`. If no file is given, ask
   for one or offer a `testdata/<case>/program.marco` example.
2. Run the chosen subcommand and show the output verbatim.
3. If `check`/`run` reports a diagnostic, read the cited `file:line:col` and the
   relevant spec section under `spec/` (e.g. Contracts, Phrases) before
   proposing a fix — Marco is strict at compile time, so the diagnostic usually
   names the exact rule.
4. When iterating on a new program, prefer `check` after each edit and `run`
   once it's clean. Reach for `test` when the file has `test ...` blocks.

## Notes

- Golden examples to copy from live under `testdata/<case>/program.marco`, each
  paired with its expected output — a good reference for syntax.
- The language spec is the source of truth in `spec/`; the engine design notes
  are in `.claude/engine/`.
