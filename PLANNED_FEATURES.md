# Planned Features for Serverku & Caddyku Integration

Based on our discussion, here is a breakdown of the features we want to build. This document serves as a blueprint before we formally propose them via OpenSpec.

## 1. Project Artifact Sync (SCP/Rsync)
**Goal:** Allow users to securely transfer their entire project directory (including `.env` files, configs, and source code) to the remote VM, rather than just copying a single `docker-compose.yml`.
*   **How it works:** 
    *   Update `ProjectConfig` to support a `Sync` field (e.g., `sync_dir: ./my-app`).
    *   During the `serverku up` provisioning phase, `serverku` uses an internal SCP or Rsync wrapper to push the local directory to the VM's persistent storage.
    *   This naturally solves the need for `.env` files, custom `nginx.conf`, or `build: .` directives in the compose file.
    *   When `docker compose up -d` runs on the VM, it has access to the full project context.

## 2. Interactive `serverku init`
**Goal:** Make onboarding frictionless so users don't have to manually write YAML files.
*   **How it works:**
    *   When running `serverku init <project>`, the CLI will prompt the user interactively.
    *   "Which provider? [GCP, DigitalOcean]"
    *   "Which region? [sgp1, nyc1, etc.]"
    *   "VM Size? [s-1vcpu-1gb, etc.]"
    *   "Enable persistent storage? [Y/n]" -> "Size in GB?"
    *   "Path to docker-compose file?"
    *   "Do you want to configure domains for automatic HTTPS? [Y/n]"

## 3. Automatic HTTPS & Routing via `caddyku` Integration
**Goal:** Provide zero-touch reverse proxy and HTTPS for applications deployed via `serverku`, using your existing `caddyku` project.
*   **How it works (in `serverku`):**
    *   Add a `Domains` configuration to `serverku`'s project YAML.
    *   If domains are configured, the `SSHProvisioner` will:
        1. Download the latest `caddyku` binary to the VM.
        2. Run `caddyku init` on the VM to bootstrap the global `caddy-proxy`.
        3. Use `caddyku init-app` (or generate a `caddyku.yaml`) on the VM for the user's uploaded `docker-compose.yml`.
        4. Start the stack.
*   **Does `caddyku` need updates?**
    *   Currently, `caddyku` is excellent for VPS environments. Since `serverku` deploys standard `docker-compose.yml` files, `caddyku init-app` can automatically patch them on the server to join the `caddy-net` network. 
    *   *We need to ensure `caddyku` can run non-interactively in a bash script (which it seems to support via CLI flags).*

## 4. (Optional) DNS Automation
**Goal:** Automatically point the user's domain to the newly created VM.
*   **How it works:**
    *   Since `serverku` knows the cloud provider and the new `ExternalIP`, it could call the DigitalOcean or GCP DNS APIs to update the `A` record for the domain dynamically on `serverku up`.

---

### Next Steps
If you agree with this scope, we can create OpenSpec proposals for these features. We can start with **`.env` Support** and the **Interactive `init`**, and then tackle the **`caddyku` Integration**! 

Let me know if you want to modify this plan or if you want to jump straight into proposing one of these using `/opsx:propose`!