## 1. Setup

- [x] 1.1 Add `github.com/digitalocean/godo` to project dependencies (`go get`).
- [x] 1.2 Create `internal/provider/digitalocean` directory.
- [x] 1.3 Create `internal/provider/digitalocean/digitalocean.go` file with the `Provider` struct scaffold.

## 2. Core Provider Implementation

- [x] 2.1 Implement `New(ctx context.Context)` factory that reads `DIGITALOCEAN_TOKEN` and initializes the `godo` client.
- [x] 2.2 Implement `CreateVM` method (create Droplet using project name, region, size, and SSH key).
- [x] 2.3 Implement spot instance validation (return error if `cfg.VM.Spot` is true).
- [x] 2.4 Implement `GetVM` method (fetch Droplet by name/ID to get external IP).
- [x] 2.5 Implement `StopVM` method (shutdown Droplet).
- [x] 2.6 Implement `DeleteVM` method (destroy Droplet).

## 3. Storage Implementation

- [x] 3.1 Implement `CreateDisk` method (create Block Storage volume).
- [x] 3.2 Implement `AttachDisk` method (attach volume to Droplet, wait for completion).
- [x] 3.3 Implement `DetachDisk` method (detach volume from Droplet).
- [x] 3.4 Implement `DeleteDisk` method (destroy volume).

## 4. Integration

- [x] 4.1 Update `cmd/serverku/lifecycle.go` `newProviderFactory` to instantiate the DigitalOcean provider when configured.
- [x] 4.2 Write basic unit tests for the `digitalocean` provider logic where possible without integration mocking, or test manually.
