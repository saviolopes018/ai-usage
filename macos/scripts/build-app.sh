#!/bin/zsh
set -euo pipefail

SCRIPT_DIR="${0:A:h}"
MACOS_DIR="${SCRIPT_DIR:h}"
OUTPUT_DIR="${1:-${MACOS_DIR}/dist}"
APP_DIR="${OUTPUT_DIR}/AI Usage Monitor.app"

cd "${MACOS_DIR}"
swift build -c release
BIN_DIR="$(swift build -c release --show-bin-path)"

mkdir -p "${APP_DIR}/Contents/MacOS" "${APP_DIR}/Contents/Resources"
cp "${BIN_DIR}/AIUsageMenu" "${APP_DIR}/Contents/MacOS/AIUsageMenu"
cp "${MACOS_DIR}/Resources/Info.plist" "${APP_DIR}/Contents/Info.plist"
chmod 755 "${APP_DIR}/Contents/MacOS/AIUsageMenu"
codesign --force --deep --sign - "${APP_DIR}"

echo "App criado em: ${APP_DIR}"
