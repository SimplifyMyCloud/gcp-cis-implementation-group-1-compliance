variable "organization_id" {
  description = "Numeric GCP Organization ID to audit (no 'organizations/' prefix)."
  type        = string

  validation {
    condition     = can(regex("^[0-9]+$", var.organization_id))
    error_message = "organization_id must be the numeric ID only, e.g. 123456789012."
  }
}

variable "host_project_id" {
  description = <<-EOT
    Project that will own the audit service account and carry API quota.

    This should be a project you control, not a customer workload project. It is
    the only resource this module writes into; everything else is an additive
    IAM binding at the organization node.
  EOT
  type        = string
}

variable "auditor_principals" {
  description = <<-EOT
    Identities permitted to impersonate the audit service account, fully
    qualified — e.g. ["user:alex@example.com", "group:sre@example.com"].

    Impersonation is the only supported authentication path. No service account
    key is created, because a key would violate CIS safeguard 5.2 — one of the
    safeguards this audit exists to test.
  EOT
  type        = list(string)

  validation {
    condition = alltrue([
      for p in var.auditor_principals :
      can(regex("^(user|group|serviceAccount):", p))
    ])
    error_message = "Each principal must be prefixed with user:, group:, or serviceAccount:."
  }
}

variable "account_id" {
  description = "Service account ID. Change it if a previous audit's account is still soft-deleted."
  type        = string
  default     = "cis-ig1-auditor"
}

variable "custom_role_prefix" {
  description = <<-EOT
    Prefix for the custom role IDs created at the organization node.

    Custom roles are soft-deleted for 7 days and their IDs stay reserved for 30.
    If you destroy and re-apply inside that window, change this prefix rather
    than fighting the reservation.
  EOT
  type        = string
  default     = "cisIg1Audit"
}

variable "enable_securitycenter" {
  description = "Grant the Security Command Center viewer role. Set false where SCC is not licensed; the binding fails otherwise."
  type        = bool
  default     = true
}

variable "enable_billing_viewer" {
  description = <<-EOT
    Grant billing.viewer at the organization node. Optional: no check needs it.
    V7 (projects with no billing account) reads each project's own billing info,
    which resourcemanager.projects.get already allows.
  EOT
  type        = bool
  default     = false
}
