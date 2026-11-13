package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// VaultStatus reports the note inventory and status of an Obsidian AgentVault.
type VaultStatus struct {
	Exists         bool     `json:"exists"`
	Path           string   `json:"path"`
	TotalNotes     int      `json:"total_notes"`
	PendingNotes   int      `json:"pending_notes"`
	PendingFiles   []string `json:"pending_files,omitempty"`
	ActiveProjects int      `json:"active_projects"`
	Projects       []string `json:"projects,omitempty"`
	RecentSession  string   `json:"recent_session,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// NoteOptions parameters for creating or appending to notes.
type NoteOptions struct {
	VaultPath string   `json:"vault_path"`
	Project   string   `json:"project"`
	Title     string   `json:"title"`
	Section   string   `json:"section,omitempty"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags,omitempty"`
}

// NoteResult reports the outcome of a note operation.
type NoteResult struct {
	Success bool     `json:"success"`
	Action  string   `json:"action"` // "created", "appended", "appended_section"
	Path    string   `json:"path"`
	Project string   `json:"project,omitempty"`
	Title   string   `json:"title,omitempty"`
	Section string   `json:"section,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// ResolveVaultPath dynamically resolves the Obsidian AgentVault root directory.
// Priority:
// 1. Explicit path parameter (if provided)
// 2. $AGENTKIT_VAULT_PATH
// 3. $AGENTKIT_AGENTVAULT_PATH
// 4. $OBSIDIAN_VAULT_PATH
// 5. $HOME/Documents/AgentVault
// 6. $HOME/AgentVault (fallback check)
func ResolveVaultPath(explicitPath string) string {
	if explicitPath != "" {
		return expandAndClean(explicitPath)
	}

	for _, envVar := range []string{"AGENTKIT_VAULT_PATH", "AGENTKIT_AGENTVAULT_PATH", "OBSIDIAN_VAULT_PATH"} {
		if val := os.Getenv(envVar); val != "" {
			return expandAndClean(val)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	docPath := filepath.Join(homeDir, "Documents", "AgentVault")
	altPath := filepath.Join(homeDir, "AgentVault")

	if fi, err := os.Stat(docPath); err == nil && fi.IsDir() {
		return docPath
	}
	if fi, err := os.Stat(altPath); err == nil && fi.IsDir() {
		return altPath
	}

	return docPath
}

func expandAndClean(p string) string {
	p = os.ExpandEnv(p)
	if strings.HasPrefix(p, "~/") || p == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				p = homeDir
			} else {
				p = filepath.Join(homeDir, p[2:])
			}
		}
	}
	abs, err := filepath.Abs(p)
	if err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(p)
}

// Status inspects the vault directory and returns inventory statistics.
func Status(vaultPath string) (VaultStatus, error) {
	resolvedPath := ResolveVaultPath(vaultPath)
	status := VaultStatus{
		Path: resolvedPath,
	}

	fi, err := os.Stat(resolvedPath)
	obsidianDir := filepath.Join(resolvedPath, ".obsidian")
	ofi, oerr := os.Stat(obsidianDir)

	if err != nil || !fi.IsDir() || oerr != nil || !ofi.IsDir() {
		status.Exists = false
		status.Error = "Obsidian AgentVault does not exist or is not initialized."
		return status, errors.New("Obsidian AgentVault not found or not initialized")
	}

	status.Exists = true

	// Count total notes excluding .obsidian
	totalNotes := 0
	_ = filepath.Walk(resolvedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".obsidian" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".md") && !strings.HasPrefix(info.Name(), ".") {
			totalNotes++
		}
		return nil
	})
	status.TotalNotes = totalNotes

	// Pending notes in User-Input/pending and 00-Inbox/pending
	pendingDir := filepath.Join(resolvedPath, "User-Input", "pending")
	inboxPendingDir := filepath.Join(resolvedPath, "00-Inbox", "pending")
	var pendingFiles []string

	for _, pDir := range []string{pendingDir, inboxPendingDir} {
		entries, err := os.ReadDir(pDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") && !strings.HasPrefix(entry.Name(), ".") {
					pendingFiles = append(pendingFiles, entry.Name())
				}
			}
		}
	}
	sort.Strings(pendingFiles)
	status.PendingNotes = len(pendingFiles)
	status.PendingFiles = pendingFiles

	// Projects
	projectsDir := filepath.Join(resolvedPath, "Projects")
	var projectFiles []string
	entries, err := os.ReadDir(projectsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") && !strings.HasPrefix(entry.Name(), ".") {
				stem := strings.TrimSuffix(entry.Name(), ".md")
				projectFiles = append(projectFiles, stem)
			}
		}
	}
	sort.Strings(projectFiles)
	status.ActiveProjects = len(projectFiles)
	status.Projects = projectFiles

	// Sessions
	sessionsDir := filepath.Join(resolvedPath, "Sessions")
	var recentSession string
	var newestTime time.Time
	sEntries, sErr := os.ReadDir(sessionsDir)
	if sErr == nil {
		for _, entry := range sEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") && entry.Name() != "README.md" && !strings.HasPrefix(entry.Name(), ".") {
				info, err := entry.Info()
				if err == nil {
					if info.ModTime().After(newestTime) || recentSession == "" {
						newestTime = info.ModTime()
						recentSession = entry.Name()
					}
				}
			}
		}
	}
	status.RecentSession = recentSession

	return status, nil
}

// Init initializes standard Obsidian AgentVault structure, templates, and trustedWorkspaces.
func Init(vaultPath string) error {
	return InitWithSettings(vaultPath, "")
}

// InitWithSettings initializes vault with an explicit settingsPath for harness registration.
func InitWithSettings(vaultPath, settingsPath string) error {
	resolved := ResolveVaultPath(vaultPath)

	// 1. Create standard directories
	dirs := []string{
		resolved,
		filepath.Join(resolved, ".obsidian"),
		filepath.Join(resolved, "00-Inbox"),
		filepath.Join(resolved, "User-Input", "pending"),
		filepath.Join(resolved, "User-Input", "processed"),
		filepath.Join(resolved, "Projects"),
		filepath.Join(resolved, "Architecture", "decisions"),
		filepath.Join(resolved, "Sessions"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// 2. Markers & gitkeeps
	keepDirs := []string{
		filepath.Join(resolved, "00-Inbox"),
		filepath.Join(resolved, "User-Input", "pending"),
		filepath.Join(resolved, "User-Input", "processed"),
		filepath.Join(resolved, "Architecture", "decisions"),
	}
	for _, kd := range keepDirs {
		gitkeep := filepath.Join(kd, ".gitkeep")
		if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
			_ = os.WriteFile(gitkeep, []byte(""), 0644)
		}
	}

	// 2b. Git ignore for Obsidian workspace, caches, and OS artifacts
	gitignorePath := filepath.Join(resolved, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		content := `.obsidian/workspace.json
.obsidian/workspace-mobile.json
.obsidian/cache
.trash/
.DS_Store
`
		if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write .gitignore: %w", err)
		}
	}

	// 3. Root notes
	rootNotes := map[string]string{
		filepath.Join(resolved, "Dashboard.md"):                         getDashboardTemplate(),
		filepath.Join(resolved, "User-Input", "README.md"):               getUserInputReadmeTemplate(),
		filepath.Join(resolved, "Architecture", "00-Overview.md"):       getArchitectureOverviewTemplate(),
		filepath.Join(resolved, "Architecture", "contracts.md"):         getArchitectureContractsTemplate(),
		filepath.Join(resolved, "Architecture", "decisions", "README.md"): getArchitectureDecisionsReadmeTemplate(),
		filepath.Join(resolved, "Sessions", "README.md"):                 getSessionsReadmeTemplate(),
	}
	for p, c := range rootNotes {
		_ = writeNoteIfMissing(p, c)
	}

	// 4. Initial project notes
	initialProjects := []string{
		"agentkit",
		"repokit",
		"riced-kubuntu-os",
		"workstation-setup",
		"augurs-bazaar",
		"augurs-saber",
	}
	for _, proj := range initialProjects {
		p := filepath.Join(resolved, "Projects", proj+".md")
		_ = writeNoteIfMissing(p, GetProjectTemplate(proj))
	}

	// 5. Maintain symlink: Path.home() / "AgentVault" -> resolved
	homeDir, err := os.UserHomeDir()
	if err == nil {
		convenientLink := filepath.Join(homeDir, "AgentVault")
		if convenientLink != resolved {
			fi, err := os.Lstat(convenientLink)
			if err == nil && (fi.Mode()&os.ModeSymlink != 0) {
				target, _ := os.Readlink(convenientLink)
				if target != resolved {
					_ = os.Remove(convenientLink)
					_ = os.Symlink(resolved, convenientLink)
				}
			} else if os.IsNotExist(err) {
				_ = os.Symlink(resolved, convenientLink)
			}
		}
	}

	// 6. Ensure vaultPath and default AgentVault are in trustedWorkspaces
	_ = ensureTrustedWorkspaces(resolved, settingsPath)

	return nil
}

func writeNoteIfMissing(path, content string) bool {
	if _, err := os.Stat(path); err == nil {
		return false
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644) == nil
}

func ensureTrustedWorkspaces(vaultPath, explicitSettingsPath string) error {
	settingsPath := explicitSettingsPath
	if settingsPath == "" {
		if env := os.Getenv("AGENTKIT_SETTINGS_PATH"); env != "" {
			settingsPath = expandAndClean(env)
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil
			}
			settingsPath = filepath.Join(homeDir, ".gemini", "antigravity", "settings.json")
		}
	}

	homeDir, _ := os.UserHomeDir()
	targetWorkspaces := []string{
		vaultPath,
		filepath.Join(homeDir, "AgentVault"),
	}

	data, err := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	if err != nil {
		settings = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			settings = make(map[string]interface{})
		}
	}

	var currentWorkspaces []string
	if wsList, ok := settings["trustedWorkspaces"].([]interface{}); ok {
		for _, item := range wsList {
			if s, ok := item.(string); ok {
				currentWorkspaces = append(currentWorkspaces, s)
			}
		}
	}

	seen := make(map[string]bool)
	var deduped []string
	for _, ws := range currentWorkspaces {
		if !seen[ws] {
			seen[ws] = true
			deduped = append(deduped, ws)
		}
	}
	modified := false
	for _, tw := range targetWorkspaces {
		if !seen[tw] {
			seen[tw] = true
			deduped = append(deduped, tw)
			modified = true
		}
	}

	if modified || len(deduped) != len(currentWorkspaces) {
		settings["trustedWorkspaces"] = deduped
		_ = os.MkdirAll(filepath.Dir(settingsPath), 0755)
		encoded, err := json.MarshalIndent(settings, "", "  ")
		if err == nil {
			_ = os.WriteFile(settingsPath, encoded, 0644)
		}
	}

	return nil
}

// ParseFrontmatterAndBody parses YAML frontmatter and markdown body from text.
func ParseFrontmatterAndBody(text string) (map[string]any, string) {
	trimmed := strings.TrimLeft(text, " \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return make(map[string]any), text
	}

	parts := strings.SplitN(trimmed, "---", 3)
	if len(parts) < 3 {
		return make(map[string]any), text
	}

	rawFM := strings.TrimSpace(parts[1])
	body := strings.TrimLeft(parts[2], "\r\n")

	meta := make(map[string]any)
	lines := strings.Split(rawFM, "\n")
	var currentKey string

	for _, line := range lines {
		lineStr := strings.TrimSpace(line)
		if lineStr == "" || strings.HasPrefix(lineStr, "#") {
			continue
		}
		if (strings.HasPrefix(lineStr, "- ") || strings.HasPrefix(line, "  - ")) && currentKey != "" {
			val := strings.TrimSpace(strings.TrimPrefix(lineStr, "- "))
			if existing, ok := meta[currentKey].([]string); ok {
				meta[currentKey] = append(existing, val)
			} else {
				meta[currentKey] = []string{val}
			}
		} else if strings.Contains(lineStr, ":") {
			kv := strings.SplitN(lineStr, ":", 2)
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			currentKey = k
			if v != "" {
				meta[k] = v
			} else {
				meta[k] = []string{}
			}
		} else {
			currentKey = ""
		}
	}

	return meta, body
}

// FormatFrontmatter formats clean YAML frontmatter with tags, type, last_updated, and updated_by.
func FormatFrontmatter(tags []string, noteType, updatedBy string, extra map[string]any) string {
	todayISO := time.Now().Format("2006-01-02")
	var lines []string
	lines = append(lines, "---")

	if len(tags) > 0 {
		lines = append(lines, "tags:")
		for _, t := range tags {
			lines = append(lines, fmt.Sprintf("  - %s", t))
		}
	}

	lines = append(lines, fmt.Sprintf("type: %s", noteType))
	lines = append(lines, fmt.Sprintf("last_updated: %s", todayISO))
	if updatedBy == "" {
		updatedBy = "agentkit"
	}
	lines = append(lines, fmt.Sprintf("updated_by: %s", updatedBy))

	if extra != nil {
		var extraKeys []string
		for k := range extra {
			if k == "tags" || k == "type" || k == "last_updated" || k == "updated_by" {
				continue
			}
			extraKeys = append(extraKeys, k)
		}
		sort.Strings(extraKeys)
		for _, k := range extraKeys {
			v := extra[k]
			switch val := v.(type) {
			case []string:
				lines = append(lines, fmt.Sprintf("%s:", k))
				for _, item := range val {
					lines = append(lines, fmt.Sprintf("  - %s", item))
				}
			case []interface{}:
				lines = append(lines, fmt.Sprintf("%s:", k))
				for _, item := range val {
					lines = append(lines, fmt.Sprintf("  - %v", item))
				}
			default:
				lines = append(lines, fmt.Sprintf("%s: %v", k, val))
			}
		}
	}

	lines = append(lines, "---")
	return strings.Join(lines, "\n")
}

var nonWordRegex = regexp.MustCompile(`[^\w\-\s]`)

func sanitizeTitle(title string) string {
	cleaned := nonWordRegex.ReplaceAllString(title, "")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.ReplaceAll(cleaned, " ", "-")
	if cleaned == "" {
		return "note"
	}
	return cleaned
}

// Note creates a structured note or appends a markdown section in Obsidian AgentVault.
func Note(opts NoteOptions) (NoteResult, error) {
	vaultPath := ResolveVaultPath(opts.VaultPath)
	res := NoteResult{
		Success: false,
	}

	if fi, err := os.Stat(vaultPath); err != nil || !fi.IsDir() {
		return res, fmt.Errorf("Obsidian AgentVault does not exist at %s", vaultPath)
	}

	project := opts.Project
	if project == "" {
		project = "general"
	}
	project = strings.TrimSuffix(project, ".md")

	title := strings.TrimSpace(opts.Title)
	content := strings.TrimSpace(opts.Content)

	// Tags
	tags := []string{"project", project}
	for _, t := range opts.Tags {
		for _, part := range strings.Split(t, ",") {
			cleaned := strings.TrimSpace(part)
			if cleaned != "" {
				found := false
				for _, existing := range tags {
					if existing == cleaned {
						found = true
						break
					}
				}
				if !found {
					tags = append(tags, cleaned)
				}
			}
		}
	}

	// Case 1: Append section into Projects/<project>.md
	if opts.Section != "" {
		projFile := filepath.Join(vaultPath, "Projects", project+".md")
		if _, err := os.Stat(projFile); os.IsNotExist(err) {
			_ = os.MkdirAll(filepath.Dir(projFile), 0755)
			_ = os.WriteFile(projFile, []byte(GetProjectTemplate(project)), 0644)
		}

		rawBytes, err := os.ReadFile(projFile)
		if err != nil {
			return res, fmt.Errorf("failed to read project file %s: %w", projFile, err)
		}
		projText := string(rawBytes)
		sectionHeading := fmt.Sprintf("## %s", opts.Section)
		var updated string

		if strings.Contains(projText, sectionHeading) {
			parts := strings.SplitN(projText, sectionHeading, 2)
			updated = parts[0] + sectionHeading + "\n" + content + "\n\n" + strings.TrimLeft(parts[1], "\r\n")
		} else {
			updated = strings.TrimRight(projText, "\r\n") + fmt.Sprintf("\n\n%s\n%s\n", sectionHeading, content)
		}

		if err := os.WriteFile(projFile, []byte(updated), 0644); err != nil {
			return res, fmt.Errorf("failed to write project file %s: %w", projFile, err)
		}

		res.Success = true
		res.Action = "appended_section"
		res.Path = projFile
		res.Section = opts.Section
		res.Project = project
		return res, nil
	}

	// Case 2: Create or append to structured note in Projects/
	safeTitle := sanitizeTitle(title)
	noteFile := filepath.Join(vaultPath, "Projects", safeTitle+".md")
	action := "created"

	if _, err := os.Stat(noteFile); err == nil {
		existingBytes, err := os.ReadFile(noteFile)
		if err != nil {
			return res, fmt.Errorf("failed to read note file %s: %w", noteFile, err)
		}
		timestamp := time.Now().Format("2006-01-02 15:04")
		appendedBlock := fmt.Sprintf("\n\n## %s Entry\n\n%s\n", timestamp, content)
		updated := strings.TrimRight(string(existingBytes), "\r\n") + appendedBlock
		if err := os.WriteFile(noteFile, []byte(updated), 0644); err != nil {
			return res, fmt.Errorf("failed to update note file %s: %w", noteFile, err)
		}
		action = "appended"
	} else {
		_ = os.MkdirAll(filepath.Dir(noteFile), 0755)
		fm := FormatFrontmatter(tags, "note", "agentkit", map[string]any{"project": project})
		body := fmt.Sprintf("# %s\n\n> [!note] Project: [[Projects/%s|%s]]\n\n%s\n", title, project, project, content)
		fullNote := fm + "\n\n" + body
		if err := os.WriteFile(noteFile, []byte(fullNote), 0644); err != nil {
			return res, fmt.Errorf("failed to create note file %s: %w", noteFile, err)
		}
	}

	// Link from Projects/<project>.md if it exists
	projFile := filepath.Join(vaultPath, "Projects", project+".md")
	if fi, err := os.Stat(projFile); err == nil && !fi.IsDir() && projFile != noteFile {
		pBytes, err := os.ReadFile(projFile)
		if err == nil {
			pText := string(pBytes)
			wikilink := fmt.Sprintf("- [[Projects/%s|%s]]", safeTitle, title)
			if !strings.Contains(pText, wikilink) {
				var updatedProj string
				if strings.Contains(pText, "## Related Notes") {
					parts := strings.SplitN(pText, "## Related Notes", 2)
					updatedProj = parts[0] + "## Related Notes\n" + wikilink + "\n" + strings.TrimLeft(parts[1], "\r\n")
				} else {
					updatedProj = strings.TrimRight(pText, "\r\n") + fmt.Sprintf("\n\n## Related Notes\n%s\n", wikilink)
				}
				_ = os.WriteFile(projFile, []byte(updatedProj), 0644)
			}
		}
	}

	res.Success = true
	res.Action = action
	res.Path = noteFile
	res.Project = project
	res.Title = title
	res.Tags = tags
	return res, nil
}
