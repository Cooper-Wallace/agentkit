package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Cooper-Wallace/agentkit/internal/vault"
)

// RunVault is the exported CLI entry point for 'agentkit vault' commands.
func RunVault(args []string) int {
	return runVault(args, os.Stdout, os.Stderr)
}

func printVaultHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: agentkit vault <subcommand> [options]

Subcommands:
  status   Report Obsidian AgentVault inventory, project counts, and pending user notes.
           Options: --path <dir>, --json

  init     Idempotently initialize AgentVault directories, templates, and trustedWorkspaces.
           Options: --path <dir>, --settings-path <path>

  ingest   Collect and process offline directives from User-Input/pending/ and project notes.
           Options: --project <name>, --dry-run, --path <dir>, --json

  sync     Idempotently sync project state, tasks, and ADR links in AgentVault.
           Options: --project <name>, --dry-run, --no-push, --path <dir>, --json

  note     Create structured note with wikilinks or append markdown section to project note.
           Options: <title>, --title <title>, --section <name>, --content <text>, --tag <tag>, --project <name>, --path <dir>, --json`)
}

func runVault(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printVaultHelp(stdout)
		return 1
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "help", "-h", "--help":
		printVaultHelp(stdout)
		return 0

	case "status":
		return runVaultStatus(subArgs, stdout, stderr)

	case "init":
		return runVaultInit(subArgs, stdout, stderr)

	case "ingest":
		return runVaultIngest(subArgs, stdout, stderr)

	case "sync":
		return runVaultSync(subArgs, stdout, stderr)

	case "note":
		return runVaultNote(subArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "Unknown vault subcommand: %s\n", subcommand)
		printVaultHelp(stderr)
		return 1
	}
}

func runVaultStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vault status", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var vaultPath string
	var jsonOutput bool

	fs.StringVar(&vaultPath, "path", "", "Path to Obsidian AgentVault root directory")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	status, err := vault.Status(vaultPath)
	if err != nil || !status.Exists {
		if jsonOutput {
			data, _ := json.MarshalIndent(status, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprintf(stdout, "Error: Obsidian AgentVault not found at %s. Run 'agentkit vault init' to initialize.\n", status.Path)
		}
		return 1
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprintln(stdout, "Obsidian AgentVault Status")
	fmt.Fprintf(stdout, "  Vault Path: %s\n", status.Path)
	fmt.Fprintf(stdout, "  Total Notes: %d\n", status.TotalNotes)
	fmt.Fprintf(stdout, "  Active Projects: %d (%s)\n", status.ActiveProjects, strings.Join(status.Projects, ", "))
	if status.PendingNotes > 0 {
		fmt.Fprintf(stdout, "  Pending User Notes: %d (%s)\n", status.PendingNotes, strings.Join(status.PendingFiles, ", "))
	} else {
		fmt.Fprintln(stdout, "  Pending User Notes: 0")
	}
	recent := status.RecentSession
	if recent == "" {
		recent = "None"
	}
	fmt.Fprintf(stdout, "  Most Recent Session: %s\n", recent)
	return 0
}

func runVaultInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vault init", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var vaultPath string
	var settingsPath string
	var harness string

	fs.StringVar(&vaultPath, "path", "", "Path to Obsidian AgentVault root directory")
	fs.StringVar(&settingsPath, "settings-path", "", "Path to harness settings.json")
	fs.StringVar(&harness, "harness", "antigravity", "Target agent harness")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	resolved := vault.ResolveVaultPath(vaultPath)
	err := vault.InitWithSettings(resolved, settingsPath)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to initialize AgentVault: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "AgentVault initialized successfully: %s\n", resolved)
	fmt.Fprintf(stdout, "Dashboard available at: %s/Dashboard.md\n", resolved)
	return 0
}

func runVaultIngest(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vault ingest", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var vaultPath string
	var project string
	var dryRun bool
	var jsonOutput bool

	fs.StringVar(&vaultPath, "path", "", "Path to Obsidian AgentVault root directory")
	fs.StringVar(&project, "project", "", "Target project name")
	fs.BoolVar(&dryRun, "dry-run", false, "Simulate ingestion without moving files")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	res, err := vault.Ingest(vaultPath, project, dryRun)
	if err != nil {
		if jsonOutput {
			data, _ := json.MarshalIndent(res, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprintf(stderr, "Vault ingest error: %v\n", err)
		}
		return 1
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	if res.IngestedCount == 0 {
		fmt.Fprintln(stdout, "No pending user inputs found.")
		return 0
	}

	fmt.Fprintf(stdout, "Ingested %d Pending Directive(s)\n", res.IngestedCount)
	for _, item := range res.Items {
		src := item.Source
		dest := item.ProcessedPath
		if dest == "" {
			dest = src
		}
		fmt.Fprintf(stdout, "  %s -> %s\n", src, dest)
		snippet := strings.ReplaceAll(item.Content, "\n", " ")
		if len(snippet) > 80 {
			snippet = snippet[:77] + "..."
		}
		fmt.Fprintf(stdout, "    Directives: %s\n", snippet)
	}
	return 0
}

func runVaultSync(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vault sync", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var vaultPath string
	var project string
	var dryRun bool
	var noPush bool
	var jsonOutput bool

	fs.StringVar(&vaultPath, "path", "", "Path to Obsidian AgentVault root directory")
	fs.StringVar(&project, "project", "", "Target project name")
	fs.BoolVar(&dryRun, "dry-run", false, "Simulate sync without modifying files")
	fs.BoolVar(&noPush, "no-push", false, "Skip remote git push")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	res, err := vault.Sync(vaultPath, project, dryRun, noPush)
	if err != nil {
		if jsonOutput {
			data, _ := json.MarshalIndent(map[string]any{"error": err.Error(), "synced": []string{}}, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprintf(stderr, "Vault sync error: %v\n", err)
		}
		return 1
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	for _, proj := range res.Synced {
		prefix := "Synced"
		if dryRun {
			prefix = "[dry-run] Would sync"
		}
		fmt.Fprintf(stdout, "  %s project note: Projects/%s.md\n", prefix, proj)
	}

	if res.GitStatus != "" {
		fmt.Fprintf(stdout, "  Git Sync Status: %s (remote synced: %v)\n", res.GitStatus, res.GitRemoteSynced)
	}
	return 0
}

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiStringFlag) Set(val string) error {
	*m = append(*m, val)
	return nil
}

func runVaultNote(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vault note", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var vaultPath string
	var project string
	var title string
	var section string
	var content string
	var jsonOutput bool
	var tags multiStringFlag

	fs.StringVar(&vaultPath, "path", "", "Path to Obsidian AgentVault root directory")
	fs.StringVar(&project, "project", "agentkit", "Target project name")
	fs.StringVar(&title, "title", "", "Note title")
	fs.StringVar(&section, "section", "", "Section name to append to project note")
	fs.StringVar(&content, "content", "", "Note content")
	fs.Var(&tags, "tag", "Tags for the note (repeatable or comma-separated)")
	fs.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Positional title support: agentkit vault note "My Title" --content "..."
	remaining := fs.Args()
	if title == "" && len(remaining) > 0 {
		title = remaining[0]
	}

	if title == "" && section == "" {
		if jsonOutput {
			fmt.Fprintln(stdout, `{"error": "Note title or section is required"}`)
		} else {
			fmt.Fprintln(stderr, "Error: Note title or section is required.")
		}
		return 1
	}

	opts := vault.NoteOptions{
		VaultPath: vaultPath,
		Project:   project,
		Title:     title,
		Section:   section,
		Content:   content,
		Tags:      tags,
	}

	res, err := vault.Note(opts)
	if err != nil {
		if jsonOutput {
			fmt.Fprintf(stdout, "{\"error\": %q}\n", err.Error())
		} else {
			fmt.Fprintf(stderr, "Vault note error: %v\n", err)
		}
		return 1
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	switch res.Action {
	case "appended_section":
		fmt.Fprintf(stdout, "Appended section '%s' to Projects/%s.md\n", res.Section, res.Project)
	case "created":
		fmt.Fprintf(stdout, "Created structured note: Projects/%s\n", res.Path)
	default:
		fmt.Fprintf(stdout, "Appended content to existing note: Projects/%s\n", res.Path)
	}
	return 0
}
