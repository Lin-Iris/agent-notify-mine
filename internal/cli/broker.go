package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hellolib/agent-notify/internal/agentprocess"
	"github.com/hellolib/agent-notify/internal/approval"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishubridge"
	"github.com/hellolib/agent-notify/internal/feishucli"
	"github.com/hellolib/agent-notify/internal/inputrequest"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/threadstore"
	"github.com/spf13/cobra"
)

func newBrokerCmd(ctx context.Context, streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "broker",
		Short: "Manage Feishu approval broker",
	}
	cmd.AddCommand(
		newBrokerRunCmd(ctx, streams),
		newBrokerStartCmd(streams),
		newBrokerStopCmd(streams),
		newBrokerStatusCmd(streams),
		newBrokerCardCmd(ctx, streams),
		newBrokerApproveCmd(streams, approval.DecisionApprove),
		newBrokerApproveCmd(streams, approval.DecisionDeny),
		newBrokerPendingCmd(streams),
		newBrokerDisconnectCmd(streams),
		newBrokerTaskCmd(ctx, streams),
		newBrokerCommandCmd(streams),
	)
	return cmd
}

func newBrokerRunCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the Feishu long-connection broker in the foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			fmt.Fprintln(streams.Stdout, "broker long connection starting")
			return startFeishuGateways(ctx, cfg)
		},
	}
}

type textApprovalHandler struct{}

func startFeishuGateways(ctx context.Context, cfg config.Config) error {
	var gateways []*feishubridge.Gateway
	for name, profile := range cfg.Profiles {
		if !profile.Enabled || profile.Feishu.AppID == "" || profile.Feishu.AppSecret == "" {
			continue
		}
		gateways = append(gateways, feishubridge.NewProfileGateway(name, profile.Feishu.AppID, profile.Feishu.AppSecret, textApprovalHandler{}))
	}
	if len(gateways) == 0 {
		return feishubridge.NewGateway(textApprovalHandler{}).Start(ctx, cfg)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, len(gateways))
	for _, gateway := range gateways {
		go func(g *feishubridge.Gateway) {
			errCh <- g.Start(runCtx, cfg)
		}(gateway)
	}
	err := <-errCh
	cancel()
	return err
}

var knownSlashCommands = map[string]bool{
	"/status": true, "/cd": true, "/ws": true, "/new": true, "/connect": true,
	"/stop": true, "/ps": true, "/exit": true, "/disconnect": true,
	"/threads": true, "/thread": true, "/tail": true, "/log": true, "/result": true,
	"/home": true, "/back": true,
}

func replyFeishuError(ctx context.Context, profile string, err error) {
	if err != nil {
		_ = sendProfileText(ctx, profile, "❌ "+err.Error())
	}
}

func feishuSenderForProfile(cfg config.Config, profile string) (*notify.FeishuSender, error) {
	p, ok := cfg.Profiles[profile]
	if !ok {
		return nil, fmt.Errorf("profile %s not found", profile)
	}
	return notify.NewProfileFeishuSender(profile, p)
}

func sendProfileText(ctx context.Context, profile, text string) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	sender, err := feishuSenderForProfile(cfg, profile)
	if err != nil {
		return err
	}
	return sender.SendText(ctx, text)
}

func (textApprovalHandler) HandleText(ctx context.Context, profile, chatID, senderID, text string) error {
	text = strings.TrimSpace(text)
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		replyFeishuError(ctx, profile, err)
		return err
	}
	if profile == "" {
		profile = cfg.Broker.ActiveProfile
	}
	if profile == "" {
		profile = "claude-main"
	}
	if _, ok := cfg.Profiles[profile]; !ok {
		err := fmt.Errorf("profile %s not found", profile)
		replyFeishuError(ctx, profile, err)
		return err
	}
	if !authorizedFeishuOperatorForProfile(cfg, profile, senderID, chatID) {
		_ = sendProfileText(ctx, profile, "⛔ 你没有权限操作此 Agent。请检查该 profile 的 feishu.owner_open_id。")
		return appendAudit("feishu text unauthorized profile=" + profile + " chat=" + chatID + " sender=" + senderID)
	}
	if handled, err := handlePendingInputText(profile, senderID, text); handled {
		if err != nil {
			replyFeishuError(ctx, profile, err)
			return err
		}
		_ = sendProfileText(ctx, profile, "✅ 已提交答案")
		return appendAudit("feishu input answered profile=" + profile + " sender=" + senderID)
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	if len(fields) >= 2 {
		var decision approval.Decision
		switch fields[0] {
		case "批准", "approve":
			decision = approval.DecisionApprove
		case "拒绝", "deny":
			decision = approval.DecisionDeny
		}
		if decision != "" {
			path, err := config.ApprovalPath()
			if err != nil {
				replyFeishuError(ctx, profile, err)
				return err
			}
			if _, err := approval.NewStore(path).Decide(fields[1], "", senderID, decision, "decided from Feishu text"); err != nil {
				replyFeishuError(ctx, profile, err)
				return err
			}
			_ = sendProfileText(ctx, profile, fmt.Sprintf("✅ 已%s请求 %s", string(decision), fields[1]))
			return appendAudit(fmt.Sprintf("feishu approval %s id=%s chat=%s sender=%s", decision, fields[1], chatID, senderID))
		}
	}
	if strings.HasPrefix(fields[0], "/") {
		isKnown := knownSlashCommands[fields[0]]
		if !isKnown {
			msg := fmt.Sprintf("未知命令: %s\n\n可用命令:\n/status /cd /ws /new /stop /ps /threads /thread /tail /log /result /home /back /disconnect\n\n直接发送文本（不加 /）可启动 Agent 任务。", fields[0])
			_ = sendProfileText(ctx, profile, msg)
			return appendAudit("feishu unknown command chat=" + chatID + " sender=" + senderID + " text=" + text)
		}
		if handled, err := handleThreadSlashCommand(ctx, Streams{}, profile, text, true); handled {
			if err != nil {
				replyFeishuError(ctx, profile, err)
			}
			return err
		}
		var out bytes.Buffer
		if err := runBrokerSlashCommand(Streams{Stdout: &out, Stderr: &out}, profile, text); err != nil {
			replyFeishuError(ctx, profile, err)
			return err
		}
		if fields[0] == "/status" || fields[0] == "/cd" || fields[0] == "/ws" || fields[0] == "/disconnect" || fields[0] == "/stop" {
			_ = sendBrokerControlCard(ctx, profile)
		}
		return appendAudit("feishu command chat=" + chatID + " sender=" + senderID + " output=" + strings.TrimSpace(out.String()))
	}
	reg, _, regErr := processRegistry()
	if regErr == nil {
		running, _ := reg.List(profile)
		for _, r := range running {
			if r.Status == "running" {
				_ = sendProfileText(ctx, profile, "⏳ 当前有任务正在运行，请等待完成后再发新任务。发送 /stop 可强制停止。")
				return appendAudit("feishu task rejected: process already running profile=" + profile + " id=" + r.ID)
			}
		}
	}
	rec, _, _, err := startThreadTask(ctx, profile, text)
	if err != nil {
		replyFeishuError(ctx, profile, err)
		return err
	}
	return appendAudit(fmt.Sprintf("feishu task start profile=%s id=%s pid=%d sender=%s", rec.Profile, rec.ID, rec.PID, senderID))
}

func (textApprovalHandler) HandleCardAction(ctx context.Context, botProfile, operatorID string, value map[string]any) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	action, _ := value["action"].(string)
	id, _ := value["approval_id"].(string)
	token, _ := value["token"].(string)
	profile, _ := value["profile"].(string)
	inputID, _ := value["input_id"].(string)
	answer, _ := value["answer"].(string)
	if profile == "" && botProfile != "" {
		profile = botProfile
	}
	if profile == "" && (action == "approve" || action == "deny") {
		if !authorizedFeishuOperator(cfg, operatorID, "") {
			return appendAudit("feishu card approval unauthorized operator=" + operatorID)
		}
		decision := approval.DecisionApprove
		if action == "deny" {
			decision = approval.DecisionDeny
		}
		if id == "" {
			return appendAudit("feishu card action missing approval_id operator=" + operatorID)
		}
		path, err := config.ApprovalPath()
		if err != nil {
			return err
		}
		_, err = approval.NewStore(path).Decide(id, token, operatorID, decision, "decided from Feishu card")
		if err != nil {
			return err
		}
		return appendAudit(fmt.Sprintf("feishu card approval %s id=%s operator=%s", decision, id, operatorID))
	}
	if profile == "" {
		return appendAudit("feishu card action missing profile operator=" + operatorID + " action=" + action)
	}
	if _, ok := cfg.Profiles[profile]; !ok {
		return appendAudit("feishu card action unknown profile=" + profile + " operator=" + operatorID + " action=" + action)
	}
	if !authorizedFeishuOperatorForProfile(cfg, profile, operatorID, "") {
		return appendAudit("feishu card unauthorized profile=" + profile + " operator=" + operatorID)
	}
	if action == "input_submit" {
		if inputID == "" {
			return appendAudit("feishu card action missing input_id operator=" + operatorID)
		}
		path, err := config.InputRequestsPath()
		if err != nil {
			return err
		}
		answers, other := inputSubmitAnswers(value, answer)
		if _, err := inputrequest.NewStore(path).AnswerValues(inputID, token, operatorID, answers, other); err != nil {
			return err
		}
		return appendAudit(fmt.Sprintf("feishu card input answered id=%s operator=%s", inputID, operatorID))
	}
	var decision approval.Decision
	switch action {
	case "approve":
		decision = approval.DecisionApprove
	case "deny":
		decision = approval.DecisionDeny
	case "broker_connect", "broker_pause", "broker_disconnect", "broker_stop", "broker_status", "home", "threads_list", "thread_new", "thread_open", "thread_use", "thread_result", "task_result", "task_tail", "task_log", "task_stop":
		if err := applyBrokerCardActionValue(ctx, action, profile, value); err != nil {
			return err
		}
		return appendAudit(fmt.Sprintf("feishu card action=%s profile=%s operator=%s", action, profile, operatorID))
	default:
		return appendAudit("feishu card action unsupported operator=" + operatorID + " action=" + action)
	}
	if id == "" {
		return appendAudit("feishu card action missing approval_id operator=" + operatorID)
	}
	path, err := config.ApprovalPath()
	if err != nil {
		return err
	}
	_, err = approval.NewStore(path).Decide(id, token, operatorID, decision, "decided from Feishu card")
	if err != nil {
		return err
	}
	return appendAudit(fmt.Sprintf("feishu card approval %s id=%s operator=%s", decision, id, operatorID))
}

func handlePendingInputText(profile, operatorID, answer string) (bool, error) {
	path, err := config.InputRequestsPath()
	if err != nil {
		return false, err
	}
	store := inputrequest.NewStore(path)
	req, ok, err := store.PendingForProfile(profile)
	if err != nil || !ok {
		return ok, err
	}
	_, err = store.Answer(req.InputID, "", operatorID, answer)
	if err != nil {
		return true, err
	}
	return true, nil
}

func inputSubmitAnswers(value map[string]any, fallback string) ([]string, string) {
	form, _ := value["_form_value"].(map[string]any)
	var answers []string
	var other string
	if form != nil {
		answers = stringValues(form["answer"])
		other = firstString(form["other"])
	}
	if len(answers) == 0 {
		answers = stringValues(value["_options"])
	}
	if len(answers) == 0 {
		answers = stringValues(value["_option"])
	}
	if len(answers) == 0 && fallback != "" {
		answers = []string{fallback}
	}
	if other == "" {
		other = firstString(value["_input_value"])
	}
	return answers, other
}

func stringValues(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := firstString(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func firstString(raw any) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func newBrokerStartCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Enable broker communication",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			cfg.Broker.Enabled = true
			cfg.Broker.LongConnection = true
			cfg.Approval.Enabled = true
			if cfg.Broker.ActiveProfile == "" {
				cfg.Broker.ActiveProfile = "claude-main"
			}
			profile := ensureProfile(&cfg, cfg.Broker.ActiveProfile)
			profile.Enabled = true
			cfg.Profiles[cfg.Broker.ActiveProfile] = profile
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			pid, alreadyRunning, err := startBrokerDaemon()
			if err != nil {
				return err
			}
			if alreadyRunning {
				fmt.Fprintf(streams.Stdout, "broker already running: pid=%d profile=%s approval_timeout=%ds\n", pid, cfg.Broker.ActiveProfile, cfg.Approval.TimeoutSeconds)
			} else {
				fmt.Fprintf(streams.Stdout, "broker enabled and started: pid=%d profile=%s approval_timeout=%ds\n", pid, cfg.Broker.ActiveProfile, cfg.Approval.TimeoutSeconds)
			}
			if profile.Workspace == "" {
				fmt.Fprintln(streams.Stdout, "当前项目未设置。请在飞书对话里发送 `/cd /具体/项目/目录` 后再发任务。")
			}
			return appendAudit("broker start profile=" + cfg.Broker.ActiveProfile)
		},
	}
}

func newBrokerStopCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Disable broker communication and reject pending approvals",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			if profile == "" {
				profile = cfg.Broker.ActiveProfile
			}
			if err := disconnectProfile(profile, "broker stopped"); err != nil {
				return err
			}
			if err := stopBrokerDaemon(); err != nil {
				fmt.Fprintf(streams.Stdout, "broker listener stop skipped: %v\n", err)
			}
			fmt.Fprintf(streams.Stdout, "broker stopped: profile=%s\n", profile)
			return appendAudit("broker stop profile=" + profile)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile to stop")
	return cmd
}

func newBrokerStatusCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show broker status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			if profile == "" {
				profile = cfg.Broker.ActiveProfile
			}
			p := ensureProfile(&cfg, profile)
			pending, _ := pendingCount()
			procs, _ := listProcesses(profile)
			brokerPID, brokerRunning := brokerDaemonStatus()
			fmt.Fprintf(streams.Stdout, "broker=%v approval=%v listener=%v pid=%d profile=%s profile_enabled=%v agent=%s workspace=%s permission=%s pending=%d processes=%d cli=%s\n",
				cfg.Broker.Enabled, cfg.Approval.Enabled, brokerRunning, brokerPID, profile, p.Enabled, p.Agent, p.Workspace, p.PermissionMode, pending, runningProcessCount(procs), cliCapabilitySummary(p.Agent))
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newBrokerCardCmd(ctx context.Context, streams Streams) *cobra.Command {
	var profile, view string
	cmd := &cobra.Command{
		Use:   "card",
		Short: "Send a Feishu broker control card",
		RunE: func(cmd *cobra.Command, args []string) error {
			if profile == "" {
				cfg, _, err := loadDefaultConfig()
				if err != nil {
					return err
				}
				profile = cfg.Broker.ActiveProfile
			}
			var err error
			switch view {
			case "home":
				err = sendBrokerControlCard(ctx, profile)
			case "threads":
				err = sendThreadListCard(ctx, profile, 1)
			case "thread":
				cfg, _, loadErr := loadDefaultConfig()
				if loadErr != nil {
					return loadErr
				}
				_, p := profileOrActive(cfg, profile)
				store, storeErr := threadStore()
				if storeErr != nil {
					return storeErr
				}
				thread, activeErr := store.EnsureActiveThread(profile, p.Workspace, p.Agent)
				if activeErr != nil {
					return activeErr
				}
				err = sendThreadDetailCard(ctx, thread)
			case "task":
				cfg, _, loadErr := loadDefaultConfig()
				if loadErr != nil {
					return loadErr
				}
				_, p := profileOrActive(cfg, profile)
				store, storeErr := threadStore()
				if storeErr != nil {
					return storeErr
				}
				thread, activeErr := store.ActiveThread(profile, p.Workspace)
				if activeErr != nil {
					return activeErr
				}
				tasks, listErr := store.ListTasks(thread.ID)
				if listErr != nil {
					return listErr
				}
				if len(tasks) == 0 {
					return fmt.Errorf("current thread has no tasks")
				}
				_, err = sendTaskStatusCard(ctx, tasks[0])
			default:
				return fmt.Errorf("unknown view: %s", view)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "broker card sent: profile=%s view=%s\n", profile, view)
			return appendAudit("broker card profile=" + profile)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&view, "view", "home", "home, threads, thread, or task")
	return cmd
}

func newBrokerPendingCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "pending",
		Short: "List pending approval requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ApprovalPath()
			if err != nil {
				return err
			}
			items, err := approval.NewStore(path).List()
			if err != nil {
				return err
			}
			now := time.Now()
			n := 0
			for _, item := range items {
				if item.Status == approval.StatusPending && now.Before(item.ExpiresAt) {
					n++
					fmt.Fprintf(streams.Stdout, "id=%s tool=%s created=%s\n", item.ApprovalID, item.Tool, item.CreatedAt.Format("15:04:05"))
				}
			}
			if n == 0 {
				fmt.Fprintln(streams.Stdout, "no pending approvals")
			}
			return nil
		},
	}
}

func newBrokerApproveCmd(streams Streams, decision approval.Decision) *cobra.Command {
	var token, operator, reason string
	use := "approve <approval_id>"
	short := "Approve a pending request"
	if decision == approval.DecisionDeny {
		use = "deny <approval_id>"
		short = "Deny a pending request"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ApprovalPath()
			if err != nil {
				return err
			}
			store := approval.NewStore(path)
			req, err := store.Decide(args[0], token, operator, decision, reason)
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "%s %s status=%s operator=%s\n", decision, req.ApprovalID, req.Status, req.OperatorID)
			return appendAudit(fmt.Sprintf("approval %s id=%s operator=%s", decision, req.ApprovalID, operator))
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "one-time approval token")
	cmd.Flags().StringVar(&operator, "operator", "local-cli", "operator open_id or local identity")
	cmd.Flags().StringVar(&reason, "reason", "", "decision reason")
	return cmd
}

func newBrokerDisconnectCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect one profile and kill its controlled processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			if profile == "" {
				profile = cfg.Broker.ActiveProfile
			}
			if err := disconnectProfile(profile, "profile disconnected"); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "profile disconnected: %s\n", profile)
			return appendAudit("profile disconnect profile=" + profile)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newBrokerTaskCmd(ctx context.Context, streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "task <prompt>",
		Short: "Start a controlled agent task for a profile",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rec, thread, task, err := startThreadTask(ctx, profile, strings.Join(args, " "))
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "started task id=%s pid=%d profile=%s thread=#%d log=%s\n", task.ID, rec.PID, rec.Profile, thread.Number, rec.LogPath)
			return appendAudit(fmt.Sprintf("task start profile=%s pid=%d id=%s thread=%s", rec.Profile, rec.PID, rec.ID, thread.ID))
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newBrokerCommandCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "command <slash-command>",
		Short: "Run a Feishu slash command locally",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			if profile == "" {
				profile = cfg.Broker.ActiveProfile
			}
			p := ensureProfile(&cfg, profile)
			fields := strings.Fields(text)
			if len(fields) == 0 {
				return nil
			}
			switch fields[0] {
			case "/status":
				pending, _ := pendingCount()
				procs, _ := listProcesses(profile)
				fmt.Fprintf(streams.Stdout, "profile=%s enabled=%v agent=%s workspace=%s permission=%s pending=%d processes=%d\n",
					profile, p.Enabled, p.Agent, p.Workspace, p.PermissionMode, pending, runningProcessCount(procs))
			case "/cd":
				if len(fields) < 2 {
					return fmt.Errorf("/cd requires a path")
				}
				abs, err := filepath.Abs(fields[1])
				if err != nil {
					return err
				}
				info, err := os.Stat(abs)
				if err != nil || !info.IsDir() {
					return fmt.Errorf("workspace is not a directory: %s", fields[1])
				}
				p.Workspace = abs
				cfg.Profiles[profile] = p
				if err := config.Save(path, cfg); err != nil {
					return err
				}
				fmt.Fprintf(streams.Stdout, "workspace=%s\n", abs)
			case "/ws":
				if len(fields) < 2 {
					return fmt.Errorf("/ws requires list, save, use, or remove")
				}
				if p.Workspaces == nil {
					p.Workspaces = map[string]string{}
				}
				switch fields[1] {
				case "list":
					for name, ws := range p.Workspaces {
						fmt.Fprintf(streams.Stdout, "%s %s\n", name, ws)
					}
				case "save":
					if len(fields) < 3 {
						return fmt.Errorf("/ws save requires a name")
					}
					if p.Workspace == "" {
						return fmt.Errorf("current workspace is empty")
					}
					p.Workspaces[fields[2]] = p.Workspace
					cfg.Profiles[profile] = p
					if err := config.Save(path, cfg); err != nil {
						return err
					}
					fmt.Fprintf(streams.Stdout, "workspace saved: %s=%s\n", fields[2], p.Workspace)
				case "use":
					if len(fields) < 3 {
						return fmt.Errorf("/ws use requires a name")
					}
					ws, ok := p.Workspaces[fields[2]]
					if !ok {
						return fmt.Errorf("workspace %s not found", fields[2])
					}
					p.Workspace = ws
					cfg.Profiles[profile] = p
					if err := config.Save(path, cfg); err != nil {
						return err
					}
					fmt.Fprintf(streams.Stdout, "workspace=%s\n", ws)
				case "remove":
					if len(fields) < 3 {
						return fmt.Errorf("/ws remove requires a name")
					}
					delete(p.Workspaces, fields[2])
					cfg.Profiles[profile] = p
					if err := config.Save(path, cfg); err != nil {
						return err
					}
					fmt.Fprintf(streams.Stdout, "workspace removed: %s\n", fields[2])
				default:
					return fmt.Errorf("unknown /ws command: %s", fields[1])
				}
			case "/stop":
				if err := killProfile(profile); err != nil {
					return err
				}
				fmt.Fprintf(streams.Stdout, "stop requested: %s\n", profile)
			case "/ps":
				items, err := listProcesses(profile)
				if err != nil {
					return err
				}
				for _, item := range items {
					fmt.Fprintf(streams.Stdout, "%s pid=%d status=%s log=%s\n", item.ID, item.PID, item.Status, item.LogPath)
				}
			case "/exit":
				if len(fields) < 2 {
					return fmt.Errorf("/exit requires id or pid")
				}
				reg, _, err := processRegistry()
				if err != nil {
					return err
				}
				if err := reg.Kill(fields[1]); err != nil {
					return err
				}
				fmt.Fprintf(streams.Stdout, "kill requested: %s\n", fields[1])
			case "/disconnect":
				p.Enabled = false
				cfg.Profiles[profile] = p
				if err := rejectPending("profile disconnected"); err != nil {
					return err
				}
				if err := killProfile(profile); err != nil {
					return err
				}
				if err := config.Save(path, cfg); err != nil {
					return err
				}
				fmt.Fprintf(streams.Stdout, "profile disconnected: %s\n", profile)
			default:
				return fmt.Errorf("unsupported command: %s", fields[0])
			}
			return appendAudit("broker command profile=" + profile + " text=" + text)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func runBrokerSlashCommand(streams Streams, profile, text string) error {
	if handled, err := handleThreadSlashCommand(context.Background(), streams, profile, text, false); handled {
		return err
	}
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	if profile == "" {
		profile = cfg.Broker.ActiveProfile
	}
	p := ensureProfile(&cfg, profile)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "/status":
		pending, _ := pendingCount()
		procs, _ := listProcesses(profile)
		fmt.Fprintf(streams.Stdout, "profile=%s enabled=%v agent=%s workspace=%s permission=%s pending=%d processes=%d\n",
			profile, p.Enabled, p.Agent, p.Workspace, p.PermissionMode, pending, runningProcessCount(procs))
	case "/cd":
		if len(fields) < 2 {
			return fmt.Errorf("/cd requires a path")
		}
		abs, err := filepath.Abs(fields[1])
		if err != nil {
			return err
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return fmt.Errorf("workspace is not a directory: %s", fields[1])
		}
		p.Workspace = abs
		cfg.Profiles[profile] = p
		if err := config.Save(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(streams.Stdout, "workspace=%s\n", abs)
	case "/ws":
		if len(fields) < 2 {
			return fmt.Errorf("/ws requires list, save, use, or remove")
		}
		if p.Workspaces == nil {
			p.Workspaces = map[string]string{}
		}
		switch fields[1] {
		case "list":
			for name, ws := range p.Workspaces {
				fmt.Fprintf(streams.Stdout, "%s %s\n", name, ws)
			}
		case "save":
			if len(fields) < 3 {
				return fmt.Errorf("/ws save requires a name")
			}
			if p.Workspace == "" {
				return fmt.Errorf("current workspace is empty")
			}
			p.Workspaces[fields[2]] = p.Workspace
			cfg.Profiles[profile] = p
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "workspace saved: %s=%s\n", fields[2], p.Workspace)
		case "use":
			if len(fields) < 3 {
				return fmt.Errorf("/ws use requires a name")
			}
			ws, ok := p.Workspaces[fields[2]]
			if !ok {
				return fmt.Errorf("workspace %s not found", fields[2])
			}
			p.Workspace = ws
			cfg.Profiles[profile] = p
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "workspace=%s\n", ws)
		case "remove":
			if len(fields) < 3 {
				return fmt.Errorf("/ws remove requires a name")
			}
			delete(p.Workspaces, fields[2])
			cfg.Profiles[profile] = p
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "workspace removed: %s\n", fields[2])
		default:
			return fmt.Errorf("unknown /ws command: %s", fields[1])
		}
	case "/new", "/connect":
		cfg.Broker.Enabled = true
		cfg.Broker.LongConnection = true
		cfg.Approval.Enabled = true
		p.Enabled = true
		cfg.Profiles[profile] = p
		if err := config.Save(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(streams.Stdout, "profile connected: %s\n", profile)
	case "/stop":
		if err := killProfile(profile); err != nil {
			return err
		}
		fmt.Fprintf(streams.Stdout, "stop requested: %s\n", profile)
	case "/ps":
		items, err := listProcesses(profile)
		if err != nil {
			return err
		}
		for _, item := range items {
			fmt.Fprintf(streams.Stdout, "%s pid=%d status=%s log=%s\n", item.ID, item.PID, item.Status, item.LogPath)
		}
	case "/exit":
		if len(fields) < 2 {
			return fmt.Errorf("/exit requires id or pid")
		}
		reg, _, err := processRegistry()
		if err != nil {
			return err
		}
		if err := reg.Kill(fields[1]); err != nil {
			return err
		}
		fmt.Fprintf(streams.Stdout, "kill requested: %s\n", fields[1])
	case "/disconnect":
		if err := disconnectProfile(profile, "profile disconnected"); err != nil {
			return err
		}
		fmt.Fprintf(streams.Stdout, "profile disconnected: %s\n", profile)
	default:
		return fmt.Errorf("unsupported command: %s", fields[0])
	}
	return appendAudit("broker command profile=" + profile + " text=" + text)
}

func startControlledTask(ctx context.Context, profile, prompt string) (agentprocess.Record, error) {
	rec, _, _, err := startThreadTask(ctx, profile, prompt)
	return rec, err
}

func applyBrokerCardAction(ctx context.Context, action, profile string) error {
	return applyBrokerCardActionValue(ctx, action, profile, nil)
}

func applyBrokerCardActionValue(ctx context.Context, action, profile string, value map[string]any) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	if profile == "" {
		profile = cfg.Broker.ActiveProfile
	}
	p := ensureProfile(&cfg, profile)
	switch action {
	case "broker_connect":
		cfg.Broker.Enabled = true
		cfg.Broker.LongConnection = true
		cfg.Approval.Enabled = true
		p.Enabled = true
		cfg.Profiles[profile] = p
		if err := config.Save(path, cfg); err != nil {
			return err
		}
	case "broker_pause":
		p.Enabled = false
		cfg.Profiles[profile] = p
		if !anyProfileEnabled(cfg) {
			cfg.Broker.Enabled = false
			cfg.Approval.Enabled = false
		}
		if err := rejectPending("profile paused from Feishu card"); err != nil {
			return err
		}
		if err := killProfile(profile); err != nil {
			return err
		}
		if err := config.Save(path, cfg); err != nil {
			return err
		}
	case "broker_disconnect":
		if err := disconnectProfile(profile, "profile disconnected from Feishu card"); err != nil {
			return err
		}
		if err := sendBrokerControlCard(ctx, profile); err != nil {
			return err
		}
		updatedCfg, _, _ := loadDefaultConfig()
		if !anyProfileEnabled(updatedCfg) {
			shutdownCurrentBrokerSoon()
		}
		return nil
	case "broker_stop":
		if err := killProfile(profile); err != nil {
			return err
		}
	case "broker_status":
	case "home":
		return sendBrokerControlCard(ctx, profile)
	case "threads_list":
		page := intFromValue(value["page"], 1)
		return sendThreadListCard(ctx, profile, page)
	case "thread_new":
		store, err := threadStore()
		if err != nil {
			return err
		}
		thread, err := store.CreateThread(profile, p.Workspace, p.Agent, "新对话")
		if err != nil {
			return err
		}
		return sendThreadDetailCard(ctx, thread)
	case "thread_open", "thread_use":
		threadID, _ := value["thread_id"].(string)
		store, err := threadStore()
		if err != nil {
			return err
		}
		thread, err := store.GetThread(threadID)
		if err != nil {
			return err
		}
		if action == "thread_use" {
			if _, err := store.UseThread(profile, p.Workspace, thread.ID); err != nil {
				return err
			}
		}
		return sendThreadDetailCard(ctx, thread)
	case "thread_result":
		threadID, _ := value["thread_id"].(string)
		store, err := threadStore()
		if err != nil {
			return err
		}
		tasks, err := store.ListTasks(threadID)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			return sendProfileText(ctx, profile, "这个对话还没有任务结果。")
		}
		_, err = sendTaskStatusCard(ctx, tasks[0])
		return err
	case "task_result", "task_tail", "task_log":
		taskID, _ := value["task_id"].(string)
		mode := "result"
		if action == "task_tail" {
			mode = "tail"
		} else if action == "task_log" {
			mode = "log"
		}
		text, err := taskText(taskID, mode, 80)
		if err != nil {
			return err
		}
		title := "模型输出 " + taskID
		if mode == "tail" {
			title = "最近输出 " + taskID
		} else if mode == "log" {
			title = "完整过程 " + taskID
		}
		sender, err := feishuSenderForProfile(cfg, profile)
		if err != nil {
			return err
		}
		return sender.SendLongText(ctx, title, text)
	case "task_stop":
		taskID, _ := value["task_id"].(string)
		store, err := threadStore()
		if err != nil {
			return err
		}
		task, err := store.GetTask(taskID)
		if err != nil {
			return err
		}
		reg, _, err := processRegistry()
		if err != nil {
			return err
		}
		if task.ProcessID != "" {
			_ = reg.Kill(task.ProcessID)
		} else if task.PID > 0 {
			_ = reg.Kill(strconv.Itoa(task.PID))
		}
		task.Status = threadstore.TaskStatusStopped
		task.Progress = "任务已停止"
		task.EndedAt = time.Now()
		_ = store.UpdateTask(task)
		if task.FeishuMessageID != "" {
			return updateTaskStatusCard(ctx, task)
		}
		_, err = sendTaskStatusCard(ctx, task)
		return err
	default:
		return fmt.Errorf("unsupported broker action: %s", action)
	}
	return sendBrokerControlCard(ctx, profile)
}

func sendBrokerControlCard(ctx context.Context, profile string) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	if profile == "" {
		profile = cfg.Broker.ActiveProfile
	}
	p := ensureProfile(&cfg, profile)
	pending, _ := pendingCount()
	procs, _ := listProcesses(profile)
	runningCount := 0
	for _, proc := range procs {
		if proc.Status == "running" {
			runningCount++
		}
	}
	activeThread := ""
	if store, err := threadStore(); err == nil {
		if thread, err := store.ActiveThread(profile, p.Workspace); err == nil {
			activeThread = fmt.Sprintf("#%d %s", thread.Number, thread.Title)
		}
	}
	sender, err := feishuSenderForProfile(cfg, profile)
	if err != nil {
		return err
	}
	_, err = sender.SendRawCard(ctx, notify.BuildBrokerControlCard(notify.BrokerControlStatus{
		Profile:        profile,
		Agent:          p.Agent,
		Workspace:      p.Workspace,
		PermissionMode: p.PermissionMode,
		BrokerEnabled:  cfg.Broker.Enabled,
		ProfileEnabled: p.Enabled,
		Pending:        pending,
		Processes:      runningCount,
		ActiveThread:   activeThread,
		CLIDiagnostics: cliCapabilitySummary(p.Agent),
	}))
	return err
}

func disconnectProfile(profile, reason string) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	if profile == "" {
		profile = cfg.Broker.ActiveProfile
	}
	if p, ok := cfg.Profiles[profile]; ok {
		p.Enabled = false
		cfg.Profiles[profile] = p
	}
	if !anyProfileEnabled(cfg) {
		cfg.Broker.Enabled = false
		cfg.Broker.LongConnection = false
		cfg.Approval.Enabled = false
	}
	if err := rejectPending(reason); err != nil {
		return err
	}
	if err := killProfile(profile); err != nil {
		return err
	}
	return config.Save(path, cfg)
}

func anyProfileEnabled(cfg config.Config) bool {
	for _, p := range cfg.Profiles {
		if p.Enabled {
			return true
		}
	}
	return false
}

func startBrokerDaemon() (int, bool, error) {
	if pid, running := brokerDaemonStatus(); running {
		return pid, true, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return 0, false, err
	}
	home, err := config.HomeDir()
	if err != nil {
		return 0, false, err
	}
	logPath := filepath.Join(home, "logs", "broker.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, false, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, false, err
	}
	defer logFile.Close()
	cmd := exec.Command(exe, "broker", "run")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, false, err
	}
	pidPath, err := config.BrokerPIDPath()
	if err != nil {
		return 0, false, err
	}
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		return 0, false, err
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		return 0, false, err
	}
	return cmd.Process.Pid, false, nil
}

func stopBrokerDaemon() error {
	pidPath, err := config.BrokerPIDPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		_ = os.Remove(pidPath)
		return err
	}
	if pid > 0 && pid != os.Getpid() {
		terminateProcessGroup(pid, 2*time.Second)
	}
	return os.Remove(pidPath)
}

func terminateProcessGroup(pid int, wait time.Duration) {
	if pid <= 0 {
		return
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Signal(syscall.SIGKILL)
	}
}

func brokerDaemonStatus() (int, bool) {
	pidPath, err := config.BrokerPIDPath()
	if err != nil {
		return 0, false
	}
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, pidAlive(pid)
}

func pidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return errors.Is(err, syscall.EPERM)
	}
	return true
}

func shutdownCurrentBrokerSoon() {
	go func() {
		time.Sleep(500 * time.Millisecond)
		pidPath, err := config.BrokerPIDPath()
		if err == nil {
			_ = os.Remove(pidPath)
		}
		os.Exit(0)
	}()
}

func authorizedFeishuOperator(cfg config.Config, openID, chatID string) bool {
	if openID == "" {
		return false
	}
	if len(cfg.Broker.AllowedChatIDs) > 0 && chatID != "" && !containsString(cfg.Broker.AllowedChatIDs, chatID) {
		return false
	}
	owner := cfg.Broker.OwnerOpenID
	if owner == "" {
		if feishuCfg, err := feishucli.ParseConfig(); err == nil {
			owner = feishuCfg.UserOpenID
		}
	}
	if openID == owner || containsString(cfg.Broker.AdminOpenIDs, openID) || containsString(cfg.Broker.AllowedOpenIDs, openID) {
		return true
	}
	return false
}

func authorizedFeishuOperatorForProfile(cfg config.Config, profileName, openID, chatID string) bool {
	if openID == "" {
		return false
	}
	if len(cfg.Broker.AllowedChatIDs) > 0 && chatID != "" && !containsString(cfg.Broker.AllowedChatIDs, chatID) {
		return false
	}
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return false
	}
	if profile.Feishu.ChatID != "" && chatID != "" && profile.Feishu.ChatID != chatID {
		return false
	}
	if openID == profile.Feishu.OwnerOpenID || containsString(cfg.Broker.AdminOpenIDs, openID) || containsString(cfg.Broker.AllowedOpenIDs, openID) {
		return true
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func intFromValue(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func cliCapabilitySummary(agent string) string {
	switch agent {
	case "claude", "claude_code":
		path, err := exec.LookPath("claude")
		if err != nil {
			return "Claude CLI 未找到，无法远程执行 Claude Code 任务"
		}
		return "Claude CLI 可用：" + path + "；支持 resume/session-id/stream-json"
	case "codex":
		path, err := exec.LookPath("codex")
		if err != nil {
			return "Codex CLI 未找到；仅安装不可调用 CLI 的 GUI 不足以执行任务"
		}
		return "Codex CLI 可用：" + path + "；支持 exec resume/json"
	default:
		return "未知 Agent：" + agent
	}
}

func ensureRemoteAgentCLIAvailable(agent string) error {
	switch agent {
	case "claude", "claude_code":
		if _, err := exec.LookPath("claude"); err != nil {
			return fmt.Errorf("Claude CLI 未找到，无法启动远程 Claude 任务。请先确认电脑终端可执行 claude --version")
		}
	case "codex":
		if _, err := exec.LookPath("codex"); err != nil {
			return fmt.Errorf("Codex CLI 未找到，无法启动远程 Codex 任务。请先确认电脑终端可执行 codex --version")
		}
	default:
		return fmt.Errorf("unsupported agent: %s", agent)
	}
	return nil
}

func ensureProfile(cfg *config.Config, name string) config.ProfileConfig {
	if cfg.Profiles == nil {
		cfg.Profiles = config.ProfilesConfig{}
	}
	if p, ok := cfg.Profiles[name]; ok {
		if p.Agent == "" {
			p.Agent = "claude"
		}
		if p.PermissionMode == "" {
			p.PermissionMode = "workspace-write"
		}
		return p
	}
	return config.ProfileConfig{
		Agent:          "claude",
		Enabled:        false,
		PermissionMode: "workspace-write",
		Workspaces:     map[string]string{},
	}
}

func pendingCount() (int, error) {
	path, err := config.ApprovalPath()
	if err != nil {
		return 0, err
	}
	return approval.NewStore(path).PendingCount()
}

func rejectPending(reason string) error {
	path, err := config.ApprovalPath()
	if err != nil {
		return err
	}
	return approval.NewStore(path).ExpirePending(reason)
}

func processRegistry() (*agentprocess.Registry, string, error) {
	path, err := config.ProcessRegistryPath()
	if err != nil {
		return nil, "", err
	}
	home, err := config.HomeDir()
	if err != nil {
		return nil, "", err
	}
	return agentprocess.NewRegistry(path), filepath.Join(home, "logs", "runs"), nil
}

func listProcesses(profile string) ([]agentprocess.Record, error) {
	reg, _, err := processRegistry()
	if err != nil {
		return nil, err
	}
	return reg.List(profile)
}

func runningProcessCount(items []agentprocess.Record) int {
	count := 0
	for _, item := range items {
		if item.Status == "running" || item.Status == "stopping" {
			count++
		}
	}
	return count
}

func killProfile(profile string) error {
	reg, _, err := processRegistry()
	if err != nil {
		return err
	}
	return reg.KillProfile(profile)
}

func appendAudit(line string) error {
	path, err := config.AuditLogPath()
	if err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	entry := time.Now().Format(time.RFC3339) + " " + line + "\n"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	defer f.Close()
	_, _ = f.WriteString(entry)
	return nil
}
