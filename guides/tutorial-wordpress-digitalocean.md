# Tutorial 2 — WordPress with HTTPS and data that survives teardown

*The real serverku workflow: persistent storage, automatic HTTPS, DNS
automation — and turning compute off at night without losing a byte.*

[WordPress](https://wordpress.org) (PHP + MySQL) is the classic two-service
Compose stack, which makes it perfect for the features that Tutorial 1
skipped: a database that must survive `down`, a domain with a Let's Encrypt
certificate, and the `down`/`up` cycle that is the whole point of serverku.

**What you'll have at the end:** `https://blog.yourdomain.com` serving
WordPress, with all content on a persistent 10 GB volume. Run
`serverku down` when you're done writing; `serverku up` brings the exact
same site back in ~3 minutes.

**Cost while up:** ~$6/month droplet + ~$1/month volume.
**Cost while down:** ~$1/month (volume only).

## Prerequisites

- Tutorial 1 completed (or at least skimmed).
- `DIGITALOCEAN_TOKEN` exported.
- A domain whose DNS is **managed by DigitalOcean** — i.e. the domain is
  added under *Networking → Domains* and your registrar points at
  `ns1–ns3.digitalocean.com`. This is what lets serverku create the A record
  for you.

## Step 1 — The Compose project

```bash
mkdir -p ~/deploys/blog
cat > ~/deploys/blog/docker-compose.yml <<'EOF'
services:
  db:
    image: mysql:8.0
    restart: unless-stopped
    environment:
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: change-me-please
      MYSQL_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - ./mysql:/var/lib/mysql

  wordpress:
    image: wordpress:6-php8.3-apache
    restart: unless-stopped
    depends_on:
      - db
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_NAME: wordpress
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: change-me-please
    volumes:
      - ./wp-content:/var/www/html/wp-content
    expose:
      - "80"
EOF
```

Two deliberate choices here:

1. **Relative bind mounts** (`./mysql`, `./wp-content`), *not* named
   volumes. serverku syncs your project into the persistent disk's mount
   path (`/data`) and runs Compose from there — so `./mysql` physically
   lives at `/data/mysql` on the volume and survives teardown. A named
   volume would live on the VM's boot disk and be **wiped on every
   `down`**.
2. **`expose`, not `ports`.** WordPress never touches the host network —
   Caddy (via Caddyku) will be the only thing listening on 80/443 and will
   proxy to `wordpress:80` internally.

## Step 2 — Init with storage

```bash
serverku init blog
```

Wizard answers: `digitalocean`, your region, `s-1vcpu-1gb`, **No** to spot,
**Yes** to persistent storage, `10` GB.

## Step 3 — Wire up routing and DNS

Open the project configuration:

```bash
serverku edit blog
```

Set the values below. See [Editing projects](editing-projects.md) for backups and
rerunning the setup wizard with `reinit`.


```yaml
name: blog
provider: digitalocean
region: sgp1

vm:
  size: s-1vcpu-1gb
  image: ubuntu-22-04-x64
  spot: false

storage:
  enabled: true
  size_gb: 10
  mount_path: /data

compose_file: ~/deploys/blog/docker-compose.yml
sync_dir: ~/deploys/blog

router:
  enabled: true
  domains:
    - domain: blog.yourdomain.com
      service: wordpress
      upstream: wordpress:80

dns:
  enabled: true
  ttl: 300
```

What each block buys you:

- **`router`** — during provisioning, serverku installs
  [Caddyku](https://github.com/jufianto/caddyku) on the VM, which runs a
  global Caddy proxy and routes `blog.yourdomain.com` to the `wordpress`
  service with an automatic Let's Encrypt certificate.
- **`dns`** — before provisioning, serverku creates/updates the A record
  `blog.yourdomain.com → <new VM IP>` in your DigitalOcean-managed zone.
  It happens *before* Caddy starts, so certificate issuance can resolve
  the name. A low `ttl` (300) keeps re-`up` cycles snappy, since every
  `up` gets a fresh IP.

## Step 4 — Up

```bash
serverku up blog
```

This run does more than Tutorial 1: create droplet → create + attach +
format the 10 GB volume → mount at `/data` → DNS A record → Docker →
rsync to `/data` → install Caddyku → issue certificate → compose up.

Give Let's Encrypt a minute after the command returns, then open
**https://blog.yourdomain.com** — the WordPress installer, with a padlock.
Finish the 5-minute install, pick a strong admin password (this is a
public site!), write a hello-world post.

## Step 5 — The money move: down, then up again

Done for the day?

```bash
serverku down blog
```

The droplet — and its hourly bill — is gone. The volume (your database,
uploads, themes) stays, for about $1/month.

Tomorrow:

```bash
serverku up blog
```

A **new** droplet boots, the **same** volume re-attaches, DNS flips to the
new IP, Caddy re-issues the cert, and your post is exactly where you left
it. This cycle is the entire reason serverku exists: a personal blog used
2 hours a day costs cents, not $6/month.

Two things to know about the cycle:

- The VM is disposable: anything outside `/data` (installed packages,
  named volumes, `/home`) resets every cycle. Put state on the volume or
  in `startup_commands`.
- The IP changes every `up`. With `dns.enabled` that's handled for you;
  without it you'd re-point DNS by hand each time.

## Step 6 — Back up before you break things

About to try a risky plugin or a major upgrade? Snapshot the volume first:

```bash
serverku backup blog --name pre-upgrade
```

Snapshots are crash-consistent. For a clean, application-consistent backup
of MySQL, take it while compute is off:

```bash
serverku down blog && serverku backup blog --name nightly
```

Restore while stopped with `serverku restore blog nightly`, then run
`serverku up blog` to attach the restored volume.

## Step 7 — Gone for good

When the blog has run its course:

```bash
serverku destroy blog
```

This deletes the droplet, project volumes, and their snapshots. It is
irreversible. Use `down` to preserve the volumes and snapshots.

## What you learned

- Relative bind mounts + persistent storage = data that outlives the VM.
- `router` + `dns` turn a bare droplet into a proper HTTPS site with zero
  console clicking.
- `down`/`up` is a routine operation, not a disaster drill.
- `backup` before anything scary.

**Next:** [Gitea on GCP — spot instances and snapshots →](tutorial-gitea-gcp.md)
