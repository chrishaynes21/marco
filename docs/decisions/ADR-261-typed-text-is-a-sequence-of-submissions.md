---
type: adr
status: accepted
date: 2026-10-27
decision: "ACCEPTED — TYPED TEXT IS A SEQUENCE OF SUBMISSIONS AND AN ACCOUNT OF HOW FAR IT GOT, AND EVERY SEGMENT OF IT IS A LINK. `do OS's Type with \"hello\"` reads like one thing and is five, and the ones after the first can be refused on their own — so what a failure leaves behind is not 'the typing failed' but a field holding `he`, on somebody's screen, and a routine whose next step was written for a field that says `hello`. Three things were wrong and each was a different kind of wrong. NOTHING WAS ACCOUNTED: `Invoke` dispatched typing through `ok(…)` rather than `delivered(…)`, so a rejected send became a failed step recorded in no account at all, `Unperformed` was empty, and `marco do` reported success about typing that did not happen. NOTHING WAS PROPAGATED: `StepType` was in `emit` and in no arm of `propagating`, so typed text was never a link — whatever followed it ran whether the text arrived or not, which is the keystroke's defect one input along and worse, because a half-filled field changes what the press after it MEANS and the window guard cannot see the difference. AND PARTIAL DELIVERY WAS UNREPRESENTABLE: the loop stopped at the first refusal and returned an error saying neither how much had gone in nor that the question had an answer, while a stop pressed mid-word is the ORDINARY way this happens rather than the exotic one. The sequence is lifted out of the `SendInput` call, as the chord's and the drag's were, so every case is a return value a test can arrange: a refusal before the first character, a refusal midway, a character whose down went in and whose up did not, a stop before typing and a stop between two characters. WHAT IS COUNTED IS SUBMISSIONS AND NOT ARRIVALS — `SendInput` returning a count says the system took the events into the input stream, not that an application received them, that the application with the keyboard is the one the routine meant, or that what arrived was rendered — so the account says `submitted`, and the only account that can say `arrived` is the application's own, which is what the `text/` desktop scenario reads and why it exists. THE UNIT IS A UNICODE CODE POINT, never a Go byte offset and never a UTF-16 code unit: `héllo` is six bytes and five characters, two emoji are eight bytes and four code units and TWO characters, and `4 of 8 characters` about two emoji is not a smaller claim but a wrong one. A code point goes in as a down and an up for each of its code units ALL IN ONE CALL, so nothing interleaves between the halves of one character; the count comes back per event, so a character is SUBMITTED (every event taken), REFUSED (none) or PARTLY ACCEPTED (some) — and the third is reported as uncertainty rather than counted either way, with the release ATTEMPTED for a unit whose down the machine took and whose up it did not. `sendUnicodeChar` cast the rune to `uint16`, so every code point above U+FFFF was TRUNCATED — U+1F600 arrived as U+F600, a private-use character, and `SendInput` took it: a successful send of the wrong character, silently, with no account that could show it. A SECRET IS NEVER MEASURED rather than redacted afterwards: the kind travels into the gesture, so no number about a secret is computed anywhere, and the backend's own error — which QUOTED THE REJECTED CHARACTER — is gone, because for a secret that put one character of a password into a log line. Length and failure position are both facts about the value, and a failure position is a prefix whose size somebody now knows. PARTIAL PROGRESS IS KEPT IN AN ACCOUNT OF ITS OWN AND NOT THE STUCK-KEY ONE: a held key means every keystroke anybody types next arrives modified, and a half-filled field means nothing of the sort — a person told their keyboard may be stuck would be told something untrue. It survives cancellation into the durable record, the run's own output and the Studio card, and the outcome stays `cancelled`, because a stop is not a failure. `input_actions` still counts ATTEMPTS, with the progress beside that number rather than in place of it. EVERY SEGMENT OF A MIXED TEXT IS ITS OWN LINK: one `StepType` emits several sentences — `user{{password}}` is a Type, a Secret and a Type — and a link wraps ONE `do`, so a step emitting three put two of them where nothing could stop at them, which is the original defect surviving in the case where it matters most. `textBySegment` splits the step BEFORE the chain is built, and `typeSentence` is the one place that decides which sentence a segment gets, because two places deciding it is two places to disagree about the route-qualified secret name — and disagreeing there means looking up a secret nobody stored. Existing saved Marco is not rewritten and legacy CONTINUATION semantics are asserted behaviourally: a legacy routine whose typing the machine refused goes on to the next step. The desktop half ran green on 2026-10-27, first time: the application's own field holds the demonstrated text character for character, and the press after the typing arrived after the last character by the application's own sequence numbers. Refused delivery stays headless — the machine takes what the harness sends — and PHYSICAL RECORDING CAPTURE REMAINS NOT EXERCISED."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/oshost/typing.go
  - internal/oshost/backend_windows.go
  - internal/codegen/codegen.go
  - cmd/marco/intake.go
  - plugins/e2e/scenario_text.go
---

# Typed text is a sequence of submissions

## One sentence, five events, and any of them can be refused

	do OS's Type with "hello".

Five characters, each its own `SendInput`, each refusable on its own. What a failure in the middle
leaves behind is not an application that was never touched: it is a text box with `he` in it, and
the step after it was written for a field that says `hello`.

Three things were wrong, and they are three different kinds of wrong.

- **Nothing was accounted.** `Invoke` dispatched typing through `ok(…)`, which turns an error into
  a failed step and records it nowhere. `Unperformed` stayed empty and `marco do` reported success
  about typing that did not happen — the same gap the drag had, in the one input where the
  evidence is sitting on the screen afterwards.
- **Nothing was propagated.** `StepType` appeared in `emit` and in no arm of `propagating`, so
  typed text was never a link: whatever followed it ran whether the text arrived or not. That is
  the keystroke's defect one input along, and worse. An Escape that did not arrive leaves the
  screen unchanged; typing that stopped half way leaves a field holding something nobody chose,
  and the press after it is a Save on a form that will be rejected — or accepted with the wrong
  value.
- **Partial delivery was unrepresentable.** The loop stopped at the first refusal and returned an
  error that said neither how far it had got nor that the question had an answer. A stop pressed
  mid-word is the ordinary way this happens.

## Submissions, in code points

**SUBMITTED and not ARRIVED.** `SendInput` returning a count says the system took that many events
into the input stream. It does not say an application received them, that the application holding
the keyboard is the one the routine meant, or that what arrived was rendered. So the account says
`submitted`, and the only account that can say `arrived` is the application's own — which is what
the desktop scenario reads, and why it exists.

**The unit is a Unicode code point.** Never a Go byte offset, never a UTF-16 code unit. `héllo` is
six bytes and five characters; two emoji are eight bytes, four code units and two characters. A
count in bytes means nothing to the person reading it, and `4 of 8 characters` about two emoji is
not a smaller claim — it is a wrong one.

A code point goes in as a down and an up for each of its code units, **all in one call**, so
nothing interleaves between the halves of one character: an application assembles one character
from two consecutive `WM_CHAR`s, and what it does with half a pair followed by somebody else's
input is its own business. The count comes back per event, so a character is in one of three
states and all three are different:

	SUBMITTED  every event taken
	REFUSED    none taken — nothing of that character went in, and nothing is owed
	PARTIAL    some taken — reported as uncertainty, never counted either way

A partial with an ODD count ended on a down, so that unit is owed an up and **the release is
attempted**. An even one is not, and sending a release there would be an event for a key that is
not down.

## The truncation that typed a different character

	kb.wScan = uint16(ch)

`sendUnicodeChar` cast the rune. Every code point above U+FFFF was truncated — U+1F600 became
U+F600, a private-use character — and `SendInput` took it, so the send succeeded. A routine that
typed an emoji typed something else, silently, every time, with no account anywhere that could
have shown it.

`wScan` is a UTF-16 code unit now, and a non-BMP code point is two of them.

## A secret is never measured

Not redacted afterwards. The kind travels into the gesture, so no number about a secret is ever
computed — not its length, not the position it stopped at. A number that exists is a number that
can be logged by mistake.

Both are facts about the value. A length narrows a password; a failure position is a prefix whose
size somebody now knows. So the sentence for a secret says that it did not all go in, names the
secret by the name that is already in the route, and says nothing else.

The backend's own error was the sharpest version of this: it **quoted the rejected character**
into the message. For a secret that is one character of somebody's password in a log line. That
function is gone.

## An account of its own, and why not the stuck-key one

A key the machine would not release means *every keystroke anybody types now arrives modified*.
Incomplete text means nothing of the sort: nothing is held, nothing afterwards is changed by it,
and a person told their keyboard may be stuck would be told something untrue.

So `PartlyTyped` is a second account beside `NotReleased`, and both are asked for. It survives a
cancellation — which discards the error on purpose, because a stop is not a failure — into the
durable record, the run's own output and the Studio card, with the outcome staying `cancelled`.

`input_actions` still counts **attempts**: a rejected Type is one action this run asked the
machine for, and the progress goes beside that number rather than in place of it.

## Every segment is a link

One `StepType` emits several sentences. `"user{{password}}"` is a Type, a Secret and a Type, and a
link is built around ONE `do` — so a step emitting three put two of them where nothing could stop
at them. Whichever one the chain wrapped, the other two ran regardless: the original defect
surviving in the case where it matters most, a password going into a form whose username never
arrived.

`textBySegment` splits the step **before the chain is built**, so each segment is its own step and
its own link. `typeSentence` is the one place that decides which sentence a segment gets, because
two places deciding that is two places to disagree about the route-qualified secret name — and
disagreeing there means looking up a secret nobody stored.

**Legacy routes are untouched**, and their continuation semantics are asserted behaviourally: a
legacy routine whose typing the machine refused goes on to the next step, which is what every
routine saved before this promises.

## What the desktop scenario can see, and what stays headless

It ran green on 2026-10-27, first time. The application's own `Name` field holds the demonstrated
text character for character, the `Notes` field beside it is untouched, and the press after the
typing arrived **after the last character** by the fixture's own sequence numbers — a press sent
while the characters were still going in would submit a form holding part of a value, and the
field would end up correct afterwards either way.

Three things are deliberately not there. A **refused** typing: the machine takes what this harness
sends, and the claim that matters about a refusal is what does NOT happen afterwards, which is
held headlessly by refusing each segment in turn. A **secret**: the text is synthetic and
non-sensitive, because a scenario typing a credential-store value would put somebody's password
into a fixture and into a run's artifacts. And **non-BMP text**: `printable` maps a key NAME to a
character, so what can be demonstrated is what a keyboard has keys for, and the surrogate-pair
path is exercised against the real adapter instead.

**Physical recording capture remains NOT EXERCISED.**

## Enforced by

- `internal/oshost` `TestACharacterIsSubmittedAsItsCodeUnits` — the units a code point decomposes
  to, including the non-BMP case the old cast truncated, asserted on the machine's own record of
  what it was handed.
- `internal/oshost` `TestTheProgressIsCountedInCodePoints` — bytes and code units are both wrong,
  and the emoji case is the one that tells them apart.
- `internal/oshost` `TestATextRefusedBeforeItStartedSubmitsNothing`,
  `TestARefusalMidwayKeepsWhatWentIn` — the refusal before the first character and in the middle,
  each with the control beside it.
- `internal/oshost` `TestACharacterTakenInPartIsUncertainAndItsReleaseAttempted`,
  `TestAPartlyTakenSurrogatePairReleasesOnlyWhatIsOwed` — the third state, and which half of a
  surrogate pair is owed an up; an even count is owed none.
- `internal/oshost` `TestARefusedReleaseIsNotReportedAsAKeyLeftDown` — the account that must not
  be borrowed, asked by the interface rather than by the sentence.
- `internal/oshost` `TestAStopBeforeTypingSubmitsNothing`, `TestAStopDuringTypingKeepsWhatWentIn`
  — both halves of a stop: nothing more is sent, what went in is kept, and the cancellation keeps
  its identity.
- `internal/oshost` `TestASecretSaysNoNumberAboutItself` — no digits at all, with the literal's
  sentence beside it as the control: suppression that cannot be told from having nothing to say is
  not suppression.
- `internal/oshost` `TestARefusedTypingIsInTheAccountsAndCountsAsAnAttempt`,
  `TestAStoppedTypingIsKeptWhereACancelledRunCannotDiscardIt`, `TestATypingThatWentInSaysNothing`,
  `TestARefusedSecretNamesTheSecretAndNotTheValue` — the host's two accounts, which one a
  cancellation belongs in, the control, and the secret's register.
- `internal/oshost` `TestACharactersEventsCarryItsCodeUnitsInOneCall` — the ADAPTER, which the
  mutation gate found untested: the gesture is driven by a fake machine, and that establishes
  nothing about what the real one hands Windows. The units, one call per character, and the count
  coming back from the machine rather than assumed.
- `internal/codegen` `TestATypedTextInAGuardedRoutineIsALink` — `do … ...` followed by `when ok?`,
  not a bare `do`.
- `internal/codegen` `TestAMixedTextIsOneLinkPerSegment` — one link per segment and in order,
  counted by the capabilities rather than read off the sentences.
- `internal/codegen` `TestADeclaredSecretKeepsItsRouteQualifiedNameInAChain` — the thing two
  copies of one decision get wrong in a way nobody notices until a run.
- `internal/codegen` `TestATypingThatDidNotHappenStopsWhatFollowsIt` — each segment refused in
  turn, with `TestATypingThatHappenedLetsTheRoutineGoOn` as the control.
- `internal/codegen` `TestALegacyRoutinesTypingIsGeneratedUnchanged`,
  `TestALegacyRoutineCarriesOnAfterATypingThatDidNotHappen` — the saved-source promise, and the
  behavioural half of it.
- `cmd/marco` `TestAStoppedRunRecordsTheTextItLeftHalfSubmitted`,
  `TestAStoppedRunThatTypedNothingRecordsOnlyThat`, `TestAStoppedSecretRecordsNoNumber` — the
  durable record and the run's own output, with the control that keeps a clean stop quiet.
- `plugins/studio` `TestAStoppedRoutineShowsTheTextItLeftHalfSubmitted`,
  `TestAStoppedRoutineShowsNoNumberAboutASecret` — the card, which is the layer a person reads.
- `plugins/e2e` `TestTheTypingScenarioRunsLikeAnyOther`,
  `TestTheTypingScenarioAsksForTheReceiptAndTheOrder` — it is in every default run, and it asks
  both of its questions. The wiring test is by AST because the first version searched the file and
  found each claim's own definition, which the mutation gate caught.
- `plugins/e2e` `TestAFieldHoldingPartOfTheTextIsAFinding`, `TestTextInAnotherFieldIsAFinding`,
  `TestAPressThatDidNotWaitForTheTextIsAFinding`,
  `TestASeededTypingThatCameOutAsKeystrokesIsRefused` — the claims the verdict rests on, including
  the prefix that is exactly what a partial submission leaves behind.
- `plugins/e2e` `text/typed text arrives and the next step follows it` — **RAN GREEN ON A DESKTOP
  ON 2026-10-27**, 1 of 1, exit 0, first run (`plugins\e2e\run.cmd -only text`). The field held
  `marco typed this 42` character for character and Save arrived as record #42, after the last
  character.
