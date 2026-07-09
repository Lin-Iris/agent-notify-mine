package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/threadstore"
)

func TestExtractAgentTextFromClaudeResult(t *testing.T) {
	output := `{"type":"system","msg":"starting"}
{"type":"result","result":"模型最终输出"}`

	if got := extractAgentText("claude", output); got != "模型最终输出" {
		t.Fatalf("extractAgentText() = %q, want 模型最终输出", got)
	}
}

func TestExtractAgentTextFromClaudeStream(t *testing.T) {
	output := `{"type":"stream_event","event":{"delta":{"text":"模型"}}}
{"type":"stream_event","event":{"delta":{"text":"输出"}}}`

	if got := extractAgentText("claude", output); got != "模型输出" {
		t.Fatalf("extractAgentText() = %q, want 模型输出", got)
	}
}

func TestExtractAgentTextFromCodexMessage(t *testing.T) {
	output := `{"type":"event","message":"working"}
{"type":"agent_message","message":"Codex 最终输出"}`

	if got := extractAgentText("codex", output); got != "Codex 最终输出" {
		t.Fatalf("extractAgentText() = %q, want Codex 最终输出", got)
	}
}

func TestTaskLogKeepsLogWhenNoFinalResult(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "task.log")
	if err := os.WriteFile(logPath, []byte("line1\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	task := threadstore.Task{LogPath: logPath}

	got, err := taskTextByTask(task, "log", 0)
	if err != nil {
		t.Fatalf("taskTextByTask() error = %v", err)
	}
	if !strings.Contains(got, "line2") {
		t.Fatalf("taskTextByTask() = %q, want log content", got)
	}
}

func TestParseModelStreamLineCapturesClaudeDelta(t *testing.T) {
	got := parseModelStreamLine(`{"type":"stream_event","event":{"delta":{"text":"模型"}}}`)

	if got.Output != "模型" {
		t.Fatalf("Output = %q, want 模型", got.Output)
	}
	if got.Reasoning != "" {
		t.Fatalf("Reasoning = %q, want empty", got.Reasoning)
	}
}

func TestParseModelStreamLineCapturesCodexAssistantMessage(t *testing.T) {
	got := parseModelStreamLine(`{"type":"agent_message","message":"Codex 输出"}`)

	if got.Output != "Codex 输出" {
		t.Fatalf("Output = %q, want Codex 输出", got.Output)
	}
}

func TestParseModelStreamLineCapturesExplicitReasoning(t *testing.T) {
	got := parseModelStreamLine(`{"type":"reasoning_summary","summary":"分析摘要"}`)

	if got.Reasoning != "分析摘要" {
		t.Fatalf("Reasoning = %q, want 分析摘要", got.Reasoning)
	}
	if got.Output != "" {
		t.Fatalf("Output = %q, want empty", got.Output)
	}
}

func TestParseModelStreamLineIgnoresToolAndPlainLog(t *testing.T) {
	for _, line := range []string{
		`{"type":"tool_call","name":"Bash","args":{"command":"git status"}}`,
		`Error: command failed`,
	} {
		got := parseModelStreamLine(line)
		if got.Output != "" || got.Reasoning != "" {
			t.Fatalf("parseModelStreamLine(%q) = %#v, want empty", line, got)
		}
	}
}

func TestTaskResultReturnsModelStreamNotLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "task.log")
	if err := os.WriteFile(logPath, []byte("log-only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	task := threadstore.Task{
		LogPath:        logPath,
		StreamOutput:   "模型流式输出",
		ReasoningTrace: "显式思考摘要",
	}

	got, err := taskTextByTask(task, "result", 0)
	if err != nil {
		t.Fatalf("taskTextByTask() error = %v", err)
	}
	if !strings.Contains(got, "模型流式输出") || !strings.Contains(got, "显式思考摘要") {
		t.Fatalf("taskTextByTask() = %q, want model output and reasoning", got)
	}
	if strings.Contains(got, "log-only") {
		t.Fatalf("taskTextByTask() = %q, should not include log", got)
	}
}

func TestTaskFinalOutputTextUsesFinalResultOnly(t *testing.T) {
	task := threadstore.Task{
		FinalResult:    "最终回答",
		StreamOutput:   "最终回答最终回答最终回答",
		ReasoningTrace: "显式思考摘要",
		Progress:       "进度预览",
	}

	if got := taskFinalOutputText(task); got != "最终回答" {
		t.Fatalf("taskFinalOutputText() = %q, want final result only", got)
	}
}

func TestTaskFinalOutputTextFallbacksWhenFinalEmpty(t *testing.T) {
	if got := taskFinalOutputText(threadstore.Task{Error: "任务失败"}); got != "任务失败" {
		t.Fatalf("taskFinalOutputText(error) = %q, want error", got)
	}
	if got := taskFinalOutputText(threadstore.Task{Progress: "暂无最终结果"}); got != "暂无最终结果" {
		t.Fatalf("taskFinalOutputText(progress) = %q, want progress", got)
	}
	if got := taskFinalOutputText(threadstore.Task{}); got != "（无模型输出）" {
		t.Fatalf("taskFinalOutputText(empty) = %q, want empty output message", got)
	}
}
