//go:build windows

package computer

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	winCommandTimeout = 10 * time.Second
	swRestore         = 9
	srccopy           = 0x00CC0020
	biRGB             = 0
	dibRGBColors      = 0
	inputKeyboard     = 1
	keyeventfKeyUp    = 0x0002
	keyeventfUnicode  = 0x0004
	mouseLeftDown     = 0x0002
	mouseLeftUp       = 0x0004
	mouseWheel        = 0x0800
	wheelDelta        = 120
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	gdi32                          = syscall.NewLazyDLL("gdi32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procEnumWindows                = user32.NewProc("EnumWindows")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessID   = user32.NewProc("GetWindowThreadProcessId")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop           = user32.NewProc("BringWindowToTop")
	procSetCursorPos               = user32.NewProc("SetCursorPos")
	procMouseEvent                 = user32.NewProc("mouse_event")
	procSendInput                  = user32.NewProc("SendInput")
	procKeybdEvent                 = user32.NewProc("keybd_event")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procGetDC                      = user32.NewProc("GetDC")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC         = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC                   = gdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap     = gdi32.NewProc("CreateCompatibleBitmap")
	procDeleteObject               = gdi32.NewProc("DeleteObject")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procBitBlt                     = gdi32.NewProc("BitBlt")
	procGetDIBits                  = gdi32.NewProc("GetDIBits")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
)

type windowsRuntime struct{}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

type input struct {
	Type    uint32
	Padding uint32
	Data    [32]byte
}

type keyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

func newOSRuntime() Runtime {
	return windowsRuntime{}
}

func (windowsRuntime) Name() string {
	return "windows"
}

func (r windowsRuntime) Screenshot(_ context.Context, out string) (Result, error) {
	if strings.TrimSpace(out) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--out is required"}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil && filepath.Dir(out) != "." {
		return Result{}, RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	img, originX, originY, err := captureWindowsImage()
	if err != nil {
		return Result{}, err
	}
	file, err := os.Create(out)
	if err != nil {
		return Result{}, RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return Result{}, RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	return result("screenshot", "Captured Windows desktop screenshot.", Observation{
		Path:     out,
		MimeType: "image/png",
		Width:    img.Bounds().Dx(),
		Height:   img.Bounds().Dy(),
		OriginX:  originX,
		OriginY:  originY,
	}, map[string]interface{}{"virtual_screen_origin_x": originX, "virtual_screen_origin_y": originY}), nil
}

func (r windowsRuntime) ListWindows(_ context.Context) (Result, error) {
	windows, err := listWindowsWin32()
	if err != nil {
		return Result{}, err
	}
	return result("list-windows", "Listed Windows desktop windows.", Observation{
		Windows:       windows,
		FocusedWindow: focusedWindowTitle(windows),
	}, map[string]interface{}{"window_count": len(windows)}), nil
}

func (r windowsRuntime) FocusWindow(_ context.Context, title string) (Result, error) {
	window, err := findWindowsWindow(title)
	if err != nil {
		return Result{}, err
	}
	if err := activateWindow(window.ID); err != nil {
		return Result{}, err
	}
	time.Sleep(200 * time.Millisecond)
	return r.ListWindows(context.Background())
}

func (r windowsRuntime) Click(_ context.Context, x int, y int) (Result, error) {
	tx, ty := translateCoordinates(x, y)
	if err := setCursor(tx, ty); err != nil {
		return Result{}, err
	}
	mouseClick(1)
	time.Sleep(50 * time.Millisecond)
	return r.actionResult("click", "Clicked Windows desktop coordinates.", map[string]interface{}{"x": x, "y": y})
}

func (r windowsRuntime) DoubleClick(_ context.Context, x int, y int) (Result, error) {
	tx, ty := translateCoordinates(x, y)
	if err := setCursor(tx, ty); err != nil {
		return Result{}, err
	}
	mouseClick(2)
	time.Sleep(50 * time.Millisecond)
	return r.actionResult("double-click", "Double-clicked Windows desktop coordinates.", map[string]interface{}{"x": x, "y": y})
}

func (r windowsRuntime) Drag(_ context.Context, fromX int, fromY int, toX int, toY int) (Result, error) {
	startX, startY := translateCoordinates(fromX, fromY)
	endX, endY := translateCoordinates(toX, toY)
	if err := setCursor(startX, startY); err != nil {
		return Result{}, err
	}
	sendMouse(mouseLeftDown, 0)
	time.Sleep(50 * time.Millisecond)
	if err := setCursor(endX, endY); err != nil {
		return Result{}, err
	}
	time.Sleep(50 * time.Millisecond)
	sendMouse(mouseLeftUp, 0)
	return r.actionResult("drag", "Dragged Windows desktop coordinates.", map[string]interface{}{"from_x": fromX, "from_y": fromY, "to_x": toX, "to_y": toY})
}

func (r windowsRuntime) TypeText(_ context.Context, text string) (Result, error) {
	if strings.TrimSpace(text) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--text is required"}
	}
	if err := sendUnicodeText(text); err != nil {
		return Result{}, err
	}
	return r.actionResult("type-text", "Typed text into the Windows desktop.", map[string]interface{}{"text": text})
}

func (r windowsRuntime) Hotkey(_ context.Context, keys string) (Result, error) {
	modifiers, normals, err := normalizeWindowsShortcut(keys)
	if err != nil {
		return Result{}, err
	}
	for _, key := range modifiers {
		keybdEvent(key, 0)
	}
	for _, key := range normals {
		keybdEvent(key, 0)
	}
	for i := len(normals) - 1; i >= 0; i-- {
		keybdEvent(normals[i], keyeventfKeyUp)
	}
	for i := len(modifiers) - 1; i >= 0; i-- {
		keybdEvent(modifiers[i], keyeventfKeyUp)
	}
	return r.actionResult("hotkey", "Sent Windows desktop shortcut.", map[string]interface{}{"keys": keys})
}

func (r windowsRuntime) Scroll(_ context.Context, amount int) (Result, error) {
	if amount == 0 {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--amount must not be zero"}
	}
	sendMouse(mouseWheel, amount*wheelDelta)
	return r.actionResult("scroll", "Scrolled Windows desktop view.", map[string]interface{}{"amount": amount})
}

func (r windowsRuntime) LaunchApp(ctx context.Context, name string) (Result, error) {
	if strings.TrimSpace(name) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--name is required"}
	}
	before, _ := listWindowsWin32()
	candidates := windowsLaunchCandidates(name)
	var attempted []string
	var lastErr error
	for _, candidate := range candidates {
		attempted = append(attempted, strings.Join(candidate, " "))
		cmdCtx, cancel := context.WithTimeout(ctx, winCommandTimeout)
		cmd := exec.CommandContext(cmdCtx, candidate[0], candidate[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008}
		err := cmd.Start()
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		window, err := waitWindowsWindow(name, before, 10*time.Second)
		if err == nil {
			_ = activateWindow(window.ID)
			windows, _ := listWindowsWin32()
			return result("launch-app", "Launched Windows desktop app.", Observation{
				Windows:       windows,
				FocusedWindow: focusedWindowTitle(windows),
			}, map[string]interface{}{"window_title": window.Title, "launched_command": strings.Join(candidate, " ")}), nil
		}
		lastErr = err
	}
	message := "app window did not appear: " + name + "; attempted: " + strings.Join(attempted, ", ")
	if lastErr != nil {
		message += "; last error: " + lastErr.Error()
	}
	return Result{}, RuntimeError{Code: "timeout", Message: message, Retryable: true}
}

func (r windowsRuntime) WaitWindow(_ context.Context, title string, timeout time.Duration) (Result, error) {
	window, err := waitWindowsWindow(title, nil, timeout)
	if err != nil {
		return Result{}, err
	}
	windows, _ := listWindowsWin32()
	return result("wait-window", "Observed Windows desktop window.", Observation{
		Windows:       windows,
		FocusedWindow: focusedWindowTitle(windows),
	}, map[string]interface{}{"window_title": window.Title}), nil
}

func (r windowsRuntime) actionResult(action string, message string, data map[string]interface{}) (Result, error) {
	windows, _ := listWindowsWin32()
	return result(action, message, Observation{Windows: windows, FocusedWindow: focusedWindowTitle(windows)}, data), nil
}

func captureWindowsImage() (*image.RGBA, int, int, error) {
	left := int32(systemMetric(smXVirtualScreen))
	top := int32(systemMetric(smYVirtualScreen))
	width := int32(systemMetric(smCXVirtualScreen))
	height := int32(systemMetric(smCYVirtualScreen))
	if width <= 0 || height <= 0 {
		return nil, 0, 0, RuntimeError{Code: "runtime_unavailable", Message: "Windows desktop screenshot requires an active desktop session", Retryable: true}
	}
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, 0, 0, RuntimeError{Code: "runtime_unavailable", Message: "GetDC failed", Retryable: true}
	}
	defer procReleaseDC.Call(0, screenDC)
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, 0, 0, RuntimeError{Code: "runtime_unavailable", Message: "CreateCompatibleDC failed", Retryable: true}
	}
	defer procDeleteDC.Call(memDC)
	bitmap, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return nil, 0, 0, RuntimeError{Code: "runtime_unavailable", Message: "CreateCompatibleBitmap failed", Retryable: true}
	}
	defer procDeleteObject.Call(bitmap)
	oldObject, _, _ := procSelectObject.Call(memDC, bitmap)
	defer procSelectObject.Call(memDC, oldObject)
	ok, _, _ := procBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), screenDC, uintptr(left), uintptr(top), srccopy)
	if ok == 0 {
		return nil, 0, 0, RuntimeError{Code: "runtime_unavailable", Message: "BitBlt failed", Retryable: true}
	}
	pixels := make([]byte, int(width*height*4))
	info := bitmapInfo{}
	info.Header.Size = uint32(unsafe.Sizeof(info.Header))
	info.Header.Width = width
	info.Header.Height = -height
	info.Header.Planes = 1
	info.Header.BitCount = 32
	info.Header.Compression = biRGB
	lines, _, _ := procGetDIBits.Call(memDC, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), dibRGBColors)
	if lines == 0 {
		return nil, 0, 0, RuntimeError{Code: "runtime_unavailable", Message: "GetDIBits failed", Retryable: true}
	}
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			index := (y*int(width) + x) * 4
			img.SetRGBA(x, y, color.RGBA{R: pixels[index+2], G: pixels[index+1], B: pixels[index], A: 255})
		}
	}
	return img, int(left), int(top), nil
}

func listWindowsWin32() ([]Window, error) {
	foreground, _, _ := procGetForegroundWindow.Call()
	var windows []Window
	callback := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		length, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if length == 0 {
			return 1
		}
		buffer := make([]uint16, length+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		title := syscall.UTF16ToString(buffer)
		if strings.TrimSpace(title) == "" {
			return 1
		}
		app := processNameForWindow(hwnd)
		if app == "" {
			app = title
		}
		windows = append(windows, Window{ID: fmt.Sprintf("0x%x", hwnd), AppName: app, Title: title, Focused: hwnd == foreground})
		return 1
	})
	ok, _, err := procEnumWindows.Call(callback, 0)
	if ok == 0 {
		return nil, RuntimeError{Code: "runtime_unavailable", Message: "EnumWindows failed: " + err.Error(), Retryable: true}
	}
	return windows, nil
}

func processNameForWindow(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	handle, _, _ := procOpenProcess.Call(0x1000, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)
	buffer := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buffer))
	ok, _, _ := procQueryFullProcessImageNameW.Call(handle, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return ""
	}
	full := syscall.UTF16ToString(buffer[:size])
	base := filepath.Base(full)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func findWindowsWindow(query string) (Window, error) {
	if strings.TrimSpace(query) == "" {
		return Window{}, RuntimeError{Code: "invalid_args", Message: "--title is required"}
	}
	windows, err := listWindowsWin32()
	if err != nil {
		return Window{}, err
	}
	queries := expandWindowsQueries(query)
	for _, window := range windows {
		if windowMatchesAny(window, queries) {
			return window, nil
		}
	}
	return Window{}, RuntimeError{Code: "not_found", Message: "window not found: " + query, Retryable: true}
}

func waitWindowsWindow(query string, before []Window, timeout time.Duration) (Window, error) {
	if strings.TrimSpace(query) == "" {
		return Window{}, RuntimeError{Code: "invalid_args", Message: "--title is required"}
	}
	beforeIDs := map[string]bool{}
	for _, window := range before {
		beforeIDs[window.ID] = true
	}
	queries := expandWindowsQueries(query)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		windows, err := listWindowsWin32()
		if err != nil {
			return Window{}, err
		}
		for _, window := range windows {
			if (!beforeIDs[window.ID] || len(beforeIDs) == 0) && windowMatchesAny(window, queries) {
				return window, nil
			}
			if windowMatchesAny(window, queries) {
				return window, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return Window{}, RuntimeError{Code: "timeout", Message: "window not found within timeout: " + query, Retryable: true}
}

func activateWindow(id string) error {
	hwnd, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(id)), "0x"), 16, 64)
	if err != nil {
		return RuntimeError{Code: "invalid_args", Message: "invalid window id: " + id}
	}
	procShowWindow.Call(uintptr(hwnd), swRestore)
	ok, _, _ := procSetForegroundWindow.Call(uintptr(hwnd))
	if ok == 0 {
		procBringWindowToTop.Call(uintptr(hwnd))
	}
	return nil
}

func translateCoordinates(x int, y int) (int, int) {
	return x + systemMetric(smXVirtualScreen), y + systemMetric(smYVirtualScreen)
}

func setCursor(x int, y int) error {
	ok, _, _ := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if ok == 0 {
		return RuntimeError{Code: "runtime_unavailable", Message: "SetCursorPos failed", Retryable: true}
	}
	return nil
}

func mouseClick(repeat int) {
	for i := 0; i < repeat; i++ {
		sendMouse(mouseLeftDown, 0)
		sendMouse(mouseLeftUp, 0)
		if i+1 < repeat {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func sendMouse(flags int, data int) {
	procMouseEvent.Call(uintptr(flags), 0, 0, uintptr(data), 0)
}

func sendUnicodeText(text string) error {
	var inputs []input
	for _, code := range utf16.Encode([]rune(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))) {
		if code == '\n' {
			keybdEvent(0x0D, 0)
			keybdEvent(0x0D, keyeventfKeyUp)
			continue
		}
		down := keyboardInput{WScan: code, DwFlags: keyeventfUnicode}
		up := keyboardInput{WScan: code, DwFlags: keyeventfUnicode | keyeventfKeyUp}
		inputs = append(inputs, keyboardInputEvent(down), keyboardInputEvent(up))
	}
	if len(inputs) == 0 {
		return nil
	}
	sent, _, _ := procSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(input{}))
	if sent != uintptr(len(inputs)) {
		return RuntimeError{Code: "runtime_unavailable", Message: "SendInput failed", Retryable: true}
	}
	return nil
}

func keyboardInputEvent(key keyboardInput) input {
	event := input{Type: inputKeyboard}
	*(*keyboardInput)(unsafe.Pointer(&event.Data[0])) = key
	return event
}

func keybdEvent(vk uint16, flags uint32) {
	procKeybdEvent.Call(uintptr(vk), 0, uintptr(flags), 0)
}

func normalizeWindowsShortcut(keys string) ([]uint16, []uint16, error) {
	if strings.TrimSpace(keys) == "" {
		return nil, nil, RuntimeError{Code: "invalid_args", Message: "--keys is required"}
	}
	modifiersByName := map[string]uint16{"ctrl": 0x11, "control": 0x11, "alt": 0x12, "shift": 0x10, "win": 0x5B, "windows": 0x5B, "cmd": 0x5B, "command": 0x5B, "meta": 0x5B}
	keysByName := map[string]uint16{"enter": 0x0D, "return": 0x0D, "escape": 0x1B, "esc": 0x1B, "tab": 0x09, "space": 0x20, "left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28, "delete": 0x2E, "del": 0x2E, "home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22, "backspace": 0x08}
	var modifiers []uint16
	var normals []uint16
	for _, part := range strings.Split(keys, "+") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		lowered := strings.ToLower(token)
		if key, ok := modifiersByName[lowered]; ok {
			modifiers = append(modifiers, key)
			continue
		}
		if key, ok := keysByName[lowered]; ok {
			normals = append(normals, key)
			continue
		}
		if len(token) == 1 {
			ch := strings.ToUpper(token)[0]
			if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
				normals = append(normals, uint16(ch))
				continue
			}
		}
		if strings.HasPrefix(lowered, "f") {
			number, err := strconv.Atoi(lowered[1:])
			if err == nil && number >= 1 && number <= 24 {
				normals = append(normals, uint16(0x6F+number))
				continue
			}
		}
		return nil, nil, RuntimeError{Code: "invalid_args", Message: "unsupported shortcut key: " + token}
	}
	if len(normals) == 0 {
		return nil, nil, RuntimeError{Code: "invalid_args", Message: "--keys must include a non-modifier key"}
	}
	return modifiers, normals, nil
}

func windowsLaunchCandidates(name string) [][]string {
	normalized := strings.TrimSpace(name)
	lowered := strings.ToLower(normalized)
	known := map[string][]string{
		"notepad": {"notepad.exe"}, "notepad.exe": {"notepad.exe"},
		"calc": {"calc.exe"}, "calc.exe": {"calc.exe"}, "calculator": {"calc.exe"},
	}
	if command, ok := known[lowered]; ok {
		candidates := [][]string{command}
		if lowered == "calc" || lowered == "calc.exe" || lowered == "calculator" {
			candidates = append(candidates, []string{"explorer.exe", `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`})
		}
		return candidates
	}
	fields := splitWindowsCommand(normalized)
	if len(fields) > 0 {
		return [][]string{fields}
	}
	return [][]string{{"cmd", "/c", "start", "", normalized}}
}

func splitWindowsCommand(value string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	for _, ch := range value {
		switch ch {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				current.WriteRune(ch)
			} else if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields
}

func expandWindowsQueries(query string) []string {
	normalized := strings.TrimSpace(query)
	aliases := map[string][]string{
		"calc": {"Calculator", "Microsoft.WindowsCalculator"}, "calc.exe": {"Calculator", "Microsoft.WindowsCalculator"}, "calculator": {"Calculator", "Microsoft.WindowsCalculator"},
		"notepad": {"Notepad"}, "notepad.exe": {"Notepad"},
	}
	queries := []string{normalized}
	if extra, ok := aliases[strings.ToLower(normalized)]; ok {
		queries = append(queries, extra...)
	}
	return queries
}

func windowMatchesAny(window Window, queries []string) bool {
	for _, query := range queries {
		needle := strings.ToLower(strings.TrimSpace(query))
		if needle == "" {
			continue
		}
		if strings.Contains(strings.ToLower(window.Title), needle) || strings.Contains(strings.ToLower(window.AppName), needle) {
			return true
		}
	}
	return false
}

func focusedWindowTitle(windows []Window) string {
	for _, window := range windows {
		if window.Focused {
			return window.Title
		}
	}
	return ""
}

func systemMetric(index int) int {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(int32(value))
}
