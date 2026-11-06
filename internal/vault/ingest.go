package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// IngestedItem records metadata about an ingested user directive or note.
type IngestedItem struct {
	Source        string   `json:"source"`
	ProcessedPath string   `json:"processed_path,omitempty"`
	Section       string   `json:"section,omitempty"`
	Title         string   `json:"title"`
	Project       string   `json:"project"`
	Tags          []string `json:"tags,omitempty"`
	Content       string   `json:"content"`
}

// IngestResult reports directives ingested and processed paths.
type IngestResult struct {
	IngestedCount int            `json:"ingested_count"`
	Items         []IngestedItem `json:"items"`
	DryRun        bool           `json:"dry_run,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// Ingest scans User-Input/pending/ and project notes, archiving directives and appending them to project state.
func Ingest(vaultPath string, project string, dryRun bool) (IngestResult, error) {
	resolved := ResolveVaultPath(vaultPath)
	res := IngestResult{
		Items:  []IngestedItem{},
		DryRun: dryRun,
	}

	fi, err := os.Stat(resolved)
	if err != nil || !fi.IsDir() {
		res.Error = fmt.Sprintf("Obsidian AgentVault does not exist at %s", resolved)
		return res, fmt.Errorf("Obsidian AgentVault does not exist at %s", resolved)
	}

	processedDir := filepath.Join(resolved, "User-Input", "processed")
	if !dryRun {
		_ = os.MkdirAll(processedDir, 0755)
	}

	now := time.Now()
	isoDate := now.Format("2006-01-02")
	isoTS := now.Format(time.RFC3339)

	// 1. Scan User-Input/pending and 00-Inbox/pending
	pendingDirs := []string{
		filepath.Join(resolved, "User-Input", "pending"),
		filepath.Join(resolved, "00-Inbox", "pending"),
	}

	for _, pDir := range pendingDirs {
		entries, err := os.ReadDir(pDir)
		if err != nil {
			continue
		}

		var mdFiles []os.DirEntry
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !strings.HasPrefix(e.Name(), ".") {
				mdFiles = append(mdFiles, e)
			}
		}
		sort.Slice(mdFiles, func(i, j int) bool {
			return mdFiles[i].Name() < mdFiles[j].Name()
		})

		for _, file := range mdFiles {
			filePath := filepath.Join(pDir, file.Name())
			rawBytes, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			rawText := string(rawBytes)
			meta, body := ParseFrontmatterAndBody(rawText)

			targetProject := project
			if targetProject == "" {
				if projMeta, ok := meta["project"].(string); ok && projMeta != "" {
					targetProject = projMeta
				} else {
					targetProject = "general"
				}
			}
			targetProject = strings.TrimSuffix(targetProject, ".md")

			title := strings.TrimSuffix(file.Name(), ".md")
			if t, ok := meta["title"].(string); ok && t != "" {
				title = t
			}

			var tags []string
			if tagList, ok := meta["tags"].([]string); ok {
				tags = tagList
			}

			// Destination filename with date prefix
			var destName string
			if strings.HasPrefix(file.Name(), isoDate) {
				destName = file.Name()
			} else {
				destName = fmt.Sprintf("%s-%s", isoDate, file.Name())
			}

			destPath := filepath.Join(processedDir, destName)
			if _, err := os.Stat(destPath); err == nil {
				counter := 1
				baseStem := strings.TrimSuffix(destName, ".md")
				for {
					destName = fmt.Sprintf("%s_%d.md", baseStem, counter)
					destPath = filepath.Join(processedDir, destName)
					if _, err := os.Stat(destPath); os.IsNotExist(err) {
						break
					}
					counter++
				}
			}

			auditStamp := fmt.Sprintf("\n\n> [!check] Ingested by agentkit on %s\n", isoTS)
			processedText := strings.TrimRight(rawText, "\r\n") + auditStamp

			relSource := filepath.Join(filepath.Base(filepath.Dir(pDir)), filepath.Base(pDir), file.Name())
			relProcessed := filepath.Join("User-Input", "processed", destName)

			if !dryRun {
				if err := os.WriteFile(destPath, []byte(processedText), 0644); err != nil {
					return res, fmt.Errorf("failed to write processed note %s: %w", destPath, err)
				}
				_ = os.Remove(filePath)

				// Append directive link or content to Projects/<project>.md if exists
				projFile := filepath.Join(resolved, "Projects", targetProject+".md")
				if _, err := os.Stat(projFile); os.IsNotExist(err) {
					_ = os.WriteFile(projFile, []byte(GetProjectTemplate(targetProject)), 0644)
				}
				if pBytes, err := os.ReadFile(projFile); err == nil {
					pText := string(pBytes)
					sectionHeading := "## Offline User Input & Notes"
					if strings.Contains(pText, sectionHeading) {
						directiveSummary := strings.TrimSpace(body)
						if len(directiveSummary) > 100 {
							directiveSummary = directiveSummary[:97] + "..."
						}
						entry := fmt.Sprintf("\n- Ingested: [[User-Input/processed/%s|%s]] — %s", strings.TrimSuffix(destName, ".md"), title, directiveSummary)
						if !strings.Contains(pText, fmt.Sprintf("[[User-Input/processed/%s|", strings.TrimSuffix(destName, ".md"))) {
							var updatedPText string
							if strings.Contains(pText, "### Processed Directives") {
								parts := strings.SplitN(pText, "### Processed Directives", 2)
								updatedPText = parts[0] + "### Processed Directives" + entry + "\n" + strings.TrimLeft(parts[1], "\r\n")
							} else {
								procBlock := fmt.Sprintf("\n\n### Processed Directives%s\n\n> [!check] Ingested by agentkit on %s\n", entry, isoTS)
								updatedPText = strings.TrimRight(pText, "\r\n") + procBlock
							}
							_ = os.WriteFile(projFile, []byte(updatedPText), 0644)
						}
					}
				}
			}

			res.Items = append(res.Items, IngestedItem{
				Source:        relSource,
				ProcessedPath: relProcessed,
				Title:         title,
				Project:       targetProject,
				Tags:          tags,
				Content:       strings.TrimSpace(rawText),
			})
		}
	}

	// 2. If project specified, inspect Projects/<project>.md for inline directives under ## Offline User Input & Notes
	if project != "" {
		projStem := strings.TrimSuffix(project, ".md")
		projFile := filepath.Join(resolved, "Projects", projStem+".md")
		if rawBytes, err := os.ReadFile(projFile); err == nil {
			projText := string(rawBytes)
			sectionHeader := "## Offline User Input & Notes"
			if strings.Contains(projText, sectionHeader) {
				parts := strings.SplitN(projText, sectionHeader, 2)
				beforeSection := parts[0]
				sectionAndAfter := parts[1]

				var sectionContent, afterSection string
				nextH2Idx := strings.Index(sectionAndAfter, "\n## ")
				if nextH2Idx != -1 {
					sectionContent = sectionAndAfter[:nextH2Idx]
					afterSection = sectionAndAfter[nextH2Idx:]
				} else {
					sectionContent = sectionAndAfter
					afterSection = ""
				}

				pendingSection := sectionContent
				processedSection := ""
				if strings.Contains(sectionContent, "### Processed Directives") {
					partsP := strings.SplitN(sectionContent, "### Processed Directives", 2)
					pendingSection = partsP[0]
					processedSection = "### Processed Directives" + partsP[1]
				}

				var meaningfulLines []string
				for _, line := range strings.Split(pendingSection, "\n") {
					lineS := strings.TrimSpace(line)
					if lineS == "" {
						continue
					}
					if strings.HasPrefix(lineS, "> [!todo]") ||
						strings.HasPrefix(lineS, "> Add task requests") ||
						strings.HasPrefix(lineS, "> [!check] Ingested") ||
						strings.HasPrefix(lineS, "- Ingested:") ||
						strings.HasPrefix(lineS, "- [[User-Input/processed/") {
						continue
					}
					meaningfulLines = append(meaningfulLines, line)
				}

				if len(meaningfulLines) > 0 {
					directiveText := strings.TrimSpace(strings.Join(meaningfulLines, "\n"))
					res.Items = append(res.Items, IngestedItem{
						Source:  fmt.Sprintf("Projects/%s.md", projStem),
						Section: "Offline User Input & Notes",
						Title:   fmt.Sprintf("%s Offline Notes", projStem),
						Project: projStem,
						Content: directiveText,
					})

					if !dryRun {
						auditEntry := fmt.Sprintf("\n\n### Processed Directives\n%s\n\n> [!check] Ingested by agentkit on %s\n", directiveText, isoTS)
						var combinedProcessed string
						if processedSection != "" {
							combinedProcessed = auditEntry + "\n" + strings.Replace(processedSection, "### Processed Directives\n", "", 1)
						} else {
							combinedProcessed = auditEntry
						}

						resetPending := "\n> [!todo] Pending Notes\n> Add task requests or design considerations here for automatic ingestion.\n"
						newSection := resetPending + combinedProcessed
						updatedText := beforeSection + sectionHeader + newSection + afterSection
						_ = os.WriteFile(projFile, []byte(updatedText), 0644)
					}
				}
			}
		}
	}

	res.IngestedCount = len(res.Items)
	return res, nil
}
