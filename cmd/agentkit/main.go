package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cooper-Wallace/agentkit/internal/apply"
	"github.com/Cooper-Wallace/agentkit/internal/audit"
	"github.com/Cooper-Wallace/agentkit/internal/catalog"
	"github.com/Cooper-Wallace/agentkit/internal/diff"
	"github.com/Cooper-Wallace/agentkit/internal/quota"
	"github.com/Cooper-Wallace/agentkit/internal/template"
)

const version = "1.0.0"

func printHelp(w io.Writer) {
	fmt.Fprintf(w, `agentkit %s - Declarative AI Agent Harness Governance, Quota Telemetry, and Standardization.

Usage:
  agentkit <command> [options]

Commands:
  audit [options]
      Audit machine compliance against standardized agent directives, permissions, and tools.
      Options: --fix, --json, --harness <name>, --settings-path <path>

  apply [options]
      Idempotently apply permissions, global directives, MCP servers, and orchestration profiles.
      Options: --dry-run, --harness <name>, --profile <name>, --settings-path <path>

  diff [options]
      Diff local configuration against repository standard baselines.
      Options: --json, --harness <name>, --settings-path <path>

  profile [show|switch <name>]
      Inspect or switch the active multi-model cognitive orchestration profile.

  quota [status|route --complexity <low|med|high>] [options]
      Inspect language model quota telemetry or route tasks based on real-time capacity.
      Options: --json, --file <path>

  template [options]
      Scaffold project-level agent directives or lifecycle hooks.
      Options: --type <agents|gemini|project-gemini|hook>, --target <dir>, --name <name>, --desc <desc>, --lang <lang>, --test <cmd>, --force

  snapshot [options]
      Export or display unmanaged local permissions and MCP servers.
      Options: --output <dir>, --json, --settings-path <path>

  caffeine <command> [options]
      Manage ambient sleep inhibition, multi-agent lease arbitration, and HUD telemetry.
      Commands: status, daemon, notify, override, cancel, autostart, log, watch

  wake [duration]
      Shorthand to force sleep inhibition lock ON (e.g. agentkit wake 1h).

  sleep
      Shorthand to release manual sleep inhibition lock.

  vault [status|init|ingest|sync|note] [options]
      Manage Obsidian AgentVault notes, ingest offline inputs, and sync project state.
      Options: --path <dir>, --project <name>, --dry-run, --no-push, --json, --section <name>, --content <text>, --tag <tag>

  dag [status|plan|run] [options]
      Plan, inspect, and execute multi-agent task execution DAGs.
      Options: --file <path>, --step <id>, --name <name>, --output <path>, --json

  version, -v, --version
      Print agentkit version.

  help, -h, --help
      Show this help message.
`, version)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printHelp(stdout)
		return 1
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0

	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "agentkit version %s\n", version)
		return 0

	case "audit":
		return runAudit(cmdArgs, stdout, stderr)

	case "apply":
		return runApply(cmdArgs, stdout, stderr)

	case "diff":
		return runDiff(cmdArgs, stdout, stderr)

	case "profile":
		return runProfile(cmdArgs, stdout, stderr)

	case "quota":
		return runQuota(cmdArgs, stdout, stderr)

	case "template":
		return runTemplate(cmdArgs, stdout, stderr)

	case "snapshot":
		return runSnapshot(cmdArgs, stdout, stderr)

	case "caffeine":
		return RunCaffeine("caffeine", cmdArgs, stdout, stderr)

	case "wake":
		return RunCaffeine("wake", cmdArgs, stdout, stderr)

	case "sleep":
		return RunCaffeine("sleep", cmdArgs, stdout, stderr)

	case "vault":
		return runVault(cmdArgs, stdout, stderr)

	case "dag":
		return runDag(cmdArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n", command)
		printHelp(stderr)
		return 1
	}
}

func runAudit(args []string, stdout, stderr io.Writer) int {
	opts := audit.AuditOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--fix":
			opts.Fix = true
		case "--verbose", "-v":
			opts.Verbose = true
		case "--settings-path":
			if i+1 < len(args) {
				i++
				opts.SettingsPath = args[i]
			}
		case "--harness":
			if i+1 < len(args) {
				i++
				// Currently harness adapter defaults to antigravity
			}
		default:
			if strings.HasPrefix(arg, "--settings-path=") {
				opts.SettingsPath = strings.TrimPrefix(arg, "--settings-path=")
			}
		}
	}

	res, err := audit.Run(opts)
	if err != nil {
		fmt.Fprintf(stderr, "Audit error: %v\n", err)
		return 1
	}

	if opts.JSON {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "JSON marshal error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		fmt.Fprint(stdout, audit.FormatAudit(res))
	}

	if res.Compliant {
		return 0
	}
	return 1
}

func runApply(args []string, stdout, stderr io.Writer) int {
	opts := apply.ApplyOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--no-backup":
			opts.NoBackup = true
		case "--no-mcp":
			opts.NoMCP = true
		case "--no-plugins":
			opts.NoPlugins = true
		case "--link-home":
			opts.LinkHome = true
		case "--profile", "--model-profile":
			if i+1 < len(args) {
				i++
				opts.ModelProfile = args[i]
			}
		case "--settings-path":
			if i+1 < len(args) {
				i++
				opts.SettingsPath = args[i]
			}
		case "--harness":
			if i+1 < len(args) {
				i++
			}
		default:
			if strings.HasPrefix(arg, "--profile=") {
				opts.ModelProfile = strings.TrimPrefix(arg, "--profile=")
			} else if strings.HasPrefix(arg, "--model-profile=") {
				opts.ModelProfile = strings.TrimPrefix(arg, "--model-profile=")
			} else if strings.HasPrefix(arg, "--settings-path=") {
				opts.SettingsPath = strings.TrimPrefix(arg, "--settings-path=")
			}
		}
	}

	res, err := apply.Apply(opts)
	if err != nil {
		fmt.Fprintf(stderr, "Apply error: %v\n", err)
		return 1
	}

	for _, msg := range res.Messages {
		fmt.Fprintln(stdout, msg)
	}
	if opts.DryRun {
		fmt.Fprintln(stdout, "\n[DRY-RUN] Preview complete. No files were modified.")
	} else {
		fmt.Fprintf(stdout, "\n[SUCCESS] Standardization successfully applied! (Active profile: %s)\n", res.ConfiguredProfile)
	}
	return 0
}

func runDiff(args []string, stdout, stderr io.Writer) int {
	opts := diff.DiffOptions{}
	asJSON := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			asJSON = true
		case "--no-mcp":
			opts.NoMCP = true
		case "--settings-path":
			if i+1 < len(args) {
				i++
				opts.SettingsPath = args[i]
			}
		case "--mcp-path":
			if i+1 < len(args) {
				i++
				opts.MCPPath = args[i]
			}
		case "--harness":
			if i+1 < len(args) {
				i++
			}
		default:
			if strings.HasPrefix(arg, "--settings-path=") {
				opts.SettingsPath = strings.TrimPrefix(arg, "--settings-path=")
			} else if strings.HasPrefix(arg, "--mcp-path=") {
				opts.MCPPath = strings.TrimPrefix(arg, "--mcp-path=")
			}
		}
	}

	res, err := diff.Diff(opts)
	if err != nil {
		fmt.Fprintf(stderr, "Diff error: %v\n", err)
		return 1
	}

	if asJSON {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "JSON marshal error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		fmt.Fprint(stdout, diff.FormatDiff(res))
	}
	return 0
}

func runProfile(args []string, stdout, stderr io.Writer) int {
	profCat, err := catalog.LoadEmbeddedProfiles()
	if err != nil {
		fmt.Fprintf(stderr, "Load profiles catalog error: %v\n", err)
		return 1
	}

	subcmd := "show"
	var targetProfile string
	applyOpts := apply.ApplyOptions{
		NoMCP:     true,
		NoPlugins: true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dry-run":
			applyOpts.DryRun = true
		case "--settings-path":
			if i+1 < len(args) {
				i++
				applyOpts.SettingsPath = args[i]
			}
		case "--gemini-dir":
			if i+1 < len(args) {
				i++
				applyOpts.GeminiDir = args[i]
			}
		default:
			if strings.HasPrefix(arg, "--settings-path=") {
				applyOpts.SettingsPath = strings.TrimPrefix(arg, "--settings-path=")
			} else if strings.HasPrefix(arg, "--gemini-dir=") {
				applyOpts.GeminiDir = strings.TrimPrefix(arg, "--gemini-dir=")
			} else if subcmd == "show" {
				subcmd = arg
			} else if targetProfile == "" {
				targetProfile = arg
			}
		}
	}

	switch subcmd {
	case "show", "list":
		fmt.Fprintln(stdout, "Available Multi-Model Cognitive Orchestration Profiles:")
		for name, p := range profCat.Profiles {
			defMark := ""
			if name == profCat.DefaultProfile {
				defMark = " (default)"
			}
			fmt.Fprintf(stdout, "\n• %s%s:\n    Description: %s\n    Root Model:  %s\n    Worker:      %s\n",
				name, defMark, p.Description, p.RootModel, p.WorkerModel)
		}
		return 0

	case "switch":
		if targetProfile == "" {
			fmt.Fprintln(stderr, "Profile name required. Usage: agentkit profile switch <name>")
			return 1
		}
		// Resolve alias
		if alias, ok := profCat.DeprecatedAliases[targetProfile]; ok {
			fmt.Fprintf(stderr, "Warning: '%s' is deprecated, using '%s'\n", targetProfile, alias)
			targetProfile = alias
		}
		if _, ok := profCat.Profiles[targetProfile]; !ok {
			fmt.Fprintf(stderr, "Unknown profile: %s\n", targetProfile)
			return 1
		}
		applyOpts.ModelProfile = targetProfile
		res, err := apply.Apply(applyOpts)
		if err != nil {
			fmt.Fprintf(stderr, "Failed to switch profile: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Switched active orchestration profile to: %s\n", res.ConfiguredProfile)
		return 0

	default:
		// Check if argument is a profile name directly
		if alias, ok := profCat.DeprecatedAliases[subcmd]; ok {
			subcmd = alias
		}
		if _, ok := profCat.Profiles[subcmd]; ok {
			applyOpts.ModelProfile = subcmd
			res, err := apply.Apply(applyOpts)
			if err != nil {
				fmt.Fprintf(stderr, "Failed to switch profile: %v\n", err)
				return 1
			}
			fmt.Fprintf(stdout, "Switched active orchestration profile to: %s\n", res.ConfiguredProfile)
			return 0
		}
		fmt.Fprintf(stderr, "Unknown profile subcommand or profile: %s\n", subcmd)
		return 1
	}
}

func runQuota(args []string, stdout, stderr io.Writer) int {
	subcmd := "status"
	var complexity string
	var quotaFile string
	asJSON := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "status":
			subcmd = "status"
		case "route":
			subcmd = "route"
		case "--json":
			asJSON = true
		case "--file":
			if i+1 < len(args) {
				i++
				quotaFile = args[i]
			}
		case "--complexity":
			if i+1 < len(args) {
				i++
				complexity = args[i]
			}
		default:
			if strings.HasPrefix(arg, "--complexity=") {
				complexity = strings.TrimPrefix(arg, "--complexity=")
			} else if strings.HasPrefix(arg, "--file=") {
				quotaFile = strings.TrimPrefix(arg, "--file=")
			}
		}
	}

	if subcmd == "status" {
		status, err := quota.Status(quotaFile)
		if err != nil {
			fmt.Fprintf(stderr, "Quota status error: %v\n", err)
			return 1
		}
		if asJSON {
			data, _ := json.MarshalIndent(status, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprint(stdout, quota.FormatStatusTable(status))
		}
		return 0
	}

	if subcmd == "route" {
		if complexity == "" {
			fmt.Fprintln(stderr, "Complexity flag required. Usage: agentkit quota route --complexity <low|med|high>")
			return 1
		}
		res, err := quota.Route(quotaFile, complexity)
		if err != nil {
			fmt.Fprintf(stderr, "Quota route error: %v\n", err)
			return 1
		}
		if asJSON {
			data, _ := json.MarshalIndent(res, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Model:  %s\nGroup:  %s\nReason: %s\n", res.Model, res.Group, res.Reason)
		}
		return 0
	}

	return 1
}

func runTemplate(args []string, stdout, stderr io.Writer) int {
	opts := template.Options{Type: "agents"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--legacy":
			opts.Type = "gemini"
		case "--force", "-f":
			opts.Force = true
		case "-t", "--type":
			if i+1 < len(args) {
				i++
				opts.Type = args[i]
			}
		case "--target", "--output", "-o":
			if i+1 < len(args) {
				i++
				opts.TargetDir = args[i]
			}
		case "-n", "--name":
			if i+1 < len(args) {
				i++
				opts.ProjectName = args[i]
			}
		case "-d", "--desc", "--description":
			if i+1 < len(args) {
				i++
				opts.Description = args[i]
			}
		case "-l", "--lang", "--language":
			if i+1 < len(args) {
				i++
				opts.Language = args[i]
			}
		case "--test":
			if i+1 < len(args) {
				i++
				opts.TestCommand = args[i]
			}
		default:
			if strings.HasPrefix(arg, "--type=") {
				opts.Type = strings.TrimPrefix(arg, "--type=")
			} else if strings.HasPrefix(arg, "--target=") {
				opts.TargetDir = strings.TrimPrefix(arg, "--target=")
			} else if strings.HasPrefix(arg, "--output=") {
				opts.TargetDir = strings.TrimPrefix(arg, "--output=")
			} else if strings.HasPrefix(arg, "--name=") {
				opts.ProjectName = strings.TrimPrefix(arg, "--name=")
			} else if strings.HasPrefix(arg, "--desc=") {
				opts.Description = strings.TrimPrefix(arg, "--desc=")
			}
		}
	}

	if err := template.Scaffold(opts); err != nil {
		fmt.Fprintf(stderr, "Template error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Scaffolded %s template in %s\n", opts.Type, opts.TargetDir)
	return 0
}

func runSnapshot(args []string, stdout, stderr io.Writer) int {
	var outputDir string
	var settingsPath string
	asJSON := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			asJSON = true
		case "--output", "-o":
			if i+1 < len(args) {
				i++
				outputDir = args[i]
			}
		case "--settings-path":
			if i+1 < len(args) {
				i++
				settingsPath = args[i]
			}
		default:
			if strings.HasPrefix(arg, "--output=") {
				outputDir = strings.TrimPrefix(arg, "--output=")
			} else if strings.HasPrefix(arg, "--settings-path=") {
				settingsPath = strings.TrimPrefix(arg, "--settings-path=")
			}
		}
	}

	dRes, err := diff.Diff(diff.DiffOptions{
		SettingsPath: settingsPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "Snapshot error: %v\n", err)
		return 1
	}

	snapshotData := map[string]interface{}{
		"unmanaged_permissions": dRes.CustomPermissions,
		"unmanaged_mcp_servers": dRes.CustomMCPServers,
	}

	if asJSON {
		data, _ := json.MarshalIndent(snapshotData, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	if outputDir != "" {
		_ = os.MkdirAll(outputDir, 0755)
		data, _ := json.MarshalIndent(snapshotData, "", "  ")
		outFile := filepath.Join(outputDir, "snapshot.json")
		if err := os.WriteFile(outFile, data, 0644); err != nil {
			fmt.Fprintf(stderr, "Failed to write snapshot file: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Snapshot saved to: %s\n", outFile)
		return 0
	}

	fmt.Fprintln(stdout, "Snapshot: Unmanaged Local Configurations")
	if len(dRes.CustomPermissions) == 0 {
		fmt.Fprintln(stdout, "  No unmanaged permissions found.")
	} else {
		fmt.Fprintf(stdout, "  Found %d unmanaged permissions:\n", len(dRes.CustomPermissions))
		for _, p := range dRes.CustomPermissions {
			fmt.Fprintf(stdout, "    • %s\n", p)
		}
	}
	if len(dRes.CustomMCPServers) == 0 {
		fmt.Fprintln(stdout, "  No unmanaged MCP servers found.")
	} else {
		fmt.Fprintf(stdout, "  Found %d unmanaged MCP servers:\n", len(dRes.CustomMCPServers))
		for _, s := range dRes.CustomMCPServers {
			fmt.Fprintf(stdout, "    • %s\n", s)
		}
	}
	return 0
}
