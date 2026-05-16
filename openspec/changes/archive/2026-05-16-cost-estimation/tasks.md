## 1. Pricing Package

- [x] 1.1 Create `internal/pricing` package.
- [x] 1.2 Add provider-aware pricing data for common GCP and DigitalOcean VM sizes.
- [x] 1.3 Add storage pricing data for GCP persistent disk and DigitalOcean block storage.
- [x] 1.4 Implement an `Estimate(cfg *config.ProjectConfig)` function returning VM hourly, disk monthly, total monthly-if-running, and missing-price indicators.

## 2. CLI Integration

- [x] 2.1 Update `serverku up` success output to show estimated VM hourly and disk monthly costs.
- [x] 2.2 Update `serverku list` output to include an estimated cost column.
- [x] 2.3 Use clear "estimate" language when prices are unavailable or approximate.

## 3. Tests

- [x] 3.1 Add unit tests for pricing estimates with known GCP and DigitalOcean sizes.
- [x] 3.2 Add unit tests for unknown sizes and disabled storage.
