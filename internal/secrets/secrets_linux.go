//go:build linux

package secrets

import (
	"os/exec"
	"strings"
)

// New uses the freedesktop secret-service via the `secret-tool` CLI.
func New() Store { return linuxStore{} }

type linuxStore struct{}

func (linuxStore) Set(name, value string) error {
	cmd := exec.Command("secret-tool", "store", "--label=marco",
		"service", "marco", "name", name)
	cmd.Stdin = strings.NewReader(value)
	return cmd.Run()
}

func (linuxStore) Get(name string) (string, bool, error) {
	out, err := exec.Command("secret-tool", "lookup", "service", "marco", "name", name).Output()
	if err != nil {
		return "", false, nil // not found
	}
	return strings.TrimRight(string(out), "\n"), true, nil
}

func (linuxStore) Delete(name string) error {
	exec.Command("secret-tool", "clear", "service", "marco", "name", name).Run()
	return nil
}

func (linuxStore) List() ([]string, error) {
	// secret-tool has no listing; not implemented.
	return nil, nil
}
