# Edit these four values, then: terraform init && terraform plan && terraform apply

organization_id = "933250405420"            # gcloud organizations list
host_project_id = "simplifymycloud-dev"     # project that owns the SA and carries API quota

auditor_principals = [                      # who may impersonate the audit identity
  "user:chris@simplifymy.cloud",
]

enable_securitycenter = false               # true only where SCC is licensed
enable_billing_viewer = false               # true if the billing account is inside this org

# Custom role IDs stay reserved for ~37 days after a destroy, but can only be
# undeleted for the first 7. Re-applying in between fails with "You can't create
# a role_id (...) which has been marked for deletion" — bump this prefix instead.
custom_role_prefix = "cisIg1Audit2"         # default "cisIg1Audit"
