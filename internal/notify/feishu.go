package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type FeishuCLIConfig struct {
	AppID         string
	AppSecret     string
	ReceiveID     string
	ReceiveIDType string
}

type feishuConfigProvider interface {
	Parse() (FeishuCLIConfig, error)
}

type clientToolsConfigProvider struct{}

func (clientToolsConfigProvider) Parse() (FeishuCLIConfig, error) {
	cfg, err := feishucli.ParseConfig()
	if err != nil {
		return FeishuCLIConfig{}, err
	}
	return FeishuCLIConfig{
		AppID:     cfg.AppID,
		AppSecret: cfg.AppSecret,
	}, nil
}

type feishuMessenger interface {
	CreatorOpenID(ctx context.Context, appID string) (string, error)
	SendCard(ctx context.Context, receiveIDType, receiveID string, card map[string]any) (string, error)
	UpdateCard(ctx context.Context, messageID string, card map[string]any) error
	SendText(ctx context.Context, receiveIDType, receiveID string, text string) error
}

type sdkFeishuMessenger struct {
	client *lark.Client
}

type FeishuSender struct {
	provider     feishuConfigProvider
	newMessenger func(appID, appSecret string) (feishuMessenger, error)
}

type BrokerControlStatus struct {
	Profile        string
	Agent          string
	Workspace      string
	PermissionMode string
	BrokerEnabled  bool
	ProfileEnabled bool
	Pending        int
	Processes      int
	ActiveThread   string
	CLIDiagnostics string
}

type ThreadListStatus struct {
	Profile   string
	Workspace string
	Page      int
	HasPrev   bool
	HasNext   bool
	Threads   []ThreadSummary
}

type ThreadSummary struct {
	ID        string
	Number    int
	Title     string
	Status    string
	UpdatedAt string
}

type ThreadDetailStatus struct {
	Profile    string
	Workspace  string
	ThreadID   string
	Number     int
	Title      string
	Status     string
	Agent      string
	LastTaskID string
	UpdatedAt  string
}

type TaskStatus struct {
	Profile   string
	Workspace string
	ThreadID  string
	TaskID    string
	Number    int
	Title     string
	Status    string
	Progress  string
	Final     string
	LogPath   string
}

func NewFeishuSender(provider feishuConfigProvider) *FeishuSender {
	return &FeishuSender{
		provider:     provider,
		newMessenger: newSDKFeishuMessenger,
	}
}

func NewDefaultFeishuSender() *FeishuSender {
	return NewFeishuSender(clientToolsConfigProvider{})
}

type staticFeishuConfigProvider struct {
	cfg FeishuCLIConfig
}

func (p staticFeishuConfigProvider) Parse() (FeishuCLIConfig, error) {
	return p.cfg, nil
}

func NewProfileFeishuSender(profileName string, profile config.ProfileConfig) (*FeishuSender, error) {
	if profile.Feishu.AppID == "" || profile.Feishu.AppSecret == "" || profile.Feishu.OwnerOpenID == "" {
		return nil, fmt.Errorf("profile %s has no feishu bot configured", profileName)
	}
	receiveID := profile.Feishu.OwnerOpenID
	receiveIDType := "open_id"
	if profile.Feishu.ChatID != "" {
		receiveID = profile.Feishu.ChatID
		receiveIDType = "chat_id"
	}
	return NewFeishuSender(staticFeishuConfigProvider{cfg: FeishuCLIConfig{
		AppID:         profile.Feishu.AppID,
		AppSecret:     profile.Feishu.AppSecret,
		ReceiveID:     receiveID,
		ReceiveIDType: receiveIDType,
	}}), nil
}

func (s *FeishuSender) Name() string { return "feishu" }

func (s *FeishuSender) Send(ctx context.Context, msg Message) error {
	cfg, err := s.provider.Parse()
	if err != nil {
		return err
	}

	messenger, err := s.newMessenger(cfg.AppID, cfg.AppSecret)
	if err != nil {
		return err
	}

	receiveIDType, receiveID, err := s.resolveReceiver(ctx, cfg, messenger)
	if err != nil {
		return err
	}

	card := s.buildCard(msg)
	_, err = messenger.SendCard(ctx, receiveIDType, receiveID, card)
	return err
}

func (s *FeishuSender) SendRawCard(ctx context.Context, card map[string]any) (string, error) {
	cfg, err := s.provider.Parse()
	if err != nil {
		return "", err
	}

	messenger, err := s.newMessenger(cfg.AppID, cfg.AppSecret)
	if err != nil {
		return "", err
	}

	receiveIDType, receiveID, err := s.resolveReceiver(ctx, cfg, messenger)
	if err != nil {
		return "", err
	}

	return messenger.SendCard(ctx, receiveIDType, receiveID, card)
}

func (s *FeishuSender) UpdateRawCard(ctx context.Context, messageID string, card map[string]any) error {
	if messageID == "" {
		return errors.New("feishu message id is empty")
	}
	cfg, err := s.provider.Parse()
	if err != nil {
		return err
	}
	messenger, err := s.newMessenger(cfg.AppID, cfg.AppSecret)
	if err != nil {
		return err
	}
	return messenger.UpdateCard(ctx, messageID, card)
}

func (s *FeishuSender) SendText(ctx context.Context, text string) error {
	cfg, err := s.provider.Parse()
	if err != nil {
		return err
	}

	messenger, err := s.newMessenger(cfg.AppID, cfg.AppSecret)
	if err != nil {
		return err
	}

	receiveIDType, receiveID, err := s.resolveReceiver(ctx, cfg, messenger)
	if err != nil {
		return err
	}

	return messenger.SendText(ctx, receiveIDType, receiveID, text)
}

func (s *FeishuSender) SendLongText(ctx context.Context, title, text string) error {
	chunks := splitText(title+"\n\n"+text, 3500)
	for _, chunk := range chunks {
		if err := s.SendText(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (s *FeishuSender) resolveReceiver(ctx context.Context, cfg FeishuCLIConfig, messenger feishuMessenger) (string, string, error) {
	if cfg.ReceiveID != "" {
		typ := cfg.ReceiveIDType
		if typ == "" {
			typ = "open_id"
		}
		return typ, cfg.ReceiveID, nil
	}
	creatorOpenID, err := messenger.CreatorOpenID(ctx, cfg.AppID)
	if err != nil {
		return "", "", err
	}
	return "open_id", creatorOpenID, nil
}

func BuildBrokerControlCard(status BrokerControlStatus) map[string]any {
	state := "已关闭"
	stateTemplate := "grey"
	toggleText := "开启通信"
	toggleType := "primary"
	toggleAction := "broker_connect"
	if status.BrokerEnabled && status.ProfileEnabled {
		state = "已开启"
		stateTemplate = "green"
		toggleText = "暂停通信"
		toggleType = "danger"
		toggleAction = "broker_pause"
	}
	workspace := status.Workspace
	if workspace == "" {
		workspace = "未设置"
	}
	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "Agent Notify 控制台",
			},
			"template": stateTemplate,
		},
		"elements": []any{
			map[string]any{
				"tag": "div",
				"fields": []any{
					map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**通信状态**\n%s", state)}},
					map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**会话**\n%s", status.Profile)}},
					map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**Agent**\n%s", status.Agent)}},
					map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**权限模式**\n%s", status.PermissionMode)}},
					map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**待审批**\n%d", status.Pending)}},
					map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**运行进程**\n%d", status.Processes)}},
				},
			},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**当前对话**\n%s", emptyAs(status.ActiveThread, "未选择")),
				},
			},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**当前项目**\n`%s`", workspace),
				},
			},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**CLI 状态**\n%s", emptyAs(status.CLIDiagnostics, "未检测")),
				},
			},
			map[string]any{
				"tag":    "action",
				"layout": "bisected",
				"actions": []any{
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": toggleText},
						"type": toggleType,
						"value": map[string]any{
							"action":  toggleAction,
							"profile": status.Profile,
						},
					},
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "查看对话"},
						"type": "default",
						"value": map[string]any{
							"action":  "threads_list",
							"profile": status.Profile,
							"page":    1,
						},
					},
				},
			},
			map[string]any{
				"tag":    "action",
				"layout": "bisected",
				"actions": []any{
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "停止任务"},
						"type": "danger",
						"value": map[string]any{
							"action":  "broker_stop",
							"profile": status.Profile,
						},
					},
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "新建对话"},
						"type": "default",
						"value": map[string]any{
							"action":  "thread_new",
							"profile": status.Profile,
						},
					},
				},
			},
			map[string]any{
				"tag":    "action",
				"layout": "bisected",
				"actions": []any{
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "刷新状态"},
						"type": "default",
						"value": map[string]any{
							"action":  "broker_status",
							"profile": status.Profile,
						},
					},
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "断开并清理"},
						"type": "danger",
						"value": map[string]any{
							"action":  "broker_disconnect",
							"profile": status.Profile,
						},
					},
				},
			},
			map[string]any{"tag": "hr"},
			map[string]any{
				"tag": "note",
				"elements": []any{
					map[string]any{"tag": "plain_text", "content": "普通消息会进入当前会话；/status、/cd、/ws、/stop、/ps、/exit、/disconnect 是控制命令。"},
				},
			},
		},
	}
}

func BuildThreadListCard(status ThreadListStatus) map[string]any {
	elements := []any{
		map[string]any{
			"tag":  "div",
			"text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**项目**\n`%s`\n**Profile**\n%s", status.Workspace, status.Profile)},
		},
	}
	if len(status.Threads) == 0 {
		elements = append(elements, map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": "暂无对话窗口。"}})
	}
	for _, thread := range status.Threads {
		elements = append(elements,
			map[string]any{"tag": "hr"},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**#%d %s**\n状态：%s\n更新：%s", thread.Number, thread.Title, thread.Status, thread.UpdatedAt),
				},
			},
			map[string]any{
				"tag":    "action",
				"layout": "bisected",
				"actions": []any{
					map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "进入"}, "type": "primary", "value": map[string]any{"action": "thread_open", "profile": status.Profile, "thread_id": thread.ID}},
					map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "查看结果"}, "type": "default", "value": map[string]any{"action": "thread_result", "profile": status.Profile, "thread_id": thread.ID}},
				},
			},
		)
	}
	actions := []any{
		map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "新建对话"}, "type": "primary", "value": map[string]any{"action": "thread_new", "profile": status.Profile}},
		map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "返回控制台"}, "type": "default", "value": map[string]any{"action": "home", "profile": status.Profile}},
	}
	if status.HasPrev {
		actions = append(actions, map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "上一页"}, "type": "default", "value": map[string]any{"action": "threads_list", "profile": status.Profile, "page": status.Page - 1}})
	}
	if status.HasNext {
		actions = append(actions, map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "下一页"}, "type": "default", "value": map[string]any{"action": "threads_list", "profile": status.Profile, "page": status.Page + 1}})
	}
	elements = append(elements, map[string]any{"tag": "hr"}, map[string]any{"tag": "action", "actions": actions})
	return simpleCard("项目对话列表", "blue", elements)
}

func BuildThreadDetailCard(status ThreadDetailStatus) map[string]any {
	elements := []any{
		map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("**#%d %s**\n状态：%s\nAgent：%s\n项目：`%s`\n最后更新：%s", status.Number, status.Title, status.Status, status.Agent, status.Workspace, status.UpdatedAt)}},
		map[string]any{"tag": "action", "layout": "bisected", "actions": []any{
			map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "继续这个对话"}, "type": "primary", "value": map[string]any{"action": "thread_use", "profile": status.Profile, "thread_id": status.ThreadID}},
			map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "查看最终结果"}, "type": "default", "value": map[string]any{"action": "thread_result", "profile": status.Profile, "thread_id": status.ThreadID}},
		}},
		map[string]any{"tag": "action", "layout": "bisected", "actions": []any{
			map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "返回对话列表"}, "type": "default", "value": map[string]any{"action": "threads_list", "profile": status.Profile, "page": 1}},
			map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "返回控制台"}, "type": "default", "value": map[string]any{"action": "home", "profile": status.Profile}},
		}},
	}
	return simpleCard("对话窗口", "turquoise", elements)
}

func BuildTaskStatusCard(status TaskStatus) map[string]any {
	template := "blue"
	title := "任务正在运行"
	if status.Status == "done" {
		template = "green"
		title = "任务已完成"
	} else if status.Status == "failed" || status.Status == "stopped" {
		template = "red"
		title = "任务未完成"
	}
	content := fmt.Sprintf("**任务 #%d**\n状态：%s\n对话：%s\n项目：`%s`", status.Number, status.Status, status.ThreadID, status.Workspace)
	if status.Progress != "" && !(status.Status == "done" && status.Final != "") {
		label := "最近进展"
		if status.Status == "running" {
			label = "模型输出中"
		}
		content += "\n\n**" + label + "**\n" + status.Progress
	}
	if status.Final != "" {
		content += "\n\n**最终结果**\n" + status.Final
	}
	elements := []any{
		map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": content}},
	}
	if status.Status == "done" {
		elements = append(elements,
			map[string]any{"tag": "action", "layout": "bisected", "actions": []any{
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "模型输出"}, "type": "primary", "value": map[string]any{"action": "task_result", "profile": status.Profile, "task_id": status.TaskID}},
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "返回对话"}, "type": "default", "value": map[string]any{"action": "thread_open", "profile": status.Profile, "thread_id": status.ThreadID}},
			}},
			map[string]any{"tag": "action", "actions": []any{
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "返回控制台"}, "type": "default", "value": map[string]any{"action": "home", "profile": status.Profile}},
			}},
		)
	} else {
		elements = append(elements,
			map[string]any{"tag": "action", "layout": "bisected", "actions": []any{
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "最近输出"}, "type": "default", "value": map[string]any{"action": "task_tail", "profile": status.Profile, "task_id": status.TaskID}},
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "完整过程"}, "type": "default", "value": map[string]any{"action": "task_log", "profile": status.Profile, "task_id": status.TaskID}},
			}},
			map[string]any{"tag": "action", "layout": "bisected", "actions": []any{
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "返回对话"}, "type": "default", "value": map[string]any{"action": "thread_open", "profile": status.Profile, "thread_id": status.ThreadID}},
				map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "返回控制台"}, "type": "default", "value": map[string]any{"action": "home", "profile": status.Profile}},
			}},
		)
	}
	if status.Status == "running" {
		elements = append(elements, map[string]any{"tag": "action", "actions": []any{
			map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": "停止任务"}, "type": "danger", "value": map[string]any{"action": "task_stop", "profile": status.Profile, "task_id": status.TaskID}},
		}})
	}
	return simpleCard(title, template, elements)
}

func simpleCard(title, template string, elements []any) map[string]any {
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": title},
			"template": template,
		},
		"elements": elements,
	}
}

// buildCard creates a rich interactive card for Feishu notification
func (s *FeishuSender) buildCard(msg Message) map[string]any {
	// Event emoji mapping
	eventEmoji := map[string]string{
		"permission_required": "🔐",
		"input_required":      "⌨️",
		"run_completed":       "✅",
		"run_failed":          "❌",
	}
	emoji := eventEmoji[msg.Event]
	if emoji == "" {
		emoji = "🔔"
	}

	// Event type mapping for display
	eventType := map[string]string{
		"permission_required": "等待授权",
		"input_required":      "等待输入",
		"run_completed":       "运行完成",
		"run_failed":          "运行失败",
	}
	eventName := eventType[msg.Event]
	if eventName == "" {
		eventName = msg.Event
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	isCodex := msg.Agent == "codex"
	footerText := "🤖 Agent Notify"
	if isCodex {
		footerText = "🤖 Codex Agent Notify"
	}

	elements := []any{
		map[string]any{
			"tag": "div",
			"fields": []any{
				map[string]any{
					"is_short": true,
					"text": map[string]any{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**事件类型**\n%s", eventName),
					},
				},
				map[string]any{
					"is_short": true,
					"text": map[string]any{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**时间**\n%s", timestamp),
					},
				},
			},
		},
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**消息内容**\n%s", msg.Body),
			},
		},
	}
	if msg.Workspace != "" && !isCodex {
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**工作目录**\n`%s`", msg.Workspace),
			},
		})
	}
	if msg.ApprovalID != "" {
		elements = append(elements,
			map[string]any{
				"tag":    "action",
				"layout": "bisected",
				"actions": []any{
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "批准"},
						"type": "primary",
						"value": map[string]any{
							"action":      "approve",
							"approval_id": msg.ApprovalID,
							"token":       msg.ApprovalToken,
							"profile":     msg.Profile,
						},
					},
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "拒绝"},
						"type": "danger",
						"value": map[string]any{
							"action":      "deny",
							"approval_id": msg.ApprovalID,
							"token":       msg.ApprovalToken,
							"profile":     msg.Profile,
						},
					},
				},
			},
		)
	} else if msg.Event == "permission_required" {
		elements = append(elements, map[string]any{
			"tag": "note",
			"elements": []any{
				map[string]any{
					"tag":     "plain_text",
					"content": "远程操作未开启或此请求不可远程审批，请回电脑授权。",
				},
			},
		})
	}
	elements = append(elements,
		map[string]any{
			"tag": "hr",
		},
		map[string]any{
			"tag": "note",
			"elements": []any{
				map[string]any{
					"tag":     "plain_text",
					"content": footerText,
				},
			},
		},
	)

	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": fmt.Sprintf("%s %s", emoji, msg.Title),
			},
			"template": s.getHeaderColor(msg.Event),
		},
		"elements": elements,
	}
}

// getHeaderColor returns the header color based on event type
func (s *FeishuSender) getHeaderColor(event string) string {
	switch event {
	case "permission_required":
		return "orange"
	case "input_required":
		return "blue"
	case "run_completed":
		return "green"
	case "run_failed":
		return "red"
	default:
		return "turquoise"
	}
}

func newSDKFeishuMessenger(appID, appSecret string) (feishuMessenger, error) {
	if appID == "" || appSecret == "" {
		return nil, errors.New("feishu app_id or app_secret is empty")
	}
	return &sdkFeishuMessenger{client: lark.NewClient(appID, appSecret)}, nil
}

func ResolveFeishuAppCreatorOpenID(ctx context.Context, appID, appSecret string) (string, error) {
	messenger, err := newSDKFeishuMessenger(appID, appSecret)
	if err != nil {
		return "", err
	}
	return messenger.CreatorOpenID(ctx, appID)
}

func (m *sdkFeishuMessenger) CreatorOpenID(ctx context.Context, appID string) (string, error) {
	req := larkapplication.NewGetApplicationReqBuilder().
		AppId(appID).
		Lang("zh_cn").
		UserIdType("open_id").
		Build()

	resp, err := m.client.Application.V6.Application.Get(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu get application failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.App == nil || resp.Data.App.CreatorId == nil || *resp.Data.App.CreatorId == "" {
		return "", errors.New("feishu application creator open_id is empty")
	}

	return *resp.Data.App.CreatorId, nil
}

func (m *sdkFeishuMessenger) SendCard(ctx context.Context, receiveIDType, receiveID string, card map[string]any) (string, error) {
	content, err := json.Marshal(card)
	if err != nil {
		return "", err
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType("interactive").
			Content(string(content)).
			Uuid(uuid.NewString()).
			Build()).
		Build()

	resp, err := m.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu send message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.MessageId == nil || *resp.Data.MessageId == "" {
		return "", errors.New("feishu send message response missing message_id")
	}

	return *resp.Data.MessageId, nil
}

func (m *sdkFeishuMessenger) UpdateCard(ctx context.Context, messageID string, card map[string]any) error {
	content, err := json.Marshal(card)
	if err != nil {
		return err
	}

	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			MsgType("interactive").
			Content(string(content)).
			Build()).
		Build()

	resp, err := m.client.Im.V1.Message.Update(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu update message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

func (m *sdkFeishuMessenger) SendText(ctx context.Context, receiveIDType, receiveID string, text string) error {
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType("text").
			Content(string(content)).
			Uuid(uuid.NewString()).
			Build()).
		Build()

	resp, err := m.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu send text failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

func splitText(text string, limit int) []string {
	if limit <= 0 || len(text) <= limit {
		return []string{text}
	}
	var chunks []string
	for len(text) > limit {
		cut := limit
		if idx := strings.LastIndex(text[:limit], "\n"); idx > limit/2 {
			cut = idx
		}
		chunks = append(chunks, text[:cut])
		text = strings.TrimLeft(text[cut:], "\n")
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
}

func emptyAs(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
