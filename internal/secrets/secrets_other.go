//go:build !windows && !darwin && !linux

package secrets

// New returns a stub on platforms without a credential backend.
func New() Store { return stub{} }

type stub struct{}

func (stub) Set(string, string) error           { return ErrUnsupported }
func (stub) Get(string) (string, bool, error)   { return "", false, ErrUnsupported }
func (stub) Delete(string) error                { return ErrUnsupported }
func (stub) List() ([]string, error)            { return nil, ErrUnsupported }
