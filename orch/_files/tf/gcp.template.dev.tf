terraform {  
    backend "gcs" {    
        bucket  = "turkeycfg"
        prefix  = "tf-backend/{{.Stackname}}"
    }
}

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

# provider "google" {
#   project = "{{.ProjectId}}"
#   region  = "{{.Region}}"
# }

provider "google-beta" {
  project = "{{.ProjectId}}"
  region  = "{{.Region}}"
}

################## network
resource "google_compute_network" "vpc" {
  provider = google-beta
  auto_create_subnetworks         = false
  delete_default_routes_on_create = false  
  name                    = "{{.Stackname}}"
  routing_mode            = "GLOBAL"
}
resource "google_compute_subnetwork" "private" {
  provider = google-beta
  name          = "{{.Stackname}}-pvt"
  region        = "{{.Region}}"
  network       = google_compute_network.vpc.name
  ip_cidr_range = "10.0.0.0/16"
  private_ip_google_access = "true"
}
resource "google_compute_subnetwork" "public" {
  provider = google-beta
  name          = "{{.Stackname}}-pub"
  region        = "{{.Region}}"
  network       = google_compute_network.vpc.name
  ip_cidr_range = "10.1.0.0/16"
}
resource "google_compute_firewall" "stream" {
  provider = google-beta
  name    = "{{.Stackname}}-stream"
  network = google_compute_network.vpc.name
  allow {
    protocol = "tcp"
    ports    = ["4443", "5349"]
  }
  allow {
    protocol = "udp"
    ports    = ["35000-65000"]
  }  
  source_ranges = ["0.0.0.0/0"]
}

################## GKE cluster
resource "google_container_cluster" "gke" {
  provider = google-beta
  name     = "{{.Stackname}}"
  location = "{{.Region}}"
  remove_default_node_pool = true
  initial_node_count       = 1
  network    = google_compute_network.vpc.name
  subnetwork = google_compute_subnetwork.public.name
  ip_allocation_policy {}  # empty == let gcp pick to avoid "cidr range not available" errors
  cluster_autoscaling {
    enabled = true
    resource_limits{
      resource_type = "memory"
      minimum = 16
      maximum = 64
    }
    resource_limits{
      resource_type = "cpu"
      minimum = 4
      maximum = 16
    }
    autoscaling_profile = "OPTIMIZE_UTILIZATION"
  }
}

resource "google_container_node_pool" "gke_nodes" {
  provider = google-beta
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
    preemptible  = true
    machine_type = "n1-highmem-2"
    # local_ssd_count = 1
    tags         = ["gke-node", "{{.Stackname}}"]
    metadata = {
      disable-legacy-endpoints = "true"
    }
  }
}

################## pgsql
resource "google_compute_global_address" "private_ip_address" {
  provider = google-beta
  name          = "{{.Stackname}}"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.vpc.id
}
resource "google_service_networking_connection" "private_vpc_connection" {
  provider = google-beta
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]
}
resource "google_sql_database_instance" "pgsql" {
  provider = google-beta
  depends_on = [google_service_networking_connection.private_vpc_connection]
  name             = "{{.Stackname}}"
  database_version = "POSTGRES_13"
  region           = "{{.Region}}"
  deletion_protection = false
  settings {
    tier = "db-custom-1-6656"
    ip_configuration {
      ipv4_enabled    = true
      private_network = google_compute_network.vpc.id
    }    
  }
}
resource "google_sql_user" "db_user" {
  provider = google-beta
  name     = "{{.DbUser}}"
  instance = google_sql_database_instance.pgsql.name
  password = "{{.DbPass}}"
}
resource "google_sql_database" "dashboard_db" {
  provider = google-beta
  name     = "dashboard"
  instance = google_sql_database_instance.pgsql.name
}
