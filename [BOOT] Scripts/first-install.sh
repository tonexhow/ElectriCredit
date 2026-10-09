#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

if [[ $EUID -eq 0 ]]; then
  echo "Run FIRST_TERMINAL_PROMPT.txt as your normal Raspberry Pi desktop user, not from a root shell." >&2
  exit 1
fi

if ! command -v sudo >/dev/null 2>&1; then
  echo "sudo is required on Raspberry Pi OS." >&2
  exit 1
fi

printf '\nElectriCredit first-time Raspberry Pi setup\n'
printf 'Project: %s\n' "$PROJECT_DIR"
printf 'User:    %s\n\n' "$(id -un)"

# Ask for sudo once up-front. The main installer runs as root but preserves this
# user via SUDO_USER for file ownership and the systemd service account.
sudo -v
exec sudo bash "$SCRIPT_DIR/install-autostart.sh"
