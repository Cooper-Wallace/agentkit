package quota

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuotaStatusSyntheticFallback(t *testing.T) {
	t.Setenv("AGENTKIT_QUOTA_CACHE", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("AGENTKIT_QUOTA_FIXTURE", "")
	t.Setenv("ANTIGRAVITY_LS_ADDRESS", "")
	t.Setenv("ANTIGRAVITY_CSRF_TOKEN", "")
	status, err := Status("")
	if err != nil {
		t.Fatalf("Status with empty file failed: %v", err)
	}
	for _, gid := range []string{"gemini", "claude", "argon"} {
		g, ok := status.Groups[gid]
		if !ok {
			t.Errorf("missing group %s in status", gid)
		}
		if g.RemainingFraction != 1.0 {
			t.Errorf("expected 1.0 fraction for %s, got %f", gid, g.RemainingFraction)
		}
		if g.Status != "healthy" {
			t.Errorf("expected healthy status for %s, got %s", gid, g.Status)
		}
	}
	if status.AllCritical {
		t.Errorf("expected AllCritical false for synthetic healthy")
	}

	table := FormatStatusTable(status)
	if !strings.Contains(table, "Group") || !strings.Contains(table, "gemini") {
		t.Errorf("unexpected table format: %s", table)
	}
}

func TestQuotaStatusFromFixture(t *testing.T) {
	tempDir := t.TempDir()
	fixturePath := filepath.Join(tempDir, "quota.json")

	// Create fixture with degraded gemini and critical argon
	fixtureContent := `{
		"groups": [
			{
				"displayName": "Gemini 3.8 Flash",
				"buckets": [
					{
						"bucketId": "gemini-1",
						"window": "12h",
						"remainingFraction": 0.10,
						"resetTime": "2026-11-06T15:00:00Z"
					}
				]
			},
			{
				"displayName": "Claude 3.7",
				"buckets": [
					{
						"bucketId": "claude-1",
						"window": "24h",
						"remainingFraction": 0.85,
						"resetTime": "2026-11-06T20:00:00Z"
					}
				]
			},
			{
				"displayName": "Gemini 4 Argon",
				"buckets": [
					{
						"bucketId": "argon-1",
						"window": "24h",
						"remainingFraction": 0.02,
						"resetTime": "2026-11-06T18:00:00Z"
					}
				]
			}
		]
	}`
	if err := os.WriteFile(fixturePath, []byte(fixtureContent), 0644); err != nil {
		t.Fatal(err)
	}

	status, err := Status(fixturePath)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	gemini := status.Groups["gemini"]
	if gemini.Status != "low" {
		t.Errorf("expected gemini status 'low', got %s", gemini.Status)
	}
	if !gemini.Low {
		t.Errorf("expected gemini.Low true")
	}

	argon := status.Groups["argon"]
	if argon.Status != "critical" {
		t.Errorf("expected argon status 'critical', got %s", argon.Status)
	}
	if !argon.Critical {
		t.Errorf("expected argon.Critical true")
	}

	claude := status.Groups["claude"]
	if claude.Status != "healthy" {
		t.Errorf("expected claude status 'healthy', got %s", claude.Status)
	}

	if status.EarliestReset != "2026-11-06T15:00:00Z" {
		t.Errorf("expected earliest reset 2026-11-06T15:00:00Z, got %s", status.EarliestReset)
	}
}

func TestQuotaRouting(t *testing.T) {
	tempDir := t.TempDir()

	// 1. All healthy
	healthyPath := filepath.Join(tempDir, "healthy.json")
	healthyContent := `{
		"groups": [
			{ "displayName": "gemini", "buckets": [{ "remainingFraction": 0.90 }] },
			{ "displayName": "argon", "buckets": [{ "remainingFraction": 0.80 }] }
		]
	}`
	_ = os.WriteFile(healthyPath, []byte(healthyContent), 0644)

	rLow, err := Route(healthyPath, "low")
	if err != nil || rLow.Model != "flash_lite" {
		t.Errorf("expected flash_lite for low healthy, got %v, err=%v", rLow, err)
	}

	rMed, err := Route(healthyPath, "med")
	if err != nil || rMed.Model != "flash" {
		t.Errorf("expected flash for med healthy, got %v, err=%v", rMed, err)
	}

	rHigh, err := Route(healthyPath, "high")
	if err != nil || rHigh.Model != "pro" {
		t.Errorf("expected pro for high with healthy argon, got %v, err=%v", rHigh, err)
	}

	// 2. Argon degraded (0.10), Gemini healthy (0.50)
	argonDegradedPath := filepath.Join(tempDir, "argon_degraded.json")
	argonDegradedContent := `{
		"groups": [
			{ "displayName": "gemini", "buckets": [{ "remainingFraction": 0.50 }] },
			{ "displayName": "argon", "buckets": [{ "remainingFraction": 0.10 }] }
		]
	}`
	_ = os.WriteFile(argonDegradedPath, []byte(argonDegradedContent), 0644)

	rHighArgonDegraded, err := Route(argonDegradedPath, "high")
	if err != nil || rHighArgonDegraded.Model != "flash" {
		t.Errorf("expected flash for high when argon degraded, got %v, err=%v", rHighArgonDegraded, err)
	}

	// 3. Both degraded (gemini 0.05, argon 0.02)
	bothDegradedPath := filepath.Join(tempDir, "both_degraded.json")
	bothDegradedContent := `{
		"groups": [
			{ "displayName": "gemini", "buckets": [{ "remainingFraction": 0.05 }] },
			{ "displayName": "argon", "buckets": [{ "remainingFraction": 0.02 }] }
		]
	}`
	_ = os.WriteFile(bothDegradedPath, []byte(bothDegradedContent), 0644)

	rLowDegraded, _ := Route(bothDegradedPath, "low")
	if rLowDegraded.Model != "inherit" {
		t.Errorf("expected inherit for low when gemini degraded, got %v", rLowDegraded)
	}

	rMedDegraded, _ := Route(bothDegradedPath, "med")
	if rMedDegraded.Model != "inherit" {
		t.Errorf("expected inherit for med when gemini degraded, got %v", rMedDegraded)
	}

	rHighBothDegraded, _ := Route(bothDegradedPath, "high")
	if rHighBothDegraded.Model != "inherit" {
		t.Errorf("expected inherit for high when both degraded, got %v", rHighBothDegraded)
	}

	// 4. Invalid complexity
	if _, err := Route(healthyPath, "ultra"); err == nil {
		t.Errorf("expected error for invalid complexity")
	}
}

func TestQuotaErrors(t *testing.T) {
	if _, err := Status("/nonexistent/quota.json"); err == nil {
		t.Errorf("expected error for missing quota file")
	}

	tempDir := t.TempDir()
	badJSON := filepath.Join(tempDir, "bad.json")
	_ = os.WriteFile(badJSON, []byte("{invalid"), 0644)
	if _, err := Status(badJSON); err == nil {
		t.Errorf("expected error for malformed json")
	}
}

func TestQuotaNestedResponseAndGemini4Models(t *testing.T) {
	tempDir := t.TempDir()

	// Happy path: nested response.groups with Gemini 4 Hydrogen, Orchestrator (Argon), Claude Sonnet 5.5
	nestedPath := filepath.Join(tempDir, "nested.json")
	nestedContent := `{
		"response": {
			"groups": [
				{
					"displayName": "Gemini 4 Hydrogen (High)",
					"buckets": [
						{
							"bucketId": "gemini-4-hydrogen",
							"window": "5h",
							"remainingFraction": 0.72,
							"resetTime": "2026-11-14T05:00:00Z"
						}
					]
				},
				{
					"displayName": "gemini-pro-orchestrator-g1-pro",
					"buckets": [
						{
							"bucketId": "argon-orchestrator",
							"window": "weekly",
							"remainingFraction": 0.12,
							"resetTime": "2026-11-19T04:18:06Z"
						}
					]
				},
				{
					"displayName": "Claude Sonnet 5.5",
					"buckets": [
						{
							"bucketId": "claude-sonnet-5-5",
							"window": "weekly",
							"remainingFraction": 0.95,
							"resetTime": "2026-11-19T23:55:20Z"
						}
					]
				}
			]
		}
	}`
	if err := os.WriteFile(nestedPath, []byte(nestedContent), 0644); err != nil {
		t.Fatal(err)
	}

	status, err := Status(nestedPath)
	if err != nil {
		t.Fatalf("Status nested failed: %v", err)
	}
	if status.Groups["gemini"].RemainingPercent != 72.0 {
		t.Errorf("expected gemini 72.0%%, got %.1f%%", status.Groups["gemini"].RemainingPercent)
	}
	if status.Groups["argon"].Status != "low" || !status.Groups["argon"].Low {
		t.Errorf("expected argon low status for 0.12, got %+v", status.Groups["argon"])
	}
	if status.Groups["claude"].RemainingPercent != 95.0 {
		t.Errorf("expected claude 95.0%%, got %.1f%%", status.Groups["claude"].RemainingPercent)
	}

	// Guarded failure / safe degrade path: empty nested response degrades cleanly to healthy 1.0
	emptyNestedPath := filepath.Join(tempDir, "empty_nested.json")
	if err := os.WriteFile(emptyNestedPath, []byte(`{"response": {}}`), 0644); err != nil {
		t.Fatal(err)
	}
	emptyStatus, err := Status(emptyNestedPath)
	if err != nil {
		t.Fatalf("Status empty nested failed: %v", err)
	}
	for _, gid := range []string{"gemini", "claude", "argon"} {
		if emptyStatus.Groups[gid].RemainingFraction != 1.0 || emptyStatus.Groups[gid].Status != "healthy" {
			t.Errorf("expected safe degrade 1.0/healthy for %s, got %+v", gid, emptyStatus.Groups[gid])
		}
	}
}

func TestQuotaCarbonAndNeonModels(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Happy path: Gemini 4 Carbon & Orchestrator (Carbon) -> argon,
	// Gemini 4 Neon (High) & Orchestrator (Neon) -> gemini.
	happyPath := filepath.Join(tempDir, "carbon_neon_healthy.json")
	happyContent := `{
		"response": {
			"groups": [
				{
					"displayName": "Gemini 4 Neon (High)",
					"buckets": [
						{
							"bucketId": "gemini-4-neon-high",
							"window": "5h",
							"remainingFraction": 0.85,
							"resetTime": "2026-11-24T18:00:00Z"
						}
					]
				},
				{
					"displayName": "Orchestrator (Neon)",
					"buckets": [
						{
							"bucketId": "orchestrator-neon",
							"window": "5h",
							"remainingFraction": 0.70,
							"resetTime": "2026-11-24T17:00:00Z"
						}
					]
				},
				{
					"displayName": "Gemini 4 Carbon",
					"buckets": [
						{
							"bucketId": "gemini-4-carbon",
							"window": "weekly",
							"remainingFraction": 0.65,
							"resetTime": "2026-11-28T00:00:00Z"
						}
					]
				},
				{
					"displayName": "Orchestrator (Carbon)",
					"buckets": [
						{
							"bucketId": "orchestrator-carbon",
							"window": "weekly",
							"remainingFraction": 0.60,
							"resetTime": "2026-11-27T12:00:00Z"
						}
					]
				},
				{
					"displayName": "Claude Sonnet 5.5",
					"buckets": [
						{
							"bucketId": "claude-sonnet-5-5",
							"window": "weekly",
							"remainingFraction": 0.90,
							"resetTime": "2026-11-29T00:00:00Z"
						}
					]
				}
			]
		}
	}`
	if err := os.WriteFile(happyPath, []byte(happyContent), 0644); err != nil {
		t.Fatal(err)
	}

	status, err := Status(happyPath)
	if err != nil {
		t.Fatalf("Status carbon/neon happy failed: %v", err)
	}
	if status.Groups["gemini"].RemainingPercent != 70.0 || status.Groups["gemini"].Status != "healthy" {
		t.Errorf("expected gemini 70.0%%/healthy (from Orchestrator (Neon)), got %+v", status.Groups["gemini"])
	}
	if status.Groups["argon"].RemainingPercent != 60.0 || status.Groups["argon"].Status != "healthy" {
		t.Errorf("expected argon 60.0%%/healthy (from Orchestrator (Carbon)), got %+v", status.Groups["argon"])
	}

	rHigh, err := Route(happyPath, "high")
	if err != nil || rHigh.Model != "pro" || rHigh.Group != "argon" {
		t.Errorf("expected high -> pro/argon, got %+v, err=%v", rHigh, err)
	}
	rMed, err := Route(happyPath, "med")
	if err != nil || rMed.Model != "flash" || rMed.Group != "gemini" {
		t.Errorf("expected med -> flash/gemini, got %+v, err=%v", rMed, err)
	}
	rLow, err := Route(happyPath, "low")
	if err != nil || rLow.Model != "flash_lite" || rLow.Group != "gemini" {
		t.Errorf("expected low -> flash_lite/gemini, got %+v, err=%v", rLow, err)
	}

	// 2. Degraded Carbon (Orchestrator (Carbon) low), Neon healthy -> high falls back to flash/gemini
	carbonDegradedPath := filepath.Join(tempDir, "carbon_degraded.json")
	carbonDegradedContent := `{
		"groups": [
			{ "displayName": "gemini-pro-orchestrator-neon", "buckets": [{ "remainingFraction": 0.80 }] },
			{ "displayName": "Gemini 4 Neon (High)", "buckets": [{ "remainingFraction": 0.75 }] },
			{ "displayName": "Gemini 4 Carbon", "buckets": [{ "remainingFraction": 0.50 }] },
			{ "displayName": "Orchestrator (Carbon)", "buckets": [{ "remainingFraction": 0.10 }] }
		]
	}`
	if err := os.WriteFile(carbonDegradedPath, []byte(carbonDegradedContent), 0644); err != nil {
		t.Fatal(err)
	}

	cdStatus, err := Status(carbonDegradedPath)
	if err != nil {
		t.Fatalf("Status carbon degraded failed: %v", err)
	}
	if cdStatus.Groups["gemini"].RemainingPercent != 75.0 || cdStatus.Groups["gemini"].Status != "healthy" {
		t.Errorf("expected gemini 75.0%%/healthy, got %+v", cdStatus.Groups["gemini"])
	}
	if cdStatus.Groups["argon"].RemainingPercent != 10.0 || cdStatus.Groups["argon"].Status != "low" {
		t.Errorf("expected argon 10.0%%/low, got %+v", cdStatus.Groups["argon"])
	}

	rHighFallback, err := Route(carbonDegradedPath, "high")
	if err != nil || rHighFallback.Model != "flash" || rHighFallback.Group != "gemini" {
		t.Errorf("expected high -> flash/gemini when Carbon degraded, got %+v, err=%v", rHighFallback, err)
	}

	// 3. Guarded fallback: Orchestrator (Neon) critical (maps to gemini) & Gemini 4 Carbon critical (maps to argon)
	bothCriticalPath := filepath.Join(tempDir, "neon_carbon_critical.json")
	bothCriticalContent := `{
		"groups": [
			{ "displayName": "Orchestrator (Neon)", "buckets": [{ "remainingFraction": 0.04 }] },
			{ "displayName": "Gemini 4 Neon (High)", "buckets": [{ "remainingFraction": 0.90 }] },
			{ "displayName": "Gemini 4 Carbon", "buckets": [{ "remainingFraction": 0.03 }] }
		]
	}`
	if err := os.WriteFile(bothCriticalPath, []byte(bothCriticalContent), 0644); err != nil {
		t.Fatal(err)
	}

	bcStatus, err := Status(bothCriticalPath)
	if err != nil {
		t.Fatalf("Status both critical failed: %v", err)
	}
	if bcStatus.Groups["gemini"].Status != "critical" || bcStatus.Groups["gemini"].RemainingPercent != 4.0 {
		t.Errorf("expected gemini critical at 4.0%% from Orchestrator (Neon), got %+v", bcStatus.Groups["gemini"])
	}
	if bcStatus.Groups["argon"].Status != "critical" || bcStatus.Groups["argon"].RemainingPercent != 3.0 {
		t.Errorf("expected argon critical at 3.0%% from Gemini 4 Carbon, got %+v", bcStatus.Groups["argon"])
	}

	for _, tier := range []string{"low", "med", "high"} {
		r, err := Route(bothCriticalPath, tier)
		if err != nil || r.Model != "inherit" || r.Group != "inherit" {
			t.Errorf("expected inherit fallback for %s when both critical, got %+v, err=%v", tier, r, err)
		}
	}
}
