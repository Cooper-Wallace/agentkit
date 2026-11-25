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
