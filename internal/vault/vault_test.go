package vault

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveVaultPath(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Explicit path parameter takes precedence
	explicit := filepath.Join(tempDir, "ExplicitVault")
	t.Setenv("AGENTKIT_VAULT_PATH", filepath.Join(tempDir, "EnvVault1"))
	t.Setenv("AGENTKIT_AGENTVAULT_PATH", filepath.Join(tempDir, "EnvVault2"))
	t.Setenv("OBSIDIAN_VAULT_PATH", filepath.Join(tempDir, "EnvVault3"))

	res := ResolveVaultPath(explicit)
	if res != explicit {
		t.Fatalf("expected explicit path %q, got %q", explicit, res)
	}

	// 2. AGENTKIT_VAULT_PATH
	res = ResolveVaultPath("")
	expectedEnv1 := filepath.Join(tempDir, "EnvVault1")
	if res != expectedEnv1 {
		t.Fatalf("expected AGENTKIT_VAULT_PATH %q, got %q", expectedEnv1, res)
	}

	// 3. AGENTKIT_AGENTVAULT_PATH fallback
	t.Setenv("AGENTKIT_VAULT_PATH", "")
	res = ResolveVaultPath("")
	expectedEnv2 := filepath.Join(tempDir, "EnvVault2")
	if res != expectedEnv2 {
		t.Fatalf("expected AGENTKIT_AGENTVAULT_PATH %q, got %q", expectedEnv2, res)
	}

	// 4. OBSIDIAN_VAULT_PATH fallback
	t.Setenv("AGENTKIT_AGENTVAULT_PATH", "")
	res = ResolveVaultPath("")
	expectedEnv3 := filepath.Join(tempDir, "EnvVault3")
	if res != expectedEnv3 {
		t.Fatalf("expected OBSIDIAN_VAULT_PATH %q, got %q", expectedEnv3, res)
	}

	// 5. Default resolution
	t.Setenv("OBSIDIAN_VAULT_PATH", "")
	home, _ := os.UserHomeDir()
	expectedDefault := filepath.Join(home, "Documents", "AgentVault")
	res = ResolveVaultPath("")
	if res != expectedDefault {
		// Or home/AgentVault if exists
		alt := filepath.Join(home, "AgentVault")
		if res != alt {
			t.Fatalf("expected default %q or %q, got %q", expectedDefault, alt, res)
		}
	}
}

func TestVaultInit(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "TestVault")
	settingsPath := filepath.Join(tempDir, "test_settings.json")

	// First init
	err := InitWithSettings(vaultPath, settingsPath)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 1. Verify directory tree
	dirs := []string{
		filepath.Join(vaultPath, ".obsidian"),
		filepath.Join(vaultPath, "00-Inbox"),
		filepath.Join(vaultPath, "User-Input", "pending"),
		filepath.Join(vaultPath, "User-Input", "processed"),
		filepath.Join(vaultPath, "Projects"),
		filepath.Join(vaultPath, "Architecture", "decisions"),
		filepath.Join(vaultPath, "Sessions"),
	}
	for _, d := range dirs {
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			t.Errorf("expected directory %q to exist", d)
		}
	}

	// 2. Verify gitignore
	gitignoreBytes, err := os.ReadFile(filepath.Join(vaultPath, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	gitignore := string(gitignoreBytes)
	if !strings.Contains(gitignore, ".obsidian/workspace") || !strings.Contains(gitignore, ".trash/") {
		t.Errorf("unexpected .gitignore contents: %s", gitignore)
	}

	// 3. Verify notes
	expectedNotes := []string{
		"Dashboard.md",
		filepath.Join("User-Input", "README.md"),
		filepath.Join("Architecture", "00-Overview.md"),
		filepath.Join("Architecture", "contracts.md"),
		filepath.Join("Architecture", "decisions", "README.md"),
		filepath.Join("Sessions", "README.md"),
		filepath.Join("Projects", "agentkit.md"),
		filepath.Join("Projects", "repokit.md"),
		filepath.Join("Projects", "riced-kubuntu-os.md"),
		filepath.Join("Projects", "workstation-setup.md"),
		filepath.Join("Projects", "augurs-bazaar.md"),
		filepath.Join("Projects", "augurs-saber.md"),
	}
	for _, note := range expectedNotes {
		fullPath := filepath.Join(vaultPath, note)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("expected note %q to exist", note)
			continue
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\n") {
			t.Errorf("note %q missing YAML frontmatter opening", note)
		}
		if !strings.Contains(content, "updated_by: agentkit") {
			t.Errorf("note %q missing updated_by", note)
		}
	}

	// 4. Verify settings trustedWorkspaces
	settingsBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		t.Fatalf("failed to parse settings JSON: %v", err)
	}
	wsList, ok := settings["trustedWorkspaces"].([]interface{})
	if !ok {
		t.Fatalf("trustedWorkspaces missing from settings")
	}
	foundVault := false
	for _, item := range wsList {
		if item == vaultPath {
			foundVault = true
			break
		}
	}
	if !foundVault {
		t.Errorf("vaultPath %q not registered in trustedWorkspaces: %v", vaultPath, wsList)
	}

	// 5. Test idempotency & customization preservation
	agentkitMD := filepath.Join(vaultPath, "Projects", "agentkit.md")
	customContent := "- [x] Custom preserved task\n"
	_ = os.WriteFile(agentkitMD, append(gitignoreBytes, []byte(customContent)...), 0644)

	err = InitWithSettings(vaultPath, settingsPath)
	if err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
	persisted, _ := os.ReadFile(agentkitMD)
	if !strings.Contains(string(persisted), "Custom preserved task") {
		t.Error("expected custom task to be preserved across idempotent Init calls")
	}
}

func TestVaultStatus(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "StatusVault")

	// Uninitialized status
	st, err := Status(vaultPath)
	if err == nil {
		t.Fatal("expected error for uninitialized vault status, got nil")
	}
	if st.Exists {
		t.Fatal("expected Exists = false for uninitialized vault")
	}
	if !strings.Contains(st.Error, "Obsidian AgentVault does not exist") {
		t.Errorf("unexpected error message: %q", st.Error)
	}

	// Initialize vault
	if err := Init(vaultPath); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	st, err = Status(vaultPath)
	if err != nil {
		t.Fatalf("Status failed on initialized vault: %v", err)
	}
	if !st.Exists {
		t.Fatal("expected Exists = true")
	}
	if st.TotalNotes < 12 {
		t.Errorf("expected at least 12 total notes, got %d", st.TotalNotes)
	}
	if st.ActiveProjects != 6 {
		t.Errorf("expected 6 active projects, got %d", st.ActiveProjects)
	}
	if st.PendingNotes != 0 {
		t.Errorf("expected 0 pending notes initially, got %d", st.PendingNotes)
	}

	// Add a pending note
	pendingFile := filepath.Join(vaultPath, "User-Input", "pending", "feature-req.md")
	_ = os.WriteFile(pendingFile, []byte("# Feature Request\nAdd vault sync.\n"), 0644)

	st, err = Status(vaultPath)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if st.PendingNotes != 1 {
		t.Fatalf("expected 1 pending note, got %d", st.PendingNotes)
	}
	if len(st.PendingFiles) != 1 || st.PendingFiles[0] != "feature-req.md" {
		t.Errorf("unexpected pending files list: %v", st.PendingFiles)
	}

	// Add a session note
	sessionFile := filepath.Join(vaultPath, "Sessions", "handoff-2026-11-06.md")
	_ = os.WriteFile(sessionFile, []byte("# Session Handoff\nSummary.\n"), 0644)

	st, err = Status(vaultPath)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if st.RecentSession != "handoff-2026-11-06.md" {
		t.Errorf("expected RecentSession 'handoff-2026-11-06.md', got %q", st.RecentSession)
	}
}

func TestVaultIngest(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "IngestVault")
	_ = Init(vaultPath)

	// Ingest with uninitialized/invalid path
	_, err := Ingest(filepath.Join(tempDir, "NonExistent"), "", false)
	if err == nil {
		t.Fatal("expected error ingesting from nonexistent vault, got nil")
	}

	// 1. Ingest pending note
	pendingFile := filepath.Join(vaultPath, "User-Input", "pending", "offline-task.md")
	content := `---
tags:
  - directive
project: agentkit
---
# Offline Task
Implement vault ingest in Go.
`
	_ = os.WriteFile(pendingFile, []byte(content), 0644)

	// Dry run test
	dryRes, err := Ingest(vaultPath, "", true)
	if err != nil {
		t.Fatalf("dry-run Ingest failed: %v", err)
	}
	if dryRes.IngestedCount != 1 {
		t.Fatalf("expected 1 item in dry run, got %d", dryRes.IngestedCount)
	}
	if _, err := os.Stat(pendingFile); os.IsNotExist(err) {
		t.Fatal("dry run should not delete pending file")
	}

	// Real ingest
	res, err := Ingest(vaultPath, "", false)
	if err != nil {
		t.Fatalf("real Ingest failed: %v", err)
	}
	if res.IngestedCount != 1 {
		t.Fatalf("expected 1 ingested note, got %d", res.IngestedCount)
	}
	item := res.Items[0]
	if item.Project != "agentkit" {
		t.Errorf("expected project 'agentkit', got %q", item.Project)
	}
	if !strings.Contains(item.Content, "Implement vault ingest in Go.") {
		t.Errorf("unexpected item content: %s", item.Content)
	}

	// Pending file must be removed
	if _, err := os.Stat(pendingFile); !os.IsNotExist(err) {
		t.Fatal("pending file should have been deleted after ingest")
	}

	// Processed file must exist with audit stamp
	processedEntries, _ := os.ReadDir(filepath.Join(vaultPath, "User-Input", "processed"))
	foundProcessed := false
	for _, pe := range processedEntries {
		if strings.Contains(pe.Name(), "offline-task.md") {
			foundProcessed = true
			pData, _ := os.ReadFile(filepath.Join(vaultPath, "User-Input", "processed", pe.Name()))
			if !strings.Contains(string(pData), "> [!check] Ingested by agentkit on") {
				t.Errorf("missing audit stamp in processed note: %s", string(pData))
			}
		}
	}
	if !foundProcessed {
		t.Fatal("processed note not found in User-Input/processed/")
	}

	// 2. Ingest from project note section
	vaultPath2 := filepath.Join(tempDir, "IngestVault2")
	_ = Init(vaultPath2)
	agentkitMD := filepath.Join(vaultPath2, "Projects", "agentkit.md")
	agentkitBytes, _ := os.ReadFile(agentkitMD)
	notesToAdd := "\nPlease add automated backup rotation for vault notes.\n"
	_ = os.WriteFile(agentkitMD, append(agentkitBytes, []byte(notesToAdd)...), 0644)

	resProject, err := Ingest(vaultPath2, "agentkit", false)
	if err != nil {
		t.Fatalf("project Ingest failed: %v", err)
	}
	if resProject.IngestedCount != 1 {
		t.Fatalf("expected 1 ingested inline note, got %d", resProject.IngestedCount)
	}
	if !strings.Contains(resProject.Items[0].Content, "automated backup rotation") {
		t.Errorf("unexpected content: %s", resProject.Items[0].Content)
	}

	// Verify audit stamp in agentkit.md
	persisted, _ := os.ReadFile(agentkitMD)
	if !strings.Contains(string(persisted), "> [!check] Ingested by agentkit on") {
		t.Errorf("missing audit stamp in agentkit.md: %s", string(persisted))
	}

	// Re-run ingest: 0 pending items
	resEmpty, err := Ingest(vaultPath2, "agentkit", false)
	if err != nil {
		t.Fatalf("empty Ingest failed: %v", err)
	}
	if resEmpty.IngestedCount != 0 {
		t.Fatalf("expected 0 ingested items on second run, got %d", resEmpty.IngestedCount)
	}
}

func TestVaultNote(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "NoteVault")
	_ = Init(vaultPath)

	// 1. Create structured note
	res, err := Note(NoteOptions{
		VaultPath: vaultPath,
		Project:   "agentkit",
		Title:     "Subagent Memory Architecture",
		Content:   "Details on persistent cross-session knowledge graph.",
		Tags:      []string{"architecture", "memory"},
	})
	if err != nil {
		t.Fatalf("Note creation failed: %v", err)
	}
	if !res.Success || res.Action != "created" {
		t.Fatalf("expected action 'created', got %q", res.Action)
	}

	notePath := filepath.Join(vaultPath, "Projects", "Subagent-Memory-Architecture.md")
	contentBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("failed to read created note: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "tags:") || !strings.Contains(content, "memory") {
		t.Errorf("missing tags in note: %s", content)
	}
	if !strings.Contains(content, "[[Projects/agentkit|agentkit]]") {
		t.Errorf("missing project wikilink: %s", content)
	}

	// Verify backlink in Projects/agentkit.md
	agentkitMD := filepath.Join(vaultPath, "Projects", "agentkit.md")
	akData, _ := os.ReadFile(agentkitMD)
	if !strings.Contains(string(akData), "[[Projects/Subagent-Memory-Architecture|Subagent Memory Architecture]]") {
		t.Errorf("missing backlink in agentkit.md: %s", string(akData))
	}

	// 2. Append to existing note
	resAppend, err := Note(NoteOptions{
		VaultPath: vaultPath,
		Project:   "agentkit",
		Title:     "Subagent Memory Architecture",
		Content:   "Follow-up entry on vector indexing.",
	})
	if err != nil {
		t.Fatalf("Note append failed: %v", err)
	}
	if resAppend.Action != "appended" {
		t.Fatalf("expected action 'appended', got %q", resAppend.Action)
	}
	updatedNote, _ := os.ReadFile(notePath)
	if !strings.Contains(string(updatedNote), "Follow-up entry on vector indexing.") {
		t.Errorf("appended content missing: %s", string(updatedNote))
	}

	// 3. Append section to project note
	resSection, err := Note(NoteOptions{
		VaultPath: vaultPath,
		Project:   "agentkit",
		Title:     "Ignored Title",
		Section:   "Sprint Retrospective",
		Content:   "Sprint 4 completed ahead of schedule.",
	})
	if err != nil {
		t.Fatalf("Note section append failed: %v", err)
	}
	if resSection.Action != "appended_section" {
		t.Fatalf("expected action 'appended_section', got %q", resSection.Action)
	}
	akAfterSection, _ := os.ReadFile(agentkitMD)
	if !strings.Contains(string(akAfterSection), "## Sprint Retrospective") ||
		!strings.Contains(string(akAfterSection), "Sprint 4 completed ahead of schedule.") {
		t.Errorf("section content missing in agentkit.md: %s", string(akAfterSection))
	}

	// Error when vault path is invalid
	_, err = Note(NoteOptions{
		VaultPath: filepath.Join(tempDir, "NonExistent"),
		Project:   "test",
		Title:     "Test",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent vault, got nil")
	}
}

func TestVaultSyncEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "SyncVault")
	_ = Init(vaultPath)

	// 1. Sync links new ADRs
	adrPath := filepath.Join(vaultPath, "Architecture", "decisions", "0010-obsidian-vault-integration.md")
	_ = os.WriteFile(adrPath, []byte("# ADR 0010\nObsidian Vault.\n"), 0644)

	res, err := Sync(vaultPath, "agentkit", false, false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if len(res.Synced) != 1 || res.Synced[0] != "agentkit" {
		t.Fatalf("expected synced [agentkit], got %v", res.Synced)
	}
	akData, _ := os.ReadFile(filepath.Join(vaultPath, "Projects", "agentkit.md"))
	if !strings.Contains(string(akData), "[[Architecture/decisions/0010-obsidian-vault-integration|0010-obsidian-vault-integration]]") {
		t.Errorf("missing ADR link in agentkit.md: %s", string(akData))
	}

	// 2. Missing project template auto-scaffolding
	resCustom, err := Sync(vaultPath, "custom-service", false, false)
	if err != nil {
		t.Fatalf("Sync custom-service failed: %v", err)
	}
	if len(resCustom.Synced) != 1 || resCustom.Synced[0] != "custom-service" {
		t.Fatalf("expected synced [custom-service], got %v", resCustom.Synced)
	}
	customMD := filepath.Join(vaultPath, "Projects", "custom-service.md")
	customBytes, err := os.ReadFile(customMD)
	if err != nil {
		t.Fatalf("failed to read created custom-service.md: %v", err)
	}
	if !strings.Contains(string(customBytes), "# custom-service") {
		t.Errorf("custom-service template mismatch: %s", string(customBytes))
	}

	// 3. Dry-run mode
	dryRes, err := Sync(vaultPath, "agentkit", true, false)
	if err != nil {
		t.Fatalf("dry-run Sync failed: %v", err)
	}
	if !dryRes.DryRun {
		t.Fatal("expected DryRun = true")
	}

	// 4. Git integration: local_only mode (no remote)
	runGitCmd(t, vaultPath, "init", "-b", "main")
	runGitCmd(t, vaultPath, "config", "user.email", "test@agentkit.internal")
	runGitCmd(t, vaultPath, "config", "user.name", "AgentKit Test")
	runGitCmd(t, vaultPath, "add", "-A")
	runGitCmd(t, vaultPath, "commit", "-m", "chore: initial commit")

	// Make dirty
	_ = os.WriteFile(filepath.Join(vaultPath, "Sessions", "handoff-local.md"), []byte("# Handoff\n"), 0644)

	resLocal, err := Sync(vaultPath, "agentkit", false, false)
	if err != nil {
		t.Fatalf("local sync failed: %v", err)
	}
	if resLocal.GitStatus != "local_only" || resLocal.GitRemoteSynced {
		t.Errorf("expected git_status 'local_only', remote_synced false; got %q, %v", resLocal.GitStatus, resLocal.GitRemoteSynced)
	}

	// 5. Git integration: no-push flag with bare remote configured
	bareRemote := filepath.Join(tempDir, "bare_remote.git")
	runGitCmd(t, tempDir, "init", "--bare", "-b", "main", bareRemote)
	runGitCmd(t, vaultPath, "remote", "add", "origin", bareRemote)
	runGitCmd(t, vaultPath, "push", "-u", "origin", "main")

	// Make dirty
	_ = os.WriteFile(filepath.Join(vaultPath, "Sessions", "handoff-no-push.md"), []byte("# No Push Note\n"), 0644)

	resNoPush, err := Sync(vaultPath, "agentkit", false, true)
	if err != nil {
		t.Fatalf("no-push sync failed: %v", err)
	}
	if resNoPush.GitStatus != "no_push" || resNoPush.GitRemoteSynced {
		t.Errorf("expected git_status 'no_push', remote_synced false; got %q, %v", resNoPush.GitStatus, resNoPush.GitRemoteSynced)
	}

	// 6. Git integration: remote push success
	_ = os.WriteFile(filepath.Join(vaultPath, "Sessions", "handoff-push-success.md"), []byte("# Push Success\n"), 0644)
	resSuccess, err := Sync(vaultPath, "agentkit", false, false)
	if err != nil {
		t.Fatalf("push success sync failed: %v", err)
	}
	if resSuccess.GitStatus != "synced" || !resSuccess.GitRemoteSynced {
		t.Errorf("expected git_status 'synced', remote_synced true; got %q, %v", resSuccess.GitStatus, resSuccess.GitRemoteSynced)
	}

	// Verify commit reached bare remote
	remoteLog := runGitCmd(t, bareRemote, "log", "-n", "1", "--oneline")
	if !strings.Contains(remoteLog, "chore(vault): sync project state and session notes") {
		t.Errorf("remote repo missing synced commit: %s", remoteLog)
	}

	// 7. Git integration: remote push failure graceful degrade
	runGitCmd(t, vaultPath, "remote", "set-url", "origin", "file:///non/existent/path/repo.git")
	_ = os.WriteFile(filepath.Join(vaultPath, "Sessions", "handoff-fail.md"), []byte("# Push Fail\n"), 0644)

	resFail, err := Sync(vaultPath, "agentkit", false, false)
	if err != nil {
		t.Fatalf("push failure should not return Go error (must degrade gracefully): %v", err)
	}
	if resFail.GitStatus != "push_failed" || resFail.GitRemoteSynced {
		t.Errorf("expected git_status 'push_failed', remote_synced false; got %q, %v", resFail.GitStatus, resFail.GitRemoteSynced)
	}

	// Invalid vault path error
	_, err = Sync(filepath.Join(tempDir, "NonExistent"), "", false, false)
	if err == nil {
		t.Fatal("expected error syncing nonexistent vault, got nil")
	}
}

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %s (%v)", args, dir, string(out), err)
	}
	return string(out)
}
