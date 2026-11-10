# Install WordPress on DigitalOcean with PR #21

Deploy WordPress and MySQL, finish the WordPress installer, publish a post,
upload an image, then replace the VM and bring the same site back. This is a
real application deployment through serverku, with acceptance checks along
the way.

The stack uses a DigitalOcean Droplet, a 10 GiB persistent volume, and an SSH
tunnel. Open WordPress at **http://localhost:8080** while the tunnel runs.
The same address works after every VM replacement, so WordPress's stored
site URL does not depend on the Droplet's changing IP. For a public site,
follow the [domain and HTTPS guide](tutorial-wordpress-digitalocean.md) after
this deployment works; this walkthrough keeps routing and DNS disabled.

This guide targets PR #21 at `0158a882c700e07e30042eef30117b044cf6de1c`.
Its cloud deployment has not been executed as part of writing this guide.

## 1. Build the CLI and prepare your app folder

Start in your serverku repository. You need Go, `ssh`, `rsync`, and `openssl`
locally, plus a DigitalOcean account with permission to create Droplets,
volumes, and SSH keys. Local Docker is optional; Docker is installed on the
Droplet by serverku.

```bash
go build -o serverku ./cmd/serverku
export WP_SERVERKU="$PWD/serverku"
export WP_ROOT="$HOME/deploys/wordpress-pr21"

# Choose another folder if this one already exists; don't overwrite an old site.
if test -e "$WP_ROOT"; then
  echo "Folder already exists: $WP_ROOT. Choose a new WP_ROOT before continuing."
else
  mkdir -p "$WP_ROOT/app"
fi
```

Continue only with a new app folder. Keep this terminal open; later commands
use `WP_ROOT` and `WP_SERVERKU`.

Copy the supplied [Compose file](wordpress_guides/docker-compose.pr21.yml):

```bash
cp guides/wordpress_guides/docker-compose.pr21.yml "$WP_ROOT/app/docker-compose.yml"
```

It runs two services:

| Service | Image | Persistent files on the VM |
| --- | --- | --- |
| MySQL | `mysql:8.4.11` | `/data/mysql` |
| WordPress | `wordpress:7.1.2-php8.3-apache` | `/data/wordpress` |

The full WordPress directory is retained, including `wp-config.php`, uploads,
plugins, and themes. Both services use relative bind mounts, so their data
lives on the attached volume when Compose runs from `/data`. MySQL's health
check waits until the WordPress database user can query its database before
WordPress starts. The image settings follow the official
[WordPress](https://github.com/docker-library/docs/blob/master/wordpress/README.md)
and [MySQL](https://github.com/docker-library/docs/blob/master/mysql/README.md)
Docker image documentation.

Generate database passwords once, into the app's `.env` file:

```bash
(umask 077
  printf 'WP_DB_PASSWORD=%s\nWP_DB_ROOT_PASSWORD=%s\n' \
    "$(openssl rand -hex 32)" "$(openssl rand -hex 32)" > "$WP_ROOT/app/.env"
)
```

Keep this file. Serverku syncs it with the app over SSH; Compose reads it
beside `docker-compose.yml`. Reuse the same passwords on subsequent `up`
commands. Changing `.env` does not change passwords in an existing MySQL
database. Do not put copies of the remote `mysql` or `wordpress` data folders
into the local app folder: syncing them could overwrite live site data.

## 2. Set up DigitalOcean and initialize the project

Use a separate serverku config directory for this deployment. It holds this
site's YAML, runtime state, and SSH keypair:

```bash
skwp() { "$WP_SERVERKU" --config-dir "$WP_ROOT/config" "$@"; }
skwp setup digitalocean
```

Enter your API token locally when prompted and check the account printed by
setup. An existing `DIGITALOCEAN_TOKEN` environment variable takes precedence
over the saved token; ensure it identifies the intended account. See
[credential setup](setup-credentials.md) for details.

Initialize with 2 GiB RAM for this two-service stack and a 10 GiB volume:

```bash
skwp init wp-pr21 --non-interactive --provider digitalocean \
  --region sgp1 --size s-1vcpu-2gb --spot=false --storage-gb 10
```

The live catalog checks region and size availability when credentials work.
This PR still needs the explicit DO image slug below. Before deploying,
confirm its availability in your account; if you already have `doctl`, use
`doctl compute image list-distribution --public`. Pick an available Ubuntu
22.04 x64 image, or an available Ubuntu 24.04 x64 image if 22.04 is unavailable.

Write this site's config:

```bash
cat > "$WP_ROOT/config/projects/wp-pr21.yaml" <<CONFIG
name: wp-pr21
provider: digitalocean
region: sgp1
vm:
  size: s-1vcpu-2gb
  image: ubuntu-22-04-x64
  spot: false
storage:
  enabled: true
  size_gb: 10
  mount_path: /data
compose_file: $WP_ROOT/app/docker-compose.yml
sync_dir: $WP_ROOT/app
router:
  enabled: false
dns:
  enabled: false
notifications: {}
CONFIG

skwp check wp-pr21
```

All preflight checks should pass. `check` does not create resources and does
not currently validate the image slug. Notifications are disabled for this
walkthrough.

## 3. Deploy WordPress

This step creates billable cloud resources:

```bash
skwp up wp-pr21
skwp status wp-pr21
```

Serverku creates the volume and Droplet, provisions the `serverku` login user,
attaches storage, installs Docker, mounts `/data`, syncs the app and `.env`,
then runs Compose. Initial image downloads and database initialization can
take several minutes.

The WordPress port binds to the Droplet's loopback interface. Open a second
terminal, change to the serverku repository, and run:

```bash
./serverku --config-dir "$HOME/deploys/wordpress-pr21/config" tunnel wp-pr21 8080:8080
```

If you chose a different `WP_ROOT`, use its `config` path here. Leave this
terminal running. Closing the tunnel does not stop the Droplet.

## 4. Finish the installer and use your site

Open **http://localhost:8080** in your browser:

1. Choose your language.
2. Set the site title to **My serverku blog**.
3. Choose an administrator username and a strong password; save the login.
4. Enter your email and select **Install WordPress**.
5. Log into **http://localhost:8080/wp-admin**.
6. Under **Posts → Add New**, publish a post titled **My first serverku deploy**.
7. Under **Media → Add New**, upload an image, then add it to the post and save.
8. Change the site title or activate an installed theme so you also have an
   application setting to check later.

Visit the homepage and the post. Confirm the uploaded image loads. These are
the acceptance checks: the database stores real content/settings, and the
WordPress filesystem stores a real upload.

If WordPress shows a database error, wait for initialization and inspect the
services rather than running the installer again. From the first terminal:

```bash
skwp ssh wp-pr21
```

On the Droplet:

```bash
findmnt /data
cd /data
sudo docker compose ps
sudo docker compose logs --tail=80 db wordpress
exit
```

Expect `/data` to be mounted, MySQL to be healthy, and WordPress to be running.
Keep logs containing credentials private. An empty Compose file, missing
`.env`, failed Docker installation, or missing DO volume device should be
resolved before continuing.

## 5. Replace the VM and confirm your site survives

Close the tunnel with Ctrl+C, then use the first terminal:

```bash
skwp down wp-pr21
skwp status wp-pr21
```

Verify in the DigitalOcean console that `serverku-wp-pr21` is gone and
`serverku-wp-pr21-data` still exists. The site is offline; the volume retains
the database and WordPress files.

Bring it back:

```bash
skwp up wp-pr21
```

Restart the same tunnel command in the second terminal and revisit
**http://localhost:8080**. Verify all of these:

- WordPress shows the existing site, not a fresh installer.
- Your administrator login still works.
- The published post and uploaded image are present.
- The site title/theme change survived.

The new Droplet gets a new IP; the tunnel keeps the browser URL unchanged.
If you see the installer, stop and investigate the attached volume and bind
mounts instead of installing a second empty site.

## 6. Optional: back up the installed site

Close the tunnel and stop compute so MySQL shuts down before the snapshot:

```bash
skwp down wp-pr21
skwp backup wp-pr21 --name wp-pr21-installed
```

Record the snapshot ID. The snapshot is a separately billed resource and
survives `down` but is permanently deleted by `destroy`. This verifies backup creation; it is not proof of restore.
Serverku has a `restore` command, but restore testing is a separate destructive
scenario involving a new volume and explicit cleanup of the previous volume.
Do not restore over this installed site just to finish the first walkthrough.

## 7. Keep the site, or remove it

To keep the installation for another day, leave it stopped after `down`.
Only storage (and any snapshot) continues billing. Run `up` and reopen the
tunnel when you want to use it.

To permanently delete the site:

```bash
skwp destroy wp-pr21
```

Confirm the prompt only if you intend to delete the site's persistent data.
PR #21 removes the Droplet, project volumes and snapshots, and generated
project SSH keys/owned account registrations. It clears resource tracking, records status `destroyed`, and
keeps the project YAML. Custom or shared SSH keys are retained. Verify
resource deletion in the DO console; keep local credentials/state until
cloud cleanup succeeds, including after a failed `up` or `destroy`.

Droplets bill while they exist, including when powered off. Volumes bill
while they exist, including when detached. The 10 GiB volume has a $1/month
rate, accrued hourly; check the current catalog price for `s-1vcpu-2gb`
before provisioning. Sources: [Droplet pricing](https://docs.digitalocean.com/products/droplets/details/pricing/)
and [volume pricing](https://docs.digitalocean.com/products/volumes/details/pricing/).
