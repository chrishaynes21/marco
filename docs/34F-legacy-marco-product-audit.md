---
type: reference
status: active
updated: 2026-08-19
source_paths:
  - internal/routes
  - internal/orchestrator
  - internal/dispatch
  - internal/nlu
  - internal/simplify
  - internal/codegen
  - internal/recorder
  - internal/macroir
  - internal/voiceteach
  - cmd/marco
  - cmd/director/learnedplay.go
  - plugins/overlay
  - plugins/web-ui
  - internal/platform/screenhost
  - internal/platform/theaterhost
---

# Roadmap 34F — Legacy Marco product audit

Read-only. **No code was moved, renamed, deleted or changed.** Every claim below was traced
through the code; where a claim is about behaviour rather than structure it names the file and
line that decides it.

Companion to [[34E-director-theater-audit]], which audited the *new* half. This one audits the
*old* half, and it reaches the opposite conclusion about ownership: 34E found a subsystem that was
built and never ran; 34F finds a product that runs and can no longer complete its own promise.

---

## 1. Executive summary

### 1.1 The one-sentence answer to the core question

If Marco had been designed from day one around Audience / Director / Theater / Stage / Actors /
Targets / Places / Plays, **almost all of the legacy machinery would still exist** — the language,
the compiler, the runtime, the act/host boundary, the recorder, the simplifier, the code
generator, the route registry, the provenance sidecars, the authority seam, the hotkey bindings,
the HUD and the control centre. What would be different is the **names and the owners**, not the
code.

Three things would never have been built as they are:

1. **`internal/dispatch` + `internal/nlu`'s Advisor layer** — a second, LLM-shaped
   phrase-to-behaviour decider that predates Director and is now reachable only from a CLI verb
   nothing in the product invokes.
2. **The typed-versus-spoken split** at `plugins/overlay/acts.go:211`, where a *spoken* phrase goes
   to the Director and a *typed* phrase goes to route lookup. That is a history of arrival, not a
   product decision.
3. **`routes/os.marco`** — a byte-identical copy of `internal/osmod/os.marco` shipped in the user's
   routes tree.

Everything else old is either still first-class, or is the infrastructure the new model stands on.

### 1.2 The finding that matters most

> **Marco can learn a play and cannot run it. The chain from *saved* to *runnable* is broken in
> four independent places, and the product tells the user it worked.**

`internal/routes` already models a Play completely: `Kind{authored, taught, learned}`, a provenance
sidecar with an integrity digest, a staging directory the resolver structurally cannot see, and
`Register`/`Unregister` to move between them. `cmd/director/learnedplay.go` drives it correctly.
`internal/orchestrator/authority.go` puts a real door between resolving a play and performing it.
The architecture in [[Learned-Plays]] is *implemented*.

And then:

| # | break | evidence |
|---|---|---|
| **1** | **Nothing in any UI ever registers a play.** `teachTail.Save` (`cmd/director/teachtail.go:176`) sets `Save: true` and deliberately not `Register`. The Learn panel has six verbs and none of them is register (`cmd/marco/learnui.go`; `cmd/marco/edit.go:934-975`). The only register in the repository is the CLI flag `director learned --register` (`cmd/director/observecmd.go:1182`). | `Teaching.Registered` (`pkg/playbill/playbill.go:244`) is plumbed to the top of the product and is **structurally always false** on the Learn path. |
| **2** | **A registered play's first line always refuses.** Every learned play opens with `do Screen's Showing with "<place>"` (`marcoexec/play.go:77`, ADR-030). There is exactly **one** production implementation of `screenhost.Recognition.CurrentSubject` in the repository, at `cmd/marco/screenwiring.go:70`, and it returns `Unavailable` unconditionally, which `screenhost.go:134` turns into a refusal. | `grep -rn "func.*CurrentSubject"` returns one non-test hit. |
| **3** | **The two halves read different memory.** Director writes `%AppData%\marco\semantic-memory.json` (`cmd/director/graph.go:52`). `marco`'s Screen host reads `routes/memory.json` (`cmd/marco/screenwiring.go:43`). Different name, different directory. Even if break 2 were fixed, `SubjectNamed` would query an empty store. | two literals, two paths. |
| **4** | **The Theater has no actor in the shipped stack.** `hosts["Accessibility"]` is wired only when `$MARCO_UIA_BRIDGE` is set (`cmd/marco/assistant.go:69`). Neither `overlay.cmd` nor `setup.ps1` sets it, though `cmd/director/main.go:664` defaults the same path for itself. A play that presses a control by name would refuse `no_actor_available`. | `grep MARCO_UIA_BRIDGE setup.ps1 overlay.cmd` → nothing. |

And the broken promise on top of them: `cmd/marco/edit.go:1558` renders

```js
if(v.learned) out.push('<p style="color:var(--run)">Saved. It is in the Routes tab.</p>');
```

A saved play lands in `routes/<app>/learned/`. `Registry.List` scans `global/`, the app's loose
files, `context/` and `focus/` and **nothing else** (`registry.go` `List`/`listDir`), which is the
whole design of [[ADR-028-a-learned-play-is-a-file-with-a-past]]. The sentence is false, and it is
false *by construction*, not by accident.

### 1.3 The structural cause

**The process that can see cannot run Plays. The process that can run Plays cannot see.**

```
marco    (cmd/marco)     runtime · act host map · orchestrator · authority seam
                         Screen act · Theater act · route registry
                         -- and no perception at all --

director (cmd/director)  perception · fusion · semantic memory · Repertoire
                         places · targets · Stage · goals · Learn
                         -- and executes only through rehearse, under a
                            one-attempt grant, never through `marco do` --
```

They meet at exactly two places today: `marco director "<phrase>"` (a thin client), and the file
tree under `routes/`. The learned-play design assumed a third meeting — the Screen host asking
someone what place is in front — and that seam was never built. `screenwiring.go` says so in its
own comment ("standalone Marco does not have it"), and then production ships standalone Marco.

**This is the seam Roadmap 35 has to close, and it is not a UI problem.**

### 1.4 What the old system is worth

A lot, and more than the new system currently has:

- **Mature and irreplaceable**: the language (lexer/parser/compile/runtime/graph), the act-and-host
  FFI boundary, `driver.CheckSource` as the one compile gate, the recorder, `simplify`, `codegen`,
  the route registry with provenance, the authority seam, `internal/activate`'s ladder, the
  panic-stop, the bridge protocol.
- **Mature and undervalued**: `routes.Binding` — a complete, ~100-line, app-scoped, chainable
  phrase-and-hotkey registry that the new model has no answer for at all. And the **FOCUS scope**,
  which brings an app to the front from anywhere before acting.
- **Mature and duplicated at the product level**: teach-by-demonstration (`orchestrator.Teach`)
  versus Learn (`director teach`). Both work. Both write `.marco` into the same tree. The user sees
  two ways to create a command with no explanation of the difference.
- **Genuinely obsolete**: `internal/dispatch`, `marco assistant`, `routes/os.marco`, the empty
  `routes/<app>/` scaffolding, `plugins/ahk-overlay` (a pointer to another repository).
- **Dormant, not dead**: the CV/anchor stack (image, colour, OCR, Vision resolvers). `MARCO_CV`
  defaults **off** (`orchestrator.go` `cvOff`), so `internal/screen`, `plugins/ocr` and
  `plugins/vision`'s route-time roles are switched off in the shipped product while remaining fully
  built and tested.

### 1.5 First-party product feedback, and what it corroborates

The person who has actually used the product, verbatim:

> **Liked:** foregrounding the target app · easy training.
> **Disliked:** no screen recognition · poor UI · weird UX.

This is not a separate opinion to be balanced against the audit. It is the lived form of it, and
it changes two priorities.

| feedback | what it is, in code | audit consequence |
|---|---|---|
| **liked — foregrounding the target app** | the **FOCUS scope** (`routes/<app>/focus/`, `Registry.Resolve` steps 2–3) plus `OS's Activate` / `Launch` / `Restore` and `winctx`. "This app's command, runnable from anywhere, which switches to it first." | **Promote from infrastructure to first-class product.** It is the single most-liked behaviour in the legacy system and the new model has **no equivalent** — Director/Theater have no concept of *bring the stage forward before performing*. Do not lose it in a scope simplification. See §10 and §19 |
| **liked — easy training** | `marco teach "<name>"` in the HUD: name it, demonstrate, press the leader, answer y/n/s and c/f/g with single keys (`controller_windows.go` `teachAsk`). No console window, no ids, no copy-paste | **Confirms Record mode must stay a first-class Learn mode**, not be absorbed into or hidden behind semantic Learn. It is also the UX bar the Director's Learn flow has to meet — Learn currently asks for a name, a place name, questions, a rehearsal grant and then silently fails to register |
| **disliked — no screen recognition** | breaks #2 and #3 above (the Screen host cannot see and reads the wrong store), **plus** `MARCO_CV` defaulting off, which disables the anchor/OCR/vision resolvers that made a taught route survive a moved control | **This is the headline finding, independently observed.** It raises Phase 0 from "correctness cleanup" to *the* product blocker, and it says the CV default is a product decision that has been left as an engineering flag |
| **disliked — poor UI** | Roadmap 35's own premise; the Edit tab (a step table over generated source) is the landing view of the control centre | Confirms §20: the step editor is an Advanced surface that is currently the front door |
| **disliked — weird UX** | the typed/spoken split (`acts.go:211`); five Director panels reachable only by typing an undocumented word; three unconnected Stop intakes; two unrelated ways to create a command; a success message that names a tab the play is not in | Confirms §13, §14 and §22 Phases 2–4. "Weird" is the accumulated cost of every duplicated intake in the matrix in §4 |

**Net effect on the plan:** Phase 0 and Phase 2 move ahead of everything cosmetic, the FOCUS scope
is added to the unified model as a named product capability rather than a resolver detail, and
"screen recognition" gets an explicit two-part answer — semantic recognition (fix the seam) and
visual recognition (decide the CV default deliberately, or retire it).

---

## 2. Legacy architecture map

### 2.1 The product as it ships

```
+- overlay.cmd -------------------------------------------------------------+
| voice.exe --model ... --wake "marco"                                       |
|   | stdin                                                                  |
| marco.exe serve --host OS=bridge:marco-macros.exe                          |
|                 --host Overlay=bridge:overlay.exe  overlay.marco           |
+---------------------------------------------------------------------------+
        |                                   |
        | Overlay act                       | OS act
        v                                   v
   overlay.exe (ebiten HUD)            marco-macros.exe (SendInput)
        |
        | shells out -- the CLI seam every front-end uses
        +- marco routes --json --------> internal/routes.Registry.List
        +- marco active --------------> internal/winctx.Active
        +- marco args "<phrase>" -----> internal/routes.ParseInvocation
        +- marco hotkey <key> --------> routes/bindings.json -> chain -> do
        +- marco do "<name>" ---------> Registry.Resolve -> Classify -> Authorize -> Run
        +- marco teach "<name>" ------> recorder -> simplify -> codegen -> Registry.Save
        +- marco ui [view] -----------> cmd/marco/edit.go (local HTTP control centre)
        +- marco director "<phrase>" -> Director service (spoken phrases only)
        +- marco director watch --json -> playbill (Watch / Diagnostics panels)
```

`marco.exe` is spawned fresh per command; `overlay.exe` is long-lived and must be restarted to pick
up a rebuild. `director.exe` is **not started by the launcher** — only `marco director "<phrase>"`
auto-starts it (`cmd/marco/director.go:155` `directorConnect(autoStart)`), and every other Director
read passes `autoStart=false` and renders "the Director service is not running".

### 2.2 The legacy concept inventory

| concept | where it lives | what it actually is |
|---|---|---|
| **route** | `internal/routes` | a `.marco` file in a scoped directory, plus optional `.rec.json`, `.origin.json` and `-anchor-*.png` sidecars |
| **route registry** | `routes/registry.go` | a directory tree; no index, no database |
| **route discovery** | `Registry.List` / `Slugs` | a scan of `global/`, `<app>/`, `<app>/context/`, `<app>/focus/`. **Not** `<app>/learned/` |
| **route scope** | `Route{App, Focus}` | GLOBAL (anywhere) · CONTEXT (only in app) · **FOCUS (anywhere, switches to the app first)** |
| **route matching** | `Registry.Resolve` + `internal/nlu.Resolve` | slug equality with a four-step scope priority; fuzzy match only in the assistant/dispatch paths |
| **route args** | `routes/args.go` | `{{name}}`/`{{1}}` placeholders; `name:value` and `... with a, b` invocation forms; `" then "` chaining |
| **bindings** | `routes/bindings.go` + `routes/bindings.json` | `{app, key, cmd}`; app-scoped with a global fallback; `cmd` may be a `then`-chain |
| **provenance** | `routes/origin.go` | `Kind{authored,taught,learned}`, digest, `From`/`To`/`Sequence`/`Evidence`, five `OriginState`s |
| **staging** | `routes/origin.go` `LearnedDir` | `<app>/learned/` — saved and structurally undiscoverable |
| **authority** | `internal/orchestrator/authority.go` | `Resolved` → `Authorize` → `Decision`; learned plays need confirmation |
| **teach (record)** | `orchestrator.Teach` | recorder → `simplify` → `macroir.Step[]` → `codegen` → `.marco` + `.rec.json` |
| **teach (narrate)** | `internal/voiceteach` | spoken/typed phrases → the same `macroir` pipeline |
| **simplify** | `internal/simplify` | event stream → clean steps: waits, key coalescing, loop folding, drag detection, arg keys |
| **codegen** | `internal/codegen` | `macroir.Step[]` → Marco on the `OS` act, with anchors/find gates |
| **anchors / CV** | `internal/screen`, `plugins/ocr`, `plugins/vision` | image · colour · edge · OCR · learned-detector resolvers for a moved click. **Off by default** |
| **cancellation** | `cmd/marco/panicstop.go`, `runtime.cancelTree` | global stop key → ctx cancel → frame-tree cancel, `finally` runs |
| **hosts / plugins** | `internal/oshost`, `internal/bridgehost`, `spec/Hosts.md` | an act is fulfilled by a process over JSON stdio, bindable per act |
| **overlay** | `plugins/overlay` | ebiten HUD; Marco-brained (`overlay.marco`), Go MVC, global hooks |
| **control centre** | `cmd/marco/edit.go` (~2 400 lines) | local HTTP app: Edit · Learn · Routes · Bindings · Config · Help |
| **secrets** | `internal/secrets`, `do OS's Secret` | credential store; never text-substituted into source |
| **dispatch** | `internal/dispatch` + `internal/nlu` | run/teach/chat/clarify decision with a trust-but-verify Advisor |
| **assistant** | `cmd/marco/assistant.go` `runAssistant` | interactive REPL over the same |
| **game packs** | `internal/gamepacks/palworld` | per-application entities/detection/safety, consumed by Director |
| **skills / hats** | — | **do not exist.** No occurrence anywhere in the repository |

### 2.3 What "route" conflates

`route` currently means **six** things depending on the sentence:

| sense | mechanism | is it really a route? |
|---|---|---|
| the user's phrase | `Registry.Resolve(app, name)` via `Slug` | **no** — this is a binding from words to a play |
| a reusable behaviour | the `.marco` file | **yes** — this is a Play |
| a graph path | `Origin.From`/`To`, `director reach` edges | **no** — this is a Director/Repertoire edge |
| compiled Marco | `driver.CheckSource` output | **no** — this is the language artifact |
| a hotkey binding | `routes.Binding` | **no** — this is a trigger |
| an action sequence | `macroir.Step[]` | **no** — this is an intermediate representation |

Three of those six are distinct product nouns (**Play**, **Binding**, **edge**). The other three
are implementation layers that should never have been called a route.

---

## 3. Current architecture map

```
Audience --asks / demonstrates / names / corrects / stops
   |
   v
Director  internal/director + cmd/director   (~83k LOC non-test)
   |  intent · goals · plan · reach · teach (Learn) · questions · authority minting
   |  hypotheses · proposals · perception -> fusion -> world state
   |  semanticmemory -- Repertoire: places, targets, relationships, goals, who said so
   v
Theater   internal/production (the port) + internal/platform/theaterhost (the adapter)
   |  Repertoire (durable) · Stage (live) · Casting · Production · Verification
   |  Actors: AccessibilityActor today; a play names none
   v
marcorunner --> legal Marco source --> internal/compile + internal/runtime
   |
   v
act host map: OS · Screen · Accessibility · Text · Vision · Theater · Overlay
   |
   v
oshost (SendInput) · bridgehost (JSON stdio subprocess)
```

**34E's phases have largely landed.** Verified in this pass:

- `internal/activate` now holds **one** activation ladder, consumed by both
  `theaterhost/accessibility.go` and the rehearsal path. 34E's "two presses, one broken" is fixed.
- `theaterhost.Perform` takes a caller-supplied `production.Verifier` (ADR-070), so the nil-verifier
  hole is closed by contract rather than by wiring.
- `rehearse.Live.WithTheater` is called from `cmd/director/rehearserun.go:123`. Rehearsal *is* a
  Theater production now. **Theater is no longer a zombie.**

What 34E left open and this audit confirms is still open: "where are we" is still answered by
several `observe.PlaceNow` call sites rather than one `Stage.Now()`.

---

## 4. Duplication matrix

| row | OLD MARCO | DIRECTOR | THEATER | verdict → future owner |
|---|---|---|---|---|
| **phrase resolution** | `nlu.Resolve` (fuzzy, offline) + `Registry.Resolve` (scoped slug) + `dispatch.Decide` (+Advisor) | intent/clarify/goal resolution against what is on screen | — | **TRUE DUPLICATION at the entry point, different responsibility underneath.** Deterministic name lookup and semantic intent inference are both needed; three deciders are not. → one Audience intake |
| **route planning** | none — a route is a fixed script | `plan`, `reach.go`, goal procedures over verified edges | — | DIFFERENT RESPONSIBILITY → Director |
| **saved behaviour** | `routes.Registry` + `origin.go` | `marcoexec/play.go` generates *into* that registry | Repertoire holds the places/targets a play names | DIFFERENT RESPONSIBILITY, correctly layered → keep all three |
| **execution** | `orchestrator.Run` → `driver` → `runtime` | `rehearse` → `marcorunner` → same `runtime` | `theaterhost.run` → `marcorunner` → same `runtime` | **CONVERGED.** One runtime, three callers. Correct |
| **host calls** | `oshost` / `bridgehost` act map | `platform/*client` over the same map | casts Actors that call the same acts | SINGLE OWNER already → keep |
| **target resolution** | recorded coordinates + `Find` anchors (image/colour/edge/OCR/vision) | `production.Target` handed to Theater | `theaterhost.Find` → Candidate → `activate` ladder | **TRUE DUPLICATION of purpose, different maturity.** Anchors are a taught-route mechanism and are off by default → Theater owns semantic targets; anchors stay an implementation detail of taught routes |
| **screen recognition** | `internal/screen` template/colour match as a wait *gate* | `perception/fusion` + `observe.CompareStructure` | consults the matcher | DIFFERENT RESPONSIBILITY (pixel gate vs place identity) → Theater/Stage owns identity; the gate stays inside codegen output. **Both are currently off or unreachable — see §1.5** |
| **current place** | `winctx.Active()` — the foreground **app name** only | `observe.PlaceNow`, called from ~7 sites (34E) | should be `Stage.Now()` | 34E open item. App name is a strictly weaker, legitimately separate fact → Theater |
| **action verification** | none — a route reports ok when the last host call returns | `rehearse.classifyOutcome` | `production.Verifier`, caller-supplied | **RESOLVED by ADR-070.** The old side has no verification and should acquire it via the Screen arrival check already generated into learned plays |
| **replay** | `.rec.json` → re-`simplify` → re-`codegen` | `actiongraph` re-lowering, `shadowreplay` diagnosis | — | DIFFERENT RESPONSIBILITY → keep both |
| **registration** | `Registry.Register` (a directory move) | calls it, from one CLI flag | — | SINGLE OWNER, **no product surface** → add the verb, keep the mechanism |
| **bindings** | `routes.Binding` — app-scoped hotkey/phrase → command chain | none | none | **NOT DUPLICATED — a gap on the new side.** Note the name collision: `internal/director/binding` is deictic reference resolution ("this file"), an unrelated concept → Audience invocation registry |
| **foregrounding the target app** | **FOCUS scope** + `OS's Activate`/`Launch`/`Restore` + `winctx` | none | none | **NOT DUPLICATED — a gap on the new side, and the most-liked behaviour in the product (§1.5)** → Audience/Theater |
| **plugins / capabilities** | bridge hosts (fulfil an act) · resolver plugins (`$MARCO_RESOLVER`) · UI plugins (drive the CLI) | game packs (`internal/gamepacks`) · providers | Actors + Casting | **THREE-AND-A-HALF extension mechanisms, all real, none named consistently.** → see §9 |
| **cancellation** | stop key → ctx cancel → `runtime.cancelTree`; overlay Esc/leader → `cancelRun`; `MARCO_NO_PANIC_STOP` hand-off | `service.Registry.Cancel` / `Server.cancelActive`; spoken "stop" bypasses the planner (`clarify.go:136`); `teach.Finish` vs `Cancel` (ADR-066) | consumes ctx; `Authority.Claim` spends once | **TRUE DUPLICATION of the product event.** Three independent "stop" intakes that do not know about each other → one chain, §14 |
| **history** | overlay `addHistory` (cmd, outcome, duration, ~1 min TTL) | `actiongraph` + `director history`/`last` + playbill `Recent` moments | — | **TRUE DUPLICATION at the product level** → one Activity |
| **naming** | `routes.Slug` + `marco rename` | play name derivation (ADR-061) · screen naming (ADR-031/069) · target names | Repertoire `Called` | DIFFERENT RESPONSIBILITY, but the product exposes two unrelated naming systems → keep both, label them |
| **learn / teach** | `orchestrator.Teach` (record → simplify → codegen → save), `voiceteach` narration | `teach` coordinator → observesession → rehearse → lower → save | performs the rehearsal | **TRUE DUPLICATION at the product level, both valuable.** Two ways to create a command, same output tree, different `Kind` → one **Learn** with two modes |
| **diagnostics** | `marco check/test/contracts`, `marco diag`, HUD log | ~60 `director` subcommands, playbill Debug, shadow tracing | refusal vocabulary | DIFFERENT RESPONSIBILITY, no single Advanced surface → one Advanced section |
| **UI state** | overlay `model.go` (MVC) · `edit.go` page state | `pkg/playbill` — "the one account" | — | **LEGACY SHIM.** The overlay's Watch panel already renders the playbill; its own model is a second store of overlapping facts → playbill is the account, UIs render it |

---

## 5. Salvage matrix

Maturity: **A** = load-bearing and well tested · **B** = works, thinly tested · **C** = built, unproven.
Migration cost: **L/M/H**.

| component | mat. | still used | dup. | new owner | salvage value | cost | recommendation |
|---|---|---|---|---|---|---|---|
| `internal/lexer` `parser` `ast` `token` `compile` `graph` `runtime` | A | yes | no | Marco (language) | irreplaceable | — | **HIGH VALUE / KEEP** |
| `internal/driver` (`CheckSource`, `RunFile`, `Serve`) | A | yes | no | Marco | the one compile gate; `learnedplay.go` depends on it | — | **HIGH VALUE / KEEP** |
| `internal/oshost` · `bridgehost` · `spec/Hosts.md` | A | yes | no | Theater (as the actor substrate) | the whole provider-neutrality story rests on it | — | **HIGH VALUE / KEEP** |
| `internal/routes` registry + scopes | A | yes | no | Play persistence | already models Plays | L | **HIGH VALUE / KEEP** (rename the *word*, not the code) |
| **FOCUS scope + `OS's Activate`/`Launch`/`Restore` + `winctx`** | A | yes | no | Audience / Theater | **the most-liked behaviour in the product (§1.5); no equivalent on the new side** | L | **KEEP AS FIRST-CLASS PRODUCT** |
| `internal/routes/origin.go` (Kind, digest, staging, Register) | A | yes | no | Play lifecycle | this *is* saved/registered/invokable | L | **HIGH VALUE / KEEP** — needs a product verb, not a rewrite |
| `internal/routes/bindings.go` | A | yes | no | Audience invocation registry | complete phrase/hotkey→behaviour layer the new model lacks | L | **HIGH VALUE / KEEP**, promote |
| `internal/routes/args.go` (placeholders, `then`, `name:value`) | A | yes | no | Audience invocation | parameterised Plays come free | L | **HIGH VALUE / KEEP** |
| `internal/orchestrator/authority.go` | A | yes | no | Director/Audience | ADR-029's door; the only consent gate on `marco do` | — | **HIGH VALUE / KEEP** |
| `internal/orchestrator` teach loop (record/simplify/save) | A | yes | **yes** (vs Learn) | Learn, mode "record" | **"easy training" (§1.5)** — deterministic capture no perception can match | M | **ADAPT** — present as one Learn mode, keep the single-key UX |
| `internal/recorder` | A | yes | no | Learn / demonstration capture | hooks, stop key, secret placeholders | — | **HIGH VALUE / KEEP** |
| `internal/simplify` | A | yes | no | Learn (record mode) | waits, loops, drags, arg keys — hard-won | — | **HIGH VALUE / KEEP** |
| `internal/codegen` | A | yes | partial | Learn (record mode) | the taught-route lowerer | — | **KEEP** |
| `internal/macroir` | A | yes | no | infrastructure | the recorded-demo IR | — | **KEEP AS INFRASTRUCTURE** |
| `internal/voiceteach` | B | yes (overlay `narrate teach`) | **yes** (vs Learn) | Learn, mode "narrate" | narration is a real third mode | M | **ADAPT / KEEP BUT HIDE** until Learn absorbs it |
| `cmd/marco/panicstop.go` + `runtime.cancelTree` | A | yes | **yes** (three stop intakes) | one cancellation chain | mature, `finally`-correct | M | **ADAPT** — keep the mechanism, unify the intake |
| `internal/secrets` + `do OS's Secret` | A | yes | no | infrastructure | a load-bearing invariant | — | **HIGH VALUE / KEEP** |
| `internal/winctx` | A | yes | no | Stage (weak form) | foreground app identity | — | **KEEP AS INFRASTRUCTURE** |
| `plugins/overlay` (HUD, MVC, hooks, `overlay.marco`) | A | yes | partial (model vs playbill) | Audience ambient surface | the only always-on surface Marco has | M | **HIGH VALUE / KEEP**, re-point its data |
| `cmd/marco/edit.go` control centre | B | yes | no | Audience full surface | Learn already lives here | M | **HIGH VALUE / KEEP**, restructure sections |
| `edit.go` step editor ("Edit" tab) | B | yes | no | Advanced → Play source | real power feature, **wrong front door (§1.5 "poor UI")** | L | **KEEP BUT HIDE** |
| `internal/screen` + anchors + `plugins/ocr` + `plugins/vision` (route-time) | B | **off by default** (`MARCO_CV`) | partial (vs Theater targets) | taught-route implementation detail | real work, currently switched off — **and its absence is felt (§1.5)** | — | **ADAPT** — decide the default deliberately; do not leave it an engineering flag |
| `plugins/vision` (Director-time detector) | B | yes | no | Perception provider | the semantic detector | — | **KEEP** |
| `internal/dispatch` + `dispatch/llm.go` | B | **CLI only** | **yes** (vs Director) | — | the trust-but-verify pattern is worth keeping as a pattern | L | **DEPRECATE** — see §13 |
| `internal/nlu` | A | yes (Resolve); Advisor path unused | partial | Audience invocation (fuzzy match) | deterministic, offline, tiny | L | **KEEP AS INFRASTRUCTURE** — the matcher survives, the Advisor does not |
| `internal/resolver` (`$MARCO_RESOLVER`) | B | assistant only | yes | — | superseded by Director | L | **DEPRECATE** |
| `cmd/marco assistant` REPL | B | **no launcher** | yes | — | a dev REPL | L | **KEEP AS DEVELOPER SURFACE**, remove from `usage` |
| `plugins/claude-resolver`, `plugins/llama` | B | only via the above | yes | — | resolver-plugin protocol proof | L | **COMPATIBILITY ONLY** |
| `plugins/web-ui` | B | no launcher; README is stale | **yes** (vs overlay Watch) | — | its Sight/Knows/Answer/Stop panels are *product prototypes* | M | **ADAPT** — harvest the surfaces, then retire the plugin |
| `plugins/ahk-overlay` | — | code lives in another repo | yes | — | documentation only | L | **DELETE LATER** (keep the README as a note) |
| `plugins/marco-app` + `pack.ps1` | B | yes (distribution) | no | packaging | single-file Windows bundle | — | **KEEP AS INFRASTRUCTURE** |
| `cmd/marco-macros` | A | yes (`OS` bridge) | no | Actor/provider process | separable input layer | — | **KEEP** |
| `plugins/uia` (C#) | A | yes (Director), **unwired for `marco`** | no | Actor (Accessibility) | the only Actor that exists | L | **HIGH VALUE / KEEP** — wire it |
| `plugins/voice` | B | yes | no | Audience input | wake word + Vosk | — | **KEEP** |
| `routes/os.marco` | — | **dead copy** | **yes** — byte-identical to `internal/osmod/os.marco` | — | none | L | **DELETE LATER** (stop shipping; never delete a user's copy) |
| empty `routes/<app>/...` scaffolding | — | no | no | — | none | L | **DELETE LATER** |
| `internal/spectest` | A | yes | no | governance | holds the language rule | — | **HIGH VALUE / KEEP** |
| `cmd/docscheck` + `internal/docsindex` | A | yes | no | developer tooling | vault integrity | — | **KEEP AS DEVELOPER SURFACE** |
| `cmd/{bakeoff,identityprobe,judgeprobe,nameprobe,twinprobe,variance}` | C | research | no | developer | probes | — | **KEEP AS DEVELOPER SURFACE** |
| `bridges/echo-bridge`, `bridges/*.ahk` | B | examples | no | docs | protocol examples | L | **KEEP AS DEVELOPER SURFACE** |

---

## 6. Dead / zombie systems

Present, compiling, tested, sometimes documented — and off the production path.

| system | why it looks alive | why it is not | verdict |
|---|---|---|---|
| **`internal/dispatch` + `marco dispatch`** | documented in `marco help`, a whole package with a designed Advisor interface, referenced by `plugins/llama` | nothing in the shipped stack calls it. The overlay's typed path goes straight to `marco do`; the spoken path goes to `marco director`. `grep -rn internal/dispatch` returns 4 hits, all CLI/doc | **ZOMBIE** — deprecate the verb, keep `nlu` |
| **`marco assistant`** | a full interactive REPL in `usage` | nothing launches it; the overlay never shells it | **ZOMBIE (dev surface)** |
| **`$MARCO_RESOLVER` / `$MARCO_ASSISTANT` plugins** | two shipped plugins, `.exe`s built, documented | reachable only from `assistant`/`dispatch` | **ZOMBIE** |
| **`plugins/web-ui`** | builds, two `.exe`s at repo root, has a README | no launcher references it; its README describes the *routes* seam while `playbill.go`/`sight.go`/`knows.go`/`answer.go` implement Director surfaces the overlay does not have | **ZOMBIE with live cargo** — see §7 |
| **`plugins/ahk-overlay`** | a plugin directory with a README | contains no code; the implementation is in `D:\Macros\MacroMarco` | **DEAD POINTER** |
| **The CV / anchor stack at route time** | `internal/screen`, `plugins/ocr`, `plugins/vision`, `codegen` anchor emission, `MARCO_FIND_*` dials, all tested | `cvOff()` defaults **on**; `orchestrator` then disables wait caps and preserves raw timings. Anchors are not produced and `Find` gates are not emitted in the default configuration | **DORMANT, deliberately** — but its absence is the "no screen recognition" complaint (§1.5), so the default is now a product decision, not a flag |
| **`routes/os.marco`** | sits in the user's routes tree, looks authoritative | byte-identical to `internal/osmod/os.marco`, which the resolver actually uses (`driver.builtinModule`) | **DEAD COPY** |
| **`routes/{discord,nms,notepad,rocketleague,testapp}/...`** | a populated-looking tree | every directory is empty; only `bindings.json` and `os.marco` are real files | **DEAD SCAFFOLD** — note that `bindings.json` still binds `` `e `` → "enter freeplay" for `rocketleague`, a route that does not exist |
| **`Teaching.Registered` / `LearnedSaved.Registered`** | plumbed through protocol, playbill, teach tail, CLI rendering | always false: no caller sets `Register: true` except one CLI flag | **UNREACHABLE FIELD** — the diagnostic of break #1 |
| **`internal/gamepacks/palworld`** | wired via `cmd/director/gamewiring.go` | reachable only from `director game`; no product surface | **ALIVE BUT UNSURFACED** (not a zombie — see §7) |

---

## 7. New systems lacking product representation

Real, tested, on the Director's live path — and invisible to a user.

| system | where | current surface | should be |
|---|---|---|---|
| **Plays** (the noun) | `routes.Kind` + `origin.go` | none. The Routes tab shows files; it does not show kind, provenance, registered-ness, or staged plays | **first-class: Plays** |
| **Register** | `Registry.Register` | `director learned --register --name X` | **a button in Learn and a row action in Plays** |
| **Repertoire / "what Marco knows"** | `semanticmemory.Store`, `observe/theater.go` `TargetStore`, `director knows` | `plugins/web-ui/knows.go` only (a zombie plugin) | **"Things Marco knows"**, per Roadmap 35 |
| **Places** (durable, named) | `semanticmemory` subjects, ADR-047/069/076 | partially in the Learn tab's HERE bar; `director knows` | **Here** |
| **Targets** | `observe.SubjectTarget`, ADR-068 | none | **Here** (what can be acted on) + Advanced |
| **Stage** | 34E's proposed `Stage.Now()`; today `PlaceNow` + playbill `Current`/`Seeing`/`Offers` | overlay `watch` panel | **Here** |
| **Verified edges / reach** | `director reach`, `plan`, ADR-056 | CLI only | **Plays → "what Marco can get to"** |
| **Goals** | `internal/director/goal`, 18 procedures | `director goals`/`goal` | Advanced (and implicitly behind Ask) |
| **Learn episode** | `teach` coordinator, ADR-075 | the Learn tab (good) — but no history, and no Try-then-Register close-out | **Learn** + **Activity** |
| **Sight / Show-me (pointing)** | `cmd/director/pointing.go`, `sightplace.go`, `marco-show.exe` | `plugins/web-ui` and `director sight`; **not in the overlay** | **Here** |
| **Questions Marco is asking** | `observeregistry` proposals; `/api/learn/answer` | Learn tab only; playbill `Question` | **Here / Ask**, ambiently |
| **Game packs** | `internal/gamepacks` | `director game`, `marco games` | Advanced |
| **Theater actor availability** | `theaterhost` casting, `no_actor_available` | none — which is exactly why break #4 went unnoticed | **Advanced → Actors** |
| **Provenance / integrity** | `OriginState{authored,intact,edited,orphaned,unreadable}` | none | **Plays** (a badge) + Advanced |

The pattern: **everything the Director learned has a CLI and no UI; everything the old product
does has a UI and no semantics.** That is the mechanical cause of "weird UX" in §1.5.

---

## 8. Route vs Play — the decision

### The question restated

Given Play = durable executable behaviour in legal Marco; Director reasons over goals and edges;
Theater performs — what is left for Route?

### The evidence

1. `internal/routes` is **not** a competing concept. It is a filesystem-backed store whose entity
   already carries `Kind{authored, taught, learned}`, a provenance sidecar, an integrity digest, a
   staging area and a registration operation. That is a Play repository.
2. What the *word* "route" additionally names in the code — the phrase, the scope priority, the
   hotkey, the `then`-chain — are **triggers**, not the behaviour.
3. `Origin.From`/`To` is a Repertoire edge, not a route.
4. There is no third thing.

### The recommendation

> **Option B, with A as the endpoint.**
>
> **A "route" today is two things wearing one word: a Play (the `.marco` artifact plus its
> provenance) and a Binding (the phrase/scope/hotkey that reaches it). Split the *semantics* now,
> retire the *word* from the product, and keep the package.**

| today | becomes | lives where |
|---|---|---|
| `routes.Route{App, Focus, Slug}` + the `.marco` + `.origin.json` | **Play** | `internal/routes` (unchanged) |
| `Registry.Resolve(app, name)` scope priority | **Binding** (implicit, name-derived) | `internal/routes` (unchanged) |
| `routes.Binding{App, Key, Cmd}` | **Binding** (explicit, hotkey) | `internal/routes` (unchanged) |
| `Route.Focus` | **the Play's stage requirement** — bring this app forward first | `internal/routes` (unchanged) |
| `Origin.From`/`To` | **edge** (Repertoire) | `semanticmemory` |
| `macroir.Step[]`, `.rec.json` | **demonstration** | `internal/macroir` |

**Not option C.** A Director plan over edges (`director reach`) is a *plan*, not a Play: it is
recomputed on every read, it holds no authority, and ADR-056 already says a goal is a destination
and not a route. Calling that a Route would resurrect exactly the confusion being removed.

**Not option D.** "Compatibility only" would strand the provenance and staging machinery, which is
the best-designed part of the old system and the only implementation of saved≠registered.

**Not option A yet.** Play cannot *replace* Route until the four breaks in §1.2 are closed;
until then "Play" would name something that does not run.

### Explicitly: do not rename the Go package

`internal/routes` may keep its name indefinitely. The cost of the rename is a large diff across
`cmd/marco`, `cmd/director` and every test; the benefit is zero for a user who never sees a package
name. **Product vocabulary and code vocabulary are allowed to differ**, and this repository already
does it deliberately for Learn/`teach` (Glossary, ADR-048).

---

## 9. Plugin vs Actor — the relationship

### What a plugin can contain today

Auditing all eleven directories under `plugins/` plus `cmd/marco-macros`, a "plugin" is **four
unrelated things**:

| kind | contract | examples | maps to |
|---|---|---|---|
| **Act host** — fulfils a Marco act over JSON stdio | `spec/Hosts.md`; selected by `--host Act=bridge:<exe>` or an env var | `marco-macros` (OS), `overlay` (Overlay), `ocr` (Text), `vision` (Vision), `uia` (Accessibility) | **Provider / Actor substrate** |
| **Advisor / resolver** — answers a decision | `$MARCO_RESOLVER` / `$MARCO_ASSISTANT` stdio protocol | `claude-resolver`, `llama` | **superseded by Director** |
| **Front-end** — drives the CLI seam | `marco routes --json` · `marco do` · `marco active` | `web-ui`, `ahk-overlay`, and `overlay` again (it is *both*) | **Audience surface** |
| **Packaging** | embeds the stack | `marco-app` | **distribution** |

A plugin also *may* contain none of a route definition, a binding, a command registration or a
config schema. There is **no plugin manifest, no dependency declaration, no version and no
enable/disable** anywhere in the repository. Enablement is an environment variable per act, and
that is the entire mechanism.

Meanwhile there are **two other extension points that are not plugins**: `internal/gamepacks`
(compiled-in capability packs, `game.Registry.Register`) and `theaterhost.Actor` (a Go interface
with one implementation, cast at run time).

### The recommendation

**Do not rename Plugin → Actor.** They are at different levels and the relationship is
one-to-many-and-sideways:

```
Actor          a role Theater can CAST to perform a Target       (Go interface, in-process)
   | is backed by
Provider       a capability source                               (accessibility, OS input, sight)
   | may be delivered as
Act host       a process fulfilling one act over the bridge      ("plugin", today)
   | may be bundled with
Integration    an act host + a game pack + starter Plays         (does not exist yet)
```

So:

- `plugins/uia` is **one Actor's provider, delivered as an act host**.
- `cmd/marco-macros` is a **provider** (OS input) that no Actor currently wraps — Theater casts
  only `AccessibilityActor`.
- `plugins/overlay` is **an act host and a front-end in one process**. That is fine and should stay;
  it is why `overlay.marco` exists.
- `plugins/ocr` and `plugins/vision` are **perception providers** *and* route-time act hosts. Two
  jobs, one binary, and the route-time job is switched off by default.
- `internal/gamepacks/palworld` is a **capability package** with no plugin form. If integrations are
  ever shipped, this is the shape they take.

**Actionable consequence:** the reason break #4 (§1.2) was invisible is that there is no surface
anywhere that says *which Actors this machine has*. Adding "Actors" to an Advanced section is the
cheapest structural fix in this whole audit.

---

## 10. Bindings — the future

### What the old binding system already solves

| requirement | `routes.Binding` / `Registry.Resolve` | status |
|---|---|---|
| phrase → behaviour | `Resolve(app, name)` over slugs | **yes** |
| aliases | — | **no** (one slug, one file) |
| imported phrases | — | **no** |
| user-defined invocation language | `Bind(app, key, cmd)`; `cmd` is any command or `then`-chain | **yes** |
| conflict resolution | four-step scope priority: app-context → app-focus → any-app-focus (sorted, deterministic) → global | **yes, and well designed** |
| scopes | GLOBAL / CONTEXT / FOCUS | **yes** |
| application context | `winctx.Active()` drives both resolution and hotkey lookup | **yes** |
| **bring the app forward before acting** | the FOCUS scope, then `OS's Activate` / `Launch` | **yes — and it is the most-liked behaviour in the product (§1.5)** |
| enable / disable | — | **no** (delete the file) |
| parameters | `{{name}}` / `{{1}}`, `name:value`, `... with a, b` (`args.go`) | **yes** |

Seven of ten, in ~250 lines, with tests. The two-tier hotkey lookup (app-scoped, then a global
fallback) and the FOCUS scope are genuinely good product design that the Director model has nothing
equivalent to.

### Does it embed old assumptions?

**One, and it is small.** `Binding.Slug` is a legacy field ("a single resolved route slug") kept for
bindings saved before chaining. `Binding.Cmd` supersedes it and `command()` already prefers it. That
is the only route-shaped assumption in the file. `Registry.Resolve` is coupled to the directory
layout, which is the same layout Plays live in, so it is not a leak.

### Recommendation

> **KEEP AS FIRST-CLASS PRODUCT, promoted to the Audience invocation layer.**

Target shape — which is almost exactly what exists:

```
Audience phrase / hotkey / spoken utterance
        |
        v
Binding registry            deterministic, explicit, user-owned
  +- explicit: bindings.json  (hotkey -> command chain)
  +- implicit: the Play's own name, scoped  (Registry.Resolve)
        |  hit                                  miss
        v                                        v
      Play  --(FOCUS? bring the app forward)  Director
        |                                  (semantic intent)
        v
   Director (authority) -> Theater (perform)
```

The **seam to add** is not inside bindings; it is above them. Today a *typed* phrase never reaches
Director and a *spoken* phrase never reaches bindings. One intake, two tiers, is the fix (§13).

**Do not** give bindings an LLM. The deterministic tier is the reason Marco works offline, and
`dispatch`'s own package comment already argues this correctly.

---

## 11. Saved / registered / invokable — the path

### Traced end to end

```
Learn (director teach / the Learn panel)
  +- teach.Coordinator.save()                     internal/director/teach/teach.go:1232
      +- teachTail.Save(route, actor, verb)       cmd/director/teachtail.go:176
          +- LearnedQuery{Save:true}              <- Register is deliberately NOT set
              +- Runtime.lifecycle()              cmd/director/learnedplay.go:247
                  +- marcoexec.LowerActionsBetween(...)   regenerate, never string-replace
                  +- compileGenerated -> driver.CheckSource         THE compile gate
                  +- reg.SaveStaged(rt, src, Origin{Kind:learned}) -> routes/<app>/learned/
                        ^ writeAtomic source first, then provenance
  -- the user is told: "Saved as X. Nothing can ask for it yet -- register it when you want to."
  -- the Learn panel instead says: "Saved. It is in the Routes tab."   <- FALSE

Register            reg.Register(rt)              routes/origin.go
  +- refuses on slug collision (never overwrites an authored play)
  +- refuses unless StagedOrigin().Verified()
  +- SaveWithOrigin -> routes/<app>/context/  +  removes the staged pair
  -- reachable ONLY from `director learned --register --name X`

Resolve             Registry.Resolve(app, name)   scope priority
Authorize           orchestrator.Classify -> Authorize -> Decision      ADR-029
                    learned + intact provenance => NeedsConfirmation (AskFirst)
Run                 orchestrator.Deps.Run -> driver -> runtime -> act host map
                    +- do Screen's Showing "<start>"   -> screenhost -> CurrentSubject -> Unavailable
                    |                                     => ALWAYS REFUSES  (break #2)
                    +- do OS's Navigate ...            -> oshost, would work
                    +- do Theater's Activate ...       -> no actor unless $MARCO_UIA_BRIDGE (break #4)
                    +- do Screen's Showing "<end>"     -> same as the entry guard
```

### Which old systems a saved Play actually needs

Exactly what [[Learned-Plays]] promised, and the promise holds:

- the `.marco` file · the routes tree · `Registry.Resolve` · `orchestrator` authority · `driver` ·
  `runtime` · the act host map.
- **not** `ProcedureCandidate`, `RehearsalEvidence`, sessions, or `.origin.json` enforcement.

The old machinery is **not** the problem. It is the most salvageable thing in the repository.

### The clean seam to build

```
Learn --> Play (staged)  --> Register --> Binding --> Ask --> Director --> Theater --> Arrive
          file + origin      one verb,     phrase/     one     authority   cast +
          user can read      one button    hotkey      intake  + intent    verify
```

with **one addition that does not exist today**: the runtime executing a Play must be able to ask
*where am I* and get the Director's answer. Two ways, and the audit recommends the second:

- **(adapter, cheap)** point `marco`'s Screen host at `$MARCO_HOME/semantic-memory.json` and give
  `liveScreens.CurrentSubject` a client that asks the running Director service, falling back to
  `Unavailable` when it is absent. Preserves `marco` as a standalone binary that degrades honestly.
- **(seam, correct)** invoke registered Plays *through* the Director, so a Play runs inside the
  process that has the Stage. `marco do` becomes a client for learned Plays and stays local for
  authored/taught ones.

Both keep `Screen's Showing` refusing when nothing can see — which is the invariant that must not be
weakened. **No path may be added that answers `ok` without positively identifying the named place.**

---

## 12. Overlay audit

### 12.1 The HUD (`plugins/overlay`)

| surface | purpose | data source | triggers | legacy concept | new owner | verdict |
|---|---|---|---|---|---|---|
| leader (`` ` ``, configurable) | arm / stop / cancel | `controller_windows.go` hooks | `arm()` / `cancelRun()` | — | Audience | **KEEP** |
| `` `m `` command line | type a command | `model.input` | `Commands.Run` → `Overlay's Run` | route name | **Ask** | KEEP, retarget |
| `` `h `` / `help` | help + route list | `marco routes --json` | opens `marco ui help` | routes | **Plays + Help** | CHANGE |
| `` `<key> `` | run the bound route | `marco hotkey` → `bindings.json` | `do` chain | binding | **Binding → Play** | **KEEP** |
| typed `<name>` | run a route | `marco do "<name>"` | Resolve→Authorize→Run | route | **Play** | KEEP, rename |
| `teach <name>` | record a demo in place | `orchestrator.Teach` via `runTeachInteractive` | recorder/simplify/codegen | taught route | **Learn (record mode)** | **KEEP — this is "easy training" (§1.5)** |
| `narrate teach <name>` / `voice teach` | narration-driven teach | `voiceteach` | codegen | taught route | **Learn (narrate mode)** | KEEP BUT HIDE |
| `ui` / `edit <route>` | open the control centre | `marco ui [view]` | detached browser | route editor | **the full surface** | KEEP |
| `config` panel | live overlay settings | `overlay.json` | `configChange`/`saveConfig` | overlay config | **Settings** | KEEP |
| `voice on/off`, `mute`, `listen` | mic gate | `overlay.json` `Voice` | `setVoice` | — | **Settings** | KEEP |
| `watch` / `director` / `insight` | what Marco sees/believes/needs | `marco director watch --json` → `pkg/playbill` | read only | — (new) | **Here** | KEEP, **promote** |
| `diagnostics` / `inspector` | evidence under Watch; captures the mouse | `marco director diagnose` | read | — (new) | **Advanced** | KEEP, hide |
| `perception` / `explain` | frozen per-element snapshot | `marco director perception --json` | read | — (new) | **Advanced** | KEEP, hide |
| spoken phrase (final) | semantic desktop control | `marco director "<phrase>"` (`acts.go:211`) | Director dispatch | — (new) | **Ask** | KEEP, **unify with typed** |
| live voice preview (`Heard`) | interim transcript | `Voice` feed | display | — | Ask | KEEP |
| command history rows | cmd · outcome · duration, ~1 min TTL | `model.addHistory` | — | route runs | **Activity** | CHANGE — merge with the action graph |
| HUD log | streamed child stdout | `streamChild` | — | — | Activity | KEEP |
| `bind` / `unbind` | manage hotkeys | `routes/bindings.json` | — | binding | **Settings → Bindings** | KEEP |
| `forget` / `rename` / `simplify` | route lifecycle | registry | — | route | **Plays** | KEEP, move |
| `press <key>` | one-shot key | `marco press` | oshost | — | Advanced | KEEP |
| `exit` / `quit` | close the stack | — | `requestQuit` | — | — | KEEP |
| Esc | cancel typing / stop a run / release panels | `cancelRun`, `closeWatch` | — | stop | **Stop** | KEEP, **unify** (§14) |
| clock · state · metrics · coords · crown | HUD chrome | `model` | — | — | — | KEEP |

**Three observations, and they are where "weird UX" comes from.**

1. The HUD's five *newest* surfaces (watch, diagnostics, perception, spoken dispatch, heard) are
   reachable **only by typing their name into the command line**. There is no affordance, no leader
   key and no listing in `helpLines()`. They are effectively undiscoverable.
2. `helpLines()` renders the three route scopes as three groups — the clearest explanation of scope
   anywhere in the product — and it lives in a HUD panel that the `help` command no longer opens
   (commit `cc7466e` re-pointed `help` at the browser).
3. The overlay maintains a private `model` of status/history/logs *and* renders `playbill.View`
   from the Director. Two accounts of "what is happening", in one window.

### 12.2 The control centre (`marco ui`, `cmd/marco/edit.go`)

| tab | purpose | data source | legacy concept | new owner | verdict |
|---|---|---|---|---|---|
| **Edit** (default) | per-step editor over one Play's generated Marco + raw source | `/api/route`, `/api/save` | route steps | **Advanced → Play source** | KEEP BUT HIDE (stop making it the landing tab) |
| **Learn** | the Director learn lifecycle: start/stop/try/cancel, HERE bar, naming, questions, watch/remember | `/api/learn*` → `service.ObserveLearn` | — (new) | **Learn** | **KEEP — this is the product** |
| **Routes** | list · run · change scope · delete | `/api/routes`, `/api/do`, `/api/scope`, `/api/delete` | routes | **Plays** | CHANGE — must show kind, provenance, registered, and **staged** plays |
| **Bindings** | add/remove leader hotkeys | `/api/bindings`, `/api/bind`, `/api/unbind` | bindings | **Settings → Bindings** | KEEP |
| **Config** | mirrors `overlay.json` (leader, wake phrase, voice, theme, HUD placement) | `/api/oconfig` | overlay config | **Settings** | KEEP |
| **Help** | written guide | static HTML | — | **Help** | KEEP, update |

The Learn tab is already correct architecture: `learnui.go` holds **no state**, forwards intent to
the Director's coordinator and renders whatever comes back. It is the model every other surface
should follow. Its two gaps are product, not structural: **no Register action**, and a **false
success message**.

### 12.3 The third front-end (`plugins/web-ui`)

Sight · Normal · Watch · Debug · Stop · Knows · Correct · Answer · Name · Show-me · Point.
These are Roadmap 35's product concepts, built, working, and living in an unlaunched plugin whose
README describes the route seam. **Harvest, then retire.**

---

## 13. Command resolution — the duplication

### Three deciders

| | input | knowledge | output |
|---|---|---|---|
| `Registry.Resolve` | a slug + the foreground app | the routes tree | a `Route`, or nothing |
| `nlu.Resolve` | free text + slugs | normalisation, token overlap, edit distance | best match + score + exact flag |
| `dispatch.Decide` | free text + slugs + app + an optional Advisor | the above, plus an LLM, re-verified | run / teach / chat / clarify |
| Director | free text | the live screen, semantic memory, goals, hypotheses | an intent, a plan, a question, or a refusal |

`dispatch` was the pre-Director answer to "what does this sentence mean". Its package comment is
explicit that it is *not* the Director. Director now does strictly more, on strictly better
evidence — and `dispatch` reaches no product surface.

### The boundary to keep

```
Binding registry   deterministic, explicit, offline, user-owned
                   -- exact slug in scope, or an explicit hotkey --
        | miss
        v
Director           natural-language interpretation, goal inference,
                   clarification, refusal
```

**Do not remove the deterministic tier.** It is why Marco works with no model, and it is what makes
"the phrase I taught runs the thing I taught" a promise rather than a probability.

**Keep `nlu.Resolve`** — the fuzzy matcher is genuinely useful for "did you mean" and for hotkey
argument hints, and it has no dependencies.

**Deprecate `dispatch.Advisor` and the two resolver plugins.** A second LLM-shaped decider that
Director already supersedes is the definition of duplication.

**The real gap is above all of them.** `acts.go:211` routes spoken input to Director and typed input
to routes. That is one intake split by input device. It should be one intake split by *whether a
Binding matched* — and it is the single largest contributor to "weird UX" (§1.5).

---

## 14. Cancellation / stop

### Where it lives now

| layer | mechanism | scope |
|---|---|---|
| **global stop key** | `cmd/marco/panicstop.go` — a `recorder` hook, `ParseStopKey`, cancels the ctx | one `marco` invocation with a real host |
| **frame tree** | `runtime.cancelTree` — marks canceled, recurses, runs `finally`, releases held input | one Marco program |
| **host atomicity** | audited per capability in `docs/director-cancellation.md`: click/key/move are atomic, drag and type are cancellable with guaranteed release | one host call |
| **overlay** | Esc / leader while running → `cancelRun()`; sets `MARCO_NO_PANIC_STOP` on children so hooks do not duel | the HUD's child process |
| **Director service** | `Server.cancelActive` / `Registry.Cancel`; spoken "stop" is routed as `RouteCancel` and never reaches the planner (`clarify.go:136`) | the in-flight Director command |
| **Learn** | `Finish` vs `Cancel` are separate verbs, separate methods, separate flags — ADR-066 | one demonstration |
| **Theater** | consumes the ctx; `Authority.Claim` is spend-once | one production |

### The problem

There are **three product-level intakes for "stop"** — the overlay's Esc/leader, a spoken "stop",
and the web-ui's Stop button — and they cancel **different things**. Pressing Esc in the HUD does
not cancel a Director command; saying "stop" does not cancel a running route; neither cancels the
other's. A user has one word and no way to know which of three systems will hear it.

### Proposed ownership chain — one, and no second

```
Audience says / presses Stop
        |
        v
Audience intake (overlay Esc · spoken "stop" · UI button · the stop key)
        |  one call, always the same
        v
Director:  cancel the requested production
           +- a Learn episode?    -> Coordinator.Finish or Cancel  (ADR-066 -- never conflate)
           +- a Director command? -> cancelActive
           +- a running Play?     -> cancel its ctx
        |
        v
Theater:   propagate to the cast production; authority is already spent, never re-widened
        |
        v
Runtime:   cancelTree -> finally blocks -> held input released
        |
        v
Host:      atomic calls complete; no later step runs
```

**Preserve verbatim**: the atomicity audit, `finally` semantics, `ReleaseHeld`, the
`MARCO_NO_PANIC_STOP` hand-off (dueling low-level hooks are a real hazard — see the
`hook-pump-must-block` invariant), and ADR-066's Finish/Cancel separation.

**Add nothing new inside Theater.** Theater consumes cancellation exactly as it consumes authority.

---

## 15. Replay / record / teach legacy

### What predates Director

| piece | what it does | still valuable? |
|---|---|---|
| `internal/recorder` | global keyboard/mouse hooks, stop key, secret placeholders, anchor key | **yes — irreplaceable.** Injected input is unattributable by design, so demonstration capture *must* be a hook |
| `internal/macroir` | the recorded-demo IR; mirrors the original MacroMarco JSON | **yes** — serialisation + re-simplification depend on it |
| `internal/simplify` | events → clean steps: rounded waits, key coalescing, exact-cycle loop folding, drag detection, typing rhythm, arg keys | **yes — high value.** Nothing on the Director side folds loops or infers drags |
| `internal/codegen` | steps → Marco on the OS act, with anchor/find gates | **yes**, for taught routes |
| `.rec.json` sidecar | the raw demonstration, kept so a route can be **re-simplified later** — folded steps have lost per-keystroke detail | **yes — a genuinely good design** |
| `marco simplify "<name>"` | re-run simplification as far as it goes | yes |
| `internal/voiceteach` | narration → the same IR | yes, as a Learn mode |

### Against the Director side

| Director | old equivalent | relationship |
|---|---|---|
| `observesession` + `Passes` | `recorder` | **different**: one interprets meaning under attribution rules, the other captures exact input |
| `demo` / `Demonstrations` | `.rec.json` | different granularity; both durable |
| `marcoexec/play.go` | `codegen` | **parallel lowerers, different vocabularies.** Both emit legal Core Marco through `driver.CheckSource` |
| `actiongraph` | `.rec.json` replay | different: re-lowers a *semantic* action; the sidecar re-simplifies a *literal* one |
| `shadowreplay` | — | diagnosis only |

### Verdict

**Delete nothing.** The two pipelines are not redundant, they are two *modes of learning*:

- **Record** — exact, deterministic, coordinate-and-keystroke, works anywhere, understands nothing.
  *This is the "easy training" the user liked (§1.5).*
- **Learn** — semantic, place-aware, verified, portable, needs perception and a cooperative app.

A user should choose between them by describing their situation ("just copy exactly what I do" vs
"understand what I'm doing"), not by discovering two unrelated commands.

**Parameterisation is already solved on the old side and absent on the new**: `{{name}}` / `{{1}}`,
the F9 arg key, `name:value` invocation, and `then`-chaining are mature. A learned Play cannot take
an argument at all.

---

## 16. Import / export / sharing

**There is none.** No import verb, no export verb, no package format, no manifest, no dependency
declaration, no version field on anything a user owns, no signing, no trust model.

### What already exists that a transport format would be built from

| artifact | path | portable? |
|---|---|---|
| Play source | `routes/<scope>/<slug>.marco` | **yes** — text, and the whole point is that it is readable |
| provenance | `<slug>.origin.json` — `Version:1`, kind, app, from/to, sequence, evidence, **source digest**, saved-at | **yes** — already versioned and integrity-checked |
| demonstration | `<slug>.rec.json` | yes, but machine-specific (coordinates) |
| anchors | `<slug>-anchor-*.png` | machine-specific (resolution, theme) |
| bindings | `routes/bindings.json` | yes, but references slugs |
| semantic memory | `%AppData%\marco\semantic-memory.json` | **carries the meaning a learned Play depends on** |
| overlay config | `%AppData%\marco\overlay.json` | yes |
| act host wiring | environment variables | **not portable — this is the gap** |

### The substrate assessment

- **`origin.go` is already ~80 % of a package format.** It has a version, an integrity digest over
  the content, a kind, and a refusal for provenance that no longer describes its file. Extending it
  to a bundle is a small step; inventing a new format would be a mistake.
- **A learned Play is not self-contained.** It says `Screen's Showing with "the audio page"`, which
  resolves against *this machine's* semantic memory. Sharing one means sharing the places it names —
  which is exactly what a Repertoire export would be.
- **A taught Play with anchors is not portable at all**, and one with raw coordinates is portable
  only to an identical display. This is a real constraint on "share a Play with a coworker" and is
  worth knowing before it is promised.
- **Trust already has a vocabulary.** `Kind` and `OriginState` distinguish "somebody wrote this"
  from "Marco composed this and it still matches", and `orchestrator.Authorize` already asks before
  running a learned Play. An imported Play from a third party is a **third** trust class that does
  not exist yet, and `Verdict`/`Reason` are closed vocabularies designed to be extended.

**Recommendation:** map only. Do not implement sharing. When it is implemented, the bundle is
`{ .marco + .origin.json + the named places from semantic memory + optional bindings }`, and it
arrives as a fourth `Kind`.

---

## 17. Config / settings

### Inventory

| setting | where | maps to | verdict |
|---|---|---|---|
| leader key | `overlay.json` `Leader` | Audience input | **KEEP — normal Settings** |
| voice on/off, wake phrase | `overlay.json` `Voice`, `$MARCO_VOICE_WAKE` | Audience input | KEEP — normal |
| theme · idle opacity · monitor · corner · width · maxLines · border · mini · metrics · coords · font | `overlay.json` | HUD presentation | KEEP — normal |
| `$MARCO_ROUTES` | env | Play store location | KEEP — advanced |
| `$MARCO_HOME` | env | Director store (action graph + semantic memory) | KEEP — advanced. **Note it does not move the Play store, and `$MARCO_ROUTES` does not move the Director store** |
| `$MARCO_MEMORY` | env | `marco`'s Screen-host memory, default `routes/memory.json` | **STALE — this is break #3.** Should default to the Director's store |
| `$MARCO_STOP_KEY` | env | cancellation | KEEP — advanced |
| `$MARCO_ARG_KEY` | env | record-mode parameterisation | KEEP — advanced |
| `$MARCO_CV`, `$MARCO_ANCHORS`, `$MARCO_CV_SENSITIVITY`, `$MARCO_FIND_*` (10 vars), `$MARCO_EDGE_*`, `$MARCO_CLICK_RADIUS`, `$MARCO_ANCHOR_*` | env | the dormant anchor stack | **A PRODUCT DECISION, not a flag.** "No screen recognition" (§1.5) is partly this default. Decide it deliberately; hide the dials either way |
| `$MARCO_OCR`, `$MARCO_OCR_*` (6) | env | Text act host + OCR tuning | KEEP — advanced |
| `$MARCO_VISION`, `$MARCO_VISION_*` (6), `$MARCO_ONNXRUNTIME` | env | Vision provider | KEEP — advanced |
| `$MARCO_UIA_BRIDGE` | env | **the Accessibility Actor** | **BROKEN DEFAULT — break #4.** `cmd/director` already defaults `plugins/uia/uia.exe`; `cmd/marco` does not |
| `$MARCO_DIRECTOR`, `$MARCO_DIRECTOR_BIN` | env | Director client | KEEP — advanced |
| `$MARCO_BIN` | env | which engine a front-end shells | KEEP — advanced |
| `$MARCO_NO_PANIC_STOP` | env | hook ownership hand-off | KEEP — internal, load-bearing |
| `$MARCO_SIMPLIFY_SAVES` | env | overlay teach flow | KEEP — internal |
| `$MARCO_NO_TEACH`, `$MARCO_NARRATE_LOCK` | env | flow control | KEEP — internal |
| `$MARCO_SHADOW_*`, `$MARCO_SP*`, `$MARCO_V2`, `$MARCO_CLASSICAL_*`, `$MARCO_PROBE_BOXES`, `$MARCO_WRITE_TRUTH`, `$MARCO_TRANSITION_AUDIT`, `$MARCO_LIVE_*`, `$MARCO_E2E_DESKTOP`, `$MARCO_SANITIZE` | env | research / test harness | KEEP — developer only, never in a settings UI |
| `$MARCO_LOG` | env | logging | KEEP — advanced |
| `$MARCO_RESOLVER`, `$MARCO_ASSISTANT`, `$MARCO_LLM_*` | env | the deprecated Advisor path | **DEPRECATE** |
| `$MARCO_UI_ADDR` | env | web-ui | zombie |

**90+ environment variables, three JSON files, two store roots, and no single place a user or a
developer can see the effective configuration.** The two settings that broke the product (§1.2
breaks 3 and 4) are both environment variables with the wrong default and no surface.

---

## 18. Product vocabulary

Every user-visible use, counted across `cmd/marco/edit.go`, `cmd/marco/main.go` and the overlay's
`acts.go` / `view.go` / `insight.go`:

| term | occurrences in user-facing text | means | recommendation |
|---|---|---|---|
| **Route** | **~224** | a saved behaviour; also its phrase; also its file | **INTERNAL ONLY.** Replace in UI with **Play** |
| **Plugin** | ~10 (docs/help only) | four unrelated things (§9) | **ADVANCED UI** only, and split the four |
| **Macro** | ~7 | a recorded route | **INTERNAL ONLY** (keep `marco-macros` as a binary name) |
| **Script** | 2 | a Marco `script` declaration | **INTERNAL ONLY** (it is a language word) |
| **Skill** | 0 | — | does not exist; **do not introduce** |
| **Hat** | 0 | — | does not exist; **do not introduce** |
| **Play** | 1 | the durable behaviour | **NORMAL UI — promote to the primary noun** |
| **Actor** | 0 user-facing (a Marco keyword, and a Theater role) | two senses | **ADVANCED UI** for the Theater sense; the language sense stays in `spec/` |
| **Learn** | present in the Learn tab | acquisition — the person demonstrates | **NORMAL UI** (Glossary + ADR-048 already fix this) |
| **Teach** | overlay `teach <name>`, `marco teach` | *acquisition* here — **which contradicts ADR-048**, where Teach means Marco guiding the person | **RESERVED.** Rename the overlay/CLI verb's *label* to Learn; keep the code |
| **Rehearse** | ~5, in Director output | one controlled attempt | **ADVANCED UI** — a normal user sees "Try it" |
| **Director** | ~23 | the whole brain | **ADVANCED UI.** Roadmap 35 says a normal user must not need to know it exists |
| **Theater** | 0 user-facing | the performing half | **INTERNAL / ADVANCED** |
| **Stage / Repertoire / Casting / Production** | 0 user-facing | ADR-068's five parts | **INTERNAL ONLY** — these are architecture words, and being proud of them is not a reason to surface them |
| **Binding / hotkey** | present | trigger → command | **NORMAL UI** (as "Shortcut" or "Binding") |
| **Scope** (global/context/focus) | present, well explained | where a Play applies, and whether it brings the app forward | **NORMAL UI** — keep the three words, they are good, and FOCUS is the liked one |

The single highest-leverage vocabulary change: **Route → Play in the UI only.** ~224 strings, zero
code renames, and it immediately makes the Learn output ("Saved as X") and the browser ("Plays")
name the same thing.

---

## 19. The proposed unified model

The repository supports the brief's model almost exactly. Two corrections and two additions.

```
Audience      the person. Asks · demonstrates · names · corrects · authorises · stops.
              The only source of semantic authority.                         (ADR-068)

Director      understands and decides. Intent · goals · plans · questions ·
              minting authority · deciding whether a goal was met.
              Does NOT act.                                                  (34E)

Theater       knows the Stage and performs.
              + Repertoire   durable: Places, Targets, edges, and who said so
              + Stage        live: application, place, targets, actors, freshness
              + Casting      which Actor can perform this part tonight
              + Production   mounting it, spending exactly one authority
              + Verification did the Scene change the way the Play said        (ADR-068/070)

Actor         a capability that can perform a Target. A Play names none.
              Backed by a Provider, often delivered as an act host process.

Play          durable executable behaviour, in legal Core Marco, with provenance.
              Three kinds: authored · taught · learned.        (routes.Kind, origin.go)
              May declare a STAGE REQUIREMENT: which application must be in front.

Binding       a trigger -- phrase, scope, or hotkey -- that reaches a Play.
              Deterministic, explicit, offline, user-owned.       (routes/bindings.go)

.marco        the language representation. Always readable, always editable,
              always compiled through one gate (driver.CheckSource).
```

### Correction 1 — Binding is first-class, not a footnote

The brief lists it last. The audit puts it beside Play: it is the entire answer to "how does a
person ask for this", it already works, and Director has nothing equivalent.

### Correction 2 — Learn has two modes, not one

Record (exact) and Learn (semantic) are both mature and neither subsumes the other. Presenting one
and hiding the other loses real capability; presenting them as two unrelated commands is the
current failure. Record mode's single-key UX is the bar (§1.5).

### Addition 1 — a Play may require a stage, and Marco brings it forward

The FOCUS scope is the most-liked behaviour in the product and has no place in the brief's model.
It belongs in it: **a Play may declare which application must be in front, and asking for it from
anywhere activates that application first.** `OS's Activate` / `Launch` / `Restore` already
implement it. Do not lose this in a scope simplification.

### Addition 2 — Stage is the missing runtime dependency

A Play that names a Place cannot run in a process with no Stage. **Every learned Play has a
runtime dependency on Theater**, and today the process that runs Plays has no Theater with eyes.
This must be in the model explicitly, or the same break returns.

### What is deliberately *not* in the model

`Scene` — a Marco language word, guarded by `internal/spectest`, and ADR-068 says explicitly it is
not mapped to a Place. **Do not map it.** Likewise `act`, `set`, `actor` (language sense) stay
where they are; §7 of the brief asks whether Theater Actors are already Marco actors, and the
answer is **no, and deliberately so**: a Theater Actor is cast at run time by the host, a Marco
`actor` is a declaration in a program. Sharing the word is already a known cost and the governance
rule in `CLAUDE.md` forbids adding language concepts to chase the metaphor.

**On §7's other questions:**

- *Are sets used as semantic types correctly?* Yes. `Target{Name, Kind}` in `theater.marco` is the
  model case — a semantic set that names no provider. `Control{Window, Element, Name, Value}` in
  `accessibility.marco` is correctly the *opposite*: provider-specific, and the act's comment says
  so.
- *Is Target one of those?* Yes — `the Target is a set` already exists in Core.
- *Does Theater belong as an act surface?* It already is one, with exactly one export
  (`Activate`), and that is the right size. Growing it needs a demonstrated language-level need.
- *Which old acts are provider-specific when they should be semantic?* `Text's Find` and
  `Vision's Locate` return a **Point**, which is a coordinate — geometry, not meaning. They are
  correct for what they are (anchor resolvers for taught routes) and must not become the way a Play
  names a thing. `Theater's Activate` is the semantic replacement and already exists.

---

## 20. Proposed overlay information architecture

**No visual design.** Sections and ownership only.

### Two surfaces, and the split is already right

The HUD is **ambient**: always there, click-through, answers "what now" in one glance.
The control centre is **full**: opened deliberately, holds everything with detail.

```
AMBIENT -- the HUD (plugins/overlay)
  Ask        one command line. Typed and spoken take the SAME path.
             <- `m command line + the spoken-phrase handler (acts.go:211), unified
             data: Binding registry -> Play, else Director
  Here       what Marco understands right now: place, what it can act on,
             whether it is waiting on me.
             <- the `watch` panel, promoted to a leader key
             data: pkg/playbill Current / Seeing / Offers / Question
  Activity   the last few things that happened, with outcomes.
             <- the command-history rows + the HUD log, merged with the action graph
  Stop       one word, one chain (§14).
             <- Esc / leader / spoken "stop"

FULL -- the control centre (marco ui)
  Learn      start · what Marco is watching · what it saw · questions ·
             name a place · Try it · SAVE · REGISTER.
             Two modes, chosen by situation: Record (exact) and Learn (semantic).
             <- the Learn tab + the overlay's `teach`, plus the two missing verbs
             data: service.ObserveLearn (unchanged) / orchestrator.Teach
  Plays      everything Marco can do, with kind, provenance, scope, trigger,
             which app it needs in front, and staged-but-unregistered plays shown as such.
             actions: Run · Register · Rename · Rebind · Change scope · Forget · View source
             <- the Routes tab + the staged directory + origin.go
  Here       places and targets Marco knows; correct or rename them.
             <- plugins/web-ui knows.go/sight.go, harvested
             data: director knows / sight / show-me
  Activity   what Marco has done, and what it learned when.
             <- director history / last + the action graph
  Settings   leader · voice + wake phrase · appearance · shortcuts (Bindings) ·
             screen recognition (the CV decision, once it is made) ·
             where things are stored.
             <- the Config tab + the Bindings tab
  Advanced   Marco source (the step editor + raw .marco) · Actors and providers,
             with availability · Diagnostics (Watch deep, perception, shadow) ·
             Goals and reach · Game packs · effective configuration.
             <- the Edit tab, demoted from default; the diagnostics panels; the
                60-command director CLI, surfaced rather than hidden
```

### What disappears from the normal product

- the word **Route** (→ Play)
- **`.marco` file paths and step tables** as the landing view (→ Advanced)
- **`director`** as a name a user must type
- **rehearsal / candidate / session / proposal / subject-id** vocabulary
- the **anchor / CV dials** (the *decision* surfaces in Settings; the ten dials do not)
- **`marco dispatch`** and **`marco assistant`** from `marco help`

### The one structural rule

> **The overlay is an Audience surface. It renders `pkg/playbill`; it does not compute a second
> account of anything.**

`learnui.go` already follows this and says so in its own comment. The overlay's private `model` of
status/history is the exception, and merging it into the playbill is Phase 4.

---

## 21. Compatibility plan

Real artifacts on disk today: `routes/bindings.json` (a Rocket League binding), `routes/os.marco`,
`routes/<app>/...` scopes, `%AppData%\marco\overlay.json`, `%AppData%\marco\semantic-memory.json`,
`%AppData%\marco\action-graph.json`, and any user's taught routes with `.rec.json` /
`-anchor-*.png`.

| recommendation | compatibility |
|---|---|
| Route → Play **in UI strings only** | **backward-compatible.** No file moves, no code renames |
| Add a Register verb to Learn + Plays | **backward-compatible.** `Registry.Register` already exists and refuses collisions |
| Show staged plays in Plays | **backward-compatible** — a new read of an existing directory |
| Fix "It is in the Routes tab." | **backward-compatible** — a string |
| Default `$MARCO_UIA_BRIDGE` to `plugins/uia/uia.exe` in `cmd/marco` | **backward-compatible.** Mirrors what `cmd/director/main.go:664` already does; an explicit env var still wins |
| Point `marco`'s Screen memory at `$MARCO_HOME/semantic-memory.json` | **needs adapter.** `$MARCO_MEMORY` must keep overriding; if `routes/memory.json` exists it should still be read. Prefer: read the Director store, fall back to the legacy path |
| Give `liveScreens.CurrentSubject` a Director client | **backward-compatible.** Absent Director ⇒ `Unavailable`, exactly as today |
| Preserve the FOCUS scope as a product capability | **backward-compatible** — it already works; this is naming and surfacing |
| Decide the CV/anchor default | **potentially breaks taught-route behaviour either way.** Turning CV on changes timings (the wait cap returns) and re-enables anchors. Gate on a measured experiment, and keep `$MARCO_CV` as the override |
| Unify the stop intake | **needs migration** of the overlay's Esc handling. Behaviour must be identical when only one thing is running |
| Merge overlay history into the playbill | **needs adapter** — the playbill has no per-command duration/outcome field today |
| Learn as one flow with two modes | **backward-compatible.** `marco teach` and `director teach` both keep working; only labels change |
| Deprecate `marco dispatch` / `marco assistant` | **backward-compatible** if the verbs stay and only leave `usage`. **Intentionally breaks** only if removed — and `plugins/llama`/`claude-resolver` would go with them |
| Remove `routes/os.marco` | **needs migration** — a user tree may contain it. It is inert (the resolver uses the built-in module), so deleting is safe, but **do not delete files in a user's routes directory**; stop shipping it instead |
| Remove empty `routes/<app>/` scaffolding | **backward-compatible** — stop creating them; leave existing ones |
| `bindings.json` legacy `Slug` field | **already adapted** by `Binding.command()`. Keep |
| Loose `routes/<app>/*.marco` read as CONTEXT | **already adapted** by `locDir`. Keep |
| Retire `plugins/web-ui` | **backward-compatible** after harvesting; nothing launches it |
| Retire `plugins/ahk-overlay` | **backward-compatible** — it is a README; the code is in another repository. Keep the README as a note |

**No user artifact is rewritten by any recommendation in this audit.**

---

## 22. Migration phases

Not a rewrite. Each phase keeps Learn working, preserves every user artifact, keeps the suite green,
and has a deterministic acceptance that a mutation must be able to break ([[Wiring-Tests]]).

### Phase 0 — Close the saved→runnable chain *(do this first; it is not cosmetic)*

Nothing in Phases 1–6 is worth doing while a learned Play cannot run. §1.5's "no screen
recognition" is this phase, observed from the outside.

1. Learn panel and overlay gain a **Register** action calling the existing
   `LearnedQuery{Register: true}`.
2. `cmd/marco/assistant.go` defaults `$MARCO_UIA_BRIDGE` to `plugins/uia/uia.exe`, as
   `cmd/director/main.go` already does for itself.
3. `cmd/marco/screenwiring.go` reads the Director's store (`$MARCO_HOME/semantic-memory.json`),
   with `$MARCO_MEMORY` still winning and the legacy path as a fallback.
4. `liveScreens.CurrentSubject` asks the Director service when one is reachable, and returns
   `Unavailable` when it is not.
5. Delete the false string at `edit.go:1558`.

**Acceptance.** One test drives the whole chain against a temporary `$MARCO_HOME` and
`$MARCO_ROUTES`: a play is saved, registered, resolved, authorised, and its entry guard *positively
identifies* the named place from a store the Director wrote. Five mutations, five named failures —
removing the Register call, the bridge default, the store path, the Director client, or the guard.
**And the guard must still refuse when nothing can see** — that assertion is the one that may not
be weakened.

### Phase 1 — Plays as the product noun

Routes tab becomes **Plays**: kind, provenance state, scope (including *which app it brings
forward*), trigger, and **staged** plays listed as "Saved — not yet askable" with a Register action.
`Route` disappears from user-facing strings. No package renames, no file moves.

**Acceptance.** A staged play appears in the list; registering it moves it and it becomes
resolvable; a `grep` gate over the UI templates finds no user-facing "route".

### Phase 2 — One invocation intake

Typed and spoken input take the same path: Binding registry (exact scope match, then hotkey) →
Director on a miss. `marco dispatch` and `marco assistant` leave `usage`. This is the largest single
reduction in "weird UX".

**Acceptance.** A typed phrase with no matching Play reaches the Director; a typed phrase that
matches a Play does **not**; the same two assertions hold for a spoken phrase. Mutation: swapping
the tiers must fail both.

### Phase 3 — One stop

The chain in §14. Every intake calls one function.

**Acceptance.** With a Play running and a Director command in flight, one Stop ends both, `finally`
blocks run, held input is released, and ADR-066's Finish/Cancel separation is untouched.

### Phase 4 — One Activity, one account

The overlay's private history/status merges into `pkg/playbill`. The HUD renders; it does not
compute.

**Acceptance.** The HUD and the control centre render the same outcome for the same run from the
same source. Mutation: a second store re-introduced must fail a test that compares them.

### Phase 5 — Actors, providers and integrations get a surface

Advanced gains a page listing Actors, their providers, whether each is available on this machine,
and why not. Plugin is retired as a single word in favour of the four in §9. Settings gains the one
screen-recognition decision (§17) rather than ten dials.

**Acceptance.** With `$MARCO_UIA_BRIDGE` unset, the page says the Accessibility Actor is
unavailable and names the missing binary. *(This is the surface that would have caught break #4.)*

### Phase 6 — Retire the dead surfaces

`internal/dispatch` and the Advisor plugins move behind a developer flag or out of the tree; stop
shipping `routes/os.marco` and the empty scope scaffolding; `plugins/web-ui`'s Sight/Knows/Answer
panels are harvested into the control centre and the plugin is retired; `plugins/ahk-overlay`
becomes a note.

**Acceptance.** The suite is green with the packages removed from the build; no user artifact was
deleted; the harvested surfaces are reachable from the control centre.

**Order matters.** Phase 0 makes the product true. Phases 1–2 make it legible. Phases 3–4 remove the
duplicated accounts. Phases 5–6 are safe only once nothing depends on them.

---

## 23. Risks

| risk | why it is real | mitigation |
|---|---|---|
| **Weakening the Screen guard to make Plays "work"** | The fastest way to make break #2 go away is to let `CurrentSubject` guess. That destroys [[ADR-031-the-user-names-the-stage]] and the "silence is never yes" invariant in one line | Phase 0's acceptance asserts the refusal path *as well as* the success path. Never add a code path that answers `ok` without identifying the named subject |
| **Stage becoming a second WorldState** | 34E's own god-object warning; still open | Stage is a projection with no storage and no sampling |
| **A second authority or foreground gate inside Theater** | Moving production into Theater tempts it; `rehearse` already has `window_not_in_front` | Authority is minted by Director/Audience and consumed by Theater. 34E's "what must not move" list stands |
| **Unifying the intake accidentally sends everything to an LLM** | The deterministic tier is what makes Marco work offline | Bindings are tier one, always, and the test asserts a matching phrase never reaches Director |
| **Renaming code to chase the metaphor** | ADR-068 explicitly refuses to create a `theater/` package; the same discipline must apply to `routes` | Product vocabulary and code vocabulary may differ. Precedent: Learn/`teach` |
| **Designing product on the dormant CV stack** | It is fully built, fully tested and switched off. It *looks* live — and its absence is already a complaint | Make the default a decision with an experiment behind it, not a flag. Until then, do not promise visual recognition |
| **Losing the FOCUS scope in a simplification** | It is a resolver detail in the code and the most-liked behaviour in use (§1.5) | Name it in the model (§19), list it in Plays (Phase 1), and test it |
| **Merging Record and Learn into one flow that loses parameterisation or the single-key UX** | `{{name}}`, the arg key and `then`-chaining exist only on the old side; a learned Play cannot take an argument. The record flow's y/n/s + c/f/g keypresses are why training felt easy | Keep both modes' full capability and interaction cost; the merge is presentational |
| **Two stores drifting again** | `$MARCO_ROUTES` and `$MARCO_HOME` are independent and neither implies the other | Phase 0 makes the Screen host read the Director's store; Phase 5's page shows the effective paths |
| **Breaking the hook invariants while unifying Stop** | `MARCO_NO_PANIC_STOP` prevents dueling low-level hooks; the pump must block, not poll | Do not move hook installation. Change the *intake*, not the mechanism |
| **The uncommitted tree** | The entire Director/Theater half is untracked (`git status`: 75 untracked paths including `cmd/director/`, `internal/director/`, `docs/`) while every commit in the log is legacy-product work | Not an architecture finding, but a real risk to any migration. Worth resolving before Phase 0 |

---

## 24. Things explicitly NOT to change yet

- **Nothing in `spec/`.** Language work is closed (`CLAUDE.md`). No new act, no new set, no new
  word. `Theater's Activate` is the semantic action surface and it is complete.
- **`scene` is not a Place.** ADR-068 refuses the mapping deliberately.
- **`this's` / `that's`** — settled.
- **No Go package renames.** Not `internal/routes`, not `internal/orchestrator`, not
  `internal/director/teach`.
- **No file moves in a user's routes tree.** Registration already moves files; nothing else may.
- **`internal/routes/origin.go`** — do not restructure. It is the best-designed file in the legacy
  half and the entire saved≠registered guarantee rests on its directory layout.
- **The authority seam** (`orchestrator/authority.go`, `production.Authority`) — do not simplify,
  do not merge the two, do not let Theater mint anything.
- **ADR-066's Finish vs Cancel** — do not collapse while unifying Stop.
- **The recorder's hook discipline** — callbacks fast, pump blocking, one hook owner.
- **`driver.CheckSource` as the single compile gate** — `learnedplay.go` depends on there being
  exactly one module list.
- **`internal/spectest`** — the governance gate. Do not relax it to let a generated Play say
  something new.
- **The Learn panel's statelessness** — `learnui.go` holds no state on purpose; every new surface
  should copy it, none should regress it.
- **The FOCUS scope's behaviour** — do not change what it does while renaming what it is called.

---

## Related

- [[34E-director-theater-audit]] — the other half; its Phases 2–4 have landed, Phase 1 has not
- [[Learned-Plays]] — the chain this audit found broken at four arrows
- [[ADR-028-a-learned-play-is-a-file-with-a-past]] — why `learned/` is invisible
- [[ADR-029-resolution-is-not-permission]] — the authority seam
- [[ADR-048-learn-teach-and-do-are-three-different-sentences]] — the product vocabulary rule
- [[ADR-068-the-theater-is-the-durable-semantic-world]] — the canonical Theater vocabulary
- [[ADR-070-one-production-body-and-the-caller-brings-the-verification]] — closed 34E's verifier hole
- [[Roadmap]] §34 (ACTIVE), §35 (Marco product experience) — this audit is the bridge between them
- [[Wiring-Tests]] — why every phase above names a mutation
