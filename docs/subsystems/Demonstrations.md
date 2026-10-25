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
  — a non-left press, a drag — is refused at generation and said at review. **A KEYSTROKE IS A
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

## Known gaps

See the *Known gaps* section of [[director-demonstrations]].

## Milestone record

[[director-demonstrations]].
