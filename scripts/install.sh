#!/usr/bin/env sh
set -eu

INSTALL_DIR="${1:-$HOME/.local/bin}"

if [ ! -f "bin/jirawarden" ]; then
    sh "$(dirname "$0")/build.sh"
fi

mkdir -p "$INSTALL_DIR"
cp bin/jirawarden "$INSTALL_DIR/jirawarden"
chmod +x "$INSTALL_DIR/jirawarden"

echo "Installed $INSTALL_DIR/jirawarden"
echo "Make sure $INSTALL_DIR is in PATH."
