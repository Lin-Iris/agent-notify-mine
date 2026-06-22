package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
)

// ── Types ──────────────────────────────────────────────────

// SessionStatus tracks the deduced lifecycle state of one session.
type SessionStatus string

const (
	SessNew       SessionStatus = "new"
	SessActive    SessionStatus = "active"
	SessCompleted SessionStatus = "completed"
	SessFailed    SessionStatus = "failed"
	SessCancelled SessionStatus = "cancelled"
)

// SessionRecord holds the deduced state for a single agent session.
type SessionRecord struct {
	SessionID   string        `json:"session_id"`
	Agent       string        `json:"agent"`
	Workspace   string        `json:"workspace"`
	Status      SessionStatus `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	HasRunEvent bool          `json:"has_run_event"` // true if any actionable event seen
	EventCount  int           `json:"event_count"`
	Notified    bool          `json:"notified"`       // terminal notification already sent
	FailureMsg  string        `json:"failure_msg,omitempty"`
}

// sessionFile is the on-disk format.
type sessionFile struct {
	Sessions map[string]SessionRecord `json:"sessions"`
}

// AdvanceDecision is returned by Advancer.Advance().
type AdvanceDecision struct {
	Notify bool              // true = caller should send a notification
	Status event.Status      // deduced status for the notification
	Reason string            // human-readable explanation (for logging)
}

// ── SessionStore ───────────────────────────────────────────

// SessionStore persists session records to a JSON file.
// Concurrent-safe via mutex.
type SessionStore struct {
	path string
	mu   sync.Mutex
}

// NewSessionStore creates or loads a session store at the given path.
func NewSessionStore(path string) *SessionStore {
	return &SessionStore{path: path}
}

// Load retrieves a session record, or returns a new one if not found.
func (s *SessionStore) Load(sessionID string) (SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.loadFile()
	if err != nil {
		return SessionRecord{}, err
	}
	rec, ok := sf.Sessions[sessionID]
	if !ok {
		return SessionRecord{
			SessionID: sessionID,
			Status:    SessNew,
		}, nil
	}
	return rec, nil
}

// Save persists a session record.
func (s *SessionStore) Save(rec SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.loadFile()
	if err != nil {
		return err
	}
	if sf.Sessions == nil {
		sf.Sessions = make(map[string]SessionRecord)
	}
	sf.Sessions[rec.SessionID] = rec
	return s.saveFile(sf)
}

// PruneOlderThan removes sessions not updated since the given time.
func (s *SessionStore) PruneOlderThan(before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.loadFile()
	if err != nil {
		return err
	}
	for id, rec := range sf.Sessions {
		if rec.UpdatedAt.Before(before) {
			delete(sf.Sessions, id)
		}
	}
	return s.saveFile(sf)
}

func (s *SessionStore) loadFile() (sessionFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return sessionFile{Sessions: map[string]SessionRecord{}}, nil
		}
		return sessionFile{}, err
	}
	var sf sessionFile
	if len(data) > 0 {
		if err := json.Unmarshal(data, &sf); err != nil {
			return sessionFile{}, err
		}
	}
	if sf.Sessions == nil {
		sf.Sessions = map[string]SessionRecord{}
	}
	return sf, nil
}

func (s *SessionStore) saveFile(sf sessionFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	// Write to temp then rename for atomicity
	tmpPath := s.path + ".tmp"
	out, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

// ── Advancer ───────────────────────────────────────────────

// Advancer implements the session state machine.
// It wraps a SessionStore and provides Advance() to process events.
type Advancer struct {
	store *SessionStore
}

// NewAdvancer creates a new Advancer backed by the given store.
func NewAdvancer(store *SessionStore) *Advancer {
	return &Advancer{store: store}
}

// Advance runs the state machine: loads or creates a session record,
// applies the event, persists the updated record, and returns a decision.
//
// State transitions:
//
//	NEW ──(any event with actionable status)──> ACTIVE
//	ACTIVE ──(PostToolUseFailure)──> FAILED (notify)
//	ACTIVE ──(Stop, HasRunEvent=true)──> COMPLETED (notify once)
//	ACTIVE ──(PermissionRequest)──> ACTIVE (notify)
//	ACTIVE ──(InputRequired)──> ACTIVE (notify)
//	ACTIVE ──(SessionEnd, HasRunEvent=true)──> COMPLETED (notify)
//	NEW ──(Stop alone)──> NEW, no notification
//	NEW ──(SessionEnd alone)──> NEW, no notification
//	COMPLETED ──(PostToolUseFailure)──> FAILED (notify again)
func (a *Advancer) Advance(evt event.Event) (AdvanceDecision, error) {
	rec, err := a.store.Load(evt.SessionID)
	if err != nil {
		return AdvanceDecision{}, err
	}

	rec.SessionID = evt.SessionID
	rec.Agent = evt.Agent
	rec.Workspace = evt.Workspace
	rec.UpdatedAt = time.Now()
	if rec.StartedAt.IsZero() {
		rec.StartedAt = rec.UpdatedAt
	}
	rec.EventCount++

	decision := a.applyEvent(evt, &rec)

	if err := a.store.Save(rec); err != nil {
		return AdvanceDecision{}, err
	}

	return decision, nil
}

func (a *Advancer) applyEvent(evt event.Event, rec *SessionRecord) AdvanceDecision {
	switch evt.HookEvent {
	case "Stop":
		// Codex 只发 Stop 作为完成信号，无其他前置事件。
		// 对 Codex 来说，Stop 本身就表示"做了工作"。
		if evt.Agent == "codex" {
			rec.HasRunEvent = true
		}
		return a.handleStop(rec)
	case "SessionEnd":
		return a.handleSessionEnd(rec)
	case "PermissionRequest":
		rec.HasRunEvent = true
		rec.Status = SessActive
		return AdvanceDecision{
			Notify: true,
			Status: event.StatusPermissionReq,
			Reason: "permission requested by agent",
		}
	case "PostToolUseFailure":
		rec.HasRunEvent = true
		rec.Status = SessFailed
		rec.Notified = true
		rec.FailureMsg = evt.Body
		return AdvanceDecision{
			Notify: true,
			Status: event.StatusFailed,
			Reason: "tool execution failure",
		}
	case "Notification":
		// CodeBuddy uses matcher-based notifications
		// The adapter has already determined the status
		rec.HasRunEvent = true
		rec.Status = SessActive
		return AdvanceDecision{
			Notify: true,
			Status: evt.Status,
			Reason: "agent notification: " + string(evt.Status),
		}
	case "PreToolUse":
		rec.HasRunEvent = true
		rec.Status = SessActive
		return AdvanceDecision{
			Notify: true,
			Status: event.StatusInputRequired,
			Reason: "pre-tool-use notification",
		}
	default:
		// Unknown event — record the activity but don't notify
		if evt.Status == event.StatusPending {
			return AdvanceDecision{
				Notify: false,
				Status: event.StatusPending,
				Reason: "unrecognized event with no actionable status",
			}
		}
		rec.HasRunEvent = true
		rec.Status = SessActive
		return AdvanceDecision{
			Notify: true,
			Status: evt.Status,
			Reason: "event status: " + string(evt.Status),
		}
	}
}

func (a *Advancer) handleStop(rec *SessionRecord) AdvanceDecision {
	if !rec.HasRunEvent {
		// Idle stop — no work was done, no notification
		return AdvanceDecision{
			Notify: false,
			Status: event.StatusPending,
			Reason: "idle stop, no events seen this session",
		}
	}
	if rec.Notified {
		// Already notified terminal status — suppress duplicate
		return AdvanceDecision{
			Notify: false,
			Status: event.StatusCompleted,
			Reason: "session already notified",
		}
	}
	rec.Status = SessCompleted
	rec.Notified = true
	return AdvanceDecision{
		Notify: true,
		Status: event.StatusCompleted,
		Reason: "session completed with activity",
	}
}

func (a *Advancer) handleSessionEnd(rec *SessionRecord) AdvanceDecision {
	if !rec.HasRunEvent {
		return AdvanceDecision{
			Notify: false,
			Status: event.StatusPending,
			Reason: "session ended with no activity",
		}
	}
	if rec.Notified {
		return AdvanceDecision{
			Notify: false,
			Status: event.StatusCompleted,
			Reason: "session already notified",
		}
	}
	rec.Status = SessCompleted
	rec.Notified = true
	return AdvanceDecision{
		Notify: true,
		Status: event.StatusCompleted,
		Reason: "session ended with activity",
	}
}
