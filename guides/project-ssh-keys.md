# Project SSH keys

Serverku generates a separate Ed25519 key for each project by default:

```text
~/.serverku/keys/kuma/id_ed25519
~/.serverku/keys/kuma/id_ed25519.pub
```

`init` and `check` prepare the local key. `up` registers it as `serverku-kuma`
in DigitalOcean, or injects it through VM metadata in GCP. Provisioning, SSH,
logs, deployment, teardown, and tunnels all use the same recorded identity.

`down` keeps keys so the next `up` can reuse them. `destroy` removes the local
generated key and its owned DigitalOcean account registration. Keys created
outside this project, custom keys, and keys referenced by another project are
retained. Registration is tracked before creating the VM, so a failed startup
does not lose ownership information. Failed key cleanup retains state and the
private key so you can retry after fixing API access.

## Use your own key

For a new project, pass an existing unencrypted private key:

```bash
serverku init kuma --ssh-key ~/.ssh/my-kuma-key
```

Or use `serverku edit kuma` before the first startup and add:

```yaml
ssh:
  private_key: ~/.ssh/my-kuma-key
```

The public key is derived from the private key; no `.pub` file is required.
Relative key paths resolve against the project YAML directory. Custom keys are
never deleted by `destroy`, including account registrations made for them.
Encrypted private keys and agent-only authentication are not supported.

`reinit --non-interactive --ssh-key <path>` also updates this setting when no
SSH identity or VM is tracked. Changing a tracked identity requires destroying
its resources first; back up persistent data before doing so. This prevents an
edit from silently breaking access or abandoning a registered key.

## Existing projects

An existing VM without recorded key metadata keeps using the legacy shared
`~/.serverku/keys/serverku_rsa`. Serverku does not change its remote access.
After that VM is removed, its next `up` uses a new project key. A recorded key
that goes missing or changes identity causes an error instead of generating a
replacement that cannot access the existing VM.

`destroy` does not automatically delete the legacy shared key. Remove its
DigitalOcean registration and local files only after all old VMs using it are
gone or have been migrated. Deleting an account registration does not revoke
the key already installed inside an existing VM.
