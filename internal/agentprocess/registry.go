package agentprocess

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Record struct {
	ID             string    `json:"id"`
	Profile        string    `json:"profile"`
	Agent          string    `json:"agent"`
	Workspace      string    `json:"workspace"`
	PermissionMode string    `json:"permission_mode"`
	PID            int       `json:"pid"`
	Command        []string  `json:"command"`
	PromptPreview  string    `json:"prompt_preview"`
	Status         string    `json:"status"`
	StartedAt      time.Time `json:"started_at"`
	EndedAt        time.Time `json:"ended_at,omitempty"`
	LogPath        string    `json:"log_path,omitempty"`
}

type StartOptions struct {
	Profile         string
	Agent           string
	Workspace       string
	PermissionMode  string
	Prompt          string
	LogsDir         string
	ThreadID        string
	TaskID          string
	NativeSessionID string
	Resume          bool
	OnOutput        func(string)
	OnExit          func(Record, string, int, error)
}

type Registry struct {
	path string
	mu   sync.Mutex
}

type fileData struct {
	Processes map[string]Record `json:"processes"`
}

func NewRegistry(path string) *Registry {
	return &Registry{path: path}
}

func (r *Registry) Start(ctx context.Context, profile, agent, workspace, permissionMode, prompt, logsDir string) (Record, error) {
	return r.StartWithOptions(ctx, StartOptions{
		Profile:        profile,
		Agent:          agent,
		Workspace:      workspace,
		PermissionMode: permissionMode,
		Prompt:         prompt,
		LogsDir:        logsDir,
	})
}

func (r *Registry) StartWithOptions(ctx context.Context, opts StartOptions) (Record, error) {
	profile := opts.Profile
	agent := opts.Agent
	workspace := opts.Workspace
	permissionMode := opts.PermissionMode
	prompt := opts.Prompt
	logsDir := opts.LogsDir
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return Record{}, err
		}
	}
	if err := ValidateWorkspace(workspace); err != nil {
		return Record{}, err
	}
	id := opts.TaskID
	if id == "" {
		id = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	logName := profile + "-" + id + ".log"
	if opts.ThreadID != "" {
		logName = profile + "-" + opts.ThreadID + "-" + id + ".log"
	}
	logPath := filepath.Join(logsDir, logName)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return Record{}, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return Record{}, err
	}

	args, err := commandArgsWithSession(agent, workspace, permissionMode, prompt, opts.NativeSessionID, opts.Resume)
	if err != nil {
		_ = logFile.Close()
		return Record{}, err
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), remoteEnvEntries(profile, id)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = logFile.Close()
		return Record{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = logFile.Close()
		return Record{}, err
	}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return Record{}, err
	}

	rec := Record{
		ID:             id,
		Profile:        profile,
		Agent:          agent,
		Workspace:      workspace,
		PermissionMode: permissionMode,
		PID:            cmd.Process.Pid,
		Command:        args,
		PromptPreview:  preview(prompt, 120),
		Status:         "running",
		StartedAt:      time.Now(),
		LogPath:        logPath,
	}
	if err := r.Save(rec); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return Record{}, err
	}

	go func() {
		var outputMu sync.Mutex
		var logMu sync.Mutex
		var output strings.Builder
		copyPipe := func(reader io.Reader) {
			scanner := bufio.NewScanner(reader)
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				logMu.Lock()
				_, _ = logFile.WriteString(line + "\n")
				logMu.Unlock()
				outputMu.Lock()
				output.WriteString(line)
				output.WriteByte('\n')
				outputMu.Unlock()
				if opts.OnOutput != nil {
					opts.OnOutput(line)
				}
			}
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); copyPipe(stdout) }()
		go func() { defer wg.Done(); copyPipe(stderr) }()
		err := cmd.Wait()
		wg.Wait()
		_ = logFile.Close()
		rec.EndedAt = time.Now()
		exitCode := 0
		if err != nil {
			rec.Status = "exited_error"
			exitCode = 1
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		} else {
			rec.Status = "exited"
		}
		_ = r.Save(rec)
		outputMu.Lock()
		finalOutput := output.String()
		outputMu.Unlock()
		if opts.OnExit != nil {
			opts.OnExit(rec, finalOutput, exitCode, err)
		}
	}()

	return rec, nil
}

func remoteEnvEntries(profile, taskID string) []string {
	return []string{
		"AGENT_NOTIFY_REMOTE_PROFILE=" + profile,
		"AGENT_NOTIFY_REMOTE_TASK_ID=" + taskID,
	}
}

func (r *Registry) Save(rec Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := r.load()
	if err != nil {
		return err
	}
	if data.Processes == nil {
		data.Processes = map[string]Record{}
	}
	data.Processes[rec.ID] = rec
	return r.save(data)
}

func (r *Registry) List(profile string) ([]Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := r.load()
	if err != nil {
		return nil, err
	}
	changed := false
	out := make([]Record, 0, len(data.Processes))
	for id, rec := range data.Processes {
		if rec.Status == "running" && !processAlive(rec.PID) {
			rec.Status = "exited_unknown"
			rec.EndedAt = time.Now()
			data.Processes[id] = rec
			changed = true
		}
		if profile == "" || rec.Profile == profile {
			out = append(out, rec)
		}
	}
	if changed {
		_ = r.save(data)
	}
	return out, nil
}

func (r *Registry) Kill(id string) error {
	rec, ok, err := r.find(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("process %s not found", id)
	}
	if rec.PID <= 0 {
		return fmt.Errorf("process %s has no pid", id)
	}
	proc, err := os.FindProcess(rec.PID)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	rec.Status = "stopping"
	return r.Save(rec)
}

func (r *Registry) KillProfile(profile string) error {
	items, err := r.List(profile)
	if err != nil {
		return err
	}
	for _, rec := range items {
		if rec.Status == "running" || rec.Status == "stopping" {
			_ = r.Kill(rec.ID)
		}
	}
	return nil
}

func (r *Registry) find(id string) (Record, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := r.load()
	if err != nil {
		return Record{}, false, err
	}
	if rec, ok := data.Processes[id]; ok {
		return rec, true, nil
	}
	for _, rec := range data.Processes {
		if rec.ID == id || strconv.Itoa(rec.PID) == id {
			return rec, true, nil
		}
	}
	return Record{}, false, nil
}

func (r *Registry) load() (fileData, error) {
	raw, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileData{Processes: map[string]Record{}}, nil
		}
		return fileData{}, err
	}
	data := fileData{Processes: map[string]Record{}}
	if len(raw) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fileData{}, err
	}
	if data.Processes == nil {
		data.Processes = map[string]Record{}
	}
	return data, nil
}

func (r *Registry) save(data fileData) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func commandArgs(agent, workspace, permissionMode, prompt string) ([]string, error) {
	return commandArgsWithSession(agent, workspace, permissionMode, prompt, "", false)
}

func commandArgsWithSession(agent, workspace, permissionMode, prompt, sessionID string, resume bool) ([]string, error) {
	switch agent {
	case "claude", "claude_code":
		mode := "acceptEdits"
		if permissionMode == "read-only" {
			mode = "plan"
		}
		args := []string{"claude", "--print", "--verbose", "--output-format", "stream-json", "--include-partial-messages", "--permission-mode", mode, "--add-dir", workspace}
		if resume && sessionID != "" {
			args = append(args, "--resume", sessionID)
		} else if sessionID != "" {
			args = append(args, "--session-id", sessionID)
		}
		args = append(args, prompt)
		return args, nil
	case "codex":
		sandbox := permissionMode
		if sandbox == "" {
			sandbox = "workspace-write"
		}
		if sandbox != "read-only" && sandbox != "workspace-write" {
			sandbox = "workspace-write"
		}
		if resume && sessionID != "" {
			return []string{"codex", "--ask-for-approval", "on-request", "exec", "--json", "--sandbox", sandbox, "--cd", workspace, "--skip-git-repo-check", "resume", sessionID, prompt}, nil
		}
		return []string{"codex", "--ask-for-approval", "on-request", "exec", "--json", "--sandbox", sandbox, "--cd", workspace, "--skip-git-repo-check", prompt}, nil
	default:
		return nil, fmt.Errorf("unsupported agent: %s", agent)
	}
}

func ValidateWorkspace(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace is not a directory: %s", path)
	}
	clean, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	}
	home, _ := os.UserHomeDir()
	temp, _ := filepath.Abs(os.TempDir())
	if resolvedTemp, err := filepath.EvalSymlinks(temp); err == nil {
		temp = resolvedTemp
	}
	blocked := []string{"/", home, temp, filepath.Join(home, "Desktop"), filepath.Join(home, "Downloads"), "/Applications", "/Library", "/System", "/Users", "/Volumes"}
	for _, root := range blocked {
		if root != "" && clean == root {
			return fmt.Errorf("workspace is too broad: %s", path)
		}
	}
	for _, prefix := range []string{"/System/", "/Library/", "/Applications/", "/Volumes/"} {
		if strings.HasPrefix(clean, prefix) {
			return fmt.Errorf("workspace is outside an allowed project area: %s", path)
		}
	}
	if rel, err := filepath.Rel(temp, clean); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("workspace is too broad: %s", path)
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func preview(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit-3] + "..."
}
