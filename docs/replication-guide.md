# Multi-Machine Replication Guide

This guide walks through configuring a fresh developer workstation (laptop, desktop, or cloud workstation) to achieve 100% Antigravity compliance in under two minutes.

---

## 1. Prerequisites

Before replicating Antigravity configurations:
1. **Google Antigravity CLI (`agy`)**: Installed at `~/.local/bin/agy`.
2. **Git & GitHub CLI**: Authenticated via `gh auth login`.
3. **Python 3.10+**: Available in system `$PATH`.

---

## 2. Replication Workflow

### Step 1: Clone the Repository
Clone `agentkit` into your personal `Projects` directory:
```bash
git clone https://github.com/Cooper-Wallace/agentkit.git ~/Projects/agentkit
cd ~/Projects/agentkit
```

### Step 2: Install CLI Symlinks
Run the standalone installer to link `agentkit` and `ak` into `~/.local/bin/`:
```bash
bash install.sh
```

Ensure `~/.local/bin` is in your shell's `$PATH`:
```bash
echo $PATH | grep -q "$HOME/.local/bin" || echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

### Step 3: Audit Current State (Optional)
Check the workstation's baseline compliance score:
```bash
agentkit audit
# or:
ak audit
```

### Step 4: Apply Standards & Directives
Idempotently merge standard permissions into `~/.gemini/antigravity-cli/settings.json` and install global `GEMINI.md` / `AGENTS.md`:
```bash
agentkit apply
# or:
ak apply
```

To also place a symlink in your home root (`~/GEMINI.md`):
```bash
ak apply --link-home
```

### Step 5: Verify Full Compliance
Run the audit engine again to confirm all checks pass:
```bash
agentkit audit
# or:
ak audit
```

Expected output:
```text
[SUCCESS] Agent harness configuration is fully compliant! (8/8 checks passed)
```

---

## 3. Integration with RKO (`riced-kubuntu-os`)

For Kubuntu systems managed by `riced-kubuntu-os`, `agentkit` standardization can be invoked directly within the bootstrap sequence:

```bash
# In riced-kubuntu-os/scripts/03-dotfiles.sh:
if [ -d "$HOME/Projects/agentkit" ]; then
    echo "Applying agentkit standards..."
    python3 "$HOME/Projects/agentkit/bin/agentkit" apply
fi
```
