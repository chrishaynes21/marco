---
type: reference
status: accepted
date: 2026-10-30
source_paths:
  - internal/routes/args.go
  - internal/simplify/simplify.go
  - internal/codegen/codegen.go
  - internal/invoke/invoke.go
  - internal/orchestrator/authority.go
  - cmd/marco/intake.go
  - cmd/marco/panicstop.go
  - cmd/marco/assistant.go
  - cmd/marco/bind.go
  - plugins/overlay/acts.go
  - plugins/studio/routines.go
---

# Text arguments — saying the part that changes

Traced at `fff901f`. The ask is a routine that accepts changing text without being recorded
again, the smallest Studio flow for entering a value, and the values carried with the exact route
through confirmation — on a contract Director's own extraction can later supply values through.

**All three increments are now built.** What was built, and what holds each rule in place, is
[[ADR-276-a-routine-may-be-told-what-to-type]]. No live execution and no acceptance run were
taken.

**D1 to D4 are ruled on and recorded in §5. Five amendments came with them and are folded
into the rules below**, each marked AMENDED: missing is not empty, the authorised invocation is
immutable, values stay out of durable diagnostics, the newline refusal is narrowed to what is
actually established, and the secret path is described as it behaves rather than as it ought to.

**The mechanism is already built, end to end.** What is missing is a way for Studio to reach it, a
refusal when nothing reaches it, and the values being part of what somebody is asked about. That
makes this a smaller slice than it sounds, and a sharper one: three gaps, each of which is a
sentence of code and a rule.

## 1. What exists

### Declaring

A name declares the arguments. `routes.SplitArgs` splits a LEARN name on `" with "`:
`learn "say hello with name"` → the routine `say hello` taking `name`; `"dm with person, message"`
takes two.

The POSITION is given during the demonstration, by tapping a reserved key —
`orchestrator.Deps.ArgKey`, passed to `simplify.Options.ArgKey`, defaulting to
`simplify.DefaultArgKey`. `simplify` turns that tap into a `Type "{{name}}"` step, named from the
declared list in order (or positional `{{N}}` when the name list runs out), and `codegen` keeps the
placeholder inside the string literal it sits in.

So the thing a person does is: name the argument when they start, and tap a key at the moment in
the demonstration where its text goes.

### Listing

`routes.ArgNames(src)` reads a routine's source and returns what it accepts, first-seen order —
both `{{name}}` placeholders and route-qualified secret arguments. `marco args "<phrase>"` prints
them one per line, which is a machine-readable listing a surface can build a prompt from, and
`plugins/overlay`'s `argHints` is the surface that does: it shells out and auto-pops `name:`
labels into the command line.

### Filling

`routes.ApplyArgs(src, named, positional)` substitutes them at run time, in
`cmd/marco/panicstop.go`'s `take` — the one place it happens. Named values match by name, case
insensitively; positional values zip onto the declared names in source order, so `with chris` is
`name:chris`; an explicit `name:value` wins over the positional mapping; each value is escaped for
the literal it lands in.

`routes.ParseInvocation` is the grammar that produces those two: `say hello name:chris` preferred,
`say hello with chris` as the positional fallback, with a named value running on until the next
`key:` token so it can contain spaces.

### And a secret-typed argument is not a text argument

`codegen` turns a declared argument whose name reads as a secret into
`do OS's Secret with "<route>:<name>"` rather than a placeholder, and `oshost.Host.SetArgs` lets a
run provide one inline and remember it. `ArgNames` returns both kinds, which matters below: a
surface that treated the list as uniformly "text to type" would put a password in a text box.

### Which surfaces can supply a value today

| surface | can it | how |
| --- | --- | --- |
| `marco do "say hello name:chris"` | **yes** | a phrase, read by `ParseInvocation` |
| `marco hotkey` | **yes** | `bind.go` carries an explicit `Play` AND `Named`/`Positional` parsed from the binding's text |
| the overlay | **yes** | `argHints` pops the labels; the person types the value into the phrase |
| `marco dispatch` / the resolver | **no** | `invoke.Request{Play: &p.Route}` with no values; they are dropped |
| **Studio's Play** | **no** | `doCommand` sends `do --play=<slug> [--app] [--focus] [--window]` and nothing else |

`cmd/marco/bind.go` is the precedent that matters, because it is already the shape being asked
for: an explicit identity and its values, travelling together, with no phrase to re-read.

## 2. The three gaps

**G1. A routine that needs text plays EMPTY, and nothing says so.** `ApplyArgs` leaves an
unfilled placeholder empty by design — it is called on every run and most routines have no
placeholders — so a Studio Play of `say hello with name` types nothing where the name goes. The
card cannot warn: `plugins/studio.Routine` has no argument field, `Routines` never asks
`ArgNames`, and `Unrunnable` says nothing about placeholders, so the routine is listed as ready
and the run reports `performed`.

The product already holds the opposite rule for the mirror case.
`TestALearnedPlayRefusesArgumentsRatherThanDroppingThem` refuses *arguments* rather than dropping
them, and says why: *"dropping them silently would let somebody watch the wrong thing happen and
believe they had asked for it."* Dropping the ABSENCE of an argument is the same sentence with the
subject changed, and nothing refuses it.

**G2. The values are not part of what is authorised.** `orchestrator.Resolved` carries the route,
the phrase, the kind and the provenance — and no values. `Authorize` runs on that, and `take`
applies the values afterwards. So there is nothing for a confirmation to show and nothing binding
what a person was shown to what gets typed. For a routine whose whole purpose is to type text
somebody supplies, the text is the part worth confirming.

**G3. There is no wire from a separate process.** `invoke.Request.Named`/`Positional` exist, with a
comment saying they are "arguments the entrance already peeled off, for a surface that already
peeled them off" — and only an in-process caller fills them. `parseDoArgs` has no flag that
does, and `performDecidedPlay` deliberately does NOT read the words of an explicit identity for
arguments:

> An EXPLICIT identity carries no words to parse. […] Re-reading a display name for arguments
> would be the same mistake as re-reading it for identity.

That rule is right and must stay — which is exactly why appending `name:value` to Studio's argv
would do nothing, and why a flag is needed rather than a phrase.

**And one defect found while tracing, which a text box makes reachable.** `escapeArg` escapes `\`
and `"` and nothing else. A value containing a LINE BREAK produces source that does not lex:

```
argcheck/nl.marco: lex: 5:19: unterminated string literal
    do OS's Type with "line one
                      ^
```

Reachable today only by typing a newline into a shell argument; one paste away from a Studio text
box.

## 3. The rules

**R1. A routine says what it needs, and the card says it.** From `routes.ArgNames` on the source
the card already carries — no new store, no new sidecar, nothing to keep in step. Each argument is
named, in the order the routine uses it.

**R2. A routine that needs text and has none REFUSES, rather than typing nothing.** The mirror of
the learned-play rule, for the same reason in the same words. It belongs at the DOOR, not only on
the card, so every entrance gets the same answer: Studio's Play is dead with the reason beside it,
and `marco do --play=…` with a missing value refuses instead of performing. A card that merely
greyed a button would leave the CLI silently typing empty text.

**AMENDED: MISSING IS NOT EMPTY, AND THE TEST IS WHETHER A VALUE WAS SUPPLIED.** Not whether it has
length. An empty string somebody supplied ON PURPOSE is a value — clearing a field is a thing
people do, and a routine that types nothing where they asked for nothing is correct. So the check
is presence, which a map answers (`v, ok := named[name]`) and a length never can; `len(v) == 0`
would refuse the one case the person meant and is the obvious wrong implementation.

**And whitespace is a value too.** A value is used exactly as supplied — not trimmed, not
collapsed — because a leading space in a typed field is text somebody typed.

**AND THE NAMES ARE CHECKED BOTH WAYS.** A value for a name the routine does not take is REFUSED,
not dropped: it is somebody expecting text to appear somewhere, and the routine has nowhere to put
it. A name given TWICE is refused rather than one of them winning, because a map cannot hold the
second and nothing afterwards could notice — which means the duplicate has to be caught where the
flags are read, before the map exists.

**R3. Values travel with the identity, never as words.** A repeatable flag — one per value, named
— filling `invoke.Request.Named`, exactly as `--play`, `--app` and `--focus` carry the identity.
The explicit-identity rule above is untouched: the phrase is still never re-read.
`TestTheEntranceNeverPutsAFlagInThePhrase` is the existing guard that a new flag must not break,
and it is the reason the flag is spelled as one rather than as trailing words.

**R4. The values are part of what is authorised, and what runs is what was authorised.**
`Resolved` carries them, the confirmation names the text that will be typed, and `take` applies
the values the decision carried rather than re-reading anything. This is the half that makes a
confirmation worth having: [[ADR-029-resolution-is-not-permission]] says resolution is not
permission, and for a routine that types what somebody supplied, the supplied text is most of what
permission is about.

**AMENDED: THE AUTHORISED INVOCATION IS IMMUTABLE, AND SEALING A MAP BY REFERENCE IS NOT THAT.**
The values are COPIED into the decision, and the seal digests the values themselves — because a
decision holding the caller's map would be a decision that changes when the caller does, and the
digest taken at authorisation would still match. That is the defect
[[ADR-271-a-rescope-is-a-reviewed-move-between-identities]] records about `Was`: a stored reading
compared against a mutable field, where what was DISPLAYED and what currently EXISTS are two
questions and only one was being asked.

So a confirmation is bound to three things: the exact route, the digest of the source as it then
stood, and the values as they then stood. If the routine has been written since, or any value has
changed, the write REFUSES and says which — the same contract the step editor's review and a
move's token already use.

**AND THERE IS A CONFIRMATION TO BIND, WHICH TODAY THERE IS NOT.** `Authorize` asks only about a
COMPOSED play; an authored or demonstrated routine is `Allowed` with `ReasonOrdinary` and nothing
is put to anybody. A routine that types text somebody supplied is the first case where the
invocation itself is worth reading back, so the Play flow gains one — for a routine with
arguments. A routine with none is unchanged, and Play still presses.

**R5. A secret-typed argument is not a text argument, and the panel must tell them apart.**
`ArgNames` returns both kinds, so a surface that treated the list as uniformly "text to type"
would put a password in a text box.

**AMENDED: SAID AS THE PATH BEHAVES.** `oshost.doSecret` does NOT prompt. It takes the value from
the CREDENTIAL STORE, and when the store does not have it the run refuses and says how to set it
(`marco secret set <key>`). So the panel says where the value comes from and what to do when it is
not there — it does not promise that Marco will ask, because nothing on that path asks.

**AND THE NEW FLAG IS NOT ANOTHER INLINE-SECRET ENTRANCE.** One already exists: a phrase carrying
`password:value` reaches `doSecret`, which uses it AND REMEMBERS it in the store. Whatever is
thought of that, it is not something to acquire a second time through a flag a window fills in. A
value offered for a secret-typed argument is therefore REFUSED by the flag, naming the argument and
`marco secret set`. The phrase path is unchanged; this slice adds nothing to it.

**R6. A value is per-run and is never stored — and that covers the DIAGNOSTICS.** Not in the
route, not in a sidecar, not in the recording, not remembered between presses, and **not in a run
record, a journal, a log line or an error message**. The secrets invariant generalises: text
somebody typed once for one run is not a durable property of the routine, and the moment it is kept
it is something to leak.

**AMENDED, because the leak is easier through a log than through a file.** A diagnostic names the
ARGUMENT and not its value: "no value was given for `message`" and never the message.
`cmd/marco/panicstop.go` already sets the precedent — it logs `"named_args", len(s.named)`, a
count — and `oshost.SetArgs` logs the KEYS. Both are the shape to follow, and the new paths must
not undo them: a flag's value must not reach a log line, a refusal's sentence, a run record, or
anything a gate collects.

**The CONFIRMATION is the exception, and it is not a diagnostic.** It exists to be read by the
person who typed the text, it is on a screen rather than on disk, and showing it is the whole point
— rendered as text, so a value that looks like markup or a control sequence reads as the
characters it is.

**R7. A value this substitution cannot represent is refused before anything runs.**

**AMENDED, AND THE REASONING NARROWED TO WHAT IS ESTABLISHED.** A carriage return or a line feed is
refused in this increment because `escapeArg` escapes `\` and `"` and nothing else, so a value
containing one produces source that does not lex — shown above. That is a fact about the
substitution as it stands, and it is the whole of the reason.

What was claimed in the first draft and is NOT established: that a demonstration cannot contain
multiline text. Nothing traced here says that, and the question of whether a routine should be able
to type several lines is a separate one with its own answer. The refusal is about today's
substitution and says so, so that whoever lifts it knows what they are lifting.

**And the rest of the character space is exercised rather than assumed.** A quotation mark, a
backslash, and text outside ASCII all go through `escapeArg` and the compiler, because "two
replacements are enough" is exactly the kind of claim that is true until somebody pastes something.
The check belongs beside `ApplyArgs`, so every entrance gets it.

**R8. Nothing about this adds an execution path.** Studio still spawns `marco do` once per press,
and the values ride on that argv. No second runner, no Studio-side substitution, and the source on
disk is never rewritten — `ApplyArgs` fills a copy in memory, which is what keeps a routine with a
placeholder a routine rather than a template that has been edited.

## 4. The smallest Studio flow

**BESIDE THE PLAY FLOW, AND BEFORE ITS CONFIRMATION.** Not below the step panel and the phrases
panel: those are maintenance, and these are REQUIRED INPUTS to running the routine. A field a
person must fill in before they can press Play belongs beside Play, next to the window it will
press in.

1. **The Play row says what the routine needs.** One line per argument, named, in use order, with a
   secret-typed one said to come from the credential store (R5).
2. **A box per text argument**, empty, nothing prefilled — see R6 and D3. No box for a secret.
3. **Play is dead until every text argument has been GIVEN a value** — given, not non-empty (R2)
   — with the reason beside it rather than a greyed button on its own.
4. **Pressing Play shows what will be typed**: the routine, where it works, the window it will
   press in, and each value as the text it is. That is the confirmation R4 binds, and it is the
   step the fields sit before.
5. **Confirming spawns the one `marco do` the press already spawns**, with the values on its argv
   and the confirmation's own token answering for them, and the run is reported exactly as today.

No history, no remembered values, no editing a placeholder, no adding or removing arguments — all
of that is a re-record or a step edit, and both exist.

## 5. The decisions, as ruled

**D1 — RULED: enforce at the door, and the card also knows.** Two statements of one rule, which this repository usually treats as a
smell — but the alternative is that one entrance types empty text, and the card's copy is what
makes the button honest rather than live-and-then-refused. The rule's owner is the door.

**D2 — RULED: named flags only.** `ApplyArgs`
already zips positional values onto the declared names, so named loses nothing, and a positional
flag would make the order of two flags significant — which is the kind of thing that is right in a
test and wrong on somebody's machine.

**D3 — RULED: Studio retains no previous value.** Per R6. It is the obvious
convenience and it is how a password ends up on disk: the window cannot tell, from `ArgNames`
alone, that a value a person typed into a plain text box was not something they would mind being
kept. If prefilling is wanted later it is a separate decision with a store behind it.

**D4 — RULED: explicit identity plus `Request.Named` is the shared contract**, reached the way
each surface already reaches the door — the
resolver path is in-process (`cmd/marco/dispatch.go`, which drops values today) and Studio's is a
flag. The CONTRACT is `Request.Named` plus an explicit `Play`, which is what `bind.go` already
does; the flag is one way of filling it, not the contract.

## 6. What must hold

Not written, and named here as not written. In the order the rules above state them.

**The listing.** A routine's arguments read off its own source, in use order, with a secret-typed
one distinguished from a text one — and a routine with no arguments offering nothing, because
every routine already saved is that and a panel on every card would be noise.

**The refusal, at the door and on the card.** A routine that needs text, played with none,
refusing rather than performing — through the production entrance, because the thing being
prevented is a `performed` outcome. And the same routine with its value supplied, performing, with
the text where the placeholder was: the control, because a contract that refuses everything passes
every test of a refusal.

**The values travelling.** A value given to Studio reaching the run, with the identity explicit and
no phrase involved; an unrecognised flag never becoming part of a phrase (the existing guard); and
a run that was authorised with a value typing THAT value rather than one re-read afterwards.

**The confirmation.** What a person is asked about naming the text that will be typed, and a value
changed after the asking refusing rather than running — which is the same binding the step editor's
review and the move's token already use, and the reason R4 puts the values on `Resolved`.

**The secret boundary.** A secret-typed argument never appearing as a text box, never on an argv,
and still reaching the credential store at run time.

**And the value that cannot be written down.** A line break refused before anything runs, with the
lexer error above as the reason it is a refusal and not an escape.

## 7. Increments

1. **The listing and the refusal.** `routes.ArgNames` through to a card that says what a routine
   needs, and a door that refuses a missing value. Nothing new travels yet; what changes is that a
   routine which would type nothing is no longer offered as ready.
2. **The wire and the confirmation.** The flag, `Resolved` carrying the values, and the
   confirmation naming the text. This is the increment the Architect's phrase is about.
3. **The Studio boxes**, on top of the two above.

Director's extraction is a fourth and stays SEPARATE: it fills `Request.Named` against an explicit
identity, which is D4's contract, and the first three increments are what make that worth doing.

**The controls this slice must include**, named by the ruling: explicit empty text, a stale source,
a value changed after confirmation, and the absence of supplied text from every durable record.

## What this would NOT establish

That a routine typed the right text **on a desktop**. Everything above is a value reaching a
substitution and a substitution reaching a compile; what a real application does with the
characters is a live trial, and the one in front of this slice is still the recording trial that
has not happened.
