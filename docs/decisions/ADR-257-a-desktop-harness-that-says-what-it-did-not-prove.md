---
type: adr
status: accepted
date: 2026-10-25
decision: "ACCEPTED — STUDIO'S PRODUCT PATH IS EXERCISED BY AN UNATTENDED HARNESS THAT DRIVES THE REAL WINDOW ON A REAL DESKTOP, AND THE ONE THING IT CANNOT ESTABLISH IS PRINTED IN EVERY REPORT. Studio's flow was testable in two places and provable in neither: the browser tests render the real page with the Go bindings STUBBED, and the backend tests drive the real backend with the recorder and the desktop REPLACED. Between them sat the whole product path — page, binding, backend, library, generated Marco, child executor, native input, window guard, run record — and the only thing that had ever exercised it was a person working through E2E.md by hand, eighteen bullets at a time. `plugins/e2e` is that path, run by one command: a bundle of binaries built fresh into a directory of its own, an isolated store, `plugins/fixture` as the application, and the real `studio.exe` driven through UI AUTOMATION so that every press runs the page's own handler and therefore the real binding and the real backend. It asserts two accounts that are never crossed — the fixture's own log of what it RECEIVED, which Marco may never read, against the run record Marco wrote — and reports the FIRST place they disagree. THE LIMIT IS STRUCTURAL AND IS STATED RATHER THAN WORKED AROUND: Marco's recorder discards injected input in both hooks, on purpose, because attributing Marco's own presses to a person would let a replay be learned as a demonstration. So no unattended run can establish that Windows captured a physical gesture, and no door was added to pretend otherwise. Routines are SEEDED through the production generator and labelled seeded, never recorded; the report carries three lines — passed desktop scenarios, passed recording-integration scenarios, and `physical recording capture: NOT EXERCISED` — and the third is printed on a green run in the same words, because a limit mentioned only beside a failure is a limit nobody reads. TWO FALSE-GREEN PATHS WERE FOUND IN REVIEW AND CLOSED. A partially skipped run exited 0, so the scenario most able to decline — the resized-window refusal — could go unrun while the harness reported success; a skip is now an ending of its own, and no shape of partial run exits 0. And a run finishing established only that nothing more would be SENT, not that the fixture had processed what it already had: every absence now crosses a DRAIN BARRIER the fixture answers from inside its own message loop, demonstrated before the first scenario against a fixture holding a press it has not seen, with what the barrier cannot establish — the system's own delivery to that queue — stated rather than implied. THE BARRIER'S OWN SELF-CHECK TOOK THREE GOES TO AIM, and both corrections are recorded as corrections: a check that asks a barrier a question it can answer harmlessly, or reads its silence one pass before the answer is decided, passes on exactly the implementation it exists to reject — which both earlier versions did, measurably, in their usual words."
supersedes: []
affects:
  - interface
  - process
source_paths:
  - plugins/e2e
  - plugins/fixture/Fixture.cs
  - internal/acceptance/boundary_test.go
  - internal/orchestrator/draft.go
---

# A desktop harness that says what it did not prove

## The gap it closes

Studio's flow had two kinds of test and neither was the product.

| | what it holds | what it replaces |
|---|---|---|
| `plugins/studio/*page_test.go` | the real page, rendered in a real browser | the Go bindings, stubbed |
| `plugins/studio/*_test.go` | the real backend | the recorder, the desktop, the child |

Both are worth having and neither can fail for the reasons that matter most: a button not wired
to the binding it looks like it calls, a control enabled at the wrong moment, a surface a person
cannot reach, a coordinate that lands four pixels off, a child spawned from the wrong directory, a
guard that reads the window at the wrong instant. Every one of those has been a defect in this
product, and every one of them was found by a person at a keyboard.

So the regression work was the owner's, eighteen manual bullets at a time, after every change.

## What the harness is

One command. It builds `marco.exe`, `director.exe` and `studio.exe` — with the Wails tags — into
a directory of this run's own, copies `fixture.exe` and `uia.exe` beside them, makes a temporary
store, and then:

1. opens `plugins/fixture`, the deterministic application this repository already tests Marco
   against, with its two oracles pointed at the artifact directory;
2. **seeds** a routine into the library through the production generator;
3. launches the real Studio on that store and waits for the page's own callback;
4. presses Studio's actual controls through UI Automation;
5. compares what the application received against what Marco wrote down.

### Why UI Automation and not a debug port

Chrome DevTools over a port was the obvious way to press an HTML control from outside, and it is
not available: `go-webview2` clears `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` in a function called
`preventEnvAndRegistryOverrides`, and Wails exposes no option that would pass one. Patching
production to leave a debugging port open in everybody's build is not a thing a harness may ask
for.

What is available is better. Chromium publishes the page to Windows' UI Automation, Marco already
has a bridge host that reads it, and that bridge is a different process reached through the
operating system. So the element address is the DOM id — Chromium maps `id="routinePlayBtn"` to
the AutomationId — pressing is `Invoke` on the control's own pattern, which runs the page's real
handler, and reading is the operating system's account of the window rather than the backend being
asked what it would have rendered.

### Why the routines are seeded, and what that costs

`Deps.DraftOf` makes a draft from a demonstration this process did not capture — events, then
`simplify`, then `codegen`, then `routes.Registry`, the same four steps in the same order through
the same functions with the same options. `Resave` uses it too, so there is one of it rather than
a harness with its own idea of what a draft is.

What the harness supplies is the events, which is the one thing a person would otherwise have
supplied with their hands. They are MEASURED rather than invented: the press point comes from the
fixture's own account of where its control is, and the window on the press comes from
`winctx.WindowShape` — the identical call the recorder makes at the identical moment. A harness
that computed the window rectangle itself would have been comparing its own arithmetic with
Marco's, and a one-pixel disagreement about a border would have looked like a replay defect.

It costs the recording half. A seeded routine says nothing about whether the hooks captured
anything, which is why that is a line in the report rather than a silence.

## The limit, stated once and printed always

Marco's recorder tests `LLKHF_INJECTED` and `LLMHF_INJECTED` in both hooks and discards what it
finds. That is not an oversight to work around: attributing Marco's own presses to a person would
let a replay be learned as a demonstration, and the measurement that established it is in
`internal/acceptance`'s own package note — a separate Windows desktop refuses `SendInput` outright,
and the standing instruction is *do not try to make injected input attributable to get an
automated E2E*.

Three options were put to the Architect: operator-assisted recording, a USB HID device presenting
itself as a real keyboard and mouse, or a gated door in the recorder. The answer was **none of
them for now**: build the unattended harness, put native recording explicitly outside its first
coverage boundary, and say so. A VM with host-controlled virtual input may turn out to provide
non-injected guest input and is parked as a later investigation, to be measured before it is
claimed.

So every report ends with the same sentence, on a green run as much as a red one:

> physical recording capture: NOT EXERCISED — Marco's hooks discard injected input by design, so
> no automated run can establish that Windows captured a physical gesture.

## Two accounts, never crossed

	the fixture says  — what it RECEIVED: the press, the control, the coordinate, the window
	Marco says        — what it BELIEVES it did: the run record it wrote

	both agree               → the result, and the only one called passed
	Marco failed and said so → an honest failure, and reportable as one
	Marco believed it won    → the dangerous case, and the reason there are two

The fixture's event log is new and is the stronger of its two oracles: a press that misses Save by
four pixels and lands on the panel behind it leaves the state file exactly as a press that never
happened does, and in the log it is a `left_down` on the wrong control with the coordinate that
produced it. `internal/acceptance`'s boundary test now scans for both oracle names across the
tree, so no production package may learn either.

## Nothing arrived, versus nothing has arrived yet

Four scenarios assert an absence: a resized window receives nothing, a stopped run sends no second
press, a killed runner sends no further press, two runs produce exactly two presses and not a
third. Every one of them reads the fixture's log after the thing sending input has stopped.

The first version of this took the run record's `ended` as the moment that made such a read sound,
and it is only half of what that read needs. `ended` establishes that nothing more will be SENT.
The fixture's log is written when its message loop DISPATCHES a message — so a fixture that is
behind, for any of the ordinary reasons an application is, can have a press sitting in its queue at
the moment the harness looks. The harness would then report that the guard refused and the
application received nothing, and be wrong in the one direction that matters: a press that
happened, reported as a press that did not. **A longer wait is not the fix.** Waiting two seconds
and finding silence establishes that nothing has arrived in two seconds.

So the fixture answers for itself. The harness writes a token, and the fixture writes it back once
its pump has gone idle *after* reading the request — an idle meaning the queue held nothing at all,
so whatever was in it has been dispatched, and dispatched is when the witness writes a press down.
It is an idle rather than an acknowledged message on purpose: a posted message is retrieved AHEAD
of queued input, so a sentinel would come back with the presses still waiting behind it, which is
worse than no barrier because it would look like proof.

The channel carries a token and nothing else. What the window received stays in the two oracles the
harness reads for itself; a synchronisation channel that reported facts would be one the harness
could be tempted to believe instead of looking.

**What it does not establish**, said here because the code says it too: input the system had
accepted but not yet handed to that queue. That handoff is done by a system thread and nothing in
either process orders against it. What bounds it is the producer — `marco` returns from sending
before it writes `ended` — and the residue is an interval inside the operating system whose end no
user-mode observation can see. The gap this closes is the one a harness can be slower than.

And it is demonstrated rather than assumed, before any scenario leans on it, against a fixture
that is genuinely behind: `--selftest` queues a press in the fixture's own message queue and stops
the loop until released. The harness reads the log there and must find nothing — **that read is the
false green, reproduced on purpose** — and reads again after the release, where the press must be.
A barrier nobody has watched hold is an assumption with a function name.

### And the self-check has to put the barrier where answering would be wrong

Those two reads are not enough, which is a correction to this record rather than a refinement of
it. The first version of the self-check asked for the barrier while the loop was stopped and
claimed that an implementation acknowledging on RECEIPT would be caught by it. It would not have
been. The fixture reached `Drain` only from its timer, which could not run until the holding loop
returned — and by then the pump had dispatched the staged press. An acknowledgement on receipt
would still have landed after the press was written down, and the check passed.

**Measured, not reasoned about.** With the idle requirement mutated out of the fixture, the
previous self-check reported `drain barrier: checked against a fixture holding a queued press` in
exactly its usual words and the run exited 0.

So the fixture's holding loop now calls `Drain` itself. The request is read with the press still
pending and nothing being dispatched, so no idle can occur, and a correct barrier is **silent**.

### And the silence has to be read after the decision, not after the question

That was still the wrong instant — the second correction to this record inside the same twenty
lines. `Drain` reads a request on one pass of a loop and compares idle counts on a LATER one, and
the harness waited for the request file to disappear, which is the first of those. So it could
assert silence and release the held loop before an implementation that acknowledges on receipt had
reached the line where it does. The reply would land after the pump dispatched the staged press,
and read as correct.

**Measured, not reasoned about, again.** The schedule was two arbitrary constants apart: the
fixture's holding loop called `Drain` every 25ms and the harness polls files every 100ms, so the
harness usually noticed the deletion only after the reply had already been written, and the
mutation was caught on 3 runs out of 3. Slow the fixture's reply evaluation to 400ms — the
deliberately delayed processing a check like this is supposed to survive — and the previous
self-check printed `drain barrier: checked against a fixture holding a queued press` in exactly its
usual words and exited 0, with an acknowledge-on-receipt barrier standing in front of it.

So the fixture now says when it has DECIDED, naming the question it decided about. `Drain` reports
whether it read a request, declined to answer for want of an idle, or answered; the holding loop
publishes that decision to `<state>.considered` with the token in it. The harness waits for a
decision about ITS token and only then asserts silence and releases, so what it is looking at has
already had its chance to answer. Under the same 400ms delay the new check ends the run at exit 3 —
*what it decided was to acknowledge it* — and removing the checkpoint from the fixture makes the
run NOT RUN at exit 2 rather than passing quietly, because the harness then has nothing to wait
for.

Recorded as two corrections rather than tidied into one design, because the general shape is worth
more than the fix: **a self-check has to be aimed at the instant the wrong implementation does the
wrong thing, and that instant is easy to miss by one pass of a loop.** Both earlier versions passed
for reasons unrelated to what they claimed.

A fixture that caught up before the harness could look is NOT RUN rather than passed: silence that
was not under pressure establishes nothing either way. Nothing here waits an interval — "it has
decided", "the question was taken" and "the fixture is still behind" are all files somebody else
wrote.

## A scenario that did not run is not a success

The exit code was 0 whenever something passed and nothing failed, which made a partial run and a
complete one the same answer. The scenario most able to decline is the resized-window refusal — the
window would not resize, the fixture would not start, the foreground was not ours — and it is also
the one that establishes the guard refuses at all. A run missing only that one exited 0, and
whatever read that code was told the product works by a harness that had not asked the question.

	0  every scenario selected ran, and passed
	1  a scenario failed — which outranks everything, because a finding outranks a gap
	2  something selected did not run: none of them, some of them, or a selection that
	   matched nothing, which establishes nothing while having failed nothing
	3  the harness itself could not get started

A partial run also says so in words, above the per-layer counts, because the counts are totalled by
a reader in their head and this sentence must not be. And `run.cmd` builds the harness and runs the
binary rather than using `go run`, which reports a child's code as a sentence on stderr and exits 1
itself — flattening 2 and 3 into "a scenario failed" for whatever is reading it.

## The child's own words, kept by the window that gathers them

The harness kept every word UI Automation could read off the window, which is not the child's
transcript: that lives in a `<pre>` behind a collapsed `<details>`, correctly absent from the
accessibility tree, because a thing nobody can see is a thing no assistive technology should claim
is there. So the one artifact saying what `marco` actually said as it ran was the one missing.

The product already gathers it. **Report a problem** writes the run's record, what the run said as
it ran, and the routine as it stands now into one file somebody can attach — and sends it nowhere,
which is the whole design of it. So the spine presses that button, and the harness keeps what the
product produced: a product path with no automated coverage at all is now exercised, and the
transcript is retained because the window gathered it rather than because the harness scraped it.

Two channels again: the window says where the file went, and the harness reads the file from the
place the window named. And an unavailable transcript is LABELLED rather than missing — `Report`
writes the section either way, with a sentence saying the window keeps only what the run in front
of it said and that the record is the durable account. The harness asserts the section exists and
reports which of the two it got. A report that silently omitted it would be the finding.

## What it will not do

- It never renames, moves or rebuilds a binary in the tree, so it cannot be the reason a
  binary somebody else was using changed.
- It never reads or writes the owner's store or library. The store is temporary and the
  library is inside it, resolved by `marcopaths` so that the harness, Studio and the `marco`
  Studio spawns all answer the question the same way.
- It refuses rather than ending somebody else's fixture, and does not refuse at all over
  somebody else's Studio: every window it reads or presses is addressed by the process id of
  the child it started.
- **It sends no input.** The only presses in a run are Marco's own. Pressing a control is UI
  Automation's `Invoke`; moving a window is `SetWindowPos`, which is furniture. There is no
  path through the module by which input exists.
- It stops rather than pressing when the foreground is not a window it owns.
- An unavailable desktop is NOT RUN with the reason, exit code 2, and never green.

## A card you press is a control, and it was not one

The routine cards were bare `div`s with an `onclick`: no name, no role, no keyboard path, and
nothing for assistive technology to announce or activate. A person who cannot use a mouse could
record a routine in that window and then be unable to select it.

They now carry `role="button"`, a tab stop, an Enter/Space handler, and an id built from the
route's whole identity — which is unique by construction, since `settings` saved for two
applications under two scopes is two routines and two ids. The accessibility fix is the reason;
that it also gives the harness a stable address for exactly one card is the consequence.

## What it found on the way in

Two observations, neither of them changed here.

**A guarded press refused for the right reason is recorded as `failed`.** The resized-window
scenario establishes that nothing at all was sent — the strongest form of that claim, because the
state file cannot tell a refusal from a press that missed — and the run record says `failed`.
`activityOutcome` writes the distinction down for the supervised side in so many words: *a door
saying no is not a thing going wrong, and the whole reason the vocabulary separates `refused` from
`failed` is so a person can tell them apart.* A window that changed size IS a door saying no, and
what the person is told is that their routine *stopped at something that did not happen*. The
scenario therefore accepts either word for "nothing was pressed" and reports the one it got;
settling which word is right is a product decision about the run vocabulary, not something a
harness may decide by asserting.

**The stale-binary trap is live on the owner's machine.** With this run's own `marco.exe` moved
aside, the readiness panel answered with its third case rather than its second: *there is no
marco.exe beside this window. `D:\Macros\marco\marco.exe` is on PATH, so Play would run whichever
marco.exe is on PATH, which may be a different build of a different age.* The repo root is on that
machine's PATH, so the one readiness case that never fails loudly is not hypothetical there.

## Enforced by

Nine scenarios, eight desktop and one recording, in about 36 seconds.

- `plugins/e2e` `play/seeded routine presses the application` — **the spine, and the only scenario
  that asserts all three channels.** A seeded routine chosen by identity and played from Studio's
  own window, with the press asserted on the fixture's control by name AND inside its rectangle,
  exactly once; the application's own count incremented by one; the run record agreeing about the
  route, the contract, the stated mode, the build and the number of input actions asked for; and
  the WINDOW's own sentence, read back through UI Automation, naming the routine, saying it
  performed, and still saying that nothing checked what the application did. It also presses
  **Report a problem** and keeps what the window wrote, which is how the child's own transcript is
  retained — the one account the rendered page does not hold.
- `plugins/e2e` `window/moved window still gets the press` and
  `window/resized window is refused with nothing sent` — **the pair, and it has to be a pair.** A
  harness that showed a press surviving a move and never checked the refusal would be evidence
  that the guard is permissive, presented as evidence that it works. The refusal is asserted on
  what the application did NOT receive, and on two facts rather than an interval somebody chose:
  the run record's own `ended`, after which nothing more can be sent, and the fixture's own word
  that its message loop had run out of work, without which the count is a lower bound.
- `plugins/e2e` `runs/two runs are two records` — two references, distinct. The failure this is
  about is not a crash: one run's account quietly replacing another's leaves a history short by one
  and a reference that resolves to the wrong attempt.
- `plugins/e2e` `runs/stop ends a run as cancelled` — a routine that presses twice eight seconds
  apart, so that there is an instant INSIDE the run; Stop pressed there; and then both halves —
  the record says cancelled, and the second press never arrives. A run that reported cancelled and
  kept pressing is the worse of the two and the state file cannot see it.
- `plugins/e2e` `runs/a killed runner leaves an unfinished record` — the `marco.exe` whose parent
  is this scenario's own Studio, killed mid-run, leaving `started` with no `ended`: the pair that
  means nobody can speak for it. **By parent and never by name**, because killing by name would
  end a run the owner had started.
- `plugins/e2e` `window/readiness notices a missing sibling` — the one readiness case that never
  fails loudly, posed by moving this run's own binary aside and put back whatever happens. It
  asserts the panel was quiet about it first, so the scenario cannot pass on a panel that says the
  same thing either way.
- `plugins/e2e` `window/a routine saved while open appears` — the library is read rather than
  remembered, and the one that was already there is still there.
- `plugins/e2e` `recording/draft, review, Save, again and replacement` — the second layer: the
  named scripted-recorder tests, run explicitly, where a name that matched nothing would be the
  quietest possible green and a skip is not a pass. Labelled apart from the desktop layer and
  never totalled with it.
- `plugins/e2e` `checkTheBarrier` — **the harness's own precondition, run before the first
  scenario and fatal if it cannot be met.** A press queued in a fixture whose loop is stopped must
  be invisible to the log; the fixture must DECIDE what to do about the drain request while that
  press is still pending and the decision must be **not to acknowledge**, which is the step that
  distinguishes a barrier from an echo; and the press must be there once the barrier answers after
  the release. Mutating the idle requirement out of the fixture ends the run at exit 3. Both
  earlier versions of this check passed that mutation in their usual words — the first because the
  request was read where answering it would have been harmless, the second because the silence was
  read one pass before the answer was decided. A fixture that caught up before being checked is NOT
  RUN. Not a scenario: it is a claim about the harness, and the layer totals are claims about
  Marco. Its sentence is printed in every report.
- `plugins/e2e` `TestTheSelfCheckLooksAfterTheFixtureHasDecided` — the ORDER inside
  `checkTheBarrier`, read off the syntax, because this has been got wrong twice in twenty lines and
  both times the code looked right. Nothing may read whether the fixture acknowledged the barrier,
  and above all nothing may release its held loop, before it has said which way it decided about
  that token.
- `plugins/e2e` `TestASkippedScenarioIsNotASuccess` — the whole table, including the two cases
  easiest to leave out: a selection that matched nothing, and a failure beside a skip where the
  failure has to win. Four shapes of partial run are asserted separately not to exit 0.
- `plugins/e2e` `TestEveryCountOfWhatArrivedIsTakenAfterTheBarrier` — read off the syntax, because
  the mistake is an omission and no reviewer catches those by reading. In a scenario, a call that
  counts what the fixture received must come after the barrier in the same function. `barrier.go`
  is excluded by name: it reads the log BEFORE the barrier deliberately.
- `plugins/e2e` `TestOnlyTheBarrierSelfCheckStagesAPress` — the fixture's `--selftest` mode is the
  one thing in this system that produces a press nobody sent, so it is confined to the self-check
  by a test rather than by everybody remembering. It also asserts the mode still exists on both
  sides, so a rename cannot make the confinement pass vacuously.
- `plugins/e2e` `TestTheHarnessSendsNoInput` — **the load-bearing test of the module.** No path
  through it names `SendInput`, `keybd_event`, `SetCursorPos`, Marco's input backend, the engine,
  or the recorder's constructor. Comments are stripped before the scan, so the rule can be
  explained beside the code without the explanation breaking it.
- `plugins/e2e` `TestTheReportAlwaysSaysPhysicalCaptureWasNotExercised` — the limit is printed on
  both paths and says why, not only that.
- `plugins/e2e` `TestTheHarnessBuildsStudioLikeTheScript` — the tags and the ldflags against
  `build.cmd`'s own. Drift here is quiet and total: the wrong tags produce a Studio that opens no
  window, every scenario fails, and the finding reads as being about Marco.
- `plugins/e2e` `TestTheCardIdIsBuiltTheSameWayThePageBuildsIt` — the harness's copy of the card's
  address against the page's expression.
- `plugins/e2e` `TestEveryScenarioIsLabelled` — a scenario with no layer would pass while being
  counted in neither total.
- `internal/acceptance` `TestMarcoCannotReadTheOracle` — now over both oracle names, with the two
  harnesses allowed as directories and everything else refused.
