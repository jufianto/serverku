# Edit or rerun setup for an existing project

`init` creates a new project. To change one you already have, use `edit` for
YAML editing or `reinit` for the setup wizard. Both change local configuration;
they do not create, delete, or redeploy cloud resources.

## Edit YAML in your editor

```bash
serverku edit kuma
```

Serverku uses `$VISUAL`, then `$EDITOR`, then `vim` or `vi`. For example:

```bash
EDITOR=vi serverku edit kuma
VISUAL='vim -f' serverku edit kuma
```

Unset `VISUAL` if you want `EDITOR` to take precedence. Editor command arguments
and quoted paths are supported; shell substitutions and pipelines are not.

In Vim, press `i` to edit, `Esc` then `:wq` to save and exit, or `:q!` to discard.
The editor opens a temporary copy in a private directory. Serverku validates the
YAML and project name after you exit. Valid changes are saved atomically, and the
previous file is backed up under `~/.serverku/backups/`. Comments and formatting
from your edited file are retained.

If validation or the editor fails, the original stays unchanged and the command
prints a recovery-file path. Edit that file to recover your work. A malformed
project with no tracked resources can also be repaired with `serverku edit`.
If another process changes the original while you are editing, serverku refuses
to overwrite the newer version.

You can edit while the VM is running. The save does not modify the running VM.
Changing provider, GCP project, region, or zone is blocked while a VM or disk is
tracked, so existing resources remain reachable using their original placement.

## Rerun the setup wizard

```bash
serverku reinit kuma
```

The existing project must have no tracked VM. Current choices are prefilled.
The wizard shows a change summary and asks whether to save. Canceling leaves the
original unchanged.

Only setup fields change. Notifications and their ntfy topic, sync directory,
routing/DNS, hooks, startup commands, and other settings are preserved. YAML
comments and custom fields are retained. State and shared SSH keys are untouched.
DigitalOcean setup omits the GCP project and zone and disables spot instances.

For scripts, specify only the fields you want to change:

```bash
serverku reinit kuma --non-interactive --size s-2vcpu-2gb
serverku reinit kuma --non-interactive --no-storage=false --storage-gb 10
```

Other available flags are `--provider`, `--project-id`, `--region`, `--zone`,
`--image`, `--spot`, `--mount-path`, and `--compose-file`. Unspecified fields keep
their existing values. A provider change gets an appropriate default image unless
an image is explicitly supplied.

A tracked persistent disk prevents changes to its placement, disabling storage,
or changing its configured size. Reinit does not resize or migrate disks.

## Apply changes

Run the existing checks after editing:

```bash
serverku check kuma
```

VM size/image changes take effect when a new VM is created with `up`, after the
old VM has been brought down. Compose and synced application changes can be
applied with `serverku deploy kuma` where supported. Setting up storage alone
does not move application data into it.

Both commands honor `--config-dir`:

```bash
serverku --config-dir /path/to/config edit kuma
serverku --config-dir /path/to/config reinit kuma
```

## Add persistence to a fresh Kuma installation

The minimal [Kuma tutorial](tutorial-uptime-kuma-digitalocean.md) is stateless.
`down` deletes that VM and Kuma's data. If you need existing monitors, settings,
or history, back up/export the data before bringing it down and restore it into
the new data directory. These commands do not perform that migration.

For a fresh installation, bring the current VM down, then enable storage:

```bash
serverku down kuma
serverku reinit kuma --non-interactive --no-storage=false --storage-gb 10
```

Add a bind mount in your local Compose file:

```yaml
services:
  kuma:
    image: louislam/uptime-kuma:1
    restart: unless-stopped
    ports:
      - "3001:3001"
    volumes:
      - ./kuma-data:/app/data
```

With `storage.mount_path: /data`, serverku writes the Compose file onto the
persistent disk. The relative `./kuma-data` directory therefore lives on that
disk. Then:

```bash
serverku check kuma
serverku up kuma
```

Kuma data now survives `down` followed by `up`. `destroy` deletes the persistent
disk and its data, while keeping the project YAML.
