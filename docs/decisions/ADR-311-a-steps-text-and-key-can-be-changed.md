---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — A STEP'S TEXT AND A STEP'S KEY CAN BE CHANGED, THROUGH THE SHARED PREVIEW/SAVE PATH. Two new step-edit operations, `text` and `key` (`orchestrator.EditText`, `EditKey`), go through the same prepare/review/save machinery as every other step edit: nothing is written until a review is saved, the stored step account is written beside the source, and the demonstration is untouched. New text is fixed text, typed exactly as given. `{{` and `}}` are refused, because an undeclared placeholder renders as a credential-store read; asking for a value stays `argument`'s. A key is checked against the keys Marco can press while the person is still looking at it. A press may be a chord; a hold is one key, and changing either half of a hold changes both. The keys Marco can press are now ONE cross-platform vocabulary (`macroir.KeyNames`), held on Windows against both the host's table and the recorder's. That guard found that a recorded Caps Lock could not be pressed, which is fixed. Studio offers both as boxes on the step rows, in the control centre's manner. The control centre's own source-line save is not used."
supersedes: []
affects:
  - interface
  - execution
source_paths:
  - internal/orchestrator/stepedit.go
  - internal/macroir/keys.go
  - internal/oshost/keys_windows.go
  - plugins/studio/stepedit.go
  - plugins/studio/frontend/index.html
---

# A step's text and key can be changed

## Why

The owner prefers the control centre's editor ([[ADR-310-studio-wears-the-control-centres-look]]),
and the first thing it offers that Studio did not is editing a step's literal text and the key a
press uses. The control centre does it by rewriting source lines and saving with
`Registry.Save`, which leaves the stored step account stale; `cmd/marco`
`TestTheLegacyEditorsSaveLeavesTheAccountMismatchedAndTheRoutineRunnable` records that. So these
are new operations on Studio's own step-edit contract, not a port of that handler.

## The decision

- **`text`** (`EditText`) replaces what a typing step types with other fixed text.
  - It is taken exactly as given, spaces included.
  - Refused, each with its own reason:
    - empty text: remove the step instead;
    - the same text it already types;
    - a step that types nothing;
    - a step whose text is already a value you supply;
    - text containing `{{` or `}}`.
  - **The braces** are refused, not escaped. A typed literal with an undeclared `{{name}}`
    renders as `do OS's Secret …`, a read of the credential store. New text with a brace in it
    would therefore change what the routine does, not what it types. Making text something you
    supply stays `argument`'s, which declares the name.
- **`key`** (`EditKey`) changes which key a press, a hold or a release uses.
  - It is lowercased and trimmed, because that is how `Step.Key` spells a key.
  - A press may be a chord (`ctrl+s`, `macroir.IsChord`). A hold is one key
    (`macroir.IsKeyName`), which is how the recorder writes a held modifier.
  - **A hold changes whole.** Changing either half of a keydown/keyup pair changes both. The
    halves are found by `holdPartner`, the function a removal uses, and the review says both
    step numbers. The same key asked of both halves is one change; different keys are refused.
- **Two things asked of one step are refused in either order:**
  - new text and removed;
  - new text and asked for;
  - two different texts;
  - a new key and removed, including the other half of a hold.
- **Studio** offers a text box on a typing row and a key box on a press, hold or release row.
  These replace the value half of the row, as a press's two numbers already do. Like every
  other box they are read when Review is pressed, and Reset puts the row back. An empty box is
  a correction on its row and is never sent. What a key may be is the backend's answer at
  review; the page keeps no list of keys.

## One list of keys

A key step's name has to mean the same thing in three places:
- the recorder that writes it (`recorder.vkNames`, Windows);
- the host that presses it (`oshost.keyMap`, Windows);
- the editor that lets a person type one.

The editor's check had to be cross-platform, so `macroir.KeyNames` is the vocabulary, and two
Windows tests hold it to both tables:
- `oshost` `TestTheHostPressesExactlyTheKeysMarcoNames`: every name resolves on the host, and the
  host resolves no name outside the list.
- `recorder` `TestEveryKeyTheRecorderWritesIsOneMarcoCanPress`.

**What the guard found.** The recorder writes `capslock`, and the host could not press it, so a
recorded Caps Lock was a step that failed with `unknown key` at run time. The host now presses
it (`vkCapital`). The guard also found that the host's `" "` entry can never be reached, because
`resolveKey` trims the name first, so it is not in the vocabulary.

## Verified (Windows only)

- **Orchestrator:** every rule and conflict, in both orders. End to end through prepare, save
  and the production driver over a recording host:
  - the changed text is what the saved routine types;
  - the changed hold is what it holds and releases, with no shift left;
  - the routine can be edited again afterwards, which shows the account was written.
- **Studio:** through `StepEditing`, `ReviewStepEdit` and `SaveStepEdit`:
  - a press's key is saved;
  - an unknown key is refused with nothing written;
  - both halves of a hold change;
  - typed text, trailing space and all, is saved after the run is declared typed.
- **Rendered,** in headless Edge with stub bindings:
  - which rows get which box, and what each box starts with;
  - what Review sends, exactly;
  - an empty key box is an error on its row and is never sent.
- **Mutants:** 17, each applied alone and the file restored, all killed. They covered: braces
  allowed; a supplied step's text changeable; the other half of a hold left alone; a removal
  ignoring new text; new text or a new key never applied; any key accepted for a press; a chord
  accepted for a hold; a key not lowercased; Studio dropping both fields; hold rows not keyed;
  the page never sending text; no key box; an empty key sent; a changed text not marked as
  changed; the host unable to press Caps Lock; and the vocabulary missing a recorded key.

## Not established

- **Not seen on the desktop.** No Studio window was opened. The routine was run against a
  recording host, not a real one.
- Inserting steps, repeats and drags are later slices.

## Enforced by

- `internal/orchestrator` `TestAStepsTextCanBeChangedToOtherFixedText`,
  `TestNewTextIsRefusedWhenItIsNotFixedText`, `TestAPressCanUseAnotherKeyMarcoCanPress`,
  `TestAKeyMarcoCannotPressIsRefusedAtReview`, `TestAHeldKeyChangesWholeFromEitherHalf`,
  `TestTwoThingsAskedOfOneStepAreRefusedInEitherOrder`, `TestAChangedTextIsWhatTheRoutineTypes`,
  `TestAChangedHoldIsWhatTheRoutineHolds`.
- `internal/oshost` `TestTheHostPressesExactlyTheKeysMarcoNames`; `internal/recorder`
  `TestEveryKeyTheRecorderWritesIsOneMarcoCanPress`.
- `plugins/studio` `TestAPressesKeyIsChangedThroughTheStepPanel`,
  `TestAKeyMarcoCannotPressIsRefusedByTheStepPanel`,
  `TestAHeldKeyIsChangedWholeThroughTheStepPanel`, `TestAStepsTextIsChangedThroughTheStepPanel`,
  `TestTheStepPanelOffersTextAndKeyBoxesAndSendsWhatWasTyped`.

## Correction: a changed key keeps no character from the old one (2026-11-04)

A review found that `EditKey` updated `Step.Key` and kept `Step.Text`. On a recorded tap, that is
the character the old key produced. `oneCharacter` trusts `Text` before `Key`, so the bug showed
up only in a later edit:
- `a` changed to `q`, saved, and then declared typed text came back as `a`.
- `a` changed to `ctrl+s` still offered `"a"` to a conversion that must refuse a chord.

Playback straight after the edit was right, and that is all the end-to-end tests above checked.

A changed key now clears `Text`, on both halves of a hold. With it empty, every reader falls back
to the key: `oneCharacter` reads `q` as `q` and refuses `ctrl+s`, and codegen renders a key step
from `Key` alone.

Held by:
- `internal/orchestrator` `TestAChangedKeyKeepsNoCharacterFromTheOldOne`;
- `TestAKeyChangedToALetterIsThatLetterWhenLaterDeclaredTyped`, which goes a → q, save, then
  typed text, and types `mqrco`;
- `TestAKeyChangedToAChordIsNotTypedTextLater`, which goes a → ctrl+s, save, then typed text,
  and is refused naming the chord.

With the fix removed, all three fail, and the second reproduces the report: the routine types
`marco`.

## Correction: a refused change keeps what was typed (2026-11-04)

A refused review used to list the steps again, which throws away every box. That was right for
the refusal it was written for: the routine was written since, so the positions no longer mean
what they meant. Key editing made a second kind of refusal ordinary. An unknown key is refused
against a list that is still current, and listing the steps again would discard the other five
boxes along with the one that is wrong.

So an operation's refusal is now marked (`orchestrator.ErrChangeRefused`, through `errors.Is`).
Studio reports it as `StepPreview.Correct`, and the page then keeps everything typed and shows
why. Every other refusal still lists the steps again.

Held by:
- `plugins/studio` `TestAKeyMarcoCannotPressIsRefusedByTheStepPanel`: an unknown key is a
  correction, and a list nobody was shown is not.
- `TestTheStepPanelOffersTextAndKeyBoxesAndSendsWhatWasTyped`: what was typed is still in the
  boxes after a correction.

Three mutants were killed:
- the mark dropped;
- the page ignoring it;
- every refusal reported as a correction.
