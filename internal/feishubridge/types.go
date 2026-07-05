package feishubridge

import "context"

type Handler interface {
	HandleText(ctx context.Context, profile, chatID, senderID, text string) error
}

type CardActionHandler interface {
	HandleCardAction(ctx context.Context, profile, operatorID string, value map[string]any) error
}

type Gateway struct {
	profile   string
	appID     string
	appSecret string
	handler   Handler
}

func NewGateway(handler Handler) *Gateway {
	return &Gateway{handler: handler}
}

func NewProfileGateway(profile, appID, appSecret string, handler Handler) *Gateway {
	return &Gateway{profile: profile, appID: appID, appSecret: appSecret, handler: handler}
}
