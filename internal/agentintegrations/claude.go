package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/claudehooks"
	"github.com/hellolib/agent-notify/internal/common"
)

// ClaudeIntegration implements Integration for Claude Code.
type ClaudeIntegration struct{}

// NewClaudeIntegration creates a new Claude Code integration.
func NewClaudeIntegration() *ClaudeIntegration {
	return &ClaudeIntegration{}
}

// Name returns the display name for Claude Code.
func (c *ClaudeIntegration) Name() string {
	return "Claude Code"
}

// DetectInstalled checks if Claude Code is available in any form:
// CLI binary, VS Code extension, or existing settings.json.
func (c *ClaudeIntegration) DetectInstalled() bool {
	// 1. CLI binary in PATH (standalone install)
	if _, err := exec.LookPath("claude"); err == nil {
		return true
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// 2. settings.json exists (shared between CLI and VS Code extension)
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); err == nil {
		return true
	}

	// 3. VS Code extension directory
	matches, _ := filepath.Glob(filepath.Join(home, ".vscode", "extensions", "anthropic.claude-code-*"))
	if len(matches) > 0 {
		return true
	}

	return false
}

// SettingsPath returns the path to Claude Code's settings.json file.
func (c *ClaudeIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude", "settings.json"), nil
	case "project":
		return filepath.Join(".claude", "settings.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install configures Claude Code to use agent-notify by setting up hooks.
// 已存在 agent-notify hook 的事件会被跳过；用户挂载的其他 hook 原样保留。
func (c *ClaudeIntegration) Install(settingsPath, binaryPath string) error {
	return claudehooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only the hook entries written by agent-notify from
// Claude Code's settings.json. User-defined hooks are preserved.
func (c *ClaudeIntegration) Uninstall(settingsPath string) error {
	return claudehooks.Uninstall(settingsPath)
}

// IsHookInstalled checks if agent-notify hooks are installed in the settings file.
func (c *ClaudeIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return claudehooks.IsInstalled(settingsPath)
}
