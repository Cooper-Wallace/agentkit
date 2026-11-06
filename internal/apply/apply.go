package apply

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Cooper-Wallace/agentkit/internal/catalog"
)

// ApplyOptions holds flags and path overrides for apply.
type ApplyOptions struct {
	SettingsPath        string
	MCPPath             string
	GeminiDir           string
	OrchestrationPath   string
	PluginsDir          string
	PluginsSrcDir       string
	GlobalDirectivesSrc string
	ProjectSkillsDir    string
	ModelProfile        string
	DryRun              bool
	NoBackup            bool
	NoMCP               bool
	NoPlugins           bool
	LinkHome            bool
	HomeDir             string
}

// ApplyResult contains summary of changes made by Apply.
type ApplyResult struct {
	SettingsPath      string   `json:"settings_path"`
	BackupPath        string   `json:"backup_path,omitempty"`
	MCPBackupPath     string   `json:"mcp_backup_path,omitempty"`
	AddedPermissions  []string `json:"added_permissions"`
	AddedMCPServers   []string `json:"added_mcp_servers,omitempty"`
	ConfiguredProfile string   `json:"configured_profile"`
	DryRun            bool     `json:"dry_run"`
	Success           bool     `json:"success"`
	Messages          []string `json:"messages"`
}

var varPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

func interpolateVar(val string, homeDir string) string {
	return varPattern.ReplaceAllStringFunc(val, func(m string) string {
		sub := varPattern.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		varName := sub[1]
		var defVal string
		hasDef := len(sub) > 2 && sub[2] != ""
		if hasDef {
			defVal = sub[2]
		}

		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}
		if hasDef {
			if strings.Contains(defVal, "$HOME") {
				defVal = strings.ReplaceAll(defVal, "$HOME", homeDir)
			} else if strings.Contains(defVal, "${HOME}") {
				defVal = strings.ReplaceAll(defVal, "${HOME}", homeDir)
			} else if strings.HasPrefix(defVal, "~") {
				defVal = filepath.Join(homeDir, defVal[1:])
			}
			return defVal
		}
		if varName == "SQLITE_DEV_DB" {
			return filepath.Join(homeDir, "dev.db")
		}
		return m
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err == nil {
		_ = os.Chmod(dst, info.Mode())
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := copyFile(path, target); err != nil {
			return err
		}
		if filepath.Base(filepath.Dir(path)) == "scripts" || filepath.Base(filepath.Dir(target)) == "scripts" {
			_ = os.Chmod(target, 0755)
		}
		return nil
	})
}

// Apply idempotently applies configurations to target machine.
func Apply(opts ApplyOptions) (*ApplyResult, error) {
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

	res := &ApplyResult{
		SettingsPath: settingsPath,
		DryRun:       opts.DryRun,
		Messages:     []string{},
	}

	// 1. Load catalogs
	permCat, err := catalog.LoadEmbeddedPermissions()
	if err != nil {
		return nil, fmt.Errorf("load permissions catalog: %w", err)
	}
	standardPermissions := catalog.GetFlatStandardPermissions(permCat, homeDir)

	settingsDefaults, _ := catalog.LoadEmbeddedSettingsDefaults()
	profilesCat, _ := catalog.LoadEmbeddedProfiles()

	modelProfile := opts.ModelProfile
	if modelProfile == "" {
		modelProfile = "argon-flash"
		if profilesCat != nil && profilesCat.DefaultProfile != "" {
			modelProfile = profilesCat.DefaultProfile
		}
	}
	res.ConfiguredProfile = modelProfile

	var profInfo catalog.ProfileInfo
	if profilesCat != nil {
		if p, ok := profilesCat.Profiles[modelProfile]; ok {
			profInfo = p
		}
	}

	// 2. Load existing settings or initialize from defaults
	existingSettings := make(map[string]interface{})
	settingsExists := false
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &existingSettings); err == nil {
			settingsExists = true
		}
	}

	if settingsDefaults != nil {
		if _, ok := existingSettings["allowNonWorkspaceAccess"]; !ok {
			existingSettings["allowNonWorkspaceAccess"] = settingsDefaults.AllowNonWorkspaceAccess
		}
		if _, ok := existingSettings["colorScheme"]; !ok {
			existingSettings["colorScheme"] = settingsDefaults.ColorScheme
		}
		if _, ok := existingSettings["model"]; !ok {
			existingSettings["model"] = settingsDefaults.Model
		}
	}

	// Permissions map
	permMap, ok := existingSettings["permissions"].(map[string]interface{})
	if !ok {
		permMap = make(map[string]interface{})
		existingSettings["permissions"] = permMap
	}

	currentAllowList, ok := permMap["allow"].([]interface{})
	if !ok {
		currentAllowList = []interface{}{}
	}

	currentAllowSet := make(map[string]bool)
	var mergedAllow []string
	for _, item := range currentAllowList {
		if s, ok := item.(string); ok {
			if !currentAllowSet[s] {
				currentAllowSet[s] = true
				mergedAllow = append(mergedAllow, s)
			}
		}
	}

	for _, p := range standardPermissions {
		if !currentAllowSet[p] {
			mergedAllow = append(mergedAllow, p)
			res.AddedPermissions = append(res.AddedPermissions, p)
			currentAllowSet[p] = true
		}
	}

	// Trusted workspaces
	var currentWorkspaces []string
	if wsList, ok := existingSettings["trustedWorkspaces"].([]interface{}); ok {
		for _, item := range wsList {
			if s, ok := item.(string); ok {
				currentWorkspaces = append(currentWorkspaces, s)
			}
		}
	}

	standardWorkspaces := []string{
		filepath.Join(homeDir, "Documents", "AgentVault"),
		filepath.Join(homeDir, "AgentVault"),
		filepath.Join(homeDir, "Projects", "agentkit"),
		filepath.Join(homeDir, "Projects", "repokit"),
	}
	for _, envVar := range []string{"AGENTKIT_VAULT_PATH", "AGENTKIT_AGENTVAULT_PATH"} {
		if val := os.Getenv(envVar); val != "" {
			standardWorkspaces = append(standardWorkspaces, val)
		}
	}

	seenWS := make(map[string]bool)
	var dedupedWS []string
	for _, ws := range currentWorkspaces {
		if !seenWS[ws] {
			seenWS[ws] = true
			dedupedWS = append(dedupedWS, ws)
		}
	}
	for _, ws := range standardWorkspaces {
		if !seenWS[ws] {
			seenWS[ws] = true
			dedupedWS = append(dedupedWS, ws)
		}
	}
	existingSettings["trustedWorkspaces"] = dedupedWS

	// Native file access permissions for standard workspaces
	for _, ws := range standardWorkspaces {
		for _, op := range []string{"read_file", "write_file"} {
			perm := fmt.Sprintf("%s(%s)", op, ws)
			if !currentAllowSet[perm] {
				mergedAllow = append(mergedAllow, perm)
				res.AddedPermissions = append(res.AddedPermissions, perm)
				currentAllowSet[perm] = true
			}
		}
	}
	permMap["allow"] = mergedAllow

	// Profile recording in settings
	existingSettings["orchestrationProfile"] = modelProfile
	existingSettings["orchestration"] = map[string]interface{}{
		"profile":      modelProfile,
		"description":  profInfo.Description,
		"root_model":   profInfo.RootModel,
		"worker_model": profInfo.WorkerModel,
		"root_slug":    profInfo.RootSlug,
	}

	// 3. Backup & write settings.json
	if !opts.DryRun {
		if settingsExists && !opts.NoBackup {
			ts := time.Now().Format("20060102_150405")
			backup := fmt.Sprintf("%s.bak.%s", settingsPath, ts)
			_ = copyFile(settingsPath, backup)
			res.BackupPath = backup
			res.Messages = append(res.Messages, fmt.Sprintf("Created backup: %s", backup))
		}
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			return nil, fmt.Errorf("mkdir settings dir: %w", err)
		}
		data, err := json.MarshalIndent(existingSettings, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal settings: %w", err)
		}
		if err := os.WriteFile(settingsPath, data, 0644); err != nil {
			return nil, fmt.Errorf("write settings file: %w", err)
		}
		res.Messages = append(res.Messages, fmt.Sprintf("Updated %s", settingsPath))
	} else {
		res.Messages = append(res.Messages, "[DRY-RUN] Skipped writing settings.json updates.")
	}

	// 4. Orchestration JSON record
	orchPath := opts.OrchestrationPath
	if orchPath == "" {
		orchPath = filepath.Join(geminiDir, "config", "orchestration.json")
	}
	if !opts.DryRun {
		orchRecord := map[string]interface{}{
			"active_profile": modelProfile,
			"updated_at":     time.Now().Format(time.RFC3339),
			"profile":        profInfo,
		}
		_ = os.MkdirAll(filepath.Dir(orchPath), 0755)
		data, _ := json.MarshalIndent(orchRecord, "", "  ")
		_ = os.WriteFile(orchPath, data, 0644)
		res.Messages = append(res.Messages, fmt.Sprintf("Recorded active orchestration profile in: %s", orchPath))
	} else {
		res.Messages = append(res.Messages, fmt.Sprintf("[DRY-RUN] Skipped writing orchestration profile to %s", orchPath))
	}

	// 5. Global Directives Linking
	directivesSrc := opts.GlobalDirectivesSrc
	if directivesSrc == "" {
		candidates := []string{
			filepath.Join("harness", "rules", "AGENTS.md"),
			filepath.Join("rules", "AGENTS.md"),
			filepath.Join(homeDir, "Projects", "agentkit", "harness", "rules", "AGENTS.md"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				directivesSrc, _ = filepath.Abs(c)
				break
			}
		}
	}

	if directivesSrc != "" {
		targetAgents := filepath.Join(geminiDir, "AGENTS.md")
		targetGemini := filepath.Join(geminiDir, "GEMINI.md")
		if !opts.DryRun {
			_ = os.MkdirAll(geminiDir, 0755)
			_ = os.Remove(targetAgents)
			_ = os.Symlink(directivesSrc, targetAgents)
			_ = os.Remove(targetGemini)
			_ = os.Symlink(directivesSrc, targetGemini)
			res.Messages = append(res.Messages, fmt.Sprintf("Linked global directives: %s -> %s", targetAgents, directivesSrc))
			res.Messages = append(res.Messages, fmt.Sprintf("Linked global directives: %s -> %s", targetGemini, directivesSrc))

			if opts.LinkHome {
				homeAgents := filepath.Join(homeDir, "AGENTS.md")
				homeGemini := filepath.Join(homeDir, "GEMINI.md")
				_ = os.Remove(homeAgents)
				_ = os.Symlink(directivesSrc, homeAgents)
				_ = os.Remove(homeGemini)
				_ = os.Symlink(directivesSrc, homeGemini)
			}
		} else {
			res.Messages = append(res.Messages, fmt.Sprintf("[DRY-RUN] Skipped symlinking directives in %s -> %s", geminiDir, directivesSrc))
		}
	}

	// 6. MCP Servers
	if !opts.NoMCP {
		mcpPath := opts.MCPPath
		if mcpPath == "" {
			mcpPath = filepath.Join(geminiDir, "config", "mcp_config.json")
		}
		mcpCat, _ := catalog.LoadEmbeddedMCP()
		if mcpCat != nil {
			userMCP := make(map[string]interface{})
			mcpExists := false
			if mcpData, err := os.ReadFile(mcpPath); err == nil {
				if err := json.Unmarshal(mcpData, &userMCP); err == nil {
					mcpExists = true
				}
			}

			srvMap, ok := userMCP["mcpServers"].(map[string]interface{})
			if !ok {
				srvMap = make(map[string]interface{})
				userMCP["mcpServers"] = srvMap
			}

			for name, srvConfig := range mcpCat.MCPServers {
				if _, ok := srvMap[name]; !ok {
					// Interpolate
					interpolatedArgs := make([]string, len(srvConfig.Args))
					for i, a := range srvConfig.Args {
						interpolatedArgs[i] = interpolateVar(a, homeDir)
					}
					srvMap[name] = map[string]interface{}{
						"command": srvConfig.Command,
						"args":    interpolatedArgs,
						"env":     srvConfig.Env,
					}
					res.AddedMCPServers = append(res.AddedMCPServers, name)
				}
			}

			if !opts.DryRun {
				if mcpExists && !opts.NoBackup {
					ts := time.Now().Format("20060102_150405")
					mcpBackup := fmt.Sprintf("%s.bak.%s", mcpPath, ts)
					_ = copyFile(mcpPath, mcpBackup)
					res.MCPBackupPath = mcpBackup
					res.Messages = append(res.Messages, fmt.Sprintf("Created MCP backup: %s", mcpBackup))
				}
				_ = os.MkdirAll(filepath.Dir(mcpPath), 0755)
				data, _ := json.MarshalIndent(userMCP, "", "  ")
				_ = os.WriteFile(mcpPath, data, 0644)
				res.Messages = append(res.Messages, fmt.Sprintf("Updated %s", mcpPath))
			} else {
				res.Messages = append(res.Messages, "[DRY-RUN] Skipped writing mcp_config.json updates.")
			}
		}
	}

	// 7. Standard Plugins Deployment
	if !opts.NoPlugins {
		pluginsSrc := opts.PluginsSrcDir
		if pluginsSrc == "" {
			candidates := []string{
				filepath.Join("harness", "plugins"),
				filepath.Join("plugins"),
				filepath.Join(homeDir, "Projects", "agentkit", "harness", "plugins"),
			}
			for _, c := range candidates {
				if info, err := os.Stat(c); err == nil && info.IsDir() {
					pluginsSrc, _ = filepath.Abs(c)
					break
				}
			}
		}

		pluginsDir := opts.PluginsDir
		if pluginsDir == "" {
			pluginsDir = filepath.Join(geminiDir, "config", "plugins")
		}

		if pluginsSrc != "" && fileExistsDir(pluginsSrc) {
			entries, _ := os.ReadDir(pluginsSrc)
			for _, entry := range entries {
				if entry.IsDir() {
					manifestPath := filepath.Join(pluginsSrc, entry.Name(), "plugin.json")
					if fileExists(manifestPath) {
						targetPlugin := filepath.Join(pluginsDir, entry.Name())
						if !opts.DryRun {
							_ = copyDir(filepath.Join(pluginsSrc, entry.Name()), targetPlugin)
							res.Messages = append(res.Messages, fmt.Sprintf("Deployed standard plugin: %s -> %s", entry.Name(), targetPlugin))
						} else {
							res.Messages = append(res.Messages, fmt.Sprintf("[DRY-RUN] Skipped deploying plugin %s -> %s", entry.Name(), targetPlugin))
						}
					}
				}
			}
		}
	}

	// 8. Global Skills Registry
	skillsDir := opts.ProjectSkillsDir
	if skillsDir == "" {
		candidates := []string{
			filepath.Join("project", "skills"),
			filepath.Join("skills"),
			filepath.Join(homeDir, "Projects", "agentkit", "project", "skills"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && info.IsDir() {
				skillsDir, _ = filepath.Abs(c)
				break
			}
		}
	}

	if skillsDir != "" && fileExistsDir(skillsDir) {
		skillsPath := filepath.Join(geminiDir, "config", "skills.json")
		skillsData := map[string]interface{}{"entries": []interface{}{}}
		if data, err := os.ReadFile(skillsPath); err == nil {
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err == nil {
				skillsData = parsed
			}
		}

		entries, ok := skillsData["entries"].([]interface{})
		if !ok {
			entries = []interface{}{}
		}

		alreadyRegistered := false
		for _, e := range entries {
			if em, ok := e.(map[string]interface{}); ok {
				if em["path"] == skillsDir {
					alreadyRegistered = true
					break
				}
			}
		}

		if !alreadyRegistered {
			entries = append(entries, map[string]interface{}{"path": skillsDir})
			skillsData["entries"] = entries
		}

		if !opts.DryRun {
			_ = os.MkdirAll(filepath.Dir(skillsPath), 0755)
			data, _ := json.MarshalIndent(skillsData, "", "  ")
			_ = os.WriteFile(skillsPath, data, 0644)
			res.Messages = append(res.Messages, fmt.Sprintf("Synchronized global skills registry (%s)", skillsPath))
		} else {
			res.Messages = append(res.Messages, fmt.Sprintf("[DRY-RUN] Would synchronize global skills registry (%s)", skillsPath))
		}
	}

	res.Success = true
	return res, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fileExistsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
