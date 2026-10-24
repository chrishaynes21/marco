---
type: reference
status: current
date: 2026-10-24
source_paths:
  - internal/recorder
  - internal/orchestrator
  - internal/codegen
  - internal/oshost
  - internal/routes
  - internal/marcopaths
  - internal/outcome
  - internal/director/runref
  - cmd/director/runrecord.go
  - cmd/marco
  - plugins/studio
---

# Coordinate-first Studio MVP — assessment and estimate, at `3d9eb60`

> **Status: revision 2 is APPROVED as the design and estimate** (2026-10-24). The first milestone
> is *"a routine you can report on"* — R1 to R3, **11 to 18 focused engineering hours including the
> prototype allowance** — with the boundaries restated as: `marco do` owns reservation, citation
> and finalisation; records distinguish run kinds and preserve their outcome vocabularies; the
> source digest identifies the bytes actually executed; interrupted attempts stay unfinished and
> absent verification evidence stays absent; and Studio can inspect and export each attempt without
> a terminal. Explicit mode, replacement protection and readiness are the next 8 to 12 hours and
> are required before the broader MVP is called repeatable. Chord cleanup stays separately scoped
> at 6 to 9 hours, covering both partial-delivery cleanup and cancellation before the tap.
> **Implementation awaits the Owner's decision; nothing here is authorized by the approval.**

**Revision 2**, after review. Four things changed: the record's MEANING is settled before any
extraction (§1a, two vocabularies that do not convert); the EXECUTOR owns the reference and only
the shared machinery is extracted (R1, R2); the claim that chord cancellation was already correct
was WRONG and is corrected against source (§4); and the estimates carry hours as well as
increments. Revision 1 said the extraction was bigger and the cancellation path was sound; both
were wrong, and where this document now contradicts it, this is the version that read the code.

**Design and estimation only.** Nothing here is implemented, nothing is migrated, and no saved
routine is rewritten. Written against source read at `3d9eb60`; every claim about current
behaviour cites the file it was read in, and the places where I am guessing say so.

The request this answers asked first for a reconciliation, because the brief that prompted it
commissioned a bridge that already exists.

## 1. Source audit

### Retained, and not to be re-estimated as missing

| Component | What it already does |
|---|---|
| `internal/recorder` | The widget recorder. Low-level hooks, both edges of a click attributed to the process that owned the foreground at that instant, and the press's geometry read in the hook — basis, outer size, DPI, owning program. |
| `internal/orchestrator` draft path | `Deps.Begin` → `Capture.Finish` → owner exclusion → `simplify` → an unsaved `Draft`. This is what Studio records through, in its own process. |
| `internal/simplify`, `internal/codegen` | The production simplifier and generator. `codegen.Route` writes legal Marco; a recording with a checked press is generated as a chain. |
| `internal/routes`, `internal/marcopaths` | The library: one absolute place, `<app>/<scope>/<slug>` identity, the `.marco` and the `.rec.json` demonstration beside it (`SaveRecording`/`LoadRecording`). |
| `internal/driver`, `internal/oshost` | Compile and perform. The guarded press resolver, and `Unperformed` — the run's own account of what did not happen. |
| `internal/outcome` | The seven words, and `ResultPrefix`/`RoutePrefix` as a wire protocol both ends import. |
| `internal/stopsignal` | One stop, raised by the gesture, by `director stop`, and by Studio's button. |
| `plugins/studio` | Record → Stop → review → Save; *Your routines* with exact identity; Play through `marco do`; one run slot shared by every kind of run; the trial journal; and `Inspect`/`Report`. |

The owner has run the whole of that live (E2E.md §63), including a mouse-driven mid-recording
visit, playback into a moved window, refusal after a resize, and Stop → cancelled.

### The genuinely missing bridge: one, and it is narrow

**A coordinate routine's run produces no durable record.** The canonical account is
`cmd/director`'s `RunRecord`, written by `saveRun` under a reference from
`internal/director/runref`, and it is written only by the supervised performer. `marco do` on a
saved routine announces `[result] <word>` and nothing else, so Studio's `Running.Reference` stays
empty and the window says *a routine — no run record*.

`Inspect` and `Report` already exist and work (`plugins/studio/history.go`): `Report` gathers a
run's record, what it said as it ran, and writes a file somebody can attach. They are unreachable
for exactly the routines this MVP is about. The reader's own `PastRun` even carries a
`json:"routine"` field that nothing writes.

So the remaining product work is not a recorder bridge. It is an account.

**And less of the account machinery is missing than the first version of this assessment said.**
`internal/director/runref` is already an importable package and already carries the citation
scheme: `Reference`, `Parse` (which Studio already uses for Acts), `Cite`, `Ended`, and the seven
words including **`started`**. In `cmd/director/runrecord.go`, `reserveRun` creates the record file
with `O_CREATE|O_EXCL` and retries on collision, writes a `started` record into it at once saying
*"reserved; this attempt had not finished when the record was last written"*, and `saveRun`
finalizes with **no** `O_CREATE` — so it can only write where a reservation already is.

Exclusive reservation, an interruption leaving a started record, and finalization that cannot
overwrite another attempt are therefore solved already, in that machinery. The work is to make the
routine executor use it, not to invent it.

### The two paths, and why they stay apart

| | Coordinate routine | Saved visual demonstration (Act) |
|---|---|---|
| Recorded by | `internal/recorder`, through the orchestrator draft path | director capture / learn |
| Saved as | generated `.marco`, with `.rec.json` beside it for regeneration | a composition through `demonstrated.Compose`, with bindings |
| Chosen in Studio | *Your routines*, by `<app>/<scope>/<slug>` | *Acts* |
| Performed by | `marco do --play` → driver + `oshost` | `director`'s supervised runner |
| Durable account | **none — the gap above** | `RunRecord` + reference |
| What its answer means | every press happened, or it stopped and said which term disagreed | verification counts: verified, contradicted, unconfirmed |

**They do not need a common storage format, and the reason is in that last row.** A routine's
answer is about delivery and geometry; an Act's is about evidence. One format would have to drop
one of those meanings or carry both and let each path ignore half. What they share is a *place*, a
*citation scheme* and a *reservation discipline* — not a record shape.

## 1a. What the record means, before anything is extracted

The two paths do not share a vocabulary, and the words are not translatable:

| | Words |
|---|---|
| `internal/director/runref` (supervised) | `started`, `verified`, `unconfirmed`, `contradicted`, `refused`, `cancelled`, `failed` |
| `internal/outcome` (a routine, over the wire) | `performed`, `clarify`, `refused`, `unavailable`, `cancelled`, `unverified`, `failed` |

Three words overlap — `refused`, `cancelled`, `failed`. The rest do not, and the dangerous
direction is obvious: nothing may turn `performed` into `verified`. A routine's run presses and
looks at nothing afterwards; `verified` is a claim about evidence it never gathered. Equally,
`unverified` is not `unconfirmed`: the first says nobody looked, the second says somebody looked
and could not tell.

So the record carries an explicit **run kind**, each path keeps its own vocabulary under its own
kind, and **no conversion exists in either direction**. A reader that does not know a kind lists it
and shows the whole of it, which `plugins/studio/history.go` already does by reading JSON names
rather than a writer's type.

**Absent verification evidence stays absent.** A routine's record omits the verification counts
rather than writing three zeroes — three zeroes read as a run that verified nothing, which is a
claim, where omission is the truth: the question was never asked.

### The minimum a routine's record identifies

- the **exact route** — app, scope and slug, the identity Studio already selects by
- the **source digest** (`routes.DigestOf`), so the record cites the source that actually ran
- the **executing build** (`internal/buildid`), which is the field that would have named the stale
  recorder a day of the owner's recordings went into
- the **library** it was read from (`internal/marcopaths`)
- the **stated mode and contract** — coordinate, and guarded or legacy, read from the saved source
  the way `cmd/marco/panicstop.go` already reads it; **`unstated` when the source does not say**,
  which keeps older routines truthfully labelled without pulling generation-contract migration
  into this slice
- **timestamps**, started and ended
- the **outcome word and the reason sentence**, the run's own, unconverted

### Counting input honestly

Three different things, and only the first is knowable today:

- **attempted** — how many input sends this run asked the machine for. Countable; `oshost` already
  wraps every send in `delivered`.
- **confirmed delivery** — not knowable. `SendInput` returning success says the queue took it, not
  that the application acted on it. Nothing in this path looks at the screen, so this field is not
  written rather than being filled in with the attempted count.
- **unknown partial delivery** — a real state, and the chord defect in §4 is exactly it: some
  modifiers delivered, the tap refused, the releases unreported. Where a run ends in that state the
  record should say so rather than reporting a clean failure.

## 2. Capability strategy

### What `MARCO_CV` / `MARCO_ANCHORS` actually do to a recording

`internal/recorder/recorder_windows.go` reads them into **package-level variables at process
start**: `var captureAnchors = anchorsEnabled()` and `var cvKitchenSink = cvMode() == "max"`. Both
default off. Three consequences, all current behaviour:

1. **Studio inherits the environment it was launched with, for the life of the window.** Double
   clicked from Explorer it gets the user's environment; started from a shell that exported
   `MARCO_CV=on` it records differently, and nothing in the window says which.
2. **With anchors on, a recording can become unsaveable — per click, depending on what was on
   screen.** A press captures a template only when the cropped region is distinctive
   (`screen.Distinctive`), or always under `MARCO_CV=max`. A press carrying a template is
   `cannotPropagate` → *"a press found by matching a picture"* → `Unpropagatable` refuses the
   whole guarded recording at Save. So the same demonstration saves or is refused depending on an
   inherited setting and the pixels under the pointer.
3. **The setting reaches replay too.** Studio's `doCommand` hands the child `os.Environ()` plus
   `MARCO_HOME`/`MARCO_ROUTES`, so the child's anchor resolution inherits whatever Studio inherited.

This is the difference between an engineering switch, which it is, and the explicit saved
capability contract the request asks for.

### The smallest honest contract

- **Studio asks for coordinate mode explicitly rather than inheriting it.** `recorder.New()` takes
  no options today and `Recorder` is `Start`/`Stop`/`Events`, so this is an additive option —
  either `NewWith(Options)` or options on `Start`. The package variables stay as the default for
  every other caller, so the overlay and director are unchanged.
- **The mode is written down**: in the draft, in the saved route's provenance, and in the run
  record (R2). A routine then says what it was recorded under instead of being guessed at from an
  environment nobody can see.
- **Studio states it in the window**, beside Record, in one line. An inherited `MARCO_CV=on` with
  coordinate mode selected means the recorder is asked for coordinate and the line says so.
- **Nothing existing is reinterpreted.** An anchored route keeps its anchors and its resolver;
  `MARCO_CV` keeps its meaning for those; no saved file is rewritten and no visual artifact is
  touched. Fusion, accessibility and vision stay preserved and opt-in, exactly as
  [[Fusion]] and [[Vision]] describe them.

## 3. Remaining lift for the vertical slice

Two units, because one of them describes packaging rather than effort.

**An increment** is a self-contained change with its tests, its mutation gate and its
documentation — the unit this project lands. **Hours** are focused engineering hours for one
developer with the Windows machine available, calibrated on the three increments landed on
2026-10-24 (the pairing fix, the key-and-press tests, and key propagation across three packages),
each of which ran about 1.5 to 3 hours including its gate and its documentation. Prototype
allowances are stated separately and are allowances, not commitments. The owner's own acceptance
runs are not counted in either figure.

### Completed (listed so it is not re-estimated)

Recording, review and Save; the library and exact identity; select → Play; moved-window replay;
resize refusal carrying the term; Stop → cancelled; discrete-key failure propagation; the
`Inspect`/`Report` surface itself; the trial journal; the outcome wire; one run slot.

### Remaining

| | Work | Increments | Hours |
|---|---|---|---|
| **R1** | **Extract only the shared machinery.** The place (`<home>/runs`), the citation scheme (already in `runref`), the exclusive reservation, and the finalise-without-create guard — parameterised by the record shape so each kind writes its own. Director's `RunRecord` stays where it is and becomes a caller; **the whole Director record implementation does not move**, because that is not a product requirement. `plugins/studio/history.go` keeps reading by JSON name, which is what lets two kinds share one reader. | 1 | 3–5 |
| **R2** | **`marco do` reserves, cites and finalises its own record.** The EXECUTOR owns the reference: it reserves before the first input, prints the citation with `runref.Cite` (Studio already parses that line for Acts), and finalises with the routine record — kind, identity, digest, build, library, mode and contract, timestamps, outcome and reason, attempted sends. Studio reads the citation and nothing more. | 1–2 | 4–7 |
| **R3** | **Report a problem, for a routine.** `Report` exists; it needs to work from a routine's reference and to carry mode, library, build and the reason. Mostly wiring and words. | 0.5–1 | 2–3 |
| **R4** | **Coordinate mode as an explicit capability** (§2). Additive recorder option, the mode recorded, the window saying it. The stub backend must keep compiling. | 1–2 | 4–6 |
| **R5** | **Replacement protection, then repeat.** `Registry.Save` is `os.WriteFile`, so re-recording a name silently destroys the routine saved under it. The review must say *this replaces the routine you saved at …* and the person must agree. Only then the "record again" affordance that keeps the name and scope. | 0.5–1 | 3–4 |
| **R6** | **Truthful readiness.** Before Record: the library it will write to, the mode, and whether the `marco.exe` beside Studio exists. Small, and it is the line that would have saved the owner's lost trial. | 0.5 | 1–2 |

**First milestone (R1 + R2 + R3): 2.5 to 4 increments, 9 to 15 hours, plus a 2 to 3 hour prototype
allowance — about 1.5 to 2.5 focused days.**

**Before the word "repeatable" is used, add R4, R5 and R6: a further 8 to 12 hours, about 1 to 1.5
days.** Explicit mode and replacement protection are part of that threshold rather than after it;
a convenient "Record again" button in front of a silent overwrite would be a worse product than
no button.

### Assumptions behind the hours

- One developer, uninterrupted, with this Windows machine available for the live checks.
- The record is a subset of what already exists per kind — no new store, no second account, no
  transmission anywhere. The bug report stays a file the person sends.
- One run slot stays; nothing here introduces concurrency.
- No migration of saved routines, and no change to the Act path's record or vocabulary.

### Prototype allowances, kept out of the estimate above

- **The three interruption cases** (2–3h, R2). Source says `reserveRun`'s `O_CREATE|O_EXCL` plus
  retries, its immediate `started` record, and `saveRun`'s missing `O_CREATE` already give
  exclusive reservation, a started record left by an interruption, and a finalisation that cannot
  touch another attempt. The prototype is to CONFIRM that under a killed child and two runs in
  quick succession, not to discover it — and to establish that the routine path inherits all three
  rather than reimplementing any.
- **How the recorder option is threaded** (R4) — a second constructor or an argument on `Start`.
  Cheap either way; the cost is which callers have to change.

## 4. Chords: assessment, and a separate estimate

### The representation is already sufficient

`simplify` emits a chord as `StepKey{Key: "ctrl+shift+esc", Count: 1}`, and
`internal/oshost/backend_windows.go` presses the modifiers, lets them settle (`chordHold`), taps
the last key, holds it, and releases everything **in reverse order, even when the tap was
rejected**. A chord is already one intentional action with explicit press and release. **No new IR
kind is needed**, and a new one would be a second representation to keep in step.

**And a chord already propagates failure**, as of `5e392ea`: the rule is on `StepKey` and does not
look at the string, so a generated guarded routine already stops at a chord the machine refused.
That is not remaining work.

### What the audit establishes as missing — one defect, one silence, one impossibility

- **A DEFECT: partial delivery leaves a modifier down.** The modifier loop returns on the first
  rejected `sendKeyDown` *without releasing the modifiers already pressed*. `ctrl+shift+esc` with
  shift rejected returns with Ctrl held, and nothing tracks it: `heldKeys` records only what
  `doKeyDown` pressed, so the end-of-route unwind cannot clean it up. The fix shape is small —
  register each modifier in `heldKeys` as it goes down and clear it on release, so the existing
  unwind covers a partly delivered chord.
- **A SILENCE: a rejected release says nothing.** The modifiers' `sendKeyUp` results are
  discarded. A release the machine refused leaves that key down while the run reports only the
  tap's failure. A release that did not happen belongs in the account like everything else.
- **A SECOND DEFECT: cancellation does not stop the tap.** The first version of this assessment
  said cancellation was already correct. **That was wrong, and the correction stands against the
  source**: `sleepCtx(ctx, chordHold)` returns the instant the context is cancelled, and execution
  then falls straight into `sendKeyDown(last…)` — so a chord cancelled while its modifiers are
  settling still injects the tap. The releases afterwards and the `ctx.Err()` at the end mean
  cleanup is ATTEMPTED and the run reports cancelled, which is what made it look right; input went
  out after the person asked for it to stop, which is what matters. A cancelled chord must stop
  BEFORE the tap, release the modifiers it has already delivered, and report any release that
  failed.
- **AN IMPOSSIBILITY: forced termination cannot be cleaned up, and must not be claimed.** A killed
  process releases nothing, so a chord routine killed mid-press can leave Ctrl held until the
  person presses it themselves. Cooperative shutdown and forced termination are different claims:
  normal unwinding establishes only the first, and a watchdog or a release-on-next-start would be a
  separate decision. Nothing here claims the second.

**And failed cleanup must not erase the original error.** Once release failures are reported they
become a second thing that did not happen, and the tap's rejection is still the reason the routine
stopped. The account holds a list for exactly this reason; the first sentence must stay the first
sentence.

### Chord estimate

| Work | Increments | Hours |
|---|---|---|
| Design note: the press/release semantics of one chord, and what each failure state means | — | 1–2 |
| Partial-delivery cleanup through `heldKeys`, stopping before the tap when cancelled, and release failures in the account without displacing the original error — with stuck-key tests | 1 | 4–6 |
| Studio naming a chord in the review in words a person reads | 0.5 | 1 |

**Chord total: 6 to 9 hours, about one focused day, separately scoped.** It is a concrete defect
in an existing capability. It is not grounds for a new input representation and not grounds for
touching Fusion.

Design first, as directed. The question to settle before any of it: whether a chord whose
modifiers were delivered and whose tap was refused should report the tap's failure alone, or say
that modifiers were pressed and released as well — the person's screen may have changed either
way, and the record's "unknown partial delivery" state (§1a) is where that answer goes.

## 5. Parked follow-ups, estimated separately

- **Typed text: 2–3 increments, 8–14 hours.** Not one `do` to wrap. `splitSecrets` can emit several
  sentences from one step — literal runs, declared arguments, and secrets resolved at run time — so
  propagation means choosing the unit and deciding what a partly typed line means. `doType` records
  nothing in the account either. And a failure reason must not quote the text, because the text can
  be a secret.
- **Holds: 1–2 increments, 5–8 hours.** `KeyDown`/`KeyUp` bracket the steps between them, so
  propagation means deciding what the release owes a press that did not happen. `heldKeys` already
  exists and the chord fix above shares it, so some of this is paid for by that work.
- **Drags and non-left clicks: stay excluded, and source supports it.** The owner's working
  routine is left clicks and Escape. A drag is refused at Save today, and a drag resolves no
  offset at all — it carries no window and is checked against none — so a guarded drag is a
  contract question before it is an implementation one. A non-left press needs one guarded
  operation carrying a button beside the point, which is a permanent word in the language for a
  case no recording has needed yet. See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].
- **Generation-contract versioning: separately estimated, with one interaction to flag.** R2
  writes a mode into a run record, and a record for a routine generated *before* the mode existed
  must not claim one. The cheapest honest answer is to write the mode only when the saved source
  says it, and otherwise *unstated* — which keeps R2 from mislabelling older routines without
  pulling the versioning work forward.

## 6. The smallest first milestone

**"A routine you can report on" — R1, R2 and R3 only.**

Record → Stop → review → Save → select → Play already works and has been shown live. This
milestone changes nothing about that flow and makes a run *citable*: one durable record per run,
and a file the owner can send when something goes wrong. It is the smallest step that turns a
working flow into a diagnosable one, which is what a repeatable MVP needs that this does not yet
have.

Then R4, R5 and R6 — explicit mode, replacement protection and truthful readiness — which together
are the threshold for calling the broader MVP **repeatable**. Not after it: re-recording is the
action a repeatable MVP encourages, and encouraging it in front of a silent overwrite would make
the product worse. A "Record again" button ships with replacement protection or it does not ship.

### Acceptance, Studio only, no terminal

1. Launch `studio.exe` from Explorer, freshly built, with Studio closed before the build.
2. The library line names the directory, and the routines saved earlier are listed.
3. Record → review the ordered actions → Save.
4. Select it and Play. Then Play it again: twice in a row, both successful. **Both runs produce
   DISTINCT references, and each report matches its own run's outcome** — one reference for two
   runs, or a report describing the other attempt, is the reservation discipline not holding.
5. Move the application's window and Play: it presses where it was shown.
6. Resize the window and Play: it refuses, pressing nothing, and the banner names the term that
   disagreed.
7. Stop mid-run: `cancelled`, not a failure.
8. **Report a problem** produces a file naming the routine's identity, the running build, the
   mode, the outcome word, the reason, and the library — without a terminal.
9. **A killed child leaves an UNFINISHED attempt** — never a completed one and never a successful
   one. Kill the `marco.exe` Studio spawned mid-run: the record stays `started` with the sentence
   the reservation wrote, the window does not report that the routine performed, and the report
   for that reference says the attempt did not finish.
10. Nothing in the window reads *"playback completed"* as *"the game did what you wanted"*. The
    sentence that ships today says the presses happened and that nothing checked the outcome, and
    that distinction is load-bearing. No routine's record ever says `verified`.
11. Existing Acts remain listed and performable through their own path, with their own vocabulary,
    unchanged.

### Compatibility risks

- **Two kinds of record in one place** (R1, R2). Mitigated by an explicit kind, no conversion
  between vocabularies, and a reader that already tolerates fields it does not know. The risk to
  watch is not storage, it is MEANING: a routine's run must never be readable as `verified`.
- **The explicit mode must not change what other callers record** (R4). The package variables stay
  the default; only Studio asks.
- **`Registry.Save` overwrites silently** (R5). This is the one place existing user data can be
  lost today, and re-recording is exactly the action a "repeatable" MVP encourages.
- **Inherited `MARCO_CV`** (§2). Until R4, a Studio launched from a CV-enabled shell can refuse to
  save a perfectly good recording, and the refusal names a picture the person never asked for.

### What stays parked

Typed text, holds, drags, non-left clicks, generation-contract versioning, and game vision —
which is not production-ready and is not claimed to be anywhere in this assessment.

## Related

- [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] — the contract the coordinate path runs
  under, and its known gaps.
- [[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]] — the library and
  identity this milestone assumes.
- [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] — the recorder's own window, and the
  mid-recording visit that the live trial exercised.
- [[ADR-248-replaying-a-demonstrated-layout]] — the fixed-layout licence a coordinate routine runs
  under.
- [[Demonstrations]] — the subsystem note for both paths.
