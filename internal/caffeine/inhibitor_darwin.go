package caffeine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// DarwinInhibitor inhibits sleep on macOS using caffeinate.
type DarwinInhibitor struct {
	mu            sync.Mutex
	caffeinateBin string
}

// NewDarwinInhibitor creates a new Darwin sleep inhibitor.
func NewDarwinInhibitor() *DarwinInhibitor {
	bin, _ := exec.LookPath("caffeinate")
	if bin == "" && runtime.GOOS == "darwin" {
		if _, err := os.Stat("/usr/bin/caffeinate"); err == nil {
			bin = "/usr/bin/caffeinate"
		}
	}
	return &DarwinInhibitor{
		caffeinateBin: bin,
	}
}

// Inhibit acquires the sleep lock via macOS caffeinate.
func (d *DarwinInhibitor) Inhibit(reason string) (ReleaseFunc, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.caffeinateBin == "" {
		return nil, errors.New("caffeinate binary not found")
	}

	cmd := exec.Command(d.caffeinateBin, "-i", "-s")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start caffeinate: %w", err)
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

// Name returns the identifier of the Darwin inhibitor.
func (d *DarwinInhibitor) Name() string {
	return "darwin-caffeinate"
}

// IsAvailable reports whether caffeinate is available on Darwin.
func (d *DarwinInhibitor) IsAvailable() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return runtime.GOOS == "darwin" && d.caffeinateBin != ""
}
