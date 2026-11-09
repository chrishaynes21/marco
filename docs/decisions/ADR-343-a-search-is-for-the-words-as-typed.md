---
type: adr
status: accepted
date: 2026-11-09
decision: "ACCEPTED — A WEB SEARCH IS FOR THE WORDS AS TYPED. When Director's resolver chooses Search the web for a line that BEGINS with \"search\" or \"search for\", the query is the rest of the line exactly as typed, not the model's reading of it. The owner typed \"search broken titan builds\" and was offered a search for \"titan builds\": the model, told that the words naming a play are never a value even when misheard, read \"search broken\" as a mishearing of \"search the web\". Whether the line is a web search at all is still the model's judgement, so \"search for wood\" still reaches the plays first and is still not read as a search locally (skills.Read, ADR-327). A line that begins otherwise (\"look up titan builds on google\") keeps the model's reading; any other skill chosen for a \"search\" line keeps its value; and \"search for\" with nothing after it has no query, not the query \"for\"."
supersedes: []
affects:
  - interface
source_paths:
  - internal/resolveroute/resolveroute.go
---

# A search is for the words as typed

Follows [[ADR-327-built-in-desktop-skills]], whose rule "a query is passed on as entered" held for
the explicit forms and not for a search the resolver chose.

## Why not read "search …" locally

That was the first fix tried and it was undone before it was committed. A search runs on the first
Enter in the widget, and `TestTheExplicitFormsAreRead` holds that "search for wood" is not a search:
in a game it is more likely somebody's play. Read locally, it would open a browser without Director
being asked whether a play answers to it.

## Not established

- Not tried in the widget. Tested at the resolver with the model's answer stated, including the
  span the owner's model returned.
- That "search broken" was read as a mishearing is an inference from the extraction prompt; the
  model's reasoning was not recorded.

## Mutation run

Four mutants (the typed query used, "search for", only the search skill, a bare "search for"); all
killed once the bare case was given its own test. The empty-query check it replaced could not fire,
because the line is trimmed first.

## Enforced by

- `internal/resolveroute`: `TestASearchTheResolverChoseIsForTheWordsAsTyped`, `TestOnlyASearchTakesTheWordsAfterSearch`.
