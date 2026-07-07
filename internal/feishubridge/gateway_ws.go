package feishubridge

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

func (g *Gateway) Start(ctx context.Context, cfg config.Config) error {
	appID := g.appID
	appSecret := g.appSecret
	if appID == "" || appSecret == "" {
		feishuCfg, err := feishucli.ParseConfig()
		if err != nil {
			return err
		}
		appID = feishuCfg.AppID
		appSecret = feishuCfg.AppSecret
	}
	eventHandler := dispatcher.NewEventDispatcher("", "")
	eventHandler.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		if g.handler == nil || event.Event == nil || event.Event.Message == nil {
			return nil
		}
		message := event.Event.Message
		chatID := stringPtr(message.ChatId)
		senderID := senderOpenID(event.Event.Sender)
		text := extractText(message.Content)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return g.handler.HandleText(ctx, g.profile, chatID, senderID, text)
	})
	eventHandler.OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
		cardHandler, ok := g.handler.(CardActionHandler)
		if !ok || event.Event == nil || event.Event.Action == nil {
			return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "info", Content: "未处理"}}, nil
		}
		operator := ""
		if event.Event.Operator != nil {
			operator = event.Event.Operator.OpenID
		}
		value := event.Event.Action.Value
		if value == nil {
			value = map[string]any{}
		}
		if event.Event.Action.FormValue != nil {
			value["_form_value"] = event.Event.Action.FormValue
		}
		if event.Event.Action.InputValue != "" {
			value["_input_value"] = event.Event.Action.InputValue
		}
		if len(event.Event.Action.Options) > 0 {
			value["_options"] = event.Event.Action.Options
		}
		if event.Event.Action.Option != "" {
			value["_option"] = event.Event.Action.Option
		}
		if err := cardHandler.HandleCardAction(ctx, g.profile, operator, value); err != nil {
			return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "error", Content: err.Error()}}, nil
		}
		return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "success", Content: "已处理"}}, nil
	})
	client := ws.NewClient(appID, appSecret, ws.WithEventHandler(eventHandler))
	return client.Start(ctx)
}

func stringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func senderOpenID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil || sender.SenderId.OpenId == nil {
		return ""
	}
	return *sender.SenderId.OpenId
}

func extractText(raw *string) string {
	if raw == nil {
		return ""
	}
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*raw), &content); err == nil && content.Text != "" {
		return content.Text
	}
	return *raw
}
