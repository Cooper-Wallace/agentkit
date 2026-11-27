package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultAgentsTemplate = `# {{PROJECT_NAME}} - Developer Directives

Welcome to {{PROJECT_NAME}}. {{PROJECT_DESCRIPTION}}

---

## 1. Primary Stack & Test Standards

- **Primary Language**: {{PRIMARY_LANGUAGE}}
- **Verification Command**: ` + "`{{TEST_COMMAND}}`" + `
- Ensure all tests pass before committing.
`

const defaultHookTemplate = `{
  "command-safety-guard": {
    "enabled": false,
    "description": "Block destructive commands"
  },
  "code-linter-formatter": {
    "enabled": false,
    "description": "Run standard linter and formatter"
  }
}`

var evidenceFiles = []string{
	"intent.md",
	"design.md",
	"agentic-workflow.md",
	"verify.md",
	"operate.md",
	"decisions.md",
}

var defaultEvidenceTemplates = map[string]string{
	"intent.md": `# {{PROJECT_NAME}} - Intent & Scope

## Purpose
{{PROJECT_DESCRIPTION}}

## Primary Stack
- **Language**: {{PRIMARY_LANGUAGE}}
- **Verification**: ` + "`{{TEST_COMMAND}}`" + `

## Goals & Success Criteria
- Define measurable functional and reliability outcomes for {{PROJECT_NAME}}.
- Document explicit boundaries and non-goals to prevent scope drift.
`,
	"design.md": `# {{PROJECT_NAME}} - Architecture & Design

## System Overview
{{PROJECT_DESCRIPTION}}

## Components & Boundaries
- **Primary Language**: {{PRIMARY_LANGUAGE}}
- Document core modules, data flows, and external interfaces.

## Error Handling & Failure Modes
- Define guarded failure paths, input validation rules, and safe degradation behavior.
`,
	"agentic-workflow.md": `# {{PROJECT_NAME}} - Agentic Workflow

## Progressive Disclosure
Keep root ` + "`AGENTS.md`" + ` concise (<= 80 lines target, <= 120 lines max) and store detailed context in ` + "`evidence/`" + `.

## Development & Verification Loop
- **Language**: {{PRIMARY_LANGUAGE}}
- **Test Command**: ` + "`{{TEST_COMMAND}}`" + `
- Create one focused branch per change and verify all tests pass before opening a PR.
`,
	"verify.md": `# {{PROJECT_NAME}} - Verification & Quality Gates

## Automated Checks
Run the primary test suite before completing any task:
` + "```bash\n{{TEST_COMMAND}}\n```" + `

## Coverage Expectations
- Happy-path behavior for {{PROJECT_NAME}} ({{PRIMARY_LANGUAGE}})
- Guarded-failure, validation-rejection, and safe-degrade edge cases
`,
	"operate.md": `# {{PROJECT_NAME}} - Operations & Runbook

## Environment & Runtime
- **Project**: {{PROJECT_NAME}}
- **Stack**: {{PRIMARY_LANGUAGE}}

## Health & Diagnostics
- Verify baseline health by running: ` + "`{{TEST_COMMAND}}`" + `
- Document deployment steps, configuration keys, and rollback procedures.
`,
	"decisions.md": `# {{PROJECT_NAME}} - Architecture Decision Records

Record significant architectural and operational decisions for {{PROJECT_NAME}} below.

## ADR-0001: Initial Evidence Baseline
- **Status**: Accepted
- **Context**: {{PROJECT_DESCRIPTION}}
- **Decision**: Adopt progressive-disclosure ` + "`evidence/`" + ` documentation alongside concise root directives.
- **Consequences**: Core verification (` + "`{{TEST_COMMAND}}`" + `) and design context stay versioned in-repo.
`,
}

// Options configures template generation.
type Options struct {
	Type        string // "agents", "gemini", "project-gemini", "hook", "evidence"
	TargetDir   string
	ProjectName string
	Description string
	Language    string
	TestCommand string
	Force       bool
	TemplateSrc string // Optional custom template source override
}

// Scaffold generates template files based on options.
func Scaffold(opts Options) error {
	tType := strings.ToLower(strings.TrimSpace(opts.Type))
	if tType == "" {
		tType = "agents"
	}

	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = "."
	}
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolve target directory: %w", err)
	}

	if err := os.MkdirAll(absTargetDir, 0755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	projectName := opts.ProjectName
	if projectName == "" {
		projectName = filepath.Base(absTargetDir)
	}
	desc := opts.Description
	if desc == "" {
		desc = "Project-specific developer workspace."
	}
	lang := opts.Language
	if lang == "" {
		lang = "Python / Polyglot"
	}
	testCmd := opts.TestCommand
	if testCmd == "" {
		testCmd = "python3 -m unittest discover tests -v"
	}

	switch tType {
	case "hook":
		hookPath := filepath.Join(absTargetDir, "hooks.json")
		if _, err := os.Stat(hookPath); err == nil && !opts.Force {
			return fmt.Errorf("%s already exists. Use --force to overwrite", hookPath)
		}
		var content []byte
		if opts.TemplateSrc != "" {
			c, err := os.ReadFile(opts.TemplateSrc)
			if err != nil {
				return fmt.Errorf("read template source: %w", err)
			}
			content = c
		} else if repoHooks := filepath.Join("harness", "hooks", "hooks.json"); fileExists(repoHooks) {
			c, err := os.ReadFile(repoHooks)
			if err == nil {
				content = c
			}
		}
		if len(content) == 0 {
			content = []byte(defaultHookTemplate)
		}
		// Validate JSON
		var js map[string]interface{}
		if err := json.Unmarshal(content, &js); err != nil {
			return fmt.Errorf("invalid json in hook template: %w", err)
		}
		if err := os.WriteFile(hookPath, content, 0644); err != nil {
			return fmt.Errorf("write hook template: %w", err)
		}
		return nil

	case "agents", "gemini", "project-gemini":
		var primaryName, secondaryName string
		switch tType {
		case "gemini":
			primaryName = "GEMINI.md"
			secondaryName = "AGENTS.md"
		case "project-gemini":
			primaryName = "project-GEMINI.md"
			secondaryName = "project-AGENTS.md"
		default: // "agents"
			primaryName = "AGENTS.md"
			secondaryName = "GEMINI.md"
		}

		primaryFile := filepath.Join(absTargetDir, primaryName)
		secondaryFile := filepath.Join(absTargetDir, secondaryName)

		if _, err := os.Stat(primaryFile); err == nil && !opts.Force {
			return fmt.Errorf("%s already exists. Use --force to overwrite", primaryFile)
		}

		var rawTemplate string
		if opts.TemplateSrc != "" {
			c, err := os.ReadFile(opts.TemplateSrc)
			if err != nil {
				return fmt.Errorf("read template source: %w", err)
			}
			rawTemplate = string(c)
		} else {
			candidates := []string{
				filepath.Join("project", "templates", "AGENTS.md"),
				filepath.Join("project", "templates", "project-GEMINI.md"),
				filepath.Join("templates", "AGENTS.md"),
			}
			for _, cPath := range candidates {
				if fileExists(cPath) {
					if c, err := os.ReadFile(cPath); err == nil {
						rawTemplate = string(c)
						break
					}
				}
			}
		}

		if rawTemplate == "" {
			rawTemplate = defaultAgentsTemplate
		}

		content := applyPlaceholders(rawTemplate, projectName, desc, lang, testCmd)

		if err := os.WriteFile(primaryFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("write primary template %s: %w", primaryFile, err)
		}

		// Symlink secondary to primary
		_ = os.Remove(secondaryFile)
		if err := os.Symlink(primaryName, secondaryFile); err != nil {
			return fmt.Errorf("create symlink %s -> %s: %w", secondaryFile, primaryName, err)
		}
		return nil

	case "evidence":
		return scaffoldEvidence(absTargetDir, opts, projectName, desc, lang, testCmd)

	default:
		return fmt.Errorf("unknown template type %q (supported: agents, gemini, project-gemini, hook, evidence)", tType)
	}
}

func scaffoldEvidence(absTargetDir string, opts Options, projectName, desc, lang, testCmd string) error {
	var customSrcDir string
	if opts.TemplateSrc != "" {
		info, err := os.Stat(opts.TemplateSrc)
		if err != nil {
			return fmt.Errorf("read template source: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("evidence template source %q must be a directory", opts.TemplateSrc)
		}
		customSrcDir = opts.TemplateSrc
	}

	evidenceDir := filepath.Join(absTargetDir, "evidence")
	if !opts.Force {
		for _, name := range evidenceFiles {
			targetPath := filepath.Join(evidenceDir, name)
			if _, err := os.Stat(targetPath); err == nil {
				return fmt.Errorf("%s already exists. Use --force to overwrite", targetPath)
			}
		}
	}

	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	for _, name := range evidenceFiles {
		rawTemplate := loadEvidenceTemplate(name, customSrcDir)
		content := applyPlaceholders(rawTemplate, projectName, desc, lang, testCmd)
		targetPath := filepath.Join(evidenceDir, name)
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write evidence template %s: %w", targetPath, err)
		}
	}
	return nil
}

func loadEvidenceTemplate(name, customSrcDir string) string {
	var candidates []string
	if customSrcDir != "" {
		candidates = append(candidates, filepath.Join(customSrcDir, name))
	}
	candidates = append(candidates,
		filepath.Join("project", "templates", "evidence", name),
		filepath.Join("templates", "evidence", name),
	)
	for _, cPath := range candidates {
		if fileExists(cPath) {
			if c, err := os.ReadFile(cPath); err == nil {
				return string(c)
			}
		}
	}
	return defaultEvidenceTemplates[name]
}

func applyPlaceholders(raw, projectName, desc, lang, testCmd string) string {
	content := raw
	content = strings.ReplaceAll(content, "{{PROJECT_NAME}}", projectName)
	content = strings.ReplaceAll(content, "{{PROJECT_DESCRIPTION}}", desc)
	content = strings.ReplaceAll(content, "{{PRIMARY_LANGUAGE}}", lang)
	content = strings.ReplaceAll(content, "{{TEST_COMMAND}}", testCmd)
	return content
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
