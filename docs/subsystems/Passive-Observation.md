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
updated: 2026-10-11
source_paths:
  - internal/director/observe
  - internal/director/observe/interface.go
  - internal/director/perception/selection/group.go
  - internal/director/observe/track.go
  - internal/director/observe/shadow.go
  - internal/director/perception/selection/selection.go
  - internal/director/perception/selection/active.go
  - cmd/perceptionreplay/score.go
  - cmd/perceptionreplay/listidentity.go
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

- [[Passive-Visual-Screen-Integration]] — the PROPOSED plan for connecting the session-local
  visual screen to the live observation path, passive only. Not built, and it states the three
  preconditions it should not be started under — the chief one being that the settle rule counts
  looks and the live cadence is twelve times slower than the recording's, so three looks is
  eleven seconds there and the damping would discard every real navigation.

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
- [[ADR-181-a-demonstration-marco-can-keep]] — the missing middle between watching and
  doing. A Draft is the ordered record of one demonstration with its evidence on the
  steps it belongs to; a Plan is what the Draft would do against the screen in front of
  it. Two refusals kept apart: a draft is not a Play (only `Confirm(i, by)` sets a step
  executable, and no amount of evidence does), and geometry is not identity (`Support`
  has no semantic value, deliberately). The press position is not merely unused — `Plan`
  takes reacquisition outcomes rather than the correlator, so the coordinates are out of
  scope. Measured on the recording: the draft is the route a person walked, 21 steps; it
  **reaches step 1 of 21** before stopping, because two named controls can never be
  resolved, while 10 of 13 can be. One step lowers through `marcoexec` to a complete
  Marco program, unrun.
- [[ADR-182-two-controls-that-could-never-be-found-again]] — why the two controls that
  blocked the learned workflow could never be reacquired, diagnosed off the pixels
  already on disk. `target_2` is a PROPOSAL failure: the TRAINING tile is never
  proposed, and the one rectangle containing the press is a 778x230 slab of stadium.
  `target_6` is an IDENTITY failure with a cause — the identity is a run of the EXIT
  TO MAIN MENU label, its recorded container is the button, and the two are the same
  rectangle, so the one rectangle that IS the button was barred from matching it. A
  rectangle a press identified the control as is no longer a container of it; a DROP
  on the second press, never a refusal to file, so the row-in-a-panel hazard stands.
  10 of 13 controls resolvable becomes 12 of 13, with no new false matches and the
  product answer still 1 of 21.
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]] — PROPOSED, unbuilt.
  The persistence boundary: a durable Workflow that carries its evidence and asserts
  nothing, a session-local Binding that recognises and expires, and an authorisation
  per action per look. Persistence does not imply recognition; recognition does not
  imply authority. Names the one thing that has to be measured first — whether an
  arrangement binds the same screen across two independent folds.
- [[ADR-180-finding-a-demonstrated-target-again]] — a demonstrated click target can be
  looked for again from the proposals alone, with the press position neither used nor
  reachable. Three outcomes, not two: matched, ambiguous, unobservable — and `matched` is
  not permission. No new threshold: `sameRect` decides, which forces two things worth
  knowing — supporting proposals are grouped into CONTROLS so one row read twice is not
  an ambiguity, and a region that merely CONTAINED the press cannot be support, because
  mutual centre containment is not a size test and a row straddling its panel's centre
  is "the same rectangle" as the panel. Measured on 900 looks: 52.2% matched, 47.8%
  abstained, 0 colliding pairs — and on the only ground truth available, second presses,
  **3 verified, 0 contradicted, 2 declined**. Two of twelve targets can never be found
  again, and a scrolled list would be matched wrongly by any geometry. It also found an
  accumulation that could never accumulate: `visualTarget.where` was documented as every
  reading of a control and always held exactly one.
- [[ADR-179-the-first-live-observation]] — the fast observer has now run against a live
  screen once, 60 seconds of Rocket League at 10fps, started by the owner. Cadence held
  exactly (600 looks, 0 dropped), the route is UNIQUELY recoverable from the transition
  counts, four screens were re-recognised on return, and no look was missing, covered or
  interrupted. Every press is accounted for from the hook to the attributed step — which
  is what found the last hole: key RELEASES incremented `Received` and landed in no
  subset of it, so eleven events reconciled to nine. The invariant is stated now rather
  than the case patched. Acquisition, proposer and whole look are timed apart from the
  next run on, because 600 looks in 60 seconds bounds a look at 100ms and says nothing
  about whether it took twenty or ninety-nine.
- [[ADR-178-the-fast-observer]] — the fast half of perception, built. It notices that the screen
  CHANGED and what was clicked; the slow semantic path keeps its own cadence and says what the
  screen IS. Its settle rule is derived from its OWN interval, because the rule was always a
  duration (~200ms) and the count is how that is measured at a rate — and it REFUSES to start at
  a cadence too coarse to damp, rather than clamping and observing at a rate whose results
  nobody should trust. Three ways a look is not evidence, counted apart: no pixels, another
  window in front, and the window's generation changing — the last gives up the standing claim
  WITHOUT recording a transition, so no edge spans a window being replaced. Publication never
  blocks. Measured over the recording: the fold itself is 0.05ms, a live look about 54.7ms, 18
  a second against the ten needed, and it reproduces every offline conclusion exactly — nine
  screens, the same look counts and settle points, 13/5/3 causes. A consumer 300ms behind drops
  177 of 300 updates and changes neither the cadence nor the conclusions. It cannot act or
  remember, and its own tests walk its imports to prove it
- [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] — the measurement that overturned the
  one above. Acquisition is **16.7ms**, not ~130ms, and neither candidate optimisation moves it:
  handle reuse removes a 0.000ms stage, and a half-size `StretchBlt` costs in the blit exactly
  what it saves in `GetDIBits`. The display is 60Hz and the total is 16.745ms at 59.7 captures a
  second on every configuration — the blit is waiting for the compositor, so it is a frame
  interval and a ceiling, not a cost. A full-resolution look is 50.5ms, about twice the cadence
  the model needs, so the capture path needs NO change and the resolution trade-off is retired.
  ADR-176's elimination argument was sound and its conclusion wrong, because the number it
  eliminated against was a comment rather than a measurement
- [[ADR-176-the-time-is-in-the-acquisition-and-a-smaller-observation-keeps-the-clicks]] — where
  an observation's time goes, measured with no screen captured. Marco's own share of a capture —
  the BGRA conversion and its 8.3MB allocation — is **2ms**, about 1.6% of the ~130ms a capture
  was noted at, which rules out the cheap fix. The proposer is 31.9ms at full resolution and
  4.6ms at a quarter. A reduced observation keeps **all 17 click targets down to 480x270** and
  loses the route correspondence, 5 steps to 2 — a quality a check aimed only at targets would
  have missed. And the ~130ms is not corroborated by the recording's own 10.0 fps with capture
  inline, so acquisition is the one stage still unmeasured and the only one large enough to be
  the bottleneck
- [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] — the settle rule was chosen
  against a 10fps recording and the live path samples at ~3.8s. Swept at every rate and every
  phase: at 3.8s the rule settles on 0–3 of nine screens while SAMPLING 5–6, so the screens were
  seen and the rule discarded them, with an 11.4-second confirmation latency at the best phase.
  Lowering it to one look fixes the live cadence and produces 113 promotions over 900 looks with
  72 unknown causes. And click attribution degrades faster than the screens do — 13/5/3 at 10fps
  becomes 0/0/12 at 3.8s. The default is unchanged and the evidence is why: the slow cadence
  cannot support this model, and the pixel proposer's measured 39ms a frame (≈26 looks/second on
  one core) says the fix is a faster local observer whose cadence is not the AI's
- [[ADR-174-three-kinds-of-transition-and-what-a-brief-screen-was]] — not every screen change is
  caused by a click, and a planner that assumed one would fail on ordinary applications. Three
  causes now: CLICK (a press on a grounded target), AUTOMATIC (no press, following Marco's own
  arrival by click — a load completing, answered by waiting) and UNKNOWN (no press and no arrival
  behind it, which is where an animation, a splash and a segmentation artefact all sit). Only a
  click is `Executable()`, and the other two carry no target at all. Measured: 13 click, 5
  automatic, 3 unknown. And a brief screen is EXPLAINED rather than deleted, with no duration
  threshold anywhere: `visual_5` turned out to be a dialog that appeared by itself and was
  dismissed with a click, `visual_8` is the loading state, and `visual_9` is genuinely
  unexplained
- [[ADR-173-the-proposers-yield-is-not-what-separates-two-screens]] — the suspicion that two
  settled screens were one screen at two proposal yields, measured and REFUTED: `visual_1` at
  236 regions and `visual_6` at 400 differ by shape as well as by count. And normalising the
  count would merge `visual_3` with `visual_4`, which are two consecutive steps of the
  demonstrated route joined by a confirmed transition and an attributed click — so it is
  rejected on measurement rather than on taste. The highest shape-similarity between any two
  settled screens is 0.695, which is the baseline a future attempt has to beat
- [[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]] — the learning unit of a
  mouse demonstration. Every region that contained a press is kept, smallest first, and the
  smallest is a stated hypothesis rather than a decision; a change is attributed to the most
  recent press made on the screen being left, inside a bound of 50 looks that was MEASURED (a
  ten-look window reported a real press as having changed nothing, because its transition took
  thirty), with the gap reported per occurrence so an implausible attribution is visible. Four
  identity rules were measured and they trade a split against a merge, so none is chosen: the
  pairs that might be one control are reported with the evidence. On the recording: 3 units
  confirmed in both demonstrations, 2 pairs that may be one control, 4 presses that changed
  nothing, 8 changes nobody pressed for — and an inspectable route with its doubts in the line
- [[ADR-171-an-arrangement-is-an-observation-and-a-screen-is-an-interpretation]] — the middle
  of three identities, built: a frame-local arrangement is an observation, a session-local
  screen is an interpretation, and the gap between them is time. A change must hold for three
  consecutive looks, each agreeing with the arrangement that STARTED the run; coming back reuses
  the screen it left, which is what makes a route visible; a blank look is a missing observation
  and never a screen, and a persisting absence gives the screen up without recording a
  transition. On the measured recording: 150 raw adjacent-frame changes became 22 promotions,
  and SEVEN transitions happened exactly twice, once per demonstration, chaining into the two
  walks the person actually made. It may not become durable, and a test that walks `internal/`
  holds that
- [[ADR-170-what-an-unnamed-arrangement-would-have-segmented]] — the screen model is one
  vacuous state on the pixels path, which killed three of four acceptance questions on the first
  mouse demonstration. So the same frames were asked what the segmenter WOULD have produced if
  unnamed geometry counted: seven screens, 13 of 17 presses followed by a different one, and the
  route visible in both runs. A measurement with production's own arithmetic, labelled as one,
  wired into nothing. The separation works; the noise is the design problem
- [[ADR-169-a-recording-that-could-not-explain-its-own-total]] — the first real mouse recording
  held 17 presses and the person who made it had clicked more times than that. Four things cause
  that difference and two of them were counted NOWHERE: a middle click or wheel notch, and a
  press Windows marks as injected. Both happen on the callback, before the queue, so `Received`
  never saw them. The callback counts them now, the session record carries the producer's
  counters whole instead of a sentence derived from one of them, and the recorder prints them
  when a run ends. Two defects found in the same reading: `controller: observed` in a session
  with no controller in it, and a press made during the final frame thrown away with the
  subscription that held it
- [[ADR-168-a-mouse-demonstration-the-recorder-could-not-hold]] — `capture-game-session` could
  not record a mouse click. It opened a navigation subscription and never pushed the window
  rectangle a press is placed against, so every press was refused `unplaceable_pointer` and
  dropped whole, and the recording said `0 inputs` — the same thing a session nobody played
  says. The position was dropped again on the way to disk and a third time on the way back in.
  Fixed, with the capture's own rectangle rather than the reference's, because a press and a row
  have to be fractions of the same rectangle. The position is demonstration evidence and never
  an execution target
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
- [[ADR-160-activating-an-entry-is-not-moving-the-mark]] — moving a highlight is not opening a
  menu, so a confirmed screen change now records which entry the origin was pointing at, as its
  own hypothesis kind with its own subject. The evidence crosses the durable boundary with the
  moves observed inside that list, and a planned route carries an approach per step: the row it
  was taken from and the chain of observed moves that reaches it. It still cannot say which key
  produces a move, which is the next thing a taught "open the third entry" needs
- [[ADR-159-a-move-of-the-mark-is-a-transition-too]] — a screen transition is two whole
  compositions, and navigating a menu changes neither, so the thing a menu mostly does left
  nothing behind. A move of the mark between two entries of a grounded list is now retained and
  reported as its own hypothesis kind, with the observed change, the attributed input and the
  unclaimed causation in three different places; the screen is provenance rather than identity,
  because the pixels discovery path segments none
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
- [[Experiment-027-what-did-you-click-and-where-did-it-take-you]] — PLANNED, not run. The first
  mouse-driven demonstration, on an unmodded game surface, and the first one whose acceptance
  questions were written down before the data existed. Says in advance which of them the current
  path cannot answer, and why.

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

- **How many presses the first mouse recording lost is not known and cannot now be
  established.** `rl-freeplay-1` holds 17, the person who made it had clicked more times, and
  the counters that would separate "the producer lost them" from "those were never navigation"
  did not exist when it was made. The next recording will say — and so will the first live
  observation, which as of 2026-10-12 reconciles the hook's received count through to what the
  correlator folded. [[ADR-169-a-recording-that-could-not-explain-its-own-total]]
  [[ADR-178-the-fast-observer]]
- **A mouse demonstration cannot identify a list, and this is structural rather than a
  defect.** The list machinery is keyed to a mark moving between entries under directional input
  ([[ADR-158-the-active-list-is-the-one-that-responds]]); a mouse hands no mark along anything.
  Measured on 900 real frames: `no list has been seen handing a mark between its members (115
  candidates watched over 898 frames)`, with every press landing on a proposed region. So
  click-target navigation depends on a different half of perception — region identity across
  frames, and screen change — than directional navigation does.
  [[Experiment-027-what-did-you-click-and-where-did-it-take-you]]
- **A click in a GAME cannot resolve to a named target.** `observe.SemanticTarget` is resolved
  against an offered control index, which the live path fills from accessibility; a game has no
  accessibility tree, so the honest answer there is `no controls were on offer`. The replay
  counts whether a press landed inside a region the proposer found — which is what makes it
  re-groundable — and does not yet join the press to the LIST MEMBER it landed on, or to the
  screen that followed. Both are needed before "I saw what you clicked and I learned where it
  took you" is a claim Marco can make.
  [[ADR-168-a-mouse-demonstration-the-recorder-could-not-hold]]
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
- **The Trackmania corpus cannot validate the activation half at all.** It is 864 frames of one
  pause menu and the screen never changes, so there is no activation to observe:
  `0 change(s) over 0 observation(s)`, which the replay now says in those words. That is a fact
  about the recording, not about the evidence - and it means the first claim of
  [[ADR-160-activating-an-entry-is-not-moving-the-mark]] rests on synthetic fixtures until a
  recording exists that walks a menu AND opens something. A capture that enters a submenu and
  returns is the smallest thing that would settle it.
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
  cadences.~~ **Fixed 2026-10-10**, [[ADR-158-the-active-list-is-the-one-that-responds]], and
  **corrected twice on 2026-10-11** in that ADR's addendum. It is `observe.MaxResponseLatency`,
  a duration, in one named place. Three things were wrong after the first fix: the replay kept a
  400ms lookback of its OWN, which compounded with the watch's debt to about 800ms and is what
  made the corpus resolve at all; the window was the interval OR the bound rather than the
  interval AND the repaint, which set to the interval alone expires at the exact instant the
  next look is taken (0 of 26 transfers attributed on a fixture where every one was the look
  after a press); and navigation was only read on a look the detector took, so live, where three
  slots in four bring no look, most presses never opened the window at all.
- ~~A move is attributed when the reader CONFIRMS it, which is a look or two after it happened.~~
  **Fixed 2026-10-11**, [[ADR-162-coverage-and-accuracy-are-different-failures]]. The reader now
  reports which row a frame POINTED AT without publishing it (`selection.Result.Provisional`),
  and the watch banks the moment plus whether the response window was open then. A move carries
  `FirstSeen` beside `At` and `Prompted` is decided as of the sighting. The published reading is
  byte-identical, which is what makes the addition incapable of regressing coverage or accuracy.
  Three things are worth keeping from how it went:
  - **The earlier answer REPLACES the later one.** Dating a move from its confirmation did not
    only lose attributions, it invented them: a press made AFTER the mark arrived opened a window
    still open when the reader agreed, so the press was credited with an outcome it cannot have
    caused — the input predicting the outcome, reachable through the clock rather than the data
    path. An `or` would have kept it.
  - **The snapshot must be taken after the window is aged to those pixels.** Placed before
    `navOpen` on the fast path, a press two seconds earlier was credited. Caught by the
    pre-existing `TestAFastMoveLongAfterThePressIsNotPrompted`.
  - **On the real corpus it changes nothing**, because one look there is 66ms and the window is
    400ms. The repair is for the live cadence, where the same one look is two seconds; drawn as
    `TestAPressBeforeThePixelsMovedSurvivesALateConfirmation`.
- **Coverage and accuracy were one number, and the number was neither.** `43 of 189 (23%)` was
  reported as "selection reading at 23%" and ranked as a reader problem. The two counts did not
  share a denominator — one was over every frame, the other over labelled frames — so their ratio
  is a ratio between populations. Taken apart in
  [[ADR-162-coverage-and-accuracy-are-different-failures]], the same corpus says:
  **coverage 185 of 829 (22%)**, of which **637 frames identified no list at all** and 7 had a
  list with no readable row; **accuracy 43 of 185 (23%)**, of which **0 were off by a neighbour**
  and 139 of 142 wrong namings had the labelled row outside the list being read. Handed a list
  containing the labelled row the reader names it **43 of 46 (93%)**, which clears the 90%
  target. So the reader is not the bottleneck at either end: it costs 7 frames of coverage out of
  829 and no neighbour confusions at all.
- ~~No tracked candidate is a list; every one that qualifies is a bucket.~~ **Fixed 2026-10-11**,
  [[ADR-164-a-list-is-the-same-list-when-its-rows-are]]. A list is now the same tracked list when
  its ROWS correspond across observations — most of this frame's rows being rows the candidate
  already knows, in the places it knows them, tested against the SHORTER of the two rows so the
  test is scale-free and a nested sliver fails it. Candidates and members both age out, and their
  evidence goes with them. Measured over 864 frames: **correct-list identification 45 → 312**,
  **confidently wrong list 96 → 0**, and **no candidate rehomes at all, with every hull equal to
  its extent** where before every qualifying candidate had a full-screen hull and 37–127
  rehomings. Coverage fell 135 → 33 and right namings 43 → 30, with wrong namings 92 → 3, and
  retained graph-ready moves went 2 → 3. The
  surviving gaps from that work are the four entries below.
- ~~The reader cannot read the list it is now handed.~~ **Mostly fixed 2026-10-11**,
  [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]]. The reader filed each row's
  history under its POSITION in the slice it was handed, so a detector that found one row more or
  fewer threw all of it away — and on this corpus the count takes nine different values and
  changes on most frames. Filed under the row's ENTRY instead: coverage 33 → 100 frames, right
  namings 30 → 78, and handed a list that contains the mark the reader is now right **78 times
  out of 78**. Two more losses sat on top of it, both found with the `why silent` diagnostic:
  temporal evidence was refused for the whole frame unless EVERY row had a baseline, and the
  confirmation machinery's `asserted`/`pending` were positions too. 212 of 312 correct-list frames
  still name nothing, and the reader's own word on most of them is that no row stands out from its
  siblings or from its own unselected level — which is genuinely unreadable shading rather than
  evidence lost in the pipeline.
- ~~The remaining selection error is the list's extent, not the reader.~~ **Largely fixed
  2026-10-12**, [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]]. A list shares an
  edge and `alike` tested the left edge and the centre but not the RIGHT one — so one partially
  found row, right-aligned with its siblings and a third of their width, cut a nine-entry menu in
  half and left the tracked list as one piece of it. A completeness funnel placed it exactly: over
  the 312 frames that published a list, the row the person was on was detected in the list's own
  column on EVERY one of them, was in a stack grouping made on 197, and was a member of the list
  on 85 — and of the 112 lost in between, 109 were in a stack holding none of the list's members
  and **4** were in its own stack. Membership and member aging were exonerated; grouping was the
  loss. With the right edge admitted: correct-list identification 312 → 559, coverage 100 → 362,
  right namings 78 → 349 at 96%, neighbour confusions 20 → 2, and **stride 10 goes from
  identifying nothing to 29 of 83 frames with 21 of 21 namings right**.
- **Wrong-list publications are no longer zero: 6 of 829 at stride 1 and 26 of 280 at stride 3.**
  A shared right edge joins rows that belong together and, on a few frames, joins something that
  does not. Stride-3 accuracy fell from 100% to 55% with it — 29 right and 24 wrong where it was
  9 and 0 — because the list there is sometimes three or four rows. More right answers and more
  wrong ones at that cadence. [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]]
- **Reducing confidently wrong selections at a coarse cadence is OPEN, and the two obvious rules
  are measured and rejected.** List completeness is now reported per look — how many rows the list
  HAS that the look did not show — and it is the shape of the problem: at stride 1 the list is
  complete on 436 of 565 looks (77%) and at stride 3 on 26 of 77 (34%). Withholding the selection
  claim whenever a row was unseen raised accuracy 96% → 98% and cut confidently wrong readings
  13 → 7 (and 24 → 5 at stride 3), and took **retained graph moves from 13 to ONE**, because a
  move is recorded between two readings of one list and the withheld looks fall between them. On
  the same corpus 65 namings were made while a row was unseen and **6** were wrong, so the rule
  gives up 59 right answers and 12 moves to remove 6 errors. The next rule has to separate those
  6 from the other 59. [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]]
- **The gap between 349 correct readings and 13 retained moves is attributed, and the remainder is
  the RECORDING.** Three gaps stacked: most of the 349 are repeated readings of a settled
  selection, and the corpus holds 33 labelled changes of row of which 26 were ever named; 13 of
  the menu candidate's 21 observed transfers became moves, with only **9 sightings dropped in the
  whole session** (5 unread too long, 2 for want of a list, 2 for the list changing), so retention
  is near its ceiling; and all 13 moves were seen exactly ONCE, on a screen that never changes, so
  every one is `thin_evidence` and no durable edge can have recognised endpoints. **No amount of
  perception work produces a durable edge on this recording.**
  [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]]
- **Superseded, kept for the measurement.** An abstention rule for an incomplete list is wanted. Nothing the reader can see
  distinguishes a claim about a row the list does not contain from one about a row it does: the 20
  wrong claims rested on the same sentence — "away from its own unselected level, while its
  neighbours are not" — as 52 of the 78 right ones. The list-level signal that is available is too
  blunt: withholding the claim whenever the matched stack showed rows the list has no entry for
  removed 7 wrong namings and cost **232 right** ones, and narrowing it to rows beyond the list's
  ENDS changed nothing, because a list's own stack routinely extends past its members. The next
  candidate is a signal from the list's history rather than from one look at it.
  [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]]
- **Superseded, kept for the measurement.** All 22 confidently wrong
  namings at stride 1 were frames where the labelled row was not inside the published list, so none
  of them was a misreading. The menu was tracked as six or seven of its nine entries; letting a list
  admit a row where its own pitch says its next row goes narrowed that and did not close it, and
  it cost 16 correct-list frames at stride 3.
  [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]]
- **A drawn list of identical rows is read once and then held, not re-read.** On the first look at
  a newly marked row the claim still standing is the row the mark has just LEFT, so the arriving
  row's own selected level is filed once as its unselected one, and from then on it sits closer to
  its own baseline than it should. Three repairs were measured — remembering after settling,
  skipping the row being confirmed, and a tighter fade guard — and each bought about one right
  answer for ten extra NEIGHBOUR confusions on 864 real frames, which is the worst error a
  navigation route can be built from. The pollution stays and the defect is open.
  [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]]
- **Superseded, kept for the measurement.** The reader could not read the list it was handed: 279 of
  the 312 frames that published the right list named nothing, and the reader's own words on most of
  them were "no row stands out from its siblings or from its own unselected level" with six to
  eight rows of the right list in hand. This was invisible while the published extent was a bucket
  spanning the screen, because the reader could find some spike almost anywhere inside it — 135
  namings of which 92 were wrong. A relaxation of the rule that decides which rows are inside the
  list, from containment to "mostly inside", was measured here and reverted: worth 4 namings, and
  no fixture could be built that it was load-bearing for. The replay reports `why silent` and `entries in hand` so the two
  failures behind one silence stay apart: a reader that looked and found nothing, and a reader
  given too few rows to have a middle. [[ADR-164-a-list-is-the-same-list-when-its-rows-are]]
- ~~Retained graph-ready moves are gated on ENTRY identity.~~ **Fixed 2026-10-11**,
  [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]]. A move was retained only
  across two readings with the same frame-local entry count, and `Selected` was an index into the
  rows the detector found inside the list. Both are now the list's own: the entry's identity and
  the list's member count. **3 → 11 retained moves** at stride 1, and labelled changes of row ever
  named 15 → 22 of 33. Durable edges are still 0, and 11 of 864 frames is not a graph.
- **Superseded, kept for the measurement.** Retained moves were gated on entry identity: 3 of 864
  frames at stride 1, against 2 before. A move is retained only across two readings of the same list with the
  same frame-local entry count, and `Selected` is an index into the rows this frame's detector
  found inside the list — a count that changes nearly every frame, so the held sighting is dropped.
  Comparing list IDENTITIES rather than published extents was measured and kept, and did not move
  this number. The repair is to make an entry a member of the tracked list rather than an index
  into one frame's rows. [[ADR-164-a-list-is-the-same-list-when-its-rows-are]]
- ~~A list cannot grow, so a nine-entry menu is tracked as six or seven.~~ **Fixed 2026-10-11**,
  [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]]. Growth was measured and
  rejected in ADR-164 for accreting to 27 members on a nine-entry menu — and that measurement
  predates the repair to `forgetColdMembers`, whose guard against shrinking a candidate below
  `minStack` was written as "how many are left to consider" rather than "how many may go", so it
  dropped at most one member per observation. Growth was unbounded and aging nearly inert.
  Retaken, the menu candidate holds at six members with no rehoming and growth removes 52 of the
  63 confidently wrong readings. It still costs 16 correct-list frames at stride 3.
- ~~At a coarse cadence nothing is identified at all.~~ **Fixed at stride 10, 2026-10-12**,
  [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]]. The cause was not the cadence: a
  menu cut in half by one badly found row has too few exclusive frames to qualify, and a whole one
  has enough. At every tenth frame the menu is now identified on 29 of 83 labelled frames, after 58
  observations, and the reader names 21 entries of which all 21 are the labelled row. **Stride 20
  is still zero.** Held by `TestAtACoarseCadenceTheCorpusIdentifiesTheMenu`.
- **Superseded, kept for the measurement.** Strides 10 and 20 of this corpus
  abstained, naming `exclusive_over_shared 28/34` as the shortfall: at 660ms between looks the mark
  has usually moved between them, and the real menu's candidate reaches 28 exclusive frames
  against 11 shared. Production runs at about 2s a look. What the coarse cadence identified before
  was not the menu — the only candidate that qualified had a final extent of y 0.078..0.355 and a
  hull of y 0.000..0.898 and had rehomed 21 times, where the menu is at y 0.528..0.857 — so its
  22 "correct" frames were a bucket drifting across the menu on its way past. Held by
  `TestAtACoarseCadenceTheCorpusIdentifiesTheMenu`, which asserts no wrong list and a readable shortfall.
  [[ADR-164-a-list-is-the-same-list-when-its-rows-are]]
- **The dwell guard is the strongest open lead and it is not in the tree.** Declining to teach the
  frame's loudest member on a non-exclusive frame was rejected in an earlier cycle for costing 88%
  of selection reads; that measurement was taken while candidates were buckets, and retaken under
  identity by rows it goes the other way — correct list 261 → 318 and coverage 25 → 38 at stride
  1, 63 → 117 and 8 → 39 at stride 3. It is still out, because it makes a DRAWN list of identical
  rows unreadable (`dwell 3` falls from 218 exclusive frames to 0) and breaks twenty fixtures.
  Narrowing it to members that already have a baseline did not help and the reason is not
  understood. [[ADR-164-a-list-is-the-same-list-when-its-rows-are]]
- **Superseded, kept for the measurements.** No tracked candidate was a list, and both geometric
  repairs were worse: `Tracker.match` used `overlap`, which divides the intersection by the SMALLER
  area — right in grouping, where a row contains its label and the label is a duplicate of the
  row, and wrong here, because any group nested inside a tracked extent scores 1.0 and is taken
  for the same list. Measured over 864 frames with the new `Hull` and `Rehomed` diagnostics:
  **every candidate that ever qualified had a hull spanning the whole screen**, and the four
  qualifiers had changed area by more than a factor of two 37, 46, 101 and 127 times, against
  0, 0, 0, 5, 6, 14, 87 and 108 for the eight that never qualified. So drifting is what EARNS a
  candidate its verdict: a bucket sees "a different member marked" whenever it absorbs a
  different group, which reads as a transfer among several holders.
  Two repairs tried and rejected, scored against the labelled list's own extent:

  | | correct list | coverage | accuracy | the reader alone |
  |---|---|---|---|---|
  | as it is | **45 of 829** | 185 | 43 (23%) | 43 of 46 (93%) |
  | intersection over union | 6 of 829 | 86 | 44 (51%) | 44 of 51 (86%) |
  | a scale guard on overlap | **0 of 829** | 269 | 87 (32%) | 87 of 89 (98%) |

  Intersection over union fragments the real menu, whose grouping varies between three and seven
  rows frame to frame, and identified nothing at all at two of four strides. The scale guard
  improved three metrics while taking the one that matters to zero, identified nothing at strides
  3 and 20, and accumulated candidates without bound — which says the nesting-tolerant match has
  been doing load-bearing work in keeping the population down. **The untried idea is a MEMBERSHIP
  test** — are these the rows this candidate already knows, by position, which `memberAt` already
  asks — because it is behavioural rather than geometric. Recorded by
  `TestANestedGroupIsNotTheListAroundIt`, which now asserts the repair and carries all four
  measurements, so the next change has to confront what these cost. The number to beat is 45 of 829 on correct-list
  identification, not coverage or accuracy. Row correspondence is what that membership idea became,
  and it reached 312 of 829. [[ADR-163-a-list-that-qualifies-alone-is-not-the-one-being-used]],
  [[ADR-164-a-list-is-the-same-list-when-its-rows-are]]
- ~~A list that qualifies alone is published with no comparison to anything.~~ **Fixed
  2026-10-11**, [[ADR-163-a-list-that-qualifies-alone-is-not-the-one-being-used]]. `Tracker.active`
  had four arms, and the one that fires when a single candidate qualifies asked nothing at all —
  not another candidate, and not whether the person's navigation had ever reached it. Scored
  against the labelled list's own extent over 864 frames:

  | arm | frames | correct list | wrong list | named | right |
  |---|---|---|---|---|---|
  | `sole_qualifier` | 144 | **0** | **144** | 139 | **0** |
  | `most_responsive` | 48 | **45** | 3 | 46 | **43** |

  139 of the session's 185 readings came from the arm that compares nothing and not one named the
  labelled row. It is also the DOMINANT publisher at every cadence — 144 of 864, 25 of 288, 55 of
  87, 15 of 44 frames — with correctness of 0%, 92%, 40% and 13%, because first past the post
  wins and which candidate gets there first depends on frame order. That is the stride
  sensitivity, named. Both arms now require `prompted >= MinTransfers`, conditional on navigation
  having been observed at all so a caller that cannot see input is unaffected. Effect: wrong-list
  frames 147 → 96 at stride 1 and 13 → 0 at stride 20, with every correct reading preserved at
  strides 1, 3 and 10; **93 wrong-list frames remain at stride 1**, because responsiveness is
  necessary and not sufficient while the candidates are buckets.
- **The list the belief reads is usually not the menu, and that is now the measured bottleneck.**
  138 of 185 namings over the full corpus came from one candidate — a band `y 0.280..0.335`, five
  and a half percent of the screen tall, nine "rows" in it — which got none right. It also happens
  to carry nine rows, which is how it satisfied the `entry named ... same 9 entries the labels
  describe` score that was documented as the one "that cannot be satisfied by a rectangle that
  happens to overlap". A list with the right number of entries can be any list at all. **A
  geometry filter was deliberately not added**: a row five pixels tall is not a row somebody reads
  text in, but saying so is the appearance assumption `selection.Tracker` exists to refuse, and
  the one previous human-scale floor cost 88% of the selection reads. It needs its own decision
  and a second corpus.
- **Over the full corpus the belief ends `ambiguous`; at every tenth frame it does not.** The
  responding share is 0.83 for the menu against 0.55 for a four-row sliver at `y 0.104..0.131`,
  a ratio of 1.5 against the required `NavMargin` of 2. **Partly explained 2026-10-11**: that
  four-row candidate is a bucket whose hull spans the whole screen and which rehomed 127 times,
  so its six attributed transfers are not one list's transfers. What is still unexplained is why
  the responsive arm never fires at all at strides 3, 10 and 20 — it needs two candidates to
  qualify, and at coarse cadences only one does.
- **Only 4 of 33 labelled row changes were ever named correctly, and 103 frames inside those gaps
  named some other row.** The ground-truth confirmation lag measures from the frame the labels put
  the new row in place, which is what the press-based latency cannot do — press-to-belief is the
  game's repaint plus Marco's delay and cannot separate them. **A gap spent silent is a slower
  answer; a gap spent naming the row the selection has left is a wrong one**, and the retained
  moves alone cannot tell them apart.
- **Responding was compared as a TALLY and is now compared as a share.** A list that moves on its
  own schedule collects attributed moves in proportion to how often it moves — with presses
  spread through a session, the fraction of any list's moves landing inside a window is roughly
  the fraction of the session the windows cover. On the real corpus the menu answered 3 of its 7
  moves and an unrelated list 5 of its 31; the tally rule picked the second. Corrected, with the
  arithmetic, in ADR-158's addendum.
- **The selection reading was scored against the wrong rectangle for two experiments, and the
  real figure is a quarter of what was reported.** `nearHeight` asks whether a believed region
  sits at a labelled row's height, and was handed the identified LIST's envelope rather than the
  named ENTRY's region. The list on the Trackmania corpus spans three quarters of the screen, so
  every labelled row was inside it; the tolerance was also three row-heights wide. Reported: 154
  of 155 (99%). Scored against the named entry at half the labelled pitch: **43 of 189 (23%)**.
  No aggregate could have caught it — `discovery 829/829` and `grouping 864/864` were both true
  throughout, because the first counts proposals covering the row and the second counts frames
  producing a stack, and neither says the stack is the menu. `cmd/perceptionreplay -inspect
  <frame>` exists now for exactly that question. Full account in
  [[Experiment-026-the-same-menu-seen-from-pixels]]'s correction. **And the repair was not held in
  place by anything** until 2026-10-11: the mutation gate put the envelope back and every floor in
  the corpus test still passed. `selectionScore.NamedAsTallAsList` is the guard — an entry is not
  half a list — with `TestANamingAsTallAsItsListIsReported` as the case it must fire on, because a
  corpus assertion that a counter is zero is also satisfied by a deleted counter.
- **The menu groups as six of its nine entries, and the ordinal is still never right.** Two
  defects in `stack` are fixed — every candidate member was compared to the SEED, which on a
  slanted panel makes the test stricter the longer the run grows, and a drawn fixture's
  equal-width rows hide it because `alike` returns on similar widths before reaching the
  alignment test. A third, in `InterfaceBelief.Selected`, is fixed too: it is documented as the
  entry "counted from the top" and was an index into grouping's emission order, which appends
  strays last. What remains is that the list the belief reads is still not the menu often enough
  to name an entry correctly, and 138 frames reading a nine-entry list name the right ordinal on
  none of them. **Cause established 2026-10-11**: those 138 frames are not the menu at all, they
  are a nine-row sliver at `y 0.280..0.335` — see the coverage and accuracy entries above and
  [[ADR-162-coverage-and-accuracy-are-different-failures]]. The menu's own readings, 43 of them,
  are right 43 times.
- **Structure from pixels is not in the live path.** [[ADR-157-vision-needs-a-cue-that-is-not-a-model]]
  measures 92% row discovery on 20 drawn screens with no model and no accessibility, and a
  session cannot use any of it: the frame lives inside the vision provider, which is a plugin,
  so the engine has no pixels at sample time. Giving it some is its own change, with its own
  provenance and privacy questions.
- ~~No recording carries pixels.~~ **Resolved**: `cmd/perceptionreplay -frames` reads L0 — a
  `capture-game-session` recording's PNGs, its inputs on the same measured clock, and its window
  identity — through `structure.Propose` and `ShadowTotals.Observe`, which is the call the live
  cycle makes. What it still cannot establish is anything about the live application: a replay
  measures how well Marco interprets a recording.
- **A chain of moves names a direction and still cannot say that pressing it ARRIVES.**
  [[ADR-161-a-direction-is-joined-to-a-move-after-the-pixels]] closed the half
  [[ADR-160-activating-an-entry-is-not-moving-the-mark]] named: a `SelectionStep` now carries the
  directions OBSERVED before each move, joined after the pixels and outside anything that reads a
  mark, with `BindingSupport` saying in five words what the evidence amounts to. The watch's
  inputs are unchanged - it still takes a COUNT of directional intents, so an input still cannot
  predict the visual outcome it is being correlated with, which is
  [[ADR-158-the-active-list-is-the-one-that-responds]]'s rule.
  What remains empty is the third claim: pressing `down` from entry 1 arrives at entry 2. It is
  empty on purpose, because it needs the press performed deliberately and the outcome checked,
  which is a rehearsal and not a passive observation. So a chain with a direction on every hop is
  a route Marco could be ASKED to take and still not one it has been shown able to take. **The
  next thing needed for a taught "open the third entry" is a verified binding, and that is a
  rehearsal rather than more observation.** On the real corpus both retained moves name `down`
  and both read `thin_evidence`, which is one observation each.
- **A binding never says which DEVICE pressed it.** The meaning crosses (`down`) and the hardware
  does not, which matters to whoever eventually performs one: a controller button is not
  reachable through a keyboard FFI. `InputLog` retains the device beside every admitted event, so
  the join exists to be made when something exists that would use it. Deliberate scope, recorded
  in ADR-161 rather than discovered later.
- **The two demonstrations are not the same route end to end.** They share an identical
  five-screen prefix (`visual_1 → 2 → 3 → 4 → 1`) and then diverge, because in the first one a
  dialog appeared and was dismissed and in the second it did not. Collapsing it is not licensed:
  somebody clicked on it. A fact about what the person did, not a defect in the reading.
  [[ADR-174-three-kinds-of-transition-and-what-a-brief-screen-was]]
- **A cause disagreement is not a route disagreement.** Two steps of the measured route were a
  CLICK on one occasion and AUTOMATIC on the other, because the load took longer than the
  attribution window. Anything planning over this has to treat the cause as per-occurrence
  evidence rather than a property of the edge.
  [[ADR-174-three-kinds-of-transition-and-what-a-brief-screen-was]]
- **Nine settled screens for four or five real ones, and the proposer's yield is not why.**
  Normalising the region count is measured and rejected: it would merge two consecutive steps of
  the demonstrated route. What IS causing the extra screens is unexplained — `visual_5` held 8
  looks, `visual_9` held 2 — and short-lived screens that appear in one demonstration and not
  the other are the shape of a load or a splash rather than a place.
  [[ADR-173-the-proposers-yield-is-not-what-separates-two-screens]]
- ~~GDI acquisition is the unmeasured bottleneck.~~ **MEASURED 2026-10-12 and it is not.**
  200 captures per configuration against a real window: acquisition is **16.7ms**, not the
  ~130ms a comment claimed. `GetDC`, the DC-and-bitmap setup and the cleanup are all 0.000ms;
  `GetDIBits` is 0.947ms full-size and 0.000ms at half; the blit is 15.6ms and is the whole of
  it. Reusing handles saves nothing (16.745 → 16.720ms) because the setup it removes already
  cost nothing, and reducing during the blit saves nothing (16.747 → 16.452ms) because
  `StretchBlt` costs what `GetDIBits` stops paying. The display is 60Hz and 1000/60 = 16.67ms —
  the blit is waiting for the compositor, so this is a frame interval and a ~60 looks/second
  CEILING rather than a latency to optimise. A full-resolution look costs 50.5ms all told, about
  20 a second, against the ten the model needs. **The capture path needs no change.**
  [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]]
- ~~GDI acquisition is the unmeasured bottleneck. The benchmark is BUILT and unrun.~~
  `director capture-benchmark --application <app>` times GetDC, the DC-and-bitmap setup, the
  blit, GetDIBits and the cleanup separately over four configurations — today's per-frame
  handles at full size, reused handles, a StretchBlt reduction at half size, and both — all
  blitting the same source rectangle so a reduced capture cannot look faster for sampling less.
  Timings, percentiles, failures, GDI handle counts, and one bit per frame saying whether the
  buffer held anything, because a silently failed blit returns black. No pixels kept, nothing
  written, nothing injected, bounded by a count and a wall clock. The owner starts it.
- ~~GDI acquisition is the unmeasured bottleneck, and measuring it needs authorisation.~~
  Everything else in an observation is priced: conversion 2ms, resize ≤1ms, proposer 4.6–31.9ms.
  `capture` creates and destroys a DC and a bitmap every frame and allocates 8.3MB twice;
  hoisting those, or reducing during the blit so `GetDIBits` moves less, are the candidates. A
  direct measurement needs a real window and has not been run.
  [[ADR-176-the-time-is-in-the-acquisition-and-a-smaller-observation-keeps-the-clicks]]
- ~~A smaller observation keeps the clicks and loses the route — so the resolution choice is a
  trade.~~ **The trade is retired**: half resolution was only ever worth buying headroom for an
  expensive acquisition, and the acquisition is 16.7ms. Full resolution fits in half the budget
  and keeps all five route steps. The measurement stands and the dilemma does not.
  [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]]
- **A smaller observation keeps the clicks and loses the route.** All 17 presses still land on a
  proposed region at 480x270; the repeated-run correspondence falls from 5 steps to 2. So a
  perception-quality check has to ask about more than targets, and the resolution choice is a
  measured trade rather than a free win.
- ~~No live validation of the fast observer exists.~~ **Done 2026-10-12**,
  [[ADR-179-the-first-live-observation]]. One 60-second supervised run: cadence held at 10.0/s
  with nothing dropped, the route recoverable, four screens re-recognised on return, 7 of 7
  presses placed and folded, no transition manufactured. Two things it did NOT establish, and
  both are now instrumented rather than argued: a look's cost was bounded at 100ms and never
  decomposed, so the 16.7ms acquisition remains a bench figure; and **the occlusion guard never
  fired, because nothing covered the window**, so `!Front` is still unexercised live.
- **A live look's stages have never been measured.** The first run reported the observer's own
  fold latency (median under a microsecond, p95 1.00ms, max 1.61ms over 600 looks — consistent
  with the recording's 0.05ms mean, which is dragged by the looks that promote). Acquisition,
  proposer and whole look are timed separately from the next run on; until one happens, every
  stage figure in this subsystem is a bench measurement.
  [[ADR-179-the-first-live-observation]]
- **A keyboard-caused change can only ever be `unknown` on this path.** The live run's single
  unknown is the return to the main menu, and the likely cause is the one keyboard event it
  counted as seen-and-not-used. The fast observer correlates pointer presses; connecting key
  intents to screen changes is a separate decision, and reporting the count is what keeps the
  silence from reading as "nobody navigated". [[ADR-179-the-first-live-observation]]
- **Reacquisition is measured on five data points.** Four of twelve targets in the recording
  were clicked more than once, giving five repeat presses: three verified, none contradicted, two
  declined. That is the owner's standard met on the evidence that exists and it is not a rate. A
  second live run designed around repeated navigation is what would turn it into one.
  [[ADR-180-finding-a-demonstrated-target-again]]
- **Two of twelve demonstrated targets can never be reacquired**, and one flickers across the
  looks of its own screen (139 matched, 74 ambiguous, 163 unobservable over 376). The refusals
  are honest and they are a real cost: a control whose reading cannot be told from a region that
  contained it is permanently unreacquirable.
  [[ADR-180-finding-a-demonstrated-target-again]]
- **A scrolled or reordered list defeats reacquisition, and geometry cannot see it.** The row
  that moves into the demonstrated row's place satisfies every geometric test there is. Recorded
  as a test that asserts the wrong answer on purpose; the recording has no scrolling list, so
  this is reasoned rather than measured on real pixels.
  [[ADR-180-finding-a-demonstrated-target-again]]
- **A learned workflow now survives the session, and still cannot act.**
  `internal/director/workflowstore` is
  [[ADR-183-remembering-recognising-and-being-allowed-to-act]]'s Workflow half: ordered
  steps with their positions, each screen's arrangement and each control's readings and
  containers under labels scoped to the artifact, what is unresolved, and an audit of who
  confirmed what. Saved from `rl-freeplay-1` it is 11,917 bytes — 21 steps, 9 screens,
  9 controls, 9 arrangements, 13 audit entries. A reloaded workflow cannot run for two
  INDEPENDENT reasons: nobody has agreed to it, and its screens are labels no session
  mints, so planning is refused on identity even after confirmation. Binding is a
  caller's decision, refused entirely while anything is unaccounted for.
- **Prose is provenance too, and that is where the first leak was.** Every structural
  field was clean and the first artifact saved from the real recording still carried two
  session identities inside an `Unresolved` sentence — “could not tell target_2 from
  [target_10] on visual_2”. They are rewritten to the artifact's labels now, and an
  identity the artifact does not describe becomes a phrase. No synthetic fixture found
  this one. [[ADR-183-remembering-recognising-and-being-allowed-to-act]]
- **A saved workflow has been reconstructed and its controls found again by a session that
  never saw the demonstration.** Two independent folds of one recording, the artifact
  crossing between them as 8,511 bytes and nothing else: **5 screens bound, 1 ambiguous,
  2 with no counterpart**, and **2 of 4 controls on a bound screen re-acquired** from the
  file's own readings through the production matcher. `control_4` matched on 52 of 54
  looks. The workflow as a whole was refused, because three screens could not be
  established. [[Experiment-033-a-workflow-asked-for-in-another-session]]
- **The conservative binder fired on the first real screen.** `screen_a` had three
  candidates above the floor, the best at 0.787, and was refused as ambiguous — exactly
  the case a best-match rule gets wrong. Validation can REJECT a correspondence the
  evidence contradicts and cannot confirm one: agreement is necessary and not
  sufficient, and nothing yet can propose a correspondence without a person.
- **The two lost controls failed for different reasons, and one of them is fixed.**
  `control_2` is the TRAINING tile: with pixel proposals alone nothing on any look came
  within 0.132 of its saved rectangle, and with equivalent detector evidence on both
  sides it is found on **21 of 28 looks at an overlap of 0.997**. An evidence-coverage
  failure, not an identity one. `control_6` is unchanged by the detector: something
  agrees with its saved reading on 24 looks and every one of those is ALSO the same
  rectangle as a region that merely contained the original press. That is an identity
  failure. [[Experiment-034-two-controls-and-why-each-was-lost]]
- **`control_6` is ADR-182's `target_6` again, across a fold boundary.** Its reading is
  the 170x14 EXIT TO MAIN MENU label and its container the 352x34 button. ADR-182's
  second-press rule fixes this within a session; fold A holds only the first press, so
  the rule never fired and the artifact was saved with the label as its identity.
  [[Experiment-034-two-controls-and-why-each-was-lost]]
- **Richer proposals are not free.** `control_7` went 16 of 19 with pixels to 12 of 19
  with the detector: more candidates is more chances for one to be indistinct from a
  container. [[Experiment-034-two-controls-and-why-each-was-lost]]
- **A reacquisition measured after a fold is measuring the wrong look.** An observer
  holds admitted detections until its observation moves on, so a finished observer
  hands back the LAST look's detections for every look. Experiment-033 under-reported
  because of it — 3 of 4 controls, not 2. Re-acquisition now runs inside a fold, which
  is also the order the live path uses.
  [[Experiment-034-two-controls-and-why-each-was-lost]]
- **The Binding half is still unbuilt, on purpose.** Nothing in the store compares an
  arrangement or picks a best match, because
  [[Experiment-029-binding-a-screen-across-two-independent-folds]] measured that relation
  binding the wrong screen at 0.886 while binding right ones at 0.556. Still needed
  before anything can be performed: a binder that can abstain, re-acquisition from stored
  readings rather than a session registry, fresh confirmation (built), and live execution
  authorisation (unbuilt).
- **A learned draft is session-local, and that is the one remaining blocker to the product.**
  It names screens identified by a count of rectangles in nine boxes, and those identities mean
  nothing in the next session. "Tomorrow: Marco, enter Free Play" needs cross-session screen
  identity, which is explicitly not authorised — so persisting a draft is its own decision.
  [[ADR-183-remembering-recognising-and-being-allowed-to-act]] proposes the boundary and names
  the measurement that has to come before it.
  [[ADR-181-a-demonstration-marco-can-keep]]
- **The proposer has no cue that could find a large artwork tile, and that is now measured.**
  Inside the TRAINING tile's 181x115 working cells there is no component larger than 125x31:
  405 fragments, nearly all of them in the text-height band, so the tile's artwork is read as
  lines of writing. Its border is not a connected component either. Three changes were
  prototyped and all three rejected on measurement — the band-height guard (no effect), a
  text-height ceiling on a merged line (removes every frame-scale blob and takes a bare
  rendered scene from 24 candidates to 265, failing the package's own test) and an outline
  cue (the premise is false). This needs a capability of a different kind.
  [[Experiment-028-the-tile-that-was-never-a-candidate]]
- **That capability exists in the tree and is not wired to this path.** The vendored
  ScreenParser detector proposes the TRAINING tile at `(397,527 360x226)` — IoU **0.928**
  against independently measured bounds — on **14 of 14** settled PLAY-menu looks, scoring
  0.37–0.44, and proposes nothing over the press point on the looks where the control is
  absent. It finds all six tiles, the pause menu's four buttons, and the stadium that
  produced the `target_2` slab yields **zero** detections. It costs **569–746ms a frame**
  against a 55–65ms look, so it can never run in the fast loop.
  [[Experiment-030-asking-the-learned-detector-for-the-tile]]
- **The tile's TEXT is the thing that would have been missed.** Five of the six tile labels
  are detected as their own `text` boxes; TRAINING's is not, because the selected tile
  renders its label without the others' dark bar. A text-first approach would have failed
  on exactly the control that blocks the workflow while succeeding on its five neighbours.
  The container was available when the text was not.
  [[Experiment-030-asking-the-learned-detector-for-the-tile]]
- **The two stacks now share one concept, and the workflow runs nine steps instead of one.**
  A candidate carries its source, the detector's class (carried, read by nothing that
  decides), its originating look, and the window and observation generations it was
  computed under. A detection is requested on a settle, at most one outstanding, admitted
  only while BOTH generations still hold, and held until one moves. Measured on
  `rl-freeplay-1`: prefix **1 of 21 → 9 of 21**, controls resolvable **12 of 13 → 13 of 13**,
  `target_2` **0 → 26** reacquisitions, **0 false matches** either way, ground truth
  unchanged, fold latency p95 **0.511ms**. It stops at step 9 because the recording ended.
  [[Experiment-031-the-workflow-gets-nine-steps-in]] · [[ADR-184-two-speeds-of-looking]]
- **A prefix is not an assessment, and the prefix was hiding a route that works.**
  Each learned step is now classified by WHY it went unexamined: verified, unresolved,
  not-reached, or no-evidence. With detections: **19 verified, 0 unresolved**, 2 not
  reached (the tail of route 1, where the recording ends). The second demonstrated route
  completes **10 of 10 from its own starting observation** — and did so pixel-only too,
  which the 1-of-21 prefix could never show because a walk cannot look past its own stop.
  [[Experiment-032-what-the-recording-can-and-cannot-say]]
- **The detector merged two identities into one control.** Pixel-only, the two presses on
  the TRAINING tile were learned as `target_2` (the stadium slab, unresolvable at 0 of
  62 looks) and `target_10` (resolvable at 5 of 62) — two identities for one control.
  With detections both steps are `target_2`, verified on both visits at 26 of 62, and all
  four repeated targets resolve at both of their steps.
  [[Experiment-032-what-the-recording-can-and-cannot-say]]
- **A screen is not an observation, and that distinction is load-bearing.** Leaving a
  screen and returning gives the same arrangement, so a delayed detection validated
  against the SCREEN would be reused on a frame nobody is looking at. An observation
  generation separates the two visits; 56 of 249 delivered candidates were refused by it
  on one recording, and 164 more expired while held.
  [[Experiment-031-the-workflow-gets-nine-steps-in]]
- **Matches moved into abstentions, which is the conservative direction.** A target whose
  identity is a detected rectangle is only re-acquirable on a look where a detection is
  held — 543 of 900 — so matched fell 674→593 and unobservable rose 450→537 while false
  matches stayed at 0. At a 4-second inference delay the integration refuses nearly
  everything and returns to exactly the pixel-only answer rather than a wrong one.
  [[Experiment-031-the-workflow-gets-nine-steps-in]]
- **No detector runs in any live path.** All of the above is offline, over recorded
  detections replayed at the measured 820ms latency. Director's live sampler still does
  not call the fast observer at all, and `stale-window` has never been exercised by
  evidence — only by a test.
- **There are two perception stacks and they barely touch.** The Director cycle
  (providers → `fusion.Engine` → Elements) already runs this detector at a frozen
  calibration and already holds every authority rule — `SourceRank`, `ActuatingSource`,
  evidence never destroyed, conflicts recorded rather than resolved. The fast observer has
  none of them, because its whole currency is `Look.Regions []observe.Region`: a rectangle
  with no record of where it came from. The fast observer now has a sourced candidate and
  the same authority vocabulary, but it still does not use `fusion.Engine`, and a
  `directorapi.Element` and a session-local rectangle remain different objects.
  [[ADR-184-two-speeds-of-looking]]
- **Recall is the fix, not suppression.** `Press` takes the smallest proposal containing a
  press, so a missing true candidate is what promotes a noise aggregate to “the control”.
  Propose the tile and the stadium slab stops mattering without being removed.
  [[Experiment-028-the-tile-that-was-never-a-candidate]]
- **A screen does not bind reliably across independent folds, and no threshold fixes it.**
  Two folds sharing no identities: 5 of 8 screens bound correctly, 2 bound confidently to
  screens the other fold had never seen, 1 ambiguous. The best wrong binding scored 0.886,
  above four of the five correct ones. The screen model has no abstention — `promote` takes
  the best above the floor — which is defensible session-locally and not for a durable
  binding. [[Experiment-029-binding-a-screen-across-two-independent-folds]]
- **One control still blocks the Rocket League workflow, and it is a PERCEPTION gap.** `target_2`
  on `visual_2` resolves on none of its looks because the TRAINING tile is never proposed: the
  only rectangle containing the press is a 778x230 slab of stadium background. No matching rule
  can admit a rectangle the proposer never offered, so the question is upstream — the proposer
  finds text and small glyphs and does not find large artwork tiles, which shows in the yields
  (90–153 proposals per look on `visual_2` against 400 on `visual_6`). `target_6` was the other
  one and is fixed. [[ADR-182-two-controls-that-could-never-be-found-again]]
- **Manual Rocket League sessions are no longer the default next step.** The owner's standing
  change: use the existing recordings, replay-based regression tests and synthetic edge cases
  first, and when live evidence is genuinely necessary, say precisely what is missing and why the
  corpus cannot provide it. [[ADR-181-a-demonstration-marco-can-keep]]
- **The fast observer exists and nothing in Director's live sampler calls it.** It is reached
  only by `director observe-screen`, which the owner starts. Wiring it into the existing
  observation cycle is a separate decision, and the cycle's own cadence is the one ADR-175 ruled
  out — so the wiring must give the observer its own clock or not happen.
  [[ADR-178-the-fast-observer]]
- **15 to 18 looks a second is this machine, this window, one recording.** Re-folding the same
  900 frames on 2026-10-12 gave 15.5 to 16.5 a second against the 18.3 first recorded — same
  code, same recording, load the only variable. The range is the finding; either end clears the
  ten the model needs. It is not a claim about other applications, other resolutions, or dialogs
  shorter than the ones measured. [[ADR-178-the-fast-observer]]
- **`observe-screen` does subscribe to live mouse events, and now its output can prove it.**
  `navsource`, a subscription drained once a look, `NavPoint` folded as presses. Until
  2026-10-12 a report of `0 click` had four causes in four places and three of them were
  invisible — including a press offered on a look with no geometry, which was discarded with no
  number anywhere. The command now reports the producer's own counts, the observer's press total
  and the unusable subset, the correlator's total, and keys seen-and-not-used; and it says `NOT
  OBSERVED` with the reason when no mouse could be watched, because an absence that is forced
  must not read as one that was observed. Over the recording: 17 offered, 0 unusable, 17 folded,
  0 blind, 5 ambiguous, against a recording known to hold seventeen.
  [[ADR-178-the-fast-observer]]
- **A full-length backpressure run is stronger than the first one.** 541 and 552 of 900 updates
  dropped, conclusions identical: nine screens, the same look counts and settle points, 13/5/3,
  all seventeen presses folded. A consumer seeing two looks in five arrived at the same session.
  [[ADR-178-the-fast-observer]]
- ~~The session-local screen model needs ≈10fps and the live path samples at ~3.8s.~~
  **Addressed by [[ADR-178-the-fast-observer]]**, which gives the model its own cadence rather
  than changing the rule. The original finding stands: Not a
  tuning problem: measured at every rate and phase, the current rule discards screens it
  sampled, and the rule that recovers them produces a promotion every eight looks at the
  recording's own rate. Click attribution goes to zero before the screens do. The way out is a
  fast local observer on its own cadence, and the proposer's 39ms a frame says that is
  affordable; where it gets its pixels is NOT answered — the existing capture costs ~130ms a
  frame, which caps a fast observer at about 7fps on its own.
  [[ADR-175-no-settling-rule-survives-a-four-second-cadence]]
- **Everything established about the repeated route exists only at ≈10fps.** Five screens of
  route correspondence at the recording's own rate, 2–3 at 300ms, 1–2 at 3.8s.
- **A session-local visual screen exists and nothing in the live path uses it.** The stabilised
  layer is built, measured and gated, and only the offline replay calls it. Wiring it into live
  observation is a separate decision from building it, and promoting one into a durable Place is
  a third. [[ADR-171-an-arrangement-is-an-observation-and-a-screen-is-an-interpretation]]
- ~~A press is attributed to the screen a fixed window later, not to the next transition.~~
  **Fixed 2026-10-12**, [[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]].
  The window was ten looks and the measured transition took thirty; attribution is to the next
  change on the screen the press was made on, bounded at fifty, with the gap reported.
- **Target identity from geometry alone does not survive two demonstrations.** Four rules
  measured: the smallest containing region splits one control across runs (12 identities, 3
  repeated units); the whole candidate set merges three visibly different menu entries (8, 4);
  both scale restrictions make one or the other worse. The conservative rule is in the tree and
  the pairs that might be one control are reported rather than merged. Settling them needs a
  third demonstration or a visual comparison of the candidate rectangles.
  [[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]]
- **Some steps of a real route are not caused by clicks.** Eight changes on the measured
  recording had no press before them, and they are not noise: entering Free Play is a LOAD
  completing, so whether a press falls inside the attribution window depends on how long the
  load took — the same change was attributed in one demonstration and not the other. A planner
  that assumed every edge needs a press would be wrong about this menu.
- **A session on the pixels discovery path segments no screens — and a MEASUREMENT now says the
  pixels could carry it.** Asked of arrangement alone, with production's own signature,
  similarity and threshold and one generic kind on every proposal, the 900-frame Rocket League
  recording separates into **seven** screens with **13 of 17 presses followed by a different
  one**, and the demonstrated route appears in both runs. Against Marco's actual `1 state, 0
  transitions`. The cue is also noisy — 150 changes between adjacent frames — so the work is
  damping rather than detection, and the durable half is still its own decision: an arrangement
  of rectangles is a much weaker identity to make permanent than a composition of roles.
  [[ADR-170-what-an-unnamed-arrangement-would-have-segmented]]
- **A session on the pixels discovery path segments no screens.** `StructuralView.Regions` comes
  from named detections and the structure proposer names nothing, so on the default discovery
  path the screen model is one vacuous state that any composition with no named roles matches.
  Selection moves are therefore grounded in the LIST that was independently seen to respond, with
  the screen honestly unknown — see [[ADR-159-a-move-of-the-mark-is-a-transition-too]]. Giving
  segmentation an unnamed-geometry cue would reach cross-session identity and durable memory, so
  it is its own decision rather than a widening.
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
