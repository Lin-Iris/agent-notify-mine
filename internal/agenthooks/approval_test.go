package agenthooks

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/approval"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
)

func TestMaybeHandleApprovalFallsBackToDefaultProfileWhenEnvVarEmpty(t *testing.T) {
	// When AGENT_NOTIFY_REMOTE_PROFILE is not set but broker is enabled
	// and a default profile has Feishu credentials, locally-started
	// sessions should still enter the remote approval flow so users
	// can approve from mobile.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "")
	cfg := config.Default()
	cfg.Broker.Enabled = true
	cfg.Approval.Enabled = true
	cfg.Approval.TimeoutSeconds = 1
	cfg.Profiles["claude-main"] = config.ProfileConfig{
		Agent:   "claude",
		Enabled: true,
		Feishu: config.ProfileFeishuConfig{
			AppID:       "cli_a",
			AppSecret:   "secret",
			OwnerOpenID: "ou_owner",
		},
	}
	cfg.Profiles["codex-main"] = config.ProfileConfig{
		Agent:   "codex",
		Enabled: true,
		Feishu: config.ProfileFeishuConfig{
			AppID:       "cli_a",
			AppSecret:   "secret",
			OwnerOpenID: "ou_owner",
		},
	}

	for _, tc := range []struct {
		name  string
		agent string
	}{
		{name: "codex", agent: "codex"},
		{name: "claude", agent: "claude_code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withApprovalPrompt(t, func(ctx context.Context, cfg config.Config, logPath string, evt event.Event, req approval.Request, profileName string, profile config.ProfileConfig) error {
				return nil
			})

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var stdout bytes.Buffer
			handled, err := MaybeHandleApproval(ctx, cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", event.Event{
				Agent:     tc.agent,
				HookEvent: "PermissionRequest",
				Status:    event.StatusPermissionReq,
				SessionID: "s1",
			}, &stdout)
			if err != nil {
				t.Fatalf("MaybeHandleApproval() error = %v", err)
			}
			if !handled {
				t.Fatal("should enter remote approval flow via default profile fallback")
			}
		})
	}
}

func TestMaybeHandleApprovalSkipsWhenBrokerDisabledAndEnvVarEmpty(t *testing.T) {
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "")
	cfg := config.Default()
	cfg.Approval.Enabled = true
	cfg.Broker.Enabled = false // broker not running
	cfg.Profiles["claude-main"] = config.ProfileConfig{
		Agent:   "claude",
		Enabled: true,
		Feishu: config.ProfileFeishuConfig{
			AppID:       "cli_a",
			AppSecret:   "secret",
			OwnerOpenID: "ou_owner",
		},
	}

	handled, err := MaybeHandleApproval(context.Background(), cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", event.Event{
		Agent:     "claude_code",
		HookEvent: "PermissionRequest",
		Status:    event.StatusPermissionReq,
		SessionID: "s1",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if handled {
		t.Fatal("should skip remote approval when broker is disabled")
	}
}

func TestMaybeHandleApprovalSkipsDisabledRemoteProfile(t *testing.T) {
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "codex-main")
	cfg := config.Default()
	cfg.Approval.Enabled = true
	cfg.Broker.Enabled = true
	profile := cfg.Profiles["codex-main"]
	profile.Enabled = false
	cfg.Profiles["codex-main"] = profile

	handled, err := MaybeHandleApproval(context.Background(), cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", event.Event{
		Agent:     "codex",
		HookEvent: "PermissionRequest",
		Status:    event.StatusPermissionReq,
		SessionID: "s1",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if handled {
		t.Fatal("disabled remote profile should not enter approval flow")
	}
}

func TestMaybeHandleApprovalWritesAllowAfterRemoteApproval(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "claude-main")
	cfg := remoteApprovalTestConfig()

	withApprovalPrompt(t, func(ctx context.Context, cfg config.Config, logPath string, evt event.Event, req approval.Request, profileName string, profile config.ProfileConfig) error {
		decideApproval(t, req, approval.DecisionApprove)
		return nil
	})

	var stdout bytes.Buffer
	handled, err := MaybeHandleApproval(context.Background(), cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", testPermissionEvent(), &stdout)
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if !handled {
		t.Fatal("remote approval should be handled")
	}
	got := decodeHookDecision(t, stdout.Bytes())
	if got.HookSpecificOutput.Decision == nil || got.HookSpecificOutput.Decision.Behavior != hookPermissionBehaviorAllow {
		t.Fatalf("Decision = %#v, want behavior %q", got.HookSpecificOutput.Decision, hookPermissionBehaviorAllow)
	}
}

func TestMaybeHandleApprovalWritesAllowForRemoteCodexApproval(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "codex-main")
	cfg := remoteApprovalTestConfig()

	withApprovalPrompt(t, func(ctx context.Context, cfg config.Config, logPath string, evt event.Event, req approval.Request, profileName string, profile config.ProfileConfig) error {
		if profileName != "codex-main" {
			t.Fatalf("profileName = %q, want codex-main", profileName)
		}
		if profile.Agent != "codex" {
			t.Fatalf("profile.Agent = %q, want codex", profile.Agent)
		}
		decideApproval(t, req, approval.DecisionApprove)
		return nil
	})

	var stdout bytes.Buffer
	handled, err := MaybeHandleApproval(context.Background(), cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", testCodexPermissionEvent(), &stdout)
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if !handled {
		t.Fatal("remote codex approval should be handled")
	}
	got := decodeCodexHookDecision(t, stdout.Bytes())
	if got.Decision != codexHookDecisionAllow {
		t.Fatalf("Decision = %#v, want %q", got, codexHookDecisionAllow)
	}
}

func TestMaybeHandleApprovalWritesDenyAfterRemoteDenial(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "claude-main")
	cfg := remoteApprovalTestConfig()

	withApprovalPrompt(t, func(ctx context.Context, cfg config.Config, logPath string, evt event.Event, req approval.Request, profileName string, profile config.ProfileConfig) error {
		decideApproval(t, req, approval.DecisionDeny)
		return nil
	})

	var stdout bytes.Buffer
	handled, err := MaybeHandleApproval(context.Background(), cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", testPermissionEvent(), &stdout)
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if !handled {
		t.Fatal("remote approval should be handled")
	}
	got := decodeHookDecision(t, stdout.Bytes())
	if got.HookSpecificOutput.Decision == nil || got.HookSpecificOutput.Decision.Behavior != hookPermissionBehaviorDeny {
		t.Fatalf("Decision = %#v, want behavior %q", got.HookSpecificOutput.Decision, hookPermissionBehaviorDeny)
	}
}

func TestMaybeHandleApprovalWritesDenyWhenWaitContextExpires(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "claude-main")
	cfg := remoteApprovalTestConfig()
	withApprovalPrompt(t, func(ctx context.Context, cfg config.Config, logPath string, evt event.Event, req approval.Request, profileName string, profile config.ProfileConfig) error {
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var stdout bytes.Buffer
	handled, err := MaybeHandleApproval(ctx, cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", testPermissionEvent(), &stdout)
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if !handled {
		t.Fatal("remote approval should be handled")
	}
	got := decodeHookDecision(t, stdout.Bytes())
	if got.HookSpecificOutput.Decision == nil || got.HookSpecificOutput.Decision.Behavior != hookPermissionBehaviorDeny {
		t.Fatalf("Decision = %#v, want behavior %q", got.HookSpecificOutput.Decision, hookPermissionBehaviorDeny)
	}
}

func remoteApprovalTestConfig() config.Config {
	cfg := config.Default()
	cfg.Broker.Enabled = true
	cfg.Approval.Enabled = true
	cfg.Profiles["claude-main"] = config.ProfileConfig{
		Agent:   "claude",
		Enabled: true,
		Feishu: config.ProfileFeishuConfig{
			AppID:       "cli_a",
			AppSecret:   "secret",
			OwnerOpenID: "ou_owner",
		},
	}
	cfg.Profiles["codex-main"] = config.ProfileConfig{
		Agent:   "codex",
		Enabled: true,
		Feishu: config.ProfileFeishuConfig{
			AppID:       "cli_a",
			AppSecret:   "secret",
			OwnerOpenID: "ou_owner",
		},
	}
	return cfg
}

func withApprovalPrompt(t *testing.T, fn func(context.Context, config.Config, string, event.Event, approval.Request, string, config.ProfileConfig) error) {
	t.Helper()
	old := sendApprovalPromptForHook
	sendApprovalPromptForHook = fn
	t.Cleanup(func() {
		sendApprovalPromptForHook = old
	})
}

func decideApproval(t *testing.T, req approval.Request, decision approval.Decision) {
	t.Helper()
	path, err := config.ApprovalPath()
	if err != nil {
		t.Errorf("ApprovalPath() error = %v", err)
		return
	}
	if _, err := approval.NewStore(path).Decide(req.ApprovalID, req.Token, "ou_owner", decision, "test decision"); err != nil {
		t.Errorf("Decide() error = %v", err)
	}
}

func decodeHookDecision(t *testing.T, raw []byte) hookDecisionOutput {
	t.Helper()
	var out hookDecisionOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("hook decision JSON = %q, error = %v", string(raw), err)
	}
	return out
}

func decodeCodexHookDecision(t *testing.T, raw []byte) codexHookDecisionOutput {
	t.Helper()
	var out codexHookDecisionOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("codex hook decision JSON = %q, error = %v", string(raw), err)
	}
	return out
}

func testPermissionEvent() event.Event {
	raw, _ := json.Marshal(map[string]any{
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git status",
		},
	})
	return event.Event{
		Agent:      "claude_code",
		HookEvent:  "PermissionRequest",
		Status:     event.StatusPermissionReq,
		SessionID:  "s1",
		Workspace:  "/tmp/project",
		RawPayload: raw,
	}
}

func testCodexPermissionEvent() event.Event {
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "PermissionRequest",
		"tool_name":       "Bash",
		"tool_input": map[string]any{
			"command": "git status",
		},
	})
	return event.Event{
		Agent:      "codex",
		HookEvent:  "PermissionRequest",
		Status:     event.StatusPermissionReq,
		SessionID:  "s1",
		Workspace:  "/tmp/project",
		RawPayload: raw,
	}
}
