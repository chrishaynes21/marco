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
one of those meanings or carry both and let each path ignore half. What they need is the boundary
they already have, plus one shared account *shape* for citing a run — which is R1 below.

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

The unit below is **one increment**: a self-contained change with its tests, its mutation gate and
its documentation — the unit this project lands, observed at roughly one per focused session.

### Completed (listed so it is not re-estimated)

Recording, review and Save; the library and exact identity; select → Play; moved-window replay;
resize refusal carrying the term; Stop → cancelled; discrete-key failure propagation; the
`Inspect`/`Report` surface itself; the trial journal; the outcome wire; one run slot.

### Remaining

| | Work | Increments |
|---|---|---|
| **R1** | **Lift the run record into an importable package.** `RunRecord` is in `cmd/director`'s main package, and `plugins/studio/history.go` already duplicates its field names through JSON because of that. Move the type plus save/read beside `runref` as `internal/director/runrecord`; `cmd/director` becomes a caller. One writer of one format, which is the whole point. | 1 |
| **R2** | **A routine's run writes a minimal record.** Reserve a reference in the routine path, announce it on the wire the way the Act path does, and write: routine identity, build, mode, contract (guarded or legacy), the outcome word, the reason, presses sent, started and ended, `fixed_layout: true`. Studio's `readRoutine` already keeps the child's lines, so it captures the reference there. | 1–2 |
| **R3** | **Report a problem, for a routine.** `Report` exists; it needs to work from a routine's reference and to carry mode, library, build and the refusal sentence. Mostly wiring and words. | 0.5–1 |
| **R4** | **Coordinate mode as an explicit capability** (§2). Additive recorder option, the mode recorded, the window saying it. The stub backend must keep compiling. | 1–2 |
| **R5** | **Easy repeat.** Two real items. `Registry.Save` is `os.WriteFile`, so re-recording a name silently destroys the routine saved under it — the review should say *this replaces the routine you saved at …*. And a "record again" affordance that keeps the name and scope. | 0.5–1 |
| **R6** | **Truthful readiness.** Before Record: the library it will write to, the mode, and whether the `marco.exe` beside Studio exists. Small, and it is the line that would have saved the owner's lost trial. | 0.5 |

**Total remaining: about 5 to 8 increments.** R1 + R2 + R3 are the spine, at 3 to 4 of them.

### Assumptions

- "A small durable routine-run result" means a subset of the existing record format, not a new
  store and not a second account.
- One run slot stays; nothing here introduces concurrency.
- The bug report stays a file the person sends. Nothing is transmitted anywhere.

### Prototype uncertainties, kept separate from the estimate

- **Where the reference is reserved** (R2). The Act path reserves in director, before the run. For
  a routine the candidates are `marco do` itself or Studio before it spawns. The exclusive-create
  guard in `reserveRun` is what makes a reference provably one attempt's, so whichever end
  reserves has to be the end that writes. Needs a prototype, not a guess.
- **How the option is threaded** (R4) — a second constructor or an argument on `Start`. Cheap
  either way; the cost is in which callers have to change.
- **Whether a routine's record should carry verification counts at all.** It has none to carry,
  and a record with three zeroes in it reads like a run that verified nothing rather than one that
  never claimed to. Probably omitted, which the format already allows.

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
- **AN IMPOSSIBILITY: forced termination cannot be cleaned up, and must not be claimed.**
  Cancellation is already cooperative and correct — `sleepCtx` returns early, the releases still
  run, `ctx.Err()` comes back — and that deserves a test rather than a change. A *killed* process
  is different: nothing in-process can release anything, so a chord routine killed mid-press can
  leave Ctrl held until the person presses it themselves. A watchdog or a release-on-next-start
  would be a separate decision. Normal unwinding does not establish this and this assessment does
  not claim it.

### Chord estimate

| Work | Increments |
|---|---|
| The partial-delivery cleanup plus release failures in the account, with stuck-key tests | 1 |
| A cancellation test for the path that is already correct | 0.5 |
| Studio naming a chord in the review in words a person reads | 0.5 |

Design first, as directed. The design question worth settling before any of it: whether a chord
whose modifiers were delivered and whose tap was refused should report the tap's failure only, or
say that the modifiers were pressed and released as well — the person's screen may have changed
either way.

## 5. Parked follow-ups, estimated separately

- **Typed text: 2–3 increments.** Not one `do` to wrap. `splitSecrets` can emit several sentences
  from one step — literal runs, declared arguments, and secrets resolved at run time — so
  propagation means choosing the unit and deciding what a partly typed line means. `doType`
  records nothing in the account either. And a failure reason must not quote the text, because the
  text can be a secret.
- **Holds: 1–2 increments.** `KeyDown`/`KeyUp` bracket the steps between them, so propagation
  means deciding what the release owes a press that did not happen. `heldKeys` already exists and
  the chord fix above shares it, so some of this is paid for by that work.
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

Then R4 and R6 (explicit mode, truthful readiness), then R5 (repeat).

### Acceptance, Studio only, no terminal

1. Launch `studio.exe` from Explorer, freshly built, with Studio closed before the build.
2. The library line names the directory, and the routines saved earlier are listed.
3. Record → review the ordered actions → Save.
4. Select it and Play. Then Play it again: twice in a row, both successful.
5. Move the application's window and Play: it presses where it was shown.
6. Resize the window and Play: it refuses, pressing nothing, and the banner names the term that
   disagreed.
7. Stop mid-run: `cancelled`, not a failure.
8. **Report a problem** produces a file naming the routine's identity, the running build, the
   mode, the outcome word, the reason, and the library — without a terminal.
9. Nothing in the window reads *"playback completed"* as *"the game did what you wanted"*. The
   sentence that ships today says the presses happened and that nothing checked the outcome, and
   that distinction is load-bearing.
10. Existing Acts remain listed and performable through their own path, unchanged.

### Compatibility risks

- **A second writer of the run format** (R1, R2). Mitigated by lifting the type so there is one
  writer and one reader rather than a duplicated field list.
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
