package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/swarmy/internal/db"
	"github.com/charmbracelet/swarmy/internal/message"
	"github.com/charmbracelet/swarmy/internal/permission"
	"github.com/charmbracelet/swarmy/internal/session"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	run func(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) error
}

func (f *fakeRunner) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, f.run(ctx, sessionID, prompt, attachments...)
}

func (f *fakeRunner) Cancel(string) {}

func TestSyncRun(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	server := NewServer(&fakeRunner{run: func(ctx context.Context, sessionID, prompt string, _ ...message.Attachment) error {
		msg, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{},
		})
		require.NoError(t, err)
		msg.AppendContent("reply: " + prompt)
		msg.AddFinish(message.FinishReasonEndTurn, "", "")
		require.NoError(t, env.messages.Update(ctx, msg))
		return nil
	}}, env.sessions, env.messages, env.permissions, Options{})

	body := map[string]any{
		"agent_name": "swarmy",
		"mode":       "sync",
		"input": []map[string]any{{
			"role": "user",
			"parts": []map[string]any{{
				"content": "hello",
			}},
		}},
	}

	resp := doRequest(t, server.Handler(), http.MethodPost, "/runs", body)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var run Run
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&run))
	require.Equal(t, RunStatusCompleted, run.Status)
	require.Len(t, run.Output, 1)
	require.Equal(t, "agent/swarmy", run.Output[0].Role)
	require.Equal(t, "reply: hello", run.Output[0].Parts[0].Content)
}

func TestAsyncRunCancel(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	runDone := make(chan error, 1)
	server := NewServer(&fakeRunner{run: func(ctx context.Context, sessionID, prompt string, _ ...message.Attachment) error {
		msg, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{},
		})
		if err != nil {
			runDone <- err
			return err
		}
		msg.AppendContent("working on " + prompt)
		if err := env.messages.Update(ctx, msg); err != nil {
			runDone <- err
			return err
		}
		<-ctx.Done()
		msg.AddFinish(message.FinishReasonCanceled, "", "")
		if err := env.messages.Update(context.Background(), msg); err != nil {
			runDone <- err
			return err
		}
		runDone <- nil
		return ctx.Err()
	}}, env.sessions, env.messages, env.permissions, Options{})

	body := map[string]any{
		"agent_name": "swarmy",
		"mode":       "async",
		"input": []map[string]any{{
			"role":  "user",
			"parts": []map[string]any{{"content": "long task"}},
		}},
	}

	resp := doRequest(t, server.Handler(), http.MethodPost, "/runs", body)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var run Run
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&run))
	require.Equal(t, RunStatusCreated, run.Status)

	cancelResp := doRequest(t, server.Handler(), http.MethodPost, "/runs/"+run.RunID+"/cancel", nil)
	defer cancelResp.Body.Close()
	require.Equal(t, http.StatusAccepted, cancelResp.StatusCode)

	require.Eventually(t, func() bool {
		statusResp := doRequest(t, server.Handler(), http.MethodGet, "/runs/"+run.RunID, nil)
		defer statusResp.Body.Close()
		var current Run
		if err := json.NewDecoder(statusResp.Body).Decode(&current); err != nil {
			return false
		}
		return current.Status == RunStatusCancelled
	}, 2*time.Second, 20*time.Millisecond)

	select {
	case err := <-runDone:
		require.True(t, err == nil || err == context.Canceled, "unexpected runner error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cancelled run to flush final message update")
	}
}

func TestAsyncRunSurvivesRequestCancellation(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	runDone := make(chan error, 1)
	server := NewServer(&fakeRunner{run: func(ctx context.Context, sessionID, prompt string, _ ...message.Attachment) error {
		time.Sleep(50 * time.Millisecond)

		msg, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{},
		})
		if err != nil {
			runDone <- err
			return err
		}
		msg.AppendContent("reply: " + prompt)
		msg.AddFinish(message.FinishReasonEndTurn, "", "")
		if err := env.messages.Update(ctx, msg); err != nil {
			runDone <- err
			return err
		}
		select {
		case <-ctx.Done():
			runDone <- ctx.Err()
			return ctx.Err()
		default:
		}
		runDone <- nil
		return nil
	}}, env.sessions, env.messages, env.permissions, Options{})

	body := map[string]any{
		"agent_name": "swarmy",
		"mode":       "async",
		"input": []map[string]any{{
			"role":  "user",
			"parts": []map[string]any{{"content": "hello"}},
		}},
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	reqCtx, cancelReq := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewReader(payload)).WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var run Run
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&run))
	cancelReq()

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async run to complete")
	}

	require.Eventually(t, func() bool {
		statusResp := doRequest(t, server.Handler(), http.MethodGet, "/runs/"+run.RunID, nil)
		defer statusResp.Body.Close()

		var current Run
		if err := json.NewDecoder(statusResp.Body).Decode(&current); err != nil {
			return false
		}
		return current.Status == RunStatusCompleted
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncRunSurvivesHTTPRequestLifecycle(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	runDone := make(chan error, 1)
	server := NewServer(&fakeRunner{run: func(ctx context.Context, sessionID, prompt string, _ ...message.Attachment) error {
		time.Sleep(50 * time.Millisecond)

		msg, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{},
		})
		if err != nil {
			runDone <- err
			return err
		}
		msg.AppendContent("reply: " + prompt)
		msg.AddFinish(message.FinishReasonEndTurn, "", "")
		if err := env.messages.Update(ctx, msg); err != nil {
			runDone <- err
			return err
		}
		select {
		case <-ctx.Done():
			runDone <- ctx.Err()
			return ctx.Err()
		default:
		}
		runDone <- nil
		return nil
	}}, env.sessions, env.messages, env.permissions, Options{})

	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)

	body := map[string]any{
		"agent_name": "swarmy",
		"mode":       "async",
		"input": []map[string]any{{
			"role":  "user",
			"parts": []map[string]any{{"content": "hello over http"}},
		}},
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, httpServer.URL+"/runs", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpServer.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var run Run
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&run))

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async HTTP run to complete")
	}

	require.Eventually(t, func() bool {
		statusResp, err := httpServer.Client().Get(httpServer.URL + "/runs/" + run.RunID)
		if err != nil {
			return false
		}
		defer statusResp.Body.Close()

		var current Run
		if err := json.NewDecoder(statusResp.Body).Decode(&current); err != nil {
			return false
		}
		return current.Status == RunStatusCompleted
	}, 2*time.Second, 20*time.Millisecond)
}

func TestSessionIDFromRequest(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	server := NewServer(&fakeRunner{run: func(ctx context.Context, sessionID, prompt string, _ ...message.Attachment) error {
		msg, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: prompt},
				message.Finish{Reason: message.FinishReasonEndTurn, Time: time.Now().Unix()},
			},
		})
		require.NoError(t, err)
		return env.messages.Update(ctx, msg)
	}}, env.sessions, env.messages, env.permissions, Options{})

	sessionID := "a4500efc-d5f4-414a-8736-fb82360b3329"
	body := map[string]any{
		"agent_name": "swarmy",
		"mode":       "sync",
		"session": map[string]any{
			"id":      sessionID,
			"history": []string{},
		},
		"input": []map[string]any{{
			"role":  "user",
			"parts": []map[string]any{{"content": "hello"}},
		}},
	}

	resp := doRequest(t, server.Handler(), http.MethodPost, "/runs", body)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var run Run
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&run))
	require.NotNil(t, run.SessionID)
	require.Equal(t, sessionID, *run.SessionID)

	sessResp := doRequest(t, server.Handler(), http.MethodGet, "/sessions/"+sessionID, nil)
	defer sessResp.Body.Close()
	require.Equal(t, http.StatusOK, sessResp.StatusCode)

	var descriptor SessionDescriptor
	require.NoError(t, json.NewDecoder(sessResp.Body).Decode(&descriptor))
	require.Equal(t, sessionID, descriptor.ID)
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Result()
}

type testEnv struct {
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)
	perm := permission.NewPermissionService(t.TempDir(), true, nil)

	return testEnv{
		sessions:    sessions,
		messages:    messages,
		permissions: perm,
	}
}
