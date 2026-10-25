## Summary
<!-- Provide a clear, concise summary of the problem and proposed solution. -->

## Type of Change
<!-- Please check the item that best describes this change: -->
- [ ] `feat`: New feature or capability
- [ ] `fix`: Bug fix
- [ ] `refactor`: Code refactor (no behavioral changes)
- [ ] `perf`: Performance improvement
- [ ] `docs`: Documentation changes
- [ ] `test`: New or updated tests
- [ ] `ci`: CI/CD pipeline or build configuration
- [ ] `chore`: Maintenance, dependencies, or tooling

## Key Changes
<!-- Bullet points detailing the specific changes made. -->
- 

## Verification & Testing
<!-- Detail the testing performed to verify this change. -->
- [ ] Automated tests pass (`pytest`, `mvn test`, `npm test`, etc.)
- [ ] Code formatting & linting checks pass
- [ ] Manual test / simulation verification (include details/logs below)

**Test Details:**
<!-- Details or output from testing -->

## Quality Gates & Sign-Off Checklist
- [ ] **Gate 1: Automated Quality Gates** (All CI checks pass: `ci / Test & Coverage`, `Semantic PR Title Check`)
- [ ] **Gate 2: Agent Review Verification** (Adversarial diff review conducted; no unresolved warnings, regressions, or security leaks)
- [ ] **Gate 3: Human Review & Manual Promotion** (PR reviewed by human developer; merge must only be triggered manually in the GitHub UI, never automated)

## Pre-Merge Checklist
- [ ] PR title follows [Conventional Commits](https://www.conventionalcommits.org/) (e.g. `feat(nav): add laser filter`)
- [ ] Branch is rebased/up-to-date with `main`
- [ ] No temporary debug code or commented-out blocks left in diff
