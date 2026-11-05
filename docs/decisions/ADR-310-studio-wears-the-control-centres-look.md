---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — STUDIO WEARS THE CONTROL CENTRE'S LOOK, ON ITS OWN PAGE AND BACKEND. The control centre (`m help`, `cmd/marco/edit.go`) is the look the owner prefers: one monospaced face, the accent as structure, compact framed action rows, an uppercase drawer with labelled groups, little prose. Studio's existing page is restyled to it rather than replaced. There is no second page and no launch switch, so every Studio workflow (Record, review, Play, window choice, supplied text, Rename, shortcuts, Help, shared themes) is untouched. Every colour still comes from the widget's palette. The page's fallback colours are now the widget's DEFAULT and LIGHT palettes exactly, so the control centre's cyan is the default palette's accent and not a colour the page chose. The richer editor (text and key edits, inserting steps, repeats, drags) follows in later slices, each through Studio's preview/save machinery. The control centre's source-line save handler is not transplanted."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/studio/frontend/index.html
---

# Studio wears the control centre's look

## Why

The owner opened `m help` and preferred it. That command opens the control centre, an older page
inside `cmd/marco/edit.go`, with its own HTTP endpoints. Two ways to use it were weighed:

- **Host its page in Studio and rewire it** to Studio's 83 bindings.
- **Restyle Studio's page to it,** and bring its editor's interactions over one at a time.

The first means rebuilding Studio's workflow state inside a page that never had it: the window a
run presses in, supplied text, read-back review, the run view, rename, shortcuts and themes. The
second keeps all of it, and keeps about a hundred rendered tests protecting it. The owner chose
the second.

The control centre's own save is not carried over. It writes the edited source straight through
`Registry.Save` and leaves the stored step account stale; `cmd/marco`
`TestTheLegacyEditorsSaveLeavesTheAccountMismatchedAndTheRoutineRunnable` records that.

## This slice: the visual shell

- **One face, monospace.** `--display` and `--text` are the monospace stack, as in the control
  centre. `--mach` stays as a separate role that marks machine facts.
- **The drawer.** The rail's places are uppercase and letter-spaced. The current one is in the
  accent, with a 3px rule down its left edge over the accent's tint. The widget sits under a
  labelled `Widget` group. The control centre's drawer slides over the page; Studio's stays open,
  because the window is wide enough, and narrows below 760px instead of hiding. Record, Play and
  every destination stay one press away.
- **Titles** are the accent, uppercase and spaced. The name is `MARCO studio` in the control
  centre's header style, with a glow mixed from the accent so each palette gets its own.
- **Compact action rows.** A routine is a framed row on the raised panel, a few pixels from the
  next. An open routine is marked by the accent's edge, not filled with its tint. Each step in
  the editor is its own framed row, and Remove uses the control centre's red delete style.
- **Controls.** Buttons are the accent on its tint. Play is the filled accent, glows on hover,
  and is still at least 44px tall (`TestPlayIsTheBiggestThingOnARowAndTheRowReadsLeftToRight`).
  Badges and tags are small square uppercase labels.
- **Less prose, by weight.** Leads and secondary lines are smaller and dimmer.

## What this slice did not do

- **No sentence was cut.** The prose a person meets most is pinned on purpose by earlier
  rulings and their tests: the capture disclosure, the F9 instructions, Simplify's cost stated
  where it is chosen, and the scope explanation at review. Shortening those is a decision about
  each sentence, not a style change, so it is left for its own pass.
- **No editor capability.** Editing text and keys, inserting steps, repeats and drags are the
  next slices.

## Reversed

The page's own comment used to record the opposite ruling. An older Studio was one 13px monospace
for everything, the owner called it "very ugly", and prose moved to a humanist face. That
reasoning was only ever written in the page. It is replaced there and named here.

What is different now is the rest of the character that came with the face: the drawer, the
framed rows, the tighter scale, and the accent used as structure.

## Verified (Windows only)

- Rendered in headless Edge with stub bindings, through the screenshot harness
  (`docs/images/studio-ui/9-harness`). The states were library, open routine, review, step
  editor and Help, dark and light, at 1180px and 640px. Files are
  `docs/images/studio-ui/11-cc-*.png`.
- **The review fixtures were refreshed to what the backend sends today.** Three harness states
  (`07-review`, `08-review-refusal`, `17-review-simplified`) still carried the pre-S9 shape: a
  forced scope with a `brings` sentence saying the other two choices would refuse. Since S9
  ([[ADR-306-a-guarded-routine-may-be-only-in-its-application]]) a guarded recording keeps Only
  in and From anywhere → app. Only the current-foreground scope is withdrawn, with
  `WhyNotThisScope`'s own sentence, and nothing writes `Brings` any more. The fixtures now send
  `default_scope` and that one `unavailable` reason, built from the backend's `ScopeLabel`
  wording. Production behaviour did not change.
- The full `plugins/studio` suite passes, along with the `plugins/e2e` page guards. The first run
  failed only the 44px Play rule, because this slice had made Play 40px. Play was put back to
  44px.
- The fallback test killed both of its mutants: the old periwinkle accent, and a light ground
  changed in one of the two light blocks only.

## Not established

- **Not seen on the desktop.** No Studio window was opened. WebView2 is Chromium, like the
  headless Edge the page was rendered in, but the installed fonts and scaling on the owner's
  machine were not checked.

## Enforced by

- `plugins/studio` `TestThePagesFallbackColoursAreTheWidgetsDefaultAndLightPalettes`: every
  fallback colour equals `widgetconfig.StudioTokens("default")` (dark) or `("light")` (both
  light blocks).
- `plugins/studio` `TestPlayIsTheBiggestThingOnARowAndTheRowReadsLeftToRight` and
  `TestTheRowsHoldTogetherInANarrowWindow`: the restyle keeps Play the biggest control on its
  row, and keeps the rows whole at narrow widths.
