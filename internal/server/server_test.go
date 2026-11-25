package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Cooper-Wallace/agentkit/internal/caffeine"
)

func startTestServer(t *testing.T) (*Server, func()) {
	mockInh := caffeine.NewMockInhibitor()
	mockHW := caffeine.NewMockHardwareReader()

	cfg := caffeine.DefaultEngineConfig()
	cfg.CooldownDuration = 30 * time.Millisecond
	cfg.ApprovalGraceDuration = 50 * time.Millisecond
	cfg.BreakerCheckInterval = 10 * time.Millisecond

	eng := caffeine.NewEngine(cfg, mockInh, mockHW)

	sCfg := ServerConfig{
		Host: "127.0.0.1",
		Port: 0, // OS assigns available ephemeral port
	}

	srv := NewServer(sCfg, eng)
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
		_ = eng.Close()
	}

	return srv, cleanup
}

func TestServer_StatusEndpoints(t *testing.T) {
	srv, cleanup := startTestServer(t)
	defer cleanup()

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	for _, endpoint := range []string{"/status", "/v1/status"} {
		resp, err := http.Get(baseURL + endpoint)
		if err != nil {
			t.Fatalf("GET %s error: %v", endpoint, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", endpoint, resp.StatusCode)
		}

		var st caffeine.EngineStatus
		if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
			t.Fatalf("decode status error: %v", err)
		}
		_ = resp.Body.Close()

		if st.State != caffeine.StateIdle {
			t.Fatalf("expected initial state IDLE, got %s", st.State)
		}
	}
}

func TestServer_NotifyEndpoints(t *testing.T) {
	srv, cleanup := startTestServer(t)
	defer cleanup()

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// 1. POST /notify with active state
	payload := map[string]interface{}{
		"session_id": "test-session-1",
		"state":      "active",
		"agent":      "test-agent",
		"task":       "coding",
		"pid":        4242,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/notify", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /notify error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var st caffeine.EngineStatus
	_ = json.NewDecoder(resp.Body).Decode(&st)
	_ = resp.Body.Close()

	if st.State != caffeine.StateActive || !st.Inhibited {
		t.Fatalf("expected ACTIVE and inhibited=true, got %s", st.State)
	}

	// 2. GET /v1/notify with query parameters
	getURL := fmt.Sprintf("%s/v1/notify?session_id=test-session-2&state=waiting_approval&agent=subagent&task=approval&pid=123", baseURL)
	resp, err = http.Get(getURL)
	if err != nil {
		t.Fatalf("GET /v1/notify error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 3. POST /notify idle
	body, _ = json.Marshal(map[string]string{
		"session_id": "test-session-1",
		"state":      "idle",
	})
	resp, _ = http.Post(baseURL+"/notify", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 4. Bad request checks
	badBody, _ := json.Marshal(map[string]string{
		"session_id": "",
	})
	resp, _ = http.Post(baseURL+"/notify", "application/json", bytes.NewReader(badBody))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty session_id, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	badStateBody, _ := json.Marshal(map[string]string{
		"session_id": "s1",
		"state":      "invalid-state",
	})
	resp, _ = http.Post(baseURL+"/notify", "application/json", bytes.NewReader(badStateBody))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid state, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestServer_OverrideAndCancel(t *testing.T) {
	srv, cleanup := startTestServer(t)
	defer cleanup()

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// 1. POST /override on
	body, _ := json.Marshal(map[string]string{
		"state":    "on",
		"duration": "1h",
	})
	resp, err := http.Post(baseURL+"/override", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /override error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var st caffeine.EngineStatus
	_ = json.NewDecoder(resp.Body).Decode(&st)
	_ = resp.Body.Close()

	if st.State != caffeine.StateOverride || !st.OverrideActive {
		t.Fatalf("expected OVERRIDE, got %s", st.State)
	}

	// 2. POST /cancel
	resp, err = http.Post(baseURL+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /cancel error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_ = json.NewDecoder(resp.Body).Decode(&st)
	_ = resp.Body.Close()

	if st.State != caffeine.StateIdle || st.OverrideActive {
		t.Fatalf("expected IDLE after cancel, got %s", st.State)
	}

	// 3. GET /v1/override with query parameters
	resp, err = http.Get(baseURL + "/v1/override?enabled=true&duration_seconds=3600")
	if err != nil {
		t.Fatalf("GET /v1/override error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 4. Invalid duration
	body, _ = json.Marshal(map[string]string{
		"state":    "on",
		"duration": "invalid-dur",
	})
	resp, _ = http.Post(baseURL+"/override", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid duration, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestServer_HistoryEndpoint(t *testing.T) {
	srv, cleanup := startTestServer(t)
	defer cleanup()

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// Generate some history
	_, _ = http.Post(baseURL+"/notify", "application/json", strings.NewReader(`{"session_id":"h1","state":"active"}`))
	_, _ = http.Post(baseURL+"/notify", "application/json", strings.NewReader(`{"session_id":"h1","state":"idle"}`))

	resp, err := http.Get(baseURL + "/history?limit=10")
	if err != nil {
		t.Fatalf("GET /history error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var entries []caffeine.JournalEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode history error: %v", err)
	}
	_ = resp.Body.Close()

	if len(entries) == 0 {
		t.Fatalf("expected at least 1 history entry")
	}
}

func TestServer_EventsSSE(t *testing.T) {
	srv, cleanup := startTestServer(t)
	defer cleanup()

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	req, err := http.NewRequest("GET", baseURL+"/events", nil)
	if err != nil {
		t.Fatalf("new request error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect to SSE error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for SSE, got %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	// Read initial state_change event
	line1, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read SSE event line error: %v", err)
	}
	if !strings.Contains(line1, "event: state_change") {
		t.Fatalf("expected event: state_change, got %s", line1)
	}

	line2, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read SSE data line error: %v", err)
	}
	if !strings.Contains(line2, "data:") {
		t.Fatalf("expected data line, got %s", line2)
	}

	// Trigger a state change via /notify and verify it arrives via SSE
	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = http.Post(baseURL+"/notify", "application/json", strings.NewReader(`{"session_id":"sse-test","state":"active"}`))
	}()

	// Read blank line delimiter
	_, _ = reader.ReadString('\n')

	// Read next event line
	evLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read triggered event line error: %v", err)
	}
	if !strings.Contains(evLine, "event: state_change") {
		t.Fatalf("expected event: state_change, got %s", evLine)
	}
}

func TestServer_DefaultsAndMethods(t *testing.T) {
	srv := NewServer(DefaultServerConfig(), nil)
	if srv.Engine() == nil {
		t.Fatalf("expected default engine created")
	}
	if srv.Port() != 18820 {
		t.Fatalf("expected default port 18820, got %d", srv.Port())
	}
	_ = srv.Stop(context.Background())
}
