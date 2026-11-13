package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Options configures autostart paths and binary locations.
type Options struct {
	BaseDir      string `json:"base_dir,omitempty"`
	AgentkitPath string `json:"agentkit_path,omitempty"`
	OS           string `json:"os,omitempty"`
}

// StatusInfo describes the current autostart persistence status.
type StatusInfo struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

// Manager manages user session autostart configuration across platforms.
type Manager struct {
	options Options
}

// NewManager creates a new autostart Manager with the provided options.
func NewManager(opts Options) *Manager {
	return &Manager{options: opts}
}

func (m *Manager) targetOS() string {
	if m.options.OS != "" {
		return m.options.OS
	}
	return runtime.GOOS
}

// GetAutostartPath resolves the target autostart file path for current OS.
func (m *Manager) GetAutostartPath() (string, error) {
	osType := m.targetOS()
	if m.options.BaseDir != "" {
		if osType == "windows" {
			return filepath.Join(m.options.BaseDir, "agentkit-caffeine.cmd"), nil
		}
		return filepath.Join(m.options.BaseDir, "autostart", "agentkit-caffeine.desktop"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}

	if osType == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "agentkit-caffeine.cmd"), nil
	}

	// Default Linux / XDG
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "autostart", "agentkit-caffeine.desktop"), nil
}

// resolveAgentkitBin determines the path to the agentkit executable.
func (m *Manager) resolveAgentkitBin() string {
	if m.options.AgentkitPath != "" {
		return m.options.AgentkitPath
	}

	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin", "agentkit")
}

// Enable creates or updates the desktop autostart entry idempotently.
func (m *Manager) Enable() (StatusInfo, error) {
	path, err := m.GetAutostartPath()
	if err != nil {
		return StatusInfo{}, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return StatusInfo{Path: path}, fmt.Errorf("failed to create autostart directory: %w", err)
	}

	bin := m.resolveAgentkitBin()

	var content string
	if m.targetOS() == "windows" {
		content = fmt.Sprintf("@echo off\r\nstart \"\" \"%s\" caffeine daemon\r\n", bin)
	} else {
		content = fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=AgentKit Caffeine
Comment=Sleep inhibition and ambient power management for AI agent workflows
Exec=%s caffeine daemon
Icon=utilities-system-monitor
Terminal=false
Categories=Utility;
X-GNOME-Autostart-enabled=true
`, bin)
	}

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return StatusInfo{Path: path}, fmt.Errorf("failed to write autostart entry: %w", err)
	}

	return StatusInfo{Enabled: true, Path: path}, nil
}

// Disable removes the autostart entry idempotently if present.
func (m *Manager) Disable() (StatusInfo, error) {
	path, err := m.GetAutostartPath()
	if err != nil {
		return StatusInfo{}, err
	}

	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return StatusInfo{Enabled: true, Path: path}, fmt.Errorf("failed to remove autostart entry: %w", err)
		}
	}

	return StatusInfo{Enabled: false, Path: path}, nil
}

// Status inspects whether the autostart entry currently exists.
func (m *Manager) Status() (StatusInfo, error) {
	path, err := m.GetAutostartPath()
	if err != nil {
		return StatusInfo{}, err
	}

	_, statErr := os.Stat(path)
	exists := statErr == nil

	return StatusInfo{Enabled: exists, Path: path}, nil
}
