package observe

import (
	"strings"
	"testing"
)

// A rehearsal question says which route it is asking about.
//
// # The live failure
//
// Two open questions read identically:
//
//	"I've watched this move twice and both times went the same way… Shall I have a go?"
//
// They were different routes — two durable Homes, both going to Bluetooth — and the Audience had
// two Yes buttons and no way to tell which was which. A question nobody can distinguish is not a
// question.

func remembered(id, called string, terms []InterfaceTerm, n int) RememberedSubject {
	return RememberedSubject{
		ID: id, Called: called,
		Structure: StructureSignature{
			Subject: SubjectState, Terms: terms, TermsKnown: true,
			Roles: map[string]int{"button": n},
		},
	}
}

func askAbout(from, to RememberedSubject) string {
	return RehearsalQuestion(
		RehearsalJudgement{Source: from.ID, Destination: to.ID, Eligible: true},
		Topology{Subjects: map[string]RememberedSubject{from.ID: from, to.ID: to}})
}

// The question names where it starts and where it goes.
//
// Dropping either endpoint must fail this.
func TestARehearsalQuestionNamesBothEnds(t *testing.T) {
	q := askAbout(
		remembered("subj_home", "Home", []InterfaceTerm{TermSettings}, 18),
		remembered("subj_bt", "Bluetooth", []InterfaceTerm{TermSettings}, 10))

	if !strings.Contains(q, "Home") {
		t.Errorf("the question does not say where it starts: %q", q)
	}
	if !strings.Contains(q, "Bluetooth") {
		t.Errorf("the question does not say where it goes: %q", q)
	}
	if strings.Contains(q, "this move") {
		t.Errorf("the question fell back to %q: %q", "this move", q)
	}
}

// Two different routes produce two different questions.
//
// THE regression. Both Homes are unnamed, so this also proves the description floor works.
func TestTwoRoutesFromDifferentPlacesAskDifferentQuestions(t *testing.T) {
	bt := remembered("subj_bt", "", []InterfaceTerm{TermSettings}, 10)
	a := askAbout(remembered("subj_home_a", "", []InterfaceTerm{TermSettings}, 18), bt)
	b := askAbout(remembered("subj_home_b", "", []InterfaceTerm{TermSettings}, 17), bt)

	if a == b {
		t.Errorf("two different routes ask the identical question, so neither can be "+
			"answered:\n%q", a)
	}
}

// No subject id ever reaches the question.
func TestARehearsalQuestionShowsNoSubjectIds(t *testing.T) {
	q := askAbout(
		remembered("subj_home", "", []InterfaceTerm{TermSettings}, 18),
		remembered("subj_bt", "", []InterfaceTerm{TermSettings}, 10))
	if strings.Contains(q, "subj_") {
		t.Errorf("a subject id reached the question: %q", q)
	}
}

// The Audience's own word wins over Marco's description.
func TestAPlaceTheAudienceNamedIsCalledThat(t *testing.T) {
	s := remembered("subj_home", "Home", []InterfaceTerm{TermSettings}, 18)
	if got := PlaceWords(s); got != "Home" {
		t.Errorf("PlaceWords = %q, want the Audience's own word", got)
	}
}

// An unnamed place still says something a person can act on.
func TestAnUnnamedPlaceIsStillDescribed(t *testing.T) {
	s := remembered("subj_x", "", []InterfaceTerm{TermSettings, TermBack}, 18)
	got := PlaceWords(s)
	if got == "" || strings.Contains(got, "subj_") {
		t.Errorf("PlaceWords = %q", got)
	}
	// THE LABEL IS ONE WORD FOR EVERY UNNAMED PLACE, deliberately — see PlaceWords, and the
	// two dogfood sessions that ran into a subject id and then into a structural inventory.
	if got != Unnamed {
		t.Errorf("PlaceWords = %q, want %q", got, Unnamed)
	}
	// AND A QUESTION THAT HAS TO TELL TWO APART STILL CAN.
	if asking := PlaceWordsAsking(s); !strings.Contains(asking, "things on it") {
		t.Errorf("PlaceWordsAsking = %q; a question needs what the screen is made of", asking)
	}
}
