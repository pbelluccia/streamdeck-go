# streamdeck-go

`streamdeck-go` is a lightweight Linux daemon for button-only Elgato Stream Deck devices. It talks directly to Linux `hidraw`, renders key images locally, and uses MPRIS media metadata through `playerctl`.

The project keeps Go dependencies small and talks directly to the device. The only runtime tools it expects are normal desktop utilities such as `playerctl` and, if you keep the default key 4 action, `wtype`.

## Features

- Direct HID support for Mini, 15-key, and XL button-only Stream Deck devices.
- Album art spread across the current key grid.
- MPRIS media previous/play-pause/next controls.
- Weather and clock overlays.
- Event-driven media updates through `playerctl --follow`.
- Per-key image hashing to avoid unnecessary redraws.
- Robust reconnect loop for disconnected or temporarily unavailable devices.
- User-level `systemd` service support.

Supported product IDs:

- `0fd9:0063` Stream Deck Mini
- `0fd9:0090` Stream Deck Mini 2022
- `0fd9:00b3` Stream Deck Mini Discord
- `0fd9:00b8` Stream Deck 6-Key Module
- `0fd9:006d` Stream Deck 2019
- `0fd9:0080` Stream Deck Mk.2
- `0fd9:00a5` Stream Deck Mk.2 Scissor Keys
- `0fd9:00b9` Stream Deck 15-Key Module
- `0fd9:006c` Stream Deck XL
- `0fd9:008f` Stream Deck XL 2022
- `0fd9:00ba` Stream Deck Module 32

The original Stream Deck `0fd9:0060`, Stream Deck +, Stream Deck Neo, Stream Deck Studio, pedals, dials, touch strips, and encoder/display features are not covered.

## Requirements

On Linux Mint / Ubuntu:

```bash
sudo apt update
sudo apt install golang-go playerctl unzip curl
```

Optional, for any JSON action that uses `wtype` on Wayland:

```bash
sudo apt install wtype
```

Optional, only when installing Codex or Claude Code hooks:

```bash
sudo apt install python3
```

The selected media player must expose MPRIS metadata. To list valid player names:

```bash
playerctl --list-all
```

The desktop Spotify client exposes MPRIS metadata.

## Quick setup

Run the setup script from the repository root:

```bash
./setup.sh
```

It will:

- build `streamdeck-go`
- download the Fluent UI Stream Deck icon pack
- install the binary to `~/.local/bin/streamdeck-go`
- install icons to `~/.local/share/streamdeck-go/icons`
- create `~/.config/streamdeck-go/config.json` if it does not exist
- install and start the `streamdeck-go.service` `systemd --user` service

If the daemon cannot open the Stream Deck, install the included `udev` rule:

```bash
./setup.sh --install-udev
```

Then unplug and reconnect the Stream Deck.

Useful setup options:

```bash
./setup.sh --skip-icons --no-start
./setup.sh --skip-icons --install-codex-hooks
./setup.sh --skip-icons --install-claude-hooks
./setup.sh --skip-icons --install-agent-hooks
```

## Build

From the repository root:

```bash
make test
make build
```

The binary is written to:

```text
bin/streamdeck-go
bin/streamdeck-admin
```

Run locally:

```bash
./bin/streamdeck-go
./bin/streamdeck-admin
```

## Device permissions

Install the included `udev` rule so the daemon can open `/dev/hidraw*` without `sudo`:

```bash
sudo install -Dm644 contrib/99-streamdeck.rules /etc/udev/rules.d/99-streamdeck.rules
sudo udevadm control --reload-rules
sudo udevadm trigger
```

Then unplug and reconnect the Stream Deck.

## Install as a user service manually

The recommended path is `./setup.sh`. For manual installation, build and copy the files yourself or use:

```bash
./scripts/install-user.sh --no-start
```

Start it:

```bash
systemctl --user enable --now streamdeck-go.service
```

Watch logs:

```bash
journalctl --user -u streamdeck-go.service -f
```

Stop it:

```bash
systemctl --user stop streamdeck-go.service
```

## Configure with the web UI

Run the local admin UI:

```bash
streamdeck-admin
```

Then open:

```text
http://127.0.0.1:8787
```

The admin reads `~/.config/streamdeck-go/config.json`, renders a model-aware preview using the same Go renderer as the daemon, and offers page/button editing, backups, restore, save, and service restart.
Actions of type `page` use a selector populated from the current pages and are validated before saving.

Typical web setup flow:

1. Install with `./setup.sh` so the binary, service, icons, and default JSON exist.
2. Run `streamdeck-admin`.
3. Open `http://127.0.0.1:8787`.
4. Edit global settings first: device, model, brightness, media player, icon directory, font, weather, and start page.
5. Edit pages, button layers, and press/hold actions.
6. Use `Save + restart` to write `config.json` and restart `streamdeck-go.service`.

`streamdeck-admin` binds to `127.0.0.1:8787` by default. To use another local port:

```bash
STREAMDECK_ADMIN_ADDR=127.0.0.1:8788 streamdeck-admin
```

Do not expose the admin UI to an untrusted network; it can edit the daemon config and restart the user service.

## Usage

Display modes:

1. Album art with overlays.
2. Album art only.
3. Display off.
4. Overlays only.

`streamdeck-go` does not take layout or behavior flags. It always reads:

```text
~/.config/streamdeck-go/config.json
```

The daemon exits if the JSON is missing or invalid.

## Inject pages

While the daemon is running, local tools can inject a page through the Unix socket:

```bash
curl --unix-socket "$XDG_RUNTIME_DIR/streamdeck-go.sock" \
  -X PUT http://streamdeck/pages/notification \
  -H 'Content-Type: application/json' \
  -d '{
    "timeout_seconds": 5,
    "background": { "type": "solid", "color": "#111827" },
    "buttons": {
      "0": {
        "layers": [
          { "type": "color", "color": "#dc2626" },
          { "type": "text", "text": "Alert", "font_size": 18 }
        ],
        "press": { "type": "page", "page": "main" }
      }
    }
  }'
```

The body is exactly the same shape as one entry under `pages` in `config.json`.
The injected page is added to the same in-memory page map and displayed immediately, so all normal page features apply: numbered buttons, layers, colors, effects, actions, animated GIFs, and `timeout_seconds`.

To clear an injected page and return to the start page when it is active:

```bash
curl --unix-socket "$XDG_RUNTIME_DIR/streamdeck-go.sock" \
  -X POST http://streamdeck/pages/notification/clear
```

The default socket path is `$XDG_RUNTIME_DIR/streamdeck-go.sock`. If `XDG_RUNTIME_DIR` is not set, it falls back to `/tmp/streamdeck-go-$UID.sock`.

## Agent hooks

`setup.sh` can install global Codex and Claude Code hooks that notify the Stream Deck when an agent needs attention or finishes a turn:

```bash
./setup.sh --skip-icons --install-agent-hooks
```

Available hook flags:

- `--install-codex-hooks`: installs `~/.codex/hooks.json` entries for `PermissionRequest`, `PostToolUse`, `UserPromptSubmit`, and `Stop`.
- `--install-claude-hooks`: installs `~/.claude/settings.json` entries for `PermissionRequest`, `PostToolUse`, `PostToolUseFailure`, `PermissionDenied`, `UserPromptSubmit`, `Stop`, and `StopFailure`.
- `--install-agent-hooks`: installs both.

The installer preserves existing hook configuration and only adds or replaces the Stream Deck hook commands. Both agents use the same notifier command:

```bash
~/.local/bin/streamdeck-agent-notify "Codex listo" "Tests OK" --agent Codex
```

Hook scripts resolve the notifier from `STREAMDECK_AGENT_NOTIFY`, then from `PATH`, then from `~/.local/bin/streamdeck-agent-notify`.

Codex and Claude Code may ask you to review/trust hooks once from their `/hooks` menu.

## Configure with JSON

The daemon and the web UI both use the same JSON file:

```text
~/.config/streamdeck-go/config.json
```

You can edit it manually, then restart the service:

```bash
$EDITOR ~/.config/streamdeck-go/config.json
systemctl --user restart streamdeck-go.service
```

Use absolute paths for `settings.icon_dir`, `settings.font.path`, and image assets outside the icon directory. Relative image and animation paths are resolved from `settings.icon_dir`.

Example config:

```json
{
  "version": 1,
  "settings": {
    "device": "auto",
    "model": "auto",
    "icon_dir": "/home/YOUR_USER/.local/share/streamdeck-go/icons",
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
            { "type": "media_play_pause" },
            { "type": "text", "text": "Spotify", "font_size": 12, "position": "lower" }
          ],
          "press": { "type": "media", "command": "play_pause" },
          "hold": { "type": "media", "command": "stop" }
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
            { "type": "datetime", "format": "ddd DD\nHH:mm", "font_size": 18 }
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
    },
    "settings": {
      "timeout_seconds": 10,
      "background": { "type": "solid" },
      "buttons": {
        "0": {
          "layers": [
            { "type": "icon", "icon": "back.png" },
            { "type": "text", "text": "Back", "font_size": 12, "position": "lower" }
          ],
          "press": { "type": "page", "page": "main" }
        }
      }
    }
  }
}
```

Supported layers:

`settings.device` may be `auto` or a `/dev/hidrawN` path. `settings.model` may be `auto`, `mini`, `classic`, or `xl`; the daemon detects the connected hardware when possible, and the web UI uses this value to choose the preview grid.

`buttons` is a numbered object keyed by physical button number, left-to-right and top-to-bottom:

- `mini`: keys `0` through `5`
- `classic`: keys `0` through `14`
- `xl`: keys `0` through `31`

Pages can use a solid color background:

```json
"background": { "type": "solid", "color": "#111827" }
```

Page backgrounds support optional `effect`:

```json
"background": {
  "type": "solid",
  "color": "#111827",
  "effect": {
    "type": "blink",
    "color": "#dc2626",
    "blink_ms": 500,
    "duration_ms": 3000,
    "repeat": 2
  }
}
```

Button visuals are defined only by `layers`. To give a button a solid background, add a `color` layer as the first layer:

```json
"layers": [
  {
    "type": "color",
    "color": "#111827",
    "effect": {
      "type": "blink",
      "color": "#16a34a",
      "blink_ms": 250
    }
  },
  { "type": "icon", "icon": "play.png" }
]
```

Color layers and page backgrounds support optional effects:

- `blink`: toggles between base `color` and `effect.color`; `blink_ms` controls toggle speed.
- `pulse`: fades smoothly from base `color` to `effect.color` and back; `duration_ms` controls the full cycle.
- `flash`: shows `effect.color` briefly, then base `color`; `duration_ms` controls the on-time.

For effects, `repeat` controls how many cycles run before stopping. Missing or `0` repeat means indefinite.

Pages support optional `timeout_seconds`. When a non-start page has a positive timeout, it returns to `settings.start_page` after that many seconds. Button activity on that page restarts the timer.

Pages can use a full-page image background:

```json
"background": { "type": "image", "path": "/path/to/background.png", "mode": "fill" }
```

Button layers are rendered on top of page backgrounds in the order listed, so an image layer can act as a button-local background and later layers can add icons or text.
Use a `color` layer when you want a per-button color to participate in layer ordering, for example below an icon but above the page background.

Text uses the system font configured in `settings.font.path`. It accepts `.ttf` and `.otf` files; if the file cannot be loaded, the renderer falls back to the built-in bitmap font.

- `empty`
- `icon` with `icon`, optional `size`, `position`, `offset-x`, `offset-y`, `outline_color`, and `outline_width`
- `color` with `color` and optional `effect`
- `image` with `path` and optional `mode`
- `animation` with GIF `path`, optional `mode`, `offset-x`, and `offset-y`
- `media_play_pause` with optional `player`
- `weather` with optional `position`, `offset-x`, `offset-y`, `color`, `outline_color`, and `outline_width`
- `datetime` with optional token `format`, `font_size`, `position`, `offset-x`, `offset-y`, `color`, `outline_color`, and `outline_width`
- `text` with `text`, optional `font_size`, `position`, `offset-x`, `offset-y`, `color`, `outline_color`, and `outline_width`

Layer `position` accepts `upper`, `center`, or `lower`. Text defaults to `center` when `position` is omitted.
Layer `offset-x` and `offset-y` are optional pixel offsets applied after `position` is calculated. Positive `offset-x` moves right, negative moves left; positive `offset-y` moves down, negative moves up.
Layer colors use hexadecimal `#RRGGBB` or `RRGGBB`. Text uses `color` for the fill and `outline_color` for the stroke. Icon and `media_play_pause` layers use `outline_color` for the icon stroke.

Image modes:

- `fit`: keep aspect ratio and fit inside the key
- `fill`: keep aspect ratio and crop to fill the key
- `center`: draw at natural size centered on the key
- `stretch`: resize exactly to the key

Supported actions:

- `media` with `command`: `previous`, `play_pause`, `next`, `play`, `pause`, or `stop`; optional `player`
- `command` with shell `command`
- `display_mode` with `command: "cycle"`
- `brightness` with `command`: `up`, `down`, or `set`
- `page` with `page` or `command` naming the target page

Brightness actions support optional `step` for `up` and `down`; the default step is `10`.
`set` uses `value`. Brightness is clamped to `0` through `100`.

```json
{ "type": "brightness", "command": "up", "step": 10 }
{ "type": "brightness", "command": "down", "step": 10 }
{ "type": "brightness", "command": "set", "value": 25 }
```

Buttons support optional `hold` with the same action shape as `press`. A button with `hold` runs `press` only when released before `settings.hold_ms`; after that threshold it runs only `hold`.

Datetime formats use readable tokens:

- `YYYY`, `YY`: year
- `MMMM`, `MMM`, `MM`, `M`: month
- `DD`, `D`: day of month
- `dddd`, `ddd`: weekday
- `HH`, `H`: 24-hour
- `hh`, `h`: 12-hour
- `mm`, `m`: minutes
- `ss`, `s`: seconds
- `A`, `a`: AM/PM

Example:

```json
{
  "layers": [
    { "type": "image", "path": "/home/YOUR_USER/Pictures/bg.png", "mode": "fill" },
    { "type": "animation", "path": "/home/YOUR_USER/Pictures/flame.gif", "mode": "fit", "offset-y": -6 },
    { "type": "icon", "icon": "lightbulb.png", "size": 42, "position": "center", "offset-y": -4, "outline_color": "#111827" },
    { "type": "text", "text": "Living", "font_size": 14, "position": "lower", "offset-y": 2, "color": "#ffffff", "outline_color": "#111827" }
  ]
}
```

Animated GIF layers are cached after first load. The daemon refreshes animated pages at 10 FPS and only uploads keys whose rendered device image changed.


If the Stream Deck is disconnected, the daemon backs off reconnect attempts up to 60 seconds. On Linux it also watches HID device changes, so a newly exposed `/dev/hidraw*` wakes the reconnect loop before the timer expires. It does not start media watchers or render work until the device is available.

## Development

Project layout:

```text
src/cmd/streamdeck-go   CLI entry point
src/internal/app        application loop, media/weather/actions
src/internal/deck       hidraw discovery and Stream Deck HID protocols
src/internal/render     image composition, device image encoding, bitmap font
contrib/                udev and systemd support files
scripts/                install helpers
setup.sh                build, icon download, and user service installer
```

Run tests:

```bash
make test
```

Run without installing:

```bash
make run
```

Install the current build over the user service and restart it:

```bash
make deploy
```
