package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Cooper-Wallace/agentkit/internal/catalog"
)

// GroupStatus details quota health for a provider group.
type GroupStatus struct {
	Name              string  `json:"name"`
	RemainingFraction float64 `json:"remainingFraction"`
	RemainingPercent  float64 `json:"remainingPercent"`
	Window            string  `json:"window"`
	ResetTime         string  `json:"resetTime"`
	Status            string  `json:"status"` // "healthy", "low", "critical", "exhausted"
	Exhausted         bool    `json:"exhausted"`
	Critical          bool    `json:"critical"`
	Low               bool    `json:"low"`
}

// QuotaStatus represents the complete quota summary.
type QuotaStatus struct {
	Groups        map[string]GroupStatus `json:"groups"`
	EarliestReset string                 `json:"earliest_reset"`
	AllCritical   bool                   `json:"all_critical"`
	WrapupNeeded  bool                   `json:"wrapup_needed"`
}

// RouteResult describes the model routing decision.
type RouteResult struct {
	Model  string `json:"model"`
	Group  string `json:"group"`
	Reason string `json:"reason"`
}

type rawBucket struct {
	BucketID          string      `json:"bucketId"`
	DisplayName       string      `json:"displayName"`
	Window            interface{} `json:"window"`
	WindowMinutes     interface{} `json:"windowMinutes"`
	RemainingFraction interface{} `json:"remainingFraction"`
	Remaining         interface{} `json:"remaining"`
	Remaining_Fraction interface{} `json:"remaining_fraction"`
	Disabled          bool        `json:"disabled"`
	ResetTime         interface{} `json:"resetTime"`
	Reset_Time        interface{} `json:"reset_time"`
}

type rawGroup struct {
	DisplayName string      `json:"displayName"`
	Name        string      `json:"name"`
	Buckets     []rawBucket `json:"buckets"`
}

type rawPayload struct {
	Groups  []rawGroup  `json:"groups"`
	Buckets []rawBucket `json:"buckets"`
}

func parseBucketRemaining(b rawBucket) float64 {
	val := b.RemainingFraction
	if val == nil {
		val = b.Remaining
	}
	if val == nil {
		val = b.Remaining_Fraction
	}
	if val == nil {
		return 1.0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return 1.0
}

func parseBucketString(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

func resolveGroupID(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "argon") {
		return "argon"
	}
	if strings.Contains(lower, "claude") {
		return "claude"
	}
	if strings.Contains(lower, "gemini") || strings.Contains(lower, "flash") {
		return "gemini"
	}
	return "gemini"
}

func getSyntheticHealthyPayload() rawPayload {
	return rawPayload{
		Groups: []rawGroup{
			{
				DisplayName: "gemini",
				Buckets: []rawBucket{
					{
						BucketID:          "gemini-default",
						DisplayName:       "Gemini Models",
						Window:            "24h",
						RemainingFraction: 1.0,
						ResetTime:         "unknown",
					},
				},
			},
			{
				DisplayName: "claude",
				Buckets: []rawBucket{
					{
						BucketID:          "claude-default",
						DisplayName:       "Claude Models",
						Window:            "24h",
						RemainingFraction: 1.0,
						ResetTime:         "unknown",
					},
				},
			},
			{
				DisplayName: "argon",
				Buckets: []rawBucket{
					{
						BucketID:          "argon-default",
						DisplayName:       "Argon Models",
						Window:            "24h",
						RemainingFraction: 1.0,
						ResetTime:         "unknown",
					},
				},
			},
		},
	}
}

func fetchRawPayload(quotaFile string) (rawPayload, error) {
	if quotaFile != "" {
		data, err := os.ReadFile(quotaFile)
		if err != nil {
			return rawPayload{}, fmt.Errorf("read quota file %s: %w", quotaFile, err)
		}
		var payload rawPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return rawPayload{}, fmt.Errorf("unmarshal quota file: %w", err)
		}
		return payload, nil
	}

	// 1. Check fixture env var
	if fixture := os.Getenv("AGENTKIT_QUOTA_FIXTURE"); fixture != "" {
		data, err := os.ReadFile(fixture)
		if err == nil {
			var payload rawPayload
			if err := json.Unmarshal(data, &payload); err == nil {
				return payload, nil
			}
		}
	}

	// 2. Query language server if configured
	lsAddress := os.Getenv("ANTIGRAVITY_LS_ADDRESS")
	csrfToken := os.Getenv("ANTIGRAVITY_CSRF_TOKEN")
	if lsAddress != "" && csrfToken != "" {
		url := fmt.Sprintf("http://%s/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary", lsAddress)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
		if err == nil {
			req.Header.Set("x-codeium-csrf-token", csrfToken)
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err == nil {
					var payload rawPayload
					if err := json.Unmarshal(data, &payload); err == nil {
						// Cache response
						cachePath := os.Getenv("AGENTKIT_QUOTA_CACHE")
						if cachePath == "" {
							if h, err := os.UserHomeDir(); err == nil {
								cachePath = filepath.Join(h, ".cache", "agentkit", "quota.json")
							}
						}
						if cachePath != "" {
							_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
							_ = os.WriteFile(cachePath, data, 0644)
						}
						return payload, nil
					}
				}
			}
		}
	}

	// 3. Fall back to cached file
	cachePath := os.Getenv("AGENTKIT_QUOTA_CACHE")
	if cachePath == "" {
		if h, err := os.UserHomeDir(); err == nil {
			cachePath = filepath.Join(h, ".cache", "agentkit", "quota.json")
		}
	}
	if cachePath != "" {
		if data, err := os.ReadFile(cachePath); err == nil {
			var payload rawPayload
			if err := json.Unmarshal(data, &payload); err == nil {
				return payload, nil
			}
		}
	}

	// 4. Fail open to synthetic healthy payload
	return getSyntheticHealthyPayload(), nil
}

// Status inspects quota telemetry and returns QuotaStatus.
func Status(quotaFile string) (QuotaStatus, error) {
	payload, err := fetchRawPayload(quotaFile)
	if err != nil {
		return QuotaStatus{}, err
	}

	quotaCat, _ := catalog.LoadEmbeddedQuota()
	switchThresh := 0.15
	wrapupThresh := 0.05
	if quotaCat != nil && quotaCat.Thresholds.SwitchThreshold > 0 {
		switchThresh = quotaCat.Thresholds.SwitchThreshold
		wrapupThresh = quotaCat.Thresholds.WrapupThreshold
	}

	groupsData := payload.Groups
	if len(groupsData) == 0 && len(payload.Buckets) > 0 {
		groupsData = []rawGroup{{DisplayName: "default", Buckets: payload.Buckets}}
	}

	groupedBuckets := map[string][]rawBucket{
		"gemini": {},
		"claude": {},
		"argon":  {},
	}

	for _, g := range groupsData {
		name := g.DisplayName
		if name == "" {
			name = g.Name
		}
		gid := resolveGroupID(name)
		for _, b := range g.Buckets {
			if !b.Disabled {
				groupedBuckets[gid] = append(groupedBuckets[gid], b)
			}
		}
	}

	summaryGroups := make(map[string]GroupStatus)
	var earliestResets []string

	for _, gid := range []string{"gemini", "claude", "argon"} {
		buckets := groupedBuckets[gid]
		var remFrac float64 = 1.0
		window := "24h"
		resetTime := "unknown"

		if len(buckets) > 0 {
			var fractions []float64
			var resets []string
			var windows []string
			for _, b := range buckets {
				rf := parseBucketRemaining(b)
				fractions = append(fractions, rf)

				rt := parseBucketString(b.ResetTime)
				if rt == "" {
					rt = parseBucketString(b.Reset_Time)
				}
				if rt != "" && rt != "unknown" {
					resets = append(resets, rt)
				}

				w := parseBucketString(b.Window)
				if w == "" {
					w = parseBucketString(b.WindowMinutes)
				}
				if w != "" {
					windows = append(windows, w)
				}
			}

			if len(fractions) > 0 {
				sort.Float64s(fractions)
				remFrac = fractions[0]
			}
			if len(resets) > 0 {
				sort.Strings(resets)
				resetTime = resets[0]
			}
			if len(windows) > 0 {
				window = windows[0]
			}
		}

		if resetTime != "unknown" {
			earliestResets = append(earliestResets, resetTime)
		}

		isExhausted := remFrac <= 0.0
		isCritical := remFrac <= wrapupThresh
		isLow := remFrac <= switchThresh

		var status string
		if isExhausted {
			status = "exhausted"
		} else if isCritical {
			status = "critical"
		} else if isLow {
			status = "low"
		} else {
			status = "healthy"
		}

		pct := float64(int(remFrac*1000+0.5)) / 10.0

		summaryGroups[gid] = GroupStatus{
			Name:              gid,
			RemainingFraction: remFrac,
			RemainingPercent:  pct,
			Window:            window,
			ResetTime:         resetTime,
			Status:            status,
			Exhausted:         isExhausted,
			Critical:          isCritical,
			Low:               isLow,
		}
	}

	allCritical := true
	for _, g := range summaryGroups {
		if !g.Critical {
			allCritical = false
			break
		}
	}

	earliestReset := "unknown"
	if len(earliestResets) > 0 {
		sort.Strings(earliestResets)
		earliestReset = earliestResets[0]
	}

	return QuotaStatus{
		Groups:        summaryGroups,
		EarliestReset: earliestReset,
		AllCritical:   allCritical,
		WrapupNeeded:  allCritical,
	}, nil
}

// Route routes task complexity to the optimal model based on quota health.
func Route(quotaFile string, complexity string) (RouteResult, error) {
	status, err := Status(quotaFile)
	if err != nil {
		return RouteResult{}, err
	}

	quotaCat, _ := catalog.LoadEmbeddedQuota()
	switchThresh := 0.15
	if quotaCat != nil && quotaCat.Thresholds.SwitchThreshold > 0 {
		switchThresh = quotaCat.Thresholds.SwitchThreshold
	}

	gemini := status.Groups["gemini"]
	argon := status.Groups["argon"]

	geminiHealthy := gemini.RemainingFraction > switchThresh
	argonHealthy := argon.RemainingFraction > switchThresh

	switch strings.ToLower(complexity) {
	case "low":
		if geminiHealthy {
			return RouteResult{
				Model:  "flash_lite",
				Group:  "gemini",
				Reason: fmt.Sprintf("Low complexity routed to flash_lite; Gemini quota healthy (%.1f%%)", gemini.RemainingPercent),
			}, nil
		}
		return RouteResult{
			Model:  "inherit",
			Group:  "inherit",
			Reason: fmt.Sprintf("Gemini quota degraded (%.1f%%); low complexity falling back to inherit", gemini.RemainingPercent),
		}, nil

	case "med":
		if geminiHealthy {
			return RouteResult{
				Model:  "flash",
				Group:  "gemini",
				Reason: fmt.Sprintf("Medium complexity routed to flash; Gemini quota healthy (%.1f%%)", gemini.RemainingPercent),
			}, nil
		}
		return RouteResult{
			Model:  "inherit",
			Group:  "inherit",
			Reason: fmt.Sprintf("Gemini quota degraded (%.1f%%); medium complexity falling back to inherit", gemini.RemainingPercent),
		}, nil

	case "high":
		if argonHealthy {
			return RouteResult{
				Model:  "pro",
				Group:  "argon",
				Reason: fmt.Sprintf("High complexity escalated to pro; Argon quota healthy (%.1f%%)", argon.RemainingPercent),
			}, nil
		}
		if geminiHealthy {
			return RouteResult{
				Model:  "flash",
				Group:  "gemini",
				Reason: fmt.Sprintf("Argon quota degraded (%.1f%%); high complexity routed to flash", argon.RemainingPercent),
			}, nil
		}
		return RouteResult{
			Model:  "inherit",
			Group:  "inherit",
			Reason: "Argon and Gemini quotas degraded; high complexity falling back to inherit",
		}, nil

	default:
		return RouteResult{}, fmt.Errorf("invalid complexity tier %q (choose low, med, high)", complexity)
	}
}

// FormatStatusTable renders the status as a formatted table.
func FormatStatusTable(summary QuotaStatus) string {
	headers := []string{"Group", "Remaining %", "Window", "Reset Time", "Status"}
	rows := [][]string{}

	for _, gid := range []string{"gemini", "claude", "argon"} {
		g, ok := summary.Groups[gid]
		if !ok {
			continue
		}
		pctStr := fmt.Sprintf("%.1f%%", g.RemainingPercent)
		rows = append(rows, []string{gid, pctStr, g.Window, g.ResetTime, g.Status})
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder
	for i, h := range headers {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(fmt.Sprintf("%-*s", colWidths[i], h))
	}
	sb.WriteString("\n")

	for i, w := range colWidths {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(strings.Repeat("-", w))
	}
	sb.WriteString("\n")

	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				sb.WriteString("  ")
			}
			sb.WriteString(fmt.Sprintf("%-*s", colWidths[i], cell))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
