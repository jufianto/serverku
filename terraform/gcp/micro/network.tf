resource "google_compute_network" "serverku-network" {
  name = "custom-serverku"
  project = var.projectID
}

resource "google_compute_subnetwork" "serverku-sub-network" {
  project = var.projectID
  name          = "serverku-subnetwork"
  ip_cidr_range = "10.17.0.0/16"
  region        = var.region
  network       = google_compute_network.serverku-network.id
}

resource "google_compute_firewall" "serverku-firewall" {
  project = var.projectID
  name    = "serverku-firewall"
  network = google_compute_network.serverku-network.id
  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["80", "22" ,"8080", "9000-9999"]
  }

  target_tags = ["serverku-fw"]
  source_ranges = [ "0.0.0.0/0" ]
}