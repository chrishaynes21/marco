---
type: experiment
status: complete
date: 2026-11-03
backend:
  - headless-edge
fixture: "The control centre's Config view script, taken verbatim from `cmd/marco/edit.go` by `run.sh`, loaded in headless Edge beside `harness.html` with `fetch` stubbed to answer a configured leader of `f3` and an activation phrase of `marco`."
result: "The leader is shown as `f3`, a save with nothing changed posts nothing, and changing only the activation phrase posts `{\"wake\":\"jarvis\"}` and nothing else. Output in `output.txt`."
supersedes: []
affects:
  - interface
source_paths:
  - cmd/marco/edit.go
  - docs/experiments/S5-control-centre-headless-check/run.sh
---

# S5 control-centre headless check

**Reproducible evidence, not a regression test.** Nothing runs this automatically, it is not in
any suite or in the gate, and a change that breaks the page will not fail anything until somebody
runs it again. It exists because the Config view's script cannot be entered from a Go test, and
[[ADR-300-one-owner-for-the-widgets-settings]] rests on what the page sends.

Run from the repository root, on Windows with Edge installed:

```sh
sh docs/experiments/S5-control-centre-headless-check/run.sh
```

It takes the script from `edit.go` at run time, so it checks the page as it is now, not a copy.
The output recorded when ADR-300 was written is `output.txt`:

- `shownLeader` is what the leader dropdown selected for a configured `f3`, which is not one of its
  five choices (it used to select the backtick, and a save then wrote it).
- `posted` is every POST body, in order: none for the save with nothing changed, then only the
  activation phrase.
- `banners` is what the page said each time.
