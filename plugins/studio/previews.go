package main

// THE PICTURES OF A DEMONSTRATION, FOR THE PERSON WHO RECORDED IT.
//
// # Why words were not enough, in the owner's own words
//
// They ran their first Act and reported that they could tell which screens were being asked for
// "but not easily" — reading `controls · display · profile · sound · video` off the screen and
// matching it against a game menu by eye. The project's requirement says the same thing with the
// reason attached:
//
//	Visual review needs retained target/start/result previews; OCR vocabulary is not an
//	adequate substitute.
//
// So a review shows the control it will press, the screen it starts from, and the screen it
// expects afterwards, as pictures. The words stay beside them: the words are what RECOGNISES a
// screen at run time and the picture is what a person recognises, and neither replaces the other.
//
// # How they get to the page
//
// As data URIs, one call per picture, fetched when a card is rendered rather than carried in
// every list. A thumbnail is a few tens of kilobytes; a list of nine demonstrations with three
// pictures each would be a megabyte of base64 on every poll, for pictures nobody has looked at.
//
// # And nothing here decides anything
//
// No run reads a picture, no recogniser consults one, and deleting every one of them leaves
// every Act performing exactly as before. They are kept only when somebody asked — Studio passes
// `--previews` and says so where it asks — and they are never sent anywhere.

import (
	"encoding/base64"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
)

// Pictured is which labels of a demonstration have a picture.
//
// Asked once per demonstration so the page knows what to render, rather than discovering it from
// a failed fetch per image. Empty for a demonstration recorded before previews existed, or one
// recorded without them, and the page shows its words alone — which is what it showed before.
type Pictured struct {
	// Screens and Controls are the artifact's own labels, so a caller joins them to the
	// interactions it is already showing.
	Screens  []string `json:"screens,omitempty"`
	Controls []string `json:"controls,omitempty"`
}

// Pictures is the labels of one demonstration that have a picture kept.
//
// Of THAT demonstration as it is saved now. The store is asked with the artifact rather than
// with the name, so a routine that has been demonstrated again shows the new recording's
// pictures or none — never the old recording's pictures beside the new one's words, which is a
// review misrepresenting what Play would perform. See internal/director/demo/previews.go.
func (a *Acts) Pictures(routine string) Pictured {
	store, err := demo.Open(a.home)
	if err != nil {
		return Pictured{}
	}
	art, ok := store.Sequence(routine)
	if !ok {
		return Pictured{}
	}
	var out Pictured
	for _, label := range store.Previews(art) {
		// SORTED INTO THE TWO KINDS BY THE LABEL ITSELF, which is the artifact's
		// own naming and the only thing that distinguishes them. A label that is
		// neither is one this build does not know about, and it is left out rather
		// than guessed at.
		switch {
		case strings.HasPrefix(label, "screen_"):
			out.Screens = append(out.Screens, label)
		case strings.HasPrefix(label, "control_"):
			out.Controls = append(out.Controls, label)
		}
	}
	return out
}

// Picture is one kept picture, as a data URI the page can put in an `img` tag.
//
// Empty when there is none, which the page treats as "show the words alone". Not an error: a
// demonstration recorded before previews existed has no pictures and that is not a fault, and
// neither is one recorded again without them — the previous recording's pictures are not this
// recording's, and the words shown beside them are.
func (a *Acts) Picture(routine, label string) string {
	store, err := demo.Open(a.home)
	if err != nil {
		return ""
	}
	art, ok := store.Sequence(routine)
	if !ok {
		return ""
	}
	raw, ok := store.Preview(art, label)
	if !ok {
		return ""
	}
	// AND THIS IS COUNTED. The page asks once per picture and keeps the element; the
	// blinking the owner reported was this call arriving 66 times in four seconds with
	// nothing on screen changing. A number nobody has to watch for; see trial.go.
	a.trial.pictured(routine, label)
	// A DATA URI RATHER THAN A FILE THE PAGE FETCHES. The window has no HTTP server
	// and should not grow one to show a picture; a binding that returns bytes is the
	// transport that already exists. See backend.go.
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}
