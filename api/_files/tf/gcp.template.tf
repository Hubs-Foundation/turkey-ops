
# variable "project_id"{
#   description = "gcp project id"
#   default = "missing"  
# }

# variable "stack_name" {
#   description = "deployment id"
#   default = "missing"
# }

# variable "region" {
#   description = "region"
#   default = "us-east1"
# }

provider "google" {
  project = "{{.ProjectId}}"
  region  = "{{.Region}}"
}

terraform {  
    backend "gcs" {    
        bucket  = "turkeycfg"
        prefix  = "tf-backend/{{.Stackname}}"
    }
}

# VPC
resource "google_compute_network" "vpc" {
  name                    = "{{.Stackname}}"
  routing_mode            = "GLOBAL"
  auto_create_subnetworks = "true"
}

# Subnet
resource "google_compute_subnetwork" "subnet1" {
  name          = "{{.Stackname}}-subnet1"
  region        = "{{.Region}}"
  network       = google_compute_network.vpc.name
  ip_cidr_range = "10.10.0.0/16"
}

# GKE cluster
resource "google_container_cluster" "gke" {
  name     = "{{.Stackname}}"
  location = "{{.Region}}"
  
  # We can't create a cluster with no node pool defined, but we want to only use
  # separately managed node pools. So we create the smallest possible default
  # node pool and immediately delete it.
  remove_default_node_pool = true
  initial_node_count       = 1

  network    = google_compute_network.vpc.name
  subnetwork = google_compute_subnetwork.subnet1.name
}

# Separately Managed Node Pool
resource "google_container_node_pool" "gke_nodes" {
  name       = "${google_container_cluster.gke.name}-node-pool"
  location   = "{{.Region}}"
  cluster    = google_container_cluster.gke.name
  node_count = 1

  node_config {
    oauth_scopes = [
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]

    labels = {
      app = "turkey"
      env = "{{.Stackname}}"
    }

    # preemptible  = true
    machine_type = "n1-standard-1"
    tags         = ["gke-node", "{{.Stackname}}"]
    metadata = {
      disable-legacy-endpoints = "true"
    }
  }
}

#k8s rbac for gke
data "google_client_config" "gke" {}

provider "kubernetes" {
  host     = "${google_container_cluster.gke.endpoint}"

  token = "${data.google_client_config.gke.access_token}"
  cluster_ca_certificate = "${base64decode(google_container_cluster.gke.master_auth.0.cluster_ca_certificate)}"

  load_config_file = false
}

# pgsql
# resource "google_compute_global_address" "private_ip_block" {
#   name         = "private-ip-block"
#   purpose       = "VPC_PEERING"
#   address_type  = "INTERNAL"
#   ip_version   = "IPV4"
#   prefix_length = 16
#   network       = google_compute_network.vpc.self_link
# }
# resource "google_service_networking_connection" "private_vpc_connection" {
#   network                 = google_compute_network.vpc.self_link
#   service                 = "servicenetworking.googleapis.com"
#   reserved_peering_ranges = [google_compute_global_address.private_ip_block.name]
# }
resource "google_sql_database_instance" "pgsql" {
  name             = "{{.Stackname}}"
  database_version = "POSTGRES_13"
  region           = "{{.Region}}"

  # depends_on = [google_service_networking_connection.private_vpc_connection]

  settings {
    tier = "db-f1-micro"
    # ip_configuration {
    #   ipv4_enabled    = false
    #   private_network = google_compute_network.vpc.id
    # }    
  }
}
resource "google_sql_user" "db_user" {
  name     = "{{.DbUser}}"
  instance = google_sql_database_instance.pgsql.name
  password = "{{.DbPass}}"
}