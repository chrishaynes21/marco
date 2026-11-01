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
updated: 2026-10-24
source_paths:
  - internal/director/demo
  - cmd/director/democmd.go
  - internal/orchestrator/generation.go
  - internal/orchestrator/regenerate.go
  - plugins/studio/routines.go
  - internal/runs
  - internal/marcopaths
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
- [[Coordinate-MVP-assessment-2026-10-24]] — what of the coordinate-first Studio path is built,
  what remains, and the estimates for chords, typed text and holds. Assessment, not a commitment.
- [[Studio-UI-audit-2026-10-29]] — the four states of Studio's main journey measured and
  redrawn: 1,436 visible words in 207 messages, 39 controls on one view, two Record entry
  points, and Play as the eighth control after selection. Audit and proposed layout, with the
  behaviour changes named apart from the layout ones. No code changed by it.

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
  Save are three acts rather than one. **And a click's two edges can belong to two processes**:
  the press that raises a window arrives while the previous window is still in front, so a click
  onto the recorder is attributed (application, recorder) and the click back (recorder,
  application) — excluded edge by edge, each lost one half, and the halves left behind were a
  press and a far-away release with nothing between them, which `simplify` reads as a drag and a
  guarded routine cannot save. So clicks are PAIRED before anything is excluded, per button and
  never across a second press of the same button, and a click either of whose edges is the
  owner's is the owner's — which is not the same rule as "the edges disagree", because a click
  that raised another program's window disagrees and is a demonstrated press
- [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] — a coordinate is a claim about a
  place only while the window it was measured in is still that window. A press records the basis
  of its offset, the window's outer size, its DPI as its own term and the program that owned it,
  all the way into saved legal Marco; replay binds the window at `Activate` and re-reads THAT
  handle before every press, refusing exactly rather than falling back. Carrying the terms is
  the opt-in, so every route already saved runs by the rule it was saved with. **The BASIS is the
  contract and everything else is a measurement**, so a saved route says three things apart — no
  basis is legacy, a basis with every term is guarded, a basis with a term missing is incomplete
  and refused naming what it lacks — and which of the three a press is in does not depend on a
  measurement having succeeded. A failed measurement means guarded replay is unavailable for that
  recording, never that its requirements become optional and never that it quietly becomes an
  ordinary point, which is pressed at its recorded coordinate having checked nothing. Review names
  what was not measured and says whether recording it again can change the answer; saving is
  allowed and is not quiet. Unreadable is never read as matching. The window is bound WITH THE
  PROCESS THAT OWNED IT, since two copies of one program share an executable name and only the
  process id separates the window the route prepared from one that inherited the handle; and the
  offset must lie inside the recorded rectangle, because origin-plus-offset is a point on the
  desktop and every other check can pass while the cursor leaves the window. **And a refusal stops
  the routine**: a guarded route is generated as a chain — the preparation and every guarded press
  each in their own capability, calling the next only when its own action went through — so
  nothing after a refusal runs, no unconditional ending can override it, and a guarded press is
  never folded into a `repeat`. The run's answer is data rather than a log line, because a root
  handler is compulsory in Marco and a routine that did not happen therefore ends tidily — and the
  account covers a press the machine would not DELIVER as well as one it could not check, which
  are the same outcome and were not the same record. A step whose failure cannot stop the routine
  is refused at generation and said at review — a press found by matching a picture or by reading
  words, and a repeated block; a non-left press was on that list until
  [[ADR-260-a-press-is-a-button-and-a-place]]. **A KEYSTROKE IS A
  LINK TOO**, and not because anything checks it: it is a link because it can fail to be
  DELIVERED, and what follows it then acts on a screen the key never changed — an Escape that did
  not arrive leaves the menu open, and geometry cannot detect that, because the window is the same
  window. A repeated key is expanded into one link per press, since a `repeat` block runs its body
  again after the thing that did not happen. The failure is kept in the run's account beside the
  presses, so the command fails with the machine's own words and Studio shows them; typed text and
  held keys are not covered and the ADR says so. New guarded
  recordings are saved to bring their application forward, since the scopes that switch nothing
  bind no window; the choice is withdrawn rather than defaulted, an impossible scope is refused at
  Save, and a routine already saved that way is saved again from its own stored demonstration
- [[ADR-252-a-library-is-a-place-and-a-routine-is-played-where-it-was-made]] — the two gaps
  between that mechanism and a product somebody can use. A library is a PLACE: the route store was
  `$MARCO_ROUTES, else "routes"` derived in four places, which is a relative path, so the product
  had as many libraries as it had launch locations — a routine saved by a Studio double-clicked in
  `plugins\studio` went where nothing else looked, and the documented remedy was to launch both
  binaries from the same directory. One resolver now answers for every binary, the answer is
  absolute including a relative override, and the default is `routes` inside the store they already
  share. Moving a default makes an old library go QUIET rather than disappear, so the window names
  any library-shaped directory beside its launch location that holds routines and says what to do;
  nothing is adopted and nothing is moved, because choosing between two libraries is the person's.
  **And saving is no longer where the flow ends**: a routine is chosen and performed by its WHOLE
  identity — `<app>/<scope>/<slug>`, its place in the library — because a name is not unique and a
  slug is not either, and every lookup that travelled by name answered with the first row that
  matched: clicking the second of two `settings` cards displayed and performed the first one, with
  the full identity reaching the child correctly after the wrong routine had been chosen. Both
  listings are keyed by identity rather than slug, since keyed by slug a context-scoped routine
  that cannot press anything disabled the valid focus-scoped routine of the same name and showed
  it that one's reason. Every saved routine is listed with the contract
  it was saved under, read off the saved Marco by one reader tied by test to the six terms the
  press guard requires, so the one that checks every press says so, the one that cannot press
  anything is not offered, and the one missing a measurement names what it lacks. Play raises the
  application itself — only the foreground process may — then spawns `marco do --play` for the
  route's WHOLE identity with the library named explicitly, because Studio acquires no execution
  loop and a second place deciding what a run's result was is a second answer to "did that work".
  The answer is the published outcome word read through the package that owns it; one run slot is
  shared with Acts and with recording, since there is one screen; and a routine's run is rendered
  and journalled as what it is — no canonical reference, no phase, no question — because a view
  expecting a supervised performance reports a perfect routine as a run that cited nothing
- [[ADR-250-the-recorder-runs-in-the-window]] — and the window those four buttons are in. The
  recorder-first path runs in Studio's OWN process, because a draft that writes nothing cannot
  cross a process boundary without being written somewhere; so Studio supplies its own process
  id as the exclusion boundary, starting is separable from finishing, and one predicate now
  answers "is something being recorded" for both recorders
- [[ADR-259-a-guarded-drag-is-two-points-in-one-window]] — and the gesture that mechanism refused
  to save. A drag already worked and was already one `do` sentence; what it lacked was a
  measurement, and the refusal's own sentence described the recorder rather than the gesture. BOTH
  EDGES of a click are measured now — a release used to carry no geometry, which was right while
  the only two-edged gesture was a click, whose release lands where its press did — and each edge
  carries the window HANDLE, its owner and the rectangle's ORIGIN as recording evidence that never
  reaches a route, because geometry and a program name cannot say WHICH window: two windows of one
  executable at one size satisfy every comparison of those. `simplify` then tells six recordings
  apart, and the distinction worth the words is between what the PERSON did — a gesture across two
  windows, a window that moved, one that was resized, a drag out of the window, each refused with
  a sentence naming it — and what the machine would not report, which gets the basis alone and no
  instruction, because telling somebody to demonstrate it again cannot help when the program had
  exited. At replay both ends resolve from ONE answer, since asking twice would let a window that
  moved between them relocate the destination; and the window is asked AGAIN immediately before
  the button goes down, against the snapshot those points were computed for, because the gesture
  AIMS in between and an injected pointer move changes applications. A change there refuses rather
  than re-bases. Cleanup is a defer inside the gesture that ARMS ON AN ACCEPTED PRESS and not on
  entry, so a panic during the aim reports nothing; a ledger would only buy the sweep's second
  attempt, and forced termination is outside all of it
- [[ADR-260-a-press-is-a-button-and-a-place]] — and the press that mechanism refused to save, for
  the same kind of reason one slice later. A right or middle press was refused because it was an
  aim and then a click at the cursor: two sentences, where a chain can only stop at one. That
  described the EMISSION. The recorder had measured these all along — `newClick` takes the button
  as a parameter and reads the window the same way for all three — and what was missing was
  somewhere in the saved route to put the button. `Point` carries `Button` now, absent meaning
  left so that not one saved file changes, and it sits with the COORDINATES rather than among the
  window terms because nothing about it is compared with the desktop: the contract reader's list
  of terms is unchanged by it, so a right press reads back as guarded with no new shape for any
  reader to learn. A host-module change and not a Core one. AN UNKNOWN BUTTON NAME IS NOW A
  REFUSAL, which is a defect this found rather than one it introduced: the flag lookup answered
  LEFT for anything it did not recognise, so a route naming a button that does not exist pressed
  the left one at a correctly guarded point — every safeguard doing its whole job while the act
  was the wrong act. It is asked FIRST, before the window, because a button nothing can press is a
  fact about the route and not about the desktop. AND THE PRESS READS THE STOP, TWICE, HAVING
  READ IT NOWHERE: on entry, so a stopped run neither presses nor interrogates a desktop it was
  never going to press in, and again immediately before the button goes down, because confirming
  the window takes time of its own and whatever is last before the press is what has to be read
  there. A press that measured nothing is unchanged, Director's own lowering is unchanged, and no
  saved routine is regenerated
- [[ADR-261-typed-text-is-a-sequence-of-submissions]] — and the input that can go in PART WAY,
  which no other one can. `do OS's Type with "hello"` is five events and any of them can be
  refused, so a failure leaves a field holding `he` and a next step written for `hello`. Typing
  was ACCOUNTED nowhere — dispatched through `ok(…)`, so a rejected send was recorded in nothing
  and the command reported success — PROPAGATED nowhere, since `StepType` was in no arm of
  `propagating`, and partial delivery was unrepresentable. What is counted now is SUBMISSIONS and
  not arrivals, because a count from `SendInput` says the system took the events and nothing
  more; the unit is a Unicode CODE POINT, never a byte offset and never a UTF-16 unit; and a
  character is submitted, refused, or PARTLY ACCEPTED — the third reported as uncertainty with
  the release attempted for a unit whose down went in without its up. The old cast truncated
  every code point above U+FFFF into a private-use character that sent successfully. A SECRET IS
  NEVER MEASURED rather than redacted: the kind travels into the gesture, so no number about it
  exists to leak, and the backend error that QUOTED the rejected character is gone. Partial
  progress is a SECOND account beside the held-key one, because a half-filled field is not
  something the computer is still doing, and it survives a cancellation into the record, the
  output and the card with the outcome staying `cancelled`. Every SEGMENT of a mixed text is its
  own link, split before the chain is built, since one step emitting three sentences left two of
  them where nothing could stop at them — a password going into a form whose username never
  arrived. Legacy routes keep their continuation semantics, asserted behaviourally
- [[ADR-262-a-routine-states-which-generation-wrote-it]] — and the thing every one of those rules
  has in common: it reaches a routine when that routine is WRITTEN, and never afterwards. A
  library therefore accumulates files written under rules that no longer match each other, and
  nothing on disk said which rules wrote a file. It cannot be inferred either — the shapes in a
  routine are a function of the demonstration as much as of the generator, which is exactly how a
  guarded drag read as `legacy` before `draggedTerms` existed. So the generator STAMPS it, as a
  comment, unconditionally: whatever called it, that generator wrote the file. IT IS PROVENANCE
  AND NOT A VERDICT and in particular it is not the contract — a current-generation routine can
  press without checking anything, because that is what its demonstration asked for. FOUR
  READINGS, none collapsing into another: current, older-and-recognised, UNSTATED (which says only
  that — not "written before versioning", a claim a handwritten file does not support), and
  UNKNOWN (a later build's number, a malformed value, or the same question answered twice). An
  unknown generation is offered nothing, because a reader that cannot say what a generation does
  cannot say what replacing it would change — and it stays inspectable and playable under exactly
  the rules it always had. Generation 1 is what this generator does, with NOTHING BEHIND IT:
  numbering the rule changes that landed before today would place routines in a history they never
  stated. OLDER IS NOT AFFECTED — a difference is a fact about two generators, so a routine is
  claimed to need replacing only where its own contents bear on the rule. And a replacement is
  four steps of which three write nothing: select, prepare, review both sources in full, then
  replace or cancel. `Resave` is not that flow and stays what it is, repairing a routine that can
  press nothing at all and rescoping to do it; this one reads the scope off where the routine
  already lives, and refuses if either the source or the recording moved while the review was open
  — which is also the whole of the protection for a hand-edited routine
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

## An attempt claims its reference, and each kind records in its own words

Both paths now put their runs in one directory, and that is **all** they share. `internal/runs`
owns the discipline — the place (`internal/marcopaths.Runs`), the claim made exclusively before
anything is sent, the provisional record left in that claim at once so an attempt that dies is
retrievable as an attempt that did not finish, and the finalisation that cannot create — and it is
handed a record it may not look inside.

The record is per kind because the **vocabularies are not translatable**. A supervised Act's run
accounts for evidence (`verified`, `contradicted`, `unconfirmed`); a coordinate routine's run
accounts for delivery (`performed`, `unverified`, `unavailable`, `clarify`). Three of seven words
overlap and nothing converts in either direction: a routine presses and looks at nothing
afterwards, so `verified` would be a claim about evidence it never gathered, and `unverified` is
not `unconfirmed` — the first says nobody looked, the second says somebody looked and could not
tell. Absent verification evidence stays absent rather than becoming three zeroes, which would
read as a run that verified nothing.

`cmd/director`'s `RunRecord` did not move; it became a caller, with its own words, its own fields
and its own build stamp. `plugins/studio/history.go` keeps reading records by their JSON names
rather than through a writer's type, which is what lets one window list a kind it does not know and
show the whole of it. [[ADR-253-the-claim-is-shared-and-the-record-is-not]]

**And a routine's run is citable now.** `marco do` claims its reference before the first input,
prints the citation, and finalises a record of its own kind: the exact route, the digest of the
saved source that ran, the library, the build, the contract (`legacy`, `guarded` or `incomplete`, through the one
reader `orchestrator.ContractOf`), `unstated` for a mode nothing states yet, both timestamps, the outcome word unconverted and the run's own sentence.
Studio READS that citation rather than minting one — the executor is the only thing present for
every run of a routine, and a reference invented by a surface would differ by who pressed Play —
which is what makes `Report` reachable for a routine at last.

The outcome stays off the citation line on purpose: `runref.Parse` strips a parenthesised word
only when it is one of the supervised seven, so a routine's word there would be read as part of
the record's path. `[result] <word>` already carries it, from the package that owns those words.

**Input is counted as actions asked for, not as sends delivered.** The count is taken in
`oshost.Invoke` against a closed classification — a test reads the switch and fails for an action
nobody classified — because `delivered` wraps four of eleven sending actions, so a count taken
there read zero for a routine that typed or dragged. One smoothed Move is one action; a press
refused by its own guard still asked, and `Unperformed` is the half that names it. Confirmed
delivery is still written nowhere: `SendInput` succeeding says the queue took the press.

**And each kind is read back in its own vocabulary.** Writing the record was half of it. Studio's
history ran every record through the supervised seven words, so a routine's `performed` fell to
their default and a run was described — and given a remedy — in the language of evidence it never
gathered. The record's `kind` is now asked, and nowhere else is; `nextOfRoutine` offers a
routine's own remedies and gives `performed` nothing, because there is nothing to do about a run
where every press happened. The word itself is `runs.KindRoutine`, imported by both binaries,
because `marco` writes it and Studio reads it with no compiler between them.

A row no longer reads **0 checked** for a routine either. That is not a count but the claim that
it checked and found nothing, beside an Act that really did check two things — three zeroes in the
file was what the record was careful to avoid, and one zero on the screen says the same thing to
the person.

**Reporting a problem works from a routine's reference.** The report carries the routine's SOURCE
as it stands — the executable truth, found by the identity in the record rather than by what the
window has selected — and says when that source is **no longer the source that ran**, by comparing
the record's digest to the file's. Re-recording is exactly what somebody does between a failure
and reporting it, so that is the likely case; without the warning a reader compares the run to a
program that never ran.

**A review of the above found three places where an absent value was read as a stated one**, and
all three of them turned a blank into a false claim rather than into a blank.

*The report attached whichever transcript the window was holding.* The transcript is one window's
memory of one child, replaced every time anything runs, and it was copied in without asking which
run it belonged to — so playing B and then reporting A produced a file with A's reference, A's
record and **B's output** under *WHAT IT SAID AS IT RAN*. The lines now have an owner and it is
asked; when it does not match, the section says so rather than being left out. The Act section had
the identical shape, found by whichever Act was open, and is now found by the record too.

*A routine's row said `input disabled`.* `live` is a field only the supervised record carries, and
a routine's states no liveness because `marco do` has no mode to distinguish — read as a plain
bool it was `false`, and the page said a run that had just typed a password never touched the
machine. The count had the mirror defect: `input_actions` is absent in the provisional record a
killed `marco.exe` leaves, and **that attempt may already have sent input**, so rendering it as `0`
put the three-zeroes claim on screen about the run a person is most likely asking about. Both are
pointers now, absent all the way through to a page that renders absence as absence; a routine's row
says which kind of run it was where the badge would be, and an Act that really had input disabled
still says so.

*And an interrupted record identified the hour and not the program.* `build`, `source_digest` and
`contract` were written only after the performance returned — the half of a run that may never
happen — so a killed child left a reference naming a routine and a time and nothing about what was
running. The saved source is now read **once**, into a snapshot taken before the reference is
claimed; the provisional record carries all three; and the same snapshot is what executes, so no
second reading of the file can make the record cite source the run never performed.

## What a recording captures is stated, not inherited

`$MARCO_CV` and `$MARCO_ANCHORS` are read once, into package variables, when a process starts.
That is the right shape for a command — you set the switch, run `marco learn`, and the process
ends — and the wrong one for a window that stays open all afternoon holding the environment of
whichever shell launched it.

The cost was not cosmetic. With anchors inherited on, a click whose cropped region happened to be
*distinctive* carried a template; a press found by matching a picture cannot be stopped at, so
`codegen.Unpropagatable` refused the **whole** guarded recording at Save. So the same
demonstration saved or was refused depending on a variable nobody in the window could see and on
the pixels that happened to be under the pointer. There was a quieter half as well: `learnOptions`
caps every recorded gap to 50ms when CV is on, so a Studio launched with `$MARCO_CV=on` against a
flat menu captured no anchor to wait on *and* took the recorded pauses out — a routine that presses
a menu before it has drawn, and that looks correct in review.

So a recording's mode is now stated by the thing that starts it, before the hooks go on, and the
statement beats the environment. `Inherited` is the zero value and is every caller there was — the
overlay, `marco learn`, `director demonstrate-visual`, the CLI — because turning the switch on and
demonstrating something is what the switch is *for* there. Studio states `coordinates`, once, to
the thing that acts on it; the sentence beside Record is a copy of that rather than the constant a
second time. The word travels on the draft, into the saved source as a comment line, and from
there into the run record off the same single read as the digest and the contract.

Absence survives all of it. A caller that states nothing writes no line; a file with no line reads
back as *unstated*; a word this build does not recognise reads back as *unstated* rather than being
echoed into a record as something verified; and the page renders a blank as a blank rather than as
`undefined` or as the likeliest answer. Every routine already in a library says nothing, and what
those captured depended on the environment of a process that has since exited.

And it survives a REPAIR, which it did not. `Resave` rebuilt a saved routine from the demonstration
beside it while stating nothing, so a coordinate routine came back out stating nothing — its card
lost the sentence and every run record of it afterwards read *unstated*. The quieter half was the
same one as above, arriving a second time: the stated mode is what lifts the 50ms wait cap, so a
window that had inherited `$MARCO_CV=on` rebuilt the routine with its pauses capped and no anchor
to wait on in their place. The mode now travels with the recording rather than with whichever
`Deps` is holding it — read from the source of the file being replaced, and used for the
simplification as well as for the line. Nothing is versioned and nothing is migrated: a repair
writes what the file already said. **The repair, and not every path that regenerates a routine:**
`SimplifyRoute` — `marco simplify` — rebuilds from the same stored recording and still drops the
line and caps the gaps, and is recorded as an open sibling rather than fixed alongside, because
writing a mode line in a path that never wrote one is a decision of its own.
[[ADR-254-what-a-recording-captures-is-stated]]

## Readiness is what has been checked, and it says what it has not

Five ways of being unable to record or to play, each of which used to be discovered by hitting it:
no input recorder on this platform, learned at **Stop** after a whole demonstration; no `marco.exe`
to play with, learned at **Play** over a game somebody had just arranged; a library that cannot be
written, learned at **Save**, which is the one instant a recording exists nowhere else; a window
that cannot name its own build, learned by comparing file timestamps; and the silent one — a
`marco.exe` **on PATH but not beside the window**, which is never learned at all, because
`marcoBin` falls back to the bare name, the spawn resolves it, and playback runs a binary of
another age perfectly successfully.

So Studio answers each by *acting*: the recorder is a platform fact, each executable is an
`os.Stat` beside this binary and then the `exec.LookPath` the spawn itself performs, the library is
a file created and removed in it, and the build is this binary's own stamp or the honest word
`unknown`. Each problem carries what is wrong, what it **stops**, and what to **do** — every one of
them has exactly one answer, and withholding it makes a diagnostic into a riddle.

And the panel says what it did **not** establish, unconditionally, including when nothing is wrong:
that the hooks will install, that the application is running, that a press will land, that anything
looked at a screen. Installing the hooks takes the keyboard and the mouse of the whole desktop, so
nothing there can have tried it. A panel that dropped the caveat on a clean machine would be read
as a promise at exactly the moment somebody believed one.
[[ADR-255-readiness-is-what-has-been-checked]]

## A recording is bounded to one chosen window, and the playback check is a different bound

Two different things wear confinement words in this codebase and they are not the same bound. The
distinction is written down here because each is easy to read as satisfying the other, and neither
does.

This heading used to read *a recording is not bounded to a window*. It stopped being true on
2026-10-28 ([[ADR-268-record-only-the-chosen-window]]) and the body below was rewritten then; the
heading was not, so the one line a reader skimming the contents took away was the retired claim.
Studio's record card made the same mistake in front of people for three days — see that ADR's
amendment of 2026-10-31.

**What IS implemented — the playback check.** A guarded press states the window it was
demonstrated in and everything needed to resolve an offset into it; at the moment of pressing,
`internal/oshost` reads that window off the live desktop and refuses if it is a different size, is
gone, or belongs to another program. See
[[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

That is a **check on one action, about one window's geometry, at the instant before acting**. It is
not a bound on where input may land. A press whose check passes is sent at a screen coordinate, and
a keystroke goes wherever the keyboard happens to be — nothing clips either to a rectangle. What
the check buys is that a press aimed at a window that has moved is re-based onto it and a press
aimed at a window that has CHANGED is refused, which is a different promise from "this routine can
only affect that window".

**A recording is of ONE WINDOW, and that is the default.** It was the whole desktop: the hooks
are low-level and global, every press anywhere entered the demonstration, and the only thing
removed was Marco's own window by process (`orchestrator.withoutOwner`). So a message answered
halfway through was in the routine — its letters as keystrokes, the coordinates of the window
it was typed in, and a picture of whatever was clicked — and the half-minute spent looking
something up became a half-minute wait the routine performed on every run.

Now `Deps.Watch` states the scope before the hooks go on, its zero value is `OneWindow`, and
Studio, `Learn` and `LearnAuto` all take that default. `WholeDesktop` is retained explicitly for
a demonstration that genuinely spans applications.

**The gate is at the instant of the event.** Not a filter at `Finish`: a decision taken afterwards
leaves the input recorded — in the buffer, in `Draft.Events`, published to whoever was
listening, and on disk beside the routine. The live channel is gated with the buffer, because a
consumer holds whatever is published; the one thing an off-target event may say is the stop
gesture, because `StopKey.Triggered` reads nothing but an `EvKey`'s name and ignores every key
outside the chord. The capture-state markers ARE published, so a surface can show what the
capture is doing rather than showing off-target input.

**Membership is the foreground, identity is the handle AND the owner, out of one reading.** The
foreground rather than the rectangle, so a drag out of the window and back is one gesture. The
handle because two windows of one application satisfy every comparison of process id, geometry
and program name; the owner beside it because Windows hands a handle back out, so the number
alone is not an identity. BOTH, from ONE snapshot per event: a click used to ask for its window
while the gate asked the foreground again, and a key was tagged with the app poller's sample and
admitted on a third reading — two readings are two instants, and the foreground changes between
them precisely when somebody clicks on another window.

**And the window is CHOSEN before capture.** The first version latched it — the first foreground
that was neither the starting surface nor its process — which is whatever came forward first, so
a terminal or a notification toast became the thing being recorded. `Begin` refuses a confined
recording that was not told which window; `Deps.Pickable` is the list every surface asks out of,
and an ambiguous name is refused rather than resolved by position. `marco learn --window` with a
prompt, a control in Studio, and the overlay passing the window the person was in — which it can
answer because the HUD reads keys through its own hook and never takes the foreground.

**What was already held when the recording began is read at the start**, every key and button,
and seeded as held-from-outside: Ctrl down when Record is pressed is a press the recording never
sees, and a ctrl+click was otherwise written down as an ordinary click.

**A gesture that crosses the boundary is unrepresentable, not trimmed** — every key and button,
and in both directions: a modifier held across the edge, and one already down on the way IN, which
would otherwise record `ctrl+click` as `click`. The activation click is the one exception and it
is bounded: the forgotten press is forgotten only when its release arrives, so an action performed
while it is still held refuses the recording anyway.

**And the application recorded is the chosen window's**, not the poller's sample. The gate on
the poller refuses a sample taken while the person was away; the other half of the same race is a
sample taken while they were away and admitted once they came back, which would put an `Activate`
for somebody's browser at the front of the routine and file it under that application. `chosenApp`
has no fallback for exactly that reason — an established identity or none — so a confined
recording holds one application switch, its own.

**And the boundary moves when the foreground does.** The markers were made only by `observe`,
which runs when the hooks see something, so leaving the window and coming back without touching
anything produced none at all and the routine waited the whole interval on every run. The
app-switch poller reports the transition through the same fenced update.

**And the pause is visible while it is happening**, which is not optional: a confined recording
goes silent the moment somebody leaves, and silence looks exactly like a recording that is still
going. `waitForStop` says the transitions — the terminal prints them and the HUD streams the same
child — and Studio asks `Capture.Paused` on the poll that draws its card.

**The absence comes out of the clock, not out of a marker.** Time inside is preserved exactly;
time away leaves a documented 250ms, which is a timing policy and not a readiness claim. It is
done to the TIMESTAMPS because a marker in a stored recording is a kind an older build would
ignore while keeping the raw gap — changed behaviour dressed as degradation. `Draft.Events`
carries no marker at all, so a stored recording holds only the kinds it always held with gaps that
are already right, and there is nothing in it for any reader to reinterpret.

**Retained means reachable.** `marco learn "<name>" --whole-desktop` asks for the old
behaviour, and either way the scope is printed before the recording starts. Studio has no control
for it yet, which is a gap rather than a decision. Both review surfaces DO say what happened:
`Draft.AwaySentence` is on the terminal's preview, because a draft shorter than the demonstration
somebody gave reads as a recorder that missed something.

**It is NOT on Studio's card, and this note said it was.** `Recording.Away` has been in the
payload since this rule landed and `TestReviewSaysTheWindowWasLeft` holds it there, but the page
has no element bound to it and never draws it — so the one sentence explaining why a routine is
shorter than the demonstration is invisible in the window most recordings are made in. Found while
adding the element beside it for `ExitSentence`; written down here rather than fixed in that slice.

See [[ADR-268-record-only-the-chosen-window]].

**And coming back here to press Stop is how a recording ends, so it is not part of one.** Alt-Tab
holds two keys down while the switcher takes the foreground, so a boundary that refused a recording
the instant somebody left it with something held refused every recording ended by the button the
window offers. The keys are REPORTED now (`EvHeldAcross`) and judged at review
(`orchestrator.exitGesture`): the trailing run of unreleased key presses before the last leaving is
the way out, and it is the only thing this boundary takes out instead of refusing. Three bounds
keep that safe — nothing may follow them, keys only, and somebody must have left rather than the
window having gone away — and a stranded key the demonstration carried on past is still a crossing.
`Draft.ExitSentence` names what was left out, on the terminal and on Studio's card.
See [[ADR-274-the-way-out-of-a-window-is-not-part-of-the-demonstration]].

**The three are still three different things.** ADR-251's window *check*, the harness's *"input
that cannot be confined to the windows this run owns is not input this run may send"*, and this
bound on what a recording may capture. All three are built now, and they remain separate
promises: the check is about a press at play time, the harness is about what a run may send, and
this is about what a demonstration contains.

## A simplified save keeps its own basis

What makes a saved routine editable afterwards is the STEP ACCOUNT beside it, or the absence of
one. No account licenses rendering the demonstration and treating the result as what the file is —
true of an ordinary save by construction, because its source came from that same rendering.

It was false of a SIMPLIFIED save, whose source was folded harder than a rebuild folds, and the
consequence sat in the middle of the journey: the review card offers Simplify directly above Save,
and the panel it drops you on offers *Edit its steps*, which refused with a sentence about the
saved source and the demonstration no longer agreeing. Nobody had done anything wrong.

A draft now carries the rules it was rendered under when those are not today's (`Draft.Basis`,
set by `draftFor` for `RebuildSimplifyTyping` alone), and `Deps.write` writes the account after the
source, the assets and the demonstration — with both digests read back from where the save put
them, never computed from what went in. The sweep of any older account stays and is not weakened.
A plain save still writes nothing, so every routine already on disk is still edited by rebuilding
from its own demonstration.

**A failed account is not a failed save.** The routine runs — execution reads only the `.marco` —
so `orchestrator.ErrStepsNotWritten` is a third outcome that every surface saving a draft tells
apart from a refusal. Reported as a refusal it was actively wrong: both surfaces answer a refused
save by asking what is already there and offering to replace it, and what is already there is the
routine the save just wrote. See [[ADR-279-a-simplified-save-keeps-its-own-basis]].

## A recording that could not be stopped, and the two workers that outlive it

`Stop` is BOUNDED — a shutdown that cannot complete says so and returns, because a run that
finished its program must report that it finished. The consequence of returning is that the
app-switch poller and the capture worker are still running: the poller inside a foreground-window
read that has not come back, the capture inside screen work and image encoding. Both publish
through package state, because the hooks are C callbacks and can reach nothing else.

So each takes the **session** its recording was given and compares it where it publishes, under
the one lock a later recording takes to raise the session and empty the buffer. Deciding and
writing have to be the same step: asked first and written afterwards, a worker can pass the check,
be descheduled, and append an app switch nobody performed to somebody else's demonstration, or
write a picture onto whatever press is at its index now. The foreground the hooks tag presses with
is published inside the same fence, or an abandoned poller decides which application a later
recording was in.

**And a recording whose HOOKS could not be taken off ends recording for the process.** That is not
a fence but a refusal, because a session number cannot reach a C callback: the hooks gate on
`recActive`, the next `Start` would set it true again, and an abandoned recording's still-installed
hooks would feed the new one. It says a restart is what fixes it, which is what the shutdown's own
message already said.

[[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]]

## A routine offers to be recorded again, and that offer replaces nothing

Re-recording is the ordinary activity of getting a routine right, and it used to mean reading the
name off the card, typing it back into the box exactly, remembering which application the first one
was saved for, and picking the same scope. Get the name wrong by a space and you have two routines
instead of a new version of one.

So the routine offers it. The route's whole **identity** goes to the backend, not its name —
`settings` can be saved for two applications under two scopes, and a window holding the word would
start a recording of whichever it found first and then offer to replace *that* — and the backend
looks the identity up again at the moment the button is pressed, because a card can be on screen
for a routine somebody has just deleted. It reads the library **once** for that and keeps what it
matched: an index established in one listing and applied to a second reading of the same directory
is a namesake, or a panic, the moment anything is added or removed between the two.

**It carries no authority to replace anything.** There is no new save path: Save goes through the
same check every other recording does, which asks what is at the target, refuses a collision, and
offers *Save as* or *Replace that routine by name*. A button that re-recorded and then replaced on
an ordinary Save would have reintroduced exactly the loss the collision check was built to prevent,
through a door marked convenience, and the person pressing it would have had more reason than
anybody to expect it.

What the label does buy is three notices: which routine this is a second recording of, said during
the recording as well as at review because the expectation forms before Save is in front of
anybody; the routine's own scope preselected, so a replacement is not silently rescoped; and a
warning when the demonstration was made in a **different application**. That last one is the single
case where Save succeeds and does the wrong thing — which routine a save writes is decided by the
application that was observed, so a re-recording made elsewhere is a different route, with no
collision to refuse and a second routine quietly kept beside the one somebody set out to replace.
[[ADR-256-recording-a-routine-again]]

## Validated by

- `internal/director/demo` unit tests (`demo.go`, `record.go`, `validate.go`)
- `director demo` commands; explainability output
- `internal/director/localizetrial` — the local-regioning experiment: its fixture, its frozen
  protocol and its evaluation. Opens no window and sends no input.
- `internal/director/controlevidence`, `internal/director/match` and `cmd/director` — what a
  pressed control is worth, measured on drawn pictures for the rules and on real 1920x1080 game
  frames for the number.
- `internal/simplify` `modifierhold_test.go` — a modifier held across a click, around a key, over
  nothing, and across both; an extra modifier inside a hold, with the held one staying out of the
  chord; the three refusals and a control beside each. Every case goes through the production
  `Simplify`, because the defect lived in the seam between classification and lowering and either
  half alone looks fixed. `TestTheHoldMarkersAreReadBeforeTheModifierState` holds the ordering off
  the source, which is the only way it can be held: the two mechanisms that would each keep a held
  modifier out of a chord are indistinguishable from outside.
- `internal/oshost` `modifierclick_test.go` — the same gesture from recording events through
  `simplify`, `codegen` and the runtime to the production `Host`: a Ctrl the machine will not
  press stops the click, one it will not release is named in the account, a cancelled run still
  attempts the release, and a legacy routine keeps the contract it was saved under.
- `internal/oshost` `TestAShortcutInsideAHeldModifierKeepsBoth` — the mixed case as an ORDERING,
  end to end, with the production `sendChord` running underneath the keystroke so the chord's own
  presses land in the same log as the hold around them: `hold ctrl`, Shift down, A down, A up,
  Shift up, the click, `release ctrl`. Ctrl appears exactly twice, which is what a chord naming it
  would break while leaving every moment the ordering asserts still present.
- `internal/orchestrator` `refusedgesture_test.go` — the draft carries the reason and `Save`
  refuses it, with nothing written to the library; and the same demonstration done properly
  saves, as the gesture.
- `plugins/studio` `refusedgesture_test.go` — the window says WHY, beside the steps it
  understood, and still offers an ordinary recording for review.

## A modifier held across a CLICK is a hold, and used to be dropped

Ctrl+click and click are different acts in most applications — one adds to a selection, one
replaces it. Demonstrate Ctrl, then a click, then release Ctrl, and `simplify` used to emit **a
plain click**: the modifier was tracked for chord-folding, found no key to fold into, and was
discarded. The routine performed a different gesture from the one that was shown and read as a
faithful recording of it, which is the one failure this subsystem is least able to make visible
afterwards.

It is now a hold, written with the `KeyDown`/`KeyUp` steps that already existed. No new word, no
new field on a click step, no language change.

**The rule.** A modifier is a hold when it spans a POINTER action, and only then. The modifiers
around a KEY still fold into a chord, because that is what `ctrl+shift+a` is and because a chord
presses and releases them itself — a chord whose modifiers were *also* written as holds would
press Ctrl twice and release it in the middle of its own expansion. And the duration rule that
makes an ordinary key a hold after half a second deliberately does not apply: a modifier held over
nothing modified nothing, and a person resting a finger on Shift while they think should not
acquire a stray hold.

**It took two changes, and either alone does nothing.** `markHolds` decides which presses are
holds; `lower` decides what is written down. The modifier branch in `lower` ran *before* the hold
markers were read and `continue`d past them — so teaching the classifier about modifiers on its
own would have left the marks right and still swallowed. Both are exercised through the real
`Simplify`, which is the only way to tell a fix from half of one.

**And a key under a hold carries only the modifiers the hold is not already supplying.** `ctrl`
down, `a`, click, `ctrl` up is one hold with a keystroke inside it: `KeyDown ctrl`, `Key "a"`, the
click, `KeyUp ctrl` — not `ctrl+a`, which would press the modifier again and release it before the
click. Add a Shift that spans only the `a` and the key becomes `Key "shift+a"`: the extra modifier
has nowhere else to go, and the held one must stay out, because `sendChord` presses and releases
exactly what it is given.

The first version of that branch emitted a bare key and dropped the extra modifier, so
`Ctrl↓ Shift↓ A↓ A↑ Shift↑ click Ctrl↑` performed Ctrl+A and then Ctrl+click where Ctrl+Shift+A
and then Ctrl+click were demonstrated. Every step looked right.

What keeps the held modifier out of that chord is not a filter — it is that the hold markers are
read BEFORE the branch that records modifier state, so a modifier written as a hold never becomes
chord state at all. There was a filter as well, briefly; it was dead code, and no input could tell
the two mechanisms apart. The ordering is pinned by a test that reads it off the source, because a
test over behaviour cannot: getting it wrong the other way produces the same output as getting it
right.

And the key carries no `Text` either way, which is the subtler half — `Text` is what lets a run of
keys coalesce into a `Type` step, and `Type` sends characters rather than keystrokes, so `a` under
a held Ctrl would arrive as the letter after a fold two passes later that nothing at that point can
see. Shift alone is the one place this is visibly a choice: outside a hold, Shift+A is how a
capital is typed and comes out as a key carrying the text `A`; inside one it is part of a shortcut,
and a capital letter is not what was asked for.

## What a recording refuses to say rather than guess at

A modifier is never silently discarded. Where the gesture cannot be written down faithfully the
recording is REFUSED, with a sentence naming the key and what to do instead — because the
alternative is the original defect, and the person who performed the gesture is the only one who
can perform it again differently.

Three ways, all of them about not knowing WHEN it was held rather than whether:

| | |
|---|---|
| never released | the recording ends with it down, so a hold written from it would have no release and would leave the key down on a real desktop |
| never pressed | it was already held when recording began, so there is no instant to press it at and no way to know what the press under it actually was |
| pressed or released mid-press | it was held for *part* of a click, and "part" is not a thing a routine can say |

Each has a control beside it where the same shape is harmless: an unbalanced modifier that nothing
happened under modified nothing, and refusing that would refuse a recording for no reason.

The third is also why the choice of which click edge decides "spans a pointer action" cannot change
an outcome — every recording where the two edges disagree is refused before the classification is
reached. That is a surviving mutation in the gate, and it survives because the two variants are
equivalent given the refusals, not because nothing tests it.

**The refusal is answered in four places and enforced in one.** `Simplify` returns it, so every
caller has to answer for it — a compiler question rather than something to remember. `Learn` and
`LearnAuto` print it and save nothing; `SimplifyRoute` prints it and leaves the routine already on
disk alone, which is the right answer there and a different one. A draft carries it, so Studio's
review shows the sentence beside the steps that were understood. And `Save` refuses it, which is
the only way a draft becomes a routine and therefore the only place the enforcement has to be.

**Compatibility.** New recordings use the corrected interpretation. Existing saved Marco is
unchanged — nothing on disk is rewritten and there is no migration. Re-recording or an explicit
`marco simplify` picks the correction up, which is a person asking for it.

## A routine can take a value, and Studio now says so

A routine can be told to ASK for a piece of text instead of typing the text that was
demonstrated. There are three ways in, and **Studio wired all three and mentioned none** — which
is why the owner went looking for the feature and reported it missing. A capability nothing
mentions is indistinguishable from one that does not exist.

| | how | where it is wired |
| --- | --- | --- |
| the name declares it | call the recording `cloud pull stone with item` | `Deps.Begin` → `routes.SplitArgs` |
| the reserved key | press **F9** during the demonstration, where the value goes | `Deps.ArgKey`; `simplify.go`’s `argKey` branch |
| the step edit | turn a typed step into a supplied one afterwards | [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]] |

`marco learn` printed the key in its prompt (`argKeyHint`). The window with the Record button did
not.

**The key is named on the record card and in the band**, from `orchestrator.ArgKeyFor` so that one
reading of `$MARCO_ARG_KEY` serves the recorder and the window alike. An unset override is F9, not
silence — the same defect the stop key shipped with three rows up the same card. Before a
recording the card says what the key is for and that the name’s `with` clause names the value;
while one runs the band names the declared values **in order**, because the Nth press fills the Nth
name.

**And a value can be marked from the review**, before anything is saved
(`Deps.SuppliedInDraft`). The operation existed only on a saved routine, so somebody who realised
at review had to save, find it in the panel, edit it, and save again. `suppliedName` is the whole
of what may be asked and `EditArgument` asks it too, so the credential refusal lives in one place;
`declaredFor` declares the name, without which the saved routine would read the value from the
credential store instead of typing it.

**A value is one step, and a recording made at a human pace is one step per character.**
`coalesceTyping` folds only what is adjacent, and a 400ms gap between keystrokes is a `wait`
between them — so a slowly typed `stone` is five `type` steps and there is nothing to pick until
**Simplify** folds the run. The card says that in those words and names the button, rather than
showing an empty chooser. And Simplify takes any marks off, having said so first and named them:
it re-derives from the events, and a position cannot honestly be carried across a list that
folded. A run typed at *typing speed* is the exception — `simplify.MinWaitMs` is 30, so gaps under
that emit no wait and the keys fold in the ordinary draft — which is how a value comes to be
marked on a recording nobody simplified.

**And a marked save writes its step list down.** A mark rewrites a step's text, so the list stops
being a rendering of the demonstration under any rules, and `accountFor` originally asked only
`Draft.Basis` — which a *rebuild* sets and an ordinary draft does not. The routine saved and ran
correctly and then could not be touched: with no account beside it, `basisFor` renders the
demonstration, compares, and refuses the edit panel, while a rebuild under today's rules
re-derives the mark away. `Draft.Changed` records that the steps were changed after they were
rendered and `Draft.ownSteps` is the one question it and `Basis` both answer. `capture.wasSupplied`
travels with `capture.wasDraft` for the same reason: the draft kept as the way back can be one
that was marked before Simplify was pressed.

**Not done, deliberately:** a button in the band that drops a placeholder mid-demonstration. It
can only be pressed by leaving the window being demonstrated in, and the mark would need a stored
recorder event kind of its own, because `Draft.Events` carries no markers at all and a rebuild
re-derives from what is stored. Making it an ordinary `EvKey` would be simulating a human press
into a recording. See [[ADR-281-a-value-can-be-asked-for-from-the-review]].

## A run of steps can be said to be typed text

Typing is written down as HELD KEYS whenever the next key goes down before the last one comes up,
which is most typing. Measured on the owner's own `cloud pull stone` recording — presses of
93–149ms overlapping the next by 12–56ms — the word saved as

```
KeyDown "s" · KeyDown "t" · KeyUp "s" · KeyDown "o" · KeyUp "t" · KeyDown "n" · KeyUp "o" · Key "e" · KeyUp "n"
```

Both typing-fold passes need a step carrying TEXT and a hold carries none, so **Simplify did
nothing** and no value could be asked for either, a value the routine asks for having to be one
step.

**The classifier is not changed.** `s↓ t↓ s↑ t↑` typed and `q↓ e↓ q↑ e↑` held in a game render
identically — `hold Q, press E, release Q`, the canonical game gesture — and no key name or
duration tells them apart. A containment rule was built, measured and reverted for exactly that
reason.

**So a person says.** `orchestrator.TypedRunsIn` proposes the runs, `typedRun` is the one validator
both ways in ask, and `Deps.TypedInDraft` applies one to the recording under review — an EDIT to
the list, with the steps visible before and after and nothing written until Save. The chooser
offers the gameplay gesture as readily as the typing, because the window cannot tell and a chooser
that hid one would have guessed. A long press converts when it is declared and the control says
beforehand what that costs: the keys in the run stop being held, the waits inside it go, the waits
outside it are untouched. A repeat is kept exactly.

Refused: a key that types nothing, an unbalanced hold, anything that is not typing, text the
routine already asks for, and a key held ACROSS the whole range — whose hold the conversion would
shorten by the waits it removes.

**And positions alone are not consent.** `Draft.Shown` is a term for the list the card drew, and
both draft operations refuse one counted against a list that has moved. One window holding a draft
means no second process, not no second writer: Simplify, going back to the original and each edit
all move the positions.

A declared run sets `Draft.Changed`, so the save writes the step account that keeps it through the
edit panel and a rebuild.

**And the saved routine's panel offers it too** (`orchestrator.EditTyped`), because the recording
that exposed this is already on disk and a repair reachable only from the review would mean
recording it again. It is reviewed ON ITS OWN: it is the one operation that changes how many steps
there are, so every position the other changes were read off moves under it, and nothing says
whether a position was read before or after the run. See
[[ADR-282-a-run-of-steps-can-be-said-to-be-typed-text]].

## What the window offers, and what it explains

**The alpha rail is one journey.** Acts and Scenes are behind `$MARCO_STUDIO_ACTS` and
`$MARCO_STUDIO_SCENES`, read once when the window's backend is made and off by default. Nothing
underneath them is removed — the views, the bindings and the capabilities are untouched, and
turning both flags on gives back exactly the rail that shipped. What is withheld is the way in,
because an alpha that offers a surface it cannot carry makes somebody pay for that at the far end
of it, after a demonstration.

The rail BUTTONS and the view SECTIONS stay in the document whatever is offered; only the list
narrows (`Features.Views`). `paintViews` used one hardcoded array as both the rail and its own
element lookup, so deleting an entry threw mid-paint and stopped the window rendering altogether.
`show` refuses a view that is not offered — a hidden button is not a closed door — and the two
reads a withheld view would want are guarded in `loadActs` and `loadDemos` themselves. The
recorder's state keeps being read, slowly, because it also paints the rail's **build stamp**.

**And Help is one section, composed by the backend.** `Acts.Help` reuses `everywhereSentence`,
`stopLabel` and `argKeyLabel`, because two of its sentences name a CONFIGURED key and a page that
wrote F12 and F9 into itself would be wrong in every installation that moved them — which is the
defect that already shipped on the record card. Five sections, each an ability the audit found
complete and invisible, each with a stable key so a small link beside Record, beside the argument
sentence and beside *Edit its steps* opens that section rather than the top of a page of
everything. The recording section says what the recorder does: Marco records ONE window and THE
WINDOW IS CHOSEN. See [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]] and
[[Studio-alpha-UI-slice-2026-10-31]].

**And the step editor is a list of rows** — number, action, value, controls. The split uses the
page's own `stepVerbs` table, the one `TestEveryStepSentenceTheBackendWritesHasAVerb` already held
against `oneStep` for every kind; the verb column is FIXED, because each row is its own grid and
an `auto` column would start the values in a different place on every line. A removed row strikes
out its value and NOT the box containing the warning that a second step goes with it: `.pair` was
a child of the struck element, and `text-decoration` propagates through the box tree and cannot be
refused by a descendant — so the sentence somebody has to read before pressing was struck through
in the one state where it appears. The paragraph above the list is in Help now; the refusals, the
pair warning, what has been asked and what a typed-run declaration costs all stay on the panel.
Every control on a row names its step. See [[ADR-284-the-step-editor-is-a-list-of-rows]].

**And a routine can be called something else**, as a reviewed move. `PrepareRescope`'s body is
`prepareMove` now and `PrepareRename` is its second entrance: a move changes an IDENTITY, and an
identity has two halves — where the routine works and what it is called. Everything the move
engine guarantees is the same one, including that the destination is read back before the
original is removed and that a destination written with the original still there is reported as
two routines.

Three things are the rename's own. The box edits the NAME: the `with …` clause is shown beside it
as a chip, read out of the SOURCE because a saved header does not carry one — the declaration
lives in the steps, as `{{item}}` in the text of a step that types. A COLLISION IS REFUSED with
nothing to agree to, so `RenameRoutine` takes the token alone and the page draws no consent row.
And a routine whose source reads a credential qualified by its own name — `<slug>:password`, which
is what `codegen` writes for a declared secret — is REFUSED, because a rename re-renders and the
qualifier would move with the name, sending the routine to the credential store under a name
nobody stored anything under. Nothing is read from the store and nothing is migrated into it.

The rename tests found a defect in the move engine that is **not** about renaming: `codegen` is
told which argument names are declared, and an undeclared `{{name}}` is not a placeholder to it —
`splitSecrets` turns it into `do OS's Secret with "name"`. The declared set came from the routine's
NAME alone, which carries no clause, so every move of a routine that asked for a value produced
one that read the credential store for it instead. Fixed in the shared path with `suppliedIn`.
STILL OPEN, and ruled on by nobody yet: the secret QUALIFIER is lost on every re-render of a saved
routine, so a rescope or a step edit turns `<slug>:password` into `password`. See
[[ADR-285-a-routine-can-be-called-something-else]].

**And a press can be moved.** A click recorded inside a window is written down twice — a point on
the screen and an offset inside that window — and the two were measured AT THE SAME INSTANT, so
the window's origin is recoverable as `(X-RelX, Y-RelY)` and the screen point moves with the
offset out of that same reading. That is the whole reason either number can be edited: without
it, changing one makes the other a lie. The OFFSET is the pair offered for a measured press,
because it is the half that still means the same place after the window has moved; a press never
measured against a window is moved by its screen point and does not acquire a window by being
moved. `orchestrator.PointIn` decides which, for every step, so no surface works it out for
itself.

Refused, each on its own row: a press found by its picture, its colour, the words beside it or
what Marco can see — the coordinate there is a position INSIDE the thing being looked for; a
drag, which has two ends; an incomplete measurement, refused rather than completed, because
filling a term in would manufacture a guarded point out of a reading nobody took; and an offset
the window could not contain. `movedPoint` COPIES the step and sets at most four numbers, so the
button, the modifier, the count and all five guard terms survive. See
[[ADR-286-a-press-can-be-moved]].

## Known gaps

See the *Known gaps* section of [[director-demonstrations]].

## Milestone record

[[director-demonstrations]].
