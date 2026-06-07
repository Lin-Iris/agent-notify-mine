package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	pushPlusHTTPTimeout      = 10 * time.Second
	pushPlusMaxErrorBodySize = 512
)

// PushPlusSender 通过 PushPlus (推送加) 微信公众号推送通知。
type PushPlusSender struct {
	token      string
	httpClient *http.Client
}

// NewPushPlusSender 创建一个 PushPlus 发送器。
func NewPushPlusSender(token string) *PushPlusSender {
	return &PushPlusSender{
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: pushPlusHTTPTimeout},
	}
}

func (s *PushPlusSender) Name() string { return "pushplus" }

func (s *PushPlusSender) Send(ctx context.Context, msg Message) error {
	if s.token == "" {
		return fmt.Errorf("pushplus: token 未配置")
	}

	data := url.Values{}
	data.Set("token", s.token)
	data.Set("title", msg.Title)
	data.Set("content", msg.Body)
	data.Set("template", "txt")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.pushplus.plus/send",
		strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("pushplus: 创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushplus: 发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, pushPlusMaxErrorBodySize))
		return fmt.Errorf("pushplus: HTTP %d: %s", resp.StatusCode, string(body))
	}

	// PushPlus 返回 JSON: {"code":200,"msg":"success"}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("pushplus: 解析响应失败: %w", err)
	}
	if result.Code != 200 {
		return fmt.Errorf("pushplus: API 错误 code=%d msg=%s", result.Code, result.Msg)
	}

	return nil
}
