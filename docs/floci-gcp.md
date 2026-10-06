# Local GCP testing with Floci

Start the emulator from the repository root (requires Docker Compose):

```bash
docker compose -f compose.floci.yml up -d
docker compose -f compose.floci.yml logs --tail 20
```

The API is available at `http://127.0.0.1:4588`. The configuration pins the
verified Floci GCP nightly-10042026 image, uses disposable memory storage, and binds only
to localhost. It needs no GCP credentials. This setup does not mount the
Docker socket; Docker-backed services such as Cloud Run and Cloud SQL need
additional configuration.

## Verified coverage and limits

On 2026-10-05, the pinned image passed a Cloud Storage smoke test: create a
bucket, upload an object, download and compare its contents, then delete
the object and bucket.

The stable `latest` image tested earlier was 0.9.0 and lacked Compute Engine.
The Compose setup now pins the nightly image published on 2026-10-04, built
from upstream commit `1bb0845c38b163ed28d58b5ff930f25cf90ab634`. Startup reports
`nightly-10042026`, includes `compute`, and the regions API responds successfully.

A local probe through serverku's real Go provider verified:

- Compute regions lookup.
- Persistent disk creation, returned ID/name/size, and deletion.
- Firewall creation, repeated creation (idempotency), and deletion, using a
  synthetic custom-mode network named `default` seeded by the probe.

The probe also found two compatibility gaps; full lifecycle testing is **not**
yet supported:

- `CreateVM` returns HTTP 400 `Missing or invalid subnetwork`. Serverku uses
  GCP's default network without specifying a subnet; Floci requires an explicit
  subnet and does not emulate auto-mode VPCs. The probe used `e2-standard-2`.
- `SnapshotDisk` returns HTTP 501 `Action createSnapshot is not implemented
  for disks`. Serverku calls the disk `createSnapshot` action; Floci documents
  snapshot creation through the global snapshots resource instead.

Future work must address those differences without changing real-GCP behavior
merely to accommodate emulator limitations. The [upstream Compute documentation](https://github.com/floci-io/floci-gcp/blob/main/docs/services/compute.md)
also excludes guest execution and traffic forwarding: SSH, rsync, Docker
installation, disk mounting, and application startup require separate testing.
Cloud DNS and Billing coverage must be checked separately.

## Run the supported provider integration test

With the Compose emulator running:

```bash
go test -tags=floci ./internal/provider/gcp -run '^TestFlociDiskAndFirewallLifecycle$' -v -count=1
```

This opt-in test connects explicitly to `http://127.0.0.1:4588/compute/v1/`
without authentication. It creates a uniquely named synthetic project, verifies
disk and firewall operations, and removes the resources. Normal `go test ./...`
does not include it. It does not test VM provisioning or snapshots.

Serverku's normal GCP constructor uses Application Default Credentials and
real API endpoints. Setting Floci's storage or gcloud environment variables
does not redirect serverku's Go Compute client. Do not use `serverku up` as
a Floci test. Future integration tests can inject a Compute service through
`NewWithService`, using an explicit local endpoint and no authentication.

## Repeat the Cloud Storage smoke test

This uses Python 3's standard library and only contacts localhost:

```bash
python3 - <<'PY'
import json
import urllib.request
import uuid

base = "http://127.0.0.1:4588"
bucket = "serverku-smoke-" + uuid.uuid4().hex[:12]

def request(method, path, data=None, content_type="application/json"):
    req = urllib.request.Request(
        base + path, data=data, method=method,
        headers={"Content-Type": content_type, "Authorization": "Bearer floci"},
    )
    with urllib.request.urlopen(req, timeout=10) as response:
        return response.read()

request("POST", "/storage/v1/b?project=floci-local",
        json.dumps({"name": bucket}).encode())
uploaded = False
try:
    payload = b"hello from serverku floci smoke test"
    request("POST", f"/upload/storage/v1/b/{bucket}/o?uploadType=media&name=hello.txt",
            payload, "text/plain")
    uploaded = True
    result = request("GET", f"/storage/v1/b/{bucket}/o/hello.txt?alt=media")
    assert result == payload, result
    print("PASS: bucket creation, upload, and download content")
finally:
    if uploaded:
        request("DELETE", f"/storage/v1/b/{bucket}/o/hello.txt")
    request("DELETE", f"/storage/v1/b/{bucket}")
    print("Smoke-test resources deleted")
PY
```

## Run serverku's existing offline tests

```bash
go test ./internal/provider/gcp ./internal/orchestrator
go test ./...
```

These tests use fake HTTP endpoints and mocked lifecycle dependencies; they
do not contact Floci or real GCP.

## Stop the emulator

```bash
docker compose -f compose.floci.yml down
```

Stopping the emulator discards its in-memory resources.
