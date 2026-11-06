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

// Options configures template generation.
type Options struct {
	Type        string // "agents", "gemini", "project-gemini", "hook"
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

		content := rawTemplate
		content = strings.ReplaceAll(content, "{{PROJECT_NAME}}", projectName)
		content = strings.ReplaceAll(content, "{{PROJECT_DESCRIPTION}}", desc)
		content = strings.ReplaceAll(content, "{{PRIMARY_LANGUAGE}}", lang)
		content = strings.ReplaceAll(content, "{{TEST_COMMAND}}", testCmd)

		if err := os.WriteFile(primaryFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("write primary template %s: %w", primaryFile, err)
		}

		// Symlink secondary to primary
		_ = os.Remove(secondaryFile)
		if err := os.Symlink(primaryName, secondaryFile); err != nil {
			return fmt.Errorf("create symlink %s -> %s: %w", secondaryFile, primaryName, err)
		}
		return nil

	default:
		return fmt.Errorf("unknown template type %q (supported: agents, gemini, project-gemini, hook)", tType)
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
