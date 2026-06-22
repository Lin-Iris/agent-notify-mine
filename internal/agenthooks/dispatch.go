package agenthooks

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
)

// ── 临时桥接 ──────────────────────────────────────────────

var sessionDir func(statePath string) string = func(statePath string) string {
	return filepath.Dir(statePath)
}

// eventToMessage 将 event.Event 转换为 notify.Message。
func eventToMessage(evt event.Event) notify.Message {
	var raw string
	if len(evt.RawPayload) > 0 {
		raw = string(evt.RawPayload)
	}
	return notify.Message{
		Agent:      evt.Agent,
		Event:      event.StatusToEventName(evt.Status),
		SessionID:  evt.SessionID,
		Workspace:  evt.Workspace,
		Title:      evt.Title,
		Body:       evt.Body,
		RawPayload: raw,
	}
}

// ── 现有 Dispatch（向后兼容） ─────────────────────────────

// Dispatch 是旧版入口，接收已构建好的 notify.Message。
// 仍被 handler 临时桥接代码调用，待全部迁移到 DispatchEvent 后移除。
func Dispatch(ctx context.Context, cfg config.Config, statePath, logPath string, msg notify.Message) error {
	store := state.NewStore(statePath)
	senders := buildSenders(cfg, msg)
	if len(senders) == 0 {
		return state.AppendLog(logPath, fmt.Sprintf("no sender enabled for event=%s", msg.Event))
	}

	dispatcher := notify.NewDispatcher(store, time.Duration(cfg.Behavior.DedupeSeconds)*time.Second, senders...)
	timeout := time.Duration(cfg.Behavior.SendTimeoutSeconds) * time.Second
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := dispatcher.SendAll(sendCtx, msg); err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("dispatch error event=%s session=%s err=%v", msg.Event, msg.SessionID, err))
	}

	return nil
}

// ── DispatchEvent（新入口，集成状态机） ───────────────────

// DispatchEvent 接收规范 Event，通过状态机推断最终状态，
// 决定是否通知以及用什么状态通知。
func DispatchEvent(ctx context.Context, cfg config.Config, statePath, logPath string, evt event.Event) error {
	// 初始化会话状态机
	sessionPath := filepath.Join(sessionDir(statePath), "sessions.json")
	store := state.NewSessionStore(sessionPath)
	advancer := state.NewAdvancer(store)

	// 状态机推进
	decision, err := advancer.Advance(evt)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("state machine error: %v", err))
	}

	// 状态机决定不通知
	if !decision.Notify {
		return state.AppendLog(logPath, fmt.Sprintf("skip event=%s session=%s reason=%s", evt.HookEvent, evt.SessionID, decision.Reason))
	}

	// 构建最终 Message
	msg := eventToMessage(evt)
	msg.Event = event.StatusToEventName(decision.Status)
	// 状态机升级状态后，标题也要跟着变，否则一直是"运行中"
	msg.Title = notify.FormatTitle(evt.Agent, msg.Event)

	// 检查该事件是否在用户配置中启用
	notifyCfg := notifyConfigForAgent(cfg, evt.Agent)
	if !contains(notifyCfg.Events, msg.Event) {
		return state.AppendLog(logPath, fmt.Sprintf("event=%s not enabled for agent=%s", msg.Event, evt.Agent))
	}

	// 构建发送者
	senders := buildSenders(cfg, msg)
	if len(senders) == 0 {
		return state.AppendLog(logPath, fmt.Sprintf("no sender enabled for event=%s", msg.Event))
	}

	// 推送
	dedupStore := state.NewStore(statePath)
	dispatcher := notify.NewDispatcher(dedupStore, time.Duration(cfg.Behavior.DedupeSeconds)*time.Second, senders...)
	timeout := time.Duration(cfg.Behavior.SendTimeoutSeconds) * time.Second
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := dispatcher.SendAll(sendCtx, msg); err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("dispatch error event=%s session=%s err=%v", msg.Event, msg.SessionID, err))
	}

	return nil
}

// ── 构建发送者 ────────────────────────────────────────────

// buildSenders 根据配置和消息，返回已启用的 Sender 列表。
// 注意：事件过滤由调用方负责（DispatchEvent 中已做），
// 这里假设 msg.Event 是用户已启用的。
func buildSenders(cfg config.Config, msg notify.Message) []notify.Sender {
	var senders []notify.Sender

	notifyCfg := notifyConfigForAgent(cfg, msg.Agent)

	// 事件过滤：只对已订阅的事件构建发送者
	if !contains(notifyCfg.Events, msg.Event) {
		return senders
	}

	if notifyCfg.Channels.System.Enabled {
		senders = append(senders, notify.NewSystemSender(notify.DefaultRunner))
	}
	if notifyCfg.Channels.Feishu.Enabled {
		senders = append(senders, notify.NewDefaultFeishuSender())
	}
	if notifyCfg.Channels.WechatWork.Enabled && notifyCfg.Channels.WechatWork.WebhookURL != "" {
		senders = append(senders, notify.NewWechatWorkSender(notifyCfg.Channels.WechatWork.WebhookURL))
	}
	if notifyCfg.Channels.DingTalk.Enabled && notifyCfg.Channels.DingTalk.WebhookURL != "" {
		senders = append(senders, notify.NewDingTalkSender(notifyCfg.Channels.DingTalk.WebhookURL))
	}
	if notifyCfg.Channels.Bark.Enabled && notifyCfg.Channels.Bark.WebhookURL != "" {
		senders = append(senders, notify.NewBarkSender(notifyCfg.Channels.Bark.WebhookURL))
	}
	if notifyCfg.Channels.ServerChan.Enabled && notifyCfg.Channels.ServerChan.SendKey != "" {
		senders = append(senders, notify.NewServerChanSender(notifyCfg.Channels.ServerChan.SendKey))
	}
	if notifyCfg.Channels.PushPlus.Enabled && notifyCfg.Channels.PushPlus.Token != "" {
		senders = append(senders, notify.NewPushPlusSender(notifyCfg.Channels.PushPlus.Token))
	}
	if notifyCfg.Channels.WxPusher.Enabled && notifyCfg.Channels.WxPusher.AppToken != "" && notifyCfg.Channels.WxPusher.UID != "" {
		senders = append(senders, notify.NewWxPusherSender(notifyCfg.Channels.WxPusher.AppToken, notifyCfg.Channels.WxPusher.UID))
	}

	return senders
}

func notifyConfigForAgent(cfg config.Config, agent string) config.AgentNotifyConfig {
	switch agent {
	case "codex":
		return cfg.Notify.Codex
	case "codebuddy":
		return cfg.Notify.CodeBuddy
	default:
		return cfg.Notify.ClaudeCode
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
