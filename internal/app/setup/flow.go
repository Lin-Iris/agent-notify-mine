package setup

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/config"
)

const (
	agentClaude       = "claude"
	agentCodex        = "codex"
	agentCodeBuddy    = "codebuddy"
	agentCursor       = "cursor"
	agentHermes       = "hermes"
	channelSystem     = "system"
	channelFeishu     = "feishu"
	channelWXWork     = "wechat-work"
	channelDingTalk   = "dingtalk"
	channelBark       = "bark"
	channelServerChan = "serverchan"
	channelPushPlus   = "pushplus"
	channelWxPusher   = "wxpusher"
	setupModeNotify   = "notification"
	setupModeRemote   = "remote_feishu"
	installScopeUsr   = "user"
	installScopePrj   = "project"
)

var channelOptions = []PromptOption{
	{Label: "系统通知", Value: channelSystem},
	{Label: "飞书", Value: channelFeishu},
	{Label: "企业微信", Value: channelWXWork},
	{Label: "钉钉", Value: channelDingTalk},
	{Label: "Bark", Value: channelBark},
	{Label: "Server酱", Value: channelServerChan},
	{Label: "PushPlus", Value: channelPushPlus},
	{Label: "WxPusher", Value: channelWxPusher},
}

type channelSelection struct {
	System     bool
	Feishu     bool
	WechatWork bool
	DingTalk   bool
	Bark       bool
	ServerChan bool
	PushPlus   bool
	WxPusher   bool
}

func (c channelSelection) hasAny() bool {
	return c.System || c.Feishu || c.WechatWork || c.DingTalk || c.Bark || c.ServerChan || c.PushPlus || c.WxPusher
}

type configureAgentRequest struct {
	ctx        context.Context
	prompter   Prompter
	output     OutputWriter
	cfg        config.Config
	agent      string
	channels   channelSelection
	events     []string
	binaryPath string
}

type configuredAgent struct {
	cfg          config.Config
	settingsPath string
}

func (s *Service) selectAgent(prompter Prompter, cfg config.Config) (string, error) {
	agentOptions, defaultAgent := s.agentOptions(cfg)
	if len(agentOptions) == 0 {
		return "", errors.New("未检测到 Claude Code 或 Codex，请先安装其中一个")
	}
	if defaultAgent == "" {
		defaultAgent = agentOptions[0].Value
	}
	return prompter.Select("选择要配置的 Agent", agentOptions, defaultAgent)
}

func isCLIInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (s *Service) agentOptions(cfg config.Config) ([]PromptOption, string) {
	var options []PromptOption
	var defaultAgent string

	if s.claudeIntegration.DetectInstalled() {
		label := "Claude Code"
		if !isCLIInstalled("claude") {
			label = "Claude Code (VS Code 扩展)"
		}
		options = append(options, PromptOption{Label: label, Value: agentClaude})
		if cfg.Agent.ClaudeCode.Enabled {
			defaultAgent = agentClaude
		}
	}

	if s.codexIntegration.DetectInstalled() {
		label := "Codex"
		if !isCLIInstalled("codex") {
			label = "Codex (Codex.app)"
		}
		options = append(options, PromptOption{Label: label, Value: agentCodex})
		if cfg.Agent.Codex.Enabled && defaultAgent == "" {
			defaultAgent = agentCodex
		}
	}

	if s.codebuddyIntegration.DetectInstalled() {
		label := "CodeBuddy"
		if !isCLIInstalled("codebuddy") {
			label = "CodeBuddy (IDE 扩展)"
		}
		options = append(options, PromptOption{Label: label, Value: agentCodeBuddy})
		if cfg.Agent.CodeBuddy.Enabled && defaultAgent == "" {
			defaultAgent = agentCodeBuddy
		}
	}

	if s.cursorIntegration.DetectInstalled() {
		label := "Cursor"
		if !isCLIInstalled("cursor") {
			label = "Cursor (IDE)"
		}
		options = append(options, PromptOption{Label: label, Value: agentCursor})
		if cfg.Agent.Cursor.Enabled && defaultAgent == "" {
			defaultAgent = agentCursor
		}
	}

	if s.hermesIntegration.DetectInstalled() {
		label := "Hermes"
		if !isCLIInstalled("hermes") {
			label = "Hermes (CLI)"
		}
		options = append(options, PromptOption{Label: label, Value: agentHermes})
		if cfg.Agent.Hermes.Enabled && defaultAgent == "" {
			defaultAgent = agentHermes
		}
	}

	return options, defaultAgent
}

func promptChannelSelection(prompter Prompter, channels config.ChannelsConfig) (channelSelection, error) {
	choices, err := prompter.MultiSelect("启用通知渠道", channelOptions, currentChannelValues(channels))
	if err != nil {
		return channelSelection{}, err
	}
	return channelSelectionFromChoices(choices), nil
}

func promptSetupMode(prompter Prompter, agent string) (string, error) {
	if !supportsRemoteFeishuConversation(agent) {
		return setupModeNotify, nil
	}
	return prompter.Select("选择配置类型", []PromptOption{
		{Label: "消息通知", Value: setupModeNotify},
		{Label: "远程飞书对话", Value: setupModeRemote},
	}, setupModeNotify)
}

func supportsRemoteFeishuConversation(agent string) bool {
	return agent == agentClaude || agent == agentCodex
}

func currentChannelValues(channels config.ChannelsConfig) []string {
	values := make([]string, 0, len(channelOptions))
	if channels.System.Enabled {
		values = append(values, channelSystem)
	}
	if channels.Feishu.Enabled {
		values = append(values, channelFeishu)
	}
	if channels.WechatWork.Enabled {
		values = append(values, channelWXWork)
	}
	if channels.DingTalk.Enabled {
		values = append(values, channelDingTalk)
	}
	if channels.Bark.Enabled {
		values = append(values, channelBark)
	}
	if channels.ServerChan.Enabled {
		values = append(values, channelServerChan)
	}
	if channels.PushPlus.Enabled {
		values = append(values, channelPushPlus)
	}
	if channels.WxPusher.Enabled {
		values = append(values, channelWxPusher)
	}
	return values
}

func channelSelectionFromChoices(choices []string) channelSelection {
	return channelSelection{
		System:     slices.Contains(choices, channelSystem),
		Feishu:     slices.Contains(choices, channelFeishu),
		WechatWork: slices.Contains(choices, channelWXWork),
		DingTalk:   slices.Contains(choices, channelDingTalk),
		Bark:       slices.Contains(choices, channelBark),
		ServerChan: slices.Contains(choices, channelServerChan),
		PushPlus:   slices.Contains(choices, channelPushPlus),
		WxPusher:   slices.Contains(choices, channelWxPusher),
	}
}

func promptEvents(prompter Prompter, agent string, currentEvents []string) ([]string, error) {
	return prompter.MultiSelect("通知事件", eventOptionsForAgent(agent), currentEvents)
}

func eventOptionsForAgent(agent string) []PromptOption {
	switch agent {
	case agentClaude:
		return claudeEventOptions
	case agentCodex:
		return codexEventOptions
	case agentCursor:
		return cursorEventOptions
	case agentHermes:
		return hermesEventOptions
	default:
		return codebuddyEventOptions
	}
}

func channelsForAgent(cfg config.Config, agent string) config.ChannelsConfig {
	switch agent {
	case agentClaude:
		return cfg.Notify.ClaudeCode.Channels
	case agentCodex:
		return cfg.Notify.Codex.Channels
	case agentCursor:
		return cfg.Notify.Cursor.Channels
	case agentHermes:
		return cfg.Notify.Hermes.Channels
	default:
		return cfg.Notify.CodeBuddy.Channels
	}
}

func eventsForAgent(cfg config.Config, agent string) []string {
	switch agent {
	case agentClaude:
		return cfg.Notify.ClaudeCode.Events
	case agentCodex:
		return cfg.Notify.Codex.Events
	case agentCursor:
		return cfg.Notify.Cursor.Events
	case agentHermes:
		return cfg.Notify.Hermes.Events
	default:
		return cfg.Notify.CodeBuddy.Events
	}
}

func (s *Service) configureAgent(req configureAgentRequest) (configuredAgent, error) {
	switch req.agent {
	case agentClaude:
		return s.configureClaude(req)
	case agentCodex:
		return s.configureCodex(req)
	case agentCodeBuddy:
		return s.configureCodeBuddy(req)
	case agentCursor:
		return s.configureCursor(req)
	case agentHermes:
		return s.configureHermes(req)
	default:
		return configuredAgent{}, fmt.Errorf("unsupported agent: %s", req.agent)
	}
}

func (s *Service) configureRemoteFeishuConversation(ctx context.Context, prompter Prompter, output OutputWriter, cfg config.Config, path, agent string) (*SetupResult, error) {
	profileName, err := remoteProfileForAgent(agent)
	if err != nil {
		return nil, err
	}

	// Check existing feishu config first
	configured, err := s.handleFeishuSetup(ctx, prompter, output, &cfg, agent)
	if err != nil {
		return nil, err
	}
	if !configured {
		output.Writef("跳过远程飞书对话配置\n")
		return &SetupResult{Agent: agent, ConfigPath: path}, nil
	}

	next := cfg
	if next.Profiles == nil {
		next.Profiles = config.ProfilesConfig{}
	}
	profile := next.Profiles[profileName]
	feishuProfile := profile.Feishu
	if !feishuProfile.HasCredentials() {
		output.Writef("为这个 Agent 配置远程飞书对话\n")
		feishuCfg, err := s.prepareFeishuConfig(ctx)
		if err != nil {
			return nil, err
		}
		if feishuCfg.AppID == "" || feishuCfg.AppSecret == "" || feishuCfg.UserOpenID == "" {
			return nil, errors.New("飞书扫码绑定没有返回完整配置")
		}
		feishuProfile = config.ProfileFeishuConfig{
			AppID:       feishuCfg.AppID,
			AppSecret:   feishuCfg.AppSecret,
			OwnerOpenID: feishuCfg.UserOpenID,
			ChatID:      profile.Feishu.ChatID,
		}
	} else {
		output.Writef("使用已有飞书配置继续\n")
	}
	profile.Agent = agent
	profile.Enabled = true
	if profile.PermissionMode == "" {
		profile.PermissionMode = "workspace-write"
	}
	if profile.Workspaces == nil {
		profile.Workspaces = map[string]string{}
	}
	profile.Feishu = feishuProfile
	next.Profiles[profileName] = profile
	next.Broker.ActiveProfile = profileName
	// Auto-enable feishu notification channel (approval uses same bot)
	setNotifyFeishuEnabled(&next, agent, true)
	if err := s.saveConfig(path, next); err != nil {
		return nil, err
	}

	output.Writef("已为这个 Agent 配置远程飞书对话: %s\n", agentName(agent))
	output.Writef("配置文件: %s\n", path)
	if profile.Workspace == "" {
		output.Writef("当前项目未设置。启动后请在这个 Agent 的飞书对话里发送 `/cd /具体/项目/目录` 后再发任务。\n")
	}
	startNow, err := prompter.Confirm("是否现在启动远程对话服务？", true)
	if err != nil {
		return nil, err
	}
	if startNow {
		if err := s.startBroker(ctx, next, path, profileName); err != nil {
			return nil, err
		}
		output.Writef("远程对话服务已启动\n")
	} else {
		output.Writef("稍后可从菜单或 broker start 启动远程对话服务\n")
	}
	return &SetupResult{Agent: agent, ConfigPath: path, SettingsPath: profileName}, nil
}

func remoteProfileForAgent(agent string) (string, error) {
	switch agent {
	case agentClaude:
		return "claude-main", nil
	case agentCodex:
		return "codex-main", nil
	default:
		return "", fmt.Errorf("unsupported remote Feishu conversation agent: %s", agent)
	}
}

func (s *Service) configureClaude(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.ClaudeCode.Channels = applyChannelSelection(next.Notify.ClaudeCode.Channels, req.channels)
	next.Notify.ClaudeCode.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	if req.channels.Feishu {
		s.syncFeishuToProfile(&next, req.agent)
	}
	channels, err := promptWebhookURLs(req.prompter, req.output, next.Notify.ClaudeCode.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.ClaudeCode.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.ClaudeCode.InstallScope)
	settingsPath, err := s.claudeIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("获取 claude settings 路径失败: %w", err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.claudeIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("安装 claude hooks 失败: %w", err)
	}
	req.output.Writef("claude hooks 安装: %s\n", settingsPath)
	next.Agent.ClaudeCode.InstallScope = agentScope
	next.Agent.ClaudeCode.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func (s *Service) configureCodex(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.Codex.Channels = applyChannelSelection(next.Notify.Codex.Channels, req.channels)
	next.Notify.Codex.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	if req.channels.Feishu {
		s.syncFeishuToProfile(&next, req.agent)
	}
	channels, err := promptWebhookURLs(req.prompter, req.output, next.Notify.Codex.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.Codex.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.Codex.InstallScope)
	settingsPath, err := s.codexIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("获取 codex hooks 路径失败: %w", err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.codexIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("安装 codex hooks 失败: %w", err)
	}
	req.output.Writef("codex hooks 安装: %s\n", settingsPath)
	req.output.Writef("提示: 请在 codex 内运行 /hooks 完成 trust 审核\n")
	next.Agent.Codex.InstallScope = agentScope
	next.Agent.Codex.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func (s *Service) configureCodeBuddy(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.CodeBuddy.Channels = applyChannelSelection(next.Notify.CodeBuddy.Channels, req.channels)
	next.Notify.CodeBuddy.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	if req.channels.Feishu {
		s.syncFeishuToProfile(&next, req.agent)
	}
	channels, err := promptWebhookURLs(req.prompter, req.output, next.Notify.CodeBuddy.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.CodeBuddy.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.CodeBuddy.InstallScope)
	settingsPath, err := s.codebuddyIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("获取 codebuddy settings 路径失败: %w", err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.codebuddyIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("安装 codebuddy hooks 失败: %w", err)
	}
	req.output.Writef("codebuddy hooks 安装: %s\n", settingsPath)
	next.Agent.CodeBuddy.InstallScope = agentScope
	next.Agent.CodeBuddy.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func (s *Service) configureCursor(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.Cursor.Channels = applyChannelSelection(next.Notify.Cursor.Channels, req.channels)
	next.Notify.Cursor.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	if req.channels.Feishu {
		s.syncFeishuToProfile(&next, req.agent)
	}
	channels, err := promptWebhookURLs(req.prompter, req.output, next.Notify.Cursor.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.Cursor.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.Cursor.InstallScope)
	settingsPath, err := s.cursorIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("获取 cursor settings 路径失败: %w", err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.cursorIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("安装 cursor hooks 失败: %w", err)
	}
	req.output.Writef("cursor hooks 安装: %s\n", settingsPath)
	next.Agent.Cursor.InstallScope = agentScope
	next.Agent.Cursor.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func (s *Service) configureHermes(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.Hermes.Channels = applyChannelSelection(next.Notify.Hermes.Channels, req.channels)
	next.Notify.Hermes.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	if req.channels.Feishu {
		s.syncFeishuToProfile(&next, req.agent)
	}
	channels, err := promptWebhookURLs(req.prompter, req.output, next.Notify.Hermes.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.Hermes.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.Hermes.InstallScope)
	settingsPath, err := s.hermesIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("获取 hermes settings 路径失败: %w", err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.hermesIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("安装 hermes hooks 失败: %w", err)
	}
	req.output.Writef("hermes hooks 安装: %s\n", settingsPath)
	next.Agent.Hermes.InstallScope = agentScope
	next.Agent.Hermes.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func applyChannelSelection(channels config.ChannelsConfig, selection channelSelection) config.ChannelsConfig {
	next := channels
	next.System.Enabled = selection.System
	next.Feishu.Enabled = selection.Feishu
	next.WechatWork.Enabled = selection.WechatWork
	next.DingTalk.Enabled = selection.DingTalk
	next.Bark.Enabled = selection.Bark
	next.ServerChan.Enabled = selection.ServerChan
	next.PushPlus.Enabled = selection.PushPlus
	next.WxPusher.Enabled = selection.WxPusher
	return next
}

func (s *Service) prepareSelectedChannels(ctx context.Context, selection channelSelection) error {
	if !selection.Feishu {
		return nil
	}
	if err := s.prepareFeishu(ctx); err != nil {
		return fmt.Errorf("飞书初始化失败: %w", err)
	}
	return nil
}

func promptWebhookURLs(
	prompter Prompter,
	output OutputWriter,
	channels config.ChannelsConfig,
	selection channelSelection,
) (config.ChannelsConfig, error) {
	next := channels
	if selection.WechatWork {
		webhookURL, err := prompter.Input("企业微信群机器人 Webhook URL", next.WechatWork.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.WechatWork.WebhookURL = webhookURL
	}
	if selection.DingTalk {
		webhookURL, err := prompter.Input("钉钉群机器人 Webhook URL", next.DingTalk.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.DingTalk.WebhookURL = webhookURL
	}
	if selection.Bark {
		webhookURL, err := prompter.Input("Bark Webhook URL", next.Bark.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.Bark.WebhookURL = webhookURL
	}
	if selection.ServerChan {
		output.Writef("📌 Server酱：打开 https://sct.ftqq.com/ 登录后，在「发送消息」页面拷贝 SendKey（SCU 开头）\n")
		sendKey, err := prompter.Input("Server酱 SendKey（SCU 开头）", next.ServerChan.SendKey)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.ServerChan.SendKey = sendKey
	}
	if selection.PushPlus {
		output.Writef("📌 PushPlus：打开 https://www.pushplus.plus/ 登录后，在「发送消息」→「一对一消息」页面拷贝 Token\n")
		token, err := prompter.Input("PushPlus Token", next.PushPlus.Token)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.PushPlus.Token = token
	}
	if selection.WxPusher {
		output.Writef("📌 WxPusher：打开 https://wxpusher.dingliqc.com/ 登录后，在「应用管理」→「应用信息」页面拷贝 AppToken，在「用户管理」页面扫码关注后获取 UID\n")
		appToken, err := prompter.Input("WxPusher AppToken", next.WxPusher.AppToken)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.WxPusher.AppToken = appToken
		uid, err := prompter.Input("WxPusher UID", next.WxPusher.UID)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.WxPusher.UID = uid
	}
	return next, nil
}

func normalizedInstallScope(scope string) string {
	if scope == installScopePrj {
		return installScopePrj
	}
	return installScopeUsr
}

// ── Feishu helpers ─────────────────────────────────────────

// setNotifyFeishuEnabled enables or disables the feishu notification channel for an agent.
func setNotifyFeishuEnabled(cfg *config.Config, agent string, enabled bool) {
	switch agent {
	case agentClaude:
		cfg.Notify.ClaudeCode.Channels.Feishu.Enabled = enabled
	case agentCodex:
		cfg.Notify.Codex.Channels.Feishu.Enabled = enabled
	case agentCodeBuddy:
		cfg.Notify.CodeBuddy.Channels.Feishu.Enabled = enabled
	case agentCursor:
		cfg.Notify.Cursor.Channels.Feishu.Enabled = enabled
	case agentHermes:
		cfg.Notify.Hermes.Channels.Feishu.Enabled = enabled
	}
}

// profileFeishuCredentials returns feishu credentials from the agent's profile.
func profileFeishuCredentials(cfg config.Config, agent string) config.ProfileFeishuConfig {
	profileName, err := remoteProfileForAgent(agent)
	if err != nil {
		return config.ProfileFeishuConfig{}
	}
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return config.ProfileFeishuConfig{}
	}
	return profile.Feishu
}

// syncFeishuToProfile reads feishu credentials from client-tools CLI and writes them
// into the agent's profile. Skips silently if already configured or unavailable.
func (s *Service) syncFeishuToProfile(cfg *config.Config, agent string) {
	if cfg.Profiles == nil {
		cfg.Profiles = config.ProfilesConfig{}
	}
	creds := profileFeishuCredentials(*cfg, agent)
	if creds.HasCredentials() {
		return // already configured, keep existing
	}

	feishuCfg, err := s.prepareFeishuConfig(context.Background())
	if err != nil || feishuCfg.AppID == "" || feishuCfg.AppSecret == "" || feishuCfg.UserOpenID == "" {
		return // can't read, skip silently
	}

	profileName, err := remoteProfileForAgent(agent)
	if err != nil || profileName == "" {
		return
	}
	profile := cfg.Profiles[profileName]
	profile.Feishu = config.ProfileFeishuConfig{
		AppID:       feishuCfg.AppID,
		AppSecret:   feishuCfg.AppSecret,
		OwnerOpenID: feishuCfg.UserOpenID,
	}
	cfg.Profiles[profileName] = profile
}

// handleFeishuSetup checks if feishu is already configured for the agent.
// If yes, offers keep/replace/remove. Runs QR scan if needed.
// Returns false if user chose to remove or skip.
func (s *Service) handleFeishuSetup(ctx context.Context, prompter Prompter, output OutputWriter, cfg *config.Config, agent string) (bool, error) {
	creds := profileFeishuCredentials(*cfg, agent)
	if creds.HasCredentials() {
		choice, err := prompter.Select("飞书当前已配置", []PromptOption{
			{Label: "保持配置，继续 ✅", Value: "keep"},
			{Label: "重新配置（覆盖）", Value: "replace"},
			{Label: "移除飞书配置", Value: "remove"},
		}, "keep")
		if err != nil {
			return false, err
		}
		switch choice {
		case "keep":
			return true, nil
		case "remove":
			profileName, _ := remoteProfileForAgent(agent)
			if profileName != "" && cfg.Profiles != nil {
				profile := cfg.Profiles[profileName]
				profile.Feishu = config.ProfileFeishuConfig{}
				cfg.Profiles[profileName] = profile
			}
			setNotifyFeishuEnabled(cfg, agent, false)
			output.Writef("飞书配置已移除\n")
			return false, nil
		case "replace":
			// 清空旧配置，强制重新扫码
			profileName, _ := remoteProfileForAgent(agent)
			if profileName != "" && cfg.Profiles != nil {
				p := cfg.Profiles[profileName]
				p.Feishu = config.ProfileFeishuConfig{}
				cfg.Profiles[profileName] = p
			}
			setNotifyFeishuEnabled(cfg, agent, false)
			output.Writef("已清除旧配置，将重新扫码\n")
			// fall through to QR scan
		}
	}
	return true, nil // proceed with QR scan
}
