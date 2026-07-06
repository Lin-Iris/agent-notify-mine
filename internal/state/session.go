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
	HasRunEvent bool          `json:"has_run_event"` // legacy activity marker kept for stored state compatibility
	EventCount  int           `json:"event_count"`
	Notified    bool          `json:"notified"` // terminal notification already sent
	FailureMsg  string        `json:"failure_msg,omitempty"`
}

// sessionFile is the on-disk format.
type sessionFile struct {
	Sessions map[string]SessionRecord `json:"sessions"`
}

// AdvanceDecision is returned by Advancer.Advance().
type AdvanceDecision struct {
	Notify bool         // true = caller should send a notification
	Status event.Status // deduced status for the notification
	Reason string       // human-readable explanation (for logging)
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
//	Completed status / Stop / SessionEnd ──> COMPLETED (notify)
//	PermissionRequest ──> ACTIVE (notify)
//	InputRequired / PreToolUse ──> ACTIVE (notify)
//	Failed status / PostToolUseFailure ──> FAILED (notify)
//	Pending unknown event ──> keep current status (no notification)
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
	switch evt.Status {
	case event.StatusCompleted:
		return a.completeSession(rec, "event completed")
	case event.StatusFailed:
		return a.failSession(rec, evt.Body, "event failed")
	case event.StatusPermissionReq:
		rec.HasRunEvent = true
		rec.Status = SessActive
		return AdvanceDecision{
			Notify: true,
			Status: event.StatusPermissionReq,
			Reason: "permission requested by agent",
		}
	case event.StatusInputRequired:
		rec.HasRunEvent = true
		rec.Status = SessActive
		return AdvanceDecision{
			Notify: true,
			Status: event.StatusInputRequired,
			Reason: "agent input required",
		}
	}

	switch evt.HookEvent {
	case "Stop":
		return a.completeSession(rec, "stop hook completed")
	case "SessionEnd":
		return a.completeSession(rec, "session ended")
	case "PermissionRequest":
		return a.activateSession(event.StatusPermissionReq, "permission requested by agent", rec)
	case "PostToolUseFailure":
		return a.failSession(rec, evt.Body, "tool execution failure")
	case "Notification":
		return a.activateSession(evt.Status, "agent notification: "+string(evt.Status), rec)
	case "PreToolUse":
		return a.activateSession(event.StatusInputRequired, "pre-tool-use notification", rec)
	default:
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

func (a *Advancer) activateSession(status event.Status, reason string, rec *SessionRecord) AdvanceDecision {
	rec.HasRunEvent = true
	rec.Status = SessActive
	return AdvanceDecision{
		Notify: true,
		Status: status,
		Reason: reason,
	}
}

func (a *Advancer) completeSession(rec *SessionRecord, reason string) AdvanceDecision {
	rec.HasRunEvent = true
	rec.Status = SessCompleted
	rec.Notified = true
	return AdvanceDecision{
		Notify: true,
		Status: event.StatusCompleted,
		Reason: reason,
	}
}

func (a *Advancer) failSession(rec *SessionRecord, failureMsg string, reason string) AdvanceDecision {
	rec.HasRunEvent = true
	rec.Status = SessFailed
	rec.Notified = true
	rec.FailureMsg = failureMsg
	return AdvanceDecision{
		Notify: true,
		Status: event.StatusFailed,
		Reason: reason,
	}
}
