## ADDED Requirements

### Requirement: Router Configuration
The project configuration SHALL support a `Router` struct containing `Enabled` (bool) and a list of `Domains`. Each `Domain` SHALL define a `Domain` name, target `Service` name, and `Upstream` address.

#### Scenario: Router configured
- **WHEN** a user defines the router block in their project YAML
- **THEN** the configuration parser SHALL successfully load these settings into the `ProjectConfig`.

### Requirement: Caddyku Installation
When routing is enabled, the provisioner SHALL download and install the `caddyku` binary on the remote VM.

#### Scenario: Download and install
- **WHEN** `Router.Enabled` is true during the provisioning phase
- **THEN** the `SSHProvisioner` SHALL execute a `curl` command on the VM to download the `caddyku` Linux amd64 release tarball, extract it, and move the binary to `/usr/local/bin/caddyku`.

### Requirement: Caddyku Initialization
When routing is enabled, the provisioner SHALL initialize the global proxy on the remote VM.

#### Scenario: Initialize global proxy
- **WHEN** `caddyku` has been installed on the VM
- **THEN** the `SSHProvisioner` SHALL run `caddyku init` and then start the resulting `caddy-proxy` stack using `docker compose up -d` in the `~/projects/caddy-proxy` directory.

### Requirement: Application Routing Integration
When routing is enabled, the provisioner SHALL configure the application to use the global proxy before starting the application containers.

#### Scenario: Configure application routes
- **WHEN** the project's docker-compose file has been synced to the deployment directory and `caddyku` is initialized
- **THEN** for each domain configured in the `Router` block, the `SSHProvisioner` SHALL run `caddyku init-app --service <Service> --domain <Domain> --upstream <Upstream>` within the deployment directory, ensuring the app is connected to `caddy-net` and the routing rules are applied.
