package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	wxPusherHTTPTimeout      = 10 * time.Second
	wxPusherMaxErrorBodySize = 512
)

// WxPusherSender 通过 WxPusher 微信公众号推送通知。
type WxPusherSender struct {
	appToken   string
	uid        string
	httpClient *http.Client
}

// NewWxPusherSender 创建一个 WxPusher 发送器。
func NewWxPusherSender(appToken, uid string) *WxPusherSender {
	return &WxPusherSender{
		appToken:   strings.TrimSpace(appToken),
		uid:        strings.TrimSpace(uid),
		httpClient: &http.Client{Timeout: wxPusherHTTPTimeout},
	}
}

// 辅助函数：截断字符串用于通知栏摘要（WxPusher 限制 100 字以内）
func truncateForSummary(s string) string {
	runes := []rune(s)
	if len(runes) > 80 {
		return string(runes[:77]) + "..."
	}
	return s
}

// 辅助函数：去除 HTML 标签
func stripHTML(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	return strings.TrimSpace(buf.String())
}

func (s *WxPusherSender) Name() string { return "wxpusher" }

func (s *WxPusherSender) Send(ctx context.Context, msg Message) error {
	if s.appToken == "" || s.uid == "" {
		return fmt.Errorf("wxpusher: appToken 或 uid 未配置")
	}

	// 构建推送内容
	content := fmt.Sprintf("<h3>%s</h3><p>%s</p>", msg.Title, msg.Body)

	// 通知栏摘要（显示在微信通知消息中）
	summary := truncateForSummary(msg.Title + " - " + stripHTML(msg.Body))

	payload := map[string]interface{}{
		"appToken":    s.appToken,
		"content":     content,
		"contentType": 2, // 2 = HTML
		"title":       msg.Title,
		"summary":     summary,
		"uids":        []string{s.uid},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("wxpusher: 序列化失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://wxpusher.zjiecode.com/api/send/message",
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wxpusher: 创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wxpusher: 发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, wxPusherMaxErrorBodySize))
		return fmt.Errorf("wxpusher: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// WxPusher 返回 JSON: {"success":true,"code":1000,"msg":"..."}
	var result struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("wxpusher: 解析响应失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("wxpusher: API 错误 code=%d msg=%s", result.Code, result.Msg)
	}

	return nil
}
