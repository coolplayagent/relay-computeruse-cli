package protocol

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/relaycomputeruse/relaycomputeruse/internal/computer"
)

type ActionRequest struct {
	Action       string `json:"action"`
	X            int    `json:"x,omitempty"`
	Y            int    `json:"y,omitempty"`
	ToX          int    `json:"to_x,omitempty"`
	ToY          int    `json:"to_y,omitempty"`
	Text         string `json:"text,omitempty"`
	Keys         string `json:"keys,omitempty"`
	Direction    string `json:"direction,omitempty"`
	Amount       int    `json:"amount,omitempty"`
	Duration     string `json:"duration,omitempty"`
	Out          string `json:"out,omitempty"`
	Title        string `json:"title,omitempty"`
	Name         string `json:"name,omitempty"`
	ObserveAfter bool   `json:"observe_after,omitempty"`
	ObserveOut   string `json:"observe_out,omitempty"`
}

type ActionResponse struct {
	OK          bool                  `json:"ok"`
	Action      string                `json:"action,omitempty"`
	Message     string                `json:"message,omitempty"`
	Observation *computer.Observation `json:"observation,omitempty"`
	Data        map[string]any        `json:"data,omitempty"`
	Error       *ErrorPayload         `json:"error,omitempty"`
}

type ErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Options struct {
	ObserveOutDefault string
}

func RiskForAction(action string) computer.Risk {
	switch normalizeAction(action) {
	case "screenshot", "zoom", "list_windows", "wait", "wait_window":
		return computer.RiskSafe
	case "left_click_drag", "launch_app":
		return computer.RiskDestructive
	default:
		return computer.RiskGuarded
	}
}

func Execute(ctx context.Context, runtime computer.Runtime, request ActionRequest, options Options) ActionResponse {
	action := normalizeAction(request.Action)
	result, err := execute(ctx, runtime, action, request)
	if err != nil {
		return errorResponse(err)
	}
	if result.Data == nil {
		result.Data = map[string]any{}
	}
	result.Data["runtime"] = runtime.Name()
	result.Data["protocol_action"] = action

	if request.ObserveAfter && action != "screenshot" && action != "zoom" {
		out := strings.TrimSpace(request.ObserveOut)
		if out == "" {
			out = strings.TrimSpace(options.ObserveOutDefault)
		}
		if out == "" {
			return ActionResponse{
				OK:    false,
				Error: &ErrorPayload{Code: "invalid_args", Message: "observe_after requires observe_out or --observe-out"},
			}
		}
		observed, err := runtime.Screenshot(ctx, out)
		if err != nil {
			return errorResponse(err)
		}
		result.Observation = observed.Observation
		result.Data["observed_after"] = true
		result.Data["observed_action"] = result.Action
	}

	return ActionResponse{
		OK:          true,
		Action:      result.Action,
		Message:     result.Message,
		Observation: &result.Observation,
		Data:        result.Data,
	}
}

func execute(ctx context.Context, runtime computer.Runtime, action string, request ActionRequest) (computer.Result, error) {
	switch action {
	case "screenshot":
		return runtime.Screenshot(ctx, request.Out)
	case "zoom":
		return runtime.Zoom(ctx, request.Out, request.X, request.Y, request.ToX, request.ToY)
	case "list_windows":
		return runtime.ListWindows(ctx)
	case "focus_window":
		return runtime.FocusWindow(ctx, request.Title)
	case "mouse_move":
		return runtime.MouseMove(ctx, request.X, request.Y)
	case "left_click":
		return runtime.Click(ctx, request.X, request.Y)
	case "right_click":
		return runtime.RightClick(ctx, request.X, request.Y)
	case "middle_click":
		return runtime.MiddleClick(ctx, request.X, request.Y)
	case "double_click":
		return runtime.DoubleClick(ctx, request.X, request.Y)
	case "triple_click":
		return runtime.TripleClick(ctx, request.X, request.Y)
	case "left_mouse_down":
		return runtime.LeftMouseDown(ctx, request.X, request.Y)
	case "left_mouse_up":
		return runtime.LeftMouseUp(ctx, request.X, request.Y)
	case "left_click_drag":
		return runtime.Drag(ctx, request.X, request.Y, request.ToX, request.ToY)
	case "type":
		return runtime.TypeText(ctx, request.Text)
	case "key":
		return runtime.Hotkey(ctx, request.Keys)
	case "hold_key":
		duration, err := parseDuration(request.Duration, "1s")
		if err != nil {
			return computer.Result{}, err
		}
		return runtime.HoldKey(ctx, request.Keys, duration)
	case "scroll":
		return runtime.Scroll(ctx, request.Direction, request.Amount)
	case "wait":
		duration, err := parseDuration(request.Duration, "1s")
		if err != nil {
			return computer.Result{}, err
		}
		return runtime.Wait(ctx, duration)
	case "launch_app":
		return runtime.LaunchApp(ctx, request.Name)
	case "wait_window":
		duration, err := parseDuration(request.Duration, "10s")
		if err != nil {
			return computer.Result{}, err
		}
		return runtime.WaitWindow(ctx, request.Title, duration)
	default:
		return computer.Result{}, computer.RuntimeError{Code: "unknown_action", Message: "unknown action: " + action}
	}
}

func normalizeAction(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	action = strings.ReplaceAll(action, "-", "_")
	switch action {
	case "click", "left_click":
		return "left_click"
	case "drag":
		return "left_click_drag"
	case "type_text":
		return "type"
	case "hotkey":
		return "key"
	case "list_windows":
		return "list_windows"
	case "focus_window":
		return "focus_window"
	case "launch_app":
		return "launch_app"
	case "wait_window":
		return "wait_window"
	default:
		return action
	}
}

func parseDuration(value string, defaultValue string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultValue
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		duration := time.Duration(seconds) * time.Second
		if duration <= 0 {
			return 0, computer.RuntimeError{Code: "invalid_args", Message: "duration must be positive"}
		}
		return duration, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, computer.RuntimeError{Code: "invalid_args", Message: fmt.Sprintf("invalid duration %q: %v", value, err)}
	}
	if duration <= 0 {
		return 0, computer.RuntimeError{Code: "invalid_args", Message: "duration must be positive"}
	}
	return duration, nil
}

func errorResponse(err error) ActionResponse {
	var runtimeErr computer.RuntimeError
	if errors.As(err, &runtimeErr) {
		return ActionResponse{OK: false, Error: &ErrorPayload{Code: runtimeErr.Code, Message: runtimeErr.Message, Retryable: runtimeErr.Retryable}}
	}
	return ActionResponse{OK: false, Error: &ErrorPayload{Code: "runtime_error", Message: err.Error(), Retryable: true}}
}
