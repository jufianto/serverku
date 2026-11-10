> Historical snapshot of the original proposal (2026-11-10). Not current brand policy or runtime documentation.

# Serverku source alignment

As of 2026-11-10. Audited the repository README, agent notes and lifecycle command descriptions. Read-only README and lifecycle source snapshots are included for product context; no runtime data or credentials were accessed.

## Product

Serverku is an on-demand cloud VM orchestrator for Docker Compose. GCP and DigitalOcean are implemented. The target use is side projects, previews, staging, demos and internal tools that do not need 24/7 compute.

`up` creates a VM, attaches configured persistent storage, provisions Docker, syncs the project and deploys Compose. `down` removes compute and retains configured storage; `destroy` removes cloud resources including persistent disks and snapshots. Only data actually stored on configured persistent storage should be described as retained. Remaining storage/provider costs can continue.

Prices may be live or estimated; source documents require estimates to remain marked. Brand materials must not convert a price estimate into a bill or claim a fixed savings percentage.

## Identity

No logo artwork or established palette was found in the inspected project. The owner selected a server + power direction in the session. This kit is original vector artwork, not a raster reconstruction. Deep teal, active teal, Sora lettering, supporting typography and usage rules are new proposals.

The new assets do not change CLI text, the user's terminal theme/font, providers, installation behavior, release assets or runtime logic. No cloud API was called and no resources were created or modified. Existing untracked `guides/kuma-guides/` work was left untouched.
