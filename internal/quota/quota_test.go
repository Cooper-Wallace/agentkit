package quota

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuotaStatusSyntheticFallback(t *testing.T) {
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
