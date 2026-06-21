package orchestrator

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider"
)

// fakeHookRunner records hook invocations and can be made to fail a given phase.
type fakeHookRunner struct {
	phases  []string            // phases invoked that had at least one command
	failOn  string              // phase to fail, "" to never fail
	byPhase map[string][]string // commands seen per phase
}

func newFakeHookRunner() *fakeHookRunner {
	return &fakeHookRunner{byPhase: map[string][]string{}}
}

func (f *fakeHookRunner) Run(ctx context.Context, phase string, commands []string, workDir string, env []string) error {
	if len(commands) > 0 {
		f.phases = append(f.phases, phase)
		f.byPhase[phase] = commands
	}
	if phase == f.failOn {
		return errors.New("boom")
	}
	return nil
}

// hookTestSetup builds an orchestrator wired with the given hook runner and a
// project config carrying the given hooks, plus a mock provider + factory.
func hookTestSetup(t *testing.T, runner HookRunner, hooks config.HooksConfig) (*Orchestrator, *mockProvider, ProviderFactory) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "serverku-hooks-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	store, err := config.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.ProjectConfig{
		Name:     "hook-project",
		Provider: "digitalocean",
		Region:   "sgp1",
		VM:       config.VMConfig{Size: "s-1vcpu-1gb", Image: "ubuntu-22-04"},
		Storage:  config.StorageConfig{Enabled: false},
		Hooks:    hooks,
	}
	if err := store.SaveProject(cfg); err != nil {
		t.Fatal(err)
	}

	mock := &mockProvider{}
	factory := func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		return mock, nil
	}
	return New(store, nil, nil, runner), mock, factory
}

func TestUp_RunsPreUpBeforeCreateVMAndPostUpAfter(t *testing.T) {
	runner := newFakeHookRunner()
	orch, mock, factory := hookTestSetup(t, runner, config.HooksConfig{
		PreUp:  []string{"echo build"},
		PostUp: []string{"echo notify"},
	})

	if _, err := orch.Up(context.Background(), "hook-project", factory); err != nil {
		t.Fatalf("Up: %v", err)
	}

	if idxOfStr(runner.phases, "pre_up") != 0 {
		t.Errorf("expected pre_up first, phases: %v", runner.phases)
	}
	if !containsStr(runner.phases, "post_up") {
		t.Errorf("expected post_up to run, phases: %v", runner.phases)
	}
	// pre_up must run before any cloud resource is created.
	if !containsStr(mock.calls, "CreateVM") {
		t.Fatal("expected CreateVM to be called")
	}
}

func TestUp_PreUpFailureAbortsBeforeAnyCloudCall(t *testing.T) {
	runner := newFakeHookRunner()
	runner.failOn = "pre_up"
	orch, mock, factory := hookTestSetup(t, runner, config.HooksConfig{PreUp: []string{"exit 1"}})

	if _, err := orch.Up(context.Background(), "hook-project", factory); err == nil {
		t.Fatal("expected Up to fail when pre_up fails")
	}
	if len(mock.calls) != 0 {
		t.Errorf("expected no cloud calls after pre_up failure, got: %v", mock.calls)
	}
}

func TestUp_PostUpFailureIsNonFatal(t *testing.T) {
	runner := newFakeHookRunner()
	runner.failOn = "post_up"
	orch, _, factory := hookTestSetup(t, runner, config.HooksConfig{PostUp: []string{"exit 1"}})

	if _, err := orch.Up(context.Background(), "hook-project", factory); err != nil {
		t.Errorf("post_up failure should be non-fatal, got: %v", err)
	}
}

func TestDown_RunsDownHooks(t *testing.T) {
	runner := newFakeHookRunner()
	orch, _, factory := hookTestSetup(t, runner, config.HooksConfig{
		PreDown:  []string{"echo drain"},
		PostDown: []string{"echo cleanup"},
	})

	// Bring it up first (no up hooks configured), then down.
	if _, err := orch.Up(context.Background(), "hook-project", factory); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if err := orch.Down(context.Background(), "hook-project", factory); err != nil {
		t.Fatalf("Down: %v", err)
	}

	if !containsStr(runner.phases, "pre_down") || !containsStr(runner.phases, "post_down") {
		t.Errorf("expected pre_down and post_down, phases: %v", runner.phases)
	}
}

func TestDestroy_FiresDestroyHooksNotDownHooks(t *testing.T) {
	runner := newFakeHookRunner()
	orch, _, factory := hookTestSetup(t, runner, config.HooksConfig{
		PreDown:     []string{"echo should-not-run"},
		PostDown:    []string{"echo should-not-run"},
		PreDestroy:  []string{"echo final-backup"},
		PostDestroy: []string{"echo cleanup"},
	})

	// Bring it up so destroy has a VM to tear down internally.
	if _, err := orch.Up(context.Background(), "hook-project", factory); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if err := orch.Destroy(context.Background(), "hook-project", factory); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	if !containsStr(runner.phases, "pre_destroy") || !containsStr(runner.phases, "post_destroy") {
		t.Errorf("expected destroy hooks, phases: %v", runner.phases)
	}
	if containsStr(runner.phases, "pre_down") || containsStr(runner.phases, "post_down") {
		t.Errorf("down hooks must NOT fire during destroy, phases: %v", runner.phases)
	}
}

func TestDestroy_PreDestroyFailureAborts(t *testing.T) {
	runner := newFakeHookRunner()
	runner.failOn = "pre_destroy"
	orch, _, factory := hookTestSetup(t, runner, config.HooksConfig{PreDestroy: []string{"exit 1"}})

	// No VM up; pre_destroy should still gate the operation.
	if err := orch.Destroy(context.Background(), "hook-project", factory); err == nil {
		t.Fatal("expected Destroy to fail when pre_destroy fails")
	}
	// Config must still exist since destroy was aborted before deletion.
	if _, err := orch.store.LoadProject("hook-project"); err != nil {
		t.Errorf("project config should survive an aborted destroy: %v", err)
	}
}

func idxOfStr(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func containsStr(s []string, v string) bool {
	return idxOfStr(s, v) >= 0
}
