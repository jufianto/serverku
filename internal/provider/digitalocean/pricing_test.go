package digitalocean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/digitalocean/godo"
)

func pricingTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := godo.New(srv.Client(), godo.SetBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("godo.New: %v", err)
	}
	return &Provider{client: client}
}

func TestVMHourlyRateUSD(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/sizes") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sizes": []godo.Size{
				{Slug: "s-1vcpu-512mb-10gb", PriceHourly: 0.00595},
				{Slug: "s-1vcpu-1gb", PriceHourly: 0.00893, PriceMonthly: 6.0},
			},
		})
	})

	rate, err := p.VMHourlyRateUSD(context.Background(), "s-1vcpu-1gb", "sgp1", false)
	if err != nil {
		t.Fatalf("VMHourlyRateUSD: %v", err)
	}
	if rate != 0.00893 {
		t.Errorf("rate = %v, want 0.00893", rate)
	}
}

func TestVMHourlyRateUSD_UnknownSize(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sizes": []godo.Size{{Slug: "s-1vcpu-1gb", PriceHourly: 0.00893}},
		})
	})

	if _, err := p.VMHourlyRateUSD(context.Background(), "does-not-exist", "sgp1", false); err == nil {
		t.Fatal("expected error for unknown size")
	}
}

func TestVMHourlyRateUSD_RejectsSpot(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no API call expected for spot lookup")
	})

	if _, err := p.VMHourlyRateUSD(context.Background(), "s-1vcpu-1gb", "sgp1", true); err == nil {
		t.Fatal("expected error for spot on digitalocean")
	}
}

func TestMonthToDateUsageUSD(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/customers/my/balance") {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"month_to_date_balance": "23.44",
			"account_balance":       "12.23",
			"month_to_date_usage":   "11.21",
			"generated_at":          "2026-07-12T08:44:38Z",
		})
	})

	usd, err := p.MonthToDateUsageUSD(context.Background())
	if err != nil {
		t.Fatalf("MonthToDateUsageUSD: %v", err)
	}
	if usd != 11.21 {
		t.Errorf("usage = %v, want 11.21", usd)
	}
}

func TestValidateCredentials(t *testing.T) {
	ok := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/account") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"account": map[string]any{"status": "active", "email": "x@example.com"}})
	})
	if err := ok.ValidateCredentials(context.Background()); err != nil {
		t.Errorf("ValidateCredentials: %v", err)
	}

	bad := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"id":"unauthorized","message":"Unable to authenticate you"}`))
	})
	if err := bad.ValidateCredentials(context.Background()); err == nil {
		t.Error("expected error on 401")
	}
}

func TestAccountEmail(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/account") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"account": map[string]any{"email": "you@example.com", "status": "active"},
		})
	})

	email, err := p.AccountEmail(context.Background())
	if err != nil {
		t.Fatalf("AccountEmail: %v", err)
	}
	if email != "you@example.com" {
		t.Errorf("AccountEmail = %q, want you@example.com", email)
	}
}

func TestAccountEmail_BadToken(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"id": "unauthorized", "message": "Unable to authenticate"})
	})
	if _, err := p.AccountEmail(context.Background()); err == nil {
		t.Error("expected an error for an unauthorized token")
	}
}
