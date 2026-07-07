package agenthooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/inputrequest"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
)

var sendInputPromptForHook = sendInputPrompt

func MaybeHandleInput(ctx context.Context, cfg config.Config, statePath, logPath string, evt event.Event, stdout io.Writer) (bool, error) {
	if evt.Agent != "claude_code" || evt.Status != event.StatusInputRequired {
		return false, nil
	}

	profileName := os.Getenv("AGENT_NOTIFY_REMOTE_PROFILE")
	if profileName == "" {
		profileName = defaultProfileForAgent(evt.Agent)
	}
	if profileName == "" {
		return false, state.AppendLog(logPath, fmt.Sprintf("input request not available: no profile for agent %s", evt.Agent))
	}
	if !cfg.Broker.Enabled {
		return false, state.AppendLog(logPath, "remote input skipped: broker disabled")
	}
	profile, ok := cfg.Profiles[profileName]
	if !ok || !profile.Enabled {
		return false, state.AppendLog(logPath, fmt.Sprintf("remote input skipped: profile %s disabled or missing", profileName))
	}
	if !profile.Feishu.HasCredentials() {
		return false, state.AppendLog(logPath, fmt.Sprintf("remote input skipped: profile %s has no feishu credentials", profileName))
	}

	inputPath, err := config.InputRequestsPath()
	if err != nil {
		return true, err
	}
	store := inputrequest.NewStore(inputPath)
	ttl := time.Duration(cfg.Approval.TimeoutSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	req := inputrequest.NewRequest(evt, profileName, ttl)
	if err := store.Create(req); err != nil {
		return true, err
	}
	_ = state.AppendLog(logPath, fmt.Sprintf("input request created id=%s ttl=%v", req.InputID, ttl))
	if err := sendInputPromptForHook(ctx, cfg, logPath, evt, req, profileName, profile); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("input prompt send error id=%s err=%v", req.InputID, err))
	}

	_ = state.AppendLog(logPath, fmt.Sprintf("input request waiting id=%s", req.InputID))
	result, err := store.Wait(ctx, req.InputID, ttl)
	if err != nil {
		return true, state.AppendLog(logPath, fmt.Sprintf("input request timed out id=%s err=%v", req.InputID, err))
	}
	if writeErr := writeHookAdditionalContext(stdout, evt.HookEvent, "用户已通过飞书回答: "+result.Answer); writeErr != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("input answer write error id=%s err=%v", req.InputID, writeErr))
	}
	return true, state.AppendLog(logPath, fmt.Sprintf("input answered id=%s operator=%s", req.InputID, result.OperatorID))
}

func sendInputPrompt(ctx context.Context, cfg config.Config, logPath string, evt event.Event, req inputrequest.Request, profileName string, profile config.ProfileConfig) error {
	body := fmt.Sprintf("提示: %s\n有效期: %s", req.Prompt, req.ExpiresAt.Format("15:04:05"))
	msg := notify.Message{
		Agent:            evt.Agent,
		Event:            "input_required",
		SessionID:        evt.SessionID,
		Workspace:        evt.Workspace,
		Profile:          profileName,
		Title:            notify.FormatTitle(evt.Agent, "input_required"),
		Body:             body,
		RawPayload:       string(evt.RawPayload),
		InputRequestID:   req.InputID,
		InputToken:       req.Token,
		InputPrompt:      req.Prompt,
		InputOptions:     req.Options,
		InputMultiSelect: req.MultiSelect,
		InputAllowOther:  req.AllowOther,
	}

	sender, err := notify.NewProfileFeishuSender(profileName, profile)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("input feishu sender error: %v", err))
	}
	sendCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Behavior.SendTimeoutSeconds)*time.Second)
	defer cancel()
	return sender.Send(sendCtx, msg)
}
