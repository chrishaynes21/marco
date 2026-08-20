package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The Director's account, served to a browser.
//
// # Why this decodes the Director's own type
//
// The same reason the overlay does, and it is the whole architectural point: there is ONE
// representation of what Marco believes, and every surface renders it rather than describing it.
// A hand-mirrored struct here would drift the first time a field's meaning changed, and it would
// drift silently, in a surface whose entire job is to tell somebody the truth.
//
// It costs this module nothing it was avoiding — `pkg/playbill` is standard library only, as the
// whole engine is.
//
// # The words come from the shared package, the layout comes from the browser
//
// `Normal()`, `Watch()` and `Deep()` produce the sentences. This file groups the resulting lines
// into sections so the page can draw cards instead of a wall of text, and that grouping is
// PRESENTATION: it reorders nothing, rewords nothing, and decides no facts. If a section heading
// moves, the cards follow it.
//
// # It is a read, and it can only be a read
//
// Every handler here shells out to `marco director watch|diagnose --json`, which reads state the
// service already holds. Nothing starts an observation, takes a sample, answers a question or
// grants anything. A panel refreshing twice a second cannot perturb the session it describes,
// which is the property that makes a live view possible at all.

// reading is which of the three renderings the page asked for.
type reading string

const (
	readNormal reading = "normal"
	readWatch  reading = "watch"
	readDeep   reading = "debug"
)

// section is one card: a heading and the lines under it.
type section struct {
	Title string    `json:"title"`
	Lines []webLine `json:"lines"`
}

// webLine is one row, with the tone the shared package attached to it.
//
// Tone travels WITH the text so the page colours by meaning. The alternative — matching on the
// words — is what the first Director panel did, and it stops working silently the moment a
// sentence is reworded.
type webLine struct {
	Text   string `json:"text"`
	Tone   string `json:"tone"`
	Indent int    `json:"indent"`
}

// accountView is what the page renders.
type accountView struct {
	// Reach says whether there is anything to show at all, and Why says what to do when
	// there is not. Three values, not a bool: "the engine could not be run" and "Marco is
	// not started" send a person to two different places.
	Reach string `json:"reach"`
	Why   string `json:"why,omitempty"`

	// Headline is the consumer reading — one word and one sentence — shown at every depth,
	// because a person switching to Debug still wants to know Marco has a question.
	Headline struct {
		Word      string `json:"word"`
		Detail    string `json:"detail,omitempty"`
		Tone      string `json:"tone"`
		Attention bool   `json:"attention"`
	} `json:"headline"`

	// Sections are the cards, in the order the shared package emitted them.
	Sections []section `json:"sections,omitempty"`

	// LearnSession is lifted out of the sections so the page can give it a first-class panel:
	// the cue, the checklist and what Marco believes you just did. It is the SAME value the
	// LEARN SESSION section renders — not a second copy — and the page shows one or the other.
	LearnSession *playbill.LearnSession `json:"learnSession,omitempty"`

	// Question is the open question, carried whole so the page can offer the ordinary
	// answers. Answering still goes through the existing response path; this is an address,
	// not an authority.
	Question *playbill.Question `json:"question,omitempty"`

	// Digest is a fingerprint of everything a person would notice a change in. The page
	// holds still while it is unchanged, so an idle Director does not repaint twice a second.
	Digest string `json:"digest,omitempty"`
	// Epoch identifies the Director instance. A change means it restarted.
	Epoch string `json:"epoch,omitempty"`
}

// fetchPlaybill asks the engine for the Director's account.
//
// `diagnose` rather than `watch` for the deep reading, because the diagnostics half is computed
// only when it is asked for — a surface that requested it on every poll would make the act of
// watching expensive.
func fetchPlaybill(r reading) (playbill.View, error) {
	verb := "watch"
	if r == readDeep {
		verb = "diagnose"
	}
	out, err := exec.Command(marcoBin(), "director", verb, "--json").Output()
	if err != nil {
		// The engine could not be run, or would not answer. Distinct from a Director that
		// is not started, and the page says so rather than showing an empty panel.
		return playbill.Unavailable(playbill.Unreachable,
			"I couldn't run the Marco engine to ask what the Director is doing."), nil
	}
	var v playbill.View
	if err := json.Unmarshal(out, &v); err != nil {
		return playbill.Unavailable(playbill.Unreachable,
			"the Marco engine answered with something I couldn't read."), nil
	}
	return v.Normalise(), nil
}

// accountFor renders one reading into cards.
func accountFor(r reading) accountView {
	v, _ := fetchPlaybill(r)

	out := accountView{Reach: string(v.Reach), Why: v.Why, Digest: v.Digest, Epoch: v.Epoch}
	h := v.Normal()
	out.Headline.Word, out.Headline.Detail = h.Word, h.Detail
	out.Headline.Tone, out.Headline.Attention = string(h.Tone), h.Attention

	if !v.Reach.Live() {
		return out
	}
	out.Question = v.Question
	if t := v.LearnSession; t.Active || t.Learned != "" || t.Stopped {
		copyOf := t
		out.LearnSession = &copyOf
	}

	// Normal is deliberately the headline and nothing else. Somebody who wanted the
	// evidence asked for Watch.
	if r == readNormal {
		return out
	}

	lines := v.Watch()
	if r == readDeep {
		lines = v.Deep()
	}
	out.Sections = sectionsOf(lines)
	return out
}

// sectionsOf groups a flat line list into cards by its own headings.
//
// PRESENTATION ONLY. The shared package decides which sections exist, what they are called and
// what order they come in; this walks the result and starts a new card whenever it emits a
// heading. Lines before the first heading — what is on screen right now — go into an unnamed
// leading card, which is where a reader looks first.
func sectionsOf(lines []playbill.Line) []section {
	var out []section
	cur := section{}
	for _, l := range lines {
		if l.Head {
			if len(cur.Lines) > 0 {
				out = append(out, cur)
			}
			cur = section{Title: l.Text}
			continue
		}
		if strings.TrimSpace(l.Text) == "" {
			continue
		}
		cur.Lines = append(cur.Lines, webLine{
			Text: l.Text, Tone: string(l.Tone), Indent: l.Indent,
		})
	}
	if len(cur.Lines) > 0 {
		out = append(out, cur)
	}
	return out
}

// handlePlaybill serves one reading.
func handlePlaybill(w http.ResponseWriter, r *http.Request) {
	view := reading(strings.ToLower(r.URL.Query().Get("view")))
	switch view {
	case readNormal, readWatch, readDeep:
	default:
		view = readWatch
	}
	writeJSON(w, accountFor(view))
}
