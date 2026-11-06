package vault

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SyncResult details synchronized projects and git status.
type SyncResult struct {
	Synced          []string `json:"synced"`
	DryRun          bool     `json:"dry_run,omitempty"`
	GitStatus       string   `json:"git_status,omitempty"`
	GitRemoteSynced bool     `json:"git_remote_synced"`
}

// Sync synchronizes project state, tasks, and ADR links, with git integration.
func Sync(vaultPath string, project string, dryRun bool, noPush bool) (SyncResult, error) {
	resolved := ResolveVaultPath(vaultPath)
	res := SyncResult{
		Synced: []string{},
		DryRun: dryRun,
	}

	fi, err := os.Stat(resolved)
	if err != nil || !fi.IsDir() {
		return res, fmt.Errorf("Obsidian AgentVault does not exist at %s", resolved)
	}

	projectsDir := filepath.Join(resolved, "Projects")
	if !dryRun {
		_ = os.MkdirAll(projectsDir, 0755)
	}

	// 1. Collect available ADRs from Architecture/decisions/
	decisionsDir := filepath.Join(resolved, "Architecture", "decisions")
	var adrLinks []string
	if dEntries, err := os.ReadDir(decisionsDir); err == nil {
		var adrFiles []string
		for _, e := range dEntries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" && !strings.HasPrefix(e.Name(), ".") {
				adrFiles = append(adrFiles, e.Name())
			}
		}
		sort.Strings(adrFiles)
		for _, f := range adrFiles {
			stem := strings.TrimSuffix(f, ".md")
			adrLinks = append(adrLinks, fmt.Sprintf("- [[Architecture/decisions/%s|%s]]", stem, stem))
		}
	}

	// 2. Determine target projects
	var targetProjects []string
	if project != "" {
		targetProjects = []string{strings.TrimSuffix(project, ".md")}
	} else {
		if pEntries, err := os.ReadDir(projectsDir); err == nil {
			for _, e := range pEntries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !strings.HasPrefix(e.Name(), ".") {
					targetProjects = append(targetProjects, strings.TrimSuffix(e.Name(), ".md"))
				}
			}
		}
		if len(targetProjects) == 0 {
			targetProjects = []string{
				"agentkit",
				"repokit",
				"riced-kubuntu-os",
				"workstation-setup",
				"augurs-bazaar",
				"augurs-saber",
			}
		}
	}
	sort.Strings(targetProjects)

	// 3. Sync each target project
	for _, proj := range targetProjects {
		projFile := filepath.Join(projectsDir, proj+".md")
		if _, err := os.Stat(projFile); err == nil {
			rawBytes, err := os.ReadFile(projFile)
			if err == nil {
				content := string(rawBytes)
				meta, body := ParseFrontmatterAndBody(content)
				meta["last_updated"] = time.Now().Format("2006-01-02")
				meta["updated_by"] = "agentkit"
				if _, ok := meta["status"]; !ok {
					meta["status"] = "active"
				}

				if len(adrLinks) > 0 {
					adrBlock := strings.Join(adrLinks, "\n")
					if !strings.Contains(body, "## Architecture Decisions (ADRs)") {
						body = strings.TrimRight(body, "\r\n") + fmt.Sprintf("\n\n## Architecture Decisions (ADRs)\n%s\n", adrBlock)
					} else {
						// Ensure each adr link is present in the document
						for _, link := range adrLinks {
							if !strings.Contains(body, link) {
								parts := strings.SplitN(body, "## Architecture Decisions (ADRs)", 2)
								body = parts[0] + "## Architecture Decisions (ADRs)\n" + link + "\n" + strings.TrimLeft(parts[1], "\r\n")
							}
						}
					}
				}

				if !strings.Contains(body, "## Active Tasks") {
					body = strings.TrimRight(body, "\r\n") + "\n\n## Active Tasks\n- [ ] Sync project state and directives\n"
				}

				var tags []string
				if tList, ok := meta["tags"].([]string); ok {
					tags = tList
				} else {
					tags = []string{"project", proj}
				}
				delete(meta, "tags")
				delete(meta, "type")
				delete(meta, "last_updated")
				delete(meta, "updated_by")

				newFM := FormatFrontmatter(tags, "project", "agentkit", meta)
				newContent := newFM + "\n\n" + strings.TrimSpace(body) + "\n"

				if !dryRun {
					_ = os.WriteFile(projFile, []byte(newContent), 0644)
				}
			}
		} else {
			if !dryRun {
				_ = os.WriteFile(projFile, []byte(GetProjectTemplate(proj)), 0644)
			}
		}
		res.Synced = append(res.Synced, proj)
	}

	// 4. Git integration
	gitDir := filepath.Join(resolved, ".git")
	if fi, err := os.Stat(gitDir); err == nil && (fi.IsDir() || !fi.IsDir()) {
		if dryRun {
			res.GitStatus = "dry-run"
		} else {
			// Stage all changes
			_, _ = runGit(resolved, "add", "-A")

			// Check status
			statusOut, _ := runGit(resolved, "status", "--porcelain")
			if strings.TrimSpace(statusOut) != "" {
				_, _ = runGit(resolved, "commit", "-m", "chore(vault): sync project state and session notes")
			}

			// Check for remote 'origin'
			remoteOut, rErr := runGit(resolved, "remote", "get-url", "origin")
			hasRemote := (rErr == nil && strings.TrimSpace(remoteOut) != "")

			if !hasRemote {
				res.GitRemoteSynced = false
				res.GitStatus = "local_only"
			} else if noPush {
				res.GitRemoteSynced = false
				res.GitStatus = "no_push"
			} else {
				// Rebase pull
				_, _ = runGit(resolved, "pull", "--rebase", "--autostash", "origin", "main")

				// Push to origin main
				pushOut, pushErr := runGit(resolved, "push", "origin", "main")
				if pushErr == nil {
					res.GitRemoteSynced = true
					res.GitStatus = "synced"
				} else {
					res.GitRemoteSynced = false
					res.GitStatus = "push_failed"
					_ = pushOut
				}
			}
		}
	}

	return res, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}
