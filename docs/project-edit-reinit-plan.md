# Project editing and reinitialization plan

Status: implemented. See [Editing projects](../guides/editing-projects.md) for usage.
This document records the agreed design and verification scope.

## Existing behavior and guidance

- `cmd/serverku/init.go` rejects a project whose YAML already exists.
- Interactive init collects provider, region, GCP project/zone, VM size/spot,
  storage, and Compose path. It does not expose every ProjectConfig field.
- Project YAML is stored in `<config-dir>/projects/<name>.yaml`; runtime state
  and shared SSH keys are stored separately. The default config dir is
  `~/.serverku/`.
- The [README configuration reference](../README.md#configuration-reference)
  describes supported fields. The [Kuma tutorial](../guides/tutorial-uptime-kuma-digitalocean.md)
  and [WordPress tutorial](../guides/tutorial-wordpress-digitalocean.md) currently
  tell users to edit YAML manually.
- `destroy` currently keeps the project YAML. README cleanup descriptions and
  command-table entries saying it deletes that YAML need correction.

## Commands

### `serverku edit <project>`

Open an existing project's YAML in `$VISUAL`, then `$EDITOR`, otherwise `vim`,
then `vi`. Connect terminal input/output directly so the editor is interactive.
Support editor arguments without executing the configuration through a shell.
Respect `--config-dir` and report the resolved project path.

Edit a temporary copy in a private directory beside the original. After the editor exits successfully,
parse YAML, validate ProjectConfig, and verify that `name` still matches the
requested project. Only then replace the original atomically and retain a
backup. Saving must preserve the edited YAML text, comments, and formatting.
Protect temporary files and backups with restrictive permissions because
project YAML can contain notification credentials.

A missing editor, editor failure, or invalid result must leave the original
untouched. Retain invalid edits in a recovery file and show its path and the
validation error. Allow opening an existing malformed config so it can be
repaired; loading it into ProjectConfig must not be required before editing.

Canceling or making no changes should leave the file untouched. This command
changes local configuration only; it does not deploy or recreate anything.

### `serverku reinit <project>`

Require an existing project and rerun the init wizard with its current values
prefilled. Keep `init` as the new-project command. An existing-project error
from `init` should suggest both `edit` and `reinit`.

Refactor the wizard to return a candidate configuration instead of saving it
itself. Init and reinit can then share provider-specific prompts and validation
without duplicating their behavior.

Merge only wizard-managed fields into the existing YAML. Preserve image
selection unless a provider change requires choosing an appropriate default.
Preserve sync directory, router/DNS, notifications and ntfy topic, hooks,
startup commands, VM time limits, and other fields the wizard does not expose.
Preserve the project name and shared SSH keys. For DigitalOcean, omit GCP
project/zone fields and keep spot disabled.

Show a concise change summary before saving. Validate and write atomically with
a backup, using the same save mechanism as edit. Canceling leaves the original
untouched. Reinit must not erase runtime state or create/delete cloud resources.

For non-interactive use, reuse init flags and modify only explicitly supplied
values; unspecified fields retain their existing values.

## Existing resources and applying changes

Reinit should refuse while a VM is tracked and explain that the current VM must
be brought down first. For a stateless project like the current Kuma setup,
`down` deletes the app data with the VM; say this explicitly in the guidance.

Edit may run while a VM is up, but changing the YAML does not change that VM.
Resource placement fields (provider, GCP project, region, zone) must not change
while an old VM or persistent disk is tracked. Apply this check to both commands.
State is needed to operate on those resources in their original location.

An existing persistent disk also needs special treatment: changing disk size or
type does not resize it, and disabling storage would strand it. Reject unsupported
storage changes while a disk is tracked with an actionable explanation. Enabling
storage on a stopped stateless project is supported: the next `up` creates a disk.
The app's Compose file must separately mount its data directory onto that disk;
reinit does not migrate existing application data.

After a successful save, explain the next step: configuration takes effect on
future lifecycle operations; VM creation settings require recreation. Compose
and application changes may be applied with `deploy` where supported.

## Documentation changes

1. Add both commands and examples to the README command table and config section.
2. Add `guides/editing-projects.md`, covering editor selection, backups, reinit,
   applying changes, and running-project/persistent-disk constraints.
3. Link the guide from `guides/README.md` and the setup-decision guide.
4. Update Kuma and WordPress manual-edit instructions to use `serverku edit`.
5. Add a Kuma example for opting into persistent storage, including an `/app/data`
   bind mount. Explain that its current stateless data needs a separate migration
   before deleting the VM if it must be retained.
6. Correct stale README statements about destroy deleting the local project YAML.

## Implementation and verification order

1. Add shared project-path resolution and safe validated-save/backup handling.
2. Implement edit and verify it with a fake editor, including paths with spaces,
   editor precedence, invalid YAML, editor failure, and repairing invalid input.
3. Refactor init and implement reinit; verify preservation of non-wizard fields,
   explicit-flag behavior, provider-specific fields, cancel behavior, and resource
   constraints. Assert that state/keys are unchanged and no cloud mutations occur.
4. Update guides and command help.
5. Run targeted tests, `go test ./...`, and build. Once implementation is complete,
   rebuild the user's installed Mac command as previously requested.

No application or cloud changes are part of preparing this plan.
