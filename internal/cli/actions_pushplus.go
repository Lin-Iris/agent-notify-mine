package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
)

func runTestPushPlus(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	token := pushPlusTokenFromConfig(cfg)
	if token == "" {
		return fmt.Errorf("未配置 PushPlus Token，请先运行配置向导")
	}

	msg := notify.Message{Title: "Agent Notify 测试", Body: "这是一条 PushPlus 测试消息"}
	if err := notify.NewPushPlusSender(token).Send(ctx, msg); err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ PushPlus 测试通知已发送")
	return nil
}

func runInitPushPlus(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	token, err := prompter.Input("PushPlus Token", pushPlusTokenFromConfig(cfg))
	if err != nil {
		return err
	}

	cfg.Notify.ClaudeCode.Channels.PushPlus.Enabled = true
	cfg.Notify.ClaudeCode.Channels.PushPlus.Token = token
	cfg.Notify.Codex.Channels.PushPlus.Enabled = true
	cfg.Notify.Codex.Channels.PushPlus.Token = token

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	fmt.Fprintln(streams.Stdout, "✅ PushPlus 配置完成")
	fmt.Fprintf(streams.Stdout, "   配置文件: %s\n", path)
	return nil
}

func pushPlusTokenFromConfig(cfg config.Config) string {
	if cfg.Notify.ClaudeCode.Channels.PushPlus.Token != "" {
		return cfg.Notify.ClaudeCode.Channels.PushPlus.Token
	}
	return cfg.Notify.Codex.Channels.PushPlus.Token
}
