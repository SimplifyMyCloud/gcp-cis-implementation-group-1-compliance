# ---------------------------------------------------------------------------
# CIS IG1 audit service account — read-only, impersonated, destroy-clean
#
# Two properties this module is built around:
#
#   1. READ-ONLY. Every permission granted is a get, list, or search. No
#      predefined role that carries a write verb is used; where the only
#      predefined option over-grants, a custom role with an explicit
#      permission list replaces it. See the three google_organization_iam_
#      custom_role resources below.
#
#   2. DESTROY-CLEAN. Every binding is created with for_each over a known set,
#      so `terraform destroy` removes exactly what `terraform apply` created —
#      no orphaned grants, nothing to reconcile by hand afterwards.
#
# ---------------------------------------------------------------------------
# WHY google_organization_iam_member AND NOT _binding OR _policy
#
# google_organization_iam_binding is AUTHORITATIVE for a role: applying it
# would remove every other member currently holding that role across the
# customer's organization. google_organization_iam_policy is worse — it is
# authoritative for the entire policy.
#
# _member is additive. It adds one principal to one role and leaves every
# existing binding untouched, and destroy removes only that one principal.
#
# Do not "simplify" these into a binding.
# ---------------------------------------------------------------------------

locals {
  sa_member = "serviceAccount:${google_service_account.auditor.email}"

  # Predefined roles, each verified to contain no write, update, delete, or
  # actuation permission. Grouped by what they let the audit read.
  predefined_roles = concat(
    [
      # Resource hierarchy and posture
      "roles/browser",                          # projects/folders/org: get, list
      "roles/orgpolicy.policyViewer",           # org policy constraints
      "roles/cloudasset.viewer",                # Cloud Asset Inventory search
      "roles/serviceusage.serviceUsageViewer",  # which APIs are enabled

      # Identity
      "roles/iam.securityReviewer",             # getIamPolicy across resources
      "roles/iam.serviceAccountViewer",         # service account metadata
      "roles/recommender.iamViewer",            # IAM Recommender findings
      "roles/privilegedaccessmanager.viewer",   # PAM entitlements
      "roles/policyanalyzer.activityAnalysisViewer", # last-authentication data

      # Logging and monitoring
      "roles/logging.viewer",                   # sinks, buckets, metrics
      "roles/logging.privateLogViewer",         # Data Access log ENTRIES — a
                                                # separate grant from the above
      "roles/monitoring.viewer",                # alert policies, channels

      # Compute and network
      "roles/compute.viewer",                   # instances, firewalls, VPCs, LBs
      "roles/osconfig.inventoryViewer",         # VM Manager OS inventory
      "roles/container.viewer",                 # GKE clusters and node pools
      "roles/gkebackup.viewer",                 # Backup for GKE plans
      "roles/dns.reader",                       # DNS policies, response policies

      # Data services
      "roles/cloudsql.viewer",                  # Cloud SQL instances and users
      "roles/datastore.viewer",                 # Firestore backups
      "roles/spanner.viewer",                   # Spanner backups
      "roles/bigquery.metadataViewer",          # dataset metadata, not contents

      # Application and supply chain
      "roles/run.viewer",                       # Cloud Run services
      "roles/cloudfunctions.viewer",            # Cloud Functions runtimes
      "roles/artifactregistry.reader",          # registry contents
      "roles/binaryauthorization.policyViewer", # admission policy
      "roles/cloudsecurityscanner.viewer",      # Web Security Scanner configs

      # Secrets and keys — metadata only, never payload
      "roles/secretmanager.viewer",             # secret metadata and rotation.
                                                # Does NOT include
                                                # versions.access, so the audit
                                                # cannot read secret values.
      "roles/cloudkms.viewer",                  # key metadata and IAM. Does NOT
                                                # include encrypt/decrypt.

      # Governance
      "roles/essentialcontacts.viewer",
      "roles/accesscontextmanager.policyReader",
    ],
    var.enable_securitycenter ? ["roles/securitycenter.adminViewer"] : [],
    var.enable_billing_viewer ? ["roles/billing.viewer"] : [],
  )

  # Custom roles replacing predefined roles that carry write permissions, or
  # filling gaps where no read-only predefined role exists.
  custom_roles = {
    StorageReader = {
      title = "CIS IG1 Audit — Storage Reader"
      # REPLACES roles/storage.admin, which grants create, update and DELETE on
      # every bucket and object in the organization. The audit only ever reads
      # bucket configuration and bucket IAM; it never touches objects, so no
      # object permission appears here at all.
      permissions = [
        "storage.buckets.get",
        "storage.buckets.getIamPolicy",
        "storage.buckets.list",
      ]
    }

    KeyReader = {
      title = "CIS IG1 Audit — Service Account Key Reader"
      # The only predefined role carrying iam.serviceAccountKeys.list is
      # roles/iam.serviceAccountKeyAdmin, which can also CREATE and DELETE
      # keys. Granting that to an audit identity would itself breach safeguard
      # 5.2 — the safeguard this permission exists to test.
      permissions = [
        "iam.serviceAccountKeys.list",
      ]
    }

    IapReader = {
      title = "CIS IG1 Audit — IAP Settings Reader"
      # roles/iap.settingsAdmin carries updateSettings alongside getSettings.
      # There is no read-only predefined equivalent.
      permissions = [
        "iap.web.getSettings",
      ]
    }
  }
}

# ---------------------------------------------------------------------------
# The audit identity
# ---------------------------------------------------------------------------

resource "google_service_account" "auditor" {
  project      = var.host_project_id
  account_id   = var.account_id
  display_name = "CIS IG1 auditor (read-only)"
  description  = "Read-only CIS IG1 compliance audit of organization ${var.organization_id}. Impersonated, never keyed. Destroy when the engagement ends."
}

# Impersonation is the authentication path. No google_service_account_key
# resource appears anywhere in this module, deliberately — creating one would
# breach safeguard 5.2 and leave key material in Terraform state.
resource "google_service_account_iam_member" "impersonation" {
  for_each = toset(var.auditor_principals)

  service_account_id = google_service_account.auditor.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = each.value
}

# ---------------------------------------------------------------------------
# Custom read-only roles
# ---------------------------------------------------------------------------

resource "google_organization_iam_custom_role" "audit" {
  for_each = local.custom_roles

  org_id      = var.organization_id
  role_id     = "${var.custom_role_prefix}${each.key}"
  title       = each.value.title
  description = "Read-only. Created by the CIS IG1 audit module; removed on terraform destroy."
  permissions = each.value.permissions
  stage       = "GA"
}

# ---------------------------------------------------------------------------
# Organization-level bindings
#
# Granted at the org node rather than per project on purpose: several checks
# iterate every project, and a role held on only some projects makes those
# loops skip the rest silently — producing a clean-looking result from a
# partial scan.
# ---------------------------------------------------------------------------

resource "google_organization_iam_member" "predefined" {
  for_each = toset(local.predefined_roles)

  org_id = var.organization_id
  role   = each.value
  member = local.sa_member
}

resource "google_organization_iam_member" "custom" {
  for_each = local.custom_roles

  org_id = var.organization_id
  role   = google_organization_iam_custom_role.audit[each.key].id
  member = local.sa_member
}
