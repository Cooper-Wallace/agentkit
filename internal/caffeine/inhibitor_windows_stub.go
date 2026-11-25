//go:build !windows

package caffeine

import "errors"

// WindowsInhibitor is a stub on non-Windows platforms.
type WindowsInhibitor struct{}

// NewWindowsInhibitor returns a stub Windows inhibitor on non-Windows platforms.
func NewWindowsInhibitor() *WindowsInhibitor {
	return &WindowsInhibitor{}
}

// Inhibit returns an error on non-Windows platforms.
func (w *WindowsInhibitor) Inhibit(reason string) (ReleaseFunc, error) {
	return nil, errors.New("windows inhibitor is only available on windows")
}

// Name returns the identifier of the Windows inhibitor.
func (w *WindowsInhibitor) Name() string {
	return "windows-kernel32"
}

// IsAvailable reports false on non-Windows platforms.
func (w *WindowsInhibitor) IsAvailable() bool {
	return false
}
