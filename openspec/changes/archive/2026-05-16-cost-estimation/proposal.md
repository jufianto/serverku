## Why

One of the primary benefits of serverku is saving money by stopping unused servers, but users have no visibility into what their running resources actually cost. Providing simple cost estimations directly in the CLI will help users manage their cloud budgets effectively.

## What Changes

- Update `serverku up` and `serverku list` output to display estimated hourly/monthly costs based on the VM size and storage size configured.
- We will add a small internal package `internal/pricing` containing hardcoded or simple lookup maps for common GCP and DigitalOcean instance and disk prices.

## Capabilities

### New Capabilities
- `cost-estimation`: Display estimated infrastructure costs to the user in CLI outputs.

### Modified Capabilities

## Impact

- **Code:** Modifies `cmd/serverku/list.go` and `cmd/serverku/lifecycle.go` (up cmd). Adds `internal/pricing`.
- **Dependencies:** None.
- **Systems:** Provides financial visibility to end users.
