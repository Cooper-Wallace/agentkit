//go:build windows

package caffeine

import (
	"errors"
	"sync"
	"syscall"
)

const (
	esContinuous       = 0x80000000
	esSystemRequired   = 0x00000001
	esAwayModeRequired = 0x00000040
)

// WindowsInhibitor inhibits sleep on Windows using SetThreadExecutionState.
type WindowsInhibitor struct {
	mu           sync.Mutex
	kernel32     *syscall.LazyDLL
	setExecution *syscall.LazyProc
}

// NewWindowsInhibitor creates a new Windows sleep inhibitor.
func NewWindowsInhibitor() *WindowsInhibitor {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	return &WindowsInhibitor{
		kernel32:     k32,
		setExecution: k32.NewProc("SetThreadExecutionState"),
	}
}

// Inhibit acquires the system sleep lock via Windows SetThreadExecutionState.
func (w *WindowsInhibitor) Inhibit(reason string) (ReleaseFunc, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	flags := uintptr(esContinuous | esSystemRequired | esAwayModeRequired)
	ret, _, err := w.setExecution.Call(flags)
	if ret == 0 {
		return nil, errors.New("SetThreadExecutionState failed: " + err.Error())
	}

	var once sync.Once
	return func() error {
		var relErr error
		once.Do(func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			res, _, err := w.setExecution.Call(uintptr(esContinuous))
			if res == 0 {
				relErr = errors.New("SetThreadExecutionState reset failed: " + err.Error())
			}
		})
		return relErr
	}, nil
}

// Name returns the identifier of the Windows inhibitor.
func (w *WindowsInhibitor) Name() string {
	return "windows-kernel32"
}

// IsAvailable reports whether the Windows inhibitor is available.
func (w *WindowsInhibitor) IsAvailable() bool {
	return true
}
