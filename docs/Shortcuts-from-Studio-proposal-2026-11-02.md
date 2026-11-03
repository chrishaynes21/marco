---
type: reference
status: proposal
date: 2026-11-02
source_paths:
  - internal/routes/bindings.go
  - internal/routes/registry.go
  - cmd/marco/bind.go
  - cmd/marco/edit.go
  - cmd/marco/panicstop.go
  - plugins/overlay/controller_windows.go
  - plugins/overlay/acts.go
  - plugins/overlay/intake.go
  - plugins/overlay/config.go
  - plugins/studio/routines.go
  - plugins/studio/acts.go
  - plugins/studio/frontend/index.html
---

# Leader shortcuts from Studio — trace and proposal

**Status: SUPERSEDED by [[Surfaces-alignment-proposal-2026-11-02]].** Its binding trace (§1) is
still referenced from there. Nothing here is built. No production code changed.
Branch `feat/demonstrated-acts`, HEAD `6c0498b`, tree clean at the time of the trace.

The owner's framing, which this proposal adopts: **the leader shortcut already works; what is
missing is assigning and maintaining shortcuts from Studio.** So the overlay press path, the
binding store and `marco bind`/`unbind`/`hotkey` are the mechanism to keep. The work is a Studio
surface on top of them, plus the smallest store changes needed so that a shortcut assigned from a
routine card runs **that** routine and no other.

Evidence labels used below:

- **[source]** read in the code at HEAD, with the location given.
- **[not established]** believed, or plausible, but not traced to a line or exercised.
- **No headless render was made.** The layouts are text mockups of a proposed UI, not
  screenshots of anything.

Related: [[Invocation]] (the door a press enters), [[ADR-084-a-plays-identity-is-its-subject]],
[[ADR-267-director-owns-route-resolution]], [[ADR-294-the-library-is-the-screen]],
[[ADR-277-a-request-may-propose-the-text-a-routine-types]].

---

## 1. Current behaviour

### 1.1 What works, and should be kept as it is

| step | what happens | where |
|---|---|---|
| leader configured | `overlay.json` `leader`, cycled among `` ` `` `/` `capslock` `tab` `f8`; default `` ` `` | [source] `plugins/overlay/config.go:34,49,61` |
| leader pressed (idle) | arms for 2 s; a press while a routine is running **cancels it** instead | [source] `controller_windows.go:163,482-492` |
| next key | `m` opens the command line, `h` opens help, leader/Esc dismiss, **any other key → `actHotkey`** | [source] `controller_windows.go:457-479` |
| hotkey act | `marco hotkey <key>` spawned and its output streamed into the HUD | [source] `overlay/acts.go:286-299`, `runMarco` → `streamChild` |
| app context | `marco hotkey` reads `winctx.Active()` **in the spawned child**. The overlay takes no foreground for a leader press (it is a low-level hook), so this is the target application | [source] `cmd/marco/bind.go:96-101`; "never takes foreground" is [not established] live, but the hook design implies it |
| lookup | `HotkeyCmd(app,key)`: an app binding first, then a global one | [source] `internal/routes/bindings.go:88-106` |
| performance | each chained step resolved **once** to a `routes.Route` and handed to `runInvocation` as an explicit identity (`invoke.Request.Play`), so `Authorize`, guards, stop and `[result]` are the ordinary door's | [source] `bind.go:110-155`; `TestAHotkeyPerformsTheBoundPlayItself` |
| arguments | a binding's command may carry its own (`greet name:sam`); a routine that needs a value and gets none is **refused** with *"This request needs text values reviewed before running. Open it in Studio, or supply the value with the request"* — never typed as empty | [source] `cmd/marco/panicstop.go:274-308` |
| assignment validated | `bindKey` refuses a step that would not resolve **exactly** (the old 0.6 fuzzy accept is gone) | [source] `bind.go:36-62`; `TestABindingIsValidatedTheWayItWillBeResolved` |
| no model | nothing on the press or bind path asks Director, `resolveroute`, the resolver plugin or Ollama | [source] `bind.go` imports; `invoke.Decide` arm 3 |

Ways to assign today: `marco bind <key> "<routine>"` in a shell, `bind <key> <routine>` typed on
the overlay command line (which spawns the same `marco bind`, `overlay/intake.go:122`), and the
**Bindings tab of the `marco edit` control centre** (`cmd/marco/edit.go:161-163,541-...`).

### 1.2 What is missing, hidden or wrong

These are the gaps. They are listed with what kind of gap each one is, because most are a missing
control and not a broken mechanism.

| # | finding | kind | where |
|---|---|---|---|
| G1 | **Studio has no shortcut control at all.** An open routine has Rename, Steps, Other names, Where it works, Source, More; nothing about a leader key | missing UI | [source] `index.html:1564-1790` |
| G2 | **Studio cannot reuse `marco bind` as it is.** `runBind` scopes the binding to `winctx.Active()` — with Studio in front, that is Studio, or whatever the platform reports for it. The binding would land under the wrong application and never fire | wiring hazard | [source] `bind.go:21` |
| G3 | **A binding stores WORDS, not an identity.** `Binding{App,Key,Cmd}`; the press re-resolves `Cmd` by name with `Registry.Resolve`'s four-scope priority. A press is "resolved once", but once **per press**, by name | store shape | [source] `bindings.go:15-20`, `bind.go:131-133`, `registry.go:428-448` |
| G4 | **Namesakes:** bind `s` (global) to the **global** `save`; in an app that also has a **context** `save`, pressing `s` runs the context one, because `Resolve` tries the foreground app's context route first. The control centre's `bindScope` also takes the **first** name match | consequence of G3 | [source] `registry.go:428-448`, `edit.go:554-570`. Traced, **not executed** |
| G5 | **Rename leaves the binding pointing at a name.** Nothing in `internal/orchestrator` or the rename/forget commands touches `bindings.json`. After a rename the key silently does nothing; if anything is later saved under the old name, the key **silently runs that** | stale binding, silent redirect | [source] grep: no binding code in `internal/orchestrator` |
| G6 | **Studio's Forget everything keeps hotkeys on purpose** — *"a key bound to a name works again as soon as you demonstrate it again"*. That is redirect-by-name stated as a feature, and it conflicts with "never silently redirect a stale binding" | **policy conflict — needs a ruling (D1)** | [source] `plugins/studio/acts.go:1681-1714` |
| G7 | **An unbound key, and a step whose routine is gone, are silent.** The leader echo appears and nothing else happens | missing feedback | [source] `bind.go:104-106,134-136` |
| G8 | **Arming shows nothing.** No list of the shortcuts available here. The overlay never reads `Bindings()`; only the control centre does | missing UI | [source] `controller_windows.go:202-205`; grep of `Bindings()` callers |
| G9 | **Collisions are silent.** `Bind` replaces an existing app+key binding without saying so | missing feedback | [source] `bindings.go:55-66` |
| G10 | **`m` and `h` can be bound and can never fire**, because the armed handler claims them first. Neither `bindKey` nor the control centre refuses them | validation gap | [source] `controller_windows.go:463-468` |
| G11 | **The echo always shows `` ` ``**, whatever leader is configured | cosmetic | [source] `controller_windows.go:204`, `acts.go:317` |
| G12 | **`bindings.json` is written in place with no lock.** Two writers (Studio and the overlay, say) can lose one another's change. `orchestrator.Keep`'s route lock does not cover it | concurrency | [source] `bindings.go:44-53` |
| G13 | **Does an explicit identity re-check a context routine's application?** A press today can only reach a context route of the foreground app, because `Resolve` only offers that. Once a binding carries an identity, the intake must refuse a context route while another app is in front. I found no such check on the explicit-identity path | **[not established]** — must be answered before S1 lands |

---

## 2. The smallest missing behaviour

1. **A binding names a routine by its identity** — `global/<slug>`, `<app>/context/<slug>`,
   `<app>/focus/<slug>`, the same `routes.Route.ID()` Studio already hands Play
   (`routines.go` `Routine.ID`). A press performs that route or refuses. It never looks for
   another routine by name.
2. **One assignment function**, used by the CLI, the overlay's `bind` word, the control centre
   and Studio. It takes an **explicit route and an explicit shortcut scope** and never reads the
   foreground (fixes G2). It refuses reserved keys and reports collisions.
3. **Studio can assign, change and remove a shortcut on an open routine**, and the library shows
   which routines have one.
4. **The overlay says something** on arm (what is available here), on an unbound key, and on a
   stale binding.

Everything else — the leader, the press, `Authorize`, stop, `[result]`, argument refusal — is
reused unchanged.

---

## 3. Proposed flows

### 3.1 Shortcut scope versus routine availability

These are two facts, and the UI states both.

- **Where the routine works** is its route scope (Studio's *Where it works* tab).
- **Where the shortcut is listened for** is the binding's `App` (`""` = everywhere).

The rule: **a shortcut may be narrower than its routine, never wider.**

| routine scope | shortcut may be | default |
|---|---|---|
| in *App* (context) | in *App* only | in *App* |
| from anywhere, opens *App* (focus) | in *App*, or everywhere | everywhere |
| anywhere (global) | everywhere, or in one named app | everywhere |

A press still goes through `Authorize` and the route's own scope. The shortcut cannot widen what
the routine is allowed to do, and that holds even if the binding file is edited by hand —
provided G13 is closed.

### 3.2 Assigning from Studio (the primary new flow)

On an open routine, a **Shortcut** line sits in the run strip under the name, beside the
window/values line. It is not a tab, because it is one fact and one action.

1. **Set shortcut…** (or `S` then Enter with the routine open — see 3.6) reveals a single key
   capture box: *"Press a key"*. It shows the configured leader as a prefix (`` ` `` + _).
2. The person presses a key. The backend's `Acts.ShortcutCheck(id, key, where)` answers with one
   of: free / already used (by which routine, in which scope) / reserved / not allowed for this
   scope. The card shows that answer **before** anything is written.
3. **Save shortcut** writes through the one assignment function. A collision needs an explicit
   **Replace** press that names the routine losing the key.
4. The line now reads `` ` S — in Notepad `` with **Change** and **Remove**.

The control centre and the CLI keep working, through the same function.

### 3.3 Launching (the overlay)

1. Press the leader. As today, the HUD arms.
2. **New:** the HUD lists the shortcuts that would fire **here**: app bindings for the foreground
   app, then global ones it does not shadow. The list comes from one new read-only call,
   `marco hotkey --list` (it reads `winctx.Active()` in the child, the same reading a press makes),
   fetched off the hook thread. If the person presses a key before the list arrives, the press runs
   anyway; the list is help, never a gate.
3. Press a key. The bound routine runs exactly as now. An unbound key now says
   *"Nothing on S here"* instead of nothing. A stale one says which routine is gone (3.5).
4. Esc or the leader again dismisses. The 2 s disarm stays; **D4** asks whether it should stretch
   while the list is showing.

### 3.4 Arguments (the smallest flow)

The refusal already exists and is right. The proposal adds as little as possible to it:

- **At assignment time**, Studio says it: *"Say hello needs `message`. Pressing the shortcut will
  ask you to Play it from Studio, or you can give it a fixed value here."*
- **Fixed values** (optional, **D3**): a box per argument, stored in the binding's existing
  command arguments exactly as `marco bind s "say hello message:hi"` does today. A name the
  generator treats as a credential is refused here, the same rule ADR-278 applies. Values are
  configuration, so they are written to `bindings.json` and to nothing else — no log, no
  `[intake]` trace.
- **At press time**, with no fixed value, the refusal reaches the HUD unchanged. No HUD text field
  in this slice. A model-proposed value never appears on this path, because a shortcut sends no
  words.

### 3.5 Rename, rescope, deletion

| event | proposed behaviour |
|---|---|
| **rename** | bindings whose identity is the old route are **rewritten to the new identity in the same operation**, under the same route lock. The key keeps working and keeps meaning that routine |
| **rescope** (where it works) | carried the same way; if the shortcut's scope is now wider than the routine allows (§3.1), the binding is **kept and marked**: it refuses with *"`` ` S`` was for Save draft in Notepad, which now works only in Word. Change or remove it in Studio."* |
| **forget one routine** | the binding stays and is **stale**: a press refuses with *"`` ` S`` ran Save draft, which was removed."* Studio lists stale shortcuts (§4, *stale*) with Remove. **D1** decides whether forget removes it instead |
| **forget everything** | **D1.** Today it keeps by-name hotkeys deliberately (G6) |
| **a new routine with the old name** | never inherits the key. It would need the binding to be assigned to it explicitly |
| **legacy binding** (words only, written before this) | **D2.** Proposed: keeps today's by-name press so nothing that works stops working, and Studio shows it as *"by name"* with **Pin to this routine**, which rewrites it to an identity after the person has seen which routine that is |

### 3.6 Keyboard operation in Studio

- Set/Change/Remove are ordinary buttons in tab order after Rename.
- The key capture box takes one key; Esc cancels and restores focus to **Set shortcut…**;
  Tab leaves it without assigning anything.
- Enter on **Save shortcut** or **Replace**. Replace is never the default button.
- The library row's shortcut chip is text, not a control, so tab order stays unchanged
  ([[ADR-294-the-library-is-the-screen]]).

### 3.7 During a recording or a run

| state | leader does | proposed |
|---|---|---|
| a routine is running | **cancels it** (today) | unchanged. A shortcut cannot start a second run while one is going, and that is stated in Help |
| overlay recording (`learn`) | is the recorder's **stop key** (today) | unchanged |
| **Studio recording** | the overlay does not know Studio is recording; `` ` S`` would start a routine into the window being recorded | **[not established]** whether `ADR-268`'s window gate admits that injected input. **D5**: refuse shortcuts while any recording is in progress, which needs a shared "recording now" signal that does not exist today |

---

## 4. Layouts (text mockups, not rendered)

The leader is shown as `` ` `` because that is the default. Every surface reads the configured one.

**Studio — open routine, ordinary**

```
 Save draft                                   [Rename]
 Works in Notepad · plays into: Untitled - Notepad
 Shortcut   ` S  in Notepad                  [Change] [Remove]
 [ Play ]                                      Simplify…
 ─ Steps │ Other names (1) │ Where it works │ Source │ More ─
```

**Studio — no shortcut yet (empty)**

```
 Shortcut   none                              [Set shortcut…]
```

…and while capturing:

```
 Shortcut   `  [ press a key ]   in [Notepad ▾]     [Cancel]
            Esc cancels. M and H belong to the widget.
```

**Studio — collision**

```
 Shortcut   ` S  in Notepad
            ` S already plays "Save as PDF" in Notepad.
            [Replace — Save as PDF loses ` S]   [Pick another key]
```

…and the shadowing case, which is not a collision and is said anyway:

```
 Shortcut   ` S  everywhere
            In Notepad, ` S plays "Save as PDF" instead.
```

**Studio — missing argument**

```
 Shortcut   ` G  in Notepad                  [Change] [Remove]
            Greet needs message. The shortcut will ask you to Play it here,
            or give it a fixed value:  message [ hello        ]  [Keep value]
```

**Studio — stale / refused**

```
 Shortcuts that no longer run anything
   ` S  in Notepad   ran "Save draft", which was removed      [Remove]
   ` P  everywhere   "Print" now works only in Word           [Change] [Remove]
```

**Widget (overlay HUD) — armed, ordinary**

```
 `   in Notepad
   S  Save draft
   G  Greet               (needs message)
   P  Print               everywhere
 Esc closes · M command line · H help
```

**Widget — armed, nothing here (empty)**

```
 `   in Notepad — no shortcuts here
 Set one on a routine in Studio · M command line · H help
```

**Widget — refusal after a press**

```
 ` G  Greet — refused
 This needs message. Play it from Studio, or give the shortcut a value.
```

```
 ` S  — Save draft was removed. Change or remove this shortcut in Studio.
```

---

## 5. Decisions that need a ruling

| # | question | recommendation |
|---|---|---|
| **D1** | Does forgetting a routine (and Forget everything) **remove** its shortcuts, or keep them stale-and-refusing? Today Forget everything keeps by-name bindings so they revive on a new routine of the same name (G6) | Keep them, **stale and refusing**, listed in Studio with Remove. No revival by name. This reverses G6's stated intent, so it is the owner's call |
| **D2** | Legacy word-only bindings: keep by-name pressing, migrate silently, or show as "by name" with an explicit **Pin**? | Keep pressing as today; show "by name" in Studio; never migrate without the person seeing the routine |
| **D3** | Fixed argument values on a shortcut — in this slice or later? | In, because the store already supports it (`Cmd` arguments) and it is the only way a shortcut can run an argument routine without Studio |
| **D4** | Should the 2 s disarm stretch while the shortcut list is visible? | Stretch to ~5 s **only while the list is on screen**, and the leader or Esc still dismisses |
| **D5** | Refuse shortcuts while any recording is in progress? | Yes, if a shared signal is cheap; otherwise state the gap in Help and defer |
| **D6** | Where does Studio read the leader from? It lives in the overlay's `overlay.json` (`$MARCO_OVERLAY_CONFIG` or `UserConfigDir/marco`) | A zero-dependency reader in `internal/` used by both the overlay and Studio, so there is one answer to "which leader" |
| **D7** | Chains (`a then b`): can Studio create them? | No. Studio binds one routine. Chains stay a CLI/control-centre feature and display as *"a chain — edit it in the control centre"* |

---

## 6. Implementation sequence

Each step is shippable alone and leaves every existing surface working.

**S0 — close G13 first.** Answer whether the explicit-identity arm refuses a context route while
another application is in front. If it does not, add the check in the intake (not in the hotkey
path), because Studio's Play depends on the same arm.
*Accept:* a test enters `runInvocation` with `Play = <app A>/context/x`, `App = B`, and gets
`refused`; it fails when the check is deleted.

**S1 — identity in the store, one assignment function.**
`Binding` gains `Route string` (the ID); `Cmd` stays for chains, arguments and legacy entries.
`orchestrator.Bind(reg, route, shortcutApp, key, replace bool) (BindResult, error)` validates:
route exists, key not reserved (`m`, `h`, the leader itself, Esc), shortcut scope ⊆ routine scope,
collision reported unless `replace`. It writes atomically under a bindings lock. `bindKey` (CLI),
`handleBind` (control centre) and the overlay's `bind` word all call it; the CLI resolves the typed
name **once**, to a route, then binds that route. `pressHotkey`: if `Route` is set, perform exactly
that route or refuse with the stale sentence — no `Resolve`.
*Accept:* namesake, stale and reserved-key tests below; `go test ./...` green; the existing binding
tests unchanged or strengthened, not deleted.

**S2 — rename and rescope carry bindings; forget per D1.**
Inside the existing route lock in the orchestrator's rename and rescope.
*Accept:* rename-then-press runs the renamed routine; a new routine saved under the old name is
**not** run by the old key.

**S3 — Studio Shortcut line.**
`Acts.Shortcuts()`, `Acts.ShortcutCheck(id,key,where)`, `Acts.SetShortcut(id,key,where,replace)`,
`Acts.RemoveShortcut(id,key,where)` — thin calls into S1's function with the route **from the
routine's ID**, never from the foreground. The library row shows a chip; stale shortcuts are listed.
*Accept:* page tests in the existing style (`*page_test.go`) for all six layouts in §4; keyboard
test; `build.cmd` only — never a plain `go build` in `plugins/studio`.

**S4 — the overlay explains itself.**
`marco hotkey --list` (read-only); HUD list on arm; "nothing on S here"; stale sentence; the echo
uses the configured leader (G11).
*Accept:* tests below; the hook callback does no I/O (the list is fetched in `processActions`'
goroutine, never the hook — CLAUDE.md's hook invariant).

**S5 (optional, D3) — fixed argument values in Studio.**

Out of scope here: a HUD text field for arguments, chains in Studio, voice, Director, perception,
Acts/Scenes, and any hardening campaign.

---

## 7. Tests

Superseded. The planned tests are listed, without claiming they exist, in
[[Surfaces-alignment-proposal-2026-11-02]] section 12.

---

## 8. Not established

- That the overlay never holds foreground when a leader press spawns `marco hotkey` (design
  implies it; no live check made).
- G13: whether the explicit-identity arm enforces a context route's application.
- G4's namesake redirect was traced through `Resolve`'s order, **not executed**.
- What a Studio recording does with input a shortcut injects into the recorded window (§3.7).
- No headless render or desktop run was made for any layout.
