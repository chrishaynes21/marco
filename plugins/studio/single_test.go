package main

// THE WIDGET'S `help` OPENS STUDIO'S HELP, AND STUDIO IS ONE WINDOW PER LIBRARY.
// See [[ADR-317-the-widgets-help-opens-studio]].

import (
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestALaunchNamesTheViewItWants(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-view", "help"}, "help"},
		{[]string{"--view", "help"}, "help"},
		{[]string{"-view=help"}, "help"},
		{[]string{"--view=history"}, "history"},
		{[]string{"-home", "x"}, ""},
		{nil, ""},
	} {
		if got := viewArg(c.args); got != c.want {
			t.Errorf("%q asks for %q, want %q", c.args, got, c.want)
		}
	}
}

func TestOnlyAnOfferedViewCanBeOpenedOn(t *testing.T) {
	f := Features{Views: []string{"routines", "history", "help"}}
	if f.offeredView(" Help ") != "help" {
		t.Error("help is offered and was not opened on")
	}
	if f.offeredView("acts") != "" {
		t.Error("a launch opened a view the rail withholds")
	}
}

func TestStudioIsOneWindowPerLibraryAndAHarnessHasItsOwn(t *testing.T) {
	noop := func([]string) {}
	if singleInstance(`C:\a`, false, 0, "", false, false, noop) == nil {
		t.Fatal("an ordinary launch has no single-instance lock; help would open a second window")
	}
	for name, l := range map[string]*options.SingleInstanceLock{
		"-isolate":    singleInstance(`C:\a`, true, 0, "", false, false, noop),
		"-seconds":    singleInstance(`C:\a`, false, 5, "", false, false, noop),
		"-evidence":   singleInstance(`C:\a`, false, 0, "e.txt", false, false, noop),
		"-autostart":  singleInstance(`C:\a`, false, 0, "", true, false, noop),
		"-autocancel": singleInstance(`C:\a`, false, 0, "", false, true, noop),
	} {
		if l != nil {
			t.Errorf("a %s launch would hand itself to the owner's window", name)
		}
	}
	if instanceID(`C:\Lib`) != instanceID(`c:\lib\`) {
		t.Error("two spellings of one library are two locks")
	}
	if instanceID(`C:\one`) == instanceID(`C:\two`) {
		t.Error("two libraries share one lock")
	}
	if !strings.HasPrefix(instanceID(`C:\one`), "marco-studio-") {
		t.Errorf("the lock is named %q", instanceID(`C:\one`))
	}
}

func TestALaterLaunchsArgumentsReachTheOpenWindow(t *testing.T) {
	var got []string
	l := singleInstance(`C:\a`, false, 0, "", false, false, func(a []string) { got = a })
	l.OnSecondInstanceLaunch(options.SecondInstanceData{Args: []string{"-view", "help"}})
	if strings.Join(got, " ") != "-view help" {
		t.Errorf("the open window was handed %q", got)
	}
}

func TestStudioOpensOnTheViewItWasAskedFor(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	be := strings.Replace(theRowBackend, "views: ['routines', 'history', 'help']}",
		"views: ['routines', 'history', 'help'], start: 'help'}", 1)
	title := renderForCounters(t, browser, `
window.runtime = {EventsOn: (name, fn) => { if (name === 'studio:view') window.onView = fn }}
`+be+`
let r = {}
setTimeout(() => {
  r.help = seen('view-help') && !seen('view-routines') ? 1 : 0
  r.listens = typeof window.onView === 'function' ? 1 : 0
  if (window.onView) { window.onView('routines') }
  r.back = seen('view-routines') && !seen('view-help') ? 1 : 0
  if (window.onView) { window.onView('help') }
  r.again = seen('view-help') ? 1 : 0
  if (window.onView) { window.onView('acts') }
  r.withheld = seen('view-acts') ? 0 : 1
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ')
}, 1100)`)
	n := counters(t, title, "help=")
	if n("help") != 1 {
		t.Errorf("a window opened with -view help is not on Help: %s", title)
	}
	if n("listens") != 1 || n("back") != 1 || n("again") != 1 {
		t.Errorf("a later launch's view does not reach the open window: %s", title)
	}
	if n("withheld") != 1 {
		t.Errorf("a later launch opened a view the rail withholds: %s", title)
	}
}
