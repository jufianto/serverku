# Choosing your setup

serverku gives you a lot of knobs — provider, region, VM size, spot, disk size,
auto-shutdown. This guide explains **how to choose** each one so your project
config fits your app and your budget. If you just want the command flow, see any
of the app tutorials in this folder; this is the "why these values?" companion.

The one idea behind everything below: **you pay full price for the VM only while
it's running, and a few cents for the disk the rest of the time.** Most choices
here are really "how do I keep the running cost low without the app falling
over?"

> Prices in this guide are approximate list prices from serverku's offline
> tables, shown to build intuition. Your real numbers come from
> `serverku status <project>` and `serverku list`, which use each provider's
> live pricing API.

## 1. Provider — GCP or DigitalOcean?

Both are fully supported. Pick based on what you need:

| You want… | Pick |
| --- | --- |
| The cheapest interruptible compute (dev, CI, personal apps) | **GCP** — spot/preemptible VMs are 60–90% cheaper |
| A hard "delete the VM after N hours" budget cap | **GCP** — `max_uptime_hours` (see §6) |
| Automatic HTTPS domains via the cloud's DNS | Either — both do DNS + Caddyku |
| The simplest networking (VMs open by default) | **DigitalOcean** — no firewall rule to manage |
| Real account month-to-date spend in `serverku status` | **DigitalOcean** — reports live usage |
| A GCP project you already have (like this repo's author's personal project) | **GCP** |

You can run different projects on different providers — the choice is per
project, not global.

## 2. Region

Pick the region **closest to whoever uses the app** — that's what determines
latency. Two secondary notes:

- Region also nudges the price (a few percent); it's rarely the deciding factor.
- On GCP you also choose a **zone** inside the region (e.g. region
  `asia-southeast1` → zone `asia-southeast1-b`). DigitalOcean has no zones.

Examples: `asia-southeast1` / `asia-southeast1-b` (Singapore, GCP),
`sgp1` (Singapore, DO), `us-central1` (Iowa, GCP), `nyc1` (New York, DO).

## 3. VM size — CPU and RAM

Size for your app's **RAM** first (running out of memory is what actually kills
containers); CPU rarely limits small apps. Start small — you can always bump the
size and `serverku up` again.

| Workload | DigitalOcean | GCP |
| --- | --- | --- |
| Static site, tiny API, bot, one small container | `s-1vcpu-1gb` (~$0.009/hr) | `e2-small` (~$0.017/hr) |
| Typical web app + small database (WordPress, Gitea) | `s-2vcpu-2gb` (~$0.027/hr) | `e2-medium` (~$0.034/hr) |
| Heavier app, multiple services, more traffic | `s-2vcpu-4gb`–`s-4vcpu-8gb` | `e2-standard-2`–`e2-standard-4` |

Note the naming difference: DigitalOcean encodes it in the name
(`s-2vcpu-4gb` = 2 vCPU / 4 GB); GCP uses machine families (`e2-medium` = 1
shared vCPU / 4 GB, `e2-standard-2` = 2 vCPU / 8 GB).

**Signs you undersized:** containers get OOM-killed or restart, `serverku logs`
shows "killed"/out-of-memory, the app is sluggish under load. Bump to the next
size and `serverku up`.

## 4. Spot / preemptible — the big cost lever

`vm.spot: true` (GCP only) gives you the same machine for **60–90% less**, with
one catch: the cloud can **reclaim it at any time** (and always within 24h).

- **Great for:** dev boxes, CI, personal apps, anything you can tolerate a
  restart on. Because serverku keeps your data on a *separate persistent disk*
  (§5), a reclaim just means the compute vanishes — your data is safe, and
  `serverku up` brings it back on a fresh instance.
- **Avoid for:** something that must never blink (a store checkout, a live demo
  during a meeting).
- **Pair it with §6** (`max_uptime_hours`) and persistent storage so a reclaim
  is a non-event.

DigitalOcean has no spot equivalent, so `spot: true` is rejected there at config
validation.

## 5. Disk — `storage.size_gb`

This is the persistent block disk mounted at `mount_path` (default `/data`).
It's the heart of serverku's model:

- **It survives `down`.** When you `serverku down`, the VM is destroyed but the
  disk stays — that's how you keep your database/files between sessions.
- **It's the only thing you pay for while idle.** Storage is ~$0.10/GB-month on
  DO, ~$0.04/GB-month on GCP. A 10 GB disk costs about **$1.00/mo (DO)** or
  **$0.40/mo (GCP)** while the VM is off — cheap insurance for your data.

**How to size it:** add up what actually lives on the disk (database files,
uploads, repos) and add headroom. 10 GB is plenty for most small apps; a
media-heavy or database-heavy app may want 20–50 GB. Prefer a little extra —
resizing an existing disk isn't a one-command operation (you'd snapshot with
`serverku backup` and restore onto a new, larger disk with `serverku restore`).

Stateless apps (a static site, a stateless API) can set `storage.enabled: false`
and skip the disk entirely — then `down`/`destroy` are equivalent and you pay
nothing while off.

## 6. `max_uptime_hours` — the budget kill-switch

Set `vm.max_uptime_hours: 12` and **GCP itself deletes the VM after 12 hours**
of runtime — enforced by the cloud, so it fires even if your laptop is off. The
persistent disk survives (it's separate), so it behaves like an automatic
`serverku down`. This is the strongest guard against "I forgot the VM was
running" bill shock.

- **GCP only.** DigitalOcean has no native auto-delete, and a powered-off
  droplet still bills — so serverku rejects `max_uptime_hours` on DO. Use the
  notification **heartbeat** instead (`notifications.*.heartbeat_hours`) to get
  pinged while a droplet is still running.
- Great paired with spot: interruptible compute *and* a hard time cap.

## 7. The cost model, end to end

```
serverku up       →  paying for VM ($/hr) + disk (tiny) + running your app
serverku down     →  VM destroyed; paying ONLY for the disk (cents/day); data kept
serverku up       →  back in ~minutes on the same disk, same data
serverku destroy  →  everything gone (VM + disk + config); paying nothing
```

So the recipe for a cheap setup is: **a small VM, spot where you can tolerate it,
a modestly-sized disk, and `down` (not idle) whenever you're not using it** —
optionally with `max_uptime_hours` or a heartbeat so nothing runs forgotten.

Check real cost any time:

```bash
serverku status myapp   # this session's accrued cost at the live rate
serverku list           # cost across all projects
```

## Putting it together — an example config

A small personal web app on GCP spot, data on a 10 GB disk, auto-deleted after
12h as a safety cap:

```yaml
name: myapp
provider: gcp
project_id: my-personal-labs-395004
region: asia-southeast1
zone: asia-southeast1-b

vm:
  size: e2-small        # 2 shared vCPU / 2 GB — plenty for a small app
  spot: true            # 60–90% cheaper; reclaimable (data is safe on the disk)
  max_uptime_hours: 12  # GCP auto-deletes after 12h; the disk survives

storage:
  enabled: true
  size_gb: 10           # ~$0.40/mo on GCP while the VM is off
  mount_path: /data

compose_file: ~/deploys/myapp/docker-compose.yml
sync_dir: ~/deploys/myapp
```

**Next:** pick a tutorial to see this in action —
[Uptime Kuma (simplest)](tutorial-uptime-kuma-digitalocean.md),
[WordPress (full production flow)](tutorial-wordpress-digitalocean.md), or
[Gitea on GCP spot](tutorial-gitea-gcp.md).
