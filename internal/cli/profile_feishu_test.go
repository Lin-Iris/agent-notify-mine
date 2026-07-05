package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
)

func TestProfileFeishuCommandSavesBotConfigAndHidesSecret(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".agent-notify", "config.yaml")
	cfg := config.Default()
	cfg.Profiles["codex-main"] = config.ProfileConfig{
		Agent:          "codex",
		Enabled:        true,
		PermissionMode: "workspace-write",
		Workspaces:     map[string]string{},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var stdout bytes.Buffer
	err := Run(context.Background(), []string{
		"profile", "feishu", "codex-main",
		"--app-id", "cli_app",
		"--app-secret", "super_secret",
		"--owner-open-id", "ou_owner",
		"--chat-id", "oc_chat",
	}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(stdout.String(), "super_secret") {
		t.Fatalf("stdout leaked app secret: %q", stdout.String())
	}

	got, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	feishu := got.Profiles["codex-main"].Feishu
	if feishu.AppID != "cli_app" || feishu.AppSecret != "super_secret" || feishu.OwnerOpenID != "ou_owner" || feishu.ChatID != "oc_chat" {
		t.Fatalf("profile feishu = %#v, want saved bot config", feishu)
	}
}

func TestProfileListDoesNotLeakFeishuSecret(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".agent-notify", "config.yaml")
	cfg := config.Default()
	profile := cfg.Profiles["claude-main"]
	profile.Feishu = config.ProfileFeishuConfig{
		AppID:       "cli_app",
		AppSecret:   "super_secret",
		OwnerOpenID: "ou_owner",
		ChatID:      "oc_chat",
	}
	cfg.Profiles["claude-main"] = profile
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"profile", "list"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "feishu=bound chat_id=oc_chat") {
		t.Fatalf("stdout = %q, want bound feishu status", out)
	}
	if strings.Contains(out, "super_secret") {
		t.Fatalf("stdout leaked app secret: %q", out)
	}
}

func TestProfileFeishuSetupScansAndSavesBotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".agent-notify", "config.yaml")
	cfg := config.Default()
	cfg.Profiles["claude-main"] = config.ProfileConfig{
		Agent:          "claude",
		Enabled:        true,
		PermissionMode: "workspace-write",
		Workspaces:     map[string]string{},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	restore := resetProfileFeishuReinitializeForTest(func(ctx context.Context) (feishucli.Config, error) {
		return feishucli.Config{
			AppID:      "scan_app",
			AppSecret:  "scan_secret",
			UserOpenID: "ou_scan_owner",
			UserName:   "Victoria",
		}, nil
	})
	defer restore()

	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"profile", "feishu", "setup", "claude-main"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, "scan_secret") {
		t.Fatalf("stdout leaked app secret: %q", out)
	}

	got, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	feishu := got.Profiles["claude-main"].Feishu
	if feishu.AppID != "scan_app" || feishu.AppSecret != "scan_secret" || feishu.OwnerOpenID != "ou_scan_owner" {
		t.Fatalf("profile feishu = %#v, want scanned bot config", feishu)
	}
}

func TestProfileFeishuSetupFallsBackToAppCreatorOpenID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".agent-notify", "config.yaml")
	cfg := config.Default()
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	restoreReinit := resetProfileFeishuReinitializeForTest(func(ctx context.Context) (feishucli.Config, error) {
		return feishucli.Config{
			AppID:     "scan_app",
			AppSecret: "scan_secret",
		}, nil
	})
	defer restoreReinit()
	oldResolve := profileFeishuResolveOwnerOpenID
	profileFeishuResolveOwnerOpenID = func(ctx context.Context, appID, appSecret string) (string, error) {
		if appID != "scan_app" || appSecret != "scan_secret" {
			t.Fatalf("resolver credentials = %q/%q, want scanned credentials", appID, appSecret)
		}
		return "ou_creator", nil
	}
	defer func() { profileFeishuResolveOwnerOpenID = oldResolve }()

	if err := Run(context.Background(), []string{"profile", "feishu", "setup", "claude-main"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Profiles["claude-main"].Feishu.OwnerOpenID != "ou_creator" {
		t.Fatalf("owner open id = %q, want ou_creator", got.Profiles["claude-main"].Feishu.OwnerOpenID)
	}
}

func TestAuthorizedFeishuOperatorForProfileUsesProfileOwner(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles["claude-main"] = config.ProfileConfig{
		Agent: "claude",
		Feishu: config.ProfileFeishuConfig{
			OwnerOpenID: "ou_claude",
			ChatID:      "oc_claude",
		},
	}
	cfg.Profiles["codex-main"] = config.ProfileConfig{
		Agent: "codex",
		Feishu: config.ProfileFeishuConfig{
			OwnerOpenID: "ou_codex",
			ChatID:      "oc_codex",
		},
	}

	if !authorizedFeishuOperatorForProfile(cfg, "claude-main", "ou_claude", "oc_claude") {
		t.Fatal("claude owner should be authorized for claude profile")
	}
	if authorizedFeishuOperatorForProfile(cfg, "claude-main", "ou_codex", "oc_claude") {
		t.Fatal("codex owner should not be authorized for claude profile")
	}
	if authorizedFeishuOperatorForProfile(cfg, "codex-main", "ou_codex", "oc_claude") {
		t.Fatal("codex owner should not be authorized from claude chat")
	}
}
