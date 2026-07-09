package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hellolib/agent-notify/internal/agentprocess"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/threadstore"
	"github.com/spf13/cobra"
)

const threadPageSize = 5

func newThreadCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{Use: "thread", Short: "Manage broker conversation threads"}
	cmd.AddCommand(
		newThreadListCmd(streams),
		newThreadNewCmd(streams),
		newThreadUseCmd(streams),
		newThreadRenameCmd(streams),
		newThreadArchiveCmd(streams),
	)
	return cmd
}

func newThreadListCmd(streams Streams) *cobra.Command {
	var profile string
	return &cobra.Command{
		Use:   "list",
		Short: "List current workspace threads",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			profile, p := profileOrActive(cfg, profile)
			store, err := threadStore()
			if err != nil {
				return err
			}
			threads, err := store.ListThreads(profile, p.Workspace, false)
			if err != nil {
				return err
			}
			for _, thread := range threads {
				fmt.Fprintf(streams.Stdout, "#%d %s status=%s updated=%s id=%s\n", thread.Number, thread.Title, thread.Status, thread.UpdatedAt.Format(time.RFC3339), thread.ID)
			}
			return nil
		},
	}
}

func newThreadNewCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "new [title]",
		Short: "Create a thread in the current workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			profile, p := profileOrActive(cfg, profile)
			title := strings.Join(args, " ")
			store, err := threadStore()
			if err != nil {
				return err
			}
			thread, err := store.CreateThread(profile, p.Workspace, p.Agent, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "thread created: #%d %s id=%s\n", thread.Number, thread.Title, thread.ID)
			return appendAudit(fmt.Sprintf("thread new profile=%s id=%s", profile, thread.ID))
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newThreadUseCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "use <id|#>",
		Short: "Set active thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			profile, p := profileOrActive(cfg, profile)
			store, err := threadStore()
			if err != nil {
				return err
			}
			thread, err := store.UseThread(profile, p.Workspace, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "active thread: #%d %s\n", thread.Number, thread.Title)
			return appendAudit(fmt.Sprintf("thread use profile=%s id=%s", profile, thread.ID))
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newThreadRenameCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "rename <id|#> <title>",
		Short: "Rename a thread",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			profile, p := profileOrActive(cfg, profile)
			store, err := threadStore()
			if err != nil {
				return err
			}
			thread, err := store.RenameThread(profile, p.Workspace, args[0], strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "thread renamed: #%d %s\n", thread.Number, thread.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newThreadArchiveCmd(streams Streams) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "archive <id|#>",
		Short: "Archive a thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			profile, p := profileOrActive(cfg, profile)
			store, err := threadStore()
			if err != nil {
				return err
			}
			thread, err := store.ArchiveThread(profile, p.Workspace, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(streams.Stdout, "thread archived: #%d %s\n", thread.Number, thread.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newTaskCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Inspect broker tasks"}
	cmd.AddCommand(newTaskTailCmd(streams), newTaskLogCmd(streams), newTaskResultCmd(streams))
	return cmd
}

func newTaskTailCmd(streams Streams) *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:   "tail <task_id|#>",
		Short: "Show recent task output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := taskText(args[0], "tail", lines)
			if err != nil {
				return err
			}
			fmt.Fprintln(streams.Stdout, text)
			return nil
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 80, "number of lines")
	return cmd
}

func newTaskLogCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "log <task_id|#>",
		Short: "Show task log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := taskText(args[0], "log", 0)
			if err != nil {
				return err
			}
			fmt.Fprintln(streams.Stdout, text)
			return nil
		},
	}
}

func newTaskResultCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "result <task_id|#>",
		Short: "Show final task result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := taskText(args[0], "result", 0)
			if err != nil {
				return err
			}
			fmt.Fprintln(streams.Stdout, text)
			return nil
		},
	}
}

func handleThreadSlashCommand(ctx context.Context, streams Streams, profile, text string, sendFeishu bool) (bool, error) {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return false, err
	}
	profile, p := profileOrActive(cfg, profile)
	store, err := threadStore()
	if err != nil {
		return false, err
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return true, nil
	}
	switch fields[0] {
	case "/threads":
		if sendFeishu {
			return true, sendThreadListCard(ctx, profile, 1)
		}
		threads, err := store.ListThreads(profile, p.Workspace, false)
		if err != nil {
			return true, err
		}
		for _, thread := range threads {
			fmt.Fprintf(streams.Stdout, "#%d %s status=%s\n", thread.Number, thread.Title, thread.Status)
		}
		return true, nil
	case "/thread":
		if len(fields) < 2 {
			return true, fmt.Errorf("/thread requires new, use, rename, or archive")
		}
		switch fields[1] {
		case "new":
			title := strings.Join(fields[2:], " ")
			thread, err := store.CreateThread(profile, p.Workspace, p.Agent, title)
			if err != nil {
				return true, err
			}
			if sendFeishu {
				return true, sendThreadDetailCard(ctx, thread)
			}
			fmt.Fprintf(streams.Stdout, "thread created: #%d %s\n", thread.Number, thread.Title)
			return true, nil
		case "use":
			if len(fields) < 3 {
				return true, fmt.Errorf("/thread use requires id or #")
			}
			thread, err := store.UseThread(profile, p.Workspace, fields[2])
			if err != nil {
				return true, err
			}
			if sendFeishu {
				return true, sendThreadDetailCard(ctx, thread)
			}
			fmt.Fprintf(streams.Stdout, "active thread: #%d %s\n", thread.Number, thread.Title)
			return true, nil
		case "rename":
			if len(fields) < 4 {
				return true, fmt.Errorf("/thread rename requires id and title")
			}
			thread, err := store.RenameThread(profile, p.Workspace, fields[2], strings.Join(fields[3:], " "))
			if err != nil {
				return true, err
			}
			if sendFeishu {
				return true, sendThreadDetailCard(ctx, thread)
			}
			fmt.Fprintf(streams.Stdout, "thread renamed: #%d %s\n", thread.Number, thread.Title)
			return true, nil
		case "archive":
			if len(fields) < 3 {
				return true, fmt.Errorf("/thread archive requires id or #")
			}
			thread, err := store.ArchiveThread(profile, p.Workspace, fields[2])
			if err != nil {
				return true, err
			}
			if sendFeishu {
				return true, sendThreadListCard(ctx, profile, 1)
			}
			fmt.Fprintf(streams.Stdout, "thread archived: #%d %s\n", thread.Number, thread.Title)
			return true, nil
		}
	case "/tail", "/log", "/result":
		if len(fields) < 2 {
			return true, fmt.Errorf("%s requires task id or #", fields[0])
		}
		mode := strings.TrimPrefix(fields[0], "/")
		lines := 80
		if len(fields) >= 3 {
			if n, err := strconv.Atoi(fields[2]); err == nil && n > 0 {
				lines = n
			}
		}
		thread, _ := store.ActiveThread(profile, p.Workspace)
		task, err := resolveTaskForThread(store, thread.ID, fields[1])
		if err != nil {
			return true, err
		}
		text, err := taskTextByTask(task, mode, lines)
		if err != nil {
			return true, err
		}
		if sendFeishu {
			sender, err := feishuSenderForProfile(cfg, profile)
			if err != nil {
				return true, err
			}
			return true, sender.SendLongText(ctx, mode+" "+task.ID, text)
		}
		fmt.Fprintln(streams.Stdout, text)
		return true, nil
	case "/home":
		if sendFeishu {
			return true, sendBrokerControlCard(ctx, profile)
		}
		fmt.Fprintln(streams.Stdout, "home")
		return true, nil
	case "/back":
		if sendFeishu {
			return true, sendBrokerControlCard(ctx, profile)
		}
		fmt.Fprintln(streams.Stdout, "back")
		return true, nil
	}
	return false, nil
}

func startThreadTask(ctx context.Context, profile, prompt string) (agentprocess.Record, threadstore.Thread, threadstore.Task, error) {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, err
	}
	if profile == "" {
		profile = cfg.Broker.ActiveProfile
	}
	p := ensureProfile(&cfg, profile)
	if !cfg.Broker.Enabled || !p.Enabled {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, fmt.Errorf("profile %s is disconnected", profile)
	}
	if err := ensureRemoteAgentCLIAvailable(p); err != nil {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, err
	}
	store, err := threadStore()
	if err != nil {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, err
	}
	thread, err := store.EnsureActiveThread(profile, p.Workspace, p.Agent)
	if err != nil {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, err
	}
	if thread.Agent == "claude" || thread.Agent == "claude_code" {
		if thread.NativeSessionID == "" {
			thread.NativeSessionID = uuid.NewString()
			thread.NativeResume = false
			_ = store.UpdateThread(thread)
		}
	}
	reg, logsDir, err := processRegistry()
	if err != nil {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, err
	}
	if running, err := runningTaskForThread(store, thread.ID); err == nil && running.ID != "" {
		return agentprocess.Record{}, thread, running, fmt.Errorf("当前对话已有任务正在运行，请等待完成后再发新任务。发送 /stop 可强制停止")
	}
	if running, err := reg.List(profile); err == nil {
		for _, rec := range running {
			if rec.Status == "running" {
				return agentprocess.Record{}, thread, threadstore.Task{}, fmt.Errorf("当前 Agent 已有任务正在运行，请等待完成后再发新任务。发送 /stop 可强制停止")
			}
		}
	}
	home, err := config.HomeDir()
	if err != nil {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, err
	}
	taskID := "task_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	logPath := filepath.Join(home, "logs", "runs", profile+"-"+thread.ID+"-"+taskID+".log")
	outputPath := filepath.Join(home, "logs", "runs", profile+"-"+thread.ID+"-"+taskID+".out")
	task, err := store.CreateTaskWithID(thread, taskID, prompt, logPath, outputPath)
	if err != nil {
		return agentprocess.Record{}, threadstore.Thread{}, threadstore.Task{}, err
	}
	task.Progress = "模型输出中..."
	_ = store.UpdateTask(task)
	if messageID, err := sendTaskStatusCard(ctx, task); err == nil {
		task.FeishuMessageID = messageID
		_ = store.UpdateTask(task)
	} else {
		_ = appendAudit(fmt.Sprintf("feishu initial task card skipped task=%s err=%v", task.ID, err))
	}

	if workspaceErr := validateTaskWorkspace(p.Workspace); workspaceErr != nil {
		task.Status = threadstore.TaskStatusFailed
		task.Error = workspaceErr.Error()
		task.Progress = taskStartErrorMessage(workspaceErr)
		task.EndedAt = time.Now()
		_ = store.UpdateTask(task)
		ensureTaskStatusCard(context.Background(), store, task, "workspace_failed")
		return agentprocess.Record{}, thread, task, workspaceErr
	}
	var taskMu sync.Mutex
	rec, err := reg.StartWithOptions(ctx, agentprocess.StartOptions{
		Profile:         profile,
		Agent:           p.Agent,
		CLIPath:         p.CLIPath,
		Workspace:       p.Workspace,
		PermissionMode:  p.PermissionMode,
		Prompt:          prompt,
		LogsDir:         logsDir,
		ThreadID:        thread.ID,
		TaskID:          task.ID,
		NativeSessionID: thread.NativeSessionID,
		Resume:          thread.NativeResume,
		OnOutput: func(line string) {
			fragment := parseModelStreamLine(line)
			if fragment.Output == "" && fragment.Reasoning == "" {
				return
			}
			taskMu.Lock()
			if latest, err := store.GetTask(task.ID); err == nil && latest.Status == threadstore.TaskStatusStopped {
				taskMu.Unlock()
				return
			}
			if fragment.Output != "" {
				task.StreamOutput += fragment.Output
			}
			if fragment.Reasoning != "" {
				if task.ReasoningTrace != "" {
					task.ReasoningTrace += "\n"
				}
				task.ReasoningTrace += fragment.Reasoning
			}
			task.Progress = streamProgress(task.StreamOutput)
			snapshot := task
			taskMu.Unlock()
			_ = os.WriteFile(outputPath, []byte(modelOutputText(snapshot)), 0o600)
			_ = store.UpdateTask(snapshot)
		},
		OnExit: func(rec agentprocess.Record, output string, exitCode int, runErr error) {
			taskMu.Lock()
			if latest, err := store.GetTask(task.ID); err == nil && latest.Status == threadstore.TaskStatusStopped {
				latest.PID = rec.PID
				latest.ProcessID = rec.ID
				latest.ExitCode = exitCode
				if latest.EndedAt.IsZero() {
					latest.EndedAt = time.Now()
				}
				if latest.Progress == "" {
					latest.Progress = "任务已停止"
				}
				snapshot := latest
				task = latest
				taskMu.Unlock()
				_ = os.WriteFile(outputPath, []byte(modelOutputText(snapshot)), 0o600)
				_ = store.UpdateTask(snapshot)
				ensureTaskStatusCard(context.Background(), store, snapshot, "stopped")
				return
			}
			task.PID = rec.PID
			task.ProcessID = rec.ID
			task.ExitCode = exitCode
			task.EndedAt = time.Now()
			task.NativeSessionID = firstSessionID(output, thread.NativeSessionID)
			task.NativeResume = task.NativeSessionID != ""
			result := extractAgentText(p.Agent, output)
			if result == "" {
				result = strings.TrimSpace(task.StreamOutput)
			}
			if runErr != nil {
				task.Status = threadstore.TaskStatusFailed
				task.Error = runErr.Error()
				task.Progress = errorSummary(output, runErr)
				task.FinalResult = ""
			} else {
				task.Status = threadstore.TaskStatusDone
				task.FinalResult = result
				if len(result) > 200 {
					task.Progress = result[:200] + "..."
				} else {
					task.Progress = result
				}
			}
			snapshot := task
			taskMu.Unlock()
			_ = os.WriteFile(outputPath, []byte(modelOutputText(snapshot)), 0o600)
			_ = store.UpdateTask(snapshot)
			if snapshot.NativeSessionID != "" && snapshot.NativeSessionID != thread.NativeSessionID && snapshot.Status == threadstore.TaskStatusDone {
				thread.NativeSessionID = snapshot.NativeSessionID
				thread.NativeResume = true
				_ = store.UpdateThread(thread)
			} else if snapshot.Status == threadstore.TaskStatusFailed {
				thread.NativeSessionID = ""
				thread.NativeResume = false
				_ = store.UpdateThread(thread)
			}
			ensureTaskStatusCard(context.Background(), store, snapshot, "final")
		},
	})
	if err != nil {
		task.Status = threadstore.TaskStatusFailed
		task.Error = err.Error()
		task.Progress = taskStartErrorMessage(err)
		task.EndedAt = time.Now()
		_ = store.UpdateTask(task)
		ensureTaskStatusCard(context.Background(), store, task, "start_failed")
		return agentprocess.Record{}, thread, task, err
	}
	task.PID = rec.PID
	task.ProcessID = rec.ID
	_ = store.UpdateTask(task)
	return rec, thread, task, nil
}

func sendThreadListCard(ctx context.Context, profile string, page int) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	profile, p := profileOrActive(cfg, profile)
	store, err := threadStore()
	if err != nil {
		return err
	}
	threads, err := store.ListThreads(profile, p.Workspace, false)
	if err != nil {
		return err
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * threadPageSize
	if start > len(threads) {
		start = len(threads)
	}
	end := start + threadPageSize
	if end > len(threads) {
		end = len(threads)
	}
	summaries := make([]notify.ThreadSummary, 0, end-start)
	for _, thread := range threads[start:end] {
		summaries = append(summaries, notify.ThreadSummary{
			ID:        thread.ID,
			Number:    thread.Number,
			Title:     thread.Title,
			Status:    thread.Status,
			UpdatedAt: thread.UpdatedAt.Format("01-02 15:04"),
		})
	}
	sender, err := feishuSenderForProfile(cfg, profile)
	if err != nil {
		return err
	}
	_, err = sender.SendRawCard(ctx, notify.BuildThreadListCard(notify.ThreadListStatus{
		Profile:   profile,
		Workspace: p.Workspace,
		Page:      page,
		HasPrev:   page > 1,
		HasNext:   end < len(threads),
		Threads:   summaries,
	}))
	return err
}

func sendThreadDetailCard(ctx context.Context, thread threadstore.Thread) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	sender, err := feishuSenderForProfile(cfg, thread.Profile)
	if err != nil {
		return err
	}
	_, err = sender.SendRawCard(ctx, notify.BuildThreadDetailCard(notify.ThreadDetailStatus{
		Profile:    thread.Profile,
		Workspace:  thread.Workspace,
		ThreadID:   thread.ID,
		Number:     thread.Number,
		Title:      thread.Title,
		Status:     thread.Status,
		Agent:      thread.Agent,
		LastTaskID: thread.LastTaskID,
		UpdatedAt:  thread.UpdatedAt.Format("01-02 15:04"),
	}))
	return err
}

func sendTaskStatusCard(ctx context.Context, task threadstore.Task) (string, error) {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return "", err
	}
	sender, err := feishuSenderForProfile(cfg, task.Profile)
	if err != nil {
		return "", err
	}
	return sender.SendRawCard(ctx, buildTaskStatus(task))
}

func updateTaskStatusCard(ctx context.Context, task threadstore.Task) error {
	if task.FeishuMessageID == "" {
		return fmt.Errorf("task %s has no feishu message id", task.ID)
	}
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}
	sender, err := feishuSenderForProfile(cfg, task.Profile)
	if err != nil {
		return err
	}
	return sender.UpdateRawCard(ctx, task.FeishuMessageID, buildTaskStatus(task))
}

func ensureTaskStatusCard(ctx context.Context, store *threadstore.Store, task threadstore.Task, stage string) {
	// stream 阶段不再更新原卡。飞书 UpdateMessage 对这些交互卡片
	// 长期返回 invalid msg_type，运行中只保留初始卡，终态另发结果卡。
	if !isTaskLifecycleBoundary(stage) {
		if task.FeishuMessageID == "" {
			_ = appendAudit(fmt.Sprintf("feishu task card skip stage=%s task=%s: no message id", stage, task.ID))
		}
		return
	}

	oldMessageID := task.FeishuMessageID
	messageID, err := sendTaskStatusCard(ctx, task)
	if err != nil {
		_ = appendAudit(fmt.Sprintf("feishu task card send failed stage=%s task=%s err=%v", stage, task.ID, err))
		return
	}
	task.FeishuMessageID = messageID
	_ = store.UpdateTask(task)
	if oldMessageID != "" {
		_ = appendAudit(fmt.Sprintf("feishu task card final sent stage=%s task=%s running_message=%s final_message=%s", stage, task.ID, oldMessageID, messageID))
	} else {
		_ = appendAudit(fmt.Sprintf("feishu task card sent stage=%s task=%s message=%s", stage, task.ID, messageID))
	}
}

func isTaskLifecycleBoundary(stage string) bool {
	switch stage {
	case "final", "stopped", "start_failed", "workspace_failed":
		return true
	default:
		return false
	}
}

func buildTaskStatus(task threadstore.Task) map[string]any {
	return notify.BuildTaskStatusCard(notify.TaskStatus{
		Profile:   task.Profile,
		Workspace: task.Workspace,
		ThreadID:  task.ThreadID,
		TaskID:    task.ID,
		Number:    task.Number,
		Status:    task.Status,
		Progress:  clamp(task.Progress, 1200),
		Final:     task.FinalResult,
		LogPath:   task.LogPath,
	})
}

func validateTaskWorkspace(workspace string) error {
	if strings.TrimSpace(workspace) == "" {
		return fmt.Errorf("workspace is not set")
	}
	return agentprocess.ValidateWorkspace(workspace)
}

func taskStartErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "workspace is not set") {
		return "当前项目未设置。请在这个 Agent 的飞书对话里发送 `/cd /具体/项目/目录`，例如 `/cd /Users/victoria/vibe-coding/agent完成提示/agent-notify-go`，然后重新发送任务。"
	}
	if strings.Contains(msg, "workspace is too broad") {
		return "当前项目目录范围太大，不能把用户目录、下载目录、桌面或系统目录作为 Agent 工作区。请发送 `/cd /具体/项目/目录`，例如 `/cd /Users/victoria/vibe-coding/agent完成提示/agent-notify-go`，然后重新发送任务。"
	}
	if strings.Contains(msg, "workspace is not a directory") {
		return "当前项目目录不存在或不是文件夹。请发送 `/cd /具体/项目/目录` 后重试。"
	}
	return msg
}

func taskText(ref, mode string, lines int) (string, error) {
	store, err := threadStore()
	if err != nil {
		return "", err
	}
	task, err := store.GetTask(ref)
	if err != nil {
		return "", err
	}
	return taskTextByTask(task, mode, lines)
}

func taskTextByTask(task threadstore.Task, mode string, lines int) (string, error) {
	switch mode {
	case "result":
		if text := modelOutputText(task); text != "" {
			return text, nil
		}
		return task.Progress, nil
	case "tail":
		if task.FinalResult != "" {
			if lines <= 0 {
				lines = 80
			}
			return tailLines(task.FinalResult, lines), nil
		}
		raw, err := os.ReadFile(task.LogPath)
		if err != nil {
			return "", err
		}
		if lines <= 0 {
			lines = 80
		}
		return tailLines(string(raw), lines), nil
	case "log":
		if task.FinalResult != "" {
			return task.FinalResult, nil
		}
		raw, err := os.ReadFile(task.LogPath)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	default:
		return "", fmt.Errorf("unknown task text mode: %s", mode)
	}
}

func taskFinalOutputText(task threadstore.Task) string {
	if text := strings.TrimSpace(task.FinalResult); text != "" {
		return text
	}
	if text := strings.TrimSpace(task.Error); text != "" {
		return text
	}
	if text := strings.TrimSpace(task.Progress); text != "" {
		return text
	}
	return "（无模型输出）"
}

func runningTaskForThread(store *threadstore.Store, threadID string) (threadstore.Task, error) {
	tasks, err := store.ListTasks(threadID)
	if err != nil {
		return threadstore.Task{}, err
	}
	for _, task := range tasks {
		if task.Status == threadstore.TaskStatusRunning {
			return task, nil
		}
	}
	return threadstore.Task{}, nil
}

type modelStreamFragment struct {
	Output    string
	Reasoning string
}

func parseModelStreamLine(line string) modelStreamFragment {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return modelStreamFragment{}
	}
	var item map[string]any
	if err := json.Unmarshal([]byte(line), &item); err != nil {
		return modelStreamFragment{}
	}
	if text := resultText(item); text != "" {
		return modelStreamFragment{Output: text}
	}
	if text := deltaText(item); text != "" {
		return modelStreamFragment{Output: text}
	}
	if text := assistantMessageText(item); text != "" {
		return modelStreamFragment{Output: text}
	}
	if text := reasoningText(item); text != "" {
		return modelStreamFragment{Reasoning: text}
	}
	return modelStreamFragment{}
}

func modelOutputText(task threadstore.Task) string {
	var parts []string
	if strings.TrimSpace(task.ReasoningTrace) != "" {
		parts = append(parts, "思考摘要\n"+strings.TrimSpace(task.ReasoningTrace))
	}
	if strings.TrimSpace(task.StreamOutput) != "" {
		parts = append(parts, "流式输出\n"+strings.TrimSpace(task.StreamOutput))
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}
	return strings.TrimSpace(task.FinalResult)
}

func streamProgress(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "模型输出中..."
	}
	const limit = 1200
	if len(text) <= limit {
		return text
	}
	return text[len(text)-limit:]
}

func resolveTaskForThread(store *threadstore.Store, threadID, ref string) (threadstore.Task, error) {
	if threadID != "" {
		if task, err := store.ResolveTask(threadID, ref); err == nil {
			return task, nil
		}
	}
	return store.GetTask(ref)
}

func threadStore() (*threadstore.Store, error) {
	threadsPath, err := config.ThreadsPath()
	if err != nil {
		return nil, err
	}
	tasksPath, err := config.TasksPath()
	if err != nil {
		return nil, err
	}
	viewsPath, err := config.ViewsPath()
	if err != nil {
		return nil, err
	}
	return threadstore.New(threadsPath, tasksPath, viewsPath), nil
}

func profileOrActive(cfg config.Config, profile string) (string, config.ProfileConfig) {
	if profile == "" {
		profile = cfg.Broker.ActiveProfile
	}
	return profile, ensureProfile(&cfg, profile)
}

func extractAgentText(_ string, output string) string {
	result := extractStructuredAgentText(output)
	if result != "" {
		return result
	}
	return finalResult(output)
}

func extractClaudeText(output string) string {
	return extractStructuredAgentText(output)
}

func extractStructuredAgentText(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if text := resultText(item); text != "" {
			return text
		}
	}
	var deltaParts []string
	var messageParts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `{"type":"stream_event"`) {
			var item map[string]any
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				continue
			}
			if text := assistantMessageText(item); text != "" {
				messageParts = append(messageParts, text)
			}
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if text := deltaText(item); text != "" {
			deltaParts = append(deltaParts, text)
		}
		if text := assistantMessageText(item); text != "" {
			messageParts = append(messageParts, text)
		}
	}
	if len(deltaParts) > 0 {
		return strings.Join(deltaParts, "")
	}
	if len(messageParts) > 0 {
		return messageParts[len(messageParts)-1]
	}
	return ""
}

func resultText(item map[string]any) string {
	if text, _ := item["result"].(string); text != "" {
		return strings.TrimSpace(text)
	}
	if text, _ := item["last_assistant_message"].(string); text != "" {
		return strings.TrimSpace(text)
	}
	return ""
}

func deltaText(item map[string]any) string {
	eventMap, _ := item["event"].(map[string]any)
	deltaMap, _ := eventMap["delta"].(map[string]any)
	text, _ := deltaMap["text"].(string)
	return text
}

func assistantMessageText(item map[string]any) string {
	if text := assistantMessageTextFromMap(item); text != "" {
		return text
	}
	for _, key := range []string{"item", "data", "message"} {
		child, _ := item[key].(map[string]any)
		if text := assistantMessageTextFromMap(child); text != "" {
			return text
		}
	}
	return ""
}

func reasoningText(item map[string]any) string {
	if text := reasoningTextFromMap(item); text != "" {
		return text
	}
	for _, key := range []string{"event", "delta", "item", "data", "message"} {
		child, _ := item[key].(map[string]any)
		if text := reasoningTextFromMap(child); text != "" {
			return text
		}
	}
	return ""
}

func reasoningTextFromMap(item map[string]any) string {
	if len(item) == 0 {
		return ""
	}
	typ, _ := item["type"].(string)
	if !looksLikeReasoning(typ) {
		for _, key := range []string{"reasoning", "reasoning_summary", "summary", "thinking", "thought"} {
			if text := contentText(item[key]); text != "" {
				return strings.TrimSpace(text)
			}
		}
		return ""
	}
	for _, key := range []string{"text", "summary", "content", "message", "reasoning", "thinking"} {
		if text := contentText(item[key]); text != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func looksLikeReasoning(typ string) bool {
	typ = strings.ToLower(typ)
	return strings.Contains(typ, "reasoning") || strings.Contains(typ, "thinking") || strings.Contains(typ, "thought") || strings.Contains(typ, "summary")
}

func assistantMessageTextFromMap(item map[string]any) string {
	if len(item) == 0 {
		return ""
	}
	role, _ := item["role"].(string)
	typ, _ := item["type"].(string)
	if role != "assistant" && !strings.Contains(typ, "assistant") && !strings.Contains(typ, "agent_message") {
		return ""
	}
	if text, _ := item["message"].(string); text != "" {
		return strings.TrimSpace(text)
	}
	if text, _ := item["text"].(string); text != "" {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(contentText(item["content"]))
}

func contentText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			switch part := item.(type) {
			case string:
				parts = append(parts, part)
			case map[string]any:
				for _, key := range []string{"text", "output_text", "content"} {
					if text := contentText(part[key]); text != "" {
						parts = append(parts, text)
						break
					}
				}
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func errorSummary(output string, runErr error) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Error:") || strings.Contains(line, "error:") || strings.Contains(line, "Error:") ||
			strings.Contains(line, "not found") || strings.Contains(line, "not exist") || strings.Contains(line, "No conversation found") {
			return line
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "{") {
			return line
		}
	}
	if runErr != nil {
		return runErr.Error()
	}
	return "未知错误"
}
func tailLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func finalResult(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	out := make([]string, 0, 20)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "{") {
			continue
		}
		out = append(out, line)
		if len(out) > 30 {
			out = out[len(out)-30:]
		}
	}
	if len(out) > 0 {
		return strings.Join(out, "\n")
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) > 30 {
			out = out[len(out)-30:]
		}
	}
	return strings.Join(out, "\n")
}

func firstSessionID(output, fallback string) string {
	for _, marker := range []string{`"session_id":"`, `"sessionId":"`, `"conversation_id":"`} {
		if idx := strings.Index(output, marker); idx >= 0 {
			rest := output[idx+len(marker):]
			if end := strings.Index(rest, `"`); end > 0 {
				return rest[:end]
			}
		}
	}
	return fallback
}

func clamp(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit-20] + "\n...(已截断)"
}
