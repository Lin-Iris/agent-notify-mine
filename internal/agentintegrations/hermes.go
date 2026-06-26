package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/hermeshooks"
)

// HermesIntegration implements Integration for Hermes Agent.
type HermesIntegration struct{}

// NewHermesIntegration creates a new Hermes integration.
func NewHermesIntegration() *HermesIntegration {
	return &HermesIntegration{}
}

// Name returns the display name for Hermes.
func (h *HermesIntegration) Name() string {
	return "Hermes"
}

// DetectInstalled checks if Hermes Agent is available.
func (h *HermesIntegration) DetectInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// 1. ~/.hermes/ directory exists
	if _, err := os.Stat(filepath.Join(home, ".hermes")); err == nil {
		return true
	}

	// 2. hermes CLI in PATH
	if _, err := exec.LookPath("hermes"); err == nil {
		return true
	}

	return false
}

// SettingsPath returns the path to Hermes's config.yaml file.
// Hermes only supports user scope; project scope returns an error.
func (h *HermesIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".hermes", "config.yaml"), nil
	case "project":
		return "", fmt.Errorf("hermes does not support project-level hooks")
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install configures Hermes to use agent-notify by setting up shell hooks.
func (h *HermesIntegration) Install(settingsPath, binaryPath string) error {
	return hermeshooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only the hook entries written by agent-notify.
func (h *HermesIntegration) Uninstall(settingsPath string) error {
	return hermeshooks.Uninstall(settingsPath)
}

// IsHookInstalled checks if agent-notify hooks are installed.
func (h *HermesIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return hermeshooks.IsInstalled(settingsPath)
}
