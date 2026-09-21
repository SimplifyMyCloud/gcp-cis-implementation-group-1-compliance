# Edit these values, then: terraform init && terraform plan && terraform apply

organization_id = "REPLACE_ORG_ID"          # gcloud organizations list
host_project_id = "REPLACE_PROJECT_ID"      # project that owns the SA and carries API quota

auditor_principals = [                      # who may impersonate the audit identity
  "user:REPLACE_EMAIL",
]

enable_securitycenter = false               # true only where SCC is activated
enable_billing_viewer = false               # optional — no check needs it

# Custom role IDs stay reserved for ~37 days after a destroy, but can only be
# undeleted for the first 7. Re-applying in between fails with "You can't create
# a role_id (...) which has been marked for deletion" — set a new prefix, e.g.:
# custom_role_prefix = "cisIg1Audit2"       # default "cisIg1Audit"
