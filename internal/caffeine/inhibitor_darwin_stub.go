//go:build !darwin

package caffeine

import "errors"

// DarwinInhibitor is a stub on non-Darwin platforms.
type DarwinInhibitor struct{}

// NewDarwinInhibitor returns a stub Darwin inhibitor on non-Darwin platforms.
func NewDarwinInhibitor() *DarwinInhibitor {
	return &DarwinInhibitor{}
}

// Inhibit returns an error on non-Darwin platforms.
func (d *DarwinInhibitor) Inhibit(reason string) (ReleaseFunc, error) {
	return nil, errors.New("darwin inhibitor is only available on darwin")
}

// Name returns the identifier of the Darwin inhibitor.
func (d *DarwinInhibitor) Name() string {
	return "darwin-caffeinate"
}

// IsAvailable reports false on non-Darwin platforms.
func (d *DarwinInhibitor) IsAvailable() bool {
	return false
}
