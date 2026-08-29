package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// MARCO, TELL ME WHEN YOU LEARN SOMETHING.
//
// # What this is, and what it deliberately is not
//
// It is a line per COMMITTED change to durable knowledge — a Place established, a way between two
// screens remembered, a name settled, a destination bound to a word. Six lines in a working
// session, not six thousand.
//
// It is not a log of what Marco perceived. Samples, walks, candidates and confidences are Marco
// LOOKING, and a person watching for the moment they can turn round and say "alright, do that
// then" would never find it under those. The distinction is the whole design: a feed that
// reported perception would be technically richer and practically useless.
//
// Nothing here decides what counts as learning. The Director reports what its semantic memory
// committed, after the write succeeded, and this prints it. If a write is refused at a bound, or
// turns out to describe a Place already held, or cannot reach the disk, this stays silent —
// which is the honest answer and the reason the feed can be believed.

// learningPoll is how often the feed asks.
//
// Durable knowledge changes at human speed — somebody opens a screen, walks somewhere, teaches a
// name — so a second is well inside the latency anybody notices, and it costs one small read of a
// ring buffer. A faster poll would be measuring the Director rather than watching the person.
const learningPoll = time.Second

// followLearning prints durable knowledge changes until interrupted.
func followLearning(client *service.Client, jsonOut bool) int {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)

	if !jsonOut {
		fmt.Println("Watching. I'll tell you when I learn something you can use.")
		// ASCII on purpose. This lands in a Windows console, and an em-dash arrives there as
		// mojibake often enough that a brand-new surface should not open with one.
		fmt.Println("  Ctrl+C to stop watching this feed - Marco keeps watching.")
		fmt.Println()
	}
	var cursor uint64
	for {
		raw, err := client.Observation(service.ObserveQuery{
			Learning: &service.ObserveLearning{After: cursor},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "marco: %v\n", err)
			return 1
		}
		var view service.LearningView
		if err := json.Unmarshal(raw, &view); err != nil {
			fmt.Fprintf(os.Stderr, "marco: the Director's reply was unreadable: %v\n", err)
			return 1
		}
		cursor = view.Newest
		if view.Missed > 0 && !jsonOut {
			// SAID, never swallowed. Somebody who looked away and came back to silence
			// would conclude Marco had learned nothing while they were gone.
			fmt.Printf("  … %d earlier change(s) have scrolled out of the Director's memory\n",
				view.Missed)
		}
		for _, e := range view.Events {
			if jsonOut {
				out, _ := json.Marshal(e)
				fmt.Println(string(out))
				continue
			}
			fmt.Println(renderLearning(e))
		}
		select {
		case <-stop:
			if !jsonOut {
				fmt.Println("\nStopped watching the feed.")
			}
			return 0
		case <-time.After(learningPoll):
		}
	}
}

// renderLearning is one committed change, in a line.
//
// The verb carries the distinction the store already makes, because a feed that said "learned"
// every time somebody walked a familiar route would train a person to stop reading it. The
// Director supplies the description with names already resolved; this chooses the words around it.
func renderLearning(e service.LearningEvent) string {
	noun := e.Kind
	switch e.Kind {
	case "place":
		noun = "place"
	case "edge":
		noun = "way"
	case "goal":
		noun = "destination"
	}
	switch e.Change {
	case "learned":
		return fmt.Sprintf("  + learned %-11s %s", noun, e.Description)
	case "strengthened":
		return fmt.Sprintf("  · saw again      %s", e.Description)
	case "named":
		return fmt.Sprintf("  + named          %s", e.Description)
	case "rebound":
		return fmt.Sprintf("  ~ moved %-13s %s", noun, e.Description)
	}
	return fmt.Sprintf("  ? %-14s %s", e.Change, e.Description)
}
