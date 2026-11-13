package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cooper-Wallace/agentkit/internal/dag"
)

// RunDag is the exported CLI entry point for 'agentkit dag' commands.
func RunDag(args []string) int {
	return runDag(args, os.Stdout, os.Stderr)
}

func printDagHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: agentkit dag <subcommand> [options]

Subcommands:
  status   Inspect current multi-agent DAG execution progress and ready worker nodes.
           Options: --file <path>, --json

  plan     Validate and generate multi-agent execution DAG with parallel dispatch layers.
           Options: --file <path>, --name <name>, --output <path>, --json

  run      Execute next ready node or specific step in the task execution DAG.
           Options: --step <id>, --file <path>, --json`)
}

func defaultCheckpointPath() string {
	return filepath.Join(".agentkit", "dag_checkpoint.json")
}

func runDag(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printDagHelp(stdout)
		return 1
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "help", "-h", "--help":
		printDagHelp(stdout)
		return 0

	case "status":
		return runDagStatus(subArgs, stdout, stderr)

	case "plan":
		return runDagPlan(subArgs, stdout, stderr)

	case "run":
		return runDagRun(subArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "Unknown dag subcommand: %s\n", subcommand)
		printDagHelp(stderr)
		return 1
	}
}

type dagStatusReport struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Total     int         `json:"total_nodes"`
	Completed int         `json:"completed_nodes"`
	Percent   float64     `json:"progress_percent"`
	Status    string      `json:"status"`
	Ready     []string    `json:"ready_nodes"`
	Nodes     []*dag.Node `json:"nodes"`
}

func runDagStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dag status", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var filePath string
	var jsonOutput bool

	fs.StringVar(&filePath, "file", "", "Path to DAG checkpoint or JSON file")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if filePath == "" {
		filePath = defaultCheckpointPath()
	}

	d, err := dag.LoadFromFile(filePath)
	if err != nil {
		if jsonOutput {
			fmt.Fprintf(stdout, "{\"error\": %q, \"exists\": false}\n", err.Error())
		} else {
			fmt.Fprintf(stderr, "DAG status error: unable to load DAG from %s: %v\n", filePath, err)
		}
		return 1
	}

	completed, total, pct := d.Progress()
	readyNodes := d.ReadyNodes()
	var readyIDs []string
	for _, r := range readyNodes {
		readyIDs = append(readyIDs, r.ID)
	}

	overallStatus := "pending"
	if d.IsCompleted() {
		overallStatus = "completed"
	} else if d.HasFailed() {
		overallStatus = "failed"
	} else if completed > 0 || len(readyNodes) > 0 {
		overallStatus = "running"
	}

	var orderedNodes []*dag.Node
	for _, id := range d.NodeOrder {
		orderedNodes = append(orderedNodes, d.Nodes[id])
	}

	report := dagStatusReport{
		ID:        d.ID,
		Name:      d.Name,
		Total:     total,
		Completed: completed,
		Percent:   pct,
		Status:    overallStatus,
		Ready:     readyIDs,
		Nodes:     orderedNodes,
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprintf(stdout, "Task Execution DAG: %s (%s)\n", d.Name, d.ID)
	fmt.Fprintf(stdout, "  Status: %s\n", strings.ToUpper(overallStatus))
	fmt.Fprintf(stdout, "  Progress: %d/%d nodes completed (%.1f%%)\n", completed, total, pct)

	if len(readyIDs) > 0 {
		fmt.Fprintf(stdout, "  Ready for Dispatch: %s\n", strings.Join(readyIDs, ", "))
	} else if d.IsCompleted() {
		fmt.Fprintln(stdout, "  Ready for Dispatch: None (All tasks complete)")
	} else {
		fmt.Fprintln(stdout, "  Ready for Dispatch: None (Awaiting prerequisites or failed)")
	}

	fmt.Fprintln(stdout, "\nNodes:")
	for _, id := range d.NodeOrder {
		node := d.Nodes[id]
		stateSymbol := "○"
		switch node.State {
		case dag.StateCompleted:
			stateSymbol = "●"
		case dag.StateRunning:
			stateSymbol = "◐"
		case dag.StateFailed:
			stateSymbol = "✖"
		}
		depsStr := ""
		if len(node.Dependencies) > 0 {
			depsStr = fmt.Sprintf(" (depends on: %s)", strings.Join(node.Dependencies, ", "))
		}
		fmt.Fprintf(stdout, "  %s [%s] %s: %s [%s]%s\n",
			stateSymbol, strings.ToUpper(string(node.State)), node.ID, node.Role, node.Complexity, depsStr)
	}

	return 0
}

func runDagPlan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dag plan", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var filePath string
	var name string
	var outputPath string
	var jsonOutput bool

	fs.StringVar(&filePath, "file", "", "Input DAG JSON template file to validate and plan")
	fs.StringVar(&name, "name", "feature-lifecycle", "DAG name when generating default feature plan")
	fs.StringVar(&outputPath, "output", "", "Output checkpoint file path")
	fs.BoolVar(&jsonOutput, "json", false, "Output execution plan in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	var d *dag.DAG
	var err error

	if filePath != "" {
		d, err = dag.LoadFromFile(filePath)
		if err != nil {
			fmt.Fprintf(stderr, "Failed to load DAG plan from %s: %v\n", filePath, err)
			return 1
		}
	} else {
		d = dag.DefaultFeatureDAG(name)
	}

	if err := d.Validate(); err != nil {
		if jsonOutput {
			fmt.Fprintf(stdout, "{\"error\": %q, \"valid\": false}\n", err.Error())
		} else {
			fmt.Fprintf(stderr, "DAG validation failed: %v\n", err)
		}
		return 1
	}

	layers, err := d.ExecutionLayers()
	if err != nil {
		fmt.Fprintf(stderr, "Failed to compute execution layers: %v\n", err)
		return 1
	}

	if outputPath == "" {
		outputPath = defaultCheckpointPath()
	}
	_ = d.SaveToFile(outputPath)

	if jsonOutput {
		type layerPlan struct {
			Layer int         `json:"layer"`
			Nodes []*dag.Node `json:"nodes"`
		}
		var planResp struct {
			ID     string      `json:"id"`
			Name   string      `json:"name"`
			Valid  bool        `json:"valid"`
			Layers []layerPlan `json:"layers"`
		}
		planResp.ID = d.ID
		planResp.Name = d.Name
		planResp.Valid = true
		for idx, l := range layers {
			planResp.Layers = append(planResp.Layers, layerPlan{Layer: idx, Nodes: l})
		}
		data, _ := json.MarshalIndent(planResp, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprintf(stdout, "Validated Task Execution DAG: %s (%s)\n", d.Name, d.ID)
	fmt.Fprintf(stdout, "Checkpoint saved to: %s\n\n", outputPath)
	fmt.Fprintln(stdout, "Parallel Execution Dispatch Layers:")

	for i, l := range layers {
		fmt.Fprintf(stdout, "  Stage %d (%d parallel worker(s)):\n", i+1, len(l))
		for _, n := range l {
			deps := "None"
			if len(n.Dependencies) > 0 {
				deps = strings.Join(n.Dependencies, ", ")
			}
			fmt.Fprintf(stdout, "    • [%s] %s (%s) — [complexity: %s] (deps: %s)\n",
				n.ID, n.Role, n.Prompt, n.Complexity, deps)
		}
	}

	return 0
}

func runDagRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dag run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var stepID string
	var filePath string
	var jsonOutput bool

	fs.StringVar(&stepID, "step", "", "Specific node ID to run and mark completed")
	fs.StringVar(&filePath, "file", "", "Path to DAG checkpoint file")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if filePath == "" {
		filePath = defaultCheckpointPath()
	}

	d, err := dag.LoadFromFile(filePath)
	if err != nil {
		if jsonOutput {
			fmt.Fprintf(stdout, "{\"error\": %q}\n", err.Error())
		} else {
			fmt.Fprintf(stderr, "Failed to load DAG checkpoint %s: %v\n", filePath, err)
		}
		return 1
	}

	targetNodeID := stepID
	if targetNodeID == "" {
		ready := d.ReadyNodes()
		if len(ready) == 0 {
			if d.IsCompleted() {
				if jsonOutput {
					fmt.Fprintln(stdout, `{"status": "completed", "message": "All DAG nodes are already completed"}`)
				} else {
					fmt.Fprintln(stdout, "DAG execution completed: all task nodes are already completed.")
				}
				return 0
			}
			if d.HasFailed() {
				if jsonOutput {
					fmt.Fprintln(stdout, `{"status": "failed", "message": "DAG contains failed nodes"}`)
				} else {
					fmt.Fprintln(stderr, "Cannot run: DAG contains failed nodes.")
				}
				return 1
			}
			if jsonOutput {
				fmt.Fprintln(stdout, `{"status": "waiting", "message": "No nodes currently ready for execution"}`)
			} else {
				fmt.Fprintln(stdout, "No nodes currently ready for execution (awaiting prerequisites).")
			}
			return 0
		}
		targetNodeID = ready[0].ID
	}

	node, exists := d.GetNode(targetNodeID)
	if !exists {
		if jsonOutput {
			fmt.Fprintf(stdout, "{\"error\": \"Node not found: %s\"}\n", targetNodeID)
		} else {
			fmt.Fprintf(stderr, "Error: Node %q not found in DAG.\n", targetNodeID)
		}
		return 1
	}

	// Transition node to running, then completed
	_ = d.SetNodeState(targetNodeID, dag.StateRunning, "")
	_ = d.SetNodeState(targetNodeID, dag.StateCompleted, fmt.Sprintf("Executed step %s successfully", targetNodeID))
	_ = d.SaveToFile(filePath)

	if jsonOutput {
		resp := map[string]any{
			"executed_step": targetNodeID,
			"role":          node.Role,
			"state":         dag.StateCompleted,
			"is_completed":  d.IsCompleted(),
		}
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprintf(stdout, "Executed DAG Step: %s [%s]\n", node.ID, node.Role)
	fmt.Fprintf(stdout, "  State: %s\n", strings.ToUpper(string(dag.StateCompleted)))
	fmt.Fprintf(stdout, "  Complexity: %s\n", node.Complexity)
	if d.IsCompleted() {
		fmt.Fprintln(stdout, "  All tasks in DAG have now completed successfully! 🎉")
	} else {
		nextReady := d.ReadyNodes()
		var nextIDs []string
		for _, nr := range nextReady {
			nextIDs = append(nextIDs, nr.ID)
		}
		if len(nextIDs) > 0 {
			fmt.Fprintf(stdout, "  Next Ready Steps: %s\n", strings.Join(nextIDs, ", "))
		}
	}

	return 0
}
