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
		ModelProfile:        "argon-flash",
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
