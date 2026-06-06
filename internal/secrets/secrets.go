// Package secrets stores secret values (passwords) in the OS credential
// manager, so routes can reference a secret by name and never contain the value
// itself. It is a platform capability behind a small interface: Windows
// Credential Manager, macOS Keychain, Linux secret-service, Windows first.
//
// The route/recording never holds the secret — the user types a {{name}}
// placeholder while teaching; codegen turns it into `do OS's Secret with
// "name"`, and the value is resolved from here at run time.
package secrets

import "errors"

// ErrUnsupported is returned on platforms without a credential backend.
var ErrUnsupported = errors.New("secrets: no credential store on this platform")

// namespace prefixes every stored entry so marco's secrets are isolated from
// other credentials in the store.
const namespace = "marco/"

// Store is the OS credential store.
type Store interface {
	// Set stores (or replaces) the value for name.
	Set(name, value string) error
	// Get returns the value and whether it was found.
	Get(name string) (value string, found bool, err error)
	// Delete removes the secret (no error if absent).
	Delete(name string) error
	// List returns the known secret names.
	List() ([]string, error)
}
