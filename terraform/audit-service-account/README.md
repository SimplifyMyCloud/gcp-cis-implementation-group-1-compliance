# CIS IG1 audit service account

Terraform for the identity that runs the audit. Read-only by construction, impersonated rather than keyed, and removed completely by `terraform destroy`.

Replaces the `gcloud` provisioning and revoke loop in phases 1 and 11 of the audit runbook (`docs/cis-ig1-audit-runbook.md` in the main repository).

## For the customer, in short

- The auditor is granted **one** permission, and it is not on your organization: the right to impersonate a single service account.
- That service account holds **35 roles, all read-only.** It cannot create, modify, delete or invoke anything, cannot read object contents, and cannot read secret values.
- **No credential file is ever created.** Impersonation issues a token that expires within the hour.
- **Every action is attributable to a named human** through the delegation chain in your audit logs, and every impersonation is recorded in Admin Activity logs, which cannot be disabled.
- **`terraform destroy` removes everything** — the account, all 35 bindings, the custom roles, the impersonation grant.

Detail for each of these is below: [what is granted](#exactly-what-is-granted) · [how impersonation works](#how-impersonation-works-and-what-the-auditor-can-do) · [what is logged](#everything-the-auditor-does-is-logged) · [teardown](#verifying-teardown)

## Why Terraform rather than the gcloud loops

**Teardown becomes verifiable.** The revoke loop iterates a list someone maintained by hand and hopes it is complete. Terraform state records exactly what was created, so destroy removes exactly that. On a large customer estate, "we think we revoked everything" is not good enough.

**The role list is reviewable before it is granted.** `terraform plan` shows the customer precisely what the audit identity will be able to read, as a diff, before anything is applied.

## Usage

```bash
# Terraform uses Application Default Credentials, which are separate from
# `gcloud auth login`. Run this once, as yourself, before init.
gcloud auth application-default login

# edit terraform.tfvars
terraform init
terraform plan                                 # review with the customer
terraform apply

eval "$(terraform output -raw impersonate_command)"
gcloud config get-value auth/impersonate_service_account   # must show the SA
```

Run the audit, then:

```bash
gcloud config unset auth/impersonate_service_account   # BEFORE destroy
terraform destroy
```

**Stop impersonating first.** The audit identity has no permission to delete itself or revoke its own bindings, so a destroy run while impersonating fails partway through and leaves grants in place.

## Exactly what is granted

Every role below is **READ**. The audit identity cannot create, modify, delete, or invoke anything in the organization.

There is no `write` or `read-write` row in this table, and that is the point — if one ever appears, the module has regressed.

| Access | Role | What it can read | IG1 controls served |
|---|---|---|---|
| **READ** | `roles/accesscontextmanager.policyReader` | VPC Service Controls perimeters and access levels | 3, 6 |
| **READ** | `roles/artifactregistry.reader` | Artifact Registry repository contents and scan findings | 2 |
| **READ** | `roles/bigquery.metadataViewer` | Dataset and table metadata — expiration and encryption settings | 3 |
| **READ** | `roles/billing.viewer` | Which projects are attached to a billing account | 1 |
| **READ** | `roles/binaryauthorization.policyViewer` | Binary Authorization admission policy | 2 |
| **READ** | `roles/browser` | Project, folder and organization names and lifecycle state | 1, 3, 4, 5, 11, 15 |
| **READ** | `roles/cloudasset.viewer` | Cloud Asset Inventory — resource configuration and IAM policies org-wide | 1, 2, 3, 4, 5, 6, 8, 10, 11, 15 |
| **READ** | `roles/cloudfunctions.viewer` | Cloud Functions runtime versions | 2 |
| **READ** | `roles/cloudkms.viewer` | KMS key metadata, rotation periods and key IAM | 11 |
| **READ** | `roles/cloudsecurityscanner.viewer` | Web Security Scanner scan configurations | 7 |
| **READ** | `roles/cloudsql.viewer` | Cloud SQL instance configuration, database users, backup settings | 2, 3, 4, 7, 11 |
| **READ** | `roles/compute.viewer` | Instances, disks, firewall rules, VPCs, subnets, routers, load balancers, VPN | 1, 2, 4, 6, 7, 8, 9, 10, 12 |
| **READ** | `roles/container.viewer` | GKE cluster and node pool configuration | 2, 4, 7, 8, 11 |
| **READ** | `roles/datastore.viewer` | Firestore backups | 11 |
| **READ** | `roles/dns.reader` | Cloud DNS policies and response policies | 8, 9 |
| **READ** | `roles/essentialcontacts.viewer` | Essential Contacts entries | 17 |
| **READ** | `roles/gkebackup.viewer` | Backup for GKE backup plans | 11 |
| **READ** | `roles/iam.securityReviewer` | IAM policies on organizations, folders, projects and resources | 4, 5, 6, 8, 11 |
| **READ** | `roles/iam.serviceAccountViewer` | Service account names, descriptions and disabled state | 5 |
| **READ** | `roles/logging.privateLogViewer` | Data Access log entries (metadata about who accessed what) | 5, 8 |
| **READ** | `roles/logging.viewer` | Log sinks, log buckets, log-based metrics and their configuration | 3, 5, 8, 9, 11, 17 |
| **READ** | `roles/monitoring.viewer` | Alert policies and notification channels | 8, 11, 17 |
| **READ** | `roles/orgpolicy.policyViewer` | Organization Policy constraints and their enforcement state | 3, 4, 5 |
| **READ** | `roles/osconfig.inventoryViewer` | OS and installed package inventory on VMs; patch job history | 2, 7, 10 |
| **READ** | `roles/policyanalyzer.activityAnalysisViewer` | Last-authentication times for accounts and keys | 5 |
| **READ** | `roles/privilegedaccessmanager.viewer` | Privileged Access Manager entitlements | 5 |
| **READ** | `roles/recommender.iamViewer` | IAM Recommender over-grant findings | 5 |
| **READ** | `roles/run.viewer` | Cloud Run service configuration | 2 |
| **READ** | `roles/secretmanager.viewer` | Secret names and rotation schedules | 5 |
| **READ** | `roles/securitycenter.adminViewer` | Security Command Center settings and findings | 7 |
| **READ** | `roles/serviceusage.serviceUsageViewer` | Which APIs are enabled on a project | 3, 4, 7 |
| **READ** | `roles/spanner.viewer` | Spanner instances and backups | 11 |
| **READ** | `IapReader` *(custom)* | Identity-Aware Proxy session settings | 4 |
| **READ** | `KeyReader` *(custom)* | That a service account key exists, and when it was created | 5 |
| **READ** | `StorageReader` *(custom)* | Bucket configuration and bucket IAM | 3, 8, 11 |

**Controls by number:** 1 Asset inventory · 2 Software inventory · 3 Data protection · 4 Secure configuration · 5 Account management · 6 Access control · 7 Vulnerability management · 8 Audit logging · 9 DNS / web protections · 10 Malware defenses · 11 Data recovery · 12 Network management · 15 Service providers · 17 Incident response

### Limits worth stating explicitly

| Role | Limit |
|---|---|
| `IapReader` *(custom)* | Replaces `roles/iap.settingsAdmin`, which can **modify** settings |
| `KeyReader` *(custom)* | Replaces `roles/iam.serviceAccountKeyAdmin`, which can **create and delete** keys. **Cannot read key material** |
| `StorageReader` *(custom)* | Replaces `roles/storage.admin`, which can **delete** buckets and objects. **No object permission at all** — cannot read, list or write file contents |
| `roles/artifactregistry.reader` | Permits downloading artifacts; no push or delete |
| `roles/cloudkms.viewer` | **Cannot encrypt or decrypt** — metadata and IAM only |
| `roles/logging.privateLogViewer` | Required in addition to `logging.viewer` to read Data Access entries |
| `roles/secretmanager.viewer` | **Cannot read secret values** — excludes `secretmanager.versions.access` |
| `roles/securitycenter.adminViewer` | Read-only despite the name — `get` and `list` on SCC only |

### What the audit identity cannot do

- Read the contents of any object in Cloud Storage
- Read the value of any secret in Secret Manager
- Encrypt or decrypt with any KMS key
- Create, modify or delete any resource, IAM binding, or organization policy
- Create a service account key — including its own
- Run, deploy, or invoke any workload

### One grant that is not held by the audit identity

`roles/iam.serviceAccountTokenCreator` appears in the module, granted **to your named auditors, on the service account**. It is what allows a human to impersonate the identity. The audit identity does not hold it and cannot impersonate anything.

## How impersonation works, and what the auditor can do

### The auditor is granted nothing on your organization

This module grants the human auditor exactly **one** permission, and it is not on your organization or any project:

```
roles/iam.serviceAccountTokenCreator   on the audit service account only
```

That is the right to *ask for a token* for one specific service account. It carries no ability to read, write, or list anything in the organization. Remove the service account and that permission refers to nothing.

Every organization-level role in this module is held by the **service account**, never by a person.

```
   Auditor (a human)
      │
      │  holds ONLY: tokenCreator, on one service account
      │  holds NOTHING on the organization
      ▼
   cis-ig1-auditor  (service account)
      │
      │  holds: 35 read-only roles at the organization node
      ▼
   Your organization — read access only
```

### What happens at the moment of a command

1. `gcloud` asks the IAM Credentials API for a token for the audit service account
2. IAM checks the human holds `tokenCreator` on that account, and issues a **short-lived token** — one hour by default
3. The command runs with the service account's permissions, not the human's
4. The token expires. It is not stored anywhere and there is no key to leak

The human's own permissions are not additive to this. A command run while impersonating carries the service account's access and nothing else — if the service account cannot read something, the command fails, regardless of what the human might otherwise be entitled to.

### The honest caveat

**This module grants the auditor no access to your organization. It cannot revoke access they already have.**

If the auditor's account holds permissions on your organization from some other source — an existing partner grant, a group membership, a prior engagement — those are untouched by this module and remain in force outside the impersonation session.

That pre-existing access should be disclosed and reviewed separately. What this module guarantees is that *the audit itself* runs with the read-only permission set documented above.

### No key exists

There is deliberately no `google_service_account_key` resource anywhere in this module.

A key is a long-lived credential in a file. It can be copied, committed, or emailed, and revoking it requires knowing it exists. Impersonation issues a token that expires within the hour and is never written to disk.

This is also why the audit passes its own test: CIS safeguard **5.2** requires eliminating static service account credentials, and an audit that created a key to check for keys would fail the control it was measuring.

## Everything the auditor does is logged

### The delegation chain names the human

This is the part worth showing your security team. When a command runs under impersonation, Cloud Audit Logs record **both** identities — the service account that made the call, and the human who authorised it:

```
protoPayload.authenticationInfo.principalEmail
    cis-ig1-auditor@<host-project>.iam.gserviceaccount.com

protoPayload.authenticationInfo.serviceAccountDelegationInfo[]
    firstPartyPrincipal.principalEmail: alex@example.com
```

Impersonation is not anonymising. Every action is attributable to a named person, and the audit trail shows the chain rather than an opaque service account.

### Watch the audit in real time

```bash
gcloud logging read \
  'protoPayload.authenticationInfo.principalEmail="cis-ig1-auditor@HOST_PROJECT.iam.gserviceaccount.com"' \
  --organization=ORG_ID --freshness=1d \
  --format="table(timestamp, protoPayload.methodName, protoPayload.resourceName)"
```

Every token issued to the auditor:

```bash
gcloud logging read \
  'protoPayload.methodName="GenerateAccessToken"
   AND protoPayload.request.name:"cis-ig1-auditor"' \
  --organization=ORG_ID --freshness=7d \
  --format="table(timestamp, protoPayload.authenticationInfo.principalEmail)"
```

That second query answers "who impersonated the auditor, and when" — the question that matters if anything is ever disputed.

### What is guaranteed to be logged, and what is not

Being precise here matters more than being reassuring.

| Log type | Default | Covers |
|---|---|---|
| **Admin Activity** | **Always on, cannot be disabled** | Configuration writes, and the `GenerateAccessToken` call each time the auditor impersonates |
| **Data Access** | **Off by default** for most services | The read calls this audit is made of |

The audit is entirely reads, so **whether every individual command appears in your logs depends on whether Data Access logging is enabled**.

Two things follow:

**The impersonation itself is always logged.** `GenerateAccessToken` is an Admin Activity event, so there is a permanent, non-disableable record of every session the auditor opened, and of who opened it — even where Data Access logging is off.

**If the reads are not logged, that is itself a finding.** CIS safeguard **8.2** requires Data Access audit logs be enabled. An organization where the audit's own activity is invisible has already failed that control, and the audit will report it as such. Enabling it before the audit runs both satisfies the safeguard and gives you a complete record of everything the auditor read.

### After the engagement

`terraform destroy` removes the identity and its permissions, but the log record remains for your configured retention period. The evidence of what the auditor did outlives the auditor's ability to do it.

---

## Scope of this directory

This module creates the audit **identity** and nothing else. It does not run any compliance check, read any finding, or produce any report.

The audit tooling itself lives in the main repository. Nothing here depends on it, and this directory can be reviewed and applied entirely on its own.

## Why three custom roles exist

Three permissions had no safe predefined role, so each gets a custom role with an explicit permission list:

| Custom role | Replaces | Why |
|---|---|---|
| `StorageReader` | `roles/storage.admin` | The predefined role grants create, update and **delete** on every bucket *and object* in the organization. The audit only reads bucket configuration and bucket IAM, so this role has no object permission at all. |
| `KeyReader` | `roles/iam.serviceAccountKeyAdmin` | The only predefined role carrying `iam.serviceAccountKeys.list` can also **create and delete keys**. Granting it to an audit identity would breach safeguard 5.2 — the safeguard this permission exists to test. |
| `IapReader` | `roles/iap.settingsAdmin` | Carries `updateSettings` alongside `getSettings`. No read-only equivalent exists. |

## State

State is **not committed**. It records the audit identity and every binding created, and `terraform destroy` reads it to remove them — lose it and you are revoking 35 organization bindings by hand.

Local state is fine here: this module is short-lived, applied and destroyed by one person during an engagement. Keep `terraform.tfstate` until teardown is complete and verified, then delete it with the rest of the engagement artefacts.

If more than one person will apply or destroy it, uncomment the GCS backend in `versions.tf` so state is shared and locked. Create the bucket with versioning on first.

`.gitignore` covers `*.tfstate`, `*.tfstate.*`, `*.tfplan`, and `.terraform/` — that last one holds provider binaries, ~116 MB for the google provider, over GitHub's file size limit.

`terraform.tfvars` **is** committed. It holds an organization ID, a project ID and auditor emails — none of them credentials — so committing it means anyone can reproduce exactly what was applied.

## Why bindings are additive

Every binding uses `google_organization_iam_member`, never `google_organization_iam_binding` or `google_organization_iam_policy`.

**This matters more than anything else in the module.** `_binding` is authoritative for a role: applying it would remove every other member currently holding that role across the customer's organization. `_policy` is authoritative for the *entire* organization policy. Either would be a serious incident on a large estate.

`_member` adds one principal to one role, leaves everything else untouched, and destroy removes only what was added.

Do not consolidate these into a binding.

## Why organization scope

Several checks iterate every project. A role held on only some projects makes those loops skip the rest **silently** — producing a clean-looking result from a partial scan, which is worse than an error.

## Known operational edges

**Custom roles are soft-deleted for 7 days, and their IDs stay reserved for 30.** Destroy and re-apply inside that window and the create fails. Either `gcloud iam roles undelete`, or change `custom_role_prefix`.

**`roles/securitycenter.adminViewer` fails where SCC is not licensed.** Set `enable_securitycenter = false`.

**Billing accounts often sit outside the audited organization.** `enable_billing_viewer` grants `roles/billing.viewer` at the org node, which only helps if the billing account lives there. Otherwise grant it directly on the billing account:

```bash
gcloud billing accounts add-iam-policy-binding BILLING_ACCOUNT_ID \
  --member="serviceAccount:$(terraform output -raw service_account_email)" \
  --role="roles/billing.viewer"
```

That binding is outside Terraform state, so remember it at teardown.

**APIs still need enabling separately.** This module grants permissions; it does not turn on services. Runbook Phase 1 covers that, including the before/after snapshot so teardown disables only what the audit enabled.

## Verifying teardown

```bash
terraform destroy
eval "$(terraform output -raw teardown_verification)"
```

Empty output means no trace of the audit identity remains at the organization node.

Then confirm the custom roles are gone:

```bash
gcloud iam roles list --organization=ORG_ID --filter="name~cisIg1Audit"
```

## What this module does not do

- Enable APIs — see runbook Phase 1
- Create the audit *results* — see `audit-run.go`
- Grant anything at project scope — everything is organization-level by design
