---
type: subsystem
status: active
owners:
  - director
depends_on:
  - programs
used_by:
  - execution
  - action-graph
updated: 2026-08-06
source_paths:
  - internal/director/marcoexec
  - internal/platform/marcorunner
  - internal/osmod/os.marco
  - internal/director/rehearse
  - internal/uiamod/accessibility.marco
  - internal/production
  - internal/platform/theaterhost
---

# Marco Boundary

The single point where the Director affects the desktop. Each planned step is **lowered to
legal Marco source**, which then goes through lexer → parser → graph → compile → runtime like
any route a person demonstrated.

There is no second path. The former `internal/platform/marcohost` adapter was deleted rather
than kept as a fallback, and a boundary test asserts no duplicate implementation exists.

## Responsibilities

- Lower a semantic step to Marco source.
- Encode values safely — note that `strconv.Quote` is *wrong* here, and why, in
  [[director-marco-boundary]].
- Compile **before** mutation, so a malformed program cannot half-apply.
- Get values back out of a completed run.
- Preserve status rather than collapsing everything into FAILED.

## What it may not do

- Call `Host.Invoke` directly.
- Express an action the language cannot. If Marco has no word for it, the capability is
  declared in `os.marco` or `uia.marco` first.

## Writing down what Marco learned

`marcoexec.LowerPlay` turns a VERIFIED procedure into an ordinary `.marco` program: `use os.`, one
actor, one verb, one flat list of `do OS's Navigate with "…"`, and the four endings. It is held to
Core v1 by the real compiler against the canonical `os.marco`, and a meaning Core has no sentence
for stops the lowering rather than widening the language.

The result is INERT: no file, no registry, no resolution path. See
[[ADR-027-what-marco-learned-becomes-marco]].

## Navigation is lowered as a MEANING

`KindNavigate` lowers to `do OS's Navigate with "confirm".` — never to a key chord. The
Director learned that somebody confirmed; which key that is belongs to the host, and the
intent→key table lives in `internal/oshost/navigate.go`. See
[[ADR-024-a-dry-step-is-not-evidence]] and [[ADR-013-navigation-is-meaning-not-keys]].

A **live** rehearsal is the same path again with `oshost.Host` beneath instead of the recorder —
see [[ADR-025-one-move-then-look]]. The composition root is the only thing that changes.

A MULTI-STEP rehearsal is the same path again, N times, with an observation between every two —
and every step lowers through `marcoexec`. There is no step-2 shortcut, which is the point:
learned behaviour must not acquire a private route to a host at the moment it gains authority.

The same boundary is what a **dry** rehearsal uses: `internal/director/rehearse` lowers one
authorized step through `marcoexec` exactly as a live one will, and the composition root installs
`recordhost.Host` instead of `oshost.Host`. There is no second path — that is the point.

## A chord is a sequence, and the releases are a promise

`do OS's Key with "ctrl+shift+a"` is one line of Marco and six events at the boundary: hold Ctrl,
hold Shift, settle, tap A, hold, then release all three in reverse. The releases are of a
different kind from the rest — a modifier this process pressed stays down on somebody's computer
until this process lets go of it, nothing else cleans that up, and the application cannot see
that it is happening.

Three defects lived in the middle of that sequence, inside the function that calls `SendInput`
where no test could reach them: a refused modifier returned leaving the earlier ones held, a stop
during the settle tapped the key anyway, and a release the machine refused was discarded entirely
— so the worst outcome of the three reported success. The sequencing is now `sendChord` against a
machine that takes one key event at a time, every key pressed gets a release ATTEMPT before it
returns — an attempt, because the machine can refuse one and that refusal is the point — and
a stuck key outranks a refused keystroke in what a caller is told — except that a stop keeps its
identity, because a stop that read as a failure would turn a person pressing Escape into a
routine that failed.

And a stopped run may not carry a stuck key away with it. `performOnePlay` discards a cancelled
run's error deliberately, so the host keeps the key it could not release in an account of its own
— `Host.NotReleased`, beside `Host.Unperformed`, and separate on purpose: that list is things
that did not happen, and this is something that did and has not finished. The outcome stays
`cancelled`; the sentence reaches the durable record, the run's own output, and the card.

The same rules then got a second slice: a **hold** is a key held down on purpose for as long as
the routine says, and the thing that cleans up after one is the end of a run — so a hold whose
release goes wrong goes wrong when nobody is watching. Holds are an ordered list released in
reverse, a refused release stays on the list for the route-end sweep to try once more, and what
the sweep still cannot release goes into the same account. The sweep used to discard every answer.

And a third slice, one device over: `do OS's Drag with …` is a button held down while the pointer
travels, and the same promise. It had the same three defects, in the same place — the whole
gesture inside the function that calls `SendInput` — and found the same way, by reading it rather
than by running it. A stop during the travel broke out of the loop and then sent the landing move
and the release anyway, so a stopped drag let go of the button at the DESTINATION: the thing being
dragged was dropped where the routine was going rather than where the pointer had got to. Nothing
read the run before the press. And every refusal `SendInput` reported was discarded, with `doDrag`
reporting through `ok` rather than `delivered` on top of it, so a drag the machine would not take
was absent from the account a command reads. The order now lives in `dragGesture` against a
machine that takes one pointer event at a time; every event carries its own point, which is what
lets a release be aimed at where the pointer IS, and the gesture tracks the last point the machine
accepted so a stop, a refused move and a refused landing all release there. The button is still in
no ledger, so a return that leaves the gesture by a path it does not own — a panic unwinding past
it — leaves it down; the guarded-drag contract proposes a ledger and a split of the backend's
`drag` so something outside it can release, and both are proposed designs rather than agreed
requirements. A ledger would have the same hard edge the chord's has: it lives inside the runner,
so it reaches ordinary returns and the unwinding where deferred code still runs, and nothing in a
terminated process runs at all. Recovering from a kill needs an owner that outlives the runner and
is outside this.

**And then the gesture got its guard.** `do OS's Drag with …` carries the window it was
demonstrated in now — two offsets and one set of window terms, because a drag's two ends are two
points in ONE window. Both ends resolve from one measurement at replay, and the window is asked
AGAIN immediately before the button goes down, against the snapshot those points were computed
for: the gesture aims in between, an injected pointer move changes applications, and a check made
before the gesture started is a check made before its own first event. A change there refuses
rather than re-bases. Cleanup is a defer inside the gesture, armed on an ACCEPTED press and not on
entry, so a panic during the aim reports nothing — no ledger and no split backend, because the
gesture is still on the stack while the unwinding happens. Existing saved Marco carries no `RelTo`
and takes the legacy path exactly as it did.
[[ADR-259-a-guarded-drag-is-two-points-in-one-window]]

Review of the gesture work found three more, and two of them were in the half the extraction does
not reach.
The **warning channel ended at the keyboard**: `Host.NotReleased` is the account a cancelled run
cannot discard, and it recognised stuck keys only, so a stop mid-drag recorded `cancelled` with
nothing beside it while the button was down. It asks for `stillHolding` now — the shape of the
thing, not the device — and the record, the output and the card needed no change, because they
take whatever sentence the host gives. A **cancellation during the aim still permitted the
mouse-down**, since the aim is a send and nothing was read between it and the press; the check sits
against the press as well now. And the **native adapter did not implement the contract the gesture
was written to**: the button events carried absolute coordinates without `MOUSEEVENTF_MOVE`, and
Windows moves the cursor only for an event that carries that flag — so the press happened wherever
the pointer already was while the gesture's contract read as satisfied. Press and release are each
the movement and then the button in ONE `SendInput` call now, `clickAt`'s shape. The lesson is
about what a seam proves: lifting the sequence out made the order reviewable and said nothing
whatever about the adapter underneath it.

Two limits, in those words: a failed release is **reported, not recovered** — none of that
machinery puts the key back up, and whoever reads the sentence is the recovery — and **forced
termination is outside it**, because a process killed between a modifier going down and its
release runs no more code. The guarantee is that every pressed key gets a release attempt by code
that gets to run. [[ADR-258-a-chord-is-a-promise-to-let-go]]

## What this caught

Lowering revealed three capabilities that worked from `oshost` but that **no line of Marco
could call**: `ClipboardGet`/`ClipboardSet`, `MoveWindow`, and the `Accessibility` act. A
direct-invoke design would never have noticed.

## Related systems

- [[Programs]] — produces the steps
- [[Semantic-Actions]] — decides *what* to lower; this decides how
- [[Action-Graph]] — stores the verb, never the lowering, so replay lowers again

## Decisions

- [[ADR-005-legal-marco-only]]
- [[ADR-024-a-dry-step-is-not-evidence]]
- [[ADR-025-one-move-then-look]]
- [[ADR-026-verification-is-derived-from-a-completed-rehearsal]]
- [[ADR-027-what-marco-learned-becomes-marco]]
- [[ADR-070-one-production-body-and-the-caller-brings-the-verification]]
- [[ADR-028-a-learned-play-is-a-file-with-a-past]]
- [[ADR-029-resolution-is-not-permission]]
- [[ADR-030-a-play-says-where-it-begins]]
- [[ADR-031-the-user-names-the-stage]]
- [[ADR-258-a-chord-is-a-promise-to-let-go]]
- [[ADR-259-a-guarded-drag-is-two-points-in-one-window]]

## Validated by

- `TestDirectorHasNoDuplicatePlatformImplementation`, `TestDirectorImportsNoPlatformCode`
  (`internal/director/boundary_test.go`)
- `cmd/director/lockrule_test.go`, `cmd/director/keystroke_test.go`
- `TestOneAuthorizedStepIsLoweredToARecordingHost`, `TestTheRealHostReceivesTheStepsOwnOrderedIntents`
  (`cmd/director`) — one authorized step through the whole real path, into a notebook
- `director lower` renders the generated program for inspection

## Known gaps

See *What is NOT compiled into Marco* in [[director-marco-boundary]] — notably the wait
engine, which stays on the Go side deliberately.

## Milestone record

[[director-marco-boundary]] — the capability table, escaping, foreground protection, secrets,
and the tests that hold it up.

## The Screen act

`screen.marco` declares one read-only export, `Showing`, answering a single question: *is the
screen the user named the one in front?* It is fulfilled by `internal/platform/screenhost` over a
three-method read-only view of the world — it cannot press a key, focus a window, run a route,
write to memory or grant anything.

It exists because a learned play has to say where it begins and where it expects to finish, and
until something could answer that sentence every guarded play refused. **No language change was
needed** — the `when ok? … or?` shape was already Core v1; what was missing was a capability.

Silence is never yes. An unknown name, an ambiguous name, an ambiguous screen, a screen nobody can
see, a missing recogniser and unreadable memory all return `failed`. A standalone `marco` with no
recogniser wired refuses rather than degrading into blind replay.

See [[ADR-030-a-play-says-where-it-begins]], [[ADR-032-a-play-says-where-it-ends]] and
[[Learned-Plays]].

## The Theater act, and the one production body

`theater.marco` declares one export, `Activate`: do to this target what pressing it would do. It
is fulfilled by `internal/platform/theaterhost`, which resolves the semantic target against
whatever can see it tonight, casts an available Actor, and runs what that Actor writes.

**An Actor writes legal Marco and performs nothing.** `Actor.Cast` returns a program; the
Production boundary runs it through an injected `MarcoRunner`. So a press reaching the machine
through the Theater is subject to exactly the guarantee this note is about — compiled before
mutation — rather than being a second route with no compile gate and nothing for a dry run to
record. An Actor that called a host directly would be that second route, and
`TestAnActorNeverReachesAHostDirectly` fails if one does.

**Both callers use it.** A saved play asks through the ordinary act. A live rehearsal asks through
`internal/production`, a contract of types and interfaces that lets the Director name a production
without importing the Theater — the boundary that keeps `internal/director/rehearse` unable to act
by itself. The rehearsal's old direct path (resolve a name to an element id here, walk a private
activation ladder) is deleted rather than kept as a fallback, for the same reason `marcohost` was.

**Authority and verification cross with the request.** A permission that can be spent once and
only for the named target; and whatever the caller can check with, which for a standalone runtime
is honestly nothing. See
[[ADR-070-one-production-body-and-the-caller-brings-the-verification]] and
[[ADR-068-the-theater-is-the-durable-semantic-world]].

The ownership question this settled — which side of the Director/Theater line each responsibility
falls on — was worked out first in [[34E-director-theater-audit]].

**A cast program cannot re-enter the Theater.** The act map it can reach is built without the
Theater in it, in `cmd/marco/theaterwiring.go`, so the recursion is impossible rather than bounded.

## The saved play owns the order, through a door of its own

A demonstrated visual routine's whole-sequence progression used to live in Go — `workflowrun.Runner`'s
cursor — while each individual effect was lowered to legal Marco and compiled. The ORDER now lives
in one saved `.marco` play, and every judgement stayed where it was.

It needed a NEW ACT, and the reason is this note's own boundary. `theaterhost.Actor` offers `Find`
and `Cast`, and `Cast` returns source and performs nothing — the Production boundary runs it
through an injected `MarcoRunner`, which is the compile gate this page is about. An Actor
therefore cannot ask a person for permission or wait for a verdict, so the supervised runner
cannot be one: it is the thing doing the casting. And `Theater's Activate` means *find a control
by this name among whatever is in front*, which is a different authority from *press the thing
somebody demonstrated, with a person agreeing to it*. One sentence meaning either, depending on
who ran the file, would be a play whose meaning depended on its host.

The measurement that decided the generated form: a FLAT two-sentence play asks for the second
interaction after the first has failed and then logs success, because an unhandled failure does
not abort a body. So the nesting is the gate, and a generator that emitted the flat form would
write a file that lies.
[[ADR-243-a-door-of-its-own-and-the-order-in-the-file]]

**And it is reachable by name.** `director perform-workflow --play "<name>"` resolves a saved Act,
preflights it — the record is shown to describe the play by rebuilding the play from it — and then
performs it through the same assembly a workflow run uses, differing in one thing only: the order
comes from the play's sentences rather than from the artifact's cursor. The observation loop moved
to its own file so that both drivers share it rather than copying it, and the between-looks work,
including the step-agreement question, is handed to the play driver rather than reimplemented.

**And a person authorises it once, by pressing Play.** `--granted <file>` performs an Act under an
authorisation given in advance — the Act, its revision, its application, its interactions by the
names somebody gave them — and the run then asks nobody anything: no start countdown, no screen
question, no agree-then-authorise pair per press. Nothing is auto-answered and nothing simulates a
keypress; the authorisation is its own type, checked against the preflighted plan before anything
is observed, and refused per press for anything it does not name. Every execution guard is
untouched, because none of them ever asked a person: the screen question is replaced by evidence
(the shape a gate, the words the discriminator, no second saved screen allowed to fit) and a run
that cannot identify the screen PAUSES with a sentence instead of pressing.
[[ADR-244-pressing-play-is-the-authorisation]]

**And the surface shows one task at a time.** A rail of three places — Acts, Scenes, Run history
— with one of them open, and a run that replaces what was being looked at while keeping Stop in
front of the person. An Act's detail is what it will do before anything happens, in the words
read off the person's own screen rather than `control_2` and `screen_a`; `ready` is no longer the
whole answer, because an Act whose starting screen was never read preflights perfectly and can
only wait; the ordered preview sends nothing, and the engineering run with input disabled is no
longer offered as a rehearsal of an Act it cannot reach the second step of. A review shows the control, the starting screen and the result as PICTURES, because the words are what recognises a screen at run time and are not what a person recognises — kept per routine and per revision, so a routine demonstrated again never shows the previous recording's pictures beside the new one's words, and read by nothing that runs. And the recorder says what evidence it has while somebody is still demonstrating, asking the same question the run will ask of the screen an Act starts on rather than a weaker one — and saying when the evidence is IN and empty, because an outcome whose window has filled cannot improve by being waited for, and a card that asked anyway cost the owner thirty seconds of a sixty-second recording. It also says, while it can still be acted on, when nothing has been read on the screen in front: an outcome's before side is taken once, at the instant of the press, so a click on a screen nobody has read yet can never be checked at all.
[[ADR-245-one-task-at-a-time]]

**And a trial of the surface collects its own evidence.** Studio writes down what a sitting at it
showed, as it happens, and `studio -trial <store>` reads it back: which builds took part — the
window, the recorder from its own stream, and the director that performed a run from the record it
wrote, with an unstamped one reported as having said nothing rather than folded in as a revision
two unknowns share; the states a recording passed through, taken from the value handed to the page
so what is written is what the person was shown, and written only when it changes; how often the
page had to ask for each picture, which is the blinking defect expressed as a number; and the
canonical result, cited by the run's own reference and read out of the run record. Four facts
nobody is asked to watch for, because every defect this surface has had was found by the owner
describing it and engineering working backwards from the sentence.
[[ADR-246-a-trial-collects-its-own-evidence]]

**And a screen that can never be established is no longer the end of the road.** GEOMETRY REPLAY
is an explicit execution mode — off unless asked for, granted from outside the planner and
outside the runner, offered in Studio beside Play and never taken for anybody. In it Marco
presses THE POINT THE PERSON CLICKED, taken from the press and never computed from a rectangle a
proposer drew around it, and only while the window is the EXACT size it was demonstrated in. A
mode rather than a fallback, which is load-bearing: the window is checked ALWAYS — at the plan
and again at the press boundary — because a geometry check that ran only after recognition failed
let a run whose screen happened to settle press into a window of any size at all. The press
boundary in the mode has its own terms, all about where a click will land plus the geometry, and
it takes NO look: no screen hypothesis, no observation generation, no reading, no proposal under
the point. The vision path is untouched and is what every other run takes. Nobody's agreement and
nobody's authorisation is waived, and what the press DID is verified exactly as before. It has
its own vocabulary — `demonstrated layout` support, a grounding that says in words that nobody
looked, an authorisation line that says "the point somebody clicked" rather than "the middle of a
region" — so no run's record describes it as recognition. The three live trials it exists for all
died at the same refusal with zero actions sent, on a menu drawn over a live 3D world that is
filed as two screens depending on what moves behind it.

**And the gate in front of all of it.** Three layers had to stop requiring a named screen, and
the outermost was found only by a live trial. The run coordinator calls its grounder on every
look and used to treat a failure to recognise as a reason to wait, so a licensed run never
reached the plan at all: two runs against the owner's pause menu agreed with the best saved
screen at 0.500 and 0.442 against a 0.55 threshold and both refused with ZERO actions sent, while
the suite was green — each inner layer's tests entered at that layer. A licensed run is no longer
held there. The grounder is still CALLED, because the screen is worth knowing when it can be and
not calling it would claim the mode stops perceiving; it is only the refusal that is lifted. What
remains vision is the verification half, on purpose: a step finished by its own outcome needs the
panel READABLE even though its screen need not be NAMEABLE, and with nothing readable the press
does not go out under any licence.
[[ADR-248-replaying-a-demonstrated-layout]]

