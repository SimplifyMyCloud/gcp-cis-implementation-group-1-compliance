terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.10"
    }
  }

  # ---------------------------------------------------------------------
  # STATE
  #
  # Local state is the default and is appropriate here: this module is
  # short-lived, applied and destroyed by one person during an engagement.
  #
  # State is NOT committed. It records the audit identity and every binding
  # created, and it is what `terraform destroy` reads to remove them. Losing
  # it means revoking 35 organization bindings by hand.
  #
  # Keep terraform.tfstate until teardown is complete and verified.
  #
  # If more than one person will apply or destroy this, uncomment the backend
  # below so state is shared and locked rather than sitting on one laptop.
  # Create the bucket first, with versioning enabled:
  #
  #   gsutil mb -p PROJECT_ID gs://BUCKET
  #   gsutil versioning set on gs://BUCKET
  #
  # backend "gcs" {
  #   bucket = "REPLACE_STATE_BUCKET"
  #   prefix = "cis-ig1-audit-service-account"
  # }
  # ---------------------------------------------------------------------
}

# The provider needs a quota project even though this module writes almost
# nothing into it — organization-level API calls bill against a project.
#
# Credentials come from Application Default Credentials. Run once, as yourself,
# before terraform init:
#
#   gcloud auth application-default login
#
# ADC is separate from `gcloud auth login`; having one does not give you the
# other, which is the usual first stumble here.
provider "google" {
  project = var.host_project_id
}
