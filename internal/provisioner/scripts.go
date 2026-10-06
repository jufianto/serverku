package provisioner

import (
	"fmt"
	"strings"
)

// installDockerScript installs Docker Engine and Docker Compose plugin on an
// Ubuntu VM using the official get.docker.com convenience script.
// The serverku SSH user is added to the docker group for non-root access.
//
// A freshly booted Ubuntu droplet is still running cloud-init and
// unattended-upgrades, which hold the apt/dpkg lock. sshd comes up before those
// finish, so we wait for cloud-init to complete and for the dpkg lock to be
// released before running apt -- otherwise apt fails with
// "E: Could not get lock /var/lib/dpkg/lock-frontend".
const installDockerScript = `set -e
sudo cloud-init status --wait >/dev/null 2>&1 || true
for i in $(seq 1 60); do
  if ! sudo fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/lib/apt/lists/lock >/dev/null 2>&1; then
    break
  fi
  echo "Waiting for apt/dpkg lock to be released (attempt $i)..."
  sleep 5
done
sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl ca-certificates
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker serverku
sudo systemctl enable docker
sudo systemctl start docker`

// diskDevicePath returns the stable /dev/disk/by-id path for an attached
// volume, which differs per provider:
//   - GCP exposes it as /dev/disk/by-id/google-<diskName>
//   - DigitalOcean exposes it as /dev/disk/by-id/scsi-0DO_Volume_<volumeName>
//
// Using the by-id path avoids the instability of /dev/sdb, which can shift.
func diskDevicePath(provider, diskName string) string {
	switch provider {
	case "digitalocean":
		return fmt.Sprintf("/dev/disk/by-id/scsi-0DO_Volume_%s", diskName)
	default: // gcp (and anything else defaults to the GCP convention)
		return fmt.Sprintf("/dev/disk/by-id/google-%s", diskName)
	}
}

// mountDiskScript returns a shell script that formats (if new) and mounts the
// provider's persistent volume to mountPath, using the stable by-id device path
// (see diskDevicePath) instead of /dev/sdb which can shift.
//
// Safety: blkid is run first. If a filesystem already exists, mkfs is skipped
// to protect user data across VM lifecycles.
func mountDiskScript(provider, diskName, mountPath string) string {
	device := diskDevicePath(provider, diskName)
	return fmt.Sprintf(`set -e
DEVICE="%s"
MOUNT_PATH="%s"

# Wait for the device to appear (it may take a moment after attach)
for i in $(seq 1 10); do
  test -e "$DEVICE" && break
  echo "Waiting for device $DEVICE (attempt $i)..."
  sleep 2
done
test -e "$DEVICE" || { echo "Device $DEVICE not found after waiting"; exit 1; }

# Format only if no filesystem is present (protects existing data)
if ! sudo blkid "$DEVICE" > /dev/null 2>&1; then
  echo "No filesystem detected on $DEVICE, formatting with ext4..."
  sudo mkfs.ext4 -F "$DEVICE"
else
  echo "Existing filesystem detected on $DEVICE, skipping format."
fi

# Mount the disk
sudo mkdir -p "$MOUNT_PATH"
sudo mount "$DEVICE" "$MOUNT_PATH"

# Add to /etc/fstab for persistence across reboots (only if not already there)
if ! grep -q "$DEVICE" /etc/fstab; then
  echo "$DEVICE $MOUNT_PATH ext4 defaults,nofail 0 2" | sudo tee -a /etc/fstab
fi

# Allow the serverku user to write to the mount path
sudo chown serverku:serverku "$MOUNT_PATH"
`, device, mountPath)
}

// writeComposeScript returns a shell script that writes composeContent to
// targetDir/docker-compose.yml on the VM using a heredoc. The SERVERKU_COMPOSE_EOF
// delimiter is unlikely to appear in any real compose file.
func writeComposeScript(composeContent string, targetDir string) string {
	// The closing heredoc delimiter must start on its own line, even when
	// the source file has no trailing newline.
	if !strings.HasSuffix(composeContent, "\n") {
		composeContent += "\n"
	}
	return fmt.Sprintf(`set -e
sudo mkdir -p "%s"
sudo chown serverku:serverku "%s"
cat > "%s/docker-compose.yml" << 'SERVERKU_COMPOSE_EOF'
%sSERVERKU_COMPOSE_EOF
`, targetDir, targetDir, targetDir, composeContent)
}

// composeUpScript returns a shell script that runs docker compose up -d in
// the given directory.
//
// docker is invoked via sudo: `usermod -aG docker serverku` only takes effect
// in a new login session, but provisioning reuses the SSH session opened before
// the group was added, so the serverku user cannot yet reach the docker socket.
// The daemon runs as root, so sudo is the reliable path. (Interactive
// `serverku ssh` later gets a fresh login where the docker group is active.)
func composeUpScript(composeDir string) string {
	return fmt.Sprintf(`set -e
cd "%s"
sudo docker compose up -d
`, composeDir)
}

// teardownScript returns a shell script that stops containers and unmounts the
// disk. Each step is best-effort (failures are logged but don't abort the
// script) because the VM is being destroyed immediately after teardown.
func teardownScript(composeDir string, mountPath string, hasCompose bool, hasStorage bool) string {
	script := "set +e\n" // best-effort: don't exit on error
	if hasCompose {
		script += fmt.Sprintf(`
echo "Stopping Docker containers..."
cd "%s" && sudo docker compose down || true
`, composeDir)
	}
	if hasStorage {
		script += fmt.Sprintf(`
echo "Unmounting disk at %s..."
sudo umount "%s" || true
`, mountPath, mountPath)
	}
	return script
}

// installCaddykuScript returns a shell script that downloads and installs caddyku
func installCaddykuScript() string {
	return `set -e
echo "Installing caddyku..."
curl -sSL https://github.com/jufianto/caddyku/releases/latest/download/caddyku_linux_amd64.tar.gz | tar -xz
sudo mv caddyku /usr/local/bin/
`
}

// initCaddyProxyScript returns a shell script that runs caddyku init
func initCaddyProxyScript() string {
	return `set -e
echo "Initializing caddy proxy network..."
caddyku init
cd ~/projects/caddy-proxy
sudo docker compose up -d
`
}

// configureAppDomainsScript returns a bash command that loops over domains and configures them
func configureAppDomainsScript(domains []string, services []string, upstreams []string, composeDir string) string {
	script := fmt.Sprintf("set -e\ncd \"%s\"\n", composeDir)
	for i, domain := range domains {
		script += fmt.Sprintf("echo \"Configuring domain %s...\"\n", domain)
		script += fmt.Sprintf("caddyku init-app --service %s --domain %s --upstream %s\n", services[i], domain, upstreams[i])
	}
	return script
}

// heartbeatScript returns a shell script that installs the on-VM heartbeat:
// a reporting script plus a systemd timer that fires every hb.Hours hours
// while the VM runs, sending to every configured channel (Telegram, ntfy).
// Because it lives on the VM it works while the local machine is offline and
// dies with the VM, so a reminder can never outlive the resource it warns
// about.
//
// Cost provenance is preserved: a live provider rate renders without markers,
// an offline table rate renders with est. markers, and no rate omits cost.
func heartbeatScript(hb HeartbeatOpts) string {
	costLine := `COST=""`
	if hb.HourlyRateUSD > 0 {
		format := `, ~$%.2f est. so far (~$` + fmt.Sprintf("%.4f", hb.HourlyRateUSD) + `/hr est.)`
		if hb.RateIsLive {
			format = `, $%.2f so far ($` + fmt.Sprintf("%.4f", hb.HourlyRateUSD) + `/hr)`
		}
		costLine = fmt.Sprintf(
			`COST=$(awk -v s="$STARTED" -v n="$NOW" 'BEGIN { printf "%s", (n-s)/3600*%.6f }')`,
			format, hb.HourlyRateUSD,
		)
	}

	// One send per configured channel; failures are independent (no set -e in
	// the reporter) so one channel being down does not silence the other.
	var sends string
	if hb.BotToken != "" && hb.ChatID != "" {
		sends += fmt.Sprintf(`curl -fsS -m 10 "https://api.telegram.org/bot%s/sendMessage" \
  -d chat_id="%s" --data-urlencode "text=${TEXT}" >/dev/null
`, hb.BotToken, hb.ChatID)
	}
	if hb.NtfyTopic != "" {
		sends += fmt.Sprintf(`curl -fsS -m 10 -H "Title: serverku: %s still running" -H "Priority: high" -H "Tags: warning,moneybag" \
  -d "${TEXT}" "%s/%s" >/dev/null
`, hb.ProjectName, hb.NtfyServer, hb.NtfyTopic)
	}

	reporter := fmt.Sprintf(`#!/bin/sh
STARTED=$(cat /var/lib/serverku/heartbeat-started)
NOW=$(date +%%s)
UPH=$(( (NOW - STARTED) / 3600 ))
UPM=$(( ((NOW - STARTED) %% 3600) / 60 ))
%s
TEXT="serverku: %s still running -- up ${UPH}h${UPM}m${COST}. Stop with: serverku down %s"
%s`, costLine, hb.ProjectName, hb.ProjectName, sends)

	return fmt.Sprintf(`set -e
sudo mkdir -p /var/lib/serverku
date +%%s | sudo tee /var/lib/serverku/heartbeat-started >/dev/null

sudo tee /usr/local/bin/serverku-heartbeat >/dev/null <<'SERVERKU_HB'
%s
SERVERKU_HB
sudo chmod 0700 /usr/local/bin/serverku-heartbeat

sudo tee /etc/systemd/system/serverku-heartbeat.service >/dev/null <<'SERVERKU_HB'
[Unit]
Description=serverku Telegram heartbeat

[Service]
Type=oneshot
ExecStart=/usr/local/bin/serverku-heartbeat
SERVERKU_HB

sudo tee /etc/systemd/system/serverku-heartbeat.timer >/dev/null <<'SERVERKU_HB'
[Unit]
Description=serverku Telegram heartbeat timer

[Timer]
OnActiveSec=%dh
OnUnitActiveSec=%dh

[Install]
WantedBy=timers.target
SERVERKU_HB

sudo systemctl daemon-reload
sudo systemctl enable --now serverku-heartbeat.timer
`, reporter, hb.Hours, hb.Hours)
}
