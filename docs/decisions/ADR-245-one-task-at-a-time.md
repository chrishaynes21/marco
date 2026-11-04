---
type: adr
status: accepted
date: 2026-10-20
decision: "ACCEPTED — Studio shows ONE task at a time. A rail of three places (Acts, Scenes, Run history) with one of them open, and a run that replaces whatever was being looked at while keeping Stop in front of the person. An Act's detail is what it will do before anything happens: the application, the screen it has to start on in the words read on that screen, and its interactions in order with what each one presses and what it expects afterwards. That ordered preview is the honest kind — it sends nothing — and the engineering run with input disabled is no longer offered as a rehearsal of an Act, because its first press never happens and it therefore cannot reach the second interaction. A run says what is happening in the words of the thing the person asked for (`Preparing Ark…`, `Step 1 of 2 · open the audio settings`, `Checking that it worked…`, `Paused before step 2 · <the run's own reason>`), and when it ends it says what happened AND what to do about it, keyed on the outcome and on what this surface already knows about the Act. Machine names never reach a person: a control is named by the words read on it, a screen by the words read on it, and a screen with no words is said to be unrecognisable rather than shown as `screen_a`. Run history is the canonical records the runner wrote, newest first, and Report a problem gathers one of them into a file the person can send — it sends nothing anywhere. There is no Resume button, because there is no pause/resume operation in the runner: a paused run re-evaluates and proceeds by itself, and the window says that instead. And the window RECORDS a demonstration, which it could not do before: it starts `director demonstrate-visual` with the application chosen from what is running, counts down its own `--delay`, ends it the recorder’s own way with a newline on its stdin so everything the recorder does after that line still happens, and then says what came of it from the saved artifact — with one remedy per way a capture can be useless, because those four causes have nothing to do with each other."
supersedes: []
affects:
  - interface
  - execution
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/learn.go
  - plugins/studio/acts.go
  - plugins/studio/doing.go
  - plugins/studio/history.go
  - plugins/studio/play.go
  - plugins/studio/said.go
---

# One task at a time

## The verdict this comes from

The window that preceded this was five numbered panels in two columns — demonstrations, compose,
Acts, perform, the run — all of it on screen at once. Everything needed to run an Act was
present and nothing said in what order to touch it. The owner, opening it for the first time:

> how do i use it, it's unintuitive. There's zero acts, I can't click into section one, there's
> eight saved and I have no clue what I'm looking at here

and then, about the one panel they did reach:

> Okay I kinda get it, except it's screen_a to screen_b how tf am I supposed to know what that
> means

The Architect's reading of the same screen: *"The existing screen makes the user assemble the
workflow mentally. Adding explanations to its five panels will not fix that."* That is the
decision. What follows is what replaced it, and what was deliberately left out.

## What a person does

A rail with three places, one open at a time.

**Acts** is what you have. Each one says its application, how many steps, and whether it is
ready — and `ready` is no longer the whole answer (below). Choosing one shows **what it will
do**: the application, the screen it has to start on, and its interactions in order, each one
saying what it presses, where, and what it expects to see afterwards. Play is under that, with
one sentence: this press authorises the Act once.

**Scenes** is what you have shown Marco, and the vocabulary is stated rather than implied: a
Scene is one interaction, an Act puts Scenes in order. An interaction Marco can perform is a
button; one it cannot is a sentence; a demonstration with nothing performable in it is one card
rather than thirteen rows that do nothing when pressed. Using one adds it to an ordered builder
with a name you can change, Move up, Move down and Remove — no positions, and no `2=name`.

**Run history** is every run this machine kept a record of, newest first, each with what it
meant and what to do about it. Selecting one offers **Report a problem**.

**A run** takes the main area the moment it starts and keeps Stop in the rail as well as on the
run screen, because somebody who has navigated away has not stopped caring.

## The four things this gets right that the old screen did not

### Machine names never reach a person

`control_2` and `screen_a` are names this machine gave itself. They cannot be recognised, cannot
be told apart from `control_1` and `screen_b`, and give no hint that one of them is the row of
tabs across the top of somebody's own settings page. So a control is named by the words READ on
it, a screen by the words read on it, and the artifact's own labels are not shown at all.

Two consequences worth stating. Fragments of one or two characters are dropped from a label
while there are longer words to show — reading a game's interface produces `od` and `ul` beside
`cosmetics` and `graphics` — but if nothing survives that, the short words are shown anyway: a
control whose only word is `OK` reads `OK`. And a screen with NO words is described as
unrecognisable, which is not a gap in the description but the warning in the next section.

### `ready` was true and insufficient

An Act starts on the screen its first interaction was demonstrated on, and that screen is
established by matching the words read on it. A screen saved without words cannot be positively
identified — the arrangement alone has been measured to confuse two screens of one menu — so a
run over it pauses, every look, until its deadline, and ends having pressed nothing.

That is exactly what happened in the owner's first live trial. The Act was listed as `ready`,
which was true: it preflights, and it would perform perfectly against a screen Marco could
recognise. The run paused correctly thirty-odd times, saying `screen_b was saved without any
words, so a reading cannot identify it`, and refused with 0 actions sent.

So an Act now carries both sentences, and the second one says where the missing thing is — in
the demonstration — and what to do about it: show it again, resting on that screen long enough
to be read. The same sentence is what a refused run offers as its next action, because the
remedy for that refusal is not another press of Play.

### The preview is honest, and the dry run is not a rehearsal

An Act's ordered preview sends nothing: it is the interactions, in order, with what each one
presses and expects, read out of the demonstration. That is what Play is pressed against.

A run with input disabled is a different thing and is no longer offered as a product gesture. It
runs the whole path and writes each press down instead of sending it — which means the first
press never happens, so the second interaction's screen never arrives, so a two-step Act cannot
be rehearsed by it at all. It stays, behind Technical details, labelled as what it is.

### A run speaks about the task

`doingOf` maps the runner's own account of itself onto the thing the person asked for, with one
message per situation a run can be in, and `nextOf` says what to do about each way one can end.
Both in Go rather than in the page, for the reason `said.go` already gives: a page with a switch
over these has no way of saying which situations it covers, and these have a test that walks all
of them.

## Learning a Scene

The window could list what had been demonstrated and could not record one — so the one thing
Marco is for was the one thing it could not do, and the way in was a terminal command with six
flags whose session ends when somebody presses ENTER at a prompt they cannot see. The owner:
*"how do i do a demonstration? And if not, isn't that something marco can do the UI can't?"*

It is `director demonstrate-visual` and it stays that. Studio starts it, says what it is doing,
ends it, and reads the store afterwards:

| | whose claim it is |
|---|---|
| bring the application forward | Studio's, and refused here when it cannot |
| the wait before recording | Studio's: it passed `--delay`, so it may count it |
| recording | the recorder's, until somebody says they are done |
| ending | a newline on its stdin — the recorder's own "press ENTER" |
| what came of it | the SAVED ARTIFACT, never a parsed sentence |

Three things in that are deliberate. The application is **chosen from what is running** rather
than typed, because the executable name is what every selector in Marco matches on and a list is
the difference between that and a guess. The recording is ended by **That's it** writing a
newline, not by a kill: everything the recorder does after that line — the report, the draft
learned from the walk, the artifact, the evidence file beside it — would be thrown away by a
window that killed the process and called it finishing. And the review is read back from the
store, because the artifact is the thing that will be performed.

**Four failures, four remedies.** A capture can be useless in four ways and they have nothing to
do with each other: nothing saved at all, a demonstration with no control in it (a menu driven
by the keyboard, and keys cannot name a control), a control with no outcome (the pause after the
click was too short — two readings are dropped as mid-response and five have to agree), and a
first press whose screen was never read (an Act over it waits instead of pressing). "Try again"
over those four would be the window guessing.

A recording and a run exclude each other. There is one screen: a recorder watching it while a
runner presses on it would be Marco demonstrating to itself, and the artifact would record
Marco's own clicks as the person's. Closing the window abandons a recording rather than keeping
half of one under the name somebody chose.

## The loop closes where it started

Learn → review → build → run, and the join between the second and the third used to be a sentence
telling somebody to go and find what they had just recorded in a list. The review now offers
**Build an Act from this**, which fills the builder with the interactions Marco can perform AND
check, in the order they were demonstrated.

The ones it cannot check are left out on purpose: an Act does not continue past an interaction
whose result was never recorded, so including one by default would be building an Act that stops
in the middle. They are still there to add by hand, where somebody choosing one can see what
they are choosing.

And the recording card now says the half of a demonstration that people skip — rest on the
screen you start from before you click. That is the reading that establishes the starting screen,
and without it the Act that comes out of the recording can only wait.

## What the first live trial refused for, and what it had to say about it

The owner built a one-interaction Act over a TrackMania demonstration and pressed Play twice.
Both runs refused in seven seconds, pressed nothing, and said:

```
what is on screen agrees with screen_b at 0.897 and nothing has been read on it yet,
so nothing corroborates it
```

The Act was right. The grant names one interaction at position 1, which starts on `screen_b`;
`screen_b` was saved with thirty-eight words on it; and the shape agreed at 0.897 and 0.935, so
they were standing on the screen their Act starts on. Nothing was pressed because
[[ADR-244-pressing-play-is-the-authorisation]] identifies that screen by its WORDS — there is
nobody mid-run to ask — and no reading of the live screen was ever current.

Two things were wrong with that, and neither of them was the Act.

**The run gave up in four seconds.** `workflowrun.DefaultPatience` is forty observations, which
is four seconds at the fast loop's cadence, and both runs spent exactly forty looks on one
unchanging sentence before stopping. Four seconds is the right budget for a control
mid-animation. It is the wrong budget for a screen identified through a source that answers about
twice a second and has a share of its answers thrown away as stale: measured on the owner's
machine during this work, OCR took a mean of 499ms, 163 of 203 frames were dropped while it was
busy, and 21 of the 38 readings that came back were refused because the observation had moved on.
Eight chances, most of them spent. So Studio now passes `--patience 150` whenever it passes
`--reading`: fifteen seconds, which is about thirty answers from the source the decision rests
on. Nothing is pressed while it waits, the pause says what it is waiting for, Stop is on screen
throughout, and the pause resolves by itself the moment the evidence arrives — waiting is the
cheapest thing this run can do.

**And the sentence was one sentence over three situations.** "Nothing has been read on it yet" is
true when OCR has not answered yet, when every answer arrives about a screen that has already
changed, and when the pixels were never the application's because the window was not in front.
The remedies are: wait; hold still, because waiting will not fix it; and click the game's own
window. The run said the first of those for four seconds while any of the three could have been
true, so the person watching could not act and could only conclude their Act was wrong.

`whyNoReading` in `cmd/director/observationloop.go` now answers which, and
`workflowstore.Standing` says it in place of the default. The order is the order of what a person
can do about it: a look that was not evidence first, because it is the one they can fix in a
click and it also explains the other two.

And the owner’s other report of that trial — that they could tell which screen was being asked
for “but not easily” — is the same problem one level up: the pause names `screen_b`, which is a
name this machine gave itself while folding looks together. The words read off that screen are
the only part of its identity a person can compare against what is in front of them, so the run
screen now carries them beside the reason. Beside, not substituted into it: the sentence belongs
to the runner and this surface does not edit it.

### And the first version of that diagnosis overstated its evidence

It counted every stale reading in the run and, from one of them, said the screen kept moving and
that waiting would not help. Two things were wrong with it, both reported by the Architect and
both reproduced here.

One discarded reading was counted again on every look that followed it. A dropped reading keeps
its stamps, so `observe.Current` goes on being false of it; a stationary screen waiting for OCR
therefore accumulated a stale reading ten times a second, and the number that came out said the
screen was moving when nothing was. `looker.held` counts the drop once.

And a total cannot tell a screen that MOVED from a screen that KEEPS moving, which is the whole
distinction: one transition discards whatever was being held and the screen then sits still,
which waiting fixes, and leaves its total standing for the rest of the run. So the strong
sentence now needs `staleSustained` — three — discarded in a row with none admitted between
them, and any reading that IS admitted resets the run, because the screen demonstrably held
still long enough to be read once. Below that the sentence says what is actually known: *an
earlier reading became outdated, and the screen in front of you is being read now.*

The sequence that gets this right is the one that was wrong before, and it is a test:
transition, thirteen looks of a perfectly stationary screen, then the late reading landing and
identifying it. One discard, not fourteen; and never the claim that waiting would not help,
because waiting was the entire remedy and it worked.


What this does NOT claim: the trial is not re-run, so which of the three the owner's refusal
actually was is still unknown. That is the point — the next run will say, in the window, without
a reproduction.

## The recorder says what it has, so nobody counts seconds

The advice that came out of the first trial was "wait two seconds after the click". That is an
estimate of how long seven OCR readings take, handed to the person least able to make it. The
Architect's words:

> The recorder knows whether it obtained evidence; the person should not have to estimate seven
> OCR readings.

`demonstrate-visual` said everything at the two ends and nothing in between, so the window could
only report a clock: *recording, 54s left*, plus a paragraph of advice. `internal/director/learnstate`
is the recording's own account of itself — one JSON object per CHANGE, the same shape `runstate`
has for a run and for the same reason — and `--progress <path>` is where it goes.

What it reports is what will actually decide whether the demonstration is worth anything.
`observe.LocalOutcome.Verified` is the same four terms `workflow.Learn` uses to decide whether an
interaction can be checked at all, and `Why` is the same sentence the end report gives. So the
window shows the thing that decides, half a second earlier, instead of an estimate:

| what the recorder knows | what the window says |
| --- | --- |
| watching, nothing read on the screen it started on | Reading the starting screen… rest where you are for a moment. |
| that screen has been read, no press yet | Click the control once, with the mouse. |
| a press, placed, outcome in flight | Checking what changed… wait for the result to settle. |
| a press that landed on nothing the proposer found | That press landed on nothing Marco could see. Click the control itself. |
| a press with a verified outcome | Result captured. Click the next control, or finish. |

A press that landed on nothing gets its own phase because waiting cannot fix it, and `Checking`
is a sentence that says waiting is all there is to do. An outcome in flight outranks it, because
a session can hold one of each — a click beside a control and then one on it — and the phase is
about the press being measured now. The blind presses are still counted and shown.

**And finishing is checked once.** The first press of *That's it* over something still in flight
refuses and says what, in the recorder's words; the person then presses **Keep recording** or
**Finish anyway**. Both are one click. The second press finishes whatever the state, because it
is their demonstration, the evidence may never arrive, and a window that would not stop recording
is worse than one that recorded nothing useful. A question that answers itself — somebody who
pressed a moment too early and then waited — is taken down by the next poll rather than needing
an answer.

What this does NOT do is make the two problems one. A screen that cannot be recognised and an
outcome that was never recorded are different faults with different fixes, and the owner's
TrackMania Act had both: nothing was pressed because the starting screen could not be identified
in four seconds, and once it is pressed the Act will still report that it could not confirm
anything, because neither of its demonstrated interactions has a recorded outcome. The Act detail
has said so in orange since it was built. These are separate lines on separate screens on
purpose.

### Readiness is asked in the run’s own words, not in weaker ones

The first version of that stream asked `observe.ScreenText.Read` about the screen a recording
started on — *did anybody look* — which is true of a frame OCR read and found nothing on. The
review beside it asked `len(On) > 0`, and `On` is a SAMPLE of six words at most for a person to
read. Both say yes about a screen the run will then refuse, and the person finds that out by
pressing Play: told the demonstration was fine, told at Play that the screen could not be
identified.

So the question is asked once, in `observe.ScreenText.Identifies`, which is the term
[[ADR-244-pressing-play-is-the-authorisation]]’s own `Standing` applies: words were read, and
there are at least `TextDisagreementWords` of them, because two screens that have said two words
each have not said enough for *this is that one* to be a claim about anything. Three places ask
it now — the run, the recorder while somebody can still fix it, and the review — and a weaker
version of it in any one of them is how a demonstration is presented as ready and then waits
forever.

Two more things about that screen were wrong, and both told somebody to do something that could
not work:

| what it did | what it does |
| --- | --- |
| judged `Walk()[0]`, where the SESSION began | judges the screen the first press that can become a step was made on — which becomes step 1, so its screen is what a granted run has to recognise. Before any such press, the screen standing now. |
| counted outcome WATCHES as the presses being checked | counts `Presses - Blind`. A watch is opened for a blind press too, so `watched` equalled `presses` always, `presses - watched` was always zero, and the `Blind` phase was unreachable — a person clicking BESIDE a control was told to wait for a result that could never arrive. |

**And a captured result does not excuse a screen nothing can name.** The phase reached
`captured`, the window said *result captured — click the next control, or finish*, and nothing
anywhere mentioned that an Act built from it would pause on its starting screen. They are two
faults with two remedies, so the recording carries both: `learnstate.Event.StartKnown` beside the
phase, a caution on the card for as long as it holds — because going back to that screen and
resting on it is the only moment it can be repaired — and one refusal at *That’s it* that names
everything missing rather than the first thing.


### “Wait for the result to settle”, about a result that had settled

The owner recorded `enter freeplay` on Rocket League and asked the question this section is the
answer to:

> it got stuck on settling for like 30s and I just had to finish the recording rather than
> waiting. Settling shouldn’t take 5 seconds right?

The recording’s own stream has the whole of it. Their first press captured 7.8 seconds after
the click, across two restarts. Their second was clicked at 10:16:59.46; 10:17:02.15 is the last
thing the recorder ever had to say about it, and the window it was reading can have taken at
most until 10:17:05 to fill — two readings skipped, five restarts at the allowance’s limit, five
readings to fill, at 480ms each. The card said *Checking what changed… wait for the result to
settle* until 10:17:31, when they gave up and clicked something else. **At least twenty-six of
those seconds were spent waiting for a reading that had already been taken.**

Nothing was wrong with the measurement, and the floor is real: two readings after a press are
dropped as mid-response and five have to stand, which on that session’s own cadence — the
stream’s reading-by-reading ticks are 480ms apart, OCR beside a running game — is a little over
three seconds, and a menu that redraws inside the window restarts it. So **four seconds is the
design and twenty-nine was not settling at all.** It was a finished negative answer with no way
to say so.

`LocalOutcome.Verified` is false in two completely different situations, and nothing
distinguished them:

| | |
| --- | --- |
| the readings are still arriving | waiting is exactly the thing to do, and the only honest sentence is how many have arrived |
| the readings have arrived and few of them stood | the watch takes no more, so the verdict cannot improve, and asking for patience is asking for something with nothing on the other end of it |

So an outcome now says whether it is **the answer** or the answer so far —
`LocalOutcome.Final`, which is the window having filled or a third screen having closed it, the
two ways `observeOutcome` stops folding readings into a watch. The second of those needed
saying out loud: `Settled` answers *did the window fill* and a watch a third screen closed
never fills and never will.

**And the card had never shown how far along a wait was.** The reading count is the one number
that makes waiting legible instead of a guess — the recorder has it, this surface exists so that
nobody has to estimate seven OCR readings, and it was only ever shown to somebody who pressed
*That’s it* too early. `Missing` is now on the card for as long as a recording is running, which
during `checking` is *3 of the 5 reading(s) persistence needs have arrived*. That sentence also
had to be written: it used to say the window had been *cut short*, which is a thing that happened
to a window rather than a thing that has not happened yet.

The recorder reads it as a third counter beside the captures, and `learnstate.Unchecked` is its
phase — its own, for the reason `Blind` is its own: waiting cannot fix it and `Checking` is a
sentence that says waiting is all there is to do. It is said in preference to `Captured` when
both are true, because *result captured — click the next control* over a press that produced
nothing is the window telling somebody their click was banked when it was not, and the count
rides beside the phase for the reason `StartKnown` does. Rocket League’s main menu carries a
rotating news panel and live counters; its words change by themselves and few of them stand,
which [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]] measured and which is
why the terms are what they are. A recorder’s job there is to say so, not to wait. The recording
itself, with the cadence and the timings read off its own stream, is
[[Experiment-095-thirty-seconds-of-settling-that-had-already-settled]].

**And one more press was being waited on that nothing was measuring.** `Watched` was
`Presses - Blind`, and `observe.MaxOutcomeWatches` caps how many presses accumulate evidence —
so past the cap the arithmetic claims a press is being measured that has no watch, and
`Captured + Unchecked` can never reach `Watched`. The phase would then be `Checking` for the
rest of the recording. It counts the watches that exist now, which cannot say that.


### The press that could never be checked, and was reported as one to wait for

The owner re-recorded and came back with the same complaint and a sharper number:

> Three presses seen, only one worked. Settling is ass, it doesn’t finish in time. How fast
> were you seeing screens settle, because for me it’s taking like 20-30s if at all.

Their stream says it was not settling at all. Press 2 captured in **5.6 seconds** across one
restart — the same cadence as before, readings 0.4–0.5s apart. Presses 1 and 3 never moved off
one sentence, for seventeen seconds and then fourteen:

> nothing was read on one side of the press, so the question was not asked

That is `BeforeLooks == 0`, and it is permanent. `watch` takes the before side ONCE, at the
instant of the press, from the readings already held of the screen it was made on; nothing
afterwards appends to it, because `observeOutcome` only fills `after`. So a press made on a
screen nothing has been read on has no before side for the rest of the session. There is
nothing to compare what appears against, and the question `Verified` answers cannot be asked
at all — not answered no, not answered later.

The saved artifact names the screen: `screen_b`, **0 words**. A Trackmania menu they passed
through too quickly for a single reading to be taken on it.

So `Final` now covers it, which stops the recorder describing it as a press to wait for. But
reporting it better does not make more presses work, and this is the one fault on this surface
that can be **prevented** rather than reported:

| | |
| --- | --- |
| `StartKnown` | `Identifies` — three words, enough to recognise that screen again at run time |
| `ScreenRead` | `Said` — anything at all, because the before side of an outcome is READINGS |

Two thresholds for two jobs, and the phases that asked the first one asked it only of the
screen a recording STARTS on and stopped at the first press. The second screen and the third
were never mentioned. `learnstate.Event.ScreenRead` is asked of the screen in front at every
look, and the card carries a caution for as long as it is false and a press has been seen:
*Marco has not read this screen yet. Rest on it for a moment before you click.*

### A console window, taking the foreground off the game

> Every time I click record, a new shell window is also launched.

`director` is a console executable, so Studio starting one created a console window — and a new
console window takes the foreground. `internal/bridgehost` found and fixed exactly this in the
Phase-0 live acceptance; Studio’s three spawn sites never got the same treatment. The black
rectangle is the lesser half: a recording has just raised the person’s game, and a console
appearing a moment later is in front of it, so the recorder photographs a covered window and
reads nothing on the screen about to be clicked.

### The window was taking its own pictures down twice a second

> Also the images blink in the ui

Both polls run at 400ms for as long as the window is open, and both rebuilt their whole subtree
every time. That is invisible for text and not for a picture: the finished review cleared its
`.shots` boxes, asked the backend for three thumbnails per step again, and filled them in when
they came back.

Measured on the page itself, with a headless browser and the bindings stubbed, over about ten
polls:

| | before | after |
| --- | --- | --- |
| picture fetches, finished review open | 66 | 6 |
| Scene-library rebuilds, same | 12 | 2 |
| picture fetches, run in flight | 108 | 9 |
| pictures the RUN screen was showing | 0 | 9 |

The last row is a second defect the first one hid. `stepsHTML` builds both the Act’s preview and
the run’s step list, both are in the document at once, and it gave their picture boxes the SAME
element ids — so `$('actShots0')` answered with whichever came first, which is the preview. The
run had been repainting the preview’s boxes and leaving its own empty.

Each list now draws once per revision and moves the mark without rebuilding, pictures are cached
by revision and label, and a box already showing exactly those pictures is left alone.

**This needed a gate a source guard cannot be.** Every other page test here greps the source,
which is the right shape for *does the page ask for X* and the wrong shape for a count over
time: inverting `if (sig === shownReview)` to `if (false)` leaves every one of those guards
passing, because the variable is still named in the file. A mutation run proved it, and the
answer is `redraw_test.go` — the real page, rendered in headless Edge with `window.go` stubbed,
counting what it asks for. It skips where there is no browser.


### A press that opened something nothing can read

> entered a map, marco couldn’t tell that we opened a map and never settled

The recording has it exactly: `screen_a`, 20 words over 8 looks — the menu; then `screen_b`,
**0 words over 5 looks**, and `screen_c`, **0 words**. The press IS recorded, as a click-caused
transition with a control. What could not happen is the check: the outcome rule wants five
independent words that appear and STAY, and live 3D has none — what OCR finds are glyphs off a
moving world, gone by the next reading.

The verdict it gave was *everything that appeared was transient*, which is true about the words
and silent about what happened. So an outcome carries the screen the press LED TO, and when that
is a different screen the sentence says so: *that press opened a different screen and only N
word(s) could be read on it that stayed — a screen with nothing readable on it, like a map or a
video, cannot be checked by what it says.* `Appeared` alone cannot tell the two apart; it is
empty both when a still screen produced nothing and when a map did.

**What is NOT done is admitting it as verified.** A wordless destination passing the rule would
let an Act chain past a press nothing confirmed, and the nesting in
[[ADR-243-a-door-of-its-own-and-the-order-in-the-file]] exists precisely to stop that. So a press
that enters a map can be demonstrated and cannot anchor an Act, which is a limit worth saying out
loud rather than a bug.

### The recorder was a day old, and nothing said so

The same recording carried a contradiction: `start_known=1` beside `screen_read=0`, which cannot
happen — `Identifies` implies `Said`. The only way to see both is a recorder that has never
heard of the second field.

Studio spawns the `director.exe` beside ITS OWN executable, and there were two copies of Studio
in the tree with a director each. The owner launches the one in `plugins/studio`, whose director
was from the previous night — so a day of recorder fixes was never in effect, every binary looked
freshly built, and the symptom was the old sentences coming back. Nothing in the window says
which recorder it is using, and the contradiction in two fields is what gave it away.

Three things stacked, and only the third is a defect. `plugins/studio` is where `go build`
drops the binary and the repo root is where binaries live beside `marco.exe` — two legitimate
homes, neither canonical, so the tree holds two of everything with independent ages. The lookup
answers WHERE and never WHICH. And the handshake that exists for exactly this was not used:
`learnstate.Version` sat at 1 while three fields were added to `Event`, and `omitempty` makes a
writer that has never heard of `screen_read` byte-identical to a current one whose value is
false.

So the version moves with the shape by TEST rather than by memory — a table of version to field
set, failing until both move together. It pins the PAIR: a first version that pinned only the
shape was survived by a mutation putting `Version` back to 1.

And a stream this build cannot vouch for is no longer silence. `Latest` returns `ErrForeign`
when a file holds lines and none of them parse at this version, which is a different thing from
a recorder that has not spoken yet; the two leave a reader with the same nothing and want
opposite answers — wait a moment, against stop and look at which binary is running. Studio says
the second where the phase sentence goes, and *That’s it* says it too rather than refusing for
want of evidence it cannot read.

`plugins/studio/build.cmd` makes all four and puts both binaries in both directories, so they
cannot drift in the first place. The guard on it reads the script's COMMANDS and not its prose:
the first version searched the whole file, found `go build -o director.exe` inside the comment
explaining why that line is there, and survived a mutation commenting the real one out.

What is still only discipline is a bare `go build` leaving a stale sibling. The difference is
that it is now loud within one recording instead of silent for a day.

**And the binaries say what they are, because git cannot.** Git identifies the SOURCE; it says
nothing about the executable somebody double-clicked, and the correction is worth writing down
because the first answer offered here was "commit, and git will tell you what you are running".
It will not. So `internal/buildid` reads the revision the toolchain already stamps, and it is
carried where somebody can see it: on every event of the recorder's stream, on every run
record, and at the top of `studio -acts` beside the path of the director it would spawn. The
window shows its own revision and the recorder's side by side, and says so when they differ —
both values, never a yes/no, because *they differ* is what somebody acts on and the two
revisions are what tell them which to rebuild.

Two things about that are deliberate. `Same` and `Differ` are not each other's negation: an
unstamped binary cannot be vouched for AND cannot be seen to disagree, and conflating those
turns every pair that carries no stamp into an alarm about a difference nobody observed — which
is how it was caught, with a test binary reporting one process as two builds. And the director
is built from the ROOT module by `build.cmd`, because `go build` stamps a revision only into a
package in the main module: built from inside `plugins/studio` it produced a director that could
not say what it was, which is the one thing it is there to do.

### Being rid of things, which this surface could not do at all

It could list demonstrations, Acts and runs and remove none of them, so a machine kept
everything anybody had ever tried on it. The owner asked for the small version and then said
what they had meant:

> I want to be able to start fresh with the testing whenever I’d like

Three sizes, one promise: the first press removes nothing and says what would go, the second
does it. One implementation of that promise, not one per kind — a mutation changing the second
press to ask again SURVIVED while the run path kept its own copy, which is the whole argument
for the shared helper in two lines.

- **A Scene** takes its artifact, its pictures and its evidence — and every Act composed from
  it, because an Act IS that demonstration’s interactions by name: its preflight rebuilds the
  play from the bindings and the artifact, so one whose demonstration has gone says `ready` and
  cannot run. The question NAMES them. Refusing while an Act existed would make a Scene
  undeletable until somebody hunted down what depended on it; taking them silently would delete
  work nobody mentioned.
- **An Act** goes on its own, leaving what was shown Marco where it is — and takes its
  bindings with it, or the name stays unusable: `demonstrated.Save` refuses a name whose
  `.scenes.json` is still there, so somebody deletes an Act, composes it again, and is told
  there is already one called that.
- **Everything**, behind a disclosure at the bottom of Run history, because doing it one card
  at a time is a chore that grows with how much testing has been done. Hotkeys are kept: a key
  bound to a name works again the moment that name is demonstrated again, so throwing it away
  is a thing to redo rather than a thing to clean up.

Two things that were not deletions and were found by writing them. `ForgetSequence` left the
EVIDENCE behind — filed beside the artifact and read by passing the artifact, so once that is
gone nothing can match it: a record of somebody’s own desktop that nothing reaches and nothing
cleans up. And there was no way to remove a play at all, so `demonstrated.Forget` is now the
mirror of `Save`, in the same file, because three files are one play and a remover living
somewhere else would eventually remove two of them.

### And the pictures had nowhere to be seen

> the pictures are gone so i guess you technically did fix the blinking pictures

They were not gone. The store had all three and the backend served them — 302KB, 310KB, 118KB.
What was gone was anywhere to see them: the post-recording review lives in the window’s memory
and goes when it restarts, an Act’s preview needs an Act to exist, and the Scene library — the
permanent home of a demonstration — had never shown a picture at all. So a demonstration made
yesterday had pictures on disk and nothing that would show them.

Each card carries a strip of its SCREENS now. The screens rather than every label its steps
name: the same screen is the `to` of one step and the `from` of the next, so walking the steps
asks for one picture three times and shows it three times.

### Run history is a thing you can clear

The surface could list runs and not remove one, so a machine accumulated them — 443 on the
owner’s, against the 25 the window shows. The directory was the archive and nothing could reach
into it but a file manager.

Deleting is now offered per run and for all of them, and three things about it are deliberate:

- **It asks first, and the first press removes nothing.** A run record is the only account of
  what Marco did to somebody’s machine and the one thing `Report` gathers from, so the thing
  that deletes must never be the thing somebody clicked first. The same two steps *That’s it*
  uses for the other irreversible thing this window offers, and one idiom for both is one thing
  to learn. The page binds the second press to the reference the question was asked ABOUT, not
  to whatever is chosen by the time it is pressed, and leaving the page puts the question away.
- **The count is the directory’s, not the list’s.** The window shows `historyShown`; a count
  taken from what is on screen would tell somebody they were deleting twenty-five when they were
  deleting four hundred.
- **A reference is matched, never joined into a path.** It arrives from the page, and
  `filepath.Join(dir, reference + ".json")` with `../../routes/os` is a deletion outside the
  store. `Forget` reads the directory and compares NAMES, so a reference naming something
  outside it matches nothing by construction rather than by a check that has to be right.

A run in flight is refused — its record is being written while this reads the directory, and
removing it would leave a live child writing to a path nothing lists. A problem report already
written is left where it is: that file is the person’s, and deleting the run it describes is not
a reason to take back something they may be about to attach to an email.


## Pictures, because words are what RECOGNISES a screen and not what a person recognises

The requirement said so plainly and this surface spent two rounds substituting anyway:

> Visual review needs retained target/start/result previews; OCR vocabulary is not an adequate
> substitute.

The owner's own report of their first trial is the same sentence from the other end: they could
tell which screen was being asked for "but not easily", reading
`controls · display · profile · sound · video` off their Act and matching it against a game menu
by eye. So a review now shows the control it will press, the screen it starts from and the screen
it expects afterwards, as pictures — and the Act's ordered preview and the run screen show them
too. The words stay beside them, because the words are what identifies a screen at RUN time and
a picture is what identifies it to a person; neither replaces the other.

**What is kept.** One thumbnail per screen at 640px, and one crop per control that was pressed,
at 360px with a quarter of the control's own size of surroundings on each side — a label with
nothing around it could be any row of any list. `internal/director/preview` is the whole of the
arithmetic, in the stdlib, and it averages when it shrinks rather than sampling: a one-pixel
stroke scaled by picking one source pixel per destination pixel vanishes outright, so the word is
still there and a person cannot read it, which defeats the entire point.

**Where they come from.** The screen thumbnail is the FIRST settled look of each screen — the
screen as it was arrived at, not as it looked half way into the next transition. The control crop
is taken from the full-resolution frame of the look the press actually arrived on, which the
correlator's own `Observations` identify by look number. Both are bounded in the session:
twelve screens and eight press frames, because a full frame is eight megabytes of RGBA and a
feature for looking at pictures must not be the reason a demonstration runs out of memory.

**And nothing that runs reads one.** No runner, grounder, verifier or recogniser consults a
picture; no decision anywhere depends on one; deleting every file leaves every Act performing
exactly as before. That is also what keeps [[ADR-181-a-demonstration-marco-can-keep]] and [[ADR-183-remembering-recognising-and-being-allowed-to-act]] intact — a
pixel is the strongest coordinate there is, and a preview is cropped FROM a region the artifact
already recorded, stored beside it under the artifact's own label, and never becomes a thing to
press. The artifact still carries no image.

### The promise had to change, and it changed where it is made

`demonstrate-visual` printed "No frames and no screenshots are kept", and that was true. Keeping
crops of somebody's desktop is a change to what a recording DOES with their screen, and the same
situation arose once before: that line used to also say "no press coordinates" and stopped being
true when the raw demonstration began keeping every press with its position.

So it is opt-in at the command — `--previews`, off by default, and the announce block says
exactly what is kept when it is on — and Studio passes it and says so **on the card where it asks
to record**, before anybody presses Record: one picture of each screen and of each control you
click, cropped from that window, in this machine's own Marco folder, read by nothing that runs,
never sent anywhere, and removed when the Scene is. The owner was asked before this was built and
said to build it.

Removal is not a loose end: `ForgetSequence` removes a routine's pictures before it removes the
routine, and refuses the whole thing if it cannot. Crops of a desktop must not outlive the thing
they were crops of, and since nothing reads them they would never be noticed again.

### A picture belongs to a revision, not to a name

Reported against the first version of this, and it is the review misrepresenting what Play would
perform — the one job a review has:

> Pictures are stored by routine name and label, without a revision. `SaveSequence` permits
> replacing the same routine name, but replacement doesn’t clear or invalidate its previous
> pictures.

Record *the audio tab* with pictures, record it again without them, and nothing in the preview
path runs at all: nothing is written, so nothing in the writing of one could clear what is there.
The review then shows the FIRST recording’s starting screen beside the SECOND one’s words, steps
and controls — of a demonstration that may have started somewhere else entirely.

The precedent was already in the same directory. `Evidence` is filed beside its artifact under the
same name and the same `SavedAt`, and it is read by passing the ARTIFACT, so the link is checked
on every read rather than assumed; the reason its own doc gives is that a name alone would match a
routine that has since been re-demonstrated. Pictures are exactly that, so they are kept under
`previews/<routine>/<revision>/<label>.png` and read the same way. A mismatch cannot be compared
wrongly because there is nothing to compare: another revision’s picture is somewhere else.

The removal is separate, and it is hygiene rather than honesty. `SaveSequence` deletes the
pictures of every other revision before it writes — on EVERY save, which is the case the
name-keyed version got wrong — because crops of somebody’s desktop under a name whose
demonstration has moved on are read by nothing and cleaned up by nothing. An artifact with no
instant has no revision, can have no pictures of its own, and is not the recording any kept
picture came from, so its save takes them all with it rather than being refused: deciding whether
somebody’s routine gets saved is not in this feature’s gift.

### What the pictures are NOT shown for

A picture is not evidence and is not consulted. The one place that distinction could blur is the
review's verdict: "Marco saw your click and could not confirm what changed" is decided by
`observe.LocalOutcome`'s four terms over readings, exactly as before, and a Scene with a
beautiful picture of its result and no recorded outcome is still not presented as ready.


## What was asked for and deliberately NOT built

- **A Resume button.** There is no pause/resume operation in the runner. A paused run
  re-evaluates its world every look and proceeds by itself the moment the world matches, so a
  Resume button would be decorative — it would claim an operation that does not exist. The
  window says what the run is waiting for and that it goes on by itself, and keeps Stop.
- **A Scene with an identity of its own.** What is recorded is a DEMONSTRATION, whose
  interactions an Act then names; a reusable named Scene with a stable reference of its own does
  not exist yet, and the window does not pretend otherwise — it calls the recording step "Learn a
  Scene" because that is what a person is doing, and everything it then shows is the
  demonstration it actually saved.
- **Scene reuse across demonstrations.** An Act performs interactions from ONE demonstration.
  The builder says so where somebody would otherwise discover it from a refusal.

  What it would take, so the next person does not have to find out: `Binding` already carries its
  own `Routine` and `Revision` per interaction, and the generated play names the demonstration
  only in a comment — so the FORMAT is nearly there. What is not is everything below it.
  `Bindings.one()` requires a single routine, `Plan` carries one `Routine`, and
  `perform-workflow` builds one `workflowrun.Runner` and one `workflowstore.SessionBinding` from
  that one artifact, with the artifact threaded through the confirm closure, the regrounder, the
  step agreement, the reacquisition and the scope gate. Two demonstrations means one coordinator
  per routine, each establishing its OWN starting screen from the same looks — because
  `screen_a` in one artifact is not `screen_a` in the other, even when it is the same pixels —
  and that is a restructuring of the command's composition root, which is where every execution
  guard is installed.

  It is also not the cheapest way to a two-step Act. One recording with two clicks in it
  composes today, and costs the person one demonstration instead of two.
- **Visual previews.** The target, the starting screen and the result are described in words
  read off them, not shown as images; the artifact retains no such pictures to show. Words are
  what there is, and they are better than labels — they are not a substitute for a picture.
- **Sending a problem report anywhere.** This window reads the owner's own store. A button that
  posted their run — the application, the words read off their screen, the reference and the
  authority — to a service would be a surface deciding to publish somebody's desktop. So Report
  a problem gathers the record, what the run said, and the Act as it stands into one file, and
  says plainly that nothing has been sent.

## Enforced by

- `plugins/studio/learn_test.go` —
  `TestRecordingAsksMarcosOwnRecorderAndFinishesItProperly` (the arguments the recorder was really
  given, the preparation sentence, and that it was FINISHED on its own terms rather than killed);
  `TestARunAndARecordingDoNotOverlap`; `TestRecordingRefusesWhatItCanAnswerFirst` (five cases,
  including a demonstration named after an existing Act, which is a compile error two steps
  later); `TestTheApplicationsOfferedAreTheOnesWithAWindow` (one entry per application, and
  neither a minimized window nor a message-only one); `TestEveryWayACaptureCanBeUselessHasItsOwnRemedy`
  (the four causes and their four remedies); `TestARecorderThatSavedNothingSaysSoAndSaysWhy`;
  `TestWhatWasLearnedIsReadBackFromTheStore` (the review comes from the artifact).
- `plugins/studio/doing_test.go` — `TestEverySituationARunCanBeInIsSaidAsATask` (one case per row
  of the specified table, including the two where nobody could say which step it was on, and the
  ended case that deliberately says nothing because `saidOf` is the headline then);
  `TestEveryEndingSaysWhatToDoAboutIt` (all eight outcomes, and that a refusal whose cause this
  surface already knows offers THAT remedy rather than the generic one).
- `plugins/studio/history_test.go` — `TestTheRunsThisMachineKeptAreListedNewestFirst` (order by
  the run's own start time, the fields read back, what it meant and what to do, the authority, a
  record with no ending marked as unfinished rather than failed, and a record that will not parse
  still listed); `TestAStoreWithNoRunsIsAnEmptyHistoryAndNotAFailure`;
  `TestReportingAProblemWritesTheRunAndTheActIntoOneFile` (the reference, the record, the
  transcript, the Act, and that it says nothing was sent); `TestAProblemWithNoRunToReportIsRefused`.
- `plugins/studio/words_test.go` — `TestAnInteractionIsDescribedByWhatWasReadOnTheScreen`;
  `TestTheOnlyWordsThereAreGetShown` (the length rule, and the `OK` case it must not swallow);
  `TestAnActThatCouldOnlyWaitSaysSoBeforeItIsRun`;
  `TestAnActPreviewsWhatItWillDoInOrder` (the ordered preview, what each step expects, and where
  the Act has to start); `TestAnActCannotBeCalledWhatOneOfItsInteractionsIsCalled`.
- `plugins/studio/play_test.go` — `TestThePageIsTheFlowThatWasAskedFor`, twenty source guards
  over the page: the rail, the empty library, the vocabulary, the ordered preview and the marked
  step, what is happening and what to do, Stop in the rail, run history, Report a problem, the
  words on controls and screens, the waiting warning, and the engineering run labelled as not a
  rehearsal.
- `cmd/director/admitreading_test.go` — `TestAReadingOfThisObservationIsWhatTheScreenSays`,
  `TestAReadingThatArrivesAboutAnOlderScreenSaysWaitingWillNotHelp`,
  `TestALookThatWasNotEvidenceSaysThatAndNotThatNothingWasRead`,
  `TestNoReadingYetSaysNothingMoreThanThat`,
  `TestAHeldReadingStopsStandingWhenTheScreenChanges`. These enter `looker.admit` through the
  production call with nothing faked but the readings, and assert what the GROUNDER receives,
  because `looker.next` holds a window tracker, a desktop capture and a window provider and
  cannot be stood up at all.
- `cmd/director/whynoreading_test.go` — `TestEveryWayAScreenCanGoUnreadHasItsOwnSentence` (one
  case per situation, and that the one a person can act on comes first);
  `TestAScreenThatKeepsMovingSaysWaitingWillNotHelp`.
- `internal/director/workflowstore/standing_test.go` —
  `TestARefusalForWantOfWordsSaysWhyThereAreNone` (the reason it was given, and the plain one
  when it was given none).
- `plugins/studio/acts_test.go` — `TestARunThatMustReadTheScreenIsGivenTimeToReadIt` (the
  argument the child was really given, and that it is more than the runner's own four seconds);
  `TestARunWithNoReaderKeepsTheRunnersOwnPatience`.
- `plugins/studio/play_test.go` — `TestThePageIsTheFlowThatWasAskedFor` also guards
  `pausedWhere`, so a paused run cannot go back to naming its screen only by the label this
  machine gave it.
- `internal/director/learnstate/learnstate_test.go` —
  `TestEverySituationARecordingCanBeInIsAPhase` (ten cases, including the blind press and the
  two where an outcome in flight outranks one); `TestEveryPhaseIsSaidAsSomethingThePersonIsDoing`
  (every phase has a sentence, none of them is its own phase word, and the four the redesign
  named are the four that are said); `TestARecordingSaysWhatChangedAndNotWhatDidNot` (four events
  for four distinct states out of six calls, and the seconds remaining deliberately not counted
  as a change); `TestARecordingThatEndedIsFinal`; `TestNoPathIsAWriterThatDoesNothing`;
  `TestHalfALineIsSkippedAndTheRestIsRead`.
- `plugins/studio/learn_test.go` — `TestFinishingEarlySaysWhatIsStillMissing` (one case per
  phase, that the reason is the recorder's own and not this window's guess, that both answers are
  offered, and that saying it again finishes it); `TestTheRecordingShowsWhatTheRecorderKnows` (the
  preparation is still this window's clock, and everything after it is the stream).
- `cmd/director/admitreading_test.go` also holds
  `TestAScreenThatMovedOnceAndThenHeldStillIsNotAScreenThatKeepsMoving` — the transition, the
  thirteen stationary looks, and the late reading that lands — and
- `internal/director/preview/preview_test.go` —
  `TestAScreenIsKeptWholeAndSmaller`; `TestASmallWindowIsNotEnlarged`;
  `TestAControlIsCroppedFromWhereItWas` (a bright patch in a dark frame, so the crop says where
  it was taken from); `TestAControlAtTheEdgeIsStillCropped` (four regions that leave the frame,
  which is how a resized window becomes a panic mid-save);
  `TestTwoReadingsOfOneControlAreCroppedTogether`; `TestAControlWithNoRegionIsRefused`;
  `TestAPreviewIsStoredAsAPictureAnythingCanRead`;
  `TestShrinkingKeepsWhatWasThereRatherThanSamplingPastIt` (the averaging, measured on
  one-pixel strokes).
- `cmd/director/keeppreviews_test.go` —
  `TestEachPictureIsFiledUnderTheLabelOfWhatItIsAPictureOf` (each screen its own colour and each
  control its own patch, with the screen thumbnail a DIFFERENT picture from the press frame, so a
  crop taken from the wrong one shows it); `TestAPressOnNothingIsNotGivenAPicture`;
  `TestASessionWithNoFramesKeepsNoPictures`; `TestASessionHoldsAtMostWhatItIsBoundedTo` (both
  bounds, and that the first thumbnail of a screen stands).
- `cmd/director/observescreenwiring_test.go` — `TestAPixelDemonstrationIsReachableAndWritesOnlyWhereItShould`
  also asserts the loop keeps no picture of a window that was not in front. Over the source, for
  the reason it already states about frontness: window capture returns the covering window's
  pixels under the target's name, so that picture would be of something else, and a mutation that
  dropped the check survived every behavioural test in the package.
- `internal/director/demo/previews_test.go` — `TestAPictureIsKeptUnderTheLabelTheArtifactUses`
  (including two routines that both have a `screen_a`); `TestForgettingARoutineForgetsItsPictures`
  (the files AND the directory, every revision of it);
  `TestPicturesOfOneRevisionAreNeverServedForAnother` (the read side alone, with nothing cleared
  and nothing saved — which is what makes the review honest even if the removal failed);
  `TestARerecordedRoutineNeverShowsTheOldRecordingsPictures` (both halves of the report: replaced
  with NO pictures, where nothing in the preview path runs at all, and replaced with only SOME,
  plus that an artifact with no instant is still saved); `TestAPictureNeedsSomethingToBelongTo`.
- `cmd/director/recordingnow_test.go` — `TestTheRecorderCountsOnlyThePressesThatCouldBecomeSteps`
  (a blind press is not a press being checked, so the phase that says waiting cannot help is
  reachable at all); `TestTheRecorderAsksWhatTheRunWillAskOfTheStartingScreen` (nothing read, two
  words, three words — and `Read` and `Said` are both true of the middle one);
  `TestTheStartingScreenIsTheOneAnActWouldStartOn` (with the screen STANDING read well, so a
  recording that judges the wrong screen calls the demonstration fine — a fixture where that
  screen happened to be unread was satisfied by the wrong answer, and the first one was);
  `TestACapturedResultDoesNotHideAnUnnameableStartingScreen`;
  `TestTheProgressStreamIsWrittenFromWhatTheRecorderKnows` (over the source: the stream is this
  reading, and the loop says it AFTER folding the look).
- `internal/director/observe/screentext_test.go` —
  `TestAScreenNeedsEnoughWordsToBeRecognisedByThem` (the one predicate three places ask).
- `internal/director/workflowstore/standing_test.go` —
  `TestAScreenSavedWithTooFewWordsCannotCarryAnActsPressEither` (one word, read live as the same
  screen plus more: the look agrees, nothing contradicts anything, and it is still not an
  identity). A mutation replacing `Identifies` with `Said` survived every test in that package
  before this existed.
- `plugins/studio/acts_test.go` —
  `TestAnInteractionSaysWhetherTheRunCouldNameTheScreenItStartsOn` (a screen with words to SHOW
  and no identity, which is the case the review used to pass).
- `plugins/studio/previews_test.go` — `TestAPictureIsServedForTheLabelAnInteractionCarries` (the
  join: the labels an interaction carries are the labels the pictures are filed under, asserted
  here because a mismatch would simply render no picture and fail nothing);
  `TestADemonstrationWithNoPicturesShowsItsWordsInstead`;
  `TestAReDemonstratedSceneShowsNoPicturesFromTheRecordingBeforeIt` (through the window, because
  this is the surface that decides which revision to ask for);
  `TestStudioAsksTheRecorderForPictures` (read off the arguments the child was really given,
  because a window that promised pictures and did not ask for them would be making a promise
  about somebody's desktop that it then did not keep).
  `TestAScreenThatNeverHoldsStillSaysSoOnceItHasBeenSeenToThreeTimes`.
- `plugins/studio/page_test.go` — `TestThePageBeginsAsADocument`, which is why a text node in
  front of the doctype cannot come back.
- And the window itself: `studio.exe -seconds N -evidence <file>` renders the page and the page
  reports back LAST, after setting itself up, so the evidence line is proof that the script ran
  rather than only that it parsed.
- `internal/director/observe/outcomefinal_test.go` —
  `TestAnOutcomeStillAccumulatingReadingsIsNotTheAnswer` (and that its sentence is about what
  has not arrived rather than about a window somebody cut short);
  `TestAWindowAThirdScreenClosedIsFinalWithoutHavingFilled` (the half `Settled` cannot express:
  never filled, never will); `TestAScreenOfChurnReachesAnAnswerAndNotAnOpenQuestion` (the
  owner’s own case — the answer is no, and the point is that it is an answer).
- `internal/director/learnstate/learnstate_test.go` —
  `TestEverySituationARecordingCanBeInIsAPhase` gained a measured-and-empty press, the owner’s
  one-of-each shape, and a press still in flight beside one, which outranks both.
- `cmd/director/recordingnow_test.go` —
  `TestAPressWhoseResultIsMeasuredAndAbsentIsNotAPressToWaitFor`;
  `TestARecordingDoesNotWaitOnAPressNothingIsMeasuring` (the watch cap).
- `plugins/studio/learn_test.go` — `TestFinishingEarlySaysWhatIsStillMissing` gained “the
  result is in and it is nothing”, which is refused and does NOT ask for more recording;
  `TestTheRecordingShowsWhatTheRecorderKnows` carries the count and asserts the sentence says
  waiting is over.
- `plugins/studio/play_test.go` — the page guards on `l.unchecked`, on `l.missing` and on the
  `learnWhy` line it paints into. The markup guard alone was not enough: a mutation deleting the
  painter left the suite green, so the element existed and stayed empty.

- `internal/director/observe/outcomefinal_test.go` —
  `TestAPressOnAScreenNothingHasBeenReadOnIsAnsweredAtOnce`, over a window deliberately left
  SHORT: a full one is final by `Settled` whatever this term says, and the first version of the
  test was satisfied by the mutation for exactly that reason.
- `cmd/director/recordingnow_test.go` —
  `TestTheRecorderSaysWhetherTheScreenInFrontHasBeenReadAtAll` (nothing, one word, three words —
  and that it is the screen IN FRONT rather than the one an Act starts on, which are the same
  screen only until the first press).
- `plugins/studio/console_test.go` — `TestNoChildDirectorIsStartedWithAConsole`, which walks
  the package for spawn sites rather than naming today’s three;
  `TestAHiddenConsoleStillHandsBackWhatTheChildSaid`, because the claim that CREATE_NO_WINDOW
  keeps the inherited handles is a claim about a Windows constant made in a comment, and Studio
  reads the recorder’s whole transcript off those pipes.
- `plugins/studio/redraw_test.go` —
  `TestTheWindowDoesNotFetchTheSamePicturesOverAndOver` (a finished review sitting open asks
  once); `TestTheWindowKeepsPicturesItIsAlreadyShowing` (choosing the same Act again takes
  nothing down, counted with a MutationObserver over the containers rather than the boxes — a
  box destroyed with its parent is one the observer stops hearing from, which is the case it is
  looking for; and both lists show their own six pictures).

- `plugins/studio/history_test.go` — `TestDeletingARunAsksBeforeItDoesIt` (the first press
  removes nothing and the question names what would go);
  `TestDeletingEveryRunCountsTheDirectoryAndNotTheList` (with more kept than listed, which is
  the only arrangement that can tell the two counts apart);
  `TestARunReferenceCannotReachOutsideTheRunsDirectory` (four aimed references, and a real file
  one directory up that has to survive); `TestARunInFlightCannotBeDeleted` (and that a DIFFERENT
  past run is still somebody’s to delete while one is running);
  `TestDeletingARunThatIsNotThereSaysSo`.
- `plugins/studio/play_test.go` — the page guards on `acts.Forget(what, false)` and
  `acts.Forget(what, true)`, which are what catch the two inversions that matter: deleting on
  the first press, and never deleting on the second.

- `plugins/studio/forget_test.go` — `TestDeletingASceneTakesTheActsBuiltFromIt` (and that its
  evidence goes, over a fixture that WRITES one: the store had none, so the first version of
  that assertion passed whether or not anything removed it, which a mutation proved);
  `TestDeletingAnActLeavesTheDemonstrationAlone` (and that the name is usable again, which it is
  not if the bindings were left); `TestStartingFreshRemovesEverythingAndSaysWhatThatIs` (over a
  hotkey file written by the test, for the same reason);
  `TestNothingIsDeletedWhileSomethingIsRunning`.
- `plugins/studio/page_test.go` — `TestThePageBeginsAsADocument` now also refuses a NUL byte in
  the page. It arrived three times in one sitting the same way: a separator meant as the four
  characters of an escape, written by a patch that interpreted it — and a page with one in it
  is a page every tool treats as binary.

- `internal/director/observe/outcomefinal_test.go` —
  `TestAPressThatOpenedAnUnreadableScreenSaysThatIsWhatHappened`, which also holds the other
  side: a press the screen sat still through is still described by its words, and must not be
  swallowed by the sentence about opening something.
- `plugins/studio/buildscript_test.go` — `TestTheBuildScriptMakesTheWholeStackFromOneCommit` (since 2026-11-04; it replaced the director-and-marco build-script test in `console_test.go` and keeps its requirements).
- `internal/director/learnstate/learnstate_test.go` —
  `TestTheStreamVersionMovesWithTheEventsShape` (the pair, and that no two versions share a
  shape, which would be two versions nothing can tell apart);
  `TestAStreamFromAnotherBuildIsNotAStreamThatHasSaidNothing`.
- `plugins/studio/learn_test.go` — `TestARecorderFromAnotherBuildIsSaidRatherThanWaitedOn`
  (and that finishing says it too); `TestTheWindowSaysWhichBuildIsRecordingBesideItsOwn`, which
  drives a genuine mismatch — a test binary carries no stamp, so both sides are supplied: the
  window's through a substitutable source and the recorder's by writing the line a recorder
  would.
- `internal/buildid/buildid_test.go` — `TestARevisionIsShortOrHonestlyUnknown`;
  `TestABuildThatCannotSayWhatItIsIsNeverVouchedFor`; `TestCannotTellIsNotADisagreement`.
- `internal/director/learnstate/learnstate_test.go` — `TestAStreamSaysWhichBuildWroteIt`,
  stamped by the Writer as `Name` and `Application` are, so no event can be written without it.

## Related

[[ADR-244-pressing-play-is-the-authorisation]] is the contract this is a surface for.
[[ADR-243-a-door-of-its-own-and-the-order-in-the-file]] is the Act itself.
