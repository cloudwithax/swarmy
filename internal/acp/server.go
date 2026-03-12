package acp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/swarmy/internal/agent"
	"github.com/charmbracelet/swarmy/internal/message"
	"github.com/charmbracelet/swarmy/internal/permission"
	"github.com/charmbracelet/swarmy/internal/pubsub"
	"github.com/charmbracelet/swarmy/internal/session"
	"github.com/google/uuid"
)

const defaultSessionTitle = "ACP Session"

type RunMode string

const (
	RunModeSync   RunMode = "sync"
	RunModeAsync  RunMode = "async"
	RunModeStream RunMode = "stream"
)

type RunStatus string

const (
	RunStatusCreated    RunStatus = "created"
	RunStatusInProgress RunStatus = "in-progress"
	RunStatusCancelling RunStatus = "cancelling"
	RunStatusCancelled  RunStatus = "cancelled"
	RunStatusCompleted  RunStatus = "completed"
	RunStatusFailed     RunStatus = "failed"
)

type ErrorCode string

const (
	ErrorCodeServerError  ErrorCode = "server_error"
	ErrorCodeInvalidInput ErrorCode = "invalid_input"
	ErrorCodeNotFound     ErrorCode = "not_found"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Data    any       `json:"data,omitempty"`
}

type MessagePart struct {
	Name            string `json:"name,omitempty"`
	ContentType     string `json:"content_type,omitempty"`
	Content         string `json:"content,omitempty"`
	ContentEncoding string `json:"content_encoding,omitempty"`
	ContentURL      string `json:"content_url,omitempty"`
	Metadata        any    `json:"metadata,omitempty"`
}

type ACPMessage struct {
	Role        string        `json:"role"`
	Parts       []MessagePart `json:"parts"`
	CreatedAt   *time.Time    `json:"created_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

type SessionDescriptor struct {
	ID      string   `json:"id"`
	History []string `json:"history"`
	State   *string  `json:"state"`
}

type AgentManifest struct {
	Name               string         `json:"name"`
	Description        string         `json:"description,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	InputContentTypes  []string       `json:"input_content_types"`
	OutputContentTypes []string       `json:"output_content_types"`
}

type Run struct {
	RunID        string       `json:"run_id"`
	AgentName    string       `json:"agent_name"`
	SessionID    *string      `json:"session_id,omitempty"`
	Status       RunStatus    `json:"status"`
	AwaitRequest any          `json:"await_request,omitempty"`
	Output       []ACPMessage `json:"output,omitempty"`
	Error        *Error       `json:"error,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	FinishedAt   *time.Time   `json:"finished_at,omitempty"`
}

type RunCreateRequest struct {
	AgentName string             `json:"agent_name"`
	SessionID *string            `json:"session_id,omitempty"`
	Session   *SessionDescriptor `json:"session,omitempty"`
	Input     []ACPMessage       `json:"input"`
	Mode      RunMode            `json:"mode"`
}

type RunResumeRequest struct {
	AwaitResume any     `json:"await_resume"`
	Mode        RunMode `json:"mode"`
}

type Options struct {
	AgentName           string
	Description         string
	AutoApproveSessions bool
}

type Runner interface {
	Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
	Cancel(sessionID string)
}

type SessionService interface {
	Create(ctx context.Context, title string) (session.Session, error)
	CreateWithID(ctx context.Context, id, title string) (session.Session, error)
	Get(ctx context.Context, id string) (session.Session, error)
}

type MessageService interface {
	Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message]
	Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error)
	List(ctx context.Context, sessionID string) ([]message.Message, error)
}

type Server struct {
	runner      Runner
	sessions    SessionService
	messages    MessageService
	permissions permission.Service
	manifest    AgentManifest
	autoApprove bool

	runsMu sync.RWMutex
	runs   map[string]*runRecord
}

type runRecord struct {
	mu         sync.RWMutex
	run        Run
	events     []json.RawMessage
	notify     chan struct{}
	done       chan struct{}
	doneClosed bool
	cancel     context.CancelFunc
	tracked    map[string]*trackedMessage
	completed  []ACPMessage
}

type trackedMessage struct {
	LastText string
	HasParts bool
}

func NewServer(
	runner Runner,
	sessions SessionService,
	messages MessageService,
	permissions permission.Service,
	opts Options,
) *Server {
	agentName := opts.AgentName
	if agentName == "" {
		agentName = "swarmy"
	}
	description := opts.Description
	if description == "" {
		description = "Swarmy headless coding agent exposed over ACP."
	}

	metadata := map[string]any{
		"annotations": map[string]any{
			"beeai_ui": map[string]any{
				"ui_type":      "hands-off",
				"display_name": "Swarmy",
			},
		},
		"framework":            "swarmy",
		"programming_language": "go",
		"tags":                 []string{"coding", "headless", "orchestration"},
	}

	return &Server{
		runner:      runner,
		sessions:    sessions,
		messages:    messages,
		permissions: permissions,
		autoApprove: opts.AutoApproveSessions,
		manifest: AgentManifest{
			Name:               agentName,
			Description:        description,
			Metadata:           metadata,
			InputContentTypes:  []string{"*/*"},
			OutputContentTypes: []string{"text/plain"},
		},
		runs: make(map[string]*runRecord),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", s.handlePing)
	mux.HandleFunc("GET /agents", s.handleListAgents)
	mux.HandleFunc("GET /agents/{name}", s.handleReadAgent)
	mux.HandleFunc("POST /runs", s.handleCreateRun)
	mux.HandleFunc("GET /runs/{run_id}", s.handleReadRun)
	mux.HandleFunc("GET /runs/{run_id}/events", s.handleListRunEvents)
	mux.HandleFunc("POST /runs/{run_id}", s.handleResumeRun)
	mux.HandleFunc("POST /runs/{run_id}/cancel", s.handleCancelRun)
	mux.HandleFunc("GET /sessions/{session_id}", s.handleReadSession)
	return mux
}

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *Server) handleListAgents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"agents": []AgentManifest{s.manifest}})
}

func (s *Server) handleReadAgent(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("name") != s.manifest.Name {
		writeACPError(w, http.StatusNotFound, ErrorCodeNotFound, fmt.Sprintf("Agent %s not found", r.PathValue("name")))
		return
	}
	writeJSON(w, http.StatusOK, s.manifest)
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req RunCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeACPError(w, http.StatusUnprocessableEntity, ErrorCodeInvalidInput, "Invalid ACP run request")
		return
	}

	if req.AgentName != s.manifest.Name {
		writeACPError(w, http.StatusNotFound, ErrorCodeNotFound, fmt.Sprintf("Agent %s not found", req.AgentName))
		return
	}
	if req.Mode != RunModeSync && req.Mode != RunModeAsync && req.Mode != RunModeStream {
		writeACPError(w, http.StatusUnprocessableEntity, ErrorCodeInvalidInput, "Run mode must be one of sync, async, or stream")
		return
	}
	if req.SessionID != nil && req.Session != nil && req.Session.ID != "" && req.Session.ID != *req.SessionID {
		writeACPError(w, http.StatusBadRequest, ErrorCodeInvalidInput, "Session ID mismatch")
		return
	}

	resolved, err := s.resolveRunInput(req.Input)
	if err != nil {
		writeACPError(w, http.StatusUnprocessableEntity, ErrorCodeInvalidInput, err.Error())
		return
	}

	sess, isNewSession, err := s.resolveSession(r.Context(), req)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	if isNewSession {
		if err := s.importHistory(r.Context(), sess.ID, resolved.History); err != nil {
			s.handleServiceError(w, err)
			return
		}
	}

	if s.autoApprove && s.permissions != nil {
		s.permissions.AutoApproveSession(sess.ID)
	}

	rec := newRunRecord(s.manifest.Name, sess.ID)
	s.storeRun(rec)
	rec.addEvent(map[string]any{"type": "run.created", "run": rec.snapshot()})

	runBaseCtx := r.Context()
	if req.Mode == RunModeAsync {
		runBaseCtx = context.WithoutCancel(runBaseCtx)
	}

	runCtx, cancel := context.WithCancel(runBaseCtx)
	rec.setCancel(cancel)
	go s.executeRun(runCtx, rec, sess.ID, resolved.Prompt, resolved.Attachments)

	if req.Mode == RunModeStream {
		s.serveRunStream(w, r, rec)
		return
	}

	if req.Mode == RunModeAsync {
		writeJSONWithStatus(w, http.StatusAccepted, rec.snapshot())
		return
	}

	<-rec.done
	writeJSON(w, http.StatusOK, rec.snapshot())
}

func (s *Server) handleReadRun(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.getRun(r.PathValue("run_id"))
	if !ok {
		writeACPError(w, http.StatusNotFound, ErrorCodeNotFound, fmt.Sprintf("Run %s not found", r.PathValue("run_id")))
		return
	}
	writeJSON(w, http.StatusOK, rec.snapshot())
}

func (s *Server) handleListRunEvents(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.getRun(r.PathValue("run_id"))
	if !ok {
		writeACPError(w, http.StatusNotFound, ErrorCodeNotFound, fmt.Sprintf("Run %s not found", r.PathValue("run_id")))
		return
	}
	events := rec.snapshotEventsDecoded()
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleResumeRun(w http.ResponseWriter, r *http.Request) {
	_ = r
	writeACPError(w, http.StatusForbidden, ErrorCodeServerError, "Swarmy ACP runs do not support resume")
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.getRun(r.PathValue("run_id"))
	if !ok {
		writeACPError(w, http.StatusNotFound, ErrorCodeNotFound, fmt.Sprintf("Run %s not found", r.PathValue("run_id")))
		return
	}

	status := rec.status()
	if status != RunStatusCompleted && status != RunStatusFailed && status != RunStatusCancelled {
		rec.setStatus(RunStatusCancelling)
		if rec.cancel != nil {
			rec.cancel()
		}
	}
	writeJSONWithStatus(w, http.StatusAccepted, rec.snapshot())
}

func (s *Server) handleReadSession(w http.ResponseWriter, r *http.Request) {
	if _, err := s.sessions.Get(r.Context(), r.PathValue("session_id")); err != nil {
		s.handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, SessionDescriptor{ID: r.PathValue("session_id"), History: []string{}})
}

func (s *Server) executeRun(ctx context.Context, rec *runRecord, sessionID, prompt string, attachments []message.Attachment) {
	defer rec.closeDone()

	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()
	messageEvents := s.messages.Subscribe(watchCtx)

	rec.setStatus(RunStatusInProgress)
	rec.addEvent(map[string]any{"type": "run.in-progress", "run": rec.snapshot()})

	done := make(chan error, 1)
	go func() {
		_, err := s.runner.Run(ctx, sessionID, prompt, attachments...)
		done <- err
	}()

	for {
		select {
		case err := <-done:
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, agent.ErrRequestCancelled) {
					s.refreshRunOutput(rec, sessionID)
					rec.finishCancelled()
					rec.addEvent(map[string]any{"type": "run.cancelled", "run": rec.snapshot()})
					return
				}
				s.refreshRunOutput(rec, sessionID)
				rec.finishFailed(toACPError(err))
				rec.addEvent(map[string]any{"type": "run.failed", "run": rec.snapshot()})
				return
			}
			s.refreshRunOutput(rec, sessionID)
			rec.finishCompleted()
			rec.addEvent(map[string]any{"type": "run.completed", "run": rec.snapshot()})
			return
		case event := <-messageEvents:
			msg := event.Payload
			if msg.SessionID != sessionID || msg.Role != message.Assistant {
				continue
			}
			s.recordMessageEvent(rec, event.Type, msg)
		case <-ctx.Done():
			s.refreshRunOutput(rec, sessionID)
			rec.finishCancelled()
			rec.addEvent(map[string]any{"type": "run.cancelled", "run": rec.snapshot()})
			return
		}
	}
}

func (s *Server) refreshRunOutput(rec *runRecord, sessionID string) {
	msgs, err := s.messages.List(context.Background(), sessionID)
	if err != nil {
		slog.Error("Failed to list ACP session messages", "session_id", sessionID, "error", err)
		return
	}

	startedAt := rec.createdAtUnix()
	output := make([]ACPMessage, 0)
	for _, msg := range msgs {
		if msg.Role != message.Assistant || msg.CreatedAt < startedAt {
			continue
		}
		acpMsg := toACPMessage(msg, s.manifest.Name)
		if len(acpMsg.Parts) == 0 {
			continue
		}
		output = append(output, acpMsg)
	}
	rec.replaceOutput(output)
}

func (s *Server) recordMessageEvent(rec *runRecord, eventType pubsub.EventType, msg message.Message) {
	acpMsg := toACPMessage(msg, s.manifest.Name)
	plainText := collectPlainText(acpMsg)

	rec.mu.Lock()
	tracked, ok := rec.tracked[msg.ID]
	if !ok {
		tracked = &trackedMessage{}
		rec.tracked[msg.ID] = tracked
	}
	prevText := tracked.LastText
	hasParts := len(acpMsg.Parts) > 0
	finish := msg.IsFinished()
	rec.mu.Unlock()

	if eventType == pubsub.CreatedEvent {
		rec.addEvent(map[string]any{"type": "message.created", "message": acpMsg})
	}

	if len(plainText) > len(prevText) {
		delta := plainText[len(prevText):]
		rec.addEvent(map[string]any{
			"type": "message.part",
			"part": MessagePart{
				ContentType:     "text/plain",
				Content:         delta,
				ContentEncoding: "plain",
			},
		})
	}

	if finish {
		rec.addEvent(map[string]any{"type": "message.completed", "message": acpMsg})
		if hasParts {
			rec.appendOutput(acpMsg)
		}
	}

	rec.mu.Lock()
	tracked.LastText = plainText
	tracked.HasParts = hasParts
	rec.mu.Unlock()
}

func (s *Server) serveRunStream(w http.ResponseWriter, r *http.Request, rec *runRecord) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeACPError(w, http.StatusInternalServerError, ErrorCodeServerError, "Streaming is not supported by this server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	idx := 0
	for {
		events, done := rec.eventsSince(idx)
		for _, event := range events {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		}
		idx += len(events)
		if done {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-rec.notify:
		}
	}
}

type resolvedRunInput struct {
	History     []ACPMessage
	Prompt      string
	Attachments []message.Attachment
}

func (s *Server) resolveRunInput(input []ACPMessage) (resolvedRunInput, error) {
	if len(input) == 0 {
		return resolvedRunInput{}, errors.New("ACP input must include at least one message")
	}
	last := input[len(input)-1]
	if last.Role != "user" {
		return resolvedRunInput{}, errors.New("The final ACP input message must use the user role")
	}

	prompt, attachments, err := splitPromptAndAttachments(last)
	if err != nil {
		return resolvedRunInput{}, err
	}
	if strings.TrimSpace(prompt) == "" && len(attachments) == 0 {
		return resolvedRunInput{}, errors.New("The final ACP user message must include content")
	}

	return resolvedRunInput{
		History:     input[:len(input)-1],
		Prompt:      prompt,
		Attachments: attachments,
	}, nil
}

func (s *Server) resolveSession(ctx context.Context, req RunCreateRequest) (session.Session, bool, error) {
	var desiredID string
	if req.SessionID != nil {
		desiredID = *req.SessionID
	}
	if req.Session != nil && req.Session.ID != "" {
		desiredID = req.Session.ID
	}

	if desiredID == "" {
		sess, err := s.sessions.Create(ctx, defaultSessionTitle)
		return sess, true, err
	}

	sess, err := s.sessions.Get(ctx, desiredID)
	if err == nil {
		return sess, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return session.Session{}, false, err
	}

	sess, err = s.sessions.CreateWithID(ctx, desiredID, defaultSessionTitle)
	return sess, true, err
}

func (s *Server) importHistory(ctx context.Context, sessionID string, history []ACPMessage) error {
	if len(history) == 0 {
		return nil
	}

	existing, err := s.messages.List(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	for _, msg := range history {
		params, err := toInternalHistoryMessage(msg)
		if err != nil {
			return err
		}
		if len(params.Parts) == 0 {
			continue
		}
		if _, err := s.messages.Create(ctx, sessionID, params); err != nil {
			return err
		}
	}
	return nil
}

func splitPromptAndAttachments(msg ACPMessage) (string, []message.Attachment, error) {
	var prompt strings.Builder
	attachments := make([]message.Attachment, 0)

	for i, part := range msg.Parts {
		data, err := partBytes(part)
		if err != nil {
			return "", nil, err
		}
		contentType := part.ContentType
		if contentType == "" {
			contentType = "text/plain"
		}

		if part.Name == "" && strings.HasPrefix(contentType, "text/") {
			prompt.Write(data)
			continue
		}

		name := part.Name
		if name == "" {
			name = fmt.Sprintf("part-%d", i+1)
		}
		attachments = append(attachments, message.Attachment{
			FileName: name,
			FilePath: name,
			MimeType: contentType,
			Content:  data,
		})
	}

	return prompt.String(), attachments, nil
}

func toInternalHistoryMessage(msg ACPMessage) (message.CreateMessageParams, error) {
	parts := make([]message.ContentPart, 0, len(msg.Parts)+1)
	for i, part := range msg.Parts {
		data, err := partBytes(part)
		if err != nil {
			return message.CreateMessageParams{}, err
		}
		contentType := part.ContentType
		if contentType == "" {
			contentType = "text/plain"
		}
		if part.Name == "" && strings.HasPrefix(contentType, "text/") {
			parts = append(parts, message.TextContent{Text: string(data)})
			continue
		}
		name := part.Name
		if name == "" {
			name = fmt.Sprintf("part-%d", i+1)
		}
		parts = append(parts, message.BinaryContent{Path: name, MIMEType: contentType, Data: data})
	}

	role := message.User
	if msg.Role != "user" {
		role = message.Assistant
		parts = append(parts, message.Finish{Reason: message.FinishReasonEndTurn, Time: time.Now().Unix()})
	}

	return message.CreateMessageParams{Role: role, Parts: parts}, nil
}

func partBytes(part MessagePart) ([]byte, error) {
	if part.ContentURL != "" {
		return nil, errors.New("content_url is not supported in this ACP integration")
	}
	if part.ContentEncoding == "" || part.ContentEncoding == "plain" {
		return []byte(part.Content), nil
	}
	if part.ContentEncoding == "base64" {
		data, err := base64.StdEncoding.DecodeString(part.Content)
		if err != nil {
			return nil, errors.New("invalid base64 ACP message part")
		}
		return data, nil
	}
	return nil, fmt.Errorf("unsupported content encoding %q", part.ContentEncoding)
}

func toACPMessage(msg message.Message, agentName string) ACPMessage {
	created := unixTimePtr(msg.CreatedAt)
	completed := completedTime(msg)
	result := ACPMessage{
		Role:        "agent/" + agentName,
		Parts:       make([]MessagePart, 0),
		CreatedAt:   created,
		CompletedAt: completed,
	}
	if msg.Role == message.User {
		result.Role = "user"
	}

	if text := strings.TrimSpace(msg.Content().Text); text != "" {
		result.Parts = append(result.Parts, MessagePart{
			ContentType:     "text/plain",
			Content:         text,
			ContentEncoding: "plain",
		})
	}
	for _, img := range msg.ImageURLContent() {
		result.Parts = append(result.Parts, MessagePart{
			ContentType: "image/*",
			ContentURL:  img.URL,
		})
	}
	for _, content := range msg.BinaryContent() {
		result.Parts = append(result.Parts, MessagePart{
			Name:            content.Path,
			ContentType:     content.MIMEType,
			Content:         base64.StdEncoding.EncodeToString(content.Data),
			ContentEncoding: "base64",
		})
	}
	return result
}

func collectPlainText(msg ACPMessage) string {
	var sb strings.Builder
	for _, part := range msg.Parts {
		if part.ContentType == "" || strings.HasPrefix(part.ContentType, "text/") {
			sb.WriteString(part.Content)
		}
	}
	return sb.String()
}

func completedTime(msg message.Message) *time.Time {
	if finish := msg.FinishPart(); finish != nil && finish.Time > 0 {
		return unixTimePtr(finish.Time)
	}
	if msg.UpdatedAt > 0 {
		return unixTimePtr(msg.UpdatedAt)
	}
	return nil
}

func unixTimePtr(v int64) *time.Time {
	if v <= 0 {
		return nil
	}
	t := time.Unix(v, 0).UTC()
	return &t
}

func toACPError(err error) *Error {
	if errors.Is(err, permission.ErrorPermissionDenied) {
		return &Error{Code: ErrorCodeServerError, Message: "Permission denied"}
	}
	return &Error{Code: ErrorCodeServerError, Message: err.Error()}
}

func newRunRecord(agentName, sessionID string) *runRecord {
	createdAt := time.Now().UTC()
	return &runRecord{
		run: Run{
			RunID:     uuid.NewString(),
			AgentName: agentName,
			SessionID: &sessionID,
			Status:    RunStatusCreated,
			CreatedAt: createdAt,
		},
		notify:    make(chan struct{}, 1),
		done:      make(chan struct{}),
		tracked:   make(map[string]*trackedMessage),
		completed: make([]ACPMessage, 0),
	}
}

func (r *runRecord) setCancel(cancel context.CancelFunc) {
	r.mu.Lock()
	r.cancel = cancel
	r.mu.Unlock()
}

func (r *runRecord) addEvent(event any) {
	raw, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal ACP event", "error", err)
		return
	}
	r.mu.Lock()
	r.events = append(r.events, raw)
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (r *runRecord) snapshot() Run {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clone := r.run
	clone.Output = append([]ACPMessage(nil), r.completed...)
	return clone
}

func (r *runRecord) snapshotEventsDecoded() []any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	decoded := make([]any, 0, len(r.events))
	for _, raw := range r.events {
		var event any
		if err := json.Unmarshal(raw, &event); err == nil {
			decoded = append(decoded, event)
		}
	}
	return decoded
}

func (r *runRecord) eventsSince(idx int) ([]json.RawMessage, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if idx >= len(r.events) {
		return nil, r.doneClosed
	}
	out := append([]json.RawMessage(nil), r.events[idx:]...)
	return out, r.doneClosed
}

func (r *runRecord) setStatus(status RunStatus) {
	r.mu.Lock()
	r.run.Status = status
	r.mu.Unlock()
}

func (r *runRecord) status() RunStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.run.Status
}

func (r *runRecord) createdAtUnix() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.run.CreatedAt.Unix()
}

func (r *runRecord) appendOutput(msg ACPMessage) {
	r.mu.Lock()
	r.completed = append(r.completed, msg)
	r.run.Output = append([]ACPMessage(nil), r.completed...)
	r.mu.Unlock()
}

func (r *runRecord) replaceOutput(output []ACPMessage) {
	r.mu.Lock()
	r.completed = append([]ACPMessage(nil), output...)
	r.run.Output = append([]ACPMessage(nil), output...)
	r.mu.Unlock()
}

func (r *runRecord) finishCompleted() {
	now := time.Now().UTC()
	r.mu.Lock()
	r.run.Status = RunStatusCompleted
	r.run.FinishedAt = &now
	r.run.Output = append([]ACPMessage(nil), r.completed...)
	r.mu.Unlock()
}

func (r *runRecord) finishCancelled() {
	now := time.Now().UTC()
	r.mu.Lock()
	r.run.Status = RunStatusCancelled
	r.run.FinishedAt = &now
	r.run.Output = append([]ACPMessage(nil), r.completed...)
	r.mu.Unlock()
}

func (r *runRecord) finishFailed(err *Error) {
	now := time.Now().UTC()
	r.mu.Lock()
	r.run.Status = RunStatusFailed
	r.run.Error = err
	r.run.FinishedAt = &now
	r.run.Output = append([]ACPMessage(nil), r.completed...)
	r.mu.Unlock()
}

func (r *runRecord) closeDone() {
	r.mu.Lock()
	if !r.doneClosed {
		r.doneClosed = true
		close(r.done)
	}
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (s *Server) storeRun(rec *runRecord) {
	s.runsMu.Lock()
	s.runs[rec.run.RunID] = rec
	s.runsMu.Unlock()
}

func (s *Server) getRun(id string) (*runRecord, bool) {
	s.runsMu.RLock()
	defer s.runsMu.RUnlock()
	rec, ok := s.runs[id]
	return rec, ok
}

func (s *Server) handleServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeACPError(w, http.StatusNotFound, ErrorCodeNotFound, "Resource not found")
		return
	}
	writeACPError(w, http.StatusInternalServerError, ErrorCodeServerError, err.Error())
}

func writeACPError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	writeJSONWithStatus(w, status, Error{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	writeJSONWithStatus(w, status, value)
}

func writeJSONWithStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("Failed to encode ACP response", "error", err)
	}
}
