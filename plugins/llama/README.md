# llama — a LOCAL-first Marco resolver plugin

An **optional** plugin that lets `marco assistant` fall back to a **small language
model** when its built-in (deterministic, offline) matcher can't confidently map a
loosely-phrased request to one of your saved routes — so Marco understands
"fire up the pirate game" as `start-sea-of-thieves`.

It runs a model **on your own machine by default** (via
[Ollama](https://ollama.com)) — no API key, no cost, nothing leaves the computer,
no public API to hammer. It speaks the **OpenAI-compatible** Chat Completions
protocol, so the *same* plugin can also drive LM Studio, a `llama.cpp` server, or
OpenAI's cloud API if you'd rather bring a key.

It lives in its **own Go module** and uses only `net/http` — no model SDK ever
becomes a dependency of marco core, which stays zero-dependency. Any program that
speaks the protocol below can replace it.

## Protocol

Same as every Marco resolver — one JSON object in (stdin), one out (stdout):

```
→ {"input":"fire up sea of thieves","routes":["start-sea-of-thieves","open-chest"]}
← {"route":"start-sea-of-thieves"}      (empty route = no match)
```

The plugin replies with an exact slug from `routes`, or `""`. marco re-verifies
the answer is a known route before using it, so a bad reply can never run.

### Converse mode (the conversational dispatcher)

The same binary also serves the richer **dispatch** protocol — pass
`"mode":"converse"` and it classifies intent instead of only matching a route:

```
→ {"mode":"converse","input":"make a command that mutes discord",
   "routes":["open-chest"],"app":"Discord"}
← {"intent":"teach","name":"mute discord","route":"",
   "reply":"Sure — show me how and I'll remember it."}
```

`intent` is one of `run` (with a verified `route`), `teach` (with a `name`), `chat`,
or `clarify` (both with a `reply`). The engine's `internal/dispatch` package owns
the policy and re-validates every answer, and is model-independent — this plugin is
just one pluggable "Advisor" behind it (`$MARCO_ASSISTANT`, falling back to
`$MARCO_RESOLVER`). Try it with `marco dispatch "<phrase>" --json`.

(This is route-level dispatch, not the Director — see `internal/director`, which
builds a world model of the desktop and plans UI actions against it.)

## Quick start (local, recommended)

```sh
# 1. Install Ollama and pull a small instruct model (one time)
#    https://ollama.com/download  — then:
ollama pull llama3.2:3b

# 2. Build the plugin
go -C plugins/llama build -o llama.exe .

# 3. Point marco at it
export MARCO_RESOLVER="$PWD/plugins/llama/llama.exe"
marco assistant
> fire up the pirate game      # the local model maps it to start-sea-of-thieves
```

With `$MARCO_RESOLVER` unset, the assistant uses only the offline matcher (no
model needed). If Ollama isn't running, the plugin returns "no match" and the
assistant simply falls back — it never blocks.

`setup.ps1 -Llama` automates all of the above on Windows (installs Ollama, pulls
the model, wires `MARCO_RESOLVER` into `overlay.cmd`).

## Configuration (environment)

| Variable | Default | Meaning |
| --- | --- | --- |
| `MARCO_LLM_URL` | `http://localhost:11434/v1` | OpenAI-compatible base URL |
| `MARCO_LLM_MODEL` | `llama3.2:3b` | model tag to ask |
| `MARCO_LLM_KEY` | *(none)* | bearer token — only for authenticated endpoints |
| `MARCO_LLM_TIMEOUT_MS` | `20000` | hard cap so a cold model can't hang the assistant |

### Using OpenAI (ChatGPT) instead — opt-in

The same plugin talks to OpenAI's API; you just repoint it and add a key. This is
**not** the default (local keeps your commands private and costs nothing):

```sh
export MARCO_LLM_URL=https://api.openai.com/v1
export MARCO_LLM_MODEL=gpt-4o-mini
export MARCO_LLM_KEY=sk-...
```
