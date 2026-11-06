package diff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cooper-Wallace/agentkit/internal/catalog"
)

func TestDiffBaselineMatch(t *testing.T) {
	tempDir := t.TempDir()
	permCat, err := catalog.LoadEmbeddedPermissions()
	if err != nil {
		t.Fatal(err)
	}
	flatPerms := catalog.GetFlatStandardPermissions(permCat, tempDir)

	settingsFile := filepath.Join(tempDir, "settings.json")
	settingsData := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": flatPerms,
		},
	}
	sb, _ := json.Marshal(settingsData)
	_ = os.WriteFile(settingsFile, sb, 0644)

	mcpFile := filepath.Join(tempDir, "mcp.json")
	mcpCat, _ := catalog.LoadEmbeddedMCP()
	mcpData := map[string]interface{}{
		"mcpServers": mcpCat.MCPServers,
	}
	mb, _ := json.Marshal(mcpData)
	_ = os.WriteFile(mcpFile, mb, 0644)

	res, err := Diff(DiffOptions{
		SettingsPath: settingsFile,
		MCPPath:      mcpFile,
		HomeDir:      tempDir,
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if res.HasDiff {
		t.Errorf("expected no diff, got missing: %v, custom: %v", res.MissingStandard, res.CustomPermissions)
	}

	formatted := FormatDiff(res)
	if !strings.Contains(formatted, "perfectly match") {
		t.Errorf("expected 'perfectly match' in output, got: %s", formatted)
	}
}

func TestDiffWithVariations(t *testing.T) {
	tempDir := t.TempDir()
	settingsFile := filepath.Join(tempDir, "settings.json")
	settingsData := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{"command(ls)", "command(custom_tool)"},
		},
	}
	sb, _ := json.Marshal(settingsData)
	_ = os.WriteFile(settingsFile, sb, 0644)

	mcpFile := filepath.Join(tempDir, "mcp.json")
	mcpData := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"custom_mcp": map[string]interface{}{"command": "tool"},
		},
	}
	mb, _ := json.Marshal(mcpData)
	_ = os.WriteFile(mcpFile, mb, 0644)

	res, err := Diff(DiffOptions{
		SettingsPath: settingsFile,
		MCPPath:      mcpFile,
		HomeDir:      tempDir,
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if !res.HasDiff {
		t.Errorf("expected HasDiff true")
	}
	if len(res.MissingStandard) == 0 {
		t.Errorf("expected missing standard permissions")
	}

	foundCustom := false
	for _, p := range res.CustomPermissions {
		if p == "command(custom_tool)" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Errorf("expected 'command(custom_tool)' in custom permissions: %v", res.CustomPermissions)
	}

	foundCustomMCP := false
	for _, m := range res.CustomMCPServers {
		if m == "custom_mcp" {
			foundCustomMCP = true
			break
		}
	}
	if !foundCustomMCP {
		t.Errorf("expected 'custom_mcp' in custom mcp servers: %v", res.CustomMCPServers)
	}

	formatted := FormatDiff(res)
	if !strings.Contains(formatted, "Missing Standard Permissions") || !strings.Contains(formatted, "Custom / Machine-Specific Permissions") {
		t.Errorf("unexpected formatted output: %s", formatted)
	}

	// JSON format check
	jsonBytes, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal DiffResult to JSON failed: %v", err)
	}
	var roundTrip DiffResult
	if err := json.Unmarshal(jsonBytes, &roundTrip); err != nil {
		t.Fatalf("unmarshal DiffResult JSON failed: %v", err)
	}
	if roundTrip.SettingsPath != settingsFile {
		t.Errorf("json round trip mismatch")
	}
}

func TestDiffErrors(t *testing.T) {
	// Missing file
	_, err := Diff(DiffOptions{
		SettingsPath: "/nonexistent/settings.json",
	})
	if err == nil {
		t.Errorf("expected error for non-existent settings file")
	}

	// Invalid json
	tempDir := t.TempDir()
	badFile := filepath.Join(tempDir, "bad.json")
	_ = os.WriteFile(badFile, []byte("{bad-json"), 0644)
	_, err = Diff(DiffOptions{
		SettingsPath: badFile,
	})
	if err == nil {
		t.Errorf("expected error for malformed json")
	}
}
