package slack

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

// captureServer returns a test server that records the last request body and
// responds with the given status code.
func captureServer(t *testing.T, status int) (*httptest.Server, *string) {
	t.Helper()
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &lastBody
}

func TestSendUp_PostsMessageWithProjectAndIP(t *testing.T) {
	srv, body := captureServer(t, http.StatusOK)
	n := New(srv.URL)

	if err := n.SendUp(context.Background(), "myapp", "1.2.3.4"); err != nil {
		t.Fatalf("SendUp: unexpected error: %v", err)
	}

	var p payload
	if err := json.Unmarshal([]byte(*body), &p); err != nil {
		t.Fatalf("request body was not valid JSON: %v (%q)", err, *body)
	}
	if !strings.Contains(p.Text, "myapp") {
		t.Errorf("expected message to contain project name, got %q", p.Text)
	}
	if !strings.Contains(p.Text, "1.2.3.4") {
		t.Errorf("expected message to contain IP, got %q", p.Text)
	}
}

func TestSendDownAndError_PostMessages(t *testing.T) {
	srv, body := captureServer(t, http.StatusOK)
	n := New(srv.URL)

	if err := n.SendDown(context.Background(), "myapp"); err != nil {
		t.Fatalf("SendDown: unexpected error: %v", err)
	}
	if !strings.Contains(*body, "myapp") {
		t.Errorf("SendDown body missing project name: %q", *body)
	}

	if err := n.SendError(context.Background(), "myapp", errors.New("boom")); err != nil {
		t.Fatalf("SendError: unexpected error: %v", err)
	}
	if !strings.Contains(*body, "boom") {
		t.Errorf("SendError body missing error text: %q", *body)
	}
}

func TestSend_EmptyWebhookIsNoop(t *testing.T) {
	// No server: if New("") tried to send, it would error on the empty URL.
	n := New("")
	if err := n.SendUp(context.Background(), "myapp", "1.2.3.4"); err != nil {
		t.Fatalf("expected empty webhook to be a no-op, got error: %v", err)
	}
}

func TestSend_Non2xxReturnsError(t *testing.T) {
	srv, _ := captureServer(t, http.StatusInternalServerError)
	n := New(srv.URL)

	if err := n.SendUp(context.Background(), "myapp", "1.2.3.4"); err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}
