---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - semantic-memory
  - learn
  - naming
  - control-centre
source_paths:
  - internal/director/semanticmemory/store.go
  - internal/director/observe/screennaming.go
  - cmd/director/learnview.go
  - cmd/director/observeregistry.go
  - cmd/marco/learnui.go
---

# ADR-069 — a name is authored, and can be taken back

## What happened

A live Learn run reached the naming question. Marco asked, in the panel:

> what do you call this screen?

There were two candidate screens. The wording would have been byte-identical about either of
them. The person answered about the one they had in mind — `Mouse Settings` — and the word landed
on the other one, the Bluetooth page, because that was the subject the proposal had been raised
about.

Then it could not be repaired:

- withdrawing the answer removed the **judgement** and left the **name**;
- uniqueness reserved `Mouse Settings` against every other place, permanently;
- so the screen they had actually meant could never receive it.

The only repair was stopping the Director, backing up `semantic-memory.json`, and editing it by
hand. That is not an acceptable thing to ask of somebody who mistyped an answer to a question
Marco asked at the moment Marco was least sure of itself.

## The two rules

> **Marco may not ask the Audience to name something it cannot ground for them.**

A question that cannot say WHICH thing it is about is not a question, it is a coin toss. If Marco
cannot describe the referent in words the person can act on, it must not ask.

> **An audience-supplied name must be reversible.**

Naming is authorship. Authorship that cannot be corrected or withdrawn is not authorship, it is a
one-way commitment extracted at the least informed moment in the whole interaction.

## What follows from them

### Grounding is a description, never an identifier

`subj_543793ccc326` is not an answer to "which one do you mean" — it is what the failure was made
of. `KnownPlace.Describes` says what a place is MADE of, in plain words ("about audio, 14 things
on it"), and `KnownPlace.Handle` travels as an opaque token the surface hands back. The panel
never shows the handle.

Two places may not be described identically. That is asserted, not hoped for: identical
descriptions reproduce the original failure exactly.

Grounding is a description rather than a highlight because the whole accessibility tree is not a
referent. Drawing a box around everything says nothing about which thing is meant.

### The identity is bound before the question is asked

`AnswerName` writes to `q.Screen`, the subject the QUESTION named — never to what is in front of
anybody now, and never to the application that happens to be current. By the time somebody types
a name they have usually moved on, often to another program. A stale proposal names the screen it
was raised about or it names nothing.

### Three operations, one subject id

| operation | request | effect |
|---|---|---|
| name | `Rename: true, Place: id, Called: "X"` | the place is called X |
| rename | the same request again with a different word | the SAME place is called Y; X is released |
| unname | `Rename: true, Place: id, Called: ""` | the place keeps its identity and is called nothing; the word is released |

Renaming does not mint a subject: every route, goal and target pointing at the old id would be
orphaned and nothing would say so. Unnaming does not delete one: removing what somebody CALLS a
place says nothing about the place, and deleting it would kill every route through it because
somebody took a word back.

An empty name is a **retraction**, not a validation error. Refusing it here is what put the person
back where they started.

### Uniqueness is over live names only

Two places may not be called the same thing AT THE SAME TIME — a name has to mean one place or a
play cannot say where it begins. But a word that nothing is currently called is FREE. Reserving
every string ever typed means one mistaken answer burns it forever.

### Which application comes from the place

`applicationOfPlace` reads the namespace off the subject. Deriving it from session context instead
made the first implementation of this ADR fail with `no remembered screen … in ""`, which is the
same class of bug at a smaller scale: guessing the referent from ambient state.

### Naming needs no focus choreography

The person is typing into Marco's own text field, so Marco is necessarily in front. Naming is
semantic editing of an identity that was bound earlier — it is not an observation, and nothing
about it depends on what is on screen when they press Save. A foreground gate here would make the
operation impossible.

### Perception's word is not the Audience's word

A durable Target carries the label off the control as EVIDENCE, in `Structure.Label`, with
`Learned` provenance. It never becomes `Called`. If it did, an observed label would outrank a
person's correction — the one thing authorship must never lose — and an observed word would also
reserve itself against every place. See
[[ADR-068-the-theater-is-the-durable-semantic-world]] for the provenance model this rests on, and
[[ADR-058-a-demonstrated-target-may-keep-its-name]] for what a demonstration may keep.

## Consequences

- Naming a target is OPTIONAL. A target is identified by place, label and kind; a person is asked
  for a word only when one is needed to write a play.
- The naming burden drops: nothing is asked about a place that is already called something, and
  nothing is asked that Marco cannot ground.
- Repair no longer requires the store. There is no supported path that ends in "edit
  `semantic-memory.json`".

## Enforced by

| claim | test |
|---|---|
| the wrong-screen failure is correctable without store surgery | `TestNamingTheWrongScreenIsCorrectableWithoutStoreSurgery` |
| renaming keeps the same place | `TestRenamingKeepsTheSamePlace` |
| unnaming keeps the place and releases the word | `TestUnnamingKeepsThePlaceAndReleasesTheWord` |
| two places may not share a name at once | `TestTwoPlacesMayNotShareANameAtOnce` |
| corrections and retractions survive a restart | `TestNamesAndTheirRemovalSurviveARestart` |
| an observed label is not an Audience name | `TestAnObservedLabelIsNotAnAudienceName` |
| a naming question says which place it means | `TestANamingQuestionSaysWhichPlaceItMeans` |
| no place is described by an identifier, and no two alike | `TestEveryPlaceIsDescribedWithoutAnIdentifier` |
| the answer binds to the subject that was asked about | `TestNamingAScreenBindsToTheSubjectThatWasAskedAbout` |
| an answer does not follow the application in front | `TestAnAnswerDoesNotFollowTheApplicationInFront` |
| naming works while Marco holds the foreground | `TestNamingWorksWhileMarcoIsInFront` |
| a rename with no place changes nothing | `TestARenameWithNoPlaceIsRefused` |
| the whole sequence, in order, across a restart | `TestTheNamingAcceptanceSequence` |
| the rename endpoint carries which place | `TestTheRenameEndpointCarriesWhichPlace` |
| an empty name reaches the Director as a retraction | `TestAnEmptyNameReachesTheDirectorAsARetraction` |
| every verb the panel offers is reachable | `TestEveryLearnVerbIsReachable` |
