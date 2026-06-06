//go:build darwin

package secrets

import (
	"os/exec"
	"strings"
)

// New uses the macOS Keychain via the `security` CLI.
func New() Store { return macStore{} }

type macStore struct{}

func acct(name string) string { return namespace + name }

func (macStore) Set(name, value string) error {
	// -U updates if it exists.
	cmd := exec.Command("security", "add-generic-password",
		"-a", acct(name), "-s", "marco", "-w", value, "-U")
	return cmd.Run()
}

func (macStore) Get(name string) (string, bool, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-a", acct(name), "-s", "marco", "-w").Output()
	if err != nil {
		return "", false, nil // not found (security exits non-zero)
	}
	return strings.TrimRight(string(out), "\n"), true, nil
}

func (macStore) Delete(name string) error {
	exec.Command("security", "delete-generic-password",
		"-a", acct(name), "-s", "marco").Run()
	return nil
}

func (macStore) List() ([]string, error) {
	// `security` has no clean per-service listing; not implemented.
	return nil, nil
}
