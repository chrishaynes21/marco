---
type: milestone
status: historical
---

# Semantic text entry and editing

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> The user asked the Director to change **text**, not to press keys.
> The Director plans text state. Marco executes the chosen editing primitive.
> Verification proves the resulting **state**, not the emitted input.

## Why this is not "type into the box"

"Type hello into the search box" is a statement about what the box should **contain**. The
keystrokes are one way of getting there, and frequently the worst one: typing sends characters
through the keyboard layout, the IME, and whatever the application does with each keystroke —
an autocomplete that rewrites the field on the third letter, a validation that rejects an
intermediate state, a dropdown that steals focus. Setting the control's value has none of those
failure modes, because there is no intermediate state for anything to react to.

The planner therefore emits `SetText` and stops. Nothing in the operation vocabulary names a
key, a modifier or a layout — a plan that had already decided "Ctrl+A then type" would have
thrown away a choice that only the execution layer, which can see what this particular control
supports, is in a position to make.

## The operations

| operation | means |
|---|---|
| `set_text` | the field should contain exactly this |
| `append_text` | add to the end of what is already there |
| `insert_text` | insert at the caret, leaving the rest alone |
| `replace_selection` | replace whatever is selected |
| `clear_text` | the field should be empty |
| `copy_selection` | put the selection on the clipboard |
| `paste_clipboard` | paste what is already on the clipboard |
| `select_all` | select the whole contents |
| `undo` / `redo` | the application's own history |
| `press_enter` | commit the field |

`append_text` is genuinely different from `set_text`, not sugar for it: appending requires
**reading** the current value, and a planner that expressed it as `SetText(old+new)` would be
deciding about the old value at planning time — before the field was observed, and possibly
wrong by the time it acted.

## The strategy ladder

| rung | strategy | chosen when |
|---|---|---|
| 1 | the control's own value API | it has one and it is writable |
| 2 | select everything, then replace | no value API — the field is emptied by **selection**, never by held backspaces |
| 3 | clipboard-assisted paste | ≥120 characters, or the text contains line breaks, tabs, or non-ASCII |
| 4 | typing | everything stronger was unavailable or refused |

**Every rung that is skipped or refused is recorded, with its reason.** A fallback that happened
silently would be indistinguishable from a preference, and the next person asking "why did an
autocomplete fire?" would have nothing to go on. The reasons reach the execution trace and
`director edit`:

```
execute  replace_selection via clipboard — verified: the value changed from nothing to
         "Lorem ipsum dolor sit amet consectetur adipiscing elit se..." (120 characters)
         (fell back — value_api: the selection is not part of the World State, so it
         cannot be replaced by a whole-value write)
```

A refusal (`unsupported: the control does not implement ValuePattern`) means "try the next
rung". A real error means **stop** — trying harder against a broken bridge only makes a mess in
the user's document. The two are separate types, not a string match.

### Why non-ASCII goes through the clipboard

The keyboard layout is the application's, not ours, and it must not be assumed to be US.
Characters outside ASCII are exactly the ones a layout may be unable to produce. Line breaks
and tabs go the same way for a different reason: typed into a single-line field a newline
submits it, and typed into a chat box it sends the message half-written. Pasting delivers them
as **data** rather than as keys the application acts on.

## The clipboard is borrowed, never taken

The clipboard belongs to the user and it usually holds something — the URL they were about to
paste, the paragraph they just cut. It has **three** states, and the third one is the safety
property:

| state | what a borrower may do |
|---|---|
| holds text | save it, paste, write it back |
| empty | borrow freely; restore writes `""` |
| holds an image, a file list, … | **refuse to borrow** |

Through the text format alone, empty and non-text both read as an empty string.
`CountClipboardFormats` separates them. Nothing here can save or reproduce an image, so pasting
over one would destroy it while reporting success — the cost of refusing is one fallback to
typing; the cost of proceeding is the user's data.

Restore runs **even after a failed paste**: a failure on our side is not a reason to keep the
user's clipboard. Whether the restore succeeded is recorded rather than assumed, and a failure
is surfaced loudly, because the user's clipboard being wrong is something only the Director
knows about:

```
— WARNING: the clipboard was borrowed and could not be restored
```

`copy_selection` is the one operation that is *supposed* to change the clipboard, so it is
neither borrowed nor restored.

## Focus

**Never type into an unfocused control.** Focus is established *structurally* — by asking the
application through the accessibility interface — and never by clicking, because a click
activates the control, and a click into a document to "focus it" may land on a hyperlink.

`Focus` returning nil only means the request was **accepted**. So the world is re-read
afterwards to confirm the target actually holds focus, and the edit is abandoned if it does
not. This matters more than it looks: input sent to an unfocused control does not fail. It
lands in whatever window *does* have focus and returns success.

## Verification

Every edit is verified against a **re-read of the control**, not against the fact that input
was sent. The difference is not academic:

- Ctrl+Z sent to an application with nothing to undo succeeds at the input layer and changes
  nothing.
- A `SetValue` on a field with a five-character limit returns success and keeps five characters.

Both pass an "the act did not error" check and both failed at what the user asked for. The
UIA bridge reads the value **back** after a write for exactly this reason, so the reply carries
what the control actually holds rather than what it was asked to hold.

| verdict | meaning |
|---|---|
| verified | the control was read and says what it should |
| unverified (**contradiction**) | the control was read and says something else — the edit **stops** |
| unverified (**unknown**) | the value could not be read, or the operation makes no text claim |

**Unknown is not false.** `select_all` and `copy_selection` change no text, so a text comparison
correctly has nothing to prove; a field that could not be read is a gap in perception, not
evidence of a failed edit. Failing on Unknown would abandon edits that actually worked.

Operations whose result is not predictable from the request — `undo`, `redo`, `insert_text`,
`replace_selection`, `paste_clipboard` — are verified by **change** rather than by equality,
which is weaker and is reported as such rather than dressed up as certainty. The caret and the
selection are not part of the World State.

### Edits are never retried on an inconclusive verification

The pipeline's recovery guard normally decides whether to retry by asking whether the screen
changed: if nothing happened, repeating cannot double-apply. That reasoning is unavailable for
an edit, because an edit that landed perfectly can leave the screen identical. Repeating an
append that already worked appends twice, and *"I could not confirm it"* is not *"it did not
happen"*.

## What was added to Marco

The rule is that if Marco lacks an editing primitive, it is added **to Marco** — the Director
remains the planner and Marco remains the execution language.

| primitive | where | why there |
|---|---|---|
| `setvalue` / `getvalue` | `plugins/uia` — `Focuser.SetValue`, `Value.cs` | setting a control's value is an accessibility capability |
| `clipboardget` / `clipboardset` | `internal/oshost/clipboard_windows.go` | reading and writing the clipboard is an OS capability |

`clipboardget` reports `Text`, `IsText` **and** `Empty`, which is what makes the three-state
borrow rule expressible at all.

## Waiting, not sleeping

An edit waits for the control's value to reflect the change by **condition**, and continues the
moment it does. A value-API write is usually verified in a single poll; a browser field that
repaints slowly is given as long as it needs, up to four seconds. Neither case costs the other
anything, which a fixed delay cannot manage.

The pipeline's own post-action region settle is **skipped entirely** for edits: the edit already
proved itself against the one thing it was about, and watching pixels afterwards would spend up
to three seconds answering a weaker question.

## Diagnostics

```
director edit           how the recent edits were carried out, and why
director edit -n 20     more of them
director edit --json    the same, machine-readable
```

```
set_text — set the value to "submitted text"
  strategy   value_api
  before     "replaced by the director"
  after      "submitted text"
  verified: the control now contains "submitted text", which is what was intended
  attempts
    * value_api      the control's own value API accepted the new value
```

`before` and `after` distinguish **empty** from **unreadable**, because only one of them proves
a `clear_text` worked.

## Phrases understood

```
type hello into the search box          set_text
type "hello world" into the message box set_text, quoting removes the ambiguity
clear the search box                    clear_text
add more text to the note               append_text
replace that with goodbye               replace_selection
select all · copy that · paste          select_all · copy_selection · paste_clipboard
undo that · redo                        undo · redo
press enter                             press_enter
```

The parser is conservative: a phrase it is not confident about falls through to the ordinary
intent planner rather than being forced into an edit. "Type up the report" is not a request to
enter the characters *"up the report"*.

Without a separator (`into`, `in`, `to`) the **whole** remainder is treated as text — "type
hello world" enters two words, because guessing a split would silently drop half of what was
dictated. No named control means the **focused** one, which is what "type hello" and "undo
that" mean.

The user's own capitalisation is preserved: parsing happens in lower case for a simpler grammar,
but the text is the user's data and is recovered verbatim from the original phrase.

## Known limitations

- **"…and press enter" is not performed.** It is parsed as a second operation, and the executor
  runs one operation per request. Rather than build a step that would be recorded and never
  run, the plan carries the gap as an assumption it does not satisfy, printed in the trace.
  Ask to press enter as a follow-up.
- **A value written through the UIA value API may not enter the application's undo stack.**
  Observed in Notepad: the edit succeeds and a subsequent `undo` correctly reports having had
  no effect.
- **The keystroke rung is covered by tests rather than by live use.** Every Win32 control tried
  implements `ValuePattern`, so rung 1 always won.
