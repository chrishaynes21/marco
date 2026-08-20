---
status: reference
---

---
# Actors {#actors}
---

Actors are miniature state machines. They are the long-lived entities in Marco that own state, speak and hear messages on channels, and coordinate execution.

---
## Capabilities {#capabilities}
---

Actors can:

- own state
- hear messages
- speak messages on channels
- react to [[Frames|Frame]] statuses
- coordinate other actors

---
## Declaration {#declaration}
---

Canonical:
```marco
the Game is an actor.
this can Save.
this's Save does...
    this is ok!
```

See [[Actions]] for action declaration forms and [[Modules]] for module structure.

---
## Messaging Syntax — Channels {#messaging-syntax}
---

Actor messaging is conversational. Speakers `says` messages; listeners `hears` them. Channels coordinate actors and are ephemeral by default.

Speaking:

```marco
says <Message> to <Target>!
```

`says` is third-person, conjugated to the implicit subject (`this` by default).

Hearing:

```marco
when <Target> hears <Message>?
when <Target> hears <Message> from <Source>?
```

The optional `from <Source>` filter narrows to a specific speaker.

### Examples

Canonical:
```marco
says SaveRequested to Game!
when Game hears SaveRequested? do Game's Save.
```

Filter by source:

```marco
when this hears SaveRequested from Player? do this's Save.
```

Actors may hear their own messages:

```marco
when this hears SaveFailed? log that's error.
```

### Speakers and Listeners

- A **speaker** uses `says ... to ...!` to emit on a channel.
- A **listener** uses `when ... hears ...?` to receive on a channel.
- Channels are conversational and ephemeral — for stream/persistent semantics, use [[Iteration#Feeds|feeds]] instead.

---
## Message Payloads {#message-payloads}
---

Messages may carry data via `with`.

### Canonical Form

```marco
says <Message> with <Data>.
says <Message> with <Data> to <Target>.
```

### Examples

Speaker:
```marco
says SaveRequested with User.
says SaveRequested with User to Game.
```

Listener:
```marco
when Game hears SaveRequested?
    when that has User?
        do Save with that's User.
```

### Rules

- the payload becomes part of the message Frame's result set
- listeners access it via `that` / `that's <field>`
- payload fields require [[Data Model#Presence and Safe Access|`has` proof]] if optional
- payload fields declared as required by the message contract are always safe

---
## Messages Are Frames {#messages-are-frames}
---

A message creates a [[Frames|Frame]] that may have:

- `input`
- `status`
- `result`
- `error`

Actor listeners observe message Frames.

### Example

Canonical:
```marco
the Game is an actor.

when this hears SaveRequested? do this's Save...
    when that is failed?
        says SaveFailed to this!

when this hears SaveFailed?
    show ErrorBanner with that's error.
```

---
## Group Routing Via Sets {#group-routing}
---

Sets may be used as routing targets. A `say` to a set fans out to every member that hears the message.

### Example

Canonical:
```marco
the Components is a set.
it has SaveButton.
it has ErrorBanner.

says ReloadRequested to Components!

when this hears ReloadRequested? do this's Reload.
```

---
## Rules {#rules}
---

- `says` emits a message Frame on a channel
- `to` chooses the hearable target
- `from` (optional, on listener side) filters by speaker
- if no target is provided, the message is heard in the current graph scope
- listeners are reactive branches
- listeners do not own returns
- each listener runs in its own Frame
- multiple listeners may hear the same message
- listener order is not guaranteed unless explicitly sequenced

See [[Graph#Messaging]] for the graph-level interpretation and [[Phrases#Branch Rule]] for why listeners do not own returns.

---
## Built-In Actors {#built-in-actors}
---

`Frame` is a built-in actor. Every line creates a Frame; capabilities like `rollback`, `retry`, and `cancel` are actor actions on that Frame. See [[Frames#Frame As Actor]].

---
## One-Line Lock {#one-line-lock}
---

Actors communicate on channels — speakers `says` messages; listeners `hears` them. Channels are conversational and ephemeral; feeds are streams.

---
## Open Questions {#open-questions}
---

- Formal grammar for `says` without a `to` target (graph-scope routing).
- Listener prioritization or explicit sequencing syntax.
- Whether actors may filter messages by content rather than name only.
- Lifetime of message Frames after all listeners have completed.
- How group routing via sets composes with [[Contracts|contract]] obligations across listeners.
