package codebuddyhooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
)

const (
	// 防抖窗口：连续 Stop 事件在此时间内不重复通知（秒）
	// CodeBuddy 的 Stop 是对话回合结束，用户需手动输入才触发下一轮，
	// 所以 3 秒足够区分"连续对话"和"任务真正结束"
	DebounceSeconds = 3
)

// DebounceFilePath 返回防抖状态文件路径，放在 agent-notify 配置目录下。
func DebounceFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", ".codebuddy_debounce"), nil
}

// RecordStop 记录一次 Stop 事件，写入防抖文件。
// 返回被覆盖的上一条记录的会话 ID（空字符串表示无覆盖）。
func RecordStop(sessionID string, workspace string) (string, error) {
	path, err := DebounceFilePath()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	now := time.Now().Unix()
	content := fmt.Sprintf("%d\n%s\n%s\n", now, sessionID, workspace)

	// 读取旧记录
	oldSession := ""
	if data, err := os.ReadFile(path); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
		if len(parts) >= 2 {
			oldSession = parts[1]
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}

	return oldSession, nil
}

// CheckAndFire 检查防抖文件中的时间戳是否匹配，如果匹配则发送通知。
// expectedTimestamp 是调用者期望的时间戳，只有文件中的时间戳 >= 这个值才发送。
// 这样可以避免"旧进程醒来后发送了过期的通知"。
func CheckAndFire(ctx context.Context, cfg config.Config, statePath, logPath, expectedTimestampStr string) error {
	path, err := DebounceFilePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取防抖文件失败: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
	if len(parts) < 2 {
		return fmt.Errorf("防抖文件格式错误")
	}

	fileTimestamp, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return fmt.Errorf("解析时间戳失败: %w", err)
	}

	expectedTimestamp, err := strconv.ParseInt(strings.TrimSpace(expectedTimestampStr), 10, 64)
	if err != nil {
		return fmt.Errorf("解析期望时间戳失败: %w", err)
	}

	// 只有文件时间戳 == 期望时间戳才发送（精确匹配）
	// 如果文件时间戳更大，说明有新的 Stop 覆盖了，跳过
	if fileTimestamp != expectedTimestamp {
		return nil // 静默跳过
	}

	sessionID := strings.TrimSpace(parts[1])
	workspace := ""
	if len(parts) >= 3 {
		workspace = strings.TrimSpace(parts[2])
	}

	msg := notify.Message{
		Agent:     "codebuddy",
		Event:     "run_completed",
		SessionID: sessionID,
		Workspace: workspace,
		Title:     notify.FormatTitle("codebuddy", "run_completed"),
		Body:      notify.DefaultBody("run_completed"),
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
