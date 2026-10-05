# Tutorial 1 — Uptime Kuma on DigitalOcean in 10 minutes

*Deploy a real app to a real cloud VM, poke at it, and tear it down —
your first full serverku lifecycle.*

[Uptime Kuma](https://github.com/louislam/uptime-kuma) is a popular
open-source uptime monitor (Node.js) that ships as a single Docker image.
That makes it the perfect first deploy: no database, no domain, no build
step.

**What you'll have at the end:** Uptime Kuma running on a $6/month droplet,
reachable at `http://<ip>:3001` — and the confidence that `destroy` really
cleans everything up.

**Cost:** a couple of cents. The droplet bills per hour (~$0.009/h for
`s-1vcpu-1gb`) and you'll destroy it within the hour.

## Prerequisites

- The `serverku` binary ([build it here](README.md#before-any-tutorial)).
- A DigitalOcean account and an API token with **write** scope
  ([create one](https://cloud.digitalocean.com/account/api/tokens)).

Export the token:

```bash
export DIGITALOCEAN_TOKEN="dop_v1_..."
```

## Step 1 — A project directory with a Compose file

serverku deploys ordinary Docker Compose projects, so that's all we create:

```bash
mkdir -p ~/deploys/kuma
cat > ~/deploys/kuma/docker-compose.yml <<'EOF'
services:
  kuma:
    image: louislam/uptime-kuma:1
    restart: unless-stopped
    ports:
      - "3001:3001"
EOF
```

We publish port `3001` directly — no reverse proxy, no domain. Keeping it
raw makes the moving parts visible; Tutorial 2 adds HTTPS properly.

## Step 2 — `serverku init`

```bash
serverku init kuma
```

The wizard asks a few questions. Answer:

- **Provider**: `digitalocean`
- **Region**: one near you, e.g. `sgp1` (Singapore) or `fra1` (Frankfurt)
- **VM size**: `s-1vcpu-1gb`
- **SPOT / Preemptible instances?**: **No** — DigitalOcean has no spot
  equivalent and `up` will refuse to run with it enabled
- **Persistent storage?**: **No** — this is a throwaway demo

> Prefer flags? The same setup non-interactively:
>
> ```bash
> serverku init kuma --non-interactive --provider digitalocean \
>   --region sgp1 --size s-1vcpu-1gb --no-storage
> ```
>
> **Gotcha:** `--spot` currently defaults to `true`, which DigitalOcean
> rejects at `up` time. If you go non-interactive, set `spot: false` in the
> config in the next step.

## Step 3 — Point the config at your project

Open your project configuration:

```bash
serverku edit kuma
```

Set the Compose file, sync dir, and image. To rerun setup later or add persistent
storage, see [Editing projects](editing-projects.md). This minimal tutorial keeps
storage disabled, so app data is deleted with the VM.


```yaml
name: kuma
provider: digitalocean
region: sgp1

vm:
  size: s-1vcpu-1gb
  image: ubuntu-22-04-x64
  spot: false

storage:
  enabled: false

compose_file: ~/deploys/kuma/docker-compose.yml
sync_dir: ~/deploys/kuma
```

## Step 4 — `serverku up`

```bash
serverku up kuma
```

Watch the log lines go by — this is the whole serverku pipeline in order:
create droplet → wait for SSH → install Docker → rsync your project →
write the Compose file → `docker compose up -d`. It takes 2–4 minutes,
mostly waiting for the droplet to boot.

At the end you get the IP:

```text
VM IP: 203.0.113.42
SSH: ssh serverku@203.0.113.42
```

Open **http://203.0.113.42:3001** — Uptime Kuma greets you with its setup
screen. Add a monitor for your favorite website and watch it turn green.

## Step 5 — Poke at the machine

Everything you'd normally do with a server, without leaving serverku:

```bash
serverku status kuma          # state, IP, uptime cost estimate
serverku logs kuma            # live `docker compose logs -f`
serverku ssh kuma             # interactive shell as user `serverku`
serverku tunnel kuma 3001:3001  # or reach it privately via localhost:3001
```

Inside the SSH session, look around — it's a normal Ubuntu box:

```bash
docker compose ps
exit
```

## Step 6 — Tear it down

```bash
serverku destroy kuma
```

Confirm the prompt (or pass `-f`). The droplet is deleted, and so are the
local config and state. Verify in the DigitalOcean console: no droplets,
no volumes, nothing billing.

> Because we skipped persistent storage, `down` and `destroy` are nearly
> equivalent here — the monitor data dies with the VM. That's fine for a
> demo, and exactly what the next tutorial fixes.

## What you learned

- A serverku project = a Docker Compose project + one YAML config.
- `up` is a one-command path from nothing to a running container.
- `logs` / `ssh` / `tunnel` cover day-to-day operations.
- `destroy` returns you to a zero-cost state.

**Next:** [WordPress with HTTPS, a real domain, and data that survives
teardown →](tutorial-wordpress-digitalocean.md)
