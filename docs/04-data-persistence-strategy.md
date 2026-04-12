# Data Persistence Strategy

## The Problem

serverku creates and destroys VMs on demand. When a VM is destroyed, all data on its boot disk is lost. Projects that need persistent data (databases, uploaded files, application state) need a strategy to survive VM lifecycle.

## Supported Modes

### Mode 1: Persistent Block Storage (Default)

```yaml
storage:
  enabled: true
  size_gb: 20
  mount_path: /data
```

**How it works:**
- A separate block storage volume (GCP Persistent Disk / DO Volume) is created alongside the project
- When `serverku up` runs: VM is created, then disk is attached and mounted at `mount_path`
- Docker volumes map to paths on the mounted disk
- When `serverku down` runs: containers stop, disk is detached, VM is destroyed
- The disk persists and retains all data
- Next `serverku up`: same disk is reattached, data is intact

**Cost when idle:**
- GCP: $0.04/GB/month (pd-standard) -> 20GB = $0.80/mo
- DO: $0.10/GB/month -> 20GB = $2.00/mo

**Docker Compose example with persistent storage:**
```yaml
version: "3.8"
services:
  app:
    image: my-app:latest
    ports:
      - "80:3000"
    volumes:
      - /data/app:/app/data

  postgres:
    image: postgres:16
    volumes:
      - /data/postgres:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
```

All data under `/data/` lives on the persistent disk and survives VM destruction.

**Limitations:**
- Disk is locked to a zone (can't move between regions without snapshotting)
- Only one VM can attach a disk at a time
- If disk fills up, you need to resize (possible but requires manual step)
- Running a database on block storage is less reliable than a managed DB service

### Mode 2: Fully Stateless

```yaml
storage:
  enabled: false
```

**How it works:**
- No persistent disk is created
- VM is completely disposable - all data is lost on `serverku down`
- Application connects to external services for state (managed database, object storage, etc.)

**Use cases:**
- Stateless API servers that connect to external databases
- Static site previews
- CI/CD runners
- Applications that use managed services (Supabase, Neon, PlanetScale, Firebase, S3, etc.)

**Docker Compose example (stateless):**
```yaml
version: "3.8"
services:
  api:
    image: my-api:latest
    ports:
      - "80:8080"
    environment:
      DATABASE_URL: postgres://user:pass@db.supabase.co:5432/mydb
      REDIS_URL: redis://my-redis.upstash.io:6379
      S3_BUCKET: my-app-uploads
```

No volumes needed. All state lives in external managed services.

**Advantages:**
- Cheapest option (no idle storage cost)
- VM is truly disposable
- Database is always available, even when VM is off
- Better for production-like setups

**Disadvantages:**
- Requires external service setup
- External services have their own costs
- More configuration needed

## Choosing Between Modes

| Factor | Block Storage | Stateless |
|--------|--------------|-----------|
| Setup complexity | Low (just works) | Medium (need external services) |
| Data safety | Good (survives VM deletion) | Excellent (managed service handles it) |
| Idle cost | $0.80-2.00/mo per 20GB | $0/mo (but external DB may cost) |
| Database reliability | Fair (self-managed) | Excellent (managed service) |
| Best for | Dev/staging, side projects | Production-like, team environments |

## Recommendation by Project Type

| Project Type | Recommended Mode | Why |
|-------------|-----------------|-----|
| Side project (full-stack) | Block Storage | Simple, cheap, self-contained |
| Client demo | Stateless | No lingering costs, quick teardown |
| Staging environment | Block Storage | Need persistent test data |
| API connected to external DB | Stateless | DB lives elsewhere anyway |
| WordPress/CMS | Block Storage | Uploads and DB need persistence |
| Data processing job | Stateless | Process and push results elsewhere |

## Future: Managed Database Provisioning

A potential v0.3.0 feature: serverku could provision a managed database alongside the VM.

```yaml
database:
  provider: neon     # or supabase, cloud-sql
  type: postgres
  size: free-tier
```

This would create a Neon/Supabase database via their APIs, inject the connection string as an environment variable into the Docker containers, and manage the DB lifecycle separately from the VM.

This is explicitly out of scope for v0.1.0 but worth keeping in mind for the architecture.
