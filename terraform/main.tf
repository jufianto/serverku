resource "google_compute_network" "vpc_network" {
  name                    = "my-custom-mode-network"
  project                 = ""
  auto_create_subnetworks = false
  mtu                     = 1460
}

resource "google_compute_subnetwork" "default" {
  name          = "my-custom-subnet"
  ip_cidr_range = "10.0.1.0/24"
  region        = "asia-southeast2"
  project       = ""
  network       = google_compute_network.vpc_network.id
}
# Create a single Compute Engine instance
resource "google_compute_instance" "default" {
  name         = "test-vm"
  machine_type = "f1-micro"
  zone         = "asia-southeast2-a"
  tags         = ["ssh"]
  project      = ""


  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-11"
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.default.id

    access_config {
      # Include this section to give the VM an external IP address
    }
  }
}
