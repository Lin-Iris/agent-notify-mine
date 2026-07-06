package cursorhooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Cursor 通过 stdin 投递的 hook JSON。
type payload struct {
	HookEventName  string   `json:"hook_event_name"`
	ConversationID string   `json:"conversation_id"`
	WorkspaceRoots []string `json:"workspace_roots"`
	Status         string   `json:"status"`        // stop 事件: "completed" | "error" | "aborted"
	ErrorMessage   string   `json:"error_message"` // postToolUseFailure 事件
	ToolName       string   `json:"tool_name"`
	Command        string   `json:"command"` // beforeShellExecution 事件
	CWD            string   `json:"cwd"`     // beforeShellExecution 事件
}

// ── Adapter ────────────────────────────────────────────────

type Adapter struct{}

func (a Adapter) AgentName() string { return "cursor" }

func (a Adapter) Parse(data []byte) (event.Event, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return event.Event{}, err
	}

	workspace := ""
	if len(p.WorkspaceRoots) > 0 {
		workspace = p.WorkspaceRoots[0]
	}

	base := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "cursor",
		HookEvent:   p.HookEventName,
		SessionID:   p.ConversationID,
		Workspace:   workspace,
		RawPayload:  json.RawMessage(data),
		ReceivedAt:  time.Now(),
	}

	switch p.HookEventName {
	case "stop":
		switch p.Status {
		case "completed":
			base.Status = event.StatusCompleted
			base.Title = notify.FormatTitle("cursor", "run_completed")
			base.Body = notify.DefaultBody("run_completed")
		case "error", "aborted":
			base.Status = event.StatusFailed
			base.Title = notify.FormatTitle("cursor", "run_failed")
			base.Body = fmt.Sprintf("任务状态: %s", p.Status)
		default:
			base.Status = event.StatusCompleted
			base.Title = notify.FormatTitle("cursor", "run_completed")
			base.Body = notify.DefaultBody("run_completed")
		}

	case "beforeShellExecution":
		// 只对危险命令通知，安全命令（ls/cat/grep 等）静默跳过
		if isSafeCommand(p.Command) {
			return event.Event{}, fmt.Errorf("safe command filtered: %s", truncateString(p.Command, 100))
		}
		base.Status = event.StatusPermissionReq
		base.Title = notify.FormatTitle("cursor", "permission_required")
		body := fmt.Sprintf("命令: %s", truncateString(p.Command, 200))
		if p.CWD != "" {
			body += fmt.Sprintf("\n目录: %s", p.CWD)
		}
		base.Body = body

	case "postToolUseFailure":
		errMsg := p.ErrorMessage
		if errMsg == "" {
			errMsg = "工具执行失败"
		}
		base.Status = event.StatusFailed
		base.Title = notify.FormatTitle("cursor", "run_failed")
		base.Body = fmt.Sprintf("工具: %s\n错误: %s", p.ToolName, truncateString(errMsg, 200))

	default:
		return event.Event{}, fmt.Errorf("unsupported cursor hook event: %s", p.HookEventName)
	}

	return base, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// isSafeCommand 判断命令是否为安全操作（无需通知用户）。
// 返回 true 表示命令安全，应静默跳过；false 表示需推送通知。
func isSafeCommand(rawCmd string) bool {
	cmd := strings.TrimSpace(rawCmd)

	// 提取基础命令（去掉路径前缀和参数）
	baseCmd := extractBaseCommand(cmd)

	// 安全命令清单：纯读取操作，不产生副作用
	safeCommands := map[string]bool{
		"ls": true, "cat": true, "head": true, "tail": true, "less": true, "more": true,
		"grep": true, "find": true, "locate": true, "which": true, "whereis": true, "type": true,
		"echo": true, "printf": true, "printenv": true, "env": true,
		"pwd": true, "cd": true, "pushd": true, "popd": true, "dirs": true,
		"wc": true, "sort": true, "uniq": true, "cut": true, "awk": true, "tr": true,
		"file": true, "stat": true, "du": true, "df": true, "free": true,
		"date": true, "uptime": true, "whoami": true, "id": true, "uname": true, "hostname": true,
		"man": true, "whatis": true, "apropos": true, "info": true,
		"clear": true, "reset": true, "true": true, "false": true, "sleep": true,
		"git":  true, // 大部分 git 子命令是安全的，下面有危险子命令过滤
		"node": true, "python": true, "python3": true, "ruby": true, "go": true, "rustc": true,
		"npm": true, "yarn": true, "pnpm": true, "cargo": true, "pip": true, "pip3": true, "gem": true,
		"npx": true, "make": true,
	}

	if safeCommands[baseCmd] {
		return !hasDangerousSubCommand(cmd) && !hasDangerousPatterns(cmd)
	}

	// 不在安全清单内的命令 → 视为危险 → 需通知
	return false
}

// extractBaseCommand 从命令行提取基础命令名。
// "/usr/bin/git" → "git"，"sudo rm -rf /" → "sudo"
func extractBaseCommand(cmd string) string {
	// 去除管道和重定向前的命令
	if idx := strings.IndexAny(cmd, "|;&"); idx >= 0 {
		cmd = cmd[:idx]
	}
	cmd = strings.TrimSpace(cmd)

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	// 取最后一段作为命令名：/usr/bin/git → git
	name := parts[0]
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

// hasDangerousSubCommand 检查安全基础命令是否带了危险子命令或参数。
func hasDangerousSubCommand(cmd string) bool {
	lower := strings.ToLower(cmd)

	// git 危险操作
	if strings.Contains(lower, "git push") ||
		strings.Contains(lower, "git reset --hard") ||
		strings.Contains(lower, "git clean") ||
		strings.Contains(lower, "git rebase") {
		return true
	}

	// npm/yarn/pnpm 危险操作
	if strings.Contains(lower, "npm publish") ||
		strings.Contains(lower, "npm unpublish") ||
		strings.Contains(lower, "npm deprecate") ||
		strings.Contains(lower, "yarn publish") ||
		strings.Contains(lower, "pnpm publish") {
		return true
	}

	// pip/gem/cargo 安装操作
	if strings.Contains(lower, "pip install") ||
		strings.Contains(lower, "pip3 install") ||
		strings.Contains(lower, "gem install") ||
		strings.Contains(lower, "cargo install") {
		return true
	}

	// npm install -g
	if strings.Contains(lower, "npm install -g") ||
		strings.Contains(lower, "npm i -g") ||
		strings.Contains(lower, "npm uninstall -g") {
		return true
	}

	return false
}

// hasDangerousPatterns 检查整条命令是否包含危险操作模式。
func hasDangerousPatterns(cmd string) bool {
	lower := strings.ToLower(cmd)

	// sudo 提权
	if strings.HasPrefix(lower, "sudo ") || strings.Contains(lower, " sudo ") {
		return true
	}

	// 删除/修改系统文件的操作
	dangerousVerbs := []string{
		"rm ", "rmdir ", "chmod ", "chown ", "chgrp ", "mkfs.",
		"dd if=", "mv /", "cp /",
		"> /etc/", "> /usr/", "> /System/", "> /boot/",
		">>/etc/", ">>/usr/", ">>/System/",
		"curl ", "wget ",
	}
	for _, verb := range dangerousVerbs {
		if strings.Contains(lower, verb) {
			return true
		}
	}

	// curl/wget 管道到 shell
	if (strings.Contains(lower, "curl ") || strings.Contains(lower, "wget ")) &&
		(strings.Contains(lower, "| bash") || strings.Contains(lower, "| sh") || strings.Contains(lower, "| zsh")) {
		return true
	}

	// 进程管理
	if strings.Contains(lower, "kill ") || strings.Contains(lower, "killall ") ||
		strings.Contains(lower, "pkill ") || strings.Contains(lower, "shutdown") ||
		strings.Contains(lower, "reboot") || strings.Contains(lower, "systemctl ") ||
		strings.Contains(lower, "service ") || strings.Contains(lower, "launchctl ") {
		return true
	}

	// docker 危险操作
	if strings.Contains(lower, "docker rm") || strings.Contains(lower, "docker rmi") ||
		strings.Contains(lower, "docker stop") || strings.Contains(lower, "docker kill") ||
		strings.Contains(lower, "docker system prune") {
		return true
	}

	return false
}
