package hook

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	verbosePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bpytest\b`),
		regexp.MustCompile(`\bunittest\b`),
		regexp.MustCompile(`\bnpm\s+test\b`),
		regexp.MustCompile(`\bpnpm\s+test\b`),
		regexp.MustCompile(`\byarn\s+test\b`),
		regexp.MustCompile(`\bcargo\s+test\b`),
		regexp.MustCompile(`\bgo\s+test\b`),
		regexp.MustCompile(`\bmvn\s+test\b`),
		regexp.MustCompile(`\bgit\s+.*?\b(diff|log|show)\b`),
		regexp.MustCompile(`\bfind\s+`),
		regexp.MustCompile(`\bgrep\s+`),
		regexp.MustCompile(`\bcat\s+`),
		regexp.MustCompile(`\bdiff\s+`),
		regexp.MustCompile(`\bdpkg\s+-l\b`),
		regexp.MustCompile(`\bapt-cache\b`),
		regexp.MustCompile(`\btree\b`),
	}

	gitSecurityRe = regexp.MustCompile(
		`git\s+(push\s+.*(main|master)|push\s+.*(-f\b|--force\b)|merge\s+(main|master)|checkout\s+(main|master)\s*(&&|;)\s*git\s+merge|switch\s+(main|master)\s*(&&|;)\s*git\s+merge)`,
	)
	destructiveRe = regexp.MustCompile(
		`(\brm\s+-[a-zA-Z]*r[a-zA-Z]*f|\brm\s+-[a-zA-Z]*f[a-zA-Z]*r|\bdd\s+if=|\bmkfs\b)`,
	)
	shellOperatorRe = regexp.MustCompile(`(\||\>\>|\>|\<\<|\&\&|\;)`)
	gitMutationRe   = regexp.MustCompile(`\bgit\s+.*?\b(commit|push|checkout|switch|add|merge|rebase|stash|init|clone)\b`)
)

type filterWrapResponse struct {
	Decision  string            `json:"decision,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Overwrite map[string]string `json:"overwrite,omitempty"`
	Status    string            `json:"status,omitempty"`
	Message   string            `json:"message,omitempty"`
}

func isVerboseCommand(cmd string) bool {
	for _, pat := range verbosePatterns {
		if pat.MatchString(cmd) {
			return true
		}
	}
	return false
}

// FilterAndWrap implements the PreToolUse hook for run_command: intercepts verbose commands for Headroom compression while preserving security gates.
func FilterAndWrap(args []string, stdin io.Reader, stdout io.Writer) int {
	for _, a := range args {
		if a == "--dry-run" {
			writeJSONLine(stdout, filterWrapResponse{
				Status:  "dry-run",
				Message: "agentkit hook filter-and-wrap is operational",
			})
			return 0
		}
	}

	payload := make(map[string]any)
	if len(args) > 0 && args[0] == "--test" {
		cmdArg := ""
		if len(args) > 1 {
			cmdArg = args[1]
		}
		payload = map[string]any{
			"toolCall": map[string]any{
				"name": "run_command",
				"args": map[string]any{
					"CommandLine": cmdArg,
				},
			},
		}
	} else if len(args) > 0 && strings.HasPrefix(args[0], "--test=") {
		payload = map[string]any{
			"toolCall": map[string]any{
				"name": "run_command",
				"args": map[string]any{
					"CommandLine": strings.TrimPrefix(args[0], "--test="),
				},
			},
		}
	} else if stdin != nil {
		if raw, err := io.ReadAll(stdin); err == nil {
			if strings.TrimSpace(string(raw)) != "" {
				var loaded map[string]any
				if err := json.Unmarshal(raw, &loaded); err == nil && loaded != nil {
					payload = loaded
				}
			}
		}
	}

	if os.Getenv("AGY_DISABLE_TOKEN_COMPRESSION") == "1" {
		writeJSONLine(stdout, filterWrapResponse{Decision: "allow"})
		return 0
	}

	var toolName, commandLine string
	var isDaemon bool

	if tc, ok := payload["toolCall"].(map[string]any); ok {
		if name, ok := tc["name"].(string); ok {
			toolName = name
		}
		if tcArgs, ok := tc["args"].(map[string]any); ok {
			if cmd, ok := tcArgs["CommandLine"].(string); ok {
				commandLine = strings.TrimSpace(cmd)
			}
			if d, ok := tcArgs["IsDaemon"].(bool); ok {
				isDaemon = d
			}
		}
	}

	if commandLine != "" {
		if gitSecurityRe.MatchString(commandLine) {
			writeJSONLine(stdout, filterWrapResponse{
				Decision: "deny",
				Reason:   "SECURITY VIOLATION: Direct pushes, force-pushes, or trunk merges to main/master are strictly blocked.",
			})
			return 0
		}
		if destructiveRe.MatchString(commandLine) {
			writeJSONLine(stdout, filterWrapResponse{
				Decision: "ask",
				Reason:   "DESTRUCTIVE OPERATION DETECTED: Explicit user approval is required.",
			})
			return 0
		}
	}

	if toolName != "run_command" || commandLine == "" {
		writeJSONLine(stdout, filterWrapResponse{Decision: "allow"})
		return 0
	}

	if isDaemon {
		writeJSONLine(stdout, filterWrapResponse{Decision: "allow"})
		return 0
	}

	if strings.Contains(commandLine, "headroom-wrap") {
		writeJSONLine(stdout, filterWrapResponse{Decision: "allow"})
		return 0
	}

	if shellOperatorRe.MatchString(commandLine) {
		writeJSONLine(stdout, filterWrapResponse{Decision: "allow"})
		return 0
	}

	if gitMutationRe.MatchString(commandLine) {
		writeJSONLine(stdout, filterWrapResponse{Decision: "allow"})
		return 0
	}

	if isVerboseCommand(commandLine) {
		writeJSONLine(stdout, filterWrapResponse{
			Decision: "allow",
			Overwrite: map[string]string{
				"CommandLine": "agentkit hook headroom-wrap -- " + commandLine,
			},
		})
		return 0
	}

	writeJSONLine(stdout, filterWrapResponse{Decision: "allow"})
	return 0
}
