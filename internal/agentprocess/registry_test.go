package agentprocess

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateWorkspaceAllowsCurrentProject(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := ValidateWorkspace(wd); err != nil {
		t.Fatalf("validateWorkspace(%q) error = %v", wd, err)
	}
}

func TestValidateWorkspaceRejectsBroadOrTemporaryDirectories(t *testing.T) {
	for _, path := range []string{"/", os.TempDir(), t.TempDir()} {
		if err := ValidateWorkspace(path); err == nil {
			t.Fatalf("validateWorkspace(%q) error = nil, want rejection", path)
		}
	}
}

func TestCommandArgsUseResumeModes(t *testing.T) {
	claudeArgs, err := commandArgsWithSession("claude", "/repo", "workspace-write", "hello", "session-1", true)
	if err != nil {
		t.Fatalf("claude commandArgsWithSession error = %v", err)
	}
	if !containsArg(claudeArgs, "--resume") || !containsArg(claudeArgs, "session-1") || !containsArg(claudeArgs, "stream-json") {
		t.Fatalf("claude args = %v, want resume stream-json", claudeArgs)
	}
	codexArgs, err := commandArgsWithSession("codex", "/repo", "workspace-write", "hello", "session-1", true)
	if err != nil {
		t.Fatalf("codex commandArgsWithSession error = %v", err)
	}
	if !containsArg(codexArgs, "resume") || !containsArg(codexArgs, "session-1") || !containsArg(codexArgs, "--json") {
		t.Fatalf("codex args = %v, want exec resume json", codexArgs)
	}
}

func TestStartWithOptionsCapturesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "codex")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hello\necho err >&2\n"), 0o755); err != nil {
		t.Fatalf("write fakeagent error = %v", err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	reg := NewRegistry(filepath.Join(dir, "processes.json"))
	done := make(chan string, 1)
	rec, err := reg.StartWithOptions(context.Background(), StartOptions{
		Profile:        "p",
		Agent:          "codex",
		Workspace:      mustGetwd(t),
		PermissionMode: "workspace-write",
		Prompt:         "ignored",
		LogsDir:        dir,
		TaskID:         "task_test",
		OnExit: func(_ Record, output string, _ int, _ error) {
			done <- output
		},
	})
	if err != nil {
		t.Fatalf("StartWithOptions error = %v", err)
	}
	if rec.PID <= 0 {
		t.Fatalf("PID = %d, want > 0", rec.PID)
	}
	select {
	case output := <-done:
		if !strings.Contains(output, "hello") || !strings.Contains(output, "err") {
			t.Fatalf("output = %q, want stdout and stderr", output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for process output")
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error = %v", err)
	}
	return wd
}
