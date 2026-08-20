---
type: milestone
status: historical
---

# Semantic goal decomposition

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> The user describes the desired outcome.
> The Director expands goals into semantic programs.
> Marco still executes only deterministic primitive operations.
> Decomposition is deterministic, typed, explainable and replayable.

## The gap this closes

The Director executed programs well and still expected the user to describe *how*.
"Rename this file to Budget" was not a request it could take. It had to be said as:

```
focus Report.txt, then press Rename, then type Budget, then press enter
```

A user who knows to say that does not need a Director.

So a **Goal** names an outcome, a **Procedure** is the hand-written expansion of that
outcome into semantic steps, and what comes out is an ordinary `program.Program`.

## The layer

```
Natural language → Goal → Procedure → Semantic Program → Execution
```

Fifteen goals: `create_folder`, `rename`, `duplicate`, `delete`, `save`, `save_as`,
`print`, `open_settings`, `open_file`, `close_without_saving`, `create_tab`, `download`,
`move`, `copy`, `paste`.

A goal carries **parameters** (the new name, the destination) and a **context** (the
application, and the thing acted on as a *phrase*). The target stays a phrase on purpose:
resolution happens when the produced program's step runs, so a goal that carried a
resolved element would be carrying a handle from before anything had a chance to move.

## Nothing downstream changed

> Expansion must produce ordinary Director Programs. No special execution engine.

This is the load-bearing property, and it is why the goal package is small. Expansion
ends in `program.Validate` — the same validator every other request goes through — so
variables, collections, control flow, clarification, replay and verification keep working
because what they receive is what they have always received.

The one wiring change is a branch at the top of `HandleProgram`, tried **before** clause
splitting. That ordering matters: "rename this file to Budget" contains `" to "`, and the
splitter would cut it into two clauses that each mean nothing. A goal is one request
however many steps it becomes.

## Procedures are hand-written

> These are typed procedures. Do not generate them with an LLM.

A procedure is a claim about what an application does, and a generated claim cannot be
reviewed before it acts. A wrong ladder rung picks a weaker mechanism; a wrong procedure
presses the wrong thing.

Rename, generically:

```
1. focus "Report.txt"                        focus
2. invoke Rename                             semantic
3. set the name to "Budget"                  edit
   waits until: an editable field has focus
4. confirm the new name                      semantic
```

Every directive is a **semantic action**, a focus, or an edit. None names a coordinate, a
keystroke or a pattern — those are chosen further down by the capability ladder, against
the control that is actually there.

### The wait is not a step

"Wait until editable" is a **precondition** on the step that needs it, which the program
layer evaluates as a semantic wait. So the Director waits for an editable field to
*exist* rather than for a duration somebody guessed.

### The conditional is not a branch

`close without saving` is the interesting one:

```
1. close the window                          semantic
2. choose Don't Save at the save prompt      semantic, best effort
```

The prompt appears only if the document is dirty, and the program layer has no branches —
deliberately. So step 2 is **best-effort**: either the prompt was there and Don't Save is
chosen, or it was not and the step verifies inconclusively and the program continues.
That is the honest shape, because the Director cannot know in advance whether the
document is dirty.

The step's *phrase* says what, never "if". Wording it as a condition is rejected by the
program validator's control-flow guard — correctly, since it cannot tell a description of
a conditional from a request for one. (It caught exactly that during development.)

## Application overrides

The registry selects most-specific-first: a procedure naming this application beats the
generic one.

| application | procedure | why it differs |
|---|---|---|
| any | `generic rename` | invokes a Rename control and types into the field that appears |
| Explorer | `explorer rename` | Rename lives on the item's **context menu**, so the menu is opened first |
| VS Code | `vscode rename symbol` | a **different operation** that shares the word |

The last is why the mechanism exists. Renaming a symbol rewrites every reference in the
project, including in files that are not open — so it carries its own risk level and
demands confirmation. Collapsing it into "rename" would let "rename this to Budget" in an
editor refactor a codebase.

## Preconditions are questions, asked first

> Clarification happens before expansion whenever possible.

A procedure declares what it needs (`target`, `name`, `destination`). A goal that does
not satisfy them is refused **before expansion**, as a typed `Refusal` carrying the
missing requirements — so a front-end asks "what should it be called?" rather than
watching the Director focus a file and then discover it has no name to type.

All missing requirements are reported at once. A move with neither a target nor a
destination asks about both rather than playing twenty questions.

A refusal with missing requirements is reported as `needs_clarification`, not `failed`:
it is answerable.

## Safety, declared before expansion

> The policy engine evaluates procedures before expansion.

Each procedure declares its mutations, and whether it is destructive, external,
irreversible, and confirmation-requiring. **Declared rather than derived from the steps**,
because the steps say what will be pressed and only the author knows what pressing it
means: "invoke the Delete menu item" is three low-risk semantic actions and one
irreversible outcome.

| procedure | risk | why |
|---|---|---|
| `generic delete` | high, confirms | whether it is recoverable depends on the application, not on anything the Director can see |
| `generic close without saving` | high, confirms | discarding unsaved work is the thing no ordinary action undoes |
| `generic print` | high, confirms | paper cannot be un-printed |
| `vscode rename symbol` | high, confirms | rewrites files that are not open |
| `generic open settings` | low | shows something; changes nothing |

## Diagnostics

```
director goals                          the vocabulary, with risk and procedures
director procedures                     every procedure and what it applies to
director procedure <name>               one procedure's safety, reasoning and steps
director explain goal "<request>"       expand a request WITHOUT running it
director explain goal "…" --app explorer   expand as a named application would
```

`explain goal` is the important one: the way to find out what "close without saving" will
do is to ask, not to try it. It reports the goal, the chosen procedure and whether it was
an override, the produced steps with their waits and best-effort markers, the procedure's
reasoning, and the safety — then says plainly that nothing was run.

None of these need a service or touch the desktop: a procedure is a property of the goal,
and an expansion is a pure function of the goal and the registry.

## What was measured

Expansion was exercised end to end through the built binary:

```
$ director explain goal "rename this file to Budget" --app explorer
Procedure    explorer rename (application override)
  1. select the focused control
  2. open the context menu for the focused control
  3. choose Rename
  4. set the name to "Budget"        waits until: the item's label is editable
  5. confirm the new name
```

**Two real bugs surfaced this way.** "Rename **this** file" satisfies the target
requirement by *pointing*, so the goal carries no label — and procedures passed that empty
label straight through, producing a step with nothing to act on. Directives now carry a
deictic flag and resolve against focus, which is the same reading the semantic layer gives
"expand this". Separately, `--app` was missing from the CLI's `valued` flag list, so its
value was reordered into the positionals and became the request — the silent failure that
list's own comment warns about.

## Hardening

A later milestone hardened the above. What changed:

**Confirmation is enforced, not merely declared.** `RequiresConfirmation` now gates
execution between expansion and step 1 — the last point at which nothing observable has
happened. Expansion is allowed first because it reads a registry and builds structs, and
because a user cannot meaningfully agree to "delete this" without being shown the steps.
Four outcomes are distinguished: `not_required`, `accepted`, `rejected` (→ *cancelled*,
not failed), `unavailable` (→ *blocked*; no confirmer, or one that errored — **never**
read as yes). An accepted confirmation covers per-step confirmations up to the risk
agreed to, and never a stricter one.

**Goal provenance is persisted** as `ActionNode.GoalProvenance` — goal, procedure,
request, step id/index/count, verification requirement, whether the step was guarded by a
precondition, and whether the procedure was generic. Diagnostic only: replay walks the
stored semantic action and never the registry, so a node replays identically after its
procedure is renamed or deleted. Absent for non-goal requests and for every node written
before this existed, so old graphs need no migration.

**Control labels are semantic roles.** A procedure names `RoleDiscardChanges`, not
"Don't Save". Each role carries an ordered alias table (16 languages for the discard
role), and a `LocalizedControls` platform adapter outranks it when one is wired. A
destructive role requires an **exact** match and refuses otherwise — "Save" is a
substring of "Don't Save", so a near match is a confident wrong answer that loses work.

**Goal selection detects ambiguity.** Specificity is a documented rule, not a priority
number: application-scoped beats generic because it constrains strictly more. Two equally
specific matches produce a deterministic ambiguity error naming both, rather than letting
registration order decide. `Registry.Validate()` catches duplicate names and permanently
shadowed registrations.

**Best-effort is formalized.** Only `FailureTargetAbsent` permits a skip. Ambiguous
targets, action failures, verification failures, policy refusals and execution errors all
report `applicable_failed`. An unverified step is `unknown` — **not** tolerable, because a
step that might have been needed must not be skipped silently.

**`director goal --dry-run "<request>"`** prints the normalized request, matched goal,
competing candidates, selected procedure, declared safety, unresolved preconditions,
expanded program, validation result, expected confirmations, deictic bindings and the
provenance that would be attached — reaching no provider at all.

## Status of each claim

Stated in one place, in one vocabulary. Only these words are used, and they are not
combined: **implemented and unit-tested**, **structurally tested**, **live-tested**,
**scaffolded only**, **not implemented**.

| capability | status |
|---|---|
| Goal parsing, procedure registry, expansion into `program.Program` | implemented and unit-tested |
| Goal-level confirmation before step 1 | implemented and unit-tested |
| Goal provenance on action-graph nodes | implemented and unit-tested |
| Localized control roles + destructive exact-match refusal | implemented and unit-tested |
| Deterministic ambiguity detection | implemented and unit-tested |
| Best-effort classification | implemented and unit-tested |
| Dry-run diagnostics | implemented and unit-tested |
| Registry validation at startup | implemented and unit-tested |
| Deictic binding model, kind vocabulary, kind enforcement | implemented and unit-tested |
| Binding runtime integration; execution refuses without one | implemented and unit-tested |
| Revalidation immediately before the first dependent action | implemented and unit-tested |
| Action-level confirmation for non-goal actions | implemented and unit-tested |
| Goal-confirmation coverage rules | implemented and unit-tested |
| Replay confirmation policy | implemented and unit-tested |
| Binding metadata in the action graph, with old-graph compatibility | implemented and unit-tested |
| Daemon installs a working `Confirmer` | implemented and unit-tested |
| `CONFIRM` protocol round trip + `director confirm` | implemented and unit-tested |
| Production request path reaches the goal layer | implemented and unit-tested |
| Per-action binding correlation, automatic after execution | implemented and unit-tested |
| Filesystem inspector (`osResources`) | implemented and unit-tested |
| **Explorer shell-item identity in the bridge** (`plugins/uia/Shell.cs`) | **implemented and unit-tested** |
| **`ResourceIdentity` through observation → fusion → binding → graph** | **implemented and unit-tested** |
| **File-versus-folder from the shell, not from the caption** | **implemented and unit-tested** |
| **Caption-only and virtual-item refusal** | **implemented and unit-tested** |
| **Revalidation by shell path** | **implemented and unit-tested** |
| Whole-goal rename correlation (`verify.CorrelateRename`) | structurally tested |
| Live harness: launch, workspace, identify, resource check, submit, graph, cleanup | implemented and unit-tested |
| **Inline-editor correlation** (`internal/director/inline`) | **implemented and unit-tested** |
| **Editor derivation before the step that acts on it** | **implemented and unit-tested** |
| **Editor value and closure as per-step verification evidence** | **implemented and unit-tested** |
| **Editor snapshot on the action-graph node, carrying no handles** | **implemented and unit-tested** |
| **Replay re-derives the editor instead of re-resolving a stored one** | **implemented and unit-tested** |
| **Explorer rename, end to end** | **not implemented** — see below |
| Goal system live validation | **not performed** |

### The live scenario: attempted, and not yet passed

The Explorer rename scenario has been RUN against a real desktop several times. It is
**not live-tested**: no rename has occurred and no filesystem verification has passed.

What each attempt established, with real input, through the production daemon:

- the daemon starts, installs its confirmer and serves;
- the Explorer window for a uniquely-named temporary folder is **positively identified**
  by title, and the harness refuses an ambiguous or absent match;
- `click Alpha.txt` resolves, plans, passes policy, **lowers to legal Marco** (`OS's
  Click`, compile ok, runtime ok), executes as real input, settles by condition, and
  **verifies** ("Alpha.txt became selected");
- the action is recorded to an action graph the production runtime wrote;
- `rename this file to Budget` reaches the **goal layer** and expands.

The blocker that stopped the previous attempt — no backing path for an Explorer item — is
**closed**. The binding now resolves live:

```
binding "this" → a file C:\...\marco-live-...\Alpha.txt — still the same object
```

with the shell's own evidence attached. How far the rename itself has got, live:

| step | live result |
|---|---|
| 1. select the file | **verified** — `Accessibility's Select`, compile ok, runtime ok |
| 2. invoke the Rename command | **verified** once (focus moved into the label); **unverified** on a later run, where the focus change was not observed inside the settle window |
| 3. set the name to "Budget" | **verified** — `set_text via value_api`, "the control now contains Budget" |
| 4. confirm the new name | not reached |

So three of the four steps have each executed and verified against real Explorer, and the
rename has never completed. **No filesystem verification has passed**, which is why this is
not live-tested.

Step 3's "verified" in that table is the one to be careful about, and it is the reason for
the milestone below. It says the CONTROL THAT WAS TARGETED now contains "Budget". It does
not say that control was the rename editor — and in a details view it was not. See *The
inline editor*.

Two procedure defects were found and fixed along the way, both with the live trace as the
evidence:

- **`explorer rename` opened the context menu**, which was a Windows 10 assumption. Windows
  11 puts Rename in the command bar as an `AppBarButton` with an Invoke pattern — and a
  context menu is its own top-level window, so its contents were never in the tree the
  Director walks. The step could not have succeeded and correctly asked for clarification.
- **A step naming no control reached the resolver with an empty query.** An edit that names
  nothing means the field the previous step opened; a verb that names nothing — confirm,
  undo, refresh — is addressed to the focused context. Both now say so explicitly, as
  ANAPHORIC references needing no binding: they point at what the procedure just produced,
  not at anything the user indicated.

The remaining flakiness is step 2's verification, not its execution: the Invoke lands and
the rename box opens, but whether the focus change is observed inside the settle window
varies. That is a verification-timing question, and it is the next thing to look at.

Three production defects have been found by running it, all fixed, all with regressions
that fail against the unfixed code:

1. **The daemon never reached the goal layer** — `HandleRequest` routed to goals only for
   multi-clause requests, so "rename this file to Budget" was parsed as renaming a
   *variable*.
2. **A selected file was unreachable** when its container held keyboard focus.
3. **An Explorer item had no identity but its caption**, which the binding layer refused.

## Explorer shell-item identity

### The mechanism

`IShellFolderViewDual`, reached through the shell's own window list
(`Shell.Application.Windows()`), matched to a window by its handle. It gives the view's
**current folder** and its **selected items**, and each `FolderItem` carries `Path`,
`IsFolder`, `IsLink` and `IsFileSystem`.

Chosen over the alternatives because it is the narrowest thing that answers the question.
UIA carries no path for Explorer items — that is what the live run demonstrated.
`IShellItem`/`IShellFolder` and PIDLs would work and would mean COM interface
declarations, manual marshalling and lifetime rules to get wrong, to obtain the same
string. Everything here is late-bound through reflection, so the bridge still builds with
one `csc.exe` invocation and no SDK.

### Correlation, and every place it refuses

The shell says "exactly one item is selected in this folder, and its path is P". The tree
says "exactly one node is selected, and it is captioned C". The resource is attached only
when both are true **and** C matches the shell's own name for that item — equal, or equal
once the extension is dropped, which is what Explorer shows with extensions hidden.

Nothing is attached when: the window is not Explorer · the shell has no view for it · the
view shows a virtual location · the current folder cannot be established · nothing is
selected · **several things are selected** · the item is not filesystem-backed · its path
does not canonicalise · its path is **not directly inside the view's folder** · the item
has disappeared · the shell and the filesystem disagree about whether it is a folder ·
the tree reports a different number of selected items than the shell · the captions
disagree.

Each of those leaves the item as a control with no file behind it, which the binding layer
already refuses. A path is never assembled from a folder and a caption.

**Shortcuts** bind to the shortcut file itself, never to what it points at: following it
would rename a file in another folder that the user is not looking at. The `Link` flag is
reported so a caller can see which it has.

### The data path

```
Explorer view (IShellFolderViewDual)
 → plugins/uia          ShellResource on the selected Node
 → uiaclient            directorapi.ResourceIdentity on the Observation
 → fusion               Element.Resource, from the first source that KNEW
 → binding.classify     kind and path from the shell, not from the caption
 → binding.Store        the request's one live binding
 → actiongraph          an immutable snapshot, path and evidence, no handles
```

The Director never imports the shell. `internal/director/boundary_test.go` has a dedicated
test for it: a procedure still asks for "a file", and the platform supplies the evidence.

## The inline editor

An application that renames IN PLACE opens a control on the object being renamed. The
Director had no name for that control. Its only handle on it was "whatever holds focus",
and in File Explorer that is dangerous in a specific way.

### The decoy

A details view contains an Edit control per column per row. The selected row's Name cell is
an Edit whose value is `Alpha.txt` and which has a ValuePattern. It is not the rename
editor. Typing into it changes a caption and renames nothing — and reports success, because
the control does now contain the new text. That is exactly what the previous live run did:
step 3 executed, verified, and renamed nothing.

Any rule that matches on CONTENTS picks the decoy.

### What Windows 11 actually does

Captured from a real Explorer window before and after invoking the command-bar Rename
button. The element count is identical, 166 → 166: one control replaces another.

```
ClassName    UIRenameTextElement     ← exists ONLY in rename mode; the marker
ControlType  ControlType.Edit
AutomationId ""                      ← none, so the class is the only handle
Value        "Alpha.txt"             ← the item's current display name
Focused      true
parent       UIItemsView             ← a SIBLING of the row, not a descendant
```

At the same moment the selected row's Name cell (`System.ItemNameDisplay`) goes **empty**:
the editor replaces its presentation. The editor belongs to a different UIA provider than
the list item, so "is it a child of the bound item?" is not a fact that exists here, and
`internal/director/inline` does not pretend it is.

Rename mode also **exits when the window loses foreground**. Observing does not disturb it;
anything that steals focus does.

### The correlation, and every clause is a way to refuse

1. The control's **class** is one this build knows to be an inline editor. This is what
   keeps the Name cell, the address bar and the search box out. A build that knows no
   editor class for the application in front of it finds NO editor rather than guessing.
2. It is in the **same window** as the bound object.
3. **Exactly one** such control exists. Two is an ambiguity and neither is chosen.
4. The shell still reports the bound object as the **selected** one.
5. Its value is the bound object's display name, allowing for a hidden extension —
   corroboration, not identification: it tells an editor opened on `Alpha.txt` from one
   opened on the item below.

Clause 5 is dropped for a COMMIT, and only for a commit (`inline.FindOpen`). By the time
an edit is committed the box deliberately no longer holds the original name, and applying
the discovery rule there would refuse the very editor whose contents are about to be
committed. That is what stopped the fourth step of a rename whose third had succeeded.

### It is a derived target, not a binding

A binding answers *which object did the user mean?* and lives for the request. A derived
target answers *which control is the application showing me RIGHT NOW for that object?* and
lives for one step. Conflating them would be a category error with teeth: a remembered
editor is a control that has since closed, and a query built from one resolves to nothing
or, worse, to whatever inherited its identifier.

So the editor is derived FRESH, in `prepare`, from the observation that step itself made,
immediately before the step runs — and if it cannot be derived, the step **stops**. Nothing
falls back to focus, and nothing reaches the resolver with an empty query. A refusal is a
failure; two editors at once is a question.

The derivation pins the step's reference to the editor's own native id, which is the only
thing that tells it from the other controls in the window containing the same text. That
pin is **request-local**: the recorded intent still says "the editor for the thing I
bound", and the graph node's stored action has the identifier taken back out of it
(`unpinEditor`).

### Verification is what the world says, not what the call returned

- After an edit whose target was a derived editor, the editor is **re-found and compared**
  (`inline_editor_value`). A mismatch FAILS the step. A capability returning success says a
  call returned; the previous live attempt had exactly that.
- After a commit, the editor being **gone** is recorded (`inline_editor_closed`) — and it is
  explicitly HALF the check. A closed editor proves the transaction ended and says nothing
  about what it ended as. The filesystem correlation is the other half.

### What history keeps

`ActionNode.Editor` is an `inline.Snapshot`: what was edited, of which resource, in which
control class, the value before and after, and the evidence that tied the editor to the
object. It keeps **no element id and no native id**, and a test asserts that over the
serialised form. Replay re-enters edit mode from the stored binding and derives a fresh
editor; there is no field there it could use to skip that.

Replay analysis knows this. A node whose requested target is a derived editor is not
re-resolved against the world — there is nothing durable to look for — and is reported as
deriving its target at run time rather than as `TARGET_MISSING`. Before that change, an
action of this kind was unreplayable on the strength of a handle nobody should have been
keeping.

### Two defects the pipeline regressions found

Both were in code that had been written and never run end to end:

1. **A query pinned only by a native id was refused as unconstrained.** `ElementQuery`
   counted role, label, text and focus as constraints and not `NativeID` — so the one way a
   derived target can be expressed resolved to "the request did not describe anything
   specific enough to look for".
2. **A commit re-applied the discovery-time value check** and refused the editor it was
   about to commit, for the reason above.

### What is covered

`internal/director/inline` covers the correlation in isolation: the real editor found · the
Name cell refused · the search box and address bar refused · a distractor editor refused ·
two editors ambiguous · another window ignored · the selection moved refused · value
verified, wrong, closed, replaced · a hidden extension · a snapshot that cannot re-find the
control.

`internal/director/execute/editor_test.go` covers the pipeline using it, against a fixture
that has the decoy in it: rename mode acts on the bound file · the text goes to the verified
editor · text that did not land fails the step · a commit is aimed at the open editor and
proved by its closure · a commit that leaves it open fails · an absent editor stops without
reaching the host, the resolver or the graph · two editors ask · the derived pin does not
escape the request · the node records the edit and no way to re-find the control · replay
re-derives.

## Known gaps

- **None of this is live-tested.** Every claim above is against fixtures built from a real
  captured tree, not against a running Explorer. The live rename has still never completed.
- **The editor's `ValuePattern` reported *Unsupported*** from PowerShell against the real
  control, while the cached walk reported `value`. The edit ladder's fallback — select all,
  then type — has never run against this control, and it is the likeliest thing to be
  wrong on the next live attempt.
- **One application, one property.** `editorClasses` knows `UIRenameTextElement` and
  `PropertyFilename`. A second file manager, or an editable cell in a grid, needs its own
  entry; the empty case stays expressible, and it means "find no editor", never "guess one".
- **A commit's final value is not recorded.** The editor is gone by the time it is looked
  at, so the snapshot's `FinalValue` is filled by the edit step and not by the commit.
- **The live scenario has not passed.** It needs a desktop with no fullscreen application
  holding the foreground.
- **`Runtime.Windows()` reports the foreground window only**, because that is what the
  bridge walks. It observes on demand when its last look is over a second old, under a
  `TryLock`.
- **Only Explorer has shell identity.** A second file manager, or a document in an editor,
  would need its own discovery — the `ResourceIdentity` shape is ready for it and the
  `Source` field is what tells them apart.
- **Control labels are guesses.** Procedures name controls in English.
- **The goal parser is narrow.** It matches prefix patterns.
