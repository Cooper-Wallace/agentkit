package apply

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyHermetic(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	geminiDir := filepath.Join(homeDir, ".gemini")
	settingsFile := filepath.Join(geminiDir, "antigravity-cli", "settings.json")
	mcpFile := filepath.Join(geminiDir, "config", "mcp_config.json")
	orchFile := filepath.Join(geminiDir, "config", "orchestration.json")
	pluginsDir := filepath.Join(geminiDir, "config", "plugins")

	// Create dummy directives source
	directivesSrc := filepath.Join(tempDir, "source_AGENTS.md")
	_ = os.WriteFile(directivesSrc, []byte("# Directives"), 0644)

	// Create dummy plugins source
	pluginsSrc := filepath.Join(tempDir, "source_plugins")
	p1Dir := filepath.Join(pluginsSrc, "test-plugin")
	p1Scripts := filepath.Join(p1Dir, "scripts")
	_ = os.MkdirAll(p1Scripts, 0755)
	_ = os.WriteFile(filepath.Join(p1Dir, "plugin.json"), []byte(`{"name":"test-plugin"}`), 0644)
	_ = os.WriteFile(filepath.Join(p1Scripts, "run.sh"), []byte("#!/bin/bash\necho ok"), 0644)

	// Create dummy skills source
	skillsSrc := filepath.Join(tempDir, "source_skills")
	_ = os.MkdirAll(skillsSrc, 0755)

	// Pre-create settings with a custom permission
	_ = os.MkdirAll(filepath.Dir(settingsFile), 0755)
	initialSettings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{"command(my_custom_tool)"},
		},
	}
	sb, _ := json.Marshal(initialSettings)
	_ = os.WriteFile(settingsFile, sb, 0644)

	// Run Apply
	res, err := Apply(ApplyOptions{
		SettingsPath:        settingsFile,
		MCPPath:             mcpFile,
		GeminiDir:           geminiDir,
		OrchestrationPath:   orchFile,
		PluginsDir:          pluginsDir,
		PluginsSrcDir:       pluginsSrc,
		GlobalDirectivesSrc: directivesSrc,
		ProjectSkillsDir:    skillsSrc,
		ModelProfile:        "argon-flash",
		HomeDir:             homeDir,
		LinkHome:            true,
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected res.Success true")
	}

	// 1. Verify backup was created
	if res.BackupPath == "" {
		t.Errorf("expected backup path for existing settings file")
	} else if _, err := os.Stat(res.BackupPath); err != nil {
		t.Errorf("backup file does not exist: %v", err)
	}

	// 2. Verify settings file updated & custom permission preserved
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("read settings failed: %v", err)
	}
	var updated map[string]interface{}
	_ = json.Unmarshal(data, &updated)
	perms := updated["permissions"].(map[string]interface{})["allow"].([]interface{})
	hasCustom := false
	for _, p := range perms {
		if p == "command(my_custom_tool)" {
			hasCustom = true
			break
		}
	}
	if !hasCustom {
		t.Errorf("custom permission was erased!")
	}
	if updated["orchestrationProfile"] != "argon-flash" {
		t.Errorf("expected orchestrationProfile 'argon-flash', got %v", updated["orchestrationProfile"])
	}

	// 3. Verify directives symlinked
	agentsLink := filepath.Join(geminiDir, "AGENTS.md")
	target, err := os.Readlink(agentsLink)
	if err != nil || target != directivesSrc {
		t.Errorf("AGENTS.md symlink mismatch: target=%s, err=%v", target, err)
	}
	geminiLink := filepath.Join(geminiDir, "GEMINI.md")
	target2, err := os.Readlink(geminiLink)
	if err != nil || target2 != directivesSrc {
		t.Errorf("GEMINI.md symlink mismatch: target=%s, err=%v", target2, err)
	}

	// 4. Verify orchestration.json
	orchData, err := os.ReadFile(orchFile)
	if err != nil {
		t.Errorf("orchestration file missing: %v", err)
	}
	if !strings.Contains(string(orchData), "argon-flash") {
		t.Errorf("expected argon-flash in orchestration.json")
	}

	// 5. Verify MCP servers
	mcpData, err := os.ReadFile(mcpFile)
	if err != nil {
		t.Errorf("mcp file missing: %v", err)
	}
	var mcpMap map[string]interface{}
	_ = json.Unmarshal(mcpData, &mcpMap)
	srvs := mcpMap["mcpServers"].(map[string]interface{})
	if _, ok := srvs["github"]; !ok {
		t.Errorf("expected github server in mcpServers")
	}

	// 6. Verify plugins deployed
	destScript := filepath.Join(pluginsDir, "test-plugin", "scripts", "run.sh")
	info, err := os.Stat(destScript)
	if err != nil {
		t.Errorf("deployed plugin script missing: %v", err)
	} else if info.Mode()&0111 == 0 {
		t.Errorf("deployed plugin script is not executable: %v", info.Mode())
	}

	// 7. Verify skills.json
	skillsFile := filepath.Join(geminiDir, "config", "skills.json")
	skData, err := os.ReadFile(skillsFile)
	if err != nil {
		t.Errorf("skills file missing: %v", err)
	}
	if !strings.Contains(string(skData), skillsSrc) {
		t.Errorf("skills.json missing skillsSrc")
	}
}

func TestApplyDryRun(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	settingsFile := filepath.Join(homeDir, "settings.json")

	res, err := Apply(ApplyOptions{
		SettingsPath: settingsFile,
		DryRun:       true,
		HomeDir:      homeDir,
	})
	if err != nil {
		t.Fatalf("Apply dry-run failed: %v", err)
	}
	if !res.DryRun {
		t.Errorf("expected DryRun true in result")
	}
	if _, err := os.Stat(settingsFile); err == nil {
		t.Errorf("settings file was created in dry-run mode!")
	}
}

func TestApplyProfileSwitch(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	geminiDir := filepath.Join(homeDir, ".gemini")
	settingsFile := filepath.Join(geminiDir, "settings.json")
	orchFile := filepath.Join(geminiDir, "config", "orchestration.json")

	// Apply flash-high
	res, err := Apply(ApplyOptions{
		SettingsPath:      settingsFile,
		OrchestrationPath: orchFile,
		GeminiDir:         geminiDir,
		ModelProfile:      "flash-high",
		HomeDir:           homeDir,
		NoMCP:             true,
		NoPlugins:         true,
	})
	if err != nil {
		t.Fatalf("Apply flash-high failed: %v", err)
	}
	if res.ConfiguredProfile != "flash-high" {
		t.Errorf("expected configured profile 'flash-high', got %s", res.ConfiguredProfile)
	}

	data, _ := os.ReadFile(settingsFile)
	var sMap map[string]interface{}
	_ = json.Unmarshal(data, &sMap)
	if sMap["orchestrationProfile"] != "flash-high" {
		t.Errorf("expected orchestrationProfile 'flash-high', got %v", sMap["orchestrationProfile"])
	}
}

func TestInterpolateVar(t *testing.T) {
	home := "/home/testuser"
	os.Setenv("TEST_VAR_123", "secret_token")
	defer os.Unsetenv("TEST_VAR_123")

	origSQL := os.Getenv("SQLITE_DEV_DB")
	os.Unsetenv("SQLITE_DEV_DB")
	defer func() {
		if origSQL != "" {
			_ = os.Setenv("SQLITE_DEV_DB", origSQL)
		}
	}()

	res1 := interpolateVar("${TEST_VAR_123}", home)
	if res1 != "secret_token" {
		t.Errorf("expected secret_token, got %s", res1)
	}

	res2 := interpolateVar("${NONEXISTENT_VAR:-$HOME/default.db}", home)
	if res2 != "/home/testuser/default.db" {
		t.Errorf("expected /home/testuser/default.db, got %s", res2)
	}

	res3 := interpolateVar("${SQLITE_DEV_DB}", home)
	if res3 != "/home/testuser/dev.db" {
		t.Errorf("expected /home/testuser/dev.db, got %s", res3)
	}
}
