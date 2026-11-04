---
type: experiment
status: partial
date: 2026-11-03
backend:
  - suite
fixture: "Full `go build ./... && go test -count=1 ./...` from the repository root, then Studio's and the overlay's suites, in the UNCOMMITTED S2 working tree (ADR-297, second review: the rename copy-carry-finish order, with the copy still being removed after a failed create or carry) on top of `922220f`. One machine, Windows 11 Home. Complete, unfiltered output kept beside this record in `Experiment-099-output/`."
result: "`cmd/marco` FAILED one test, `TestTheRecordDoesNotRelabelALaterGenerationAsItsOwn` (`generationrecord_test.go:115`). It performs the same routine twice, once with a later generation stamp and once without, through the REAL OS host against an application that does not exist (`no-such-application-9f2c`), and requires both runs to stop for the same stated reason. The stamped run stopped after the 20s launch timeout with *did not open within the launch timeout*; the unstamped run stopped at once with *is not running and could not be launched*. Every other engine package (115), Studio and the overlay passed. CAUSE UNKNOWN. Whether the generation stamp, the uncommitted change or the machine produced the difference is NOT established."
supersedes: []
affects:
  - execution
source_paths:
  - cmd/marco/generationrecord_test.go
  - cmd/marco/guardedroute_test.go
  - cmd/marco/routinerecord_test.go
---

# Experiment-099: two activation failures for one absent application

## Why this is its own record

S2 ([[ADR-297-a-shortcut-names-its-routine-and-has-one-owner]]) is held uncommitted, and this
failure must not be folded into its tally either way. It is recorded apart for the same reason as
[[Experiment-098-a-director-panic-seen-once-with-no-trace]]: a failure counted inside somebody
else's green result is one nobody looks for again.

## What was seen

```
--- FAIL: TestTheRecordDoesNotRelabelALaterGenerationAsItsOwn (20.10s)
    generationrecord_test.go:115: the stamped routine stopped because a-routine stopped: the
    no-such-application-9f2c window could not be brought to the front: activate:
    "no-such-application-9f2c" did not open within the launch timeout and the unstamped one
    because a-routine stopped: the no-such-application-9f2c window could not be brought to the
    front: activate: "no-such-application-9f2c" is not running and could not be launched
FAIL	github.com/chaynes-simpleclouds/marco/cmd/marco	118.642s
```

In the same `cmd/marco` output, other tests that activate the same absent application also show
both sentences: *did not open within the launch timeout* 5 times and *is not running and could
not be launched* 14 times. Those tests compare nothing across two runs, so they pass either way.
Packages that pass print no logs, so the earlier full runs' outputs (which passed `cmd/marco`)
cannot say how often each outcome happened there.

## What can and cannot be said

- **Can:** that two real activation attempts for the same absent application, in one test,
  failed in two different ways, and that the test is written to fail exactly then. The fixture
  (`aLibraryWith`) uses the real `oshost`, so the outcome depends on what Windows does when asked
  to launch a name that resolves to nothing.
- **Cannot:** that the generation stamp caused the difference. The two different launch failures
  are equally consistent with the machine answering the same request two ways.
- **Cannot:** that the change under test is independent of it. The failing step is in code that
  change does not edit, and that does not establish independence: `cmd/marco`'s intake, which
  the change did edit, is on the path of both runs.

## What is not done, deliberately

- The comparison is not weakened. It is the point of the test: an unknown generation must run
  exactly as an unstamped routine does.
- No rerun to get a green result.
- **A possible separate correction, not made here:** give that test's fixture a host whose
  activation of the absent application fails one deterministic way, so the two runs can differ
  only because of the stamp, and keep the generation-equivalence assertion as it is. That would be
  its own change, with its own record.

## If it recurs

Keep the complete output (as here) and add it to this record. A second instance of the
*did not open within the launch timeout* outcome on the UNSTAMPED run would be evidence that the
stamp does not decide it; this record does not claim that yet.

## Later runs

| when | what | this test |
|---|---|---|
| 2026-11-03, the S3 working tree | full `go test -count=1 ./...`, complete output kept outside the repository | passed |

A pass is not evidence the cause is gone; it was not run to establish a rate, and this record still
claims nothing about the cause.
