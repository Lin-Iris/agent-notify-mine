package setup

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

// mockIntegration implements agentintegrations.Integration for testing
type mockIntegration struct {
	name            string
	detectInstalled bool
	settingsPath    string
	installErr      error
	isHookInstalled bool
}

func (m *mockIntegration) Name() string {
	return m.name
}

func (m *mockIntegration) DetectInstalled() bool {
	return m.detectInstalled
}

func (m *mockIntegration) SettingsPath(scope string) (string, error) {
	return m.settingsPath, nil
}

func (m *mockIntegration) Install(settingsPath, binaryPath string) error {
	return m.installErr
}

func (m *mockIntegration) Uninstall(settingsPath string) error {
	return nil
}

func (m *mockIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return m.isHookInstalled, nil
}

// mockPrompter implements Prompter for testing
type mockPrompter struct {
	selectIdx      int
	selectResult   string
	selectResults  []string
	selectMessages []string
	selectOptions  [][]PromptOption
	multiResult    []string
	multiResults   [][]string
	multiOptions   [][]PromptOption
	confirmResult  bool
	confirmCalled  bool
	inputResult    string
	inputResults   []string
}

func (m *mockPrompter) Select(message string, options []PromptOption, defaultValue string) (string, error) {
	m.selectMessages = append(m.selectMessages, message)
	m.selectOptions = append(m.selectOptions, options)
	if len(m.selectResults) > 0 {
		value := m.selectResults[0]
		m.selectResults = m.selectResults[1:]
		return value, nil
	}
	return m.selectResult, nil
}

func (m *mockPrompter) MultiSelect(message string, options []PromptOption, defaults []string) ([]string, error) {
	m.multiOptions = append(m.multiOptions, options)
	if len(m.multiResults) > 0 {
		value := m.multiResults[0]
		m.multiResults = m.multiResults[1:]
		return value, nil
	}
	return m.multiResult, nil
}

func (m *mockPrompter) Confirm(message string, defaultValue bool) (bool, error) {
	m.confirmCalled = true
	return m.confirmResult, nil
}

func (m *mockPrompter) Input(message, defaultValue string) (string, error) {
	if len(m.inputResults) > 0 {
		value := m.inputResults[0]
		m.inputResults = m.inputResults[1:]
		return value, nil
	}
	return m.inputResult, nil
}

// mockOutputWriter implements OutputWriter for testing
type mockOutputWriter struct {
	output string
}

func (m *mockOutputWriter) Writef(format string, args ...any) {
	m.output += fmt.Sprintf(format, args...)
}

// mockFeishuPreparer implements FeishuPreparer for testing
type mockFeishuPreparer struct {
	called  bool
	prepare bool
	err     error
	cfg     FeishuConfig
}

func (m *mockFeishuPreparer) EnsureReady(ctx context.Context) error {
	m.called = true
	return m.err
}

func (m *mockFeishuPreparer) Prepare(ctx context.Context) (FeishuConfig, error) {
	m.prepare = true
	return m.cfg, m.err
}

type mockConfigLoader struct {
	defaultPath string
	loadedPath  string
	savedPath   string
	loadedCfg   config.Config
	savedCfg    config.Config
}

type mockBrokerStarter struct {
	called     bool
	profile    string
	configPath string
	cfg        config.Config
	err        error
}

func (m *mockBrokerStarter) Start(ctx context.Context, cfg config.Config, configPath, profile string) error {
	m.called = true
	m.profile = profile
	m.configPath = configPath
	m.cfg = cfg
	return m.err
}

func (m *mockConfigLoader) Load(path string) (config.Config, error) {
	m.loadedPath = path
	return m.loadedCfg, nil
}

func (m *mockConfigLoader) Save(path string, cfg config.Config) error {
	m.savedPath = path
	m.savedCfg = cfg
	return nil
}

func (m *mockConfigLoader) DefaultPath() (string, error) {
	return m.defaultPath, nil
}

func TestService_Name(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_NoAgentsDetected(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithCodeBuddyIntegration(&mockIntegration{name: "CodeBuddy", detectInstalled: false}),
	)

	prompter := &mockPrompter{}
	output := &mockOutputWriter{}

	_, err := svc.Run(context.Background(), prompter, output, "", "")
	if err == nil {
		t.Fatal("expected error when no agents detected")
	}
}

func TestService_ClaudeIntegration(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{
			name:            "Claude Code",
			detectInstalled: true,
			settingsPath:    "/tmp/.claude/settings.json",
			isHookInstalled: true,
		}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithFeishuPreparer(&mockFeishuPreparer{}),
	)

	prompter := &mockPrompter{
		selectResults: []string{"claude", setupModeNotify},
		multiResult:   []string{"feishu", "system"},
	}
	output := &mockOutputWriter{}

	// Create a temp config path
	result, err := svc.Run(context.Background(), prompter, output, "/tmp/test-config.yaml", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %q", result.Agent)
	}
}

func TestService_CodexIntegration(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{
			name:            "Codex",
			detectInstalled: true,
			settingsPath:    "/tmp/.codex/config.toml",
			isHookInstalled: true,
		}),
		WithFeishuPreparer(&mockFeishuPreparer{}),
	)

	prompter := &mockPrompter{
		selectResults: []string{"codex", setupModeNotify},
		multiResult:   []string{"feishu", "system"},
	}
	output := &mockOutputWriter{}

	result, err := svc.Run(context.Background(), prompter, output, "/tmp/test-config.yaml", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Agent != "codex" {
		t.Errorf("expected agent 'codex', got %q", result.Agent)
	}
}

func TestService_UsesInjectedConfigLoader(t *testing.T) {
	loader := &mockConfigLoader{
		defaultPath: "/tmp/injected-config.yaml",
		loadedCfg:   config.Default(),
	}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: true, settingsPath: "/tmp/.claude/settings.json"}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithConfigLoader(loader),
	)
	prompter := &mockPrompter{selectResults: []string{"claude", setupModeNotify}, multiResult: []string{"system"}}
	output := &mockOutputWriter{}

	_, err := svc.Run(context.Background(), prompter, output, "", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if loader.loadedPath != "/tmp/injected-config.yaml" {
		t.Fatalf("loadedPath = %q, want %q", loader.loadedPath, "/tmp/injected-config.yaml")
	}
	if loader.savedPath != "/tmp/injected-config.yaml" {
		t.Fatalf("savedPath = %q, want %q", loader.savedPath, "/tmp/injected-config.yaml")
	}
}

func TestService_RemoteFeishuConversationSavesClaudeProfile(t *testing.T) {
	loader := &mockConfigLoader{
		defaultPath: "/tmp/injected-config.yaml",
		loadedCfg:   config.Default(),
	}
	feishu := &mockFeishuPreparer{cfg: FeishuConfig{
		AppID:      "scan_app",
		AppSecret:  "scan_secret",
		UserOpenID: "ou_owner",
		UserName:   "Victoria",
	}}
	broker := &mockBrokerStarter{}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: true, settingsPath: "/tmp/.claude/settings.json"}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithFeishuPreparer(feishu),
		WithBrokerStarter(broker),
		WithConfigLoader(loader),
	)
	prompter := &mockPrompter{
		selectResults: []string{"claude", setupModeRemote},
		confirmResult: true,
	}
	output := &mockOutputWriter{}

	result, err := svc.Run(context.Background(), prompter, output, "", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Agent != "claude" {
		t.Fatalf("agent = %q, want claude", result.Agent)
	}
	if !feishu.prepare {
		t.Fatal("remote setup should call Feishu Prepare")
	}
	profile := loader.savedCfg.Profiles["claude-main"]
	if profile.Agent != "claude" || !profile.Enabled {
		t.Fatalf("profile = %#v, want enabled claude profile", profile)
	}
	if profile.Feishu.AppID != "scan_app" || profile.Feishu.AppSecret != "scan_secret" || profile.Feishu.OwnerOpenID != "ou_owner" {
		t.Fatalf("profile feishu = %#v, want scanned config", profile.Feishu)
	}
	if broker.profile != "claude-main" || !broker.called {
		t.Fatalf("broker start = called %v profile %q, want claude-main", broker.called, broker.profile)
	}
	if strings.Contains(output.output, "scan_secret") {
		t.Fatalf("output leaked app secret: %q", output.output)
	}
}

func TestService_RemoteFeishuConversationCanSkipBrokerStart(t *testing.T) {
	loader := &mockConfigLoader{
		defaultPath: "/tmp/injected-config.yaml",
		loadedCfg:   config.Default(),
	}
	broker := &mockBrokerStarter{}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: true, settingsPath: "/tmp/.codex/config.toml"}),
		WithFeishuPreparer(&mockFeishuPreparer{cfg: FeishuConfig{
			AppID:      "codex_app",
			AppSecret:  "codex_secret",
			UserOpenID: "ou_codex",
		}}),
		WithBrokerStarter(broker),
		WithConfigLoader(loader),
	)
	prompter := &mockPrompter{
		selectResults: []string{"codex", setupModeRemote},
		confirmResult: false,
	}

	if _, err := svc.Run(context.Background(), prompter, &mockOutputWriter{}, "", "/tmp/agent-notify"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	profile := loader.savedCfg.Profiles["codex-main"]
	if profile.Agent != "codex" || profile.Feishu.AppID != "codex_app" {
		t.Fatalf("codex profile = %#v, want scanned codex config", profile)
	}
	if broker.called {
		t.Fatal("broker should not start when user declines")
	}
}

func TestService_RemoteFeishuConversationKeepExistingFeishuSkipsQR(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles = config.ProfilesConfig{
		"codex-main": {
			Agent:          "codex",
			Enabled:        false,
			Workspace:      "/tmp/project",
			PermissionMode: "workspace-write",
			Feishu: config.ProfileFeishuConfig{
				AppID:       "existing_app",
				AppSecret:   "existing_secret",
				OwnerOpenID: "ou_existing",
				ChatID:      "oc_existing",
			},
		},
	}
	loader := &mockConfigLoader{
		defaultPath: "/tmp/injected-config.yaml",
		loadedCfg:   cfg,
	}
	feishu := &mockFeishuPreparer{cfg: FeishuConfig{
		AppID:      "scan_app",
		AppSecret:  "scan_secret",
		UserOpenID: "ou_scan",
	}}
	broker := &mockBrokerStarter{}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: true, settingsPath: "/tmp/.codex/config.toml"}),
		WithFeishuPreparer(feishu),
		WithBrokerStarter(broker),
		WithConfigLoader(loader),
	)
	prompter := &mockPrompter{
		selectResults: []string{"codex", setupModeRemote, "keep"},
		confirmResult: false,
	}

	if _, err := svc.Run(context.Background(), prompter, &mockOutputWriter{}, "", "/tmp/agent-notify"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if feishu.prepare {
		t.Fatal("keep existing feishu config should not trigger QR setup")
	}
	if !prompter.confirmCalled {
		t.Fatal("keep existing feishu config should still ask whether to start broker")
	}
	profile := loader.savedCfg.Profiles["codex-main"]
	if !profile.Enabled {
		t.Fatal("profile should be enabled")
	}
	if profile.Feishu.AppID != "existing_app" || profile.Feishu.AppSecret != "existing_secret" || profile.Feishu.OwnerOpenID != "ou_existing" || profile.Feishu.ChatID != "oc_existing" {
		t.Fatalf("profile feishu = %#v, want existing config", profile.Feishu)
	}
	if loader.savedCfg.Broker.ActiveProfile != "codex-main" {
		t.Fatalf("active profile = %q, want codex-main", loader.savedCfg.Broker.ActiveProfile)
	}
	if !loader.savedCfg.Notify.Codex.Channels.Feishu.Enabled {
		t.Fatal("codex feishu notification channel should be enabled")
	}
	if broker.called {
		t.Fatal("broker should not start when user declines")
	}
}

func TestService_RemoteFeishuOptionHiddenForUnsupportedAgent(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithCodeBuddyIntegration(&mockIntegration{name: "CodeBuddy", detectInstalled: true, settingsPath: "/tmp/codebuddy"}),
	)
	prompter := &mockPrompter{
		selectResult: "codebuddy",
		multiResults: [][]string{
			{"system"},
			{"run_completed"},
		},
	}

	if _, err := svc.Run(context.Background(), prompter, &mockOutputWriter{}, "/tmp/test-config.yaml", "/tmp/agent-notify"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, msg := range prompter.selectMessages {
		if msg == "选择配置类型" {
			t.Fatal("unsupported remote agent should not show setup mode selection")
		}
	}
}

func TestDedupeStrings(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{[]string{"a", "b", "a", "c"}, []string{"a", "b", "c"}},
		{[]string{}, []string{}},
		{[]string{"a"}, []string{"a"}},
		{[]string{"a", "a", "a"}, []string{"a"}},
	}

	for _, tt := range tests {
		result := dedupeStrings(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("dedupeStrings(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
