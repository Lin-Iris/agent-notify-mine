package threadstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ThreadStatusIdle     = "idle"
	ThreadStatusRunning  = "running"
	ThreadStatusComplete = "complete"
	ThreadStatusFailed   = "failed"
	ThreadStatusArchived = "archived"

	TaskStatusRunning = "running"
	TaskStatusDone    = "done"
	TaskStatusFailed  = "failed"
	TaskStatusStopped = "stopped"
)

type Thread struct {
	ID              string    `json:"id"`
	Number          int       `json:"number"`
	Profile         string    `json:"profile"`
	Workspace       string    `json:"workspace"`
	Agent           string    `json:"agent"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`
	NativeSessionID string    `json:"native_session_id,omitempty"`
	NativeResume    bool      `json:"native_resume"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastTaskID      string    `json:"last_task_id,omitempty"`
	Archived        bool      `json:"archived"`
}

type Task struct {
	ID              string    `json:"id"`
	Number          int       `json:"number"`
	ThreadID        string    `json:"thread_id"`
	Profile         string    `json:"profile"`
	Workspace       string    `json:"workspace"`
	Agent           string    `json:"agent"`
	Prompt          string    `json:"prompt"`
	Status          string    `json:"status"`
	PID             int       `json:"pid,omitempty"`
	ProcessID       string    `json:"process_id,omitempty"`
	LogPath         string    `json:"log_path,omitempty"`
	OutputPath      string    `json:"output_path,omitempty"`
	FeishuMessageID string    `json:"feishu_message_id,omitempty"`
	FinalResult     string    `json:"final_result,omitempty"`
	StreamOutput    string    `json:"stream_output,omitempty"`
	ReasoningTrace  string    `json:"reasoning_trace,omitempty"`
	Progress        string    `json:"progress,omitempty"`
	ExitCode        int       `json:"exit_code,omitempty"`
	Error           string    `json:"error,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at,omitempty"`
	NativeResume    bool      `json:"native_resume"`
	NativeSessionID string    `json:"native_session_id,omitempty"`
}

type View struct {
	ID         string    `json:"id"`
	OperatorID string    `json:"operator_id,omitempty"`
	Profile    string    `json:"profile,omitempty"`
	Workspace  string    `json:"workspace,omitempty"`
	ThreadID   string    `json:"thread_id,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
	View       string    `json:"view"`
	Page       int       `json:"page,omitempty"`
	Previous   string    `json:"previous,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type Store struct {
	threadsPath string
	tasksPath   string
	viewsPath   string
	mu          sync.Mutex
}

type threadFile struct {
	Active  map[string]string `json:"active"`
	Threads map[string]Thread `json:"threads"`
}

type taskFile struct {
	Tasks map[string]Task `json:"tasks"`
}

type viewFile struct {
	Views map[string]View `json:"views"`
}

func New(threadsPath, tasksPath, viewsPath string) *Store {
	return &Store{threadsPath: threadsPath, tasksPath: tasksPath, viewsPath: viewsPath}
}

func (s *Store) EnsureActiveThread(profile, workspace, agent string) (Thread, error) {
	active, err := s.ActiveThread(profile, workspace)
	if err == nil && active.ID != "" {
		return active, nil
	}
	return s.CreateThread(profile, workspace, agent, "默认对话")
}

func (s *Store) CreateThread(profile, workspace, agent, title string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return Thread{}, err
	}
	now := time.Now()
	if title == "" {
		title = "新对话"
	}
	thread := Thread{
		ID:           "th_" + strconv.FormatInt(now.UnixNano(), 36),
		Number:       nextThreadNumber(tf.Threads, profile, workspace),
		Profile:      profile,
		Workspace:    workspace,
		Agent:        agent,
		Title:        title,
		Status:       ThreadStatusIdle,
		NativeResume: true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	tf.Threads[thread.ID] = thread
	if tf.Active == nil {
		tf.Active = map[string]string{}
	}
	tf.Active[activeKey(profile, workspace)] = thread.ID
	return thread, s.saveThreads(tf)
}

func (s *Store) ListThreads(profile, workspace string, includeArchived bool) ([]Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return nil, err
	}
	out := make([]Thread, 0, len(tf.Threads))
	for _, thread := range tf.Threads {
		if thread.Profile != profile || thread.Workspace != workspace {
			continue
		}
		if thread.Archived && !includeArchived {
			continue
		}
		out = append(out, thread)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) ActiveThread(profile, workspace string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return Thread{}, err
	}
	id := tf.Active[activeKey(profile, workspace)]
	if id == "" {
		return Thread{}, errors.New("active thread not found")
	}
	thread, ok := tf.Threads[id]
	if !ok || thread.Archived {
		return Thread{}, errors.New("active thread not found")
	}
	return thread, nil
}

func (s *Store) UseThread(profile, workspace, ref string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return Thread{}, err
	}
	thread, err := resolveThread(tf.Threads, profile, workspace, ref)
	if err != nil {
		return Thread{}, err
	}
	if tf.Active == nil {
		tf.Active = map[string]string{}
	}
	tf.Active[activeKey(profile, workspace)] = thread.ID
	return thread, s.saveThreads(tf)
}

func (s *Store) RenameThread(profile, workspace, ref, title string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return Thread{}, err
	}
	thread, err := resolveThread(tf.Threads, profile, workspace, ref)
	if err != nil {
		return Thread{}, err
	}
	thread.Title = title
	thread.UpdatedAt = time.Now()
	tf.Threads[thread.ID] = thread
	return thread, s.saveThreads(tf)
}

func (s *Store) ArchiveThread(profile, workspace, ref string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return Thread{}, err
	}
	thread, err := resolveThread(tf.Threads, profile, workspace, ref)
	if err != nil {
		return Thread{}, err
	}
	thread.Archived = true
	thread.Status = ThreadStatusArchived
	thread.UpdatedAt = time.Now()
	tf.Threads[thread.ID] = thread
	if tf.Active[activeKey(profile, workspace)] == thread.ID {
		delete(tf.Active, activeKey(profile, workspace))
	}
	return thread, s.saveThreads(tf)
}

func (s *Store) GetThread(id string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return Thread{}, err
	}
	thread, ok := tf.Threads[id]
	if !ok {
		return Thread{}, fmt.Errorf("thread %s not found", id)
	}
	return thread, nil
}

func (s *Store) UpdateThread(thread Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return err
	}
	thread.UpdatedAt = time.Now()
	tf.Threads[thread.ID] = thread
	return s.saveThreads(tf)
}

func (s *Store) CreateTask(thread Thread, prompt, logPath, outputPath string) (Task, error) {
	return s.CreateTaskWithID(thread, "", prompt, logPath, outputPath)
}

func (s *Store) CreateTaskWithID(thread Thread, id, prompt, logPath, outputPath string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tf, err := s.loadThreads()
	if err != nil {
		return Task{}, err
	}
	taskf, err := s.loadTasks()
	if err != nil {
		return Task{}, err
	}
	now := time.Now()
	if id == "" {
		id = "task_" + strconv.FormatInt(now.UnixNano(), 36)
	}
	task := Task{
		ID:              id,
		Number:          nextTaskNumber(taskf.Tasks, thread.ID),
		ThreadID:        thread.ID,
		Profile:         thread.Profile,
		Workspace:       thread.Workspace,
		Agent:           thread.Agent,
		Prompt:          prompt,
		Status:          TaskStatusRunning,
		LogPath:         logPath,
		OutputPath:      outputPath,
		StartedAt:       now,
		NativeResume:    thread.NativeResume,
		NativeSessionID: thread.NativeSessionID,
	}
	taskf.Tasks[task.ID] = task
	thread.Status = ThreadStatusRunning
	thread.LastTaskID = task.ID
	thread.UpdatedAt = now
	tf.Threads[thread.ID] = thread
	if err := s.saveTasks(taskf); err != nil {
		return Task{}, err
	}
	return task, s.saveThreads(tf)
}

func (s *Store) UpdateTask(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	taskf, err := s.loadTasks()
	if err != nil {
		return err
	}
	taskf.Tasks[task.ID] = task
	if err := s.saveTasks(taskf); err != nil {
		return err
	}
	tf, err := s.loadThreads()
	if err != nil {
		return err
	}
	if thread, ok := tf.Threads[task.ThreadID]; ok {
		thread.UpdatedAt = time.Now()
		thread.LastTaskID = task.ID
		if task.Status == TaskStatusRunning {
			thread.Status = ThreadStatusRunning
		} else if task.Status == TaskStatusFailed {
			thread.Status = ThreadStatusFailed
		} else {
			thread.Status = ThreadStatusComplete
		}
		if task.NativeSessionID != "" {
			thread.NativeSessionID = task.NativeSessionID
			thread.NativeResume = task.NativeResume
		}
		tf.Threads[thread.ID] = thread
		return s.saveThreads(tf)
	}
	return nil
}

func (s *Store) GetTask(ref string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	taskf, err := s.loadTasks()
	if err != nil {
		return Task{}, err
	}
	return resolveTask(taskf.Tasks, "", ref)
}

func (s *Store) ResolveTask(threadID, ref string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	taskf, err := s.loadTasks()
	if err != nil {
		return Task{}, err
	}
	return resolveTask(taskf.Tasks, threadID, ref)
}

func (s *Store) ListTasks(threadID string) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	taskf, err := s.loadTasks()
	if err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(taskf.Tasks))
	for _, task := range taskf.Tasks {
		if threadID == "" || task.ThreadID == threadID {
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out, nil
}

func (s *Store) SaveView(view View) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vf, err := s.loadViews()
	if err != nil {
		return err
	}
	now := time.Now()
	if view.ID == "" {
		view.ID = "view_" + strconv.FormatInt(now.UnixNano(), 36)
	}
	if view.CreatedAt.IsZero() {
		view.CreatedAt = now
	}
	if view.ExpiresAt.IsZero() {
		view.ExpiresAt = now.Add(24 * time.Hour)
	}
	vf.Views[view.ID] = view
	return s.saveViews(vf)
}

func (s *Store) loadThreads() (threadFile, error) {
	raw, err := os.ReadFile(s.threadsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return threadFile{Active: map[string]string{}, Threads: map[string]Thread{}}, nil
		}
		return threadFile{}, err
	}
	var tf threadFile
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &tf); err != nil {
			return threadFile{}, err
		}
	}
	if tf.Active == nil {
		tf.Active = map[string]string{}
	}
	if tf.Threads == nil {
		tf.Threads = map[string]Thread{}
	}
	return tf, nil
}

func (s *Store) saveThreads(tf threadFile) error {
	return writeJSON(s.threadsPath, tf)
}

func (s *Store) loadTasks() (taskFile, error) {
	raw, err := os.ReadFile(s.tasksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return taskFile{Tasks: map[string]Task{}}, nil
		}
		return taskFile{}, err
	}
	var tf taskFile
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &tf); err != nil {
			return taskFile{}, err
		}
	}
	if tf.Tasks == nil {
		tf.Tasks = map[string]Task{}
	}
	return tf, nil
}

func (s *Store) saveTasks(tf taskFile) error {
	return writeJSON(s.tasksPath, tf)
}

func (s *Store) loadViews() (viewFile, error) {
	raw, err := os.ReadFile(s.viewsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return viewFile{Views: map[string]View{}}, nil
		}
		return viewFile{}, err
	}
	var vf viewFile
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &vf); err != nil {
			return viewFile{}, err
		}
	}
	if vf.Views == nil {
		vf.Views = map[string]View{}
	}
	return vf, nil
}

func (s *Store) saveViews(vf viewFile) error {
	return writeJSON(s.viewsPath, vf)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func activeKey(profile, workspace string) string {
	return profile + "\x00" + workspace
}

func nextThreadNumber(threads map[string]Thread, profile, workspace string) int {
	max := 0
	for _, thread := range threads {
		if thread.Profile == profile && thread.Workspace == workspace && thread.Number > max {
			max = thread.Number
		}
	}
	return max + 1
}

func nextTaskNumber(tasks map[string]Task, threadID string) int {
	max := 0
	for _, task := range tasks {
		if task.ThreadID == threadID && task.Number > max {
			max = task.Number
		}
	}
	return max + 1
}

func resolveThread(threads map[string]Thread, profile, workspace, ref string) (Thread, error) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "#")
	for _, thread := range threads {
		if thread.Profile != profile || thread.Workspace != workspace || thread.Archived {
			continue
		}
		if thread.ID == ref || strconv.Itoa(thread.Number) == ref {
			return thread, nil
		}
	}
	return Thread{}, fmt.Errorf("thread %s not found", ref)
}

func resolveTask(tasks map[string]Task, threadID, ref string) (Task, error) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "#")
	for _, task := range tasks {
		if threadID != "" && task.ThreadID != threadID {
			continue
		}
		if task.ID == ref || task.ProcessID == ref || strconv.Itoa(task.Number) == ref {
			return task, nil
		}
		if task.PID > 0 && strconv.Itoa(task.PID) == ref {
			return task, nil
		}
	}
	return Task{}, fmt.Errorf("task %s not found", ref)
}
