variable "projectID" {
  type = string
  description = "gcp projects id"
  sensitive = true
}

variable "region" {
  type = string
}

variable "zone" {
  type = string
}

variable "boot-disk-name" {
  type = string
}

variable "boot-disk-loc" {
  type = string
}