package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cooper-Wallace/agentkit/internal/apply"
)

func setupStandardEnvironment(t *testing.T) (string, AuditOptions) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	geminiDir := filepath.Join(homeDir, ".gemini")
	settingsPath := filepath.Join(geminiDir, "antigravity-cli", "settings.json")
	mcpPath := filepath.Join(geminiDir, "config", "mcp_config.json")
	orchPath := filepath.Join(geminiDir, "config", "orchestration.json")
	pluginsDir := filepath.Join(geminiDir, "config", "plugins")

	// Set PATH to include home/.local/bin
	localBin := filepath.Join(homeDir, ".local", "bin")
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", localBin+":"+origPath)

	// Create dummy directives
	directivesSrc := filepath.Join(tempDir, "AGENTS.md")
	_ = os.WriteFile(directivesSrc, []byte("# Directives"), 0644)

	// Create dummy plugins source with executable scripts
	pluginsSrc := filepath.Join(tempDir, "plugins_src")
	for _, p := range []string{"autonomous-orchestrator", "quota-guardian", "wake-lock"} {
		pDir := filepath.Join(pluginsSrc, p)
		pScripts := filepath.Join(pDir, "scripts")
		_ = os.MkdirAll(pScripts, 0755)
		_ = os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(`{"name":"`+p+`"}`), 0644)
		script := filepath.Join(pScripts, "hook.sh")
		_ = os.WriteFile(script, []byte("#!/bin/bash"), 0755)
	}

	// Create dummy skills source
	skillsSrc := filepath.Join(tempDir, "skills_src")
	_ = os.MkdirAll(skillsSrc, 0755)

	opts := AuditOptions{
		SettingsPath:        settingsPath,
		MCPPath:             mcpPath,
		GeminiDir:           geminiDir,
		OrchestrationPath:   orchPath,
		PluginsDir:          pluginsDir,
		GlobalDirectivesDir: geminiDir,
		ProjectSkillsDir:    skillsSrc,
		HomeDir:             homeDir,
	}

	// Apply everything cleanly
	_, err := apply.Apply(apply.ApplyOptions{
		SettingsPath:        settingsPath,
		MCPPath:             mcpPath,
		GeminiDir:           geminiDir,
		OrchestrationPath:   orchPath,
		PluginsDir:          pluginsDir,
		PluginsSrcDir:       pluginsSrc,
		GlobalDirectivesSrc: directivesSrc,
		ProjectSkillsDir:    skillsSrc,
		ModelProfile:        "carbon-flash",
		HomeDir:             homeDir,
	})
	if err != nil {
		t.Fatalf("setup apply failed: %v", err)
	}

	return origPath, opts
}

func TestAuditCompliant(t *testing.T) {
	origPath, opts := setupStandardEnvironment(t)
	defer os.Setenv("PATH", origPath)

	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Audit Run failed: %v", err)
	}

	if res.TotalChecks != 14 {
		t.Errorf("expected 14 checks, got %d", res.TotalChecks)
	}
	if !res.Compliant {
		t.Errorf("expected Compliant true, got %d/%d passed", res.PassedChecks, res.TotalChecks)
		for _, c := range res.Checks {
			if !c.Passed {
				t.Logf("Failed check: %s (%s)", c.Name, c.Message)
			}
		}
	}
	if res.ComplianceScore != 100 {
		t.Errorf("expected compliance score 100, got %d", res.ComplianceScore)
	}

	formatted := FormatAudit(res)
	if !strings.Contains(formatted, "[SUCCESS]") {
		t.Errorf("expected [SUCCESS] in formatted audit: %s", formatted)
	}
}

func TestAuditNonCompliant(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	geminiDir := filepath.Join(homeDir, ".gemini")

	opts := AuditOptions{
		SettingsPath: filepath.Join(geminiDir, "settings.json"),
		MCPPath:      filepath.Join(geminiDir, "mcp.json"),
		GeminiDir:    geminiDir,
		HomeDir:      homeDir,
	}

	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Audit Run failed: %v", err)
	}

	if res.TotalChecks != 14 {
		t.Errorf("expected 14 checks, got %d", res.TotalChecks)
	}
	if res.Compliant {
		t.Errorf("expected Compliant false for empty environment")
	}

	formatted := FormatAudit(res)
	if !strings.Contains(formatted, "[NON-COMPLIANT]") {
		t.Errorf("expected [NON-COMPLIANT] in formatted audit: %s", formatted)
	}

	// JSON check
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal audit json failed: %v", err)
	}
	var roundTrip AuditResult
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal audit json failed: %v", err)
	}
	if roundTrip.TotalChecks != 14 {
		t.Errorf("round trip total checks mismatch: %d", roundTrip.TotalChecks)
	}
}

func TestAuditFixFlag(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	geminiDir := filepath.Join(homeDir, ".gemini")
	settingsFile := filepath.Join(geminiDir, "antigravity-cli", "settings.json")
	mcpFile := filepath.Join(geminiDir, "config", "mcp_config.json")
	orchFile := filepath.Join(geminiDir, "config", "orchestration.json")
	pluginsDir := filepath.Join(geminiDir, "config", "plugins")

	// Set PATH to include local bin
	localBin := filepath.Join(homeDir, ".local", "bin")
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", localBin+":"+origPath)
	defer os.Setenv("PATH", origPath)

	// Pre-create directive file at geminiDir/AGENTS.md
	_ = os.MkdirAll(geminiDir, 0755)
	_ = os.WriteFile(filepath.Join(geminiDir, "AGENTS.md"), []byte("# Directives"), 0644)

	// Pre-create dummy plugins
	for _, p := range []string{"autonomous-orchestrator", "quota-guardian", "wake-lock"} {
		pDir := filepath.Join(pluginsDir, p)
		_ = os.MkdirAll(filepath.Join(pDir, "scripts"), 0755)
		_ = os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(`{"name":"`+p+`"}`), 0644)
	}

	// Run audit with Fix: true on incomplete environment
	opts := AuditOptions{
		SettingsPath:      settingsFile,
		MCPPath:           mcpFile,
		GeminiDir:         geminiDir,
		OrchestrationPath: orchFile,
		PluginsDir:        pluginsDir,
		HomeDir:           homeDir,
		Fix:               true,
	}

	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run with Fix failed: %v", err)
	}
	if res.PassedChecks <= 6 {
		t.Errorf("expected fix to increase passed checks, got %d", res.PassedChecks)
	}
}

func TestAuditProjectDirectives(t *testing.T) {
	origPath, baseOpts := setupStandardEnvironment(t)
	defer os.Setenv("PATH", origPath)

	// 1. Happy path: concise AGENTS.md (<= 80 lines) -> 15 checks, compliant
	happyDir := filepath.Join(t.TempDir(), "happy-proj")
	if err := os.MkdirAll(happyDir, 0755); err != nil {
		t.Fatalf("mkdir happyDir failed: %v", err)
	}
	conciseContent := strings.Repeat("# Line\n", 40)
	if err := os.WriteFile(filepath.Join(happyDir, "AGENTS.md"), []byte(conciseContent), 0644); err != nil {
		t.Fatalf("write concise AGENTS.md failed: %v", err)
	}

	happyOpts := baseOpts
	happyOpts.ProjectDir = happyDir
	res, err := Run(happyOpts)
	if err != nil {
		t.Fatalf("Audit Run (happy project dir) failed: %v", err)
	}
	if res.TotalChecks != 15 {
		t.Errorf("expected 15 checks, got %d", res.TotalChecks)
	}
	if !res.Compliant {
		t.Errorf("expected Compliant true, got %d/%d", res.PassedChecks, res.TotalChecks)
	}
	last := res.Checks[len(res.Checks)-1]
	if last.ID != 15 || last.Name != "project_directives_progressive_disclosure" || last.Status != "pass" || !last.Passed {
		t.Errorf("unexpected check 15 result: %+v", last)
	}

	// GEMINI.md fallback check
	geminiOnlyDir := filepath.Join(t.TempDir(), "gemini-fallback")
	_ = os.MkdirAll(geminiOnlyDir, 0755)
	_ = os.WriteFile(filepath.Join(geminiOnlyDir, "GEMINI.md"), []byte("# Concise Gemini\n"), 0644)
	fallbackCheck := CheckProjectDirectives(geminiOnlyDir)
	if fallbackCheck.Status != "pass" || !fallbackCheck.Passed {
		t.Errorf("expected GEMINI.md fallback to pass, got %+v", fallbackCheck)
	}

	// 2. Guarded warning: AGENTS.md > 120 lines -> warn, non-compliant
	warnDir := filepath.Join(t.TempDir(), "warn-proj")
	if err := os.MkdirAll(warnDir, 0755); err != nil {
		t.Fatalf("mkdir warnDir failed: %v", err)
	}
	bloatedContent := strings.Repeat("# Bloated directive line\n", MaxProjectAgentsLines+5)
	if err := os.WriteFile(filepath.Join(warnDir, "AGENTS.md"), []byte(bloatedContent), 0644); err != nil {
		t.Fatalf("write bloated AGENTS.md failed: %v", err)
	}

	warnOpts := baseOpts
	warnOpts.ProjectDir = warnDir
	res, err = Run(warnOpts)
	if err != nil {
		t.Fatalf("Audit Run (warn project dir) failed: %v", err)
	}
	if res.TotalChecks != 15 {
		t.Errorf("expected 15 checks, got %d", res.TotalChecks)
	}
	if res.Compliant {
		t.Errorf("expected Compliant false when AGENTS.md exceeds %d lines", MaxProjectAgentsLines)
	}
	last = res.Checks[len(res.Checks)-1]
	if last.Status != "warn" || last.Passed {
		t.Errorf("expected warn & Passed=false, got %+v", last)
	}
	if !strings.Contains(last.Message, "progressive disclosure") {
		t.Errorf("expected progressive disclosure recommendation in message, got %q", last.Message)
	}

	// 3. Guarded failure: missing AGENTS.md and GEMINI.md -> fail, non-compliant
	missingDir := filepath.Join(t.TempDir(), "missing-proj")
	if err := os.MkdirAll(missingDir, 0755); err != nil {
		t.Fatalf("mkdir missingDir failed: %v", err)
	}

	missingOpts := baseOpts
	missingOpts.ProjectDir = missingDir
	res, err = Run(missingOpts)
	if err != nil {
		t.Fatalf("Audit Run (missing project dir) failed: %v", err)
	}
	if res.TotalChecks != 15 {
		t.Errorf("expected 15 checks, got %d", res.TotalChecks)
	}
	if res.Compliant {
		t.Errorf("expected Compliant false when AGENTS.md is missing")
	}
	last = res.Checks[len(res.Checks)-1]
	if last.Status != "fail" || last.Passed {
		t.Errorf("expected fail & Passed=false, got %+v", last)
	}
}
