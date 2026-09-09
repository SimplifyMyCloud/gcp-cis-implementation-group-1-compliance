# Edit these four values, then: terraform init && terraform plan && terraform apply

organization_id = "REPLACE_ORG_ID"          # gcloud organizations list
host_project_id = "REPLACE_PROJECT_ID"      # project that owns the SA and carries API quota

auditor_principals = [                      # who may impersonate the audit identity
  "user:REPLACE_EMAIL",
]

enable_securitycenter = false               # true only where SCC is licensed
enable_billing_viewer = false               # true if the billing account is inside this org
