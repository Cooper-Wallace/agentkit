package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cooper-Wallace/agentkit/internal/apply"
	"github.com/Cooper-Wallace/agentkit/internal/catalog"
)

// AuditCheck represents a single compliance check.
type AuditCheck struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"` // "pass", "fail", "warn", "info"
	Message     string `json:"message"`
	Passed      bool   `json:"passed"`
}

// AuditResult summarizes all compliance check results.
type AuditResult struct {
	Harness         string       `json:"harness"`
	SettingsPath    string       `json:"settings_path"`
	TotalChecks     int          `json:"total_checks"`
	PassedChecks    int          `json:"passed_checks"`
	ComplianceScore int          `json:"compliance_score"`
	Compliant       bool         `json:"compliant"`
	Checks          []AuditCheck `json:"checks"`
}

// AuditOptions provides flags and path overrides for the audit command.
type AuditOptions struct {
	SettingsPath        string
	MCPPath             string
	GeminiDir           string
	OrchestrationPath   string
	PluginsDir          string
	GlobalDirectivesDir string
	ProjectSkillsDir    string
	HomeDir             string
	JSON                bool
	Fix                 bool
	Verbose             bool
}

// Run performs the 14 Antigravity compliance checks.
func Run(opts AuditOptions) (*AuditResult, error) {
	homeDir := opts.HomeDir
	if homeDir == "" {
		if h, err := os.UserHomeDir(); err == nil {
			homeDir = h
		}
	}

	geminiDir := opts.GeminiDir
	if geminiDir == "" {
		geminiDir = filepath.Join(homeDir, ".gemini")
	}

	settingsPath := opts.SettingsPath
	if settingsPath == "" {
		settingsPath = filepath.Join(geminiDir, "antigravity-cli", "settings.json")
	}

	mcpPath := opts.MCPPath
	if mcpPath == "" {
		mcpPath = filepath.Join(geminiDir, "config", "mcp_config.json")
	}

	pluginsDir := opts.PluginsDir
	if pluginsDir == "" {
		pluginsDir = filepath.Join(geminiDir, "config", "plugins")
	}

	orchPath := opts.OrchestrationPath
	if orchPath == "" {
		orchPath = filepath.Join(geminiDir, "config", "orchestration.json")
	}

	skillsPath := filepath.Join(geminiDir, "config", "skills.json")

	projectSkillsDir := opts.ProjectSkillsDir
	if projectSkillsDir == "" {
		candidates := []string{
			filepath.Join("project", "skills"),
			filepath.Join("skills"),
			filepath.Join(homeDir, "Projects", "agentkit", "project", "skills"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && info.IsDir() {
				projectSkillsDir, _ = filepath.Abs(c)
				break
			}
		}
	}

	permCat, err := catalog.LoadEmbeddedPermissions()
	if err != nil {
		return nil, fmt.Errorf("load permissions catalog: %w", err)
	}
	standardPermissions := catalog.GetFlatStandardPermissions(permCat, homeDir)
	stdPermSet := make(map[string]bool)
	for _, p := range standardPermissions {
		stdPermSet[p] = true
	}

	mcpCat, _ := catalog.LoadEmbeddedMCP()
	profilesCat, _ := catalog.LoadEmbeddedProfiles()

	// Load user settings if present
	var userSettings map[string]interface{}
	settingsExists := false
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &userSettings); err == nil {
			settingsExists = true
		}
	}

	userAllowSet := make(map[string]bool)
	if settingsExists {
		if pMap, ok := userSettings["permissions"].(map[string]interface{}); ok {
			if allowList, ok := pMap["allow"].([]interface{}); ok {
				for _, item := range allowList {
					if s, ok := item.(string); ok {
						userAllowSet[s] = true
					}
				}
			}
		}
	}

	var checks []AuditCheck
	passedCount := 0

	addCheck := func(id int, name, desc, status, msg string, passed bool) {
		checks = append(checks, AuditCheck{
			ID:          id,
			Name:        name,
			Description: desc,
			Status:      status,
			Message:     msg,
			Passed:      passed,
		})
		if passed {
			passedCount++
		}
	}

	// 1. Settings File Existence Check
	if settingsExists {
		addCheck(1, "settings_file_exists", "Settings file exists", "pass",
			fmt.Sprintf("Settings file exists (%s)", settingsPath), true)
	} else {
		addCheck(1, "settings_file_exists", "Settings file exists", "fail",
			fmt.Sprintf("Settings file missing (%s)", settingsPath), false)
	}

	// 2. Permissions Allow List
	var missingPerms []string
	var activePerms []string
	for _, p := range standardPermissions {
		if userAllowSet[p] {
			activePerms = append(activePerms, p)
		} else {
			missingPerms = append(missingPerms, p)
		}
	}
	if len(missingPerms) == 0 {
		addCheck(2, "standard_permissions_active", "Standard permissions active", "pass",
			fmt.Sprintf("Standard Permissions: All %d pre-approved permissions active (%d/%d active)",
				len(standardPermissions), len(activePerms), len(standardPermissions)), true)
	} else {
		addCheck(2, "standard_permissions_active", "Standard permissions active", "fail",
			fmt.Sprintf("Standard Permissions: %d of %d permissions missing (%d/%d active)",
				len(missingPerms), len(standardPermissions), len(activePerms), len(standardPermissions)), false)
	}

	// 3. Global Agent Directives Check
	directivesDir := opts.GlobalDirectivesDir
	if directivesDir == "" {
		directivesDir = geminiDir
	}
	globalAgents := filepath.Join(directivesDir, "AGENTS.md")
	globalGemini := filepath.Join(directivesDir, "GEMINI.md")
	if fileExists(globalAgents) || fileExists(globalGemini) {
		targetFound := globalAgents
		if !fileExists(globalAgents) {
			targetFound = globalGemini
		}
		addCheck(3, "global_directives_installed", "Global agent directives installed", "pass",
			fmt.Sprintf("Global Agent Directives installed (%s)", targetFound), true)
	} else {
		addCheck(3, "global_directives_installed", "Global agent directives installed", "fail",
			fmt.Sprintf("Global Agent Directives missing (%s)", globalAgents), false)
	}

	// 4. Non-workspace access setting
	if userSettings != nil && userSettings["allowNonWorkspaceAccess"] == true {
		addCheck(4, "workspace_boundary_enabled", "Workspace boundary setting", "pass",
			"Workspace Boundary: 'allowNonWorkspaceAccess' enabled", true)
	} else {
		addCheck(4, "workspace_boundary_enabled", "Workspace boundary setting", "warn",
			"Workspace Boundary: 'allowNonWorkspaceAccess' not explicitly enabled", false)
	}

	// 5. Trusted Workspaces Check
	var userWorkspaces []string
	if userSettings != nil {
		if wsList, ok := userSettings["trustedWorkspaces"].([]interface{}); ok {
			for _, item := range wsList {
				if s, ok := item.(string); ok {
					userWorkspaces = append(userWorkspaces, s)
				}
			}
		}
	}
	expectedVaultPaths := map[string]bool{
		filepath.Join(homeDir, "Documents", "AgentVault"): true,
		filepath.Join(homeDir, "AgentVault"):             true,
	}
	for _, envVar := range []string{"AGENTKIT_VAULT_PATH", "AGENTKIT_AGENTVAULT_PATH"} {
		if v := os.Getenv(envVar); v != "" {
			expectedVaultPaths[v] = true
		}
	}
	agentkitPath := filepath.Join(homeDir, "Projects", "agentkit")
	repokitPath := filepath.Join(homeDir, "Projects", "repokit")

	hasVault := false
	hasAgentkit := false
	hasRepokit := false
	for _, ws := range userWorkspaces {
		if expectedVaultPaths[ws] {
			hasVault = true
		}
		if ws == agentkitPath {
			hasAgentkit = true
		}
		if ws == repokitPath {
			hasRepokit = true
		}
	}

	if hasVault && hasAgentkit && hasRepokit {
		addCheck(5, "trusted_workspaces_whitelisted", "Trusted workspaces whitelisted", "pass",
			"Trusted Workspaces: AgentVault, agentkit, and repokit whitelisted", true)
	} else {
		var missingWS []string
		if !hasVault {
			missingWS = append(missingWS, fmt.Sprintf("AgentVault (%s)", filepath.Join(homeDir, "Documents", "AgentVault")))
		}
		if !hasAgentkit {
			missingWS = append(missingWS, fmt.Sprintf("agentkit (%s)", agentkitPath))
		}
		if !hasRepokit {
			missingWS = append(missingWS, fmt.Sprintf("repokit (%s)", repokitPath))
		}
		addCheck(5, "trusted_workspaces_whitelisted", "Trusted workspaces whitelisted", "warn",
			fmt.Sprintf("Trusted Workspaces: Required workspace path(s) missing from trustedWorkspaces (%s)",
				strings.Join(missingWS, ", ")), false)
	}

	// 6. Native File Access Permissions Check
	hasReadVault := false
	for vPath := range expectedVaultPaths {
		if userAllowSet[fmt.Sprintf("read_file(%s)", vPath)] {
			hasReadVault = true
			break
		}
	}
	hasReadAgentkit := userAllowSet[fmt.Sprintf("read_file(%s)", agentkitPath)]
	hasReadRepokit := userAllowSet[fmt.Sprintf("read_file(%s)", repokitPath)]

	if hasReadVault && hasReadAgentkit && hasReadRepokit {
		addCheck(6, "native_file_access_active", "Native file access permissions active", "pass",
			"Native File Access: AgentVault, agentkit, and repokit read permissions active", true)
	} else {
		var missingFP []string
		if !hasReadVault {
			missingFP = append(missingFP, fmt.Sprintf("AgentVault (read_file(%s))", filepath.Join(homeDir, "Documents", "AgentVault")))
		}
		if !hasReadAgentkit {
			missingFP = append(missingFP, fmt.Sprintf("agentkit (read_file(%s))", agentkitPath))
		}
		if !hasReadRepokit {
			missingFP = append(missingFP, fmt.Sprintf("repokit (read_file(%s))", repokitPath))
		}
		addCheck(6, "native_file_access_active", "Native file access permissions active", "warn",
			fmt.Sprintf("Native File Access: Required read_file permission(s) missing from permissions.allow (%s)",
				strings.Join(missingFP, ", ")), false)
	}

	// 7. Color Scheme Check (Always passes, informational if not dark)
	if userSettings != nil && userSettings["colorScheme"] == "dark" {
		addCheck(7, "ui_theme_configured", "UI Theme configured", "pass",
			"UI Theme: 'colorScheme' set to 'dark'", true)
	} else {
		scheme := "not set"
		if userSettings != nil && userSettings["colorScheme"] != nil {
			scheme = fmt.Sprintf("%v", userSettings["colorScheme"])
		}
		addCheck(7, "ui_theme_configured", "UI Theme configured", "info",
			fmt.Sprintf("UI Theme: 'colorScheme' is '%s'", scheme), true)
	}

	// 8. PATH verification for ~/.local/bin
	localBin := filepath.Join(homeDir, ".local", "bin")
	currentPath := os.Getenv("PATH")
	if strings.Contains(":"+currentPath+":", ":"+localBin+":") {
		addCheck(8, "env_path_includes_local_bin", "Environment PATH includes local bin", "pass",
			fmt.Sprintf("Environment PATH includes %s", localBin), true)
	} else {
		addCheck(8, "env_path_includes_local_bin", "Environment PATH includes local bin", "warn",
			fmt.Sprintf("Environment PATH missing %s", localBin), false)
	}

	// 9. MCP Server Standards Check
	stdMCPServers := []string{"github", "fetch", "repomix"}
	if mcpCat != nil && len(mcpCat.MCPServers) > 0 {
		stdMCPServers = nil
		for s := range mcpCat.MCPServers {
			stdMCPServers = append(stdMCPServers, s)
		}
	}

	var configuredMCPServers map[string]bool
	if mcpData, err := os.ReadFile(mcpPath); err == nil {
		var userMCP map[string]interface{}
		if err := json.Unmarshal(mcpData, &userMCP); err == nil {
			if srvMap, ok := userMCP["mcpServers"].(map[string]interface{}); ok {
				configuredMCPServers = make(map[string]bool)
				for s := range srvMap {
					configuredMCPServers[s] = true
				}
			}
		}
	}

	var missingMCP []string
	for _, s := range stdMCPServers {
		if !configuredMCPServers[s] {
			missingMCP = append(missingMCP, s)
		}
	}

	if len(missingMCP) == 0 {
		addCheck(9, "mcp_configuration_compliant", "MCP configuration compliant", "pass",
			fmt.Sprintf("MCP Configuration: All %d standard servers configured (%s)", len(stdMCPServers), mcpPath), true)
	} else {
		addCheck(9, "mcp_configuration_compliant", "MCP configuration compliant", "fail",
			fmt.Sprintf("MCP Configuration: %d of %d standard servers missing (%s)",
				len(missingMCP), len(stdMCPServers), strings.Join(missingMCP, ", ")), false)
	}

	// 10, 11, 12. Standard Plugins Audit
	standardPlugins := []string{"autonomous-orchestrator", "quota-guardian", "wake-lock"}
	for idx, pName := range standardPlugins {
		checkID := 10 + idx
		targetPlugin := filepath.Join(pluginsDir, pName)
		manifest := filepath.Join(targetPlugin, "plugin.json")
		if fileExists(manifest) {
			scriptsDir := filepath.Join(targetPlugin, "scripts")
			allExec := true
			if entries, err := os.ReadDir(scriptsDir); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						info, err := e.Info()
						if err == nil && info.Mode()&0111 == 0 {
							allExec = false
						}
					}
				}
			}
			if allExec {
				addCheck(checkID, fmt.Sprintf("plugin_%s_installed", strings.ReplaceAll(pName, "-", "_")),
					fmt.Sprintf("Plugin '%s' installed and operational", pName), "pass",
					fmt.Sprintf("Plugin '%s' installed and operational (%s)", pName, targetPlugin), true)
			} else {
				addCheck(checkID, fmt.Sprintf("plugin_%s_installed", strings.ReplaceAll(pName, "-", "_")),
					fmt.Sprintf("Plugin '%s' installed and operational", pName), "warn",
					fmt.Sprintf("Plugin '%s' scripts missing executable permission (+x)", pName), true)
			}
		} else {
			addCheck(checkID, fmt.Sprintf("plugin_%s_installed", strings.ReplaceAll(pName, "-", "_")),
				fmt.Sprintf("Plugin '%s' installed and operational", pName), "fail",
				fmt.Sprintf("Plugin '%s' missing (%s)", pName, targetPlugin), false)
		}
	}

	// 13. Orchestration Profile Audit
	var configuredProfile string
	if userSettings != nil {
		if p, ok := userSettings["orchestrationProfile"].(string); ok && p != "" {
			configuredProfile = p
		} else if oMap, ok := userSettings["orchestration"].(map[string]interface{}); ok {
			if p, ok := oMap["profile"].(string); ok && p != "" {
				configuredProfile = p
			}
		}
	}
	if configuredProfile == "" && fileExists(orchPath) {
		if oData, err := os.ReadFile(orchPath); err == nil {
			var oMap map[string]interface{}
			if err := json.Unmarshal(oData, &oMap); err == nil {
				if p, ok := oMap["active_profile"].(string); ok && p != "" {
					configuredProfile = p
				}
			}
		}
	}

	if configuredProfile != "" {
		profDesc := configuredProfile
		if profilesCat != nil {
			if pInfo, ok := profilesCat.Profiles[configuredProfile]; ok && pInfo.Description != "" {
				profDesc = pInfo.Description
			}
		}
		addCheck(13, "orchestration_profile_recorded", "Orchestration profile recorded", "pass",
			fmt.Sprintf("Orchestration Profile: '%s' active (%s)", configuredProfile, profDesc), true)
	} else {
		addCheck(13, "orchestration_profile_recorded", "Orchestration profile recorded", "warn",
			"Orchestration Profile: No profile explicitly recorded (default fallback: argon-flash)", false)
	}

	// 14. Global Skills Registry Audit
	skillsConfigured := false
	if fileExists(skillsPath) {
		if sData, err := os.ReadFile(skillsPath); err == nil {
			var sMap map[string]interface{}
			if err := json.Unmarshal(sData, &sMap); err == nil {
				if entries, ok := sMap["entries"].([]interface{}); ok {
					for _, e := range entries {
						if em, ok := e.(map[string]interface{}); ok {
							if em["path"] == projectSkillsDir {
								skillsConfigured = true
								break
							}
						}
					}
				}
			}
		}
	}

	if skillsConfigured {
		addCheck(14, "global_skills_registry_configured", "Global skills registry configured", "pass",
			fmt.Sprintf("Global Skills Registry: configured (%s)", skillsPath), true)
	} else {
		addCheck(14, "global_skills_registry_configured", "Global skills registry configured", "fail",
			fmt.Sprintf("Global Skills Registry: project skills missing from %s", skillsPath), false)
	}

	totalChecks := len(checks)
	score := 0
	if totalChecks > 0 {
		score = int((float64(passedCount) / float64(totalChecks)) * 100)
	}
	compliant := passedCount == totalChecks

	result := &AuditResult{
		Harness:         "antigravity",
		SettingsPath:    settingsPath,
		TotalChecks:     totalChecks,
		PassedChecks:    passedCount,
		ComplianceScore: score,
		Compliant:       compliant,
		Checks:          checks,
	}

	// If --fix was requested and not yet compliant, apply fixes and re-audit
	if opts.Fix && !compliant {
		applyOpts := apply.ApplyOptions{
			SettingsPath:        settingsPath,
			MCPPath:             mcpPath,
			GeminiDir:           geminiDir,
			OrchestrationPath:   orchPath,
			PluginsDir:          pluginsDir,
			ProjectSkillsDir:    projectSkillsDir,
			HomeDir:             homeDir,
			GlobalDirectivesSrc: filepath.Join(geminiDir, "AGENTS.md"),
		}
		if _, err := apply.Apply(applyOpts); err == nil {
			// Re-run without fix to prevent loop
			optsNoFix := opts
			optsNoFix.Fix = false
			return Run(optsNoFix)
		}
	}

	return result, nil
}

// FormatAudit formats the AuditResult matching bin/agentkit audit terminal output.
func FormatAudit(res *AuditResult) string {
	var sb strings.Builder
	sb.WriteString("\nAuditing Google Antigravity Standards via agentkit\n")
	sb.WriteString(fmt.Sprintf("Harness: %s\n", res.Harness))
	sb.WriteString(fmt.Sprintf("Target Settings: %s\n", res.SettingsPath))

	for _, c := range res.Checks {
		var symbol string
		switch c.Status {
		case "pass":
			symbol = "✔"
		case "fail":
			symbol = "✖"
		case "warn":
			symbol = "⚠"
		case "info":
			symbol = "ℹ"
		default:
			symbol = "•"
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", symbol, c.Message))
	}

	sb.WriteString("\n" + strings.Repeat("─", 60) + "\n")
	if res.Compliant {
		sb.WriteString(fmt.Sprintf("[SUCCESS] Google Antigravity configuration is fully compliant! (%d/%d checks passed)\n",
			res.PassedChecks, res.TotalChecks))
	} else {
		sb.WriteString(fmt.Sprintf("[NON-COMPLIANT] Compliance score: %d%% (%d/%d checks passed).\n",
			res.ComplianceScore, res.PassedChecks, res.TotalChecks))
		sb.WriteString("Run agentkit apply to bring your workstation into compliance.\n")
	}

	return sb.String()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
