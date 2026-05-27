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
		"screenshot":   {Name: "screenshot", Risk: computer.RiskSafe, Run: runScreenshot},
		"list-windows": {Name: "list-windows", Risk: computer.RiskSafe, Run: runListWindows},
		"focus-window": {Name: "focus-window", Risk: computer.RiskGuarded, Run: runFocusWindow},
		"click":        {Name: "click", Risk: computer.RiskGuarded, Run: runClick},
		"double-click": {Name: "double-click", Risk: computer.RiskGuarded, Run: runDoubleClick},
		"drag":         {Name: "drag", Risk: computer.RiskDestructive, Run: runDrag},
		"type-text":    {Name: "type-text", Risk: computer.RiskGuarded, Run: runTypeText},
		"hotkey":       {Name: "hotkey", Risk: computer.RiskGuarded, Run: runHotkey},
		"scroll":       {Name: "scroll", Risk: computer.RiskGuarded, Run: runScroll},
		"launch-app":   {Name: "launch-app", Risk: computer.RiskDestructive, Run: runLaunchApp},
		"wait-window":  {Name: "wait-window", Risk: computer.RiskSafe, Run: runWaitWindow},
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

func runClick(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("click")
	x := fs.Int("x", 0, "x coordinate")
	y := fs.Int("y", 0, "y coordinate")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.Click(ctx, *x, *y)
}

func runDoubleClick(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("double-click")
	x := fs.Int("x", 0, "x coordinate")
	y := fs.Int("y", 0, "y coordinate")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.DoubleClick(ctx, *x, *y)
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

func runScroll(ctx context.Context, runtime computer.Runtime, args []string) (computer.Result, error) {
	fs := newCommandFlagSet("scroll")
	amount := fs.Int("amount", 0, "signed scroll amount")
	if err := fs.Parse(args); err != nil {
		return computer.Result{}, err
	}
	return runtime.Scroll(ctx, *amount)
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
  list-windows
  focus-window --title <text>
  click --x <n> --y <n>
  double-click --x <n> --y <n>
  drag --from-x <n> --from-y <n> --to-x <n> --to-y <n>
  type-text --text <text>
  hotkey --keys <Ctrl+A>
  scroll --amount <n>
  launch-app --name <app>
  wait-window --title <text> --timeout <duration>
`) + "\n"
}
