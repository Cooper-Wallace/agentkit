package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Cooper-Wallace/agentkit/internal/catalog"
	"github.com/Cooper-Wallace/agentkit/internal/quota"
)

var (
	protectedScopeRe = regexp.MustCompile(`(?i)\b(test|refactor|debug|security)\b`)
	complexityTagRe  = regexp.MustCompile(`(?i)\[complexity:\s*(low|med|high)\]`)
	groupOrder       = []string{"gemini", "claude", "argon"}
)

type guardQuotaResponse struct {
	Decision  string            `json:"decision,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Overwrite map[string]string `json:"overwrite,omitempty"`
	Status    string            `json:"status,omitempty"`
	Message   string            `json:"message,omitempty"`
}

func writeJSONLine(w io.Writer, v any) {
	if w == nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = w.Write(append(data, '\n'))
}

func resolveModelGroup(modelName string, cat *catalog.QuotaCatalog) string {
	trimmed := strings.TrimSpace(strings.ToLower(modelName))
	if trimmed == "" || trimmed == "inherit" {
		return ""
	}

	switch trimmed {
	case "flash_lite", "flash":
		return "gemini"
	case "pro":
		return "argon"
	}

	if strings.Contains(trimmed, "neon") {
		return "gemini"
	}
	if strings.Contains(trimmed, "carbon") || strings.Contains(trimmed, "argon") || strings.Contains(trimmed, "orchestrator") {
		return "argon"
	}
	if strings.Contains(trimmed, "claude") || strings.Contains(trimmed, "opus") || strings.Contains(trimmed, "sonnet") || strings.Contains(trimmed, "gpt") {
		return "claude"
	}
	if strings.Contains(trimmed, "gemini") || strings.Contains(trimmed, "hydrogen") || strings.Contains(trimmed, "flash") {
		return "gemini"
	}

	if cat != nil {
		for key, val := range cat.Groups {
			keyLower := strings.ToLower(key)
			switch v := val.(type) {
			case string:
				if strings.Contains(trimmed, strings.ToLower(v)) || strings.Contains(trimmed, keyLower) {
					return keyLower
				}
			case []any:
				for _, pat := range v {
					if strings.Contains(trimmed, strings.ToLower(fmt.Sprintf("%v", pat))) {
						return keyLower
					}
				}
			}
		}
	}

	return ""
}

func loadQuotaCatalog() *catalog.QuotaCatalog {
	if cfgEnv := os.Getenv("AGENTKIT_QUOTA_CONFIG"); cfgEnv != "" {
		if cat, err := catalog.LoadQuotaFile(cfgEnv); err == nil && cat != nil {
			return cat
		}
	}
	cat, _ := catalog.LoadEmbeddedQuota()
	return cat
}

func extractRootGroup(payload map[string]any, cat *catalog.QuotaCatalog) string {
	var rootModelInput string
	if v, ok := payload["modelName"].(string); ok && v != "" {
		rootModelInput = v
	} else if v, ok := payload["rootModel"].(string); ok && v != "" {
		rootModelInput = v
	} else if sess, ok := payload["session"].(map[string]any); ok {
		if v, ok := sess["model"].(string); ok && v != "" {
			rootModelInput = v
		}
	}

	if rootModelInput != "" {
		if g := resolveModelGroup(rootModelInput, cat); g != "" {
			return g
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return ""
	}
	orchPath := filepath.Join(homeDir, ".gemini", "config", "orchestration.json")
	data, err := os.ReadFile(orchPath)
	if err != nil {
		return ""
	}
	var orchCfg map[string]any
	if err := json.Unmarshal(data, &orchCfg); err != nil {
		return ""
	}

	for _, key := range []string{"model", "root_model", "active_profile"} {
		if v, ok := orchCfg[key].(string); ok && v != "" {
			return resolveModelGroup(v, cat)
		}
	}
	return ""
}

func groupRemaining(groups map[string]quota.GroupStatus, groupName string) float64 {
	if g, ok := groups[groupName]; ok {
		return g.RemainingFraction
	}
	return 1.0
}

// GuardQuota implements the PreToolUse hook for invoke_subagent enforcing quota monitoring and complexity routing.
func GuardQuota(args []string, stdin io.Reader, stdout io.Writer) int {
	for _, a := range args {
		if a == "--dry-run" {
			writeJSONLine(stdout, guardQuotaResponse{
				Status:  "dry-run",
				Message: "agentkit hook guard-quota is operational",
			})
			return 0
		}
	}

	if bypass := os.Getenv("AGENTKIT_QUOTA_GUARD"); bypass == "off" || bypass == "0" {
		writeJSONLine(stdout, guardQuotaResponse{
			Decision: "allow",
			Reason:   "Bypassed via AGENTKIT_QUOTA_GUARD",
		})
		return 0
	}

	inputJSON := ""
	if len(args) > 0 && args[0] == "--test" {
		if len(args) > 1 {
			inputJSON = args[1]
		} else {
			inputJSON = "{}"
		}
	} else if len(args) > 0 && strings.HasPrefix(args[0], "--test=") {
		inputJSON = strings.TrimPrefix(args[0], "--test=")
	} else if stdin != nil {
		if raw, err := io.ReadAll(stdin); err == nil {
			inputJSON = string(raw)
		}
	}

	payload := make(map[string]any)
	if strings.TrimSpace(inputJSON) != "" {
		if err := json.Unmarshal([]byte(inputJSON), &payload); err != nil || payload == nil {
			payload = make(map[string]any)
		}
	}

	var callArgs map[string]any
	if tc, ok := payload["toolCall"].(map[string]any); ok {
		if a, ok := tc["args"].(map[string]any); ok {
			callArgs = a
		}
	}
	if len(callArgs) == 0 {
		_, hasModel := payload["Model"]
		_, hasPrompt := payload["Prompt"]
		if hasModel || hasPrompt {
			callArgs = payload
		}
	}

	var originalModel, prompt string
	if callArgs != nil {
		if m, ok := callArgs["Model"].(string); ok {
			originalModel = m
		}
		if p, ok := callArgs["Prompt"].(string); ok {
			prompt = p
		}
	}

	quotaCat := loadQuotaCatalog()
	wrapupThresh := 0.05
	switchThresh := 0.15
	fallbacks := map[string][]string{
		"flash_lite": {"inherit"},
		"flash":      {"inherit"},
		"pro":        {"flash", "inherit"},
		"inherit":    {},
	}
	if quotaCat != nil {
		if quotaCat.Thresholds.WrapupThreshold > 0 {
			wrapupThresh = quotaCat.Thresholds.WrapupThreshold
		}
		if quotaCat.Thresholds.SwitchThreshold > 0 {
			switchThresh = quotaCat.Thresholds.SwitchThreshold
		}
		if len(quotaCat.Fallbacks) > 0 {
			fallbacks = quotaCat.Fallbacks
		}
	}

	summary, err := quota.Status("")
	if err != nil {
		writeJSONLine(stdout, guardQuotaResponse{Decision: "allow"})
		return 0
	}

	groups := summary.Groups
	allCritical := len(groups) > 0
	for _, g := range groups {
		if g.RemainingFraction > wrapupThresh {
			allCritical = false
			break
		}
	}

	if allCritical {
		writeJSONLine(stdout, guardQuotaResponse{
			Decision: "deny",
			Reason: "QUOTA WRAP-UP: All model quotas are critically low. Stop delegating new subagents. " +
				"Follow the quota-wrapup skill: run test suite, commit WIP to feature branch, " +
				"write handoff.md, and notify user.",
		})
		return 0
	}

	var complexity string
	if m := complexityTagRe.FindStringSubmatch(prompt); len(m) == 2 {
		complexity = strings.ToLower(m[1])
	}

	modelWasDefault := originalModel == "" || originalModel == "inherit"
	targetModel := originalModel
	if targetModel == "" {
		targetModel = "inherit"
	}

	if modelWasDefault && complexity != "" {
		switch complexity {
		case "low":
			targetModel = "flash_lite"
		case "med":
			targetModel = "flash"
		case "high":
			if groupRemaining(groups, "argon") > switchThresh {
				targetModel = "pro"
			} else {
				targetModel = "flash"
			}
		}
	}

	if targetModel == "flash_lite" || (complexity == "low" && modelWasDefault) {
		if protectedScopeRe.MatchString(prompt) {
			targetModel = "flash"
		}
	}

	var targetGroup string
	switch targetModel {
	case "flash_lite", "flash":
		targetGroup = "gemini"
	case "pro":
		targetGroup = "argon"
	default:
		if targetModel != "inherit" {
			targetGroup = resolveModelGroup(targetModel, quotaCat)
		}
	}

	rootGroup := extractRootGroup(payload, quotaCat)
	if targetModel == "inherit" && rootGroup != "" {
		targetGroup = rootGroup
	}

	var groupInfo quota.GroupStatus
	var hasGroupInfo bool
	if targetGroup != "" {
		groupInfo, hasGroupInfo = groups[targetGroup]
	}

	isDegraded := hasGroupInfo && groupInfo.RemainingFraction <= switchThresh

	if isDegraded {
		fbCandidates, ok := fallbacks[targetModel]
		if !ok {
			fbCandidates = []string{"inherit"}
		}

		var fallbackModel string
		var fallbackGroup string

		for _, cand := range fbCandidates {
			if cand == "inherit" {
				if rootGroup != "" {
					if rootGroup != targetGroup {
						if groupRemaining(groups, rootGroup) > switchThresh {
							fallbackModel = "inherit"
							fallbackGroup = rootGroup
							break
						}
					}
				} else {
					var otherHealthy string
					for _, gName := range groupOrder {
						if gName != targetGroup {
							if gData, exists := groups[gName]; exists && gData.RemainingFraction > switchThresh {
								otherHealthy = gName
								break
							}
						}
					}
					if otherHealthy != "" {
						fallbackModel = "inherit"
						fallbackGroup = otherHealthy
						break
					}
				}
			} else {
				candGroup := resolveModelGroup(cand, quotaCat)
				candRem := 1.0
				if candGroup != "" {
					candRem = groupRemaining(groups, candGroup)
				}
				if candRem > switchThresh {
					fallbackModel = cand
					fallbackGroup = candGroup
					break
				}
			}
		}

		if fallbackModel == "" {
			argonRem := groupRemaining(groups, "argon")
			geminiRem := groupRemaining(groups, "gemini")
			claudeRem := groupRemaining(groups, "claude")

			if targetGroup != "argon" && argonRem > switchThresh {
				fallbackModel = "pro"
				fallbackGroup = "argon"
			} else if targetGroup != "gemini" && geminiRem > switchThresh {
				fallbackModel = "flash"
				fallbackGroup = "gemini"
			} else if claudeRem > switchThresh {
				fallbackModel = "inherit"
				fallbackGroup = "claude"
			} else {
				fallbackModel = "inherit"
				fallbackGroup = ""
			}
		}

		pct := int(math.Round(groupInfo.RemainingFraction * 100.0))
		reset := groupInfo.ResetTime
		if reset == "" {
			reset = "unknown"
		}
		groupDesc := ""
		if fallbackGroup != "" {
			groupDesc = fmt.Sprintf(" (%s group)", fallbackGroup)
		}
		reason := fmt.Sprintf("Quota degraded for %s (%d%% remaining, resets %s). Rerouted subagent to %s%s.",
			targetGroup, pct, reset, fallbackModel, groupDesc)

		writeJSONLine(stdout, guardQuotaResponse{
			Decision:  "allow",
			Overwrite: map[string]string{"Model": fallbackModel},
			Reason:    reason,
		})
		return 0
	}

	origCompare := originalModel
	if origCompare == "" {
		origCompare = "inherit"
	}
	if targetModel != origCompare {
		writeJSONLine(stdout, guardQuotaResponse{
			Decision:  "allow",
			Overwrite: map[string]string{"Model": targetModel},
		})
		return 0
	}

	writeJSONLine(stdout, guardQuotaResponse{Decision: "allow"})
	return 0
}
