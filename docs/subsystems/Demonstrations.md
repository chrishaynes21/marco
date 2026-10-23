---
type: subsystem
status: active
owners:
  - director
depends_on:
  - action-graph
  - goals
used_by:
  - goals
updated: 2026-08-06
source_paths:
  - internal/director/demo
  - cmd/director/democmd.go
---

# Demonstrations

Recording what somebody did, and extracting a **semantic procedure** from it — the second way
a procedure comes to exist, beside the hand-written ones in [[Goals]].

## Recording adds no observation path

The recorder reads the action graph. It does not open a new way of watching the screen, which
means demonstration recording inherits every constraint perception already has rather than
introducing a parallel set.

## What a step keeps, and what it cannot

A recorded step keeps the verb and the query. It cannot keep a coordinate, a handle or a
window generation, for the reasons in [[ADR-009-window-identity-is-ephemeral]].

Goal recovery then reads the actions and proposes what the demonstration was *for*.

## What becomes a parameter

The pieces that varied, or that are obviously an argument — a filename, a target. This is
proposed and shown, never inferred silently.

## Approval is a step nothing skips

A learned procedure is an **ordinary procedure**: same type, same validator, same safety
declaration, same downstream path. It does not get a privileged route into execution because
a person demonstrated it.

## Goal-centric since 2026-08-17

A demonstration is EVIDENCE, never the capability. `A → B → C` decomposes into one
candidate per grown edge; the leg arriving where the person stopped is the one the Learn tail
(`learnTail`) carries forward, and the destination becomes a durable goal in the person's own
words ([[ADR-056-a-goal-is-a-destination-not-a-route]]). Attributed input survives every failure
of interpretation ([[ADR-057-attributed-input-survives-interpretation]]), and a click that
resolved to a named control is as learnable as a keypress
([[ADR-058-a-demonstrated-target-may-keep-its-name]]) — it rehearses as an invocation of
that control, though a saved play still cannot say it (`cannot_say_pointer`, follow-on).

## Responsibilities

- Record from the action graph.
- Recover a goal from a sequence.
- Propose parameters.
- Refuse what cannot be represented.
- Require approval before a procedure becomes usable.

## Related systems

- [[Action-Graph]] — the source
- [[Goals]] — where a learned procedure lands
- [[Programs]] — unchanged by any of this

## Decisions

- [[ADR-009-window-identity-is-ephemeral]]
- [[ADR-005-legal-marco-only]]
- [[ADR-053-a-search-box-on-the-last-page-is-not-a-step]]
- [[ADR-054-the-one-shot-candidate-belongs-to-the-session]]
- [[ADR-056-a-goal-is-a-destination-not-a-route]]
- [[ADR-097-language-names-the-outcome-the-graph-decides-the-way]] — the layer above the graph:
  one resolver shared by the performer and the diagnostic, a phrase that means two things is a
  question rather than a sort order, a reused name says what it used to mean, and aliases are
  two names over one destination with no duplicate topology
- [[ADR-096-observe-and-learn-are-two-doors-into-one-graph]] — Observe and Learn differ in
  acquisition INTENT and not in what they produce: both write canonical Places, Targets and
  Edges into one store, an edge's identity is its two endpoints and nothing else, and a Play is
  a registered phrase plus a readable artifact rather than the execution path
- [[ADR-057-attributed-input-survives-interpretation]]
- [[ADR-065-operating-marco-is-not-demonstrating-to-it]]
- [[ADR-066-stop-is-a-product-event]]
- [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] — the same two decisions, arrived
  at again in the widget recorder once Studio got a visible Stop button: the recorder's own
  window is excluded wherever it appears, by the process that owned the foreground at the
  INSTANT of each event rather than by a name an interval poller sampled, and Stop, Cancel and
  Save are three acts rather than one
- [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] — a coordinate is a claim about a
  place only while the window it was measured in is still that window. A press records the basis
  of its offset, the window's outer size, its DPI as its own term and the program that owned it,
  all the way into saved legal Marco; replay binds the window at `Activate` and re-reads THAT
  handle before every press, refusing exactly rather than falling back. Carrying the terms is
  the opt-in, so every route already saved runs by the rule it was saved with. **And the terms go
  all of them or none**: the recording side degrades to an ordinary point when it cannot measure
  everything, the replay side refuses anything that states the basis and omits a term, and
  unreadable is never read as matching — because a check skipped for want of a term is a guard
  whose strength depends on what the recording happened to capture and which says nowhere that it
  was skipped. The window is bound WITH THE PROCESS THAT OWNED IT, since two copies of one program
  share an executable name and only the process id separates the window the route prepared from
  one that inherited the handle; and the offset must lie inside the recorded rectangle, because
  origin-plus-offset is a point on the desktop and every other check can pass while the cursor
  leaves the window
- [[ADR-250-the-recorder-runs-in-the-window]] — and the window those four buttons are in. The
  recorder-first path runs in Studio's OWN process, because a draft that writes nothing cannot
  cross a process boundary without being written somewhere; so Studio supplies its own process
  id as the exclusion boundary, starting is separable from finishing, and one predicate now
  answers "is something being recorded" for both recorders
- [[ADR-058-a-demonstrated-target-may-keep-its-name]]
- [[ADR-074-one-demonstration-every-leg-reviewed]] — every leg of one demonstration is reviewed,
- [[ADR-075-a-learn-episode-outlives-its-sessions]] — session boundaries are not Audience workflow
- [[ADR-077-consent-is-the-audiences-authority-is-marcos]] — a yes may never be reported as a
  decline; an answer.s application comes from the question
  boundaries; the newest Runner is no authority on an older answer
  in walk order, and an absent question is not an answer

## What a demonstrated click is evidence of

The point somebody clicked is authoritative about WHICH INSTANCE of a control they meant, and is
not a claim that the same instance still occupies that location. A template is the other way
round: it says what the control looks like and nothing about which of two identical ones was
intended. [[Experiment-096-the-click-says-which-one-the-template-only-says-what]] measured the
gap on a seeded fixture — matching the template anywhere in the window chose the control in the
*other* panel thirty times out of thirty whenever the demonstrated one was absent or covered,
with no ambiguity reported, while the same matching restricted to the neighbourhood the click
predicts made no wrong-instance selection at all and refused every unsupported case. It is an
experiment and not a decision; nothing in production was changed to run it.
[[Experiment-097-a-translucent-menu-is-not-what-defeats-a-local-template]] then asked the same
method the next question, on the owner's own captures of a real translucent game menu: the
template survives the world moving behind it (0.92 to 0.99, against rivals at 0.59 to 0.72), the
wrong-instance hazard is present there too — a repeated volume value ranks a DIFFERENT row above
the demonstrated one — and the threshold frozen against an opaque fixture does not transfer.

**And a demonstration now keeps what its controls are worth.** Per pressed control: the click in
WINDOW pixels, the size of the capture it is relative to, a display scale recorded as UNKNOWN
because nothing in the recording path observes one, a crop of the control AT CAPTURE resolution,
and a two-sided measurement of that crop — the worst it matches itself across the looks ENDING AT
the press, against the best anything within 160px matches the same template. The looks ending at
it rather than after it, because a press that works closes the menu it was in and a control
measured across that reads as unstable because the click SUCCEEDED. A threshold sits in the gap
between the two numbers, or there is no gap and the recording says so while the person is still in
front of the screen. The region comes from the proposal under the press rather than from a
constant, because a constant means something different at every capture scale. Nothing that runs
reads any of it: it is what a guard around a learned coordinate would be built from, collected
first so the guard has something to be built FROM.
[[ADR-247-what-a-click-is-evidence-of]]

**And a demonstration records the point of every press.** The click itself, taken from the press
and never computed from a rectangle a proposer drew around it — a proposal is generous on purpose
and its middle is a different point, which on an off-centre click can be a different control.
Kept per STEP rather than per control, because a control pressed twice was pressed twice and the
two points are two facts while readings merge. A proposal is not required for any of it: a press
the proposer found NOTHING under still records where it was, and on a menu no recogniser can name
that is the ordinary case. It is demonstration evidence and nothing on the recognition path reads
it. [[ADR-248-replaying-a-demonstrated-layout]]

Kept by BOTH loops that build a step: the one that walks the transitions a demonstration made,
and the one that picks up presses which changed a panel without changing the screen. It reached
only the first at first, which refused exactly the presses carrying the best evidence in the
project — on the owner's second Ark recording the two panel presses match themselves at 0.99999
against rivals at 0.35 and 0.44, while the one transition step proposed a region covering the
whole window and matched itself at 0.73. A press that changes a panel under a tab bar that stays
put is the ordinary case on the menus that route exists for, so it was also most of them.

**And a demonstration records the window it was made in.** Outside the pictures licence, because
a size is not a picture of anybody's screen — and nothing at all when that window was resized
partway, because that is not a demonstration of a fixed layout. It exists so that a replay can be
REFUSED: every reading here is normalised to its window, so the arithmetic produces a
confident-looking point at any size whatever, and this is the only thing that can say whether the
window a demonstrated point is about to be pressed in is the one it was demonstrated in. A
routine recorded before this is refused on that route rather than trusted, and told that a fresh
demonstration is what fixes it. [[ADR-248-replaying-a-demonstrated-layout]]

**And there is one look number, kept by the observer.** Everything a demonstration files under a
look — the raw moment, the picture of the screen, the band of pixels a control is calibrated from
— is read back by the look the OBSERVER attributed the press to. The loop kept a second counter
of its own, taken before the fold, which ran one behind; every lookup between the two halves
missed by one and returned nothing, so no control crop and no control evidence was written by any
build, and nothing said so. Found by the absence of a directory three recordings should have
made. A total miss where presses were attributed and looks were measured is now a sentence rather
than a count of zero.

## Validated by

- `internal/director/demo` unit tests (`demo.go`, `record.go`, `validate.go`)
- `director demo` commands; explainability output
- `internal/director/localizetrial` — the local-regioning experiment: its fixture, its frozen
  protocol and its evaluation. Opens no window and sends no input.
- `internal/director/controlevidence`, `internal/director/match` and `cmd/director` — what a
  pressed control is worth, measured on drawn pictures for the rules and on real 1920x1080 game
  frames for the number.

## Known gaps

See the *Known gaps* section of [[director-demonstrations]].

## Milestone record

[[director-demonstrations]].
