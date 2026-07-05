// Command marco-app is the bundled, self-extracting Marco for Windows. pack.ps1 embeds the
// engine (marco.exe, marco-macros.exe), the overlay (overlay.exe + overlay.marco), and the
// starter routes into this single binary. On launch it unpacks them under %LOCALAPPDATA%\Marco
// and runs the overlay serve stack — so a user double-clicks one Marco.exe with no toolchain.
//
// It is its own module (stdlib only) so a plain `go build ./...` in the engine never needs the
// staged binaries; build the bundle with pack.ps1.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// The staged stack, embedded at build time by pack.ps1. These files don't exist in a fresh
// checkout — pack.ps1 builds and stages them into assets/ before compiling this module.
//
//go:embed assets/marco.exe assets/marco-macros.exe assets/overlay.exe assets/overlay.marco assets/version.txt
//go:embed all:assets/routes
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Marco:", err)
		os.Exit(1)
	}
}

func run() error {
	base := filepath.Join(localAppData(), "Marco")
	binDir := filepath.Join(base, "bin")
	routesDir := filepath.Join(base, "routes")

	ver, _ := assets.ReadFile("assets/version.txt")
	if err := extractBins(binDir, string(ver)); err != nil {
		return fmt.Errorf("unpack: %w", err)
	}
	// Seed the starter routes on first run only; a user's later edits are never overwritten.
	if _, err := os.Stat(routesDir); os.IsNotExist(err) {
		if err := seedRoutes(routesDir); err != nil {
			return fmt.Errorf("seed routes: %w", err)
		}
	}

	marco := filepath.Join(binDir, "marco.exe")
	os.Setenv("MARCO_BIN", marco)
	os.Setenv("MARCO_ROUTES", routesDir)
	fmt.Printf("Marco %s — routes at %s\n", string(ver), routesDir)

	// Mirror overlay.cmd: marco serve wires the OS + Overlay bridge hosts and runs overlay.marco.
	args := []string{"serve",
		"--host", "OS=bridge:" + filepath.Join(binDir, "marco-macros.exe"),
		"--host", "Overlay=bridge:" + filepath.Join(binDir, "overlay.exe"),
		filepath.Join(binDir, "overlay.marco")}
	args = append(args, os.Args[1:]...) // pass through any extra flags
	cmd := exec.Command(marco, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func localAppData() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d
	}
	return os.TempDir()
}

// extractBins unpacks the embedded binaries into binDir, skipping the copy when the stamped
// version already matches — so relaunching doesn't relock a running instance's exes.
func extractBins(binDir, ver string) error {
	stamp := filepath.Join(binDir, ".version")
	if ver != "" {
		if b, err := os.ReadFile(stamp); err == nil && string(b) == ver {
			return nil
		}
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"marco.exe", "marco-macros.exe", "overlay.exe", "overlay.marco"} {
		data, err := assets.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(binDir, name), data, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(stamp, []byte(ver), 0o644)
}

// seedRoutes copies the embedded starter routes into routesDir, preserving the folder tree.
func seedRoutes(routesDir string) error {
	return fs.WalkDir(assets, "assets/routes", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("assets/routes", p)
		dst := filepath.Join(routesDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := assets.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}
