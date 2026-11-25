package caffeine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HardwareMetrics captures hardware telemetry relevant to sleep inhibition safety.
type HardwareMetrics struct {
	BatteryPercent  float64 `json:"battery_percent"`
	BatteryCharging bool    `json:"battery_charging"`
	HasBattery      bool    `json:"has_battery"`
	CPUTempC        float64 `json:"cpu_temp_c"`
	LidClosed       bool    `json:"lid_closed"`
	HasLid          bool    `json:"has_lid"`
}

// CircuitBreakerStatus describes whether any hardware circuit breakers are tripped.
type CircuitBreakerStatus struct {
	Tripped        bool   `json:"tripped"`
	Reason         string `json:"reason,omitempty"`
	BatteryTripped bool   `json:"battery_tripped"`
	ThermalTripped bool   `json:"thermal_tripped"`
	LidTripped     bool   `json:"lid_tripped"`
	CeilingTripped bool   `json:"ceiling_tripped"`
}

// CircuitBreakerConfig defines threshold parameters for hardware safety.
type CircuitBreakerConfig struct {
	MaxBatteryDischargePercent float64       `json:"max_battery_discharge_percent"`
	MaxCPUTempC                float64       `json:"max_cpu_temp_c"`
	AllowLidClosed             bool          `json:"allow_lid_closed"`
	MaxContinuousLockDuration  time.Duration `json:"max_continuous_lock_duration"`
}

// DefaultCircuitBreakerConfig returns standard safety thresholds.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxBatteryDischargePercent: 10.0,
		MaxCPUTempC:                92.0,
		AllowLidClosed:             false,
		MaxContinuousLockDuration:  4 * time.Hour,
	}
}

// HardwareReader reads current hardware metrics.
type HardwareReader interface {
	ReadMetrics() (HardwareMetrics, error)
}

// SystemHardwareReader reads hardware telemetry from system paths (sysfs/procfs).
type SystemHardwareReader struct {
	SysfsRoot  string
	ProcfsRoot string
}

// NewSystemHardwareReader creates a SystemHardwareReader using default root paths.
func NewSystemHardwareReader() *SystemHardwareReader {
	return &SystemHardwareReader{
		SysfsRoot:  "/sys",
		ProcfsRoot: "/proc",
	}
}

// ReadMetrics queries system files for battery, thermals, and lid switch state.
func (r *SystemHardwareReader) ReadMetrics() (HardwareMetrics, error) {
	metrics := HardwareMetrics{}

	sysRoot := r.SysfsRoot
	if sysRoot == "" {
		sysRoot = "/sys"
	}
	procRoot := r.ProcfsRoot
	if procRoot == "" {
		procRoot = "/proc"
	}

	// 1. Read Battery telemetry
	powerSupplyDir := filepath.Join(sysRoot, "class", "power_supply")
	entries, err := os.ReadDir(powerSupplyDir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, "BAT") {
				metrics.HasBattery = true
				batDir := filepath.Join(powerSupplyDir, name)

				// Capacity
				capData, cErr := os.ReadFile(filepath.Join(batDir, "capacity"))
				if cErr == nil {
					if val, pErr := strconv.ParseFloat(strings.TrimSpace(string(capData)), 64); pErr == nil {
						metrics.BatteryPercent = val
					}
				}

				// Status
				statusData, sErr := os.ReadFile(filepath.Join(batDir, "status"))
				if sErr == nil {
					st := strings.TrimSpace(string(statusData))
					if strings.EqualFold(st, "Charging") || strings.EqualFold(st, "Full") {
						metrics.BatteryCharging = true
					}
				}
				break
			}
		}
	}

	// 2. Read CPU temperature across thermal zones and hwmon
	maxTemp := 0.0
	thermalPattern := filepath.Join(sysRoot, "class", "thermal", "thermal_zone*", "temp")
	if files, err := filepath.Glob(thermalPattern); err == nil && len(files) > 0 {
		for _, f := range files {
			data, rErr := os.ReadFile(f)
			if rErr == nil {
				if raw, pErr := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); pErr == nil {
					tempC := raw
					if raw > 1000.0 {
						tempC = raw / 1000.0
					}
					if tempC > maxTemp {
						maxTemp = tempC
					}
				}
			}
		}
	}

	// If no thermal_zone, check hwmon
	if maxTemp == 0.0 {
		hwmonPattern := filepath.Join(sysRoot, "class", "hwmon", "hwmon*", "temp*_input")
		if files, err := filepath.Glob(hwmonPattern); err == nil {
			for _, f := range files {
				data, rErr := os.ReadFile(f)
				if rErr == nil {
					if raw, pErr := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); pErr == nil {
						tempC := raw
						if raw > 1000.0 {
							tempC = raw / 1000.0
						}
						if tempC > maxTemp {
							maxTemp = tempC
						}
					}
				}
			}
		}
	}
	metrics.CPUTempC = maxTemp

	// 3. Read Laptop Lid switch
	lidPattern := filepath.Join(procRoot, "acpi", "button", "lid", "*", "state")
	if files, err := filepath.Glob(lidPattern); err == nil && len(files) > 0 {
		metrics.HasLid = true
		for _, f := range files {
			data, rErr := os.ReadFile(f)
			if rErr == nil {
				if strings.Contains(strings.ToLower(string(data)), "closed") {
					metrics.LidClosed = true
					break
				}
			}
		}
	}

	return metrics, nil
}

// MockHardwareReader provides controllable hardware telemetry for unit testing.
type MockHardwareReader struct {
	mu      sync.Mutex
	metrics HardwareMetrics
	err     error
}

// NewMockHardwareReader creates a new MockHardwareReader.
func NewMockHardwareReader() *MockHardwareReader {
	return &MockHardwareReader{
		metrics: HardwareMetrics{
			BatteryPercent:  100.0,
			BatteryCharging: true,
			HasBattery:      true,
			CPUTempC:        45.0,
			LidClosed:       false,
			HasLid:          true,
		},
	}
}

// ReadMetrics returns the configured metrics or error.
func (m *MockHardwareReader) ReadMetrics() (HardwareMetrics, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return HardwareMetrics{}, m.err
	}
	return m.metrics, nil
}

// SetMetrics updates the mock metrics.
func (m *MockHardwareReader) SetMetrics(metrics HardwareMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = metrics
}

// SetError updates the mock error.
func (m *MockHardwareReader) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// EvaluateCircuitBreakers evaluates hardware metrics and lock duration against safety thresholds.
func EvaluateCircuitBreakers(
	metrics HardwareMetrics,
	lockStartTime time.Time,
	config CircuitBreakerConfig,
) CircuitBreakerStatus {
	status := CircuitBreakerStatus{}

	// 1. Battery check: release immediately if battery percent <= 10% and discharging
	if metrics.HasBattery && !metrics.BatteryCharging && metrics.BatteryPercent <= config.MaxBatteryDischargePercent {
		status.BatteryTripped = true
		status.Tripped = true
		status.Reason = fmt.Sprintf("Battery critical: %.1f%% and discharging (<= %.1f%% threshold)",
			metrics.BatteryPercent, config.MaxBatteryDischargePercent)
		return status
	}

	// 2. Thermal check: release if CPU temperature > 92°C
	if metrics.CPUTempC > config.MaxCPUTempC {
		status.ThermalTripped = true
		status.Tripped = true
		status.Reason = fmt.Sprintf("Thermal ceiling exceeded: %.1f°C > %.1f°C threshold",
			metrics.CPUTempC, config.MaxCPUTempC)
		return status
	}

	// 3. Lid check: default sleep on laptop lid closed unless explicitly allowed
	if metrics.HasLid && metrics.LidClosed && !config.AllowLidClosed {
		status.LidTripped = true
		status.Tripped = true
		status.Reason = "Laptop lid closed"
		return status
	}

	// 4. Runaway ceiling: 4.0h continuous ceiling
	if !lockStartTime.IsZero() && config.MaxContinuousLockDuration > 0 {
		elapsed := time.Since(lockStartTime)
		if elapsed >= config.MaxContinuousLockDuration {
			status.CeilingTripped = true
			status.Tripped = true
			status.Reason = fmt.Sprintf("Runaway ceiling reached: lock held continuously for %v (limit: %v)",
				elapsed.Round(time.Second), config.MaxContinuousLockDuration)
			return status
		}
	}

	return status
}
