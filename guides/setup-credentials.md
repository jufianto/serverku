# Setting up cloud credentials

Before serverku can create anything, it needs to talk to your cloud account.
This guide explains **what you need, how serverku authenticates, and where your
credentials live** — for both DigitalOcean and GCP.

The golden rule: **serverku never handles your cloud password.** It reads
credentials that already live on your machine, and `serverku setup <provider>`
puts them in place and **verifies them with a real API call** before you create
anything billable. Both flows print the authenticated account so you can confirm
you're using the right one.

```bash
serverku setup digitalocean   # paste an API token; verified and saved
serverku setup gcp            # isolated Google login, pick a project, verified
```

## The mental model (both providers)

- serverku acts **as you** — using a credential stored locally, never a password.
- **Precedence is the same idea on both clouds:** an environment variable you set
  yourself always wins; otherwise serverku uses the credential it saved; otherwise
  it falls back to the system default.
- `serverku check <project>` re-verifies a project's credentials any time.

---

## DigitalOcean

### What you need
A **personal access token** with write scope from
<https://cloud.digitalocean.com/account/api/tokens>.

### How it works
`serverku setup digitalocean`:

1. Prompts for the token (input is masked).
2. **Verifies it** against the DigitalOcean account API — a bad token is
   rejected on the spot — and prints the account email.
3. Saves it to `~/.serverku/credentials.yaml` with owner-only `0600` permissions.

From then on serverku uses it automatically. There's no OAuth — DigitalOcean
auth is just this token. (DO droplets are also open by default, so there's no
firewall step to configure.)

**Resolution order:** `DIGITALOCEAN_TOKEN` environment variable (if set) →
the saved `credentials.yaml`.

---

## GCP

GCP is where people get confused, because **three different things are all
called "gcloud"** — and serverku only uses one of them:

| Thing | What it is | Does serverku use it? |
| --- | --- | --- |
| `gcloud auth login` | the account the **gcloud CLI** acts as | ❌ No |
| gcloud config default project | a default **for gcloud's own commands** | ❌ No (serverku uses `project_id` from your project config) |
| **ADC** (Application Default Credentials) | a **credential file** that client libraries read | ✅ **Yes — only this** |

### What you need
- The [gcloud CLI](https://cloud.google.com/sdk/docs/install) installed.
- A GCP **project** with **billing enabled** and the **Compute Engine API** on.

### How it works
`serverku setup gcp`:

1. Runs the Google login flow — but writes the resulting ADC into serverku's
   **own** directory, `~/.serverku/gcloud/` (via `CLOUDSDK_CONFIG`), so your
   system-wide `~/.config/gcloud` is **never touched**.
2. Shows the authenticated account (`Authenticated as: you@example.com`).
3. **Lists the projects that account can see** and lets you pick one.
4. Records your choice as the ADC quota project and verifies Compute access.

serverku then points `GOOGLE_APPLICATION_CREDENTIALS` at that isolated ADC for
all of its GCP calls, and `serverku init --provider gcp` pre-fills `project_id`
from the project you picked.

**Resolution order:** a `GOOGLE_APPLICATION_CREDENTIALS` you set yourself →
serverku's isolated ADC (`~/.serverku/gcloud/`) → the system default ADC.

serverku drives the official `gcloud` flow — it never handles Google OAuth
itself.

### Keeping work and personal accounts separate

Because `setup gcp` keeps its credentials in serverku's own directory, it **does
not touch your everyday gcloud login**. So if your daily `gcloud` is signed into
a work account but you want serverku to deploy under a personal one, just run:

```bash
serverku setup gcp   # log in with your PERSONAL account, then pick a personal project
```

Your work `gcloud`/ADC stay exactly as they were.

> **Note on projects:** serverku uses the `project_id` in your project config.
> If a `serverku` command 403s with a project name you didn't expect (e.g. a
> work project), it usually means the credential is one account but the project
> belongs to another — pick the matching project during `setup gcp`.

### Checking which Google account an ADC belongs to

`serverku setup gcp` prints it. To check the raw credentials yourself, ask
Google to introspect the token:

```bash
# the system-default ADC:
gcloud auth application-default print-access-token \
  | { read t; curl -s "https://www.googleapis.com/oauth2/v3/tokeninfo?access_token=$t"; }

# serverku's isolated ADC:
CLOUDSDK_CONFIG="$HOME/.serverku/gcloud" \
  gcloud auth application-default print-access-token \
  | { read t; curl -s "https://www.googleapis.com/oauth2/v3/tokeninfo?access_token=$t"; }
```

The response includes an `"email"` field.

---

## Where credentials live

| Provider | Stored at | Form | Wins over the stored value |
| --- | --- | --- | --- |
| DigitalOcean | `~/.serverku/credentials.yaml` (`0600`) | API token | `DIGITALOCEAN_TOKEN` env var |
| GCP | `~/.serverku/gcloud/` (isolated ADC) | ADC file | `GOOGLE_APPLICATION_CREDENTIALS` env var |

Both live under your serverku config dir (override with `--config-dir`), kept
out of `projects/` because that directory is shareable config.

## Bring your own (advanced)

`serverku setup` is a convenience, not a requirement — if you already manage
credentials your own way, serverku picks them up:

- **DigitalOcean:** `export DIGITALOCEAN_TOKEN=...`
- **GCP:** set `GOOGLE_APPLICATION_CREDENTIALS` to a user ADC file **or a
  service-account key** (headless/CI). It always wins over serverku's stored
  credentials.

Either way, run `serverku check <project>` to confirm access before your first
`up`.

**Next:** [Choosing your setup](choosing-your-setup.md) — picking provider,
size, disk, and spot.
