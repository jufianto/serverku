package provisioner

import "fmt"

// installDockerScript installs Docker Engine and Docker Compose plugin on an
// Ubuntu VM using the official get.docker.com convenience script.
// The serverku SSH user is added to the docker group for non-root access.
const installDockerScript = `set -e
sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl ca-certificates
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker serverku
sudo systemctl enable docker
sudo systemctl start docker`

// mountDiskScript returns a shell script that formats (if new) and mounts the
// GCP persistent disk to mountPath. The stable device path
// /dev/disk/by-id/google-<diskName> is used instead of /dev/sdb which can shift.
//
// Safety: blkid is run first. If a filesystem already exists, mkfs is skipped
// to protect user data across VM lifecycles.
func mountDiskScript(diskName string, mountPath string) string {
	device := fmt.Sprintf("/dev/disk/by-id/google-%s", diskName)
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
	return fmt.Sprintf(`set -e
sudo mkdir -p "%s"
sudo chown serverku:serverku "%s"
cat > "%s/docker-compose.yml" << 'SERVERKU_COMPOSE_EOF'
%sSERVERKU_COMPOSE_EOF
`, targetDir, targetDir, targetDir, composeContent)
}

// composeUpScript returns a shell script that runs docker compose up -d in
// the given directory.
func composeUpScript(composeDir string) string {
	return fmt.Sprintf(`set -e
cd "%s"
docker compose up -d
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
cd "%s" && docker compose down || true
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
