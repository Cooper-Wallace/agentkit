#!/usr/bin/env bash
# install.sh: Symlink agentkit into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="${SCRIPT_DIR}/bin/agentkit"
TARGET_DIR="${HOME}/.local/bin"

if [[ ! -f "$BIN_SRC" ]]; then
    echo -e "\033[1;31m[ERROR]\033[0m Could not find binary source at: $BIN_SRC"
    exit 1
fi

mkdir -p "$TARGET_DIR"
chmod +x "$BIN_SRC"

# Primary executable
ALIASES=("agentkit")

# Clean up legacy symlinks
rm -f "${TARGET_DIR}/ak" "${TARGET_DIR}/agy-standardise" "${TARGET_DIR}/antigravity-standardise"

echo -e "\033[1;32m[SUCCESS]\033[0m Installing agentkit..."
for alias_name in "${ALIASES[@]}"; do
    target_path="${TARGET_DIR}/${alias_name}"
    ln -sf "$BIN_SRC" "$target_path"
    echo "  • ${target_path} -> $BIN_SRC"
done

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
else
    echo "agentkit is ready to use! Try running: agentkit --help"
fi

echo "To configure plugins and autostart wake-lock, run: agentkit apply"
