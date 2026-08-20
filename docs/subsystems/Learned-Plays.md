---
type: subsystem
status: active
owners:
  - director
depends_on:
  - passive-observation
  - demonstrations
  - semantic-memory
  - marco-boundary
used_by:
  - service
updated: 2026-08-10
source_paths:
  - internal/director/observe/lowering.go
  - internal/director/observe/rehearsal.go
  - internal/director/observe/screennaming.go
  - internal/director/marcoexec/play.go
  - internal/director/rehearse
  - internal/platform/screenhost
  - internal/routes/origin.go
  - internal/orchestrator/authority.go
  - cmd/director/learnedplay.go
  - cmd/director/observeregistry.go
---

# Learned Plays

How something Marco *watched* becomes something Marco *performs* — and how each step earns the
next. This is the chain ADRs 023–032 constrain; read this note before any of them.

## The chain

```
demonstration  →  rehearsal  →  verification  →  judgement  →  Marco source
   (watched)      (one try)      (derived)      (may it be     (a file a person
                                                 written down)   can read)
                                                       ↓
   name the screens  →  save  →  register  →  resolve  →  authorize  →  run  →  arrive
     (the user's         (a file  (findable)  (which     (may I?)              (did it
      words)              on disk)             play)                            work?)
                         └─ one Learn ─┘
```

**Each arrow is a wall, not a pipe.** Watching is not verification; verification is not
permission to write it down; writing it down is not saving; saving is not registering; resolving
is not permission to run; running every step is not succeeding. Every one of those confusions has
its own ADR because every one of them was a real temptation.

**A wall says what the mechanism keeps apart. It does not say who crosses it.** The
save→register wall is real — two operations, two directories, a refusal on collision — and the
Learn workflow crosses it in one act, because the Audience's *"learn X"* is the permission to
make X askable. A person who demonstrates a behaviour and names it has not asked for a file they
cannot use, and there is no second consent left for them to give. See
[[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]]. The *developer*
lifecycle (`director learned --save` / `--register`) still crosses the two separately.

And it registers as a **focus** route, not a context one — a learned play is asked for from
wherever the Audience happens to be, and the Director brings its application forward itself.
Registered as context it resolved only while that application was already in front, which meant
Marco offering to teach a play it had just learned. See
[[ADR-080-a-learned-play-is-asked-for-from-anywhere]].

## The three things that are not each other

| | |
|---|---|
| `CandidateAssessment` | what the demonstration evidence supports |
| `RehearsalJudgement` | is that enough to **ask** for one controlled experiment? |
| `RehearsalGrant` | the user said yes to **one attempt** |

Evidence is not authority. Eligibility is not authority. A question is not authority. A previous
yes to *learn this* authorised **watching** — see [[ADR-023-rehearsal-is-attempt-scoped-authority]].

## Verification is derived, never stored

`ProcedureCandidate.Verified` is always false and deliberately vestigial. What counts is the
assessment folded with stored rehearsal evidence — the route completed, the digest still matches,
both endpoints are still remembered — and it is **recomputed on every read**. A candidate that was
lowerable last week may not be now, because a screen was contradicted or the demonstration was
revised. See [[ADR-021-a-judgement-is-recomputed-not-recorded]] and
[[ADR-026-verification-is-derived-from-a-completed-rehearsal]].

## What a play is allowed to say

Navigation meanings, in order, with repeats intact — and the two screen names the user gave.
**Nothing else.** No subject ids, no screen states, no digests, no window handles, no counts, no
verdicts, no account of how Director came to believe any of it.

Director may know *why* it chose an action. Marco says *what* it intends to do. A Marco program
is a play somebody reads — [[ADR-027-what-marco-learned-becomes-marco]] and the governance rule
in `spec/Core.md`.

## The shape, and why it is that shape

```marco
this's Mute does...
    do Screen's Showing with "the pause menu"...
        when ok?
            do OS's Navigate with "down".
            do OS's Navigate with "confirm".
            do Screen's Showing with "the audio page"...
                when ok?
                    this is ok!
                or?
                    this is failed with error "this play expected to finish on the audio page"!
        or?
            this is failed with error "this play starts on the pause menu"!
```

Read it as a sentence: *this play only starts on the pause menu; it presses down then confirm; it
expects to finish on the audio page.*

**Everything is nested on purpose.** An earlier shape asked the entry question first and let the
`or?` arm return, with the steps after the block. It compiled — and it did not guard, because a
return inside an arm ends *the arm*, not the capability. Nesting makes the guarantee structural:
there is no line after the block for control to fall through to, so the only path to an effect is
through an `ok`, and the only path to `ok!` is through the arrival check.

> **Compiling is not behaving.** A test that only reads the source, or only checks that it
> compiles, would have shipped that bug. See [[ADR-030-a-play-says-where-it-begins]].

## No new syntax was needed for any of it

`do <Act>'s <Cap> with "…"... when ok? … or? …` is Core v1 composition — the same shape `OS's
Focus` has always used. What was missing was never a language construct but a **capability**:
nothing could answer *"is the screen the user named the one in front?"*. `screen.marco` declares
one read-only export and Core v1 is unchanged. The same capability answers both the entry
condition and the arrival check; asking it after the effects instead of before them needed
nothing new at all — [[ADR-032-a-play-says-where-it-ends]].

## The play's own name is derived, and announced

A play is a sentence, so saving one needs an actor and a verb. They are derived from the
phrase the person typed — first word the verb, the rest the actor (`open mouse settings` →
`do MouseSettings's Open`) — validated at the request boundary before anybody demonstrates
anything, and **said out loud** before the demonstration rather than met for the first time
in a saved file. [[ADR-061-a-derived-name-is-said-out-loud]].

This is a different thing from the screen names below, and the distinction is the point: a
play's name is an identifier for a file, derived from words typed for that purpose; a
SCREEN's name is executable meaning resolved against durable memory at run time, and comes
only from a person.

## The screen names are the user's, and only the user's

`Screen's Showing with "…"` is **executable meaning**, not a label: the string is resolved against
durable memory at run time. So there is no name Director may invent. `observe.ScreenName` has one
constructor, `UserSuppliedScreenName`, which exists to be a call site a reviewer can grep — an OCR
line, an accessibility label or a window title cannot reach durable memory by being assigned to
the right variable. See [[ADR-031-the-user-names-the-stage]] and [[Semantic-Memory]].

The demand is **not** `subject.Called == ""`. Marco asks when a concrete artifact is blocked: a
verified procedure ready to be written down whose `JudgeLowering` refused with `screen_unnamed`.
A sweep over unnamed subjects would be a collection loop wearing a question's clothes.

`LoweringJudgement.Unnamed` lists the subjects still needing a name, **source first**. Naming one
and recomputing surfaces the next — no queue, no remembered question. There is one
`AskNameScreen`, not one per role: the place being named is just a remembered screen, and a
subject that is the destination of one play may be the source of another.

## Success is arrival, not emission

A host can send `down` and `confirm` perfectly while a dialog eats them or the menu had one more
row than the demonstration did. Before the arrival check, the play reported `ok!` as soon as the
last key went out — a claim about the *host* dressed up as a claim about the *application*.

The two failures are different in kind, and the difference is visible:

| | |
|---|---|
| wrong start | zero effects; the arrival is never even asked about |
| wrong destination | the effects already happened, and the play still fails |

Marco does not undo, retry, replay, send recovery input, mutate semantic memory, or invalidate the
play. Runtime evidence says *this invocation did not end where it expected* — in the play's own
words, with no internals shown.

## Silence is never yes

Every way of failing to establish that a screen is the named one returns `failed`: an unknown
name, an ambiguous name, an ambiguous screen, a screen nobody can see, a missing recogniser,
unreadable memory. There is no path through `internal/platform/screenhost` that answers `ok`
without having positively identified the named subject. A standalone `marco` with no recogniser
wired refuses rather than degrading into blind replay.

## Saved ≠ registered ≠ permitted

Route discovery is a directory scan of `global/`, `<app>/`, `<app>/context/` and `<app>/focus/`,
so `<app>/learned/` is **structurally invisible** — saved-but-not-registered is a property of the
layout rather than a flag somebody could forget to check. See
[[ADR-028-a-learned-play-is-a-file-with-a-past]].

Registering makes a play *findable*. It does not make it *runnable*: `internal/orchestrator`
`Resolve → Authorize → Run` puts a door between knowing which play the user means and performing
it — [[ADR-029-resolution-is-not-permission]].

A completed Learn asks for both halves, so the ordinary end state is saved **and** registered
([[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]]). Saved-and-not-
registered is still reachable — a slug already taken is the common way — and it is a state with
its own refusal (`play_not_registered`), its own sentence (*"the play is safe — it is a file you
can read and edit"*), and its own reason carried up from the registry. It is never reported as
`save_failed`, and no surface may claim such a play is in the Routes tab. `Unregister` reaches
the staged copy too, so a refused registration does not strand an artifact nothing can remove.

Where a learned play lands: `<app>/context/`. It resolves while its application is in front and
nowhere else. Pinned by `TestALearnedPlayIsRegisteredWhenItIsSaved`; whether that should be
`focus/` is an open product question.

## The source owns the meaning; the sidecar owns nothing

`.origin.json` records where a play came from. It **enforces nothing**. A user who deletes the
entry guard owns a play with no entry condition; a user who deletes the arrival check owns a play
that no longer makes that claim. Both run, and Marco stops claiming it verified the file. The
digest is computed over the final generated source, both conditions included.

## Nothing hidden is needed at invocation

A saved play needs its `.marco`, semantic memory, the Screen host, and the ordinary invocation
machinery. **No `ProcedureCandidate`, no `RehearsalEvidence`, no `.origin.json` enforcement.** The
play itself carries what Director learned. That is the architectural payoff of the whole chain,
and it is the first thing an audit should try to disprove.

## Production call sites

An auditor's shortest path to the real behaviour. Each is a place where deleting one line must
break a named test.

| what | where |
|---|---|
| judgement recomputed from memory | `cmd/director/learnedplay.go` `LearnedPlay` |
| `screen_unnamed` → naming question | `cmd/director/learnedplay.go`, guarded by `refusedFor` |
| naming question created, idempotent | `cmd/director/observeregistry.go` `ProposeScreenName` |
| raw text → `UserSuppliedScreenName` | `cmd/director/observeregistry.go` `Observation`, `case q.Name` |
| answer → `Store.NameSubject` | `cmd/director/observeregistry.go` `AnswerName` |
| both claims emitted | `internal/director/marcoexec/play.go` `lowerPlay` |
| compiled against the real acts | `cmd/director/learnedplay.go` `compileGenerated` |
| save / register / forget | `cmd/director/learnedplay.go` `lifecycle` |
| resolve → authorize → run | `internal/orchestrator/authority.go` |
| the screen question answered | `internal/platform/screenhost/screenhost.go` `showing` |

## The rehearsal reaches the right window, or waits

Three corrections from the 2026-08-17 live-run audit, all on the attempt path:

- **The step record carries its scope.** Every live step's outcome is classified by
  recalling against durable memory, which is application-namespaced; the scope was once
  left off the record and every live rehearsal on every application ended
  `stopped_at_step`/`unrecognised`. Mutation-gated by
  `TestALiveStepIsClassifiedAgainstItsOwnApplication`.
- **Input has no address**, so a real attempt refuses `window_not_in_front` before the
  grant is claimed and re-checks before every step; upstream that refusal is the patient
  case, like the rest of [[ADR-055-an-authorised-rehearsal-waits-for-its-start]]'s set.
  [[ADR-060-input-has-no-address]].
- **A demonstrated click rehearses as an invocation.** A `point` step with a named target
  lowers to `Accessibility's Invoke` on the control resolved live at emission time
  (uniqueness demanded, refusal before emission otherwise) —
  [[ADR-058-a-demonstrated-target-may-keep-its-name]].
- **And it is now written down as one.** `Control.Name` names a control by its LABEL, which
  the host resolves against the live tree when the play runs — the run-time name-resolving
  activation this used to wait for. A lowered step is a `LoweredAction`: a navigation meaning
  or a named control. The refusal that used to cover every click narrowed to
  `no_target_to_name` and now means only what it says — a click nobody could attribute to a
  control. [[ADR-067-a-play-may-name-a-control]].

A yes that creates no authority is said out loud: the runner records the closed
`AuthorizationRefusal`, the tail carries it up, and the timeout blames the failed
authorization rather than the silence — and never reports it as the Audience declining
([[ADR-077-consent-is-the-audiences-authority-is-marcos]]).

A learned Play is PERFORMED by the Director, not by standalone Marco: fresh Stage, plan over
verified edges, then the same bounded walker a rehearsal uses, verifying after each edge.
Proven from a cold process with an unrelated application in front.
[[ADR-078-a-learned-play-is-performed-by-the-director]].

## Related

- [[Passive-Observation]] — where candidates come from
- [[Demonstrations]] — how a procedure is extracted from what was shown
- [[Semantic-Memory]] — durable subjects, and the one string a user writes
- [[Marco-Boundary]] — every desktop effect lowers to legal Marco source
- [[Wiring-Tests]] — why every mechanism here has a mutation gate
- [[Audit]] — the invariants, and the places they are thinnest
