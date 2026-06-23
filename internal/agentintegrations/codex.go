package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/codexhooks"
	"github.com/hellolib/agent-notify/internal/common"
)

// CodexIntegration implements Integration for Codex.
type CodexIntegration struct{}

// NewCodexIntegration creates a new Codex integration.
func NewCodexIntegration() *CodexIntegration {
	return &CodexIntegration{}
}

// Name returns the display name for Codex.
func (c *CodexIntegration) Name() string {
	return "Codex"
}

// DetectInstalled checks if Codex is available in any form:
// CLI binary, Codex.app GUI, or existing config directory.
func (c *CodexIntegration) DetectInstalled() bool {
	// 1. CLI binary in PATH
	if _, err := exec.LookPath("codex"); err == nil {
		return true
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// 2. Codex.app GUI (macOS)
	appPaths := []string{
		"/Applications/Codex.app",
		filepath.Join(home, "Applications", "Codex.app"),
	}
	for _, p := range appPaths {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return true
		}
	}

	// 3. 配置目录存在（hooks.json 或 config.toml 任一个）
	codexDir := filepath.Join(home, ".codex")
	if fi, err := os.Stat(codexDir); err == nil && fi.IsDir() {
		for _, name := range []string{"hooks.json", "config.toml"} {
			if _, err := os.Stat(filepath.Join(codexDir, name)); err == nil {
				return true
			}
		}
	}

	return false
}

// SettingsPath returns the path to Codex's hooks.json file.
// Codex 同时支持 ~/.codex/hooks.json 与 ~/.codex/config.toml 内联 [hooks]；
// 这里统一使用 hooks.json，结构上与 Claude settings.json 对齐，便于维护。
func (c *CodexIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".codex", "hooks.json"), nil
	case "project":
		return filepath.Join(".codex", "hooks.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install 写入 Codex hooks.json，订阅 permission_request 与 stop 事件。
// 已存在 agent-notify hook 的事件会被跳过；用户挂载的其他 hook 原样保留。
func (c *CodexIntegration) Install(settingsPath, binaryPath string) error {
	return codexhooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only the hook entries written by agent-notify from
// Codex's hooks.json. User-defined hooks are preserved. The
// [features] hooks toggle in config.toml is NOT removed — it is a generic
// switch other hooks may depend on.
func (c *CodexIntegration) Uninstall(settingsPath string) error {
	return codexhooks.Uninstall(settingsPath)
}

// IsHookInstalled 检查 Codex hooks.json 中是否已经登记了 handle-codex-hook。
func (c *CodexIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return codexhooks.IsInstalled(settingsPath)
}
