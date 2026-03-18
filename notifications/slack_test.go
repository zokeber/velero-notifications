package notifications

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestNewSlackNotifierRequiresHTTPSWebhook(t *testing.T) {
	t.Parallel()

	_, err := NewSlackNotifier(SlackConfig{Webhook: "http://example.com/webhook"})
	if err == nil {
		t.Fatal("expected error for non-https webhook")
	}
}

func TestSlackNotifierNotifyBuildsBlockPayload(t *testing.T) {
	t.Parallel()

	var (
		captured   slackPayload
		handlerErr error
		mu         sync.Mutex
	)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.Method != http.MethodPost {
			handlerErr = fmt.Errorf("unexpected method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			handlerErr = fmt.Errorf("read request body: %w", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := json.Unmarshal(body, &captured); err != nil {
			handlerErr = fmt.Errorf("unmarshal request body: %w", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier, err := NewSlackNotifier(SlackConfig{
		Webhook:  server.URL,
		Channel:  "velero-notifications",
		Username: "Velero",
		Prefix:   "[Velero]",
	})
	if err != nil {
		t.Fatalf("new notifier: %v", err)
	}
	notifier.client = server.Client()

	err = notifier.Notify("Completed", "Backup demo completed successfully.")
	if err != nil {
		t.Fatalf("notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if handlerErr != nil {
		t.Fatalf("handler error: %v", handlerErr)
	}

	if !strings.Contains(captured.Text, "Completed") {
		t.Fatalf("expected payload text to include status, got %q", captured.Text)
	}

	if len(captured.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(captured.Attachments))
	}

	attachment := captured.Attachments[0]
	if attachment.Color != statusMap["completed"].color {
		t.Fatalf("expected completed color %q, got %q", statusMap["completed"].color, attachment.Color)
	}

	if len(attachment.Blocks) < 4 {
		t.Fatalf("expected block kit payload, got %d blocks", len(attachment.Blocks))
	}

	if got := attachment.Blocks[0].Text; got == nil || !strings.Contains(got.Text, "*Cluster:*") {
		t.Fatal("expected first block to contain cluster section")
	}
}

func TestSlackNotifierNotifyEscapesMrkdwnContent(t *testing.T) {
	t.Parallel()

	var (
		captured   slackPayload
		handlerErr error
		mu         sync.Mutex
	)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			handlerErr = fmt.Errorf("read request body: %w", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := json.Unmarshal(body, &captured); err != nil {
			handlerErr = fmt.Errorf("unmarshal request body: %w", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier, err := NewSlackNotifier(SlackConfig{
		Webhook:  server.URL,
		Channel:  "velero-notifications",
		Username: "Velero",
	})
	if err != nil {
		t.Fatalf("new notifier: %v", err)
	}
	notifier.client = server.Client()

	err = notifier.Notify("PartiallyFailed", "Backup velero-homelab-16-20260318172303 finished with status: PartiallyFailed.\n\nStart Time: Wed, Mar 18, 2026 at 5:23 PM UTC, End Time: Wed, Mar 18, 2026 at 5:49 PM UTC.\n\nProgress: 341/341 items processed (with 2 errors).\nFailure Reason: Failed <prod> & needs <@U123>")
	if err != nil {
		t.Fatalf("notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if handlerErr != nil {
		t.Fatalf("handler error: %v", handlerErr)
	}

	if len(captured.Attachments) == 0 || len(captured.Attachments[0].Blocks) < 5 {
		t.Fatalf("invalid attachment blocks in payload: %+v", captured.Attachments)
	}

	attachment := captured.Attachments[0]
	if attachment.Blocks[2].Type != "section" || len(attachment.Blocks[2].Fields) != 2 {
		t.Fatalf("expected section with start/end time fields, got %+v", attachment.Blocks[2])
	}

	start := attachment.Blocks[2].Fields[0].Text
	end := attachment.Blocks[2].Fields[1].Text

	if !strings.Contains(start, "03/18/26 at 5:23 PM UTC") {
		t.Fatalf("expected normalized start time, got %q", start)
	}

	if !strings.Contains(end, "03/18/26 at 5:49 PM UTC") {
		t.Fatalf("expected normalized end time, got %q", end)
	}

	progressBlock := attachment.Blocks[3]
	if progressBlock.Text == nil || !strings.Contains(progressBlock.Text.Text, "*Progress:*") {
		t.Fatalf("expected progress block, got %+v", progressBlock)
	}

	failureReasonBlock := attachment.Blocks[4]
	if failureReasonBlock.Text == nil {
		t.Fatalf("expected failure reason block, got %+v", failureReasonBlock)
	}

	if !strings.Contains(failureReasonBlock.Text.Text, "&lt;@U123&gt;") {
		t.Fatalf("expected escaped Slack mention token in failure reason, got %q", failureReasonBlock.Text.Text)
	}

	encoded, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("marshal captured payload: %v", err)
	}

	payloadString := string(encoded)
	if !strings.Contains(payloadString, "\\u0026lt;@U123\\u0026gt;") {
		t.Fatalf("expected encoded escaped mention token in JSON payload, got %q", payloadString)
	}
}

func TestSlackNotifierNotifyFailuresOnlySkipsCompleted(t *testing.T) {
	t.Parallel()

	var (
		mu         sync.Mutex
		requestCnt int
	)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCnt++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier, err := NewSlackNotifier(SlackConfig{
		Webhook:      server.URL,
		FailuresOnly: true,
	})
	if err != nil {
		t.Fatalf("new notifier: %v", err)
	}
	notifier.client = server.Client()

	if err := notifier.Notify("Completed", "Backup done"); err != nil {
		t.Fatalf("notify completed: %v", err)
	}

	if err := notifier.Notify("Failed", "Backup failed"); err != nil {
		t.Fatalf("notify failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requestCnt != 1 {
		t.Fatalf("expected only one request for failures_only=true, got %d", requestCnt)
	}
}
