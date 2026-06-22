package state

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
)

func tempSessionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sessions.json")
}

func TestCodexStopNotifies(t *testing.T) {
	// Codex 只发 Stop，无 PermissionRequest 前置事件
	// 状态机不应判定为 idle stop
	store := NewSessionStore(tempSessionPath(t))
	adv := NewAdvancer(store)

	evt := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "codex",
		HookEvent:   "Stop",
		Status:      event.StatusPending,
		SessionID:   "sess-codex-1",
		Workspace:   "/tmp",
		ReceivedAt:  time.Now(),
	}

	decision, err := adv.Advance(evt)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if !decision.Notify {
		t.Fatalf("Codex Stop should notify, got Notify=false reason=%s", decision.Reason)
	}
	if decision.Status != event.StatusCompleted {
		t.Fatalf("Status = %q, want %q", decision.Status, event.StatusCompleted)
	}
}

func TestClaudeStopIdleDoesNotNotify(t *testing.T) {
	// Claude Code 可能空闲时触发 Stop（打开即关闭）
	// 无前置事件时不应通知
	store := NewSessionStore(tempSessionPath(t))
	adv := NewAdvancer(store)

	evt := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "claude_code",
		HookEvent:   "Stop",
		Status:      event.StatusPending,
		SessionID:   "sess-claude-1",
		Workspace:   "/tmp",
		ReceivedAt:  time.Now(),
	}

	decision, err := adv.Advance(evt)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if decision.Notify {
		t.Fatalf("Claude idle Stop should NOT notify, got Notify=true reason=%s", decision.Reason)
	}
}

func TestPermissionRequestNotifies(t *testing.T) {
	store := NewSessionStore(tempSessionPath(t))
	adv := NewAdvancer(store)

	evt := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "codex",
		HookEvent:   "PermissionRequest",
		Status:      event.StatusPermissionReq,
		SessionID:   "sess-codex-2",
		Workspace:   "/tmp",
		ReceivedAt:  time.Now(),
	}

	decision, err := adv.Advance(evt)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if !decision.Notify {
		t.Fatal("PermissionRequest should notify")
	}
	if decision.Status != event.StatusPermissionReq {
		t.Fatalf("Status = %q, want %q", decision.Status, event.StatusPermissionReq)
	}
}

func TestStopAfterPermissionNotifiesOnce(t *testing.T) {
	store := NewSessionStore(tempSessionPath(t))
	adv := NewAdvancer(store)

	// Send PermissionRequest first
	perm := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "codex",
		HookEvent:   "PermissionRequest",
		Status:      event.StatusPermissionReq,
		SessionID:   "sess-codex-3",
		Workspace:   "/tmp",
		ReceivedAt:  time.Now(),
	}
	permDecision, err := adv.Advance(perm)
	if err != nil {
		t.Fatal(err)
	}
	if !permDecision.Notify {
		t.Fatal("PermissionRequest should notify")
	}

	// Then Stop — should also notify as completed
	stop := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "codex",
		HookEvent:   "Stop",
		Status:      event.StatusPending,
		SessionID:   "sess-codex-3",
		Workspace:   "/tmp",
		ReceivedAt:  time.Now(),
	}
	stopDecision, err := adv.Advance(stop)
	if err != nil {
		t.Fatal(err)
	}
	if !stopDecision.Notify {
		t.Fatal("Stop after permission should notify")
	}
	if stopDecision.Status != event.StatusCompleted {
		t.Fatalf("Status = %q, want %q", stopDecision.Status, event.StatusCompleted)
	}

		// Third Stop — 状态机不阻止，由 dispatcher 时间窗口去重
	dupStop := stop
	dupStop.EventID = event.NewEventID()
	dupDecision, err := adv.Advance(dupStop)
	if err != nil {
		t.Fatal(err)
	}
	if !dupDecision.Notify {
			t.Fatal("duplicate Stop should notify (delegate dedup to dispatcher)")
	}
}

func TestPostToolUseFailureOverridesCompleted(t *testing.T) {
	store := NewSessionStore(tempSessionPath(t))
	adv := NewAdvancer(store)

	// Stop (completed)
	stop := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "claude_code",
		HookEvent:   "Stop",
		Status:      event.StatusPending,
		SessionID:   "sess-claude-2",
		Workspace:   "/tmp",
		ReceivedAt:  time.Now(),
	}
	// First set HasRunEvent via PermissionRequest
	p1, _ := adv.Advance(event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "claude_code",
		HookEvent:   "PermissionRequest",
		Status:      event.StatusPermissionReq,
		SessionID:   "sess-claude-2", Workspace: "/tmp", ReceivedAt: time.Now(),
	})
	if !p1.Notify {
		t.Fatal("first PermissionRequest should notify")
	}

	d1, _ := adv.Advance(stop)
	if !d1.Notify {
		t.Fatal("Stop after permission should notify")
	}

	// PostToolUseFailure later — should override
	fail := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "claude_code",
		HookEvent:   "PostToolUseFailure",
		Status:      event.StatusFailed,
		SessionID:   "sess-claude-2",
		Workspace:   "/tmp",
		ReceivedAt:  time.Now(),
	}
	failDecision, err := adv.Advance(fail)
	if err != nil {
		t.Fatal(err)
	}
	if !failDecision.Notify {
		t.Fatal("PostToolUseFailure after completed should notify")
	}
	if failDecision.Status != event.StatusFailed {
		t.Fatalf("Status = %q, want %q", failDecision.Status, event.StatusFailed)
	}
}

func TestSessionPrune(t *testing.T) {
	path := tempSessionPath(t)
	store := NewSessionStore(path)

	// Create a session record
	rec := SessionRecord{
		SessionID: "old-sess",
		Status:    SessCompleted,
		UpdatedAt: time.Now().Add(-48 * time.Hour),
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}

	// Prune older than 24h
	if err := store.PruneOlderThan(time.Now().Add(-24 * time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Verify it's gone
	loaded, err := store.Load("old-sess")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != SessNew {
		t.Fatal("pruned session should return new record")
	}

}
