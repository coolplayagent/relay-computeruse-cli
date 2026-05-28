package computer

import (
	"context"
	"time"
)

type Risk string

const (
	RiskSafe        Risk = "safe"
	RiskGuarded     Risk = "guarded"
	RiskDestructive Risk = "destructive"
)

type Window struct {
	ID      string `json:"id"`
	AppName string `json:"app_name"`
	Title   string `json:"title"`
	Focused bool   `json:"focused"`
}

type Observation struct {
	Path          string   `json:"path,omitempty"`
	MimeType      string   `json:"mime_type,omitempty"`
	Width         int      `json:"width,omitempty"`
	Height        int      `json:"height,omitempty"`
	OriginX       int      `json:"origin_x,omitempty"`
	OriginY       int      `json:"origin_y,omitempty"`
	Windows       []Window `json:"windows,omitempty"`
	FocusedWindow string   `json:"focused_window,omitempty"`
}

type Result struct {
	Action      string                 `json:"action"`
	Message     string                 `json:"message"`
	Observation Observation            `json:"observation,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

type Runtime interface {
	Name() string
	Screenshot(ctx context.Context, out string) (Result, error)
	Zoom(ctx context.Context, out string, x1 int, y1 int, x2 int, y2 int) (Result, error)
	ListWindows(ctx context.Context) (Result, error)
	FocusWindow(ctx context.Context, title string) (Result, error)
	MouseMove(ctx context.Context, x int, y int) (Result, error)
	Click(ctx context.Context, x int, y int) (Result, error)
	RightClick(ctx context.Context, x int, y int) (Result, error)
	MiddleClick(ctx context.Context, x int, y int) (Result, error)
	DoubleClick(ctx context.Context, x int, y int) (Result, error)
	TripleClick(ctx context.Context, x int, y int) (Result, error)
	LeftMouseDown(ctx context.Context, x int, y int) (Result, error)
	LeftMouseUp(ctx context.Context, x int, y int) (Result, error)
	Drag(ctx context.Context, fromX int, fromY int, toX int, toY int) (Result, error)
	TypeText(ctx context.Context, text string) (Result, error)
	Hotkey(ctx context.Context, keys string) (Result, error)
	HoldKey(ctx context.Context, keys string, duration time.Duration) (Result, error)
	Scroll(ctx context.Context, direction string, amount int) (Result, error)
	Wait(ctx context.Context, duration time.Duration) (Result, error)
	LaunchApp(ctx context.Context, name string) (Result, error)
	WaitWindow(ctx context.Context, title string, timeout time.Duration) (Result, error)
}

type RuntimeError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e RuntimeError) Error() string {
	return e.Message
}

func NewRuntime(mode string) Runtime {
	if mode == "fake" {
		return NewFakeRuntime()
	}
	return newOSRuntime()
}

func result(action string, message string, observation Observation, data map[string]interface{}) Result {
	if data == nil {
		data = map[string]interface{}{}
	}
	return Result{
		Action:      action,
		Message:     message,
		Observation: observation,
		Data:        data,
	}
}
