package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	restoreDiskFunc   func(ctx context.Context, config provider.DiskConfig, snapshot string) (*provider.Disk, error)

	listSnapshotsFunc  func(context.Context, provider.SnapshotQuery) ([]provider.Snapshot, error)
	deleteSnapshotFunc func(context.Context, provider.Snapshot) error
	deletedSnapshots   map[string]bool
	// Track calls for assertions
	calls []string
}

func (m *mockProvider) CreateDiskFromSnapshot(ctx context.Context, config provider.DiskConfig, snapshot string) (*provider.Disk, error) {
	m.calls = append(m.calls, "CreateDiskFromSnapshot")
	if m.restoreDiskFunc != nil {
		return m.restoreDiskFunc(ctx, config, snapshot)
	}
	return &provider.Disk{ID: "restored-disk-1", Name: config.Name, Zone: config.Zone, SizeGB: config.SizeGB, Provider: "mock"}, nil
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

func (m *mockProvider) ListProjectSnapshots(ctx context.Context, q provider.SnapshotQuery) ([]provider.Snapshot, error) {
	m.calls = append(m.calls, "ListProjectSnapshots")
	if m.listSnapshotsFunc != nil {
		return m.listSnapshotsFunc(ctx, q)
	}
	var result []provider.Snapshot
	for _, snap := range q.Tracked {
		if !m.deletedSnapshots[snap.ID] {
			result = append(result, snap)
		}
	}
	return result, nil
}
func (m *mockProvider) DeleteSnapshot(ctx context.Context, snap provider.Snapshot) error {
	m.calls = append(m.calls, "DeleteSnapshot")
	if m.deleteSnapshotFunc != nil {
		if err := m.deleteSnapshotFunc(ctx, snap); err != nil {
			return err
		}
	}
	if m.deletedSnapshots == nil {
		m.deletedSnapshots = map[string]bool{}
	}
	m.deletedSnapshots[snap.ID] = true
	return nil
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
	mock.attachDiskFunc = func(_ context.Context, vmName, diskName string) error {
		if vmName != "serverku-test-project" || diskName != "serverku-test-project-data" {
			t.Errorf("name-based attachment got %q, %q", vmName, diskName)
		}
		return nil
	}

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
	if state.VMID != "" || state.VMName != "" || state.ExternalIP != "" || state.StoppedAt == nil {
		t.Fatalf("deleted VM remains tracked: %+v", state)
	}
}

type mockIDAttacher struct {
	*mockProvider
	attachedID string
}

func (m *mockIDAttacher) AttachDiskByID(_ context.Context, _ string, diskID string) error {
	m.attachedID = diskID
	return nil
}

func TestUpUsesSavedDiskIDForAttachment(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reuse=%t", reuse), func(t *testing.T) {
			orch, mock, _ := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})
			wantID := "disk-456"
			if reuse {
				state := config.NewState("test-project", "digitalocean", "sgp1", "")
				state.DiskID = "existing-volume-id"
				state.DiskName = "serverku-test-project-data"
				wantID = state.DiskID
				if err := orch.store.SaveState(state); err != nil {
					t.Fatal(err)
				}
			}
			attacher := &mockIDAttacher{mockProvider: mock}
			factory := func(context.Context, *config.ProjectConfig) (provider.CloudProvider, error) { return attacher, nil }
			if _, err := orch.Up(context.Background(), "test-project", factory); err != nil {
				t.Fatal(err)
			}
			if attacher.attachedID != wantID {
				t.Fatalf("attached ID = %q, want %q", attacher.attachedID, wantID)
			}
			for _, call := range mock.calls {
				if call == "AttachDisk" || (reuse && call == "CreateDisk") {
					t.Fatalf("unexpected call %s", call)
				}
			}
		})
	}
}

func TestUpFallsBackToDiskNameWithoutSavedID(t *testing.T) {
	orch, mock, _ := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})
	state := config.NewState("test-project", "digitalocean", "sgp1", "")
	state.DiskName = "serverku-test-project-data"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}
	var usedName bool
	mock.attachDiskFunc = func(_ context.Context, _, diskName string) error {
		usedName = diskName == state.DiskName
		return nil
	}
	attacher := &mockIDAttacher{mockProvider: mock}
	factory := func(context.Context, *config.ProjectConfig) (provider.CloudProvider, error) { return attacher, nil }
	if _, err := orch.Up(context.Background(), "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if !usedName || attacher.attachedID != "" {
		t.Fatal("legacy state without a disk ID must use name-based attachment")
	}
}

func TestUpDiskAttachFailureReportsCleanupFailure(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})
	mock.attachDiskFunc = func(context.Context, string, string) error { return fmt.Errorf("attach rejected") }
	mock.destroyVMFunc = func(context.Context, string) error { return fmt.Errorf("deletion forbidden") }
	_, err := orch.Up(context.Background(), "test-project", factory)
	if err == nil || !strings.Contains(err.Error(), "attach rejected") || !strings.Contains(err.Error(), "automatic VM cleanup failed") || !strings.Contains(err.Error(), "deletion forbidden") {
		t.Fatalf("missing attachment or cleanup error: %v", err)
	}
	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != config.StatusError || state.VMID == "" || state.VMName == "" || state.DiskID == "" {
		t.Fatalf("resources must remain tracked after failed cleanup: %+v", state)
	}
	if !strings.Contains(state.ErrorMsg, "deletion forbidden") {
		t.Fatalf("cleanup failure not saved: %s", state.ErrorMsg)
	}
	for _, call := range mock.calls {
		if call == "DeleteDisk" {
			t.Fatal("persistent disk deleted")
		}
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
	expectedCalls := []string{"DetachDisk", "DestroyVM", "ListProjectSnapshots", "DeleteDisk"}
	if len(mock.calls) != len(expectedCalls) {
		t.Fatalf("got %d calls, want %d: %v", len(mock.calls), len(expectedCalls), mock.calls)
	}
	for i, call := range expectedCalls {
		if mock.calls[i] != call {
			t.Errorf("call[%d] = %q, want %q", i, mock.calls[i], call)
		}
	}

	// The project config must be PRESERVED -- destroy removes cloud resources,
	// not the project definition, so it can be brought back with `up`.
	if !orch.store.ProjectExists("test-project") {
		t.Error("project config should be preserved after Destroy")
	}
	// A minimal destroyed record preserves the last lifecycle action.
	st, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatalf("LoadState after Destroy: %v", err)
	}
	if st.Status != config.StatusDestroyed || st.VMName != "" || st.DiskName != "" {
		t.Errorf("state should be reset after Destroy, got %+v", st)
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
	expectedCalls := []string{"ListProjectSnapshots", "DeleteDisk"}
	if len(mock.calls) != len(expectedCalls) {
		t.Fatalf("got %d calls, want %d: %v", len(mock.calls), len(expectedCalls), mock.calls)
	}
}

// ---------------------------------------------------------------------------
// mockProvisioner records calls for assertion in tests 6.4-6.6
// ---------------------------------------------------------------------------

type mockProvisioner struct {
	provisionFunc func(ctx context.Context, opts provisioner.ProvisionOpts) error
	deployFunc    func(ctx context.Context, opts provisioner.DeployOpts) error
	teardownFunc  func(ctx context.Context, opts provisioner.TeardownOpts) error

	provisionCalls []provisioner.ProvisionOpts
	deployCalls    []provisioner.DeployOpts
	teardownCalls  []provisioner.TeardownOpts
}

func (m *mockProvisioner) Deploy(ctx context.Context, opts provisioner.DeployOpts) error {
	m.deployCalls = append(m.deployCalls, opts)
	if m.deployFunc != nil {
		return m.deployFunc(ctx, opts)
	}
	return nil
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
	if state.VMID != "" || state.VMName != "" || state.ExternalIP != "" {
		t.Errorf("deleted VM remains tracked: %+v", state)
	}
	if state.DiskID == "" || state.DiskName == "" {
		t.Error("persistent disk tracking must survive VM cleanup")
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

func TestDestroyFirewallFailureRetainsStateForRetry(t *testing.T) {
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
	if err := orch.Destroy(ctx, "test-project", factory); err == nil {
		t.Fatal("destroy must report failed firewall cleanup")
	}
	if _, err := os.Stat(orch.store.StateDir() + "/test-project.json"); err != nil {
		t.Fatal("cleanup state must survive a failed destroy", err)
	}
	if !orch.store.ProjectExists("test-project") {
		t.Error("project config should be preserved even when firewall cleanup fails")
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

// ---------------------------------------------------------------------------
// Deploy
// ---------------------------------------------------------------------------

// deployTestSetup returns an orchestrator whose project is in a given running
// state, with a mock provisioner to capture Deploy calls.
func deployTestSetup(t *testing.T, running bool) (*Orchestrator, *mockProvisioner) {
	t.Helper()
	mp := &mockProvisioner{}
	orch, _, _ := testSetupWithProvisioner(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	}, mp)

	// Deploy reads the SSH key path; make sure the key exists.
	if _, _, err := orch.store.EnsureSSHKeys(); err != nil {
		t.Fatal(err)
	}

	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	if running {
		state.Status = config.StatusRunning
		state.VMName = "serverku-test-project"
		state.ExternalIP = "1.2.3.4"
	} else {
		state.Status = config.StatusStopped
	}
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}
	return orch, mp
}

func TestDeploy(t *testing.T) {
	orch, mp := deployTestSetup(t, true)

	// Give the project a sync dir so DeployOpts carries it.
	cfg, err := orch.store.LoadProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	cfg.SyncDir = "/tmp/some-project"
	if err := orch.store.SaveProject(cfg); err != nil {
		t.Fatal(err)
	}

	if err := orch.Deploy(context.Background(), "test-project"); err != nil {
		t.Fatalf("Deploy() error: %v", err)
	}

	if len(mp.deployCalls) != 1 {
		t.Fatalf("Deploy() called %d times, want 1", len(mp.deployCalls))
	}
	opts := mp.deployCalls[0]
	if opts.Host != "1.2.3.4" {
		t.Errorf("opts.Host = %q, want 1.2.3.4", opts.Host)
	}
	if opts.SSHUser != "serverku" {
		t.Errorf("opts.SSHUser = %q, want serverku", opts.SSHUser)
	}
	if !opts.StorageEnabled || opts.MountPath != "/data" {
		t.Errorf("storage opts not carried: %+v", opts)
	}
	if opts.SyncDir != "/tmp/some-project" {
		t.Errorf("opts.SyncDir = %q", opts.SyncDir)
	}
}

func TestDeployNotRunning(t *testing.T) {
	orch, mp := deployTestSetup(t, false)

	err := orch.Deploy(context.Background(), "test-project")
	if err == nil {
		t.Fatal("Deploy() should fail when the project is not running")
	}
	if len(mp.deployCalls) != 0 {
		t.Errorf("provisioner.Deploy should not be called; got %d calls", len(mp.deployCalls))
	}
}

func TestDeployFailureLeavesStateRunning(t *testing.T) {
	orch, mp := deployTestSetup(t, true)
	mp.deployFunc = func(_ context.Context, _ provisioner.DeployOpts) error {
		return fmt.Errorf("rsync exploded")
	}

	if err := orch.Deploy(context.Background(), "test-project"); err == nil {
		t.Fatal("Deploy() should propagate provisioner failure")
	}

	// A failed deploy must not touch the running VM or its state.
	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != config.StatusRunning || state.ExternalIP != "1.2.3.4" {
		t.Errorf("state must stay running after failed deploy, got %s/%s", state.Status, state.ExternalIP)
	}
}

// ---------------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------------

func TestRestore(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	})

	// Stopped project with an existing (old) disk.
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusStopped
	state.DiskName = "serverku-test-project-data"
	state.DiskID = "old-disk"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	res, err := orch.Restore(context.Background(), "test-project", "snap-abc", "serverku-test-project-data-restored", false, factory)
	if err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if res.NewDiskName != "serverku-test-project-data-restored" {
		t.Errorf("NewDiskName = %q", res.NewDiskName)
	}
	if res.OldDeleted {
		t.Error("old disk should be kept by default")
	}
	if !containsStr(mock.calls, "CreateDiskFromSnapshot") {
		t.Errorf("expected CreateDiskFromSnapshot; calls: %v", mock.calls)
	}
	if containsStr(mock.calls, "DeleteDisk") {
		t.Errorf("DeleteDisk must not be called without --delete-old; calls: %v", mock.calls)
	}

	// State now points at the restored disk.
	got, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if got.DiskName != "serverku-test-project-data-restored" || got.DiskID != "restored-disk-1" {
		t.Errorf("state not repointed: %s / %s", got.DiskName, got.DiskID)
	}
}

func TestRestoreDeleteOld(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})

	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusStopped
	state.DiskName = "old-disk-name"
	state.DiskID = "old-disk"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	res, err := orch.Restore(context.Background(), "test-project", "snap-abc", "new-disk", true, factory)
	if err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if !res.OldDeleted {
		t.Error("expected old disk to be deleted with --delete-old")
	}
	if !containsStr(mock.calls, "DeleteDisk") {
		t.Errorf("expected DeleteDisk; calls: %v", mock.calls)
	}
}

func TestRestoreRejectsRunningProject(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})

	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.Status = config.StatusRunning
	state.VMName = "serverku-test-project"
	state.DiskName = "d"
	if err := orch.store.SaveState(state); err != nil {
		t.Fatal(err)
	}

	if _, err := orch.Restore(context.Background(), "test-project", "snap", "new", false, factory); err == nil {
		t.Fatal("Restore() should reject a running project")
	}
	if containsStr(mock.calls, "CreateDiskFromSnapshot") {
		t.Error("no disk should be created for a running project")
	}
}

func TestRestoreRequiresStorage(t *testing.T) {
	orch, _, factory := testSetup(t, config.StorageConfig{Enabled: false})

	if _, err := orch.Restore(context.Background(), "test-project", "snap", "new", false, factory); err == nil {
		t.Fatal("Restore() should fail when storage is disabled")
	}
}

// ---------------------------------------------------------------------------
// Preflight
// ---------------------------------------------------------------------------

// credMockProvider adds the CredentialValidator capability to mockProvider.
type credMockProvider struct {
	*mockProvider
	credErr error
}

func (m *credMockProvider) ValidateCredentials(ctx context.Context) error {
	m.calls = append(m.calls, "ValidateCredentials")
	return m.credErr
}

func preflightSetup(t *testing.T, mutate func(*config.ProjectConfig)) (*Orchestrator, ProviderFactory, *credMockProvider) {
	t.Helper()
	orch, mock, _ := testSetup(t, config.StorageConfig{Enabled: false})
	cfg, err := orch.store.LoadProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(cfg)
		if err := orch.store.SaveProject(cfg); err != nil {
			t.Fatal(err)
		}
	}
	cm := &credMockProvider{mockProvider: mock}
	factory := func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		return cm, nil
	}
	return orch, factory, cm
}

func findCheck(res *PreflightResult, name string) (PreflightCheck, bool) {
	for _, c := range res.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return PreflightCheck{}, false
}

func TestPreflightAllPass(t *testing.T) {
	dir := t.TempDir()
	compose := dir + "/docker-compose.yml"
	if err := os.WriteFile(compose, []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orch, factory, cm := preflightSetup(t, func(cfg *config.ProjectConfig) {
		cfg.ComposeFile = compose
		cfg.SyncDir = dir
	})

	res, err := orch.Preflight(context.Background(), "test-project", factory)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if !res.AllOK() {
		t.Errorf("expected all checks to pass, got: %+v", res.Checks)
	}
	if !containsStr(cm.calls, "ValidateCredentials") {
		t.Error("expected credentials to be checked via ValidateCredentials")
	}
	for _, name := range []string{"config", "compose file", "sync dir", "ssh keys", "credentials"} {
		if c, ok := findCheck(res, name); !ok || !c.OK {
			t.Errorf("check %q missing or failed: %+v", name, c)
		}
	}
}

func TestPreflightMissingComposeFile(t *testing.T) {
	orch, factory, _ := preflightSetup(t, func(cfg *config.ProjectConfig) {
		cfg.ComposeFile = "/does/not/exist/docker-compose.yml"
	})

	res, err := orch.Preflight(context.Background(), "test-project", factory)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if res.AllOK() {
		t.Fatal("expected preflight to fail on missing compose file")
	}
	if c, _ := findCheck(res, "compose file"); c.OK {
		t.Error("compose file check should have failed")
	}
}

func TestPreflightBadCredentials(t *testing.T) {
	orch, _, _ := preflightSetup(t, nil)
	cm := &credMockProvider{mockProvider: &mockProvider{}, credErr: fmt.Errorf("401 unauthorized")}
	factory := func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		return cm, nil
	}

	res, err := orch.Preflight(context.Background(), "test-project", factory)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if c, _ := findCheck(res, "credentials"); c.OK {
		t.Error("credentials check should have failed")
	}
	if res.AllOK() {
		t.Error("preflight should fail overall on bad credentials")
	}
}

func TestPreflightDNSWithoutSupport(t *testing.T) {
	// mockProvider does not implement DNSManager, so dns.enabled must fail.
	orch, factory, _ := preflightSetup(t, func(cfg *config.ProjectConfig) {
		cfg.DNS.Enabled = true
		cfg.Router.Domains = []config.DomainConfig{{Domain: "x.example.com", Service: "web", Upstream: "web:80"}}
	})

	res, err := orch.Preflight(context.Background(), "test-project", factory)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if c, ok := findCheck(res, "dns"); !ok || c.OK {
		t.Errorf("dns check should be present and failing: %+v", c)
	}
}

func TestUpProvisioningFailureReportsCleanupFailure(t *testing.T) {
	mp := &mockProvisioner{provisionFunc: func(context.Context, provisioner.ProvisionOpts) error {
		return fmt.Errorf("compose rejected")
	}}
	orch, mock, factory := testSetupWithProvisioner(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"}, mp)
	mock.destroyVMFunc = func(context.Context, string) error { return fmt.Errorf("deletion forbidden") }
	_, err := orch.Up(context.Background(), "test-project", factory)
	if err == nil || !strings.Contains(err.Error(), "compose rejected") || !strings.Contains(err.Error(), "automatic VM cleanup failed") || !strings.Contains(err.Error(), "deletion forbidden") {
		t.Fatalf("missing provisioning or cleanup failure: %v", err)
	}
	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != config.StatusError || state.VMID == "" || state.VMName == "" || state.ExternalIP == "" {
		t.Fatalf("VM must remain tracked after failed deletion: %+v", state)
	}
	if !strings.Contains(state.ErrorMsg, "deletion forbidden") {
		t.Errorf("cleanup error not saved: %s", state.ErrorMsg)
	}
	for _, call := range mock.calls {
		if call == "DeleteDisk" {
			t.Fatal("disk deleted on failed provisioning")
		}
	}
}
