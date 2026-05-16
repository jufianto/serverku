## Context

`serverku` helps save money by making it easy to tear down VMs, but it doesn't currently tell the user how much money they are actually spending. By providing rough cost estimates directly in the CLI, users can make better decisions about when to run or stop their projects.

## Goals / Non-Goals

**Goals:**
- Show estimated hourly cost when a project is running.
- Show estimated monthly cost of the persistent disk (which runs 24/7 even if the VM is stopped).
- Display these estimates in the output of `serverku up` and `serverku list`.

**Non-Goals:**
- We are not building a real-time billing API integration with the cloud providers. We will use hardcoded approximations for the most common instance types.
- We will not account for network egress costs, which are highly variable.

## Decisions

**1. Pricing Data Structure:**
We will create a simple `internal/pricing` package with maps defining hourly rates for common VM sizes (e.g., `s-1vcpu-1gb` -> $0.0089/hr) and monthly rates per GB for block storage.
- *Rationale*: Hardcoding is much simpler and faster than querying pricing APIs, and approximations are sufficient for the user's needs.

## Risks / Trade-offs

- **[Risk] Prices change** → *Mitigation*: We must clearly label the output as an "Estimate" and periodically update the hardcoded maps in future releases.
