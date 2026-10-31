---
type: adr
status: accepted
date: 2026-10-30
decision: "ACCEPTED — A REQUEST MAY PROPOSE THE TEXT A ROUTINE TYPES, AND A PROPOSAL IS NEVER A SUPPLY. Director asks the configured resolver a THIRD question on the one plugin boundary — one executable named by `$MARCO_RESOLVER`, one JSON in, one out — after it has already selected a routine, and gets back the words of the request that are each argument's value. Nothing acts on the answer: a value reaches a surface as a `resolveroute.Proposal` to be shown, confirmed or corrected, and the thing that runs a routine is still the confirmation built in [[ADR-276-a-routine-may-be-told-what-to-type]]. THE PLUGIN ANSWERS IN OFFSETS AND NEVER IN TEXT, and that is the whole safety of it: `resolver.Span` has no field for a value, so an invented, paraphrased, translated, re-cased or tidied value is not something this boundary refuses — it is something the boundary cannot express, and `Span.Value` copies the words out of Marco's own copy of the request, VERBATIM. Every answer is validated per span and a bad one is refused rather than repaired: a name nobody asked about, the same name twice (NEITHER is taken, because choosing between them would be guessing on a model's behalf), an offset outside the request, `start >= end`, an offset off a rune boundary, a span that is only whitespace. A SECRET IS NEVER ASKED ABOUT — it is left out of the question rather than filtered out of the answer, because asking would send somebody's password to a model and answering would carry it back and put it in a field on a card; `do OS's Secret …` at run time remains the only path. A PROPOSAL TRAVELS WITH THE REVISION ITS NAMES CAME FROM (`Answer.SourceAt`, the digest of the source `Takes` was read from, off the SAME read), so a routine saved again before a confirmation is prepared can be refused rather than crossed into — even when the new version declares the same names, because the same name in a rewritten routine may be typed somewhere else entirely. AND A PROPOSAL FOLLOWS THE IDENTITY: proposals are read from the candidate the answer offers, which is the zero Candidate when it offers none, so a question carries no values by the same value they would have been read from. THE PLUGIN IS PART OF THE DELIVERY, because extending a protocol is not extraction: `plugins/llama` answers the third question by asking the model to QUOTE THE WORDS — a 3B model cannot count bytes and one asked for offsets answers with numbers that are confidently wrong — and then LOCATING those words in the request, so invention dies twice, independently, and the wire still carries only offsets. 23 of 23 applied mutations killed across three rounds on the question, 5 of 5 on Director's wiring, and the probe reached a real local model: `message=hello there`, verbatim. TWO UNREACHABLE GUARDS OF MY OWN WERE REMOVED rather than kept, and one test was corrected that had never reached the arm it named. THE CONFIRMATION PATH DERIVES THE PROVENANCE RATHER THAN TAKING IT: a surface says what it OFFERED and what came BACK, and `orchestrator.PrepareProposed` compares them — a proposal left alone is `extracted`, one that differs is `edited`, one nobody proposed is `typed` — so a window that prefilled a box and forgot to relabel it cannot tell somebody a model read their sentence correctly when they are the one who fixed it; and the test is PRESENCE, so clearing a prefilled box is a correction and an empty proposal is still a proposal. The provenance is SEALED with the values, because a label changed after the confirmation was read is the same class of thing as a value changed after it, and that label is the only thing saying whose words these are. STUDIO PREFILLS AN EDITABLE BOX for the current request only, says beside it where the text came from, repaints that clause on the keystroke rather than on a rebuild, and sends BOTH what it offered and what came back plus the revision — and a reading belongs to ONE routine and ONE request: another routine's field of the same name is left empty, a new request drops the boxes that came OF the last reading while leaving a person's own typing alone, and an answer that selects nothing clears it too. 20 of 20 mutations killed on the contract, 23 of 23 on the window and the page, and FOUR defects were found by mutations rather than by failing tests: a page clear that destroyed hand-typed values, a confirmation card left on screen with a live Run button after the state behind it was dropped, a hidden card keeping its old lines and an enabled button, and two clears that disagreed with one another a line apart. AND A SURFACE THAT CANNOT REVIEW A PROPOSAL SAYS SO: the CLI, a hotkey and the overlay's command line — which the voice path arrives through — have no field and no confirmation step, so a routine that needs text and was given none is REFUSED with an actionable sentence naming the argument and where to settle it, rather than run with a guess or failing quietly. Recognised by the refusal's TYPE (`routes.ValuesNeeded`) and not by its wording, and gated on whether anything CONFIRMED (`expect`, whose own documentation already says that empty means the CLI, a hotkey and the overlay pinned nothing) rather than on a list of surface names that would have to be kept right — so Studio is never told to open Studio, `Request.Source` stays recorded-and-never-consulted, and the other three things `CheckArgs` says keep their own reasons. 13 of 13 mutations killed, one of them a REAL survivor first: nothing had checked that the clause belongs to one refusal, and sending somebody to review text when they have misspelt an argument is sending them to look at the wrong thing."
supersedes: []
affects:
  - execution
source_paths:
  - internal/routes/args.go
  - cmd/marco/panicstop.go
  - internal/resolver/extract.go
  - internal/resolveroute/resolveroute.go
  - cmd/director/resolveroutewiring.go
  - plugins/llama/extract.go
  - plugins/llama/main.go
  - internal/routes/args.go
  - internal/orchestrator/playvalues.go
  - plugins/studio/playvalues.go
  - plugins/studio/frontend/index.html
---

# A request may propose the text a routine types

The trace, the alternatives, the four amendments and the rules this was built to are
[[Extracting-argument-values-proposal-2026-10-30]].

[[ADR-276-a-routine-may-be-told-what-to-type]] made a routine able to accept text and made Studio
able to supply it, with the value sealed into a confirmation beside the route and the source.
What it deliberately left out was the half a person would reach for first: somebody who says
*write a note saying hello there* has already said what to type, and was then asked to type it
again into a box.

This is that half, and it is one question on a boundary that already existed.

## The third question on the one plugin boundary

There is exactly one boundary between Marco and a model: one executable named by
`$MARCO_RESOLVER`, one JSON object in on stdin, one out on stdout. `Interpret` records at length
why a second boundary would be a second thing to configure, a second thing to build, a second
place for a key to live and a second answer to *is a model wired up*. So this is a third question
on that protocol, identified by a FIELD rather than by a mode word:

```
→ {"input":"write a note saying hello there","extract":{"names":["message"]}}
← {"values":[{"name":"message","start":20,"end":31}]}
```

A plugin that knows only how to pick a route, or only routes and verbs, sees a field it does not
recognise and answers as it always did; the caller is told plainly that nothing extracted
anything, which is the same answer an unconfigured machine gives.

**It rides `$MARCO_RESOLVER` and not `$MARCO_INTERPRETER`.** Two variables, separate on purpose:
the second names the plugin that would answer which observed CONTROL was meant, and
`internal/director/meaning` — the question held in shadow mode — is its only reader. Configuring
extraction therefore cannot configure the shadow interpreter, and
`TestTheInterpreterIsNotWiredIntoAnything` stays green because nothing here goes near it.

**What the plugin is told is the words and a closed list of names.** Nothing about the screen —
not a control, not a window, not a coordinate, not what Marco remembers — because a value is a
span of the person's own sentence, so the evidence a screen question would need is deliberately
absent and a plugin that wanted to send a desktop somewhere has nothing here to send. And nothing
about the routine beyond the NAMES of what it takes: a plugin that knew what the routine did could
start answering a different question.

## Offsets, never text, and that is the whole safety of it

`resolver.Span` has no field for a value. There is nowhere for a plugin to put a word Marco did
not send it, so an invented, paraphrased, translated, re-cased, trimmed or tidied value is not
something this boundary refuses — it is something it **cannot express**. `Span.Value` is the one
place a value comes into existence, and it reads the bytes out of Marco's own copy of the request.

`internal/director/values/provenance.go` is the shape being followed, and says why: everything
there is *designed to be safe rather than redacted to be safe*, because a struct with a plaintext
field has to be scrubbed at every boundary and will eventually be missed at one.

The discriminating case is a span with valid offsets BESIDE invented text. The offsets are used
and the text is never read, so what comes out is the request's own words — asserted, because that
is the shape a model's answer takes when it has done the job and then helpfully tidied it.

**And verbatim means verbatim.** Not trimmed, not collapsed, not re-cased: a value is used exactly
as supplied, which is already the rule for one that was typed, so a span taking in the space
before the words keeps it. A mutation that trimmed the value, and another that lowered it,
survived every other test in the file — somebody's *Anne* is not asking for *anne* — which is why
there is a test about nothing else.

## Every answer is validated, and a bad one is refused rather than repaired

Each is a refusal of THAT span rather than of the whole answer: one unusable span should not cost
the person the others. An answer with no usable span at all reports nothing extracted, which is
the same as not having asked.

| refused | because |
| --- | --- |
| a name the routine does not declare | there is nowhere for that text to go, and `CheckArgs` would refuse it at the door anyway |
| the same name twice | two spans for one argument is an answer with no reading, and **neither** is taken — keeping the first would be this package picking on a model's behalf, and first in a list is not a judgement |
| an offset outside the request | it indexes something that was never sent |
| `start >= end` | an empty span is not a value; a value that is empty is SUPPLIED, which is a different thing and never reaches this question |
| an offset off a rune boundary | it would slice a character in half, and a model counting characters where Marco counts bytes is the likeliest way to get here |
| a span that is only whitespace | nobody means *type three spaces* by naming a span, and this is what a miscount looks like from outside |

## A secret is never asked about

The hardest rule here, and it is an omission rather than a filter: a secret argument is left out of
the question, so a routine whose only argument is a secret produces no question at all and there
is no answer to have to refuse.

A secret resolves from the credential store at run time — `do OS's Secret …` — and the reason is
that a password must not pass through a routine, a recording or a diagnostic on its way to being
typed. A request is none of those and is worse than all of them: asking would send somebody's
password to a model, and answering would carry it back on an `Answer` and put it in a field on a
card. A span that named a secret anyway is dropped twice: by `Extract`, for naming something nobody
asked about, and by `proposals`, which knows WHY it was not asked.

The library still REPORTS a secret, as a secret (`Candidate.Takes` carries `routes.Argument`), so a
surface knows to send somebody to the credential store instead of to a text box.

## A proposal travels with the revision its names came from

The argument names an extraction was asked about came from one revision of one routine. If that
routine is saved again before the confirmation is prepared, the proposal must be refused and asked
afresh — **even when the new version declares the same names**, because the same name in a
rewritten routine may be typed somewhere else entirely, and a value that crossed into a routine it
was never prepared for is text going to a place nobody proposed it for.

So `Candidate.SourceAt` and `Answer.SourceAt` carry the digest of the source `Takes` was read from,
off the SAME read as the routine's name — one read, so the names and the version cannot be of
different revisions. An answer with nothing usable in it names no revision: a revision carried with
no values is a confirmation prepared for a routine nobody proposed anything for.

The mechanism it hands to is already built and already tested: `Request.Expect` and
`Invocation.SourceAt`, one step along.

## A proposal follows the identity, and the two guards that could not fire

A value beside a question is a value waiting for a routine to attach itself to: whichever routine
the person then picked would arrive carrying text read for a different one's argument.

What holds that is the VALUE, not a condition. Proposals are read from the candidate the answer
offers, and an answer that offers no routine carries the zero `Candidate`, which declares no
arguments — so there is nothing to ask about and nothing to propose, produced by the same value the
proposals would have been read from.

**It was written twice as a condition first, and neither condition could fire.** A clearing in the
ambiguity re-check, and a `Kind == run && ID != ""` test in front of the read. The re-check only
runs when the answer was NOT a run, so it can never be undoing a run's values; a mutation deleting
either one survived the whole suite. An unreachable guard reads exactly like an enforced rule — to
a reader and to a test alike — which makes it worse than the absence of one. Both were removed and
the reason written at the site, including the part that now stands on the shape of the function
rather than on an assertion: if a later edit reads a value before the re-check, or lets `said.Kind`
change after the switch, this decision needs making again rather than extending.

## The plugin is part of the delivery, and it is asked for words

Extending a protocol is not extraction: a question nothing answers is a field in a struct. So
`plugins/llama` answers the third question — and the way it answers is the one interesting thing
in it.

A 3B instruct model cannot count bytes. One asked for `start` and `end` answers with numbers that
are confidently wrong, which, validated, means nothing is ever extracted. So the model is asked for
the thing it is good at — **quote the words** — and the plugin LOCATES those words in the request
and reports where it found them. A value it cannot find is not reported.

Which means invention dies twice, independently and for the same reason. A model that paraphrased,
corrected the spelling or made something up produces words that are not in the sentence, so there
is nowhere to point and no span to send; and if a span were sent anyway, the engine reads the text
out of its own copy and never looks at anything the plugin said about it.

Locating tries the words, then the words with the quotes and spaces a model wraps them in stripped
off, each exactly and then case-insensitively — and a fold that CHANGES LENGTH is not matched at
all, because an offset into a lowered string names other bytes in the original. There is no further
attempt that relaxes into a resemblance: *near enough to something they said* is how a value nobody
said gets typed somewhere.

## The confirmation path, and why the provenance is derived

`orchestrator.PrepareProposed(rt, values, proposed, proposedAt)` is `PrepareInvocation` with two
more facts: what a request PROPOSED, and the revision those proposals were read from.

**The provenance is derived, never taken on trust.** A surface says what it offered and what came
back; the contract compares them. A name in `proposed` whose value is unchanged is `extracted`; one
that differs is `edited`, because the person corrected it and it is theirs now; a name that was
never proposed is `typed`. The alternative — a surface asserting the label — fails in exactly one
direction and it is the direction that matters: a window that prefilled a box and forgot to relabel
it would tell somebody a model read their sentence correctly when they are the one who fixed it.

**And the comparison is on PRESENCE.** `v, ok := proposed[name]`, so clearing a prefilled box is a
CORRECTION rather than an absence, and a proposal of the empty string is still a proposal. This is
[[ADR-276-a-routine-may-be-told-what-to-type]]'s own rule reaching one layer further; a mutation
that tested the length instead survived until the empty-proposal cases were written, and the first
version of that test had none.

**The provenance is sealed with the values.** A label changed after the confirmation was read is
the same class of thing as a value changed after it — what the struct says it showed is not what it
now holds — and it matters more than it looks, because that label is the only thing telling
somebody whether a model or they themselves produced the text they are about to send.

The four words live in `internal/routes`, beside `routes.Argument`: `internal/orchestrator` and
`internal/resolveroute` do not import one another and both import that, and a vocabulary kept in
two places is two vocabularies the first time either changes. Their distinctness is held by the
COMPILER — `saidWhere` branches on all four, so two of them being the same string is a duplicate
case in an expression switch — which is a stronger guarantee than a test and is named as the
compiler's, because it lasts only as long as one function happens to branch on all four.

## Studio's box, and what it is not

The field beside Play is prefilled from the answer, editable, and labelled; the clause is repainted
on every keystroke rather than when the fields are next rebuilt, so it cannot go on saying a model
read the sentence while somebody types over it. `ReviewPlay` carries what was offered, what came
back, and the revision — one door, not a second method beside the plain one.

**A value SOMEBODY TYPED gets no clause.** They are reading their own text back, in the one place
that has to be easy to check, and a line telling them they typed it is noise. The two that do get
one are the two they could mistake for their own: a reading of their request, and that reading
after they corrected it.

**Prefilling is not the thing the last slice refused.** That slice refused REMEMBERING — a value
from a previous run reappearing, which would be a property of the routine and would get confirmed
by somebody who never typed it. This is a reading of the request in front of them, put where they
can see and change it, and it lives exactly as long as that request does. The page comment that
said *nothing is prefilled, here or ever* now states the distinction instead of being quietly
contradicted.

**A reading belongs to one routine and one request.** Another routine's field of the same name is
left empty and unlabelled, because however alike the names, a reading of one routine's arguments
beside another's would put text read for somewhere else in front of somebody as though they had
asked for it. And a new request drops the boxes that came OF the last reading — the prefill, or
that prefill after a correction — while leaving alone anything the person typed into a field no
reading ever proposed for. An answer that selects nothing clears it too, which is why the clearing
runs before the asking rather than where a new reading is taken.

## Four defects a mutation found and no failing test did

Worth recording separately, because all four were cases of a rule being stated and not held:

1. **A page clear that destroyed hand-typed values.** Two clears, a few lines apart, disagreeing:
   one carefully dropped only the boxes a reading had filled, and the other emptied the lot.
2. **A confirmation card left on screen after the state behind it was dropped.** `willType = null`
   with nothing repainting from it — the rule was in the comment and the window did not obey it, so
   a new request left a live Run button over a confirmation of something else.
3. **A hidden card keeping its old lines and an enabled button.** `drawWillType` hid the container
   and returned, leaving the last confirmation's text inside it and `routineConfirmBtn` enabled.
   Out of reach behind the hidden parent, so not a press anybody could make — and state that comes
   back the moment anything shows the container again.
4. **A test asserting arity as a proxy.** `TestThePageSendsValuesOnceAndConfirmsByToken` checked
   that the review took two arguments. It now checks the whole call, including that an ordinary
   press proposes nothing and names no revision, which is the claim the count was standing in for.

## A surface that cannot review a proposal says so

A proposal is only useful where somebody can see it and correct it before anything is typed. Three
entrances have no field and no confirmation step — the CLI, a hotkey, and the overlay's command
line, which the voice path arrives through — and for a routine that needs text and was given none
they could run with a guess, fail quietly, or EXPLAIN. The first is what every boundary in this
slice exists to prevent, the second is worse than a refusal, so it is the third:

> "write a note": this routine types text that has to be supplied, and no value was given for
> "message". This request needs text values reviewed before running. Open it in Studio, or supply
> the value with the request

**One sentence, in one place, for all three.** The overlay and the voice path spawn `marco` and
stream its output into the HUD, so the engine's door is the only site; there is no second copy of
the rule in the overlay, and what holds the two modules together is a test in each — the door's
exact words asserted in `cmd/marco`, and the sentence reaching the HUD asserted in
`plugins/overlay` against a real child.

**Recognised by its type, not its wording.** `routes.ValuesNeeded` carries the argument names, and
the clause is added on `errors.As`. Matching the sentence would have worked until somebody improved
it, and the thing that broke would have been a surface going quiet.

**Gated on whether anything CONFIRMED, not on which surface asked.** `expect` is the digest an
invocation was authorised against, and `invoke.Request.Expect` already documents what empty means:
*the CLI, a hotkey and the overlay pin nothing because nothing was confirmed to them*. So empty IS
the condition, read off the request rather than off a list of surface names that would have to be
kept right — and Studio, which sends one, is never told to open Studio. `Request.Source` is
deliberately not consulted: it is recorded and never read, and this was not the place to start.

**And it belongs to ONE of the four things `CheckArgs` says.** The other three are wrong at every
entrance — a name the routine does not take is a name it does not take in Studio as much as in a
terminal — so a misspelt argument, a pasted line break and more values than the routine has places
for keep their own reasons. A mutation that dropped the type check and kept only the confirmation
gate survived everything else, which is how that came to be tested.

**What does not change:** a value supplied explicitly still runs, positional or named, with nothing
said about reviewing it; and a routine that takes no arguments is an ordinary routine, which is
every routine saved before any of this existed. `marco dispatch` is untouched because it classifies
without acting, so it never reaches this refusal.

## Enforced by

**The question and its answer** — `internal/resolver/extract_test.go`:
- `TestASpanIsCopiedOutOfTheRequest` — a span becomes a value by being copied out of the request.
- `TestAValueIsCopiedVerbatimAndNeverTidied` — a span with a space in it keeps it, and a
  capitalised value keeps its capitals. Written because mutations that trimmed and lowered the
  value survived every other test here.
- `TestAPluginCannotAnswerWithTextOfItsOwn` — text instead of offsets, text under another field
  name, and the discriminating case: valid offsets beside invented text, where the offsets win.
- `TestABadSpanIsRefusedRatherThanRepaired` — all eleven refusals in the table above, plus a
  route-only and a verb-only plugin's answer, unparseable output and silence.
- `TestASpanOffARuneBoundaryIsRefused` — a span starting mid-character, against a request whose
  bytes and runes differ, with the aligned span passing beside it.
- `TestAGoodSpanSurvivesABadOneBesideIt` — one unusable span does not cost the person the others.
- `TestThePluginIsToldTheWordsAndTheNamesAndNothingElse` — asserted on the bytes that reached the
  subprocess, including that the route and verb fields are ABSENT rather than empty-but-present.
- `TestAPluginThatHangsExtractsNothing`, `TestACancelledQuestionExtractsNothing`,
  `TestNoPluginExtractsNothing` — one honest answer for every failure.
- `TestAQuestionWithNoNamesOrNoWordsIsNotAsked` — checked on whether the plugin RAN.
- `TestExtractionDoesNotReadTheInterpretersVariable` — an interpreter alone is not an extractor.

**The policy** — `internal/resolveroute/proposals_test.go`:
- `TestARequestsOwnWordsAreProposedForTheArgument` — the value, the provenance, the revision, and
  that the request asked about is the one the spans index.
- `TestTheProvenanceOfAProposedValueIsSaidAsExtracted` — against the literal, because every other
  test compares with the constant and would read the same if the constant said something else.
- `TestASecretIsNeverAskedAboutAndNeverProposed`, `TestASecretIsLeftOutOfAQuestionItSharesWithText`,
  `TestASpanNamingASecretIsNotProposed` — nothing is asked, nothing is answered, nothing is kept.
- `TestProposalsAreInTheOrderTheRoutineDeclares`,
  `TestAnArgumentNothingAnsweredForHasNoProposal`,
  `TestAnAnswerWithNothingUsableInItNamesNoRevision`.
- `TestNothingIsAskedWhenThereIsNothingToAskAbout` — a routine that takes nothing, a refusal, and
  no extractor at all, with the routine still selected.
- `TestAQuestionCarriesNoProposals`, `TestOnlyARunCarriesProposals`,
  `TestProposalsDoNotMakeAnIneligibleRoutineAnswer` — a proposal follows the identity.
- `TestExtractionChangesNothingAboutTheDecision` — the whole answer, compared with the question
  wired up and absent.

**Director's own reach** — `cmd/director/extractwiring_test.go`:
- `TestDirectorsOwnHandlerReachesTheExtractionQuestion` — through `Runtime.ResolveRoute`, with a
  real plugin executable the test builds, because a stand-in establishes the policy and nothing
  about whether the production path reaches it. The mutation that nils the production line was
  killed only by a COMPILE ERROR before this existed.
- `TestWithNoPluginDirectorProposesNothing` — the other half: the values came from the plugin.
- `TestTheLibrarySaysWhatEachRoutineTakes` — the names and the revision, off one read, with a
  secret reported as a secret.

**The plugin** — `plugins/llama/extract_test.go`:
- `TestExtractQuotedWordsBecomeASpan`, `TestExtractInventedWordsAreNotReported` (ten shapes),
  `TestExtractRecasedWordsComeBackAsWritten`, `TestExtractStripsTheModelsOwnPunctuation`,
  `TestExtractChattyReply`, `TestExtractAnswersWhatItCanAndLeavesTheRest`,
  `TestExtractReportsSpansInTheDeclaredOrder`, `TestExtractWithNoNamesOrNoWordsAsksNothing`
  (checked on whether the model was reached), `TestExtractIgnoresABodyThatCameWithAStatus`,
  `TestTheAnswerCarriesOffsetsAndNeverText`, `TestLocateDoesNotFoldWhenFoldingMovesTheBytes`.

**The confirmation path** — `internal/orchestrator/proposedvalues_test.go`:
- `TestWhereAValueCameFromIsDerivedFromWhatWasProposed` — seven cases, written with an explicit
  `offered` flag rather than inferring it from the value being empty, which is the defect itself.
- `TestClearingAProposalIsACorrectionAndNotAnAbsence`,
  `TestTheFourProvenanceWordsAreFourDifferentWords`,
  `TestTheConfirmationSaysWhereAValueCameFrom` (including that a typed value carries no clause and
  that a correction does not show the words it replaced).
- `TestAProposalForAnOlderRevisionIsRefused` — the same argument name on a rewritten routine, which
  a check on the names would pass; the refusal names the routine and not the value, and the same
  values against the current revision run.
- `TestAnOldRevisionIsSaidBeforeAnUnknownArgumentIs`, `TestTheProvenanceIsSealedLikeTheValues`,
  `TestASecretCarriesNoValueAndNoProvenance`, `TestWithNothingProposedEveryValueIsTyped`.

**Studio's window** — `plugins/studio/proposedvaluesacts_test.go`:
- `TestTheWindowConfirmsAProposalAsARecordOfWhatWasSaid`,
  `TestTheWindowConfirmsACorrectionAsTheirOwn`,
  `TestTheWindowRefusesAProposalForAnOlderRevision`,
  `TestAnOrdinaryPressCarriesNoProposalAndNoClause`.

**Studio's page** — `plugins/studio/proposedvaluespage_test.go` and
`plugins/studio/proposalclearedpage_test.go`:
- `TestAProposalFillsTheBoxAndSaysWhereItCameFrom` — filled, labelled, and Play LIVE, because a box
  that looked filled while nothing stood behind it would leave the button dead with the text in
  front of somebody.
- `TestCorrectingAProposalIsSaidImmediately`,
  `TestPlaySendsTheProposalAndTheRevisionBesideTheValues`,
  `TestAFieldNothingProposedForIsTheirsAlone` (the half that keeps the drop rule honest),
  `TestAProposalDoesNotReachAnotherRoutinesFieldOfTheSameName`,
  `TestASecondRequestDoesNotInheritTheFirstsProposal`,
  `TestAnAnswerThatSelectsNothingClearsTheLastProposal`,
  `TestANewRequestTakesTheConfirmationCardWithIt`.

**The surfaces that cannot review one** — `cmd/marco/valuesneeded_test.go`, through
`runInvocation`, which is what `marco do` and `marco hotkey` both reach:
- `TestASurfaceWithNoReviewSaysWhereToReviewIt` — explained, nothing pressed, nothing typed, and
  the argument named.
- `TestOnlyAMissingValueIsSentToStudio` — the other three refusals keep their own reasons. Written
  because the mutation that dropped the type check survived everything else.
- `TestAnExplicitlySuppliedValueStillRunsAndIsNotSentToStudio`, `TestAPositionalValueStillRuns`,
  `TestARoutineThatTakesNothingRunsWithNothingSaidAboutReviewing` — the two things that must not
  change.
- `TestAConfirmedRequestIsNotToldToOpenStudio` — an entrance that pinned a version is one that
  already showed somebody what the routine would type.
- `TestTheEngineStillSaysWhatThisExpects` — the door's exact words, because another module holds a
  copy of them.

**And the overlay, which is also the voice path** — `plugins/overlay/valuesneeded_test.go`, through
the production streamer and a real child:
- `TestTheHudShowsWhereToReviewAValueItCannotAskFor` — the whole explanation reaches the HUD.
- `TestTheHudSaysNothingAboutReviewingAnOrdinaryRun` — and an ordinary run gains nothing.

**The live probe** — `cmd/director/resolveroutelive_test.go`, behind `-tags resolver`:
- `TestTheRealPluginAnswersTheExtractionQuestion`.

## The mutation gate

| round | applied | killed |
| --- | --- | --- |
| the question (`internal/resolver`) | 16 | 13 |
| the question, round two | 10 | 6 |
| the question, round three | 6 | 6 |
| the policy (`internal/resolveroute`) | 13 | 11 |
| the policy, after the restructure | 5 | 4 |
| Director's wiring | 5 | 5 |
| the confirmation path (`internal/orchestrator`) | 15 | 13 |
| the confirmation path, round two | 5 | 5 |
| the window and the page | 17 | 13 |
| the window and the page, round two | 8 | 8 |
| R8, the surfaces with no review | 13 | 13 |

Four survivors of the first two rounds were real gaps and were closed with tests: the verbatim
rule (twice — trimming and lowering), a length-changing fold, a question that was never sent, and a
model server answering with a status. Two more led to the restructure above. **Written down and not
closed, because they are redundant rather than untested:** `path == ""` in `Extract`, which
`exec.Command("")` would fail on anyway; `len(resp.Values) == 0`, which the final
*nothing usable* check already covers and which IS killed; and a mutation that reads proposals a
second time before the ambiguity re-check, whose result the real read overwrites — an equivalent
mutant, not a gap.

## A test that had never reached the arm it named

`TestOnlyARunCarriesProposals` handed the resolver a learn, a chat and a clarification, and
asserted that none of them carried values. It passed, and a mutation that handed a near miss the
library's first routine survived it.

`dispatch.Decide` VALIDATES a decision: a chat or a clarification with nothing to say is refused,
and so is a learn offer with no name. The test supplied neither, so all three decisions were
REJECTED and fell through to the deterministic arm — three rejections, asserted about as though
they were three kinds of answer. It now supplies what each kind requires and FAILS FIRST if the
answer is not the kind it was given, because a rejection proves nothing about the kind.

## What this does NOT establish

- **Three of the four provenance words are in use.** `extracted`, `edited` and `typed` are
  produced; `parsed` is declared and belongs to the deterministic grammar one door along, in
  `cmd/marco`'s intake, which this slice did not touch.
- **Nothing here is a measurement of comprehension.** The probe asked one deliberately easy
  sentence and got `message=hello there` back, verbatim. That is a connection: whether a model
  reads sentences WELL needs a corpus and a denominator, which is what
  [[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]] cost, and one
  sentence answered correctly is the most misleading possible evidence about it. The route half of
  this same plugin reads a second paraphrase as a request to teach a new command, three times out
  of three.
- **No desktop.** Nothing in this slice has been exercised by a hand.
- **Observed while probing, and not fixed:** the FIRST resolution after `ollama serve` paid the
  model load against Director's 30s `resolveTimeout` and failed
  `TestAParaphraseReachesTheResolverThroughDirector`; the same test passed in 0.22s once the model
  was warm. The timeout was NOT changed — the honest reading is that a cold first call can exceed
  it, which is a product question about what a surface shows while waiting, not a number to raise
  until a test goes green.
