package orchestrator

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider"
	"github.com/jufianto/serverku/internal/provisioner"
)

// mockProvider implements provider.CloudProvider for testing.
type mockProvider struct {
	createVMFunc      func(ctx context.Context, config provider.VMConfig) (*provider.VM, error)
	destroyVMFunc     func(ctx context.Context, vmID string) error
	startVMFunc       func(ctx context.Context, vmID string) error
	stopVMFunc        func(ctx context.Context, vmID string) error
	getVMStatusFunc   func(ctx context.Context, vmID string) (*provider.VMStatus, error)
	getExternalIPFunc func(ctx context.Context, vmID string) (string, error)
	waitForReadyFunc  func(ctx context.Context, vmID string) error
	createDiskFunc    func(ctx context.Context, config provider.DiskConfig) (*provider.Disk, error)
	deleteDiskFunc    func(ctx context.Context, diskID string) error
	attachDiskFunc    func(ctx context.Context, vmID string, diskID string) error
	detachDiskFunc    func(ctx context.Context, vmID string, diskID string) error
	snapshotDiskFunc  func(ctx context.Context, diskName string, snapshotName string) (string, error)

	// Track calls for assertions
	calls []string
}

func (m *mockProvider) CreateVM(ctx context.Context, config provider.VMConfig) (*provider.VM, error) {
	m.calls = append(m.calls, "CreateVM")
	if m.createVMFunc != nil {
		return m.createVMFunc(ctx, config)
	}
	return &provider.VM{ID: "vm-123", Name: config.Name, Zone: "zone-1", Provider: "mock"}, nil
}

func (m *mockProvider) DestroyVM(ctx context.Context, vmID string) error {
	m.calls = append(m.calls, "DestroyVM")
	if m.destroyVMFunc != nil {
		return m.destroyVMFunc(ctx, vmID)
	}
	return nil
}

func (m *mockProvider) StartVM(ctx context.Context, vmID string) error {
	m.calls = append(m.calls, "StartVM")
	if m.startVMFunc != nil {
		return m.startVMFunc(ctx, vmID)
	}
	return nil
}

func (m *mockProvider) StopVM(ctx context.Context, vmID string) error {
	m.calls = append(m.calls, "StopVM")
	if m.stopVMFunc != nil {
		return m.stopVMFunc(ctx, vmID)
	}
	return nil
}

func (m *mockProvider) GetVMStatus(ctx context.Context, vmID string) (*provider.VMStatus, error) {
	m.calls = append(m.calls, "GetVMStatus")
	if m.getVMStatusFunc != nil {
		return m.getVMStatusFunc(ctx, vmID)
	}
	return &provider.VMStatus{ID: "vm-123", Name: vmID, State: provider.VMStateRunning, ExternalIP: "1.2.3.4"}, nil
}

func (m *mockProvider) GetExternalIP(ctx context.Context, vmID string) (string, error) {
	m.calls = append(m.calls, "GetExternalIP")
	if m.getExternalIPFunc != nil {
		return m.getExternalIPFunc(ctx, vmID)
	}
	return "1.2.3.4", nil
}

func (m *mockProvider) WaitForReady(ctx context.Context, vmID string) error {
	m.calls = append(m.calls, "WaitForReady")
	if m.waitForReadyFunc != nil {
		return m.waitForReadyFunc(ctx, vmID)
	}
	return nil
}

func (m *mockProvider) CreateDisk(ctx context.Context, config provider.DiskConfig) (*provider.Disk, error) {
	m.calls = append(m.calls, "CreateDisk")
	if m.createDiskFunc != nil {
		return m.createDiskFunc(ctx, config)
	}
	return &provider.Disk{ID: "disk-456", Name: config.Name, Zone: "zone-1", SizeGB: config.SizeGB, Provider: "mock"}, nil
}

func (m *mockProvider) DeleteDisk(ctx context.Context, diskID string) error {
	m.calls = append(m.calls, "DeleteDisk")
	if m.deleteDiskFunc != nil {
		return m.deleteDiskFunc(ctx, diskID)
	}
	return nil
}

func (m *mockProvider) AttachDisk(ctx context.Context, vmID string, diskID string) error {
	m.calls = append(m.calls, "AttachDisk")
	if m.attachDiskFunc != nil {
		return m.attachDiskFunc(ctx, vmID, diskID)
	}
	return nil
}

func (m *mockProvider) DetachDisk(ctx context.Context, vmID string, diskID string) error {
	m.calls = append(m.calls, "DetachDisk")
	if m.detachDiskFunc != nil {
		return m.detachDiskFunc(ctx, vmID, diskID)
	}
	return nil
}

func (m *mockProvider) SnapshotDisk(ctx context.Context, diskName string, snapshotName string) (string, error) {
	m.calls = append(m.calls, "SnapshotDisk")
	if m.snapshotDiskFunc != nil {
		return m.snapshotDiskFunc(ctx, diskName, snapshotName)
	}
	return "snap-789", nil
}

// testSetup creates a temp directory, store, and saves a test project config.
func testSetup(t *testing.T, storageCfg config.StorageConfig) (*Orchestrator, *mockProvider, ProviderFactory) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "serverku-orch-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	store, err := config.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Save test project config
	cfg := &config.ProjectConfig{
		Name:      "test-project",
		Provider:  "gcp",
		ProjectID: "my-gcp-project",
		Region:    "us-central1",
		Zone:      "us-central1-a",
		VM: config.VMConfig{
			Size:  "e2-medium",
			Image: "ubuntu-22-04",
			Spot:  true,
		},
		Storage: storageCfg,
	}
	if err := store.SaveProject(cfg); err != nil {
		t.Fatal(err)
	}

	mock := &mockProvider{}
	factory := func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		return mock, nil
	}

	orch := New(store, nil, nil, nil)
	return orch, mock, factory
}

func TestUpWithStorage(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	ctx := context.Background()
	result, err := orch.Up(ctx, "test-project", factory)
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Verify result
	if result.VMName != "serverku-test-project" {
		t.Errorf("VMName = %q, want %q", result.VMName, "serverku-test-project")
	}
	if result.VMID != "vm-123" {
		t.Errorf("VMID = %q, want %q", result.VMID, "vm-123")
	}
	if result.ExternalIP != "1.2.3.4" {
		t.Errorf("ExternalIP = %q, want %q", result.ExternalIP, "1.2.3.4")
	}
	if result.DiskName != "serverku-test-project-data" {
		t.Errorf("DiskName = %q, want %q", result.DiskName, "serverku-test-project-data")
	}
	if result.DiskID != "disk-456" {
		t.Errorf("DiskID = %q, want %q", result.DiskID, "disk-456")
	}

	// Verify correct call order
	expectedCalls := []string{"CreateDisk", "CreateVM", "AttachDisk", "WaitForReady", "GetExternalIP"}
	if len(mock.calls) != len(expectedCalls) {
		t.Fatalf("got %d calls, want %d: %v", len(mock.calls), len(expectedCalls), mock.calls)
	}
	for i, call := range expectedCalls {
		if mock.calls[i] != call {
			t.Errorf("call[%d] = %q, want %q", i, mock.calls[i], call)
		}
	}

	// Verify state was saved correctly
	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatalf("LoadState error: %v", err)
	}
	if state.Status != config.StatusRunning {
		t.Errorf("state.Status = %q, want %q", state.Status, config.StatusRunning)
	}
	if state.VMID != "vm-123" {
		t.Errorf("state.VMID = %q, want %q", state.VMID, "vm-123")
	}
	if state.VMName != "serverku-test-project" {
		t.Errorf("state.VMName = %q, want %q", state.VMName, "serverku-test-project")
	}
	if state.DiskID != "disk-456" {
		t.Errorf("state.DiskID = %q, want %q", state.DiskID, "disk-456")
	}
	if state.ExternalIP != "1.2.3.4" {
		t.Errorf("state.ExternalIP = %q, want %q", state.ExternalIP, "1.2.3.4")
	}
}

func TestUpWithoutStorage(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: false})

	ctx := context.Background()
	result, err := orch.Up(ctx, "test-project", factory)
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	if result.DiskName != "" {
		t.Errorf("DiskName = %q, want empty (no storage)", result.DiskName)
	}

	// Should NOT have CreateDisk or AttachDisk calls
	expectedCalls := []string{"CreateVM", "WaitForReady", "GetExternalIP"}
	if len(mock.calls) != len(expectedCalls) {
		t.Fatalf("got %d calls, want %d: %v", len(mock.calls), len(expectedCalls), mock.calls)
	}
	for i, call := range expectedCalls {
		if mock.calls[i] != call {
			t.Errorf("call[%d] = %q, want %q", i, mock.calls[i], call)
		}
	}
}

func TestUpAlreadyRunning(t *testing.T) {
	orch, _, factory := testSetup(t, config.StorageConfig{Enabled: false})

	// Set state to running
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusRunning
	state.VMName = "serverku-test-project"
	state.ExternalIP = "1.2.3.4"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err := orch.Up(ctx, "test-project", factory)
	if err == nil {
		t.Fatal("Up() should fail when project is already running")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestUpVMCreationFails(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	mock.createVMFunc = func(ctx context.Context, config provider.VMConfig) (*provider.VM, error) {
		return nil, fmt.Errorf("quota exceeded")
	}

	ctx := context.Background()
	_, err := orch.Up(ctx, "test-project", factory)
	if err == nil {
		t.Fatal("Up() should fail when VM creation fails")
	}

	// Verify disk was still created and state preserves disk info
	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatalf("LoadState error: %v", err)
	}
	if state.Status != config.StatusError {
		t.Errorf("state.Status = %q, want %q", state.Status, config.StatusError)
	}
	if state.DiskName == "" {
		t.Error("disk should be preserved when VM creation fails")
	}
}

func TestUpDiskAttachFails(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	mock.attachDiskFunc = func(ctx context.Context, vmID, diskID string) error {
		return fmt.Errorf("disk attach error")
	}

	ctx := context.Background()
	_, err := orch.Up(ctx, "test-project", factory)
	if err == nil {
		t.Fatal("Up() should fail when disk attach fails")
	}

	// Verify VM was destroyed but disk was kept
	hasDestroyVM := false
	for _, call := range mock.calls {
		if call == "DestroyVM" {
			hasDestroyVM = true
		}
	}
	if !hasDestroyVM {
		t.Error("should have called DestroyVM after attach failure")
	}

	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatalf("LoadState error: %v", err)
	}
	if state.DiskName == "" {
		t.Error("disk should be preserved when attach fails")
	}
}

func TestUpReusesExistingDisk(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	// Pre-save state with existing disk
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.DiskID = "existing-disk-789"
	state.DiskName = "serverku-test-project-data"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := orch.Up(ctx, "test-project", factory)
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Should reuse existing disk, not create a new one
	for _, call := range mock.calls {
		if call == "CreateDisk" {
			t.Error("should NOT call CreateDisk when disk already exists")
		}
	}

	if result.DiskID != "existing-disk-789" {
		t.Errorf("DiskID = %q, want %q", result.DiskID, "existing-disk-789")
	}
}

func TestDown(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	// Set state to running with a VM and disk
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusRunning
	state.VMID = "vm-123"
	state.VMName = "serverku-test-project"
	state.DiskID = "disk-456"
	state.DiskName = "serverku-test-project-data"
	state.ExternalIP = "1.2.3.4"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := orch.Down(ctx, "test-project", factory); err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	// Verify correct call order: detach disk then destroy VM
	expectedCalls := []string{"DetachDisk", "DestroyVM"}
	if len(mock.calls) != len(expectedCalls) {
		t.Fatalf("got %d calls, want %d: %v", len(mock.calls), len(expectedCalls), mock.calls)
	}
	for i, call := range expectedCalls {
		if mock.calls[i] != call {
			t.Errorf("call[%d] = %q, want %q", i, mock.calls[i], call)
		}
	}

	// Verify state after down
	finalState, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatalf("LoadState error: %v", err)
	}
	if finalState.Status != config.StatusStopped {
		t.Errorf("state.Status = %q, want %q", finalState.Status, config.StatusStopped)
	}
	if finalState.VMID != "" {
		t.Errorf("state.VMID = %q, want empty", finalState.VMID)
	}
	if finalState.VMName != "" {
		t.Errorf("state.VMName = %q, want empty", finalState.VMName)
	}
	if finalState.ExternalIP != "" {
		t.Errorf("state.ExternalIP = %q, want empty", finalState.ExternalIP)
	}
	// Disk should be preserved
	if finalState.DiskID != "disk-456" {
		t.Errorf("state.DiskID = %q, want %q (should be preserved)", finalState.DiskID, "disk-456")
	}
	if finalState.DiskName != "serverku-test-project-data" {
		t.Errorf("state.DiskName = %q, want %q (should be preserved)", finalState.DiskName, "serverku-test-project-data")
	}
}

func TestDownNotRunning(t *testing.T) {
	orch, _, factory := testSetup(t, config.StorageConfig{Enabled: false})

	ctx := context.Background()
	err := orch.Down(ctx, "test-project", factory)
	if err == nil {
		t.Fatal("Down() should fail when project is not running")
	}
}

func TestStatusReconcileTerminated(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: false})

	// Set state to running
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusRunning
	state.VMID = "vm-123"
	state.VMName = "serverku-test-project"
	state.ExternalIP = "1.2.3.4"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	// Mock: VM was terminated (SPOT reclaimed)
	mock.getVMStatusFunc = func(ctx context.Context, vmID string) (*provider.VMStatus, error) {
		return &provider.VMStatus{
			Name:  vmID,
			State: provider.VMStateTerminated,
		}, nil
	}

	ctx := context.Background()
	result, err := orch.Status(ctx, "test-project", factory)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	// State should be reconciled to stopped
	if result.Status != config.StatusStopped {
		t.Errorf("Status = %q, want %q", result.Status, config.StatusStopped)
	}
	if result.VMName != "" {
		t.Errorf("VMName = %q, want empty (VM was terminated)", result.VMName)
	}
}

func TestStatusReconcileRunning(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: false})

	// Set state to starting (maybe we crashed during Up)
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusStarting
	state.VMID = "vm-123"
	state.VMName = "serverku-test-project"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	// Mock: VM is actually running with an IP
	mock.getVMStatusFunc = func(ctx context.Context, vmID string) (*provider.VMStatus, error) {
		return &provider.VMStatus{
			Name:       vmID,
			State:      provider.VMStateRunning,
			ExternalIP: "5.6.7.8",
		}, nil
	}

	ctx := context.Background()
	result, err := orch.Status(ctx, "test-project", factory)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if result.Status != config.StatusRunning {
		t.Errorf("Status = %q, want %q", result.Status, config.StatusRunning)
	}
	if result.ExternalIP != "5.6.7.8" {
		t.Errorf("ExternalIP = %q, want %q", result.ExternalIP, "5.6.7.8")
	}
}

func TestDestroy(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	// Set state to running with VM and disk
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusRunning
	state.VMID = "vm-123"
	state.VMName = "serverku-test-project"
	state.DiskID = "disk-456"
	state.DiskName = "serverku-test-project-data"
	state.ExternalIP = "1.2.3.4"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := orch.Destroy(ctx, "test-project", factory); err != nil {
		t.Fatalf("Destroy() error: %v", err)
	}

	// Should have: DetachDisk, DestroyVM (from Down), then DeleteDisk
	expectedCalls := []string{"DetachDisk", "DestroyVM", "DeleteDisk"}
	if len(mock.calls) != len(expectedCalls) {
		t.Fatalf("got %d calls, want %d: %v", len(mock.calls), len(expectedCalls), mock.calls)
	}
	for i, call := range expectedCalls {
		if mock.calls[i] != call {
			t.Errorf("call[%d] = %q, want %q", i, mock.calls[i], call)
		}
	}

	// Verify config and state are deleted
	if orch.store.ProjectExists("test-project") {
		t.Error("project config should be deleted after Destroy")
	}
}

func TestDestroyStoppedWithDisk(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	// Set state to stopped with a disk (no VM)
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusStopped
	state.DiskID = "disk-456"
	state.DiskName = "serverku-test-project-data"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := orch.Destroy(ctx, "test-project", factory); err != nil {
		t.Fatalf("Destroy() error: %v", err)
	}

	// Should only delete disk (no VM to destroy)
	expectedCalls := []string{"DeleteDisk"}
	if len(mock.calls) != len(expectedCalls) {
		t.Fatalf("got %d calls, want %d: %v", len(mock.calls), len(expectedCalls), mock.calls)
	}
}

// ---------------------------------------------------------------------------
// mockProvisioner records calls for assertion in tests 6.4-6.6
// ---------------------------------------------------------------------------

type mockProvisioner struct {
	provisionFunc func(ctx context.Context, opts provisioner.ProvisionOpts) error
	teardownFunc  func(ctx context.Context, opts provisioner.TeardownOpts) error

	provisionCalls []provisioner.ProvisionOpts
	teardownCalls  []provisioner.TeardownOpts
}

func (m *mockProvisioner) Provision(ctx context.Context, opts provisioner.ProvisionOpts) error {
	m.provisionCalls = append(m.provisionCalls, opts)
	if m.provisionFunc != nil {
		return m.provisionFunc(ctx, opts)
	}
	return nil
}

func (m *mockProvisioner) Teardown(ctx context.Context, opts provisioner.TeardownOpts) error {
	m.teardownCalls = append(m.teardownCalls, opts)
	if m.teardownFunc != nil {
		return m.teardownFunc(ctx, opts)
	}
	return nil
}

// testSetupWithProvisioner is like testSetup but accepts a custom provisioner.
func testSetupWithProvisioner(t *testing.T, storageCfg config.StorageConfig, prov provisioner.Provisioner) (*Orchestrator, *mockProvider, ProviderFactory) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "serverku-orch-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	store, err := config.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure SSH keys exist so Down() can find the private key path for teardown.
	if _, _, err := store.EnsureSSHKeys(); err != nil {
		t.Fatalf("EnsureSSHKeys: %v", err)
	}

	cfg := &config.ProjectConfig{
		Name:      "test-project",
		Provider:  "gcp",
		ProjectID: "my-gcp-project",
		Region:    "us-central1",
		Zone:      "us-central1-a",
		VM: config.VMConfig{
			Size:  "e2-medium",
			Image: "ubuntu-22-04",
			Spot:  true,
		},
		Storage: storageCfg,
	}
	if err := store.SaveProject(cfg); err != nil {
		t.Fatal(err)
	}

	mock := &mockProvider{}
	factory := func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		return mock, nil
	}

	orch := New(store, prov, nil, nil)
	return orch, mock, factory
}

// ---------------------------------------------------------------------------
// 6.4 TestUpCallsProvisioner
// ---------------------------------------------------------------------------

func TestUpCallsProvisioner(t *testing.T) {
	mp := &mockProvisioner{}
	orch, _, factory := testSetupWithProvisioner(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	}, mp)

	ctx := context.Background()
	result, err := orch.Up(ctx, "test-project", factory)
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	if len(mp.provisionCalls) != 1 {
		t.Fatalf("Provision() called %d times, want 1", len(mp.provisionCalls))
	}

	opts := mp.provisionCalls[0]
	if opts.Host != result.ExternalIP {
		t.Errorf("Provision opts.Host = %q, want %q", opts.Host, result.ExternalIP)
	}
	if opts.SSHUser != "serverku" {
		t.Errorf("Provision opts.SSHUser = %q, want %q", opts.SSHUser, "serverku")
	}
	if !opts.StorageEnabled {
		t.Error("Provision opts.StorageEnabled = false, want true")
	}
	if opts.DiskName != result.DiskName {
		t.Errorf("Provision opts.DiskName = %q, want %q", opts.DiskName, result.DiskName)
	}
	if opts.MountPath != "/data" {
		t.Errorf("Provision opts.MountPath = %q, want %q", opts.MountPath, "/data")
	}
}

// ---------------------------------------------------------------------------
// 6.5 TestUpProvisioningFailureDestroysVM
// ---------------------------------------------------------------------------

func TestUpProvisioningFailureDestroysVM(t *testing.T) {
	mp := &mockProvisioner{
		provisionFunc: func(_ context.Context, _ provisioner.ProvisionOpts) error {
			return fmt.Errorf("docker install failed")
		},
	}
	orch, mock, factory := testSetupWithProvisioner(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	}, mp)

	ctx := context.Background()
	_, err := orch.Up(ctx, "test-project", factory)
	if err == nil {
		t.Fatal("Up() should fail when Provision() returns an error")
	}

	// VM must be destroyed after provisioning failure
	hasDestroyVM := false
	for _, call := range mock.calls {
		if call == "DestroyVM" {
			hasDestroyVM = true
		}
	}
	if !hasDestroyVM {
		t.Errorf("expected DestroyVM call after provisioning failure; got calls: %v", mock.calls)
	}

	// Disk must NOT be deleted -- data must survive
	for _, call := range mock.calls {
		if call == "DeleteDisk" {
			t.Errorf("unexpected DeleteDisk call after provisioning failure; disk must be preserved")
		}
	}

	// State should be error
	state, err2 := orch.store.LoadState("test-project")
	if err2 != nil {
		t.Fatalf("LoadState error: %v", err2)
	}
	if state.Status != config.StatusError {
		t.Errorf("state.Status = %q, want %q", state.Status, config.StatusError)
	}
}

// ---------------------------------------------------------------------------
// 6.6 TestDownCallsTeardown
// ---------------------------------------------------------------------------

func TestDownCallsTeardown(t *testing.T) {
	mp := &mockProvisioner{}
	orch, mock, factory := testSetupWithProvisioner(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	}, mp)

	// Set state to running with a VM and disk
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusRunning
	state.VMID = "vm-123"
	state.VMName = "serverku-test-project"
	state.DiskID = "disk-456"
	state.DiskName = "serverku-test-project-data"
	state.ExternalIP = "1.2.3.4"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := orch.Down(ctx, "test-project", factory); err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	// Teardown must have been called
	if len(mp.teardownCalls) != 1 {
		t.Fatalf("Teardown() called %d times, want 1", len(mp.teardownCalls))
	}

	// Teardown must be called before DetachDisk and DestroyVM
	// The provider mock records provider calls; teardown happens before both.
	// We verify indirectly: after Down, both provider ops must have fired.
	hasDetach := false
	hasDestroy := false
	for _, call := range mock.calls {
		switch call {
		case "DetachDisk":
			hasDetach = true
		case "DestroyVM":
			hasDestroy = true
		}
	}
	if !hasDetach {
		t.Error("expected DetachDisk to be called during Down")
	}
	if !hasDestroy {
		t.Error("expected DestroyVM to be called during Down")
	}

	opts := mp.teardownCalls[0]
	if opts.Host != "1.2.3.4" {
		t.Errorf("Teardown opts.Host = %q, want %q", opts.Host, "1.2.3.4")
	}
	if opts.SSHUser != "serverku" {
		t.Errorf("Teardown opts.SSHUser = %q, want %q", opts.SSHUser, "serverku")
	}
}

// ---------------------------------------------------------------------------
// firewallMockProvider adds the optional FirewallManager capability on top of
// mockProvider, mirroring how the GCP provider layers it onto CloudProvider.
// ---------------------------------------------------------------------------

type firewallMockProvider struct {
	*mockProvider
	ensureFirewallFunc func(ctx context.Context, projectName string) error
	deleteFirewallFunc func(ctx context.Context, projectName string) error
}

func (m *firewallMockProvider) EnsureFirewall(ctx context.Context, projectName string) error {
	m.calls = append(m.calls, "EnsureFirewall")
	if m.ensureFirewallFunc != nil {
		return m.ensureFirewallFunc(ctx, projectName)
	}
	return nil
}

func (m *firewallMockProvider) DeleteFirewall(ctx context.Context, projectName string) error {
	m.calls = append(m.calls, "DeleteFirewall")
	if m.deleteFirewallFunc != nil {
		return m.deleteFirewallFunc(ctx, projectName)
	}
	return nil
}

func firewallTestSetup(t *testing.T, storageCfg config.StorageConfig) (*Orchestrator, *firewallMockProvider, ProviderFactory) {
	t.Helper()
	orch, mock, _ := testSetup(t, storageCfg)
	fwMock := &firewallMockProvider{mockProvider: mock}
	factory := func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		return fwMock, nil
	}
	return orch, fwMock, factory
}

func indexOfCall(calls []string, name string) int {
	for i, c := range calls {
		if c == name {
			return i
		}
	}
	return -1
}

func TestUpEnsuresFirewallBeforeVM(t *testing.T) {
	orch, mock, factory := firewallTestSetup(t, config.StorageConfig{Enabled: false})

	ctx := context.Background()
	if _, err := orch.Up(ctx, "test-project", factory); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	fwIdx := indexOfCall(mock.calls, "EnsureFirewall")
	vmIdx := indexOfCall(mock.calls, "CreateVM")
	if fwIdx == -1 {
		t.Fatalf("EnsureFirewall was not called; calls: %v", mock.calls)
	}
	if vmIdx == -1 {
		t.Fatalf("CreateVM was not called; calls: %v", mock.calls)
	}
	if fwIdx > vmIdx {
		t.Errorf("EnsureFirewall (index %d) should run before CreateVM (index %d); calls: %v", fwIdx, vmIdx, mock.calls)
	}
}

func TestUpFirewallFailureAborts(t *testing.T) {
	orch, mock, factory := firewallTestSetup(t, config.StorageConfig{Enabled: false})
	mock.ensureFirewallFunc = func(ctx context.Context, projectName string) error {
		return fmt.Errorf("permission denied")
	}

	ctx := context.Background()
	if _, err := orch.Up(ctx, "test-project", factory); err == nil {
		t.Fatal("Up() should fail when EnsureFirewall fails")
	}

	if idx := indexOfCall(mock.calls, "CreateVM"); idx != -1 {
		t.Errorf("CreateVM should not be called after firewall failure; calls: %v", mock.calls)
	}

	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != config.StatusError {
		t.Errorf("state.Status = %q, want %q", state.Status, config.StatusError)
	}
}

func TestUpWithoutFirewallCapability(t *testing.T) {
	// Providers that do not implement FirewallManager (e.g. DigitalOcean) must
	// work exactly as before.
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: false})

	ctx := context.Background()
	if _, err := orch.Up(ctx, "test-project", factory); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if idx := indexOfCall(mock.calls, "EnsureFirewall"); idx != -1 {
		t.Errorf("EnsureFirewall should not appear for a provider without the capability; calls: %v", mock.calls)
	}
}

func TestDestroyDeletesFirewall(t *testing.T) {
	orch, mock, factory := firewallTestSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusStopped
	state.DiskID = "disk-456"
	state.DiskName = "serverku-test-project-data"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := orch.Destroy(ctx, "test-project", factory); err != nil {
		t.Fatalf("Destroy() error: %v", err)
	}

	diskIdx := indexOfCall(mock.calls, "DeleteDisk")
	fwIdx := indexOfCall(mock.calls, "DeleteFirewall")
	if diskIdx == -1 || fwIdx == -1 {
		t.Fatalf("expected DeleteDisk and DeleteFirewall; calls: %v", mock.calls)
	}
}

func TestDestroyFirewallFailureIsNonFatal(t *testing.T) {
	orch, mock, factory := firewallTestSetup(t, config.StorageConfig{Enabled: false})
	mock.deleteFirewallFunc = func(ctx context.Context, projectName string) error {
		return fmt.Errorf("api unavailable")
	}

	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusStopped
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := orch.Destroy(ctx, "test-project", factory); err != nil {
		t.Fatalf("Destroy() should succeed despite firewall cleanup failure, got: %v", err)
	}
	if orch.store.ProjectExists("test-project") {
		t.Error("project config should be deleted even when firewall cleanup fails")
	}
}

func TestUpPassesHeartbeatToProvisioner(t *testing.T) {
	mp := &mockProvisioner{}
	orch, _, factory := testSetupWithProvisioner(t, config.StorageConfig{Enabled: false}, mp)

	// Enable the heartbeat on the saved project config.
	cfg, err := orch.store.LoadProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Notifications.Telegram = config.TelegramConfig{
		BotToken:       "123:abc",
		ChatID:         "42",
		HeartbeatHours: 6,
	}
	if err := orch.store.SaveProject(cfg); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := orch.Up(ctx, "test-project", factory); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	if len(mp.provisionCalls) != 1 {
		t.Fatalf("Provision() called %d times, want 1", len(mp.provisionCalls))
	}
	hb := mp.provisionCalls[0].Heartbeat
	if hb.Hours != 6 || hb.BotToken != "123:abc" || hb.ChatID != "42" || hb.ProjectName != "test-project" {
		t.Errorf("unexpected heartbeat opts: %+v", hb)
	}
	// testSetup uses gcp/e2-medium/spot, which the offline table does not
	// price for spot -- but a rate must never be presented as live without a
	// PriceCatalog provider.
	if hb.RateIsLive {
		t.Errorf("rate must not be live without a PriceCatalog provider: %+v", hb)
	}
}

func TestUpNoHeartbeatWithoutConfig(t *testing.T) {
	mp := &mockProvisioner{}
	orch, _, factory := testSetupWithProvisioner(t, config.StorageConfig{Enabled: false}, mp)

	ctx := context.Background()
	if _, err := orch.Up(ctx, "test-project", factory); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if hb := mp.provisionCalls[0].Heartbeat; hb.Hours != 0 {
		t.Errorf("heartbeat should be disabled without config, got %+v", hb)
	}
}
