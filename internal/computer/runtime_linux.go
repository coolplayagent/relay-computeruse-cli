//go:build linux

package computer

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const linuxCommandTimeout = 10 * time.Second

type linuxRuntime struct{}

func newOSRuntime() Runtime {
	return linuxRuntime{}
}

func (linuxRuntime) Name() string {
	return "linux"
}

func (r linuxRuntime) Screenshot(ctx context.Context, out string) (Result, error) {
	if err := requireDisplay(); err != nil {
		return Result{}, err
	}
	if err := ensureOutputPath(out); err != nil {
		return Result{}, err
	}
	commands := screenshotCommands(out)
	if len(commands) == 0 {
		return Result{}, RuntimeError{Code: "missing_dependency", Message: "Linux screenshots require one of: gnome-screenshot, grim, scrot, import, flameshot"}
	}
	var failures []string
	for _, command := range commands {
		_ = os.Remove(out)
		if err := runCommand(ctx, command, nil); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		info, err := os.Stat(out)
		if err == nil && info.Size() > 0 {
			width, height := pngDimensions(out)
			return result("screenshot", "Captured Linux desktop screenshot.", Observation{
				Path:     out,
				MimeType: "image/png",
				Width:    width,
				Height:   height,
			}, nil), nil
		}
		failures = append(failures, "screenshot command produced no output: "+strings.Join(command, " "))
	}
	return Result{}, RuntimeError{Code: "command_failed", Message: strings.Join(failures, "; "), Retryable: true}
}

func (r linuxRuntime) Zoom(ctx context.Context, out string, x1 int, y1 int, x2 int, y2 int) (Result, error) {
	if err := requireDisplay(); err != nil {
		return Result{}, err
	}
	if err := ensureOutputPath(out); err != nil {
		return Result{}, err
	}
	tmp, err := os.CreateTemp("", "computer-use-zoom-*.png")
	if err != nil {
		return Result{}, RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)
	if _, err := r.Screenshot(ctx, tmpPath); err != nil {
		return Result{}, err
	}
	file, err := os.Open(tmpPath)
	if err != nil {
		return Result{}, RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	img, err := png.Decode(file)
	_ = file.Close()
	if err != nil {
		return Result{}, RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	cropped, err := cropImage(img, x1, y1, x2, y2)
	if err != nil {
		return Result{}, err
	}
	if err := writePNG(out, cropped); err != nil {
		return Result{}, err
	}
	return result("zoom", "Captured Linux desktop zoom region.", Observation{
		Path: out, MimeType: "image/png", Width: cropped.Bounds().Dx(), Height: cropped.Bounds().Dy(),
	}, nil), nil
}

func (r linuxRuntime) ListWindows(ctx context.Context) (Result, error) {
	windows, err := r.listWindows(ctx, true, false)
	if err != nil {
		return Result{}, err
	}
	return result("list-windows", "Listed Linux desktop windows.", Observation{
		Windows:       windows,
		FocusedWindow: focusedWindowTitle(windows),
	}, map[string]interface{}{"window_count": len(windows)}), nil
}

func (r linuxRuntime) FocusWindow(ctx context.Context, title string) (Result, error) {
	window, err := r.findWindow(ctx, title)
	if err != nil {
		return Result{}, err
	}
	if commandExists("wmctrl") {
		if err := runCommand(ctx, []string{"wmctrl", "-ia", window.ID}, nil); err != nil {
			return Result{}, err
		}
	} else {
		if err := runInputCommand(ctx, []string{"xdotool", "windowactivate", "--sync", window.ID}); err != nil {
			return Result{}, err
		}
	}
	time.Sleep(200 * time.Millisecond)
	return r.ListWindows(ctx)
}

func (r linuxRuntime) MouseMove(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y)}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "mouse-move", "Moved Linux desktop pointer.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) Click(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "click", "1"}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "click", "Clicked Linux desktop coordinates.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) RightClick(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "click", "3"}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "right-click", "Right-clicked Linux desktop coordinates.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) MiddleClick(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "click", "2"}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "middle-click", "Middle-clicked Linux desktop coordinates.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) DoubleClick(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "click", "--repeat", "2", "1"}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "double-click", "Double-clicked Linux desktop coordinates.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) TripleClick(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "click", "--repeat", "3", "1"}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "triple-click", "Triple-clicked Linux desktop coordinates.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) LeftMouseDown(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "mousedown", "1"}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "left-mouse-down", "Pressed Linux desktop left mouse button.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) LeftMouseUp(ctx context.Context, x int, y int) (Result, error) {
	if err := runInputCommand(ctx, []string{"xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "mouseup", "1"}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "left-mouse-up", "Released Linux desktop left mouse button.", map[string]interface{}{"x": x, "y": y})
}

func (r linuxRuntime) Drag(ctx context.Context, fromX int, fromY int, toX int, toY int) (Result, error) {
	if err := runInputCommand(ctx, []string{
		"xdotool", "mousemove", "--sync", strconv.Itoa(fromX), strconv.Itoa(fromY),
		"mousedown", "1", "mousemove", "--sync", strconv.Itoa(toX), strconv.Itoa(toY), "mouseup", "1",
	}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "drag", "Dragged Linux desktop coordinates.", map[string]interface{}{"from_x": fromX, "from_y": fromY, "to_x": toX, "to_y": toY})
}

func (r linuxRuntime) TypeText(ctx context.Context, text string) (Result, error) {
	if strings.TrimSpace(text) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--text is required"}
	}
	if err := runInputCommand(ctx, []string{"xdotool", "type", "--delay", "1", "--", text}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "type-text", "Typed text into the Linux desktop.", map[string]interface{}{"text": text})
}

func (r linuxRuntime) Hotkey(ctx context.Context, keys string) (Result, error) {
	normalized := normalizeLinuxShortcut(keys)
	if normalized == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--keys is required"}
	}
	if err := runInputCommand(ctx, []string{"xdotool", "key", normalized}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "hotkey", "Sent Linux desktop shortcut.", map[string]interface{}{"keys": keys})
}

func (r linuxRuntime) HoldKey(ctx context.Context, keys string, duration time.Duration) (Result, error) {
	normalized := normalizeLinuxShortcut(keys)
	if normalized == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--keys is required"}
	}
	if duration <= 0 {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--duration must be positive"}
	}
	if err := runInputCommand(ctx, []string{"xdotool", "keydown", normalized}); err != nil {
		return Result{}, err
	}
	time.Sleep(duration)
	if err := runInputCommand(ctx, []string{"xdotool", "keyup", normalized}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "hold-key", "Held Linux desktop key.", map[string]interface{}{"keys": keys, "duration_ms": duration.Milliseconds()})
}

func (r linuxRuntime) Scroll(ctx context.Context, direction string, amount int) (Result, error) {
	if amount == 0 {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--amount must not be zero"}
	}
	direction, _, horizontal, err := normalizeScroll(direction, amount)
	if err != nil {
		return Result{}, err
	}
	button := "5"
	if direction == "up" {
		button = "4"
	}
	if horizontal {
		button = "7"
		if direction == "left" {
			button = "6"
		}
	}
	repeat := amount
	if repeat < 0 {
		repeat = -repeat
	}
	if err := runInputCommand(ctx, []string{"xdotool", "click", "--repeat", strconv.Itoa(repeat), button}); err != nil {
		return Result{}, err
	}
	return r.pointerResult(ctx, "scroll", "Scrolled Linux desktop view.", map[string]interface{}{"amount": amount, "direction": direction})
}

func (r linuxRuntime) Wait(ctx context.Context, duration time.Duration) (Result, error) {
	if duration <= 0 {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--duration must be positive"}
	}
	select {
	case <-ctx.Done():
		return Result{}, RuntimeError{Code: "runtime_error", Message: ctx.Err().Error(), Retryable: true}
	case <-time.After(duration):
	}
	return r.pointerResult(ctx, "wait", "Waited Linux desktop duration.", map[string]interface{}{"duration_ms": duration.Milliseconds()})
}

func (r linuxRuntime) LaunchApp(ctx context.Context, name string) (Result, error) {
	if strings.TrimSpace(name) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--name is required"}
	}
	before, _ := r.listWindows(ctx, false, false)
	command, err := resolveLinuxLaunchCommand(name)
	if err != nil {
		return Result{}, err
	}
	env := os.Environ()
	env = appendIfMissing(env, "GDK_BACKEND=x11")
	env = appendIfMissing(env, "QT_QPA_PLATFORM=xcb")
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = env
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return Result{}, RuntimeError{Code: "command_failed", Message: err.Error(), Retryable: true}
	}
	window, err := r.waitForWindowMatch(ctx, name, before, 10*time.Second)
	if err != nil {
		return Result{}, err
	}
	windows, _ := r.listWindows(ctx, false, false)
	return result("launch-app", "Launched Linux desktop app.", Observation{
		Windows:       windows,
		FocusedWindow: focusedWindowTitle(windows),
	}, map[string]interface{}{"window_title": window.Title, "launched_command": strings.Join(command, " ")}), nil
}

func (r linuxRuntime) WaitWindow(ctx context.Context, title string, timeout time.Duration) (Result, error) {
	window, err := r.waitForWindowMatch(ctx, title, nil, timeout)
	if err != nil {
		return Result{}, err
	}
	windows, _ := r.listWindows(ctx, false, false)
	return result("wait-window", "Observed Linux desktop window.", Observation{
		Windows:       windows,
		FocusedWindow: focusedWindowTitle(windows),
	}, map[string]interface{}{"window_title": window.Title}), nil
}

func (r linuxRuntime) pointerResult(ctx context.Context, action string, message string, data map[string]interface{}) (Result, error) {
	windows, _ := r.listWindows(ctx, false, false)
	return result(action, message, Observation{Windows: windows, FocusedWindow: focusedWindowTitle(windows)}, data), nil
}

func (r linuxRuntime) findWindow(ctx context.Context, query string) (Window, error) {
	if strings.TrimSpace(query) == "" {
		return Window{}, RuntimeError{Code: "invalid_args", Message: "--title is required"}
	}
	windows, err := r.listWindows(ctx, true, false)
	if err != nil {
		return Window{}, err
	}
	for _, window := range windows {
		if windowMatches(window, query) {
			return window, nil
		}
	}
	return Window{}, RuntimeError{Code: "not_found", Message: "window not found: " + query, Retryable: true}
}

func (r linuxRuntime) waitForWindowMatch(ctx context.Context, query string, before []Window, timeout time.Duration) (Window, error) {
	if strings.TrimSpace(query) == "" {
		return Window{}, RuntimeError{Code: "invalid_args", Message: "--title is required"}
	}
	beforeIDs := map[string]bool{}
	for _, window := range before {
		beforeIDs[window.ID] = true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		windows, err := r.listWindows(ctx, true, false)
		if err != nil {
			return Window{}, err
		}
		for _, window := range windows {
			if (!beforeIDs[window.ID] || len(beforeIDs) == 0) && windowMatches(window, query) {
				return window, nil
			}
			if windowMatches(window, query) {
				return window, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return Window{}, RuntimeError{Code: "timeout", Message: "window not found within timeout: " + query, Retryable: true}
}

func (r linuxRuntime) listWindows(ctx context.Context, required bool, requireInput bool) ([]Window, error) {
	if err := requireDisplay(); err != nil {
		return nil, err
	}
	if commandExists("wmctrl") {
		return listWindowsWmctrl(ctx, requireInput)
	}
	if commandExists("xdotool") {
		return listWindowsXdotool(ctx, requireInput)
	}
	if required {
		return nil, RuntimeError{Code: "missing_dependency", Message: "Linux window discovery requires wmctrl or xdotool"}
	}
	return nil, nil
}

func listWindowsWmctrl(ctx context.Context, requireInput bool) ([]Window, error) {
	active, _ := activeWindowID(ctx, requireInput)
	output, err := commandOutput(ctx, []string{"wmctrl", "-lx"}, nil)
	if err != nil {
		return nil, err
	}
	var windows []Window
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		title := strings.TrimSpace(strings.Join(fields[4:], " "))
		appName := deriveLinuxAppName(fields[3], title)
		windows = append(windows, Window{ID: fields[0], AppName: appName, Title: title, Focused: normalizeWindowID(fields[0]) == active})
	}
	return windows, nil
}

func listWindowsXdotool(ctx context.Context, requireInput bool) ([]Window, error) {
	active, _ := activeWindowID(ctx, requireInput)
	output, err := commandOutput(ctx, []string{"xdotool", "search", "--onlyvisible", "--name", "."}, nil)
	if err != nil {
		return nil, err
	}
	var windows []Window
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		id := strings.TrimSpace(line)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		title, err := commandOutput(ctx, []string{"xdotool", "getwindowname", id}, nil)
		if err != nil {
			continue
		}
		class := readWindowClass(ctx, id)
		windows = append(windows, Window{ID: id, AppName: deriveLinuxAppName(class, title), Title: strings.TrimSpace(title), Focused: normalizeWindowID(id) == active})
	}
	return windows, nil
}

func screenshotCommands(out string) [][]string {
	var commands [][]string
	if commandExists("gnome-screenshot") {
		commands = append(commands, []string{"gnome-screenshot", "-f", out})
	}
	if commandExists("grim") {
		commands = append(commands, []string{"grim", out})
	}
	if commandExists("scrot") {
		commands = append(commands, []string{"scrot", out})
	}
	if commandExists("import") {
		commands = append(commands, []string{"import", "-window", "root", out})
	}
	if commandExists("flameshot") {
		commands = append(commands, []string{"flameshot", "full", "-p", out})
	}
	return commands
}

func runInputCommand(ctx context.Context, command []string) error {
	if err := requireDisplay(); err != nil {
		return err
	}
	if !commandExists("xdotool") {
		return RuntimeError{Code: "missing_dependency", Message: "Linux input control requires xdotool"}
	}
	return runCommand(ctx, command, nil)
}

func runCommand(ctx context.Context, command []string, env []string) error {
	_, err := commandOutput(ctx, command, env)
	return err
}

func commandOutput(ctx context.Context, command []string, env []string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, linuxCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, command[0], command[1:]...)
	if env != nil {
		cmd.Env = env
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = strings.TrimSpace(stdout.String())
		}
		if details == "" {
			details = err.Error()
		}
		return "", RuntimeError{Code: "command_failed", Message: strings.Join(command, " ") + ": " + details, Retryable: true}
	}
	return stdout.String(), nil
}

func resolveLinuxLaunchCommand(name string) ([]string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return nil, RuntimeError{Code: "invalid_args", Message: "--name is required"}
	}
	if parsed := splitShellLike(normalized); len(parsed) > 0 && commandExists(parsed[0]) {
		return parsed, nil
	}
	lowered := strings.ToLower(normalized)
	var candidates [][]string
	if lowered == "calculator" || lowered == "calc" {
		if commandExists("gtk-launch") {
			candidates = append(candidates, []string{"gtk-launch", "org.gnome.Calculator"})
		}
		candidates = append(candidates, []string{"gnome-calculator"}, []string{"kcalc"}, []string{"mate-calc"}, []string{"galculator"}, []string{"xcalc"})
	} else {
		candidates = append(candidates, []string{normalized})
	}
	for _, command := range candidates {
		if commandExists(command[0]) {
			return command, nil
		}
	}
	return nil, RuntimeError{Code: "not_found", Message: fmt.Sprintf("could not resolve Linux launch command for %q", name)}
}

func requireDisplay() error {
	if strings.TrimSpace(os.Getenv("DISPLAY")) != "" || strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" {
		return nil
	}
	return RuntimeError{Code: "runtime_unavailable", Message: "Linux desktop runtime requires DISPLAY or WAYLAND_DISPLAY", Retryable: true}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func activeWindowID(ctx context.Context, required bool) (string, error) {
	if !commandExists("xdotool") {
		if required {
			return "", RuntimeError{Code: "missing_dependency", Message: "Linux input control requires xdotool"}
		}
		return "", nil
	}
	output, err := commandOutput(ctx, []string{"xdotool", "getactivewindow"}, nil)
	if err != nil {
		output, err = commandOutput(ctx, []string{"xdotool", "getwindowfocus"}, nil)
	}
	if err != nil {
		if required {
			return "", err
		}
		return "", nil
	}
	return normalizeWindowID(output), nil
}

func readWindowClass(ctx context.Context, id string) string {
	if !commandExists("xprop") {
		return ""
	}
	output, err := commandOutput(ctx, []string{"xprop", "-id", id, "WM_CLASS"}, nil)
	if err != nil {
		return ""
	}
	parts := strings.Split(output, "\"")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[len(parts)-2])
	}
	return strings.TrimSpace(output)
}

func deriveLinuxAppName(class string, title string) string {
	if class != "" {
		first := strings.Split(class, ".")[0]
		return strings.TrimSpace(strings.ReplaceAll(first, "-", " "))
	}
	return strings.TrimSpace(title)
}

func normalizeWindowID(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(normalized, "0x") {
		parsed, err := strconv.ParseInt(normalized[2:], 16, 64)
		if err == nil {
			return strconv.FormatInt(parsed, 10)
		}
	}
	return normalized
}

func normalizeLinuxShortcut(keys string) string {
	aliases := map[string]string{
		"control": "ctrl", "ctrl": "ctrl", "command": "super", "cmd": "super",
		"meta": "super", "option": "alt", "alt": "alt", "shift": "shift",
		"enter": "Return", "return": "Return", "esc": "Escape", "escape": "Escape",
		"del": "Delete", "delete": "Delete", "tab": "Tab", "space": "space",
		"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	}
	var parts []string
	for _, part := range strings.Split(strings.TrimSpace(keys), "+") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if alias, ok := aliases[strings.ToLower(token)]; ok {
			parts = append(parts, alias)
		} else {
			parts = append(parts, token)
		}
	}
	return strings.Join(parts, "+")
}

func windowMatches(window Window, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	return strings.Contains(strings.ToLower(window.Title), needle) || strings.Contains(strings.ToLower(window.AppName), needle)
}

func focusedWindowTitle(windows []Window) string {
	for _, window := range windows {
		if window.Focused {
			return window.Title
		}
	}
	return ""
}

func appendIfMissing(env []string, assignment string) []string {
	key := strings.SplitN(assignment, "=", 2)[0] + "="
	for _, item := range env {
		if strings.HasPrefix(item, key) {
			return env
		}
	}
	return append(env, assignment)
}

func splitShellLike(value string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, ch := range value {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				current.WriteRune(ch)
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields
}

func pngDimensions(path string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	header := make([]byte, 24)
	if _, err := file.Read(header); err != nil {
		return 0, 0
	}
	if !bytes.Equal(header[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return 0, 0
	}
	width := int(header[16])<<24 | int(header[17])<<16 | int(header[18])<<8 | int(header[19])
	height := int(header[20])<<24 | int(header[21])<<16 | int(header[22])<<8 | int(header[23])
	return width, height
}
