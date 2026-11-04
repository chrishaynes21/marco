---
type: proposal
status: approved
date: 2026-11-03
source_paths:
  - cmd/marco/intake.go
  - internal/orchestrator/scopeterms.go
  - internal/orchestrator/keep.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/rescope.go
  - internal/oshost/oshost.go
  - internal/codegen/codegen.go
  - plugins/studio/recording.go
---

# S9 review: guarded routines may be "Only in this application"

**Approved by the owner, 2026-11-03, without a generation bump, with four corrections applied below
(marked *Corrected*).** Existing guarded in-place routines become runnable without a rewrite or an
acknowledgement, and still run only when explicitly invoked. Guarded routines with no application
stay unavailable, and the default stays **From anywhere → this application**.

R1 of [[Surfaces-alignment-proposal-2026-11-02]]. This is a review for the owner before any code
is written. The default scope stays **From anywhere → this application**. All this would do is
make the first scope, **Only in**, available to a guarded routine.

## 1. Why it is unavailable today

- A guarded press checks the window it was recorded in, at the press (`oshost.confirmWindow`). It
  checks:
  - that a window is **bound**, alive and **in front**;
  - that its owner is the process bound;
  - that its image is the recorded `Process`;
  - that its size is the recorded `WinW`×`WinH`, exactly;
  - that its scale is the recorded `Dpi`.
- A window is bound in one of two ways:
  - by `Activate` (`bindAt`), which only a **From anywhere → app** route carries;
  - by a window handed to the run (`GiveWindow`, which is Studio's Play).
- An **Only in** route has its `Activate` stripped (`stripLeadingActivate`), so outside Studio
  nothing binds, and `confirmWindow` refuses every press with *no window was brought to the
  front for it*.
- So `ScopeFor` makes **From anywhere → app** the only scope for a guarded draft.
  `WhyNotThisScope` refuses the other two at four places: Keep, the learn paths, rescope review,
  and `keepWhatWasShown`. Studio withdraws the chooser and says why.

## 2. The proposal: bind the window in front, at admission, from one reading

- **Only where it is needed.** *Corrected:* the binding is made only for a routine whose saved
  presses check their window (`ContractOf` guarded or incomplete). A legacy in-place routine keeps
  exactly its existing admission (the foreground application, read as today) and its coordinate
  behaviour. No binding is attempted for it, so it gains no new refusal.
- **Which window.** *Corrected:* at admission, the window in front becomes THIS RUN's target. It
  need not be the particular document or window the routine was recorded in: an eligible sibling
  window of the application that is already in front at admission can be the one selected, and it
  is then checked press by press against the recorded size, scale and program. A sibling that takes
  the foreground AFTER the binding is refused at the next press, because it is not the bound
  handle. Nothing here establishes which document a window holds.
- **Where.** In the intake, at the one place every performance passes (`performOnePlay`, where
  `AdmitInPlace` already runs for an Only-in routine), and nowhere else.
- **How.** The foreground is read **once**: `ForegroundProcess` gives the handle and the owner, and
  `ProcessImage(owner)` gives the image.
  - That one reading is both admission's `Foreground` (today `appOf(d)`, a separate reading) and
    the window bound. A second reading would be a second instant, and the foreground changes
    between two readings exactly when somebody clicks elsewhere.
  - If the image is the routine's application, the host binds that handle and owner through a new
    host method, `BindInFront(hwnd, pid)`. It is `GiveWindow` without the raise: it verifies the
    window is alive, owned by `pid`, has a readable image, and is in front.
  - It does **not** set `activated`, for the reason `GiveWindow` gives: that flag changes where
    legacy window-relative clicks resolve their origin.
- **No raise and no search.** Nothing is brought forward. Nothing looks for another window of the
  program (`winctx.Bind`, `findWindow`). Another window of the same application in front is
  another window, and is refused at the first press.
- **Studio's Play is unchanged** (§6). A handed window is bound by `GiveWindow` and admitted by
  `HandedTo`. The foreground binding is only for a run that was handed nothing: the widget's
  shortcuts, the widget's command line, `marco do`, and Director's offers.

## 3. When it refuses

| situation | where | sentence (existing unless marked new) |
|---|---|---|
| another application in front | admission | *this routine is "Only in notepad", and chrome is in front — nothing was performed …* |
| foreground unreadable (no window, no owner, no image) | admission | *… Marco could not tell which application is in front — nothing was performed* |
| foreground is the app but the window would not verify (closed, owner changed, not in front) between reading and binding | `BindInFront` | new, on `GiveWindow`'s wording: *the notepad window in front closed / belongs to another process / is no longer in front — nothing was performed* |
| a handed window of another program (Studio) | admission | unchanged |
| during the run: window closed, not in front, resized, rescaled, owner changed, other program | each press, `confirmWindow` | unchanged: one press refuses, and the run stops there as it does today |

The bind happens at admission. The checks happen at each press, and each press refuses on its
own. Nothing about the measurement moves.

## 4. Press-time checks

*Corrected:* **a moved window is still pressed.** Each press reads the bound window's origin
fresh and presses at origin + offset, so a window dragged elsewhere between admission and a press,
or between two presses, is pressed where it now is. That is the existing guarded behaviour, and it
is preserved and tested. What refuses is unchanged: a different size, a different scale, a closed
window, a window no longer in front, and a different owner or program.

**Otherwise unchanged.** `confirmWindow` is untouched: geometry, scale, process and owner, at each press, of
a fresh reading. The drag's terms (ADR-259) are untouched. The only difference is that an Only-in
run now has a binding to check against. It is the window that was in front at admission, where a
From-anywhere run has the window `Activate` found.

One consequence is worth stating. **From anywhere** brings the window forward and may restore it;
**Only in** takes the window as the person left it. A resized or rescaled window therefore refuses
under Only in where From anywhere might have restored it. That is the contract (*runs only while
notepad is in front, and brings nothing forward*), and the refusal names the size or scale.

## 5. Generated source and the generation

- **The generated source does not change.** A guarded Only-in routine is the guarded presses
  without the leading `Activate`, which is exactly what `render` writes for context scope today.
  No new statement, no new host action in the source, no Marco word. The binding is an entrance
  rule, like Studio's handoff, which is what the language governance rule asks for.
- **So no new generation.** `codegen.Generation` stays 1. A generation states which generator
  rules wrote a file (ADR-262), and none change. `ContractOf` reads these routines as `guarded`,
  as it already does.
- **The rule that changes** is `ScopeFor` / `WhyNotThisScope`. A guarded draft would allow **Only
  in** and **From anywhere → app**, and still refuse **From anywhere → current foreground**,
  which names no application and so has nothing to validate. `DefaultScope` still opens on
  **From anywhere → app**.
- **Builds that do not have this: decided, no bump.** *Corrected:* an older engine binds nothing
  for a guarded Only-in routine, so its guarded presses refuse. But **the routine may type, press a
  key or do other unguarded things before the first guarded press, and an older engine performs
  those.** So it is NOT established that an older engine does nothing wrong with such a routine.
  A generation bump would not protect against that either: an unknown generation stays playable
  under the rules it always had (ADR-262), on purpose. An older Studio may also offer **Resave**,
  which rewrites it as From anywhere. The same-build stack rule keeps one installation's components
  together. It does **not** stop an older, internally consistent installation opening the shared
  library. Recorded as a runtime compatibility limitation, with no mechanism claimed against it.

## 6. Studio's test Play

Studio is in front when someone presses Play, so admission by foreground could never pass. This
was settled in R3:

1. Play chooses a window of the routine's application from `Pickable` (ADR-272), showing it
   beside Play.
2. It **raises that window**: the one explicit raise, made because the person pressed Play.
3. It hands it over with `--window=<handle>:<owner>`.
4. `GiveWindow` verifies the window is alive, owned by that process, readable, and then
   `exactlyInFront`.
5. `AdmitInPlace` admits by `HandedTo`, and the guarded presses check the handed window.

Nothing in that path needs to change. What changes in Studio:

- the scope chooser offers **Only in** for a guarded recording (today it is withdrawn, with its
  reason). It still opens on **From anywhere**, with *runs only while notepad is in front, and
  brings nothing forward* beside it;
- the Unrunnable card (below).

The raise is the person's own press in Studio. It does not make "no raise" untrue of the routine:
outside Studio nothing is raised.

## 7. Existing routines and the review flows

| what | today | after |
|---|---|---|
| guarded, From anywhere | runs | unchanged |
| legacy (unguarded), Only in | admitted by foreground, runs | **unchanged**, and no binding is attempted (*corrected*) |
| incomplete contract | refuses each press naming the missing term | unchanged |
| **guarded, Only in, saved before the rule** (`Unrunnable`, offered **Resave** → From anywhere) | refuses every press | **runs** when explicitly invoked, under the same admission and press checks, with **no rewrite and no acknowledgement** (*decided*). The file is already exactly what a new save would write. `Unrunnable()` stops listing app-bearing Only-in routines and keeps listing guarded routines with no application |
| guarded, no application (current foreground) | Unrunnable, Resave | unchanged |
| stored steps (step account) | rendered under the routine's own scope | unchanged. Scope only decides the leading `Activate` |
| rebuilds (`Prepare`: simplify typing, generation rebuild; step-edit `Replace`) of a guarded Only-in routine | refused (scope rule) | allowed; they render the same shape under its own scope |
| rescope review, guarded From anywhere → Only in | refused in review | allowed, and the review must say what changes: *no longer brings notepad forward; runs only while it is in front, in the window that is; refuses if that window is resized or rescaled* beside `Was`/`Now`, which differ by the `Activate` line |
| rescope review, guarded → current foreground | refused | still refused |
| rename (R7) of a guarded Only-in routine | n/a | same move, same scope |

## 8. What would hold it

Tests, entered through production, each with a mutation:

- the binding is made from the reading admission uses (no second instant);
- another app in front refuses with nothing bound;
- *Corrected:* through the intake into the REAL host with stated window facts:
  - the correct foreground is bound and pressed at its origin plus the offset;
  - no raise and no search happen;
  - a window moved between presses is pressed where it now is;
  - a resized, rescaled, closed or backgrounded window, or one that has changed owner or
    program, refuses;
  - a sibling that takes the foreground after binding refuses;
  - Studio's validated handoff is entered through `--window`, never by assigning `HandedTo` in
    a test;
- a legacy in-place routine attempts no binding and keeps its admission;
- no raise and no search are reachable on this path;
- a handed window still wins and still must be the app;
- `activated` is not set, so a legacy window-relative press resolves where it did;
- a guarded draft offers Only in, keeps the default, and still refuses current foreground;
- an existing guarded Only-in file runs without rewrite;
- rescope review allows Only in and says what changes;
- `Unrunnable` lists only no-application guarded routines.

A desktop scenario in the harness (`gate.ps1` stage 5) is the one place the real foreground,
geometry and scale would be exercised. It is not part of this slice unless authorised.

## 9. Not in this slice

- Guarded routines under **From anywhere → current foreground**.
- Routines Director performs (demonstrated Acts); they have their own window rules.
- Any change to `confirmWindow`, to the recorder, or to the default scope.
- A desktop run, rebuild or launch.
