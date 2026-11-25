package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Cooper-Wallace/agentkit/internal/hook"
)

func printHookHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: agentkit hook <guard-quota|notify-wake|filter-and-wrap|headroom-wrap> [options]

Subcommands:
  guard-quota       PreToolUse hook for quota monitoring, fallback routing, and wrap-up gating.
                    Options: --dry-run

  notify-wake       PreInvocation/PreToolUse/Stop hook to notify the caffeine daemon of agent state.
                    Arguments: <active|idle> [--dry-run]

  filter-and-wrap   PreToolUse hook to rewrite verbose commands through headroom-wrap for token compression.
                    Options: --dry-run

  headroom-wrap     Execute a command and compress verbose output with Reversible CCR caching.
                    Usage: agentkit hook headroom-wrap [--dry-run] -- <command> [args...]`)
}

func runHook(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printHookHelp(stdout)
		return 1
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "help", "-h", "--help":
		printHookHelp(stdout)
		return 0

	case "guard-quota":
		return hook.GuardQuota(subArgs, os.Stdin, stdout)

	case "notify-wake":
		return hook.NotifyWake(subArgs, os.Stdin, stdout)

	case "filter-and-wrap":
		return hook.FilterAndWrap(subArgs, os.Stdin, stdout)

	case "headroom-wrap":
		return hook.HeadroomWrap(subArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "Unknown hook subcommand: %s\n", subcommand)
		printHookHelp(stderr)
		return 1
	}
}
