package feishucli

import (
	"context"

	larksuite "github.com/hellolib/client-tools/app/larksuite"
)

type Config struct {
	AppID      string
	AppSecret  string
	UserOpenID string
	UserName   string
}

var (
	prepareCLI = larksuite.PrepareLarkCLI
	parseCLI   = larksuite.ParseCliConfig
)

func ParseConfig() (Config, error) {
	cfg, err := parseCLI()
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppID:      cfg.AppID,
		AppSecret:  cfg.AppSecret,
		UserOpenID: cfg.UserOpenId,
		UserName:   cfg.UserName,
	}, nil
}

func EnsureReady(ctx context.Context) (Config, error) {
	cfg, err := ParseConfig()
	if err == nil && cfg.AppID != "" && cfg.AppSecret != "" && cfg.UserOpenID != "" {
		return cfg, nil
	}

	// Config missing, incomplete, or no authorized user — re-run QR setup.
	if err := prepareCLI(ctx); err != nil {
		return Config{}, err
	}

	return ParseConfig()
}

func Reinitialize(ctx context.Context) (Config, error) {
	if err := prepareCLI(ctx); err != nil {
		return Config{}, err
	}
	return ParseConfig()
}
