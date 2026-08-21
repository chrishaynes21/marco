package main

import (
	"image/color"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Rendering the Director's playbill.
//
// # This file chooses colour and layout, and nothing else
//
// Every sentence on screen came from `playbill.View.Watch()`. Nothing here decides what
// Marco believes, how sure it is, or what to call a screen. That is not a convention: the
// overlay module cannot reach the Director's analysis at all, and the only thing it
// imports is the account type.
//
// The value of that shows in the TONE. The old Director panel coloured rows by matching
// on their text — a row containing "failed:" turned red — which works until somebody
// rewords a sentence, and then stops working silently. Here the tone travels WITH the
// line, decided by the layer that knows what it means.
//
// # The visual grammar
//
//	a status chip          one word, the consumer reading, always at the top
//	a labelled hairline    section headings, so the eye can skip a section it doesn't want
//	a left rail            evidence sitting under a claim, dimmer and inset
//	one accent             reserved for things WAITING ON A PERSON, and nothing else
//
// The last one is the rule that matters. If everything interesting is highlighted then a
// pending question looks like a provider count, and the one thing a person has to notice
// is the one thing they will not.

// watchRow is one laid-out row: the text, how it should read, and how far in it sits.
type watchRow struct {
	text   string
	tone   playbill.Tone
	indent int
	head   bool
}

const (
	// watchLineH is the row pitch. A touch looser than the config/help panels: this is
	// prose to be read rather than a table to be scanned.
	watchLineH = 15.0
	// watchRail is where the evidence rail sits, and watchInset where inset text starts.
	watchRail  = 4.0
	watchInset = 13.0
)

// watchRows flattens the account into wrapped rows.
//
// The wrapping is done once and shared by Draw and layoutHeight, so the window grows to
// exactly what is drawn. Two implementations of this drifting apart is how a panel comes
// to have a row hidden below its own bottom edge.
func watchRows(s snapshot, innerW float64) []watchRow {
	// layoutHeight runs in Update, which can happen before the first Draw — the same
	// guard the other row counters use.
	fontOnce.Do(initFonts)
	lines := s.watch.Watch()
	if s.wmode == watchDeep {
		lines = s.watch.Deep()
	}
	var out []watchRow
	for _, l := range lines {
		if l.Text == "" && !l.Head {
			out = append(out, watchRow{})
			continue
		}
		avail := innerW
		if l.Indent > 0 {
			avail -= watchInset
		}
		if l.Head {
			// A heading is never wrapped: it is two or three words by construction, and
			// a wrapped one would break the hairline that runs off its end.
			out = append(out, watchRow{text: strings.ToUpper(l.Text), tone: l.Tone, head: true})
			continue
		}
		for _, w := range wrapText(l.Text, faceSmall, avail) {
			out = append(out, watchRow{text: w, tone: l.Tone, indent: l.Indent})
		}
	}
	return out
}

func watchRowCount(s snapshot, innerW float64) int {
	// One for the status chip, plus the rows themselves. The chip is always drawn: a
	// panel that showed nothing while the Director was unreachable would read as a panel
	// that had not loaded.
	return 1 + len(watchRows(s, innerW))
}

// drawWatch renders the panel and returns the y it finished at.
func drawWatch(dst *ebiten.Image, pad, y, innerW float64, s snapshot) float64 {
	y = drawWatchChip(dst, pad, y, innerW, s)

	for _, r := range watchRows(s, innerW) {
		switch {
		case r.text == "":
			y += watchLineH * 0.45
		case r.head:
			y = drawWatchHead(dst, pad, y, innerW, r.text)
		default:
			x := pad
			col := toneColor(r.tone)
			if r.indent > 0 {
				x = pad + watchInset
				// The rail: a hairline under the claim this evidence belongs to. It is
				// what lets somebody skim the claims and ignore the reasons, which is
				// most of what reading this panel is.
				vector.StrokeLine(dst,
					float32(pad+watchRail), float32(y+1),
					float32(pad+watchRail), float32(y+watchLineH-1),
					1, aBg(th.sep), false)
				col = lerp(th.bg, col, 0.82)
			}
			drawText(dst, r.text, x, y, faceSmall, col)
			y += watchLineH
		}
	}
	return y
}

// drawWatchChip is the status chip: the NORMAL reading, always at the top.
//
// The same one word a consumer surface would show, on the developer surface, from the
// same value. That is the architectural claim of this milestone made visible: if the chip
// and the rows below it ever disagree, the bug is in one shared function rather than in
// two implementations that have to be compared by hand.
func drawWatchChip(dst *ebiten.Image, pad, y, innerW float64, s snapshot) float64 {
	h := s.headline
	col := toneColor(h.Tone)

	label := strings.ToUpper(h.Word)
	if label == "" {
		label = "READY"
	}
	tw, _ := text.Measure(label, faceSmall, 0)

	// A filled chip only when something is WAITING ON A PERSON. Everything else gets a
	// plain dot, because an overlay in which every state is highlighted has no highlight.
	if h.Attention {
		var p vector.Path
		roundRectPath(&p, float32(pad), float32(y-2), float32(tw+16), 15, 7)
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(aBg(lerp(th.bg, col, 0.30)))
		vector.FillPath(dst, &p, &vector.FillOptions{}, op)
		drawText(dst, label, pad+8, y, faceSmall, col)
	} else {
		stateDot(dst, pad+4, y+6, col)
		drawText(dst, label, pad+watchInset, y, faceSmall, col)
	}

	// The mode, right-justified and deliberately quiet.
	mode := watchModeLabel(s)
	mw, _ := text.Measure(mode, faceSmall, 0)
	drawText(dst, mode, pad+innerW-mw, y, faceSmall, lerp(th.bg, th.idle, 0.42))
	return y + watchLineH + 4
}

// watchModeLabel names which reading is on screen.
//
// Capturing the mouse is never a silent state. If the overlay is eating clicks meant for
// the game, the reason and the way out have to be on screen — so the label for the mode
// that captures says both, and it is one function rather than a string in the drawing
// code so a test can check it is still there.
func watchModeLabel(s snapshot) string {
	if s.wmode == watchDeep {
		return "diagnostics · mouse captured · Esc"
	}
	// HERE, not "watch". The control centre calls this same belief HERE, and one belief
	// with two names is a thing a person has to learn twice — so the panel's own label,
	// the word that opens it and the section in the browser are now one word. The old
	// spelling still answers as an undocumented alias (commands.go), exactly as `teach`
	// still answers for `learn`.
	return cmdHere
}

// normalRow is the NORMAL reading as one line for the always-visible hint.
//
// The minimal consumer rendering, and the proof that one representation drives all three:
// the same `View` that produces the Watch panel's rows produces this, through the same
// `Normal()` reduction the CLI's `marco director normal` uses.
//
// Empty when there is nothing to say. A resting Marco says nothing, which is the correct
// consumer default and the only way the line keeps its meaning for the moment Marco is
// actually waiting on somebody.
func normalRow(s snapshot) string {
	h := s.headline
	switch {
	case h.Word == "" || h.Word == "Ready":
		return ""
	case h.Detail == "":
		return h.Word
	}
	return h.Word + " — " + h.Detail
}

// drawWatchHead is a section heading: a quiet label with a hairline running off its end.
func drawWatchHead(dst *ebiten.Image, pad, y, innerW float64, label string) float64 {
	y += 4
	w := drawText(dst, label, pad, y, faceSmall, lerp(th.bg, th.idle, 0.52))
	if x := pad + w + 8; x < pad+innerW {
		vector.StrokeLine(dst, float32(x), float32(y+5), float32(pad+innerW), float32(y+5),
			1, aBg(th.sep), false)
	}
	return y + watchLineH
}

// toneColor maps a tone onto the active theme.
//
// The mapping is the whole of the overlay's editorial contribution, and it is
// deliberately conservative: `plain` is body text, `muted` is quieter than body text, and
// only `accent` and `alarm` are loud. A theme change moves all of it at once, which is
// why this reads the palette rather than naming colours.
func toneColor(t playbill.Tone) color.RGBA {
	switch t {
	case playbill.Good:
		return th.run
	case playbill.Doubt:
		return th.listen
	case playbill.Alarm:
		return th.errc
	case playbill.Accent:
		return th.name
	case playbill.Muted:
		return lerp(th.bg, th.desc, 0.85)
	}
	return th.cmd
}
