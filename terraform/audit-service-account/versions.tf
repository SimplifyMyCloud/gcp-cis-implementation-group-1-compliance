terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.10"
    }
  }
}

# The provider needs a quota project even though this module writes almost
# nothing into it — organization-level API calls bill against a project.
#
# Credentials come from Application Default Credentials. Run this once before
# terraform init, as yourself:
#
#   gcloud auth application-default login
#
# ADC is separate from `gcloud auth login`; having one does not give you the
# other, which is the usual first stumble here.
provider "google" {
  project = var.host_project_id
}
