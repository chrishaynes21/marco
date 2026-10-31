---
type: reference
status: accepted
date: 2026-10-30
source_paths:
  - internal/resolver/interpret.go
  - internal/resolver/which.go
  - internal/dispatch/dispatch.go
  - internal/dispatch/llm.go
  - internal/resolveroute/resolveroute.go
  - internal/director/meaning/meaning.go
  - internal/director/execute/pipeline.go
  - internal/director/values/values.go
  - internal/director/values/provenance.go
  - internal/routes/args.go
  - internal/orchestrator/playvalues.go
  - cmd/marco/dispatch.go
---

# Extracting argument values from a request

Traced at `377f1b4`. The ask: Director works out, from what somebody said, what text a routine
should be given — on the explicit route-and-values confirmation contract built in
[[ADR-276-a-routine-may-be-told-what-to-type]].

**D1 to D4 are ruled on and recorded in §4, and four amendments came with them** — folded into
the rules below and each marked AMENDED. The largest turns an aspiration into something checkable:
the plugin identifies SPANS and Marco copies the text out of the request itself, so invented or
paraphrased text cannot be an extraction rather than merely being discouraged from being one.

The trace's finding is that almost all of the shape already exists, and that the one genuinely new
question is **not** a new mechanism but a new QUESTION on a boundary that already has two of them
and documents how to add a third. What needs deciding is not how to ask but what an answer is
allowed to do.

## 1. What exists

### One boundary to a model, with two questions on it

`$MARCO_RESOLVER` names one executable. One JSON object in on stdin, one out on stdout. Two
questions travel over it:

| question | asked by | the plugin is told | it answers |
| --- | --- | --- | --- |
| which saved routine did this name | `internal/dispatch` (`PluginAdvisor`) | the words, the offered labels, the application | an intent and a slug |
| what did this sentence MEAN | `internal/resolver.Interpret` | the words and a closed list of verbs | one verb, a phrase, an ordinal, a flag |

`internal/resolver/interpret.go` says in its own words how the second one was added, and it is the
template for a third:

> So this is an additional QUESTION on the existing protocol rather than a new protocol. The
> request gains one optional field and the response gains one; a plugin that only knows how to
> pick a route sees a field it does not recognise, answers `{"route":""}` as it always did, and
> the caller is told plainly that nothing interpreted the sentence.

And what the plugin is told is bounded on purpose: **nothing about the screen.** Not a label, not a
window title, not an element, not a coordinate, not what Marco remembers — because the question is
what a sentence means, not which thing on the desktop it points at.

### Two model-asking things, with different statuses — and the distinction matters most here

**`internal/resolver.Interpret` is wired**, as a parse fallback only: `execute.Pipeline.parseWith`
consults it when and only when the deterministic reading came back `IntentUnknown`, and
`TestADeterministicReadingIsNeverReinterpreted` holds that. A deterministic reading is never
reinterpreted.

**`internal/director/meaning` is the one in SHADOW MODE.** Its own package comment is explicit:
*"Nothing in resolution or execution asks this question. The only caller is the evaluation that
measures it, and a tree walk holds that — `cmd/director TestTheInterpreterIsNotWiredIntoAnything`."*
It asks which observed CONTROL a request meant, and the reason it is held back is stated: both
halves — does it identify the right control, and does it know when it does not know — need
measuring before anything acts on either, and a mechanism that is already acting cannot be
measured against what it displaced.

**So the standing constraint "keep the semantic interpreter in shadow mode" is about `meaning`,
and extracting an argument value is not that question.** A value is a span of the person's own
sentence. Answering it needs the words and the argument NAMES and nothing about the desktop — which
is the first kind of question, not the second. `TestTheInterpreterIsNotWiredIntoAnything` is a tree
walk and would fail if this slice reached for `meaning`; it must not.

### What a "value" already means to Director

`internal/director/values` is program-local semantic data, governed by one distinction:

> Semantic target variables answer "WHICH OBJECT?". Captured values answer "WHAT INFORMATION?".
> Objects always re-resolve. Values never re-resolve.

It already has `KindText` — *"literal text the user supplied or selected"* — and
`SourceLiteral`. And `provenance.go` carries a discipline this slice should simply adopt:

> Everything in this file is DESIGNED to be safe rather than redacted to be safe […] none of the
> types here has anywhere to put the content — not a Text, not a Value, not an any.

That is the same rule [[ADR-276-a-routine-may-be-told-what-to-type]] arrived at independently for
supplied text, one layer along. A value's provenance can be explained without the value.

### And the contract an extraction would hand values to

Built and green: `invoke.Request.Named` with an explicit `Play` (D4 of the last slice),
`routes.CheckArgs` refusing a missing, unknown or unwritable value, `routes.CheckFlagged` refusing
a secret through a flag, `orchestrator.Invocation` with a token sealing the route, the source
digest and the values, and `Request.Expect` carrying the confirmed version into the child.

### Where a request's words stop today

`cmd/marco/dispatch.go`:

```go
func requestFor(p proposal) invoke.Request {
	return invoke.Request{Play: &p.Route, Source: invoke.SourceCLI}
}
```

No values, and the comment above it says why the WORDS must not travel: the door would resolve
them again, and with two routines of one name that is how a confirmation comes to perform a
different routine from the one it described. So values have to travel AS VALUES — which is exactly
what the contract now provides, and why this slice is reachable at all.

`resolveroute.Answer` carries the route identity, the scope, the offer count and which arm
answered — and nothing about arguments. `dispatch.Decision` has no slot for a value either, nor
does its wire shape (`intent`, `route`, `name`, `reply`).

## 2. The gap, stated narrowly

Nothing asks what text a selected routine should be given, and nothing carries an answer. Director
selects the routine (`resolveroute`), the routine declares what it needs (`routes.Arguments`), and
the two have never been introduced.

Three things are therefore missing and nothing else is:

1. a third question on the one boundary — the request, and the argument names;
2. somewhere for the answer to live between the resolver and the confirmation;
3. a rule for what an answer is ALLOWED to do, which is the whole of the risk.

## 3. The rules

**R1. It is a question about words, and it is not the shadow-mode question.** The plugin is told
the person's own sentence and the argument NAMES the routine declares. Nothing about the screen —
no label, no element, no window, no coordinate — because a value is a span of the sentence and the
evidence for a screen question is deliberately absent from this boundary. `meaning` is untouched,
and the tree walk that holds it stays green.

**AMENDED: AND "A SPAN OF THE SENTENCE" IS ENFORCED RATHER THAN HOPED FOR.** The plugin answers
with OFFSETS, not text — `{"name":"message","start":22,"end":33}` — and Marco copies the value out
of the request it sent. A plugin therefore cannot invent, paraphrase, translate, tidy or re-case a
value: the only thing it can get wrong is WHICH span, which is visible in the confirmation beside
the argument it was proposed for.

Every answer is validated before anything is copied, and each of these is a refusal rather than a
repair:

| refused | because |
| --- | --- |
| a name the routine does not declare | there is nowhere for that text to go, and `CheckArgs` would refuse it anyway |
| the same name twice | two spans for one argument is an answer with no reading |
| an offset outside the request | it indexes something that was never sent |
| `start >= end` | an empty span is not a value; a value that is empty is SUPPLIED, which is a different thing |
| an offset off a rune boundary | it would slice a character in half, and a model counting characters where Marco counts bytes is the likeliest way to get here |
| a span that is only whitespace | nobody means "type three spaces" by naming a span, and this is what a miscount looks like |

And what is copied is copied VERBATIM: not trimmed, not collapsed, not normalised. A value is used
as supplied, which is already the rule for one that was typed — so a span including a leading space
includes it, and a span that is only spaces is refused above rather than silently tidied into
nothing.

**R2. A third question on the existing protocol, not a second protocol.** One optional field in,
one out, exactly as `Interpret` was added. A plugin that does not know it answers as it always did
and the caller is told plainly that nothing extracted anything. There is one place a model is
configured and one answer to "is a model wired up".

**R3. The deterministic reading answers first and is never reinterpreted.** `name:value` and
`with a, b` already work and already win; `routes.CheckArgs` already refuses what they got wrong.
A model is asked only for the arguments the deterministic reading left unsupplied — the rule
`TestADeterministicReadingIsNeverReinterpreted` already holds one layer along, applied here.

**AMENDED: AND "UNSUPPLIED" IS PRESENCE, WHICH MAKES AN EXPLICIT EMPTY VALUE SUPPLIED.** The
distinction [[ADR-276-a-routine-may-be-told-what-to-type]] turns on, reaching one layer further:
somebody who gave `message=` meant "type nothing here", and asking a model to find them a value
would be overruling the clearest thing they said. So extraction is offered only the names with no
value PRESENT — `v, ok := named[name]` — and never the names whose value happens to be empty.

**R4. An extracted value is a PROPOSAL, and a proposal reaches a screen rather than a keyboard.**
This is the load-bearing rule and the reason the slice is safe to build now rather than after a
measurement. An extraction does not become an invocation: it populates the confirmation that
[[ADR-276-a-routine-may-be-told-what-to-type]] built, the person reads what would be typed as
text, and nothing presses until they confirm. `meaning`'s own posture is the precedent — *"What it
may do, which is propose […] a Proposal is an element id and a sentence. It is not a resolution: it
carries no confidence, no authority, no action"* — and the difference here is that there is now
somewhere for a proposal to be shown.

**R5. A proposal may be CORRECTED; a prepared confirmation may not.**

**AMENDED, because the first wording contradicted D4.** It said an extraction the person edits
"refuses", which would have made a prefilled box unusable — the whole point of prefilling is that
they can fix it. The two moments are different and the rule belongs to the second:

- **Before a confirmation is prepared**, a prefilled box is a PROPOSAL and editing it is
  correction. Pressing Play then prepares a FRESH confirmation from what is in the boxes now, and
  that value is labelled as the person's own rather than as extracted (R7).
- **After a confirmation is prepared**, it is sealed: the token covers the route, the source digest
  and those values, so a value changed between reading the confirmation and pressing it refuses.
  That is `orchestrator.ConfirmInvocation` as already built and tested.

So an extraction gets no weaker a binding than a typed value, and no stronger a one either — what
is sealed is what the person last saw, whoever put it there.

**AMENDED: AND THE ROUTINE'S VERSION TRAVELS WITH THE PROPOSAL.** The argument names an extraction
was asked about came from one revision of one routine. If that routine is saved again before the
confirmation is prepared, the proposal is refused and asked afresh — **even when the new version
declares the same names**, because the same name in a rewritten routine may be typed somewhere
else entirely, and a value that crossed into a routine it was never prepared for is text going to
a place nobody proposed it for. So the answer carries the source digest the names were read from,
and preparation compares it. The mechanism already exists: it is `Request.Expect` and
`Invocation.SourceAt`, one step earlier in the chain.

**R6. A secret-typed argument is never extracted.** `routes.Arguments` already says which are
secret and `CheckFlagged` already refuses one through a flag. A model proposing a password out of
a sentence, into a box, onto a command line is the one path this must not open — and the credential
store is where that value comes from.

**R7. Which arm supplied each value is said, per argument.** Not a count and not a global claim.
`resolveroute.ResolverSentence` exists because *"the configured resolver answered this" about a
free offline match is the false impression this whole sentence exists to prevent* — and with some
values parsed deterministically and others proposed by a model, a single sentence about the request
would be that false impression per argument.

So each value says how it arrived, and AMENDED there are four answers, not three: **typed** (given
as a flag or a field), **parsed** (the deterministic grammar read it out of the phrase),
**extracted** (a span a model proposed), and **edited** (a proposal the person corrected — which is
theirs now, not the model's, and must not go on being credited to it).

And none of these labels carries the value. The provenance says where a value came from and the
confirmation says what it is; `internal/director/values/provenance.go` is the shape to follow,
being *designed to be safe rather than redacted to be safe*.

**R8. A surface that cannot show a proposal says so, and guesses nothing.**

**AMENDED, and it is the other half of D1.** The overlay, the voice path and `marco do` have no
field and no confirmation step, so for a routine whose values would have to be extracted they can
do one of three things: run with a guess, fail silently, or EXPLAIN. The first is what every
boundary in this trace exists to prevent and the second is worse than a refusal, so each of them
says what it cannot do — that this routine needs text worked out from the request, that this
surface cannot show what it would type before typing it, and that the value can be given directly
instead. A refusal somebody can act on, in the shape `whyNoDirector` and `ResolverSentence`
already use.

**R9. Extraction cannot widen anything.** It supplies values for arguments the routine declares;
`CheckArgs` still refuses an unknown name, a missing one and one that cannot be written down, and
the authority door is unchanged. An extracted value is subject to every rule a typed one is.

## 4. The decisions, as ruled

**D1 — RULED: a confirmation is required, always.** The sharpest question in the slice. Studio has a card; `marco do` prints and runs; the overlay and the voice path have neither a
field nor a confirmation step. **Recommended: no — never, in this slice**, and the cost stated
plainly: a spoken or typed request whose values were EXTRACTED cannot run until a surface shows
them, so the overlay and voice gain nothing from this slice until they gain somewhere to show a
proposal. The alternative is a model's reading of a sentence typing text into somebody's
application with nobody having seen it, which is the thing every boundary traced above exists to
prevent.

**D2 — RULED: Director owns it, asked after route selection.** In `internal/resolveroute`, after
it has selected a routine — because that is the moment the routine's arguments become knowable, Director
already owns resolution ([[ADR-267-director-owns-route-resolution]]), and every surface already
asks Director rather than the plugin. `dispatch` stays the route question; this is a second ask
beside it, not a widening of `Decision`.

**D3 — RULED: a missing value uses the existing fields.** Nothing special — it is missing, and the
existing refusal and the existing field apply. Half-extracted is the common
case and must not be a separate state.

**D4 — RULED: proposals prefill editable boxes, for the current request only.** The last slice
ruled that Studio retains no previous value (D3 there), and this is a different thing: not a
remembered value but a reading of THIS request, which is why it may fill a box and why nothing
about it survives the request. Visibly filled and editable, with R7's label saying it was
extracted — because a proposal nobody can correct
before confirming is worse than one they can.

## 5. What must hold

Not written, and named here as not written.

**The question's boundary.** What the plugin is told is the words and the argument names, and a
test that asserts the request carries nothing about the screen — the same shape `interpret.go`'s
own boundary test takes. And `meaning` still asked by nothing, which its existing tree walk already
holds and which this slice must leave green.

**The order.** A value the deterministic reading supplied is never replaced by an extracted one,
and a request that needs no model does not reach one — reported as such, per R7, so a free offline
answer cannot be credited to a resolver.

**The posture.** An extracted value reaching a confirmation and never a keyboard: nothing typed,
nothing pressed, no run record, until a person has confirmed what would be typed. And the
confirmation's token covering extracted values exactly as it covers typed ones — edited before
confirming, refused.

**The boundaries that already exist, still holding.** A secret never extracted. An unknown name
still refused. A value spanning lines still refused. No value in a log line, a run record or a
refusal's sentence.

**And a plugin that cannot answer.** Nothing extracted, said plainly, and the routine still
reachable by typing the value — because the first thing that will happen on a machine with no model
configured is this question going unanswered.

## 6. The increments

1. **The question, its answer, and the plugin that answers it.** Asked by `resolveroute` and
   carried back on `Answer`, with nothing acting on it: a value proposed and shown, every
   validation in R1, and the per-argument provenance of R7.

   **AMENDED: THE PLUGIN IS PART OF THE DELIVERY.** Extending a protocol is not extraction — a
   question nothing answers is a field in a struct. `plugins/llama` gains the third question, and
   an older plugin that does not know it still answers as it always did, which is the behaviour
   `interpret.go` already guarantees and which this must not break.

   And a BOUNDED LIVE PROBE, behind `-tags resolver` where the live resolver test already lives,
   establishing exactly one thing: that Director reaches the plugin and gets spans back that
   validate. **It is not a comprehension benchmark and must not be reported as one** — whether a
   model reads sentences WELL is the measurement
   [[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]] is the model for,
   with a corpus and a denominator, and a probe that answered one sentence correctly would be the
   most misleading possible evidence about that.
2. **The confirmation path**, so a proposed value reaches `orchestrator.Invocation` and is sealed
   with the route and the source — including the version check of R5, so a proposal prepared
   against one revision cannot be confirmed against another.
3. **Studio's box**, prefilled and labelled per D4, with correction producing a fresh
   confirmation.

## What this would NOT establish

That a model reads anybody's sentence WELL. Everything above is about what an answer is allowed to
do, not about whether the answer is right — and the measurement that would say so is the kind
[[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]] did for rewording,
which is a separate piece of work with its own corpus. The rules are written so that a bad
extraction costs somebody a glance at a confirmation rather than text typed into their application.

## Increment 1, as built — 2026-10-30

[[ADR-277-a-request-may-propose-the-text-a-routine-types]] is the decision and names the tests.
What landed: `internal/resolver/extract.go` (the question, and the `Span` with no text field),
`internal/resolveroute` (`Candidate.Takes`, `Candidate.SourceAt`, `Proposal`, `FromExtracted`,
`Resolver.Extract` and `proposals`), `cmd/director/resolveroutewiring.go` (the library reporting
what each routine takes off ONE read, and the production `Extract: resolver.Extract`), and
`plugins/llama/extract.go` with its dispatch arm.

Three things worth carrying forward from building it:

- **The plugin is asked for WORDS, not offsets.** A 3B model cannot count bytes, and one asked for
  `start` and `end` answers with numbers that are confidently wrong — which, validated, means
  nothing is ever extracted. So it quotes the words and the plugin LOCATES them. The wire still
  carries only offsets, so invention dies twice and independently.
- **Two guards I wrote could not fire**, and a mutation deleting either survived the whole suite.
  Both came out; the rule rides the chosen candidate instead. An unreachable guard reads exactly
  like an enforced rule, which makes it worse than no guard at all.
- **A test of mine had never reached the arm it named.** `dispatch.Decide` refuses a chat or a
  clarification with nothing to say and a learn offer with no name, so three cases that believed
  they were exercising three kinds of answer were exercising three rejections.

The probe reached a real local model and came back with `message=hello there`, verbatim. That is a
connection and not a measurement, for the reason stated above it.

**Still unbuilt from the rules above: R8**, and increments 2 and 3. Nothing reads `Answer.Values`
yet.

## Increments 2 and 3, as built — 2026-10-30

`orchestrator.PrepareProposed` takes what will run, what was proposed and the revision the
proposals' names were read from; `Invocation.From` carries the per-argument provenance, DERIVED by
comparing the two maps and sealed with the values. Studio's `ReviewPlay` carries all three, the
page prefills an editable box, labels it, and repaints the label on the keystroke.

R5's two halves both landed: a proposal may be CORRECTED before a confirmation is prepared (the
correction is then the person's own, labelled `edited`), and the prepared confirmation is sealed —
a value or a label changed after it refuses.

Three things worth carrying forward:

- **The provenance had to be DERIVED, not asserted.** A surface claiming the label fails in exactly
  one direction, and it is the direction that matters: a window that prefilled a box and forgot to
  relabel it would tell somebody a model read their sentence correctly when they are the one who
  fixed it.
- **Four defects came from mutations rather than failing tests**, and all four were a rule stated
  and not held: two page clears that disagreed a line apart, a confirmation card left on screen
  with a live button after the state behind it was dropped, a hidden card keeping its old lines and
  an enabled button, and a test asserting an argument COUNT as a proxy for what was sent.
- **R6's prefill rule needed restating, not overruling.** The last slice refused REMEMBERING a
  value; this shows a reading of the request in front of the person. The page comment that said
  *nothing is prefilled, here or ever* states the distinction now.

**Still unbuilt: R8.** The overlay, the voice path and `marco do` do not yet explain that they
cannot show a proposal. And nothing here has been exercised on a desktop.

## R8, as built — 2026-10-30

One sentence at the engine's one door, because the overlay and the voice path both spawn `marco`
and stream its output: there is no second copy of the rule in a surface. Recognised by
`routes.ValuesNeeded` rather than by the wording, and gated on `expect` — whether anything
CONFIRMED — rather than on a list of surface names, so Studio is never told to open Studio and
`Request.Source` stays recorded-and-never-consulted.

One real survivor first: nothing had checked that the clause belongs to ONE of the four things
`CheckArgs` says. Telling somebody to review text when they have misspelt an argument, pasted a
line break, or given more values than the routine has places for is sending them to look at the
wrong thing.

**All of R1 to R9 are now built.** What remains unexercised is a desktop.
