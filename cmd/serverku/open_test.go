package main

import (
	"testing"

	"github.com/jufianto/serverku/internal/config"
)

func runningState(ip string) *config.ProjectState {
	return &config.ProjectState{Status: config.StatusRunning, ExternalIP: ip}
}

func TestResolveOpenURL_IPFallback(t *testing.T) {
	cfg := &config.ProjectConfig{Name: "demo"}
	url, err := resolveOpenURL(cfg, runningState("1.2.3.4"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "http://1.2.3.4" {
		t.Errorf("got %q, want http://1.2.3.4", url)
	}
}

func TestResolveOpenURL_FirstDomainWins(t *testing.T) {
	cfg := &config.ProjectConfig{
		Name: "demo",
		Router: config.RouterConfig{
			Enabled: true,
			Domains: []config.DomainConfig{
				{Domain: "app.example.com", Service: "web"},
				{Domain: "api.example.com", Service: "api"},
			},
		},
	}
	url, err := resolveOpenURL(cfg, runningState("1.2.3.4"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://app.example.com" {
		t.Errorf("got %q, want https://app.example.com", url)
	}
}

func TestResolveOpenURL_SelectByDomainAndService(t *testing.T) {
	cfg := &config.ProjectConfig{
		Name: "demo",
		Router: config.RouterConfig{
			Enabled: true,
			Domains: []config.DomainConfig{
				{Domain: "app.example.com", Service: "web"},
				{Domain: "api.example.com", Service: "api"},
			},
		},
	}
	state := runningState("1.2.3.4")

	// Select by service name.
	url, err := resolveOpenURL(cfg, state, "api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://api.example.com" {
		t.Errorf("got %q, want https://api.example.com", url)
	}

	// Select by domain name.
	url, err = resolveOpenURL(cfg, state, "api.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://api.example.com" {
		t.Errorf("got %q, want https://api.example.com", url)
	}
}

func TestResolveOpenURL_UnknownSelector(t *testing.T) {
	cfg := &config.ProjectConfig{
		Name: "demo",
		Router: config.RouterConfig{
			Enabled: true,
			Domains: []config.DomainConfig{{Domain: "app.example.com", Service: "web"}},
		},
	}
	if _, err := resolveOpenURL(cfg, runningState("1.2.3.4"), "nope"); err == nil {
		t.Error("expected error for unknown domain/service selector")
	}
}

func TestResolveOpenURL_SelectorWithoutRouter(t *testing.T) {
	cfg := &config.ProjectConfig{Name: "demo"}
	if _, err := resolveOpenURL(cfg, runningState("1.2.3.4"), "web"); err == nil {
		t.Error("expected error selecting a domain when router is disabled")
	}
}

func TestResolveOpenURL_NotRunning(t *testing.T) {
	cfg := &config.ProjectConfig{Name: "demo"}
	state := &config.ProjectState{Status: config.StatusStopped, ExternalIP: "1.2.3.4"}
	if _, err := resolveOpenURL(cfg, state, ""); err == nil {
		t.Error("expected error for a stopped project")
	}
}

func TestResolveOpenURL_RunningNoIP(t *testing.T) {
	cfg := &config.ProjectConfig{Name: "demo"}
	if _, err := resolveOpenURL(cfg, runningState(""), ""); err == nil {
		t.Error("expected error when running project has no external IP")
	}
}

func TestResolveOpenURL_RouterEnabledNoDomainsFallsBackToIP(t *testing.T) {
	cfg := &config.ProjectConfig{
		Name:   "demo",
		Router: config.RouterConfig{Enabled: true},
	}
	url, err := resolveOpenURL(cfg, runningState("1.2.3.4"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "http://1.2.3.4" {
		t.Errorf("got %q, want http://1.2.3.4", url)
	}
}
