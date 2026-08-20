---
type: subsystem
status: active
owners:
  - marco
depends_on:
  - learned-plays
  - demonstrations
used_by:
  - service
  - visibility
updated: 2026-08-20
source_paths:
  - internal/plays
  - internal/routes
  - internal/orchestrator/authority.go
  - cmd/marco/assistant.go
  - cmd/marco/edit.go
  - cmd/marco/learnui.go
---

# Plays

The Audience's account of everything Marco can do: what a **Play** is, what a **Binding** is,
where a Play stands between being written down and being askable, and — for every one of those
facts — which line of code decides it.

This note is about the PRODUCT MODEL. [[Learned-Plays]] is about how one particular kind of Play
comes to exist; read that for the chain from watching to arrival, and this for what the person is
then holding. [[Invocation]] is how a request finds one of these — the exact lookup below is arm
four of the intake, and everything it misses belongs to Director.

## A Play is the durable behaviour; a Binding is what reaches it

```
    phrase / hotkey / spoken utterance        BINDING   (a trigger)
                    |
                    v
     the .marco file + its .origin.json        PLAY     (a behaviour)
                    |
                    v
        orchestrator: Resolve -> Authorize -> Run
```

The old word covered both, and [[34F-legacy-marco-product-audit]] §2.3 counted **six** things the
codebase called a route: the phrase, the `.marco` artifact, a Director edge, compiled Marco, a
hotkey and an IR step list. Three of those are genuinely distinct product nouns — Play, Binding,
edge — and the other three are implementation layers.

A Binding is `routes.Binding{App, Key, Cmd}` (explicit, a hotkey) or the Play's own scoped name
resolved by `Registry.Resolve` (implicit). Either way it is a way IN, never the thing itself, and
the surfaces keep them apart: the control centre lists bindings under their own tab and the Plays
listing carries them in a separate `bindings` array, so nobody forgets a macro in order to un-bind
a key. `internal/plays` `TestABindingReachesAPlayWithoutBecomingOne` is that separation.

## Why the noun is Play and the package is still `internal/routes`

**Product vocabulary and implementation vocabulary are allowed to differ — where the difference
costs a reader nothing.** A package name is that case exactly: nobody outside this repository ever
sees the words `internal/routes`, so renaming it is a large diff across `cmd/marco`, `cmd/director`
and every test in exchange for nothing a person could notice.

That is a different question from **a word the product needs back**. The acquisition flow was also
spelled `teach` in the code from end to end, and it was renamed to `learn` all the way down —
because *Teach* names a feature this project intends to build, and a word cannot be spent twice
([[ADR-048-learn-teach-and-do-are-three-different-sentences]],
[[ADR-086-one-acquisition-one-word-one-request]]). So `teach` is not a precedent for keeping an
internal name; it is the counter-example, and it is the test to apply here: nothing else needs the
word *routes*, so `internal/routes` keeps it.

The audit's recommendation was explicit about this ([[34F-legacy-marco-product-audit]] §8, *"do not
rename the Go package"*), and [[ADR-081-a-durable-behaviour-is-a-play]] is where the decision
lives.

`internal/plays` is the seam where the two vocabularies meet. It is a **projection**: every field
it produces is a rendering of something `internal/routes` already knows, and a package boundary is
the honest place to put a translation, because it can be pointed at.

## The three kinds

`routes.Kind` has three values and `plays.KindWord` gives each one a word a person reads:

| `routes.Kind` | shown as | means |
|---|---|---|
| `KindAuthored` | **Authored** | somebody wrote the file |
| `KindTaught` | **Recorded** | a demonstration, turned into source |
| `KindLearned` | **Learned** | Director watched, rehearsed, verified, wrote it down |

### Recorded, not Taught

The obvious label for `KindTaught` is taken. **Teach** is reserved for Marco guiding a person
through something Marco already knows — the opposite direction of travel — and
[[ADR-048-learn-teach-and-do-are-three-different-sentences]] spent the word on purpose, because
that feature is a small step from the visual grounding that already exists and would have no name
left if `teach` were spent twice. *Recorded* says what actually happened and collides with nothing.

### Nothing writes `KindTaught`, so the listing infers it

The only `Kind` any production path assigns is `KindLearned`. A demonstrated play gets no sidecar
at all, so its provenance reads `authored`, and it would list as somebody's own writing. The
`.rec.json` beside it is the only durable evidence that a person demonstrated it, so
`plays.taughtIfRecorded` promotes authored → taught when `Registry.HasRecording` finds one.

The inference runs **one way only**. A recording present means recorded; a recording absent means
nothing, because a play demonstrated before recordings were kept has none and so does a
hand-written one. And it lives in the projection rather than in `KindOf`, because `KindOf` feeds
the authority door, which treats authored and taught identically — a listing has no business
changing what that door sees. Deleting it must fail `TestARecordedPlayIsNotShownAsAuthored`.

## The three scopes

| `plays.Scope` | badge | what it does |
|---|---|---|
| `ScopeGlobal` | Anywhere | app-less; answers anywhere and switches nothing |
| `ScopeContext` | In the app | answers only while its application is ALREADY in front |
| `ScopeFocus` | From anywhere | answers from anywhere **and Marco brings the application forward first** |

**Focus says what it does.** "From anywhere" alone is also true of a global play, and the
difference — the activation — is the capability the audit found people liked most
([[34F-legacy-marco-product-audit]] §1.5, §10). So `Scope.Says` names the application (*"You can
ask for it from anywhere — Marco brings chrome forward first"*), `plays.Play.Activates` carries the
same fact as its own field so a shortened column cannot drop it, and the CLI row reads `from
anywhere (brings chrome forward)`. Collapsing focus into either neighbour must fail
`TestFocusIsPresentedDistinctlyFromContext` and `TestFocusReadsDifferentlyFromContext`.

`ScopeOf` tests **app-less first, then Focus** — the same order `Registry.dir` uses to choose the
folder, so the label cannot disagree with the location. The copy this replaced tested `Focus`
first, and would have called an app-less play "focus" the moment a Route was built from user input
(`TestAnAppLessPlayIsGlobalWhateverItsFocusBit`).

### A learned Play registers as FOCUS, deliberately

`routes.LearnedFocus` is a constant, `true`, and it is a DECISION rather than a fact on disk:
`stagedDir` puts every staged play in `<app>/learned/` whatever `Route.Focus` says, so the bit
cannot be recovered by reading the filesystem. Being a constant is what keeps the code that STAGES
a play and the code that LISTS one reading the same fact — a listing that decided independently
could promise an activation that registration would not perform
(`TestAStagedPlayListsAsBringingItsApplicationForward`).

Registered as context, a learned Play resolved only while the application it would take you to was
already in front, and Marco offered to Learn a play it had learned four minutes earlier. That is
the live failure in [[ADR-080-a-learned-play-is-asked-for-from-anywhere]]. Focus is the contract
the performer already had: `Runtime.PerformGoal` brings the application forward itself before it
reads the Stage.

## Saved, registered, and the five standings

Saving and registering are **two operations against two directories**, not a flag. Route discovery
scans `global/`, `<app>/`, `<app>/context/` and `<app>/focus/`; `<app>/learned/` is not among them,
so a saved-and-unregistered play is invisible to the resolver *structurally*
([[ADR-028-a-learned-play-is-a-file-with-a-past]]). A `"registered": true` that disagreed with the
filesystem cannot be written down, because nothing writes it down.

`plays.Life` folds two facts — can the resolver reach it, and does its provenance still describe
it — into the standing a surface shows:

| `Life` | word | askable? | how it is reached |
|---|---|---|---|
| `ready` | Ready | yes | registered, provenance absent or intact |
| `edited` | Ready · edited | yes | registered, and the person changed the file |
| `unverified` | Needs attention | yes | registered, sidecar this version cannot read |
| `saved` | Saved — not askable yet | no | staged, provenance intact — the Register button belongs here |
| `stuck` | Saved — cannot be registered | no | staged, provenance no longer describes the file |

`edited` is **not damage**. The person was invited to edit their play and did; it still runs, and
Marco simply stops claiming it is the artifact Director verified.

`stuck` exists because `Registry.Register` refuses anything whose staged provenance is not
verified. Offering a Register button beside such a play would be an offer Marco cannot keep, so
`Life.Registerable()` gates the button in the control centre and the `(marco register "…")` hint in
`marco plays` alike (`TestAStagedPlayThatCannotBeRegisteredSaysSo`,
`TestAStuckPlayIsNotOfferedRegistration`).

**A wall says what the mechanism keeps apart; it does not say who crosses it.** A completed Learn
asks for both halves in one act, because naming a behaviour is the permission to make it askable —
[[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]]. Saved-and-not-registered
stays reachable (a taken name is the ordinary cause), which is exactly why the product now has a
`saved` row and a Register button rather than a file no command mentions.

And registering is still not permission: `Resolve → Authorize → Run` keeps the door between knowing
which play the person means and performing it ([[ADR-029-resolution-is-not-permission]]).

## Two enumerations, joined in the projection — never one widened

```
reg.List()        what the resolver can see        --+
                                                     +--> plays.List()   the product listing
reg.ListStaged()  what it structurally cannot      --+
```

`Registry.List` **cannot see staged plays, by design**, and that is the property being protected.
`Registry.Resolve` walks `List`; widening `List` to include the staging directory would make every
staged play answerable from every application, which is the whole thing the two directories exist
to prevent. So the join happens in `plays.List` — a projection with no authority — and nowhere
lower down. Deleting the `ListStaged` half must fail `TestStagedPlaysAreListedAndStayUnresolvable`.

`Registry.List` and `Registry.Resolve` were not touched. The registry API is **additive**:
`ListStaged`, `KindOf`, `KindOfStaged`, `HasRecording`, `MoveOrigin`, and the `LearnedFocus`
constant.

**Registered is not a stored flag.** It is which enumeration the row came out of, so a `Registered`
that disagreed with the filesystem is unrepresentable.

### Browsing is read-only, and that is a property with a test

Every call under `plays.List` is `os.ReadDir`, `os.ReadFile` or `os.Stat`. The temptations sit
right beside it: `Registry.ScopeDir` reads like a writer and is not, while `Save`, `SaveRecording`,
`SaveStaged`, `WriteAssets` and `writeAtomic` all begin with `os.MkdirAll`. Adding any writer to
that path must fail `TestBrowsingPlaysChangesNothingOnDisk` — and listing an empty store must
create nothing, which is `TestListingStagedPlaysCreatesNoDirectory`.

## One decider for kind, and the door reads it too

`routes.Registry.KindOf` is THE answer to *what kind of play is this, and does its past still
describe it*. It is not `Origin.Kind`: an orphaned or unreadable sidecar still carries a `kind`
field, and believing it would call a file learned on the strength of provenance that describes
nothing. State and kind are read together or not at all
(`TestKindOfDoesNotBelieveAnUnreadableSidecar`).

`orchestrator.Classify` — the call on the way to the authority door — reads the same function the
listing does:

```go
func Classify(reg routes.Registry, rt routes.Route, phrase string) Resolved {
    kind, state := reg.KindOf(rt)
    return Resolved{Route: rt, Phrase: phrase, Kind: kind, Provenance: state}
}
```

So a surface cannot call a play *learned* that the door calls *authored*.
`TestTheDoorAndTheListingAgreeAboutEveryProvenance` walks every provenance state and asserts they
agree; `TestAnOrphanedSidecarDoesNotMakeAPlayLearned` holds the state/kind pairing itself.

Staged plays use `KindOfStaged`, never `KindOf`: the sidecar is in `learned/`, and asking about the
registered location reports the learned play Director just wrote as somebody's own writing
(`TestAStagedPlayIsNotReportedAsAuthored`).

## A play's past travels with the file

Two operations move a play, and both carry the `.origin.json` with it, **verbatim**:
`Registry.Rename` (within a scope) and the control centre's `/api/scope`, via `Registry.MoveOrigin`.
Copying the bytes rather than rebuilding them is the difference between carrying a past and
rewriting one — a play the person edited stays `edited` rather than being silently re-verified by a
move. See [[ADR-082-a-plays-past-travels-with-the-file]].

## Where each fact comes from

| fact | decided by |
|---|---|
| which plays are askable | `routes.Registry.List` — the four scanned directories |
| which plays are staged | `routes.Registry.ListStaged` — `<app>/learned/` |
| the two, joined | `internal/plays` `List` |
| kind + provenance | `routes.Registry.KindOf` / `KindOfStaged`, also read by `orchestrator.Classify` |
| authored → recorded | `plays.taughtIfRecorded`, from `Registry.HasRecording` |
| scope | `plays.ScopeOf`, from `Route.App` then `Route.Focus` |
| the scope a staged play will take | `routes.LearnedFocus` |
| standing | `plays.LifeOf(registered, state)` |
| the words for all of it | `plays.Scope.Word/Says`, `Life.Word/Says`, `KindWord/KindSays` |
| what Learn says when it finishes | `plays.AfterLearn`, rendered by `cmd/marco/learnui.go` `addLifecycle` |

## The surfaces, and what each is allowed to show

| surface | shows | why |
|---|---|---|
| **Plays tab** (`marco ui`, `cmd/marco/edit.go` `/api/plays`) | registered AND staged, with Register / scope / delete | the product view, and the landing tab when no play is named |
| **`marco plays`** | the same two groups, printed apart | what a person HAS |
| **`marco register "<name>"`** | — | it exists because `marco plays` names it |
| **`marco routes`** | registered only | what may be OFFERED to a front end |
| **`/api/routes`** | registered only | the step editor's picker; a staged play is not openable |
| **Learn panel** | `life` / `life_word` / `life_says` | the same words the Plays list uses for the same file |

The two groups are printed apart rather than mixed with a badge, because the difference is not a
nuance: one of them answers when you ask for it and the other does not
(`TestThePlaysListingShowsRegisteredAndStagedPlaysDifferently`,
`TestMarcoPlaysShowsTheSavedPlayMarcoRoutesMustNotOffer`).

`marco routes` keeps its name, its result set and its first four JSON keys — `name`, `slug`, `app`,
`scope` — because consumers outside this module parse them. Keys were **added** (`kind`, `life`,
`registered`, `activates`); a decoder ignores what it does not know. Renaming one of the four must
fail `TestMarcoRoutesJSONKeepsItsPublishedKeys`, and widening the command to `plays.List` must fail
`TestMarcoRoutesOffersOnlyPlaysThatCanAnswer` — a front end that offered a staged name would be
advertising a capability `marco do` cannot find.

The `"[route] "` line `marco do` prints is **wire protocol, not user text**: `plugins/overlay`
parses it, and it did not change with the vocabulary. What did change is the unknown-command error,
now the single prefix `"no play matches "` (`cmd/marco/panicstop.go`, `cmd/marco/bind.go`) with the
overlay's suppression constant `noPlayMatches` moved to match (`TestTheUnknownCommandErrorIsOnePrefix`
and `plugins/overlay/acts_test.go`). It now has a companion, `"[result] "`, which says what BECAME
of the invocation in one of six words — see [[Invocation]] for both lines and why neither can be
derived from the other.

The view IDENTIFIERS in the control centre did not move either: the tab a person reads says
**Plays** while the URL, the `data-view` and `marco ui routes` still say `routes`, and `marco ui
plays` reaches the same view (`TestThePlaysTabIsLabelledPlaysOverTheRoutesIdentifier`,
`TestMarcoUiPlaysOpensThePlaysView`).

### The two surfaces used to disagree, which is why they now share a function

The Learn panel said *"Saved. It is in the Routes tab."* on the strength of a file existing — a
claim about DISCOVERY made from a fact about STORAGE, and false for exactly the play it was said
about. `plays.AfterLearn` is now the one place that turns *saved* + *registered* into words, and
both surfaces read it (`TestLearnReportsTheSameStandingThePlaysListWould`,
`TestTheLearnPanelRendersTheLifecycleWords`, `TestLearnAndThePlaysListingSayTheSameThing`).

## Handles: the slug, never the display name

`Play.Name` is the slug with its dashes back as spaces, and that transformation is **lossy in the
direction that matters** — re-deriving a slug from a shown name can land on a different file, or on
none. So the listing carries `Play.Slug` and every action posts it back unchanged
(`TestRegisteringActsOnTheSlugTheListingCarried`). `marco register` re-slugs the phrase a person
typed and takes the application from the staged row rather than from the foreground window, because
the foreground when you type that command is a terminal
(`TestRegisteringASavedPlayDoesNotDependOnWhatIsInFront`).

## Related

- [[Invocation]] — how a request reaches one of these, and what happens to everything else
- [[Learned-Plays]] — how a Play that Marco learned comes to exist, and each wall on the way
- [[Demonstrations]] — where a Recorded play comes from
- [[Marco-Boundary]] — a Play is legal Marco, and so is everything else that reaches the desktop
- [[34F-legacy-marco-product-audit]] — the audit that counted the six senses of "route"
- [[ADR-081-a-durable-behaviour-is-a-play]] · [[ADR-082-a-plays-past-travels-with-the-file]]
- [[ADR-048-learn-teach-and-do-are-three-different-sentences]] — why Teach is reserved, and why
  that made `teach` the one internal name worth renaming
- [[ADR-086-one-acquisition-one-word-one-request]] — the rename that carried it out
- [[ADR-080-a-learned-play-is-asked-for-from-anywhere]] — why a learned Play is focus-scoped
- [[Glossary]] — the words, in one place
