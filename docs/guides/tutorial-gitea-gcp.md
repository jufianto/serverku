# Tutorial 3 — Gitea on GCP spot instances

*Your own Git server (written in Go) on Google Cloud, running on
spot pricing, with disk snapshots as your safety net.*

[Gitea](https://github.com/go-gitea/gitea) is a lightweight self-hosted Git
service — a single Go binary that happily runs with SQLite. It's a great
fit for the "personal infrastructure you don't need 24/7" niche: bring it
up to push/review code, take it down after.

This tutorial adds three things to what you know from Tutorials 1–2:

1. **GCP** as the provider (auth, project, zone).
2. **Spot/preemptible VMs** — 60–90% cheaper, and serverku knows how to
   cope when Google reclaims one.
3. **`serverku backup`** as a routine, not an emergency.

**Cost while up:** an `e2-small` spot instance is roughly $4–5/month
*if it ran all month* — you'll pay for hours, so realistically cents.
The 10 GB `pd-standard` disk is ~$0.40/month.

## Prerequisites

- A GCP project with **billing enabled** and the **Compute Engine API**
  turned on.
- The `gcloud` CLI authenticated with Application Default Credentials —
  serverku's GCP provider uses ADC, not API keys:

```bash
gcloud auth application-default login
gcloud config set project YOUR_PROJECT_ID
gcloud services enable compute.googleapis.com dns.googleapis.com
```

(`dns.googleapis.com` only matters if you later enable `dns:` with a Cloud
DNS zone; harmless otherwise.)

## Step 1 — The Compose project

```bash
mkdir -p ~/deploys/gitea
cat > ~/deploys/gitea/docker-compose.yml <<'EOF'
services:
  gitea:
    image: gitea/gitea:1.22
    restart: unless-stopped
    environment:
      GITEA__database__DB_TYPE: sqlite3
      GITEA__server__SSH_PORT: "9022"
    volumes:
      - ./gitea:/data
    ports:
      - "8080:3000"   # web UI
      - "9022:22"     # git-over-ssh
EOF
```

Familiar patterns from Tutorial 2: a relative bind mount (`./gitea`) so
repositories live on the persistent disk, SQLite to keep the stack to one
container. Two port choices worth explaining:

- Git SSH is published on `9022` (not `22`, which belongs to the VM's own
  sshd that serverku itself uses).
- The web UI is published on `8080` and SSH on `9022` because those fall
  inside the firewall rule serverku creates on GCP (see Step 3).

## Step 2 — Init for GCP

```bash
serverku init gitea
```

Wizard answers:

- **Provider**: `gcp` — you'll be asked for the **project ID** and a
  **zone** (e.g. `asia-southeast1-b`); both are required for GCP.
- **VM size**: `e2-small` (2 shared vCPU / 2 GB) is plenty for Gitea.
- **SPOT / Preemptible?**: **Yes** — this is where GCP shines.
- **Persistent storage**: **Yes**, `10` GB.

Then point the config at the project in
`~/.serverku/projects/gitea.yaml`:

```yaml
name: gitea
provider: gcp
project_id: your-project-id
region: asia-southeast1
zone: asia-southeast1-b

vm:
  size: e2-small
  image: ubuntu-22-04     # resolved to the ubuntu-2204-lts image family
  spot: true

storage:
  enabled: true
  size_gb: 10
  mount_path: /data

compose_file: ~/deploys/gitea/docker-compose.yml
sync_dir: ~/deploys/gitea
```

## Step 3 — Know your open ports

GCP's default network blocks inbound traffic, so on `up` serverku creates a
firewall rule (`serverku-gitea-fw`) targeting the VM's network tag. It
allows:

```text
tcp: 22, 80, 443, 8080, 9000-9999   (plus icmp)
```

The rule is created once and survives every `down`/`up` cycle (it targets
the tag, not the instance); `destroy` removes it. Our Compose file
publishes on `8080` and `9022` precisely because they're in this set — if
your own app needs a port outside it, either remap the host port into
`9000-9999` or add a rule manually with
`gcloud compute firewall-rules create`.

## Step 4 — Up

```bash
serverku up gitea
```

Same pipeline as DigitalOcean — firewall rule, instance, disk attach +
mount, Docker, rsync to `/data`, compose up — just with Compute Engine
vocabulary in the logs. Grab the IP from the output and open
**http://\<ip\>:8080**.

Gitea's installer is pre-filled from our environment variables; click
through, create the admin user, make a repo, and push to it:

```bash
git remote add gitea ssh://git@<ip>:9022/you/yourrepo.git
git push gitea main
```

## Step 5 — Living with spot instances

The deal with spot: Google can reclaim the VM at any time (rarely more
than once a day in practice, but contractually "whenever"). Why that's
acceptable here:

- Your repos are on the persistent disk — a preemption loses nothing but
  uptime.
- `serverku status gitea` **reconciles** state with GCP: if the instance
  was reclaimed, serverku notices the VM is gone, clears the stale
  IP/instance from local state, and marks the project `stopped`. Recovery
  is just `serverku up gitea` again.

```bash
serverku status gitea   # after a preemption: reports stopped, cleans state
serverku up gitea       # back in ~3 minutes, same disk, same repos
```

For a personal Git server that trade is easily worth 60–91% off. If a
teammate depends on it during work hours, set `spot: false` and pay
on-demand rates.

## Step 6 — Snapshots as routine

Repos are precious, so make `backup` a habit, not an incident response:

```bash
serverku backup gitea                     # serverku-gitea-<timestamp>
serverku backup gitea --name pre-1.23     # before upgrading the image
```

The disk can be snapshotted while the VM runs (crash-consistent). For a
perfectly clean copy of the SQLite DB, snapshot while down:

```bash
serverku down gitea && serverku backup gitea --name weekly
```

Snapshots live in GCP independently of the disk. Even after
`serverku destroy`, you can create a new disk from a snapshot in the
console and rebuild.

## Step 7 — Down / destroy

```bash
serverku down gitea      # instance gone, disk + firewall rule stay (~$0.40/mo)
serverku destroy gitea   # instance + disk + firewall rule + local config gone
```

Snapshots are the only thing `destroy` leaves behind.

## What you learned

- GCP needs three extra things: ADC auth, `project_id`, and a `zone`.
- serverku manages the GCP firewall rule for you — just publish your app
  on the allowed ports (`80`, `443`, `8080`, `9000-9999`).
- `spot: true` + persistent disk + `status` reconciliation = very cheap,
  slightly interruptible infrastructure.
- Snapshots are cheap insurance that outlive even `destroy`.

---

That's the full tour. From here, mix and match: Caddyku + DNS from
[Tutorial 2](tutorial-wordpress-digitalocean.md) works on GCP too (Cloud
DNS zones), and hooks (`pre_up: npm run build`) slot into any of these
flows — see the [Configuration Reference](../README.md#configuration-reference).
