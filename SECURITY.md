# Security Policy

serverku handles cloud credentials and SSH keys, so security reports are
taken seriously.

## Supported versions

Only the latest release is supported with security fixes.

## Reporting a vulnerability

Please **do not open a public issue** for security vulnerabilities.
Instead, use GitHub's private vulnerability reporting on this repository
(Security tab → "Report a vulnerability").

You can expect an acknowledgement within a few days. Please include steps
to reproduce and the impact you believe the issue has.

## Security model (what to measure reports against)

- Cloud credentials (`DIGITALOCEAN_TOKEN`, GCP Application Default
  Credentials) live only on the user's machine and are never transferred
  to VMs or written by serverku.
- The managed SSH private key stays local (`~/.serverku/keys/`); only the
  public key is injected into VMs.
- The opt-in heartbeat places one sending credential on the VM in a
  root-only (0700) script: the ntfy topic name (account-less) or a
  Telegram bot token (revocable). This trade-off is documented in the
  README; reports that *escalate beyond* that documented blast radius are
  in scope.
- Local lifecycle hooks execute arbitrary commands from project configs by
  design; running untrusted configs is documented as unsafe.
