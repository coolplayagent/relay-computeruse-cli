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
	ListWindows(ctx context.Context) (Result, error)
	FocusWindow(ctx context.Context, title string) (Result, error)
	Click(ctx context.Context, x int, y int) (Result, error)
	DoubleClick(ctx context.Context, x int, y int) (Result, error)
	Drag(ctx context.Context, fromX int, fromY int, toX int, toY int) (Result, error)
	TypeText(ctx context.Context, text string) (Result, error)
	Hotkey(ctx context.Context, keys string) (Result, error)
	Scroll(ctx context.Context, amount int) (Result, error)
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
