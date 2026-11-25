package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldAgents(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "my-service")

	err := Scaffold(Options{
		Type:        "agents",
		TargetDir:   target,
		ProjectName: "my-service",
		Description: "A test microservice.",
		Language:    "Go",
		TestCommand: "go test ./...",
	})
	if err != nil {
		t.Fatalf("Scaffold agents failed: %v", err)
	}

	primary := filepath.Join(target, "AGENTS.md")
	content, err := os.ReadFile(primary)
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(content), "my-service") {
		t.Errorf("content missing project name: %s", string(content))
	}
	if !strings.Contains(string(content), "go test ./...") {
		t.Errorf("content missing test command: %s", string(content))
	}

	secondary := filepath.Join(target, "GEMINI.md")
	linkTarget, err := os.Readlink(secondary)
	if err != nil {
		t.Fatalf("failed to read symlink GEMINI.md: %v", err)
	}
	if linkTarget != "AGENTS.md" {
		t.Errorf("expected link target 'AGENTS.md', got %q", linkTarget)
	}

	// Try without force -> should fail
	err = Scaffold(Options{
		Type:      "agents",
		TargetDir: target,
	})
	if err == nil {
		t.Errorf("expected error when file already exists without force")
	}

	// With force -> should succeed
	err = Scaffold(Options{
		Type:      "agents",
		TargetDir: target,
		Force:     true,
	})
	if err != nil {
		t.Errorf("expected success with force, got %v", err)
	}
}

func TestScaffoldGemini(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "legacy-service")

	err := Scaffold(Options{
		Type:        "gemini",
		TargetDir:   target,
		ProjectName: "legacy-service",
	})
	if err != nil {
		t.Fatalf("Scaffold gemini failed: %v", err)
	}

	primary := filepath.Join(target, "GEMINI.md")
	if _, err := os.Stat(primary); err != nil {
		t.Errorf("expected GEMINI.md to exist")
	}

	secondary := filepath.Join(target, "AGENTS.md")
	linkTarget, err := os.Readlink(secondary)
	if err != nil || linkTarget != "GEMINI.md" {
		t.Errorf("expected symlink AGENTS.md -> GEMINI.md, got %q, err=%v", linkTarget, err)
	}
}

func TestScaffoldProjectGemini(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "proj-gemini")

	err := Scaffold(Options{
		Type:      "project-gemini",
		TargetDir: target,
	})
	if err != nil {
		t.Fatalf("Scaffold project-gemini failed: %v", err)
	}

	primary := filepath.Join(target, "project-GEMINI.md")
	if _, err := os.Stat(primary); err != nil {
		t.Errorf("expected project-GEMINI.md to exist")
	}
}

func TestScaffoldHook(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "hook-test")

	err := Scaffold(Options{
		Type:      "hook",
		TargetDir: target,
	})
	if err != nil {
		t.Fatalf("Scaffold hook failed: %v", err)
	}

	hookFile := filepath.Join(target, "hooks.json")
	content, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}
	if !strings.Contains(string(content), "command-safety-guard") {
		t.Errorf("unexpected hook content: %s", string(content))
	}

	// Without force -> should fail
	err = Scaffold(Options{
		Type:      "hook",
		TargetDir: target,
	})
	if err == nil {
		t.Errorf("expected error when hook already exists without force")
	}

	// Custom template source
	customSrc := filepath.Join(tempDir, "custom-hook.json")
	_ = os.WriteFile(customSrc, []byte(`{"my-hook": {"enabled": true}}`), 0644)
	err = Scaffold(Options{
		Type:        "hook",
		TargetDir:   target,
		Force:       true,
		TemplateSrc: customSrc,
	})
	if err != nil {
		t.Fatalf("expected success with custom template src: %v", err)
	}
	c, _ := os.ReadFile(hookFile)
	if !strings.Contains(string(c), "my-hook") {
		t.Errorf("expected custom hook content, got %s", string(c))
	}
}

func TestScaffoldErrors(t *testing.T) {
	tempDir := t.TempDir()
	err := Scaffold(Options{
		Type:      "invalid-type",
		TargetDir: tempDir,
	})
	if err == nil {
		t.Errorf("expected error for unknown template type")
	}

	// Bad custom template source
	err = Scaffold(Options{
		Type:        "hook",
		TargetDir:   tempDir,
		Force:       true,
		TemplateSrc: "/nonexistent/template.json",
	})
	if err == nil {
		t.Errorf("expected error for non-existent template source")
	}
}

func TestScaffoldAgentsWithCustomTemplateSrc(t *testing.T) {
	tempDir := t.TempDir()
	customSrc := filepath.Join(tempDir, "custom-agents.md")
	_ = os.WriteFile(customSrc, []byte("custom agents: {{PROJECT_NAME}}"), 0644)
	err := Scaffold(Options{
		Type:        "agents",
		TargetDir:   tempDir,
		ProjectName: "my-custom-proj",
		TemplateSrc: customSrc,
		Force:       true,
	})
	if err != nil {
		t.Fatalf("Scaffold with custom template src failed: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(tempDir, "AGENTS.md"))
	if !strings.Contains(string(content), "custom agents: my-custom-proj") {
		t.Errorf("unexpected content: %s", string(content))
	}
}

func TestScaffoldHookInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	badHook := filepath.Join(tempDir, "bad-hook.json")
	_ = os.WriteFile(badHook, []byte("{invalid-json"), 0644)
	err := Scaffold(Options{
		Type:        "hook",
		TargetDir:   tempDir,
		TemplateSrc: badHook,
		Force:       true,
	})
	if err == nil {
		t.Errorf("expected error for invalid json in hook template")
	}
}

func TestScaffoldDefaultTargetDir(t *testing.T) {
	// targetDir empty defaults to "."
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origDir) }()

	err := Scaffold(Options{
		Type:        "agents",
		ProjectName: "local-proj",
	})
	if err != nil {
		t.Fatalf("Scaffold with empty targetDir failed: %v", err)
	}
	if _, err := os.Stat("AGENTS.md"); err != nil {
		t.Errorf("expected AGENTS.md in current directory")
	}
}

func TestScaffoldEvidence(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "evidence-proj")

	err := Scaffold(Options{
		Type:        "evidence",
		TargetDir:   target,
		ProjectName: "evidence-proj",
		Description: "An evidence-backed service.",
		Language:    "Go",
		TestCommand: "go test -race ./...",
	})
	if err != nil {
		t.Fatalf("Scaffold evidence failed: %v", err)
	}

	expectedFiles := []string{
		"intent.md",
		"design.md",
		"agentic-workflow.md",
		"verify.md",
		"operate.md",
		"decisions.md",
	}
	evidenceDir := filepath.Join(target, "evidence")
	for _, name := range expectedFiles {
		path := filepath.Join(evidenceDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected evidence file %s to exist: %v", name, err)
		}
		text := string(data)
		if !strings.Contains(text, "evidence-proj") {
			t.Errorf("evidence file %s missing project name: %s", name, text)
		}
		if strings.Contains(text, "{{") {
			t.Errorf("evidence file %s contains unreplaced placeholder: %s", name, text)
		}
	}

	// Verify specific placeholders substituted across files
	intentData, _ := os.ReadFile(filepath.Join(evidenceDir, "intent.md"))
	if !strings.Contains(string(intentData), "An evidence-backed service.") {
		t.Errorf("intent.md missing description: %s", string(intentData))
	}
	verifyData, _ := os.ReadFile(filepath.Join(evidenceDir, "verify.md"))
	if !strings.Contains(string(verifyData), "go test -race ./...") || !strings.Contains(string(verifyData), "Go") {
		t.Errorf("verify.md missing language or test command: %s", string(verifyData))
	}

	// Guarded failure: existing file must not be overwritten without --force
	sentinel := "DO NOT OVERWRITE"
	sentinelPath := filepath.Join(evidenceDir, "decisions.md")
	if err := os.WriteFile(sentinelPath, []byte(sentinel), 0644); err != nil {
		t.Fatalf("write sentinel failed: %v", err)
	}
	err = Scaffold(Options{
		Type:        "evidence",
		TargetDir:   target,
		ProjectName: "should-fail",
	})
	if err == nil {
		t.Errorf("expected error when evidence file already exists without force")
	}
	afterFail, _ := os.ReadFile(sentinelPath)
	if string(afterFail) != sentinel {
		t.Errorf("existing file was overwritten despite missing --force: got %q", string(afterFail))
	}

	// Overwrite with --force
	err = Scaffold(Options{
		Type:        "evidence",
		TargetDir:   target,
		ProjectName: "forced-proj",
		Force:       true,
	})
	if err != nil {
		t.Fatalf("expected success with --force, got %v", err)
	}
	afterForce, _ := os.ReadFile(sentinelPath)
	if !strings.Contains(string(afterForce), "forced-proj") {
		t.Errorf("expected overwritten file to contain forced-proj, got %s", string(afterForce))
	}

	// Custom TemplateSrc directory
	customDir := filepath.Join(tempDir, "custom-evidence")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatalf("mkdir customDir failed: %v", err)
	}
	for _, name := range expectedFiles {
		body := "CUSTOM " + name + " | {{PROJECT_NAME}} | {{PROJECT_DESCRIPTION}} | {{PRIMARY_LANGUAGE}} | {{TEST_COMMAND}}"
		if err := os.WriteFile(filepath.Join(customDir, name), []byte(body), 0644); err != nil {
			t.Fatalf("write custom template %s failed: %v", name, err)
		}
	}
	customTarget := filepath.Join(tempDir, "custom-target")
	err = Scaffold(Options{
		Type:        "evidence",
		TargetDir:   customTarget,
		ProjectName: "custom-app",
		Description: "Custom desc",
		Language:    "Rust",
		TestCommand: "cargo test",
		TemplateSrc: customDir,
	})
	if err != nil {
		t.Fatalf("Scaffold evidence with custom TemplateSrc failed: %v", err)
	}
	for _, name := range expectedFiles {
		data, err := os.ReadFile(filepath.Join(customTarget, "evidence", name))
		if err != nil {
			t.Fatalf("read custom output %s failed: %v", name, err)
		}
		want := "CUSTOM " + name + " | custom-app | Custom desc | Rust | cargo test"
		if string(data) != want {
			t.Errorf("custom %s = %q, want %q", name, string(data), want)
		}
	}

	// Guarded error: invalid TemplateSrc (non-existent or non-dir)
	err = Scaffold(Options{
		Type:        "evidence",
		TargetDir:   filepath.Join(tempDir, "bad-src-target"),
		TemplateSrc: filepath.Join(tempDir, "does-not-exist"),
	})
	if err == nil {
		t.Errorf("expected error for non-existent evidence TemplateSrc")
	}

	notADir := filepath.Join(tempDir, "file.md")
	_ = os.WriteFile(notADir, []byte("hi"), 0644)
	err = Scaffold(Options{
		Type:        "evidence",
		TargetDir:   filepath.Join(tempDir, "file-src-target"),
		TemplateSrc: notADir,
	})
	if err == nil {
		t.Errorf("expected error when evidence TemplateSrc is a file instead of directory")
	}
}
