package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmbeddedCatalogs(t *testing.T) {
	// Permissions
	permCat, err := LoadEmbeddedPermissions()
	if err != nil {
		t.Fatalf("LoadEmbeddedPermissions failed: %v", err)
	}
	if len(permCat.Categories) == 0 {
		t.Errorf("expected categories in permissions catalog")
	}
	flatPerms := GetFlatStandardPermissions(permCat, "/mock/home")
	if len(flatPerms) == 0 {
		t.Errorf("expected flattened permissions, got empty")
	}

	// MCP
	mcpCat, err := LoadEmbeddedMCP()
	if err != nil {
		t.Fatalf("LoadEmbeddedMCP failed: %v", err)
	}
	if _, ok := mcpCat.MCPServers["github"]; !ok {
		t.Errorf("expected 'github' in embedded mcpServers")
	}

	// Profiles
	profCat, err := LoadEmbeddedProfiles()
	if err != nil {
		t.Fatalf("LoadEmbeddedProfiles failed: %v", err)
	}
	if profCat.DefaultProfile == "" {
		t.Errorf("expected default_profile in profiles catalog")
	}
	if _, ok := profCat.Profiles["argon-flash"]; !ok {
		t.Errorf("expected 'argon-flash' in profiles catalog")
	}

	// Quota
	quotaCat, err := LoadEmbeddedQuota()
	if err != nil {
		t.Fatalf("LoadEmbeddedQuota failed: %v", err)
	}
	if quotaCat.Thresholds.SwitchThreshold <= 0 {
		t.Errorf("expected positive switch_threshold")
	}

	// Settings Defaults
	settingsCat, err := LoadEmbeddedSettingsDefaults()
	if err != nil {
		t.Fatalf("LoadEmbeddedSettingsDefaults failed: %v", err)
	}
	if !settingsCat.AllowNonWorkspaceAccess {
		t.Errorf("expected allowNonWorkspaceAccess true")
	}
}

func TestCatalogCustomOverrides(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Custom Permissions
	permPath := filepath.Join(tempDir, "custom_perms.json")
	permJSON := `{
		"categories": {
			"custom": {
				"commands": ["my_cmd"]
			}
		},
		"mcp_tools": {
			"mcp1": ["mcp:read"]
		},
		"tools": {
			"t1": { "permissions": ["tool:run"] }
		},
		"file_access": {
			"fa1": ["read_file(${HOME}/test)"]
		}
	}`
	if err := os.WriteFile(permPath, []byte(permJSON), 0644); err != nil {
		t.Fatal(err)
	}
	pCat, err := LoadPermissionsFile(permPath)
	if err != nil {
		t.Fatalf("LoadPermissionsFile failed: %v", err)
	}
	flat := GetFlatStandardPermissions(pCat, "/custom/home")
	expected := []string{"command(my_cmd)", "mcp:read", "tool:run", "read_file(/custom/home/test)"}
	if len(flat) != len(expected) {
		t.Fatalf("expected %d perms, got %d", len(expected), len(flat))
	}
	for i, exp := range expected {
		if flat[i] != exp {
			t.Errorf("perm[%d] expected %q, got %q", i, exp, flat[i])
		}
	}

	// 2. Custom MCP
	mcpPath := filepath.Join(tempDir, "custom_mcp.json")
	mcpJSON := `{
		"mcpServers": {
			"custom": {
				"command": "custom-cmd",
				"args": ["--arg1"]
			}
		}
	}`
	if err := os.WriteFile(mcpPath, []byte(mcpJSON), 0644); err != nil {
		t.Fatal(err)
	}
	mCat, err := LoadMCPFile(mcpPath)
	if err != nil {
		t.Fatalf("LoadMCPFile failed: %v", err)
	}
	if _, ok := mCat.MCPServers["custom"]; !ok {
		t.Errorf("expected 'custom' in mCat.MCPServers")
	}

	// 3. Custom Profiles
	profPath := filepath.Join(tempDir, "custom_profiles.json")
	profJSON := `{
		"default_profile": "test-prof",
		"profiles": {
			"test-prof": {
				"description": "test",
				"root_model": "model-1",
				"worker_model": "model-2",
				"root_slug": "test"
			}
		}
	}`
	if err := os.WriteFile(profPath, []byte(profJSON), 0644); err != nil {
		t.Fatal(err)
	}
	prCat, err := LoadProfilesFile(profPath)
	if err != nil {
		t.Fatalf("LoadProfilesFile failed: %v", err)
	}
	if prCat.DefaultProfile != "test-prof" {
		t.Errorf("expected default_profile 'test-prof', got %q", prCat.DefaultProfile)
	}

	// 4. Custom Quota
	quotaPath := filepath.Join(tempDir, "custom_quota.json")
	quotaJSON := `{
		"thresholds": {
			"switch_threshold": 0.20,
			"wrapup_threshold": 0.10,
			"cache_ttl_seconds": 30
		},
		"complexity_to_model": {
			"low": { "model": "m_lite", "group": "gemini" }
		},
		"fallbacks": {
			"m_lite": ["inherit"]
		}
	}`
	if err := os.WriteFile(quotaPath, []byte(quotaJSON), 0644); err != nil {
		t.Fatal(err)
	}
	qCat, err := LoadQuotaFile(quotaPath)
	if err != nil {
		t.Fatalf("LoadQuotaFile failed: %v", err)
	}
	if qCat.Thresholds.SwitchThreshold != 0.20 {
		t.Errorf("expected switch_threshold 0.20, got %f", qCat.Thresholds.SwitchThreshold)
	}

	// 5. Custom Settings Defaults
	setPath := filepath.Join(tempDir, "custom_settings.json")
	setJSON := `{
		"allowNonWorkspaceAccess": false,
		"colorScheme": "light",
		"model": "custom-model",
		"trustedWorkspaces": ["/workspace"]
	}`
	if err := os.WriteFile(setPath, []byte(setJSON), 0644); err != nil {
		t.Fatal(err)
	}
	sCat, err := LoadSettingsDefaultsFile(setPath)
	if err != nil {
		t.Fatalf("LoadSettingsDefaultsFile failed: %v", err)
	}
	if sCat.AllowNonWorkspaceAccess {
		t.Errorf("expected allowNonWorkspaceAccess false")
	}
	if sCat.ColorScheme != "light" {
		t.Errorf("expected colorScheme 'light', got %q", sCat.ColorScheme)
	}
}

func TestCatalogErrorHandling(t *testing.T) {
	// Missing files
	if _, err := LoadPermissionsFile("/nonexistent/file.json"); err == nil {
		t.Errorf("expected error for missing permissions file")
	}
	if _, err := LoadMCPFile("/nonexistent/file.json"); err == nil {
		t.Errorf("expected error for missing mcp file")
	}
	if _, err := LoadProfilesFile("/nonexistent/file.json"); err == nil {
		t.Errorf("expected error for missing profiles file")
	}
	if _, err := LoadQuotaFile("/nonexistent/file.json"); err == nil {
		t.Errorf("expected error for missing quota file")
	}
	if _, err := LoadSettingsDefaultsFile("/nonexistent/file.json"); err == nil {
		t.Errorf("expected error for missing settings defaults file")
	}

	// Invalid JSON
	tempDir := t.TempDir()
	badJSON := filepath.Join(tempDir, "bad.json")
	if err := os.WriteFile(badJSON, []byte("{invalid-json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPermissionsFile(badJSON); err == nil {
		t.Errorf("expected error for invalid json in permissions")
	}
	if _, err := LoadMCPFile(badJSON); err == nil {
		t.Errorf("expected error for invalid json in mcp")
	}
	if _, err := LoadProfilesFile(badJSON); err == nil {
		t.Errorf("expected error for invalid json in profiles")
	}
	if _, err := LoadQuotaFile(badJSON); err == nil {
		t.Errorf("expected error for invalid json in quota")
	}
	if _, err := LoadSettingsDefaultsFile(badJSON); err == nil {
		t.Errorf("expected error for invalid json in settings")
	}

	// Flatten nil
	if res := GetFlatStandardPermissions(nil, ""); res != nil {
		t.Errorf("expected nil for nil catalog, got %v", res)
	}
}
