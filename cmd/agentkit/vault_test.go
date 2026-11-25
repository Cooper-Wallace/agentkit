package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_VaultStatus(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "CLIVault")

	var stdout, stderr bytes.Buffer

	// 1. Uninitialized status (text)
	code := runVault([]string{"status", "--path", vaultPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for uninitialized vault status, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Obsidian AgentVault not found") {
		t.Errorf("unexpected output: %s", stdout.String())
	}

	// 2. Uninitialized status (json)
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"status", "--path", vaultPath, "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for uninitialized vault status json, got %d", code)
	}
	var errData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &errData); err != nil {
		t.Fatalf("failed to parse JSON status: %v", err)
	}
	if errData["exists"] != false {
		t.Errorf("expected exists: false, got %v", errData["exists"])
	}

	// 3. Init vault
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"init", "--path", vaultPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("vault init failed with code %d: %s", code, stderr.String())
	}

	// 4. Initialized status (text)
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"status", "--path", vaultPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("vault status failed: %d", code)
	}
	if !strings.Contains(stdout.String(), "Obsidian AgentVault Status") {
		t.Errorf("unexpected status output: %s", stdout.String())
	}

	// 5. Initialized status (json)
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"status", "--path", vaultPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("vault status json failed: %d", code)
	}
	var statusData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &statusData); err != nil {
		t.Fatalf("failed to parse JSON status: %v", err)
	}
	if statusData["exists"] != true {
		t.Errorf("expected exists: true, got %v", statusData["exists"])
	}
	if total, ok := statusData["total_notes"].(float64); !ok || total < 12 {
		t.Errorf("expected at least 12 notes, got %v", statusData["total_notes"])
	}
}

func TestCLI_VaultInit(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "InitVault")
	settingsPath := filepath.Join(tempDir, "settings.json")

	var stdout, stderr bytes.Buffer
	code := runVault([]string{"init", "--path", vaultPath, "--settings-path", settingsPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "AgentVault initialized successfully") {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}

	// Verify dashboard note
	dashPath := filepath.Join(vaultPath, "Dashboard.md")
	if _, err := os.Stat(dashPath); os.IsNotExist(err) {
		t.Fatalf("Dashboard.md missing at %s", dashPath)
	}

	// Verify trustedWorkspaces in settings.json
	settingsBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}
	if !strings.Contains(string(settingsBytes), vaultPath) {
		t.Errorf("vaultPath missing from settings.json: %s", string(settingsBytes))
	}
}

func TestCLI_VaultIngest(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "IngestVault")

	var stdout, stderr bytes.Buffer
	_ = runVault([]string{"init", "--path", vaultPath}, &stdout, &stderr)

	// Create pending file
	pendingPath := filepath.Join(vaultPath, "User-Input", "pending", "task-1.md")
	_ = os.WriteFile(pendingPath, []byte("---\nproject: agentkit\n---\n# Task 1\nDirectives.\n"), 0644)

	// Ingest (dry-run, json)
	stdout.Reset()
	stderr.Reset()
	code := runVault([]string{"ingest", "--path", vaultPath, "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run ingest failed with code %d", code)
	}
	var dryData map[string]interface{}
	_ = json.Unmarshal(stdout.Bytes(), &dryData)
	if count, ok := dryData["ingested_count"].(float64); !ok || count != 1 {
		t.Errorf("expected 1 ingested count, got %v", dryData["ingested_count"])
	}

	// Real ingest (human output)
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"ingest", "--path", vaultPath, "--project", "agentkit"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("real ingest failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Ingested 1 Pending Directive(s)") {
		t.Errorf("unexpected output: %s", stdout.String())
	}

	// Ingest when empty
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"ingest", "--path", vaultPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("empty ingest failed with code %d", code)
	}
	if !strings.Contains(stdout.String(), "No pending user inputs found.") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestCLI_VaultSync(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "SyncVault")

	var stdout, stderr bytes.Buffer
	_ = runVault([]string{"init", "--path", vaultPath}, &stdout, &stderr)

	// Add ADR
	adrPath := filepath.Join(vaultPath, "Architecture", "decisions", "0011-git-worktree.md")
	_ = os.WriteFile(adrPath, []byte("# ADR 0011\nGit worktrees.\n"), 0644)

	// Sync (json)
	stdout.Reset()
	stderr.Reset()
	code := runVault([]string{"sync", "--path", vaultPath, "--project", "agentkit", "--no-push", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("sync failed with code %d: %s", code, stderr.String())
	}
	var syncData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &syncData); err != nil {
		t.Fatalf("failed to parse sync JSON: %v", err)
	}
	syncedList, ok := syncData["synced"].([]interface{})
	if !ok || len(syncedList) != 1 || syncedList[0] != "agentkit" {
		t.Errorf("unexpected synced list: %v", syncData["synced"])
	}

	// Sync (text, dry-run)
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"sync", "--path", vaultPath, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run sync failed with code %d", code)
	}
	if !strings.Contains(stdout.String(), "[dry-run] Would sync") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestCLI_VaultNote(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "NoteVault")

	var stdout, stderr bytes.Buffer
	_ = runVault([]string{"init", "--path", vaultPath}, &stdout, &stderr)

	// 1. Create structured note (json)
	stdout.Reset()
	stderr.Reset()
	code := runVault([]string{
		"note",
		"--path", vaultPath,
		"--project", "agentkit",
		"--title", "Design Spec",
		"--content", "Specification details.",
		"--tag", "spec,v1",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("create note failed with code %d: %s", code, stderr.String())
	}
	var noteData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &noteData); err != nil {
		t.Fatalf("failed to parse note JSON: %v", err)
	}
	if noteData["action"] != "created" {
		t.Errorf("expected action 'created', got %v", noteData["action"])
	}

	// 2. Append note (text)
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{
		"note",
		"--path", vaultPath,
		"--project", "agentkit",
		"Design Spec",
		"--content", "Second entry.",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("append note failed with code %d", code)
	}
	if !strings.Contains(stdout.String(), "Appended content to existing note") {
		t.Errorf("unexpected output: %s", stdout.String())
	}

	// 3. Append section to project note
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{
		"note",
		"--path", vaultPath,
		"--project", "agentkit",
		"--section", "Roadmap 2027",
		"--content", "Autonomous DAG orchestration.",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("append section failed with code %d", code)
	}
	if !strings.Contains(stdout.String(), "Appended section 'Roadmap 2027'") {
		t.Errorf("unexpected output: %s", stdout.String())
	}

	// 4. Missing title and section error
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"note", "--path", vaultPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 when missing title and section, got %d", code)
	}
}

func TestCLI_VaultHelpAndInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Help
	code := runVault([]string{"help"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Usage: agentkit vault") {
		t.Fatalf("unexpected help response: code %d, output %s", code, stdout.String())
	}

	// No args
	stdout.Reset()
	code = runVault([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for no args, got %d", code)
	}

	// Unknown subcommand
	stdout.Reset()
	stderr.Reset()
	code = runVault([]string{"unknown-sub"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "Unknown vault subcommand") {
		t.Fatalf("expected error for unknown subcommand, got %d", code)
	}

	// Exported RunVault
	code = RunVault([]string{"help"})
	if code != 0 {
		t.Fatalf("RunVault help failed with code %d", code)
	}
}
