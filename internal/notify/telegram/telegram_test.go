package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestNotifier returns a Notifier pointed at a test server that records the
// last request path and body and responds with the given status code.
func newTestNotifier(t *testing.T, status int) (*Notifier, *string, *string) {
	t.Helper()
	var lastPath, lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	n := New("test-token", "12345")
	n.baseURL = srv.URL
	return n, &lastPath, &lastBody
}

func TestSendUp_PostsToBotEndpointWithPayload(t *testing.T) {
	n, path, body := newTestNotifier(t, http.StatusOK)

	if err := n.SendUp(context.Background(), "myapp", "1.2.3.4"); err != nil {
		t.Fatalf("SendUp: unexpected error: %v", err)
	}

	if *path != "/bottest-token/sendMessage" {
		t.Errorf("unexpected request path: %q", *path)
	}

	var p payload
	if err := json.Unmarshal([]byte(*body), &p); err != nil {
		t.Fatalf("request body was not valid JSON: %v (%q)", err, *body)
	}
	if p.ChatID != "12345" {
		t.Errorf("expected chat_id 12345, got %q", p.ChatID)
	}
	if p.ParseMode != "Markdown" {
		t.Errorf("expected parse_mode Markdown, got %q", p.ParseMode)
	}
	if !strings.Contains(p.Text, "myapp") || !strings.Contains(p.Text, "1.2.3.4") {
		t.Errorf("message missing project/IP: %q", p.Text)
	}
}

func TestSendError_IncludesErrorText(t *testing.T) {
	n, _, body := newTestNotifier(t, http.StatusOK)

	if err := n.SendError(context.Background(), "myapp", errors.New("boom")); err != nil {
		t.Fatalf("SendError: unexpected error: %v", err)
	}
	if !strings.Contains(*body, "boom") {
		t.Errorf("SendError body missing error text: %q", *body)
	}
}

func TestSend_MissingCredentialsIsNoop(t *testing.T) {
	// Missing token or chat ID should short-circuit before any HTTP call.
	for _, tc := range []struct{ token, chat string }{
		{"", "12345"},
		{"test-token", ""},
	} {
		n := New(tc.token, tc.chat)
		if err := n.SendUp(context.Background(), "myapp", "1.2.3.4"); err != nil {
			t.Errorf("token=%q chat=%q: expected no-op, got error: %v", tc.token, tc.chat, err)
		}
	}
}

func TestSend_Non2xxReturnsError(t *testing.T) {
	n, _, _ := newTestNotifier(t, http.StatusBadRequest)

	if err := n.SendUp(context.Background(), "myapp", "1.2.3.4"); err == nil {
		t.Fatal("expected error on 400 response, got nil")
	}
}
