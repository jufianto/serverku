# serverku Tutorials

Hands-on, blog-style walkthroughs that take you from zero to a running app —
and back down to zero — using real open-source projects.

## The core idea

Every tutorial follows the same lifecycle:

```text
init ──► up ──► (use it: status / logs / ssh / tunnel) ──► down ──► up ──► ... ──► destroy
                                                             │
                                              compute is OFF, data is KEPT
```

You pay for the VM only between `up` and `down`. Persistent data lives on a
block-storage disk that survives `down` and is only deleted by `destroy`.

## Pick a tutorial

| Tutorial | App (language) | Provider | What it teaches |
| --- | --- | --- | --- |
| [1. Uptime Kuma in 10 minutes](tutorial-uptime-kuma-digitalocean.md) | Uptime Kuma (JS/Node) | DigitalOcean | The minimal flow: one container, no domain, no disk. Your first `up`/`destroy`. |
| [2. WordPress with HTTPS and persistent data](tutorial-wordpress-digitalocean.md) | WordPress + MySQL (PHP) | DigitalOcean | The full production flow: persistent storage, Caddyku HTTPS, DNS automation, the `down`/`up` money-saving cycle. |
| [3. Gitea on GCP spot instances](tutorial-gitea-gcp.md) | Gitea (Go) | GCP | GCP auth and config, cheap spot/preemptible VMs, disk snapshots with `serverku backup`. |

New to serverku? Do them in order — each one builds on concepts from the last.

## Before any tutorial

Build the CLI once:

```bash
git clone https://github.com/jufianto/serverku.git
cd serverku
go build -o serverku ./cmd/serverku
# optional: move it onto your PATH
sudo mv serverku /usr/local/bin/
```

You also need `ssh` and `rsync` installed locally (macOS and most Linux
distros ship both).

## One rule to remember about data

When storage is enabled, serverku syncs your project into the persistent
disk's mount path (default `/data`) and runs Compose from there. That means:

- **Relative bind mounts persist.** `./mysql:/var/lib/mysql` lives at
  `/data/mysql` on the persistent disk — it survives `down`.
- **Named volumes do NOT persist.** They live in `/var/lib/docker` on the
  VM's boot disk, which is destroyed on every `down`.

Always use relative bind mounts (`./something`) for data you care about.
The tutorials model this correctly.
