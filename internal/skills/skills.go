// Package skills is Marco's BUILT-IN DESKTOP SKILLS: things it can do on the desktop that nobody
// had to demonstrate, reached by the same words and the same review a saved routine is.
//
// Two, in the first slice:
//
//	Focus app        "focus discord": bring a window of a program that is ALREADY OPEN to the
//	                 front — the most recently active one when it has several — and say so when
//	                 it is not open. Launching is a different request and is not done here.
//	Search the web   "search the web for ark black pearl locations": open the configured search
//	                 provider's results for the words as entered, in the default browser. The
//	                 browser is Windows' setting and the provider is Marco's (overlay.json
//	                 "search"); they are separate on purpose.
//
// # Where they sit
//
// A SAVED ROUTINE'S NAME ALWAYS WINS: a skill's words are read only when no routine took the line,
// so somebody's own "focus" routine is never shadowed. The explicit forms are read here, locally,
// with no model. A paraphrase ("switch me back to Discord") reaches Director's resolver, which is
// offered the skills beside the routines and may name one; Marco then reads the skill's explicit
// line back for the same review. What the skill DOES — which window, which URL — is decided here
// and never by a model. See [[ADR-327-built-in-desktop-skills]].
//
// Stdlib only, like the rest of the engine, and no desktop: the window choice is made over a list
// the caller supplies, so it is held by tests on any machine.
package skills

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

// Skill is one built-in desktop skill.
type Skill struct {
	// Slug is its identity, `focus-app`. On the wire its whole identity is "builtin/<slug>",
	// which no saved routine can have.
	Slug string
	// Name is how it reads back: "Focus app".
	Name string
	// Arg is the one value it takes, "app".
	Arg string
	// Lead is the words that begin it, longest first; the value is the rest of the line.
	Lead []string
	// Bare is the words that name it with no value, so "focus app" asks for the app.
	Bare []string
	// Says is a line for a model choosing between routines: what it does.
	Says string
	// Put is the words its explicit line begins with when Marco writes one: "focus ".
	Put string
}

// Focus and Search are the two skills' slugs.
const (
	Focus  = "focus-app"
	Search = "search-the-web"
)

// IDPrefix begins a skill's whole identity, as `global/` begins a routine's.
const IDPrefix = "builtin/"

var all = []Skill{
	{Slug: Focus, Name: "Focus app", Arg: "app",
		Lead: []string{"focus app ", "focus on ", "switch to ", "focus "},
		Bare: []string{"focus app", "focus"},
		Says: "bring an application that is already open to the front", Put: "focus "},
	{Slug: Search, Name: "Search the web", Arg: "query",
		Lead: []string{"search the web for ", "web search for ", "search the web ", "web search "},
		Bare: []string{"search the web", "web search"},
		Says: "search the web for something, in the default browser", Put: "search the web for "},
}

// All is every built-in skill.
func All() []Skill { return append([]Skill(nil), all...) }

// Of is the skill with a slug.
func Of(slug string) (Skill, bool) {
	for _, s := range all {
		if s.Slug == slug {
			return s, true
		}
	}
	return Skill{}, false
}

// ID is a skill's whole identity.
func (s Skill) ID() string { return IDPrefix + s.Slug }

// Read is whether a line is a skill's explicit form, and the value it carries ("" when it names
// the skill and gives nothing). The value is the rest of the line AS TYPED: a query is passed on
// as entered, so only the spaces around it are trimmed.
func Read(line string) (Skill, string, bool) {
	line = strings.TrimSpace(line)
	low := strings.ToLower(line)
	for _, s := range all {
		for _, b := range s.Bare {
			if low == b {
				return s, "", true
			}
		}
	}
	for _, s := range all {
		for _, l := range s.Lead {
			if strings.HasPrefix(low, l) {
				v := strings.TrimSpace(line[len(l):])
				if s.Slug == Focus {
					// An application's name is not a sentence: a full stop typed after it
					// is not part of it.
					v = strings.TrimSpace(strings.TrimRight(v, ".!?"))
				}
				if v == "" {
					return s, "", true
				}
				return s, v, true
			}
		}
	}
	return Skill{}, "", false
}

// LineFor is a skill's explicit line with a value, the line the command line reviews: "focus
// Discord", "search the web for ark black pearl locations". With no value it is the skill's name,
// whose reading then asks for one.
func LineFor(slug, value string) string {
	s, ok := Of(slug)
	if !ok {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.ToLower(s.Name)
	}
	return s.Put + value
}

// ── FOCUS APP ──

// Window is one top-level window on the desktop, in the order the window system lists them:
// front to back, so the first window of a program is its most recently active.
type Window struct {
	Handle    uintptr
	PID       uint32
	Image     string // the program, lowercased, no path or extension: "discord"
	Title     string
	Minimized bool
}

// Ambiguous is a name that matched more than one program. The person is asked which.
type Ambiguous struct {
	Name     string
	Programs []string
}

func (a Ambiguous) Error() string {
	return fmt.Sprintf("%q could be %s — say which: %s", a.Name, orList(a.Programs),
		"focus "+a.Programs[0])
}

// NotOpen is a name no open window answers to.
type NotOpen struct{ Name string }

func (n NotOpen) Error() string {
	return fmt.Sprintf("%s isn't open — no window of it is on the desktop, so nothing was brought "+
		"forward (Marco doesn't launch it from here)", n.Name)
}

// ChooseWindow is the window "focus <name>" brings forward, out of ws (front to back).
//
// The program is matched, then its most recently active window taken:
//
//  1. a program whose executable IS the name ("discord", "Discord.exe", "notepad");
//  2. else one whose executable begins with it ("discord" → discordptb);
//  3. else one whose window titles END with the name, which is where an application writes its
//     own ("… - Visual Studio Code", "Steam"). Not anywhere in a title: a browser tab about
//     Notepad is not Notepad.
//
// Within a step, MORE THAN ONE PROGRAM IS A QUESTION (Ambiguous), never a choice. Marco's own
// windows are never chosen.
//
// Taking any window but a program's first must fail TestFocusTakesTheMostRecentlyActiveWindow.
func ChooseWindow(name string, ws []Window, ours func(image string) bool) (Window, error) {
	want := key(name)
	if want == "" {
		return Window{}, fmt.Errorf("say which application to focus")
	}
	var mine []Window
	for _, w := range ws {
		if ours != nil && ours(w.Image) {
			continue
		}
		mine = append(mine, w)
	}
	rules := []func(Window) bool{
		func(w Window) bool { return key(w.Image) == want },
		func(w Window) bool { return strings.HasPrefix(key(w.Image), want) },
		func(w Window) bool { return titleNames(w.Title, name) },
	}
	for _, match := range rules {
		first := map[string]Window{}
		var order []string
		for _, w := range mine {
			if !match(w) {
				continue
			}
			if _, seen := first[w.Image]; !seen {
				first[w.Image] = w
				order = append(order, w.Image)
			}
		}
		switch len(order) {
		case 0:
			continue
		case 1:
			return first[order[0]], nil
		}
		sort.Strings(order)
		return Window{}, Ambiguous{Name: name, Programs: order}
	}
	return Window{}, NotOpen{Name: name}
}

// titleNames is whether a window's title ends with an application's name, as applications write
// their own: the whole title, or after " - ", " — ", " | ".
func titleNames(title, name string) bool {
	t, n := strings.ToLower(strings.TrimSpace(title)), strings.ToLower(strings.TrimSpace(name))
	if n == "" || t == "" {
		return false
	}
	if t == n {
		return true
	}
	for _, sep := range []string{" - ", " — ", " – ", " | "} {
		if strings.HasSuffix(t, sep+n) {
			return true
		}
	}
	return false
}

// key is a program's name reduced for comparison: lowercased, ".exe" dropped, letters and digits
// only, so "Discord.exe", "discord" and "Disc ord" are one key.
func key(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".exe")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func orList(xs []string) string {
	switch len(xs) {
	case 1:
		return xs[0]
	case 2:
		return xs[0] + " or " + xs[1]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " or " + xs[len(xs)-1]
}

// ── SEARCH THE WEB ──

// DefaultProvider is the search provider when none is configured.
const DefaultProvider = "google"

// providers are the named search providers; {query} is where the words go.
var providers = map[string]string{
	"google":     "https://www.google.com/search?q={query}",
	"bing":       "https://www.bing.com/search?q={query}",
	"duckduckgo": "https://duckduckgo.com/?q={query}",
	"brave":      "https://search.brave.com/search?q={query}",
	"startpage":  "https://www.startpage.com/do/search?q={query}",
}

// Providers is the named providers, sorted.
func Providers() []string {
	out := make([]string, 0, len(providers))
	for k := range providers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ProviderName is how a configured provider reads back: its name, or the host of a template.
func ProviderName(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		return DefaultProvider
	}
	if _, ok := providers[p]; ok {
		return p
	}
	if u, err := url.Parse(strings.ReplaceAll(provider, "{query}", "q")); err == nil && u.Host != "" {
		return u.Host
	}
	return provider
}

// SearchURL is the results page for a query from a provider: a name ("google", "duckduckgo"), a
// web address with {query} in it, or "" for the default. The query is passed AS ENTERED, only
// encoded for the address.
//
// A provider that is not a web address is REFUSED, not opened: what this returns is handed to the
// shell, and the shell opens programs as readily as pages.
//
// Opening anything but an http(s) address must fail TestASearchOpensOnlyAWebAddress.
func SearchURL(provider, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("say what to search for")
	}
	p := strings.TrimSpace(provider)
	if p == "" {
		p = DefaultProvider
	}
	tmpl, named := providers[strings.ToLower(p)]
	if !named {
		tmpl = p
	}
	// THE WORDS GO IN THE ADDRESS, NEVER IN WHERE IT GOES: a template whose host is the query
	// would send the search to whatever site the words name.
	if t, err := url.Parse(strings.ReplaceAll(tmpl, "{query}", "QUERY")); err == nil &&
		strings.Contains(t.Host, "QUERY") {
		return "", fmt.Errorf("the search provider %q puts the words in the site's name, so "+
			"nothing was searched", provider)
	}
	if !strings.Contains(tmpl, "{query}") {
		return "", fmt.Errorf("the search provider %q is not one of %s and has no {query} in it, "+
			"so nothing was searched", provider, strings.Join(Providers(), ", "))
	}
	out := strings.ReplaceAll(tmpl, "{query}", url.QueryEscape(query))
	u, err := url.Parse(out)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", fmt.Errorf("the search provider %q is not a web address (https://…{query}), "+
			"so nothing was searched", provider)
	}
	return out, nil
}

// Program is one open program as the question about which one somebody meant sees it: a label
// the answer must copy, the name its window gives it, and its most recently active window.
type Program struct {
	Label  string
	Name   string
	Window Window
}

// Programs is each open program once, front to back, with a unique slug-shaped label made from
// the application's own name in its window title ("Google Chrome" out of "… - Google Chrome"),
// never the rest of a title: a page or a document's name is not the program and stays here.
// Marco's own programs are left out.
func Programs(ws []Window, ours func(image string) bool) []Program {
	seen, taken := map[string]bool{}, map[string]bool{}
	var out []Program
	for _, w := range ws {
		if seen[w.Image] || (ours != nil && ours(w.Image)) {
			continue
		}
		seen[w.Image] = true
		name := AppName(w.Title)
		label := ""
		for _, try := range []string{slug(name), slug(w.Image)} {
			if try != "" && !taken[try] {
				label = try
				break
			}
		}
		for n := 2; label == ""; n++ {
			if try := fmt.Sprintf("%s-%d", slug(w.Image), n); !taken[try] {
				label = try
			}
		}
		taken[label] = true
		if name == "" {
			name = w.Image
		}
		out = append(out, Program{Label: label, Name: name, Window: w})
	}
	return out
}

// ours are Marco's own programs, which "focus" never brings forward and nothing is told to listen
// for.
var ours = map[string]bool{"marco": true, "director": true, "overlay": true,
	"marco-app": true, "marco-macros": true, "studio": true, "web-ui": true, "voice": true,
	"llama": true}

// Ours is whether a program (its executable's name, no path or extension) is one of Marco's own.
func Ours(image string) bool { return ours[strings.ToLower(image)] }

// SpokenNames is what somebody could call each open program when they say "focus …": its
// executable's name, and the name its window gives it. The window's name is taken only where the
// title marks it as the application's own — after " - ", " — ", " – " or " | " — or where the whole
// title IS the program ("Steam", "Destiny 2" for destiny2). The rest of a title is a page, a
// document or a person, and is never listed. Marco's own programs are left out.
//
// Taking a title that names neither must fail TestOnlyAnApplicationsOwnNameIsSpoken.
func SpokenNames(ws []Window) []string {
	var out []string
	for _, p := range Programs(ws, Ours) {
		out = append(out, p.Window.Image)
		t := strings.TrimSpace(p.Window.Title)
		if p.Name == "" || p.Name == p.Window.Image {
			continue
		}
		if p.Name != t || key(t) == key(p.Window.Image) {
			out = append(out, p.Name)
		}
	}
	return out
}

// Words is every word that begins or names a skill, and the everyday ways of asking for an
// application: "focus", "switch to", "search the web", "go back to". The recogniser is told them so
// the request around a name is heard as said.
func Words() []string {
	var out []string
	for _, s := range all {
		for _, l := range s.Lead {
			out = append(out, strings.TrimSpace(l))
		}
		out = append(out, s.Bare...)
	}
	for _, o := range Opens {
		out = append(out, strings.TrimSpace(o))
	}
	return out
}

// AppName is the application's name in a window title: what follows the last " - ", " — ",
// " – " or " | ", which is where applications write their own, or the whole title when there is
// none ("Steam", "ChatGPT").
func AppName(title string) string {
	t := strings.TrimSpace(title)
	cut := -1
	sepLen := 0
	for _, sep := range []string{" - ", " — ", " – ", " | "} {
		if i := strings.LastIndex(t, sep); i > cut {
			cut, sepLen = i, len(sep)
		}
	}
	if cut >= 0 {
		return strings.TrimSpace(t[cut+sepLen:])
	}
	return t
}

// slug is a label: lowercase letters and digits, words joined by "-".
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	return b.String()
}

// Opens are the everyday ways of asking for an application that are Focus app ONLY when the rest
// of the line is the name of a program that is open right now: "open chat gp t" with ChatGPT open.
// Unlike "focus" and "switch to", these words are also how routines are named ("open social",
// "go to free play"), so a line whose rest names no open program is not claimed and goes on as it
// did, to Director, where a misheard routine name is recovered.
var Opens = []string{"switch me back to ", "switch back to ", "switch me to ", "go back to ",
	"bring up ", "show me ", "open ", "go to "}

// ReadOpen is the rest of a line beginning with one of Opens, as typed.
func ReadOpen(line string) (string, bool) {
	line = strings.TrimSpace(line)
	low := strings.ToLower(line)
	for _, o := range Opens {
		if strings.HasPrefix(low, o) {
			if rest := strings.TrimSpace(strings.TrimRight(line[len(o):], ".!?")); rest != "" {
				return rest, true
			}
		}
	}
	return "", false
}

// NamesOpenProgram is whether a name IS an open program's, by its executable or by the
// application's own name in its window title, reduced the same way ("chat gp t" is chatgpt).
// Not a prefix and not a model: this decides whether everyday words are claimed at all.
func NamesOpenProgram(name string, ws []Window, ours func(image string) bool) bool {
	want := key(name)
	if want == "" {
		return false
	}
	for _, w := range ws {
		if ours != nil && ours(w.Image) {
			continue
		}
		if key(w.Image) == want || key(AppName(w.Title)) == want {
			return true
		}
	}
	return false
}
