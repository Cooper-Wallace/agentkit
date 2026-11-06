#!/usr/bin/env bash
# install.sh: Build and install agentkit into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${HOME}/.local/bin"
TARGET_BIN="${TARGET_DIR}/agentkit"

mkdir -p "$TARGET_DIR"

# Ensure Go is available
if ! command -v go >/dev/null 2>&1; then
    if [[ -x "/tmp/go/bin/go" ]]; then
        export PATH="/tmp/go/bin:$PATH"
    else
        echo -e "\033[1;31m[ERROR]\033[0m Go 1.23+ is required to build agentkit."
        exit 1
    fi
fi

echo -e "\033[1;34m[INFO]\033[0m Compiling agentkit from source..."
cd "$SCRIPT_DIR"
go build -ldflags="-s -w" -o "${TARGET_BIN}" ./cmd/agentkit
chmod +x "${TARGET_BIN}"

# Remove legacy symlinks if present
for legacy in "${TARGET_DIR}/ak" "${TARGET_DIR}/agy-standardise" "${TARGET_DIR}/antigravity-standardise"; do
    if [[ -L "$legacy" || -f "$legacy" ]]; then
        rm -f "$legacy"
        echo -e "\033[1;34m[INFO]\033[0m Removed legacy symlink: $legacy"
    fi
done

echo -e "\033[1;32m[SUCCESS]\033[0m Installed agentkit -> $TARGET_BIN"

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    # shellcheck disable=SC2016
    echo '  export PATH="$HOME/.local/bin:$PATH"'
else
    echo "agentkit is ready to use! Try running: agentkit --help"
fi

echo "To configure plugins and autostart wake-lock, run: agentkit apply"
