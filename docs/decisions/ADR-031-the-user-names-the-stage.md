---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - semantic-memory
  - programs
  - marco-boundary
---

# The user names the stage

`Screen's Showing` is fulfilled. A play can now be refused before its first effect because it is
on the wrong screen — and the name in that sentence belongs to the person, not to Director.

---
## Decision 1 — one durable string, and it is offered rather than observed

`RememberedSubject.Called` is the first durable string in this system that comes from a person
instead of from perception. That is the whole privacy boundary: an OCR line, an accessibility
label or a window title may never land there, because none of them was offered.

The distinction is provenance, not content. `the pause menu` typed by a user is allowed;
`the pause menu` read off a screen is not.

---
## Decision 2 — exact, scoped, and never nearest-match

`application + name → one durable subject`. Case-insensitive, otherwise exact. No substring, no
fuzzy score, no cross-application fallback — `settings` in one program is not `settings` in
another, and a play that resolved the wrong one would press keys in the wrong place.

Two subjects in one application may not share a name: the second assignment is refused, because
first-match-wins would make a play's first line depend on file order. A duplicate that got in some
other way is **ambiguous**, and ambiguous is a refusal.

---
## Decision 3 — the host looks and compares, and can do nothing else

`screenhost.Recognition` has three methods and every one is a read. The package reaches no OS host,
no driver, no window activation, no rehearsal grant, no execution pipeline, no orchestrator.
**Asking where you are must not be a way of doing something.**

Five internal outcomes — recognised, ambiguous, unobservable, unavailable, unrecognised — collapse
to `failed` at the language boundary and stay apart in the diagnostics, because turning *"I could
not look"* into *"I looked and it was different"* would send somebody to fix the wrong thing.

> **Amendment, 2026-08-20 — six outcomes, because the rule above was being broken one level in.**
>
> `Unknown` is documented as *"a screen observed **cleanly** and matching nothing remembered"*, and
> nothing enforced the word "cleanly". A reading that reached a window and not the page fell
> through to it.
>
> Measured live, on Windows Settings: the right application, the right window, in front, full
> screen — and sixteen structures where the same page had been learned with a hundred and
> forty-eight, one of them a rectangle covering three quarters of the frame with nothing in it.
> Marco said *"I don't recognise this screen"*. That is the sentence for a page somebody should
> open a different one of, and the page was never the problem; changing it produced the identical
> refusal, three runs running.
>
> **`Unreadable`** joins the vocabulary: a window that was seen, whose content could not be read.
> It refuses exactly as hard — see
> [[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]]. What changed is that Marco is
> accurate about why.
>
> The distinction is decided at `observe.PlaceNow`, the one current-place answer, so every caller
> gets it — and it is decided on ARRANGEMENT rather than richness: a space the window gives its
> content, with nothing observed inside it and nowhere else in the window populated either.
> Never on a control count. `148 → 130` is a responsive page and must keep meaning what it means;
> `148 → 16 plus an empty rectangle` is a page that never arrived.

---
## Decision 4 — standalone Marco fails closed, and that is the architecture

A Marco started on its own has semantic memory but no recogniser, so `CurrentSubject` reports
`unavailable` and every guarded play refuses.

It does not skip the guard, assume ok, fall back to OCR text matching, or degrade into blind
replay. **Director figures out the play; Marco performs it; Director still provides the eyes while
it does.** That is stated plainly rather than contorted around.

> **Amendment, 2026-08-20 — the last sentence is now wired.** `CurrentSubject` used to return
> `unavailable` *unconditionally*: the eyes were promised and never connected. It now ASKS the
> Director which place is showing, when one is reachable, and still answers `unavailable` when none
> is. The recogniser is still the Director's and there is still only one of it — this is that
> sentence implemented, not a second recogniser and not a relaxation.
>
> **The refusal is unchanged and may not be weakened.** No Director, a Director that cannot see, a
> place it does not recognise, an outcome word this build has never heard of, or a `recognised` with
> nothing named in it — every one of them refuses. Silence is still never yes.
>
> *Why it mattered now.* A learned Play with intact provenance is delegated to the Director and never
> met the stub. An **edited** one is not delegated — editing makes it an ordinary play — so it took
> the local runner and refused at its own first line with "Marco could not check", immediately after
> the authority seam had told the person *"it runs like anything else you have written"*. Routing
> edited plays to the Director instead would have been worse, not better: the Director re-plans from
> the goal and never reads the file, so the person's edits would have been silently discarded. The
> honest fix was to let the local runner see. See
> [[ADR-078-a-learned-play-is-performed-by-the-director]].

---
## Decision 5 — the source carries the requirement, and the sidecar carries none of it

`.origin.json` records where a play came from. It does not enforce anything. A user who deletes the
`Screen's Showing` line owns a play with no entry condition — it runs, on any screen, and Marco
stops claiming it verified the file.

Proved adversarially: remove the guard, put the wrong screen in front, and the keys go out. That is
the correct outcome, and a system where the sidecar quietly kept guarding would have failed it.

## Enforced by

- `TestTheWrongScreenSendsNothing` — authorized, resolved, compiled, and zero effects.
- `TestTheRightScreenRunsThePlayFromItsOwnSource`, `TestOnlyAPositiveMatchLetsThePlayBegin`.
- `TestRemovingTheGuardFromTheSourceRemovesTheGuard`.
- `TestAScreenNameIsScopedToItsApplication`, `TestASecondScreenCannotTakeATakenName`,
  `TestAScreenNameSurvivesARestart`.
- `TestEveryWayOfNotKnowingRefuses`, `TestAHostThatCannotLookRefuses`,
  `TestTheScreenHostCannotAct`.

---
## Decision 6 — Marco asks, and the answer belongs to the screen that was asked about

**Added Roadmap 31.** Decisions 1–5 built the machinery and left it unreachable: `AskNameScreen`
existed as vocabulary and `Store.NameSubject` existed as a safe durable write, and no production
path connected them. A verified play that could not say where it started simply stayed
unlowerable, forever, with nobody asked anything.

The demand is **not** `subject.Called == ""`. It is a real artifact being blocked: `JudgeLowering`
refusing with `screen_unnamed` for a rehearsal-verified candidate. That refusal, at the lowering
choke point, is the only thing in this system that makes Marco ask what a screen is called. A
sweep over unnamed subjects would be a collection loop wearing a question's clothes.

The question carries `SubjectRef{Application, ID}` — the same durable identity semantic memory
uses across a restart — because the answer arrives late. By the time somebody types *the pause
menu* the screen has changed and is often in another program. Ambient state may not redirect it.

The answer is a **typed response of its own** (`ObserveScreenName` → `UserSuppliedScreenName`), not
a text field bolted onto the three-word vocabulary every other question uses. The generic
`Respond` refuses a naming question outright, which is what keeps the closed vocabulary closed.

The durable write happens **before** the question is settled. A refused name — invalid, too long,
already taken in that application — leaves the question open, because a closed question the user
did not successfully answer is a prompt they can never see again.

## Enforced by

- `TestTheWrongScreenSendsNothing` — authorized, resolved, compiled, and zero effects.
- `TestTheRightScreenRunsThePlayFromItsOwnSource`, `TestOnlyAPositiveMatchLetsThePlayBegin`.
- `TestRemovingTheGuardFromTheSourceRemovesTheGuard`.
- `TestAScreenNameIsScopedToItsApplication`, `TestASecondScreenCannotTakeATakenName`,
  `TestAScreenNameSurvivesARestart`, `TestAnAmbiguousScreenNameResolvesToNothing`.
- `TestEveryWayOfNotKnowingRefuses`, `TestAHostThatCannotLookRefuses`,
  `TestTheScreenHostCannotAct`.
- `TestUserSuppliedScreenNameRefusesWhatAPersonWouldNotType`,
  `TestAnUnnamedStartingScreenIsNotWrittenDown`, `TestTheEntryConditionIsTheNameTheUserGave`.
- `TestAVerifiedPlayThatCannotSayWhereItStartsAsks` — the wiring: Marco actually asks.
- `TestNamingAScreenBindsToTheSubjectThatWasAskedAbout`,
  `TestAnAnswerDoesNotFollowTheApplicationInFront` — the answer lands on the right screen.
- `TestAnInvalidNameChangesNothingAndTheQuestionStaysOpen`,
  `TestADuplicateNameIsRefusedThroughTheAnswerPath`,
  `TestANamingQuestionIsNotAnsweredWithYesOrNo`.
- `TestTheNamingLifecycleUnblocksTheLearnedPlay` — verified → blocked → asked → answered → written.
