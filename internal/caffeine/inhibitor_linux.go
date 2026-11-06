package caffeine

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
)

// LinuxInhibitor inhibits sleep on Linux using systemd-inhibit.
type LinuxInhibitor struct {
	mu         sync.Mutex
	inhibitBin string
}

// NewLinuxInhibitor creates a new Linux sleep inhibitor.
func NewLinuxInhibitor() *LinuxInhibitor {
	bin, _ := exec.LookPath("systemd-inhibit")
	return &LinuxInhibitor{
		inhibitBin: bin,
	}
}

// Inhibit acquires the system sleep lock via systemd-inhibit.
func (l *LinuxInhibitor) Inhibit(reason string) (ReleaseFunc, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.inhibitBin == "" {
		return nil, errors.New("systemd-inhibit binary not found")
	}

	cmd := exec.Command(
		l.inhibitBin,
		"--what=idle:sleep",
		"--who=agentkit-caffeine",
		fmt.Sprintf("--why=%s", reason),
		"sleep",
		"infinity",
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start systemd-inhibit: %w", err)
	}

	var once sync.Once
	return func() error {
		var relErr error
		once.Do(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		return relErr
	}, nil
}

// Name returns the identifier of the Linux inhibitor.
func (l *LinuxInhibitor) Name() string {
	return "linux-systemd"
}

// IsAvailable reports whether systemd-inhibit is available on Linux.
func (l *LinuxInhibitor) IsAvailable() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return runtime.GOOS == "linux" && l.inhibitBin != ""
}
