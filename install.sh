#!/usr/bin/env bash
# install.sh: Symlink agy-standardise into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="${SCRIPT_DIR}/bin/agy-standardise"
TARGET_DIR="${HOME}/.local/bin"
TARGET_BIN="${TARGET_DIR}/agy-standardise"
TARGET_ALIAS="${TARGET_DIR}/antigravity-standardise"

if [[ ! -f "$BIN_SRC" ]]; then
    echo -e "\033[1;31m[ERROR]\033[0m Could not find binary source at: $BIN_SRC"
    exit 1
fi

mkdir -p "$TARGET_DIR"
chmod +x "$BIN_SRC"

# Create symlinks for both primary command and alias
ln -sf "$BIN_SRC" "$TARGET_BIN"
ln -sf "$BIN_SRC" "$TARGET_ALIAS"

echo -e "\033[1;32m[SUCCESS]\033[0m Symlinked:"
echo "  • $TARGET_BIN -> $BIN_SRC"
echo "  • $TARGET_ALIAS -> $BIN_SRC"

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
else
    echo "agy-standardise is ready to use! Try running: agy-standardise --help"
fi
