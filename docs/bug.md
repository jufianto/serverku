# Bug tracker

Working log of bugs found while running serverku against a real DigitalOcean
project (`wpblog`). Newest first. Update the **Status** line as each is traced
and fixed.

Status legend: `OPEN` · `FIXED` · `WORKAROUND` (unblocked, root cause remains).

---

## BUG-6 — `docker compose up` fails with docker.sock permission denied

- **Status:** FIXED (staged, not yet committed)
- **Provider:** both (any first-time provision)
- **Symptom:** provisioning reaches "running docker compose up -d" then fails:
  `permission denied while trying to connect to the docker API at
  unix:///var/run/docker.sock` (e.g. `unable to get image 'mysql:8.0'`).

**Root cause.** `installDockerScript` runs `usermod -aG docker serverku`
(`scripts.go:26`), but Linux group membership only takes effect in a **new**
login session. The provisioner reuses the single SSH session opened *before*
the group was added (`provisioner.go`), so within that session the serverku
user is still not in the `docker` group and cannot reach the root-owned
`/var/run/docker.sock`.

**Fix.** Run docker via `sudo` in the provisioning scripts (`composeUpScript`,
`teardownScript`, `initCaddyProxyScript`) — the daemon runs as root, so this is
reliable regardless of the caller's group. The `usermod` still stands so that a
later interactive `serverku ssh` (a fresh login) gets passwordless docker.
Covered by an added assertion in `TestComposeUpScript`.

**Alternative considered.** Reconnect SSH after `usermod` so subsequent commands
inherit the docker group. Rejected as more invasive than sudo for automated,
non-interactive provisioning.

---

## BUG-5 — Docker install races the boot-time apt/dpkg lock

- **Status:** FIXED (staged, not yet committed)
- **Provider:** both (any freshly-booted Ubuntu VM)
- **Symptom:** provisioning fails during "installing Docker..." with
  `E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process
  <pid> (apt-get)`.

**Root cause.** On a fresh Ubuntu droplet/VM, `cloud-init` and
`unattended-upgrades` are still running `apt-get` at boot. `sshd` comes up
before they finish, so serverku connects and immediately runs its own
`apt-get update` (`internal/provisioner/scripts.go:9`), colliding on the dpkg
lock. It's a race — a retry sometimes succeeds once the background job ends.

**Fix.** `installDockerScript` now waits before touching apt:
`sudo cloud-init status --wait`, then a bounded loop (`fuser` on
`lock-frontend` / `lock` / `lists/lock`, up to ~5 min) until the lock is free.

**Note.** BUG-2's cloud-init `user_data` (creates the `serverku` user) also runs
at boot; waiting on `cloud-init status --wait` covers that too.

---

## BUG-4 — `destroy` deletes the local project config, not just cloud resources

- **Status:** FIXED (staged, not yet committed)
- **Where:** `internal/orchestrator/orchestrator.go:681-687` (`Destroy`, step 3);
  command help `cmd/serverku/lifecycle.go:244`.
- **Symptom:** after `serverku destroy <name>`, `~/.serverku/projects/<name>.yaml`
  is gone. The user must re-`init` (and re-enter region/size/notifications/etc.)
  to recreate the project, even though they only wanted the cloud VM/disk torn
  down.

**Current behavior.** `Destroy` tears down cloud resources (VM, disk, firewall)
and then deletes **both** local files:

```go
o.store.DeleteState(projectName)    // state/<name>.json   — fine
o.store.DeleteProject(projectName)  // projects/<name>.yaml — deletes user config
```

**Expected behavior (per project owner).** `destroy` should remove the *cloud*
resources (VM + persistent storage + firewall) and the runtime **state**, but
**keep the local project definition** (`projects/<name>.yaml`) so the project can
be brought back with `up` without re-`init`. This matches the Terraform-style
split between "the definition" and "the provisioned resources."

**Fix.** Dropped the `DeleteProject` call from `Destroy`; `DeleteState` stays, so
the project returns to the clean post-init condition (`LoadState` reports
`StatusStopped` when the state file is absent) with its config intact. Updated
the destroy command `Short`/`Long` help, the confirmation warning, and the
success message to say the config is kept. Tests updated to assert the config is
preserved and state is reset.

**Follow-up (not done):** a `--purge` / `--remove-config` flag for users who
explicitly want the definition deleted too.

**Impact of the bug on this session:** the `wpblog` project config was lost to a
`destroy`; it had to be recreated from scratch.

---

# Enhancements

Not bugs — current behavior is correct, but under-validated / worth improving.

## ENH-4 — Selectable VM size / region in `init` (live catalog)

- **Status:** PLANNED
- **Where:** new `CatalogLister` provider capability, `cmd/serverku/init.go`
  (interactive + `--size`/`--region` validation), a docs guide.

**Problem.** Interactive `init` asks for VM size and region as free-text
(`huh.NewInput`), so the user must already know a valid slug
(`s-1vcpu-1gb`, `e2-medium`, `sgp1`, ...). A wrong value only fails later at
create time (see BUG-1). There's no in-tool discovery.

**Design (layered — live, static fallback, docs).**
1. **Live picklist.** Add optional `CatalogLister` capability:
   `ListRegions(ctx)` and `ListSizes(ctx, region)`, reusing the existing
   `Sizes.List` (DO, already used in `pricing.go`) and `Regions.List`/machine
   types (GCP). In interactive `init`, replace the size/region inputs with
   `huh.NewSelect` showing human labels (slug + vCPU/RAM/disk + $/mo). Order:
   **region first, then sizes** (GCP machine types are region-scoped; DO
   availability varies by region). Curate to a sensible subset (DO basic `s-*`;
   GCP `e2`/`n2`) with a "show all / enter manually" escape.
2. **Static fallback.** `init` can run before `serverku setup`, so if the live
   fetch fails (no creds / offline), fall back to a small curated shortlist
   baked into serverku (5-8 sizes/provider with prices) as a Select + free-text.
3. **Docs guide.** "Choosing a VM size & region" page listing recommended picks
   and linking to the live provider pages (DO sizes/pricing, GCP machine types).
4. **Non-interactive bonus.** Validate `--size`/`--region` against the catalog
   and print nearest valid suggestions on error, instead of a late create-time
   422 (addresses the BUG-1 class of failure).

**Tradeoff.** Raw catalogs are large (DO ~90 sizes, GCP hundreds) -- curation /
filtering is essential or the list is as overwhelming as the blank field.

## ENH-3 — Local-first project config (`./serverku.yaml`)

- **Status:** PLANNED
- **Where:** `internal/config/store.go` (`LoadProject`), a new cmd-level
  resolver, and the project commands in `cmd/serverku/`.

**Goal.** Let a project be defined by a `serverku.yaml` living in the working
directory (version-controlled next to the code / compose file), falling back to
the central `~/.serverku/projects/<name>.yaml` store. Chosen model: **ambient**
(fixed filename `serverku.yaml`; the project name comes from the file, so the
`<name>` CLI arg becomes optional) and **current-directory only** (no parent
walk).

**Resolution rule** (shared `resolveProject(args)` helper):
1. `./serverku.yaml` exists → use it; its `name:` identifies the project. If a
   `<name>` arg is also given and mismatches the file's `name:`, error.
2. No local file but `<name>` given → central `~/.serverku/projects/<name>.yaml`
   (current behavior).
3. Neither → error: "no serverku.yaml in this directory; pass a project name or
   run from a project directory."

**Changes.**
- Project-name arg becomes optional (`MaximumNArgs(1)`) on up, down, deploy,
  destroy, status, logs, ssh, tunnel, backup, restore, check, ntfy, notify, open.
- Store: `LoadProjectFrom(path)` / `SaveProjectTo(path)`; resolver returns
  `(cfg, name, sourcePath)`; `SaveProject` writes back to the local file when the
  config came from one.
- **State and keys stay central** in `~/.serverku/` keyed by `cfg.Name` — never
  written next to code (runtime/secret data must not land in a repo).
- `--config-dir` still controls the central base dir; local lookup is
  independent of it.

**Known limitations (v1).**
- `serverku list` shows central projects only; a local-only project won't appear
  unless you're in its dir (could add a `(local)` marker later).
- `serverku init` still writes central; a `serverku init --local` to scaffold
  `./serverku.yaml` is a follow-up.
- No parent-directory walk (cwd only).

## ENH-2 — Live component inventory in `serverku status`

- **Status:** DONE (staged, not yet committed)
- **Where:** `internal/provider/provider.go` (new `ComponentLister` capability +
  `Component`/`ComponentQuery`), `digitalocean.go`, `gcp.go`, `cmd/serverku/status.go`.

**Why.** `status` showed only VM/IP/disk, so it was unclear what cloud resources
a project actually created — especially the ones `destroy` leaves behind
(orphans). Users had no built-in way to know what to clean up manually.

**What.** Added an optional `ComponentLister` capability (type-assertion pattern
like `DNSManager`/`FirewallManager`). `serverku status` now prints a
`Components (live):` section listing each managed resource, whether it currently
exists, and flagging orphans (`⚠ destroy won't remove`) with a manual-cleanup
warning. Live-verified against the provider API:

- DigitalOcean: VM, Volume (removed by destroy); SSH key, Snapshots (orphans).
- GCP: VM, Disk, Firewall (removed by destroy); Snapshots (orphans). SSH key
  rides in instance metadata and dies with the VM, so it is not listed.

**Known limitation (follow-up).** DNS A records (when `dns.enabled`) are not yet
included in the live inventory — they are also orphaned by `destroy`
(see the DO SSH-key / snapshot / DNS orphan notes under the component
inventory). Add DNS-record enumeration to both listers.

## ENH-1 — Preflight validation of compose ↔ sync_dir consistency

- **Status:** OPEN
- **Where:** `check` / `internal/orchestrator/orchestrator.go` (`checkComposeFile`,
  around lines 845-900).

**Context.** `docker compose up -d` runs **on the server**
(`internal/provisioner/provisioner.go:253`, over SSH), not on the laptop. The
laptop only reads the compose content, rsyncs `sync_dir`, and writes
`docker-compose.yml` onto the VM. So anything the compose file references from
the local project (build context / Dockerfile, `env_file`, relative bind-mount
sources) only exists on the VM if `sync_dir` carried it up. Today `check` only
verifies the compose file is present, non-empty, valid YAML, has a `services:`
key, and that `sync_dir` (if set) exists — it does **not** cross-check the two,
so mismatches surface as a confusing failure mid-`up` after a VM already exists.

**Proposed checks** (parse the already-unmarshalled `services` map):

1. **`build:` present but `sync_dir` empty** → ERROR. The build context (incl.
   Dockerfile) never reaches the VM, so the server-side build fails.
2. **`build: ./x` with `sync_dir` set** → verify `./x` exists inside `sync_dir`.
3. **`env_file: ./…` but `sync_dir` empty** → ERROR. Env file isn't uploaded.
4. **relative bind mount `./…:/…` but `sync_dir` empty** → WARN. Mounts as an
   empty dir on the VM (no seeded content/data).
5. *(nice-to-have)* **named volumes + storage** → WARN that named volumes live
   under `/var/lib/docker/volumes` on the boot disk and do NOT survive `down`
   unless the data is bind-mounted under the storage `mount_path`.

Checks 1 and 3 are hard failures; 4 and 5 are warnings. Bounded work — no full
compose-spec engine, just key-existence checks on the parsed map. Extend
`checkComposeFile` to also take `sync_dir` (or add a sibling checker).

**Why:** catches every deploy-time failure mode of the compose/sync_dir combo
*before* a VM is created, instead of after.

---

## BUG-3 — DigitalOcean storage mount uses a GCP-only device path

- **Status:** FIXED (staged, not yet committed)
- **Provider:** DigitalOcean (any project with `storage.enabled: true`)
- **Symptom:** provisioning hangs on the mount step, then fails with
  `Device /dev/disk/by-id/google-<disk> not found after waiting`.

**Root cause.** `mountDiskScript` hardcodes the GCP device path:

```go
// internal/provisioner/scripts.go:23
device := fmt.Sprintf("/dev/disk/by-id/google-%s", diskName)
```

On DigitalOcean an attached block-storage volume appears at
`/dev/disk/by-id/scsi-0DO_Volume_<volume-name>`, never `google-…`, so the
wait loop (`scripts.go:29-34`) times out.

**Impact.** Persistent storage is effectively unusable on DigitalOcean:
`enabled: false` works but nothing survives `down`; `enabled: true` fails at
mount. GCP is unaffected.

**Fix.** Made the device path provider-aware. Added `diskDevicePath(provider,
diskName)` in `internal/provisioner/scripts.go` — emits `google-<diskName>` for
GCP and `scsi-0DO_Volume_<volumeName>` for DO. `ProvisionOpts` gained a
`Provider` field (set from `cfg.Provider` in the orchestrator) that
`mountDiskScript` uses. Covered by `TestMountDiskScriptDigitalOcean` and
`TestDiskDevicePath`.

**Verified on a live droplet** (`serverku up wpblog`, 2026-07-24):
`ls -l /dev/disk/by-id/` shows
`scsi-0DO_Volume_serverku-wpblog-data -> ../../sda`, and `/data` mounts from
`/dev/sda`. The device path is confirmed correct; WordPress + MySQL came up with
data on the persistent volume.

---

## BUG-2 — DigitalOcean droplet has no `serverku` SSH user (auth fails)

- **Status:** FIXED (staged, not yet committed)
- **Provider:** DigitalOcean
- **Symptom:** provisioning SSH loop: first attempts `connection refused`
  (sshd still booting — benign), then
  `ssh: handshake failed: ssh: unable to authenticate, attempted methods
  [none publickey]` on every retry, forever.

**Root cause.** The orchestrator always connects as user `serverku`
(`internal/orchestrator/orchestrator.go:316`), compose dir is
`/home/serverku`, and `serverku ssh` uses `serverku@`. GCP satisfies this by
injecting the key as instance metadata `ssh-keys = "serverku:<pubkey>"`
(`internal/provider/gcp/gcp.go:155`), which the Google guest agent turns into
a `serverku` OS user. DigitalOcean has no equivalent — it only injects the
account key into **root's** `authorized_keys`, and the droplet-create request
sent no cloud-init, so no `serverku` user ever existed.

**Fix.** Added cloud-init `user_data` to the DO droplet-create request that
provisions the `serverku` user (passwordless sudo + the same pubkey), via a new
`serverkuUserData` helper in `internal/provider/digitalocean/digitalocean.go`.
Covered by `TestServerkuUserData`.

---

## BUG-1 — DigitalOcean rejects the default image slug `ubuntu-22-04`

- **Status:** WORKAROUND (config edited; code default still wrong for DO)
- **Provider:** DigitalOcean
- **Symptom:** `up` fails at droplet creation with
  `422 ... You specified an invalid image for Droplet creation.`

**Root cause.** The shared default image is `ubuntu-22-04`
(`internal/config/project.go:255`). GCP maps this to an image *family* via
`resolveImageFamily` (`gcp.go:580`), but the DigitalOcean provider passes the
slug through verbatim (`digitalocean.go` → `Slug: cfg.Image`). DigitalOcean's
actual slug is `ubuntu-22-04-x64`, so the default is rejected.

**Workaround applied.** Set `vm.image: ubuntu-22-04-x64` in
`~/.serverku/projects/wpblog.yaml`.

**Proposed fix.** Map common slugs in the DO provider (mirroring GCP):
`ubuntu-22-04` → `ubuntu-22-04-x64`, `ubuntu-24-04` → `ubuntu-24-04-x64`, etc.
Optionally validate the slug at config time so it fails in `check` rather than
mid-`up` with an opaque 422.
