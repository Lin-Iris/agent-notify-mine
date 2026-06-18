package codebuddyhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

// debounceFireCommand 是触发防抖通知的内部命令名。
const debounceFireCommand = "handle-codebuddy-fire-stop"

// Handle 处理 CodeBuddy hook 的 stdin 输入，解析事件并分发通知。
// Stop 事件使用防抖机制：连续 Stop 在 DebounceSeconds 内不重复通知。
func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("codebuddy: read stdin error: %v", err))
	}

	_ = state.AppendLog(logPath, fmt.Sprintf("codebuddy: raw stdin: %s", string(data)))

	// 检查是否是 Stop 事件（需要防抖）
	var raw struct {
		HookEventName string `json:"hook_event_name"`
		SessionID     string `json:"session_id"`
		CWD           string `json:"cwd"`
	}
	if err := json.Unmarshal(data, &raw); err == nil && raw.HookEventName == "Stop" {
		// 记录防抖时间戳，并启动后台进程
		// 后台进程等待 DebounceSeconds 秒后，检查防抖文件是否被更新
		// 如果未被更新（没有新的 Stop），则发送通知
		recordAndDebounce(raw.SessionID, raw.CWD, logPath)
		return nil
	}

	// 非 Stop 事件走正常流程
	msg, err := ParseMessage(data)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("codebuddy: skip event: %v", err))
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}

// recordAndDebounce 记录 Stop 事件到防抖文件，并启动后台检查进程。
// 每次 Stop 都启动一个后台进程，但只有最后一个（时间戳未被后续覆盖的）会真正发送通知。
func recordAndDebounce(sessionID, workspace, logPath string) {
	_, err := RecordStop(sessionID, workspace)
	if err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("codebuddy: debounce record error: %v", err))
		return
	}

	now := time.Now().Unix()

	binaryPath, err := os.Executable()
	if err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("codebuddy: get executable error: %v", err))
		return
	}

	cmd := exec.Command("nohup", binaryPath, debounceFireCommand, fmt.Sprintf("%d", now))
	if err := cmd.Start(); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("codebuddy: start debounce process error: %v", err))
		return
	}
}
