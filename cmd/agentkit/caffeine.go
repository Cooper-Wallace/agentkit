package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Cooper-Wallace/agentkit/internal/autostart"
	"github.com/Cooper-Wallace/agentkit/internal/caffeine"
	"github.com/Cooper-Wallace/agentkit/internal/server"
)

// RunCaffeine executes the caffeine, wake, or sleep command line workflows.
func RunCaffeine(command string, args []string, writers ...io.Writer) int {
	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)
	if len(writers) > 0 && writers[0] != nil {
		stdout = writers[0]
	}
	if len(writers) > 1 && writers[1] != nil {
		stderr = writers[1]
	}

	// Shorthand commands
	if command == "wake" {
		for _, a := range args {
			if a == "-h" || a == "--help" || a == "help" {
				fmt.Fprintln(stdout, "Usage: agentkit wake [duration]\n\nForce sleep inhibition lock ON.\nOptional duration: e.g. 1h, 30m, 1h30m, indefinite (default: indefinite).")
				return 0
			}
		}
		duration := ""
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			duration = args[0]
			args = args[1:]
		}
		overrideArgs := []string{"override", "--on"}
		if duration != "" {
			overrideArgs = append(overrideArgs, "--duration", duration)
		}
		overrideArgs = append(overrideArgs, args...)
		return executeCaffeineSubcommand("override", overrideArgs, stdout, stderr)
	}

	if command == "sleep" {
		for _, a := range args {
			if a == "-h" || a == "--help" || a == "help" {
				fmt.Fprintln(stdout, "Usage: agentkit sleep\n\nRelease manual sleep inhibition lock.")
				return 0
			}
		}
		overrideArgs := append([]string{"override", "--off"}, args...)
		return executeCaffeineSubcommand("override", overrideArgs, stdout, stderr)
	}

	// agentkit caffeine <subcommand>
	if len(args) < 1 {
		printCaffeineHelp(stdout)
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "help", "-h", "--help":
		printCaffeineHelp(stdout)
		return 0
	case "status", "daemon", "notify", "override", "cancel", "autostart", "log", "watch":
		return executeCaffeineSubcommand(subcmd, subArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown caffeine subcommand: %s\n", subcmd)
		printCaffeineHelp(stderr)
		return 1
	}
}

func printCaffeineHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: agentkit caffeine <command> [options]
       agentkit wake [duration]
       agentkit sleep

Cross-platform system sleep inhibition, multi-agent lease arbitration, and ambient HUD telemetry.

Commands:
  status [--json] [--port <port>]
      Inspect current sleep inhibition lock state, active leases, cooldown, and hardware breakers.

  daemon [--port <port>] [--foreground]
      Start the loopback HTTP and SSE caffeine server (default 127.0.0.1:18820).

  notify --state <active|idle|waiting_approval> [--session-id <id>] [--port <port>]
      Notify engine of an agent execution state transition.

  override <--on|--off> [--duration <dur>] [--port <port>]
      Manually force sleep lock ON or OFF (duration e.g. 1h, 30m, indefinite).

  cancel [--port <port>]
      Cancel active anti-flapping cooldown or manual override.

  autostart <--enable|--disable|--status>
      Manage user desktop session autostart configuration.

  log [--json] [--limit <n>] [--port <port>]
      Display rolling activity transition journal.

  watch [--port <port>]
      Stream live Server-Sent Events (SSE) telemetry.

Shorthands:
  agentkit wake [duration]   (Alias for override --on [--duration <dur>])
  agentkit sleep             (Alias for override --off)`)
}

func executeCaffeineSubcommand(subcmd string, args []string, stdout, stderr io.Writer) int {
	port := 18820
	for i := 0; i < len(args); i++ {
		if args[i] == "--port" && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil && p > 0 {
				port = p
			}
		} else if strings.HasPrefix(args[i], "--port=") {
			if p, err := strconv.Atoi(strings.TrimPrefix(args[i], "--port=")); err == nil && p > 0 {
				port = p
			}
		}
	}

	switch subcmd {
	case "status":
		return runCaffeineStatus(args, port, stdout, stderr)
	case "daemon":
		return runCaffeineDaemon(args, port, stdout, stderr)
	case "notify":
		return runCaffeineNotify(args, port, stdout, stderr)
	case "override":
		return runCaffeineOverride(args, port, stdout, stderr)
	case "cancel":
		return runCaffeineCancel(args, port, stdout, stderr)
	case "autostart":
		return runCaffeineAutostart(args, stdout, stderr)
	case "log":
		return runCaffeineLog(args, port, stdout, stderr)
	case "watch":
		return runCaffeineWatch(args, port, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown subcommand: %s\n", subcmd)
		return 1
	}
}

func runCaffeineStatus(args []string, port int, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/status", port)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		if asJSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"state":     "OFFLINE",
				"inhibited": false,
				"error":     fmt.Sprintf("daemon offline on port %d", port),
			}, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprintf(stderr, "Caffeine Daemon: OFFLINE (127.0.0.1:%d unreachable)\n", port)
			fmt.Fprintln(stderr, "Run 'agentkit caffeine daemon' to start the service.")
		}
		return 1
	}
	defer resp.Body.Close()

	if asJSON {
		_, _ = io.Copy(stdout, resp.Body)
		fmt.Fprintln(stdout)
		return 0
	}

	var st caffeine.EngineStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		fmt.Fprintf(stderr, "Failed to decode status response: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "AgentKit Caffeine Status:")
	fmt.Fprintf(stdout, "  State:               %s (Inhibited: %v)\n", st.State, st.Inhibited)
	fmt.Fprintf(stdout, "  Active Leases:       %d\n", st.ActiveLeases)
	if st.CooldownRemaining > 0 {
		fmt.Fprintf(stdout, "  Anti-Flap Cooldown:  %.1fs remaining\n", st.CooldownRemaining)
	}
	if st.OverrideActive {
		if st.OverrideRemaining < 0 {
			fmt.Fprintln(stdout, "  Manual Override:     ON (indefinite)")
		} else {
			fmt.Fprintf(stdout, "  Manual Override:     ON (%.1fs remaining)\n", st.OverrideRemaining)
		}
	} else {
		fmt.Fprintln(stdout, "  Manual Override:     OFF")
	}

	if st.Hardware.HasBattery {
		chg := "Discharging"
		if st.Hardware.BatteryCharging {
			chg = "Charging"
		}
		fmt.Fprintf(stdout, "  Battery:             %.0f%% (%s)\n", st.Hardware.BatteryPercent, chg)
	}
	if st.Hardware.CPUTempC > 0 {
		fmt.Fprintf(stdout, "  CPU Temperature:     %.1f°C\n", st.Hardware.CPUTempC)
	}
	if st.Hardware.HasLid {
		lid := "open"
		if st.Hardware.LidClosed {
			lid = "closed"
		}
		fmt.Fprintf(stdout, "  Laptop Lid:          %s\n", lid)
	}

	if st.CircuitBreakers.Tripped {
		fmt.Fprintf(stdout, "  Safety Breakers:     TRIPPED (%s)\n", st.CircuitBreakers.Reason)
	} else {
		fmt.Fprintln(stdout, "  Safety Breakers:     Healthy")
	}

	return 0
}

func runCaffeineDaemon(args []string, port int, stdout, stderr io.Writer) int {
	foreground := false
	for _, a := range args {
		if a == "--foreground" || a == "-f" {
			foreground = true
		}
	}

	if !foreground {
		// Spawn daemon in background
		exe, err := os.Executable()
		if err != nil {
			exe = "agentkit"
		}
		cmd := exec.Command(exe, "caffeine", "daemon", "--foreground", fmt.Sprintf("--port=%d", port))
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Stdin = nil
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(stderr, "Failed to launch background daemon: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Started caffeine daemon in background on port %d (PID: %d)\n", port, cmd.Process.Pid)
		return 0
	}

	// Foreground mode
	srv := server.NewServer(server.ServerConfig{
		Host: "127.0.0.1",
		Port: port,
	}, nil)

	if err := srv.Start(); err != nil {
		fmt.Fprintf(stderr, "Failed to start caffeine daemon: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Caffeine loopback daemon running on %s (Press Ctrl+C to stop)\n", srv.Addr())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Fprintln(stdout, "\nShutting down caffeine daemon...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Stop(ctx)

	fmt.Fprintln(stdout, "Caffeine daemon stopped cleanly.")
	return 0
}

func runCaffeineNotify(args []string, port int, stdout, stderr io.Writer) int {
	var stateStr, sessionID, agent, task string
	pid := 0

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--state" && i+1 < len(args):
			i++
			stateStr = args[i]
		case strings.HasPrefix(arg, "--state="):
			stateStr = strings.TrimPrefix(arg, "--state=")
		case (arg == "--session-id" || arg == "--session") && i+1 < len(args):
			i++
			sessionID = args[i]
		case strings.HasPrefix(arg, "--session-id="):
			sessionID = strings.TrimPrefix(arg, "--session-id=")
		case strings.HasPrefix(arg, "--session="):
			sessionID = strings.TrimPrefix(arg, "--session=")
		case arg == "--agent" && i+1 < len(args):
			i++
			agent = args[i]
		case strings.HasPrefix(arg, "--agent="):
			agent = strings.TrimPrefix(arg, "--agent=")
		case arg == "--task" && i+1 < len(args):
			i++
			task = args[i]
		case strings.HasPrefix(arg, "--task="):
			task = strings.TrimPrefix(arg, "--task=")
		case arg == "--pid" && i+1 < len(args):
			i++
			pid, _ = strconv.Atoi(args[i])
		}
	}

	if stateStr == "" {
		fmt.Fprintln(stderr, "Error: --state <active|idle|waiting_approval> is required.")
		return 1
	}

	if sessionID == "" {
		sessionID = fmt.Sprintf("cli-%d", os.Getpid())
	}

	payload := map[string]interface{}{
		"session_id": sessionID,
		"state":      stateStr,
		"agent":      agent,
		"task":       task,
		"pid":        pid,
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("http://127.0.0.1:%d/notify", port)
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "Failed to send notify: %v (daemon may be offline)\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stderr, "Notify rejected (%d): %s\n", resp.StatusCode, string(respBody))
		return 1
	}

	var st caffeine.EngineStatus
	_ = json.NewDecoder(resp.Body).Decode(&st)
	fmt.Fprintf(stdout, "Notification processed: Session '%s' -> %s (System State: %s, Inhibited: %v)\n",
		sessionID, stateStr, st.State, st.Inhibited)
	return 0
}

func runCaffeineOverride(args []string, port int, stdout, stderr io.Writer) int {
	enabled := false
	stateSpecified := false
	duration := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--on" || arg == "on" || arg == "enable":
			enabled = true
			stateSpecified = true
		case arg == "--off" || arg == "off" || arg == "disable":
			enabled = false
			stateSpecified = true
		case arg == "--duration" && i+1 < len(args):
			i++
			duration = args[i]
		case strings.HasPrefix(arg, "--duration="):
			duration = strings.TrimPrefix(arg, "--duration=")
		}
	}

	if !stateSpecified {
		fmt.Fprintln(stderr, "Error: Must specify --on or --off.")
		return 1
	}

	stateVal := "off"
	if enabled {
		stateVal = "on"
	}

	payload := map[string]string{
		"state":    stateVal,
		"duration": duration,
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("http://127.0.0.1:%d/override", port)
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "Failed to set override: %v (daemon may be offline)\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stderr, "Override failed (%d): %s\n", resp.StatusCode, string(respBody))
		return 1
	}

	var st caffeine.EngineStatus
	_ = json.NewDecoder(resp.Body).Decode(&st)
	if enabled {
		if duration != "" {
			fmt.Fprintf(stdout, "Manual sleep lock enabled for %s (System State: %s)\n", duration, st.State)
		} else {
			fmt.Fprintf(stdout, "Manual sleep lock enabled indefinitely (System State: %s)\n", st.State)
		}
	} else {
		fmt.Fprintf(stdout, "Manual sleep lock disabled (System State: %s, Inhibited: %v)\n", st.State, st.Inhibited)
	}
	return 0
}

func runCaffeineCancel(args []string, port int, stdout, stderr io.Writer) int {
	url := fmt.Sprintf("http://127.0.0.1:%d/cancel", port)
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to cancel: %v (daemon may be offline)\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stderr, "Cancel failed (%d): %s\n", resp.StatusCode, string(respBody))
		return 1
	}

	var st caffeine.EngineStatus
	_ = json.NewDecoder(resp.Body).Decode(&st)
	fmt.Fprintf(stdout, "Cancelled active cooldown/override. System State: %s (Inhibited: %v)\n", st.State, st.Inhibited)
	return 0
}

func runCaffeineAutostart(args []string, stdout, stderr io.Writer) int {
	action := "--status"
	for _, a := range args {
		if a == "--enable" || a == "--disable" || a == "--status" {
			action = a
		} else if strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "Unknown autostart flag: %s. Use --enable, --disable, or --status.\n", a)
			return 1
		}
	}

	mgr := autostart.NewManager(autostart.Options{})

	switch action {
	case "--enable":
		st, err := mgr.Enable()
		if err != nil {
			fmt.Fprintf(stderr, "Failed to enable autostart: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Successfully enabled desktop autostart at: %s\n", st.Path)
		return 0
	case "--disable":
		st, err := mgr.Disable()
		if err != nil {
			fmt.Fprintf(stderr, "Failed to disable autostart: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Successfully disabled desktop autostart (entry removed from: %s)\n", st.Path)
		return 0
	case "--status":
		st, err := mgr.Status()
		if err != nil {
			fmt.Fprintf(stderr, "Failed to query autostart status: %v\n", err)
			return 1
		}
		if st.Enabled {
			fmt.Fprintf(stdout, "Autostart: ENABLED (%s)\n", st.Path)
		} else {
			fmt.Fprintf(stdout, "Autostart: DISABLED (%s not configured)\n", st.Path)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown autostart flag: %s. Use --enable, --disable, or --status.\n", action)
		return 1
	}
}

func runCaffeineLog(args []string, port int, stdout, stderr io.Writer) int {
	asJSON := false
	limit := 50
	for i := 0; i < len(args); i++ {
		if args[i] == "--json" {
			asJSON = true
		} else if (args[i] == "--limit" || args[i] == "-n") && i+1 < len(args) {
			i++
			if l, err := strconv.Atoi(args[i]); err == nil && l > 0 {
				limit = l
			}
		}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/history?limit=%d", port, limit)
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to query history journal: %v (daemon may be offline)\n", err)
		return 1
	}
	defer resp.Body.Close()

	if asJSON {
		_, _ = io.Copy(stdout, resp.Body)
		fmt.Fprintln(stdout)
		return 0
	}

	var entries []caffeine.JournalEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		fmt.Fprintf(stderr, "Failed to decode history: %v\n", err)
		return 1
	}

	if len(entries) == 0 {
		fmt.Fprintln(stdout, "No state transitions recorded in history.")
		return 0
	}

	fmt.Fprintf(stdout, "Recent State Transitions (%d entries):\n", len(entries))
	for _, e := range entries {
		sid := ""
		if e.SessionID != "" {
			sid = fmt.Sprintf(" [%s]", e.SessionID)
		}
		fmt.Fprintf(stdout, "  %s  %-10s -> %-10s  %s%s\n",
			e.Timestamp.Format("15:04:05"), e.FromState, e.ToState, e.Reason, sid)
	}

	return 0
}

func runCaffeineWatch(args []string, port int, stdout, stderr io.Writer) int {
	url := fmt.Sprintf("http://127.0.0.1:%d/events", port)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	maxEvents := 0
	for i := 0; i < len(args); i++ {
		if args[i] == "--once" {
			maxEvents = 1
		} else if (args[i] == "--count" || args[i] == "-c") && i+1 < len(args) {
			i++
			if c, err := strconv.Atoi(args[i]); err == nil && c > 0 {
				maxEvents = c
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Fprintf(stderr, "Watch error: %v\n", err)
		return 1
	}

	client := http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to connect to SSE stream: %v (daemon may be offline)\n", err)
		return 1
	}
	defer resp.Body.Close()

	fmt.Fprintf(stdout, "Connected to caffeine SSE stream on 127.0.0.1:%d (Ctrl+C to quit)\n", port)

	eventCount := 0
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil {
				return 0
			}
			fmt.Fprintf(stderr, "SSE stream disconnected: %v\n", err)
			return 1
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var st caffeine.EngineStatus
			if err := json.Unmarshal([]byte(data), &st); err == nil {
				fmt.Fprintf(stdout, "[%s] State: %-10s Inhibited: %-5v Leases: %d Cooldown: %.1fs Override: %.1fs\n",
					time.Now().Format("15:04:05"), st.State, st.Inhibited, st.ActiveLeases, st.CooldownRemaining, st.OverrideRemaining)
			} else {
				fmt.Fprintln(stdout, line)
			}
			eventCount++
			if maxEvents > 0 && eventCount >= maxEvents {
				return 0
			}
		}
	}
}
