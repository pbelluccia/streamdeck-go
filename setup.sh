#!/usr/bin/env bash
set -Eeuo pipefail

readonly ICON_PACK_URL_DEFAULT="https://github.com/czottmann/streamdeck-iconpack-fluentui-system-icons/archive/refs/heads/main.zip"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${HOME}/.local/share/streamdeck-go"
CONFIG_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/streamdeck-go"
CONFIG_FILE="${CONFIG_DIR}/config.json"
SYSTEMD_DIR="${HOME}/.config/systemd/user"
SERVICE_NAME="streamdeck-go.service"
LEGACY_SERVICE_NAME="streamdeckd.service"
DAEMON_BIN_NAME="streamdeck-go"
LEGACY_DAEMON_BIN_NAME="streamdeckd"
CODEX_DIR="${HOME}/.codex"
CLAUDE_DIR="${HOME}/.claude"
ICON_PACK_URL="${ICON_PACK_URL:-$ICON_PACK_URL_DEFAULT}"
ICON_VARIANT="${ICON_VARIANT:-regular}"
START_SERVICE=true
INSTALL_UDEV=false
INSTALL_CODEX_HOOKS=false
INSTALL_CLAUDE_HOOKS=false
SKIP_ICONS=false
TMPDIR=""

usage() {
  cat <<EOF
Usage: ./setup.sh [OPTIONS]

Builds streamdeck-go, downloads the Fluent UI Stream Deck icon pack, installs
the user service, and starts it.

Options:
  --no-start             Install but do not start the user service
  --install-udev         Also install the udev rule with sudo
  --install-codex-hooks  Install global Codex hooks for Stream Deck alerts
  --install-claude-hooks Install global Claude Code hooks for Stream Deck alerts
  --install-agent-hooks  Install both Codex and Claude Code hooks
  --skip-icons           Do not download icons; keep existing installed icons
  --icon-variant NAME    regular or filled (default: regular)
  -h, --help             Show this help

Environment overrides:
  ICON_PACK_URL          Icon pack zip URL
  ICON_VARIANT           regular or filled
EOF
}

log() {
  printf '==> %s\n' "$*"
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$TMPDIR" && -d "$TMPDIR" ]]; then
    rm -rf -- "$TMPDIR"
  fi
}

trap cleanup EXIT

require_command() {
  local -r name="$1"
  command -v "$name" >/dev/null 2>&1 || die "missing command: $name"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --no-start)
        START_SERVICE=false
        shift
        ;;
      --install-udev)
        INSTALL_UDEV=true
        shift
        ;;
      --install-codex-hooks)
        INSTALL_CODEX_HOOKS=true
        shift
        ;;
      --install-claude-hooks)
        INSTALL_CLAUDE_HOOKS=true
        shift
        ;;
      --install-agent-hooks)
        INSTALL_CODEX_HOOKS=true
        INSTALL_CLAUDE_HOOKS=true
        shift
        ;;
      --skip-icons)
        SKIP_ICONS=true
        shift
        ;;
      --icon-variant)
        [[ $# -ge 2 ]] || die "--icon-variant requires a value"
        ICON_VARIANT="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown option: $1"
        ;;
    esac
  done
}

validate_config() {
  case "$ICON_VARIANT" in
    regular|filled) ;;
    *) die "--icon-variant must be 'regular' or 'filled'" ;;
  esac
}

download_icons() {
  local pack_dir
  TMPDIR="$(mktemp -d)"

  log "Downloading icon pack"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$ICON_PACK_URL" -o "$TMPDIR/iconpack.zip"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMPDIR/iconpack.zip" "$ICON_PACK_URL"
  else
    die "missing command: curl or wget"
  fi

  log "Extracting icon pack"
  unzip -q "$TMPDIR/iconpack.zip" -d "$TMPDIR"
  pack_dir="$(find "$TMPDIR" -type d -name "fluentui-system-icons-${ICON_VARIANT}.sdIconPack" | head -n 1)"
  [[ -n "$pack_dir" && -d "$pack_dir/icons" ]] || die "could not find ${ICON_VARIANT} .sdIconPack/icons in downloaded archive"

  log "Installing icons to $DATA_DIR/icons"
  rm -rf -- "$DATA_DIR/icons"
  mkdir -p "$DATA_DIR/icons"
  cp -R "$pack_dir/icons/." "$DATA_DIR/icons/"
}

build_and_install_binary() {
  log "Building streamdeck-go"
  mkdir -p "$SCRIPT_DIR/bin" "$BIN_DIR"
  (cd "$SCRIPT_DIR/src" && go build -o "$SCRIPT_DIR/bin/${DAEMON_BIN_NAME}" ./cmd/streamdeck-go)
  (cd "$SCRIPT_DIR/src" && go build -o "$SCRIPT_DIR/bin/streamdeck-admin" ./cmd/streamdeck-admin)
  install -Dm755 "$SCRIPT_DIR/bin/${DAEMON_BIN_NAME}" "$BIN_DIR/${DAEMON_BIN_NAME}"
  install -Dm755 "$SCRIPT_DIR/bin/streamdeck-admin" "$BIN_DIR/streamdeck-admin"
  rm -f "$SCRIPT_DIR/bin/${LEGACY_DAEMON_BIN_NAME}" "$BIN_DIR/${LEGACY_DAEMON_BIN_NAME}"
}

install_default_config() {
  mkdir -p "$CONFIG_DIR"
  if [[ -e "$CONFIG_FILE" ]]; then
    log "Keeping existing config at $CONFIG_FILE"
    return
  fi

  log "Installing default config to $CONFIG_FILE"
  cat > "$CONFIG_FILE" <<EOF
{
  "version": 1,
  "settings": {
    "device": "auto",
    "model": "auto",
    "icon_dir": "${DATA_DIR}/icons",
    "brightness": 20,
    "hold_ms": 1000,
    "font": {
      "path": "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"
    },
    "media": {
      "player": "spotify"
    },
    "weather": {
      "location": "Buenos Aires",
      "refresh_minutes": 10
    },
    "start_page": "main"
  },
  "pages": {
    "main": {
      "background": { "type": "media_art", "mode": "fill" },
      "buttons": {
        "0": {
          "layers": [
            { "type": "icon", "icon": "previous.png" }
          ],
          "press": { "type": "media", "command": "previous" }
        },
        "1": {
          "layers": [
            { "type": "media_play_pause" }
          ],
          "press": { "type": "media", "command": "play_pause" }
        },
        "2": {
          "layers": [
            { "type": "icon", "icon": "next.png" }
          ],
          "press": { "type": "media", "command": "next" }
        },
        "3": {
          "layers": [
            { "type": "color", "color": "#1f2937" },
            { "type": "weather" }
          ],
          "press": { "type": "command", "command": "" }
        },
        "4": {
          "layers": [
            { "type": "color", "color": "#111827" },
            { "type": "datetime", "format": "ddd DD\\nHH:mm", "font_size": 18 }
          ],
          "press": { "type": "command", "command": "wtype -M win -P M" }
        },
        "5": {
          "layers": [
            { "type": "empty" }
          ],
          "press": { "type": "display_mode", "command": "cycle" }
        }
      }
    }
  }
}
EOF
}

install_service() {
  log "Installing user service"
  mkdir -p "$SYSTEMD_DIR"
  install -Dm644 "$SCRIPT_DIR/contrib/${SERVICE_NAME}" "$SYSTEMD_DIR/${SERVICE_NAME}"
  rm -f "$SYSTEMD_DIR/${SERVICE_NAME}.d/override.conf"
  if [[ -e "$SYSTEMD_DIR/${LEGACY_SERVICE_NAME}" ||
        -L "$SYSTEMD_DIR/default.target.wants/${LEGACY_SERVICE_NAME}" ||
        -d "$SYSTEMD_DIR/${LEGACY_SERVICE_NAME}.d" ]]; then
    log "Removing legacy user service ${LEGACY_SERVICE_NAME}"
  fi
  systemctl --user disable --now "$LEGACY_SERVICE_NAME" >/dev/null 2>&1 || true
  rm -f "$SYSTEMD_DIR/${LEGACY_SERVICE_NAME}" \
    "$SYSTEMD_DIR/default.target.wants/${LEGACY_SERVICE_NAME}" \
    "$SYSTEMD_DIR/${LEGACY_SERVICE_NAME}.d/override.conf"
  rmdir "$SYSTEMD_DIR/${LEGACY_SERVICE_NAME}.d" >/dev/null 2>&1 || true
  systemctl --user daemon-reload
}

install_codex_hooks() {
  log "Installing global Codex hooks"
  mkdir -p "$CODEX_DIR/hooks" "$BIN_DIR"
  install -Dm755 "$SCRIPT_DIR/scripts/agent-hooks/streamdeck-agent-notify.py" "$BIN_DIR/streamdeck-agent-notify"
  install -Dm755 "$SCRIPT_DIR/scripts/agent-hooks/streamdeck-agent-notify.py" "$BIN_DIR/codex-streamdeck-notify"
  install -Dm755 "$SCRIPT_DIR/scripts/agent-hooks/codex-streamdeck-hook.py" "$CODEX_DIR/hooks/streamdeck_notify.py"
  python3 "$SCRIPT_DIR/scripts/install-agent-hooks.py" \
    --agent codex \
    --config "$CODEX_DIR/hooks.json" \
    --command "$CODEX_DIR/hooks/streamdeck_notify.py"
}

install_claude_hooks() {
  log "Installing global Claude Code hooks"
  mkdir -p "$CLAUDE_DIR/hooks" "$BIN_DIR"
  install -Dm755 "$SCRIPT_DIR/scripts/agent-hooks/streamdeck-agent-notify.py" "$BIN_DIR/streamdeck-agent-notify"
  install -Dm755 "$SCRIPT_DIR/scripts/agent-hooks/claude-streamdeck-hook.py" "$CLAUDE_DIR/hooks/streamdeck_notify.py"
  python3 "$SCRIPT_DIR/scripts/install-agent-hooks.py" \
    --agent claude \
    --config "$CLAUDE_DIR/settings.json" \
    --command "$CLAUDE_DIR/hooks/streamdeck_notify.py"
}

install_udev_rule() {
  log "Installing udev rule with sudo"
  sudo install -Dm644 "$SCRIPT_DIR/contrib/99-streamdeck.rules" /etc/udev/rules.d/99-streamdeck.rules
  sudo rm -f /etc/udev/rules.d/99-streamdeck-mini.rules
  sudo udevadm control --reload-rules
  sudo udevadm trigger
}

start_service() {
  if [[ "$START_SERVICE" == true ]]; then
    log "Starting user service"
    systemctl --user enable "$SERVICE_NAME"
    systemctl --user restart "$SERVICE_NAME"
  fi
}

print_summary() {
  cat <<EOF

Installed:
  ${BIN_DIR}/${DAEMON_BIN_NAME}
  ${BIN_DIR}/streamdeck-admin
  ${DATA_DIR}/icons
  ${CONFIG_FILE}
  ${SYSTEMD_DIR}/${SERVICE_NAME}

Useful commands:
  streamdeck-admin
  systemctl --user status ${SERVICE_NAME}
  journalctl --user -u ${SERVICE_NAME} -f
  systemctl --user restart ${SERVICE_NAME}
  systemctl --user stop ${SERVICE_NAME}

If the daemon cannot open the Stream Deck, install the udev rule:
  ./setup.sh --install-udev
Then unplug and reconnect the Stream Deck.
EOF
  if [[ "$INSTALL_CODEX_HOOKS" == true || "$INSTALL_CLAUDE_HOOKS" == true ]]; then
    cat <<EOF

Agent hook notes:
  Codex hooks:      ${CODEX_DIR}/hooks.json
  Claude hooks:     ${CLAUDE_DIR}/settings.json
  Notifier command: ${BIN_DIR}/streamdeck-agent-notify

Review hooks once from the agent UI if prompted:
  Codex:  /hooks
  Claude: /hooks
EOF
  fi
}

main() {
  parse_args "$@"
  validate_config

  require_command go
  require_command systemctl
  require_command install
  require_command find
  if [[ "$SKIP_ICONS" == false ]]; then
    require_command unzip
  fi
  if [[ "$INSTALL_CODEX_HOOKS" == true || "$INSTALL_CLAUDE_HOOKS" == true ]]; then
    require_command python3
  fi

  mkdir -p "$DATA_DIR"

  build_and_install_binary
  if [[ "$SKIP_ICONS" == false ]]; then
    download_icons
  fi
  install_default_config
  install_service
  if [[ "$INSTALL_CODEX_HOOKS" == true ]]; then
    install_codex_hooks
  fi
  if [[ "$INSTALL_CLAUDE_HOOKS" == true ]]; then
    install_claude_hooks
  fi
  if [[ "$INSTALL_UDEV" == true ]]; then
    install_udev_rule
  fi
  start_service
  print_summary
}

main "$@"
