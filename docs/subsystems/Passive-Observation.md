---
type: subsystem
status: active
owners:
  - director
depends_on:
  - perception
  - vision
  - windows
used_by:
  - vision
  - game-packs
updated: 2026-10-09
source_paths:
  - internal/director/observe
  - internal/director/observe/interface.go
  - internal/director/observesession
  - internal/director/learn
  - cmd/director/observecmd.go
  - cmd/director/learncmd.go
  - cmd/director/learnsessionwiring.go
  - cmd/marco/watchui.go
  - cmd/director/observelook.go
  - cmd/director/observeledger.go
  - cmd/director/observeambient.go
  - cmd/director/observewiring.go
  - cmd/director/experiment.go
---

# Passive Observation

Watching a running application for minutes at a time, sampling what is on screen so temporal
analysis has something to work with. It never acts.

## The guarantee is structural

The promise is not that this code does not click. It is that it **could not**, because
nothing it can reach knows how. A boundary test walks the package's transitive dependencies
and fails if anything capable of affecting the machine is reachable at all. See
[[ADR-010-passive-observation-cannot-execute]].

## Privacy: the classifier is two-stage, and the first stage is structural

The original rule asked what the **text** looked like — word length, letter ratio, a marker
blacklist. That cannot work, and a live Chrome session proved it: `"Chris Haynes Plus"` is
three ordinary capitalised words, indistinguishable by shape from `"Exit To Menu"`.

What separates them is what they are **attached to**. A button's name is a fact about the
interface; text inside an icon's box is a fact about the person using it.

1. **Structural eligibility** — only `button`, `menu_item`, `menu`, `tab`, `checkbox`,
   `radio` may hold plaintext. Everything else defaults to private, and the allowlist is
   closed, so a newly added role is refused until somebody decides otherwise.
2. **Shape**, as defence in depth — even a button's name is refused if it looks like a token.

Everything else keeps role + length + confidence + digest, which is all temporal analysis
ever needed. The digest is what makes "this changed" observable without "this said" being
retained.

**What it costs, stated plainly.** The detector in use emits one class that maps to
`RoleIcon`, so a game's menu labels are withheld too — the same `RESUME GAME` that reads
perfectly is stored as a digest, because nothing structural vouches for it being a button.
That is the honest trade: a classifier loose enough to keep those would also keep a friends
list. The remedy is a detector with a real class vocabulary — see [[Vision]].

**The allowlist is now one policy and it also decides what is READ.**
`directorapi.ElementRole.NameablePlaintext` since 2026-08-09
([[ADR-017-structure-earns-a-name-text-never-earns-structure]]). Four copies existed — this
classifier, the shadow diagnostic, the vision benchmark, and the scoped label reader — and the
reader's copy would have been the one that decided which regions an OCR engine is pointed at.
Nameability now gates the reading as well as the keeping, so an unsayable region costs no round
trip and its text never enters the process at all.

**Two surfaces, kept apart.** A *session-local safe label* may hold readable text for a nameable
role, because a structural role vouches for it. A *durable semantic term* is never text — it is
a member of the closed vocabulary in `observe/terms.go`, and the durable representation has
nowhere to put a string. Text read from a `text` REGION takes the stricter deal unconditionally:
it may become terms and may never be released in the clear, because nothing structural vouches
for what a text region says.

## Metrics

`internal/director/observe/metrics.go` scores semantic usefulness rather than detection
volume: stable-entity yield, anonymous ratio, structural-role coverage, safe-label
opportunity, flicker rate, transition utility, unstable-structure ratio. Named thresholds,
every failure reported rather than the first, deterministic output.

`director benchmark-vision <session>` scores a stored session with no game running.

## Presence is state-relative, and global absence is not detector failure

The correction that came out of [[Experiment-007-state-relative-tracking]], and the one thing to
understand before reading any track table.

A session is segmented into generic, session-local **screen states** (`state_1`, `state_2`, …)
from raw detection composition — role counts plus coarse normalised arrangement, computed before
any track is touched so state identity is never downstream of tracking. Every track then carries
**two** presence figures:

- **global** — `Seen / Eligible` over every valid inference in the session
- **state-local** — `Seen / Eligible` over the inferences in which its own state was active

Both are true and they answer different questions. A pause-menu button in a session that is half
gameplay is genuinely `bursty` globally and genuinely `persistent` within its own screen. The
live Rocket League rows read `8/16, 50%, bursty` and `8/8, 100%, persistent in state_3`
simultaneously.

**A UI structure may be conditionally persistent.** An element is judged against the
opportunities where its state was active, not against every frame — a pause-menu button being
absent during gameplay is correct evidence, not a miss. Absence counts only when the track's own
state is confidently active, a valid inference ran, and it was not detected. A different state,
an unplaceable transition, a skipped slot, a failed inference and an unproven target are all
unknown, and unknown stays unknown ([[ADR-006-unknown-is-not-false]]).

`ShadowTrack.Stable()` is the global predicate and answers **wrongly** for anything
state-dependent; `StateStable()` is the one a capability should read. States are numbered, never
named — deciding that `state_2` is "the pause menu" is interpretation and belongs to a game pack
([[Game-Packs]]).

Tracks persistent in exactly one state, cut into runs by vertical arrangement, form
**structural groups** — the first thing here that describes an interface element rather than a
box. "Exactly one" is load-bearing: an element equally reliable on two screens is ambient and
belongs to neither structure.

## The state graph has edges

A transition records what the player was observed to do before it — see [[Navigation]] for the
producer and [[ADR-013-navigation-is-meaning-not-keys]] for why it records meanings and not keys.

The wording throughout is **correlated, never caused**, and the report is arranged to keep it
that way: the dominant intent is printed with its support, competing intents survive beside it,
and changes with no navigation before them are counted rather than omitted — a transition that
usually has no input is the evidence against reading the others as caused.

Navigation is banked across slots the detector sat out, because screen evidence and input
evidence fail independently and at the real skip rate the keypress that opens a menu very often
lands in a slot the cadence gate declined.

## Related systems

- [[Vision]] — one of two structural sources; see [[ADR-036-a-screen-is-a-composition-not-a-provider]]
- [[Navigation]] — the input evidence that gives the state graph its edges
- [[Perception]] — the pipeline, and the pinning gap below
- [[Windows]] — the callback exhaustion this workload exposed

## Telling somebody what was learned

`marco observe --follow` prints a line per COMMITTED change to durable knowledge — a Place
established, a way between two screens remembered, a name settled, a destination bound to a word:

	+ learned place       Mouse
	+ learned way         Home -> Bluetooth & devices
	· saw again           Home -> Bluetooth & devices
	+ learned destination "mouse" -> Mouse

Not a log of what Marco perceived. Samples, walks, candidates and confidences are Marco LOOKING,
and a person waiting for the moment they can say "alright, do that then" would never find it under
those.

The events come from `semanticmemory.Store` after its own write succeeded. Nothing upstream
announces and nothing predicts, so a Place refused at a bound, a signature that matched a record
already held, or a file that could not be written all arrive as silence. Names are resolved when
somebody looks rather than when the write happened, because a Place is established on one pass and
named on a later one. See [[ADR-111-a-demonstration-takes-the-slot-from-watching]].

**Watching yields to a demonstration.** One observation session runs at a time, and until 38A
ambient watching simply held it: every `marco learn` came back `no_observation` — *"I couldn't
watch — I lost sight of that window"* — so the product's headline loop could not be walked from a
command line. Ambient now stands aside for a Learn, exactly as Light Mode does, and takes the slot
back afterwards.

## What interface is in front of it

A session folds a belief about the screen it is watching NOW, beside the totals it accumulates
about the session as a whole: which list is being used, what is chosen in it, and what changed
since the last look. [[ADR-147-the-interface-in-front-of-a-session-is-a-shadow-belief]].

It is SHADOW evidence and it is folded with the rest of it, for the reason player input is:
putting it on the authoritative sample would make it something fusion could reach, and "a menu
is open and Settings is selected" is not something Marco should be able to believe its way into
acting on. It is `provisional`, `ambiguous` or `unknown`, it never claims the entries it found
are all of them, and it never claims one can be pressed.

The pixel evidence under it is two numbers per region, measured by the vision provider where
the frame still exists and carried along with the geometry. A session cannot see.

**A look nobody took is not an empty screen.** Live, the shadow cadence lets an inference start
every two seconds while the session samples twice a second, so most looks bring nothing. Those
mark the belief **stale** — unconfirmed for N looks, still resting on sample M — and after ten
of them it is dropped with that as the reason. A detector that ran and found nothing is the
other case, and that means `unknown` at once. A bounded journal records the looks at which the
answer was different, with their times, so a report says what the session emitted rather than
what a client caught while polling.

**What it is allowed to do.** No authority over belief: fusion cannot read any of it. It MAY
inform hypotheses and proposed actions — it already does, since `observe.Hypotheses` is derived
from these totals. It can never authorize an effect and can never prove one happened. The
session report used to print a wider claim than that, and the wider claim was false.

**On a game the detector has to be allowed to run at all.** The purchase policy declines an
inference whenever the primary reading claims to be sufficient, and on a game it always claims
that. Measured live: one inference in 78 looks. A declined slot now says it was declined rather
than calling itself cadence pacing, every sample names the policy it ran behind, and
`$MARCO_SHADOW_UNGATED` collects the evidence the policy decision will need.
[[ADR-148-a-claim-of-sufficiency-is-not-a-claim-about-a-game]].

## What a live game session actually measures

Numbers from `director observe-game --application trackmania`, 240s at a 500ms interval with
the detector ungated and a 1s cadence, through the production sampler. They are here because
every earlier figure on this page came from a recording, and three of the defects that mattered
most were invisible to one.

| | |
| --- | --- |
| slots offered / real inferences | 55 / 55 |
| unproven, failed, never-asked | 0, 0, 0 |
| detections / nameable | 316 / 268 |
| inference latency | median 1972ms, p95 2326ms |
| authoritative samples in 240s | 55 (the inference blocks the cycle) |
| track matching | 47 joined at mean IoU 0.92, 7 separated at mean IoU 0.41 |
| longest-lived candidate | seen in 47 of 55 looks, one episode, persistent |
| screen-state transitions | 40, every one unattributed |
| physical input events received | 0 |

Three things that follow from it, and are not interpretations of a screenshot:

**Candidate identity survives successive observations.** One track was matched in 47 of 55
consecutive looks as a single episode at mean IoU 0.92, while 7 low-overlap observations were
separated rather than merged. That is the question [[ADR-147-the-interface-in-front-of-a-session-is-a-shadow-belief]]
set, answered live.

**The belief abstains, and two independent signals agree about why.** Status stayed `unknown`
with *"no list has been seen handing a mark between its members (2 candidates watched over 55
frames)"*, and the navigation producer independently reported 55 consecutive inferences with no
input. Nobody navigated; the belief said so from the screen side and the input side at once, and
`entries_known_complete` was false throughout.

**The transition signal is dominated by detector instability.** 40 transitions across 55
inferences, with the degenerate single-pane reading re-entered 19 times and most tracks sitting
at confidence 0.16–0.19 against a 0.15 floor. Anything that wrote a transition down because the
screen appeared to change would mostly be recording flicker.

## What the two findings above became

**Both were fixed as general causes, not as applications**, and both are measured rather than
assumed.

A screen change is now a proposal that a second consecutive look has to agree with
([[ADR-151-a-screen-that-changed-for-one-frame-did-not-change]]). The confidence floor was NOT
raised: the menu entries sit in the same 0.16–0.19 band as the fluctuation, so a floor that
removed one would remove the other. What separates them is persistence. A single unreadable
frame is a dropout carried as uncertainty on the pending change rather than a pair of
transitions through `state_unknown`, and the navigation observed on it is kept — the press that
changes a screen lands before the frame that shows it. The gate reports both of its error
directions, and the report prints the unflattering one: refusals that a deliberate press
preceded, named as the gate's own error rate.

The Director now watches a controller
([[ADR-150-silence-is-not-evidence-about-a-device-nobody-watched]]), reusing the read-only
`padsource` adapter the capture work already had — the Windows backend binds `XInputGetState`
and not `XInputSetState`, so there is no setter to call. Every observation carries which
hardware it came from, and the question that matters is answerable: a session may report its
quiet as the PLAYER's only when every device in the vocabulary was watched, available and
present. A controller nobody plugged in says so, and a Director with no pad observer prints
`NOT WATCHED: controller`.

**Still not measured:** a live run with real controller navigation. Latency is the reason it
matters — at ~2.0s a look, a change needs two looks, so Marco can follow a menu somebody moves
through every few seconds and cannot follow one they move through quickly. Whether the window
should be wider, narrower or time-based is a question for `rejected_with_input` on a real run.

### Holding still cannot identify an interface, so the protocol no longer opens with it

`watch-interface.cmd`'s six phases were reordered on 2026-10-10, before the next run rather
than after it, because two findings from [[Experiment-024-watching-a-player-navigate-a-real-menu]]
together made the old order unable to succeed:

- The menu was legible for **slots 1–10 only**, 0–43s. Over the next 150s the detector admitted
  nothing on 56 looks of 72.
- A list is identified by being seen **handing a mark between its members**. `selection`'s rule
  wants, of one candidate, 10 judged frames, 8 with exactly one member marked, ≥2 transfers and
  ≥2 distinct holders. A screen nobody touches produces zero transfers.

The legible window and the HOLD STILL phase were the same stretch of the run, so the one phase
that could have afforded an identification was the one phase structurally incapable of
producing one. `group_1` came out of that run with 24 members, 16 buttons and `episodes: 0` — a
list, watched, never used.

MOVE SLOWLY is now first and takes the largest share; HOLD STILL moved to phase 5, where it
asks something it could not ask first: an interface that **has** been identified, and a mark
nobody is moving — does the belief hold, and does the selection stay where it was put? Nothing
about the detector or the gate changed here. This is a protocol defect, and it is recorded as a
subsystem fact because the arithmetic that makes it one is in `selection/active.go`, not in the
script.

### The pause menu is not a list, and the map-selection screen is

[[Experiment-025-the-menu-was-the-wrong-screen]] ran the reordered protocol and the acceptance
test failed again, for an unrelated reason. Navigation was no longer blind — the menu was
legible in every phase — and no list was ever identified, across 92 directional presses.

As this detector sees it, the Trackmania **pause menu** is about five boxes at unrelated
positions and sizes. Zero vertical stacks of three or more similar rectangles in 67 navigation
looks, and neither a `possible_choice_group` nor a `possible_menu_like_state` formed in any of
those phases. The rule needs a mark handed between the members of a list, and there was no list.

The **map-selection screen** is the opposite, and the run only saw it because the operator
exited the map by mistake in the last phase: 20.8 boxes a look against 4–5, and the only
uniform list in the whole run — 8 rows, gaps 0.032–0.034, identical width, height and left
edge. Director named it a choice group with nine seconds left on the clock, too late for the
exit→maps transition to become a `possible_transition_action`.

So `watch-interface.cmd` takes a third argument: `menu` keeps Experiment-025's protocol
unchanged, because a baseline that gets edited is not a baseline, and `maps` is four phases
pointed at the map list with it **already on screen** — slow, hold, reverse, fast, in that
order.

HOLD cannot come first in either protocol, and the second reason is stronger than the transfer
one. A member's baseline is built only from frames in which that member was **not** marked, so
a row lit from the first look never establishes one and is skipped from the judgement
altogether. **A mark that has never moved is invisible to this tracker by construction.**

### A frame can be of the wrong window with every identity guard satisfied

Capturing by window avoids stealing focus from the thing being observed, and it is not
occlusion-safe: Windows can return the pixels of whatever is covering the target, under the
target's own name. Experiment-025 captured the Windows taskbar on one look of 107 — a dense row
of icons across the bottom, a text field, eleven equal slivers — with `target_losses` at 0, the
window generation unchanged and provenance resolved cleanly. All three of those are about
window **identity**, and this is not an identity failure.

The Director now asks which window is in front, per look, and a look taken behind another
window carries no evidence: it is refused by the belief the same way an unprovable one is, and
the session counts it. Three states and not two — `target`, `other`, and the honest
`unknown` for a desktop nothing could resolve — because treating unknown as the target smuggles
a covering window's pixels in as game evidence, while treating it as another window can discard
an entire run and report the quiet as a finding. Same shape as [[ADR-006-unknown-is-not-false]].

It cannot raise the window, take focus or re-capture. The observation path is passive by
construction, and a resolver that could act would quietly repair the situation it exists to
detect — the reason `trackerResolver` cannot acquire either.

**The cost lands on the operator**, and the script says so: clicking the banner window to read
it refuses those looks. Visible is fine; in front is not.

## Decisions

- [[ADR-010-passive-observation-cannot-execute]]
- [[ADR-147-the-interface-in-front-of-a-session-is-a-shadow-belief]] — the screen it is
  watching now, as a belief it can be asked about
- [[ADR-148-a-claim-of-sufficiency-is-not-a-claim-about-a-game]] — why the detector ran once in
  seventy-eight looks against a real game, and what now says so
- [[ADR-149-an-experiment-asks-its-own-detector]] — and why the switch that was meant to
  fix that removed one of two gates in series: a provider nobody asked, counted as an
  inference that could not prove its window, and a belief that read an empty screen off
  a look that never happened
- [[ADR-150-silence-is-not-evidence-about-a-device-nobody-watched]] — the Director
  watches a controller, every observation says which hardware it came from, and a
  session cannot report quiet as the player's unless it watched every device
- [[ADR-152-slow-discovery-fast-tracking]] — built. ScreenParser discovers the rows;
  a 250ms ticker off the observation cycle re-reads which one holds the mark. Only a
  discovery may identify a list; a fast read may only move the mark inside one, and says
  so with two timestamps
- [[ADR-153-a-blackout-is-not-a-destination]] — `state_unknown` can no longer be
  confirmed as the screen in front of a session. An unreadable run is uncertainty, then an
  expiry of the belief; one bare look no longer destroys an interface belief either.
  **Addendum 2026-10-10**: that expiry is now stamped at the look that expired it rather than
  at the evidence it gave up, and names the last look that brought any — the journal exists to
  say *when* the answer changed, and on the belief's own fields `At` means something else
- [[ADR-155-a-learn-session-has-one-owner]] — a learn step owns the session state start to
  finish; the two things a person can say while a pass is running arrive through a small
  synchronised box and are applied at the step's boundary, or by the caller when no step is
  there. `Session()` returns a snapshot. No lock is ever held across a pass, because
  cancellation that waits on the thing being cancelled is not cancellation
- [[ADR-154-geometry-is-evidence-even-when-the-words-are-not]] — the detector was
  discarding ~70 rectangles a frame because nobody could read them, and the selection
  reader needs the rectangles and none of their words. Kept on the experiment's instance,
  counted apart from named structure, and never able to become a control.
  **Addendum 2026-10-10**: and the session now reports where the detector's output went —
  proposed, accepted, refused, by reason. The decision rested on a `considered 77, rejected
  76` reading taken by hand after the run, because `detections 0` could not distinguish a
  detector that saw nothing from one that refused everything it saw
- [[ADR-158-the-active-list-is-the-one-that-responds]] — two REAL lists can both hand a mark
  between their members, and geometry will never separate them. The one the navigation reaches
  is the one whose mark moved while the person was pressing something. The input only ever
  labels a transfer the shading already proved, is given as a count with no direction so it
  cannot predict the outcome it is correlated with, and two lists that both answer stay
  ambiguous
- [[ADR-157-vision-needs-a-cue-that-is-not-a-model]] — every piece of geometry in Marco came
  from one learned detector in a plugin the default build leaves inert, so grouping and
  selection could see only what it proposed. Replaying a real session measured the cost: the
  menu was a groupable list on about ten looks of 107. A model-free proposer now finds structure
  in pixels in the engine, with two classical cues and no dependencies — and is not yet wired
  into the live path
- [[ADR-156-a-refusal-is-a-measurement]] — the tracker had computed, on every look of every
  session, exactly why each candidate list was refused, and nothing ever read it. One caller
  used `Tracker.Evidence()`, to count how many returned true. Now reported per candidate with
  the requirements it falls short of, and the rule is stated once so the report cannot disagree
  with the belief it explains
- [[ADR-151-a-screen-that-changed-for-one-frame-did-not-change]] — a change needs a
  second look agreeing with it; a dropout is not a journey; and both error directions
  of that decision are counted, including the refusals that may have been real
- [[ADR-152-slow-discovery-fast-tracking]] — the measurement: reading a nine-entry
  menu's shade costs 597us against a 1972ms inference. The design, the existing rule that
  shapes it, and the one number still missing
- [[ADR-111-a-demonstration-takes-the-slot-from-watching]] — watching yields the one observation
  slot to a demonstration, and `marco observe --follow` reports what durable memory COMMITTED
- [[ADR-093-observe-is-attention-not-recording]] — `marco observe` is the ambient lifecycle:
  one supervisor on the one substrate, the ZERO licence so nothing it sees becomes durable,
  a transient buffer whose size tracks novelty rather than time, and no authority or desktop
  lease of any kind
- [[ADR-094-observe-gathers-evidence-learn-promotes-it]] — "learn what I just did": the trail
  gains what somebody PRESSED, an action is attributed to the screen it was performed on rather
  than to whatever is in front when it is read, and an explicit Learn promotes a selected walk
  under the same licence a live one declares. Observe still holds none
- [[ADR-095-repeated-observation-may-become-knowledge]] -- what ambient watching learns is a
  SEMANTIC GRAPH EDGE, not a demonstration, under a policy separate from watching and off by
  default: one clean traversal is already knowledge, repetition is strength on the edge rather
  than a gate in front of it, a candidate ledger that is not knowledge, no contradiction, the
  same admission boundary an explicit Learn uses, and no name invented for anything. Edges
  learned in different sessions compose into routes nobody demonstrated as a whole
- [[ADR-096-observe-and-learn-are-two-doors-into-one-graph]] -- and explicit Learn writes the
  SAME graph: one store, one identity test, one planner. What differs is the licence and the
  intent, never the knowledge. Edges taught through one door compose with edges taught through
  the other, in either order, with no duplicate topology
- [[ADR-003-evidence-authority-by-source]]
- [[ADR-012-presence-is-state-relative]]
- [[ADR-013-navigation-is-meaning-not-keys]]
- [[ADR-039-a-surface-and-a-place-inside-it]]
- [[ADR-040-a-few-scales-were-not-better-than-one]]
- [[ADR-041-a-screen-is-not-its-dominant-group]]
- [[ADR-043-teaching-is-two-passes-not-a-new-capture]]
- [[ADR-044-a-teach-attempt-is-one-episode]]
- [[ADR-047-a-place-is-remembered-a-meaning-is-answered]]
- [[ADR-063-a-pass-remembers-every-place-it-settled-on]]
- [[ADR-064-the-order-of-a-walk-is-evidence]]
- [[ADR-065-operating-marco-is-not-demonstrating-to-it]]

## Related experiments

- [[Experiment-002-dnfc-observation-baseline]]
- [[Experiment-006-button-track-fragmentation]]
- [[Experiment-007-state-relative-tracking]]
- [[Experiment-011-two-level-identity-against-real-software]]
- [[Experiment-012-why-explorer-cannot-be-pointed-at]]
- [[Experiment-013-one-sample-in-ninety-one]]
- [[Experiment-024-watching-a-player-navigate-a-real-menu]] — the first run with a
  player navigating a real menu by controller. The presses were observed; the menu was
  not. Why, measured.

## Validated by

- `TestTheObservationPackageCannotAct`, `TestTheObservationPackageStaysGameAgnostic`
  (`internal/director/observe/boundary_test.go`)
- `TestTheRunnerCannotAct` (`internal/director/observesession/boundary_test.go`)
- `cmd/director/observeguard_test.go`
- `Runtime.Handle` refuses while a session is active (mutation guard)
- `TestTheProductionSessionPathSegmentsScreenStates`, `TestScreenStatesReachNothingAuthoritative`
  (`internal/director/observesession/statewiring_test.go`) — state segmentation is reachable
  from the production path and reaches nothing authoritative
- `internal/director/observe/screenstate_test.go` — the state-relative eligibility suite

## Known gaps

- ~~A targeted session is vision-only by construction.~~ **Corrected 2026-08-06**: the
  sampler simply never set `observation.Request.Window`. Accessibility now reaches a
  targeted session — see [[director-accessibility-targeting]].
- Durable storage, fixture export and event streaming are **not built**, deliberately:
  nothing should become durable while the classifier was wrong.
- ~~`learn.Coordinator` has no synchronisation, and a cancel can be lost.~~ **Fixed
  2026-10-10**, [[ADR-155-a-learn-session-has-one-owner]]. Found 2026-10-09 by `go test
  -race` while race-checking the fast visual path's new goroutine; unrelated to it and
  pre-existing since 2026-08-19. Ten races across five `cmd/director` tests, every one of
  them `Cancel` or `Finish` against `Advance` on plain bools with no mutex anywhere on the
  type, which meant **`director learn --cancel` was not guaranteed to stop a demonstration**.
- **`go test -race` is not part of the gate.** The documented gate is `go build ./... && go
  test ./...`, and the one time the race build was run it found the defect above, which the
  plain suite had been green over for seven weeks. Making it part of the gate is its own
  decision: it is roughly four times slower on `cmd/director`, which is already the slowest
  package. Until then, run it when adding a goroutine.
- ~~A session cannot say where its detector's output went.~~ **Fixed 2026-10-10**,
  [[ADR-154-geometry-is-evidence-even-when-the-words-are-not]]'s addendum. `detections 0` had
  two opposite explanations and the record held neither; Experiment-024's central finding
  therefore rested on a separate reading taken after the run.
- ~~A belief's expiry is timestamped at the evidence it gave up.~~ **Fixed 2026-10-10**,
  [[ADR-153-a-blackout-is-not-a-destination]]'s addendum.
- ~~A session reports a refusal it cannot explain.~~ **Fixed 2026-10-10**,
  [[ADR-156-a-refusal-is-a-measurement]]. Five unrelated defects produced the same sentence —
  nothing measurable, candidate churn, no visible mark, several marks at once, or a mark that
  never moved — and telling them apart after Experiment-025 meant regrouping the run's own
  geometry with a script that was not the production rule.
- ~~A window capture can be of the wrong window with every guard satisfied.~~ **Fixed
  2026-10-10**: frontness is asked per look and a look taken behind another window carries no
  evidence. What is NOT fixed is the underlying hazard — a window that is merely *covered*
  while still in front is not detected, because being in front is all the platform is asked.
  Not-in-front is the condition under which the pixels **can** be somebody else's; it is not
  proof of occlusion, and occlusion without a focus change remains invisible here.
- **A mark that DWELLS on one row breaks exclusivity, and the obvious repair is worse.** On a
  frame where exclusivity was not established, the baseline rule teaches every present member
  that its current look is its unselected level — including the member that was selected. On a
  synthetic five-row list, exclusivity collapses somewhere between a dwell of 6 and 8
  observations: at dwell 10 it reports 0 exclusive frames, 198 shared and no transfers at all.
  Declining to teach the frame's most conspicuous member repaired that completely and cost 88%
  of the selection reads on 864 real frames (155 frames naming an entry fell to 19, identified
  after slipped from look 28 to 294); narrowing it to members with no baseline did not recover
  them. It needs a discriminator that is not frame-local brightness. **Varying the rows'
  brightness does NOT avoid it** — that was the guess and the measurement disproved it — so why
  the real corpus does not exhibit it is not yet understood. Recorded by
  `TestAMarkThatDwellsIsStillRefusedAsShared`, which asserts the defect so a future repair has
  to confront the cost.
- ~~An input's attribution window is counted in observations, which is not the same thing at two
  cadences.~~ **Fixed 2026-10-10**, [[ADR-158-the-active-list-is-the-one-that-responds]]. It is
  `observe.MaxResponseLatency`, a duration, in one named place: the attributable interval is the
  gap since the last look OR 400ms, whichever is longer. Carried as a decaying debt rather than a
  timestamp comparison, because a look's absolute time and an `InputEvent`'s producer-relative
  `AtMS` must never be joined. The replay's special case is gone.
- **Structure from pixels is not in the live path.** [[ADR-157-vision-needs-a-cue-that-is-not-a-model]]
  measures 92% row discovery on 20 drawn screens with no model and no accessibility, and a
  session cannot use any of it: the frame lives inside the vision provider, which is a plugin,
  so the engine has no pixels at sample time. Giving it some is its own change, with its own
  provenance and privacy questions.
- **No recording carries pixels.** The replay in `cmd/perceptionreplay` reads L1 — the
  detector's own output — which is enough to exercise grouping, tracking, the reader and the
  belief. It cannot test a change to the detector, to admission, or to how shading is measured,
  because those stages PRODUCED L1. Those need L0: frames, inputs and window identity, recorded
  during a supervised session. Until that exists, every change upstream of L1 is measurable only
  against drawn fixtures.
- **A mark that never moves cannot be seen at all.** A member's baseline is built only from
  frames in which it was not marked, so a permanently lit row never establishes one and is
  skipped from the judgement. Correct for the rule's purpose and surprising enough to have
  wasted a whole phase of two experiments: a protocol cannot open by holding still, and a
  screen whose selection never moves cannot be read by this path however long it is watched.
- **A run's `at_ms` and a belief moment's `sample` are two different counters.** A press
  carries how many *inferences* had run; a moment carries which *sample* it was folded at.
  They were equal in Experiment-024 because the inference blocked the cycle, and
  [[ADR-152-slow-discovery-fast-tracking]] decouples them on purpose — so any analysis that
  joins a press to a selection move must join on TIME (`started_at` + `at_ms` against the
  moment's absolute `at`), never on the two numbers. Not a defect in either field; a trap in
  reading them together, recorded because the wrong join looks right.

## Milestone record

Recorded in `HANDOFF.md` rather than a dedicated milestone document.

## What a screen requires

Four separate things, and keeping them separate is the point:

| | requires |
|---|---|
| **existence** | a validated window, and one observed composition — a bounded set of (role, window-relative region) pairs from any admissible source |
| **place** | the surface it belongs to, and whether a part of that surface now holds a kind of thing it did not ([[ADR-039-a-surface-and-a-place-inside-it]]) |
| **recognition** | a durable structure signature matching a remembered subject, with a discriminator ([[ADR-016-cross-session-identity-is-structural-and-conservative]]) |
| **naming** | a word a person typed ([[ADR-031-the-user-names-the-stage]]) |
| **interpretation** | a hypothesis with its contradictions ([[ADR-014-hypotheses-are-evidence-not-identity]]) |

An unfamiliar screen exists without being recognised. A recognised screen exists without a
name. **No screen requires OCR text to exist** — terms are credited to a screen that already
exists, never the other way round ([[ADR-017-structure-earns-a-name-text-never-earns-structure]]).

This is what lets learning say `unknown screen A → action → unknown screen B` long before
semantic memory knows what either is.

## Where the composition comes from

`observe.StructuralView`, chosen by `StructureOf`: the fused authoritative world first, the
structural detector where fusion saw no structure at all. Provenance travels with it, because
*observed and empty* is a screen and *unobserved* is not. See
[[ADR-036-a-screen-is-a-composition-not-a-provider]].

Until 2026-08-10 the only source was the detector, which is opt-in — so the default Director
had no screen model for any application. The `Related systems` line below recorded that as a
property of the session rather than as the blocker it was.

## Two levels of identity

A composition is compared twice, and the second comparison runs only when the first said *same
surface*:

| | question | measured as |
|---|---|---|
| **surface** | is this the same application surface | weighted Jaccard over the whole signature, against a running mean |
| **state** | is this the same place inside it | mass on `role@col,row` keys that **arrived or left**, per region of the coarse grid |

One comparison could answer only one of them, and the one it answered was the one that cannot
see a local change: a live session watching somebody open a second view twice recorded a single
motionless screen. The second measures **composition, not amount**, because amount ranks the
cases the wrong way round — a list loading another page changes more structure than a panel of a
different kind, and means nothing.

A state carries `Surface`; a state with none is its own surface. `Surface`, `LocalFrom` and
`LocalCell` are all session-local — a counter, a counter and a grid coordinate — and nothing
durable is keyed on them.

### What survives a restart

Durable identity is **per local state, namespaced by application**: `stateFingerprint` reads one
`ScreenState`, and `SurfaceID` reaches nothing durable. Two places inside one application become
two remembered subjects, two names, two learned-play endpoints and two ends of a directed edge —
proved through the production paths rather than by construction.

The seam: the durable signature carries role composition, the local comparison carries
`role@col,row`. Two places differing only in **where** the same structure sits are two states in
a session and one subject in the store.

Local structural comparison is **bounded on purpose**. A change too small or too sparse to have
a composition does not define a new place; it may still be evidence, and it may still be a
property of the place it happened in. The bound is `MinLocalCellStructures`, and a measurement
across a whole corpus established that it — not spatial resolution — is what keeps a caret, a
pointer or one changed control from becoming somewhere new
([[ADR-040-a-few-scales-were-not-better-than-one]]).

**Recorded limit.** An overlay of the same kind of structure over the same kind of content is
invisible to both comparisons. At (role, coarse-cell) resolution it is the same observation as
more of that content arriving.

This is an **information** limit, not a resolution one, and that distinction was measured rather
than assumed: a bounded multi-scale field was built, given four overlapping scales and two
alignments, and saw exactly what the single grid saw. What is missing is a dimension the admitted
structural vocabulary does not carry — plausibly containment, layering, or the chronology of
things appearing and disappearing. Those are hypotheses about the gap, not planned work, and
nothing in Director implements or awaits them. See
[[ADR-039-a-surface-and-a-place-inside-it]] and [[ADR-040-a-few-scales-were-not-better-than-one]].

## Provider independence, and the escalation this leaves open

All four combinations produce the honest answer, and none is privileged by construction:

| accessibility | detector | screens from |
|---|---|---|
| useful | absent | the fused world |
| useful | present | the fused world; the detector is compared, never merged |
| unavailable | useful | the detector |
| unavailable | absent | none, and `StructureWhy` says which silence it was |

Because the choice is one pure function over one sample, a future perception policy —
cheap evidence, then stronger evidence only where the cheap answer was uncertain — has a
single place to express itself. Nothing in the current path assumes a fixed provider set, a
fixed order, or that every source runs on every sample: the detector's cadence gate already
declines most slots and the screen model is built anyway.

## Capture first, interpret second

`ShadowTotals.InputLog` is the bounded, ordered record of every admitted input event in a
session, banked before ANY gate — the structural return, the quiet expiry, the state change
that consumes the attribution buffer. It claims nothing about meaning; it is the record
that the person acted, with the context known at the time, and no failure of interpretation
can erase it. [[ADR-057-attributed-input-survives-interpretation]].

Session transitions carry `TargetedSequence` — the ordered run plus what each event was
aimed at, when the evidence identified it ([[ADR-058-a-demonstrated-target-may-keep-its-name]]).
The durable topology folds these down to plain `NavSequence`, structurally: a label read
off a screen cannot outlive the session by riding an edge.

## The transition substrate

```
composition ─▶ ScreenSegmenter.Observe        screenstate.go:479
                 │  similarity >= 0.55        ─▶ SAME screen        (:501)
                 │  coverage   >= 0.75        ─▶ held, promoted on recurrence (:505)
                 │  otherwise                 ─▶ mint a NEW screen  (:524)
                 ▼
               note(from, to, inputs)          transition + attribution (:667)
                 │  inputs drained per inference in ShadowTracker.Observe
                 ▼
               ShadowTotals.Transitions        session-local, numbered
                 ▼
               RelationshipsFrom               relationship.go:298 — BOTH endpoints must
                 │                             resolve to remembered subjects, or the edge
                 │                             stays session-local
                 ▼
               Memory.RememberRelationships    durable, directed, bounded
                 ▼
               AssessLearning                  closed refusals; never a cause
```

Attribution is **banked before** the structural gate and **drained on every inference**, so
navigation survives a slot nothing looked at and cannot be attached to a change several samples
later. A transition exists whether or not anything was seen before it; `Unattributed` is the
control evidence that keeps the correlated ones readable as correlations.

### Why an edge stayed session-local

`RelationshipReport` names it, in a **closed** vocabulary — `source_unresolved` ·
`destination_unresolved` · `both_unresolved` · `same_subject` — recorded per refused transition.
Added because a missing edge looks identical whatever caused it, and the first real Learn failure
on Windows Settings was diagnosed at exactly this boundary rather than guessed at.

### One unreadable sample no longer loses the route

Live Settings recorded a single movement as two halves across a sample it could not place:

```
state_1        → state_unknown    preceded {confirm: 1}     destination_unresolved
state_unknown  → state_2          unattributed 1            source_unresolved
```

`bridgeUnsettled` (`bridge.go`) recovers the adjacency A→B from that, and asserts nothing about
what the unplaced sample was. Bounded by `Crossing.Run < StatePromotionCount` — segmentation's own
line between a transition frame and a screen, not a number invented for the occasion — plus
`Continuity.Unbroken()`, two resolving endpoints, and `from != to`. Nine refusals in the
`BridgeRefusal` vocabulary say why not.
[[ADR-049-a-change-nobody-could-read-is-still-one-change]].

### …and neither does a walk with several of them

It recovered exactly **one** interval per session until 2026-08-17, because `Transitions` is keyed
by `(From, To)` and therefore cannot say what followed what. A two-step demonstration crosses two
transition frames, which aggregates to two entries into `state_unknown` and two exits out of it —
`A→C, B→B` fits the same counts as `A→B, B→C` — so it refused `ambiguous_interval` and lost BOTH
adjacencies. At ordinary human speed every step crosses a frame, so that was the common case, not
an edge case.

`ShadowTotals.Crossings` is the session's **walk**: every change, in order, written at the one
call site (`note`) every change already passes through. It is thin on purpose — no navigation, no
counts, no interpretation, all of which stay on the aggregate. `unsettledIntervals` reads it, so
each excursion has one entry and one exit by construction, and `A → ? → C → ? → B` now yields
`A→C` and `C→B` rather than a refusal — C stays in the middle. A truncated walk
(`EvictedCrossings > 0`) is refused outright rather than paired across the gap.
[[ADR-064-the-order-of-a-walk-is-evidence]].

## What a screen must have to be REMEMBERED

Existence is cheap; durability is not, and the requirement chain is worth reading in one place:

```
structure → track (exclusive to one screen, presence ≥ 0.80)
          → structural group (an even vertical run)
          → hypothesis about the screen
          → a person answers it            ← OR: an explicit `learn "…"` pass ends here
          → a DISCRIMINATING signature (read terms, or an envelope)
          → durable subject
          → and only then can it be one end of a relationship
```

Every link is a real refusal with a reason, and four of them were invisible until this was
traced: the track bound (fixed, [[ADR-038-session-bounds-are-sized-for-the-evidence]]), the
group's vertical-run requirement, the discriminator rule — and *"a person answers it"*, which was
the only door in — so an application nobody had happened to answer a question about was an
application nobody could Learn anything in. A licensed Learn pass is the second door, and
it persists identity only:
[[ADR-047-a-place-is-remembered-a-meaning-is-answered]].

## Learn: the same machinery, on purpose

Until now Marco had to **notice** a habit before it could learn one — a route needed independent
corroboration across sessions before the invitation policy would let it ask. `director learn
"open downloads"` reverses the timing without touching the evidence model.

```
director learn "open downloads" --window-id window_37

  Hold still a moment while I learn where we're starting.
  Okay — go ahead and show me.
  I'd like to see that once more, from where we started.
  I think I understand. Want me to try it once?
```

The coordinator (`internal/director/learn`) owns no evidence, no identity, no judgement and no
authority. Each line above is a phase; each phase waits on an object that already owns the fact —
`observe.PlaceNow` for where we are, the durable topology for what changed, the ordinary armed
`Capture` for the demonstration, `AssessCandidate` for whether it holds up.

The demonstration capture is a **confirmation** mechanism and cannot discover a destination, so
Learn runs two passes and arms the capture between them through the ordinary pending learning
request — see [[ADR-043-teaching-is-two-passes-not-a-new-capture]] for why that beats giving the
capture an open destination.

**Learn grants nothing.** It cannot press a key, and permission to rehearse is still a separate
yes through the ledger. `ready_to_rehearse` and `naming` are *waiting* phases: the coordinator
polls the question that already exists and prints the command that answers it.

The tail follows the same lifecycle to its end — the authorised rehearsal, the lowering judgement,
the naming questions the judgement demands, the one persistence path — and only a saved file lets
anything say `I learned "open downloads"`. `Session.Learned` reads the artifact, never the phase.

A Learn attempt is **one episode**: its passes fold their evidence but claim one independent
sighting between them, so asking for it explicitly cannot manufacture the corroboration the
invitation policy exists to require. See [[ADR-044-a-teach-attempt-is-one-episode]].

A Learn pass may also **establish a place** — make the screen the user is standing on durably
recognisable, persisting no judgement about what it is. That is the other half of the same fact:
`learn "…"` is an explicit human semantic event, and both consequences travel together on
`observesession.Episode`. Without it, Learn refused at its first step against any application
nobody had happened to answer a question about, and *which* question Marco raised was never the
user's to choose. At most one place per pass, only where it could ever be matched again, and
`Result.Places` says which of the closed reasons applied when none was.
See [[ADR-047-a-place-is-remembered-a-meaning-is-answered]].

## Where a person sees it

Ambient watching is background behaviour, so its state is a first-class answer rather than a
diagnostic — a mode nobody can see the state of is the shape of surveillance whatever its
intentions. `AmbientView` carries it, and the control centre's **Here** panel renders it beside
what Marco can currently see: whether Marco is watching, whether what it watches may become
**durable memory** (a separate agreement, and a separate switch — see
[[ADR-095-repeated-observation-may-become-knowledge]]), and what its store has actually committed.

The surface may ask and may not decide. Four verbs, all attention or permission; an unrecognised
one is a read rather than the nearest match. It writes nothing to semantic memory, and the learning
events it draws are the store's own after its own write committed, already worded — so it cannot
name a Place even by accident. See
[[ADR-112-the-loop-belongs-where-a-person-is-already-looking]] and
[[ADR-111-a-demonstration-takes-the-slot-from-watching]].

### Three product states, and learning is inside watching

What a person operates is not the two permissions. It is three states — **not watching**,
**watching**, **watching and learning** — with one status line each and a sentence saying what
Marco is doing. `LEARN ⊂ WATCH`: asking Marco to learn starts watching with it, stopping watching
ends learning, and stopping learning leaves attention alone. There is no user-facing
"learning but not watching"; the Director makes it unreachable and the strip reports the
combination as inconsistent rather than dressing it up as a mode somebody chose. See
[[ADR-113-learning-is-inside-watching]].

The two panels below the strip answer two different questions, and saying which is what teaches
the distinction that explaining Observe and Learn never did:

```
CURRENT       what Marco sees now
JUST LEARNED  what Marco wrote down
```

`HerePlace.Perceived` is what the screen in front says it is called — the settled reading, through
`observe.SettledPlaceNameFor`, written nowhere. So watching alone can say *this screen says it is
"Mouse"* while committing nothing, and the difference between seeing and remembering is visible
rather than documented.

## Naming a Place Marco already knows

A Place is established the first time Marco can recognise it; a name settles by RECURRENCE. The
two almost never coincide, so **naming is its own sweep** rather than something that happens while
establishing — `observe.PlaceNamesToRecord`, over every state the session has seen, whether or not
any of them is new.

Ambient watching runs that sweep on every reading when learning is on, through `promotion.call`,
the one door a semantic name reaches the store by. It needs no walk, no edge, no goal, no explicit
Learn and no second Place: `PlaceNamesToRecord` skips a state memory cannot recall and
`ObserveSemanticName` refuses a subject the store does not hold, so enrichment can never fork an
identity.

Before 38A.1 there was no such sweep, and the only naming write ambient could reach lived inside
`promotion.establish` — once, at Place creation, and only when an EDGE was promoted. The first
dogfood measured the result: forty-five durable Settings places, none named, while perception could
name them the whole time. The rule was right and nothing on the path a person uses called it.

## What ambient learning may keep, and what it produces

Two permissions travel with Watch & Learn, and only one of them used to arrive.

**Promotion** is asked in `ambient.Judge`: may this candidate become durable knowledge.
**The control's name** is asked much earlier, in `liveSampler.mayNameTargets`, at the push that
offers the window's controls to the navigation producer — and a candidate whose control has no
name is refused by rule (`Act.Representable` is false, and the verdict is
`Never / control_not_named`) whatever the promotion policy says.

`NameablePlaintext` is `{button, menu_item, menu, tab, checkbox, radio}`. Anything that navigates
by `list_item`, `tree_item`, `link` or `row` — Windows Settings, most file managers, most
sidebars — needs the licence, and ambient sessions declare none. So ambient learning could not
promote anything in those interfaces at all until `mayNameTargets` learned to read the ambient
learning switch as the second door to a permission `ambientPromotionLicence` had always declared.
Read live, per cycle, because somebody presses the button mid-session. See
[[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]].

Watching alone still keeps no control name, and the shape filter is unconditional either way.

### Topology is learned; a goal is meant

Ambient promotion writes places and the ways between them. It writes **no goal** — `admitWatched`
is explicit about it — so a perfect traversal still leaves nothing a person can ask for by name.
The canonical way to close that gap is the retrospective Learn, `ObserveLearn{Recent: true, Name}`,
which promotes the walk, keeps the demonstration and records the goal in one act.

```
known Place  →  known way between screens  →  a name you gave it  →  Try it
```

### And learning says what it made of what you did

`AmbientEvidence` renders every candidate with the policy's own verdict and its own sentence, and
the control centre's Here panel draws it. Learned, seen again, or refused with the reason —
never silence, because a mode indicator with no observable result is a light rather than a
product. One rendering rule is enforced rather than assumed: a promoted candidate reports
`never / already_known`, so a surface reading the verdict alone would draw a refusal beside the one
relationship Marco actually learned.

## One experiment at a time

Marco holds at most one proposal: a promoted candidate edge it has never walked itself, chosen
deterministically and stated as a claim — from HERE, doing THIS, you arrive THERE — with a reason
made of the record's own fields. `Runtime.Experiment` is the read; it chooses nothing durable and
moves nothing, because observation permission is not actuation permission and neither is a page
refresh.

Running it is `Runtime.TestEdge`, and it is a projection of the performance path rather than a
second executor: `bringForward`, `freshLook`, `observe.PlanToGoal`, `performPlan`,
`confirmArrival`, the command registry, the per-step authority and the actuation lease, all
unchanged. What is new is that the SOURCE is required before the action — an experiment run from
the wrong screen tests nothing and presses a control nobody chose — and that the desktop is given
back afterwards, on every path out including the ones where Marco gave up.

Positioning uses the canonical planner with the canonical eligibility and does not explore. An
experiment whose source nothing connects to refuses, and says what it needed and where it was.

See [[ADR-115-one-experiment-and-the-desktop-given-back]].
