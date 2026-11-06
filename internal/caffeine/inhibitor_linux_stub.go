//go:build !linux

package caffeine

import "errors"

// LinuxInhibitor is a stub on non-Linux platforms.
type LinuxInhibitor struct{}

// NewLinuxInhibitor returns a stub Linux inhibitor on non-Linux platforms.
func NewLinuxInhibitor() *LinuxInhibitor {
	return &LinuxInhibitor{}
}

// Inhibit returns an error on non-Linux platforms.
func (l *LinuxInhibitor) Inhibit(reason string) (ReleaseFunc, error) {
	return nil, errors.New("linux inhibitor is only available on linux")
}

// Name returns the identifier of the Linux inhibitor.
func (l *LinuxInhibitor) Name() string {
	return "linux-systemd"
}

// IsAvailable reports false on non-Linux platforms.
func (l *LinuxInhibitor) IsAvailable() bool {
	return false
}
