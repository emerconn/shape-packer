terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.31.0"
    }
  }
}

provider "google" {
  project = "basic-bison-138323"
}

resource "google_sql_database_instance" "emerconn-pqsql" {
  name                = "emerconn-pqsql"
  region              = "us-east5"
  database_version    = "POSTGRES_18"
  deletion_protection = false

  settings {
    edition           = "ENTERPRISE"
    tier              = "db-f1-micro"
    disk_type         = "PD_HDD"
    disk_size         = 10
    disk_autoresize   = false
    availability_type = "ZONAL"
  }
}
