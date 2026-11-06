package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Cooper-Wallace/agentkit/internal/caffeine"
	"github.com/Cooper-Wallace/agentkit/internal/server"
)

func startCLITestServer(t *testing.T) (*server.Server, func()) {
	mockInh := caffeine.NewMockInhibitor()
	mockHW := caffeine.NewMockHardwareReader()

	cfg := caffeine.DefaultEngineConfig()
	eng := caffeine.NewEngine(cfg, mockInh, mockHW)

	sCfg := server.ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}

	srv := server.NewServer(sCfg, eng)
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
		_ = eng.Close()
	}

	return srv, cleanup
}

func TestCLI_CaffeineHelpAndUnknown(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// No args -> prints help, returns 1
	code := RunCaffeine("caffeine", []string{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for no args, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: agentkit caffeine") {
		t.Fatalf("expected help text in stdout")
	}

	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0 for help, got %d", code)
	}

	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"invalid-subcommand"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for unknown subcommand, got %d", code)
	}
}

func TestCLI_WakeAndSleepShorthands(t *testing.T) {
	srv, cleanup := startCLITestServer(t)
	defer cleanup()

	portFlag := fmt.Sprintf("--port=%d", srv.Port())

	// 1. agentkit wake
	var stdout, stderr bytes.Buffer
	code := RunCaffeine("wake", []string{portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("wake failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Manual sleep lock enabled indefinitely") {
		t.Fatalf("unexpected wake output: %s", stdout.String())
	}

	// 2. agentkit wake 30m
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("wake", []string{"30m", portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("wake 30m failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "30m") {
		t.Fatalf("expected 30m in output: %s", stdout.String())
	}

	// 3. agentkit sleep
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("sleep", []string{portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("sleep failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Manual sleep lock disabled") {
		t.Fatalf("unexpected sleep output: %s", stdout.String())
	}
}

func TestCLI_Status(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// 1. Status with daemon offline
	code := RunCaffeine("caffeine", []string{"status", "--port=39999"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 when daemon offline, got %d", code)
	}
	if !strings.Contains(stderr.String(), "OFFLINE") {
		t.Fatalf("expected OFFLINE message in stderr")
	}

	// Offline with --json
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"status", "--port=39999", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	var offlineMap map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &offlineMap); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if offlineMap["state"] != "OFFLINE" {
		t.Fatalf("expected state OFFLINE in JSON")
	}

	// 2. Status with daemon online
	srv, cleanup := startCLITestServer(t)
	defer cleanup()

	portFlag := fmt.Sprintf("--port=%d", srv.Port())
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"status", portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "AgentKit Caffeine Status") {
		t.Fatalf("unexpected status output: %s", stdout.String())
	}

	// Status online with --json
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"status", portFlag, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0 with --json")
	}
	var stMap map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &stMap); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
}

func TestCLI_NotifyAndCancel(t *testing.T) {
	srv, cleanup := startCLITestServer(t)
	defer cleanup()

	portFlag := fmt.Sprintf("--port=%d", srv.Port())
	var stdout, stderr bytes.Buffer

	// 1. Notify without state -> error
	code := RunCaffeine("caffeine", []string{"notify", portFlag}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for missing state")
	}

	// 2. Notify active
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"notify", "--state", "active", "--session-id", "s-1", "--agent", "test", portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("notify active failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Session 's-1' -> active") {
		t.Fatalf("unexpected notify output: %s", stdout.String())
	}

	// 3. Cancel
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"cancel", portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cancel failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Cancelled active cooldown/override") {
		t.Fatalf("unexpected cancel output: %s", stdout.String())
	}
}

func TestCLI_OverrideErrors(t *testing.T) {
	srv, cleanup := startCLITestServer(t)
	defer cleanup()

	portFlag := fmt.Sprintf("--port=%d", srv.Port())
	var stdout, stderr bytes.Buffer

	// Missing --on or --off
	code := RunCaffeine("caffeine", []string{"override", portFlag}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for missing --on/--off")
	}

	// Invalid duration
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"override", "--on", "--duration", "invalid-dur", portFlag}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for invalid duration")
	}
}

func TestCLI_OfflineErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	offlinePort := "--port=39998"

	// Notify offline
	code := RunCaffeine("caffeine", []string{"notify", "--state=active", offlinePort}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for notify offline")
	}

	// Override offline
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"override", "--on", offlinePort}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for override offline")
	}

	// Cancel offline
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"cancel", offlinePort}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for cancel offline")
	}

	// Log offline
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"log", offlinePort}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for log offline")
	}

	// Watch offline
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"watch", offlinePort}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for watch offline")
	}
}

func TestCLI_AutostartActions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer

	// Enable
	code := RunCaffeine("caffeine", []string{"autostart", "--enable"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("autostart enable failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "enabled desktop autostart") {
		t.Fatalf("unexpected enable output: %s", stdout.String())
	}

	// Status (enabled)
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"autostart", "--status"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "ENABLED") {
		t.Fatalf("expected autostart ENABLED")
	}

	// Disable
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"autostart", "--disable"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("autostart disable failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "disabled desktop autostart") {
		t.Fatalf("unexpected disable output: %s", stdout.String())
	}
}

func TestCLI_NotifyFlags(t *testing.T) {
	srv, cleanup := startCLITestServer(t)
	defer cleanup()

	portFlag := fmt.Sprintf("--port=%d", srv.Port())
	var stdout, stderr bytes.Buffer

	code := RunCaffeine("caffeine", []string{
		"notify",
		"--state=waiting_approval",
		"--session=sess-123",
		"--agent=test-agent",
		"--task=review",
		"--pid=999",
		portFlag,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("notify failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "waiting_approval") {
		t.Fatalf("expected waiting_approval in output: %s", stdout.String())
	}
}

func TestCLI_Autostart(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Status
	code := RunCaffeine("caffeine", []string{"autostart", "--status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("autostart status failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Autostart:") {
		t.Fatalf("unexpected autostart output: %s", stdout.String())
	}

	// Invalid action flag
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"autostart", "--invalid"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for invalid autostart flag")
	}
}

func TestCLI_Log(t *testing.T) {
	srv, cleanup := startCLITestServer(t)
	defer cleanup()

	portFlag := fmt.Sprintf("--port=%d", srv.Port())
	var stdout, stderr bytes.Buffer

	// Generate transition
	RunCaffeine("caffeine", []string{"notify", "--state", "active", portFlag}, &stdout, &stderr)
	stdout.Reset()
	stderr.Reset()

	// Text log
	code := RunCaffeine("caffeine", []string{"log", portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("log failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Recent State Transitions") {
		t.Fatalf("unexpected log output: %s", stdout.String())
	}

	// JSON log
	stdout.Reset()
	stderr.Reset()
	code = RunCaffeine("caffeine", []string{"log", "--json", portFlag}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("log --json failed: %s", stderr.String())
	}
	var entries []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatalf("decode log json error: %v", err)
	}
}

func TestCLI_Watch(t *testing.T) {
	srv, cleanup := startCLITestServer(t)
	defer cleanup()

	portFlag := fmt.Sprintf("--port=%d", srv.Port())
	var stdout, stderr bytes.Buffer

	code := RunCaffeine("caffeine", []string{"watch", portFlag, "--once"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("watch returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Connected to caffeine SSE stream") {
		t.Fatalf("unexpected watch output: %s", stdout.String())
	}
}

func TestCLI_MainRunIntegration(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Test main run function with caffeine
	code := run([]string{"caffeine", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run caffeine help failed with code %d", code)
	}

	// Test main run function with wake
	code = run([]string{"wake", "--help"}, &stdout, &stderr)
	if code != 1 && code != 0 {
		t.Fatalf("run wake returned unexpected code %d", code)
	}

	// Test main run function with sleep
	code = run([]string{"sleep", "--help"}, &stdout, &stderr)
	if code != 1 && code != 0 {
		t.Fatalf("run sleep returned unexpected code %d", code)
	}
}
