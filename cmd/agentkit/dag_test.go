package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cooper-Wallace/agentkit/internal/dag"
)

func TestCLI_DagPlan(t *testing.T) {
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "dag_checkpoint.json")

	var stdout, stderr bytes.Buffer

	// 1. Plan default feature DAG (text)
	code := runDag([]string{"plan", "--name", "my-feature", "--output", checkpointPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0 for plan, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Validated Task Execution DAG: Feature Lifecycle: my-feature") {
		t.Errorf("unexpected plan output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Parallel Execution Dispatch Layers:") {
		t.Errorf("missing dispatch layers: %s", stdout.String())
	}

	// 2. Plan default feature DAG (JSON)
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"plan", "--name", "my-feature-json", "--output", checkpointPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0 for plan json, got %d", code)
	}
	var planData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &planData); err != nil {
		t.Fatalf("failed to parse plan JSON: %v", err)
	}
	if planData["valid"] != true {
		t.Errorf("expected valid: true, got %v", planData["valid"])
	}
	layers, ok := planData["layers"].([]interface{})
	if !ok || len(layers) != 4 {
		t.Errorf("expected 4 layers in plan JSON, got %v", planData["layers"])
	}

	// 3. Plan with invalid/cyclic file
	badFile := filepath.Join(tempDir, "cyclic.json")
	dBad := dag.NewDAG("bad", "Bad")
	_ = dBad.AddNode(&dag.Node{ID: "A"})
	_ = dBad.AddNode(&dag.Node{ID: "B"})
	_ = dBad.AddDependency("B", "A")
	_ = dBad.AddDependency("A", "B")
	_ = dBad.SaveToFile(badFile)

	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"plan", "--file", badFile, "--output", checkpointPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for invalid cyclic DAG, got %d", code)
	}
}

func TestCLI_DagStatus(t *testing.T) {
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "dag_status.json")

	var stdout, stderr bytes.Buffer

	// 1. Status when file missing
	code := runDag([]string{"status", "--file", filepath.Join(tempDir, "missing.json")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 when checkpoint is missing, got %d", code)
	}

	// Missing with JSON
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"status", "--file", filepath.Join(tempDir, "missing.json"), "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for JSON status when file missing, got %d", code)
	}

	// 2. Create checkpoint
	_ = runDag([]string{"plan", "--name", "status-test", "--output", checkpointPath}, &stdout, &stderr)

	// 3. Status text output
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"status", "--file", checkpointPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Task Execution DAG: Feature Lifecycle: status-test") {
		t.Errorf("unexpected status output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Ready for Dispatch: p1-planning") {
		t.Errorf("missing ready node p1-planning: %s", stdout.String())
	}

	// 4. Status JSON output
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"status", "--file", checkpointPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status json failed with code %d", code)
	}
	var statusData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &statusData); err != nil {
		t.Fatalf("failed to parse status JSON: %v", err)
	}
	if statusData["status"] != "running" { // ready nodes exist
		t.Errorf("expected running status, got %v", statusData["status"])
	}
	readyList, ok := statusData["ready_nodes"].([]interface{})
	if !ok || len(readyList) != 1 || readyList[0] != "p1-planning" {
		t.Errorf("expected [p1-planning] ready, got %v", statusData["ready_nodes"])
	}
}

func TestCLI_DagRun(t *testing.T) {
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "dag_run.json")

	var stdout, stderr bytes.Buffer
	_ = runDag([]string{"plan", "--name", "run-test", "--output", checkpointPath}, &stdout, &stderr)

	// 1. Run automatic next ready node (p1-planning)
	stdout.Reset()
	stderr.Reset()
	code := runDag([]string{"run", "--file", checkpointPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Executed DAG Step: p1-planning") {
		t.Errorf("unexpected run output: %s", stdout.String())
	}

	// 2. Run specific step with JSON (p2a-backend)
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"run", "--file", checkpointPath, "--step", "p2a-backend", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run step failed with code %d: %s", code, stderr.String())
	}
	var runData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &runData); err != nil {
		t.Fatalf("failed to parse run JSON: %v", err)
	}
	if runData["executed_step"] != "p2a-backend" {
		t.Errorf("expected executed_step 'p2a-backend', got %v", runData["executed_step"])
	}

	// 3. Run unknown step
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"run", "--file", checkpointPath, "--step", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 running nonexistent node, got %d", code)
	}

	// 4. Complete remaining steps: p2b-cli, p3-verification, p4-wrapup
	_ = runDag([]string{"run", "--file", checkpointPath, "--step", "p2b-cli"}, &stdout, &stderr)
	_ = runDag([]string{"run", "--file", checkpointPath, "--step", "p3-verification"}, &stdout, &stderr)
	_ = runDag([]string{"run", "--file", checkpointPath, "--step", "p4-wrapup"}, &stdout, &stderr)

	// 5. Run when already completed
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"run", "--file", checkpointPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0 running completed DAG, got %d", code)
	}
	if !strings.Contains(stdout.String(), "all task nodes are already completed") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestCLI_DagHelpAndInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Help
	code := runDag([]string{"help"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Usage: agentkit dag") {
		t.Fatalf("unexpected help response: code %d, output %s", code, stdout.String())
	}

	// No args
	stdout.Reset()
	code = runDag([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for no args, got %d", code)
	}

	// Unknown subcommand
	stdout.Reset()
	stderr.Reset()
	code = runDag([]string{"unknown-sub"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "Unknown dag subcommand") {
		t.Fatalf("expected error for unknown subcommand, got %d", code)
	}

	// Exported RunDag
	code = RunDag([]string{"help"})
	if code != 0 {
		t.Fatalf("RunDag help failed with code %d", code)
	}
}
