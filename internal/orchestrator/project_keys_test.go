package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider"
)

type keyManagerProvider struct {
	*mockProvider
	created   bool
	deleteErr error
	deletes   int
	public    string
}

func (p *keyManagerProvider) EnsureProjectSSHKey(_ context.Context, _ string, public string) (*provider.SSHKey, error) {
	p.public = public
	return &provider.SSHKey{ID: "42", Created: p.created}, nil
}

func (p *keyManagerProvider) DeleteSSHKey(_ context.Context, id, public string) error {
	if id != "42" || public != p.public {
		return fmt.Errorf("wrong key identity")
	}
	p.deletes++
	return p.deleteErr
}

func TestProjectSSHKeyLifecycleOwnership(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		created, custom, failVM, failCleanup bool
	}{
		{name: "generated and owned", created: true},
		{name: "existing account key", created: false},
		{name: "custom key", created: true, custom: true},
		{name: "VM creation fails", created: true, failVM: true},
		{name: "key deletion fails", created: true, failCleanup: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orch, mock, _ := testSetup(t, config.StorageConfig{Enabled: false})
			if tc.custom {
				customStore, err := config.NewStore(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				customKey, err := customStore.ResolveProjectSSHKey(&config.ProjectConfig{Name: "custom"}, nil, true)
				if err != nil {
					t.Fatal(err)
				}
				cfg, err := orch.store.LoadProject("test-project")
				if err != nil {
					t.Fatal(err)
				}
				cfg.SSH.PrivateKey = customKey.PrivatePath
				if err := orch.store.SaveProject(cfg); err != nil {
					t.Fatal(err)
				}
			}
			p := &keyManagerProvider{mockProvider: mock, created: tc.created}
			factory := func(context.Context, *config.ProjectConfig) (provider.CloudProvider, error) { return p, nil }
			mock.createVMFunc = func(_ context.Context, cfg provider.VMConfig) (*provider.VM, error) {
				if cfg.SSHKeyID != "42" || cfg.SSHPubKey != p.public {
					t.Error("VM did not receive the registered key")
				}
				if tc.failVM {
					return nil, fmt.Errorf("VM creation rejected")
				}
				return &provider.VM{ID: "vm-123", Name: cfg.Name}, nil
			}
			_, upErr := orch.Up(context.Background(), "test-project", factory)
			if (upErr != nil) != tc.failVM {
				t.Fatalf("Up error: %v", upErr)
			}
			state, err := orch.store.LoadState("test-project")
			if err != nil {
				t.Fatal(err)
			}
			path, public := state.SSHPrivateKeyPath, state.SSHPublicKey
			if state.SSHKeyOwned != (tc.created && !tc.custom) || state.SSHKeyManaged == tc.custom {
				t.Fatalf("incorrect key ownership: %+v", state)
			}
			if !tc.failVM {
				if err := orch.Down(context.Background(), "test-project", factory); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatal("down deleted project key")
				}
				p.created = false
				if _, err := orch.Up(context.Background(), "test-project", factory); err != nil {
					t.Fatal(err)
				}
				state, err = orch.store.LoadState("test-project")
				if err != nil {
					t.Fatal(err)
				}
				if state.SSHPublicKey != public || state.SSHKeyOwned != (tc.created && !tc.custom) {
					t.Fatal("down/up changed key or lost ownership")
				}
			}
			if tc.failCleanup {
				p.deleteErr = fmt.Errorf("key deletion forbidden")
			}
			err = orch.Destroy(context.Background(), "test-project", factory)
			if tc.failCleanup {
				if err == nil {
					t.Fatal("key deletion failure hidden")
				}
				state, err := orch.store.LoadState("test-project")
				if err != nil {
					t.Fatal(err)
				}
				if !state.SSHKeyOwned {
					t.Fatal("key ownership lost after failed cleanup")
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatal("private key removed before cloud cleanup succeeded")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantDeletes := 0
			if tc.created && !tc.custom {
				wantDeletes = 1
			}
			if p.deletes != wantDeletes {
				t.Fatalf("cloud key deletions=%d,want=%d", p.deletes, wantDeletes)
			}
			_, err = os.Stat(path)
			if tc.custom {
				if err != nil {
					t.Fatal("custom key deleted")
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal("generated project key retained")
			}
		})
	}
}

func TestDestroyRetainsGeneratedKeyReferencedAsCustom(t *testing.T) {
	orch, mock, _ := testSetup(t, config.StorageConfig{})
	p := &keyManagerProvider{mockProvider: mock, created: true}
	factory := func(context.Context, *config.ProjectConfig) (provider.CloudProvider, error) { return p, nil }
	if _, err := orch.Up(context.Background(), "test-project", factory); err != nil {
		t.Fatal(err)
	}
	state, err := orch.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	other, err := orch.store.LoadProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	other.Name = "other"
	other.SSH.PrivateKey = state.SSHPrivateKeyPath
	if err := orch.store.SaveProject(other); err != nil {
		t.Fatal(err)
	}
	if err := orch.Destroy(context.Background(), "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if p.deletes != 0 {
		t.Fatal("deleted an account key referenced by another project")
	}
	if _, err := os.Stat(state.SSHPrivateKeyPath); err != nil {
		t.Fatal("deleted a private key referenced by another project")
	}
	retained, err := orch.store.LoadState("test-project")
	if err != nil || !retained.SSHKeyOwned || retained.VMName != "" {
		t.Fatalf("retained key ownership lost: %+v, %v", retained, err)
	}
	if err := orch.store.DeleteProject("other"); err != nil {
		t.Fatal(err)
	}
	if err := orch.Destroy(context.Background(), "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if p.deletes != 1 {
		t.Fatal("unreferenced retained key could not be cleaned up")
	}
}
