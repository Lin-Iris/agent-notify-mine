package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/app/doctor"
	"github.com/hellolib/agent-notify/internal/app/setup"
	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/claudehooks"
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/config"
)

// cliPrompter adapts CLI Prompter to setup.Prompter
type cliPrompter struct {
	p Prompter
}

func (cp *cliPrompter) Select(message string, options []setup.PromptOption, defaultValue string) (string, error) {
	cliOptions := make([]PromptOption, len(options))
	for i, o := range options {
		cliOptions[i] = PromptOption{Label: o.Label, Value: o.Value}
	}
	return cp.p.Select(message, cliOptions, defaultValue)
}

func (cp *cliPrompter) MultiSelect(message string, options []setup.PromptOption, defaults []string) ([]string, error) {
	cliOptions := make([]PromptOption, len(options))
	for i, o := range options {
		cliOptions[i] = PromptOption{Label: o.Label, Value: o.Value}
	}
	return cp.p.MultiSelect(message, cliOptions, defaults)
}

func (cp *cliPrompter) Confirm(message string, defaultValue bool) (bool, error) {
	return cp.p.Confirm(message, defaultValue)
}

func (cp *cliPrompter) Input(message, defaultValue string) (string, error) {
	return cp.p.Input(message, defaultValue)
}

// cliOutputWriter adapts Streams to setup/doctor OutputWriter
type cliOutputWriter struct {
	streams Streams
}

func (w *cliOutputWriter) Writef(format string, args ...any) {
	fmt.Fprintf(w.streams.Stdout, format, args...)
}

// feishuPreparerAdapter adapts the prepareFeishuCLI function to setup.FeishuPreparer
type feishuPreparerAdapter struct{}

func (f *feishuPreparerAdapter) EnsureReady(ctx context.Context) error {
	return prepareFeishuCLI(ctx)
}

func (f *feishuPreparerAdapter) Prepare(ctx context.Context) (setup.FeishuConfig, error) {
	cfg, err := profileFeishuEnsureReady(ctx)
	if err != nil {
		return setup.FeishuConfig{}, err
	}
	if cfg.UserOpenID == "" && cfg.AppID != "" && cfg.AppSecret != "" {
		ownerOpenID, err := profileFeishuResolveOwnerOpenID(ctx, cfg.AppID, cfg.AppSecret)
		if err != nil {
			return setup.FeishuConfig{}, fmt.Errorf("feishu QR setup missing owner open_id and fallback lookup failed: %w", err)
		}
		cfg.UserOpenID = ownerOpenID
	}
	return setup.FeishuConfig{
		AppID:      cfg.AppID,
		AppSecret:  cfg.AppSecret,
		UserOpenID: cfg.UserOpenID,
		UserName:   cfg.UserName,
	}, nil
}

type brokerStarterAdapter struct {
	streams Streams
}

func (b *brokerStarterAdapter) Start(ctx context.Context, cfg config.Config, configPath, profile string) error {
	_ = ctx
	cfg.Broker.Enabled = true
	cfg.Broker.LongConnection = true
	cfg.Approval.Enabled = true
	cfg.Broker.ActiveProfile = profile
	p := ensureProfile(&cfg, profile)
	p.Enabled = true
	cfg.Profiles[profile] = p
	if err := config.Save(configPath, cfg); err != nil {
		return err
	}
	pid, alreadyRunning, err := startBrokerDaemon()
	if err != nil {
		return err
	}
	if alreadyRunning {
		fmt.Fprintf(b.streams.Stdout, "broker already running: pid=%d profile=%s approval_timeout=%ds\n", pid, profile, cfg.Approval.TimeoutSeconds)
	} else {
		fmt.Fprintf(b.streams.Stdout, "broker enabled and started: pid=%d profile=%s approval_timeout=%ds\n", pid, profile, cfg.Approval.TimeoutSeconds)
	}
	if err := sendBrokerControlCard(ctx, profile); err != nil {
		fmt.Fprintf(b.streams.Stdout, "远程对话服务已启动，但控制台卡发送失败: %v\n", err)
		fmt.Fprintf(b.streams.Stdout, "稍后可以手动运行: agent-notify broker card --profile %s\n", profile)
	}
	if p.Workspace == "" {
		fmt.Fprintln(b.streams.Stdout, "当前项目未设置。请在飞书对话里发送 `/cd /具体/项目/目录` 后再发任务。")
	}
	return appendAudit("broker start profile=" + profile)
}

func runInitFlow(ctx context.Context, streams Streams, prompter Prompter, configPath, settingsPath, binaryPath string) error {
	_ = settingsPath // kept for backward compatibility

	svc := setup.NewService(
		setup.WithClaudeIntegration(agentintegrations.NewClaudeIntegration()),
		setup.WithCodexIntegration(agentintegrations.NewCodexIntegration()),
		setup.WithCodeBuddyIntegration(agentintegrations.NewCodeBuddyIntegration()),
		setup.WithCursorIntegration(agentintegrations.NewCursorIntegration()),
		setup.WithHermesIntegration(agentintegrations.NewHermesIntegration()),
		setup.WithFeishuPreparer(&feishuPreparerAdapter{}),
		setup.WithBrokerStarter(&brokerStarterAdapter{streams: streams}),
	)

	cliPrompter := &cliPrompter{p: prompter}
	output := &cliOutputWriter{streams: streams}

	_, err := svc.Run(ctx, cliPrompter, output, configPath, binaryPath)
	return err
}

func runPrintClaudeHooks(streams Streams, binaryPath string) error {
	settings := claudehooks.BuildHookSettings(common.ResolveBinaryPath(binaryPath))
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(streams.Stdout, string(data))
	return err
}

func runInstallClaudeHooks(scope, binaryPath string) error {
	path, err := settingsPathForAgent("claude", scope)
	if err != nil {
		return err
	}
	return claudehooks.Install(path, common.ResolveBinaryPath(binaryPath))
}

func runTestFeishu(ctx context.Context, streams Streams) error {
	svc := tester.NewService(
		tester.WithFeishuPreparer(&feishuPreparerAdapter{}),
	)
	result, err := svc.TestFeishu(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runTestSystem(ctx context.Context, streams Streams) error {
	svc := tester.NewService()
	result, err := svc.TestSystem(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runTestWechatWork(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	// Try claude config first, fall back to codex
	webhookURL := cfg.Notify.ClaudeCode.Channels.WechatWork.WebhookURL
	if webhookURL == "" {
		webhookURL = cfg.Notify.Codex.Channels.WechatWork.WebhookURL
	}
	if webhookURL == "" {
		return fmt.Errorf("未配置企业微信 Webhook URL，请先运行配置向导")
	}

	svc := tester.NewService()
	result, err := svc.TestWechatWork(ctx, webhookURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitWechatWork(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	// Get current webhook URL (use claude's if available)
	currentURL := cfg.Notify.ClaudeCode.Channels.WechatWork.WebhookURL
	if currentURL == "" {
		currentURL = cfg.Notify.Codex.Channels.WechatWork.WebhookURL
	}

	webhookURL, err := prompter.Input("企业微信群机器人 Webhook URL", currentURL)
	if err != nil {
		return err
	}

	// Update both agents with the same webhook URL
	cfg.Notify.ClaudeCode.Channels.WechatWork.Enabled = true
	cfg.Notify.ClaudeCode.Channels.WechatWork.WebhookURL = webhookURL
	cfg.Notify.Codex.Channels.WechatWork.Enabled = true
	cfg.Notify.Codex.Channels.WechatWork.WebhookURL = webhookURL

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	fmt.Fprintln(streams.Stdout, "✅ 企业微信 Webhook 配置完成")
	fmt.Fprintf(streams.Stdout, "配置文件: %s\n", path)
	return nil
}

func runDoctor(streams Streams) error {
	svc := doctor.NewService(
		doctor.WithClaudeIntegration(agentintegrations.NewClaudeIntegration()),
		doctor.WithCodexIntegration(agentintegrations.NewCodexIntegration()),
	)
	result, err := svc.Run()
	if err != nil {
		return err
	}
	output := &cliOutputWriter{streams: streams}
	svc.Print(output, result)
	return nil
}

func loadDefaultConfig() (config.Config, string, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.Config{}, "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, path, nil
}

func printCurrentNotifyConfig(streams Streams) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	fmt.Fprintf(streams.Stdout, "配置文件: %s\n\n", path)

	statusIcon := func(enabled bool) string {
		if enabled {
			return "✅"
		}
		return "❌"
	}

	// Fixed width table with ASCII borders.
	// 终端宽度规划：Agent=14, 飞书/系统/钉钉/Bark 列宽 6（含两侧空格），企业微信列宽 10。
	// emoji ✅/❌ 按 2 列宽计算，所以 6 宽列填 "  ✅  "（2 空格 + emoji + 2 空格），
	// 10 宽列填 "    ✅    "（4 空格 + emoji + 4 空格）。
	fmt.Fprintln(streams.Stdout, "+--------------+------+------+----------+------+------+")
	fmt.Fprintln(streams.Stdout, "| Agent        | 飞书 | 系统 | 企业微信 | 钉钉 | Bark |")
	fmt.Fprintln(streams.Stdout, "+--------------+------+------+----------+------+------+")
	fmt.Fprintf(streams.Stdout, "| %-12s |  %s  |  %s  |    %s    |  %s  |  %s  |\n", "Claude Code",
		statusIcon(cfg.Notify.ClaudeCode.Channels.Feishu.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.System.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.WechatWork.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.DingTalk.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.Bark.Enabled))
	fmt.Fprintf(streams.Stdout, "| %-12s |  %s  |  %s  |    %s    |  %s  |  %s  |\n", "Codex",
		statusIcon(cfg.Notify.Codex.Channels.Feishu.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.System.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.WechatWork.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.DingTalk.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.Bark.Enabled))
	fmt.Fprintln(streams.Stdout, "+--------------+------+------+----------+------+------+")

	return nil
}

// settingsPathForAgent returns the settings path for the given agent and scope.
// Currently only Claude has manual install-hooks subcommands; the Codex path is
// handled exclusively through the init flow + CodexIntegration.
func settingsPathForAgent(agent, scope string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch agent {
	case "claude":
		switch scope {
		case "user":
			return filepath.Join(home, ".claude", "settings.json"), nil
		case "project":
			return filepath.Join(".claude", "settings.json"), nil
		default:
			return "", fmt.Errorf("unsupported scope: %s", scope)
		}
	default:
		return "", fmt.Errorf("unsupported agent: %s", agent)
	}
}
