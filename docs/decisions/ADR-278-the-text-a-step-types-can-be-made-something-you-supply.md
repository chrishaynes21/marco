---
type: adr
status: accepted
date: 2026-10-30
decision: "ACCEPTED — A STEP'S TEXT CAN BE MADE SOMETHING YOU SUPPLY, FROM THE PANEL THAT SHOWS THE ROUTINE. A third edit operation (`orchestrator.EditArgument`) replaces a Type step's literal with `{{name}}`, which is the placeholder [[ADR-276-a-routine-may-be-told-what-to-type]] already built everything on — so nothing downstream changed. THE REASON IS DISCOVERABILITY, NOT CAPABILITY: this was already possible two ways and both were invisible — name the recording `say hello with message` so the phrase declares an argument, and tap a RESERVED KEY (f9) at the moment in the demonstration where the text goes. Both must be known in advance and neither is reachable from any surface; the owner of this repository could not find them, which is what asked for this. As an edit it needs no foreknowledge and no second demonstration, and it works on routines saved before any of it existed. THE CONTROL IS OFFERED ONLY ON A STEP THAT TYPES TEXT, because a box beside a click would be a control that refuses after somebody had typed a name into it; a step whose text is already supplied SAYS SO instead. A NAME IS LOWERCASED and must be what `routes`' own pattern reads back, or the routine would type two braces and the name at them. AND A NAME THE GENERATOR MEANS AS A CREDENTIAL IS REFUSED — `codegen.MeansASecret`: `password`, `token`, `pin`, `key` and the rest resolve from the CREDENTIAL STORE rather than being typed, so somebody asking for text they supply and happening to choose one of those words would get a routine that types whatever is saved there. THE DEFECT THIS SHIPPED WITH FOR AN HOUR, found by reading the rendered source and invisible to every assertion about the step list: `codegen` keeps a `{{name}}` in a typed literal only when the name is DECLARED, so the first version rendered `do OS's Secret with \"message\"` where `do OS's Type with \"{{message}}\"` belonged. `applyEdits` now returns the names it declared, and a placeholder already in a step list declares itself (`suppliedIn`) — without which the SECOND edit of an edited routine refused, saying the routine no longer agreed with itself. 17 of 17 mutations killed; one unreachable guard was declined rather than written, and two of my own test cases were removed for asserting a combination that cannot exist. AND THE SAME DEFECT WAS AT A THIRD SITE, found by the regression test the Architect asked for: the REBUILD review's today's-rules arm rendered `do OS's Secret with \"message\"` as the version it offered to replace the routine with, because it too declared only what the NAME declares. `declaredFor` is now the one owner of \"what a render of these stored steps must declare\" and all three ask it. The action on a saved routine is called **Simplify…** with the review promised beneath it rather than in its label, and the UNSAVED recording review offers it too — previewing a simplified draft, keeping the way back to what was demonstrated, and writing nothing until Save, through the same `SimplifiedDraft` the terminal review asks. A DEAD SENTENCE WAS WIRED UP ON THE WAY: `Prepared.StepsWarning` has existed since step editing did and Studio never asked for it, so a rebuild that discards an edit said nothing beforehand. 14 further mutations: 12 killed and one EQUIVALENT (written down). THE ONE THAT WAS UNTESTED IS CLOSED: the refusal of a simplification the generator will not write down, verified with the stranded-ctrl fixture — the reason is shown, the recording under review is NOT displaced by a version that could never be saved, and nothing is written. 35 distinct mutations across the slice: 33 killed, 1 equivalent, 1 a nil check kept on purpose."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/stepedit.go
  - internal/orchestrator/regenerate.go
  - internal/orchestrator/draft.go
  - plugins/studio/recording.go
  - plugins/studio/generation.go
  - internal/codegen/codegen.go
  - plugins/studio/stepedit.go
  - plugins/studio/frontend/index.html
---

# The text a step types can be made something you supply

[[ADR-276-a-routine-may-be-told-what-to-type]] made a routine able to accept text and Studio able
to supply it; [[ADR-277-a-request-may-propose-the-text-a-routine-types]] made a request able to
propose the value. Both assumed a routine that already DECLARED an argument, and the two ways to
get one were:

- name the recording `say hello with message`, so the learn phrase declares it; and
- tap **f9** (`simplify.DefaultArgKey`) during the demonstration, where the text goes.

Both have to be known in advance. Neither is mentioned by any surface, and the owner of this
repository — who asked for the feature — could not find them. A capability nobody can reach is not
a capability, which is the same lesson [[ADR-265-say-what-you-want-and-studio-offers-the-routine]]
records about a resolver that was complete and unreached for a release.

## As an edit, because that needs nothing known in advance

The step panel already re-renders a routine from its own step list
([[ADR-273-a-routine-writes-down-what-it-does]] and its step-editing slice), so a third operation
fits the contract exactly: apply it to the list, render again, review, save. Somebody types the
text they actually want, sees it in the list, and says **this is the part that changes** — and it
works on routines saved long before any of this existed.

What it writes is `{{name}}`, which is what `routes.Arguments` reads, what Studio's field beside
Play asks for, what `routes.ApplyArgs` fills at run time, and what a request's proposal is asked
about. Nothing downstream needed changing, which is why this is three lines of IR rather than a
feature.

## What it refuses, and why each one

| refused | because |
| --- | --- |
| a step that types no text | there is nothing about a click or a key press to supply, and a box beside one would be a control that refuses after a name had been typed into it |
| text already supplied | a second pass would bury one placeholder inside another and the routine would type the braces |
| a name outside `[a-z0-9_-]+` | `routes`' own pattern is what reads it back, and a name it will not read is written into the source as text that LOOKS like a placeholder and is never filled |
| a name the generator means as a credential | see below |
| the same step also removed | one operation per step, which is the rule the other two already follow |

**And a name is lowercased.** Every reader downstream compares these case-insensitively —
`CheckArgs` lowers them, `zipPositional` lowers them — so a stored `{{Message}}` beside a field
asking for `message` is a confusion with nothing to gain from it.

## A name the generator reads from the credential store

`codegen` resolves a placeholder called `password`, `pass`, `passwd`, `pwd`, `pin`, `secret`,
`token`, `otp`, `apikey`, `api-key` or `key` from the **credential store** instead of typing it.
So somebody asking for text they supply and happening to choose one of those words would get a
routine that types whatever is saved there — a surprise nobody asked for, about a password, and
they cannot be expected to know the list.

So it is refused by name, with the reason said plainly. `codegen.MeansASecret` is now exported for
it: one list, in the package that acts on it, because two would disagree the first time either
changed and the disagreement would be a routine reading a credential where somebody expected their
own words.

## The defect this shipped with for an hour

**Every assertion about the step list passed while the rendered routine was wrong.** The step
became `Text: "{{message}}"`, which is exactly right, and the source it rendered to was:

```
do OS's Secret with "message".
```

`codegen.splitSecrets` keeps a `{{name}}` inside a typed literal only when the name is **declared**
(or numeric); any other one becomes a credential-store read. The declarations have always come
from the learn phrase, so an edit that invented a placeholder and rendered against the routine's
OLD declarations produced a routine that reads a password where the person asked for their own
words.

Found by printing the render in a throwaway test, not by a failing one. Two fixes came of it:

1. **`applyEdits` returns the names it declared**, and `PrepareEditOf` adds them to the render's
   declarations. Returned rather than re-derived by the caller, because the caller would then be
   working out what an edit meant a second time and the two readings could disagree.
2. **A placeholder already in a step list declares itself** (`suppliedIn`). Without this the
   SECOND edit of an edited routine refused: the panel re-renders the stored steps to check they
   reproduce the saved source, and `{{message}}` with nothing declaring it rendered to a credential
   read, so the routine appeared not to agree with itself. Nothing new is stored and no format
   changed — the placeholder was always the fact, and this is the reading of it.

`suppliedIn` leaves a credential's name UNDECLARED, because declaring one changes what is rendered
(a global secret becomes a route-qualified one) and a routine already on disk with `{{password}}`
typed in it would stop matching its own source and become uneditable.

## Three sites, one owner

The defect above was fixed twice before it was fixed once. `codegen` keeps a `{{name}}` in a typed
literal only when the name is DECLARED, and THREE places render a stored step list:

1. the step panel's own render of the edited list — fixed first, by returning the names the edit
   declared;
2. the step panel's BASIS check, which re-renders the stored steps to confirm they still reproduce
   the saved source — fixed second, because without it the second edit of an edited routine
   refused;
3. the REBUILD review's today's-rules arm — found third, by the regression test the Architect asked
   for, rendering `do OS's Secret with "message"` as the version it offered to replace the routine
   with.

Three fixes of one thing is a sign the question has no owner, so `declaredFor` is it: a routine's
name without its arguments, and every argument a render of these steps must declare — the name's
own, plus the placeholders the steps already hold. A fourth render asks it instead of
rediscovering it.

## And the two rebuild arms do different things to an edit

Worth writing down because the first version of the test asserted the wrong one:

- **Today's rules** rebuilds from the stored step ACCOUNT when there is one, and the account is
  what an edit changed — so a supplied value SURVIVES. `DiscardsEdits` is false.
- **Simplify typing** re-derives from the DEMONSTRATION, which is never edited, so the fixed text
  comes back and the routine asks for nothing. `DiscardsEdits` is true.

Both are correct and they are not the same, which is exactly why a person has to be told which one
they are looking at — and they were not.

## A sentence the contract wrote and the window never asked for

`orchestrator.Prepared.StepsWarning` has existed since step editing did:

> This rebuild replaces the stored steps with a fresh simplification of the demonstration.

Studio's review carried `Caution`, which is about WHICH REBUILD was chosen, and nothing about the
steps. So somebody who had made a step's text into a supplied value, then pressed Simplify… on the
saved routine, would get back the recording with the fixed text in it and the argument gone —
having been told nothing beforehand. It is on the review card now.

Found while writing a regression test about something else entirely, and it is the pattern this
repository keeps finding: complete code, produced and never invoked, which is why a wiring test
has to enter through the production constructor rather than call the thing it is about.

## Simplify… on the unsaved recording review

The ruling's second half. The saved routine's card has offered a simplification since there was
one, and `marco learn`'s terminal review has offered it for longer — so the one surface where
somebody LOOKS at a demonstration they just made, with the steps in front of them and Save under
their hand, was the only one that could not simplify it. They had to save it and then simplify the
saved routine: two writes to get the version they wanted.

`orchestrator.Deps.SimplifiedDraft` is the one owner, and the terminal review now goes through it
too. The review keeps the ORIGINAL rather than the previous version, because simplifying twice
still leaves one thing a person wants to return to and it is what they did. Nothing is written by
either button; Save writes whichever version is on screen, which is the rule it already followed.

**A known limitation, recorded rather than hidden — and fixed on 2026-10-31:** a routine saved
from a SIMPLIFIED draft could not be step-edited afterwards. The panel edits only a routine whose
source is what its own account renders, a fresh save wrote no account, so the panel re-derived from
the demonstration — which a plain save matches by construction and a simplified one does not. It
was not new (the terminal review has been able to save a simplified routine for far longer than the
panel has existed) but this was the first surface where both were reachable in one sitting, and a
UI audit found it in the middle of the journey rather than at the end of a test file.

The fix named here is the one that was made: a save writes its own step account, in the save path
every surface shares. A simplified draft carries the rules it was rendered under and the save puts
them on disk beside the routine; a plain save still writes nothing, so every routine already in a
library is still edited by rebuilding from its own demonstration. See
[[ADR-279-a-simplified-save-keeps-its-own-basis]].
`TestASimplifiedSaveCanBeStepEditedAndSavedAgain` walks Simplify → Save → Edit → Save again
through the window's own entrances; it is the renamed test that used to assert the refusal.

## Two things declined rather than written

**A guard against a `{{name}}` colliding with a secret ARGUMENT of the same routine.**
`routes.Arguments` reports placeholders first and secrets after and drops a name it has seen, so a
`{{token}}` beside a `do OS's Secret …:token` would make the secret vanish from the list and offer
somebody a text box for their password. Real, and unreachable here: `macroir` has no step kind for
a secret, so a routine this editor will touch contains none. A guard for it would be one that
cannot fire, which reads to the next person exactly like an enforced rule — two of those were
written and removed in the slice this feature serves. The paragraph saying what to add is at the
site.

**Two of my own test cases**, asserting that a step cannot be both "supplied" and "given a
duration". A step is a Type or a Wait and never both, so one of the two operations is refused for
the step's kind before any collision could arise: the cases passed on the wrong refusal and read
as though the collision were checked.

## Enforced by

**The operation** — `internal/orchestrator/stepargument_test.go`:
- `TestTheTextAStepTypesBecomesSomethingSupplied` — the literal becomes a placeholder, nothing
  else moves, and the review says what the text WAS.
- `TestAnEditedRoutineTakesTheNameItWasGiven` — asked of `routes.Arguments`, which is what every
  surface reads, rather than of the source text.
- `TestASuppliedNameIsLowercased`, `TestANameThatWouldNotBeReadBackIsRefused` (nine shapes),
  `TestANameTheGeneratorMeansAsACredentialIsRefused` (all thirteen words, and an ordinary name
  beside them).
- `TestOnlyAStepThatTypesTextCanBeSupplied`, `TestTextAlreadySuppliedIsNotSuppliedAgain`,
  `TestAStepIsNotBothSuppliedAndSomethingElse`, `TestSupplyingAStepThatIsNotThereIsRefused`.
- `TestAPlaceholderInTheStepsDeclaresItselfExceptACredential` — `suppliedIn`'s two rules, including
  that an `Activate` step's application name is not read as an argument.
- `TestTheOtherTwoOperationsAreUnchanged` — the control: a third case in a switch is a chance to
  have changed the other two.

**The window** — `plugins/studio/stepargumentacts_test.go`:
- `TestSupplyingAStepsTextMakesTheRoutineAskForIt` — the end-to-end claim: edited, saved, and the
  field appears beside Play, with Play refusing until it is given a value.
- `TestTheEditedRoutineTypesTheValueRatherThanReadingACredential` — reads the SOURCE that would be
  written, which is the only thing that could have caught the defect above.
- `TestAnEditedRoutineCanBeEditedAgain`, `TestTheStepPanelSaysWhichStepTypesText`,
  `TestAnAlreadySuppliedStepSaysSoInsteadOfOfferingAgain`,
  `TestTheWindowRefusesANameThePlaceholderCannotHave`.

**The page** — `plugins/studio/stepargumentpage_test.go`:
- `TestOnlyAStepThatTypesTextOffersToBeSupplied`, `TestAnEmptyNameIsNotSent`,
  `TestTheSummaryNamesTheSuppliedStepAndNotARemoval` — the last because the summary was a ternary
  that read "step N is removed" for anything that was not a wait, so this operation would have
  told somebody their step was being deleted.

## The mutation gate

DISTINCT MUTATIONS, not applications. A re-asked survivor is the SAME mutation and counting it
twice inflates the total — which the first version of this table did, reporting 21 of 21 by adding
four re-runs to seventeen mutations.

| round | distinct | killed | survived |
| --- | --- | --- | --- |
| the operation | 17 | 14 | 3 |
| the operation, re-asked | — | 3 | 0 |
| the rulings | 14 | 11 | 3 |
| the rulings, re-asked | — | 1 | 2 |
| the unrepresentable arm | 4 | 3 | 1 |

**35 distinct: 33 killed, 1 equivalent, 1 a nil check kept on purpose.**

The operation's three survivors were the script running the wrong suite for a page-side claim, and
one of its fourteen kills was a COMPILE ERROR first — `needed` declared and not used — and re-done
as a behaviour change, because a mutation the compiler rejects says nothing about the tests.

Of the rulings' three, one was real and serious: simplifying a draft that TAKES text handed
`codegen` the bare name, so its `{{message}}` rendered as a credential read. No test simplified a
draft that took anything, and `TestSimplifyingADraftThatTakesTextKeepsItTyped` is now the one that
does — which also exercises the OLD declaration path (the phrase's `with message` clause and the f9
tap) that this whole slice exists to give somebody an alternative to.

**One is EQUIVALENT**, written down rather than papered over: `declaredFor` dropping the name's own
arguments changes no render, because an argument the name declares and the steps do not hold
appears in no literal to be declared for.

### And one was UNTESTED, which was a gap and not a curiosity — now closed

`an unrepresentable simplification is offered anyway` — deleting this arm of [Acts.SimplifyDraft]:

```go
case next.Unrepresentable != "":
    return "a simplified version cannot be saved as it stands, so it is not offered: " + …
```

**What it changes:** a simplified draft `codegen` will not write down is ACCEPTED and swapped in as
the version under review, displacing the original. The card then shows steps somebody can read and
approve, and Save refuses them — a window offering a press the door declines, with the draft they
had already taken away.

**It is inside an approved requirement.** The ruling for this surface was *preview a simplified
draft, allow returning to the original, write nothing until Save*; previewing a version that cannot
be saved fails the first clause.

**And it is testable, with a fixture that already exists.** `internal/orchestrator/exitgesture_test.go`
builds the state: a held `ctrl` stranded across the window boundary makes a draft unrepresentable.
The arm is reachable rather than theoretical, and the terminal review's own comment says why — *a
refusal cannot arrive here that was not already true above… what gets refused is a property of the
EVENTS* — so a review holding an unrepresentable draft, which Studio shows (`capUnsupported` exists
for exactly that), simplifies to the same refusal.

**Closed by `TestAnUnsavableSimplificationIsRefusedAndKeepsTheOriginal`**, which asks the three
things that have to be true at once: the reason is shown and names the key that was held, the
recording under review is unchanged (same steps, not marked as a simplification, and no way back
offered to a draft that was never replaced), and nothing is written. Three mutations kill it now —
deleting the arm, swallowing the reason, and dropping the generator's own words from it.

Two things the fixture taught, both corrections to this paragraph's earlier claims:

- **The demonstration cannot be saved either.** A held key stranded across the boundary is a
  property of the EVENTS, so every re-derivation of them refuses. That does not weaken the claims:
  what they are about is a refused simplification not displacing whatever is under review, which
  holds either way. It is asserted rather than assumed now.
- **The card was not silent about it.** The first version of the test read `Recording().Unsupported`
  — which is `codegen.Unpropagatable`, a different question — found it empty, and concluded the
  draft was fine. The draft's own refusal is in `Said`, and the card shows it. There was no defect
  there; there was a test reading the wrong field.

**And it left one survivor, kept on purpose:** `case next == nil`. Unreachable through this path,
because `draftToSave` has already refused a nil draft and `SimplifiedDraft` returns nil only for a
nil input. It stays because it is a NIL CHECK BEFORE A DEREFERENCE of a pointer-returning function
whose own documentation says it may answer nil — a different thing from the two logic guards this
slice removed for being unreachable, and the failure it would prevent is a panic in somebody's
window rather than a rule going quiet. Written down rather than tested, because the honest test
would have to construct a state the callers make impossible.

## A skip that fired, and what it hid

`cmd/gate` reported one finding on this work: an UNDECLARED SKIP in
`TestAnEditedRoutineCanBeEditedAgain`. The test reached for a wait to change as its second edit,
found none — the fixture is an activate and a type, and the trailing 250ms is `codegen`'s settle
rather than a step — and skipped.

So it was passing while making no claim at all, and the claim was the one the whole render fix
existed to carry. The answer was not to declare the skip: the fixture now types TWO runs and the
second edit supplies the second one, which also asserts that both placeholders render as `Type`.

This is what the ledger rule is for, and it caught exactly what it was written to catch.

## What this does NOT establish

- **No desktop.** Nothing here has been exercised by a hand.
- **It does not remove the other two ways in.** The learn phrase's `with name` clause and the f9
  key both still work and are still undocumented on every surface. This slice adds a discoverable
  way; it does not make the other two discoverable.
- **It does not offer the reverse.** A supplied step cannot be turned back into fixed text, because
  the literal is gone once the routine is re-rendered. Nothing offers what it cannot do.
- **It replaces the WHOLE of a step's text, not part of it.** The format could hold a placeholder
  beside fixed text — `Dear {{name}},` is a thing a routine can type, and `codegen` splits a mixed
  literal correctly — but choosing part of a step's text is not something this offers. Said on the
  control and in the panel's own note, because a control that looked as though it might would be
  read as offering it.
- **Observed while running the full suite, unrelated to this change:**
  `internal/director/service TestApprovingSomethingThatWasNeverExtractedFails` failed once on
  `bind: An operation on a socket could not be performed because the system lacked sufficient
  buffer space`, with Studio and its Director running beside a heavy test run; it passed alone
  immediately after.

  **UNEXPLAINED, and left that way.** Passing alone does not establish a cause: it is consistent
  with a machine resource limit under concurrent load, and equally with something intermittent in
  that package or in the listener it binds. Nothing here distinguishes them, so nothing here
  claims to — and it was not retried into silence either. The reading to avoid is the comfortable
  one, which is why this paragraph says less than the first version of it did.
