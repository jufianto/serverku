## ADDED Requirements

### Requirement: DigitalOcean Provider Implementation
The system SHALL provide a `internal/provider/digitalocean` package that implements the `internal/provider.CloudProvider` interface using the `github.com/digitalocean/godo` client.

#### Scenario: Droplet Creation
- **WHEN** the orchestrator calls `CreateVM` with a valid configuration
- **THEN** the provider SHALL create a DigitalOcean Droplet using the specified region, size, and the `serverku_rsa.pub` SSH key.

#### Scenario: Spot Instance Request
- **WHEN** the project configuration requests a Spot instance (`VM.Spot == true`)
- **THEN** the provider SHALL return an error because DigitalOcean does not support Spot instances in the same manner.

#### Scenario: Storage Volume Creation
- **WHEN** the orchestrator calls `CreateDisk` with storage enabled
- **THEN** the provider SHALL create a DigitalOcean Block Storage volume in the correct region.

#### Scenario: Droplet Deletion
- **WHEN** the orchestrator calls `DeleteVM`
- **THEN** the provider SHALL destroy the DigitalOcean Droplet via the API.

#### Scenario: Client Authentication
- **WHEN** the provider is instantiated
- **THEN** it SHALL use the `DIGITALOCEAN_TOKEN` environment variable to authenticate the `godo` client.

### Requirement: Provider Factory Integration
The system SHALL support selecting the DigitalOcean provider via the `ProviderFactory` in `cmd/serverku/lifecycle.go`.

#### Scenario: DigitalOcean Configured
- **WHEN** the project config has `Provider: "digitalocean"`
- **THEN** `newProviderFactory` SHALL return a valid DigitalOcean provider instance without error.
