---
type: adr
status: accepted
date: 2026-10-28
decision: "ACCEPTED — DIRECTOR OWNS ROUTE RESOLUTION, AND EVERY SURFACE ASKS IT. Studio grew a request box and the whole of scope eligibility, candidate labelling and the resolver conversation lived in `plugins/studio` — its own module, which the widget cannot import. Wiring the widget to it would have meant a second scope policy, a second way of building labels and a second place deciding which routine a sentence named: two answers to one question. So the policy is `internal/resolveroute` and Director is its only caller, reached by every surface over the service protocol they already speak. EXACT VERSUS FUZZY IS AN IMPLEMENTATION DETAIL INSIDE THAT PACKAGE AND NOT AN OWNERSHIP BOUNDARY: a surface does not hold half of a decision, so there is NO local fallback — Studio's ask box, the widget and `marco dispatch` all go dark when Director does, and say so, while cards and Play keep working because neither ever asked it anything. `RESOLVE_ROUTE` is read-only, returns a route IDENTITY in the three parts an invocation is built from, and grants nothing: running a routine is still the person pressing Play through the one intake's explicit-identity arm, and `internal/invoke` is untouched. It does consult the configured resolver plugin, which is a subprocess — zero execution means zero route execution and zero input, not zero child processes. THE FOREGROUND NEEDS PROVENANCE AND AN OBSERVATION'S AGE IS NOT PROOF: the window-system provider takes its foreground claim from `winprovider.offered()`, which drops every Marco-owned surface, so with Studio in front NO application observation is produced and the last one stands — fresh, and already wrong; and the accessibility provider takes the bridge's first window with no such exclusion, so a fresh observation can name a Marco window. Both were traced. So a context is granted only by a foreground reading taken NOW through the excluding path, a stated target remains the authoritative override, and a basis that cannot be relied on carries NO application at all — not the app with a caveat, because the field the eligibility filter and the plugin are given must only be filled when it is known. Global and anywhere-in-application routines are eligible under every basis; context-only routines are offered under `stated` and `observed` alone. Director VALIDATES what is submitted: a target this library has nothing saved for is said, and a caller naming a different library is told which two and SELECTS NOTHING, cleared by Director rather than left to a surface to remember. And a refusal is not an answer with nothing in it — the service answers a bad request with an ErrorPayload in a good envelope, so unmarshalling it produced a zero answer that read as \"nothing matched\", which would have made a protocol the Director does not speak indistinguishable from a request it understood; `service.RequestError` carries the code, and the three states are reported apart. `marco dispatch` and the `marco assistant` REPL are thin adapters over the same owner, so `internal/dispatch` has exactly one caller. AND AN ADAPTER CARRIES THE WHOLE IDENTITY OR IT CARRIES NOTHING: the first versions reduced the answer to its SLUG, and the REPL then confirmed that slug and sent it through the door as WORDS — which the door resolves again, so with two routines of one name the routine performed was whichever the second lookup landed on and not the one that was agreed to. A `proposal` carries the route in the three parts an invocation is built from, the question names the identity, and the JSON actually contains it. AND THE WIDGET{Q}S DISPOSITIONS ARE EXPLICIT: a bool meant the door read every answer but a run as permission to offer a recording, so "which of your two routines called settings did you mean?" became "shall I record another one" and a Director that could not be asked produced the same offer — a local fallback by another route. Only Director{Q}s own `learn` decision becomes a learn offer; a clarification, a chat reply, a refusal, a mismatch, a timeout and an unreadable reply all say what happened and offer nothing further. The consequence is stated rather than buried: WITH DIRECTOR UNAVAILABLE THE WIDGET NO LONGER OFFERS TO RECORD ANYTHING, which is the boundary working as agreed, and `learn <name>` remains one of the overlay{Q}s own words. And the resolver child no longer outlives the ask: the bound was a `select` around a goroutine, so the wait ended and the child kept running — the shape of the eight `marco do` processes this repository spent a release gate on. It is `CommandContext` plus `Output` now, so the bound and the reaping are one act. No new transport, no planner work, no recording work."
supersedes: []
affects:
  - execution
source_paths:
  - internal/resolveroute/resolveroute.go
  - internal/resolveroute/where.go
  - cmd/director/resolveroutewiring.go
  - internal/director/service/protocol.go
  - plugins/studio/resolve.go
  - plugins/overlay/resolve.go
  - cmd/marco/resolve.go
---

# Director owns route resolution

## Two answers to one question, nearly

ADR-265 gave Studio a request box, and gave it the resolution to go with it: scope eligibility,
slug-shaped candidate labels, the namesake rules, the resolver conversation. All of it in
`plugins/studio`, which is its own Go module.

The next slice was to give the widget the same thing. It cannot import any of that — so it would
have meant a second scope policy, a second label scheme and a second place deciding which routine
a sentence named. The product would have had two answers to one question, which is the shape of
defect this repository keeps finding.

**So Director owns it.** `internal/resolveroute` is the policy and Director is its only caller;
every surface asks over the service protocol it already speaks. Exact matching versus a model's
opinion is an implementation detail inside that package rather than an ownership boundary: a
surface does not hold half of a decision. There is therefore **no local fallback** — and that is
the cost, named: Studio's ask box, the widget's offer and `marco dispatch` all go dark when
Director does. Cards and Play keep working, because neither ever asked it anything.

Director rather than the engine's intake, because the intake is the **authority door** and its
play lookup is exact on purpose: "near enough" is a judgement, and judgement is Director's.
`internal/invoke` is untouched by this.

## The foreground needs provenance

Director knowing the foreground does not make it the intended target, and an observation's age is
evidence rather than proof. Both failure modes were traced in the code:

- **The window-system provider** takes its foreground claim from `winprovider.offered()`, whose
  own comment says *"A Marco-owned presentation surface is not one of them."* So when Studio or
  the overlay is in front, no candidate carries `Foreground`, **no application observation is
  produced at all**, and the last one simply stands — naming an application that is no longer
  there, with nothing in the value saying so. It can be one second old and already wrong.
- **The accessibility provider** takes `snap.Windows[0]` with no such exclusion, so a fresh
  observation can name a Marco window.

So `resolveroute.Where` grants a context only from a foreground reading taken **now**, through the
path that excludes Marco's own surfaces. If it names an application, that application is in front;
if it names nothing, one of ours is — which is exactly the case that must not unlock a routine
working only in place.

A **stated** target beats both readings, because it is the only one of the three that is a
statement of intent rather than an observation — and a window has the foreground whenever somebody
is typing into it, so a surface's own override is how a person says what they meant.

**And a basis that cannot be relied on carries no application at all.** Not the app with a
warning beside it: the field the eligibility filter and the plugin are given is filled only when it
is known, and what was last seen goes into the sentence a person reads, as prose. That also makes a
Marco-owned observation harmless without a list of Marco's window names to check against — it
cannot be corroborated, so it is not a context, so it names nothing.

	stated         the person named it. Admits context-only routines.
	observed       corroborated against the live foreground. Admits them.
	last-external  an observation that could not be corroborated. Does NOT.
	unknown        no reading at all. Does NOT.

Global and anywhere-in-application routines are eligible under every basis, because neither
depends on what is in front.

## Director validates what is submitted

A **target** this library has nothing saved for is said rather than quietly honoured: honouring it
produces an answer with no in-place routine in it and nothing saying why, which reads as "Marco has
nothing for this" when the truth is "not in the library I am reading". It is still read against
what they named, because what they said is what they meant.

A **library** that is not the caller's is named — both paths — and **selects nothing**. Cleared by
Director rather than left to a surface to remember: an answer that must not be acted on does not
carry the thing to act on. Both sides answer the library through `marcopaths`, so they agree unless
the two processes were given different `$MARCO_ROUTES`; the same reasoning `Prepared` already
carries for a review built in one library and replaced in another.

## Resolution grants nothing, and the plugin is not an exception

`RESOLVE_ROUTE` is read-only, `Mutating()` still has one case, and what comes back is a route
identity in the three parts an invocation is built from — so a surface never takes the string apart
and never looks a name up again. Running a routine is the person pressing Play, which goes through
the intake's explicit-identity arm with every guard that was already there.

**Zero execution means zero route execution and zero input, not zero child processes.** The
configured resolver plugin is a subprocess and is the mechanism this exists to reuse; what must not
happen is a performer, a raise, a press, or product data written. That is what the test asserts.

## A refusal is not an answer with nothing in it

Found while wiring the client. The service answers a bad request with an `ErrorPayload` in a
perfectly good envelope, so unmarshalling it into the response type **succeeded** and produced a
zero answer — which reads as "nothing matched". A protocol the Director does not speak would have
been indistinguishable from a request it understood and could not satisfy.

`service.RequestError` carries the code, with `Incompatible()` for the protocol case, and every
surface tells the three apart: it is not running, it is running and speaks another protocol, or it
answered and refused. Rendering all three as the first sends somebody to start a service that is
already running.

## What the widget does with it

At the moment it already had: nothing took the phrase, and it is about to offer to record a
routine. Somebody who typed *make the pictures nicer* may well have a routine for it under another
name, and that offer is then the one answer that is certainly wrong. So Director is asked first,
through `marco resolve --json` — a client, with `AutoStart` false, because a read does not start a
service that observes the desktop.

If a routine is named, the widget offers **that** instead, and a yes sends it back through the same
door with the whole identity. If there is no answer, the reason is said and the learn offer stands
exactly as it did.

## And the dropdown stayed local

Studio's target selector reads the library itself. That is **library browsing, not resolution
policy**: it answers "which applications have something saved for them", which is the same listing
its cards come from, and nothing is eligible or ineligible because of what it says. What keeps it
safe is that Director validates what is submitted — so a stale or wrong list costs somebody a
sentence and never a selection.

## An adapter carries the whole identity or it carries nothing

The first adapters reduced Director's answer to its slug. The REPL confirmed that slug, and
`runDo` sent it through the door as **words** — which the door resolves again, by the registry's
own priority order. With `settings` saved for two applications, or under two scopes of one, the
routine performed was whichever the second lookup landed on and not the one Director proposed and
the person agreed to. The comment there said the whole identity was in the JSON for anything that
needed it; only the slug was.

So a `proposal` carries the route in the three parts an invocation is built from, the confirmation
question names the identity rather than just a name, and `requestFor` builds the
explicit-identity request — extracted so it can be asserted without performing anything, since
`marco do` presses for real.

## The widget's dispositions are explicit

The helper answered a bool and the door read every false as permission to offer a recording. So
*which of your two routines called settings did you mean?* became *shall I record another one*,
and a Director that could not be asked produced the same offer — which is a **local fallback by
another route**, in a design whose whole point is that there is not one.

	offeredRun     a routine was named and the HUD is asking whether to run it
	wouldLearn     DIRECTOR said this is something to record
	saidWhyNot     something was said and nothing further is offered

**The consequence, named rather than buried: with Director unavailable the widget no longer offers
to record anything.** That is the boundary working as agreed. What a person can still always do is
ask — `learn <name>` is one of the overlay's own words and reaches the recorder directly.

And a `run` answer carrying no identity used to say nothing at all, because the reply was logged
only for non-run kinds. It says so now.

## The resolver child does not outlive the ask

The bound was a `select` around a goroutine: the wait ended, the caller moved on, and the child
kept running — holding a pipe nobody would read and, through it, whatever the plugin had
started. A surface that abandons a process it spawned is the shape of the eight `marco do`
processes this repository spent a release gate on. `CommandContext` plus `Output` makes the bound
and the reaping one act, and a timeout produces a reason rather than an offer.

## Enforced by

- `internal/resolveroute` `TestOnlyTheScopesTheContextAllowsAreOffered` — the three scopes against
  every basis: with a context stated, global, both anywhere-in-application routines and that
  application's in-place one, and not another application's; under `last-external` and `unknown`,
  NO in-place routine even when an application is attached to the context, because the filter reads
  the BASIS; and a corroborated foreground admitting them again, without which refusing every
  in-place routine would pass the rest.
- `internal/resolveroute` `TestAStatedApplicationBeatsEveryObservation` and
  `TestALiveForegroundReadingAdmitsInPlaceRoutines` — what a person named wins, and the live
  reading is what is carried when the two disagree.
- `internal/resolveroute` `TestAFreshObservationThatIsNoLongerInFrontUnlocksNothing` and
  `TestAFreshMarcoOwnedObservationUnlocksNothing` — the two cases age would have let through:
  neither admits an in-place routine, and neither carries an application.
- `internal/resolveroute` `TestAgeChangesTheSentenceAndNotTheBasis` — a ten-minute-old and a
  one-second-old observation reach the same basis, because what makes either wrong is the
  foreground having moved and not the clock.
- `internal/resolveroute` `TestEveryOfferedLabelIsDistinguishable`,
  `TestADerivedLabelTheMatcherCannotTellApartIsEscalated`,
  `TestNeitherNamesakeIsSilentlyPreselected`, `TestASharedNameIsAQuestionAndNotASelection`,
  `TestTheAnswerSaysWhetherTheResolverTookPart`, `TestAnExactNameNeverReachesTheResolver` and
  `TestAnUnofferedAnswerSelectsNothing` — the label and resolver-participation claims, moved here
  with the policy and unchanged in substance.
- `internal/resolveroute` `TestEveryAnswerNamesTheLibraryItResolvedAgainst` — every answer, of
  every kind, says which library it is about.
- `cmd/director` `TestTheLibraryDirectorOffersIsItsOwn` — read through the same two readers
  everything else uses, with each routine's identity and its already-saved scope.
- `cmd/director` `TestATargetThisLibraryHasNothingForIsSaid` — a target with nothing saved for it
  is said, is still honoured as what they meant, and offers only what does not depend on it.
- `cmd/director` `TestALibraryMismatchIsNamedAndSelectsNothing` — both paths named, the answer
  carries no routine to run, it is not a run, and the sentence a person reads says why.
- `cmd/director` `TestResolvingPerformsNothing` — the library byte-for-byte unchanged across five
  requests, no run record reserved, and `RequestResolveRoute.Mutating()` still false. The plugin
  subprocess is allowed.
- `cmd/director` `TestAParaphraseReachesTheResolverThroughDirector` — the real plugin, the real
  model server, a real paraphrase, through Director's own handler, returning the whole identity.
  Declared in `gate-skips.txt`; it is the only test that exercises the connection rather than a
  stand-in.
- `plugins/studio` `TestTheAskBoxSelectsWhatDirectorNamedAndRunsNothing`,
  `TestTheAskBoxSaysWhenTheResolverWasSilent`, `TestTheAskBoxSaysWhenNothingIsKnownToBeInFront`,
  `TestTheAskBoxSaysDirectorIsNotRunning` and
  `TestTheAskBoxTellsAProtocolMismatchFromAMissingService` — the real page in a headless browser:
  the request and the target arrive together, the basis is shown and not just the application, the
  named card is SELECTED, `PlayRoutine` is called zero times, and a Director that could not answer
  says which of the three it was — with nothing else claimed, because there is no answer to draw.
- `plugins/studio` `TestTheTargetsAreTheApplicationsWithInPlaceRoutines` — the dropdown, which is
  browsing: an application with no window is still offered, marked.
- `plugins/overlay` `TestTheDoorOffersTheRoutineDirectorNamed` and
  `TestSayingYesRunsTheIdentityAndNotTheName` — through `dispatchIntake`, which is where the
  defect was: the offer carries the whole identity, there is no learn offer beside it, and the
  invocation a yes produces is `--play`/`--app`/`--focus` with the typed words NOWHERE in it.
- `plugins/overlay` `TestOnlyDirectorsLearnDecisionBecomesALearnOffer` — eight answers through the
  door: a clarification, a chat reply, a run with nothing to run, a Director that is not running,
  a library mismatch, a timeout and an unreadable reply all say what happened and offer NOTHING
  further; only Director’s own `learn` decision becomes a learn offer, which is the control
  without which a widget that had stopped offering to learn anything would pass.
- `plugins/overlay` `TestTheAskIsBoundedAndSaysSo` — the ask returns with a reason rather than
  leaving the caller waiting on a child it has stopped listening to.
- `plugins/overlay` `TestTheLearnOfferFiresOnlyWhenNothingTookIt` — unchanged in what it holds
  about the door, and now supplying Director’s decision, because the offer is that decision.
- `plugins/overlay` `TestTheResolverChildIsBoundedAndReaped` — a REAL child that hangs, reached by
  pointing `$MARCO_BIN` at this binary with a `TestMain` answering as that child (the runner builds
  its own argv and cannot be told to run one named test): the ask comes back well inside the
  child’s life, says it timed out and how long it waited, and carries no answer. With the control
  beside it — a child that answers — without which a runner that always timed out would pass.
- `cmd/marco` `TestAConfirmedProposalInvokesTheExactRouteDirectorNamed` — three namesakes, and the
  request a confirmation becomes built by the PRODUCTION function: it names the route and carries
  no words for the door to resolve.
- `cmd/marco` `TestResolvingTheNameAgainLandsSomewhereElse` — what the name resolves to, that it is
  not what Director would have proposed for two of the three, and that the explicit identity
  reaches exactly the route that was agreed to.
- `cmd/marco` `TestTheAdvisorProposesAndDoesNotPerform` — the confirmation still comes first, now
  naming `runChosen` and failing if `runDo` comes back.

See [[ADR-265-say-what-you-want-and-studio-offers-the-routine]], [[Invocation]] and [[Plays]].
