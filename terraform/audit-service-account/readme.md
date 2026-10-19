# audit-service-account

Creates the identity the audit runs as: one service account, three custom roles at the organization, and the bindings that let auditors impersonate it. No keys.

## Apply

```bash
cp terraform.tfvars terraform.tfvars.local   # or edit in place
```

| Variable | Value |
|---|---|
| `organization_id` | Numeric org ID |
| `host_project_id` | Project that owns the SA and carries API quota |
| `auditor_principals` | `["user:name@customer.gov"]` — who may impersonate |
| `enable_securitycenter` | `true` only where SCC is activated |
| `enable_billing_viewer` | Optional. No check requires it |
| `custom_role_prefix` | Default `cisIg1Audit`. See below |

```bash
terraform init && terraform plan
```

```bash
terraform apply
```

```bash
terraform output
```

## Verify

```bash
../../gcloud/verify.sh
```

Confirms: no keys, no write verbs, expected bindings present.

## Destroy

```bash
terraform destroy
```

Then confirm nothing remains:

```bash
../../gcloud/verify.sh
```

## Custom role IDs are reserved after deletion

A destroyed custom role ID stays reserved for roughly 37 days and can only be undeleted in the first 7. Re-applying in between fails:

```
You can't create a role_id (...) which has been marked for deletion
```

Set a new prefix:

```hcl
custom_role_prefix = "cisIg1Audit2"
```

## Files

| | |
|---|---|
| `main.tf` | Service account, custom roles, org bindings |
| `variables.tf` | Inputs |
| `outputs.tf` | SA email, role IDs — feed `SA_EMAIL` in `config/audit.env` |
| `versions.tf` | Provider pins |
| `terraform.tfvars` | Committed with `REPLACE_*` placeholders. No credentials |

State and `.terraform/` are gitignored. The provider binary alone exceeds GitHub's file size limit.
