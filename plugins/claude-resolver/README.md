# claude-resolver — a Marco resolver plugin

An **optional** plugin that lets `marco assistant` fall back to **Claude Haiku**
when its built-in (deterministic, offline) matcher can't confidently map a
loosely-phrased request to one of your saved routes.

It lives in its **own Go module** so the Anthropic SDK never becomes a
dependency of marco itself — marco core stays zero-dependency. Any program that
speaks the protocol below can replace this one.

## Protocol

One JSON object in (stdin), one out (stdout):

```
→ {"input":"fire up sea of thieves","routes":["start-sea-of-thieves","open-chest"]}
← {"route":"start-sea-of-thieves"}      (empty route = no match)
```

The plugin must reply with an exact slug from `routes`, or `""`. marco verifies
the answer is a known route before using it.

## Build & use

```sh
go -C plugins/claude-resolver build -o claude-resolver .
export MARCO_RESOLVER="$PWD/plugins/claude-resolver/claude-resolver"
export ANTHROPIC_API_KEY=sk-...
marco assistant
> fire up the pirate game      # Haiku maps it to start-sea-of-thieves
```

With `$MARCO_RESOLVER` unset, the assistant uses only the local matcher (no
network, no key needed).
