package caffeine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEngine_MultiSessionLeases(t *testing.T) {
	mockInh := NewMockInhibitor()
	mockHW := NewMockHardwareReader()

	cfg := DefaultEngineConfig()
	cfg.CooldownDuration = 30 * time.Millisecond
	cfg.ApprovalGraceDuration = 50 * time.Millisecond
	cfg.BreakerCheckInterval = 10 * time.Millisecond

	engine := NewEngine(cfg, mockInh, mockHW)
	defer engine.Close()

	// Initial state must be IDLE
	st := engine.GetStatus()
	if st.State != StateIdle {
		t.Fatalf("expected initial state IDLE, got %s", st.State)
	}
	if st.Inhibited {
		t.Fatalf("expected initial inhibited false")
	}

	// 1. Session A notifies active
	st, err := engine.Notify("sess-A", "active", "agentA", "test task", 1001)
	if err != nil {
		t.Fatalf("notify active error: %v", err)
	}
	if st.State != StateActive || !st.Inhibited {
		t.Fatalf("expected ACTIVE and inhibited=true, got %s, inhibited=%v", st.State, st.Inhibited)
	}
	if st.ActiveLeases != 1 {
		t.Fatalf("expected 1 active lease, got %d", st.ActiveLeases)
	}

	// 2. Session B notifies active
	st, err = engine.Notify("sess-B", "active", "agentB", "test task 2", 1002)
	if err != nil {
		t.Fatalf("notify active error: %v", err)
	}
	if st.State != StateActive || st.ActiveLeases != 2 {
		t.Fatalf("expected ACTIVE and 2 active leases, got %s, count=%d", st.State, st.ActiveLeases)
	}

	// 3. Session A notifies idle -> Session B is still active ("Any Yes = YES")
	st, err = engine.Notify("sess-A", "idle", "", "", 0)
	if err != nil {
		t.Fatalf("notify idle error: %v", err)
	}
	if st.State != StateActive || !st.Inhibited {
		t.Fatalf("expected state to remain ACTIVE while sess-B is active, got %s", st.State)
	}
	if st.ActiveLeases != 1 {
		t.Fatalf("expected 1 active lease, got %d", st.ActiveLeases)
	}

	// 4. Session B notifies idle -> enters anti-flapping COOLDOWN
	st, err = engine.Notify("sess-B", "idle", "", "", 0)
	if err != nil {
		t.Fatalf("notify idle error: %v", err)
	}
	if st.State != StateCooldown || !st.Inhibited {
		t.Fatalf("expected state COOLDOWN with lock held, got %s, inhibited=%v", st.State, st.Inhibited)
	}
	if st.CooldownRemaining <= 0 {
		t.Fatalf("expected cooldown remaining > 0, got %f", st.CooldownRemaining)
	}

	// 5. Wait for cooldown to expire -> transitions to IDLE and releases lock
	time.Sleep(50 * time.Millisecond)
	st = engine.GetStatus()
	if st.State != StateIdle || st.Inhibited {
		t.Fatalf("expected IDLE and inhibited=false after cooldown, got %s, inhibited=%v", st.State, st.Inhibited)
	}
}

func TestEngine_AntiFlappingCancellation(t *testing.T) {
	mockInh := NewMockInhibitor()
	mockHW := NewMockHardwareReader()

	cfg := DefaultEngineConfig()
	cfg.CooldownDuration = 60 * time.Millisecond

	engine := NewEngine(cfg, mockInh, mockHW)
	defer engine.Close()

	// Activate session and then idle it to trigger cooldown
	_, _ = engine.Notify("sess-1", "active", "agent1", "", 0)
	st, _ := engine.Notify("sess-1", "idle", "", "", 0)
	if st.State != StateCooldown {
		t.Fatalf("expected COOLDOWN, got %s", st.State)
	}

	// A new session arrives during cooldown -> cancels cooldown immediately!
	time.Sleep(10 * time.Millisecond)
	st, err := engine.Notify("sess-2", "active", "agent2", "", 0)
	if err != nil {
		t.Fatalf("notify active error: %v", err)
	}
	if st.State != StateActive {
		t.Fatalf("expected ACTIVE after interruption, got %s", st.State)
	}
	if st.CooldownRemaining != 0 {
		t.Fatalf("expected cooldown remaining to be 0, got %f", st.CooldownRemaining)
	}
}

func TestEngine_ApprovalGraceTimer(t *testing.T) {
	mockInh := NewMockInhibitor()
	mockHW := NewMockHardwareReader()

	cfg := DefaultEngineConfig()
	cfg.CooldownDuration = 30 * time.Millisecond
	cfg.ApprovalGraceDuration = 40 * time.Millisecond

	engine := NewEngine(cfg, mockInh, mockHW)
	defer engine.Close()

	// 1. Session requests user approval
	st, err := engine.Notify("sess-approval", "waiting_approval", "agent", "awaiting human", 0)
	if err != nil {
		t.Fatalf("notify waiting_approval error: %v", err)
	}
	if st.State != StateWaitingApproval || !st.Inhibited {
		t.Fatalf("expected WAITING_APPROVAL and inhibited=true, got %s", st.State)
	}

	// 2. User responds before expiry -> transitions back to active
	st, err = engine.Notify("sess-approval", "active", "agent", "continued", 0)
	if err != nil {
		t.Fatalf("notify active error: %v", err)
	}
	if st.State != StateActive || !st.Inhibited {
		t.Fatalf("expected ACTIVE, got %s", st.State)
	}

	// 3. Request approval again and let 120s grace period expire
	st, _ = engine.Notify("sess-approval", "waiting_approval", "agent", "awaiting human again", 0)
	if st.State != StateWaitingApproval {
		t.Fatalf("expected WAITING_APPROVAL, got %s", st.State)
	}

	// Wait for grace timer to expire -> triggers cooldown -> triggers idle
	time.Sleep(80 * time.Millisecond)
	st = engine.GetStatus()
	// Should be in cooldown or idle
	if st.State == StateWaitingApproval {
		t.Fatalf("expected approval to have expired, still in %s", st.State)
	}

	// Wait past cooldown
	time.Sleep(50 * time.Millisecond)
	st = engine.GetStatus()
	if st.State != StateIdle || st.Inhibited {
		t.Fatalf("expected IDLE and inhibited=false, got %s, inhibited=%v", st.State, st.Inhibited)
	}
}

func TestEngine_ManualOverride(t *testing.T) {
	mockInh := NewMockInhibitor()
	mockHW := NewMockHardwareReader()

	cfg := DefaultEngineConfig()
	engine := NewEngine(cfg, mockInh, mockHW)
	defer engine.Close()

	// Enable indefinite override
	st, err := engine.SetOverride(true, "indefinite")
	if err != nil {
		t.Fatalf("set override error: %v", err)
	}
	if st.State != StateOverride || !st.Inhibited || !st.OverrideActive {
		t.Fatalf("expected OVERRIDE, inhibited=true, override_active=true")
	}
	if st.OverrideRemaining != -1.0 {
		t.Fatalf("expected -1.0 for indefinite override remaining, got %f", st.OverrideRemaining)
	}

	// Disable override
	st, err = engine.SetOverride(false, "")
	if err != nil {
		t.Fatalf("set override false error: %v", err)
	}
	if st.State != StateIdle || st.Inhibited || st.OverrideActive {
		t.Fatalf("expected IDLE after disabling override, got %s", st.State)
	}

	// Enable override with timed duration
	st, err = engine.SetOverride(true, "30ms")
	if err != nil {
		t.Fatalf("set override duration error: %v", err)
	}
	if st.State != StateOverride || !st.Inhibited {
		t.Fatalf("expected OVERRIDE, got %s", st.State)
	}

	// Wait for override duration to expire
	time.Sleep(50 * time.Millisecond)
	st = engine.GetStatus()
	if st.State != StateIdle || st.Inhibited {
		t.Fatalf("expected IDLE after timed override expiration, got %s", st.State)
	}
}

func TestEngine_Cancel(t *testing.T) {
	mockInh := NewMockInhibitor()
	mockHW := NewMockHardwareReader()

	cfg := DefaultEngineConfig()
	cfg.CooldownDuration = 100 * time.Millisecond
	engine := NewEngine(cfg, mockInh, mockHW)
	defer engine.Close()

	// Cancel on active cooldown
	_, _ = engine.Notify("sess-1", "active", "", "", 0)
	st, _ := engine.Notify("sess-1", "idle", "", "", 0)
	if st.State != StateCooldown {
		t.Fatalf("expected COOLDOWN, got %s", st.State)
	}

	st, err := engine.Cancel()
	if err != nil {
		t.Fatalf("cancel error: %v", err)
	}
	if st.State != StateIdle || st.Inhibited {
		t.Fatalf("expected IDLE immediately after cancel, got %s", st.State)
	}

	// Cancel on active override
	_, _ = engine.SetOverride(true, "indefinite")
	st = engine.GetStatus()
	if st.State != StateOverride {
		t.Fatalf("expected OVERRIDE, got %s", st.State)
	}

	st, err = engine.Cancel()
	if err != nil {
		t.Fatalf("cancel error: %v", err)
	}
	if st.State != StateIdle || st.Inhibited {
		t.Fatalf("expected IDLE after cancelling override, got %s", st.State)
	}
}

func TestEngine_CircuitBreakers(t *testing.T) {
	mockInh := NewMockInhibitor()
	mockHW := NewMockHardwareReader()

	cfg := DefaultEngineConfig()
	cfg.BreakerCheckInterval = 10 * time.Millisecond
	cfg.BreakerConfig.MaxBatteryDischargePercent = 10.0
	cfg.BreakerConfig.MaxCPUTempC = 92.0

	engine := NewEngine(cfg, mockInh, mockHW)
	defer engine.Close()

	// 1. Activate session
	_, _ = engine.Notify("sess-safe", "active", "", "", 0)
	st := engine.GetStatus()
	if !st.Inhibited {
		t.Fatalf("expected inhibited=true")
	}

	// 2. Battery drops to 9% and discharging -> breaker trips
	mockHW.SetMetrics(HardwareMetrics{
		BatteryPercent:  9.0,
		BatteryCharging: false,
		HasBattery:      true,
		CPUTempC:        50.0,
		LidClosed:       false,
	})

	time.Sleep(30 * time.Millisecond)
	st = engine.GetStatus()
	if st.Inhibited {
		t.Fatalf("expected sleep lock to be dropped by battery circuit breaker")
	}
	if !st.CircuitBreakers.BatteryTripped || !st.CircuitBreakers.Tripped {
		t.Fatalf("expected BatteryTripped=true, got %+v", st.CircuitBreakers)
	}

	// 3. Restore battery, test thermal breaker
	mockHW.SetMetrics(HardwareMetrics{
		BatteryPercent:  80.0,
		BatteryCharging: true,
		HasBattery:      true,
		CPUTempC:        95.0, // > 92.0°C
		LidClosed:       false,
	})

	_, _ = engine.Notify("sess-thermal", "active", "", "", 0)
	time.Sleep(30 * time.Millisecond)
	st = engine.GetStatus()
	if st.Inhibited {
		t.Fatalf("expected sleep lock to be dropped by thermal circuit breaker")
	}
	if !st.CircuitBreakers.ThermalTripped {
		t.Fatalf("expected ThermalTripped=true")
	}

	// 4. Test Lid breaker
	mockHW.SetMetrics(HardwareMetrics{
		BatteryPercent:  80.0,
		BatteryCharging: true,
		HasBattery:      true,
		CPUTempC:        50.0,
		HasLid:          true,
		LidClosed:       true,
	})

	_, _ = engine.Notify("sess-lid", "active", "", "", 0)
	time.Sleep(30 * time.Millisecond)
	st = engine.GetStatus()
	if st.Inhibited {
		t.Fatalf("expected sleep lock to be dropped by lid closed breaker")
	}
	if !st.CircuitBreakers.LidTripped {
		t.Fatalf("expected LidTripped=true")
	}

	// 5. Test Continuous lock ceiling
	now := time.Now()
	lockStart := now.Add(-5 * time.Hour)
	cbCfg := DefaultCircuitBreakerConfig()
	cbCfg.MaxContinuousLockDuration = 4 * time.Hour
	cb := EvaluateCircuitBreakers(HardwareMetrics{BatteryCharging: true, CPUTempC: 50.0}, lockStart, cbCfg)
	if !cb.CeilingTripped || !cb.Tripped {
		t.Fatalf("expected CeilingTripped=true, got %+v", cb)
	}
}

func TestEngine_SubscribersAndHistory(t *testing.T) {
	mockInh := NewMockInhibitor()
	mockHW := NewMockHardwareReader()

	cfg := DefaultEngineConfig()
	cfg.HistoryLimit = 10
	engine := NewEngine(cfg, mockInh, mockHW)
	defer engine.Close()

	ch := engine.Subscribe()
	defer engine.Unsubscribe(ch)

	_, _ = engine.Notify("sess-events", "active", "agentX", "taskX", 0)

	select {
	case ev := <-ch:
		if ev.Type != "state_change" {
			t.Fatalf("expected state_change event, got %s", ev.Type)
		}
		if ev.Status.State != StateActive {
			t.Fatalf("expected event state ACTIVE, got %s", ev.Status.State)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for subscriber event")
	}

	history := engine.GetHistory(5)
	if len(history) == 0 {
		t.Fatalf("expected non-empty history journal")
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input    string
		expected time.Duration
		err      bool
	}{
		{"", 0, false},
		{"indefinite", 0, false},
		{"forever", 0, false},
		{"1h", 1 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"1h30m", 90 * time.Minute, false},
		{"2 hours", 2 * time.Hour, false},
		{"15 mins", 15 * time.Minute, false},
		{"45 seconds", 45 * time.Second, false},
		{"-5m", 0, true},
		{"invalid", 0, true},
	}

	for _, tc := range cases {
		d, err := ParseDuration(tc.input)
		if tc.err && err == nil {
			t.Errorf("expected error for input '%s', got %v", tc.input, d)
		}
		if !tc.err && err != nil {
			t.Errorf("unexpected error for input '%s': %v", tc.input, err)
		}
		if !tc.err && d != tc.expected {
			t.Errorf("for '%s', expected %v, got %v", tc.input, tc.expected, d)
		}
	}
}

func TestSystemHardwareReader(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock sysfs battery
	batDir := filepath.Join(tmpDir, "sys", "class", "power_supply", "BAT0")
	if err := os.MkdirAll(batDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(batDir, "capacity"), []byte("75\n"), 0644)
	_ = os.WriteFile(filepath.Join(batDir, "status"), []byte("Discharging\n"), 0644)

	// Mock thermal zone
	thermDir := filepath.Join(tmpDir, "sys", "class", "thermal", "thermal_zone0")
	if err := os.MkdirAll(thermDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(thermDir, "temp"), []byte("55000\n"), 0644)

	// Mock lid
	lidDir := filepath.Join(tmpDir, "proc", "acpi", "button", "lid", "LID0")
	if err := os.MkdirAll(lidDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(lidDir, "state"), []byte("state:      closed\n"), 0644)

	reader := &SystemHardwareReader{
		SysfsRoot:  filepath.Join(tmpDir, "sys"),
		ProcfsRoot: filepath.Join(tmpDir, "proc"),
	}

	metrics, err := reader.ReadMetrics()
	if err != nil {
		t.Fatalf("ReadMetrics error: %v", err)
	}
	if !metrics.HasBattery || metrics.BatteryPercent != 75 || metrics.BatteryCharging {
		t.Fatalf("unexpected battery metrics: %+v", metrics)
	}
	if metrics.CPUTempC != 55.0 {
		t.Fatalf("expected 55.0 C, got %f", metrics.CPUTempC)
	}
	if !metrics.HasLid || !metrics.LidClosed {
		t.Fatalf("expected lid closed, got %+v", metrics)
	}
}

func TestInhibitorImplementations(t *testing.T) {
	mock := NewMockInhibitor()
	if mock.Name() != "mock" {
		t.Fatalf("expected mock name")
	}
	if !mock.IsAvailable() {
		t.Fatalf("expected available")
	}

	rel, err := mock.Inhibit("testing")
	if err != nil {
		t.Fatalf("mock inhibit failed: %v", err)
	}
	if !mock.IsActive() || mock.Reason() != "testing" {
		t.Fatalf("mock should be active")
	}
	if err := rel(); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if mock.IsActive() {
		t.Fatalf("mock should be inactive")
	}

	// Test mock failure configurations
	mock.SetFailInhibit(true)
	if _, err := mock.Inhibit("should fail"); err == nil {
		t.Fatalf("expected inhibit failure")
	}
	mock.SetFailInhibit(false)

	rel, _ = mock.Inhibit("test release fail")
	mock.SetFailRelease(true)
	if err := rel(); err == nil {
		t.Fatalf("expected release failure")
	}

	// Test Darwin inhibitor stub/instantiation
	darwinInh := NewDarwinInhibitor()
	if darwinInh.Name() != "darwin-caffeinate" {
		t.Fatalf("expected darwin-caffeinate")
	}

	// Test Windows inhibitor stub/instantiation
	winInh := NewWindowsInhibitor()
	if winInh.Name() != "windows-kernel32" {
		t.Fatalf("expected windows-kernel32")
	}

	// Test Linux inhibitor
	linuxInh := NewLinuxInhibitor()
	if linuxInh.Name() != "linux-systemd" {
		t.Fatalf("expected linux-systemd")
	}

	// Test NewSystemInhibitor
	sysInh := NewSystemInhibitor()
	if sysInh == nil {
		t.Fatalf("expected non-nil system inhibitor")
	}

	// Stubs on non-native platform
	_, _ = darwinInh.Inhibit("stub")
	_ = darwinInh.IsAvailable()
	_, _ = winInh.Inhibit("stub")
	_ = winInh.IsAvailable()
}

func TestEngine_EdgeCasesAndCoverage(t *testing.T) {
	// 1. Mock Hardware Reader error
	mockHW := NewMockHardwareReader()
	mockHW.SetError(os.ErrNotExist)
	_, err := mockHW.ReadMetrics()
	if err == nil {
		t.Fatalf("expected error from SetError")
	}

	// 2. Mock Inhibitor methods
	mockInh := NewMockInhibitor()
	mockInh.SetAvailable(false)
	if mockInh.IsAvailable() {
		t.Fatalf("expected available false")
	}
	_, _ = mockInh.Inhibit("call1")
	if mockInh.InhibitCalls() != 1 {
		t.Fatalf("expected 1 inhibit call")
	}

	// 3. NewSystemHardwareReader default
	sysHW := NewSystemHardwareReader()
	_, _ = sysHW.ReadMetrics()

	// 4. Engine validation and closed errors
	eng := NewEngine(EngineConfig{}, mockInh, mockHW)
	_, err = eng.Notify("", "active", "", "", 0)
	if err == nil {
		t.Fatalf("expected error on empty session_id")
	}
	_, err = eng.Notify("sess-1", "invalid-state", "", "", 0)
	if err == nil {
		t.Fatalf("expected error on invalid state")
	}

	// Close engine and test post-close calls
	_ = eng.Close()
	_ = eng.Close() // idempotent
	_, err = eng.Notify("sess-1", "active", "", "", 0)
	if err == nil {
		t.Fatalf("expected error after engine closed")
	}
	_, err = eng.SetOverride(true, "1h")
	if err == nil {
		t.Fatalf("expected error after engine closed")
	}
	_, err = eng.Cancel()
	if err == nil {
		t.Fatalf("expected error after engine closed")
	}
}
