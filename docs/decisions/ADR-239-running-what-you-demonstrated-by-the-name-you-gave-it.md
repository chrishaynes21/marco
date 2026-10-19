---
type: adr
status: accepted
date: 2026-10-18
decision: "ACCEPTED — a demonstration is performed by the NAME the person saved it under, with `perform-workflow --name`, resolved through the same demo store `correct`, `reacquire` and `what-i-did` already read. `--workflow` stays and stays a file. Exactly one of the two, never a fallback. And a saved routine is told what it can actually do: a pixel routine is run by sight rather than reported unrunnable, an UNSCOPED one is not offered to a command that will refuse it, and a step carrying no outcome expectation says that its action can be performed and cannot be verified."
supersedes: []
affects:
  - execution
  - learning
source_paths:
  - cmd/director/performworkflow.go
  - cmd/director/democmd.go
  - cmd/director/performbyname_test.go
  - cmd/director/howtorun_test.go
---

# ADR-239 — running what you demonstrated, by the name you gave it

## The gap

The owner's framing, after the controlled pixel milestone:

> We've independently validated the two halves of the product: Marco can record what a human
> demonstrates, and Marco can execute and verify a pixel-based action. We haven't yet proven
> those halves work together.
>
> Identify the smallest remaining integration gap preventing a person from demonstrating a
> game-menu action, saving it, resetting the menu, and invoking that action by name.

It was two things, and the second is worse than the first.

**`perform-workflow` could not be given a name.** `demonstrate-visual` scopes a demonstration to
the application it watched and saves it in the demo store under the person's own name, with the
raw demonstration beside it. `correct`, `reacquire` and `what-i-did` all take that name. The one
command that ACTS took `--workflow <path>`, so running what you had just demonstrated meant
finding its JSON under the config directory and passing that.

**And the product said it could not be run at all.** `director sequences` reported, of every
pixel routine:

	None of these can be run: they were learned from PIXELS and carry no
	verbs. Measure one instead:
	  director reacquire "ark menu 2" --application <app>

That was true when it was written. `runnable` asks whether the ORDINARY execution path can act,
and a routine learned from pixels carries no verb for it to act on. It stopped being true the
moment `perform-workflow` learned to ground a saved control from pixels, authorise it, press it
and verify the outcome — and nothing updated the sentence. Marco was reporting the thing it had
just learned to do as impossible, and `reacquire` measures whether a control can be found again
and presses nothing, so the advice led away from the capability rather than to it.

Three routines on the owner's machine were in exactly that state: demonstrated by hand against
Ark on 2026-10-17, and each one reported unrunnable.

## The decision

`--name` resolves through `demo.Store.Sequence`, the same lookup the sibling commands use, with
the same two refusals they give: a routine that cannot be READ is a file to go and look at, and
a name nobody saved is a demonstration to go and give. One sentence for both would send somebody
to the wrong place.

**Exactly one of `--name` and `--workflow`, and not a fallback.** A name that is also tried as a
path, or a path treated as a name, turns a typo into a different routine running — and this
command presses a mouse button. Both given is a refusal; neither is a refusal; and a name that
is not saved lists nothing, because guessing which saved routine somebody meant is how the wrong
one runs.

`--workflow` stays, and stays a FILE: a derived fixture is an artifact nobody saved by name, and
the controlled execution tests run from one.

## And a saved routine is told what it can do

One statement of it, `howToRun`, used by both the list and the single-routine view, in the order
of what the routine carries:

| what it carries | what it is offered |
|---|---|
| verbs | `director execute "run <name>"`, as before |
| visual evidence, scoped | `director perform-workflow --name <name> --live --reading` |
| visual evidence, UNSCOPED | nothing to run; demonstrate it again |
| neither | nothing to run and nothing to look for |

**An unscoped routine is deliberately not offered to the execution path**, because it would be
refused there: an artifact saved before workflows recorded their application is valid for NO
application, and the identity cannot be invented here — it has to come from a window that was
observed. That is not a nearly-runnable routine and must not read like one.

**And a step with no outcome expectation says so.** Performed-and-not-verified is a different
offer from performed-and-verified, and a person about to let Marco drive their desktop should be
told which one they are getting.

## What this means for the next milestone, measured rather than assumed

The three hand-demonstrated Ark routines carry **no `application` field and no `outcome`
field** — they predate [[ADR-236-a-workflow-belongs-to-the-application-it-was-shown-in]] and
[[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]] respectively. So they cannot be
run live (unscoped) and could not be verified if they were (nothing to verify against).

The first genuinely human-demonstrated pixel action therefore needs a FRESH demonstration. That
is a fact about the evidence those recordings contain, not a gap in the path, and it is the
honest answer to "can we do this with what we already have": no, and here is exactly which two
fields are missing.

## Enforced by

`cmd/director` `TestADemonstrationIsPerformedByTheNameItWasSavedUnder`,
`TestANameNobodyTaughtIsRefusedWithSomewhereToGo`, `TestAFileAndANameAreNotBothAnswers`,
`TestAnArtifactFileStillPerforms`,
`TestAnUnreadableFileAndAnUnreadableNameSayDifferentThings` and
`TestARoutineThatCannotBeReadSaysSoRatherThanThatItWasNeverTaught` — which drive the real
resolution against a real store in a temporary `MARCO_HOME`, so the artifact that comes back is
the one the store wrote, scope and outcome expectation included.

And `TestAScopedPixelRoutineIsOfferedToThePixelExecutionPath`,
`TestAnUnscopedRoutineIsNotOfferedToACommandThatWillRefuseIt`,
`TestARoutineWithNoOutcomeSaysItCannotBeVerified`,
`TestAVerbRoutineStillGoesToTheOrdinaryPath` and
`TestARoutineWithNeitherSaysSoRatherThanOfferingSomething`, with the pre-existing
`TestAPixelRoutineIsNotOfferedAsRunnable` updated rather than deleted: its fixture is unscoped,
so it now asserts the new reason and that the routine is NOT sent to the execution path. The
unscoped case also asserts that
`ScopeAllows` agrees with the advice, so the day the gate changes, the sentence is found to be
wrong by a test rather than by a person.

Eight mutations over the two, all killed.

And `reacquire` is still offered for an unscoped routine, because measuring whether its controls
can be found again needs no scope and presses nothing. The first version of this dropped that
line along with the false claim, and dropped something true with it: a routine that cannot be
RUN can still be MEASURED.

## See also

- [[ADR-236-a-workflow-belongs-to-the-application-it-was-shown-in]]
- [[ADR-238-a-verdict-needs-both-sides-and-a-window-of-its-own]]
- [[Experiment-090-the-first-guarded-pixel-action]]
