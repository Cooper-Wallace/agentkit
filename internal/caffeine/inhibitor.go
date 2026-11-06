package caffeine

import "runtime"

// ReleaseFunc releases a previously acquired sleep inhibition lock.
type ReleaseFunc func() error

// Inhibitor defines the interface for platform sleep inhibitors.
type Inhibitor interface {
	// Inhibit acquires the system sleep lock with a given reason.
	// Returns a ReleaseFunc to release the lock, or an error if acquisition failed.
	Inhibit(reason string) (ReleaseFunc, error)

	// Name returns the identifier of the inhibitor implementation.
	Name() string

	// IsAvailable reports whether this inhibitor can operate on the current system.
	IsAvailable() bool
}

// NewSystemInhibitor returns the platform-appropriate inhibitor.
func NewSystemInhibitor() Inhibitor {
	switch runtime.GOOS {
	case "linux":
		lin := NewLinuxInhibitor()
		if lin.IsAvailable() {
			return lin
		}
	case "darwin":
		dar := NewDarwinInhibitor()
		if dar.IsAvailable() {
			return dar
		}
	case "windows":
		win := NewWindowsInhibitor()
		if win.IsAvailable() {
			return win
		}
	}
	return NewMockInhibitor()
}
