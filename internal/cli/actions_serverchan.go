package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
)

func runTestServerChan(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	sendKey := serverChanKeyFromConfig(cfg)
	if sendKey == "" {
		return fmt.Errorf("未配置 Server酱 SendKey，请先运行配置向导")
	}

	msg := notify.Message{Title: "Agent Notify 测试", Body: "这是一条 Server酱 测试消息"}
	if err := notify.NewServerChanSender(sendKey).Send(ctx, msg); err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ Server酱 测试通知已发送")
	return nil
}

func runInitServerChan(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	sendKey, err := prompter.Input("Server酱 SendKey（SCU 开头）", serverChanKeyFromConfig(cfg))
	if err != nil {
		return err
	}

	cfg.Notify.ClaudeCode.Channels.ServerChan.Enabled = true
	cfg.Notify.ClaudeCode.Channels.ServerChan.SendKey = sendKey
	cfg.Notify.Codex.Channels.ServerChan.Enabled = true
	cfg.Notify.Codex.Channels.ServerChan.SendKey = sendKey

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	fmt.Fprintln(streams.Stdout, "✅ Server酱 配置完成")
	fmt.Fprintf(streams.Stdout, "   配置文件: %s\n", path)
	return nil
}

func serverChanKeyFromConfig(cfg config.Config) string {
	if cfg.Notify.ClaudeCode.Channels.ServerChan.SendKey != "" {
		return cfg.Notify.ClaudeCode.Channels.ServerChan.SendKey
	}
	return cfg.Notify.Codex.Channels.ServerChan.SendKey
}
