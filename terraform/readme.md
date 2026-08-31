# Terraform

Provisions the read-only service account that runs the CIS IG1 audit.

Full detail is in [`audit-service-account/readme.md`](audit-service-account/readme.md) — what every permission grants, how impersonation works, and what gets logged. This page is just the run instructions.

---

## Create

```bash
cd terraform/audit-service-account

# Terraform uses Application Default Credentials — separate from `gcloud auth login`
gcloud auth application-default login

cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
organization_id = "123456789012"
host_project_id = "your-audit-project"
auditor_principals = ["user:you@yourdomain.com"]

enable_securitycenter = false   # true only where SCC is licensed
enable_billing_viewer = true    # true if the billing account is inside this org
```

Then:

```bash
terraform init
terraform plan      # review — nothing is created yet
terraform apply
```

Expect around 40 resources: one service account, three custom roles, ~35 organization bindings, one impersonation grant.

## Use

```bash
eval "$(terraform output -raw impersonate_command)"
gcloud config get-value auth/impersonate_service_account
```

That must print `cis-ig1-auditor@…`.

**`gcloud auth list` will still show your own address, and that is correct.** Impersonation does not change the authenticated account — you stay signed in as yourself, and gcloud exchanges that credential for a short-lived service account token on every call. The active account never changes, which is exactly why the audit log records both identities.

## Delete

```bash
# Stop impersonating FIRST — the service account cannot delete itself
gcloud config unset auth/impersonate_service_account

terraform destroy
```

State records exactly what was created, so destroy removes exactly that.

---

## Verify by hand in the Console

Terraform reporting success is not the same as the customer's organization being clean. Check both, in the browser, after `apply` and again after `destroy`.

### After apply — confirm what exists

**1. The service account exists and has no keys**

IAM & Admin → Service Accounts → select `cis-ig1-auditor` → **Keys** tab.

> Expected: **no keys listed.** The audit is impersonated, never keyed. A key here would mean something created one outside Terraform.

**2. The roles are held by the service account, not by you**

IAM & Admin → IAM → set the resource selector to the **organization** → find `cis-ig1-auditor`.

> Expected: around 35 roles, every name ending in `viewer`, `reader`, or `Viewer`, plus the three `cisIg1Audit…` custom roles.
>
> Then search for **your own** account in the same view. It should hold whatever it held before this module ran, and nothing new.

**3. The custom roles grant only reads**

IAM & Admin → Roles → filter on `cisIg1Audit`.

> Expected: three roles. Click each and read the permission list.
>
> - `cisIg1AuditStorageReader` — three `storage.buckets.*` permissions, **no object permissions**
> - `cisIg1AuditKeyReader` — `iam.serviceAccountKeys.list` only
> - `cisIg1AuditIapReader` — `iap.web.getSettings` only

**4. You can impersonate it, and only you**

IAM & Admin → Service Accounts → `cis-ig1-auditor` → **Permissions** tab.

> Expected: the principals from `auditor_principals`, holding Service Account Token Creator. Nobody else.

### After destroy — confirm nothing remains

**1. Service account gone**

IAM & Admin → Service Accounts.

> Expected: `cis-ig1-auditor` absent. Check **Deleted service accounts** too — it should not be sitting there pending purge.

**2. No organization bindings left**

IAM & Admin → IAM → organization scope → search `cis-ig1-auditor`.

> Expected: no results. Tick **Include Google-provided role grants** so nothing is hidden by the default filter.

**3. Custom roles gone**

IAM & Admin → Roles → filter `cisIg1Audit`.

> Expected: no results. Custom roles are soft-deleted for 7 days, so they may appear with a **Deleted** label — that is normal and they purge themselves.

**4. The audit trail remains**

Logging → Logs Explorer:

```
protoPayload.methodName="GenerateAccessToken"
protoPayload.request.name:"cis-ig1-auditor"
```

> Expected: entries for every impersonation session, each naming the human who authorised it. This should **still be there** after teardown — the record of what the auditor did outlives the auditor's ability to do it.

### Equivalent CLI checks

```bash
# no bindings remain
gcloud organizations get-iam-policy ORG_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members~cis-ig1-auditor" \
  --format="value(bindings.role)"

# no custom roles remain
gcloud iam roles list --organization=ORG_ID --filter="name~cisIg1Audit"

# service account gone
gcloud iam service-accounts describe \
  cis-ig1-auditor@HOST_PROJECT.iam.gserviceaccount.com 2>&1 | tail -1
```

Expected: empty, empty, and `NOT_FOUND`.

---

## If something is left behind

Terraform state is the record of what was created. If `destroy` failed partway:

```bash
terraform state list        # what Terraform still believes it owns
terraform destroy           # safe to re-run
```

Two known edges:

**Custom role IDs stay reserved for 30 days** after deletion. Re-applying inside that window fails on the create. Either `gcloud iam roles undelete`, or change `custom_role_prefix` in `terraform.tfvars`.

**A `billing.viewer` grant made directly on a billing account** is outside Terraform state and will not be removed by destroy. If you granted it by hand, revoke it by hand.
