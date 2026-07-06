package hermeshooks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"gopkg.in/yaml.v3"
)

const hookCommandMarker = "handle-hermes-hook"

// managedHermesEvents 是本插件托管的 Hermes shell hook 事件列表。
var managedHermesEvents = []struct {
	EventKey   string
	SubCommand string
}{
	{EventKey: "pre_approval_request", SubCommand: "permission_required"},
	{EventKey: "post_llm_call", SubCommand: "run_completed"},
}

// BuildHookSettings 返回 Hermes config.yaml 中 hooks 部分的默认值（用于显示/参考）。
func BuildHookSettings(binaryPath string) []HermesHookEntry {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	var entries []HermesHookEntry
	for range managedHermesEvents {
		entries = append(entries, HermesHookEntry{Command: hookCommand(binaryPath), Timeout: 10})
	}
	return entries
}

// Install 在 ~/.hermes/config.yaml 的 hooks 块中写入条目。
func Install(path string, binaryPath string) error {
	cfg, err := loadHermesConfig(path)
	if err != nil {
		return err
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)

	if cfg.Hooks == nil {
		cfg.Hooks = &HermesHooksConfig{}
	}

	for _, evt := range managedHermesEvents {
		command := hookCommand(binaryPath)
		entry := HermesHookEntry{Command: command, Timeout: 10}

		switch evt.EventKey {
		case "post_llm_call":
			if hasHermesManagedHook(cfg.Hooks.PostLLMCall) {
				cfg.Hooks.PostLLMCall = normalizeHermesManagedHooks(cfg.Hooks.PostLLMCall, command)
				continue
			}
			cfg.Hooks.PostLLMCall = append(cfg.Hooks.PostLLMCall, entry)
		case "pre_approval_request":
			if hasHermesManagedHook(cfg.Hooks.PreApprovalRequest) {
				cfg.Hooks.PreApprovalRequest = normalizeHermesManagedHooks(cfg.Hooks.PreApprovalRequest, command)
				continue
			}
			cfg.Hooks.PreApprovalRequest = append(cfg.Hooks.PreApprovalRequest, entry)
		}
	}
	cfg.HooksAutoAccept = true

	return saveHermesConfig(path, cfg)
}

func hookCommand(binaryPath string) string {
	return binaryPath + " " + hookCommandMarker
}

// IsInstalled 检查配置中是否已存在 agent-notify 的 hook。
func IsInstalled(path string) (bool, error) {
	cfg, err := loadHermesConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	if cfg.Hooks == nil {
		return false, nil
	}

	if hasHermesManagedHook(cfg.Hooks.PostLLMCall) {
		return true, nil
	}
	if hasHermesManagedHook(cfg.Hooks.PreApprovalRequest) {
		return true, nil
	}
	return false, nil
}

// Uninstall 从配置中移除 agent-notify 写入的 hook 条目。
func Uninstall(path string) error {
	cfg, err := loadHermesConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	if cfg.Hooks == nil {
		return nil
	}

	cfg.Hooks.PostLLMCall = filterHermesHooks(cfg.Hooks.PostLLMCall)
	cfg.Hooks.PreApprovalRequest = filterHermesHooks(cfg.Hooks.PreApprovalRequest)

	if len(cfg.Hooks.PostLLMCall) == 0 && len(cfg.Hooks.PreApprovalRequest) == 0 {
		cfg.Hooks = nil
		cfg.HooksAutoAccept = false
	}

	return saveHermesConfig(path, cfg)
}

func hasHermesManagedHook(entries []HermesHookEntry) bool {
	for _, e := range entries {
		if strings.Contains(e.Command, hookCommandMarker) {
			return true
		}
	}
	return false
}

func normalizeHermesManagedHooks(entries []HermesHookEntry, command string) []HermesHookEntry {
	for i := range entries {
		if strings.Contains(entries[i].Command, hookCommandMarker) {
			entries[i].Command = command
		}
	}
	return entries
}

func filterHermesHooks(entries []HermesHookEntry) []HermesHookEntry {
	cleaned := entries[:0]
	for _, e := range entries {
		if !strings.Contains(e.Command, hookCommandMarker) {
			cleaned = append(cleaned, e)
		}
	}
	return cleaned
}

func loadHermesConfig(path string) (*HermesConfig, error) {
	cfg := &HermesConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func saveHermesConfig(path string, cfg *HermesConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// 读取原有文件，保留其他字段
	original := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		yaml.Unmarshal(data, &original)
	}

	hooksMap, _ := original["hooks"].(map[string]any)
	if cfg.Hooks != nil {
		if hooksMap == nil {
			hooksMap = map[string]any{}
		}
		if len(cfg.Hooks.PostLLMCall) > 0 {
			hooksMap["post_llm_call"] = hermesEntriesToAny(cfg.Hooks.PostLLMCall)
		} else {
			delete(hooksMap, "post_llm_call")
		}
		if len(cfg.Hooks.PreApprovalRequest) > 0 {
			hooksMap["pre_approval_request"] = hermesEntriesToAny(cfg.Hooks.PreApprovalRequest)
		} else {
			delete(hooksMap, "pre_approval_request")
		}
		if len(hooksMap) > 0 {
			original["hooks"] = hooksMap
		} else {
			delete(original, "hooks")
		}
	} else {
		if hooksMap, ok := original["hooks"].(map[string]any); ok {
			delete(hooksMap, "post_llm_call")
			delete(hooksMap, "pre_approval_request")
			if len(hooksMap) == 0 {
				delete(original, "hooks")
			}
		}
	}
	original["hooks_auto_accept"] = cfg.HooksAutoAccept

	out, err := yaml.Marshal(original)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func hermesEntriesToAny(entries []HermesHookEntry) []any {
	result := make([]any, 0, len(entries))
	for _, e := range entries {
		entry := map[string]any{"command": e.Command}
		if e.Timeout > 0 {
			entry["timeout"] = e.Timeout
		}
		result = append(result, entry)
	}
	return result
}
