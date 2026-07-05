// Package setup provides the setup/init flow service for agent-notify.
// It handles agent configuration and hook installation.
package setup

import (
	"context"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/config"
)

// Prompter interface for user interactions.
type Prompter interface {
	Select(message string, options []PromptOption, defaultValue string) (string, error)
	MultiSelect(message string, options []PromptOption, defaults []string) ([]string, error)
	Confirm(message string, defaultValue bool) (bool, error)
	Input(message, defaultValue string) (string, error)
}

// PromptOption represents a selectable option in prompts.
type PromptOption struct {
	Label string
	Value string
}

// FeishuConfig is the Feishu app/user config returned by QR setup.
type FeishuConfig struct {
	AppID      string
	AppSecret  string
	UserOpenID string
	UserName   string
}

// FeishuPreparer prepares the Feishu CLI for use.
type FeishuPreparer interface {
	EnsureReady(ctx context.Context) error
	Prepare(ctx context.Context) (FeishuConfig, error)
}

// BrokerStarter starts the remote conversation broker for a profile.
type BrokerStarter interface {
	Start(ctx context.Context, cfg config.Config, configPath, profile string) error
}

// OutputWriter handles output messages.
type OutputWriter interface {
	Writef(format string, args ...any)
}

// Service handles the init/setup flow for agent-notify.
type Service struct {
	claudeIntegration    agentintegrations.Integration
	codexIntegration     agentintegrations.Integration
	codebuddyIntegration agentintegrations.Integration
	cursorIntegration    agentintegrations.Integration
	hermesIntegration    agentintegrations.Integration
	feishuPreparer       FeishuPreparer
	brokerStarter        BrokerStarter
	configLoader         ConfigLoader
}

// ConfigLoader loads and saves configuration.
type ConfigLoader interface {
	Load(path string) (config.Config, error)
	Save(path string, cfg config.Config) error
	DefaultPath() (string, error)
}

// SetupResult contains the result of a setup operation.
type SetupResult struct {
	Agent        string
	ConfigPath   string
	SettingsPath string
}

// NewService creates a new setup service.
func NewService(opts ...Option) *Service {
	s := &Service{
		claudeIntegration:    agentintegrations.NewClaudeIntegration(),
		codexIntegration:     agentintegrations.NewCodexIntegration(),
		codebuddyIntegration: agentintegrations.NewCodeBuddyIntegration(),
		cursorIntegration:    agentintegrations.NewCursorIntegration(),
		hermesIntegration:    agentintegrations.NewHermesIntegration(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Option configures the service.
type Option func(*Service)

// WithClaudeIntegration sets the Claude integration.
func WithClaudeIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.claudeIntegration = i }
}

// WithCodexIntegration sets the Codex integration.
func WithCodexIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.codexIntegration = i }
}

// WithCodeBuddyIntegration sets the CodeBuddy integration.
func WithCodeBuddyIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.codebuddyIntegration = i }
}

// WithCursorIntegration sets the Cursor integration.
func WithCursorIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.cursorIntegration = i }
}

// WithHermesIntegration sets the Hermes integration.
func WithHermesIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.hermesIntegration = i }
}

// WithFeishuPreparer sets the Feishu preparer.
func WithFeishuPreparer(p FeishuPreparer) Option {
	return func(s *Service) { s.feishuPreparer = p }
}

// WithBrokerStarter sets the remote conversation broker starter.
func WithBrokerStarter(b BrokerStarter) Option {
	return func(s *Service) { s.brokerStarter = b }
}

// WithConfigLoader sets the config loader.
func WithConfigLoader(l ConfigLoader) Option {
	return func(s *Service) { s.configLoader = l }
}

var claudeEventOptions = []PromptOption{
	{Label: "需要授权 (permission_required)", Value: "permission_required"},
	{Label: "等待输入 (input_required)", Value: "input_required"},
	{Label: "任务完成 (run_completed)", Value: "run_completed"},
	{Label: "任务失败 (run_failed)", Value: "run_failed"},
}

// codexEventOptions 仅包含 Codex hooks 当前可靠支持的事件：
// PermissionRequest → permission_required，Stop → run_completed。
// input_required / run_failed 目前 Codex 没有对应 hook，故不暴露给用户。
var codexEventOptions = []PromptOption{
	{Label: "需要授权 (permission_required)", Value: "permission_required"},
	{Label: "任务完成 (run_completed)", Value: "run_completed"},
}

var codebuddyEventOptions = []PromptOption{
	{Label: "需要授权 (permission_required)", Value: "permission_required"},
	{Label: "等待输入 (input_required)", Value: "input_required"},
	{Label: "任务完成 (run_completed)", Value: "run_completed"},
	{Label: "任务失败 (run_failed)", Value: "run_failed"},
}

// cursorEventOptions 包含 Cursor hooks 当前支持的事件：
// beforeShellExecution → permission_required（注意：对所有 shell 命令触发，不止授权操作）
// stop (status=completed) → run_completed
// postToolUseFailure → run_failed
// input_required Cursor 目前没有对应 hook。
var cursorEventOptions = []PromptOption{
	{Label: "需要授权 (permission_required)", Value: "permission_required"},
	{Label: "任务完成 (run_completed)", Value: "run_completed"},
	{Label: "任务失败 (run_failed)", Value: "run_failed"},
}

// hermesEventOptions 包含 Hermes shell hooks 当前支持的事件：
// pre_approval_request → permission_required
// post_llm_call → run_completed
// run_failed / input_required Hermes 目前没有对应 hook。
var hermesEventOptions = []PromptOption{
	{Label: "需要授权 (permission_required)", Value: "permission_required"},
	{Label: "任务完成 (run_completed)", Value: "run_completed"},
}

// Run executes the init flow.
func (s *Service) Run(ctx context.Context, prompter Prompter, output OutputWriter, configPath, binaryPath string) (*SetupResult, error) {
	cfg, path, err := s.loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	selectedAgent, err := s.selectAgent(prompter, cfg)
	if err != nil {
		return nil, err
	}

	mode, err := promptSetupMode(prompter, selectedAgent)
	if err != nil {
		return nil, err
	}
	if mode == setupModeRemote {
		return s.configureRemoteFeishuConversation(ctx, prompter, output, cfg, path, selectedAgent)
	}

	channels, err := promptChannelSelection(prompter, channelsForAgent(cfg, selectedAgent))
	if err != nil {
		return nil, err
	}
	if !channels.hasAny() {
		return s.disableAgentNotification(cfg, path, selectedAgent, output)
	}

	events, err := promptEvents(prompter, selectedAgent, eventsForAgent(cfg, selectedAgent))
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return s.disableAgentNotification(cfg, path, selectedAgent, output)
	}

	configured, err := s.configureAgent(configureAgentRequest{
		ctx: ctx, prompter: prompter, output: output, cfg: cfg,
		agent: selectedAgent, channels: channels, events: events, binaryPath: binaryPath,
	})
	if err != nil {
		return nil, err
	}
	if err := s.saveConfig(path, configured.cfg); err != nil {
		return nil, err
	}
	output.Writef("配置文件: %s\n", path)

	return &SetupResult{Agent: selectedAgent, ConfigPath: path, SettingsPath: configured.settingsPath}, nil
}

func (s *Service) loadConfig(configPath string) (config.Config, string, error) {
	path := configPath
	var err error
	if path == "" {
		path, err = s.defaultConfigPath()
		if err != nil {
			return config.Config{}, "", err
		}
	}
	cfg, err := s.loadConfigFile(path)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, path, nil
}

func (s *Service) saveConfig(path string, cfg config.Config) error {
	if s.configLoader != nil {
		return s.configLoader.Save(path, cfg)
	}
	return config.Save(path, cfg)
}

func (s *Service) defaultConfigPath() (string, error) {
	if s.configLoader != nil {
		return s.configLoader.DefaultPath()
	}
	return config.DefaultPath()
}

func (s *Service) loadConfigFile(path string) (config.Config, error) {
	if s.configLoader != nil {
		return s.configLoader.Load(path)
	}
	return config.Load(path)
}

func (s *Service) prepareFeishu(ctx context.Context) error {
	if s.feishuPreparer != nil {
		return s.feishuPreparer.EnsureReady(ctx)
	}
	return nil
}

func (s *Service) prepareFeishuConfig(ctx context.Context) (FeishuConfig, error) {
	if s.feishuPreparer != nil {
		return s.feishuPreparer.Prepare(ctx)
	}
	return FeishuConfig{}, nil
}

func (s *Service) startBroker(ctx context.Context, cfg config.Config, path, profile string) error {
	if s.brokerStarter == nil {
		return nil
	}
	return s.brokerStarter.Start(ctx, cfg, path, profile)
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// disableAgentNotification disables all notification channels for the given agent
// and saves the configuration. This is called when the user doesn't select any
// channels or events.
func (s *Service) disableAgentNotification(cfg config.Config, path, agent string, output OutputWriter) (*SetupResult, error) {
	switch agent {
	case "claude":
		cfg.Notify.ClaudeCode.Channels.Feishu.Enabled = false
		cfg.Notify.ClaudeCode.Channels.System.Enabled = false
		cfg.Notify.ClaudeCode.Channels.WechatWork.Enabled = false
		cfg.Notify.ClaudeCode.Channels.DingTalk.Enabled = false
		cfg.Notify.ClaudeCode.Channels.Bark.Enabled = false
		cfg.Notify.ClaudeCode.Events = nil
		cfg.Agent.ClaudeCode.Enabled = false
	case "codex":
		cfg.Notify.Codex.Channels.Feishu.Enabled = false
		cfg.Notify.Codex.Channels.System.Enabled = false
		cfg.Notify.Codex.Channels.WechatWork.Enabled = false
		cfg.Notify.Codex.Channels.DingTalk.Enabled = false
		cfg.Notify.Codex.Channels.Bark.Enabled = false
		cfg.Notify.Codex.Events = nil
		cfg.Agent.Codex.Enabled = false
	case "codebuddy":
		cfg.Notify.CodeBuddy.Channels.Feishu.Enabled = false
		cfg.Notify.CodeBuddy.Channels.System.Enabled = false
		cfg.Notify.CodeBuddy.Channels.WechatWork.Enabled = false
		cfg.Notify.CodeBuddy.Channels.DingTalk.Enabled = false
		cfg.Notify.CodeBuddy.Channels.Bark.Enabled = false
		cfg.Notify.CodeBuddy.Events = nil
		cfg.Agent.CodeBuddy.Enabled = false
	case "cursor":
		cfg.Notify.Cursor.Channels.Feishu.Enabled = false
		cfg.Notify.Cursor.Channels.System.Enabled = false
		cfg.Notify.Cursor.Channels.WechatWork.Enabled = false
		cfg.Notify.Cursor.Channels.DingTalk.Enabled = false
		cfg.Notify.Cursor.Channels.Bark.Enabled = false
		cfg.Notify.Cursor.Events = nil
		cfg.Agent.Cursor.Enabled = false
	case "hermes":
		cfg.Notify.Hermes.Channels.Feishu.Enabled = false
		cfg.Notify.Hermes.Channels.System.Enabled = false
		cfg.Notify.Hermes.Channels.WechatWork.Enabled = false
		cfg.Notify.Hermes.Channels.DingTalk.Enabled = false
		cfg.Notify.Hermes.Channels.Bark.Enabled = false
		cfg.Notify.Hermes.Events = nil
		cfg.Agent.Hermes.Enabled = false
	}

	if err := s.saveConfig(path, cfg); err != nil {
		return nil, err
	}
	output.Writef("%s 通知已关闭\n", agentName(agent))
	output.Writef("配置文件: %s\n", path)

	return &SetupResult{
		Agent:      agent,
		ConfigPath: path,
	}, nil
}

func agentName(agent string) string {
	switch agent {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "codebuddy":
		return "CodeBuddy"
	case "cursor":
		return "Cursor"
	case "hermes":
		return "Hermes"
	default:
		return agent
	}
}
