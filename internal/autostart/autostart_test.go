package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutostart_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := NewManager(Options{
		BaseDir:      tmpDir,
		AgentkitPath: "/usr/local/bin/agentkit",
	})

	// 1. Initial status must be false
	st, err := mgr.Status()
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	if st.Enabled {
		t.Fatalf("expected enabled=false initially")
	}

	// 2. Enable autostart
	st, err = mgr.Enable()
	if err != nil {
		t.Fatalf("enable error: %v", err)
	}
	if !st.Enabled {
		t.Fatalf("expected enabled=true after enable")
	}

	// Check file content
	content, err := os.ReadFile(st.Path)
	if err != nil {
		t.Fatalf("read autostart file error: %v", err)
	}
	if !strings.Contains(string(content), "/usr/local/bin/agentkit caffeine daemon") {
		t.Fatalf("unexpected autostart content: %s", string(content))
	}

	// 3. Status must now report enabled=true
	st, err = mgr.Status()
	if err != nil || !st.Enabled {
		t.Fatalf("expected status enabled=true")
	}

	// 4. Re-enable (idempotent)
	st, err = mgr.Enable()
	if err != nil || !st.Enabled {
		t.Fatalf("expected re-enable to succeed idempotently")
	}

	// 5. Disable autostart
	st, err = mgr.Disable()
	if err != nil {
		t.Fatalf("disable error: %v", err)
	}
	if st.Enabled {
		t.Fatalf("expected enabled=false after disable")
	}

	// Verify file is removed
	if _, err := os.Stat(st.Path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted")
	}

	// 6. Disable again (idempotent)
	st, err = mgr.Disable()
	if err != nil || st.Enabled {
		t.Fatalf("expected disable to be idempotent")
	}
}

func TestAutostart_DefaultPaths(t *testing.T) {
	// Test with explicit XDG_CONFIG_HOME
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)

	mgr := NewManager(Options{})
	path, err := mgr.GetAutostartPath()
	if err != nil {
		t.Fatalf("GetAutostartPath error: %v", err)
	}
	if !strings.HasPrefix(path, tmpConfig) {
		t.Fatalf("expected path to start with %s, got %s", tmpConfig, path)
	}

	bin := mgr.resolveAgentkitBin()
	if bin == "" {
		t.Fatalf("expected non-empty agentkit binary path")
	}

	// Test without XDG_CONFIG_HOME
	t.Setenv("XDG_CONFIG_HOME", "")
	mgr2 := NewManager(Options{})
	path2, err := mgr2.GetAutostartPath()
	if err != nil || path2 == "" {
		t.Fatalf("expected valid path without XDG_CONFIG_HOME")
	}
}

func TestAutostart_WindowsMode(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(Options{
		BaseDir:      tmpDir,
		AgentkitPath: "C:\\bin\\agentkit.exe",
		OS:           "windows",
	})

	st, err := mgr.Enable()
	if err != nil {
		t.Fatalf("enable windows autostart error: %v", err)
	}
	if !st.Enabled || !strings.HasSuffix(st.Path, ".cmd") {
		t.Fatalf("expected .cmd file for windows, got %s", st.Path)
	}

	content, _ := os.ReadFile(st.Path)
	if !strings.Contains(string(content), "agentkit.exe") {
		t.Fatalf("expected agentkit.exe in batch file, got %s", string(content))
	}

	// Default windows path without baseDir
	t.Setenv("APPDATA", tmpDir)
	mgrWin := NewManager(Options{OS: "windows"})
	winPath, err := mgrWin.GetAutostartPath()
	if err != nil || !strings.Contains(winPath, "Startup") {
		t.Fatalf("expected windows startup path, got %s", winPath)
	}

	// Unset APPDATA
	t.Setenv("APPDATA", "")
	winPath2, err := mgrWin.GetAutostartPath()
	if err != nil || winPath2 == "" {
		t.Fatalf("expected fallback windows path")
	}
}

func TestAutostart_Errors(t *testing.T) {
	// Read-only path or invalid directory
	mgr := NewManager(Options{
		BaseDir: "/proc/non-existent-readonly",
	})
	_, err := mgr.Enable()
	if err == nil {
		t.Fatalf("expected error on read-only directory")
	}

	// Make file non-removable
	tmpDir := t.TempDir()
	mgr2 := NewManager(Options{BaseDir: tmpDir})
	st, _ := mgr2.Enable()

	// Replace file with directory so remove fails
	_ = os.Remove(st.Path)
	_ = os.Mkdir(st.Path, 0755)
	_ = os.WriteFile(filepath.Join(st.Path, "child"), []byte("sub"), 0644)

	_, err = mgr2.Disable()
	if err == nil {
		t.Fatalf("expected error deleting non-empty directory with os.Remove")
	}
}
