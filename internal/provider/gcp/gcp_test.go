package gcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jufianto/serverku/internal/provider"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// --- pure helper tests -------------------------------------------------------

func TestMapGCPStatus(t *testing.T) {
	cases := map[string]provider.VMStatusState{
		"RUNNING":      provider.VMStateRunning,
		"STOPPED":      provider.VMStateStopped,
		"SUSPENDED":    provider.VMStateStopped,
		"STAGING":      provider.VMStateStarting,
		"PROVISIONING": provider.VMStateStarting,
		"STOPPING":     provider.VMStateStopping,
		"SUSPENDING":   provider.VMStateStopping,
		"TERMINATED":   provider.VMStateTerminated,
		"WAT":          provider.VMStateUnknown,
		"":             provider.VMStateUnknown,
	}
	for in, want := range cases {
		if got := mapGCPStatus(in); got != want {
			t.Errorf("mapGCPStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractExternalIP(t *testing.T) {
	if ip := extractExternalIP(nil); ip != "" {
		t.Errorf("nil instance: got %q, want empty", ip)
	}
	if ip := extractExternalIP(&compute.Instance{}); ip != "" {
		t.Errorf("no interfaces: got %q, want empty", ip)
	}

	inst := &compute.Instance{
		NetworkInterfaces: []*compute.NetworkInterface{
			{AccessConfigs: []*compute.AccessConfig{{NatIP: ""}}},
			{AccessConfigs: []*compute.AccessConfig{{NatIP: "203.0.113.7"}}},
		},
	}
	if ip := extractExternalIP(inst); ip != "203.0.113.7" {
		t.Errorf("got %q, want 203.0.113.7", ip)
	}
}

func TestResolveImageFamily(t *testing.T) {
	cases := map[string]string{
		"":             defaultImageFamily,
		"ubuntu-22-04": defaultImageFamily,
		"ubuntu-2204":  defaultImageFamily,
		"ubuntu-20-04": "ubuntu-2004-lts",
		"ubuntu-24-04": "ubuntu-2404-lts-amd64",
		"custom-image": "custom-image", // passthrough
	}
	for in, want := range cases {
		if got := resolveImageFamily(in); got != want {
			t.Errorf("resolveImageFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveZoneAndProjectFallback(t *testing.T) {
	g := NewWithService(nil, "default-proj", "default-zone")

	if got := g.resolveZone(""); got != "default-zone" {
		t.Errorf("resolveZone(\"\") = %q, want default-zone", got)
	}
	if got := g.resolveZone("override-zone"); got != "override-zone" {
		t.Errorf("resolveZone(override) = %q, want override-zone", got)
	}
	if got := g.resolveProjectID(""); got != "default-proj" {
		t.Errorf("resolveProjectID(\"\") = %q, want default-proj", got)
	}
	if got := g.resolveProjectID("override-proj"); got != "override-proj" {
		t.Errorf("resolveProjectID(override) = %q, want override-proj", got)
	}
}

// --- fake-endpoint flow tests ------------------------------------------------

// fakeProvider returns a GCPProvider whose compute service is pointed at the
// given handler, so API request building and response handling are exercised
// without touching real GCP.
func fakeProvider(t *testing.T, h http.HandlerFunc) *GCPProvider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	svc, err := compute.NewService(context.Background(),
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("compute.NewService: %v", err)
	}
	return NewWithService(svc, "test-proj", "test-zone")
}

// notFoundBody is a GCP-style 404 error payload whose text contains "notFound"
// so isNotFoundErr recognizes it.
const notFoundBody = `{"error":{"code":404,"message":"The resource was not found (notFound)","errors":[{"reason":"notFound","message":"notFound"}]}}`

func TestGetVMStatus_RunningWithIP(t *testing.T) {
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/instances/myvm") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(compute.Instance{
			Id:     42,
			Name:   "myvm",
			Status: "RUNNING",
			NetworkInterfaces: []*compute.NetworkInterface{
				{AccessConfigs: []*compute.AccessConfig{{NatIP: "198.51.100.4"}}},
			},
		})
	})

	st, err := g.GetVMStatus(context.Background(), "myvm")
	if err != nil {
		t.Fatalf("GetVMStatus: %v", err)
	}
	if st.ID != "42" || st.Name != "myvm" {
		t.Errorf("unexpected identity: id=%q name=%q", st.ID, st.Name)
	}
	if st.State != provider.VMStateRunning {
		t.Errorf("state = %q, want running", st.State)
	}
	if st.ExternalIP != "198.51.100.4" {
		t.Errorf("external IP = %q, want 198.51.100.4", st.ExternalIP)
	}
}

func TestGetVMStatus_NotFoundIsTerminated(t *testing.T) {
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, notFoundBody)
	})

	st, err := g.GetVMStatus(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("expected nil error for missing VM, got %v", err)
	}
	if st.State != provider.VMStateTerminated {
		t.Errorf("state = %q, want terminated", st.State)
	}
}

func TestGetExternalIP_NoIPIsError(t *testing.T) {
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(compute.Instance{Name: "myvm", Status: "RUNNING"})
	})

	if _, err := g.GetExternalIP(context.Background(), "myvm"); err == nil {
		t.Fatal("expected error when VM has no external IP, got nil")
	}
}

func TestDestroyVM_NotFoundIsIdempotent(t *testing.T) {
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, notFoundBody)
	})

	if err := g.DestroyVM(context.Background(), "ghost"); err != nil {
		t.Fatalf("expected nil error for already-deleted VM, got %v", err)
	}
}

func TestCreateVM_BuildsSpotInstanceRequest(t *testing.T) {
	var insertBody compute.Instance
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/operations/"):
			// Operation poll: report completion immediately.
			_ = json.NewEncoder(w).Encode(compute.Operation{Name: "op-1", Status: "DONE"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/instances"):
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &insertBody)
			_ = json.NewEncoder(w).Encode(compute.Operation{Name: "op-1", Status: "PENDING"})
		case strings.HasSuffix(r.URL.Path, "/instances/spotvm"):
			_ = json.NewEncoder(w).Encode(compute.Instance{Id: 7, Name: "spotvm", Status: "PROVISIONING"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	vm, err := g.CreateVM(context.Background(), provider.VMConfig{
		Name:        "spotvm",
		MachineType: "e2-medium",
		Spot:        true,
		SSHPubKey:   "ssh-rsa AAAetc",
	})
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	if vm.ID != "7" || vm.Name != "spotvm" || vm.Zone != "test-zone" || vm.Provider != "gcp" {
		t.Errorf("unexpected VM: %+v", vm)
	}

	// Verify the request we built for GCP.
	if insertBody.Scheduling == nil || insertBody.Scheduling.ProvisioningModel != "SPOT" || !insertBody.Scheduling.Preemptible {
		t.Errorf("expected SPOT/preemptible scheduling, got %+v", insertBody.Scheduling)
	}
	if !strings.Contains(insertBody.MachineType, "e2-medium") {
		t.Errorf("machineType = %q, want it to reference e2-medium", insertBody.MachineType)
	}
	var hasSSHKey bool
	if insertBody.Metadata != nil {
		for _, item := range insertBody.Metadata.Items {
			if item.Key == "ssh-keys" && item.Value != nil && strings.Contains(*item.Value, "ssh-rsa AAAetc") {
				hasSSHKey = true
			}
		}
	}
	if !hasSSHKey {
		t.Errorf("expected ssh-keys metadata to be set, got %+v", insertBody.Metadata)
	}
}
