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