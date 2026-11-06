package diff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Cooper-Wallace/agentkit/internal/catalog"
)

// DiffResult holds the differences between settings/mcp and the repository baseline.
type DiffResult struct {
	SettingsPath      string   `json:"settings_path"`
	MCPPath           string   `json:"mcp_path,omitempty"`
	MissingStandard   []string `json:"missing_permissions"`
	CustomPermissions []string `json:"custom_permissions"`
	MissingMCPServers []string `json:"missing_mcp_servers,omitempty"`
	CustomMCPServers  []string `json:"custom_mcp_servers,omitempty"`
	HasDiff           bool     `json:"has_diff"`
}

// DiffOptions specifies parameters for diffing.
type DiffOptions struct {
	SettingsPath string
	MCPPath      string
	NoMCP        bool
	HomeDir      string
}

// Diff computes the differences between target settings/mcp and the baseline catalogs.
func Diff(opts DiffOptions) (*DiffResult, error) {
	homeDir := opts.HomeDir
	if homeDir == "" {
		if h, err := os.UserHomeDir(); err == nil {
			homeDir = h
		}
	}

	settingsPath := opts.SettingsPath
	if settingsPath == "" {
		settingsPath = filepath.Join(homeDir, ".gemini", "antigravity-cli", "settings.json")
	}

	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("target settings file not found: %s", settingsPath)
	}

	var userSettings map[string]interface{}
	if err := json.Unmarshal(settingsData, &userSettings); err != nil {
		return nil, fmt.Errorf("parse settings file: %w", err)
	}

	permCat, err := catalog.LoadEmbeddedPermissions()
	if err != nil {
		return nil, fmt.Errorf("load permissions catalog: %w", err)
	}
	standardPerms := catalog.GetFlatStandardPermissions(permCat, homeDir)
	stdSet := make(map[string]bool)
	for _, p := range standardPerms {
		stdSet[p] = true
	}

	userAllow := make(map[string]bool)
	if pMap, ok := userSettings["permissions"].(map[string]interface{}); ok {
		if allowList, ok := pMap["allow"].([]interface{}); ok {
			for _, item := range allowList {
				if s, ok := item.(string); ok {
					userAllow[s] = true
				}
			}
		}
	}

	var missingStandard []string
	for _, p := range standardPerms {
		if !userAllow[p] {
			missingStandard = append(missingStandard, p)
		}
	}
	sort.Strings(missingStandard)

	var customPermissions []string
	for p := range userAllow {
		if !stdSet[p] {
			customPermissions = append(customPermissions, p)
		}
	}
	sort.Strings(customPermissions)

	// MCP diff
	var missingMCP []string
	var customMCP []string
	mcpPath := opts.MCPPath
	if !opts.NoMCP {
		if mcpPath == "" {
			mcpPath = filepath.Join(homeDir, ".gemini", "config", "mcp_config.json")
		}
		mcpCat, err := catalog.LoadEmbeddedMCP()
		if err == nil {
			stdMCPServers := make(map[string]bool)
			for s := range mcpCat.MCPServers {
				stdMCPServers[s] = true
			}

			userMCPServers := make(map[string]bool)
			if mcpData, err := os.ReadFile(mcpPath); err == nil {
				var userMCP map[string]interface{}
				if err := json.Unmarshal(mcpData, &userMCP); err == nil {
					if srvMap, ok := userMCP["mcpServers"].(map[string]interface{}); ok {
						for s := range srvMap {
							userMCPServers[s] = true
						}
					}
				}
			}

			for s := range stdMCPServers {
				if !userMCPServers[s] {
					missingMCP = append(missingMCP, s)
				}
			}
			sort.Strings(missingMCP)

			for s := range userMCPServers {
				if !stdMCPServers[s] {
					customMCP = append(customMCP, s)
				}
			}
			sort.Strings(customMCP)
		}
	}

	hasDiff := len(missingStandard) > 0 || len(customPermissions) > 0 || len(missingMCP) > 0 || len(customMCP) > 0

	return &DiffResult{
		SettingsPath:      settingsPath,
		MCPPath:           mcpPath,
		MissingStandard:   missingStandard,
		CustomPermissions: customPermissions,
		MissingMCPServers: missingMCP,
		CustomMCPServers:  customMCP,
		HasDiff:           hasDiff,
	}, nil
}

// FormatDiff formats DiffResult for terminal output.
func FormatDiff(res *DiffResult) string {
	if !res.HasDiff {
		return "Local permissions perfectly match the repository baseline.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Diffing Settings against Baseline: %s\n", res.SettingsPath))

	if len(res.MissingStandard) > 0 {
		sb.WriteString(fmt.Sprintf("Missing Standard Permissions (%d permissions to be added):\n", len(res.MissingStandard)))
		for _, p := range res.MissingStandard {
			sb.WriteString(fmt.Sprintf("  - %s\n", p))
		}
	}

	if len(res.CustomPermissions) > 0 {
		sb.WriteString(fmt.Sprintf("Custom / Machine-Specific Permissions (%d permissions):\n", len(res.CustomPermissions)))
		for _, p := range res.CustomPermissions {
			sb.WriteString(fmt.Sprintf("  + %s\n", p))
		}
	}

	if len(res.MissingMCPServers) > 0 {
		sb.WriteString(fmt.Sprintf("Missing Standard MCP Servers (%d):\n", len(res.MissingMCPServers)))
		for _, s := range res.MissingMCPServers {
			sb.WriteString(fmt.Sprintf("  - %s\n", s))
		}
	}

	if len(res.CustomMCPServers) > 0 {
		sb.WriteString(fmt.Sprintf("Custom / Machine-Specific MCP Servers (%d):\n", len(res.CustomMCPServers)))
		for _, s := range res.CustomMCPServers {
			sb.WriteString(fmt.Sprintf("  + %s\n", s))
		}
	}

	return sb.String()
}
