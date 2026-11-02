---
type: adr
status: accepted
date: 2026-11-02
decision: "ACCEPTED — STUDIO SAYS WHAT IT ALREADY KNEW, WHERE THE PERSON IS LOOKING. Five repairs from [[Studio-coherence-audit-2026-11-02]], none of them new capability. (1) `Recording.Away` is drawn on the review card. The backend has sent it since recordings learned to pause, with a comment saying it belongs there, and the page never read it, so a routine came back shorter than the demonstration with nothing saying why. (2) A REFUSED RENAME IS SAID BESIDE THE NAME BOX, whether it was refused before a review or by preparing one, and the review panel opens only for a rename that can happen. The panel used to open for a collision, describe where the renamed routine would be filed and that everything would come with it, and only then say it would not happen, a card's length below the box. The collision sentence now names the routine as the person typed it and its application (`\"open settings\" is already a routine in arkascended`), not the route id it is filed under. (3) The run confirmation's lines span the row; they were dropped into the verb column of a three-column grid and broke over two lines. (4) The ask box's application selector always has `no application`: an empty answer matched the empty starting signature, so with nothing running it had no options at all. (5) EVERY PLAY IS DARK WHILE SOMETHING IS BEING RECORDED, with `Finish the recording to play.` on its row, because the backend refuses a Play then and a live button answering with a refusal is a press wasted on finding out. A routine that cannot run keeps its own reason. And the line naming what Play waits for says `Enter item to play.` — it said `below`, and the field is above it at a narrow width."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
  - internal/orchestrator/rename.go
---

# Feedback the window had and did not give

## What happened

[[Studio-coherence-audit-2026-11-02]] findings 5, 8, 9, 10, 15 and 17. The Architect's ruling:

> Show `Away`, fix confirmation-list layout, keep rename collisions beside the name field,
> initialize the empty application selector, and disable Play during recording with a short
> reason. Rename refusal must not first describe a successful outcome.

and, on the mockups:

> Replace directional wording such as "Fill in item **below**" with "Enter item to play."

## Two that changed a test's claim, and why

**The rename test said the opposite.** `TestARefusedRenameSaysSoWhereTheAnswerIs` asserted that a
collision belongs in the review panel, *"because there is a review to read"*. There is a review,
but it describes an outcome that will not happen. The answer to both refusals is a different name,
typed where the cursor already is. The test now asserts both refusals beside the box and no
review. A `refused` that arrives with a token hands the token back.

**The backend's sentence changed, in one place.** `internal/orchestrator/rename.go` wrote the
collision with the route id. It now says the name somebody typed and the application, or *that
works everywhere* for a routine with none. The rescope engine's own *taken* sentence, which offers
a consented replacement, is unchanged. That sentence is about a place, and its reader chose the
place.

## Enforced by

- `plugins/studio` tests:
  - `TestAReviewSaysHowLongTheRecordingWasLeft`;
  - `TestAnOrdinaryReviewHasNoAwayLine`;
  - `TestEveryPlayIsDarkAndSaysWhyWhileRecording`, which also checks that a forced press opens
    nothing and runs nothing;
  - `TestPlaySaysWhatItIsWaitingForWithoutADirectionAndTheConfirmationIsOneLine`, which checks
    the wording, the selector's `no application`, and that the confirmation line is one line tall.
- `plugins/studio` `TestARefusedRenameSaysSoWhereTheAnswerIs`: both refusals beside the box, no
  review panel, nothing written.
- `internal/orchestrator` `TestARenameOntoAnOccupiedNameIsRefusedWithNothingToAgreeTo`: the
  sentence names the routine and its application, and contains no route id.

Five mutations were run, one per repair, and each failed its test: not showing `away`, the empty
starting signature, a live Play during recording, the confirmation line in the verb column, and a
collision routed back to the review panel.
