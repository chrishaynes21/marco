---
type: reference
status: proposed
date: 2026-10-29
source_paths:
  - internal/orchestrator/draft.go
  - internal/orchestrator/keep.go
  - internal/orchestrator/regenerate.go
  - internal/resolveroute/resolveroute.go
  - internal/oshost/oshost.go
  - internal/winctx/winctx_windows.go
  - plugins/studio/routines.go
  - plugins/studio/play.go
  - plugins/studio/frontend/index.html
---

# Three questions Studio runs together, and what each one is missing

Traced at `fb7e677`. **Nothing is implemented here.** This is what exists, what is missing, and
the smallest behaviour that would close each gap — one proposal, three slices, in the order they
should be built.

The three questions must stay apart, because they are decided at different moments, by different
people, about different things:

| | the question | decided | where it lives | durable? |
| --- | --- | --- | --- | --- |
| **Recording scope** | which window is this a recording **of**? | before the hooks go on | `Deps.Watch`, `Deps.Window` | no — it shapes what is captured |
| **Route availability** | from where may this routine be **asked for**? | at Save | the folder the route is written to | **yes** — it is the route's identity |
| **Playback target** | which window should **this run** act in? | at Play | nothing: there is no such thing yet | **no, and must not become so** |

Collapsing any two is a real defect, not a tidiness problem. Recording scope and availability were
one question once, which is why `marco learn` still asks "where?" twice in two vocabularies
(`askScope` and `chooseScope` in `internal/orchestrator/orchestrator.go`, the second retained for
`LearnVoice`). Availability and playback target would be worse: a window handle written into a
route is an identity that stops being true the next time the application starts — see
[[ADR-071-a-window-is-not-a-place]].

---

## 1. Simplify: the review-and-replace capability nobody can find

### What exists

The whole of it, and it is careful. `internal/orchestrator/regenerate.go` is four steps —
SELECT, PREPARE, REVIEW, REPLACE — of which only the last writes:

- `Deps.Prepare` renders a routine again from the demonstration stored beside it, **in memory**,
  along with a digest of both inputs.
- The review shows both sources, which difference the change table names, and any refusal — a
  rebuild under today's rules may legitimately refuse where the old save did not.
- `Deps.Replace` re-reads both and refuses if either moved while the review was open, naming
  which. This is also the whole of the protection for a routine somebody edited by hand: their
  own text is the left-hand pane, so what replacing it costs them is on screen before they decide.
- Two explicit choices, not one improvement applied quietly: `RebuildCurrent` and
  `RebuildSimplifyTyping`. The second folds each run of typing into one step, which removes the
  pauses inside it — a behaviour change, so it has its own words and its own caution.
- The backend already answers **whether there is anything worth reviewing**:
  `orchestrator.Bears(diffs)` is true only when a difference between generations bears on *this*
  routine's contents, which is the only thing that licenses saying a routine needs an update.
  Studio surfaces it as `Routine.Update`.

See [[ADR-262-a-routine-states-which-generation-wrote-it]].

### What is missing

Nothing in the mechanism. Everything in the finding of it.

- Both buttons live inside `<details>Rebuild and re-record</details>`, closed on arrival.
- **`Routine.Update` is computed, sent to the page, included in the redraw signature — and never
  rendered.** The page reads it only to decide whether to redraw. The one fact that says "there is
  something here worth your attention" is invisible.
- `Simplify typing pauses` is offered identically whether or not it would change a single step.
  The honest reading of a greyed-out-or-not pair of buttons is "these always apply".

### The smallest missing behaviour

**One line on the selected routine, when `update` is true, saying so and opening the detail.**
No backend change, no new call, no write. The capability stops being hidden; what it does when
pressed is unchanged, including every consent and refusal.

Deliberately **not** proposed: computing in advance whether simplifying would change anything.
That means building the rebuild for every listed routine on every poll, and `Prepare` reads a
demonstration off disk and re-simplifies it. The cheap honest alternative is that pressing Review
writes nothing — which is already true, and can be said in the sentence.

---

## 2. Route availability: three scopes that are real, and one that cannot be changed

### What exists

All three, end to end, and they are not cosmetic — the folder *is* the identity:

```
GLOBAL   routes/global/<slug>.marco          app-less; resolves anywhere, switches nothing
CONTEXT  routes/<app>/context/<slug>.marco   only while <app> is in front; switches nothing
FOCUS    routes/<app>/focus/<slug>.marco     from anywhere; brings <app> forward first
```

- `orchestrator.Scope` (`ScopeApp`, `ScopeAppAnywhere`, `ScopeEverywhere`) is the stated form;
  `Scope.mode()` converts it; `routes.Registry` writes the folder.
- **Availability decides what the ask box can answer with.** `resolveroute.Eligible` is three
  rules: global always; anywhere-in-application always, because it raises its own window; in-place
  **only** when that application is the context *and* the context is one that may be relied on.
  A routine saved "only while X is in front" is invisible to a request made from anywhere else —
  by design, and nowhere said in Studio. See [[ADR-267-director-owns-route-resolution]].
- **The guarded contract already constrains the choice, and says why.** `ScopeFor` returns the one
  scope a guarded draft can honestly be saved under, and `WhyNotThisScope` writes the refusal:
  every press checks the window it was made in, and two of the three scopes bring no window
  forward, so every press would refuse. Studio withdraws the chooser rather than defaulting it,
  and says the sentence instead. See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].
- `Deps.Resave` already rescopes one routine — from its own demonstration, to `ScopeAppAnywhere`,
  deleting the old file once the new one is written.

### What is missing

- **The three choices are a bare `<select>` labelled `it works`.** Each option says what the
  routine does (`only while Discord is in front`) and none says what it costs: that asking for it
  by name from anywhere else will not find it.
- **A saved routine's availability cannot be changed.** `Resave` is the only rescope and it is
  gated on `Unrunnable` — a routine that already refuses every press, where there is nothing to
  weigh up. For a routine that works, the only way to move it between the three is to record it
  again.
- The forced-scope sentence is shown at review and never again. A person who returns to a guarded
  routine a week later finds no answer to "why can't I make this work in place?"

### The smallest missing behaviour

**`Rescope(route, want Scope)`: save the routine again, from its own demonstration, under a scope
the person states.** The mechanism exists in `Resave`, which already re-simplifies the stored
demonstration, writes under a stated scope and deletes the old identity. What has to be added
around it is the discipline `Resave` does not need and this does:

1. `WhyNotThisScope` decides which of the three are offerable, and the forbidden ones are shown
   **with their reason**, not hidden — the question "why can't I?" is the one being answered.
2. It goes through `Keep` with a **consent naming the version the person was shown**, not through
   `Save`, which writes over. `Resave` may skip consent because the file it replaces refuses every
   press; this one replaces a routine that works. See
   [[ADR-266-one-set-of-rules-for-keeping-a-recording]].
3. A routine with no stored demonstration cannot be rescoped, and says so — the same refusal
   Review already makes.

Smaller first slice, if one is wanted: the explanation alone, no new behaviour — each option says
where it can be asked for, and the saved routine's panel says it too. That is worth doing even if
`Rescope` is never built, and it is a prerequisite for `Rescope` being understandable.

---

## 3. Playback target: there is no such thing, and the guard cannot tell two windows apart

This is the one with a defect behind it rather than only a gap.

### What exists

Three independent choosers, none of which the person touches:

1. **Studio picks a window to raise.** `theWindowToRaise` in `plugins/studio/play.go` takes the
   application's windows and prefers the one that is up, then the biggest — a launcher, a splash
   and the main window belong to one executable, and the one somebody is looking at is the one
   with the pixels. Minimized windows are candidates on purpose.
2. **The route picks one again.** `do OS's Activate with "discord"` reaches
   `winctx.Bind`, which is `findWindow`: the **first visible top-level window** whose executable
   matches. `EnumWindows` walks in z-order, so this is usually the window Studio just raised.
   Nothing makes it so.
3. **The guard checks the window that was bound.** `Host.confirmWindow` refuses unless it is
   alive, in front, the same process id that was bound, the same program, exactly the same size,
   and exactly the same scale. See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

And one mechanism that is already right and is not wired to any of this: `winctx.ActivateTitle`
brings forward the one window whose title contains a phrase and **refuses an ambiguous match
rather than picking one**. Director uses it for its own workflows; `marco do` does not.

### What is missing, and the defect it leaves

**Two windows of one application are indistinguishable to the guard.** Everything
`confirmWindow` compares — program, process id, size, scale — is identical for two File Explorer
windows, two browser windows, two copies of a game's settings dialog. A routine recorded in one
presses in whichever `findWindow` returns, and the guard confirms it, correctly, as a window of
exactly the shape that was recorded. The guard was never a claim about *which* window; it is a
claim about what the window is *like*. Nothing anywhere states which one was meant, because
nothing can: there is no flag, no field, no question.

Two smaller things fall out of the same hole:

- **`marco do` has no window argument.** `parseDoArgs` in `cmd/marco/assistant.go` takes
  `--play`, `--app`, `--focus`, `--source`. A surface that has already decided which window it
  means cannot say so — which is precisely the complaint `doMoveWindow` records about naming a
  window by app instead of by handle.
- **A routine that works anywhere has no target at all.** `PlayRoutine` raises nothing when
  `App` is empty, so the run begins with Studio in front and the keystrokes go wherever the
  foreground is. *(Read off the code path, not observed — it wants a test before it is stated as
  fact.)* And the page tells that person `Play brings the application forward and presses for
  real`, which is a sentence about an application that routine does not have. That one is mine,
  from the visual pass, and it is a one-line fix whichever way this goes.

### The smallest missing behaviour

**A window stated for one run, carried to the one place that binds it.** Three parts, none of
which touches the language or the saved routine:

1. **Studio asks only when there is a question.** `Deps.Pickable` already lists windows a person
   can choose between, foreground first, Marco's own excluded. Filtered to the routine's
   application: nought or one window, no question is asked and nothing changes; two or more, the
   chooser appears, defaulting to what `theWindowToRaise` would have picked.
2. **`marco do --window=hwnd:N`** carries it, beside `--app`, as a fact about this invocation.
3. **The host binds the stated window** instead of `findWindow`'s first — after confirming it
   belongs to the application the route names. A stated window that has closed, or that belongs
   to another program, **refuses**; it must not fall back to "the first one", which would be the
   ambiguity coming back in silently while the person believed they had answered it.

**The guard does not change at all, and is not weakened.** It still refuses unless the bound
window is alive, in front, the same process, the same program, the same size and the same scale.
Stating a window decides only *which* window gets bound — the thing that was previously decided by
enumeration order. The guard gets stricter in effect, because the window it confirms is now the
one somebody named.

**Nothing is stored.** A handle is not an identity ([[ADR-071-a-window-is-not-a-place]]) and
neither is a title; a title may ADDRESS a window and must never identify one. The choice is made
at Play, used by one run, and forgotten — which is exactly why this is not the same question as
availability.

### What this cannot reach

Only routes that bring a window forward — which is the guarded case, and the one that matters.
A context routine carries no `Activate`, so there is nothing to bind: choosing a window there
means choosing what **Studio** raises before the run, which needs no flag. A global routine has no
window in it at all; the honest answer for one of those is the gap named above, not a target.

---

## Order, and what each slice costs

1. **Simplify discoverability.** One sentence on a routine that has one. No backend change.
   Smallest, and it makes an existing capability real.
2. **Availability, said in words.** Each scope explains where the routine can be asked for, at
   the save and on the saved routine, in the vocabulary `resolveroute.Eligible` actually uses.
   No behaviour change.
3. **`Rescope`.** The first real behaviour: `Resave`'s mechanism, `WhyNotThisScope`'s rules, and
   `Keep`'s consent. Refuses what the contract forbids, and says why rather than hiding it.
4. **Playback target.** The largest, and the one with a defect under it: a chooser in Studio, a
   `--window` on `marco do`, a stated binding in the host, and a refusal where there is now a
   silent choice.

## Not proposed

- **Step editing.** Separate, and not authorised.
- **A window in a route**, by handle or by title. See above.
- **Any new Marco word.** All four slices are surfaces, flags and host behaviour; none of them
  needs the language to say anything it cannot already say, which is the test
  `CLAUDE.md`'s governance rule sets.
- **Touching the recording scope.** `Deps.Watch` and the chosen window are settled
  ([[ADR-268-record-only-the-chosen-window]]) and are not what any of this is about.
