---
type: reference
status: active
updated: 2026-08-20
source_paths:
  - internal/invoke
  - internal/orchestrator
  - internal/routes
  - internal/plays
  - internal/outcome
  - internal/stopsignal
  - internal/director/observe
  - internal/director/rehearse
  - internal/director/execute
  - internal/platform/theaterhost
  - cmd/marco/intake.go
  - cmd/marco/perform.go
  - cmd/director/perform.go
  - cmd/director/rehearserun.go
  - plugins/overlay
---

# 34F duplication matrix — who owns each concept, and what is still doubled

Written at the close of Roadmap 34F, after Phases 0–4 landed. It is a **re-verification**, not a
fresh audit: nine read-only audits were written mid-campaign against `810ea1d`/`fc1919d`, and this
note re-walks every one of their headline claims against the tree at `ac8da6c`. Where an audit is
now wrong it is marked **CLOSED** and the fix is named, because a matrix that repeats a finding
somebody already closed is worse than no matrix — it sends the next session to fix something twice.

Companion to [[34F-legacy-marco-product-audit]] §4 (the original, much shorter matrix) and to
[[34F-legacy-matrix]] (what to keep, deprecate and delete). [[34F-observe-readiness]] carries the
perception half, which is deliberately not repeated here.

## How to read a row

- **CANONICAL OWNER** — the one place that decides the fact, `file:line`.
- **REMAINING DUPLICATES** — anything else that decides the same fact for itself.
- **WHY EACH REMAINS** — `L` legitimate distinction · `C` compatibility · `A` accident · `Z` zombie.
- **RETIREMENT PLAN** — what closing it costs and what it would break.

A row with no duplicates is not padding. It is the record that somebody consolidated it, so the
next reader does not go looking.

---

## 1. Invocation — turning "what a person asked for" into a performance

**CANONICAL OWNER** — `cmd/marco/intake.go:122 runInvocation` (the one process entry) over
`internal/invoke/invoke.go:146 Decide` (the pure decision). Every product entrance reaches it:
`marco do` (CLI, overlay, control centre), a hotkey press (`cmd/marco/bind.go` `pressHotkey`,
which enters with an explicit `Request{Play:…}` and never re-reads the words), the control
centre's clicked Run, and `plugins/web-ui`'s Run button.

| # | duplicate | why | retirement |
|---|---|---|---|
| I-1 | `orchestrator.Deps.Do` / `.Resolve` / `.Run` | — | **CLOSED in Phase 3.** The functions are gone; `internal/orchestrator` now exposes only `Learn`/`LearnAuto`/`LearnVoice`/`SimplifyRoute` and the authority door. The audits' "most dangerous zombie in the tree" no longer exists. |
| I-2 | `marco director "<phrase>"` (`cmd/marco/director.go`) | **C** | Still open. A bare phrase goes straight to the Director with **no Play lookup**, so a Play the intake would have matched exactly is instead reinterpreted against the screen. The overlay no longer uses it; `plugins/web-ui/playbill.go` still shells it for read-only verbs. **Plan:** narrow the verb to `marco director <subcommand>` and let phrases enter `runInvocation`, or document the asymmetry where the verb is defined. |
| I-3 | `cmd/marco/intake.go` grammar re-resolve | **L** | Keep. It runs deliberately AFTER `Decide` so a Play's own name beats the grammar, and `TestAPlayNamedWithTheGrammarIsStillReachable` holds it. Worth one line in `internal/invoke`'s doc: `Decide` is the only place words become a *Play-or-Director verdict*, not the only place words become a Play. |
| I-4 | `cmd/marco/edit.go findRouteByName` — a scope-blind name→Route scan | **C** | Still open. It ignores `Registry.Resolve`'s four-step scope precedence, so a name-only Run from the control centre can land on a different app's Play than the same words typed into the HUD. **Plan:** fence it to the two payload paths that need it and say so at the definition. |
| I-5 | `internal/dispatch` + `marco dispatch` | **Z** | Deprecate the verb. `marco help` now lists it under *"Developer surfaces, kept working and not part of the normal product"*, which was the finding; the package still holds a second run/learn/chat/clarify taxonomy. |
| I-6 | `nlu.Resolve` deciding which Play `marco args` / `marco simplify` acts on | — | **CLOSED.** `runAssistantSimplify` resolves exactly and the matcher may now only *suggest* (`TestSimplifyRewritesOnlyThePlayItWasAskedFor`). This one mattered: `marco simplify` rewrites a Play's source in place, and the overlay offers it as a command word with `MARCO_SIMPLIFY_SAVES=1`. |

**Guard strength.** `TestEveryEntranceRoutesThroughTheOneIntake` is a string grep over two named
files. It would pass unchanged if a tenth entrance appeared tomorrow. The precedent to copy is
`internal/platform/navsource/pump_test.go`, which walks the **whole tree** for hook sites rather
than naming them. Until that exists, "one intake" is an observation rather than an invariant.

---

## 2. Semantic interpretation — "what do these words mean?"

**CANONICAL OWNER — split, correctly, in two.** Name lookup is
`internal/routes/registry.go:302 Registry.Resolve`; meaning-from-the-world is
`internal/director/execute/program.go:498 Pipeline.HandleRequest`. `invoke.Decide` is the arbiter,
and it is genuinely pure — no filesystem, no service, and `Source` is recorded and never read
(`TestSourceCannotChangeTheDecision`).

Remaining: `intent.IsControlPhrase` is called from three places and that is the right shape (one
function, three callers). `ParseClarification` is asked in the intake and again in the service on
purpose, so a stale question cannot hijack a known Play. `internal/director/target.Resolver`
shares the word *resolve* with `Registry.Resolve` and `nlu.Resolve` while doing something else
entirely — element resolution inside a world state. **All `L`. Keep.**

> **BOUNDARY, not a duplicate — Binding lookup ≠ Director reasoning.** `pressHotkey` resolves the
> binding to an identity ONCE and hands `invoke.Request{Play:&rt}` to the intake, which then reads
> no words at all. `TestAHotkeyPerformsTheBoundPlayItself` holds it. Merging these would make a
> hotkey a phrase again, and a phrase can be reinterpreted.

---

## 3. Current Stage — "what is here now"

**CANONICAL OWNER — there still is none.** `grep 'type Stage'` returns three unrelated types
(`pkg/playbill`'s learning stage, `rehearse.Stage`'s produce/verify pair, `execute.Stage`'s trace
phase). "What is here now" is assembled from four facts — foreground application (`winctx.Active`),
which window (`perception/windowref`, under three different precedence rules), which place
(`observe.PlaceNow`), and whether a place is showing as seen from `cmd/marco` — and while
`PlaceNow` is canonical for the *resolution*, each of its callers chooses the evidence in front of
it for itself.

**WHY — `A`, and it is [[34E-director-theater-audit]]'s open item unchanged.** `PlaceNow`'s own doc
says the wide interface is what made four answers look harmless; the resolver was consolidated and
the evidence selection in front of it was not.

**RETIREMENT PLAN — this is the consolidation OBSERVE most needs.** One `Stage.Now(application)`
owning evidence selection, with the `PlaceNow` callers reading it. Sized and argued in
[[34F-observe-readiness]]; it is 35A work, not 34F work.

> **BOUNDARY — Stage ≠ semantic memory.** Held in code, absent as a noun. `observe.PlaceNow` is a
> pure projection over a narrow `Recogniser` and **cannot write**. Because there is no Stage type,
> that distinction currently lives in a doc comment rather than in the type system.

---

## 4. Current Place — identity of the screen

**CANONICAL OWNER** — `internal/director/observe/place.go:58 PlaceNow` + `semanticmemory.Recall`.

| # | duplicate | why | retirement |
|---|---|---|---|
| P-1 | `cmd/marco/screenwiring.go liveScreens.CurrentSubject` returning `Unavailable` unconditionally | — | **CLOSED in Phase 3.** It now **asks the Director** over protocol v9 and refuses only when it cannot ask. This closed a real hole the audits found: an **edited** learned play loses `Resolved.Learned()`, takes the local runner, and used to refuse at its own first `do Screen's Showing` line — so editing a Play in the step editor made it unrunnable, while the authority door was deliberately blessing exactly that edit. Held by `TestTheDirectorsAnswerSatisfiesTheEntryGuard`, `TestNoDirectorMeansMarcoCannotSeeAndSaysSo` and `TestADirectorThatDoesNotRecogniseTheScreenRefuses`. |
| P-2 | `cmd/director/sightplace.go PlaceStatus` (`nowhere`/…) | **L** | A *pointing* status, not a place identity. Keep. |
| P-3 | `internal/screen` template/colour matching | **Z-in-practice** | `MARCO_CV` defaults off in all three switches. The default is still an undecided product question — see [[34F-legacy-matrix]]. |

34F break #3 (two memory files) is **FIXED**: both processes resolve `$MARCO_MEMORY` else
`$MARCO_HOME/semantic-memory.json`, with paired tests either side. The *default derivation* is
still written twice (see §12 R-3).

---

## 5. Planning

**CANONICAL OWNERS — two, legitimately different.** Semantic-action planning
(`internal/director/plan` + `goal`, entered by `HandleRequest`) plans an arbitrary request against
the live world. Learned-route planning (`observe.PlanToGoal`) plans over **verified edges between
remembered Places**. Neither computes the other's answer; two callers assemble the inputs
independently, which is minor.

> **BOUNDARY — Director planning ≠ Theater performance.** Intact and structural: neither planner
> touches a host. `internal/director/rehearse` is the only way a press becomes an effect
> (`TestALivePressIsPutOnByTheTheater`).

---

## 6. Execution / performance

**CANONICAL OWNER** — three layers, one runtime: `internal/runtime` + `internal/driver` (the one
Marco engine), `internal/platform/marcorunner` (compile-then-run; the only way the Director reaches
a host), and `internal/director/rehearse/live.go:470 Live.Perform` — **the one edge walker**, shared
by rehearsal and execution and held by `TestRehearsalAndExecutionShareOneWalker`.

| # | duplicate | why | retirement |
|---|---|---|---|
| E-1 | two composition roots for the shared walker, differing by one `WithForeground` call | — | **CLOSED in Phase 3.** `cmd/director/rehearserun.go:167 walker` is now THE root and `cmd/director/perform.go:657 performer` delegates to it. This was a real safety regression hiding behind a green suite: the per-step "the user alt-tabbed, do not type into their email" refusal was live for the rehearsal Marco asks permission for and **dead for the play the Audience asks for** — exactly backwards from how anybody would choose it. Held by `TestEveryLiveWalkerChecksTheForeground`, which enumerates every function in `cmd/director` that builds a `rehearse.Live` rather than naming files. |
| E-2 | a `marcorunner` built and discarded in `performer()` | — | **CLOSED** with E-1. |
| E-3 | `internal/director/execute` pipeline → `marcoexec` → `marcorunner` | **L** | A different thing: arbitrary verbs against a world model, not a walk over verified edges. Keep, documented. |
| E-4 | `cmd/director/lowerwiring.go RunOperation` → `effects.Do` | **L**-as-diagnostic / **A**-as-exposure | Still open. It executes one typed Operation with **no authority, no grant, no policy stage**, and it is exposed over the service protocol (`RequestRunOperation`) to any client, not only the CLI. Its own comment argues it is "NOT a bypass" because it shares the executor and the foreground guard — true of the *executor*, false of the *permission*. **Plan:** keep as a developer surface; gate it, and say at the call site that it is unauthorised. |
| E-5 | `marco press` → `oshost.Invoke` | **L** as a primitive | Keep. It is reachable from the product (the overlay claims `press` as a verb, typed **or spoken**), and `cmd/marco/press.go:21-45` now carries the whole argument for why it does not pass `Decide`: a key press names no Play, resolves no identity and has no provenance to weigh. The audits asked for this boundary to be documented rather than discovered; it is. |
| E-6 | `marco run` / `marco serve` with real hosts | **L** | The language runner, and production — `overlay.cmd` runs `marco serve … overlay.marco`. Keep. |
| E-7 | `orchestrator.Deps.Run` | — | **CLOSED** with I-1. |

---

## 7. Target resolution

**CANONICAL OWNERS — three, for three kinds of target.** A semantic UI element in a world state →
`internal/director/target.Resolver`. A named control on the live stage →
`theaterhost.Theater.Activate` (Actor `Find` → Candidate → the `internal/activate` ladder). A
durable Place/Target in memory → `internal/director/observe` + `semanticmemory`.

Remaining: the recorded-coordinate / template / OCR / vision stack for taught routes (`L` as a
mechanism, `Z` in the shipped default, since CV is off); `internal/activate` as ONE ladder (the 34E
fix, holding); and two Actor rosters, one per process — `L`, but see §14.

---

## 8. Verification

**CANONICAL OWNERS — two questions and one arrival check.** "Did this ACTION do what the plan
said" → `internal/director/verify.Verifier`. "Did this STEP land us on the next Place" →
`rehearse/live.go settled` plus `production.Verifier`. "Did the whole walk ARRIVE" →
`cmd/director/perform.go confirmArrival`, which takes a **fresh look** rather than assuming.

| # | duplicate | why | retirement |
|---|---|---|---|
| V-1 | a generated Play's own `do Screen's Showing "<place>"` guard | **L** | A Play's own precondition, for a Play run locally. On the product path the Director re-plans and never executes these lines ([[ADR-078-a-learned-play-is-performed-by-the-director]]). Since Phase 3 the local path can actually answer it — see P-1. |
| V-2 | `cmd/marco/theaterwiring.go` builds the Theater with **no** verifier; the Director builds it with one | **L** | Honestly documented at the definition: the answer is `not_verified`, never a claimed success. Keep. |
| V-3 | `production.Refusal` (6 words) vs `theaterhost.Refusal` (5 of the same) vs `rehearse`'s own vs `service.PerformView.Refusal` strings | **A** for the first pair | **Still open.** `theaterhost` already imports `production` and casts between them with a bare string conversion. **Plan:** delete `theaterhost.Refusal` and use `production.Refusal`; keep `rehearse`'s `refusalFor`, which is a legitimate translation between layers. Nothing was consolidated here in Phase 4 — Phase 4 unified the *product outcome* vocabulary, which is a different set of six words for a different question. |

---

## 9. Authority

**There is no canonical owner. There are three regimes, and they do not know about each other.**

| regime | owner | what it gates | who mints |
|---|---|---|---|
| A | `internal/orchestrator/authority.go:126 Authorize` + `AskFirst` | performing a **Play** from `cmd/marco` | the Audience answering y/n |
| B | `observe.NewRehearsalGrant`, spent in `rehearse/live.go BeginAttempt` | **one bounded attempt at one edge** — bounded inputs, bounded duration, spent once | an explicit request, or the proposal ledger |
| C | `internal/director/policy` + the confirmation broker | one **semantic action** in the `Handle` pipeline | contributed rules, which may only narrow |

**This is the largest un-named structure in the product.** A reader must currently discover that
"authority" means three different things in three packages. Regime A exists because *a learned Play
is Marco's composition*; regime C's sufficiency for `marco director "click save"` is legitimate
because a semantic action is the person's own instruction. Both are defensible; neither is written
down in one place. **Plan: one page naming the three regimes and the boundaries between them.**
Two smaller residues: `theaterhost.Host.activate` deliberately skips `Theater.Perform`'s claim and
the argument for that lives only in the *other* entry's doc comment; `RunOperation` has no regime
at all (E-4).

---

## 10. Cancellation

**CANONICAL OWNER — one chain, and since Phase 3 it crosses processes.**
`intent.IsControlPhrase` → `invoke.KindControl` → `cmd/marco/intake.go runControl` →
`directorStop` → `internal/director/service/server.go:802 cancelActive` → the active command's ctx,
**plus** `internal/stopsignal`, which is how one `marco` process stops a Play running in another.
See [[ADR-087-one-stop-and-it-crosses-a-process-boundary]].

| # | duplicate | why | retirement |
|---|---|---|---|
| C-1 | the overlay's local kill beside `stopTheDirector` | **L** | Keep. "Immediacy is local, authority is remote" — the local half decides nothing and cannot consume the phrase. |
| C-2 | `withPanicStop` (stop key → ctx cancel) | **L** | Keep. The in-process kill switch, disabled when the overlay owns the hooks. |
| C-3 | a live rehearsal running under `context.Background()` | — | **CLOSED in Phase 3.** `learnTail.Rehearse` now names and passes its context and `Runtime.Rehearse` takes one. Pressing **Try** in the Learn panel used to start real input that **nothing in the product could stop** — not the panel's Cancel, not `director stop`, not the overlay's Esc, not a spoken "stop". |
| C-4 | the Learn session as a fourth cancellation domain, invisible to `service.Registry` | — | **CLOSED in Phase 3.** `cancelActive` has a third arm for the Learn episode, held by tests in `internal/director/service/stop_test.go` that name the mutation: delete the arm and "stop" answers *nothing is running* while somebody is mid-demonstration. |
| C-5 | the recorder's stop key ending a demonstration | **L** | A different event. Keep. |

**And one thing that was never a duplicate but was simply false:** `finally` did not run on
cancellation, while `spec/Core.md` says normatively that it does — with a worked example of
releasing a held key, which is precisely the work that must happen when somebody presses stop.
Fixed in Phase 3 — [[ADR-088-cleanup-runs-when-the-audience-stops]] — held by
`TestFinallyRunsAfterLanguageCancel` and three siblings. A green suite and a false normative
sentence coexisted for the whole life of the feature.

---

## 11. Acquisition (Learn)

**FOUR mechanisms.** The brief names two as a boundary; there are four.

| # | mechanism | owner | produces |
|---|---|---|---|
| 1 | **Record Learn** | `orchestrator.Deps.Learn` / `LearnAuto` → recorder → simplify → codegen | a `.marco` + `.rec.json`; displayed as **Recorded** |
| 2 | **Narrate Learn** | `internal/voicelearn`, via `Deps.LearnVoice` | a `.marco`; read-only during capture |
| 3 | **Semantic Learn** | `internal/director/learn.Coordinator` + `cmd/director/learnsessionwiring.go` | a **staged** `.marco` + `.origin.json`, kind `learned` |
| 4 | **Semantic demonstration → procedure** | `internal/director/demo` | a `demo.Learned` **procedure** in the goal registry — not a Play, and with no product surface |

> **BOUNDARY — Record Learn ≠ Semantic Learn.** Held, and still under-named. 1 and 3 are genuinely
> different (exact replay versus an understood destination); 2 is a third capture *modality*
> sharing 1's output; 4 produces a different artifact entirely. **Do not merge 1 and 3.** Do
> present 1 and 2 as two modes of one Learn, and do give 4 a surface or call it a research surface
> — right now it is a fourth thing spelled "learn/demonstrate" with a CLI and nothing else.

---

## 12. Persistence / Play repository

**CANONICAL OWNER** — `internal/routes.Registry` (the store) + `internal/routes/origin.go`
(kind, digest, staging, `Register`) + `internal/plays` (the product projection).

`internal/plays` is the cleanest consolidation in the tree: `List` = `Registered` + `Staged`, every
field a rendering of something `routes` already decided, read-only by test
(`TestBrowsingPlaysChangesNothingOnDisk`), and every consumer reads it.

| # | duplicate | why | retirement |
|---|---|---|---|
| R-1 | `cmd/marco/edit.go handleRoutes` reading `reg.List()` directly | **C** | The Edit picker's legacy shape, documented at the definition. Keep. |
| R-2 | `cmd/director/learnedplay.go:231 learnedRegistry()` re-deriving `$MARCO_ROUTES` independently of `cmd/marco/assistant.go:29 routesDir()` | **A** | Two `package main`s, no shared helper. **Still open.** |
| R-3 | `cmd/director/graph.go:39 defaultHome()` vs `cmd/marco/director.go:169 directorDir()` | **A** | **Still open, and the file itself argues against it**: `graph.go` says a guard that computed the real location a second time could drift from the one the store actually uses. There is a second one, in the other binary. The paired memory-path tests both set `MARCO_HOME` explicitly, so the *default* derivation is duplicated and unpinned. 34F break #3 is in remission, not cured. |
| — | **Plan for R-2 and R-3** | — | One `internal/marcopaths` owning `$MARCO_HOME`, `$MARCO_ROUTES` and the uia bridge discovery. `S`-sized, and it closes AA-2 as well. |
| R-4 | `routes/os.marco`, byte-identical to `internal/osmod/os.marco` | **Z** | Stop shipping it. **Never delete a file from a user's routes directory.** |
| R-5 | `routes/bindings.json` binding `` `e `` to a Play that does not exist, plus empty `routes/<app>/` scaffolding | **Z** | Stop creating the scaffold; leave existing directories alone. |
| R-6 | five stores with no single account: the routes tree, `semantic-memory.json`, `action-graph.json`, `bindings.json`, `overlay.json` | **L** | Different things, legitimately. **Plan: write one "where Marco keeps things" page.** There is none, and consolidating persistence without it will miss one. |

---

## 13. Bindings

**CANONICAL OWNER** — `internal/routes/bindings.go` (`Binding{App,Key,Slug,Cmd}` at
`<Dir>/bindings.json`), with `cmd/marco/bind.go bindKey` as the validating door.

| # | duplicate | why | retirement |
|---|---|---|---|
| B-1 | `cmd/marco/edit.go:506 handleBind` calls `reg.Bind` directly and validates **nothing** | **A, with a live consequence** | **STILL OPEN — the most concrete unclosed defect in this matrix.** `bindKey` exists precisely because a binding accepted on a fuzzy score stored words the press could never resolve: *"a binding that reports success and can never fire is worse than one that refuses"*, held by `TestABindingIsValidatedTheWayItWillBeResolved`. The control-centre endpoint reintroduces exactly that — no per-step `Reg.Resolve`, and it infers the app with the scope-blind `findRouteByName` rather than the app the press will resolve in. **Plan: route `/api/bind` through `bindKey`.** `S`-sized. |
| B-2 | `internal/director/binding` — deictic reference ("this file") | **L** | An unrelated concept sharing a word. **Plan:** rename to `internal/director/deixis`, or document loudly. It will cost somebody an hour. |
| B-3 | the overlay's `Leader` versus `bindings.json` keys | **L** | The leader is the prefix, the binding is the key after it. Document. |

---

## 14. Actor availability

**CANONICAL OWNER** — `theaterhost.Theater.Roster` (`theaterhost.go:307`) + `Host.Roster` (`:447`).
Since Phase 4 an Actor **asks its provider** whether it can act and carries a **reason** when it
cannot, with `Reason` empty always when it can — held by `TestAReadyPlayerNeverCarriesAReason`.

| # | fact | why | retirement |
|---|---|---|---|
| AA-1 | the only roster surface builds a **fresh `marco`-side Theater**; the Director's Theater is constructed per-call and has no diagnostic at all | **A** | **Still open.** The process being reported on is not the process that performs learned plays. **Plan:** add a roster to the Director's own diagnostics, or hoist the Theater to a `Runtime` field so it can be asked. |
| AA-2 | two Actor-discovery policies for one plugin (`accessibilityBridge()` versus `cmd/director`'s `--accessibility` default) | **A** | Consolidate with R-2/R-3. |
| AA-3 | `plugins/uia` is simultaneously a bridge host (fulfils an *act*) and the substrate for an Actor (cast by the Theater) | **L** | Keep, and document — this is currently legible from one doc comment. |

34F break #4 is **FIXED**: the bridge is discovered without `$MARCO_UIA_BRIDGE`, held by
`TestALearnedPlayFindsAnActorWithoutBeingTold`.

> **BOUNDARY — Plugin ≠ Actor.** Held, barely legible. A bridge host fulfils an *act*; an Actor is
> *cast* at run time by the Theater. `plugins/uia` being both is why they read as one thing. There
> is exactly **one** Actor implementation in the whole product (`AccessibilityActor`); everything
> else under `plugins/` is an act host, an advisor, a front end, an event feed or an installer.

---

## 15. Product status — "what is Marco doing / what happened"

**CANONICAL OWNER — `internal/outcome`, since Phase 4.** Six words
(`performed`/`clarify`/`refused`/`unavailable`/`cancelled`/`failed`), the `"[result] "` and
`"[route] "` wire literals, exit codes, and parsing — all in one engine package that the engine,
the HUD **and** the control centre import. Beside it, two accounts on different timescales that
are not duplicates of it: `pkg/playbill` (the Director's live account) and `service.CommandState`
(per-command state in the command registry).

| # | duplicate | why | retirement |
|---|---|---|---|
| PS-1 | the overlay restating the six words as literals across a module boundary | — | **CLOSED in Phase 4.** The overlay imports `internal/outcome`; the words exist once. |
| PS-2 | the control centre rendering `running:` forever, having fire-and-forgotten a child | — | **CLOSED in Phase 4.** `cmd/marco/runaccount.go` gives each run an id the page polls, and reports the **real** outcome read off the child's `[result] ` line. |
| PS-3 | `plugins/overlay/model.go:429 addHistory` — a second bounded in-memory command history beside playbill `Recent` and the action graph | **A** | **Open at `ac8da6c`.** One Activity, on the playbill. `M`-sized, because the playbill has no per-command duration/outcome field today. Work on a shared Activity account was in flight in the same tree while this note was written (`cmd/marco/activity.go`); check before starting it. |
| PS-4 | `cmd/marco/oconfig.go:15 overlayConfig` hand-mirroring `plugins/overlay/config.go Config` | **C**, and **lossy** | **Still open, and it can delete a user's setting.** Unknown-to-here keys are not preserved, so adding a field to the overlay's `Config` and not to marco's mirror means the control centre **drops it on save**. The overlay already imports `internal/invoke` and `internal/director/intent`, so the module boundary is not the obstacle. |
| PS-5 | three copies of the `MARCO_CV`/`MARCO_ANCHORS` decision (`orchestrator.go:124`, `oshost.go:881`, `recorder_windows.go:47`) plus two of `cvSensitivity` | **A**, hand-maintained | **Still open.** Harmless while CV is off everywhere; a landmine the day the default is flipped, which 34F says it should be. **Plan: one `internal/cvflag`.** |
| PS-6 | `MARCO_NO_TEACH` | **Z** | **CLOSED.** The setter is gone and `plugins/overlay/stop_test.go` refuses its return. One stale reference survives in a `cmd/marco` test comment — see *Handoffs*. |
| PS-7 | `invoke.SourceWeb` with no setter | — | **CLOSED in Phase 3.** `plugins/web-ui` now sends `do --source=web` and, for a listed Play's Run button, `--play=<slug>`. |

---

## The second-Marco question

> Is there a path by which the product can take an intent all the way to real input without going
> through `invoke.Decide → runInvocation → authority → the shared Perform walker → Theater`?

The mid-campaign audit found **nine**. Re-verified at `ac8da6c`, **five remain, all narrow, and
each is defensible on its own terms.** The four that closed are the four that mattered.

### Closed

| was | closed by |
|---|---|
| Learn panel **Try** → a live rehearsal under `context.Background()`, outside `service.Registry` — invisible to `director status`, unrefusable as busy, and unreachable by every stop the product has | Phase 3: a real context all the way down, and a Learn episode `cancelActive` can find |
| `director rehearse --live` sharing that Background-context defect | Phase 3, with the above |
| `plugins/web-ui` shelling `marco do "<display name>"` with no `--source` and no `--play` — the last legacy-shaped invocation in the repository, and the reason a Run button there could reach Director while the same button in the control centre could not | Phase 3 |
| `orchestrator.Deps.Do` — a complete second spine (resolve → authorize → run → learn-on-unknown) that **only tests could enter**, and worse than the live one: no context, no `ApplyArgs` | Phase 3: deleted, with the authority tests moved onto the production entry |

### Remaining, and the verdict on each

| # | entry | verdict |
|---|---|---|
| SM-1 | `marco press <key>` (an overlay verb, typed **or spoken**) → `oshost.Invoke` | **Legitimate, and now argued at the definition.** A key press names no Play and has no provenance; routing it through `Decide` would send "press control c" to Director to hunt for something on screen by that name. It has `withPanicStop`. The product model must **name** this boundary rather than let somebody rediscover it. |
| SM-3 | `marco director "<phrase>"` → `Submit` → `Handle` → pipeline → real input | **Legitimate as a Director client verb, but it skips Play lookup**, so a Play Marco knows exactly can be reinterpreted against the screen. The overlay no longer uses it. See I-2 — this is the one with an actual retirement plan. |
| SM-4 | `marco serve --host OS=bridge:… overlay.marco` | **Legitimate.** The language runner, in production. Benign today — `overlay.marco` calls no `OS's` anything — but the capability is wired open. |
| SM-5 | `/api/bind` → an unvalidated binding whose later press *does* enter the intake but silently skips unresolvable steps | **Not a second Marco — it rejoins the spine — but not acceptable either.** It can persist a trigger that reports success and can never fire. See B-1. |
| SM-6 | `director op <operation>` → `RunOperation` → `effects.Do`, over the service protocol | **The weakest of the five.** Developer-only by intent, but exposed to any client, with no policy stage, no confirmation and no grant. See E-4. |
| SM-9 | `marco run --host windows <file>` | **Legitimate.** The language runner; has `withPanicStop`. |

**The honest summary:** there is no second decider and no second performer any more. There are a
handful of narrow primitives that reach real input directly, each defensible, and the work left is
to **name them in the product model** rather than to close them — except `/api/bind`, which should
simply be routed through the door that already exists.

---

## The five boundaries, re-checked

| boundary | held? | evidence |
|---|---|---|
| **Binding lookup ≠ Director reasoning** | **held** | `pressHotkey` resolves once and passes an explicit identity; `Decide` then reads no words |
| **Director planning ≠ Theater performance** | **held** | no planner touches a host; `rehearse` is the only path from press to effect |
| **Record Learn ≠ Semantic Learn** | **held, under-named** | four mechanisms, two of them Record-family; the product explains the difference nowhere |
| **Plugin ≠ Actor** | **held, barely legible** | one Actor implementation in the tree; `plugins/uia` is both a bridge host and an Actor's substrate |
| **Stage ≠ semantic memory** | **held in code, absent as a noun** | `PlaceNow` is a projection and cannot write — but there is no Stage type, so this lives in a comment |

---

## Ranked backlog after Phases 0–4

| rank | item | row | effort |
|---|---|---|---|
| 1 | `/api/bind` → `bindKey` — an unfireable trigger is a promise the product cannot keep | B-1 | S |
| 2 | one `Stage.Now()` owning evidence selection in front of `PlaceNow` | §3 | L |
| 3 | replace the grep-based one-intake test with an enumerating walk | §1 | M |
| 4 | one `internal/marcopaths` (`$MARCO_HOME`, `$MARCO_ROUTES`, bridge discovery) | R-2/R-3/AA-2 | S |
| 5 | share `overlayConfig`/`Config` — today the control centre can delete a setting it does not know | PS-4 | S |
| 6 | one page naming the three authority regimes | §9 | S |
| 7 | delete `theaterhost.Refusal`, use `production.Refusal` | V-3 | S |
| 8 | one `internal/cvflag` | PS-5 | S |
| 9 | a Director-side Theater roster diagnostic | AA-1 | S |
| 10 | one Activity: retire the overlay's private history onto the playbill | PS-3 | M |
| 11 | narrow or route `marco director "<phrase>"` | I-2 | M |
| 12 | gate `RunOperation`, or fence it off the service protocol | E-4 | S |
| 13 | stop shipping `routes/os.marco` and the empty scaffold | R-4/R-5 | S |
| 14 | rename `internal/director/binding` | B-2 | S |
| 15 | write "where Marco keeps things" | R-6 | S |

## Handoffs — things outside this note's scope

- `cmd/marco/playscli_test.go` sets `MARCO_NO_TEACH=1` with a comment saying *"the overlay's
  setting; without it this learns instead"*. Nothing reads that variable any more, so the comment
  describes behaviour that no longer exists and the `Setenv` is inert. Cosmetic — but it is exactly
  the kind of stale breadcrumb that costs a future session an hour.
- `CLAUDE.md` says the suite is 73 packages; `go list ./...` reports 117.
- `plugins/overlay` carried a fully dead in-HUD help panel at `ac8da6c` — `helpLines()` had no
  caller, `showHelp` was referenced by nothing outside its own definition, and `helpOn` was only
  ever set false, because `help` opens the control centre's Help screen instead. **`helpLines` was
  removed by concurrent work during this same campaign**, so verify the residue (`showHelp`,
  `helpOn`, the `view.go` branch that renders it) rather than assuming either state.

## Related

- [[34F-legacy-marco-product-audit]] · [[34F-legacy-matrix]] · [[34F-observe-readiness]]
- [[34E-director-theater-audit]] · [[Invocation]] · [[Plays]] · [[Wiring-Tests]]
- [[ADR-083-one-invocation-intake]] · [[ADR-085-a-performance-is-a-registry-command]] ·
  [[ADR-087-one-stop-and-it-crosses-a-process-boundary]] ·
  [[ADR-088-cleanup-runs-when-the-audience-stops]]
