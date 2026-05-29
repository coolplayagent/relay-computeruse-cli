# RelayComputerUse

RelayComputerUse is a small desktop automation CLI for agents. It exposes
Windows and Linux computer-use actions as subcommands and prints machine-readable
JSON to stdout.

The binary name is `relay-computer-use`.

## Commands

```bash
relay-computer-use screenshot --out screen.png
relay-computer-use zoom --out zoom.png --x1 100 --y1 100 --x2 500 --y2 400
relay-computer-use list-windows
relay-computer-use focus-window --title "Notepad"
relay-computer-use mouse-move --x 120 --y 240
relay-computer-use click --x 120 --y 240
relay-computer-use right-click --x 120 --y 240
relay-computer-use middle-click --x 120 --y 240
relay-computer-use double-click --x 120 --y 240
relay-computer-use triple-click --x 120 --y 240
relay-computer-use left-mouse-down --x 120 --y 240
relay-computer-use left-mouse-up --x 120 --y 240
relay-computer-use --allow-risk destructive drag --from-x 120 --from-y 240 --to-x 360 --to-y 420
relay-computer-use type-text --text "hello"
relay-computer-use hotkey --keys "Ctrl+A"
relay-computer-use hold-key --keys Shift --duration 1s
relay-computer-use scroll --direction down --amount 3
relay-computer-use wait --duration 1s
relay-computer-use --allow-risk destructive launch-app --name notepad
relay-computer-use wait-window --title "Notepad" --timeout 10s
echo '{"action":"screenshot","out":"screen.png"}' | relay-computer-use exec-json
```

Global flags must appear before the command:

```bash
relay-computer-use --runtime fake --pretty screenshot --out screen.png
relay-computer-use --allow-risk destructive launch-app --name notepad
```

## JSON Output

Successful calls return:

```json
{
  "ok": true,
  "action": "screenshot",
  "message": "Captured screenshot.",
  "observation": {
    "path": "screen.png",
    "mime_type": "image/png",
    "width": 320,
    "height": 180,
    "windows": []
  },
  "data": {
    "risk": "safe",
    "runtime": "fake"
  }
}
```

Failures return `ok=false` and a normalized error:

```json
{
  "ok": false,
  "error": {
    "code": "policy_denied",
    "message": "action launch-app requires risk destructive; current --allow-risk is guarded",
    "retryable": false
  }
}
```

Screenshots are written to files. The JSON response returns the file path,
dimensions, mime type, and virtual screen origin when the backend can discover
it.

## JSON Action Protocol

`exec-json` accepts a single action request from stdin, `--in <path>`, or
`--json <request>`. It returns the same normalized JSON response as the
subcommands.

```bash
relay-computer-use --runtime fake exec-json --json '{"action":"left_click","x":120,"y":240}'
relay-computer-use --runtime fake exec-json --observe-out observed.png --json '{"action":"left_click","x":120,"y":240,"observe_after":true}'
```

Supported action names are `screenshot`, `zoom`, `list_windows`,
`focus_window`, `mouse_move`, `left_click`, `right_click`, `middle_click`,
`double_click`, `triple_click`, `left_mouse_down`, `left_mouse_up`,
`left_click_drag`, `type`, `key`, `hold_key`, `scroll`, `wait`, `launch_app`,
and `wait_window`. Hyphenated names and existing CLI aliases are normalized.

When `observe_after` is true for an input action, the runtime captures a
screenshot after executing the action and returns that screenshot as the
observation. Provide `observe_out` in the request or `--observe-out` on the
command line.

## Risk Policy

RelayComputerUse performs policy checks but does not ask humans for approval.
Agent runtimes should handle user approval before invoking risky commands.

Risk levels:

- `safe`: screenshot, zoom, list windows, wait, wait window
- `guarded`: focus window, mouse move, click, double-click, right-click,
  middle-click, triple-click, mouse down/up, type text, hotkey, hold key, scroll
- `destructive`: drag, launch app

The default `--allow-risk` is `guarded`. Destructive commands require
`--allow-risk destructive`.

## Underscore action aliases

The CLI also accepts action names with underscores:
`left_click`, `right_click`, `middle_click`, `double_click`, `triple_click`,
`mouse_move`, `left_mouse_down`, `left_mouse_up`, `left_click_drag`,
`type`, `key`, and `hold_key`.

## Runtime Selection

- `--runtime auto`: use the host OS backend.
- `--runtime fake`: use a scripted backend for CI and deterministic tests.

On unsupported platforms, use `--runtime fake`.

## Linux Requirements

Linux requires an active graphical session with `DISPLAY` or `WAYLAND_DISPLAY`.

Install these tools as appropriate:

- Window discovery: `wmctrl`, or `xdotool` with `xprop`
- Input control: `xdotool`
- Screenshot: one of `gnome-screenshot`, `grim`, `scrot`, ImageMagick `import`,
  or `flameshot`
- Calculator smoke test: one of `gnome-calculator`, `kcalc`, `mate-calc`,
  `galculator`, `xcalc`, or `gtk-launch org.gnome.Calculator`

RelayComputerUse starts Linux GUI apps with `GDK_BACKEND=x11` and
`QT_QPA_PLATFORM=xcb` when those variables are not already set.

## Windows Requirements

Windows requires a normal interactive desktop session. The backend uses Win32
APIs for screenshots, window enumeration, focus, mouse input, keyboard input,
and text input. Application launch supports common aliases such as `notepad`,
`calc`, `calculator`, `Notepad`, and `Calculator`.

## Smoke Tests

Windows:

```powershell
relay-computer-use launch-app --name notepad --allow-risk destructive
relay-computer-use wait-window --title Notepad
relay-computer-use screenshot --out notepad.png
relay-computer-use type-text --text "hello from RelayComputerUse"
relay-computer-use hotkey --keys "Ctrl+A"
```

Linux:

```bash
relay-computer-use launch-app --name calculator --allow-risk destructive
relay-computer-use wait-window --title calculator
relay-computer-use screenshot --out calculator.png
relay-computer-use scroll --amount -3
```
