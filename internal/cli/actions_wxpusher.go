package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
)

func runTestWxPusher(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	appToken, uid := wxPusherCredsFromConfig(cfg)
	if appToken == "" || uid == "" {
		return fmt.Errorf("未配置 WxPusher AppToken 或 UID，请先运行配置向导")
	}

	msg := notify.Message{Title: "Agent Notify 测试", Body: "这是一条 WxPusher 测试消息"}
	if err := notify.NewWxPusherSender(appToken, uid).Send(ctx, msg); err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ WxPusher 测试通知已发送")
	return nil
}

func runInitWxPusher(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	curToken, curUID := wxPusherCredsFromConfig(cfg)

	appToken, err := prompter.Input("WxPusher AppToken", curToken)
	if err != nil {
		return err
	}
	uid, err := prompter.Input("WxPusher UID", curUID)
	if err != nil {
		return err
	}

	cfg.Notify.ClaudeCode.Channels.WxPusher.Enabled = true
	cfg.Notify.ClaudeCode.Channels.WxPusher.AppToken = appToken
	cfg.Notify.ClaudeCode.Channels.WxPusher.UID = uid
	cfg.Notify.Codex.Channels.WxPusher.Enabled = true
	cfg.Notify.Codex.Channels.WxPusher.AppToken = appToken
	cfg.Notify.Codex.Channels.WxPusher.UID = uid

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	fmt.Fprintln(streams.Stdout, "✅ WxPusher 配置完成")
	fmt.Fprintf(streams.Stdout, "   配置文件: %s\n", path)
	return nil
}

func wxPusherCredsFromConfig(cfg config.Config) (string, string) {
	if cfg.Notify.ClaudeCode.Channels.WxPusher.AppToken != "" {
		return cfg.Notify.ClaudeCode.Channels.WxPusher.AppToken, cfg.Notify.ClaudeCode.Channels.WxPusher.UID
	}
	return cfg.Notify.Codex.Channels.WxPusher.AppToken, cfg.Notify.Codex.Channels.WxPusher.UID
}
