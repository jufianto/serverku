package digitalocean

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
	"golang.org/x/crypto/ssh"
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

func TestListComponentsSSHKeyIdentity(t *testing.T) {
	pub, err := ssh.NewPublicKey(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public())
	if err != nil {
		t.Fatal(err)
	}
	publicKey := string(ssh.MarshalAuthorizedKey(pub))
	fingerprint := ssh.FingerprintLegacyMD5(pub)
	for _, tc := range []struct {
		name         string
		publicKey    string
		status       int
		wantPresent  bool
		wantUnknown  bool
		wantKeyCalls int
	}{
		{name: "key reused under another project name", publicKey: publicKey, status: 200, wantPresent: true, wantKeyCalls: 1},
		{name: "key not registered", publicKey: publicKey, status: 404, wantKeyCalls: 1},
		{name: "API permission failure", publicKey: publicKey, status: 403, wantUnknown: true, wantKeyCalls: 1},
		{name: "missing local public key", wantUnknown: true},
		{name: "invalid local public key", publicKey: "invalid", wantUnknown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keyCalls := 0
			p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("inventory must be read-only: %s", r.Method)
				}
				switch r.URL.Path {
				case "/v2/droplets":
					_, _ = w.Write([]byte(`{"droplets":[]}`))
				case "/v2/account/keys/" + fingerprint:
					keyCalls++
					w.WriteHeader(tc.status)
					if tc.status == 200 {
						_ = json.NewEncoder(w).Encode(map[string]any{"ssh_key": godo.Key{ID: 42, Name: "serverku-serverku-wpblog", Fingerprint: fingerprint}})
					} else {
						_, _ = w.Write([]byte(`{"id":"lookup_failed","message":"lookup failed"}`))
					}
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			})
			components, err := p.ListComponents(context.Background(), provider.ComponentQuery{VMName: "serverku-kuma", SSHPubKey: tc.publicKey})
			if err != nil {
				t.Fatal(err)
			}
			var key *provider.Component
			for i := range components {
				if components[i].Kind == "SSH key" {
					key = &components[i]
				}
			}
			if key == nil || key.Present != tc.wantPresent || key.LookupFailed != tc.wantUnknown || !key.Shared || key.RemovedByDestroy {
				t.Fatalf("incorrect key status: %+v", key)
			}
			if tc.wantPresent && key.Name != "serverku-serverku-wpblog" {
				t.Fatalf("must show actual reused key name: %+v", key)
			}
			if keyCalls != tc.wantKeyCalls {
				t.Fatalf("key API calls = %d, want %d", keyCalls, tc.wantKeyCalls)
			}
		})
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
