---
type: adr
status: accepted
date: 2026-11-01
decision: "ACCEPTED — A RE-RENDER KEEPS THE CREDENTIAL IT WAS READING, OR REFUSES. `codegen` writes a DECLARED secret argument under a ROUTE-QUALIFIED name — `sign-in-to-the-portal:password` — and an undeclared one as the bare `password`, and those are two different entries in somebody's credential store. The declared set handed to every re-render came from `routes.SplitArgs` of the routine's NAME, and a SAVED routine's header does not carry the `with …` clause: `Save` writes `type my address`, and the declaration lives in the STEPS as `{{password}}` in the text of a step that types. So the set was EMPTY for every routine in the library, and the five operations that re-render one each rewrote the identifier silently — the review shows the same step list, the same sentence shape and the same word `Secret`, and the only thing that changed is which password comes back, which is none. The step editor did not even get that far: it renders the untouched steps and refuses unless they reproduce the file, so a routine that reads a credential COULD NOT BE EDITED AT ALL, under a sentence about its source and its steps no longer agreeing. TWO HALVES, AND THE SECOND IS NOT A BELT FOR THE FIRST. `renderArgsFor` PRESERVES: it reads the source being replaced and declares exactly the names that source reads under their qualified form, so the render reproduces them — which is why it reads the SOURCE rather than declaring every placeholder it finds, because a routine holding a bare `{{token}}` that its source reads as the global `token` would otherwise be qualified, the same defect pointing the other way. That is enough for the three operations that render the STORED STEPS and keep the route name: a step edit, a rebuild under today's rules, and a move between scopes. `secretsKept` REFUSES, and covers the two where preservation cannot work at all — a RENAME changes the route name so the identifier changes by construction, and a RE-SIMPLIFICATION renders the DEMONSTRATION, which never held the placeholder because a secret's value does not enter a recording. The refusal names the credential it reads now AND the one it would read instead. The step editor deliberately does NOT compare: it cannot fire there, because preservation plus the reproduce-check already refuse anything this cannot keep, and it WOULD fire for something legitimate — removing the step that types the password is an edit somebody may want, and the other three change nothing about the steps so a lost credential there is always an accident. ONE COMPOSER AND ONE READER, in `codegen`: `SecretNameFor` is the same composition `typeSentence` writes with, and `SecretsIn` reads them back, because a second of either is a second thing to disagree with the first and disagreeing here means looking up a secret nobody stored. NOTHING READS, COPIES OR MIGRATES A CREDENTIAL; this compares generated text."
supersedes: []
affects:
  - execution
source_paths:
  - internal/codegen/secretname.go
  - internal/orchestrator/secretkeep.go
  - internal/orchestrator/stepedit.go
  - internal/orchestrator/regenerate.go
  - internal/orchestrator/rescope.go
---

# A re-render keeps the credential it was reading

The correction [[ADR-285-a-routine-can-be-called-something-else]] recorded as an open finding and
did not make. The Architect:

> A step edit or rescope must not silently change which credential a routine reads. Preserve the
> exact existing lookup identifier through every supported re-render, including rename. Do not
> read, copy or migrate credentials. If a source cannot be preserved safely, refuse that
> operation explicitly.

## The defect

```go
// codegen, typeSentence
if seg.qualified {
    name = secretName(g.base, seg.text)   // "sign-in-to-the-portal:password"
}
return fmt.Sprintf("OS's Secret with \"%s\"", escape(name))
```

`qualified` is `isDeclared`. The declared set came from the routine's **name** — and a saved
routine's header has no `with …` clause, because `Save` writes the base name and the declaration
lives in the steps as `{{password}}`.

So every re-render of a saved routine wrote the **bare** name. Two different entries in a
credential store; one routine that signs in with nothing; and every review of it looks correct.

**And the step editor could not open such a routine at all.** It renders the untouched stored
steps and refuses unless they reproduce the file byte for byte — the right rule, firing for the
wrong reason:

> *its saved source is not what its stored steps render to under today's rules, so the two no
> longer agree*

## Preserve

`renderArgsFor(name, src, steps)` = what the routine declares (`declaredFor`) **plus** the
credentials this source already reads under their qualified name.

It reads the **source**, not the steps alone, and that is the whole of the design. Qualification
is not a property of the name; it is a property of what the file says. A routine can hold a bare
`{{token}}` that its source reads as the global `token`, and declaring every secret placeholder
would qualify it — the same defect pointing the other way. So each name is declared only when the
source already reads `SecretNameFor(base, name)`, and a name the source reads under neither shape
is left undeclared and caught by the comparison.

That is enough for the three operations that render the stored steps and keep the route name:

| | |
| --- | --- |
| a step edit | the steps hold the placeholder; the name does not change |
| a rebuild under today's rules | same |
| a move between scopes | same — the slug is the identity, and a rescope keeps it |

## Refuse

`secretsKept(was, now, doing)` compares the names read before and after, and covers the two where
preservation has nothing to work from:

- a **rename** changes the route name, so the qualified identifier changes with it by
  construction;
- a **re-simplification** renders the **demonstration**, which never held the placeholder — a
  secret's value does not enter a recording, which is the whole of why `OS's Secret` exists. The
  rebuilt routine would compile, run, and type nothing into the password field.

The refusal names the credential it reads now and the one it would read instead. The alternative
is moving the credential, and this code does not touch a credential store.

### And the step editor deliberately does not compare

Two reasons pointing the same way. It **cannot fire** there: preservation plus the
reproduce-check already refuse anything this cannot keep, so an edit of such a routine never
starts. And it **would fire for something legitimate**: removing the step that types the password
is an edit somebody may well want, and it leaves a routine reading no credential. The other three
change nothing about the steps, so a lost credential there is always an accident; here it can be
the request.

A guard that cannot fire reads to the next person exactly like an enforced rule — two of those
were written and removed in the slice that produced this editor.

## One composer, one reader

`codegen.SecretNameFor` is the same composition `typeSentence` writes with, and
`codegen.SecretsIn` reads them back out of a generated source. ADR-285 avoided deriving the
qualifier for exactly this reason and compared two sources instead; now there is one place for
both, so the comparison and the generator cannot drift. A second composer or a second reader is a
second thing to disagree with the first, and disagreeing here means looking up a secret nobody
stored.

## What this does not do

- It does not read, write, copy or migrate anything in a credential store.
- It does not change what the step editor offers to turn into a supplied value — `declaredFor`
  and `suppliedIn` answer that question and are untouched.
- It does not change the DRAFT path. A draft carries its arguments in `Draft.Args` from the
  phrase it was named with, so its secrets were never unqualified.

## Enforced by

- `cmd/marco` `TestAnEditedRoutineStillAsksForTheCredentialItAlwaysAskedFor` — **execution
  evidence**, against a SUBSTITUTED credential store that writes down every name it is asked for.
  The run asks for `sign-in-to-the-portal:password` before the edit and for the same name after
  it. The stored value is EMPTY, which submits no characters, so nothing is typed into whatever
  has focus; nobody's real credential manager is touched.
- `cmd/marco` `TestARefusedRebuildLeavesTheRoutineAskingForWhatItAlwaysAskedFor` — the other
  half, at the run: a rebuild that cannot keep the credential is refused by name, and the routine
  still asks for what it always asked for.
- `internal/orchestrator` `TestARoutineThatReadsACredentialCanBeEditedAndKeepsIt` — it can be
  edited **at all**, the edited version reads the same credential, the saved file does, and it
  can be edited again.
- `internal/orchestrator` `TestAMoveKeepsTheCredentialItWasReading` — **this test recorded the
  defect**. It was written as a pinned finding that would fail loudly the day it was fixed; it
  did, and it now holds the correction.
- `internal/orchestrator` `TestARebuildUnderTodaysRulesKeepsTheCredential` and
  `TestARebuildThatCannotKeepTheCredentialIsRefused` — the two halves of one question with
  different answers, because what a rebuild can preserve depends on what it renders FROM.
- `internal/orchestrator` `TestARenameThatWouldMoveACredentialNameIsRefused` — refused at
  preparation and at the write, with the routine unchanged.
- `internal/orchestrator` `TestWhatARenderDeclaresComesFromWhatTheSourceReads` — a qualified
  credential is declared, a bare one is NOT, and a value somebody supplies still is.
- `internal/orchestrator` `TestACredentialTheSourceDoesNotReadIsNotDeclared` and
  `TestTheRefusalNamesBothCredentials`.

Every assertion names the identifier. A routine that still says `Secret` and says it about
another name is the defect, not the fix.

## See also

- [[ADR-285-a-routine-can-be-called-something-else]] — where this was found and recorded
- [[ADR-276-a-routine-may-be-told-what-to-type]] — the placeholder all of this is built on
- [[ADR-279-a-simplified-save-keeps-its-own-basis]] — the step account the three preserving paths
  render from
