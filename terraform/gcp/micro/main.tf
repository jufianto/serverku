resource "google_compute_instance" "default" {
  name         = "vm-serverku-medium"
  machine_type = "e2-medium"
  zone         = var.zone
  tags         = ["ssh", "serverku-fw"]
  project      = var.projectID

    boot_disk {
    auto_delete = true
    device_name = var.boot-disk-name

    initialize_params {
      image = "projects/ubuntu-os-cloud/global/images/ubuntu-2004-focal-v20230907"
      size  = 10
      type  = "pd-standard"
    }

    mode = "READ_WRITE"
  }

    scheduling {
    automatic_restart   = false
    on_host_maintenance = "TERMINATE"
    preemptible         = true
    provisioning_model  = "SPOT"
  }


  metadata_startup_script = file("./startup.sh")



  network_interface {
    network = google_compute_subnetwork.serverku-sub-network.id
    access_config {
      # Include this section to give the VM an external IP address
    }
  }
}