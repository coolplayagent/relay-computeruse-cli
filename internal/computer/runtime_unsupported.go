//go:build !linux && !windows

package computer

import (
	"context"
	"runtime"
	"time"
)

type unsupportedRuntime struct{}

func newOSRuntime() Runtime {
	return unsupportedRuntime{}
}

func (unsupportedRuntime) Name() string { return "unsupported" }

func (unsupportedRuntime) err() error {
	return RuntimeError{
		Code:      "unsupported_platform",
		Message:   "RelayComputerUse does not support " + runtime.GOOS + "; use --runtime fake for scripted validation",
		Retryable: false,
	}
}

func (r unsupportedRuntime) Screenshot(context.Context, string) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) Zoom(context.Context, string, int, int, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) ListWindows(context.Context) (Result, error) { return Result{}, r.err() }
func (r unsupportedRuntime) FocusWindow(context.Context, string) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) MouseMove(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) Click(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) RightClick(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) MiddleClick(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) DoubleClick(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) TripleClick(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) LeftMouseDown(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) LeftMouseUp(context.Context, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) Drag(context.Context, int, int, int, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) TypeText(context.Context, string) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) Hotkey(context.Context, string) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) HoldKey(context.Context, string, time.Duration) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) Scroll(context.Context, string, int) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) Wait(context.Context, time.Duration) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) LaunchApp(context.Context, string) (Result, error) {
	return Result{}, r.err()
}
func (r unsupportedRuntime) WaitWindow(context.Context, string, time.Duration) (Result, error) {
	return Result{}, r.err()
}
