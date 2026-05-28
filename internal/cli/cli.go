package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/relaycomputeruse/relaycomputeruse/internal/computer"
)

type response struct {
	OK          bool                   `json:"ok"`
	Action      string                 `json:"action,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Observation *computer.Observation  `json:"observation,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       *errorPayload          `json:"error,omitempty"`
}

type errorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type options struct {
	Runtime   string
	AllowRisk computer.Risk
	Pretty    bool
}

type commandSpec struct {
	Name string
	Risk computer.Risk
	Run  func(context.Context, computer.Runtime, []string) (computer.Result, error)
}

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, commandName, commandArgs, err := parseGlobal(args)
	if err != nil {
		writeError(stdout, opts.Pretty, "invalid_args", err.Error(), false)
		return 2
	}
	if commandName == "" || commandName == "help" || commandName == "--help" || commandName == "-h" {
		_, _ = fmt.Fprint(stderr, usage())
		return 0
	}

	spec, ok := commands()[commandName]
	if !ok {
		writeError(stdout, opts.Pretty, "unknown_command", "unknown command: "+commandName, false)
		return 2
	}
	if !riskAllowed(opts.AllowRisk, spec.Risk) {
		writeError(
			stdout,
			opts.Pretty,
			"policy_denied",
			fmt.Sprintf("action %s requires risk %s; current --allow-risk is %s", commandName, spec.Risk, opts.AllowRisk),
			false,
		)
		return 3
	}

	runtime := computer.NewRuntime(opts.Runtime)
	result, err := spec.Run(ctx, runtime, commandArgs)
	if err != nil {
		writeRuntimeError(stdout, opts.Pretty, err)
		return 1
	}
	result.Data["risk"] = string(spec.Risk)
	result.Data["runtime"] = runtime.Name()
	writeJSON(stdout, opts.Pretty, response{
		OK:          true,
		Action:      result.Action,
		Message:     result.Message,
		Observation: &result.Observation,
		Data:        result.Data,
	})
	return 0
}

func parseGlobal(args []string) (options, string, []string, error) {
	opts := options{Runtime: "auto", AllowRisk: computer.RiskGuarded}
	fs := flag.NewFlagSet("relay-computer-use", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.Runtime, "runtime", opts.Runtime, "runtime: auto or fake")
	allowRisk := fs.String("allow-risk", string(opts.AllowRisk), "max risk: safe, guarded, destructive")
	fs.BoolVar(&opts.Pretty, "pretty", false, "pretty-print JSON")

	index := 0
	for index < len(args) {
		item := args[index]
		if item == "--" {
			index++
			break
		}
		if !strings.HasPrefix(item, "-") {
			break
		}
		if item == "-h" || item == "--help" {
			return opts, "help", nil, nil
		}
		if strings.Contains(item, "=") {
			if err := fs.Parse([]string{item}); err != nil {
				return opts, "", nil, err
			}
			index++
			continue
		}
		if item == "--pretty" {
			if err := fs.Parse([]string{item}); err != nil {
				return opts, "", nil, err
			}
			index++
			continue
		}
		if index+1 >= len(args) {
			return opts, "", nil, fmt.Errorf("missing value for %s", item)
		}
		if err := fs.Parse([]string{item, args[index+1]}); err != nil {
			return opts, "", nil, err
		}
		index += 2
	}
	if opts.Runtime != "auto" && opts.Runtime != "fake" {
		return opts, "", nil, fmt.Errorf("--runtime must be auto or fake")
	}
	risk, err := parseRisk(*allowRisk)
	if err != nil {
		return opts, "", nil, err
	}
	opts.AllowRisk = risk
	if index >= len(args) {
		return opts, "", nil, nil
	}
	return opts, args[index], args[index+1:], nil
}

func commands() map[string]commandSpec {
	return map[string]commandSpec{
		"screenshot":      {Name: "screenshot", Risk: computer.RiskSafe, Run: runScreenshot},
		"zoom":            {Name: "zoom", Risk: computer.RiskSafe, Run: runZoom},
		"list-windows":    {Name: "list-windows", Risk: computer.RiskSafe, Run: runListWindows},
		"focus-window":    {Name: "focus-window", Risk: computer.RiskGuarded, Run: runFocusWindow},
		"mouse-move":      {Name: "mouse-move", Risk: computer.RiskGuarded, Run: runMouseMove},
		"mouse_move":      {Name: "mouse-move", Risk: computer.RiskGuarded, Run: runMouseMove},
		"click":           {Name: "click", Risk: computer.RiskGuarded, Run: runClick},
		"left-click":      {Name: "click", Risk: computer.RiskGuarded, Run: runClick},
		"left_click":      {Name: "click", Risk: computer.RiskGuarded, Run: runClick},
		"right-click":     {Name: "right-click", Risk: computer.RiskGuarded, Run: runRightClick},
		"right_click":     {Name: "right-click", Risk: computer.RiskGuarded, Run: runRightClick},
		"middle-click":    {Name: "middle-click", Risk: computer.RiskGuarded, Run: runMiddleClick},
		"middle_click":    {Name: "middle-click", Risk: computer.RiskGuarded, Run: runMiddleClick},
		"double-click":    {Name: "double-click", Risk: computer.RiskGuarded, Run: runDoubleClick},
		"double_click":    {Name: "double-click", Risk: computer.RiskGuarded, Run: runDoubleClick},
		"triple-click":    {Name: "triple-click", Risk: computer.RiskGuarded, Run: runTripleClick},
		"triple_click":    {Name: "triple-click", Risk: computer.RiskGuarded, Run: runTripleClick},
		"left-mouse-down": {Name: "left-mouse-down", Risk: computer.RiskGuarded, Run: runLeftMouseDown},
		"left_mouse_down": {Name: "left-mouse-down", Risk: computer.RiskGuarded, Run: runLeftMouseDown},
		"left-mouse-up":   {Name: "left-mouse-up", Risk: computer.RiskGuarded, Run: runLeftMouseUp},
		"left_mouse_up":   {Name: "left-mouse-up", Risk: computer.RiskGuarded, Run: runLeftMouseUp},
		"drag":            {Name: "drag", Risk: computer.RiskDestructive, Run: runDrag},
		"left-click-drag": {Name: "drag", Risk: computer.RiskDestructive, Run: runDrag},
		"left_click_drag": {Name: "drag", Risk: computer.RiskDestructive, Run: runDrag},
		"type-text":       {Name: "type-text", Risk: computer.RiskGuarded, Run: runTypeText},
		"type":            {Name: "type-text", Risk: computer.RiskGuarded, Run: runTypeText},
		"hotkey":          {Name: "hotkey", Risk: computer.RiskGuarded, Run: runHotkey},
		"key":             {Name: "hotkey", Risk: computer.RiskGuarded, Run: runHotkey},
		"hold-key":        {Name: "hold-key", Risk: computer.RiskGuarded, Run: runHoldKey},
		"hold_key":        {Name: "hold-key", Risk: computer.RiskGuarded, Run: runHoldKey},
		"scroll":          {Name: "scroll", Risk: computer.RiskGuarded, Run: runScroll},
		"wait":            {Name: "wait", Risk: computer.RiskSafe, Run: runWait},
		"launch-app":      {Name: "launch-app", Risk: computer.RiskDestructive, Run: runLaunchApp},
		"wait-window":     {Name: "wait-window", Risk: computer.RiskSafe, Run: runWaitWindow},
	}
}

func runScreenshot(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("screenshot")
	out := fs.String("out", "", "output PNG path")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.Screenshot(ctx, *out)
}

func runZoom(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("zoom")
	out := fs.String("out", "", "output PNG path")
	x1 := fs.Int("x1", 0, "left coordinate")
	y1 := fs.Int("y1", 0, "top coordinate")
	x2 := fs.Int("x2", 0, "right coordinate")
	y2 := fs.Int("y2", 0, "bottom coordinate")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.Zoom(ctx, *out, *x1, *y1, *x2, *y2)
}

func runListWindows(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("list-windows")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.ListWindows(ctx)
}

func runFocusWindow(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("focus-window")
	title := fs.String("title", "", "window title")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.FocusWindow(ctx, *title)
}

func runMouseMove(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("mouse-move", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.MouseMove(ctx, x, y)
}

func runClick(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("click", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.Click(ctx, x, y)
}

func runRightClick(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("right-click", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.RightClick(ctx, x, y)
}

func runMiddleClick(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("middle-click", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.MiddleClick(ctx, x, y)
}

func runDoubleClick(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("double-click", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.DoubleClick(ctx, x, y)
}

func runTripleClick(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("triple-click", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.TripleClick(ctx, x, y)
}

func runLeftMouseDown(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("left-mouse-down", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.LeftMouseDown(ctx, x, y)
}

func runLeftMouseUp(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	x, y, err := parsePoint("left-mouse-up", args)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.LeftMouseUp(ctx, x, y)
}

func parsePoint(name string, args []string) (int, int, error) {
	fs := newCommandFlagSet(name)
	x := fs.Int("x", 0, "x coordinate")
	y := fs.Int("y", 0, "y coordinate")
	if err := fs.Parse(args); err != nil {
		return 0, 0, err
	}
	return *x, *y, nil
}

func runDrag(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("drag")
	fromX := fs.Int("from-x", 0, "start x coordinate")
	fromY := fs.Int("from-y", 0, "start y coordinate")
	toX := fs.Int("to-x", 0, "end x coordinate")
	toY := fs.Int("to-y", 0, "end y coordinate")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.Drag(ctx, *fromX, *fromY, *toX, *toY)
}

func runTypeText(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("type-text")
	text := fs.String("text", "", "text to type")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.TypeText(ctx, *text)
}

func runHotkey(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("hotkey")
	keys := fs.String("keys", "", "shortcut, for example Ctrl+A")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.Hotkey(ctx, *keys)
}

func runHoldKey(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("hold-key")
	keys := fs.String("keys", "", "key or shortcut to hold")
	durationText := fs.String("duration", "1s", "hold duration")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	duration, err := parseDuration(*durationText)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.HoldKey(ctx, *keys, duration)
}

func runScroll(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("scroll")
	amount := fs.Int("amount", 0, "signed scroll amount")
	direction := fs.String("direction", "", "scroll direction: up, down, left, or right")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.Scroll(ctx, *direction, *amount)
}

func runWait(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("wait")
	durationText := fs.String("duration", "1s", "wait duration")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	duration, err := parseDuration(*durationText)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.Wait(ctx, duration)
}

func runLaunchApp(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("launch-app")
	name := fs.String("name", "", "application name or executable")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.LaunchApp(ctx, *name)
}

func runWaitWindow(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("wait-window")
	title := fs.String("title", "", "window title")
	timeoutText := fs.String("timeout", "10s", "timeout duration")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	timeout, err := parseDuration(*timeoutText)
	if err != nil {
		return computer.Result{}, err
	}
	return runtime.WaitWindow(ctx, *title, timeout)
}

func newCommandFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parseDuration(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("--timeout is required")
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid --timeout: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("--timeout must be positive")
	}
	return duration, nil
}

func parseRisk(value string) (computer.Risk, error) {
	switch computer.Risk(strings.ToLower(strings.TrimSpace(value))) {
	case computer.RiskSafe:
		return computer.RiskSafe, nil
	case computer.RiskGuarded:
		return computer.RiskGuarded, nil
	case computer.RiskDestructive:
		return computer.RiskDestructive, nil
	default:
		return "", fmt.Errorf("--allow-risk must be safe, guarded, or destructive")
	}
}

func riskAllowed(allow computer.Risk, required computer.Risk) bool {
	return riskRank(allow) >= riskRank(required)
}

func riskRank(risk computer.Risk) int {
	switch risk {
	case computer.RiskSafe:
		return 1
	case computer.RiskGuarded:
		return 2
	case computer.RiskDestructive:
		return 3
	default:
		return 0
	}
}

func writeRuntimeError(stdout io.Writer, pretty bool, err error) {
	var runtimeErr computer.RuntimeError
	if errors.As(err, &runtimeErr) {
		writeError(stdout, pretty, runtimeErr.Code, runtimeErr.Message, runtimeErr.Retryable)
		return
	}
	writeError(stdout, pretty, "runtime_error", err.Error(), true)
}

func writeError(stdout io.Writer, pretty bool, code string, message string, retryable bool) {
	writeJSON(stdout, pretty, response{
		OK: false,
		Error: &errorPayload{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	})
}

func writeJSON(stdout io.Writer, pretty bool, payload response) {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	_ = encoder.Encode(payload)
}

func usage() string {
	return strings.TrimSpace(`
RelayComputerUse

Usage:
  relay-computer-use [--runtime auto|fake] [--allow-risk safe|guarded|destructive] [--pretty] <command> [flags]

Commands:
  screenshot --out <path>
  zoom --out <path> --x1 <n> --y1 <n> --x2 <n> --y2 <n>
  list-windows
  focus-window --title <text>
  mouse-move --x <n> --y <n>
  click --x <n> --y <n>
  right-click --x <n> --y <n>
  middle-click --x <n> --y <n>
  double-click --x <n> --y <n>
  triple-click --x <n> --y <n>
  left-mouse-down --x <n> --y <n>
  left-mouse-up --x <n> --y <n>
  drag --from-x <n> --from-y <n> --to-x <n> --to-y <n>
  type-text --text <text>
  hotkey --keys <Ctrl+A>
  hold-key --keys <Shift> --duration <duration>
  scroll --direction <up|down|left|right> --amount <n>
  wait --duration <duration>
  launch-app --name <app>
  wait-window --title <text> --timeout <duration>
`) + "\n"
}
