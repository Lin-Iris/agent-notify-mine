package threadstore

import (
	"path/filepath"
	"testing"
)

func TestStoreThreadsAreIsolatedByWorkspace(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "threads.json"), filepath.Join(t.TempDir(), "tasks.json"), filepath.Join(t.TempDir(), "views.json"))

	a, err := store.CreateThread("claude-main", "/repo/a", "claude", "A")
	if err != nil {
		t.Fatalf("CreateThread A error = %v", err)
	}
	b, err := store.CreateThread("claude-main", "/repo/b", "claude", "B")
	if err != nil {
		t.Fatalf("CreateThread B error = %v", err)
	}
	if a.Number != 1 || b.Number != 1 {
		t.Fatalf("thread numbers = %d/%d, want isolated #1/#1", a.Number, b.Number)
	}

	threads, err := store.ListThreads("claude-main", "/repo/a", false)
	if err != nil {
		t.Fatalf("ListThreads error = %v", err)
	}
	if len(threads) != 1 || threads[0].Title != "A" {
		t.Fatalf("ListThreads = %+v, want only A", threads)
	}
}

func TestStoreTaskLifecycleUpdatesThread(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "threads.json"), filepath.Join(t.TempDir(), "tasks.json"), filepath.Join(t.TempDir(), "views.json"))
	thread, err := store.CreateThread("claude-main", "/repo", "claude", "Work")
	if err != nil {
		t.Fatalf("CreateThread error = %v", err)
	}
	task, err := store.CreateTaskWithID(thread, "task_test", "prompt", "/tmp/task.log", "/tmp/task.out")
	if err != nil {
		t.Fatalf("CreateTask error = %v", err)
	}
	if task.Number != 1 || task.Status != TaskStatusRunning {
		t.Fatalf("task = %+v, want running #1", task)
	}
	task.Status = TaskStatusDone
	task.FinalResult = "done"
	if err := store.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask error = %v", err)
	}
	gotThread, err := store.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread error = %v", err)
	}
	if gotThread.Status != ThreadStatusComplete || gotThread.LastTaskID != task.ID {
		t.Fatalf("thread after task = %+v, want complete last task", gotThread)
	}
}
