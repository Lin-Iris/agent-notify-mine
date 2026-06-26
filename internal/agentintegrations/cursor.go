package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/cursorhooks"
)

// CursorIntegration implements Integration for Cursor IDE.
type CursorIntegration struct{}

// NewCursorIntegration creates a new Cursor integration.
func NewCursorIntegration() *CursorIntegration {
	return &CursorIntegration{}
}

// Name returns the display name for Cursor.
func (c *CursorIntegration) Name() string {
	return "Cursor"
}

// DetectInstalled checks if Cursor is available.
func (c *CursorIntegration) DetectInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// 1. ~/.cursor/ directory exists
	if _, err := os.Stat(filepath.Join(home, ".cursor")); err == nil {
		return true
	}

	// 2. macOS: /Applications/Cursor.app
	if _, err := os.Stat("/Applications/Cursor.app"); err == nil {
		return true
	}

	// 3. cursor CLI in PATH
	if _, err := exec.LookPath("cursor"); err == nil {
		return true
	}

	return false
}

// SettingsPath returns the path to Cursor's hooks.json file.
func (c *CursorIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cursor", "hooks.json"), nil
	case "project":
		return filepath.Join(".cursor", "hooks.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install configures Cursor to use agent-notify by setting up hooks.
func (c *CursorIntegration) Install(settingsPath, binaryPath string) error {
	return cursorhooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only the hook entries written by agent-notify.
func (c *CursorIntegration) Uninstall(settingsPath string) error {
	return cursorhooks.Uninstall(settingsPath)
}

// IsHookInstalled checks if agent-notify hooks are installed.
func (c *CursorIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return cursorhooks.IsInstalled(settingsPath)
}
