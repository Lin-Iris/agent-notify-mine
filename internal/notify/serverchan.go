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
	serverChanHTTPTimeout      = 10 * time.Second
	serverChanMaxErrorBodySize = 512
)

// ServerChanSender 通过 Server酱 (ServerChan) 微信公众号推送通知。
type ServerChanSender struct {
	sendKey    string
	httpClient *http.Client
}

// NewServerChanSender 创建一个 Server酱 发送器。
func NewServerChanSender(sendKey string) *ServerChanSender {
	return &ServerChanSender{
		sendKey:    strings.TrimSpace(sendKey),
		httpClient: &http.Client{Timeout: serverChanHTTPTimeout},
	}
}

func (s *ServerChanSender) Name() string { return "serverchan" }

func (s *ServerChanSender) Send(ctx context.Context, msg Message) error {
	if s.sendKey == "" {
		return fmt.Errorf("serverchan: send_key 未配置")
	}

	endpoint := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", s.sendKey)

	data := url.Values{}
	data.Set("title", msg.Title)
	data.Set("desp", msg.Body)
	data.Set("channel", "9") // 微信公众号通道

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("serverchan: 创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("serverchan: 发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, serverChanMaxErrorBodySize))
		return fmt.Errorf("serverchan: HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Server酱 返回 JSON: {"code":0,"message":"success"}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("serverchan: 解析响应失败: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("serverchan: API 错误 code=%d msg=%s", result.Code, result.Message)
	}

	return nil
}
