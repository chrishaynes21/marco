package main

// THE ONE BUILD COMMAND MAKES EVERY PIECE, FROM ONE COMMIT, WHERE IT IS LOOKED FOR.
//
// build.cmd is what a developer is told to run (CLAUDE.md, and Open widget's own fix in a tree),
// so these read the script itself. They cannot check what is ON disk: the outputs are gitignored
// and absent on a machine that has not built yet.
//
// What they hold, and each was a real way to go wrong:
//   - Studio is built with the Wails tags, or it opens no window.
//   - The director and marco beside EACH studio.exe, because Studio spawns its own siblings
//     (directorBin, marcoBin). A stale sibling cost the owner several recordings.
//   - The engine pieces are built from the ROOT module, or they carry no revision.
//   - The widget's stack — marco-macros and the overlay too — lands exactly where
//     widgetstack.Beside looks from BOTH studio.exe locations. The first build script made
//     only Studio, the director and marco, so a freshly built Studio could never open the widget:
//     the bridge and the overlay were always another build.
//   - One commit: a dirty tree is refused, everything is staged, every stamp is checked against
//     HEAD, and only then is anything put in place.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetstack"
)

// buildScript is build.cmd's commands, without its comments: the comments quote the commands
// they explain, and a mutation commenting a real line out once survived a search of the whole
// file that way.
func buildScript(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("build.cmd")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.EqualFold(l, "rem") || strings.HasPrefix(strings.ToLower(l), "rem ") {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

func lineOf(lines []string, re *regexp.Regexp) int {
	for i, l := range lines {
		if re.MatchString(l) {
			return i
		}
	}
	return -1
}

func TestTheBuildScriptMakesTheWholeStackFromOneCommit(t *testing.T) {
	lines := buildScript(t)

	// WHAT IS BUILT, AND FROM WHERE. Every build writes into the stage, never into place.
	builds := map[string]string{} // output -> the command
	buildRe := regexp.MustCompile(`^go (?:-C (\S+) )?build .*-o "%STAGE%\\([\w-]+\.exe)" (\S+)`)
	for _, l := range lines {
		if strings.HasPrefix(l, "go ") && strings.Contains(l, " build ") {
			m := buildRe.FindStringSubmatch(l)
			if m == nil {
				t.Errorf("a build that does not write into the stage: %s", l)
				continue
			}
			builds[m[2]] = l
		}
	}
	for out, want := range map[string]*regexp.Regexp{
		"studio.exe":         regexp.MustCompile(`-tags %TAGS% -ldflags "[^"]*-H windowsgui[^"]*" .* \.( |$)`),
		"studio-console.exe": regexp.MustCompile(`-tags %TAGS% -o .* \.( |$)`),
		"director.exe":       regexp.MustCompile(` \.\\cmd\\director( |$)`),
		"marco.exe":          regexp.MustCompile(` \.\\cmd\\marco( |$)`),
		"marco-macros.exe":   regexp.MustCompile(` \.\\cmd\\marco-macros( |$)`),
		"overlay.exe":        regexp.MustCompile(`^go -C plugins\\overlay build .* \.( |$)`),
	} {
		if !want.MatchString(builds[out]) {
			t.Errorf("%s is not built as it must be (%s); the script has %q", out, want, builds[out])
		}
	}
	if lineOf(lines, regexp.MustCompile(`^set TAGS=desktop,production$`)) < 0 {
		t.Error("Studio is not built with the Wails tags, so it opens no window")
	}
	// The engine pieces from the root module, the only place that stamps them.
	push, pop := lineOf(lines, regexp.MustCompile(`^pushd "%ROOT%"$`)), -1
	for i := push + 1; push >= 0 && i < len(lines); i++ {
		if lines[i] == "popd" {
			pop = i
			break
		}
	}
	for _, out := range []string{"director.exe", "marco.exe", "marco-macros.exe", "overlay.exe"} {
		i := lineOf(lines, regexp.MustCompile(regexp.QuoteMeta(`"%STAGE%\`+out+`"`)+` (\.|\.\\cmd)`))
		if push < 0 || i < push || i > pop {
			t.Errorf("%s is not built from the root module, so it carries no revision", out)
		}
	}

	// ONE COMMIT: a dirty tree refused before anything is built, every stamp checked against
	// HEAD and unmodified, and only then anything put in place.
	dirty := lineOf(lines, regexp.MustCompile(`git -C "%ROOT%" status --porcelain`))
	refuse := lineOf(lines, regexp.MustCompile(`^if defined DIRTY \($`))
	first := lineOf(lines, regexp.MustCompile(`^go .*build `))
	if dirty < 0 || refuse < dirty || first < refuse || !blockExits(lines, refuse) {
		t.Error("a tree with uncommitted changes is not refused before the build")
	}
	running := lineOf(lines, regexp.MustCompile(`Get-Process`))
	if running < 0 || running > first || !blockExits(lines, running+1) {
		t.Error("a destination held by a running process is not refused before the build")
	}
	check := lineOf(lines, regexp.MustCompile(`^for %%f in \(([\w. -]+)\) do \($`))
	if check < 0 {
		t.Fatal("the staged builds' stamps are not checked")
	}
	checked := strings.Fields(regexp.MustCompile(`\(([\w. -]+)\)`).FindStringSubmatch(lines[check])[1])
	for out := range builds {
		if !contains(checked, out) {
			t.Errorf("%s is put in place without its stamp being checked", out)
		}
	}
	if !strings.Contains(lines[check+1], `findstr /c:"vcs.revision=%HEAD%"`) || !strings.Contains(lines[check+1], "goto :failed") ||
		!strings.Contains(lines[check+2], `findstr /c:"vcs.modified=false"`) || !strings.Contains(lines[check+2], "goto :failed") {
		t.Errorf("the stamp check does not refuse a stamp that is not HEAD, unmodified:\n%s\n%s", lines[check+1], lines[check+2])
	}

	// WHERE EACH PIECE GOES, resolved in a tree, against where the widget and Studio look.
	root := t.TempDir()
	for _, f := range []string{"go.mod", "plugins/studio/build.cmd"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	here := filepath.Join(root, "plugins", "studio")
	dirs := map[string]string{"%HERE%": here, "%ROOT%": root, "%OVERLAY%": filepath.Join(root, "plugins", "overlay")}
	put := map[string]bool{}
	putRe := regexp.MustCompile(`^call :put ([\w-]+\.exe) "(%\w+%)" \|\| goto :mixed$`)
	for i, l := range lines {
		if m := putRe.FindStringSubmatch(l); m != nil {
			if i < check {
				t.Errorf("%s is put in place before the stamps are checked", m[1])
			}
			if _, built := builds[m[1]]; !built {
				t.Errorf("%s is put in place and never built", m[1])
			}
			put[filepath.Join(dirs[m[2]], m[1])] = true
		}
	}
	want := []string{
		filepath.Join(here, "studio.exe"), filepath.Join(root, "studio.exe"),
		// Studio spawns these beside itself.
		filepath.Join(here, "director.exe"), filepath.Join(here, "marco.exe"),
		filepath.Join(root, "director.exe"), filepath.Join(root, "marco.exe"),
	}
	for _, from := range []string{root, here} {
		for _, c := range widgetstack.Beside(from).Components() {
			if c.Program {
				want = append(want, c.Path)
			}
		}
	}
	for _, p := range want {
		if !put[p] {
			t.Errorf("the script does not put %s, where it is looked for", p)
		}
	}

	// AND IT IS THE ONE THE INSTRUCTIONS NAME. A script nobody is told to run goes stale with
	// the thing it builds.
	doc, err := os.ReadFile(filepath.Join("..", "..", "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "plugins/studio/build.cmd") {
		t.Error("the build instructions do not name this script, so somebody following them " +
			"builds Studio with a plain `go build` and gets a binary that opens no window")
	}
}

// blockExits is whether the parenthesised block opened on line i ends the script with a failure
// before it closes.
func blockExits(lines []string, i int) bool {
	for j := i + 1; j < len(lines) && lines[j] != ")"; j++ {
		if lines[j] == "exit /b 1" {
			return true
		}
	}
	return false
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
