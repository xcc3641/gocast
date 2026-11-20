//go:build !darwin

package keepawake

// System returns the system's power assertions: none but on macOS.
func System() Backend { return noBackend{} }

type noBackend struct{}

func (noBackend) Create(AssertionType, string) (Assertion, error) { return 0, ErrUnsupported }
func (noBackend) Release(Assertion) error                         { return ErrUnsupported }
