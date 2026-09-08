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
Clone `antigravity-standardise` into your personal `Projects` directory:
```bash
git clone https://github.com/Cooper-Wallace/antigravity-standardise.git ~/Projects/antigravity-standardise
cd ~/Projects/antigravity-standardise
```

### Step 2: Install CLI Symlinks
Run the standalone installer to link `agy-standardise` and `antigravity-standardise` into `~/.local/bin/`:
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
agy-standardise audit
```

### Step 4: Apply Standards & Directives
Idempotently merge standard permissions into `~/.gemini/antigravity-cli/settings.json` and install global `GEMINI.md` / `AGENTS.md`:
```bash
agy-standardise apply
```

To also place a symlink in your home root (`~/GEMINI.md`):
```bash
agy-standardise apply --link-home
```

### Step 5: Verify Full Compliance
Run the audit engine again to confirm all checks pass:
```bash
agy-standardise audit
```

Expected output:
```text
[SUCCESS] Antigravity CLI configuration is fully compliant! (6/6 checks passed)
```

---

## 3. Integration with RKO (`riced-kubuntu-os`)

For Kubuntu systems managed by `riced-kubuntu-os`, Antigravity CLI standardization can be invoked directly within the bootstrap sequence:

```bash
# In riced-kubuntu-os/scripts/03-dotfiles.sh:
if [ -d "$HOME/Projects/antigravity-standardise" ]; then
    echo "Applying Antigravity standards..."
    python3 "$HOME/Projects/antigravity-standardise/bin/agy-standardise" apply
fi
```
