---
type: reference
status: current
date: 2026-10-29
source_paths:
  - internal/recorder
  - internal/orchestrator
  - internal/platform/winprovider
  - internal/runtime
  - cmd/gate
  - cmd/director
---

# Alpha candidate — `feat/demonstrated-acts` at `2770d46`

> **Clean-checkout verification: PASSED for `2770d46`**, on 2026-10-29 at 18:21Z. A fresh
> `--no-hardlinks` clone, checked out detached, holding only what git has — no untracked file,
> nothing ignored — then `go build ./...` and `go run ./cmd/gate` inside it:
> *passed, 0 failures, 0 findings, 30 tolerated skips, in 5m8s*, with go1.26.2.
> It did NOT pack, install, run the desktop-tagged tests or any desktop scenario.
> Full record in `cmd/cleanroom`'s own output; the stage is
> [[ADR-270-packaging-is-gated-on-a-clean-checkout]].

What a person can do with this tree, end to end: name a command in plain words, demonstrate it
once in a window they chose, be asked whether to keep it, and press Play. The demonstration is
admitted from ONE window; the routine is filed under the application it was shown in; and the
run refuses to act on any other one.

This page is the handoff — what landed, what has been verified and with what, what has NOT been
run, and what a reader should not conclude from the green. It is a checkpoint, not an
architectural note; `docs/Director.md` and the ADRs remain the truth about design.

## The four commits this handoff is about

| Commit | Subject | What it settles |
| --- | --- | --- |
| `24b618f` | recorder: a recording is of one window, and the window is chosen | The window is given, not inferred. Membership and the recorded application come from ONE reading. The gate is at the instant of the event and covers the live channel as well as the buffer. Time away comes out of the timestamps rather than a marker beside them. |
| `435b789` | gate: a screen-free stage, and the tests that need the screen behind a tag | Stage 1 is usable while somebody is using their computer. Fourteen Director run-logic tests state their own desktop; three that need live perception are behind `-tags desktop` and REPORTED rather than silently absent. |
| `67e20a0` | runtime: the install decision must not take the frame's lock twice | A status listener installed on a frame that had already finished self-deadlocked on a non-reentrant mutex, hanging the run. |
| `1dcef6f` | repo: a pattern meant for one artifact swallowed 420 lines of source | **The repository as committed did not build.** `.gitignore` carried `coverage.*`, which matched `cmd/perceptionreplay/coverage.go` as readily as a `.out` file, so that package existed on one machine and in no clone. |
| `9833e37` | gate: packing is gated on a clean checkout of the commit | Adding that file fixed the known omission and established nothing about anybody else building the commit. `cmd/cleanroom` clones it, refuses unless the checkout holds only what git has, builds it, runs the screen-free gate there, and names the commit in the result; packing refuses unless that record says PASSED and names the commit about to be packed. |
| `eca0515` | repo: the line ending a clone gets is decided here, not by the machine | **A fresh clone got different bytes.** `* text=auto` with no `eol=` left every unlisted extension to `core.autocrlf`, so the golden corpus came out CRLF and 108 of `internal/driver`'s cases failed with a `got` and a `want` that printed identically. |
| `2770d46` | tests: stop reading one machine, and run what this repository makes | **The suite could not run anywhere else.** Two governance gates walked the owner's route library; five families of test skipped on artifacts only this machine had. Default tests use repository-controlled inputs now; selected integration tests fail on a missing prerequisite instead of skipping. |

Held by, in the same order:

- `internal/recorder` `TestAConfinedApplicationSwitchNamesTheChosenWindow`,
  `internal/orchestrator` `TestTheWindowInFrontIsOfferedFirst`, `plugins/studio`
  `TestTheWindowListReachesThePage`
- `cmd/director` `TestADesktopTestCanObserveButNeverAct`,
  `TestEveryDesktopTaggedFileIsHeldByTheGuard`, `cmd/gate`
  `TestADesktopTestThatFailsStopsTheGate`
- `internal/runtime` `TestAListenerOnAFrameThatAlreadyFinishedDoesNotDeadlock`
- `internal/wiringcheck` `TestNoIgnorePatternHidesASourceFile` and
  `TestTheIgnoreRuleCatchesBothAccidentsItIsAbout`

Two of the four were not asked for, and both are separate commits so either can be reverted on
its own. `67e20a0` was found by making the suite honest — `go test ./...` hit a ten-minute
package timeout. `1dcef6f` was found by checking that the other three could be bisected, which
is the paragraph below.

See [[ADR-268-record-only-the-chosen-window]] and
[[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]].

### The repository did not build from a clone, and nothing could have said so

`go build ./...` passed here and failed in every checkout of this history, for the whole life of
`cmd/perceptionreplay/coverage.go` — 420 lines holding the walker, the classifier and the
coverage report that `workflow.go` calls. `git status` does not list an ignored file; that is
what ignoring means. The suite ran against the working tree, which had the file. So did the
gate. A reviewer reading a commit saw `workflow.go` calling functions that, as far as the
repository was concerned, did not exist, and no tool objected.

It is the second time. `.gitignore` already carries a note about `routes/` having hidden
`internal/routes/origin.go` — the same accident, understood, paid for, and written down as prose
two lines below where `coverage.*` was later added. A comment cannot fail, which is this
repository's own argument for `internal/wiringcheck`; the rule now lives there.

**What this means for bisecting.** `24b618f`, `435b789` and `67e20a0` do not build from a clone,
because the fix is after them. From `1dcef6f` onward a clone builds. `1dcef6f` and `9833e37`
build but do not pass the suite from a clone, because the line endings are fixed in `eca0515`;
`eca0515` passes everything except the five dependency families, which are fixed in `2770d46`.
**`2770d46` is the first commit in this history that a clone can build AND gate.** Each of those
facts came from a throwaway clone or worktree, not from reading the diffs.

### The stage found three things, in three runs

Worth keeping in this order, because each was invisible to the one before it.

| Run | Commit | Result | What it was |
| --- | --- | --- | --- |
| 1 | `9833e37` | 110 failures, 9 findings | 108 line endings, plus the five dependency families |
| 2 | `eca0515` | 2 failures, 9 findings | the five dependency families alone |
| 3 | `2770d46` | **passed** | — |

None of the three was visible to `go build ./...`, the suite, `cmd/gate`, `docscheck` or
`wiringcheck` in the working tree, because all of them read the directory the defects were hidden
in.

## The arc behind them

| Commit | Subject |
| --- | --- |
| `02fae4d` | adapters: carry the whole identity, and say what you did about it |
| `1190786` | director: own route resolution, and both surfaces ask it |
| `8cd62d9` | director: own route resolution, and make the context prove itself |
| `6d86c4c` | orchestrator: the rebuild path joins the route's critical section |
| `3030298` | routes: the version comparison is inside the route's critical section |
| `f4c973a` | orchestrator: consent is to a version, and a fresh save is exclusive |
| `91379f5` | orchestrator: one set of rules for keeping a recording |
| `53f0330` | studio: the resolver is told which application the person chose |
| `a72551f` | studio: not asked is not answered, and a shared name belongs to nobody |

[[ADR-264-the-gate-answers-for-what-did-not-run]] ·
[[ADR-265-say-what-you-want-and-studio-offers-the-routine]] ·
[[ADR-266-one-set-of-rules-for-keeping-a-recording]] ·
[[ADR-267-director-owns-route-resolution]]

## What has been verified, and with what

Every row was run against this tree on 2026-10-29.

| Check | Command | Result |
| --- | --- | --- |
| The working tree builds | `go build ./...` | builds |
| **The repository AS COMMITTED builds AND gates itself** | `go run ./cmd/cleanroom` | **PASSED for `2770d46`** — a fresh clone holding only what git has, then `go build ./...` and `go run ./cmd/gate` inside it: 0 failures, 0 findings, 30 tolerated skips, 5m8s. This is the only check that can say so; the working tree is not evidence about a clone, and three times today the two disagreed. |
| Formatting | `gofmt -l internal cmd plugins` | clean |
| Engine suite | `go test ./...` | green, 73 packages |
| The gate's screen-free stage | `go run ./cmd/gate` | passed — 0 failures, 0 findings, 30 tolerated skips, 4m39s |
| The tests that need the screen | `go test -tags desktop ./cmd/director/` | ok, 6.2s |
| The runtime under the race detector | `go test -race -count=5 ./internal/runtime/` | ok, 2.1s, no race reported |
| The concurrency-heavy programs that drive it | `go test -race -count=5 ./internal/driver/ -run TestGolden/...` over the ten status/parallel/lock cases | ok, 1.4s, no race reported |
| That the detector was armed at all | a deliberate two-goroutine increment in a throwaway module | `WARNING: DATA RACE`, so the two clean rows above are a measurement and not a silent no-op |
| The deadlock, hunted | the same five-count run forty times, twice | 80 attempts, no hang; and the regression test fails in 5s when the fix is reverted |
| Tagged configurations still compile | `go vet -tags resolver ./cmd/director/`, `go vet -tags corpus ./cmd/perceptionreplay/` | both compile |
| The docs vault | `go run ./cmd/docscheck` | no problems |
| Production wiring | `go test ./internal/wiringcheck/` | green |

**The race check was deliberately narrow**, on instruction: the runtime package and the
concurrency-heavy programs that drive it, with no desktop-tagged tests and no input injection.
It is not a repository-wide race campaign and must not be read as one. Running it on Windows
needs cgo and gcc — mingw-w64 via winget `BrechtSanders.WinLibs.POSIX.UCRT`.

**Mutation gates.** The chosen-window corrections: 5 of 5 mutations killed. The classification
slice: 16 of 18, with 2 recorded as unfalsifiable and the reasoning written into the ADR. The
new ignore gate: 2 of 2 — restoring `coverage.*` makes the live gate fail naming the file it
hid, and a `matchesIgnore` that always answers no fails the table in seven places. Across all
three, four of the kills were survivors first: a skip is green, a guard rule is unfalsifiable
from an unbroken tree, a printed stage header is not evidence the stage ran, and a rule about a
broken tree cannot be established from an unbroken one.

## Remaining acceptance checks — NOT RUN

| Check | Command | Why it has not run |
| --- | --- | --- |
| The packaged gate: bundle, isolated install, every registered desktop scenario | `.\gate.ps1` | HELD. It takes the screen and has not been authorised to. Stages 2–5 of 5 are unexercised; stage 1 is the row above. |
| The physical recording trial | a person demonstrating into Studio on the real desktop | NOT EXERCISED, and labelled so since Roadmap 36. Everything about the chosen window is verified by automation only. |
| The live resolver connection | `go test -tags resolver ./cmd/director/` | Needs `plugins/llama` built and a model server answering. No commit here changes the resolver; `435b789` moved that test's build configuration, and the configuration compiles. |
| The full-frame perception corpus | `go test -tags corpus ./cmd/perceptionreplay/` | About 145s, and required for a perception change. No commit here changes perception. |
| `plugins/vision`, `plugins/ocr`, `plugins/llama` | per-plugin suites | Each needs something this repository does not contain — gcc and a vendored runtime, tesseract, a model server — and nothing in the alpha bundle uses them. The gate prints each with its reason. |

## Known limits

- **A machine with no desktop has not been observed running the converted tests.** That it
  would is argued from `winprovider.DesktopForTest` covering every window read, not measured.
  There is no headless Windows machine here to measure it on: this box is Windows Home, with no
  Sandbox, no Hyper-V and no RDP host. An untested environment, not a blocker.
- **Physical capture is unexercised.** Stage 1 passing WHILE A GAME WAS RUNNING is a claim about
  the harness, not about recording from a live desktop.
- **Three declared-BROKEN gates remain** (Roadmap 37): the two segmenter blackout tests in
  `internal/director/observe` and `TestAnAnsweredQuestionIsNoLongerOffered` in `cmd/director`.
  Each builds a scenario, finds the scenario no longer produces the condition it is about, and
  skips. They are declared with reasons beginning `BROKEN:`, which the gate counts and prints
  apart on every run — so the gate passes and keeps saying they are there. Deciding whether the
  behaviour still exists to be gated is a judgement about perception, not a test repair.
- **30 tolerated skips**, every one declared by name with a reason — 27 in the engine module, 3
  in `plugins/overlay`, three of them the parked findings above.
- **Studio has no whole-desktop recording control.** `marco learn --whole-desktop` is the only
  way to ask for the old behaviour, and the widget's unattended `LearnAuto` refuses to save at
  all because nobody has looked.
- **`plugins/voice` does not build without the Vosk C headers.** `vosk_api.h` is not vendored,
  and the module fails the same way in this working tree as in a clone, so it is an environment
  requirement (`setup.ps1 -Voice`) rather than a defect in the history. Nothing in the alpha
  bundle uses it.
- **Seven `go vet` notes pre-date these commits**: six `possible misuse of unsafe.Pointer` in
  `internal/recorder`, `internal/platform/navsource` and `internal/oshost`, all of them
  hook-callback `lParam` casts, and one `append with no values` at
  `cmd/director/padwiring.go:110`. The three commits here add no new flagged construct.
- **There is still no `-race` job in CI.** Both the data race fixed in the multi-active
  scheduler work and the deadlock fixed in `67e20a0` were invisible to the default suite, and
  the detector is run by hand on a machine that had to have gcc installed for it.
- **The eight orphaned `marco do` processes have a detector but not a cause.** The gate reports
  a survivor as a finding and can end one whose start it can show; why eight accumulated in the
  first place is unestablished.
- **Narration still writes outside `orchestrator.Keep`.** `LearnVoice` and
  `marco learn --narrate` go through the retained `saveTaught`/`chooseScope`, so the one set of
  rules for keeping a recording does not yet cover them.
- **The semantic interpreter stays in shadow mode.** No current model is wired into execution.

## What this handoff does not claim

- Not that the product has been run end to end by a person on this tree. It has not.
- Not that stage 1 passing proves the earlier 8-then-6 findings were environmental rather than a
  regression. What was established is narrower, and was traced rather than inferred: every one
  of those messages came from one fixture that photographed the live foreground window, and with
  the window identity STATED all eleven affected tests fail with a sentence about the
  application — *a workflow may only act on the application it was shown in* — while with drawn
  pixels and a live identity all eleven pass with a game animating. The capture needed no seam;
  the window platform did.
- Not that the three `-tags desktop` tests ran in the gate. They did not. The gate reports them
  NOT RUN with the command that runs them, which is the point of moving them.
- Not that every claim in this page was true when it was first written. The build rows said
  "builds" on the strength of the working tree, and were corrected after a worktree said
  otherwise. The reason the page now distinguishes the two is that the distinction cost three
  defects.
- Not that a clean checkout passing means the bundle is reproducible. It means the COMMITTED
  SOURCE builds and gates itself. Packing, installing, the desktop-tagged tests and every desktop
  scenario are stages 3 to 6 and are still held; the clean-checkout result says in its own output
  that it did not run them.
- Not that `plugins/voice` builds anywhere. It needs the Vosk C headers, which are not vendored,
  and it fails identically in the working tree and in a clone. The clean-checkout run does not
  build it, because `cmd/gate` does not.
