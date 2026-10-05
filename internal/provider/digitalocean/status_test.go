package digitalocean

import (
	"context"
	"net/http"
	"testing"

	"github.com/jufianto/serverku/internal/provider"
)

func TestGetVMStatusMissingDroplet(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"droplets":[]}`))
	})
	status, err := p.GetVMStatus(context.Background(), "serverku-kuma")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != provider.VMStateTerminated {
		t.Fatalf("state = %s, want terminated", status.State)
	}
}

func TestGetVMStatusDoesNotTreatLookupFailureAsDeletion(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"id":"forbidden","message":"no access"}`))
	})
	if status, err := p.GetVMStatus(context.Background(), "serverku-kuma"); err == nil || status != nil {
		t.Fatalf("lookup failure must remain an error: %+v, %v", status, err)
	}
}
