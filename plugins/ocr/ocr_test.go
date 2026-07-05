package main

import "testing"

// sampleTSV is a trimmed tesseract TSV: a header, two non-word rows (level < 5,
// conf -1), and three word rows.
const sampleTSV = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
	"1\t1\t0\t0\t0\t0\t0\t0\t1920\t1080\t-1\t\n" +
	"4\t1\t1\t1\t1\t0\t100\t200\t300\t40\t-1\t\n" +
	"5\t1\t1\t1\t1\t1\t100\t200\t80\t40\t96\tMute\n" +
	"5\t1\t1\t1\t1\t2\t190\t200\t120\t40\t95\tMicrophone\n" +
	"5\t1\t1\t1\t2\t1\t100\t260\t140\t40\t90\tDeafen\n"

func TestParseTSV(t *testing.T) {
	words, err := parseTSV(sampleTSV)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 3 {
		t.Fatalf("want 3 words, got %d: %+v", len(words), words)
	}
	if words[0] != (Word{Text: "Mute", X: 100, Y: 200, W: 80, H: 40, Conf: 96}) {
		t.Fatalf("first word = %+v", words[0])
	}
}

func TestParseTSVSkipsHeaderAndNonWords(t *testing.T) {
	words, _ := parseTSV(sampleTSV)
	for _, w := range words {
		if w.Text == "" {
			t.Fatalf("blank word leaked through: %+v", w)
		}
	}
}

func TestMatchWordExact(t *testing.T) {
	words, _ := parseTSV(sampleTSV)
	w, ok := matchWord(words, "mute") // case-insensitive
	if !ok || w.Text != "Mute" {
		t.Fatalf("exact match = %+v ok=%v", w, ok)
	}
}

func TestMatchWordSubstring(t *testing.T) {
	words, _ := parseTSV(sampleTSV)
	w, ok := matchWord(words, "phone") // inside "Microphone"
	if !ok || w.Text != "Microphone" {
		t.Fatalf("substring match = %+v ok=%v", w, ok)
	}
}

func TestMatchWordExactBeatsSubstring(t *testing.T) {
	words := []Word{
		{Text: "Unmute", X: 0, Y: 0, W: 60, H: 20},
		{Text: "Mute", X: 100, Y: 0, W: 40, H: 20},
	}
	w, ok := matchWord(words, "mute")
	if !ok || w.Text != "Mute" {
		t.Fatalf("expected exact 'Mute' to win over substring 'Unmute', got %+v", w)
	}
}

func TestMatchWordPhrase(t *testing.T) {
	words := []Word{
		{Text: "Start", X: 100, Y: 500, W: 80, H: 30},
		{Text: "Game", X: 190, Y: 500, W: 70, H: 30},
		{Text: "Quit", X: 100, Y: 560, W: 60, H: 30},
	}
	w, ok := matchWord(words, "start game")
	if !ok {
		t.Fatal("phrase 'start game' not matched")
	}
	// Merged box spans both words: x 100..260, so centre x = 180.
	if cx := w.X + w.W/2; cx != 180 {
		t.Fatalf("merged centre x = %d, want 180 (box %+v)", cx, w)
	}
}

func TestMatchWordPhraseNotOnSameLine(t *testing.T) {
	words := []Word{
		{Text: "Start", X: 100, Y: 500, W: 80, H: 30},
		{Text: "Game", X: 190, Y: 900, W: 70, H: 30}, // far below — different line
	}
	if _, ok := matchWord(words, "start game"); ok {
		t.Fatal("words on different lines should not match as a phrase")
	}
}

func TestMatchWordMiss(t *testing.T) {
	words, _ := parseTSV(sampleTSV)
	if _, ok := matchWord(words, "settings"); ok {
		t.Fatal("unexpectedly matched absent text")
	}
}
