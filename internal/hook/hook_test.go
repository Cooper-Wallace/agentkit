package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeQuotaFixture(t *testing.T, payload any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "quota_fixture.json")
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestGuardQuota(t *testing.T) {
	t.Run("dry-run", func(t *testing.T) {
		var out bytes.Buffer
		code := GuardQuota([]string{"--dry-run"}, nil, &out)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var resp map[string]any
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v (%s)", err, out.String())
		}
		if resp["status"] != "dry-run" {
			t.Errorf("expected status dry-run, got %v", resp["status"])
		}
		if !strings.Contains(fmt.Sprintf("%v", resp["message"]), "guard-quota is operational") {
			t.Errorf("unexpected dry-run message: %v", resp["message"])
		}
	})

	t.Run("env bypass", func(t *testing.T) {
		for _, val := range []string{"0", "off"} {
			t.Setenv("AGENTKIT_QUOTA_GUARD", val)
			var out bytes.Buffer
			code := GuardQuota(nil, strings.NewReader(`{}`), &out)
			if code != 0 {
				t.Fatalf("expected exit 0, got %d", code)
			}
			var resp map[string]any
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if resp["decision"] != "allow" || !strings.Contains(fmt.Sprintf("%v", resp["reason"]), "AGENTKIT_QUOTA_GUARD") {
				t.Errorf("unexpected bypass response for %q: %+v", val, resp)
			}
		}
	})

	t.Run("all groups critical triggers wrapup deny", func(t *testing.T) {
		fixture := writeQuotaFixture(t, map[string]any{
			"groups": []map[string]any{
				{"displayName": "gemini", "buckets": []map[string]any{{"remainingFraction": 0.02}}},
				{"displayName": "claude", "buckets": []map[string]any{{"remainingFraction": 0.01}}},
				{"displayName": "argon", "buckets": []map[string]any{{"remainingFraction": 0.00}}},
			},
		})
		t.Setenv("AGENTKIT_QUOTA_FIXTURE", fixture)
		t.Setenv("AGENTKIT_QUOTA_CACHE", filepath.Join(t.TempDir(), "cache.json"))

		payload := `{"toolCall":{"name":"invoke_subagent","args":{"Model":"flash","Prompt":"Write code"}}}`
		var out bytes.Buffer
		code := GuardQuota([]string{"--test", payload}, nil, &out)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var resp map[string]any
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp["decision"] != "deny" {
			t.Errorf("expected deny, got %v", resp["decision"])
		}
		if !strings.Contains(fmt.Sprintf("%v", resp["reason"]), "QUOTA WRAP-UP:") {
			t.Errorf("expected QUOTA WRAP-UP reason, got %v", resp["reason"])
		}
	})

	t.Run("complexity routing and protected scope upgrade", func(t *testing.T) {
		healthyFixture := writeQuotaFixture(t, map[string]any{
			"groups": []map[string]any{
				{"displayName": "gemini", "buckets": []map[string]any{{"remainingFraction": 0.90}}},
				{"displayName": "claude", "buckets": []map[string]any{{"remainingFraction": 0.90}}},
				{"displayName": "argon", "buckets": []map[string]any{{"remainingFraction": 0.90}}},
			},
		})
		t.Setenv("AGENTKIT_QUOTA_FIXTURE", healthyFixture)
		t.Setenv("AGENTKIT_QUOTA_CACHE", filepath.Join(t.TempDir(), "cache.json"))

		cases := []struct {
			name      string
			payload   string
			wantModel string
		}{
			{
				name:      "low complexity",
				payload:   `{"Prompt":"[complexity: low] rename variable"}`,
				wantModel: "flash_lite",
			},
			{
				name:      "low complexity upgraded on test",
				payload:   `{"Model":"inherit","Prompt":"[complexity: low] add unit test coverage"}`,
				wantModel: "flash",
			},
			{
				name:      "low complexity upgraded on security",
				payload:   `{"Prompt":"[complexity: low] check security headers"}`,
				wantModel: "flash",
			},
			{
				name:      "med complexity",
				payload:   `{"toolCall":{"args":{"Prompt":"[complexity: med] implement endpoint"}}}`,
				wantModel: "flash",
			},
			{
				name:      "high complexity escalates to pro when argon healthy",
				payload:   `{"toolCall":{"args":{"Model":"inherit","Prompt":"[complexity: high] design architecture"}}}`,
				wantModel: "pro",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var out bytes.Buffer
				code := GuardQuota(nil, strings.NewReader(tc.payload), &out)
				if code != 0 {
					t.Fatalf("expected exit 0, got %d", code)
				}
				var resp guardQuotaResponse
				if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if resp.Decision != "allow" {
					t.Errorf("expected allow, got %q", resp.Decision)
				}
				if resp.Overwrite["Model"] != tc.wantModel {
					t.Errorf("expected overwrite Model=%q, got %+v", tc.wantModel, resp.Overwrite)
				}
			})
		}

		// When Argon is degraded, high complexity stays on flash
		argonDegraded := writeQuotaFixture(t, map[string]any{
			"groups": []map[string]any{
				{"displayName": "gemini", "buckets": []map[string]any{{"remainingFraction": 0.90}}},
				{"displayName": "claude", "buckets": []map[string]any{{"remainingFraction": 0.90}}},
				{"displayName": "argon", "buckets": []map[string]any{{"remainingFraction": 0.10}}},
			},
		})
		t.Setenv("AGENTKIT_QUOTA_FIXTURE", argonDegraded)
		var out bytes.Buffer
		GuardQuota([]string{"--test", `{"Prompt":"[complexity: high] design system"}`}, nil, &out)
		var resp guardQuotaResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Overwrite["Model"] != "flash" {
			t.Errorf("expected flash when argon degraded, got %+v", resp)
		}
	})

	t.Run("model group mapping neon before orchestrator", func(t *testing.T) {
		cases := map[string]string{
			"flash_lite":                   "gemini",
			"flash":                        "gemini",
			"pro":                          "argon",
			"gemini-4-neon-high":           "gemini",
			"gemini-pro-orchestrator-neon": "gemini",
			"Orchestrator (Neon)":          "gemini",
			"gemini-4-carbon-high":         "argon",
			"gemini-pro-orchestrator":      "argon",
			"Orchestrator (Carbon)":        "argon",
			"gemini-4-argon":               "argon",
			"claude-sonnet-5-5":            "claude",
			"claude-opus-5-5":              "claude",
			"gpt-oss-120b":                 "claude",
			"gemini-4-hydrogen-high":       "gemini",
			"inherit":                      "",
			"":                             "",
		}
		for model, want := range cases {
			if got := resolveModelGroup(model, nil); got != want {
				t.Errorf("resolveModelGroup(%q) = %q, want %q", model, got, want)
			}
		}
	})

	t.Run("degraded quota reroute and root group detection", func(t *testing.T) {
		geminiDegraded := writeQuotaFixture(t, map[string]any{
			"groups": []map[string]any{
				{"displayName": "gemini", "buckets": []map[string]any{{"remainingFraction": 0.10, "resetTime": "2026-11-25T18:00:00Z"}}},
				{"displayName": "argon", "buckets": []map[string]any{{"remainingFraction": 0.85}}},
				{"displayName": "claude", "buckets": []map[string]any{{"remainingFraction": 0.90}}},
			},
		})
		t.Setenv("AGENTKIT_QUOTA_FIXTURE", geminiDegraded)
		t.Setenv("AGENTKIT_QUOTA_CACHE", filepath.Join(t.TempDir(), "cache.json"))

		// Root model in degraded gemini group -> skips inherit, reroutes to argon/pro
		payloadWithRoot := `{"modelName":"gemini-4-neon-high","toolCall":{"args":{"Model":"flash","Prompt":"Implement feature"}}}`
		var out1 bytes.Buffer
		GuardQuota([]string{"--test", payloadWithRoot}, nil, &out1)
		var resp1 guardQuotaResponse
		if err := json.Unmarshal(out1.Bytes(), &resp1); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp1.Overwrite["Model"] != "pro" {
			t.Errorf("expected reroute to pro, got %+v", resp1)
		}
		if !strings.Contains(resp1.Reason, "Quota degraded for gemini (10% remaining, resets 2026-11-25T18:00:00Z)") {
			t.Errorf("unexpected reason: %q", resp1.Reason)
		}

		// Root model read from ~/.gemini/config/orchestration.json (claude -> inherit healthy)
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		orchDir := filepath.Join(tmpHome, ".gemini", "config")
		if err := os.MkdirAll(orchDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(orchDir, "orchestration.json"), []byte(`{"root_model":"claude-sonnet-5-5"}`), 0644); err != nil {
			t.Fatal(err)
		}
		payloadNoRoot := `{"toolCall":{"args":{"Model":"flash","Prompt":"Implement feature"}}}`
		var out2 bytes.Buffer
		GuardQuota([]string{"--test", payloadNoRoot}, nil, &out2)
		var resp2 guardQuotaResponse
		if err := json.Unmarshal(out2.Bytes(), &resp2); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp2.Overwrite["Model"] != "inherit" || !strings.Contains(resp2.Reason, "(claude group)") {
			t.Errorf("expected reroute to inherit (claude group), got %+v", resp2)
		}
	})

	t.Run("malformed stdin fails open cleanly", func(t *testing.T) {
		healthyFixture := writeQuotaFixture(t, map[string]any{
			"groups": []map[string]any{
				{"displayName": "gemini", "buckets": []map[string]any{{"remainingFraction": 0.90}}},
			},
		})
		t.Setenv("AGENTKIT_QUOTA_FIXTURE", healthyFixture)
		t.Setenv("AGENTKIT_QUOTA_CACHE", filepath.Join(t.TempDir(), "cache.json"))

		var out bytes.Buffer
		code := GuardQuota(nil, strings.NewReader("{bad json"), &out)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var resp guardQuotaResponse
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp.Decision != "allow" {
			t.Errorf("expected allow, got %q", resp.Decision)
		}
	})
}

func TestNotifyWake(t *testing.T) {
	t.Run("dry-run", func(t *testing.T) {
		var out bytes.Buffer
		code := NotifyWake([]string{"--dry-run"}, nil, &out)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var resp map[string]any
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp["status"] != "dry-run" {
			t.Errorf("expected status dry-run, got %v", resp["status"])
		}
	})

	t.Run("offline wake server fails open silently", func(t *testing.T) {
		// Bind and immediately close to get an unreachable ephemeral port
		srv := httptest.NewServer(http.NewServeMux())
		u, _ := url.Parse(srv.URL)
		portStr := u.Port()
		srv.Close()

		var out bytes.Buffer
		payload := `{"conversationId":"sess-offline","toolCall":{"name":"run_command"}}`
		code := NotifyWake([]string{"--port", portStr}, strings.NewReader(payload), &out)
		if code != 0 {
			t.Fatalf("expected exit 0 on fail-open, got %d", code)
		}
		var resp map[string]string
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp["decision"] != "allow" {
			t.Errorf("expected allow, got %v", resp)
		}
	})

	t.Run("live httptest wake server POSTs for toolCall, stop, and test mode", func(t *testing.T) {
		var received []wakeNotificationPayload
		var receivedPaths []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPaths = append(receivedPaths, r.URL.Path)
			body, _ := io.ReadAll(r.Body)
			var p wakeNotificationPayload
			if err := json.Unmarshal(body, &p); err == nil {
				received = append(received, p)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		portStr := u.Port()

		// 1. PreToolUse payload -> active + allow
		var outPre bytes.Buffer
		prePayload := `{"conversationId":"conv-abc","toolCall":{"name":"run_command"}}`
		if code := NotifyWake([]string{"active", "--port", portStr}, strings.NewReader(prePayload), &outPre); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var respPre map[string]string
		_ = json.Unmarshal(outPre.Bytes(), &respPre)
		if respPre["decision"] != "allow" {
			t.Errorf("expected decision allow, got %v", respPre)
		}

		// 2. Stop payload (state inferred as idle from terminationReason) -> idle + stop
		var outStop bytes.Buffer
		stopPayload := `{"conversationId":"conv-abc","terminationReason":"COMPLETE"}`
		if code := NotifyWake([]string{"--port=" + portStr}, strings.NewReader(stopPayload), &outStop); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var respStop map[string]string
		_ = json.Unmarshal(outStop.Bytes(), &respStop)
		if respStop["decision"] != "stop" {
			t.Errorf("expected decision stop, got %v", respStop)
		}

		// 3. --test mode ignores stdin and outputs {}
		t.Setenv("AGENTKIT_WAKE_PORT", portStr)
		var outTest bytes.Buffer
		if code := NotifyWake([]string{"--test", "active", "--session", "sess-cli", "--agent", "agy", "--task", "testing"}, strings.NewReader(`{"toolCall":{"name":"ignored"}}`), &outTest); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if strings.TrimSpace(outTest.String()) != "{}" {
			t.Errorf("expected empty JSON object {}, got %q", outTest.String())
		}

		if len(received) != 3 {
			t.Fatalf("expected 3 HTTP POSTs, got %d", len(received))
		}
		for i, p := range receivedPaths {
			if p != "/v1/notify" {
				t.Errorf("request %d path = %q, want /v1/notify", i, p)
			}
		}
		if received[0].SessionID != "conv-abc" || received[0].State != "active" || received[0].Task != "run_command" {
			t.Errorf("unexpected pre payload: %+v", received[0])
		}
		if received[1].SessionID != "conv-abc" || received[1].State != "idle" {
			t.Errorf("unexpected stop payload: %+v", received[1])
		}
		if received[2].SessionID != "sess-cli" || received[2].Agent != "agy" || received[2].Task != "testing" || received[2].PID <= 0 {
			t.Errorf("unexpected test payload: %+v", received[2])
		}
	})
}

func TestFilterAndWrap(t *testing.T) {
	t.Run("dry-run", func(t *testing.T) {
		var out bytes.Buffer
		code := FilterAndWrap([]string{"--dry-run"}, nil, &out)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var resp filterWrapResponse
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp.Status != "dry-run" {
			t.Errorf("expected status dry-run, got %q", resp.Status)
		}
	})

	t.Run("disable toggle", func(t *testing.T) {
		t.Setenv("AGY_DISABLE_TOKEN_COMPRESSION", "1")
		var out bytes.Buffer
		code := FilterAndWrap([]string{"--test", "git push origin main"}, nil, &out)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var resp filterWrapResponse
		_ = json.Unmarshal(out.Bytes(), &resp)
		if resp.Decision != "allow" || len(resp.Overwrite) > 0 {
			t.Errorf("expected plain allow when disabled, got %+v", resp)
		}
	})

	t.Run("safety preservation", func(t *testing.T) {
		denyCases := []string{
			"git push origin main",
			"git push -f origin feat/test",
			"git push origin feat/test --force",
			"git merge master",
			"git checkout main && git merge feat/x",
		}
		for _, cmd := range denyCases {
			var out bytes.Buffer
			FilterAndWrap([]string{"--test", cmd}, nil, &out)
			var resp filterWrapResponse
			_ = json.Unmarshal(out.Bytes(), &resp)
			if resp.Decision != "deny" || !strings.HasPrefix(resp.Reason, "SECURITY VIOLATION:") {
				t.Errorf("cmd %q: expected deny/SECURITY VIOLATION, got %+v", cmd, resp)
			}
		}

		askCases := []string{
			"rm -rf /tmp/x",
			"rm -fr ./build",
			"dd if=/dev/zero of=/tmp/img bs=1M count=1",
			"mkfs /dev/sda1",
		}
		for _, cmd := range askCases {
			var out bytes.Buffer
			FilterAndWrap([]string{"--test", cmd}, nil, &out)
			var resp filterWrapResponse
			_ = json.Unmarshal(out.Bytes(), &resp)
			if resp.Decision != "ask" || !strings.HasPrefix(resp.Reason, "DESTRUCTIVE OPERATION DETECTED:") {
				t.Errorf("cmd %q: expected ask/DESTRUCTIVE OPERATION DETECTED, got %+v", cmd, resp)
			}
		}
	})

	t.Run("verbose command rewrite", func(t *testing.T) {
		verboseCmds := []string{
			"pytest tests -v",
			"go test ./internal/hook/... -v",
			"git diff HEAD~1",
			"git log -n 20",
			"grep -rn pattern src/",
			"find . -name '*.go'",
		}
		for _, cmd := range verboseCmds {
			var out bytes.Buffer
			FilterAndWrap([]string{"--test", cmd}, nil, &out)
			var resp filterWrapResponse
			_ = json.Unmarshal(out.Bytes(), &resp)
			want := "agentkit hook headroom-wrap -- " + cmd
			if resp.Decision != "allow" || resp.Overwrite["CommandLine"] != want {
				t.Errorf("cmd %q: expected overwrite %q, got %+v", cmd, want, resp)
			}
		}
	})

	t.Run("skip conditions", func(t *testing.T) {
		skipPayloads := []struct {
			name    string
			payload string
		}{
			{"non-run_command", `{"toolCall":{"name":"edit_file","args":{"CommandLine":"pytest"}}}`},
			{"empty command", `{"toolCall":{"name":"run_command","args":{"CommandLine":"   "}}}`},
			{"daemon", `{"toolCall":{"name":"run_command","args":{"CommandLine":"pytest -v","IsDaemon":true}}}`},
			{"already headroom-wrap", `{"toolCall":{"name":"run_command","args":{"CommandLine":"agentkit hook headroom-wrap -- pytest -v"}}}`},
			{"already headroom-wrap.py", `{"toolCall":{"name":"run_command","args":{"CommandLine":"python3 headroom-wrap.py -- pytest -v"}}}`},
			{"pipe", `{"toolCall":{"name":"run_command","args":{"CommandLine":"git diff | head -n 20"}}}`},
			{"redirect", `{"toolCall":{"name":"run_command","args":{"CommandLine":"go test ./... > out.txt"}}}`},
			{"chain", `{"toolCall":{"name":"run_command","args":{"CommandLine":"go test ./... && echo done"}}}`},
			{"git commit", `{"toolCall":{"name":"run_command","args":{"CommandLine":"git commit -m 'test'"}}}`},
			{"git checkout", `{"toolCall":{"name":"run_command","args":{"CommandLine":"git checkout -b feat/diff"}}}`},
			{"git status", `{"toolCall":{"name":"run_command","args":{"CommandLine":"git status"}}}`},
		}
		for _, tc := range skipPayloads {
			t.Run(tc.name, func(t *testing.T) {
				var out bytes.Buffer
				code := FilterAndWrap(nil, strings.NewReader(tc.payload), &out)
				if code != 0 {
					t.Fatalf("expected exit 0, got %d", code)
				}
				var resp filterWrapResponse
				_ = json.Unmarshal(out.Bytes(), &resp)
				if resp.Decision != "allow" || len(resp.Overwrite) > 0 {
					t.Errorf("expected allow without overwrite, got %+v", resp)
				}
			})
		}
	})
}

func TestHeadroomWrap(t *testing.T) {
	t.Run("dry-run", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := HeadroomWrap([]string{"--dry-run"}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		var resp map[string]string
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp["status"] != "dry-run" {
			t.Errorf("expected status dry-run, got %v", resp)
		}
	})

	t.Run("short output passthrough and non-zero exit preservation", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		var out, errBuf bytes.Buffer
		code := HeadroomWrap([]string{"--", "echo", "hello headroom"}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if strings.TrimSpace(out.String()) != "hello headroom" {
			t.Errorf("unexpected output: %q", out.String())
		}
		if strings.Contains(out.String(), "[Headroom CCR:") {
			t.Errorf("short output should not be compressed")
		}

		out.Reset()
		errBuf.Reset()
		codeErr := HeadroomWrap([]string{"--", "sh", "-c", "echo failing >&2; exit 7"}, &out, &errBuf)
		if codeErr != 7 {
			t.Errorf("expected exit code 7 preserved, got %d", codeErr)
		}
		if !strings.Contains(out.String(), "failing") {
			t.Errorf("expected stderr captured in combined output, got %q", out.String())
		}
	})

	t.Run("disable toggle skips compression", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("AGY_DISABLE_TOKEN_COMPRESSION", "1")
		var out, errBuf bytes.Buffer
		longStr := strings.Repeat("a", 2000)
		code := HeadroomWrap([]string{"--", "printf", "%s", longStr}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if out.String() != longStr {
			t.Errorf("expected uncompressed output of len %d, got len %d", len(longStr), out.Len())
		}
	})

	t.Run("JSON SmartCrusher array and object", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("AGY_COMPRESSION_THRESHOLD", "200")

		items := make([]map[string]any, 40)
		for i := range items {
			items[i] = map[string]any{
				"index":   i,
				"name":    fmt.Sprintf("node_%d", i),
				"payload": strings.Repeat("x", 40),
			}
		}
		jsonBytes, _ := json.MarshalIndent(items, "", "  ")
		jsonFile := filepath.Join(t.TempDir(), "data.json")
		if err := os.WriteFile(jsonFile, jsonBytes, 0644); err != nil {
			t.Fatal(err)
		}

		var out, errBuf bytes.Buffer
		code := HeadroomWrap([]string{"--", "cat", jsonFile}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d (stderr: %s)", code, errBuf.String())
		}
		output := out.String()
		if !strings.Contains(output, "_headroom_omitted") {
			t.Errorf("expected _headroom_omitted in JSON crushed output:\n%s", output)
		}
		if !strings.Contains(output, "[Headroom CCR:") {
			t.Errorf("expected Headroom CCR breadcrumb")
		}

		// Also verify object with nested long slice
		obj := map[string]any{"status": "ok", "items": items}
		objBytes, _ := json.MarshalIndent(obj, "", "  ")
		crushedObj, ok := crushJSON(string(objBytes))
		if !ok || !strings.Contains(crushedObj, "_headroom_omitted") {
			t.Errorf("expected object JSON SmartCrusher to omit items, ok=%v", ok)
		}
	})

	t.Run("Git Diff Structural Compressor", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		var lines []string
		lines = append(lines,
			"diff --git a/foo.go b/foo.go",
			"index 1234567..89abcdef 100644",
			"--- a/foo.go",
			"+++ b/foo.go",
			"@@ -1,40 +1,40 @@ func one() {",
		)
		for i := 0; i < 35; i++ {
			lines = append(lines, fmt.Sprintf("+    lineHunk1_%03d := \"payload data %03d\"", i, i))
		}
		lines = append(lines, "@@ -50,40 +50,40 @@ func two() {")
		for i := 0; i < 35; i++ {
			lines = append(lines, fmt.Sprintf("+    lineHunk2_%03d := \"payload data %03d\"", i, i))
		}
		diffFile := filepath.Join(t.TempDir(), "test.diff")
		if err := os.WriteFile(diffFile, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		var out, errBuf bytes.Buffer
		code := HeadroomWrap([]string{"--", "cat", diffFile}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		output := out.String()
		for _, want := range []string{
			"diff --git a/foo.go b/foo.go",
			"@@ -1,40 +1,40 @@ func one() {",
			"@@ -50,40 +50,40 @@ func two() {",
			"hunk lines compressed by Headroom CCR",
			"[Headroom CCR:",
		} {
			if !strings.Contains(output, want) {
				t.Errorf("expected %q in diff output", want)
			}
		}
	})

	t.Run("Error-Preserving Log Crusher and CCR Cache", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		var lines []string
		for i := 0; i < 40; i++ {
			lines = append(lines, fmt.Sprintf("test_%03d (suite.Case) ... ok [duration=0.012s]", i))
		}
		lines = append(lines, "FAIL: test_important_case (suite.CriticalSuite)")
		for i := 41; i < 85; i++ {
			lines = append(lines, fmt.Sprintf("test_%03d (suite.Case) ... ok [duration=0.012s]", i))
		}
		logContent := strings.Join(lines, "\n") + "\n"
		logFile := filepath.Join(t.TempDir(), "test.log")
		if err := os.WriteFile(logFile, []byte(logContent), 0644); err != nil {
			t.Fatal(err)
		}

		var out, errBuf bytes.Buffer
		code := HeadroomWrap([]string{"--", "cat", logFile}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		output := out.String()
		if !strings.Contains(output, "FAIL: test_important_case") {
			t.Errorf("expected critical FAIL line preserved")
		}
		if !strings.Contains(output, "routine lines compressed by Headroom CCR") {
			t.Errorf("expected routine lines divider")
		}
		re := regexp.MustCompile(`Full output cached at (/[^\s\]]+\.txt)`)
		m := re.FindStringSubmatch(output)
		if len(m) != 2 {
			t.Fatalf("expected cache file path in breadcrumb: %s", output)
		}
		cached, err := os.ReadFile(m[1])
		if err != nil {
			t.Fatalf("failed to read cached file %s: %v", m[1], err)
		}
		if string(cached) != logContent {
			t.Errorf("cached file content mismatch")
		}
	})

	t.Run("char truncation for dense single-line output and cache pruning", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		cacheDir := filepath.Join(tmpHome, ".cache", "antigravity-headroom")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Pre-seed 105 cache files with staggered mod times
		baseTime := time.Now().Add(-200 * time.Minute)
		for i := 0; i < 105; i++ {
			p := filepath.Join(cacheDir, fmt.Sprintf("old_%03d.txt", i))
			_ = os.WriteFile(p, []byte("old"), 0644)
			ts := baseTime.Add(time.Duration(i) * time.Minute)
			_ = os.Chtimes(p, ts, ts)
		}

		dense := strings.Repeat("Z", 2000)
		denseFile := filepath.Join(t.TempDir(), "dense.txt")
		_ = os.WriteFile(denseFile, []byte(dense), 0644)

		var out, errBuf bytes.Buffer
		code := HeadroomWrap([]string{"--", "cat", denseFile}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !strings.Contains(out.String(), "800 chars compressed by Headroom CCR") {
			t.Errorf("expected 800 chars compressed divider, got: %s", out.String())
		}

		entries, err := os.ReadDir(cacheDir)
		if err != nil {
			t.Fatal(err)
		}
		txtCount := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".txt") {
				txtCount++
			}
		}
		if txtCount != 100 {
			t.Errorf("expected cache pruned to 100 .txt files, got %d", txtCount)
		}
	})

	t.Run("execution failure guarded", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := HeadroomWrap([]string{"--", "/nonexistent/binary/xyz_123", "arg1"}, &out, &errBuf)
		if code == 0 {
			t.Errorf("expected non-zero exit code for missing binary, got 0")
		}
		_ = strconv.Itoa(code)
	})
}
