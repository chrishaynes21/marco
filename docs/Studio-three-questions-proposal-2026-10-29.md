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

Traced at `fb7e677`. This is what exists, what is missing, and the smallest behaviour that would
close each gap — one proposal, four slices, in the order they should be built.

**Status.** The two that change no behaviour are **implemented**: Simplify's discoverability and
the availability explanations, with the app-less Play sentence corrected. **`Rescope` is built**, contract and
surface: the mechanism and its rules in `internal/orchestrator/rescope.go`, held by tests that
stand in both of its interleavings, and the panel for it in `plugins/studio/rescope.go` with the
page rendered against it. **Playback targeting is a contract, ruled on and not built**, and is
the remaining slice. See [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]].

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

- Both buttons lived inside `<details>Rebuild and re-record</details>`, closed on arrival.
- **`Routine.Update` was computed, sent to the page, included in the redraw signature — and never
  rendered.** The page read it only to decide whether to redraw.
- `Simplify typing pauses` is offered identically whether or not it would change a single step.
  That one stands: see below for why finding out in advance costs a rebuild per routine per poll,
  and what is said instead.

### The smallest missing behaviour — implemented

**A visible `Review simplification` beside Play, conditioned on the stored demonstration.** No
backend change, no new call, no write. The capability stops being hidden; what it does when
pressed is unchanged, including every consent and refusal.

**And NOT conditioned on `update`, which was this proposal's own mistake.** `Update` is about
GENERATIONS — whether a difference between the rules that wrote this file and today's bears on
it — and a routine today's rules wrote rebuilds byte for byte under them while still being full
of typing pauses worth folding. Making `update` the entrance would have hidden simplification
from most of a library. The entrance is `offer`: the routine still has its own recording beside
it, and this build can explain its generation. Today's-rules review joins it on the same row only
when `update` is true, because that IS the question `update` answers.

Deliberately **not** proposed: computing in advance whether simplifying would change anything.
That means building the rebuild for every listed routine on every poll, and `Prepare` reads a
demonstration off disk and re-simplifies it. The honest alternative is already there — pressing
Review writes nothing, which is said beside the button, and the review card then says in words
when the rebuilt version is identical to the routine you have.

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

- **The three choices were a bare `<select>` labelled `it works`.** Each option said what the
  routine does (`only while Discord is in front`) and none said what it costs: that asking for it
  by name from anywhere else will not find it. *(Fixed: each option now says both, and the
  selected routine's panel says where it can be asked for.)*
- **A saved routine's availability cannot be changed.** `Resave` is the only rescope and it is
  gated on `Unrunnable` — a routine that already refuses every press, where there is nothing to
  weigh up. For a routine that works, the only way to move it between the three is to record it
  again.
- The forced-scope sentence is shown at review and never again. A person who returns to a guarded
  routine a week later finds no answer to "why can't I make this work in place?" *(Partly fixed:
  the sentence that stands where the withdrawn chooser was now says why, at the save. On a SAVED
  routine it is still not said, because the honest answer there is "and here is how to move it",
  which is `Rescope`.)*

### The smallest missing behaviour

**Done first, and now implemented (`fb7e677`…):** the explanation alone, no new behaviour — each
scope says where the routine can be ASKED FOR as well as where it works, at the save and on the
saved routine, and the withdrawn chooser says why. That is worth having even if `Rescope` is never
built, and nothing about `Rescope` is understandable without it.

#### Then: Rescope is a reviewed MOVE BETWEEN IDENTITIES — now built

**Implemented as the contract below plus the five completion requirements added in review**, all
of which are in `internal/orchestrator/rescope.go` and held by tests:

1. The review's token binds the original's source **and demonstration**, the destination identity
   **and what was at it**, the library, the scope asked for, and the exact proposed output.
2. Both identities are locked in a deterministic order — `lockOrder`, taken from the identities
   themselves — and both are revalidated under those locks before anything is written.
3. A destination collision needs separate, version-bound consent; without it the creation is
   `O_CREATE|O_EXCL`, so a routine appearing inside the lock is the failure rather than something
   written over.
4. The destination's source **and demonstration** are read back before the original is removed,
   and the original is revalidated immediately before removal — a newer version is never deleted.
5. A failure leaves the original intact wherever possible, and a destination that succeeded with
   a removal that failed reports **"Copied; original retained"**, never "Moved".

Selecting the scope a routine already has is refused as nothing to do. A hand edit is preserved
unless the person approves the displayed rebuilt source — which is what the review shows and what
the token binds.

**What is not built: the surface.** Nothing in Studio calls either function yet, and that is the
next bounded step.

The reasoning each rule came from:

**Not a save under a different word.** A route's identity is its folder, so changing the scope
creates a route at a new path and ends one at an old path — two files, two libraries' worth of
things that can already be there, and a window between them. `Resave` is useful machinery and is
**not a complete move contract**: it is allowed to be immediate precisely because the file it
replaces refuses every press, so there is nothing to lose and nothing to collide with that is
worth protecting.

The contract, which must be built before the behaviour:

1. **The destination is checked, and a collision needs its own consent.** A routine of this name
   may already exist at the destination scope — `settings` in place and `settings` from anywhere
   are two routes and always were. Consent to move is not consent to overwrite. `Keep`'s
   `Consent` names one routine AND the version of it somebody was shown, re-read at the write;
   a move with no destination consent is `routes.Registry.Create` with `O_CREATE|O_EXCL`, which
   refuses rather than writes.
2. **The source is shown before approval, because rebuilding can replace a hand edit.** The move
   re-renders the routine from its stored demonstration — which is the only honest way to change
   the scope, since the `Activate` a focus route carries is generated and a context route's is
   stripped — and a person who edited their generated file by hand would lose that edit. So the
   resulting source is on the screen, beside what is saved now, before anything is written. This
   is the same discipline `regenerate.go` already applies, for the same reason, and it is not
   optional here just because the person asked for a move rather than a rebuild.
3. **The original is removed only after the destination exists and has been re-validated.** Write
   the new route, read it back, check it is what was approved — then delete the old one. In that
   order, with no window where neither exists, and a failed delete leaves two routines and SAYS
   so rather than reporting a move that half happened. (`Resave` deletes after a successful save
   already; what it does not do is re-read.)
4. **`WhyNotThisScope` decides which destinations are offerable**, and the forbidden ones are
   shown **with their reason** rather than hidden — "why can't I?" is the question being asked.
5. **A routine with no stored demonstration cannot be moved**, and says so: there is nothing to
   render from, and rewriting generated Marco by hand is how two things that have to agree stop
   agreeing.

See [[ADR-266-one-set-of-rules-for-keeping-a-recording]] for the lock, the consent-to-a-version
rule and the exclusive create, all of which this uses unchanged.

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

**And the precise limit of the fix, stated before it is built.** Today's guard validates **the
window bound for execution** — that the window this run brought forward is still alive, in front,
the same program, the same process, the same size and the same scale. It does not validate that
this is the window, or the document, the demonstration was made in. Explicit playback selection
fixes the CHOICE: it replaces enumeration order with a person's answer, and carries that answer
into the binding. **It does not establish semantic equivalence with the recorded document**, and
nothing proposed here does. Two Explorer windows showing different folders remain two windows
Marco cannot tell apart on its own; what changes is that it stops guessing and the person says
which. Anything stronger would need something durable about the document, which is a different
question and is not this one — see [[ADR-071-a-window-is-not-a-place]].

Two smaller things fall out of the same hole:

- **`marco do` has no window argument.** `parseDoArgs` in `cmd/marco/assistant.go` takes
  `--play`, `--app`, `--focus`, `--source`. A surface that has already decided which window it
  means cannot say so — which is precisely the complaint `doMoveWindow` records about naming a
  window by app instead of by handle.
- **A routine that works anywhere has no target at all.** `PlayRoutine` raises nothing when
  `App` is empty, so the run begins with Studio in front and the keystrokes go wherever the
  foreground is. *(Read off the code path, not observed — it wants a test before it is stated as
  fact.)* The page used to tell that person `Play brings the application forward and presses for
  real`, about an application that routine does not have; that sentence was mine, from the visual
  pass, and is **corrected** — it now says that nothing is brought forward and the presses go
  into whatever is in front.

### The smallest missing behaviour

**STUDIO MUST NOT PLAY WITHOUT AN IDENTIFIED EXTERNAL WINDOW.** Ruled, and it resolves the open
question this section used to leave to the owner:

- **One candidate** may be selected automatically — and is shown beside Play, not assumed.
- **Several candidates** require a visible choice. Nothing is picked on the person's behalf.
- **No candidate** means Play is **unavailable, with the reason**. It does not run into whatever
  happens to be in front, which is this window.
- **Global availability does not mean "send input into Studio."** A routine that can be asked for
  from anywhere still has to act on something, and that something is named.
- **The selected window receives the INITIAL HANDOFF**, and that is all it may be described as.
  It is not a guarantee that every subsequent keystroke stays there: a routine can switch windows
  itself, a dialog can take focus, an application can move the foreground. Saying "confined to
  this window" would be a promise the foreground does not make — which is the same class of claim
  `Window-checked` was corrected to avoid.
- **Existing CLI behaviour is preserved** unless the invocation explicitly supplies the new window
  selection. `marco do --play=… --app=…` keeps doing exactly what it does today; the new flags are
  additive, and a run that names no window behaves as it always has.

**A window stated for one run — HANDLE AND OWNING PROCESS — carried to everything that acts on
it.** None of it touches the language or the saved routine:

1. **Studio asks only when there is a question.** `Deps.Pickable` already lists windows a person
   can choose between, foreground first, Marco's own excluded. Nought or one candidate, no
   question is asked and nothing changes; two or more, the chooser appears, defaulting to what
   `theWindowToRaise` would have picked.
2. **The selected window is visible beside Play**, not only inside the chooser. What a run is
   about to act on is part of what Play means, and a choice made and then hidden is a choice the
   person cannot check before pressing.
3. **`marco do --window=hwnd:N --window-pid:P`** carries it beside `--app`, as a fact about this
   invocation. **The PID travels with the handle**, because a handle alone is not an identity:
   Windows reuses them, and the guard's own process check is only meaningful against a process
   that was named at selection time.
4. **One decision, used twice.** Studio raises THAT handle instead of consulting
   `theWindowToRaise` again, and the host binds THAT handle instead of `findWindow`'s first.
   Today those are two independent choosers that happen to agree; after this there is one answer
   and two users of it.
5. **Disappearance or a change of owner REFUSES.** A stated window that has closed, or whose
   handle now belongs to another process, stops the run and says which — it must never fall back
   to "the first one", which is the ambiguity returning silently while the person believes they
   answered it.

**It covers all three kinds of route, because availability does not answer this question:**

- **Application-bound (focus).** As above: the handle and PID reach `Activate`'s binding, and the
  guard then checks that window as it always did.
- **In place (context).** The route carries no `Activate`, so there is nothing for the host to
  bind — but Studio still raises something before the run, and *that* is the choice. Same
  chooser, same handle, same refusals; it ends at the raise instead of at a binding.
- **Anywhere (global).** The hard one, and the one with no answer today: a global routine names
  no application, so **where its keystrokes should go is not implied by anything it carries**.
  Availability says it can be asked for from anywhere; it does not say what it acts on. Play
  therefore needs an **explicit handoff**: the person names the window this run is for, Studio
  raises it and hands the foreground on, exactly as it does for an application. **Naming nothing
  does not run** — that was the one open question here and it is answered above: no candidate
  means Play is unavailable with the reason, because the alternative is pressing into this window.

**The guard does not change at all, and is not weakened.** It still refuses unless the bound
window is alive, in front, the same process, the same program, the same size and the same scale.
Stating a window decides only *which* window gets bound — previously decided by enumeration
order — and the PID carried beside the handle gives the existing process check something that was
named by a person rather than found by a search.

**Nothing is stored.** A handle is not an identity ([[ADR-071-a-window-is-not-a-place]]) and
neither is a title; a title may ADDRESS a window and must never identify one. The choice is made
at Play, used by one run, and forgotten — which is exactly why this is not the same question as
availability.

---

## Order, and what each slice costs

1. **Simplify discoverability — DONE.** `Review simplification` is beside Play, conditioned on
   the routine still having its own demonstration (`offer`) and **not** on the generator's rules
   having moved (`update`), which answers a different question: a routine today's rules wrote
   rebuilds byte for byte under them and can still be full of typing pauses. Today's-rules review
   joins it on the same row only when a generation difference bears on that file. Both say that
   nothing is written, and the review card already says when a rebuild would change nothing.
2. **Availability, said in words — DONE.** Each scope says where the routine can be ASKED FOR as
   well as where it works, in the vocabulary `resolveroute.Eligible` uses, at the save and on the
   saved routine; the withdrawn chooser says why a guarded recording has one choice. And the
   sentence under Play no longer promises an application an app-less routine does not name.
3. **`Rescope` — DONE, contract and surface.** The first real behaviour, and a
   **reviewed move between identities** rather than a save under another word: both ends locked
   in a fixed order and revalidated, the destination checked with its own version-bound consent,
   the resulting source shown before approval because the rebuild can replace a hand edit, the
   destination read back, and the original removed only after that — reported as a COPY when the
   removal does not happen. Twelve mutations for the contract and thirty for the surface, all
   killed, two of them standing in instants nothing else can reach. The surface offers all three
   availability choices WITH the reason beside any that cannot be taken, both identities and both
   sources before approval, a separate agreement for an occupied destination that the page cannot
   author — the token and one boolean are all it sends — and Moved, `Copied; original retained`
   and leftover litter as three distinct renderings, after which the affected identities are read
   again.
4. **Playback target.** The largest, and the one with a defect under it: a chooser in Studio, the
   selected window visible beside Play, a handle AND its owning process carried through the raise
   and the binding, one decision where there are now two, a refusal on disappearance or a change
   of owner — an explicit handoff for global routines, and **no Play at all without an identified
   external window**.

## Not proposed

- **Step editing.** Separate, and not authorised.
- **A window in a route**, by handle or by title. See above.
- **Any new Marco word.** All four slices are surfaces, flags and host behaviour; none of them
  needs the language to say anything it cannot already say, which is the test
  `CLAUDE.md`'s governance rule sets.
- **Touching the recording scope.** `Deps.Watch` and the chosen window are settled
  ([[ADR-268-record-only-the-chosen-window]]) and are not what any of this is about.

## The rendered states, after the two implemented slices

At `docs/images/studio-ui/`, thirteen headless renders of the production page. The three that
show this work:

| | |
| --- | --- |
| `4-selected-routine.png` | an application routine: where it is asked for, `Play brings Discord forward`, `Review simplification` |
| `4e-selected-in-place.png` | in place: **asked for only while Notepad is in front**, and today's-rules review beside the simplification because a generation difference bears on it |
| `4d-selected-anywhere.png` | no application: **`Play presses for real, into whatever is in front when it runs`** — the sentence that used to promise an application this routine does not have |

The fixtures behind them were corrected too: `app-front` was not one of the three scopes, and
there was no app-less routine in the set at all — which is why the sentence that was wrong about
one could not be seen in any picture.
