---
type: adr
status: accepted
date: 2026-10-28
decision: "ACCEPTED — SAY WHAT YOU WANT, AND STUDIO OFFERS THE ROUTINE IT THINKS YOU MEANT. A library of saved routines was reachable only by recognising a card and clicking it, so a person with thirty of them had to remember what they had called each one. Studio now takes a request in a person's own words and SELECTS the routine it names; the thing that performs one is the Play button that was already there, pressed by a person, with the guards it already had. Nothing in the suggestion path starts a process, raises a window or presses anything. The resolver it asks was ALREADY HERE AND NOTHING WAS CALLING IT: `internal/dispatch` has had a Dispatcher, an Advisor interface, a plugin client that speaks to a local Ollama through `plugins/llama`, and a `validate` that accepts a run only when the route is one the caller offered — and `setup.ps1` pins `$MARCO_RESOLVER` and `$MARCO_ASSISTANT` at that plugin, so on a set-up machine it was configured, installed and dead. Its callers were `marco dispatch` and the `marco assistant` REPL, which the file itself calls a developer surface that nothing launches; the overlay — the widget — never called it, because its one door is `marco do --source=typed`, whose play lookup is exact and deliberately not fuzzy. THE THREE SCOPES ARE PRESERVED AND THE TARGET APPLICATION IS STATED, NEVER INFERRED: a global routine is always eligible, an anywhere-in-application routine is eligible from wherever you are because it brings its own application forward, and an in-place routine is offered ONLY when its application is the stated target. Two inferences were available and both are wrong — Studio owns the foreground whenever somebody types in it, so reading the foreground answers \"Studio\", and treating \"this application has a window\" as eligibility would make every running application's in-place commands reachable from anywhere, which is what anywhere-in-application already means and erases the difference between the two. With no target stated, no in-place routine is offered and the answer says so. AND THE RESOLVER IS TOLD WHICH APPLICATION, which it was not: the target filtered the candidates and appeared in the window while `Decide` was given an empty context, so Studio knew which application was meant and Ollama did not — and the ambiguity answer claimed it was "asked about with the target stated" when the asking said nothing. The reasoning for passing nothing had been written for a different question: an application Studio worked out for ITSELF would be a fiction, because Studio owns the foreground whenever somebody types into it, but this one is not worked out. It is context for the question and not another filter on the answer, so eligibility is unchanged. WHAT THE RESOLVER IS OFFERED IS A LABEL AND WHAT COMES BACK IS A KEY INTO A MAP: a route IS its scope and its slug, the identity is path-shaped and reduces to one unmatchable word in the deterministic matcher, so the candidates are slug-shaped labels and the identity travels beside them. Uniqueness is checked on `nlu.Key` — the matcher's OWN reduction rather than a copy of it — because two labels that differ as strings can be one candidate to it: a hyphen is a space and \"in\" is a stop word, so a derived `settings-in-ark` collides with a routine genuinely slugged `settings-ark`. Every routine claims its own slug first and only then does anything derive a label, so the listing's order cannot decide who keeps their own name; a label that is still taken escalates to a counter. The returned label is NEVER resolved again — it is looked up in the map, and the whole identity is what Play is given. AND THE RESOLVER'S PART IS VISIBLE, because two of the three arms need no model at all: an exact name runs offline and a near miss asks \"did you mean\", so a window built on this could work, look right, and never once reach Ollama. A decision now says which arm answered — exact, advisor, fallback — and whether the Advisor was absent, NOT ASKED, answered, or ASKED AND SILENT. Two of those four were got wrong first time. Silent is the state that used to be indistinguishable from never having configured one; NOT ASKED is the exact arm, and reporting it as `answered` — which the first version did, on the reasoning that an Advisor existed — made a free offline match say "the configured resolver answered this", which is a fact about what is INSTALLED standing in for a fact about what HAPPENED, and is the same false impression these states were added to remove. AND A SHARED NAME BELONGS TO NOBODY: unique labels stop two routines being confused for one, and they are not an answer to the PERSON{Q}s ambiguity. The first version still handed the bare slug to one of the namesakes, so a request for `settings` was an exact match on that label — answered offline, selecting whichever routine the listing happened to put first, with neither the resolver nor the person asked which of the three was meant. So when eligible routines share a normalized name every one of them is qualified, the bare name matches no label, and a request for it that nothing resolves becomes a question naming the alternatives rather than a near miss pointing at an arbitrary one or an offer to record a third. Held by eight headless tests plus three rendered-page ones, and by ONE LIVE TEST that asks the real plugin and the real model server with a real paraphrase and asserts the whole identity that comes back, because a stand-in Advisor establishes nothing about the connection and a suite of stand-ins is exactly what the dead resolver already had. 18 of 18 mutations killed. Overlay wiring is deliberately NOT in this: the widget still has its exact-only door. SUPERSEDED IN PART by [[ADR-267-director-owns-route-resolution]]: putting the policy in this window’s own module was the wrong place, because the widget cannot import it and a second surface resolving separately would be two answers to one question. The policy, the labels, the namesake rules and the resolver conversation all moved to `internal/resolveroute` behind Director, and the tests named below moved with them; what is still this window’s is the request box, the target dropdown and what it shows."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/suggest.go
  - plugins/studio/frontend/index.html
  - internal/dispatch/dispatch.go
  - internal/nlu/nlu.go
  - internal/routes/registry.go
---

# Say what you want, and Studio offers the routine

## The library you had to remember

Record → Stop → review → Save worked, and then the routine was a card. To run it you recognised
it and clicked it. That is fine for three routines and it is not fine for thirty: the names are
yours, you chose them weeks ago, and the one you want is the one you cannot remember the name of.

So Studio takes a request in your own words — *adjust ark video quality* — and **selects** the
routine it names. It does not run it. The thing that runs one is the Play button that was already
there, pressed by a person, with every guard it already had: the whole-identity lookup, the
refusal of a routine that would not run, the raise that only a foreground-owning window may do,
the bundle check, the one run slot, and `marco do --play=` landing on the intake's explicit-identity
arm where nothing is ever looked up from words again. Nothing in the suggestion path starts a
process, raises a window or presses anything, which is what makes it safe to press.

## The resolver was already here, and nothing was calling it

This is the part worth recording plainly, because the shape of it is this repository's most
repeated defect.

`internal/dispatch` has had the whole of this for a long time: a `Dispatcher` with a documented
policy, an `Advisor` interface, a `PluginAdvisor` that shells out to an external plugin over a
tiny JSON protocol, and a `validate` that accepts a `run` **only** when the proposed route is one
the caller offered. `plugins/llama` answers that protocol against a local Ollama. `setup.ps1`
pins both `$MARCO_RESOLVER` and `$MARCO_ASSISTANT` at the built plugin. On a set-up machine the
resolver was configured, installed, and reachable.

By nothing a person uses. Its callers were `marco dispatch "<phrase>" --json` — whose own comment
says it is "the machine seam a front-end (the overlay) uses to converse" — and `converseTurn` in
the `marco assistant` REPL, which the file itself describes as a developer surface that nothing
launches. **The overlay never called either.** The widget's one door is
`marco do --source=typed <phrase>` → `internal/invoke.Decide`, whose play lookup is exact and
deliberately not fuzzy, with everything else going to Director. The legacy-audit note had already
filed `$MARCO_RESOLVER` as a zombie.

So this slice reuses an existing resolver **that the widget was not actually calling**. The code
was complete, tested, documented, installed by the installer, and dead — which is why the next
section exists.

## A decision says which arm answered

Two of `Decide`'s three arms need no model. An exact route name runs offline and free; a near miss
asks *did you mean*. Both are indistinguishable, in their answers, from the Advisor having
decided. A surface built on this could therefore work, look right, be demonstrated, and never
once reach the thing somebody installed — which is the position the resolver was already in.

So the arm is part of the answer. `Decision.Source` is `exact`, `advisor`, `fallback` or `none`,
and `Decision.Advisor` is `absent`, **`not-asked`**, `answered` or **`silent`**. Studio renders
both, beside the request.

Two of the four were wrong on the first pass, in opposite directions. **Silent** — configured,
asked, and having nothing to say — is the state that used to be invisible: on a machine where
Ollama is not running it looks exactly like a machine where nobody configured a resolver. And
**not-asked** is the exact arm, which was reported as `answered` because `advisorState` answered
"there is one" when the question was "what happened to it". So a free offline match on an exact
name printed *the configured resolver answered this*. That is the same class of mistake as the
gate’s orphan — a fact about what EXISTS standing in for a fact about what HAPPENED — and it
is the one direction nobody would have noticed, because it flatters the feature. `advisorState`
now never answers `answered`: each arm says what actually became of the Advisor from there.

It does **not** claim which kind of silence. `Advisor.Advise` reports "I have nothing" for a
plugin that declined, a plugin that is missing, a model server that is down and a reply that would
not parse, and inventing a distinction would be a guess about somebody's machine. What it says
truthfully is that the configured resolver was asked and did not answer, and where to look.

Nothing branches on either field. They are shown.

## The scopes are preserved, and the target is stated

A route **is** its scope (see `internal/routes`), and the three mean different things:

	GLOBAL                    anywhere. Always eligible.
	ANYWHERE-IN-APPLICATION   brings its own application forward, so it is eligible from
	                          wherever you are — across applications, which is its point.
	APPLICATION/CONTEXT       works only while its application is in front. Eligible ONLY
	                          when that application is the stated target.

**The target is chosen, never inferred.** Two inferences were available and both are wrong.
Studio owns the foreground whenever somebody types into it, so reading the foreground answers
"Studio". And treating *this application has a window* as eligibility would make every running
application's in-place commands reachable from anywhere — which is precisely what
anywhere-in-application already means, so it would erase the difference between the two app
scopes entirely.

With no target stated, no in-place routine is offered, and the answer says that rather than
guessing. Whether a target is currently running is shown as a **hint and not a filter**: an
application with no window is still a target somebody may choose, because Play raises it, and the
choice is theirs. The target is echoed back in the answer and rendered from there, so the page and
the backend cannot disagree about which application a request was read against.

**And the resolver is told which application it is.** It was not, at first: the target filtered
the candidates and was rendered beside the request while `Decide` was handed an empty context —
so the window knew which application was meant and Ollama did not. The ambiguity answer added in
the same round made that visible by claiming a shared name is *asked about with the target
stated*, which was not true of the asking.

The reasoning for passing nothing had been written for a different question, and was right about
that one: an application Studio worked out for *itself* would be a fiction, because Studio owns
the foreground whenever somebody types into it. This one is not worked out. It is the one the
person chose, and withholding it left the resolver guessing at exactly the thing it is best placed
to settle. It is **context for the question, not another filter on the answer** — eligibility is
unchanged, so a global routine and an anywhere-in-application one are still offered whatever is
chosen.

A routine that would refuse as it stands is not offered at all. Offering it would make the
resolver's best case a dead end.

## A label is offered and an identity comes back

The candidates handed to the resolver are **labels**, not identities, for two reasons that both
have to hold. The identity is path-shaped — `ark/context/settings` — and the deterministic matcher
strips everything that is not alphanumeric or a space *without inserting one*, so it reduces to
`arkcontextsettings` and matches nothing anybody would type. And the converse prompt asks for a
slug. So a label is slug-shaped, and the identity travels beside it in a map.

**Uniqueness is checked on the matcher's own key.** Two labels that differ as strings can be one
candidate to it: a hyphen is a space and `in` is a stop word, so a derived `settings-in-ark` and a
routine genuinely slugged `settings-ark` both reduce to `settings ark`. An answer naming either
names both. The check is `nlu.Key`, which is the function `nlu.Resolve` itself calls rather than a
reimplementation of it — a second copy of a normalization is how a surface comes to believe two
labels are distinct while the matcher behind it cannot tell them apart.

**Every routine claims its own slug first**, in a pass of its own, and only then does anything
derive a label. One pass gave whoever the listing happened to put first whatever it asked for: a
routine really called `settings-in-ark` would lose its own name to a label derived for `settings`
saved in Ark, and then get a label derived from a name that was never its. Both stayed distinct
and both mapped correctly, so nothing was *wrong* — it was arbitrary on the listing's order, which
is the kind of thing that is right in the test and wrong on somebody's machine. A label still
taken after that escalates to a counter, which is never pretty, always works, and is reached only
when a library genuinely holds two routines this cannot otherwise tell apart.

**And a shared name belongs to nobody.** Unique labels stop two routines being confused for
one; they are not an answer to the person’s ambiguity, and the first version mistook the one for
the other. The first eligible `settings` kept the bare name and the rest were qualified — every
label distinct, every one mapping to the right identity, the collision problem solved. But a
request for "settings" was then an **exact match** on that label: answered offline, selecting
whichever routine the listing happened to put first, with neither the resolver nor the person
asked which of the three was meant. An ambiguity resolved by listing order and reported as an
exact match.

So when eligible routines share a normalized name, **every one of them is qualified** and the bare
name is left belonging to nobody. A request for it matches no label exactly, so the resolver is
asked — with the target stated, which is what it is for, and an answer from it is kept and says
`advisor`. If nothing resolves it, what is left would be a near miss pointing at an arbitrary one
of them (*did you mean settings anywhere?*) or an offer to RECORD a routine called `settings` when
three already answer to it. Both are the ambiguity being answered rather than asked, so both are
replaced by the question and the alternatives — in `plays.Scope.Says`, the product’s own words for
the three scopes, because the difference between them is the thing being asked about.

**The returned label is never resolved again.** It is a key into the map. `dispatch.validate` has
already refused anything that was not offered, and this is the second of two checks rather than
the only one: a name is not unique and neither is a slug, and a second lookup is how a surface
comes to display one routine and perform another — which `Routine.ID` exists because of.

## What the live test is for

Eight headless tests and three rendered-page tests hold the policy with a stand-in Advisor: which
routines are eligible, under what labels, which arm answered, what the window shows, and that
finding a routine runs nothing.

None of that establishes that a request ever leaves the process. **A suite of stand-ins is exactly
what the dead resolver already had.** So one test asks the real plugin, which asks the real model
server, with a phrase that shares only the application's name with the routine's — *adjust ark
video quality* for `ark-graphics` — and asserts the whole identity that comes back. It declines
when the plugin is not built or nothing is listening, declared in `gate-skips.txt` with that
reason, so a machine where nothing was asked says so instead of printing `ok`.

It is a demonstration that the connection works and **not** a claim about how well the model
understands. The probe behind it is worth recording: a further paraphrase — *make the visuals in
the dinosaur game look nicer* — is read by `llama3.2:3b` as a request to teach a new command,
three times out of three. That reached Ollama and came back; it is the model's reach, not a defect
in any of this. Of seven phrasings tried, six resolved identically three times each.

Also measured: a cold model load is about 2.6s, and the very first converse call of the session
produced nothing at all — which degraded to the deterministic arm, exactly as designed. That was
not reproduced in sixteen later calls, including after unloading the model, so its cause is **not
established** and is recorded rather than explained.

## What is deliberately not in this

Overlay wiring. The widget still has its exact-only door, and giving it this is a separate change
with its own question — the overlay does not own the foreground, so its target is a different
fact. Also out: any change to `internal/invoke`, a second execution path in Studio, a new scope,
and argument extraction.

## Enforced by

**The policy's tests moved to [[ADR-267-director-owns-route-resolution]]**, with the policy. They
are the same claims — scope eligibility, label distinguishability, namesakes, which arm answered,
an unoffered answer selecting nothing, the resolver's part said out loud, and one real paraphrase
through the real plugin — held in `internal/resolveroute` and `cmd/director` where both surfaces
reach them, and listed there. Naming them here as well would be two notes claiming the same tests.

What is still this window's, and held here:

- `plugins/studio` `TestTheTargetsAreTheApplicationsWithInPlaceRoutines` — the target dropdown,
  which is library BROWSING and not resolution policy: with no windows at all the application is
  still offered as a target, marked as not running, because whether it is running is a hint and
  not a filter. What keeps that safe is that Director validates what is submitted.
- `plugins/studio` `TestTheAskBoxSelectsWhatDirectorNamedAndRunsNothing` and the four cases beside
  it — the real page in a headless browser, now rendering Director's answer: the request and
  the target arrive together, the BASIS is shown and not just the application, the named card is
  SELECTED, `PlayRoutine` is called zero times, and a Director that could not answer says which of
  the three it was.

See [[Learned-Plays]], [[Invocation]] and [[ADR-264-the-gate-answers-for-what-did-not-run]].
