package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for agent-notify.
type Config struct {
	Version  int            `yaml:"version"`  // 配置版本号
	Agent    AgentConfig    `yaml:"agent"`    // Agent 安装配置
	Notify   NotifyConfig   `yaml:"notify"`   // 通知配置
	Behavior BehaviorConfig `yaml:"behavior"` // 行为配置
	Broker   BrokerConfig   `yaml:"broker,omitempty"`
	Approval ApprovalConfig `yaml:"approval,omitempty"`
	Profiles ProfilesConfig `yaml:"profiles,omitempty"`
}

// AgentConfig holds configuration for supported agents.
type AgentConfig struct {
	ClaudeCode AgentTargetConfig `yaml:"claude_code"` // Claude Code 配置
	Codex      AgentTargetConfig `yaml:"codex"`       // Codex 配置
	CodeBuddy  AgentTargetConfig `yaml:"codebuddy"`   // CodeBuddy 配置
	Cursor     AgentTargetConfig `yaml:"cursor"`      // Cursor 配置
	Hermes     AgentTargetConfig `yaml:"hermes"`      // Hermes 配置
}

// AgentTargetConfig holds configuration for a specific agent.
type AgentTargetConfig struct {
	Enabled      bool   `yaml:"enabled"`       // 是否启用该 Agent 的通知
	InstallScope string `yaml:"install_scope"` // 安装范围: user 或 project
}

// NotifyConfig holds notification configuration for all agents.
type NotifyConfig struct {
	ClaudeCode AgentNotifyConfig `yaml:"claude_code"` // Claude Code 通知配置
	Codex      AgentNotifyConfig `yaml:"codex"`       // Codex 通知配置
	CodeBuddy  AgentNotifyConfig `yaml:"codebuddy"`   // CodeBuddy 通知配置
	Cursor     AgentNotifyConfig `yaml:"cursor"`      // Cursor 通知配置
	Hermes     AgentNotifyConfig `yaml:"hermes"`      // Hermes 通知配置
}

// AgentNotifyConfig holds notification configuration for a single agent.
type AgentNotifyConfig struct {
	Events   []string       `yaml:"events,omitempty"` // 通知事件列表，如: permission_required, input_required, run_completed, run_failed
	Channels ChannelsConfig `yaml:"channels"`         // 通知渠道配置
}

// ChannelsConfig holds configuration for notification channels.
type ChannelsConfig struct {
	Feishu     ChannelConfig           `yaml:"feishu"`      // 飞书通知配置
	System     ChannelConfig           `yaml:"system"`      // 系统通知配置
	WechatWork WechatWorkChannelConfig `yaml:"wechat_work"` // 企业微信通知配置
	DingTalk   DingTalkChannelConfig   `yaml:"dingtalk"`    // 钉钉通知配置
	Bark       BarkChannelConfig       `yaml:"bark"`        // Bark 通知配置
	ServerChan ServerChanChannelConfig `yaml:"serverchan"`  // Server酱 通知配置
	PushPlus   PushPlusChannelConfig   `yaml:"pushplus"`    // PushPlus 通知配置
	WxPusher   WxPusherChannelConfig   `yaml:"wxpusher"`    // WxPusher 通知配置
}

// ChannelConfig holds configuration for a single notification channel.
type ChannelConfig struct {
	Enabled bool `yaml:"enabled"` // 是否启用该通知渠道
}

// WechatWorkChannelConfig holds configuration for WeChat Work (企业微信) webhook notifications.
type WechatWorkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用企业微信通知
	WebhookURL string `yaml:"webhook_url"` // 群机器人 Webhook URL
}

// DingTalkChannelConfig holds configuration for DingTalk (钉钉) webhook notifications.
type DingTalkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用钉钉通知
	WebhookURL string `yaml:"webhook_url"` // 群机器人 Webhook URL
}

// BarkChannelConfig holds configuration for Bark webhook notifications.
type BarkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用 Bark 通知
	WebhookURL string `yaml:"webhook_url"` // Bark 推送 URL
}

// ServerChanChannelConfig holds configuration for ServerChan (Server酱) WeChat push notifications.
type ServerChanChannelConfig struct {
	Enabled bool   `yaml:"enabled"`  // 是否启用 Server酱 通知
	SendKey string `yaml:"send_key"` // Server酱 SendKey (SCU 开头)
}

// PushPlusChannelConfig holds configuration for PushPlus (推送加) WeChat push notifications.
type PushPlusChannelConfig struct {
	Enabled bool   `yaml:"enabled"` // 是否启用 PushPlus 通知
	Token   string `yaml:"token"`   // PushPlus Token
}

// WxPusherChannelConfig holds configuration for WxPusher WeChat push notifications.
type WxPusherChannelConfig struct {
	Enabled  bool   `yaml:"enabled"`   // 是否启用 WxPusher 通知
	AppToken string `yaml:"app_token"` // WxPusher AppToken
	UID      string `yaml:"uid"`       // WxPusher 用户 UID
}

// BehaviorConfig holds behavior configuration.
type BehaviorConfig struct {
	DedupeSeconds      int    `yaml:"dedupe_seconds"`       // 去重时间窗口（秒），同一事件在此时间内不重复发送
	SendTimeoutSeconds int    `yaml:"send_timeout_seconds"` // 发送超时时间（秒）
	Locale             string `yaml:"locale"`               // 语言设置，如: zh-CN, en-US
}

type BrokerConfig struct {
	Enabled        bool     `yaml:"enabled"`
	ActiveProfile  string   `yaml:"active_profile,omitempty"`
	LongConnection bool     `yaml:"long_connection"`
	SocketPath     string   `yaml:"socket_path,omitempty"`
	OwnerOpenID    string   `yaml:"owner_open_id,omitempty"`
	AdminOpenIDs   []string `yaml:"admin_open_ids,omitempty"`
	AllowedOpenIDs []string `yaml:"allowed_open_ids,omitempty"`
	AllowedChatIDs []string `yaml:"allowed_chat_ids,omitempty"`
}

type ApprovalConfig struct {
	Enabled        bool   `yaml:"enabled"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	DefaultAccess  string `yaml:"default_access"`
	MaxAccess      string `yaml:"max_access"`
	CodexMode      string `yaml:"codex_mode"` // notify_only or hook_decision
}

type ProfilesConfig map[string]ProfileConfig

type ProfileConfig struct {
	Agent          string              `yaml:"agent"`
	Workspace      string              `yaml:"workspace"`
	Enabled        bool                `yaml:"enabled"`
	PermissionMode string              `yaml:"permission_mode"`
	Feishu         ProfileFeishuConfig `yaml:"feishu,omitempty"`
	FeishuChatID   string              `yaml:"feishu_chat_id,omitempty"`
	Workspaces     map[string]string   `yaml:"workspaces,omitempty"`
}

type ProfileFeishuConfig struct {
	AppID       string `yaml:"app_id,omitempty"`
	AppSecret   string `yaml:"app_secret,omitempty"`
	OwnerOpenID string `yaml:"owner_open_id,omitempty"`
	ChatID      string `yaml:"chat_id,omitempty"`
}

func Default() Config {
	allEvents := []string{"permission_required", "input_required", "run_completed", "run_failed"}
	codexEvents := []string{"permission_required", "run_completed"}

	return Config{
		Version: 1,
		Agent: AgentConfig{
			ClaudeCode: AgentTargetConfig{
				Enabled:      true,
				InstallScope: "user",
			},
			Codex: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
			CodeBuddy: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
			Cursor: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
			Hermes: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
		},
		Notify: NotifyConfig{
			ClaudeCode: AgentNotifyConfig{
				Events: append([]string(nil), allEvents...),
				Channels: ChannelsConfig{
					System:     ChannelConfig{Enabled: true},
					Feishu:     ChannelConfig{Enabled: false},
					WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
					DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
					Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
					ServerChan: ServerChanChannelConfig{Enabled: false, SendKey: ""},
					PushPlus:   PushPlusChannelConfig{Enabled: false, Token: ""},
					WxPusher:   WxPusherChannelConfig{Enabled: false, AppToken: "", UID: ""},
				},
			},
			Codex: AgentNotifyConfig{
				Events: append([]string(nil), codexEvents...),
				Channels: ChannelsConfig{
					System:     ChannelConfig{Enabled: false},
					Feishu:     ChannelConfig{Enabled: false},
					WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
					DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
					Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
					ServerChan: ServerChanChannelConfig{Enabled: false, SendKey: ""},
					PushPlus:   PushPlusChannelConfig{Enabled: false, Token: ""},
					WxPusher:   WxPusherChannelConfig{Enabled: false, AppToken: "", UID: ""},
				},
			},
			CodeBuddy: AgentNotifyConfig{
				Events: append([]string(nil), allEvents...),
				Channels: ChannelsConfig{
					System:     ChannelConfig{Enabled: true},
					Feishu:     ChannelConfig{Enabled: false},
					WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
					DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
					Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
					ServerChan: ServerChanChannelConfig{Enabled: false, SendKey: ""},
					PushPlus:   PushPlusChannelConfig{Enabled: false, Token: ""},
					WxPusher:   WxPusherChannelConfig{Enabled: false, AppToken: "", UID: ""},
				},
			},
			Cursor: AgentNotifyConfig{
				Events: []string{"permission_required", "run_completed", "run_failed"},
				Channels: ChannelsConfig{
					System:     ChannelConfig{Enabled: false},
					Feishu:     ChannelConfig{Enabled: false},
					WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
					DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
					Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
					ServerChan: ServerChanChannelConfig{Enabled: false, SendKey: ""},
					PushPlus:   PushPlusChannelConfig{Enabled: false, Token: ""},
					WxPusher:   WxPusherChannelConfig{Enabled: false, AppToken: "", UID: ""},
				},
			},
			Hermes: AgentNotifyConfig{
				Events: []string{"permission_required", "run_completed"},
				Channels: ChannelsConfig{
					System:     ChannelConfig{Enabled: false},
					Feishu:     ChannelConfig{Enabled: false},
					WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
					DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
					Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
					ServerChan: ServerChanChannelConfig{Enabled: false, SendKey: ""},
					PushPlus:   PushPlusChannelConfig{Enabled: false, Token: ""},
					WxPusher:   WxPusherChannelConfig{Enabled: false, AppToken: "", UID: ""},
				},
			},
		},
		Behavior: BehaviorConfig{
			DedupeSeconds:      10,
			SendTimeoutSeconds: 5,
			Locale:             "zh-CN",
		},
		Broker: BrokerConfig{
			Enabled:        false,
			ActiveProfile:  "claude-main",
			LongConnection: true,
		},
		Approval: ApprovalConfig{
			Enabled:        false,
			TimeoutSeconds: 300,
			DefaultAccess:  "workspace-write",
			MaxAccess:      "workspace-write",
			CodexMode:      "notify_only",
		},
		Profiles: ProfilesConfig{
			"claude-main": {
				Agent:          "claude",
				Workspace:      "",
				Enabled:        false,
				PermissionMode: "workspace-write",
				Workspaces:     map[string]string{},
			},
			"codex-main": {
				Agent:          "codex",
				Workspace:      "",
				Enabled:        false,
				PermissionMode: "workspace-write",
				Workspaces:     map[string]string{},
			},
		},
	}
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "config.yaml"), nil
}

func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "state.json"), nil
}

func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "agent-notify.log"), nil
}

func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify"), nil
}

func ApprovalPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "approvals.json"), nil
}

func ProcessRegistryPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "processes.json"), nil
}

func AuditLogPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "audit.log"), nil
}

func BrokerPIDPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "broker.pid"), nil
}

func ThreadsPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "threads.json"), nil
}

func TasksPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tasks.json"), nil
}

func ViewsPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "views.json"), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}

	// 先解析到空结构体，避免默认值干扰
	cfg := Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// 填充默认值（仅对未设置的字段）
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Agent.ClaudeCode.InstallScope == "" {
		cfg.Agent.ClaudeCode.InstallScope = "user"
	}
	if cfg.Agent.Codex.InstallScope == "" {
		cfg.Agent.Codex.InstallScope = "user"
	}
	if cfg.Agent.CodeBuddy.InstallScope == "" {
		cfg.Agent.CodeBuddy.InstallScope = "user"
	}
	if cfg.Agent.Cursor.InstallScope == "" {
		cfg.Agent.Cursor.InstallScope = "user"
	}
	if cfg.Agent.Hermes.InstallScope == "" {
		cfg.Agent.Hermes.InstallScope = "user"
	}
	if len(cfg.Notify.CodeBuddy.Events) == 0 {
		cfg.Notify.CodeBuddy.Events = append([]string(nil), cfg.Notify.ClaudeCode.Events...)
	}
	if !cfg.Notify.CodeBuddy.Channels.System.Enabled &&
		!cfg.Notify.CodeBuddy.Channels.Feishu.Enabled &&
		!cfg.Notify.CodeBuddy.Channels.WechatWork.Enabled &&
		!cfg.Notify.CodeBuddy.Channels.DingTalk.Enabled &&
		!cfg.Notify.CodeBuddy.Channels.Bark.Enabled &&
		!cfg.Notify.CodeBuddy.Channels.ServerChan.Enabled &&
		!cfg.Notify.CodeBuddy.Channels.PushPlus.Enabled &&
		!cfg.Notify.CodeBuddy.Channels.WxPusher.Enabled {
		cfg.Notify.CodeBuddy.Channels.System.Enabled = true
	}
	if len(cfg.Notify.Cursor.Events) == 0 {
		cfg.Notify.Cursor.Events = []string{"permission_required", "run_completed", "run_failed"}
	}
	if len(cfg.Notify.Hermes.Events) == 0 {
		cfg.Notify.Hermes.Events = []string{"permission_required", "run_completed"}
	}
	if cfg.Behavior.DedupeSeconds == 0 {
		cfg.Behavior.DedupeSeconds = 60
	}
	if cfg.Behavior.SendTimeoutSeconds == 0 {
		cfg.Behavior.SendTimeoutSeconds = 5
	}
	if cfg.Behavior.Locale == "" {
		cfg.Behavior.Locale = "zh-CN"
	}
	if cfg.Broker.ActiveProfile == "" {
		cfg.Broker.ActiveProfile = "claude-main"
	}
	if cfg.Approval.TimeoutSeconds == 0 {
		cfg.Approval.TimeoutSeconds = 300
	}
	if cfg.Approval.DefaultAccess == "" {
		cfg.Approval.DefaultAccess = "workspace-write"
	}
	if cfg.Approval.MaxAccess == "" {
		cfg.Approval.MaxAccess = "workspace-write"
	}
	if cfg.Approval.CodexMode == "" {
		cfg.Approval.CodexMode = "notify_only"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = ProfilesConfig{}
	}
	for name, profile := range cfg.Profiles {
		if profile.Workspaces == nil {
			profile.Workspaces = map[string]string{}
			cfg.Profiles[name] = profile
		}
	}

	return cfg, nil
}

// allChannelsDisabled checks if no notification channels are enabled.
func allChannelsDisabled(ch ChannelsConfig) bool {
	return !ch.System.Enabled &&
		!ch.Feishu.Enabled &&
		!ch.WechatWork.Enabled &&
		!ch.DingTalk.Enabled &&
		!ch.Bark.Enabled &&
		!ch.ServerChan.Enabled &&
		!ch.PushPlus.Enabled &&
		!ch.WxPusher.Enabled
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
