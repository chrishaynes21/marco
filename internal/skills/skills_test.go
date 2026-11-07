package skills

import (
	"errors"
	"strings"
	"testing"
)

// THE EXPLICIT FORMS ARE READ, WITH THE VALUE AS TYPED, AND THE BARE NAME ASKS FOR ONE.
func TestTheExplicitFormsAreRead(t *testing.T) {
	for _, c := range []struct{ line, slug, value string }{
		{"focus Discord", Focus, "Discord"},
		{"Focus discord.", Focus, "discord"},
		{"focus app Visual Studio Code", Focus, "Visual Studio Code"},
		{"switch to steam", Focus, "steam"},
		{"focus", Focus, ""},
		{"Focus app", Focus, ""},
		{"search the web for Ark black pearl locations", Search, "Ark black pearl locations"},
		{"search the web for  what is 2+2?  ", Search, "what is 2+2?"},
		{"web search ark wiki", Search, "ark wiki"},
		{"search the web", Search, ""},
	} {
		s, v, ok := Read(c.line)
		if !ok || s.Slug != c.slug || v != c.value {
			t.Errorf("Read(%q) = %q %q %v, want %q %q", c.line, s.Slug, v, ok, c.slug, c.value)
		}
	}
	for _, line := range []string{"focusing on work", "open social", "search for wood", ""} {
		if s, _, ok := Read(line); ok {
			t.Errorf("Read(%q) read as %s", line, s.Slug)
		}
	}
}

// THE LINE MARCO WRITES FOR A SKILL READS BACK AS THAT SKILL AND VALUE.
func TestALineWrittenForASkillReadsAsIt(t *testing.T) {
	for _, s := range All() {
		for _, v := range []string{"", "Discord", "ark black pearl"} {
			got, val, ok := Read(LineFor(s.Slug, v))
			if !ok || got.Slug != s.Slug || val != v {
				t.Errorf("LineFor(%s, %q) = %q reads as %q %q", s.Slug, v, LineFor(s.Slug, v), got.Slug, val)
			}
		}
	}
}

var desk = []Window{
	{Handle: 1, Image: "chrome", Title: "Discord | #general - Google Chrome"},
	{Handle: 2, Image: "discord", Title: "#raids - Discord"},
	{Handle: 3, Image: "code", Title: "skills.go - marco - Visual Studio Code"},
	{Handle: 4, Image: "discord", Title: "Discord"},
	{Handle: 5, Image: "overlay", Title: "Marco"},
	{Handle: 6, Image: "notepad", Title: "notes.txt - Notepad", Minimized: true},
	{Handle: 7, Image: "studio64", Title: "Android Studio"},
	{Handle: 8, Image: "devenv", Title: "Solution1 - Microsoft Visual Studio"},
}

func marco(image string) bool { return image == "overlay" || image == "marco" }

// THE PROGRAM IS MATCHED AND ITS MOST RECENTLY ACTIVE WINDOW TAKEN.
func TestFocusTakesTheMostRecentlyActiveWindow(t *testing.T) {
	for _, c := range []struct {
		name string
		want uintptr
	}{
		{"Discord", 2}, {"discord.exe", 2}, {"notepad", 6}, {"Visual Studio Code", 3},
		{"code", 3}, {"android studio", 7},
	} {
		w, err := ChooseWindow(c.name, desk, marco)
		if err != nil || w.Handle != c.want {
			t.Errorf("focus %q = window %d, %v; want %d", c.name, w.Handle, err, c.want)
		}
	}
}

// A NAME TWO PROGRAMS ANSWER TO IS ASKED ABOUT; ONE NONE DOES IS SAID; MARCO IS NEVER CHOSEN.
func TestFocusAsksWhenTheNameIsAmbiguousAndSaysWhenNothingIsOpen(t *testing.T) {
	ws := append([]Window{{Handle: 9, Image: "discordptb", Title: "Discord PTB"},
		{Handle: 10, Image: "discordcanary", Title: "Discord Canary"}}, desk[2], desk[5])
	_, err := ChooseWindow("discord", ws, marco)
	var amb Ambiguous
	if !errors.As(err, &amb) || len(amb.Programs) != 2 {
		t.Fatalf("two Discords: %v", err)
	}
	if !strings.Contains(err.Error(), "say which") {
		t.Errorf("the question does not ask: %v", err)
	}
	var none NotOpen
	if _, err := ChooseWindow("spotify", desk, marco); !errors.As(err, &none) ||
		!strings.Contains(err.Error(), "isn't open") {
		t.Errorf("spotify: %v", err)
	}
	if _, err := ChooseWindow("overlay", desk, marco); !errors.As(err, &none) {
		t.Errorf("Marco's own window was chosen: %v", err)
	}
	// A browser tab about Discord is not Discord.
	if _, err := ChooseWindow("discord", desk[:1], marco); !errors.As(err, &none) {
		t.Errorf("a tab whose title names Discord was taken for it: %v", err)
	}
}

// THE QUERY GOES IN AS ENTERED, AND ONLY A WEB ADDRESS IS EVER OPENED.
func TestASearchOpensOnlyAWebAddress(t *testing.T) {
	got, err := SearchURL("", "Ark black pearl locations")
	if err != nil || got != "https://www.google.com/search?q=Ark+black+pearl+locations" {
		t.Errorf("default: %q %v", got, err)
	}
	got, err = SearchURL("DuckDuckGo", "a&b=c")
	if err != nil || got != "https://duckduckgo.com/?q=a%26b%3Dc" {
		t.Errorf("named: %q %v", got, err)
	}
	got, err = SearchURL("https://example.org/find?term={query}", "x y")
	if err != nil || got != "https://example.org/find?term=x+y" {
		t.Errorf("template: %q %v", got, err)
	}
	for _, p := range []string{"calc.exe {query}", "file:///C:/Windows/{query}", "C:\\x.bat {query}",
		"nosuchprovider", "https://{query}", "ftp://files.example/{query}", "javascript://x/{query}"} {
		if u, err := SearchURL(p, "q"); err == nil {
			t.Errorf("provider %q opened %q", p, u)
		}
	}
	if _, err := SearchURL("", "  "); err == nil {
		t.Error("an empty query was searched")
	}
}

// EACH OPEN PROGRAM ONCE, BY ITS OWN NAME, AND NOTHING ELSE OF ITS TITLE.
func TestTheOpenProgramsAreNamedAsTheirWindowsNameThem(t *testing.T) {
	ws := []Window{
		{Handle: 1, Image: "chrome", Title: "(101) Some Video - YouTube - Google Chrome"},
		{Handle: 2, Image: "chatgpt", Title: "ChatGPT"},
		{Handle: 3, Image: "steamwebhelper", Title: "Steam"},
		{Handle: 4, Image: "chrome", Title: "Inbox - Gmail - Google Chrome"},
		{Handle: 5, Image: "overlay", Title: "marco overlay"},
		{Handle: 6, Image: "notepad", Title: "secret plans.txt - Notepad"},
		{Handle: 7, Image: "notepad2", Title: "Notepad"},
	}
	got := Programs(ws, marco)
	want := []struct {
		label, name string
		handle      uintptr
	}{{"google-chrome", "Google Chrome", 1}, {"chatgpt", "ChatGPT", 2}, {"steam", "Steam", 3},
		{"notepad", "Notepad", 6}, {"notepad2", "Notepad", 7}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if got[i].Label != w.label || got[i].Name != w.name || got[i].Window.Handle != w.handle {
			t.Errorf("program %d = %q %q %d, want %q %q %d", i, got[i].Label, got[i].Name,
				got[i].Window.Handle, w.label, w.name, w.handle)
		}
	}
	for _, p := range got {
		if strings.Contains(p.Name, "YouTube") || strings.Contains(p.Name, "secret") {
			t.Errorf("a title's content was kept: %q", p.Name)
		}
	}
}
