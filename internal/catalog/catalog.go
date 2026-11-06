package catalog

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/Cooper-Wallace/agentkit"
)

// PermissionsCatalog represents the permissions.json specification.
type PermissionsCatalog struct {
	Schema      string                        `json:"$schema,omitempty"`
	Version     string                        `json:"version,omitempty"`
	Description string                        `json:"description,omitempty"`
	Categories  map[string]PermissionCategory `json:"categories"`
	MCPTools    map[string]PermissionGroup    `json:"mcp_tools,omitempty"`
	Tools       map[string]PermissionGroup    `json:"tools,omitempty"`
	FileAccess  map[string]PermissionGroup    `json:"file_access,omitempty"`
}

// PermissionCategory groups command definitions.
type PermissionCategory struct {
	Description string   `json:"description,omitempty"`
	Commands    []string `json:"commands"`
}

// PermissionGroup contains a list of permission strings and description.
type PermissionGroup struct {
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
}

// UnmarshalJSON handles both array of strings and object with "permissions" field.
func (p *PermissionGroup) UnmarshalJSON(data []byte) error {
	var stringList []string
	if err := json.Unmarshal(data, &stringList); err == nil {
		p.Permissions = stringList
		return nil
	}
	type Alias PermissionGroup
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*p = PermissionGroup(alias)
	return nil
}

// MCPCatalog represents standard mcp.json configuration.
type MCPCatalog struct {
	Schema       string                    `json:"$schema,omitempty"`
	Version      string                    `json:"version,omitempty"`
	Description  string                    `json:"description,omitempty"`
	MCPServers   map[string]MCPServerConfig `json:"mcpServers"`
	Requirements map[string]MCPRequirement  `json:"_requirements,omitempty"`
}

// MCPServerConfig defines an MCP server invocation.
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPRequirement specifies environment/binary prerequisites.
type MCPRequirement struct {
	Description  string   `json:"description,omitempty"`
	RequiredBins []string `json:"required_bins,omitempty"`
	RequiredEnv  []string `json:"required_env,omitempty"`
	SetupHint    string   `json:"setup_hint,omitempty"`
}

// ProfilesCatalog defines model orchestration profiles.
type ProfilesCatalog struct {
	DefaultProfile    string                 `json:"default_profile"`
	DeprecatedAliases map[string]string      `json:"deprecated_aliases,omitempty"`
	Profiles          map[string]ProfileInfo `json:"profiles"`
}

// ProfileInfo holds profile tier details.
type ProfileInfo struct {
	Description     string                    `json:"description"`
	RootModel       string                    `json:"root_model"`
	WorkerModel     string                    `json:"worker_model"`
	RootSlug        string                    `json:"root_slug"`
	QuotaFallback   []string                  `json:"quota_fallback,omitempty"`
	RootReasoning   string                    `json:"root_reasoning,omitempty"`
	ComplexityTiers map[string]ComplexityTier `json:"complexity_tiers,omitempty"`
}

// ComplexityTier maps complexity levels to models.
type ComplexityTier struct {
	Model      string `json:"model,omitempty"`
	EscalateTo string `json:"escalate_to,omitempty"`
	Use        string `json:"use,omitempty"`
}

// QuotaCatalog represents quota thresholds and routing rules.
type QuotaCatalog struct {
	Thresholds        QuotaThresholds              `json:"thresholds"`
	Groups            map[string]interface{}       `json:"groups,omitempty"`
	ComplexityToModel map[string]ComplexityMapping `json:"complexity_to_model"`
	Fallbacks         map[string][]string          `json:"fallbacks"`
}

// QuotaThresholds defines quota capacity percentages.
type QuotaThresholds struct {
	SwitchThreshold float64 `json:"switch_threshold"`
	WrapupThreshold float64 `json:"wrapup_threshold"`
	CacheTTLSeconds int     `json:"cache_ttl_seconds"`
}

// ComplexityMapping defines routing target per complexity.
type ComplexityMapping struct {
	Model         string `json:"model"`
	EscalateTo    string `json:"escalate_to,omitempty"`
	Group         string `json:"group"`
	EscalateGroup string `json:"escalate_group,omitempty"`
	Use           string `json:"use,omitempty"`
}

// SettingsCatalog represents settings.default.json defaults.
type SettingsCatalog struct {
	AllowNonWorkspaceAccess bool     `json:"allowNonWorkspaceAccess"`
	ColorScheme             string   `json:"colorScheme"`
	Model                   string   `json:"model"`
	TrustedWorkspaces       []string `json:"trustedWorkspaces"`
}

// LoadEmbeddedPermissions loads permissions from embedded config.
func LoadEmbeddedPermissions() (*PermissionsCatalog, error) {
	data, err := agentkit.ConfigFS.ReadFile("config/permissions.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded permissions.json: %w", err)
	}
	return parsePermissions(data)
}

// LoadPermissionsFile loads permissions catalog from a file.
func LoadPermissionsFile(path string) (*PermissionsCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read permissions file %s: %w", path, err)
	}
	return parsePermissions(data)
}

func parsePermissions(data []byte) (*PermissionsCatalog, error) {
	var cat PermissionsCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("unmarshal permissions catalog: %w", err)
	}
	return &cat, nil
}

// LoadEmbeddedMCP loads MCP config from embedded config.
func LoadEmbeddedMCP() (*MCPCatalog, error) {
	data, err := agentkit.ConfigFS.ReadFile("config/mcp.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded mcp.json: %w", err)
	}
	return parseMCP(data)
}

// LoadMCPFile loads MCP config from a file.
func LoadMCPFile(path string) (*MCPCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mcp file %s: %w", path, err)
	}
	return parseMCP(data)
}

func parseMCP(data []byte) (*MCPCatalog, error) {
	var cat MCPCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("unmarshal mcp catalog: %w", err)
	}
	return &cat, nil
}

// LoadEmbeddedProfiles loads profiles from embedded config.
func LoadEmbeddedProfiles() (*ProfilesCatalog, error) {
	data, err := agentkit.ConfigFS.ReadFile("config/profiles.json")
	if err != nil {
		// profiles.json is a symlink in config/, so fallback to orchestration.json
		data, err = agentkit.ConfigFS.ReadFile("config/orchestration.json")
	}
	if err != nil {
		return nil, fmt.Errorf("read embedded profiles: %w", err)
	}
	return parseProfiles(data)
}

// LoadProfilesFile loads profiles from a file.
func LoadProfilesFile(path string) (*ProfilesCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profiles file %s: %w", path, err)
	}
	return parseProfiles(data)
}

func parseProfiles(data []byte) (*ProfilesCatalog, error) {
	var cat ProfilesCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("unmarshal profiles catalog: %w", err)
	}
	return &cat, nil
}

// LoadEmbeddedQuota loads quota catalog from embedded config.
func LoadEmbeddedQuota() (*QuotaCatalog, error) {
	data, err := agentkit.ConfigFS.ReadFile("config/quota.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded quota.json: %w", err)
	}
	return parseQuota(data)
}

// LoadQuotaFile loads quota catalog from a file.
func LoadQuotaFile(path string) (*QuotaCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read quota file %s: %w", path, err)
	}
	return parseQuota(data)
}

func parseQuota(data []byte) (*QuotaCatalog, error) {
	var cat QuotaCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("unmarshal quota catalog: %w", err)
	}
	return &cat, nil
}

// LoadEmbeddedSettingsDefaults loads default settings from embedded config.
func LoadEmbeddedSettingsDefaults() (*SettingsCatalog, error) {
	data, err := agentkit.ConfigFS.ReadFile("config/settings.default.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded settings.default.json: %w", err)
	}
	return parseSettingsDefaults(data)
}

// LoadSettingsDefaultsFile loads default settings from a file.
func LoadSettingsDefaultsFile(path string) (*SettingsCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read settings defaults file %s: %w", path, err)
	}
	return parseSettingsDefaults(data)
}

func parseSettingsDefaults(data []byte) (*SettingsCatalog, error) {
	var cat SettingsCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("unmarshal settings defaults: %w", err)
	}
	return &cat, nil
}

// GetFlatStandardPermissions flattens permissions catalog into a string list.
func GetFlatStandardPermissions(catalog *PermissionsCatalog, homeDir string) []string {
	if catalog == nil {
		return nil
	}
	if homeDir == "" {
		h, err := os.UserHomeDir()
		if err == nil {
			homeDir = h
		}
	}

	var permissions []string
	seen := make(map[string]bool)

	addPerm := func(p string) {
		if !seen[p] {
			seen[p] = true
			permissions = append(permissions, p)
		}
	}

	// 1. Categories in deterministic key order
	var catKeys []string
	for k := range catalog.Categories {
		catKeys = append(catKeys, k)
	}
	sort.Strings(catKeys)
	for _, k := range catKeys {
		cat := catalog.Categories[k]
		for _, cmd := range cat.Commands {
			addPerm(fmt.Sprintf("command(%s)", cmd))
		}
	}

	// 2. MCP Tools
	var mcpKeys []string
	for k := range catalog.MCPTools {
		mcpKeys = append(mcpKeys, k)
	}
	sort.Strings(mcpKeys)
	for _, k := range mcpKeys {
		for _, p := range catalog.MCPTools[k].Permissions {
			addPerm(p)
		}
	}

	// 3. Tools
	var toolKeys []string
	for k := range catalog.Tools {
		toolKeys = append(toolKeys, k)
	}
	sort.Strings(toolKeys)
	for _, k := range toolKeys {
		for _, p := range catalog.Tools[k].Permissions {
			addPerm(p)
		}
	}

	// 4. File access (${HOME} and $HOME replaced)
	var faKeys []string
	for k := range catalog.FileAccess {
		faKeys = append(faKeys, k)
	}
	sort.Strings(faKeys)
	for _, k := range faKeys {
		for _, p := range catalog.FileAccess[k].Permissions {
			resolved := strings.ReplaceAll(p, "${HOME}", homeDir)
			resolved = strings.ReplaceAll(resolved, "$HOME", homeDir)
			addPerm(resolved)
		}
	}

	return permissions
}

// Ensure fs.ErrNotExist is referenced if needed
var _ = fs.ErrNotExist
