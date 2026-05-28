package computer

import (
	"context"
	"image"
	"image/color"
	"strings"
	"time"
)

type FakeRuntime struct {
	windows []Window
}

func NewFakeRuntime() *FakeRuntime {
	return &FakeRuntime{
		windows: []Window{
			{ID: "window-relay", AppName: "RelayComputerUse", Title: "RelayComputerUse Demo", Focused: true},
			{ID: "window-browser", AppName: "Browser", Title: "Chrome DevTools", Focused: false},
		},
	}
}

func (r *FakeRuntime) Name() string {
	return "fake"
}

func (r *FakeRuntime) Screenshot(_ context.Context, out string) (Result, error) {
	if err := ensureOutputPath(out); err != nil {
		return Result{}, err
	}
	img := fakeScreenshotImage()
	if err := writePNG(out, img); err != nil {
		return Result{}, err
	}
	return result(
		"screenshot",
		"Captured fake screenshot.",
		Observation{Path: out, MimeType: "image/png", Width: img.Bounds().Dx(), Height: img.Bounds().Dy(), Windows: r.windows, FocusedWindow: r.focusedTitle()},
		r.baseData(),
	), nil
}

func (r *FakeRuntime) Zoom(_ context.Context, out string, x1 int, y1 int, x2 int, y2 int) (Result, error) {
	if err := ensureOutputPath(out); err != nil {
		return Result{}, err
	}
	cropped, err := cropImage(fakeScreenshotImage(), x1, y1, x2, y2)
	if err != nil {
		return Result{}, err
	}
	if err := writePNG(out, cropped); err != nil {
		return Result{}, err
	}
	return result("zoom", "Captured fake zoom region.", Observation{
		Path: out, MimeType: "image/png", Width: cropped.Bounds().Dx(), Height: cropped.Bounds().Dy(), Windows: r.windows, FocusedWindow: r.focusedTitle(),
	}, r.baseData()), nil
}

func fakeScreenshotImage() *image.RGBA {
	width := 320
	height := 180
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.RGBA{R: 230, G: 235, B: 238, A: 255}
			if y < 24 {
				c = color.RGBA{R: 38, G: 50, B: 56, A: 255}
			} else if x >= 36 && x <= 300 && y >= 42 && y <= 156 {
				c = color.RGBA{R: 246, G: 248, B: 250, A: 255}
				if y < 66 {
					c = color.RGBA{R: 208, G: 216, B: 224, A: 255}
				}
				if x >= 52 && x <= 140 && y >= 82 && y <= 112 {
					c = color.RGBA{R: 120, G: 144, B: 156, A: 255}
				}
				if x >= 166 && x <= 270 && y >= 84 && y <= 94 {
					c = color.RGBA{R: 76, G: 175, B: 120, A: 255}
				}
				if x >= 166 && x <= 246 && y >= 110 && y <= 120 {
					c = color.RGBA{R: 93, G: 120, B: 140, A: 255}
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func (r *FakeRuntime) ListWindows(_ context.Context) (Result, error) {
	return result(
		"list-windows",
		"Listed fake windows.",
		Observation{Windows: r.windows, FocusedWindow: r.focusedTitle()},
		r.baseData(),
	), nil
}

func (r *FakeRuntime) FocusWindow(_ context.Context, title string) (Result, error) {
	if strings.TrimSpace(title) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--title is required"}
	}
	for i := range r.windows {
		r.windows[i].Focused = strings.Contains(strings.ToLower(r.windows[i].Title), strings.ToLower(title))
	}
	return r.ListWindows(context.Background())
}

func (r *FakeRuntime) MouseMove(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("mouse-move", "Moved fake pointer.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) Click(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("click", "Clicked fake coordinates.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) RightClick(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("right-click", "Right-clicked fake coordinates.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) MiddleClick(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("middle-click", "Middle-clicked fake coordinates.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) DoubleClick(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("double-click", "Double-clicked fake coordinates.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) TripleClick(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("triple-click", "Triple-clicked fake coordinates.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) LeftMouseDown(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("left-mouse-down", "Pressed fake left mouse button.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) LeftMouseUp(_ context.Context, x int, y int) (Result, error) {
	return r.pointerResult("left-mouse-up", "Released fake left mouse button.", x, y, 0, 0, false), nil
}

func (r *FakeRuntime) Drag(_ context.Context, fromX int, fromY int, toX int, toY int) (Result, error) {
	return r.pointerResult("drag", "Dragged fake coordinates.", fromX, fromY, toX, toY, true), nil
}

func (r *FakeRuntime) TypeText(_ context.Context, text string) (Result, error) {
	if strings.TrimSpace(text) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--text is required"}
	}
	data := r.baseData()
	data["text"] = text
	return result("type-text", "Typed fake text.", Observation{Windows: r.windows, FocusedWindow: r.focusedTitle()}, data), nil
}

func (r *FakeRuntime) Hotkey(_ context.Context, keys string) (Result, error) {
	if strings.TrimSpace(keys) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--keys is required"}
	}
	data := r.baseData()
	data["keys"] = keys
	return result("hotkey", "Sent fake hotkey.", Observation{Windows: r.windows, FocusedWindow: r.focusedTitle()}, data), nil
}

func (r *FakeRuntime) HoldKey(_ context.Context, keys string, duration time.Duration) (Result, error) {
	if strings.TrimSpace(keys) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--keys is required"}
	}
	if duration <= 0 {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--duration must be positive"}
	}
	data := r.baseData()
	data["keys"] = keys
	data["duration_ms"] = duration.Milliseconds()
	return result("hold-key", "Held fake key.", Observation{Windows: r.windows, FocusedWindow: r.focusedTitle()}, data), nil
}

func (r *FakeRuntime) Scroll(_ context.Context, direction string, amount int) (Result, error) {
	if amount == 0 {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--amount must not be zero"}
	}
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction == "" {
		direction = "down"
	}
	if direction != "up" && direction != "down" && direction != "left" && direction != "right" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--direction must be up, down, left, or right"}
	}
	data := r.baseData()
	data["amount"] = amount
	data["direction"] = direction
	return result("scroll", "Scrolled fake view.", Observation{Windows: r.windows, FocusedWindow: r.focusedTitle()}, data), nil
}

func (r *FakeRuntime) Wait(_ context.Context, duration time.Duration) (Result, error) {
	if duration <= 0 {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--duration must be positive"}
	}
	data := r.baseData()
	data["duration_ms"] = duration.Milliseconds()
	return result("wait", "Waited fake duration.", Observation{Windows: r.windows, FocusedWindow: r.focusedTitle()}, data), nil
}

func (r *FakeRuntime) LaunchApp(_ context.Context, name string) (Result, error) {
	if strings.TrimSpace(name) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--name is required"}
	}
	window := Window{ID: "window-" + strings.ToLower(strings.ReplaceAll(name, " ", "-")), AppName: name, Title: name + " Window", Focused: true}
	windows := make([]Window, 0, len(r.windows)+1)
	for _, item := range r.windows {
		item.Focused = false
		windows = append(windows, item)
	}
	windows = append(windows, window)
	r.windows = windows
	data := r.baseData()
	data["launched_command"] = name
	return result("launch-app", "Launched fake app.", Observation{Windows: r.windows, FocusedWindow: window.Title}, data), nil
}

func (r *FakeRuntime) WaitWindow(_ context.Context, title string, _ time.Duration) (Result, error) {
	if strings.TrimSpace(title) == "" {
		return Result{}, RuntimeError{Code: "invalid_args", Message: "--title is required"}
	}
	window := Window{ID: "window-wait", AppName: title, Title: title, Focused: true}
	data := r.baseData()
	return result("wait-window", "Observed fake window.", Observation{Windows: []Window{window}, FocusedWindow: title}, data), nil
}

func (r *FakeRuntime) pointerResult(action string, message string, x int, y int, toX int, toY int, hasEnd bool) Result {
	data := r.baseData()
	data["x"] = x
	data["y"] = y
	if hasEnd {
		data["to_x"] = toX
		data["to_y"] = toY
	}
	return result(action, message, Observation{Windows: r.windows, FocusedWindow: r.focusedTitle()}, data)
}

func (r *FakeRuntime) focusedTitle() string {
	for _, window := range r.windows {
		if window.Focused {
			return window.Title
		}
	}
	return ""
}

func (r *FakeRuntime) baseData() map[string]interface{} {
	return map[string]interface{}{"runtime": r.Name()}
}
