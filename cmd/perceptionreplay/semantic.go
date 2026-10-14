package main

// Does what a screen SAID separate two screens the arrangement merged?
//
// # The question, narrowly
//
// [[Experiment-041-the-control-Marco-found-for-itself]] navigated Rocket League to Free Play with
// three real clicks and could not establish that it had arrived. The diagnosis was not the
// destination threshold: `screen_d` agreed with the live destination at 0.750 and ranked first.
// The observer had already merged the Training page and the Free Play submenu into ONE live
// identity, so the destination matcher was never offered a distinct screen to recognise.
//
// So: fold the SAME recording twice, through the same production observer, and change one thing.
//
//	pixel-only    what production does today
//	with text     the same looks, each carrying what OCR read on it
//
// Two numbers decide it. How many screens each fold settles, and whether the looks that one fold
// calls one screen the other calls two. Everything else in the report is there to show the
// change is not simply fragmentation: a run that doubled every screen would separate the two
// menus too, and would have broken every route in the process.
//
// # Why this is offline and stays offline
//
// OCR costs hundreds of milliseconds a frame against a 100ms observation cadence. The recording
// is 900 frames of real pixels on one clock, so the question can be answered exactly without
// asking anybody to play the game again — and the answer is about the production fold, because
// it IS the production fold. See `foldRange`.

import (
	"context"
	"encoding/json"
	"fmt"

	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/platform/ocrclient"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// readWords reads every frame once and returns what each one said.
//
// Cached beside the recording, because OCR over 900 frames is minutes and the fold is seconds:
// a question worth asking twice should not cost the reading twice. The cache is keyed by nothing
// but the directory, so deleting the file is how you re-read it.
func readWords(dir string, frames []frameRecord, stride int, eng *ocrclient.Engine) (
	[]observe.ScreenText, error) {

	out := make([]observe.ScreenText, len(frames))
	cache := filepath.Join(dir, "words.jsonl")
	if rows, err := readJSONL[wordRow](cache); err == nil && len(rows) > 0 {
		for _, r := range rows {
			if r.Look >= 0 && r.Look < len(out) {
				out[r.Look] = observe.TextOf(directorapi.SourceOCR, r.Phrases)
			}
		}
		fmt.Printf("  words: %d reading(s) from %s\n", len(rows), cache)
		return out, nil
	}
	if eng == nil {
		return nil, fmt.Errorf("no OCR engine and no %s to read from", cache)
	}

	f, err := os.Create(cache)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ctx := context.Background()
	start, done := time.Now(), 0
	for i, fr := range frames {
		if stride > 1 && i%stride != 0 {
			continue
		}
		img, rerr := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if rerr != nil {
			continue
		}
		got, oerr := eng.Recognize(ctx, ocr.ImageInput{Image: img})
		if oerr != nil {
			return nil, fmt.Errorf("look %d: %w", i, oerr)
		}
		var phrases []string
		for _, g := range got {
			if g.Text == "" || g.Bounds.Empty() {
				continue
			}
			phrases = append(phrases, g.Text)
		}
		out[i] = observe.TextOf(directorapi.SourceOCR, phrases)
		if err := writeJSONLRow(f, wordRow{Look: i, Phrases: phrases}); err != nil {
			return nil, err
		}
		done++
		if done%25 == 0 {
			fmt.Printf("  read %d look(s) in %s\n", done, time.Since(start).Round(time.Second))
		}
	}
	fmt.Printf("  words: %d reading(s) in %s, cached in %s\n",
		done, time.Since(start).Round(time.Second), cache)
	return out, nil
}

// wordRow is one look's reading, cached.
type wordRow struct {
	Look    int      `json:"look"`
	Phrases []string `json:"phrases,omitempty"`
}

// semanticFold runs ONE fresh observer over the whole recording, with or without text.
//
// Fresh is the point, as in `foldRange`: nothing is carried between the two folds, so `visual_1`
// in one and `visual_1` in the other are unrelated names that happen to collide.
// `delay` is how many looks after its own frame each reading is delivered, and it changes
// WHICH PATH is being measured. Zero hands the reading to the look it describes, through
// `Look.Text` -- which is look-local and which no live source can satisfy. Anything else
// hands it over later, through `Look.Readings`, which is the live path: the reading carries
// the window generation and observation epoch of the frame it came from, and the observer
// decides whether that context is still the one it is in.
func semanticFold(frames []frameRecord, regions [][]observe.Region, readable []bool,
	words []observe.ScreenText, withText bool, delay int) (fold, observe.VisualStability,
	fastobserve.Stats, map[observe.VisualScreenID]observe.ScreenText, error) {

	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		return fold{}, observe.VisualStability{}, fastobserve.Stats{}, nil, err
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	defer obs.Close()

	f := fold{at: map[int]observe.VisualScreenID{},
		arr: map[observe.VisualScreenID]observe.VisualArrangement{}}
	base := time.Unix(1700000000, 0).UTC()
	// THE EPOCH EACH LOOK WAS FOLDED UNDER, recorded as the fold goes so a reading of
	// look N can be stamped with the context look N was actually taken in. A live
	// source reads the epoch off the observer at the moment it captures; this is the
	// same thing, replayed.
	epochAt := make([]uint64, len(frames))
	for i := range frames {
		fr := frames[i]
		lk := fastobserve.Look{
			At: base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Window: fastobserve.WindowRef{
				ID: "hwnd:recorded", Generation: fr.WindowGeneration,
				Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
			},
			Front:   true,
			Regions: regions[i],
		}
		if !readable[i] {
			lk.Missing = true
		}
		switch {
		case !withText:
		case delay <= 0:
			// THE SYNCHRONOUS PATH, which exists to be compared against.
			if i < len(words) {
				lk.Text = words[i]
			}
		default:
			// THE LIVE PATH. A reading of look i-delay arrives now, stamped
			// with the context THAT look was taken in -- the window
			// generation of its own frame and the observation epoch that
			// was standing when it was folded. The observer judges it.
			src := i - delay
			if src >= 0 && src < len(words) && words[src].Read() {
				lk.Readings = []observe.ScreenReading{{
					Text: words[src], FromLook: src,
					Generation: frames[src].WindowGeneration,
					Epoch:      epochAt[src],
				}}
			}
		}
		epochAt[i] = obs.Epoch()
		obs.Observe(lk)
		f.at[i] = obs.Screen()
	}
	said := map[observe.VisualScreenID]observe.ScreenText{}
	for _, s := range obs.Screens() {
		f.arr[s.ID] = obs.Arrangement(s.ID)
		said[s.ID] = obs.SaidBy(s.ID)
	}
	return f, obs.Stability(), obs.Stats(), said, nil
}

// reportSemanticScreens is the experiment.
func reportSemanticScreens(dir string, limit, stride int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	fmt.Printf("%s: %d frame(s)\n", dir, len(frames))

	eng, _, why := defaultOCR()
	if why != "" {
		fmt.Printf("  OCR: %s\n", why)
	}
	words, werr := readWords(dir, frames, stride, eng)
	if werr != nil {
		return werr
	}
	nread := 0
	for _, w := range words {
		if w.Read() {
			nread++
		}
	}

	regions, readable := proposeAll(dir, frames)

	plain, pstats, _, _, perr := semanticFold(frames, regions, readable, nil, false, 0)
	if perr != nil {
		return perr
	}
	sync, sstats, _, _, serr := semanticFold(frames, regions, readable, words, true, 0)
	if serr != nil {
		return serr
	}

	// ── AND THE LIVE PATH, AT EVERY LATENCY WORTH ASKING ABOUT ──
	//
	// A look is 100ms and OCR is 300-600ms on a 1920x1080 frame, so the honest range
	// is three to six looks behind; the rest are there to show where it stops working
	// and why. The SAME readings in every row -- only when they arrive changes.
	delays := []int{1, 3, 5, 8, 12, 20}
	type row struct {
		delay  int
		f      fold
		st     observe.VisualStability
		stats  fastobserve.Stats
		said   map[observe.VisualScreenID]observe.ScreenText
		splits int
	}
	var live []row
	for _, d := range delays {
		f, st, stats, said, err := semanticFold(frames, regions, readable,
			words, true, d)
		if err != nil {
			return err
		}
		live = append(live, row{delay: d, f: f, st: st, stats: stats, said: said})
	}

	fmt.Println()
	fmt.Println("  the same 900 looks and the same 300 readings; only WHEN they arrive")
	fmt.Printf("  %-26s %7s %7s %7s %7s %7s %7s\n",
		"", "screens", "settled", "offered", "admitted", "stale", "refused")
	fmt.Printf("  %-26s %7d %7d %7s %7s %7s %7s\n", "pixel-only",
		countScreens(plain), pstats.Settled, "-", "-", "-", "-")
	fmt.Printf("  %-26s %7d %7d %7d %7d %7d %7d\n",
		"with text, same look", countScreens(sync), sstats.Settled,
		len(words), nread, 0, sstats.TextRefusedMerge)
	for _, r := range live {
		stale := r.stats.ReadingsStaleEpoch + r.stats.ReadingsStaleWindow
		fmt.Printf("  %-26s %7d %7d %7d %7d %7d %7d\n",
			fmt.Sprintf("late by %d look(s)", r.delay),
			countScreens(r.f), r.st.Settled, r.stats.ReadingsOffered,
			r.stats.ReadingsAdmitted, stale, r.stats.ReadingsRefusedMerge)
	}

	// ── AND THE ONE QUESTION THE WHOLE THING IS FOR ──
	//
	// Not "how many screens" but "are THOSE two screens apart". A fold can score well
	// on every count above and have merged the one pair this exists to separate.
	fmt.Println()
	fmt.Println("  are the Training page and the Free Play submenu two screens")
	fmt.Printf("  %-26s %s\n", "pixel-only", verdictOf(plain, nil))
	fmt.Printf("  %-26s %s\n", "with text, same look",
		verdictOf(sync, syncSaid(frames, regions, readable, words)))
	for _, r := range live {
		fmt.Printf("  %-26s %s\n",
			fmt.Sprintf("late by %d look(s)", r.delay), verdictOf(r.f, r.said))
	}

	// The richest fold is the one the rest of the report describes.
	rich, rstats, _, said := sync, sstats, fastobserve.Stats{}, syncSaid(frames, regions,
		readable, words)
	for _, r := range live {
		if r.delay == 5 {
			rich, rstats, said = r.f, r.st, r.said
		}
	}
	_ = rstats

	// ── WHICH SCREENS SPLIT, AND INTO WHAT ──
	//
	// For each pixel-only screen, how the richer fold distributed its looks. One row per
	// screen, so a fold that split one menu in two is visible beside a fold that split
	// everything -- which would be fragmentation wearing this change's clothes.
	fmt.Println()
	fmt.Println("  how each pixel-only screen's looks were distributed by the richer fold")
	fmt.Printf("  %-12s %6s  %s\n", "pixel-only", "looks", "with text")
	for _, id := range screenOrder(plain) {
		by := map[observe.VisualScreenID]int{}
		n := 0
		for look, sc := range plain.at {
			if sc != id {
				continue
			}
			n++
			by[rich.at[look]]++
		}
		parts := make([]string, 0, len(by))
		for _, k := range sortedIDs(by) {
			parts = append(parts, fmt.Sprintf("%s x%d", k, by[k]))
		}
		mark := ""
		if len(by) > 1 {
			mark = "   <- SPLIT"
		}
		fmt.Printf("  %-12s %6d  %s%s\n", id, n, joinWith(parts, ", "), mark)
	}

	// ── WHAT EACH SCREEN OF THE RICHER FOLD SAYS ──
	fmt.Println()
	fmt.Println("  what each screen of the richer fold was read to say (stable words only)")
	for _, id := range screenOrder(rich) {
		t := said[id]
		if !t.Read() {
			fmt.Printf("  %-12s (nobody read it)\n", id)
			continue
		}
		fmt.Printf("  %-12s %d reading(s): %s\n", id, t.Looks,
			joinWith(t.List(), " "))
	}

	// ── DOES A SCREEN LEFT AND RETURNED TO STILL REJOIN ──
	//
	// The criterion that decides whether any of this is usable. Reuse is what makes a
	// route possible: walking back out of a submenu has to arrive at the screen it left,
	// or every demonstration is a chain of new places and no repetition is visible. A
	// fold that separated two menus by giving every visit its own identity would show the
	// separation above and have destroyed the thing it is for.
	fmt.Println()
	fmt.Println("  visits per screen -- a screen entered, left and returned to must rejoin")
	fmt.Printf("  %-12s %6s %7s   %-12s %6s %7s\n",
		"pixel-only", "looks", "visits", "with text", "looks", "visits")
	pv, rv := visits(plain), visits(rich)
	po, ro := screenOrder(plain), screenOrder(rich)
	for i := 0; i < len(po) || i < len(ro); i++ {
		left, right := "", ""
		if i < len(po) {
			left = fmt.Sprintf("%-12s %6d %7d", po[i],
				looksOf(plain, po[i]), pv[po[i]])
		} else {
			left = fmt.Sprintf("%-12s %6s %7s", "", "", "")
		}
		if i < len(ro) {
			right = fmt.Sprintf("%-12s %6d %7d", ro[i],
				looksOf(rich, ro[i]), rv[ro[i]])
		}
		fmt.Printf("  %s   %s\n", left, right)
	}
	previsit, rrevisit := revisited(pv), revisited(rv)
	fmt.Printf("\n  %-44s %4d of %d\n", "pixel-only screens entered more than once",
		previsit, len(po))
	fmt.Printf("  %-44s %4d of %d\n", "with-text screens entered more than once",
		rrevisit, len(ro))

	// ── AND THE TWO DISTRIBUTIONS THE THRESHOLDS SIT BETWEEN ──
	reportTextMargins(rich, said)
	return nil
}

// visits counts how many separate runs of looks each screen holds.
//
// A run is a stretch of consecutive looks standing on one screen. Two runs of one id means the
// screen was left and returned to, and the identity was reused -- which is the property a route
// is read from.
func visits(f fold) map[observe.VisualScreenID]int {
	out := map[observe.VisualScreenID]int{}
	last := observe.VisualScreenID("")
	for look := 0; look < len(f.at); look++ {
		id, ok := f.at[look]
		if !ok {
			continue
		}
		switch id {
		case "", observe.VisualScreenUnknown, observe.VisualScreenUnobserved:
			// NOT A BOUNDARY. A blank look or a warming one does not end a
			// visit: the standing screen survives it by design, and counting
			// it as a return would report a visit per failed capture.
			continue
		}
		if id != last {
			out[id]++
		}
		last = id
	}
	return out
}

func looksOf(f fold, id observe.VisualScreenID) int {
	n := 0
	for _, sc := range f.at {
		if sc == id {
			n++
		}
	}
	return n
}

func revisited(v map[observe.VisualScreenID]int) int {
	n := 0
	for _, c := range v {
		if c > 1 {
			n++
		}
	}
	return n
}

// reportTextMargins prints how alike the screens of one fold are, by words and by arrangement.
//
// The thing a later reader needs in order to tell whether TextKept and TextNovel are still the
// right numbers: not whether they worked once, but whether there is still a gap for them to sit
// in.
func reportTextMargins(f fold, said map[observe.VisualScreenID]observe.ScreenText) {
	ids := screenOrder(f)
	fmt.Println()
	fmt.Println("  every pair of the richer fold, by both measures")
	fmt.Printf("  %-12s %-12s %11s %7s %6s %6s\n",
		"", "", "arrangement", "words", "kept", "novel")
	type row struct {
		a, b                   observe.VisualScreenID
		arr, words, kept, novl float64
	}
	var rows []row
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			x, y := ids[i], ids[j]
			k, n := observe.TextOverlap(said[x], said[y])
			rows = append(rows, row{
				a: x, b: y,
				arr:   observe.ArrangementSimilarity(f.arr[x], f.arr[y]),
				words: observe.TextSimilarity(said[x], said[y]),
				kept:  k, novl: n,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].arr > rows[j].arr })
	for _, r := range rows {
		mark := ""
		if r.arr >= observe.VisualScreenMatch {
			mark = "  arrangement would merge these"
			if apart, _ := observe.TextsDisagree(said[r.a], said[r.b]); apart {
				mark = "  arrangement would merge these; words refuse"
			}
		}
		fmt.Printf("  %-12s %-12s %11.3f %7.3f %6.2f %6.2f%s\n",
			r.a, r.b, r.arr, r.words, r.kept, r.novl, mark)
	}
}

func countScreens(f fold) int { return len(screenOrder(f)) }

// screenOrder is the real screens of a fold, in label order.
func screenOrder(f fold) []observe.VisualScreenID {
	seen := map[observe.VisualScreenID]bool{}
	for _, id := range f.at {
		switch id {
		case "", observe.VisualScreenUnknown, observe.VisualScreenUnobserved:
			continue
		}
		seen[id] = true
	}
	return sortedIDs(seen)
}

// sortedIDs sorts screen ids by their minted number, so visual_10 follows visual_9.
func sortedIDs[V any](m map[observe.VisualScreenID]V) []observe.VisualScreenID {
	out := make([]observe.VisualScreenID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := screenNumber(out[i]), screenNumber(out[j])
		if a != b {
			return a < b
		}
		return out[i] < out[j]
	})
	return out
}

func screenNumber(id observe.VisualScreenID) int {
	n, s := 0, string(id)
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] < '0' || s[i] > '9' {
			break
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int(s[i]-'0')
		}
	}
	return n
}

func joinWith(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// writeJSONLRow appends one row to an open JSONL file.
func writeJSONLRow(f *os.File, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// defaultOCR builds the OCR engine the same way Director does, or says why it cannot.
func defaultOCR() (*ocrclient.Engine, string, string) {
	bridge := os.Getenv("DIRECTOR_OCR")
	if bridge == "" {
		for _, c := range []string{
			filepath.Join("plugins", "ocr", "ocr.exe"),
			filepath.Join("plugins", "ocr", "ocr"),
		} {
			if _, err := os.Stat(c); err == nil {
				bridge = c
				break
			}
		}
	}
	if bridge == "" {
		return nil, "", "no OCR plugin found — build plugins/ocr, or set $DIRECTOR_OCR"
	}
	if _, err := os.Stat(bridge); err != nil {
		return nil, "", "the OCR plugin is not at " + bridge
	}
	h := bridgehost.New(bridge)
	if t := tesseractPath(); t != "" {
		h = h.WithEnv("MARCO_TESSERACT=" + t)
	}
	return ocrclient.New(h), bridge, ""
}

// tesseractPath finds the engine the OCR plugin shells out to.
//
// The installer puts it under Program Files without adding it to PATH, so naming the usual
// locations is what makes this run on a machine where nobody has edited their environment.
func tesseractPath() string {
	if p := os.Getenv("MARCO_TESSERACT"); p != "" {
		return p
	}
	for _, c := range []string{
		`C:\Program Files\Tesseract-OCR\tesseract.exe`,
		`C:\Program Files (x86)\Tesseract-OCR\tesseract.exe`,
	} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// ── the one question the whole thing is for ───────────────────────────────────

// trainingWords and freePlayWords are the labels that identify the two screens of the measured
// failure, as OCR actually reads them.
//
// Used ONLY to recognise which of a fold's screens is which, so the report can say whether those
// two are apart. Never used to produce a fold: every identity in every fold above is settled from
// pixels and readings with no knowledge of these at all. This is the oracle, beside the verdict.
var (
	trainingWords = []string{"aerial", "goalie", "striker"}
	freePlayWords = []string{"free", "join", "online"}
)

// holdsAll reports whether a vocabulary contains every one of these words.
func holdsAll(t observe.ScreenText, words []string) bool {
	if !t.Said() {
		return false
	}
	for _, w := range words {
		if _, ok := t.Words[w]; !ok {
			return false
		}
	}
	return true
}

// verdictOf says whether the Training page and the Free Play submenu came out as two screens.
//
// With no vocabularies — the pixel-only fold — there is nothing to recognise them BY, which is
// itself the finding: a fold that saved only arrangements cannot even be asked this question
// about its own screens.
func verdictOf(f fold, said map[observe.VisualScreenID]observe.ScreenText) string {
	if said == nil {
		return "cannot be asked: the fold kept no words, so nothing identifies either screen"
	}
	var training, freePlay []observe.VisualScreenID
	for _, id := range screenOrder(f) {
		if holdsAll(said[id], trainingWords) {
			training = append(training, id)
		}
		if holdsAll(said[id], freePlayWords) {
			freePlay = append(freePlay, id)
		}
	}
	switch {
	case len(training) == 0 || len(freePlay) == 0:
		return fmt.Sprintf("NOT FOUND: %d screen(s) read as the Training page and %d as "+
			"the Free Play submenu", len(training), len(freePlay))
	case len(training) == 1 && len(freePlay) == 1 && training[0] == freePlay[0]:
		return fmt.Sprintf("MERGED: both are %s", training[0])
	}
	shared := map[observe.VisualScreenID]bool{}
	for _, a := range training {
		for _, b := range freePlay {
			if a == b {
				shared[a] = true
			}
		}
	}
	if len(shared) > 0 {
		return fmt.Sprintf("PARTLY MERGED: training %v, free play %v", training, freePlay)
	}
	return fmt.Sprintf("APART: training %v, free play %v", training, freePlay)
}

// syncSaid re-runs the synchronous fold for its vocabularies.
//
// A second run rather than a second return value, because the comparison table wants the stats
// of every fold and the verdict wants the words of two of them, and threading both through six
// rows was less readable than folding twice over a cached reading set.
func syncSaid(frames []frameRecord, regions [][]observe.Region, readable []bool,
	words []observe.ScreenText) map[observe.VisualScreenID]observe.ScreenText {

	_, _, _, said, err := semanticFold(frames, regions, readable, words, true, 0)
	if err != nil {
		return nil
	}
	return said
}
