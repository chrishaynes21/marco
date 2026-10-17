---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
fixture: "the committed desktop perception corpus — what production believed about real windows, read by the real accessibility walk and fused by the real engine, reviewed by a person and committed. Windows Settings on two pages to build against; File Explorer, Microsoft Edge and Paint held out. No desktop was touched: nothing was executed, no window raised, no input watched or injected."
result: deterministic-rewording-raises-ordinary-phrasing-from-60-to-73-percent-on-the-application-it-was-built-against-and-from-68-to-79-percent-on-three-it-was-not-with-zero-wrong-controls-and-it-closes-spelling-and-wrapping-words-while-closing-one-paraphrase-in-thirteen
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - docs/decisions/ADR-218-a-request-reworded-in-the-screens-own-words.md
  - internal/director/rephrase
  - fixtures/rewording
---

# Experiment 069 — rewording closes the spelling gap, and not the meaning gap

The implementation and evaluation of [[ADR-218-a-request-reworded-in-the-screens-own-words]]
— stages (a) and (b) of [[ADR-217-proposed-grounding-a-request-in-what-is-on-screen]]. Stage
(c), the advisory interpreter, was deliberately not built. This is the evidence for whether
it should be.

## The numbers

Sixty-eight ordinary instructions, each with an oracle written down beforehand. Two sets: one
on the application rewording was built against, one on three applications nothing in it was
built for and nothing was tuned against afterwards.

| | Settings (built against) | Explorer + Edge + Paint (held out) |
|---|---|---|
| correct, literal reading alone | 18 / 30 (60%) | 23 / 34 (68%) |
| **correct, with rewording** | **22 / 30 (73%)** | **27 / 34 (79%)** |
| **wrong control, from rewording** | **0** | **0** |
| ambiguity answered without asking | 0 | 0 |
| literal answers changed | 0 | 0 |
| cost | 31µs → 78µs per request | 29µs → 146µs per request |

Three Settings cases and one held-out case are excluded from both denominators as
**unmeasured**: they are already-satisfied cases, and the committed samples record no
selection state at all, so nothing in them can say which page is the current one.

By phrasing, held out beside built-against:

| phrasing | built against | held out |
|---|---|---|
| exact label | 5/5 | 6/6 |
| extra words ("the X panel", "X settings") | 6/8 | **7/7** |
| spelling ("personalisation", "favourites") | 4/4 | **3/4** |
| ambiguous, correctly preserved | 2/2 | **4/4** |
| absent, correctly refused | 5/5 | **6/6** |
| paraphrase ("themes", "bookmarks", "dark mode") | 0/6 | 1/7 |

**The held-out column is the one that counts**, and it is the better of the two. That is the
result this evaluation was designed to be able to disbelieve: the failure mode this codebase
has had eight times is a mechanism that works on the application it was written against, and
three applications nobody tuned against — a file manager whose navigation is a tree, a browser
whose page content is links, a paint program that is almost entirely toolbar buttons — do
better than Settings does.

## What it closed

**Wrapping words and kind nouns.** `the address field` reaches the Address Bar text field in
Explorer; `the Storage section` reaches a LINK in Edge, because the role the person's noun
named was tried first and then fell back rather than refusing; `the theme dropdown` reaches
the combo box.

**Spellings a person brought with them.** `favourites` → Favorites, `appearence` →
Appearance, `magnifer` → Magnifier, `personalisation` → Personalization. All four from the
screen's own vocabulary; none from a table of synonyms.

**And it left every genuine ambiguity alone.** Explorer's toolbar `Details` beside its
view-mode `Details`; its column header `Name` beside one `Name` cell per row; Edge's two
`Close` buttons; Paint's `Zoom` slider beside its `Zoom` combo box. Four of four, on
applications nobody looked at while building.

## What it did not close, and the boundary is sharp

**Paraphrase: one in thirteen.** `themes` for Personalization. `wallpaper`. `wifi` for
Network & internet. `bookmarks` for Favorites. `dark mode` for Theme. `uninstall programs` for
Apps. `draw a rectangle` for Shapes. `make the brush bigger` for Size. `go up one level`.
`turn off notifications`.

Every one of these fails for the same reason, and it is the reason stated in the design:
rewording may only ask with words the screen is showing. `themes` is not on the screen.
Neither is `bookmarks`. **These are knowledge questions, and a deterministic reading does not
know anything.**

The one paraphrase that passed — `make a new folder` → Explorer's `New` button — passed by
the literal reading's substring clause, not by rewording. It is luck rather than
understanding, and it happens to be right.

**Transpositions.** `refersh` for Refresh is two edits under a one-edit rule, and was put in
the held-out set to measure the boundary rather than to pass it. It did not pass.

## Two defects in the LITERAL reading, found by the evaluation and not fixed

The oracle caught these, not a test. Both are in resolution by the words the person used,
which rewording never reaches, and both are left alone because the instruction for this work
was to preserve deterministic resolution.

**One: a short label inside a longer request answers it.** `bluetooth settings` and `the
bluetooth panel`, on the Bluetooth page, both resolve to the Bluetooth **on/off switch** —
not to the page. `textScore`'s `strings.Contains(want, got)` clause scores a label CONTAINED
IN the request at 0.5, the switch is the only usable candidate, nothing contests it, and the
resolver reports a confident single answer at 0.485.

This is the more serious of the two, and it is worth being precise about why: rewording would
have got it right. Asked again with "settings" set aside and the destination roles tried
first, the navigation item resolves and the switch is rejected as the wrong kind of control.
The literal reading answers first, confidently, and rewording is never reached — which is
correct behaviour under property 1 and the wrong answer for the person.

So the largest remaining wrong-control risk is not in rewording. It is the literal reading's
willingness to answer a request it only partly matched.

**Two: the same clause is what made many extra-word cases pass without rewording.** Six of
the eight Settings extra-word cases are answered literally, some at confidences a hair above
the floor — `time and language` resolves at 0.259 against a `MinScore` of 0.25. The clause is
doing a great deal of load-bearing work, right and wrong, and nobody chose that.

## Already satisfied: implemented, measured as unmeasurable, not wired in

The rule is `Element.Selected` — the interface's own report of which destination is current.
Unit-tested. Deliberately narrow: a rule that read "already open" off a page header repeating
a navigation label would be a rule about applications that happen to print their page name,
and the first one that did not would be told its request was satisfied when it was not, which
does nothing and reports success.

It cannot be measured on this corpus, because `desktopElement` did not record `selected`. The
field is recorded in new captures now; the committed samples predate it. Four cases are
reported as **unmeasured**, which is the honest reading: a sample that never carried the
evidence is not evidence against the rule.

And it is not wired into execution, which is a product decision rather than a resolution
detail: acting on "nothing needs doing" means a request can succeed without performing its
action, and that changes the plan, the policy verdict, the confirmation and the record
afterwards. **Taken since, in [[ADR-221-a-question-the-person-already-answered]]**: a satisfied
request now performs nothing and says so, and `TestSatisfactionIsDecidedInExactlyOnePlace`
holds the narrower boundary that remains — one place decides it.

## One oracle I got wrong, recorded rather than quietly fixed

`Devices`, on the Settings Bluetooth page, was written down as **ambiguous**: two elements
carry exactly that label. One of them is static text, which is not a control and never was a
candidate — so there is one operable `Devices` and resolving to it is right. The mistake was
mine, in the oracle, written from reading labels without checking operability. The correction
and the reason are in `fixtures/rewording/settings.json` beside the case.

## The recommendation

**Deterministic rewording is worth having and is not sufficient.**

It is worth having: +13 points on the built-against application, +11 on three held out, zero
wrong controls, every ambiguity preserved, every absent control still refused, no additional
observation, and about 50µs a request. It generalises — the held-out numbers are better than
the tuned ones — because what it knows is UI vocabulary rather than any application's.

It is not sufficient, and the shape of the insufficiency is clear rather than vague: it
closes every phrasing gap that is about FORM and none that is about MEANING. One paraphrase in
thirteen. A person who does not know an interface's vocabulary is not usually mis-spelling its
words or wrapping them in "the"; they are using a different word. `themes`, `bookmarks`, `dark
mode`, `wifi` — these are what "feels like it understands me" is actually made of.

So the honest answer to the question this was run to settle: **(a) and (b) do not make the
need for (c) go away, and they do make it cheap and safe to try.** Rewording leaves a
mechanism in place that proposes candidates and cannot invent them, and an interpreter would
be a third source of attempts into the same pipeline, bound by the same four refusals and
scored by the same oracle on the same held-out applications — with one count that must stay
at zero.

Two things to settle before that, and the evidence here says they are worth more than a
plugin:

1. **The literal reading's partial-match clause.** It is answering requests it does not
   understand, confidently, and it reaches the person before anything else does. Fixing it
   is also what would let rewording answer `bluetooth settings` correctly.
2. **Already satisfied, measured for real.** One capture with `selected` recorded settles
   whether the rule works. "Apps is already open" is a better answer than the right click,
   and it is nearly free.

## Enforced by

`cmd/director` `TestRewordingNeverChoosesTheWrongControl` runs both committed sets as a gate:
no wrong control, no ambiguity answered, no literal answer changed, and a held-out set must
exist. `TestRewordingAnswersWhatTheLiteralReadingCouldNot` fails if rewording stops earning
its place. Re-run the report with `director assess-rewording [--verbose]`.
