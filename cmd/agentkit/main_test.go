package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIBasics(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// No args -> exit 1
	code := run([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for no args, got %d", code)
	}

	// Help -> exit 0
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"help"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "agentkit") {
		t.Errorf("help failed, code=%d, out=%s", code, stdout.String())
	}

	// -h
	code = run([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("-h failed, code=%d", code)
	}

	// Version -> exit 0
	stdout.Reset()
	code = run([]string{"version"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "agentkit version") {
		t.Errorf("version failed, code=%d, out=%s", code, stdout.String())
	}

	// -v
	code = run([]string{"-v"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("-v failed, code=%d", code)
	}

	// Unknown command -> exit 1
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"unknown-cmd"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for unknown command, got %d", code)
	}
}

func TestCLIAuditAndApply(t *testing.T) {
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")
	mcpFile := filepath.Join(tempDir, "mcp.json")

	var stdout, stderr bytes.Buffer

	// Apply dry-run
	code := run([]string{"apply", "--dry-run", "--settings-path", settingsFile}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "[DRY-RUN]") {
		t.Errorf("apply dry-run failed, code=%d, out=%s, err=%s", code, stdout.String(), stderr.String())
	}

	// Apply real
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"apply", "--settings-path", settingsFile, "--no-mcp", "--no-plugins"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "[SUCCESS]") {
		t.Errorf("apply failed, code=%d, out=%s, err=%s", code, stdout.String(), stderr.String())
	}

	// Audit on incomplete setup -> exit 1
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"audit", "--settings-path", settingsFile, "--mcp-path", mcpFile}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected audit non-compliant exit 1, got %d", code)
	}

	// Audit with --json
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"audit", "--json", "--settings-path", settingsFile}, &stdout, &stderr)
	var js map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &js); err != nil {
		t.Errorf("audit --json produced invalid JSON: %v, out=%s", err, stdout.String())
	}
}

func TestCLIDiffAndSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")
	_ = os.WriteFile(settingsFile, []byte(`{"permissions": {"allow": ["command(custom_cmd)"]}}`), 0644)

	var stdout, stderr bytes.Buffer

	// Diff text
	code := run([]string{"diff", "--settings-path", settingsFile, "--no-mcp"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Missing Standard Permissions") {
		t.Errorf("diff failed, code=%d, out=%s", code, stdout.String())
	}

	// Diff json
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"diff", "--json", "--settings-path", settingsFile, "--no-mcp"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("diff --json failed, code=%d", code)
	}
	var diffJS map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &diffJS); err != nil {
		t.Errorf("diff --json invalid JSON: %v", err)
	}

	// Snapshot text
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"snapshot", "--settings-path", settingsFile}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "command(custom_cmd)") {
		t.Errorf("snapshot failed, code=%d, out=%s", code, stdout.String())
	}

	// Snapshot JSON
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"snapshot", "--json", "--settings-path", settingsFile}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("snapshot --json failed, code=%d", code)
	}
	var snapJS map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &snapJS); err != nil {
		t.Errorf("snapshot --json invalid JSON: %v", err)
	}

	// Snapshot to output dir
	outDir := filepath.Join(tempDir, "snapshot_out")
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"snapshot", "--output", outDir, "--settings-path", settingsFile}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("snapshot --output failed, code=%d", code)
	}
	if _, err := os.Stat(filepath.Join(outDir, "snapshot.json")); err != nil {
		t.Errorf("snapshot file missing in outDir")
	}
}

func TestCLIProfile(t *testing.T) {
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")
	geminiDir := filepath.Join(tempDir, ".gemini")

	var stdout, stderr bytes.Buffer

	// Profile show
	code := run([]string{"profile", "show"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "argon-flash") {
		t.Errorf("profile show failed, code=%d, out=%s", code, stdout.String())
	}

	// Profile switch missing name -> exit 1
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"profile", "switch"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for missing switch name, got %d", code)
	}

	// Profile switch invalid name -> exit 1
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"profile", "switch", "nonexistent-tier"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for invalid profile, got %d", code)
	}

	// Profile switch valid with custom settings path
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"profile", "switch", "flash-high", "--settings-path", settingsFile, "--gemini-dir", geminiDir}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "flash-high") {
		t.Errorf("profile switch failed, code=%d, out=%s, err=%s", code, stdout.String(), stderr.String())
	}

	// Direct name switch
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"profile", "opus-flash", "--settings-path", settingsFile, "--gemini-dir", geminiDir}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "opus-flash") {
		t.Errorf("profile direct switch failed, code=%d, out=%s, err=%s", code, stdout.String(), stderr.String())
	}
}

func TestCLIQuota(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Quota status
	code := run([]string{"quota", "status"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "gemini") {
		t.Errorf("quota status failed, code=%d, out=%s", code, stdout.String())
	}

	// Quota status --json
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"quota", "status", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("quota status --json failed, code=%d", code)
	}
	var qJS map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &qJS); err != nil {
		t.Errorf("quota status --json invalid JSON: %v", err)
	}

	// Quota route high
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"quota", "route", "--complexity", "high"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Model:") {
		t.Errorf("quota route failed, code=%d, out=%s", code, stdout.String())
	}

	// Quota route --json
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"quota", "route", "--complexity", "low", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("quota route --json failed, code=%d", code)
	}

	// Quota route missing complexity -> exit 1
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"quota", "route"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for quota route without complexity, got %d", code)
	}
}

func TestCLITemplate(t *testing.T) {
	tempDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	// Template agents
	target := filepath.Join(tempDir, "proj1")
	code := run([]string{"template", "--target", target, "--name", "proj1"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Scaffolded agents") {
		t.Errorf("template agents failed, code=%d, out=%s, err=%s", code, stdout.String(), stderr.String())
	}

	// Template legacy (gemini)
	target2 := filepath.Join(tempDir, "proj2")
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"template", "--target", target2, "--legacy"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Scaffolded gemini") {
		t.Errorf("template legacy failed, code=%d, out=%s", code, stdout.String())
	}

	// Template hook
	target3 := filepath.Join(tempDir, "proj3")
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"template", "--target", target3, "--type", "hook"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Scaffolded hook") {
		t.Errorf("template hook failed, code=%d, out=%s", code, stdout.String())
	}

	// Template fail without force
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"template", "--target", target}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for template without force on existing dir, got %d", code)
	}
}

func TestCLIFlagsAndBranches(t *testing.T) {
	tempDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	// 1. Audit flags: --verbose, --settings-path=, --harness, error branch
	code := run([]string{"audit", "--verbose", "--harness", "antigravity", "--settings-path=" + filepath.Join(tempDir, "missing.json")}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for missing settings in audit, got %d", code)
	}

	// 2. Apply flags: --link-home, --no-backup, --model-profile=, --harness
	stdout.Reset()
	stderr.Reset()
	applySettings := filepath.Join(tempDir, "apply_settings.json")
	code = run([]string{
		"apply",
		"--dry-run",
		"--link-home",
		"--no-backup",
		"--model-profile=argon-flash",
		"--harness", "antigravity",
		"--settings-path=" + applySettings,
	}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("apply with flags failed, code=%d, err=%s", code, stderr.String())
	}

	// Apply with error
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"apply", "--settings-path", "/dev/null/forbidden/settings.json"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for invalid apply path, got %d", code)
	}

	// 3. Diff flags: --mcp-path=, --harness, error branch
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"diff", "--settings-path=" + applySettings, "--mcp-path=" + filepath.Join(tempDir, "mcp.json"), "--harness", "antigravity"}, &stdout, &stderr)
	if code != 1 { // applySettings does not exist yet
		t.Errorf("expected exit 1 for missing diff settings, got %d", code)
	}

	// 4. Profile alias replacement (pro-flash -> flash-high)
	stdout.Reset()
	stderr.Reset()
	geminiDir := filepath.Join(tempDir, ".gemini")
	code = run([]string{"profile", "switch", "pro-flash", "--settings-path", applySettings, "--gemini-dir", geminiDir}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stderr.String(), "deprecated") {
		t.Errorf("profile switch with alias failed: code=%d, err=%s", code, stderr.String())
	}

	// Direct alias switch
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"profile", "pro-flash", "--settings-path", applySettings, "--gemini-dir", geminiDir}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "flash-high") {
		t.Errorf("profile direct alias failed: code=%d, out=%s", code, stdout.String())
	}

	// 5. Quota flags: --file=, --complexity=, error branch
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"quota", "status", "--file=" + filepath.Join(tempDir, "missing_quota.json")}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for missing quota file, got %d", code)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"quota", "route", "--complexity=med"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Model:  flash") {
		t.Errorf("quota route --complexity=med failed, code=%d, out=%s", code, stdout.String())
	}

	// 6. Template flags: -t, -n, -d, -l, --test, --desc=, error branch
	stdout.Reset()
	stderr.Reset()
	tmplTarget := filepath.Join(tempDir, "tmpl_full")
	code = run([]string{
		"template",
		"-t", "agents",
		"-o", tmplTarget,
		"-n", "full-service",
		"--desc=Comprehensive microservice",
		"-l", "Go",
		"--test", "go test ./...",
	}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Scaffolded agents") {
		t.Errorf("template with full flags failed: code=%d, out=%s, err=%s", code, stdout.String(), stderr.String())
	}

	// 7. Snapshot error branch and clean state branch
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"snapshot", "--settings-path", filepath.Join(tempDir, "missing_snap.json")}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for missing snapshot settings, got %d", code)
	}

	// Snapshot clean state (no unmanaged)
	stdout.Reset()
	stderr.Reset()
	cleanSettings := filepath.Join(tempDir, "clean_settings.json")
	_ = os.WriteFile(cleanSettings, []byte(`{"permissions": {"allow": []}}`), 0644)
	code = run([]string{"snapshot", "--settings-path=" + cleanSettings}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "No unmanaged permissions found") {
		t.Errorf("snapshot clean failed, code=%d, out=%s", code, stdout.String())
	}
}
