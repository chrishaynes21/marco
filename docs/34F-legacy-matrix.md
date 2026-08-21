---
type: reference
status: active
updated: 2026-08-20
source_paths:
  - internal/routes
  - internal/recorder
  - internal/simplify
  - internal/codegen
  - internal/macroir
  - internal/nlu
  - internal/dispatch
  - internal/resolver
  - internal/gamepacks
  - internal/live
  - internal/mlog
  - internal/graph
  - pkg/playbill
  - cmd/marco
  - cmd/director
  - plugins/web-ui
  - plugins/ahk-overlay
---

# 34F legacy matrix — what stays, what is compatibility, what goes

Every major system in the tree with **exactly one** status, plus the compatibility table for every
surface a person or an out-of-tree script could already depend on, plus the exhaustive CLI verb
surface of both binaries.

Written at the close of Roadmap 34F, after Phases 0–4. It supersedes the salvage table in
[[34F-legacy-marco-product-audit]] §5 and the compatibility plan in its §21 — not because those
were wrong, but because four of that audit's findings have since been closed and repeating them
would send somebody to fix them twice. Companion: [[34F-duplication-matrix]] (who owns what) and
[[34F-observe-readiness]] (what the next roadmap needs).

## The status words, and what each one obliges

| status | meaning | obligation |
|---|---|---|
| **KEEP** | part of the product, now and after 35 | maintain |
| **KEEP AS INFRASTRUCTURE** | load-bearing, but never named to a person | maintain; keep out of product vocabulary |
| **KEEP AS DEVELOPER SURFACE** | real, useful, not advertised | maintain; keep out of `marco help`'s normal section |
| **COMPATIBILITY ONLY** | exists because something already depends on it | never recommend it; never grow it |
| **DEPRECATED** | superseded, still works | say so once, in one place |
| **DEAD** | no production caller | delete when convenient; do not extend |
| **DELETE LATER** | agreed to go, blocked on something | name the blocker |
| **DELETED** | gone, recorded here so a reader stops looking | — |

**The rule that governs the whole table:** *do not break scripts unnecessarily, aliases may stay,
but an alias must never appear as the normal recommended product.* Every violation of that rule
found during 34F is flagged **⚠** below.

---

## 1. The legacy matrix

### 1.1 The language and its substrate

| system | what it is | who calls it now | status |
|---|---|---|---|
| `internal/lexer` `parser` `ast` `token` `compile` `runtime` | the Marco language | `internal/driver`; everything | **KEEP** |
| `internal/graph` | the compiled program graph the runtime walks | `internal/compile`, `internal/driver`, `internal/runtime` | **KEEP AS INFRASTRUCTURE.** It was listed as a legacy candidate; it is the compiler's IR, not legacy |
| `internal/driver` | `CheckSource` / `RunFile` / `Serve` — the ONE compile gate | `cmd/marco`, `cmd/director/learnedplay.go` | **KEEP** |
| `internal/osmod` `textmod` `visionmod` `uiamod` `screenmod` `theatermod` | embedded act surfaces | `internal/driver` `builtinModule` | **KEEP** |
| `internal/spectest` | the language-governance gate | CI | **KEEP** |
| `internal/mlog` | stdlib structured logger behind `$MARCO_LOG` | `cmd/marco`, `internal/orchestrator`, `internal/oshost` | **KEEP AS INFRASTRUCTURE** |
| `internal/oshost` / `internal/bridgehost` | act → process FFI | `cmd/marco`, `cmd/director` | **KEEP** |
| `internal/secrets` | credential store behind `do OS's Secret` | `cmd/marco/secret.go`, oshost | **KEEP** |
| `internal/winctx` | foreground application identity | `cmd/marco` | **KEEP AS INFRASTRUCTURE** |
| `internal/activate` | the one activation ladder | `theaterhost`, `rehearse` | **KEEP** |
| `internal/stopsignal` | one stop across process boundaries | `cmd/marco`, the overlay | **KEEP** (new in Phase 3) |
| `internal/outcome` | the six product outcomes and their wire literals | engine, HUD, control centre | **KEEP** (new in Phase 4) |

### 1.2 The Play store (formerly "routes")

| system | what it is | status |
|---|---|---|
| `internal/routes` | the filesystem store for Plays: scoped directories, slugs, sidecars | **KEEP.** The package name is deliberately unchanged — product vocabulary and implementation vocabulary may differ where the difference costs a reader nothing, and nobody outside the repository sees a package name |
| `internal/routes/origin.go` | kind, digest, staging (`LearnedDir`), `Register` | **KEEP** |
| `routes.KindTaught` | the "taught" provenance value | **DEAD (the value), KEEP (the enum).** Nothing writes it; `taughtIfRecorded` infers **Recorded** from the `.rec.json` sidecar instead. The enum stays because it is a format constant |
| `internal/routes/bindings.go` | app-scoped hotkey → command chain | **KEEP — promoted.** A hotkey now enters the intake as an *explicit identity* |
| `routes.Binding.Slug` | the pre-chaining single-slug field | **COMPATIBILITY ONLY**, adapted by `Binding.command()` |
| `Registry.locDir`'s loose-file case | a context Play saved before the `context/` split | **COMPATIBILITY ONLY**, silent and cheap |
| `internal/routes/args.go` | `{{name}}`/`{{1}}`, `name:value`, `… with a,b`, `" then "` chaining | **KEEP** |
| **FOCUS scope** | a Play that runs from anywhere and brings its application forward first | **KEEP — FIRST-CLASS PRODUCT.** Surfaced as `plays.ScopeFocus` + `Play.Activates`, in the overlay help groups and in the plays JSON. The most-liked behaviour in the product and it has no equivalent anywhere in Director; a scope simplification must not lose it |
| `internal/plays` | the Audience-facing projection over the store | **KEEP** (new in Phase 1; not legacy) |

### 1.3 Invocation

| system | status |
|---|---|
| `internal/invoke` — the ONE semantic decision | **KEEP** (new in Phase 2) |
| `cmd/marco/intake.go runInvocation` — the ONE process entry | **KEEP** |
| `orchestrator.Deps.Do` / `.Resolve` / `.Run` | **DELETED in Phase 3.** A complete second invocation spine with no production caller, worse than the live one (no context, no argument substitution). The authority tests that used to enter through it now enter through the production door |
| `internal/nlu` | **KEEP AS INFRASTRUCTURE.** Since Phase 4 it may only *suggest* ("did you mean"), never decide which Play a verb acts on |
| `internal/dispatch` (+ its LLM Advisor) | **DEPRECATED.** A second run/learn/chat/clarify taxonomy; reachable from `marco dispatch` and the REPL only |
| `internal/resolver` (`$MARCO_RESOLVER`) | **DEPRECATED.** REPL only; kept as a proof of the plugin protocol |
| `cmd/marco` `runAssistant` REPL | **DEPRECATED (developer surface).** Nothing launches it |

### 1.4 Acquisition (Learn)

| system | status |
|---|---|
| `internal/recorder` | **KEEP.** Global hooks, stop key, secret placeholders, anchor key |
| `internal/simplify` | **KEEP.** Events → clean steps: waits, coalescing, loop folding, drag detection, argument keys |
| `internal/codegen` | **KEEP.** Steps → Marco on the OS act. `marcoexec/play.go` is a *parallel* lowerer for learned Plays with a different vocabulary; they are not duplicates of each other |
| `internal/macroir` | **KEEP AS INFRASTRUCTURE.** The recorded-demonstration IR |
| `orchestrator.Learn` / `LearnAuto` / `LearnVoice` / `SimplifyRoute` | **KEEP.** Record-mode and narrate-mode acquisition; the live half of a package whose invocation half was deleted |
| `internal/voicelearn` | **KEEP.** Narration → the same IR |
| `internal/director/learn` | **KEEP.** The semantic Learn coordinator |
| `internal/director/demo` | **KEEP AS DEVELOPER SURFACE.** Produces a `demo.Learned` *procedure*, not a Play, and has no product surface at all |
| `service.ObserveTeach` | **DELETED** at protocol v8; merged into `ObserveLearn` |
| the `teach` word in live acquisition code | **DELETED.** Held by `TestNoLiveAcquisitionCodeIsNamedTeach`, which names the surviving CLI/overlay aliases explicitly so they cannot grow |

### 1.5 Perform / Theater / Director

| system | status |
|---|---|
| `internal/orchestrator/authority.go` | **KEEP.** [[ADR-029-resolution-is-not-permission]]'s door, on every invocation |
| `internal/production` + `internal/platform/theaterhost` | **KEEP.** The Theater port and its adapter |
| `cmd/marco/perform.go` | **KEEP** (new in Phase 0). The `marco do` → Director bridge for learned Plays |
| `internal/gamepacks/palworld` | **KEEP AS INFRASTRUCTURE / UNSURFACED.** Compiled in, reachable from `director game`; `marco games` is its only other door and nothing calls it |
| `internal/screen` | **KEEP AS INFRASTRUCTURE** for capture; its *route-time anchor* role is off by default |
| the CV / anchor stack at route time (`MARCO_CV`, `MARCO_ANCHORS`) | **DEPRECATED-BY-DEFAULT, AND UNDECIDED.** 34F said this must become a deliberate product decision. It has not been made. Turning it on changes taught-route timings and re-enables anchors; leaving it off means a whole resolution mechanism ships dead. **Gate on a measured experiment, keep `$MARCO_CV` as the override, and decide** |
| `internal/live` | **KEEP AS DEVELOPER SURFACE.** Behind `//go:build livevalidation` + `$MARCO_LIVE_VALIDATION`; off the default path by construction |
| `pkg/playbill` | **KEEP.** The one read-only account a surface renders. Backstage words were taken off it in Phase 4 |
| `pkg/directorapi` | **KEEP.** The Director ↔ platform boundary |
| `pkg/referent` | **KEEP.** Deictic reference → screen geometry |

### 1.6 Front ends and plugins

| system | status |
|---|---|
| `plugins/overlay` | **KEEP.** The HUD; an act host and a front end in one process |
| `cmd/marco/edit.go` — the control centre (`marco ui`) | **KEEP.** Learn · Plays · Bindings · Config · Help |
| `cmd/marco/learnui.go` | **KEEP.** The Learn panel's stateless API |
| `plugins/uia` (C#) | **KEEP.** The Accessibility Actor's provider, and the only Actor substrate in the product |
| `cmd/marco-macros` | **KEEP.** The OS bridge (SendInput) |
| `plugins/voice` | **KEEP.** Vosk wake word + transcripts, as an event feed |
| `plugins/ocr` | **KEEP.** Text act host and OCR perception provider |
| `plugins/vision` | **KEEP.** Vision act host and semantic detector |
| `plugins/vision-groundingdino` | **KEEP AS DEVELOPER SURFACE.** One Python script; research |
| `plugins/marco-app` + `pack.ps1` | **KEEP AS INFRASTRUCTURE.** The single-file Windows bundle |
| `plugins/claude-resolver`, `plugins/llama` | **COMPATIBILITY ONLY.** Reachable only from `marco assistant` / `marco dispatch`, both deprecated |
| `plugins/web-ui` | **DEPRECATED.** A second front end with its own vocabulary that shells `director.exe` directly. Nothing launches it; `setup.ps1 -WebUI` builds it. Its Sight / Knows / Answer panels are the *only* surfaces for several real capabilities, so **harvest before retiring** |
| `plugins/ahk-overlay` | **DELETE LATER.** A README pointing at another repository; no code. Convert it to a `docs/` note |
| `bridges/echo-bridge`, `bridges/*.ahk` | **KEEP AS DEVELOPER SURFACE.** Protocol examples |
| `cmd/{bakeoff,identityprobe,nameprobe,twinprobe,variance}` | **KEEP AS DEVELOPER SURFACE.** Research probes |
| `cmd/docscheck` + `internal/docsindex` | **KEEP AS DEVELOPER SURFACE.** Vault integrity |
| the overlay's in-HUD help panel (`helpLines`, `showHelp`, `helpOn`) | **DEAD at `ac8da6c`** — no caller, and `helpOn` only ever set false, because `help` opens the control centre's Help screen instead. `helpLines` was removed by concurrent work during this campaign; check the residue rather than assuming either state |

### 1.7 Others, not on the original list

| system | status |
|---|---|
| `internal/director/binding` — deictic reference ("this file") | **KEEP**, but the name collides with `routes.Binding` and they are unrelated concepts. Rename or document loudly |
| `internal/platform/navsource` | **KEEP.** The low-level nav hook and the blocking-pump invariant |
| `internal/platform/recordhost` | **KEEP.** Recording as an act |
| `internal/director/shadowreplay`, `perception/shadow` | **KEEP AS DEVELOPER SURFACE.** Identity-loss diagnosis |
| `internal/director/visionbench` | **KEEP AS DEVELOPER SURFACE.** Frozen-corpus scoring |
| root `*.exe` build output | **DELETE LATER** (developer hygiene; git-ignored, not shipped) |

---

## 2. The compatibility table

| LEGACY SURFACE | NEW OWNER | COMPATIBILITY STATUS | DEPRECATION STATUS | REMOVE WHEN |
|---|---|---|---|---|
| `marco routes` / `marco routes --json` | `internal/plays` | **SUPPORTED, NOT AN ALIAS.** Deliberately narrower than `marco plays`: only what can answer. Out-of-module front ends call it | **Not deprecated**; documented in README and `marco help` | Never, or when every out-of-tree front end is retired |
| the JSON keys `name` / `slug` / `app` / `scope` | `internal/plays` | **FROZEN CONTRACT.** May be added to, never renamed. Held by `TestMarcoRoutesJSONKeepsItsPublishedKeys` | n/a | Never |
| the `"[route] <name>"` stdout line | `internal/outcome` | **WIRE PROTOCOL, not user text.** The overlay parses it | n/a | Never without a protocol bump |
| the `"[result] <outcome>"` stdout line | `internal/outcome` | **WIRE PROTOCOL.** Both sides now import the same constant rather than restating it | n/a | Never without a protocol bump |
| the `no play matches …` error prefix | the intake | **FROZEN PREFIX**, prefix-matched by the overlay to turn a miss into an offer to Learn; it contains the literal `marco learn` | n/a | Never without updating the overlay |
| `marco teach` | `marco learn` | **ALIAS, works** | Deprecated; undocumented except once in README's command reference | With the legacy-verb sweep |
| `director teach` | `director learn` | **ALIAS, works** | Deprecated, undocumented | Same sweep |
| overlay `` `m teach <name> `` | `` `m learn `` | **ALIAS, works** | Deprecated, undocumented; held by `plugins/overlay` `learnalias_test.go` | Same sweep |
| overlay `narrate teach` / `voice teach` | `narrate learn` | **ALIAS, works** | Named as an alias in the control centre's Help tab, *after* the canonical form. Acceptable under the rule — it is not presented as the normal product — but it is the only place an alias is written down at all, and the sweep should take it | Same sweep |
| `dispatch.IntentLearn` → the wire value `"teach"` | — | **FROZEN VALUE** for out-of-tree resolver plugins; explicitly allow-listed by the governance test | Frozen deliberately | With `internal/dispatch` |
| `marco dispatch` | Director | works | **Deprecated in practice.** `marco help` lists it under *"Developer surfaces, kept working and not part of the normal product"* — this was ⚠ during the campaign and is now correct | Phase 6 |
| `marco assistant` | the overlay + control centre | works | **Deprecated in practice**, same section of `marco help`. ⚠ **README still uses it as the hero example** — see *Open flags* | Phase 6 |
| `$MARCO_RESOLVER`, `$MARCO_ASSISTANT`, `$MARCO_LLM_*` | Director | read only by `internal/resolver` and `internal/dispatch/llm.go` | ⚠ **actively installed** by `setup.ps1 -Llama` / `-Resolver` into `overlay.cmd`, where nothing reads them | Phase 6 |
| `plugins/claude-resolver`, `plugins/llama` | — | build, run, speak the protocol | **COMPATIBILITY ONLY**; still installer flags | With `marco assistant` |
| `plugins/web-ui`, `$MARCO_UI_ADDR` | the control centre | builds; `setup.ps1 -WebUI` builds it; **nothing launches it** | **DEPRECATED** | After Sight / Knows / Answer are harvested into `marco ui` |
| `plugins/ahk-overlay` | — | README only; the code is in another repository | **DEAD POINTER** | Convert to a `docs/` note |
| the `routes/` directory layout (`global/`, `<app>/context/`, `<app>/focus/`) | `internal/routes` | **UNCHANGED**; `$MARCO_ROUTES` overrides | Not deprecated; the *word* is internal-only now | Never |
| loose `routes/<app>/*.marco` read as CONTEXT | `Registry.locDir` | **ADAPTED** | Compatibility only | Never — silent and cheap |
| `routes/bindings.json` | `internal/routes/bindings.go` | **UNCHANGED** | — | Never |
| the legacy `slug` field in `bindings.json` | `Binding.command()` | **ADAPTED** | Compatibility only | Never |
| `<slug>.origin.json`, `Version: 1` | provenance | **UNCHANGED**, digest-verified | — | Never without a version bump |
| `<slug>.rec.json`, `<slug>-anchor-*.png` | recorder / simplify | **UNCHANGED**; carried by Rename and Delete | — | Never |
| the `<app>/learned/` staging directory | staged Plays | **UNCHANGED**; structurally unresolvable by design ([[ADR-028-a-learned-play-is-a-file-with-a-past]]) | — | Never |
| `%APPDATA%\marco\semantic-memory.json` | the Director **and** `marco`'s Screen host | **now shared** — this was 34F break #3 | `$MARCO_MEMORY` wins in both processes | Never |
| `$MARCO_MEMORY` | both processes | **honoured identically** | — | Never |
| `routes/memory.json` (the old Screen store) | — | **no longer read anywhere** | Gone | Already gone — and nothing deletes a user's copy |
| `$MARCO_UIA_BRIDGE` | the Accessibility Actor | **still wins**; discovery is now the fallback rather than the requirement | — | Never |
| `$MARCO_DIRECTOR=off` | the overlay's watch panel only | **narrowed** — it no longer gates dispatch | — | Keep |
| `$MARCO_CV` / `$MARCO_ANCHORS` / `MARCO_FIND_*` | anchors | **default OFF** | Undecided | Needs a decision, not a removal |
| `MARCO_NO_TEACH` | — | **removed.** Nothing ever read it; the overlay's `stop_test.go` refuses its return | Gone | Already gone |
| `MARCO_SIMPLIFY_SAVES` | `marco simplify` | still read: simplify-and-save with no prompt, set by the overlay | Keep — but note `simplify` now resolves the Play **exactly**, which is what made the silent save safe | Keep |
| `marco ui routes` view id | the Plays tab | **kept**; `marco ui plays` maps onto it | Not deprecated — a view id is not a product word | Never |
| `/api/routes`, `/api/route`, `/api/scope`, `/api/delete`, `/api/do`, `/api/bind` | the control centre | **local, same-process, unversioned.** No out-of-tree consumer found | Internal | Free to change |
| `marco director <subverb>` | the Director client | the overlay depends on `watch` / `diagnose` / `perception` / `normal` | Internal seam | With the overlay |
| protocol version **9** | the Director service | **fails loudly on mismatch**, which is the intended failure | — | Bump per change |

### 2.1 User artifacts — confirmed not rewritten

Nothing under the user's `routes/` tree is tracked by git. Present on the development machine:
`routes/bindings.json` (one binding, to a Play that does not exist), `routes/os.marco`
(byte-identical to `internal/osmod/os.marco`, and inert — the resolver uses the built-in module),
and a handful of empty scope directories. Under `%APPDATA%\marco\`: `action-graph.json`,
`director-history.json`, `director-service.json`, `director-stop`, `overlay.json`,
`semantic-memory.json`, `variables.json`, and empty `demonstrations/` and `learned/`.

**No recommendation in this note, and nothing that landed in Phases 0–4, rewrites any of them.**
The only file-moving operation in the product is `Registry.Register`, which moves a staged pair
into a resolvable scope and refuses on slug collision. `forgetPlay` deletes a registered Play's
source and origin and deliberately does not reach `<app>/learned/`.

**Never delete a file from a user's routes directory.** `routes/os.marco` and the empty scaffold
should stop being *created and shipped*; existing copies stay where they are.

### 2.2 Open flags — an alias or a deprecated surface presented as the product

| ⚠ | where | why it breaks the rule | fix |
|---|---|---|---|
| ⚠ 1 | `README.md`'s command reference lists `marco assistant` inline and unmarked among `do` / `learn` / `simplify` / `ui` | The prose two hundred lines earlier does call it "a developer surface", but a reference table read on its own presents it as an ordinary verb. (The **hero example is already fixed** — it is `marco learn` → `marco plays` → `marco do`, showing the `[route]` / `[result]` lines. The mid-campaign audit's "README hero is stale and uses the reserved word" is **STALE**) | Mark it in the reference table, or move it to a developer block there |
| ⚠ 2 | `setup.ps1 -Llama` / `-Resolver` write `$MARCO_RESOLVER` / `$MARCO_ASSISTANT` into `overlay.cmd`, where **nothing reads them** | An installer flag that configures a dead seam tells the person they enabled something | Drop the flags, or point them at Director |
| ⚠ 3 | `marco bind` / `unbind` sit in `marco help`'s **developer** section | A Binding is first-class product ([[34F-legacy-marco-product-audit]] §19 correction 1). Demoting it to a developer surface contradicts the model, and the control centre has a whole Bindings tab | Move them into the normal section |
| — | the control centre's Help tab names `narrate teach` / `voice teach` **as aliases**, after the canonical `narrate learn` | Borderline, and currently acceptable: an alias named as an alias is not an alias presented as the product | Take it with the legacy-verb sweep |

---

## 3. CLI verb surface — `cmd/marco`

**NORMAL** = offered to a person as the product · **DEVELOPER** = real, kept, not advertised ·
**COMPATIBILITY** = alias or legacy kept for scripts · **DEAD** = no production caller.

| verb | mark | note |
|---|---|---|
| `help` / `-h` / `--help` | NORMAL | now separates a "Developer surfaces" section explicitly |
| `do` | NORMAL | the product entrance; `--source`, `--play`, `--app`, `--focus` |
| `stop` | NORMAL | one word, and since Phase 3 it crosses processes |
| `press` | NORMAL | an overlay verb, typed or spoken; reaches real input directly and deliberately |
| `learn` | NORMAL | `--narrate` selects narration mode |
| `teach` | **COMPATIBILITY** | undocumented alias for `learn` |
| `simplify` | NORMAL | resolves the named Play **exactly** since Phase 4 |
| `edit` | NORMAL | opens one Play's editor |
| `ui` | NORMAL | `ui plays\|routes\|bindings\|config\|help\|edit` |
| `wake` | DEVELOPER (internal) | read by `overlay.cmd` |
| `assistant` | **DEVELOPER** | ⚠ still the README hero |
| `games` | **DEAD** | no caller anywhere; its own doc comment claims a caller that does not exist |
| `director` | NORMAL (thin client) | sub-verbs below |
| `dispatch` | **DEVELOPER** | the pre-Director phrase classifier |
| `plays` | NORMAL | the product listing, staged Plays included |
| `register` | NORMAL | makes a staged Play askable |
| `routes` | **COMPATIBILITY (supported)** | narrower than `plays`; out-of-module front ends depend on it and on its first four JSON keys |
| `active` | DEVELOPER (internal) | the overlay polls it |
| `bind` | NORMAL | ⚠ currently listed as developer in `marco help` |
| `unbind` | NORMAL | ⚠ same |
| `hotkey` | DEVELOPER (internal) | the overlay's `Hotkey` act |
| `forget` | NORMAL | `forget all` is guarded |
| `rename` | NORMAL | carries the sidecars |
| `args` | DEVELOPER (internal) | the overlay's argument hints |
| `secret` | NORMAL | `set` / `list` / `rm` |
| `diag` | DEVELOPER | Windows only |
| `vision` | DEVELOPER | `vision detect <png>` |
| `run` | NORMAL (language) | |
| `serve` | NORMAL (language) | `overlay.cmd` uses it |
| `test` | NORMAL (language) | |
| `contracts` | DEVELOPER (language) | |
| `check` | NORMAL (language) | |

### 3.1 `marco director <subverb>`

`stop` / `cancel`, `status`, `history`, `last`, `shutdown` — **NORMAL** (thin client).
`watch` / `playbill`, `normal` — **NORMAL** (the overlay's Here panel).
`diagnose` / `diagnostics`, `perception`, `explain`, `world`, `events`, `live-analysis` / `live`,
`observation-events` — **DEVELOPER**.
Anything else is submitted as a **phrase** — see [[34F-duplication-matrix]] I-2: this is the one
product-reachable path that skips Play lookup entirely.

---

## 4. CLI verb surface — `cmd/director`

`cmd/director/main.go` describes itself in its own header as the Director's **development
front-end**. Treat the whole binary as DEVELOPER except where the product reaches it.

| verb | mark |
|---|---|
| `serve` | **NORMAL (internal)** — the service `marco director` auto-starts |
| `perform` | **NORMAL (internal)** — the walker `marco do` delegates to for learned Plays, via the service |
| `status`, `stop`, `shutdown` | **NORMAL (internal)** |
| `learn` | DEVELOPER — the product reaches the same coordinator through `ObserveLearn` |
| `teach` | **COMPATIBILITY** |
| `learned` (incl. `--save`, `--register`) | DEVELOPER — superseded for the product by `marco register` |
| `rehearse` | DEVELOPER |
| `light` | DEVELOPER (instrument) — **structurally the seed of OBSERVE**; see [[34F-observe-readiness]] |
| `reach`, `plan`, `goal`, `goals`, `procedures` / `procedure` | DEVELOPER |
| `execute` | DEVELOPER |
| `knows`, `sight`, `show-me`, `answer`, `name-screen`, `revise`, `withdraw`, `confirm` | DEVELOPER — these are product concepts with **no `marco ui` home yet**; 34F §7 remains open |
| `inspect`, `graph`, `show`, `analyze` / `analyse`, `history`, `last`, `explain`, `actions`, `observations`, `fusion`, `windows`, `frames`, `edit`, `trace`, `lower`, `op`, `wait`, `visual`, `ocr`, `collections`, `capabilities`, `demonstrate`, `demonstrations`, `demonstration`, `extract`, `vision`, `game`, `observe-game`, `observation-sessions`, `observation-session`, `observation-insights`, `cancel-observation` | DEVELOPER |
| `shadow-trace`, `shadow-replay`, `capture-vision-fixture`, `benchmark-vision` | DEVELOPER (research) |
| `reset-test-state` | DEVELOPER (harness; refuses unless `$MARCO_HOME` is a sandbox) |
| `help` / `-h` / `--help` | DEVELOPER |

**No verb in `cmd/director` is DEAD.** Every one is reachable, and for several subsystems the CLI
verb is the only surface that exists. `director op` is the one to watch: it reaches real input with
no authority regime at all and is exposed over the service protocol, not just the CLI.

## Related

- [[34F-legacy-marco-product-audit]] · [[34F-duplication-matrix]] · [[34F-observe-readiness]]
- [[Plays]] · [[Invocation]] · [[Glossary]]
- [[ADR-086-one-acquisition-one-word-one-request]] ·
  [[ADR-048-learn-teach-and-do-are-three-different-sentences]]
