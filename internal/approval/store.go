package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hellolib/agent-notify/internal/event"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusDenied   Status = "denied"
	StatusExpired  Status = "expired"
)

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionDeny    Decision = "deny"
)

type Request struct {
	ApprovalID     string          `json:"approval_id"`
	Agent          string          `json:"agent"`
	SessionID      string          `json:"session_id"`
	Workspace      string          `json:"workspace"`
	Tool           string          `json:"tool"`
	CommandSummary string          `json:"command_summary,omitempty"`
	CommandDigest  string          `json:"command_digest"`
	RawDigest      string          `json:"raw_digest"`
	Token          string          `json:"token"`
	Status         Status          `json:"status"`
	Reason         string          `json:"reason,omitempty"`
	OperatorID     string          `json:"operator_open_id,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	DecidedAt      *time.Time      `json:"decided_at,omitempty"`
	RawPayload     json.RawMessage `json:"raw_payload,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

type fileData struct {
	Requests map[string]Request `json:"requests"`
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func NewRequest(evt event.Event, ttl time.Duration) Request {
	now := time.Now()
	tool, toolInputSummary, toolInputDigest := toolSummary(evt.RawPayload)
	rawDigest := digestBytes(evt.RawPayload)
	return Request{
		ApprovalID:     shortID(),
		Agent:          evt.Agent,
		SessionID:      evt.SessionID,
		Workspace:      evt.Workspace,
		Tool:           tool,
		CommandSummary: toolInputSummary,
		CommandDigest:  toolInputDigest,
		RawDigest:      rawDigest,
		Token:          uuid.NewString(),
		Status:         StatusPending,
		CreatedAt:      now,
		ExpiresAt:      now.Add(ttl),
		RawPayload:     evt.RawPayload,
	}
}

func (s *Store) Create(req Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return err
	}
	if data.Requests == nil {
		data.Requests = map[string]Request{}
	}
	data.Requests[req.ApprovalID] = req
	return s.save(data)
}

func (s *Store) Get(id string) (Request, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return Request{}, false, err
	}
	req, ok := data.Requests[id]
	return req, ok, nil
}

func (s *Store) List() ([]Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Request, 0, len(data.Requests))
	for _, req := range data.Requests {
		out = append(out, req)
	}
	return out, nil
}

func (s *Store) PendingCount() (int, error) {
	items, err := s.List()
	if err != nil {
		return 0, err
	}
	now := time.Now()
	count := 0
	for _, item := range items {
		if item.Status == StatusPending && now.Before(item.ExpiresAt) {
			count++
		}
	}
	return count, nil
}

func (s *Store) Decide(id, token, operator string, decision Decision, reason string) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return Request{}, err
	}
	req, ok := data.Requests[id]
	if !ok {
		return Request{}, fmt.Errorf("approval %s not found", id)
	}
	if req.Status != StatusPending {
		return Request{}, fmt.Errorf("approval %s already %s", id, req.Status)
	}
	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		data.Requests[id] = req
		_ = s.save(data)
		return Request{}, fmt.Errorf("approval %s expired", id)
	}
	if token != "" && token != req.Token {
		return Request{}, errors.New("approval token mismatch")
	}

	now := time.Now()
	req.OperatorID = operator
	req.Reason = reason
	req.DecidedAt = &now
	switch decision {
	case DecisionApprove:
		req.Status = StatusApproved
	case DecisionDeny:
		req.Status = StatusDenied
	default:
		return Request{}, fmt.Errorf("unknown decision: %s", decision)
	}
	data.Requests[id] = req
	return req, s.save(data)
}

func (s *Store) ExpirePending(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return err
	}
	now := time.Now()
	for id, req := range data.Requests {
		if req.Status == StatusPending {
			req.Status = StatusDenied
			req.Reason = reason
			req.DecidedAt = &now
			data.Requests[id] = req
		}
	}
	return s.save(data)
}

func (s *Store) Wait(ctx context.Context, id string, ttl time.Duration) (Request, error) {
	timer := time.NewTimer(ttl)
	defer timer.Stop()
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		req, ok, err := s.Get(id)
		if err != nil {
			return Request{}, err
		}
		if !ok {
			return Request{}, fmt.Errorf("approval %s not found", id)
		}
		switch req.Status {
		case StatusApproved, StatusDenied:
			return req, nil
		case StatusExpired:
			return req, fmt.Errorf("approval %s expired", id)
		}
		if time.Now().After(req.ExpiresAt) {
			_, _ = s.Decide(id, "", "", DecisionDeny, "approval timed out")
			req.Status = StatusExpired
			return req, fmt.Errorf("approval %s timed out", id)
		}

		select {
		case <-ctx.Done():
			return Request{}, ctx.Err()
		case <-timer.C:
			_, _ = s.Decide(id, "", "", DecisionDeny, "approval timed out")
			return Request{}, fmt.Errorf("approval %s timed out", id)
		case <-ticker.C:
		}
	}
}

func (s *Store) load() (fileData, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileData{Requests: map[string]Request{}}, nil
		}
		return fileData{}, err
	}
	data := fileData{Requests: map[string]Request{}}
	if len(raw) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fileData{}, err
	}
	if data.Requests == nil {
		data.Requests = map[string]Request{}
	}
	return data, nil
}

func (s *Store) save(data fileData) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func shortID() string {
	return strings.ReplaceAll(uuid.NewString()[:8], "-", "")
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

func toolSummary(raw []byte) (string, string, string) {
	var payload struct {
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
	}
	_ = json.Unmarshal(raw, &payload)
	tool := payload.ToolName
	if tool == "" {
		tool = "unknown"
	}
	inputRaw, _ := json.Marshal(payload.ToolInput)
	if len(inputRaw) == 0 || string(inputRaw) == "null" {
		inputRaw = raw
	}
	return tool, summarizeToolInput(payload.ToolInput, raw), digestBytes(inputRaw)
}

func summarizeToolInput(input map[string]any, fallback []byte) string {
	if len(input) == 0 {
		return truncateApprovalSummary(strings.TrimSpace(string(fallback)))
	}
	for _, key := range []string{"command", "cmd", "description", "query", "pattern", "path", "file_path", "url", "prompt"} {
		value, ok := input[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return truncateApprovalSummary(text)
		}
	}
	raw, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "无法解析工具参数"
	}
	return truncateApprovalSummary(string(raw))
}

func truncateApprovalSummary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "未提供工具参数"
	}
	const limit = 1200
	if len(text) <= limit {
		return text
	}
	return text[:limit-20] + "\n...(已截断)"
}
