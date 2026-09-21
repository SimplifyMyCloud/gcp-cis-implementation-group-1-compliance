output "service_account_email" {
  description = "The audit identity. Pass to gcloud as the impersonation target."
  value       = google_service_account.auditor.email
}

output "impersonate_command" {
  description = "Switch gcloud to the audit identity."
  value       = "gcloud config set auth/impersonate_service_account ${google_service_account.auditor.email}"
}

output "verify_command" {
  description = <<-EOT
    Confirm impersonation is active. Must print the service account.

    Note: `gcloud auth list` is NOT the right check — it shows the authenticated
    account, which stays as your own user. Impersonation layers a short-lived
    service account token over that credential rather than replacing it, which
    is precisely why audit logs can record both identities.
  EOT
  value       = "gcloud config get-value auth/impersonate_service_account"
}

output "audit_command" {
  description = <<-EOT
    For reference only — the command the audit tooling will run once
    impersonating. That tooling is separate from this module and is not
    required to create, verify, or destroy the service account.
  EOT
  value       = "go run audit-run.go -scope=org -org=${var.organization_id} -pack ./audit-state/org"
}

output "roles_granted" {
  description = "Every role this module binds at the organization node. Cross-check against `terraform destroy` output."
  value = sort(concat(
    local.predefined_roles,
    [for k, v in google_organization_iam_custom_role.audit : v.id],
  ))
}

output "role_count" {
  description = "How many organization-level bindings exist. Use it to verify teardown removed all of them."
  value       = length(local.predefined_roles) + length(local.custom_roles)
}

output "teardown_verification" {
  description = "Run after destroy. Empty output means no trace of the audit identity remains."
  value       = <<-EOT
    gcloud organizations get-iam-policy ${var.organization_id} \
      --flatten="bindings[].members" \
      --filter="bindings.members:${google_service_account.auditor.email}" \
      --format="value(bindings.role)"
  EOT
}
