package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

type stubFeishuConfigProvider struct {
	cfg FeishuCLIConfig
	err error
}

func (p stubFeishuConfigProvider) Parse() (FeishuCLIConfig, error) {
	if p.err != nil {
		return FeishuCLIConfig{}, p.err
	}
	return p.cfg, nil
}

type stubFeishuMessenger struct {
	creatorAppID      string
	creatorOpenID     string
	sentReceiveID     string
	sentReceiveIDType string
	messageID         string
	updatedID         string
	sentCard          map[string]any
	updatedCard       map[string]any
	creatorErr        error
	sendErr           error
	updateErr         error
}

func (m *stubFeishuMessenger) CreatorOpenID(ctx context.Context, appID string) (string, error) {
	m.creatorAppID = appID
	if m.creatorErr != nil {
		return "", m.creatorErr
	}
	return m.creatorOpenID, nil
}

func (m *stubFeishuMessenger) SendCard(ctx context.Context, receiveIDType, receiveID string, card map[string]any) (string, error) {
	m.sentReceiveIDType = receiveIDType
	m.sentReceiveID = receiveID
	m.sentCard = card
	return m.messageID, m.sendErr
}

func (m *stubFeishuMessenger) UpdateCard(ctx context.Context, messageID string, card map[string]any) error {
	m.updatedID = messageID
	m.updatedCard = card
	return m.updateErr
}

func (m *stubFeishuMessenger) SendText(ctx context.Context, receiveIDType, receiveID string, text string) error {
	m.sentReceiveIDType = receiveIDType
	m.sentReceiveID = receiveID
	return m.sendErr
}

func TestFeishuSenderSendUsesCLIConfigAndCreator(t *testing.T) {
	provider := stubFeishuConfigProvider{
		cfg: FeishuCLIConfig{
			AppID:     "cli_app",
			AppSecret: "secret",
		},
	}
	messenger := &stubFeishuMessenger{creatorOpenID: "ou_creator"}
	sender := NewFeishuSender(provider)
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		if appID != "cli_app" {
			t.Fatalf("appID = %q, want cli_app", appID)
		}
		if appSecret != "secret" {
			t.Fatalf("appSecret = %q, want secret", appSecret)
		}
		return messenger, nil
	}

	msg := Message{Event: "permission_required", SessionID: "session-123", Workspace: "/path/to/project", Title: "Claude Code 等待授权", Body: "项目: demo"}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if messenger.creatorAppID != "cli_app" {
		t.Fatalf("creator appID = %q, want cli_app", messenger.creatorAppID)
	}
	if messenger.sentReceiveID != "ou_creator" {
		t.Fatalf("receiveOpenID = %q, want ou_creator", messenger.sentReceiveID)
	}
	if messenger.sentCard == nil {
		t.Fatal("sentCard is nil, want card")
	}
	// Verify card has header with title
	header, ok := messenger.sentCard["header"].(map[string]any)
	if !ok {
		t.Fatal("card header is missing")
	}
	title, ok := header["title"].(map[string]any)
	if !ok {
		t.Fatal("card header title is missing")
	}
	if title["content"] == nil {
		t.Fatal("card header title content is missing")
	}
}

func TestFeishuSenderSendReturnsConfigError(t *testing.T) {
	sender := NewFeishuSender(stubFeishuConfigProvider{err: errors.New("missing config")})

	err := sender.Send(context.Background(), Message{Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("Send() error = nil, want config error")
	}
	if err.Error() != "missing config" {
		t.Fatalf("Send() error = %v, want missing config", err)
	}
}

func TestFeishuSenderSendRawCardReturnsMessageID(t *testing.T) {
	provider := stubFeishuConfigProvider{cfg: FeishuCLIConfig{AppID: "app", AppSecret: "secret"}}
	messenger := &stubFeishuMessenger{creatorOpenID: "ou_creator", messageID: "om_123"}
	sender := NewFeishuSender(provider)
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		return messenger, nil
	}

	got, err := sender.SendRawCard(context.Background(), map[string]any{"header": map[string]any{}})
	if err != nil {
		t.Fatalf("SendRawCard() error = %v", err)
	}
	if got != "om_123" {
		t.Fatalf("SendRawCard() message id = %q, want om_123", got)
	}
}

func TestFeishuSenderUpdateRawCardUsesMessageID(t *testing.T) {
	provider := stubFeishuConfigProvider{cfg: FeishuCLIConfig{AppID: "app", AppSecret: "secret"}}
	messenger := &stubFeishuMessenger{}
	sender := NewFeishuSender(provider)
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		return messenger, nil
	}

	card := map[string]any{"header": map[string]any{"title": "updated"}}
	if err := sender.UpdateRawCard(context.Background(), "om_456", card); err != nil {
		t.Fatalf("UpdateRawCard() error = %v", err)
	}
	if messenger.updatedID != "om_456" {
		t.Fatalf("updated id = %q, want om_456", messenger.updatedID)
	}
	if messenger.updatedCard == nil {
		t.Fatal("updated card should be set")
	}
}

func TestProfileFeishuSenderUsesProfileBotAndChatID(t *testing.T) {
	sender, err := NewProfileFeishuSender("codex-main", config.ProfileConfig{
		Feishu: config.ProfileFeishuConfig{
			AppID:       "profile_app",
			AppSecret:   "profile_secret",
			OwnerOpenID: "ou_owner",
			ChatID:      "oc_chat",
		},
	})
	if err != nil {
		t.Fatalf("NewProfileFeishuSender() error = %v", err)
	}
	messenger := &stubFeishuMessenger{messageID: "om_profile"}
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		if appID != "profile_app" || appSecret != "profile_secret" {
			t.Fatalf("app credentials = %q/%q, want profile credentials", appID, appSecret)
		}
		return messenger, nil
	}

	got, err := sender.SendRawCard(context.Background(), map[string]any{"header": map[string]any{}})
	if err != nil {
		t.Fatalf("SendRawCard() error = %v", err)
	}
	if got != "om_profile" {
		t.Fatalf("message id = %q, want om_profile", got)
	}
	if messenger.sentReceiveIDType != "chat_id" || messenger.sentReceiveID != "oc_chat" {
		t.Fatalf("receiver = %s/%s, want chat_id/oc_chat", messenger.sentReceiveIDType, messenger.sentReceiveID)
	}
}

func TestProfileFeishuSenderFallsBackToOwnerOpenID(t *testing.T) {
	sender, err := NewProfileFeishuSender("claude-main", config.ProfileConfig{
		Feishu: config.ProfileFeishuConfig{
			AppID:       "profile_app",
			AppSecret:   "profile_secret",
			OwnerOpenID: "ou_owner",
		},
	})
	if err != nil {
		t.Fatalf("NewProfileFeishuSender() error = %v", err)
	}
	messenger := &stubFeishuMessenger{messageID: "om_profile"}
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		return messenger, nil
	}

	if _, err := sender.SendRawCard(context.Background(), map[string]any{}); err != nil {
		t.Fatalf("SendRawCard() error = %v", err)
	}
	if messenger.sentReceiveIDType != "open_id" || messenger.sentReceiveID != "ou_owner" {
		t.Fatalf("receiver = %s/%s, want open_id/ou_owner", messenger.sentReceiveIDType, messenger.sentReceiveID)
	}
}

func TestBuildCardContainsBody(t *testing.T) {
	sender := &FeishuSender{}
	msg := Message{
		Event:     "permission_required",
		Title:     "测试标题",
		Body:      "这是测试消息内容",
		Workspace: "/test/path",
	}

	card := sender.buildCard(msg)

	// 验证 card 结构
	elements, ok := card["elements"].([]any)
	if !ok {
		t.Fatal("card elements should be a slice")
	}

	// 查找包含 Body 的元素
	found := false
	for _, el := range elements {
		if elMap, ok := el.(map[string]any); ok {
			if text, ok := elMap["text"].(map[string]any); ok {
				if content, ok := text["content"].(string); ok {
					if contains(content, "这是测试消息内容") {
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Error("card should contain message body content")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildCardFooterDoesNotHardcodeClaudeCode(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{Event: "run_completed", Title: "Codex 运行完成", Body: "done"})

	elements, ok := card["elements"].([]any)
	if !ok {
		t.Fatal("card elements should be a slice")
	}

	foundClaudeCode := false
	for _, el := range elements {
		elMap, ok := el.(map[string]any)
		if !ok || elMap["tag"] != "note" {
			continue
		}
		noteElements, ok := elMap["elements"].([]any)
		if !ok {
			continue
		}
		for _, noteEl := range noteElements {
			noteMap, ok := noteEl.(map[string]any)
			if !ok {
				continue
			}
			content, _ := noteMap["content"].(string)
			if contains(content, "Claude Code") {
				foundClaudeCode = true
			}
		}
	}

	if foundClaudeCode {
		t.Fatal("card footer should not hardcode Claude Code")
	}
}

func TestBuildCardOmitsWorkspaceForCodexNotification(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{Event: "run_completed", Title: "运行完成", Body: "done", Workspace: "/tmp/project", Agent: "codex"})

	elements, ok := card["elements"].([]any)
	if !ok {
		t.Fatal("card elements should be a slice")
	}

	for _, el := range elements {
		elMap, ok := el.(map[string]any)
		if !ok {
			continue
		}
		text, ok := elMap["text"].(map[string]any)
		if !ok {
			continue
		}
		content, _ := text["content"].(string)
		if contains(content, "**工作目录**") {
			t.Fatalf("card should omit workspace for Codex notification, got %q", content)
		}
	}
}

func TestBuildApprovalCardHidesApprovalIDText(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Event:         "permission_required",
		Title:         "Claude Code 等待授权",
		Body:          "工具: Bash\n授权内容:\ngit status",
		ApprovalID:    "ap_123",
		ApprovalToken: "token",
	})

	allText := cardAllText(card)
	if contains(allText, "审批 ID") || contains(allText, "文本备用") || contains(allText, "ap_123") {
		t.Fatalf("approval card should hide approval id text, got %q", allText)
	}
	if !contains(allText, "git status") {
		t.Fatalf("approval card should include authorization content, got %q", allText)
	}
}

func TestBuildPlainPermissionCardHasNoApprovalButtons(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Event: "permission_required",
		Title: "Codex 等待授权",
		Body:  "工具: Bash\n授权内容:\ngit status",
		Agent: "codex",
	})

	allText := cardAllText(card)
	if contains(allText, "批准") || contains(allText, "拒绝") {
		t.Fatalf("plain permission card should not show approval buttons, got %q", allText)
	}
	if !contains(allText, "请回电脑授权") {
		t.Fatalf("plain permission card should explain local authorization, got %q", allText)
	}
}

func TestBuildApprovalCardIncludesProfileInButtonValues(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Event:         "permission_required",
		Title:         "Codex 等待授权",
		Body:          "工具: Bash",
		Profile:       "codex-main",
		ApprovalID:    "ap_123",
		ApprovalToken: "token",
	})

	if !cardValueContains(card, "profile", "codex-main") {
		t.Fatalf("approval card should include profile in button values: %#v", card)
	}
}

func cardAllText(value any) string {
	switch v := value.(type) {
	case map[string]any:
		var out string
		for key, item := range v {
			if key == "value" {
				continue
			}
			out += cardAllText(item)
		}
		return out
	case []any:
		var out string
		for _, item := range v {
			out += cardAllText(item)
		}
		return out
	case string:
		return v + "\n"
	default:
		return ""
	}
}

func cardValueContains(value any, key string, want string) bool {
	switch v := value.(type) {
	case map[string]any:
		if got, ok := v[key].(string); ok && got == want {
			return true
		}
		for _, item := range v {
			if cardValueContains(item, key, want) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if cardValueContains(item, key, want) {
				return true
			}
		}
	}
	return false
}
