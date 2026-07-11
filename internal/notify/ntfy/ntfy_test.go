package ntfy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recorded struct {
	path     string
	body     string
	title    string
	priority string
}

func testNotifier(t *testing.T) (*Notifier, *recorded) {
	t.Helper()
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.path = r.URL.Path
		rec.body = string(b)
		rec.title = r.Header.Get("Title")
		rec.priority = r.Header.Get("Priority")
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "serverku-demo-abc123"), rec
}

func TestSendUp(t *testing.T) {
	n, rec := testNotifier(t)

	if err := n.SendUp(context.Background(), "demo", "1.2.3.4"); err != nil {
		t.Fatalf("SendUp: %v", err)
	}
	if rec.path != "/serverku-demo-abc123" {
		t.Errorf("path = %q, want topic path", rec.path)
	}
	if !strings.Contains(rec.body, "demo") || !strings.Contains(rec.body, "1.2.3.4") {
		t.Errorf("body missing project/ip: %q", rec.body)
	}
	if rec.title == "" {
		t.Error("expected a Title header")
	}
}

func TestSendErrorIsHighPriority(t *testing.T) {
	n, rec := testNotifier(t)

	if err := n.SendError(context.Background(), "demo", context.DeadlineExceeded); err != nil {
		t.Fatalf("SendError: %v", err)
	}
	if rec.priority != "high" {
		t.Errorf("priority = %q, want high", rec.priority)
	}
}

func TestSendServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	n := New(srv.URL, "topic")
	if err := n.SendTest(context.Background(), "demo"); err == nil {
		t.Fatal("expected error on 4xx response")
	}
}

func TestEmptyTopicIsNoop(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	n := New(srv.URL, "")
	if err := n.SendUp(context.Background(), "demo", "1.2.3.4"); err != nil {
		t.Fatalf("SendUp: %v", err)
	}
	if called {
		t.Error("no request expected for empty topic")
	}
}
