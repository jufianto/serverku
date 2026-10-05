package digitalocean

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
	"golang.org/x/crypto/ssh"
)

func TestCreateVMSSHKeyReuse(t *testing.T) {
	public, err := ssh.NewPublicKey(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public())
	if err != nil {
		t.Fatal(err)
	}
	publicKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(public))) + " local-comment"
	fingerprint := ssh.FingerprintLegacyMD5(public)
	for _, tc := range []struct {
		name          string
		initialStatus int
		createStatus  int
		retryStatus   int
		wantCreates   int
		wantGets      int
		wantError     string
	}{
		{name: "reuse key registered under another name", initialStatus: 200, wantGets: 1},
		{name: "register missing key", initialStatus: 404, createStatus: 201, wantCreates: 1, wantGets: 1},
		{name: "reuse concurrent registration", initialStatus: 404, createStatus: 422, retryStatus: 200, wantCreates: 1, wantGets: 2},
		{name: "propagate lookup permission failure", initialStatus: 403, wantGets: 1, wantError: "failed to look up SSH key"},
		{name: "propagate unrelated create rejection", initialStatus: 404, createStatus: 422, retryStatus: 404, wantCreates: 1, wantGets: 2, wantError: "failed to create SSH key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gets, creates, droplets := 0, 0, 0
			existing := godo.Key{ID: 42, Name: "another-project", Fingerprint: fingerprint, PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(public))) + " different-comment"}
			p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v2/account/keys/"+fingerprint:
					gets++
					status := tc.initialStatus
					if gets > 1 {
						status = tc.retryStatus
					}
					w.WriteHeader(status)
					if status == 200 {
						_ = json.NewEncoder(w).Encode(map[string]any{"ssh_key": existing})
					} else {
						_ = json.NewEncoder(w).Encode(map[string]string{"id": "error", "message": "lookup failed"})
					}
				case r.Method == http.MethodPost && r.URL.Path == "/v2/account/keys":
					creates++
					var req godo.KeyCreateRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					if req.Name != "serverku-kuma" || req.PublicKey != publicKey {
						t.Errorf("unexpected key request: %+v", req)
					}
					w.WriteHeader(tc.createStatus)
					if tc.createStatus == 201 {
						_ = json.NewEncoder(w).Encode(map[string]any{"ssh_key": existing})
					} else {
						_ = json.NewEncoder(w).Encode(map[string]string{"id": "unprocessable_entity", "message": "SSH Key is already in use on your account"})
					}
				case r.Method == http.MethodPost && r.URL.Path == "/v2/droplets":
					droplets++
					var req struct {
						SSHKeys []string `json:"ssh_keys"`
						Image   string   `json:"image"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					if req.Image != "ubuntu-22-04-x64" {
						t.Errorf("image = %q, want DO slug", req.Image)
					}
					if len(req.SSHKeys) != 1 || req.SSHKeys[0] != fingerprint {
						t.Errorf("wrong SSH key: %+v", req.SSHKeys)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"droplet": godo.Droplet{ID: 123, Name: "kuma"}})
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(500)
				}
			})
			vm, err := p.CreateVM(context.Background(), provider.VMConfig{Name: "kuma", Region: "sgp1", Image: "ubuntu-22-04", SSHPubKey: publicKey})
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				if droplets != 0 {
					t.Fatal("created droplet after key failure")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if vm.ID != "123" || droplets != 1 {
					t.Fatalf("unexpected VM or droplet calls: %+v, %d", vm, droplets)
				}
			}
			if gets != tc.wantGets || creates != tc.wantCreates {
				t.Errorf("GETs=%d POSTs=%d, want %d/%d", gets, creates, tc.wantGets, tc.wantCreates)
			}
		})
	}
}

func TestCreateVMRejectsInvalidSSHKey(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid key must fail before API calls") })
	if _, err := p.CreateVM(context.Background(), provider.VMConfig{SSHPubKey: "invalid"}); err == nil || !strings.Contains(err.Error(), "invalid SSH public key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
