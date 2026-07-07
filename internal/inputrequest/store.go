package inputrequest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hellolib/agent-notify/internal/event"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusAnswered Status = "answered"
	StatusExpired  Status = "expired"
)

type Request struct {
	InputID      string          `json:"input_id"`
	Agent        string          `json:"agent"`
	Profile      string          `json:"profile"`
	SessionID    string          `json:"session_id"`
	Workspace    string          `json:"workspace"`
	Prompt       string          `json:"prompt"`
	Options      []string        `json:"options,omitempty"`
	MultiSelect  bool            `json:"multi_select,omitempty"`
	AllowOther   bool            `json:"allow_other,omitempty"`
	RawDigest    string          `json:"raw_digest"`
	Token        string          `json:"token"`
	Status       Status          `json:"status"`
	Answer       string          `json:"answer,omitempty"`
	AnswerValues []string        `json:"answer_values,omitempty"`
	OtherAnswer  string          `json:"other_answer,omitempty"`
	OperatorID   string          `json:"operator_open_id,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	AnsweredAt   *time.Time      `json:"answered_at,omitempty"`
	RawPayload   json.RawMessage `json:"raw_payload,omitempty"`
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

func NewRequest(evt event.Event, profile string, ttl time.Duration) Request {
	now := time.Now()
	prompt, options, multiSelect, allowOther := InputDetails(evt)
	return Request{
		InputID:     shortID(),
		Agent:       evt.Agent,
		Profile:     profile,
		SessionID:   evt.SessionID,
		Workspace:   evt.Workspace,
		Prompt:      prompt,
		Options:     options,
		MultiSelect: multiSelect,
		AllowOther:  allowOther,
		RawDigest:   digestBytes(evt.RawPayload),
		Token:       uuid.NewString(),
		Status:      StatusPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		RawPayload:  evt.RawPayload,
	}
}

func InputDetails(evt event.Event) (string, []string, bool, bool) {
	if prompt, options, multiSelect, allowOther, ok := questionDetails(evt.RawPayload); ok {
		if prompt == "" {
			prompt = "Claude Code 正在等待你的输入"
		}
		return prompt, options, multiSelect, allowOther
	}
	message := rawMessage(evt.RawPayload)
	if message == "" {
		message = strings.TrimPrefix(evt.Body, "提示: ")
	}
	message = strings.TrimSpace(message)
	prompt := cleanPrompt(message)
	options, allowOther := extractOptions(message)
	if prompt == "" {
		prompt = "Claude Code 正在等待你的输入"
	}
	return prompt, options, false, allowOther
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
	data.Requests[req.InputID] = req
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

func (s *Store) Answer(id, token, operator, answer string) (Request, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return Request{}, errors.New("answer is empty")
	}
	if req, ok, err := s.Get(id); err != nil {
		return Request{}, err
	} else if ok && req.AllowOther && len(req.Options) > 0 && !contains(req.Options, answer) {
		return s.AnswerValues(id, token, operator, nil, answer)
	}
	return s.AnswerValues(id, token, operator, []string{answer}, "")
}

func (s *Store) AnswerValues(id, token, operator string, values []string, other string) (Request, error) {
	values = cleanValues(values)
	other = strings.TrimSpace(other)
	if len(values) == 0 && other == "" {
		return Request{}, errors.New("answer is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return Request{}, err
	}
	req, ok := data.Requests[id]
	if !ok {
		return Request{}, fmt.Errorf("input request %s not found", id)
	}
	if req.Status != StatusPending {
		return Request{}, fmt.Errorf("input request %s already %s", id, req.Status)
	}
	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		data.Requests[id] = req
		_ = s.save(data)
		return Request{}, fmt.Errorf("input request %s expired", id)
	}
	if token != "" && token != req.Token {
		return Request{}, errors.New("input request token mismatch")
	}
	if !req.MultiSelect && len(values) > 1 {
		return Request{}, errors.New("single-select input received multiple answers")
	}
	if other != "" && !req.AllowOther {
		return Request{}, errors.New("custom answer is not allowed")
	}
	if len(req.Options) > 0 {
		for _, value := range values {
			if !contains(req.Options, value) {
				return Request{}, fmt.Errorf("answer %q is not an allowed option", value)
			}
		}
	}

	now := time.Now()
	req.AnswerValues = values
	req.OtherAnswer = other
	req.Answer = formatAnswer(values, other)
	req.OperatorID = operator
	req.AnsweredAt = &now
	req.Status = StatusAnswered
	data.Requests[id] = req
	return req, s.save(data)
}

func (s *Store) PendingForProfile(profile string) (Request, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return Request{}, false, err
	}
	now := time.Now()
	var best Request
	for id, req := range data.Requests {
		if req.Status != StatusPending || req.Profile != profile {
			continue
		}
		if now.After(req.ExpiresAt) {
			req.Status = StatusExpired
			data.Requests[id] = req
			continue
		}
		if best.InputID == "" || req.CreatedAt.Before(best.CreatedAt) {
			best = req
		}
	}
	if err := s.save(data); err != nil {
		return Request{}, false, err
	}
	return best, best.InputID != "", nil
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
			return Request{}, fmt.Errorf("input request %s not found", id)
		}
		switch req.Status {
		case StatusAnswered:
			return req, nil
		case StatusExpired:
			return req, fmt.Errorf("input request %s expired", id)
		}
		if time.Now().After(req.ExpiresAt) {
			_, _ = s.expire(id)
			return req, fmt.Errorf("input request %s timed out", id)
		}

		select {
		case <-ctx.Done():
			return Request{}, ctx.Err()
		case <-timer.C:
			_, _ = s.expire(id)
			return Request{}, fmt.Errorf("input request %s timed out", id)
		case <-ticker.C:
		}
	}
}

func (s *Store) expire(id string) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return Request{}, err
	}
	req, ok := data.Requests[id]
	if !ok {
		return Request{}, fmt.Errorf("input request %s not found", id)
	}
	req.Status = StatusExpired
	data.Requests[id] = req
	return req, s.save(data)
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

func rawMessage(raw []byte) string {
	var payload struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &payload)
	return payload.Message
}

func questionDetails(raw []byte) (string, []string, bool, bool, bool) {
	var payload struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			Questions []struct {
				Question    string `json:"question"`
				Header      string `json:"header"`
				MultiSelect bool   `json:"multiSelect"`
				Options     []struct {
					Label       string `json:"label"`
					Description string `json:"description"`
				} `json:"options"`
			} `json:"questions"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", nil, false, false, false
	}
	if payload.ToolName != "AskUserQuestion" || len(payload.ToolInput.Questions) == 0 {
		return "", nil, false, false, false
	}
	q := payload.ToolInput.Questions[0]
	prompt := strings.TrimSpace(q.Question)
	if prompt == "" {
		prompt = strings.TrimSpace(q.Header)
	}
	var options []string
	allowOther := false
	for _, option := range q.Options {
		label := strings.TrimSpace(option.Label)
		if label == "" {
			continue
		}
		if isOtherLabel(label) {
			allowOther = true
		}
		options = append(options, label)
	}
	return prompt, dedupe(options), q.MultiSelect, allowOther, true
}

func cleanPrompt(message string) string {
	message = strings.TrimSpace(message)
	for _, prefix := range []string{
		"Claude is waiting for your input: ",
		"Claude is waiting for your input",
		"waiting for your input: ",
		"waiting for input: ",
		"needs input: ",
	} {
		if strings.HasPrefix(strings.ToLower(message), strings.ToLower(prefix)) {
			message = strings.TrimSpace(message[len(prefix):])
			break
		}
	}
	lines := strings.Split(message, "\n")
	var kept []string
	optionLine := regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+(.+)$`)
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			kept = append(kept, line)
			continue
		}
		if strings.EqualFold(text, "Other") || optionLine.MatchString(text) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func extractOptions(message string) ([]string, bool) {
	lineOption := regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+(.+)$`)
	var out []string
	allowOther := false
	for _, line := range strings.Split(message, "\n") {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if isOtherLabel(text) {
			allowOther = true
			out = append(out, "Other")
			continue
		}
		if m := lineOption.FindStringSubmatch(text); len(m) == 2 {
			option := strings.TrimSpace(m[1])
			if isOtherLabel(option) {
				allowOther = true
			}
			out = append(out, option)
		}
	}
	return dedupe(out), allowOther
}

func isOtherLabel(label string) bool {
	label = strings.TrimSpace(label)
	return strings.EqualFold(label, "Other") || label == "其他" || label == "其它"
}

func cleanValues(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return dedupe(out)
}

func formatAnswer(values []string, other string) string {
	parts := append([]string(nil), values...)
	if other != "" {
		parts = append(parts, other)
	}
	return strings.Join(parts, ", ")
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func shortID() string {
	return strings.ReplaceAll(uuid.NewString()[:8], "-", "")
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}
