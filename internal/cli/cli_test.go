package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestFakeScreenshotReturnsJSON(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "screen.png")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--runtime", "fake", "screenshot", "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", payload["ok"])
	}
	if payload["action"] != "screenshot" {
		t.Fatalf("unexpected action: %#v", payload["action"])
	}
	observation := payload["observation"].(map[string]interface{})
	if observation["path"] != out {
		t.Fatalf("unexpected screenshot path: %#v", observation["path"])
	}
}

func TestDestructiveCommandRequiresExplicitRisk(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--runtime", "fake", "launch-app", "--name", "notepad"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	errorPayload := payload["error"].(map[string]interface{})
	if errorPayload["code"] != "policy_denied" {
		t.Fatalf("unexpected error code: %#v", errorPayload["code"])
	}
}

func TestDestructiveCommandRunsWithRiskOverride(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(
		context.Background(),
		[]string{"--runtime", "fake", "--allow-risk", "destructive", "launch-app", "--name", "notepad"},
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}
}

func TestClaudeStyleActionAliases(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--runtime", "fake", "right_click", "--x", "10", "--y", "20"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["action"] != "right-click" {
		t.Fatalf("unexpected action: %#v", payload["action"])
	}
}

func TestFakeZoomReturnsCroppedImage(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "zoom.png")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--runtime", "fake", "zoom", "--out", out, "--x1", "10", "--y1", "20", "--x2", "110", "--y2", "70"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	observation := payload["observation"].(map[string]interface{})
	if observation["width"] != float64(100) || observation["height"] != float64(50) {
		t.Fatalf("unexpected zoom dimensions: %#v", observation)
	}
}

func TestWaitCommand(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--runtime", "fake", "wait", "--duration", "1ms"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}
}
