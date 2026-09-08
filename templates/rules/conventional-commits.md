---
trigger: model_decision
description: Conventional Commits specification rules and git commit hygiene guidelines.
---

# Conventional Commits 1.0.0 Specification

All commits in managed repositories must conform to the Conventional Commits 1.0.0 specification:

```text
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Allowed Types
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes only
- `style`: Formatting changes that do not affect code logic (whitespace, formatting)
- `refactor`: Code changes that neither fix a bug nor add a feature
- `perf`: Code changes that improve performance
- `test`: Adding missing tests or correcting existing tests
- `build`: Changes affecting the build system or external dependencies
- `ci`: Changes to CI configuration files and scripts
- `chore`: Maintenance tasks, version bumps, or miscellaneous tooling updates
- `revert`: Reverting a previous commit

### Rules
- The description must be in lowercase and use the imperative mood (e.g., "add", not "added").
- Do not end the subject line with a period.
- Breaking changes must include a `!` before the colon (e.g., `feat(api)!: breaking change`) or a `BREAKING CHANGE:` footer.
