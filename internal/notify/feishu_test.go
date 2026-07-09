package notify

import (
	"context"
	"errors"
	"strings"
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
	sentCards         []map[string]any
	sentPosts         []map[string]any
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
	m.sentCards = append(m.sentCards, card)
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

func (m *stubFeishuMessenger) SendPost(ctx context.Context, receiveIDType, receiveID string, post map[string]any) error {
	m.sentReceiveIDType = receiveIDType
	m.sentReceiveID = receiveID
	m.sentPosts = append(m.sentPosts, post)
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

func TestFeishuSenderSendLongTextUsesMarkdownCards(t *testing.T) {
	provider := stubFeishuConfigProvider{cfg: FeishuCLIConfig{AppID: "app", AppSecret: "secret", ReceiveID: "ou_owner", ReceiveIDType: "open_id"}}
	messenger := &stubFeishuMessenger{messageID: "om_md"}
	sender := NewFeishuSender(provider)
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		return messenger, nil
	}

	if err := sender.SendLongText(context.Background(), "模型输出 task_1", "# 标题\n\n- A\n- B\n\n```go\nfmt.Println(\"ok\")\n```"); err != nil {
		t.Fatalf("SendLongText() error = %v", err)
	}
	if len(messenger.sentCards) != 1 {
		t.Fatalf("sent cards = %d, want 1", len(messenger.sentCards))
	}
	if !cardTextContains(messenger.sentCards[0], "# 标题") {
		t.Fatalf("markdown card should include rendered content: %#v", messenger.sentCards[0])
	}
	if !cardHasTextTag(messenger.sentCards[0], "lark_md") {
		t.Fatalf("long text card should use lark_md: %#v", messenger.sentCards[0])
	}
}

func TestFeishuSenderSendMarkdownPostUsesRichTextBubble(t *testing.T) {
	provider := stubFeishuConfigProvider{cfg: FeishuCLIConfig{AppID: "app", AppSecret: "secret", ReceiveID: "ou_owner", ReceiveIDType: "open_id"}}
	messenger := &stubFeishuMessenger{}
	sender := NewFeishuSender(provider)
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		return messenger, nil
	}

	text := "# 标题\n\n- A\n- **B**\n\n[链接](https://example.com)\n\n```go\nfmt.Println(\"ok\")\n```"
	if err := sender.SendMarkdownPost(context.Background(), "模型输出 task_1", text); err != nil {
		t.Fatalf("SendMarkdownPost() error = %v", err)
	}
	if len(messenger.sentCards) != 0 {
		t.Fatalf("sent cards = %d, want 0", len(messenger.sentCards))
	}
	if len(messenger.sentPosts) != 1 {
		t.Fatalf("sent posts = %d, want 1", len(messenger.sentPosts))
	}
	if _, ok := messenger.sentPosts[0]["post"]; ok {
		t.Fatalf("IM post content should not include helpdesk-style post wrapper: %#v", messenger.sentPosts[0])
	}
	if _, ok := messenger.sentPosts[0]["zh_cn"]; !ok {
		t.Fatalf("IM post content should include zh_cn at top level: %#v", messenger.sentPosts[0])
	}
	if !postTextContains(messenger.sentPosts[0], "标题") || !postTextContains(messenger.sentPosts[0], "fmt.Println") {
		t.Fatalf("post should include markdown content: %#v", messenger.sentPosts[0])
	}
	if !postHasTag(messenger.sentPosts[0], "a") {
		t.Fatalf("post should include link element: %#v", messenger.sentPosts[0])
	}
	if postHasKey(messenger.sentPosts[0], "style") {
		t.Fatalf("post should not include unsupported style fields: %#v", messenger.sentPosts[0])
	}
	if !postHasKey(messenger.sentPosts[0], "un_escape") {
		t.Fatalf("post text elements should include un_escape: %#v", messenger.sentPosts[0])
	}
}

func TestFeishuSenderSendMarkdownPostSplitsLongContentIntoPosts(t *testing.T) {
	provider := stubFeishuConfigProvider{cfg: FeishuCLIConfig{AppID: "app", AppSecret: "secret", ReceiveID: "ou_owner", ReceiveIDType: "open_id"}}
	messenger := &stubFeishuMessenger{}
	sender := NewFeishuSender(provider)
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		return messenger, nil
	}

	if err := sender.SendMarkdownPost(context.Background(), "模型输出 task_1", strings.Repeat("很长的 Markdown 内容\n\n", 400)); err != nil {
		t.Fatalf("SendMarkdownPost() error = %v", err)
	}
	if len(messenger.sentPosts) < 2 {
		t.Fatalf("sent posts = %d, want at least 2", len(messenger.sentPosts))
	}
	if len(messenger.sentCards) != 0 {
		t.Fatalf("sent cards = %d, want 0", len(messenger.sentCards))
	}
	if !postTitleContains(messenger.sentPosts[0], "(1/") {
		t.Fatalf("first post should include sequence title: %#v", messenger.sentPosts[0])
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

func TestBuildPlainInputCardHasNoApprovalButtons(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Event: "input_required",
		Title: "Claude Code 等待输入",
		Body:  "提示: Claude is waiting for your input",
		Agent: "claude_code",
	})

	allText := cardAllText(card)
	if contains(allText, "批准") || contains(allText, "拒绝") {
		t.Fatalf("plain input card should not show approval buttons, got %q", allText)
	}
}

func TestBuildInputRequestCardIncludesOptions(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Event:           "input_required",
		Title:           "Claude Code 等待输入",
		Body:            "提示: 能看到这个弹窗吗？",
		Profile:         "claude-main",
		InputRequestID:  "in_123",
		InputToken:      "token",
		InputPrompt:     "能看到这个弹窗吗？",
		InputOptions:    []string{"看到了", "没看到", "Other"},
		InputAllowOther: true,
	})

	allText := cardAllText(card)
	for _, want := range []string{"看到了", "没看到", "Other"} {
		if !contains(allText, want) {
			t.Fatalf("input card should include option %q, got %q", want, allText)
		}
	}
	if !cardValueContains(card, "action", "input_submit") {
		t.Fatalf("input card should include input_submit action: %#v", card)
	}
	if !cardValueContains(card, "input_id", "in_123") {
		t.Fatalf("input card should include input_id: %#v", card)
	}
	if countCardActions(card, "input_submit") != 3 {
		t.Fatalf("input card should include one submit action per option in this fixture: %#v", card)
	}
}

func TestBuildInputRequestCardUsesButtonsForMultiSelectFallback(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Event:            "input_required",
		Title:            "Claude Code 等待输入",
		Profile:          "claude-main",
		InputRequestID:   "in_123",
		InputToken:       "token",
		InputPrompt:      "选择多个",
		InputOptions:     []string{"A", "B"},
		InputMultiSelect: true,
	})

	if countCardActions(card, "input_submit") != 2 {
		t.Fatalf("input card should include one submit action per option: %#v", card)
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

func cardHasTextTag(value any, tag string) bool {
	switch v := value.(type) {
	case map[string]any:
		if v["tag"] == tag {
			return true
		}
		for _, item := range v {
			if cardHasTextTag(item, tag) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if cardHasTextTag(item, tag) {
				return true
			}
		}
	}
	return false
}

func cardHasTag(value any, tag string) bool {
	switch v := value.(type) {
	case map[string]any:
		if v["tag"] == tag {
			return true
		}
		for _, item := range v {
			if cardHasTag(item, tag) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if cardHasTag(item, tag) {
				return true
			}
		}
	}
	return false
}

func countCardActions(value any, action string) int {
	switch v := value.(type) {
	case map[string]any:
		count := 0
		if value, ok := v["value"].(map[string]any); ok && value["action"] == action {
			count++
		}
		for _, item := range v {
			count += countCardActions(item, action)
		}
		return count
	case []any:
		count := 0
		for _, item := range v {
			count += countCardActions(item, action)
		}
		return count
	default:
		return 0
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

func postTextContains(value any, want string) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if key == "text" {
				if text, ok := item.(string); ok && strings.Contains(text, want) {
					return true
				}
				continue
			}
			if postTextContains(item, want) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if postTextContains(item, want) {
				return true
			}
		}
	case []map[string]any:
		for _, item := range v {
			if postTextContains(item, want) {
				return true
			}
		}
	case [][]map[string]any:
		for _, row := range v {
			if postTextContains(row, want) {
				return true
			}
		}
	case string:
		return strings.Contains(v, want)
	}
	return false
}

func postHasTag(value any, tag string) bool {
	switch v := value.(type) {
	case map[string]any:
		if v["tag"] == tag {
			return true
		}
		for _, item := range v {
			if postHasTag(item, tag) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if postHasTag(item, tag) {
				return true
			}
		}
	case []map[string]any:
		for _, item := range v {
			if postHasTag(item, tag) {
				return true
			}
		}
	case [][]map[string]any:
		for _, row := range v {
			if postHasTag(row, tag) {
				return true
			}
		}
	}
	return false
}

func postHasKey(value any, key string) bool {
	switch v := value.(type) {
	case map[string]any:
		if _, ok := v[key]; ok {
			return true
		}
		for _, item := range v {
			if postHasKey(item, key) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if postHasKey(item, key) {
				return true
			}
		}
	case []map[string]any:
		for _, item := range v {
			if postHasKey(item, key) {
				return true
			}
		}
	case [][]map[string]any:
		for _, row := range v {
			if postHasKey(row, key) {
				return true
			}
		}
	}
	return false
}

func postTitleContains(post map[string]any, want string) bool {
	zhCN, _ := post["zh_cn"].(map[string]any)
	title, _ := zhCN["title"].(string)
	return strings.Contains(title, want)
}
