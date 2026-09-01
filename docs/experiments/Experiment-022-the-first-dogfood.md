---
type: experiment
status: partial
findings: 8
date: 2026-08-29
backend:
  - production-perception
  - semantic-memory
fixture: cmd/marco/observefeed.go
result: two wiring defects behind one silence - ambient learning could not keep the name of a list item so nothing in Settings was learnable and the page never said why
supersedes: []
source_paths:
  - cmd/marco/observefeed.go
  - cmd/director/learningfeed.go
  - cmd/director/learnsessionwiring.go
  - cmd/marco/watchui.go
  - cmd/director/observelook.go
  - cmd/director/observeledger.go
  - cmd/director/ambientnaming_test.go
  - cmd/director/observewiring.go
  - cmd/director/ambientlabel_test.go
  - cmd/director/experiment.go
  - cmd/director/experiment_test.go
---

# Experiment 022 — the first dogfood

## Question

> Can Marco watch normal computer use, learn useful topology, say what it learned, survive a
> restart, and then use that knowledge?

## Method

Isolated `$MARCO_HOME`, cold store, real Director, the real product commands: `marco observe
learn`, `marco observe --follow`, `marco learn`. The real user's home was hashed before and after
and is unchanged.

**One honest limit on the method, stated up front.** Navigation was driven by `ms-settings:` URI
activation and window management, not by a hand on a mouse. That was the only way to drive it from
here, and it turns out to matter — see below.

## What the feed does

Built for this session, because nothing surfaced durable change. It reports what
`semanticmemory.Store` committed, after the write succeeded, and nothing else:

```
Watching. I'll tell you when I learn something you can use.

  + learned place       Mouse
  + named               Mouse
```

With learning off it says so rather than sitting blank. Through the whole session it produced
**zero false events** — including through everything below, where Marco genuinely learned nothing
and correctly said nothing.

## Finding 1 — you could not teach Marco while Marco was watching

The headline loop, from a cold home, doing what the product invites:

```
marco observe learn      → Marco is watching, learning from what it sees
marco learn "…"          → phase: refused
                           refused: no_observation
                           "I couldn't watch — I lost sight of that window."
```

Every time. With watching **off**, the identical command reached `ready_for_demo` and established
a Place. So it is a slot conflict, not the window — and the message points at the window, which
sends somebody to look for a fault that is not there.

One observation runs at a time by design. Ambient watching held it; Learn was refused. **Watch me,
then teach me could not be walked from a command line at all.**

Fixed as a `DOGFOOD_BLOCKER_FIX`, because everything after it was unreachable. Background
attention now yields to a demonstration, which is the rule Light Mode already followed. See
[[ADR-111-a-demonstration-takes-the-slot-from-watching]].

**And the first attempt at the fix did not work**, which is worth as much as the fix. It went into
`Runtime.Learn` — reached only by the control surface — so `marco learn` went on being refused
exactly as before. The rule was right and nothing on the path a person types called it. Fourth
occurrence in this series.

After the fix, measured: `ready_for_demo`, one Place committed, the feed said
`+ learned place Mouse`, and `marco observe status` still reported *"on a screen it knows"* — the
slot came back.

## Finding 2 — the observation half needs a hand on the mouse

Seven Settings pages visited, 180 samples, 10 ambient sessions, 10 transitions recorded — and
**zero candidate edges**. The cause is in `noticed`:

```go
if len(s.Did) != 1 {
    return
}
```

A transition becomes candidate evidence only when exactly one human ACTION is attributed to it.
Actions come from real input events, stamped with the screen state that was current when they were
banked. Driving navigation by URI presses nothing, so from Marco's point of view the screen changed
on its own — and it correctly learned nothing.

**That is the mechanism working, and this method cannot exercise it.** Learning "this control leads
there" requires seeing the control pressed. The remaining phases — edges, goals, execution,
composition, different starting places, midway entry, recovery — all sit behind that, so they were
not reached.

## Finding 3 — two status surfaces disagree

At the same moment:

```
marco observe status              →  noticed 0 screens and 8 moves so far
marco observe status --evidence   →  Marco hasn't watched you go anywhere yet
```

Both true under their own meanings — the first counts the transient buffer, the second the
candidate ledger — and together they read as a contradiction. A person cannot tell from either
that what is missing is an attributed action.

## What was verified

| | |
|---|---|
| feed reports committed knowledge | yes — `learned place`, `named` |
| false learning events | **0** |
| Place established from a cold store | yes, `subj_71727a02470f` |
| survives a Director restart | yes |
| recognised after restart | yes — `recognised subj_71727a02470f` |
| watching resumes after a demonstration | yes |
| edges / goals / execution / composition | **not reached** |

## What a human session needs to do next

Turn on `marco observe learn --follow`, then navigate Windows Settings **by clicking**: Home →
Bluetooth & devices → Mouse, and Home → System → Display. Watch for `+ learned way`. Then
`marco learn` a destination, watch for `+ learned destination`, and immediately try it from
somewhere else.

Everything up to that point is now verified. Everything after it is unmeasured.

## Correction — the method was two consoles, and that was the finding

Everything above was measured out of **two PowerShell windows**: one holding the Director, one
printing `marco observe --follow`. The instructions written at the end of it told a person to open
both.

That is engineering scaffolding, not the product. Somebody alt-tabbing between a Director console
and an Observe console to work out whether their assistant learned something is testing the
plumbing — and the finding is not about the feed, which was correct, but about where it was made to
live.

The loop now lands on the control centre's **Here** panel, which already answered "what can Marco
see and does it recognise it". It gained the three things it was missing: whether Marco may
**remember** what it is watching, what it has just **committed**, and a **Try it** that runs the
words through the same door a typed `marco do` uses. See
[[ADR-112-the-loop-belongs-where-a-person-is-already-looking]].

**One command now, not two windows:** `marco ui`, then open Here. The page starts the Director
itself when a button that turns something on is pressed.

Nothing about what the feed reports changed. It is still the store's own committed writes, still
four words held apart, still zero false events.

### And the wiring gate that was missing

`serveMux` did not exist: the mux was built inline in `runEdit`, which binds a port and then blocks,
so no test could ask what the control centre actually serves. Three endpoints could have been
written, called from the page, and reached by nothing — the fourth occurrence of that shape on this
branch, caught this time before it happened rather than after. Six mutations were run against the
new code and all six were killed, including deleting the registration call outright.

## What a human session needs to do next — revised

Run `marco ui`, open **Here**, press **Watch and learn**. Then navigate Windows Settings **by
clicking**: Home → Bluetooth & devices → Mouse, and Home → System → Display. Watch the JUST LEARNED
box for `learned way`. Then press **Try it**.

The hand on the mouse is still required and still cannot be scripted from here — see Finding 2,
which is unchanged.

## Finding 4 — nothing on the watching path can name a Place

Raised from dogfood as "naming looks worse than it used to". Investigated read-only, changing
nothing. **It is not a 37K regression, and it is worse than one.**

### 37K did not change what gets named

`AdmittedPlaceName` became `ExplainPlaceName` with the verdict discarded. Compared branch by
branch against `cf67be1^`: every `continue` is still a `continue`, every bail-out still returns no
name, and `NameLevel` never gates admission — `Admitted: true` is set regardless of level and
`out.Name = found` is reached in exactly the old cases. 37K added *explanation*, not rejection.

[[Experiment-020-what-does-this-screen-say-it-is]] measured the rule **working** on a wide page:
`DESTINATION "Printers & scanners"` at 1500px, Mouse correct at 1500/1000/850. Nothing after 37K
touched the rule.

### And yet the real store holds no names at all

The production `semantic-memory.json`, read without writing:

| | |
|---|---|
| subjects | 21 |
| `applicationframehost` (Windows Settings) places | 45 records |
| carrying `called` (the Audience's word) | **0** |
| carrying `semantic` (what Marco worked out) | **0** |

Both fields are `omitempty`, so that is real absence rather than an encoding artefact.

### Why: the rule is measured on a path production does not use

`director name-probe` calls `placeNameEvidence(world)` → `ExplainPlaceName` on ONE collection.
Production feeds the same producer into a **recurrence gate** and then writes from one of two
places, neither of which ambient watching reaches:

```
placeNameEvidence(world) -> AdmittedPlaceName -> sem.PlaceName
   -> ScreenState.PlaceNames[n]++
   -> settledPlaceName: needs top >= StatePromotionCount (2), and no tie
   -> written by ONE of:
        observesession/runner.go  PlaceNamesToRecord   only inside an explicit Learn
        cmd/director/observepromote.go  shape.Called    only over a promoted WALK
```

The walk path needs candidate edges, and Finding 2 above measured **zero** of those, because an
edge needs exactly one attributed human action. So during plain *Watch and learn* — the mode the
control centre now invites somebody to turn on — **no Place can receive a name by any route.**

That is why JUST LEARNED says `learned place` and then shows a subject hash. The feed is telling
the truth: a Place really was established, and it really has no name.

### What is NOT established

Whether an earlier build DID name these places. That needs a store from before this branch, and
there is not one — the "regression" half of the report is unproven and may be a long-standing gap
that only became visible once a surface started showing it. The naming rule working in
[[Experiment-020-what-does-this-screen-say-it-is]] and the store holding zero names are both
measured; the claim that it used to be better is not.

## Finding 5 — Observe and Learn are exposed as peers and should be a containment

Product feedback from the same session. The architecture separates observation permission from
durable-learning permission, and the control centre exposes that distinction directly — two
switches a person must understand to operate.

They are not peers to the person using them. `Learn` without `Watch` is meaningless, so the
relationship is containment:

```
WATCH
  └── LEARN
```

Pressing Learn should ensure Watch; stopping Watch necessarily ends learning; Learn may be stopped
while watching continues. The surface should say what Marco is DOING — *watching, but not saving
new knowledge* / *learning places and connections as you use your computer* — rather than name the
two permissions and ask somebody to compose them.

Deferred to 38A.1 rather than patched here, so it lands as one considered correction over a cluster
of dogfood failures instead of a reaction to each.

## Resolution — 38A.1, and the cause was one thing wearing two faces

Findings 4 and 5 were reported as two annoyances. They are one defect and one presentation of it,
and both are fixed in [[ADR-113-learning-is-inside-watching]].

### The wire, not the rule

Finding 4's cause is now proven rather than inferred, by a test that starts where a person starts:
`Runtime.EnableAmbientLearning` — the exact call the control centre's **Watch & Learn** button
makes — then one supervisor reading, then the assertion made against the semantic-memory file
reopened from disk.

```
before   AdmittedPlaceName → PlaceNames[n]++ → settledPlaceName → (nothing)
after    AdmittedPlaceName → PlaceNames[n]++ → settledPlaceName
                           → PlaceNamesToRecord → promotion.call → ObserveSemanticName
```

The sweep runs on every ambient reading, above the established-place early return in `ambientLook`,
because an already-established Place is exactly the case that had no path. It needs no walk, no
edge, no goal, no explicit Learn and no second Place — which is what Finding 4 measured as
impossible.

**Settlement is unchanged.** `director name-probe` still reads one sample and production still
requires recurrence and still refuses a tie; a screen that said two different things equally often
is still refused, and there is a production-path test that says so. The naming rule itself was not
touched, and the known-wrong cases (collapsed Settings, Printers at 850px) remain known wrong.

The "used to be better" half of the original report stays **unproven** and is now unfalsifiable
without a pre-branch store. What is established is that the rule works when called and that nothing
on the ambient path called it.

### Watching, and the thing you can do while watching

Finding 5 is now the product model. Three states, one status line each:

```
○ Not watching            Marco is not looking at your screen.
● Watching                Marco can see what you are doing, and is not saving anything new.
● Watching & learning     Marco can see what you are doing, and may write down the places it
                          works out.
```

`DisableAmbient` clears the learning policy with it, so `learning: yes, watching: no` is
unreachable; if a Director ever reports it anyway the strip says the state is inconsistent rather
than drawing it as a mode somebody chose. Measured live against a Director in an isolated home:

| from | verb | watching | learning |
|---|---|---|---|
| not watching | Watch & Learn | yes | yes |
| watching and learning | Stop learning | yes | no |
| watching | Stop watching | no | no |
| watching and learning | Stop watching | no | no |

The Here view also lost its **second** Watch button. It had two — the ambient strip's and Light
Mode's — both starting observation, both called Watch, reporting different state. That is most of
why Finding 5 read as confusion about architecture: it was partly confusion about which switch.

### And the JUST LEARNED suspicion was the same defect

The feed was truthful the whole time. `learned place` followed by a subject hash was a Place that
genuinely had no name. With the wire in place the same event is now `named Mouse`, announced after
the durable write and once — `ObserveSemanticName` is idempotent, so a sweep on every reading says
nothing after the first.

### Method: an isolated home from here on

Subsequent 38A sessions run against `%LOCALAPPDATA%\marco-dogfood` via `dogfood.cmd`, so the graph
can be inspected and reset as often as an experiment needs. Ordinary production behaviour inside
it; the isolation is one environment variable and `director reset-test-state` refuses any home that
is the default one.

The real store changed during the dogfood — `a21dcd43` → `0d4a0cf2` → `addfc612` → `d914bf65`,
the last on a clean shutdown. That was Marco doing what it was asked to do, and it was left alone.

### Still needing human hands

Everything above the desktop is proven deterministically and the state machine is proven live. What
is not: a person opening Windows Settings, pressing **Watch & Learn**, and walking
`Home → Bluetooth & devices → Mouse` with a real mouse. Foreground activation and injected-input
attribution are both refused by design, so the acquisition half of this experiment cannot be
scripted — the same limit recorded after every live milestone on this branch.

## Finding 6 — a clean human traversal under Watch & Learn produced nothing, and the reason was one boolean

**Reproduced, live, by a person, 2026-08-30, in the isolated dogfood home.** Watch & Learn on;
`Home → Bluetooth & devices → Mouse` walked twice with real clicks in Windows Settings. The
control centre said nothing: no learned, no goal, no Try it, no reason.

The store held the whole transaction, and every part of it was right except one field:

```
subjects: []
watched:
  Home → Bluetooth & devices    activate  list_item  target:""  seen 4
  Bluetooth & devices → Mouse   activate  list_item  target:""  seen 2
  Mouse → …                     activate  list_item  target:""  seen 1
```

Place names settled correctly on both ends. Attribution fired on every move. `director status`
said `Learning: yes`, `16 moves noticed`, `7 relationships seen`, `0 remembered`. The Director's
own evidence read named the cause in one sentence, three times:

```
 x activate in applicationframehost
     traversed 4 times
     I couldn't read what the control was called, so I can't say what to press
```

### The chain

| | |
|---|---|
| `AdmittedTargetLabel` | `list_item` is not on the plaintext role allowlist, and `demonstration` was false |
| `Actionable.Label` | `""` — withheld at the push |
| `SemanticTarget` | a resolved press with a role and no name |
| `Act.Representable()` | false |
| `ambient.Judge` | `Never / control_not_named` — structurally unpromotable, forever |

Ambient sessions start with the zero episode, so `liveSampler.nameActivatedTargets` was false.
Windows Settings navigates entirely by list items. **Ambient learning could not promote anything
in Settings, ever** — and the same holds for any interface that navigates by list items, tree
items, links or rows.

`ambientPromotionLicence()` has returned `LearnLicence()` — which holds `NameActivatedTargets` —
since the day it was written. The permission was declared and perception was never told. The sixth
wiring defect of this shape on this branch.

### Two things were wrong, not one

The blocker is the boolean. The **product** failure is that the page was silent about it: the
Director had an actionable sentence and said it only to a terminal. Both are fixed in
[[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]] — the label licence follows
ambient learning, read live rather than copied at session start, and the Here panel renders the
Director's own verdicts under **WHAT MARCO MADE OF IT**.

### And the goal gap was real, not a bug

Ambient learning admits topology and creates no goal — deliberately, and unchanged. So even a
perfect traversal leaves nothing to Try. The canonical way to close that gap is the retrospective
Learn (`/api/learn/recent`), which **existed and had never been called by the page**. It is now
offered as *Name what I just did*, and its commit lands in JUST LEARNED as `learned goal`, from
which Try it follows.

### Method note

Before this run the dogfood home was cold. Candidates recorded under the old behaviour keep their
empty control name permanently — the name is part of the candidate's handle and folding never
fills one in — so `dogfood.cmd reset` precedes the next run. The real store was read only and is
unchanged at `d914bf65`.

### Still needing human hands

The label crossing, the graph write, the feed announcement and the refusal sentence are all held by
deterministic production-path tests. The live traversal is not scriptable: foreground activation
and injected-input attribution are both refused by design.

## Finding 7 — Marco had no legible sense of an active experiment, and expected to be staged

Reported after the third dogfood session. Marco observed well and discovered routes; a person
watching it could not answer what it was focused on, what it was about to try, why, what starting
place it needed, whether it was waiting for them, whether it worked, or whether it had given their
computer back. Every observation, discovery, attempt and state change competed for one space.

Two halves, and the second is the sharper one: **if Marco decides to try something, getting the
desktop into the required starting state is part of running the experiment.** It was leaving the
person to arrange the stage, and afterwards leaving them standing wherever the experiment ended.

### The audit came first, and most of the behaviour already existed

| the brief asked for | already there |
|---|---|
| plan from here to a target place | `observe.PlanToGoal` |
| foreground the required application | `bringForward`, inside every `PerformGoal` since Phase 0 |
| verify the source before acting | `freshLook` + `planningProof` |
| verify the destination | `confirmArrival` |
| authority, actuation lease, recovery | `beginPerformance`, the walker, `carryOn` |
| stoppable, visible, refuses a second command | the command registry |
| **snapshot and restore the desktop** | **nothing** |
| **test ONE learned edge** | **nothing** — `Rehearse` is grant-scoped, `PerformGoal` is goal-scoped |
| **one legible focus** | **nothing** |

So [[ADR-115-one-experiment-and-the-desktop-given-back]] adds three things and reuses the rest:
choosing the experiment, requiring the source before the action, and giving the desktop back.

### What is now on the primary surface

```
NOW              what Marco sees
READY TO TEST    Bluetooth & devices — open Mouse → Mouse
                 I watched you open Mouse from Bluetooth & devices 4 times.
                 I have not tried it myself.
                 [ Test what I learned ]   [ Stop ]
LAST RESULT      what Marco wrote down            [ Go there ]
▸ EVERYTHING MARCO HAS NOTICED
```

`Test what I learned` and `Go there` are two acts and two buttons: one proves a connection and
gives the desktop back, the other accomplishes what somebody asked for and leaves them there.

### Recorded rather than done

Foregrounding is `winctx.Activate` — a Director platform call, not a Marco act. That is
pre-existing and shared with every performance; making it legal Marco is a change to the execution
substrate and was deliberately left alone. The brief asked for exactly that report rather than a
quiet redesign.

The attempt's steps render when it finishes rather than streaming while it runs.

### Still needing human hands

Every refusal path, the choice of experiment, the reason, the restoration and the stop are held by
deterministic tests, with eleven mutations run and killed. What is not scriptable is the part
where Marco actually walks: the five live acceptances need a person to press Test from the wrong
application and watch what happens to their desktop.

## Finding 8 — watching was pointed at an executable, and Windows gave it two windows

A person turned Watch & Learn on and walked `Home → Bluetooth & devices → Mouse` **three times**.
Marco noticed nothing, for twenty minutes, and said nothing at all.

The Director's own session report had it, unread:

```
State: target_unavailable
Target: application applicationframehost
Samples: 0   skipped: 39
applicationframehost has more than one window ... select one by --window-id (ambiguous)
```

`currentWindowSelector` built `Selector{Application: winctx.Active()}`. This desktop had **Settings
and Realtek Audio Console open at once**, both hosted in one `applicationframehost` on one PID, so
the selector was ambiguous, every reading was skipped, and — because *starting* the session
succeeded — nothing anywhere reported a failure. Twenty seconds later the supervisor did it again.

Perception was never the problem: a session pointed at the same window by title read it
immediately, 8 samples, 31 controls, pointer resolving to named controls. This also explains why
the 00:42 dogfood worked and this one did not — the audio console was not open then.

Fixed in [[ADR-116-watching-follows-the-window-not-the-executable]]: the foreground is now a
primary selector, and ambient watching uses it. The window in front is what ambient watching is
*about*; naming its executable was a different question with a different answer.

### And the twenty-second phantom, found by instrumenting rather than by asking

While chasing it, a poller on an **untouched** desktop showed `transitions` climbing 1 → 16 in five
minutes, one every ten polls of a two-second poller — metronomic, exactly `ambientSession`. A
screen Marco does not recognise is keyed on `session:state`, so every rollover renamed the same
screen and recorded a crossing from it to itself.

The first fix was wrong in an instructive way. It suppressed the crossing but kept the OLD key, so
the *second* reading of the new session compared against the previous session and crossed anyway —
the phantom delayed by one reading, not removed. It survived both a live measurement (7 of 12
rollovers still crossed, which I briefly mis-attributed to page repaint) and a test that read once
per session. A stray duplicated line in that test is what exposed it.

```
before   12 rollovers → 7 transitions on an untouched desktop
after     5 rollovers → 0 transitions, 114 samples, 132 seconds
```

### Method note

Three of the four wrong turns in this investigation came from reading a field that does not mean
what it looks like. `AmbientView.Application` is only written on a reading that produced a *place*,
so "it says chrome" is not "it is watching chrome". The measurements that settled it were the
Director's own session report and a poller, not inference from status.
