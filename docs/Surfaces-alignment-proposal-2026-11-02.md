---
type: reference
status: proposal
date: 2026-11-02
supersedes:
  - Shortcuts-from-Studio-proposal-2026-11-02
source_paths:
  - plugins/studio/routines.go
  - plugins/studio/target.go
  - plugins/studio/rescope.go
  - plugins/studio/recording.go
  - plugins/studio/resolve.go
  - plugins/studio/acts.go
  - plugins/studio/frontend/index.html
  - plugins/overlay/controller_windows.go
  - plugins/overlay/acts.go
  - plugins/overlay/intake.go
  - plugins/overlay/commands.go
  - plugins/overlay/resolve.go
  - plugins/overlay/config.go
  - plugins/marco-app/main.go
  - cmd/marco/assistant.go
  - cmd/marco/intake.go
  - cmd/marco/bind.go
  - cmd/marco/edit.go
  - cmd/marco/oconfig.go
  - internal/routes/bindings.go
  - internal/routes/registry.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/keep.go
  - internal/orchestrator/orchestrator.go
  - internal/orchestrator/rescope.go
  - internal/invoke/invoke.go
  - internal/codegen/codegen.go
---

# One Marco, two faces — aligning Studio, the widget and the CLI

**Status: proposal. Nothing here is built. No production code, owner library, binary or desktop
was touched.** Branch `feat/demonstrated-acts`, HEAD `6c0498b`. This supersedes
[[Shortcuts-from-Studio-proposal-2026-11-02]], whose binding trace (its §1, G1–G13) is still
accurate and is referenced rather than repeated.

- **Studio** is the workbench: record, review, edit, test, manage.
- **The widget** (the overlay: leader key + command line + HUD) is the compact keyboard console
  used while working or playing.
- **The CLI** is a third entrance to the same operations.

"Same wiring" means one routine identity, one library, one resolution policy, one binding store,
one argument rule, one execution door, one stop and one result. Each face may collect those
differently; none may own its own version of them.

Evidence labels: **[source]** read at HEAD with the location given; **[traced]** followed through
the code but not executed; **[not established]** believed, not shown. No headless render, build or
desktop run was made. Layouts are text mockups.

Related: [[Invocation]], [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]],
[[ADR-266-one-set-of-rules-for-keeping-a-recording]], [[ADR-267-director-owns-route-resolution]],
[[ADR-268-record-only-the-chosen-window]], [[ADR-251-a-press-checks-the-window-it-was-recorded-in]],
[[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]],
[[ADR-276-a-routine-may-be-told-what-to-type]], [[ADR-277-a-request-may-propose-the-text-a-routine-types]],
[[ADR-294-the-library-is-the-screen]].

---

## 0. Amendment 1 — rulings received (2026-11-02)

The owner ruled R1–R8 and corrected five points. **Where this section and a later section
disagree, this section wins**; the later sections have been edited to match, and the original text
of anything withdrawn is not kept.

| # | ruling |
|---|---|
| R1 | Guarded **Only in this application**: approved as a **separate behaviour slice**. Bind the actual foreground window without raising or searching, require the target application, keep the press-time checks. Its generation/compatibility implications come back for review **before** it is coded. Until then the scope is shown unavailable with its reason. |
| R2 | Default **From anywhere → this application**, through one shared default. The CLI prints the changed Enter default. **With no application known: ask or refuse** — never invent one, never silently choose the current-foreground scope. |
| R3 | Explicit in-place invocations are checked at run time. **Studio's validated handoff to a matching application window satisfies it; merely supplying `--window` does not.** `Source` stays diagnostic, never permission. |
| R4 | Foreground-scoped assignment plus an explicit **everywhere** option. Studio supplies its chosen scope and never inherits its own foreground. |
| R5 | Studio's *Start the widget* is a separately reviewable follow-on **after** the shared wiring. Login startup **deferred**. Shared themes stay on the roadmap. |
| R6 | **Forget removes shortcuts. Legacy name-only bindings require explicit reassignment before they run.** No silent migration, revival by name or redirect. Legacy data is preserved and the surface explains what needs reassigning. |
| R7 | CLI edits converge on the reviewed operations with explicit consent — **after verifying each bypass**. Verified (§8): `marco simplify` is **already** the reviewed path (`SimplifyRoute` → `Prepare`/`Replace`, `orchestrator.go:824-866`); only `marco rename` bypasses it (`assistant.go:619-643` → `Reg.Rename`). |
| R8 | Forget one routine: an exact-routine confirmation that names what is removed and reports partial cleanup honestly — not the Forget-everything card reused. |

Corrections:

1. **Studio's find box stays Director-only.** The local exact arm proposed for S8 is withdrawn; the
   dependency is deliberate (ADR-267). Listing and explicit Play stay independent of Director. F3 is
   no longer listed as a defect.
2. **Extracted arguments stay proposals.** Forwarding them as `--arg` needs a review-and-confirm
   flow; an ordinary route offer is not one. So the widget's offer ignoring Director's values
   (F4) is the *correct* outcome until that flow exists, not a defect.
3. **No fixed-value authoring or storage in this work.** Existing configured values in bindings
   (`` `e = enter freeplay with 3 ``) keep working and are documented in §10.
4. **Collision consent is to a binding VERSION**, not a route ID: the same routine can keep its
   identity while the binding on a key changes. `Assign` takes the binding version the person was
   shown and re-validates it under the bindings lock (§5.1).
5. **Delegated findings verified individually** (§8). Widget focus and recording-time shortcut
   admission are stated as **not demonstrated**, not as properties.

Requirements retained from the earlier proposal: **a recording in progress blocks routine
shortcuts**; the **two-second arming stays** initially; shared configuration **preserves unknown
fields**; route and binding locks have **one documented order** (§5.8).

**Preferred argument UX (attached):** deterministic command templates and completion first,
Director extraction for freer phrasing afterwards (§5.4).

**This change implements only the foundation**: the shared scope vocabulary and default (R2), and
run-time admission for in-place routines (R3). Shortcut storage, surface controls, argument
interaction and the launcher remain later increments, each preceded by a rendered interface.

---

## 1. What already is shared (and must stay so)

Most of the machinery is already one implementation. The divergence is almost entirely in how the
faces *enter* it.

| concern | owner today | reached by |
|---|---|---|
| library location | `internal/marcopaths` | all three [source] `marcopaths.go:70-110` |
| the run door | `cmd/marco` `runInvocation` → `invoke.Decide` (six arms) | every run, from every face, is a spawned `marco do` / `marco hotkey` [source] `cmd/marco/intake.go:111` |
| explicit identity | `invoke.Request.Play` (`--play/--app/--focus`) | Studio Play, control-centre Run, widget's confirmed offer, hotkey [source] `invoke.go:131` |
| argument values on an identity | `--arg=name=value` → `Request.Named/Flagged` | Studio only [source] `assistant.go:252-285` |
| version pin | `--expect=<digest>` | Studio only [source] `invoke.go:139-151` |
| keeping a recording | `orchestrator.Keep` + `WhyNotThisScope` | Studio, `marco learn`, widget `learn` [source] `keep.go:259-285` |
| moving between scopes | `orchestrator.PrepareRescope`/`Rescope` | Studio only [source] `rescope.go:201,469` |
| language → routine | Director `RESOLVE_ROUTE` → `resolveroute` | Studio find box, widget fallback, `marco resolve`/`dispatch` [source] `resolveroutewiring.go:57` |
| stop | `stopsignal.Raise(Home())` | all three [source] `stopsignal.go`, Studio `acts.go:1526`, widget `acts.go:895-948` |
| result | `[result]` line → `internal/outcome` | Studio `routines.go:757`, widget `outcome.go:89` |
| bindings | `routes.Registry` `Bind/Unbind/HotkeyCmd`, `<library>/bindings.json` | CLI, widget words, control centre; **not Studio** |
| leader + overlay settings | `overlay.json` | widget `config.go:64`, control centre `oconfig.go:15` (two structs); **not Studio** |

**Recommendation that follows from this table:** do not add a new service and do not move the
door into Director. The boundary already exists — `cmd/marco` + `internal/orchestrator` +
`internal/routes` for routines, Director for language. What is missing is (a) a few operations that
only one face reaches, (b) three policies that are decided differently per face, and (c) the
widget's reachability.

---

## 2. Capability matrix

Legend: ✔ offered · ◐ offered differently / partially · ✘ absent. "Owner" is the shared
implementation the face reaches, or **(face)** where the face carries its own policy.

### 2.1 Find and select

| capability | Studio | widget | CLI | owner | divergence |
|---|---|---|---|---|---|
| list routines | ✔ library (2.5 s poll) `routines.go:216` | ◐ `plays` panel via `marco routes --json` `commands.go:270` | ✔ `marco plays`/`routes` | `routes.Registry.List` / `plays.Registered` | two listing builders; Studio adds runnability (`Unrunnable`), widget does not |
| exact name | ◐ **through Director, by design** (ADR-267) `resolve.go:31-35`; the library list and Play need no Director | ✔ typed phrase → `Decide` arm 4 | ✔ `marco do <name>` | `Registry.Resolve` / Director | a deliberate difference: the widget's typed name is a *run*, Studio's find box is a *question* |
| other names (aliases) | ✔ edit `answers.go` | ◐ matched only by Director, not by arm 4 `aliases.go:235` | ◐ same | `orchestrator` alias + Director | an alias works in the widget only while Director runs |
| natural language | ✔ `SuggestRoutine` → `RESOLVE_ROUTE` | ◐ fallback `marco resolve --json`, offer on "unavailable" only `overlay/resolve.go:92-163` | ✔ `marco resolve`/`dispatch` | Director `resolveroute` (ADR-267) | widget's offer is reachable in a narrow race only (agent-traced; see §8) |
| stated target app | ✔ `askTarget` dropdown | ✘ | ✔ `resolve --target` | `resolveroute.Where` | — |

### 2.2 Run an exact routine

| capability | Studio | widget | CLI | owner | divergence |
|---|---|---|---|---|---|
| names it by | identity `--play --app --focus` | **phrase** (`do --source=typed <words>`) `intake.go:37` | phrase or identity | `invoke.Decide` | widget re-resolves words each time; namesakes possible |
| target window | **chosen** in Studio, raised by handle, `--window=hwnd:pid` for **every scope** `routines.go:497-521` | none passed; focus routines `Activate` by program name; others press into the foreground | `--window` optional | `orchestrator.GivenWindow`, `oshost.GiveWindow` | see §3 — the same scope label behaves three ways |
| foreground read | Studio itself (picker excludes Studio) | child's `winctx.Active()` (excludes nothing) | child's `winctx.Active()` | — | **three foreground readings**: Studio picker, `winctx.Active`, Director's `foregroundNow` (excludes Marco windows) |
| argument values | ✔ values form, Director proposals, provenance, `--arg`, `--expect` | ◐ only in the phrase grammar (`say hello with chris`); `:` cannot be typed `controller_windows.go:502-517` | ✔ phrase or `--arg` | `routes.ApplyArgs`, `PrepareInvocation` | widget's confirmed offer carries no values (`overlay/resolve.go:42-60`) — **correct** until a review-and-confirm flow exists (Amendment 1, correction 2) |
| missing value | form asks | refused with sentence telling you to use Studio | refused | `panicstop.go:254-308` | the widget has no way to supply one |
| confirmation | token + `ConfirmPlay` | `AskFirst` prompt answered in HUD; **Enter = no**, but the HUD's own offers treat Enter = yes | stdin prompt | `orchestrator.AskFirst` | Enter means opposite things in one HUD |
| run source in trace | **not passed** → recorded as `cli` | `typed`/`spoken`/`hotkey` | `cli` | `invoke.Source` | Studio runs are mislabelled |

### 2.3 Leader shortcuts

| capability | Studio | widget | CLI | control centre (`marco ui`) | owner | divergence |
|---|---|---|---|---|---|---|
| assign | ✘ | ✔ `bind <key> <routine>` → `marco bind`, scope = **foreground** | ✔ `marco bind`, scope = foreground | ✔ Bindings tab, scope = **routine's app** (`bindScope`) | `bindKey` + `Registry.Bind` | two scope rules; collision silently replaces (`bindings.go:55-66`) |
| list | ✘ | ✘ (only via `ui bindings`, opens a browser) | ✘ (no `marco bindings`) | ✔ | `Registry.Bindings` | the face used while playing cannot show what its keys do |
| invoke | ✘ | ✔ leader + key → `marco hotkey <k>` | ✔ `marco hotkey` | — | `pressHotkey`: app binding, then global, resolved by **name** to an identity per press | identity re-derived from words (G3/G4) |
| change / remove | ✘ | ✔ `unbind <key>` (foreground scope only) | ✔ `marco unbind` | ✔ | `Registry.Unbind` | — |
| per-app key + global key | — | ✔ by typing `bind` in each app | ✔ | ◐ global routines always land global | `HotkeyCmd` app-first | works today; see §4.3 |
| survives rename | ✘ | ✘ | ✘ | ✘ | — | G5 |
| result/outcome | — | ◐ streamed lines; no outcome row in the HUD history (agent-traced `acts.go:295`) | ✔ exit code | — | `outcome` | hotkey runs are not in the widget's history |

### 2.4 Record, review, save

| capability | Studio | widget | CLI | control centre | owner | divergence |
|---|---|---|---|---|---|---|
| start | ✔ window list → `StartRecording`, hooks **in Studio's process** | ✔ `learn <name>` → `marco learn --window <front>` | ✔ `marco learn` (asks which window) | ◐ Learn panel → Director `ObserveLearn` | `Deps.Begin` / `Keep` (Director path differs) | three recorders: in-Studio, `marco learn`, Director's LearnSession |
| stop | Stop button or `MARCO_STOP_KEY` | the leader (passed as `MARCO_STOP_KEY`) | stop key | Director | recorder `StopKey` | — |
| review | ✔ steps, simplify vs original | ✘ (`s` simplifies and saves blind) | ✘ | ◐ | `SimplifiedDraft` | ADR-263's reason Studio is the front door |
| scope at save | select, **default "only in"** `index.html:3314` | `marco learn` prompt, **Enter = only in** `orchestrator.go:565` | same | — | `Keep` + `WhyNotThisScope` | defaults agree with each other and **disagree with the ruled default** (§3) |
| narrate | ✘ | ✔ `narrate learn` | ✔ `--narrate` | — | `LearnVoice` | widget narrate likely broken with >1 window (agent-traced, §8) |

### 2.5 Stop, progress, results

| capability | Studio | widget | CLI | owner |
|---|---|---|---|---|
| stop a run | Stop → `stopsignal.Raise` | leader while running → `stopRunningPlays` + `marco director stop`; `stop` word → `marco do stop` | `marco stop` / `do stop` | `stopsignal` + `directorStop` — one mechanism, **three verbs** |
| progress | "Performing X in App…" only | Director status line `[n/m]` | stdout | — |
| result | `outcome.FromLine` + run reference + Report | `outcome` → HUD status + history | exit code + `[result]` | `internal/outcome`, `runref` |

### 2.6 Configuration

| item | Studio | widget | CLI | control centre | owner |
|---|---|---|---|---|---|
| library | read-only display | inherits env | env | env | `marcopaths` ✔ one owner |
| leader key | ✘ not read | ✔ `config` panel | ✘ | ✔ `/api/oconfig` | **two structs over one file**; the widget's has no `wake` field, so a widget save drops the control centre's wake word (agent-traced, `config.go:105-117` vs `oconfig.go:30`) |
| resolver | ✘ (Director's) | ✘ | ✘ | ✘ | Director `$MARCO_RESOLVER` ✔ one owner |

### 2.7 Edit routines

| capability | Studio | widget | CLI | owner | divergence |
|---|---|---|---|---|---|
| rename | reviewed move `PrepareRename` → `Rescope` | `rename a to b` → `Reg.Rename` | same | **two** | CLI/widget rename skips the review, the lock and the version check [source] `assistant.go:619-643` |
| simplify | reviewed `Prepare/Replace` | `simplify x` → `SimplifyRoute` | same | **one** | `SimplifyRoute` already calls `Prepare`/`Replace` [source] `orchestrator.go:824-866`; only the interaction differs |
| change scope | ✔ reviewed | ✘ | ✘ | `Rescope` | — |
| forget one routine | **✘** | ✔ `forget x` | ✔ | `forgetPlay` | Studio cannot delete a routine |

---

## 3. The three routine scopes, traced

The ruled vocabulary, adopted unchanged:

| mode | internal | available from | what Marco does |
|---|---|---|---|
| **Only in this application** | `ScopeApp` / context | while the app is in front | runs there; brings nothing forward |
| **From anywhere → this application** *(default)* | `ScopeAppAnywhere` / focus | anywhere | brings the app forward, then runs |
| **From anywhere → current foreground** | `ScopeEverywhere` / global | anywhere | runs into whatever is in front; no target app, no window-check guarantee |

The code already has exactly three, with these meanings in its comments (`draft.go:60-70`) and in
Studio's backend sentences (`rescope.go:226-241`). The divergence is in what each face *does*
with them.

### 3.1 How each mode behaves today

| | Only in this application | From anywhere → this application | From anywhere → current foreground |
|---|---|---|---|
| **generated route** | no `Activate`, **no foreground check** [source] `codegen.go:161-167,277` | leads with `do OS's Activate with "App"` | no `Activate` |
| **found by phrase** (widget/CLI arm 4) | only while App is the context (`Registry.Resolve` order) | from anywhere | from anywhere |
| **found by Director** | only while App is the context (`resolveroute.Eligible`) | anywhere | anywhere |
| **explicit identity** (`--play --app`) | **runs regardless of what is in front** — `--app` sets `req.App` to the route's app, not the foreground [source] `assistant.go:244-246`, `intake.go:112`. [traced, not executed] — this is G13 confirmed by reading | Activate by program name, or `--window` | presses into the foreground |
| **Studio test-Play** | Studio **chooses and raises** a window of App, passes `--window` — so it *does* bring it forward [source] `routines.go:497-521` | same as left | Studio **requires a chosen external window and raises it** — so it never presses "into the current foreground" (which would be Studio) |
| **widget** | runs if typed while App is in front; otherwise the phrase misses | runs from anywhere | runs into the game/app in front — the intended behaviour |
| **hotkey** | same as widget (resolved by name in the child's foreground) | same | same |

**Called out, not concealed:** Studio's labels say "switching nothing" for two modes, but Studio's
Play switches for all three. That is defensible — Studio is in front when you press Play, so a
test-Play has to hand the foreground to *something* — but it is a **test harness behaviour**, and
the label should say so rather than describe the scope (§5.2 layout).

### 3.2 Guards versus scope

`ScopeFor` (`draft.go:858`) forces **From anywhere → this application** for any recording with a
window-checked press (`RelTo`), and `WhyNotThisScope` refuses the other two at save and at rescope,
in all faces (`keep.go:259-271`). So today:

| recording | Only in this app | From anywhere → this app | From anywhere → current foreground |
|---|---|---|---|
| no window-checked press (keys, chords, typing) | ✔ | ✔ | ✔ |
| window-checked presses | **✘ refused** | ✔ (forced) | **✘ refused** |

The refusal of the third mode is what the revision asks for. **The refusal of the first mode is a
difference from the ruled table**, which implies "only in this application" is legitimate for a
guarded recording. It is refused today because a context route carries no `Activate`, and
`Activate` is what binds the window a guard checks against. Allowing it needs new behaviour —
**R1**.

Nothing here strips measurements: the generator refuses rather than dropping `RelTo`. That stays.

### 3.3 Defaults

| face | default today | ruled |
|---|---|---|
| Studio save | `app` (only in) `index.html:3314` | **from anywhere → this app** |
| `marco learn` prompt | Enter = only in `orchestrator.go:565` | same |
| widget `learn` | `marco learn` prompt answered in the HUD | same |
| `LearnAuto` (unattended) | `ScopeApp` `orchestrator.go:713-717` | same |
| `LearnVoice` / `chooseScope` | context `orchestrator.go:585-605` | same |

Five sites, one consistent default, all the opposite of the ruling. **R2**: change the default in
one place — a single `orchestrator.DefaultScope(dr)` that every site calls — so it cannot drift.
The guarded case already yields the ruled default.

---

## 4. Shortcuts: what the widget had, and why it went quiet

### 4.1 History

| when | commit | what |
|---|---|---|
| 2026-06-09 | `e0baa4d` | overlay UX: leader, command line, `bind` |
| 2026-07-05 | `ed31f5f`, `257b6d5` | **control centre** (`marco ui`, `cmd/marco/edit.go`): a browser page with Plays · **Bindings** · Config · Help; the overlay's `ui`/`edit` words open it |
| 2026-08-20 | `f576692` | control centre split into Here · Learn · Plays · Activity · **Bindings** · Settings · Help; `/api/bind` made to validate exactly like a press |
| 2026-10-27 | `b3f9a6e` (ADR-263) | **the installer opens Studio; the overlay starts only with `Marco.exe --overlay`** |
| 2026-11-02 | `10603eb` | wording only ("play" → "routine") |

### 4.2 Does it still work underneath?

Yes, every piece is present at HEAD [source]:

- `bind`, `unbind`, `ui`, `edit` are still listed words in the widget table (`commands.go:157-167`).
- `marco bind/unbind/hotkey/ui/edit` are still registered (`cmd/marco/main.go:48-120`).
- The control centre still serves `/api/bindings`, `/api/bind`, `/api/unbind`, `/api/oconfig`
  (`edit.go:161-166`) with tests (`editbind_test.go`). **No test starts `marco ui` itself.**

**Why it became unavailable:** not because anything was deleted. ADR-263 made Studio the default
front door (for the review, which only Studio can show) and put the whole widget behind
`--overlay`, and Studio links to neither the widget nor the control centre. An installed Marco
therefore offers no way to assign, list or see shortcuts, and no visible way to the widget at all.
ADR-263 itself says the overlay "is a different product surface rather than an older version of
this one" — this proposal takes that sentence at its word.

**Recover, don't replace:** the widget's words, `marco bind`, the leader and the press path are
kept as they are. The control centre's Bindings tab is the only list-and-edit UI that exists; its
one advantage (scope from the routine, not the browser) is the rule every face should use.

### 4.3 Per-app keys and global keys

The owner's two cases, both supported by today's store (`HotkeyCmd`: binding for the foreground
app first, then global) [source] `bindings.go:88-106`:

```
`e  in App A          → cloud pull stone      (routine: only in A, or anywhere → A)
`e  in App B          → enter freeplay        (routine: only in B, or anywhere → B)
`c  everywhere        → copy                  (routine: anywhere → current foreground; Ctrl+C)
`c  in App C          → something else        (shadows the global `c while C is in front)
```

The rule: **a shortcut may be narrower than its routine, never wider.**

| routine scope | shortcut may be listened for | default |
|---|---|---|
| only in App | App only | App |
| anywhere → App | App, another named app, or everywhere | everywhere |
| anywhere → current foreground | everywhere, or one named app | everywhere |

So a global `` `c `` for a guarded recording is impossible (the routine can't be in that scope in
the first place), and `` `e `` in App B cannot run a routine that works only in App A.

What breaks the two cases today: the control centre lands every global routine's binding as global
(so a per-app `` `c `` override cannot be made there); a second binding on the same app+key
silently replaces the first; nothing shows what `` `e `` does in the current app; and the binding
stores words, so a namesake or rename redirects it (G3–G5).

---

## 5. Proposed shared boundary

No new process. Three small owners in packages that already exist, each called by every face.

```
          Studio (Wails)         widget (overlay)          CLI (marco …)
           │ collects:            │ collects:               │ collects:
           │  clicks, forms,      │  leader keys, typed     │  argv, stdin
           │  chosen window       │  words, foreground      │
           ▼                      ▼                         ▼
   ┌───────────────────────────────────────────────────────────────────┐
   │ internal/orchestrator  (in-process for Studio; via marco for the  │
   │ widget, which stays a thin process-spawner)                        │
   │   Keep / Rescope / Rename / Simplify / Forget   ← one reviewed set │
   │   DefaultScope, WhyNotThisScope                 ← scope policy     │
   │   Shortcuts: Assign / Check / Remove / List / Carry  ← NEW, §5.1   │
   └───────────────────────────────────────────────────────────────────┘
   ┌───────────────────────────────────────────────────────────────────┐
   │ cmd/marco runInvocation → invoke.Decide        ← the one run door  │
   │   identity + --arg + --expect + --window + --source               │
   │   context-scope foreground check  ← NEW (G13), §5.3               │
   └───────────────────────────────────────────────────────────────────┘
   ┌──────────────────────────────┐  ┌────────────────────────────────┐
   │ Director RESOLVE_ROUTE       │  │ stopsignal · outcome · runref  │
   │ (language → identity) as now │  │ as now                         │
   └──────────────────────────────┘  └────────────────────────────────┘
   store: marcopaths library · bindings.json · overlay.json (one reader)
```

### 5.1 Shortcuts — one owner

`orchestrator.Shortcuts` over `routes.Registry`:

- `Check(route ID, listenIn app|"", key) → free | taken(binding, version) | shadows/shadowed-by | reserved | wider-than-routine | recording-in-progress`
- `Assign(route ID, listenIn, key, consent)` — `consent` is the **version of the binding the
  person was shown** on that app+key (a digest over the stored entry, or "free" for none),
  re-read and compared **under the bindings lock** at the write. A route ID is not consent: the
  routine on the key can stay the same while its binding (chain, values, scope) changes.
- `Remove(listenIn, key, consent)` with the same version rule; `List(app)` → what each key does
  **in app** (app bindings, then un-shadowed globals); `Carry(oldID, newID)` called by
  `Rescope`/`Rename` inside their lock; **`Forget` removes the routine's shortcuts** (R6).
- `Binding` gains `Route` (an ID); `Cmd` is kept for chains, configured values and legacy entries.
- **Legacy name-only bindings** (R6) are kept on disk untouched, **do not run**, and say why: the
  press refuses with *"`e was set up by name before shortcuts named a routine — reassign it to
  run it"*; `List` and every surface show them as *needs reassigning*. Reassigning is an ordinary
  `Assign` with the legacy entry's version as consent.

Faces then differ **only in how they collect `listenIn`**:

| face | `listenIn` comes from |
|---|---|
| Studio | an explicit selector, defaulted from the routine's scope (§4.3) — never the foreground |
| widget `bind <key> <routine>` | the foreground at the moment it is typed (today's meaning, kept) — but the routine is resolved once to an ID and the scope rule is enforced |
| widget `bind <key> <routine> everywhere` | **new word** for the global case from the widget |
| CLI | `marco bind` as today; `--everywhere` / `--app=<name>` added |
| control centre | its selector, defaulting from the routine (today's `bindScope`) |

### 5.2 Run — one door, explicit target provenance

Every face already spawns `marco do`. Keep that. Close the gaps:

1. **Studio passes `--source=studio`** (trace correctness).
2. **The widget names a routine it already knows by identity.** Tab-completed or listed rows carry
   the ID; a free phrase still goes through `Decide` arms 4–6 unchanged.
3. **Target provenance is stated by the entrance, not re-guessed:**

| entrance | target | provenance |
|---|---|---|
| Studio test-Play | the window chosen in Studio, raised by handle, `--window` | `chosen` — Studio's front-ness is excluded by construction |
| widget phrase / hotkey | the foreground at the keypress, read in the child | `foreground` — the widget never takes focus (`main.go:122-132`, click-through, hook-only) |
| widget command line | same | same — *the open command line does not steal focus* |
| CLI | foreground, or `--window` | stated |

Studio's labels change so that "switching nothing" describes the routine and the run strip says
what *this test* does: *"Play from Studio brings Untitled – Notepad forward first."*

### 5.3 The context-scope check (G13)

**Built in this change** (see §14). One shared rule, `orchestrator.AdmitInPlace`, asked by
`performOnePlay` — the single place every routine performance passes, whatever arm decided it —
immediately before the authority door:

- the route is **Only in this application** (app set, focus not set);
- if the run was **handed a window**, the program that owns it **as validated by the host**
  (`oshost.GiveWindow`: alive, owner pid unchanged, image read from the process) must be the
  route's application. A `--window` naming any other program's window is refused — supplying the
  flag is not admission;
- otherwise the application in front **at that instant** (`winctx.Active`, read there and not
  carried from the intake) must be the route's application; an unreadable foreground refuses;
- `Source` is not consulted.

Studio's Play satisfies it by construction: it hands over a window of the routine's application
that it has just raised. Focus and current-foreground routines are not affected.

### 5.4 Arguments from the keyboard console

**Preferred UX: deterministic command templates and completion first; Director extraction for
freer phrasing afterwards.** The widget behaves like a small CLI over the same route-and-values
contract Studio uses. No model is involved in this syntax.

The invocation is displayed as a template — **cloud pull** `‹resource›` — and all three of these
produce the **same explicit identity and `{resource: "stone"}`**:

```text
cloud pull stone
cloud pull with stone
cloud pull resource:stone
```

- **An exact routine name wins.** If `cloud pull stone` is itself a routine, it is that routine —
  never reinterpreted as `cloud pull` + an argument. This is today's whole-phrase-first order
  (`grammar`, `intake.go:342`, `TestAPlayNamedWithTheGrammarIsStillReachable`), kept.
- **Multiple arguments need clear boundaries.** Positional filling is offered only when it is
  unambiguous (one argument, or one remaining); with several, named syntax (`name:value`) is
  available for values with spaces, and an ambiguous line is **asked about, not guessed**.
- On screen: a muted `‹resource›` hint after completing `cloud pull`; supplied values in a
  distinct colour **plus** their label (colour never carries meaning alone); a missing value says
  **"Resource needed · cloud pull stone"**; a compact read-back **Cloud pull · resource: stone**
  before running. `{{resource}}` stays the internal placeholder.
- The values travel as `--arg=name=value` with the identity. That is safe here, unlike an
  extracted proposal, because the person typed them and the read-back shows them before Enter
  runs it.
- `vkToRune` gains the punctuation a value needs (`: , . / ' "`, shifted digits) — in the hook,
  with no I/O.
- **Director's extracted values stay proposals** (Amendment 1, correction 2): the widget may show
  them in the read-back for the person to confirm or correct, through the ADR-277 provenance
  flow, and never forwards them unreviewed.
- **No fixed-value authoring** on shortcuts in this work; existing configured values keep working
  (§10).

This is the order of resolution for a typed line: whole phrase as a routine name → template
match against a routine's declared arguments → Director. Only the middle step is new, and it
produces an identity, so it reaches the door through arm 3 rather than as words.

### 5.5 Edits

`marco rename` calls the reviewed `PrepareRename` → `Rescope` with consent given by the person
who typed it (the CLI's ENTER-MEANS-NO question, ADR-266). `marco simplify` is **left alone** — it
is already the reviewed path (verified, §8).
Studio gains **Forget** for one routine (R8): a confirmation naming the routine, its scope, its
demonstration and its shortcuts; the result reports each part removed or not removed, through
`forgetPlay`'s logic moved into the orchestrator.

### 5.6 Configuration

One zero-dependency reader/writer for `overlay.json` in `internal/` used by the widget, the
control centre and Studio, which **preserves unknown fields** on every write (a read-modify-write
over a raw map, not a struct round-trip — the struct round-trip is what loses `wake` today, F5).
Studio reads the leader so its shortcut line shows the real key. Library and resolver stay where
they are.

### 5.7 Reachability (R5 — follow-on, separately reviewed)

- After the shared wiring: Studio Help gains *"Use Marco while you play"* with a **Start the
  widget** button (the bundled overlay stack, as `Marco.exe --overlay`), and shows whether it is
  running.
- The widget stays a separate process and a separate choice — not removed, not replaced by "open
  Studio". Login startup is deferred; shared themes stay on the roadmap.

### 5.8 Recording, arming, locks

- **A recording in progress blocks routine shortcuts.** Needs a "recording now" fact both faces
  can read; the shape (a file under `$MARCO_HOME` beside `stopsignal`, written by `Begin`, cleared
  by `Finish`, carrying the session and the recorder's pid so a dead recorder's file is ignored)
  comes back for review with the shortcut slice. Whether today's recorder admits input a shortcut
  injects is **not demonstrated** either way, so the block is stated as a requirement, not as a
  fix for an observed defect.
- **Arming stays two seconds.** The arm list (§6.3) is help, never a gate, and does not stretch
  the window in the first increment.
- **One lock order:** a route lock (`Registry.Lock`, per route) is taken **before** the bindings
  lock, never after; an operation needing two route locks takes them in route-ID order. `Carry`
  and `Forget` hold the route lock(s) and then the bindings lock; `Assign`/`Remove` take only the
  bindings lock and **re-read** the route under it rather than taking its lock, so no path takes
  bindings → route. Documented on both lock functions and held by a test that walks callers.

---

## 6. Journeys

### 6.1 Build and test in Studio

1. Record → choose window *Notepad* → demonstrate → Stop.
2. Review; scope selector opens on **From anywhere → Notepad** (ruled default).
3. Save. Play: Studio raises the chosen Notepad window, runs, shows the result line.

### 6.2 Assign its shortcut — from either face

- **Studio:** open routine → **Shortcut: none · Set…** → press `e` → *Listened for: Notepad*
  (default from scope) → Check says free → Save.
- **Widget:** in Notepad, `` `m `` `bind e save draft` Enter → HUD: *"`e in Notepad → Save draft"*.
- Same `orchestrator.Shortcuts.Assign`; same file; same result.

### 6.3 Invoke it while playing

`` ` `` → the HUD lists the keys that fire **here** (`Shortcuts.List(app)`), fetched off the hook
thread → `e` → `marco hotkey e` → bound **identity** → the door → result in the HUD and in its
history.

### 6.4 Supply changing text

`` `m `` → type `cloud p` → Tab completes **cloud pull** and shows a muted `‹resource›` → type
`stone` → the read-back shows **Cloud pull · resource: stone** → Enter → the identity runs with
`--arg=resource=stone`. A shortcut bound to an argument routine with no configured value refuses
with **"Resource needed · cloud pull stone"** and opens the command line on `cloud pull ▌`.

### 6.5 Stop and understand

`` ` `` while running → stop (unchanged) → HUD: *"Greet — stopped by you after 2 of 5 steps"*
from the same `[result]` line Studio reads; Studio's history shows the same run, source `hotkey`.

---

## 7. Layouts (text mockups, not rendered)

### 7.1 Studio — save, scope selector (all three modes)

```
 Save "Save draft"
 Where it works
  (•) From anywhere → Notepad       brings Notepad forward, then runs   ← default
  ( ) Only in Notepad               runs only while Notepad is in front
  ( ) From anywhere → current foreground   no target app, no window checks
 [ Save ]
```

Guarded recording (today's rule):

```
 Where it works
  (•) From anywhere → Notepad       brings Notepad forward, then runs
  ( ) Only in Notepad               — not available: 3 clicks check the Notepad window,
                                      and this mode brings no window forward   [R1]
  ( ) From anywhere → current foreground
                                    — not available: those clicks need Notepad's window.
                                      Record it again without window-checked clicks,
                                      or keep this mode.
```

The **Where it works** tab on an open routine uses the same three rows, through `ScopeChoices` →
`PrepareRescope` → reviewed `Rescope` (no second scope system).

### 7.2 Studio — run strip

```
 Save draft                                  [Rename]
 From anywhere → Notepad · Play brings "Untitled - Notepad" forward first
 Shortcut  `e  in Notepad                    [Change] [Remove]
 [ Play ]                                    Simplify…
```

Only-in mode: `Only in Notepad · Play from Studio brings "Untitled - Notepad" forward to test it`.
Current-foreground mode: `From anywhere → current foreground · Play from Studio presses into
"Untitled - Notepad" (chosen below)`.

### 7.3 Studio — shortcut capture, per-app and global

```
 Shortcut  `[ press a key ]   listened for in  [Notepad ▾ | everywhere | other app…]  [Cancel]
```

```
 Shortcut  `c  everywhere
           In Paint, `c runs "Paint copy" instead.          (shadowing — said, not refused)
```

```
 Shortcut  `e  in Notepad
           `e already runs "Save as PDF" in Notepad.
           [Replace — Save as PDF loses `e]  [Pick another key]
```

```
 Shortcut  `e  everywhere — not available: "Save draft" works only in Notepad.
```

### 7.4 Studio — library chips and stale shortcuts

```
 Save draft          From anywhere → Notepad      `e Notepad
 Copy                From anywhere → current foreground   `c everywhere
 Shortcuts that no longer run anything
   `s  in Notepad   ran "Old draft", which was removed            [Remove]
```

### 7.5 Widget — armed

```
 `  in Notepad
   e  Save draft
   g  Greet            needs message
   c  Copy             everywhere
 m command line · h help · Esc closes
```

In App B:

```
 `  in Game B
   e  Enter freeplay
   c  Copy             everywhere
```

### 7.6 Widget — command line, values, refusal, result

```
 > bind e enter freeplay            → `e in Game B → Enter freeplay
 > bind c copy everywhere           → `c everywhere → Copy
 > bind e save draft                → refused: Save draft works only in Notepad
 > cloud pull ‹resource›                       (muted hint after Tab)
 > cloud pull stone▌                           Cloud pull · resource: stone
 > cloud pull                                  Resource needed · cloud pull stone
 > greet name:sam message:see you at 8▌        Greet · name: sam · message: see you at 8
 > greet sam see you at 8                      Two values needed — say which: name:… message:…
 `e  was set up by name before shortcuts named a routine — reassign it to run it
 `e  Save draft — performed (5 steps) · 1.2 s
 `e  Save draft — refused: Notepad is not in front (only in Notepad)
 `x  nothing on x in Game B
```

---

## 8. Findings, each verified on its own

Every row was re-read at the cited lines by the author of this note; a delegated claim that could
not be confirmed that way is listed as unverified, not as a defect.

| # | finding | status | evidence |
|---|---|---|---|
| F1 | an explicit identity performs an "Only in" routine with any app in front | **[source] confirmed by reading; fixed in this change (§14)** | `assistant.go:244-246` sets `req.App` from `--app`; `intake.go:112` keeps it; `codegen.go:161-167,277` gives a context route no `Activate` and no check |
| F2 | Studio runs recorded as source `cli` | [source] | `plugins/studio/routines.go:553` passes no `--source`; `assistant.go:228` defaults to `SourceCLI` |
| ~~F3~~ | ~~Studio find box needs Director for an exact name~~ | **withdrawn**: deliberate (ADR-267) | none |
| ~~F4~~ | ~~widget offer drops proposed values~~ | **withdrawn as a defect**: `resolved` has no values field (`overlay/resolve.go:42-60`) and `runOfferedRoutine` sends none (`acts.go:1183-1193`); that is the ruled behaviour until a review flow exists | [source] |
| F5 | a widget save of `overlay.json` drops `wake` | [source] | the widget's `Config` has no `Wake` (`overlay/config.go:26-41`) and `saveConfig` marshals the struct (`:105-117`); the control centre's has it (`cmd/marco/oconfig.go:30`) |
| F6 | Enter means yes to the widget's own offers and no to `AskFirst` | [source] | `controller_windows.go:298,305` (`ans == ""` runs); `controller_windows.go:392` sends the empty answer; `authority.go:262-275` refuses anything but `y`/`yes` |
| F7 | widget `narrate learn` with more than one window asks "Which window" on stdin, which the widget feeds with narration; and the chosen window is discarded | [source] traced, not executed | overlay spawns `learn --narrate <name>` with no `--window` (`acts.go:603`); `runAssistantLearn` → `chooseTheWindow(d, "", false)` prompts on stdin (`assistant.go:794-802, 697-702`); `runNarrateLearn` builds a fresh `newDeps()` (`assistant.go:883`) |
| F8 | the widget's learn offer is reachable only in a race | **unverified**: depends on Director timing; not confirmed by reading alone | delegated: `overlay/outcome.go:112-114`, `overlay/resolve.go:156-163` |
| F9 | hotkey runs leave no status line or history row in the widget | [source] | `Hotkey` uses `runMarco` → `streamChild` (`acts.go:295,1214`), not `runRecordTracked`, which is where `setStatus`/`addHistory` happen (`acts.go:1001-1012`) |
| F10 | the default scope is "only in" on five sites | [source]; **fixed in this change** | §3.3 |
| F11 | `marco rename` bypasses the reviewed move | [source] | `assistant.go:619-643` → `Reg.Rename` |
| (none) | `marco simplify` bypasses review | **refuted** | `SimplifyRoute` → `Prepare`/`Replace` (`orchestrator.go:824-866`) |
| (none) | the widget never takes the foreground | **not demonstrated**: the flags intend it (`overlay/main.go:121-131`: `InitUnfocused`, mouse passthrough, `RunnableOnUnfocused`); no path shows Windows honours them in every case | none |
| (none) | a Studio recording admits input a widget shortcut injects | **not demonstrated** either way | none |
| G1–G13 | binding gaps | see [[Shortcuts-from-Studio-proposal-2026-11-02]] §1.2; G13 = F1 | |

---

## 9. Rulings

Ruled 2026-11-02; see Amendment 1 (§0) for the text. No ruling is outstanding for the foundation.
Open for the later increments: R1's generation/compatibility review, and the shape of the
"recording now" signal (§5.8).

---

## 10. Compatibility plan

| existing | after |
|---|---|
| `bindings.json` entries that name a routine by words (`Cmd`/`Slug`) | **kept on disk untouched; they stop running** and refuse with a sentence naming the key and saying to reassign it; every surface lists them as *needs reassigning* (R6). Behaviour change, in the shortcut increment, not in this one |
| configured values in a binding (`` `e = enter freeplay with 3 ``) | kept and still applied once the binding is reassigned to an identity; no new authoring UI |
| chains (`a then b`) | kept as `Cmd`; reassigning a chain is a CLI/control-centre operation |
| `marco bind <k> "<cmd>"` | unchanged surface; resolves once to an identity and binds that |
| widget `bind`/`unbind` words | unchanged meaning (foreground scope); plus `everywhere` |
| control centre Bindings tab | kept; calls `Shortcuts` |
| `marco ui`, `edit`, `--overlay` | unchanged |
| forgetting a routine | removes its shortcuts (R6) |
| **`marco learn` Enter at the scope question** | **From anywhere → this application** (R2). **Changed in this change**, and said in the prompt |
| **`marco learn --auto` / `LearnVoice` with no application** | **refused / asked** instead of saved for the current foreground (R2). **Changed in this change** |
| **`marco do --play X --app A`, X only-in-A, A not in front** | **refused** (R3). **Changed in this change** |
| `marco rename` | reviewed move with consent (R7), later increment |
| `marco simplify` | unchanged (already reviewed) |

Nothing working in the widget is disabled or replaced with "open Studio".

---

## 11. Implementation sequence

Each step ships alone; every face keeps working after each. **Each step that adds or changes a
control is preceded by a rendered interface** (a headless render of the page with stubbed
bindings, per CLAUDE.md, or a rendered mockup for the HUD) and a review of it.

- **S0: scope vocabulary and default. Done in this change (§14).**
- **S1: run-time admission for in-place routines. Done in this change (§14).**
  Studio `--source=studio` (F2) is folded into S3, because it changes Studio code.
- **S2: shortcut owner. Done (§15, [[ADR-297-a-shortcut-names-its-routine-and-has-one-owner]]).** `orchestrator.Shortcuts` (Check/Assign/Remove/List/Carry/Forget),
  `Binding.Route`, binding-version consent, the bindings lock and the lock order (§5.8), legacy
  bindings refuse (R6), the "recording now" fact blocks shortcuts; `bindKey`, the control centre
  and the widget words call it; `everywhere`; `marco bindings` (list). CLI only, no new control.
- **S3: Studio, rendered first.** Then the scope labels, the run-strip sentence per scope,
  `--source=studio`, the shortcut line and library chips (§7.1–7.4). **The shortcut controls and
  library chips are done** ([[ADR-298-studio-sets-shortcuts-through-the-one-owner]]); the scope
  labels, the run-strip sentence and `--source=studio` are not started.
- **S4: the widget explains itself, rendered first.** Arm list off the hook thread, "nothing on
  x", configured leader in the echo, hotkey runs tracked (F9), one Enter meaning (F6).
- **S5: one `overlay.json` reader preserving unknown fields** (F5).
- **S6: command templates and completion (§5.4), rendered first.** Then Director proposals in the
  read-back through the ADR-277 flow.
- **S7: edits converge.** `marco rename` reviewed (R7); Studio Forget-one (R8); narrate's window
  (F7).
- **S8: Start the widget from Studio (R5), separately reviewed.**
- **S9: R1, after its generation/compatibility review.**

---

## 12. Acceptance tests

Synthetic fixtures; temporary `$MARCO_HOME`/`$MARCO_ROUTES`; a stated foreground; the dry-run
host. Tests marked **(built)** exist after this change; the rest belong to later increments.

| test | enters | asserts |
|---|---|---|
| **(built)** default scope | `orchestrator.DefaultScope` | unguarded + app → app-anywhere; guarded → app-anywhere; no app → none, with a reason |
| **(built)** Enter at the scope question | `askScope` with an empty line | app-anywhere, and the prompt names it as the Enter default |
| **(built)** no application is asked, not saved for the foreground | `askScope` with no app, answer empty / `y` | empty → nothing saved; `y` → current foreground |
| **(built)** unattended with no application | `LearnAuto`'s scope choice | refused, nothing written |
| **(built)** one vocabulary | `orchestrator.ScopeLabel` per scope; CLI prompt; `saidKept` | the three ruled labels, from one table |
| **(built)** only-in, another app in front | `AdmitInPlace`, and the intake with foreground B | refused, nothing performed |
| **(built)** only-in, its app in front | same, foreground A | admitted |
| **(built)** a handed-over window of another program is not admission | `AdmitInPlace` with a validated window of program B | refused although a window was given |
| **(built)** a handed-over window of the application is admission | validated window of A, foreground Studio | admitted (Studio's handoff) |
| **(built)** unreadable foreground | foreground "" | refused |
| **(built)** focus and current-foreground routines are not checked | app-anywhere / global routes, foreground B | admitted |
| **(built)** source is not admission | every `invoke.Source` | identical verdicts |
| Studio Play of an only-in routine | `doCommand` + child with a matching window | admitted; argv carries `--window`, `--source=studio` (S3) |
| a guarded recording cannot be saved for the current foreground | `Keep(ScopeEverywhere)` on a `RelTo` draft | refused with the remedy; no `RelTo` removed |
| scope choices show all three with reasons | `ScopeChoices` on guarded and unguarded routines | three rows; unavailable rows carry `Why` |
| the same key runs different routines per app | `Assign(e, A→X)`, `Assign(e, B→Y)`, `pressHotkey` in A and B | X in A, Y in B |
| an app key shadows the global key | global `c → copy`, `c` in C → Z | copy in A; Z in C; `List(C)` reports the shadow |
| a shortcut cannot be wider than its routine | `Assign(e, everywhere → only-in-A routine)` | refused |
| a global shortcut for a current-foreground routine | `Assign(c, everywhere → copy)` | accepted; a press in any app runs copy |
| consent is to the binding version shown | `Assign` with a stale version after the binding changed but kept its routine | refused under the lock |
| a legacy binding refuses until reassigned | old-format `bindings.json` → `pressHotkey` | refused with the reassign sentence; file unchanged |
| forgetting a routine removes its shortcuts | `Forget` → `List` | gone |
| a recording blocks shortcuts | "recording now" fact set → `pressHotkey` | refused, nothing performed |
| the lock order is route then bindings | caller walk over lock sites | no bindings → route acquisition |
| every face assigns through one owner | `marco bind`, `/api/bind`, widget `bind`, Studio | each reaches `Shortcuts.Assign` (mutation: bypass one, fails) |
| Studio's shortcut scope is never the foreground | Studio assignment with foreground = Studio | lands in the chosen app or everywhere |
| the arm list is what a press would fire | `List(app)` vs `pressHotkey` per key | same answer per key |
| a template invocation reaches one identity | `cloud pull stone`, `cloud pull with stone`, `cloud pull resource:stone` | same identity, `{resource: stone}`, no model asked |
| an exact name wins over a template | routines `cloud pull` and `cloud pull stone` | `cloud pull stone` runs the latter, no values |
| ambiguous positional values are asked about | two-argument routine, positional line | asked, nothing run |
| extracted values are never forwarded unreviewed | widget offer with Director `Values` | argv has no `--arg` until a confirmation |
| one overlay config keeps unknown fields | widget save over a file with `wake` and an unknown key | both preserved |
| arming stays two seconds | controller arm timeout | 2 s |
| a hotkey needs no model | `pressHotkey` with a recording Director stub | zero calls |

Each new guard gets a mutation run, per the repository's wiring practice.

---

## 13. Not established

- That the widget never holds the foreground (flags only; §8).
- What a Studio recording does with input a shortcut injects (§8, §5.8).
- F8.
- Whether R1's "bind the foreground window without raising" can be done with existing `oshost`
  calls. That is part of R1's review.

---

## 14. What this change built (S0 + S1)

Recorded in [[ADR-296-one-scope-vocabulary-one-default-and-in-place-is-checked-at-the-door]].

| piece | where |
|---|---|
| `ScopeLabel`, `ScopeMeans` (the three ruled names), `DefaultScope`, `AdmitInPlace` | `internal/orchestrator/scopeterms.go` (new) |
| terminal question: Enter = From anywhere → app, says Enter changed; no app = `[y]es/[N]o` | `orchestrator.go` `askScope`, `askAmongScopes` |
| narration: same question; no app asked | `orchestrator.go` `chooseScope`, `saveTaught` |
| unattended save: shared default; no app = not saved | `orchestrator.go` `LearnAuto` |
| save line uses the shared label | `orchestrator.go` `scopeLabel` |
| Studio chooser opens on the backend's default | `plugins/studio/recording.go` `Recording.DefaultScope`, `index.html` (one line) |
| validated window owner carried to the door | `cmd/marco/assistant.go` `handOverTheWindow` → `Deps.HandedTo` |
| the admission check | `cmd/marco/intake.go` `performOnePlay` |

Not changed, deliberately: Studio's chooser **labels** and run-strip sentences (S3, after a
render), the widget's listing (S4), shortcuts (S2), `--source=studio` (S3).

Tests updated because they pinned the old default rather than what they test: ten
`stopbutton_test.go` lookups and two `keep_test.go` lookups now read the `focus` path that the
unattended save uses; `scope_test.go`'s confirmation text; the no-application test replaced by
the ruled behaviour.

---

## 15. What S2 built

Recorded in [[ADR-297-a-shortcut-names-its-routine-and-has-one-owner]]. CLI and backend only: no
surface control changed.

| piece | where |
|---|---|
| the owner: assign, check, remove, list, carry, forget, press | `internal/orchestrator/shortcuts.go` (new) |
| identities in the store, version digest, `ParseID`, bindings lock, atomic locked update | `internal/routes/bindings.go`; `lockFile` shared with the route locks |
| the "recording now" fact | `internal/recordingnow` (new): one file per recording process, pid + start time |
| announced by `Begin`, cleared by `Finish` | `internal/orchestrator/draft.go`; `Deps.RecordingHome` from `newDeps` and Studio's recording |
| `marco bind [--everywhere\|--in=<app>]`, `--reassign`, `unbind`, `bindings [--here] [--json]`, `hotkey` | `cmd/marco/bind.go` |
| control centre bind/unbind/list through the owner | `cmd/marco/edit.go` |
| forget removes; move carries; replace drops first | `forgetPlay`, control-centre delete and scope move, CLI rename, `Rescope` |

Differences from §5.1 as proposed, each smaller rather than larger:

- `Check` is `WhyNotShortcut` + `ShortcutAt`; there is no separate `recording-in-progress` state at
  assignment — the hold-off is at the press, where the input would be injected.
- The widget's `everywhere` word is **not** added: the word table is a widget surface. The CLI has
  `--everywhere` and `--in=<app>`.
- Legacy entries are matched by name only when a routine is forgotten (removing one that would have
  reached it is the safe direction); a move never touches them.

### Corrections after review

The first S2 version had three P1 gaps (cleanup after the identity changed; assignment outside
the route locks; recording admission failing open). All three are closed by ordering, recorded in
[[ADR-297-a-shortcut-names-its-routine-and-has-one-owner]] under *Corrections after review*. Two
additions to the design above came with them: a press PINS the bytes of each routine it resolved
(`invoke.Request.Pinned`), and Director's three recording entrances announce themselves too.

A second review found one more lifecycle case: the CLI's unreviewed rename carried shortcuts first
and rolled them back on failure, so a rename and a rollback that both failed left keys that the
next routine saved under the new name would have answered. `RenameInPlace` now keeps the original
until the destination holds the routine AND its shortcuts have moved (copy, carry, finish), and
nothing is rolled back. Recorded under *Second review* in the same ADR.
A failed create or carry deletes nothing: what is at the new name is left and reported, because
bytes are not evidence of who wrote a file. And by ruling, a replacement somebody agrees to keeps
the routine's shortcuts (a new version of the same routine) and its question says so once; Studio's
Replace card is to say the same in S3.
