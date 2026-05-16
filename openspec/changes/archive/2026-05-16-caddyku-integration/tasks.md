## 1. Configuration Setup

- [x] 1.1 Add `RouterConfig` and `DomainConfig` structs to `internal/config/project.go`.
- [x] 1.2 Add `Router` field to `ProjectConfig`.
- [x] 1.3 Add `RouterEnabled` (bool) and `Domains` (slice of domain configs) fields to `ProvisionOpts` in `internal/provisioner/provisioner.go`.
- [x] 1.4 Update `cmd/serverku/lifecycle.go` to map `cfg.Router` to `ProvisionOpts`.

## 2. Provisioner Scripts

- [x] 2.1 Add a `installCaddykuScript()` function returning the bash command to download and install `caddyku` to `/usr/local/bin/caddyku`.
- [x] 2.2 Add an `initCaddyProxyScript()` function returning the bash command to run `caddyku init` and start the proxy stack in `~/projects/caddy-proxy`.
- [x] 2.3 Add a `configureAppDomainsScript(domains, composeDir)` function that loops over domains and returns the combined bash command to execute `caddyku init-app` for each domain within the `composeDir`.

## 3. Provisioning Logic Integration

- [x] 3.1 In `internal/provisioner/provisioner.go`'s `Provision` method, after Docker is installed and directories are set up, check if `opts.RouterEnabled` is true.
- [x] 3.2 If true, execute `installCaddykuScript()`.
- [x] 3.3 Execute `initCaddyProxyScript()`.
- [x] 3.4 Execute `configureAppDomainsScript()`.
- [x] 3.5 Allow the rest of the provisioning (like `docker compose up -d`) to continue normally.