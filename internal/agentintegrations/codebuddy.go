package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/codebuddyhooks"
	"github.com/hellolib/agent-notify/internal/common"
)

// CodeBuddyIntegration 实现 CodeBuddy 的 Integration 接口。
type CodeBuddyIntegration struct{}

// NewCodeBuddyIntegration 创建 CodeBuddy 集成实例。
func NewCodeBuddyIntegration() *CodeBuddyIntegration {
	return &CodeBuddyIntegration{}
}

// Name 返回显示名称。
func (c *CodeBuddyIntegration) Name() string {
	return "CodeBuddy"
}

// DetectInstalled 检查 codebuddy CLI 是否安装。
func (c *CodeBuddyIntegration) DetectInstalled() bool {
	_, err := exec.LookPath("codebuddy")
	return err == nil
}

// SettingsPath 返回 CodeBuddy settings.json 的路径。
func (c *CodeBuddyIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".codebuddy", "settings.json"), nil
	case "project":
		return filepath.Join(".codebuddy", "settings.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install 配置 CodeBuddy settings.json 中的 hooks。
func (c *CodeBuddyIntegration) Install(settingsPath, binaryPath string) error {
	return codebuddyhooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall 移除 agent-notify 写入的 hook 条目。
func (c *CodeBuddyIntegration) Uninstall(settingsPath string) error {
	return codebuddyhooks.Uninstall(settingsPath)
}

// IsHookInstalled 检查 hooks 是否已安装。
func (c *CodeBuddyIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return codebuddyhooks.IsInstalled(settingsPath)
}
