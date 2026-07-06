package codebuddyhooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
)

const hookCommandMarker = "handle-codebuddy-hook"
const preToolUseCommandMarker = "handle-codebuddy-pretooluse"

// preToolUseMatcher 匹配需要通知的高风险工具
const preToolUseMatcher = "execute_command|write_to_file|delete_file|create_file|ask_followup_question|edit_and_apply|edit"

type eventMeta struct {
	EventKey   string
	SubCommand string
	HasMatcher bool
}

// managedEvents 是标准 hook 事件（PreToolUse 有特殊逻辑，单独处理）
var managedEvents = []eventMeta{
	{EventKey: "PermissionRequest", SubCommand: "permission_required", HasMatcher: true},
	{EventKey: "Notification", SubCommand: "input_required", HasMatcher: false},
	{EventKey: "Stop", SubCommand: "run_completed", HasMatcher: false},
	{EventKey: "PostToolUseFailure", SubCommand: "run_failed", HasMatcher: false},
	{EventKey: "SessionEnd", SubCommand: "run_completed", HasMatcher: false},
}

// preToolUseEntry 生成 PreToolUse hook 条目（含 matcher + timeout）
func preToolUseEntry(binaryPath string) map[string]any {
	return map[string]any{
		"matcher": preToolUseMatcher,
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": preToolUseCommand(binaryPath),
				"timeout": 10,
			},
		},
	}
}

// BuildHookSettings 构建 CodeBuddy settings.json 中的 hooks 配置。
func BuildHookSettings(binaryPath string) map[string]any {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	hooks := map[string]any{}

	for _, evt := range managedEvents {
		command := hookCommand(binaryPath)
		entry := map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": command,
				},
			},
		}
		if evt.HasMatcher {
			entry["matcher"] = ""
		}
		hooks[evt.EventKey] = []any{entry}
	}

	hooks["PreToolUse"] = []any{preToolUseEntry(binaryPath)}
	return map[string]any{"hooks": hooks}
}

// Install 增量写入 hooks：已存在标记 hook 的事件跳过。
func Install(path string, binaryPath string) error {
	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	// 标准事件
	for _, evt := range managedEvents {
		if eventHasManagedHook(hooks, evt.EventKey) {
			normalizeManagedHookCommands(hooks, evt.EventKey, hookCommand(binaryPath))
			continue
		}
		command := hookCommand(binaryPath)
		entry := map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": command,
				},
			},
		}
		if evt.HasMatcher {
			entry["matcher"] = ""
		}
		entries := toAnySlice(hooks[evt.EventKey])
		entries = append(entries, entry)
		hooks[evt.EventKey] = entries
	}

	// PreToolUse 特殊处理
	if eventHasManagedHook(hooks, "PreToolUse") {
		normalizeManagedHookCommands(hooks, "PreToolUse", preToolUseCommand(binaryPath))
	} else {
		entries := toAnySlice(hooks["PreToolUse"])
		entries = append(entries, preToolUseEntry(binaryPath))
		hooks["PreToolUse"] = entries
	}

	settings["hooks"] = hooks
	return writeSettings(path, settings)
}

func hookCommand(binaryPath string) string {
	return binaryPath + " " + hookCommandMarker
}

func preToolUseCommand(binaryPath string) string {
	return binaryPath + " " + preToolUseCommandMarker
}

// IsInstalled 检查是否已安装 agent-notify 的 hook。
func IsInstalled(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, err
		}
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}

	for _, evt := range managedEvents {
		if eventHasManagedHook(hooks, evt.EventKey) {
			return true, nil
		}
	}
	if eventHasManagedHook(hooks, "PreToolUse") {
		return true, nil
	}
	return false, nil
}

// Uninstall 仅移除 agent-notify 写入的 hook 条目，保留用户自定义的。
func Uninstall(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return nil
	}

	for event, raw := range hooks {
		entries := toAnySlice(raw)
		cleaned := entries[:0]
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				cleaned = append(cleaned, entry)
				continue
			}
			inner := toAnySlice(entryMap["hooks"])
			keptInner := inner[:0]
			for _, h := range inner {
				if !isManagedHook(h) {
					keptInner = append(keptInner, h)
				}
			}
			if len(keptInner) == 0 {
				continue
			}
			entryMap["hooks"] = keptInner
			cleaned = append(cleaned, entryMap)
		}
		if len(cleaned) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = cleaned
		}
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	return writeSettings(path, settings)
}

func readSettings(path string) (map[string]any, error) {
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func eventHasManagedHook(hooks map[string]any, event string) bool {
	for _, entry := range toAnySlice(hooks[event]) {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range toAnySlice(entryMap["hooks"]) {
			if isManagedHook(h) {
				return true
			}
		}
	}
	return false
}

func normalizeManagedHookCommands(hooks map[string]any, event, command string) {
	for _, entry := range toAnySlice(hooks[event]) {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range toAnySlice(entryMap["hooks"]) {
			hookMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hookMap["command"].(string)
			if strings.Contains(cmd, hookCommandMarker) || strings.Contains(cmd, preToolUseCommandMarker) {
				hookMap["command"] = command
			}
		}
	}
}

func isManagedHook(hook any) bool {
	m, ok := hook.(map[string]any)
	if !ok {
		return false
	}
	cmd, _ := m["command"].(string)
	return strings.Contains(cmd, hookCommandMarker) || strings.Contains(cmd, preToolUseCommandMarker)
}

func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []map[string]any:
		out := make([]any, 0, len(s))
		for _, item := range s {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}
