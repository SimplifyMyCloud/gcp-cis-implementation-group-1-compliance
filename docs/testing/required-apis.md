# Required APIs and IAM roles — CIS IG1 audit

Found by running the audit end to end against a test organization. Every entry has **evidence
from a real run**: either an error the audit hit, or Google's own request metrics showing the audit
identity calling that API, billed to that project. "Checks" columns are generated from the commands
in `docs/cis-ig1-cli-validation.md`.

| | |
|---|---|
| Test organization | `933250405420` (simplifymy.cloud) |
| Audit host project | `simplifymycloud-dev` (number `288261943767`) — owns the audit SA |
| Project-scope target | `iq9-gcp-dev-yamato` (number `168357744801`) |
| Audit identity | `cis-ig1-auditor@simplifymycloud-dev.iam.gserviceaccount.com` (impersonated) |
| Final clean run | run 8, 2026-09-14 — org pass and project pass both `RUN STATUS: OK`, 0 DENIED, 0 ERROR |

## Where an API must be enabled

Every Google API call is billed to a **consumer project**, and the API must be enabled in *that*
project or the call fails with `SERVICE_DISABLED`. Running as an impersonated service account,
the audit's calls split two ways — **per API, not per check**:

- **Billed to the audit host project** (the project that owns the audit SA). These must be
  enabled in the host project or checks ERROR. `audit-run.go` now says so explicitly:
  `API DISABLED IN AUDIT HOST PROJECT: <api> on <project>`.
- **Billed to the project being audited.** Enabling these in the host project does nothing. If
  one is off in an audited project, that product can't be in use there, and the check is correctly
  N/A. **Don't enable these in customer projects**; the audit is read-only.

Verified directly: `gcloud container clusters list --project=iq9-gcp-dev-yamato
--billing-project=simplifymycloud-dev` still failed with *Kubernetes Engine API has not been used in
project iq9-gcp-dev-yamato*. Cloud SQL Admin is the counter-intuitive one: its calls bill to the
host project even though the instances are in the audited project.

## 1. Enable in the audit host project — 17 APIs

```bash
gcloud services enable --project="$AUDIT_PROJECT" \
  accesscontextmanager.googleapis.com \
  bigquery.googleapis.com \
  cloudasset.googleapis.com \
  cloudbilling.googleapis.com \
  cloudresourcemanager.googleapis.com \
  essentialcontacts.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  logging.googleapis.com \
  monitoring.googleapis.com \
  orgpolicy.googleapis.com \
  policyanalyzer.googleapis.com \
  recommender.googleapis.com \
  securitycenter.googleapis.com \
  serviceusage.googleapis.com \
  spanner.googleapis.com \
  sqladmin.googleapis.com
```

| # | API | Checks | Methods called in run 8 | Evidence | Date |
|---|---|---|---|---|---|
| 1 | `accesscontextmanager.googleapis.com` | V36, V108 | ListAccessPolicies, ListAccessLevels, ListServicePerimeters | metrics — billed to host | 2026-09-14 |
| 2 | `bigquery.googleapis.com` | V38 (`bq`) | ListDatasets, GetDataset | metrics — billed to host | 2026-09-14 |
| 3 | `cloudasset.googleapis.com` | V1 V2 V3 V4 V5 V8 V15 V22 V25 V26 V27 V29 V31 V33 V34 V46 V50 V53 V55 V56 V59 V61 V66 V68 V69 V76 V81 V86 V87 V102 V105 V132 V150 V154 V160 V175 V177 | SearchAllResources, SearchAllIamPolicies, ListFeeds (80 calls) | metrics — billed to host | 2026-09-14 |
| 4 | `cloudbilling.googleapis.com` | V7 | GetProjectBillingInfo | metrics — billed to host | 2026-09-14 |
| 5 | `cloudresourcemanager.googleapis.com` | V3 V7 V31 V77 V78 V96 V99 V113 V127 V166 V167 V175; `audit-run.go` host lookup | ListProjects, GetProject, GetOrganization, GetIamPolicy | metrics — billed to host | 2026-09-14 |
| 6 | `essentialcontacts.googleapis.com` | V181 V183 V184 | ListContacts | metrics — billed to host | 2026-09-14 |
| 7 | `iam.googleapis.com` | V88 V91 V94 V98 | ListServiceAccounts, ListServiceAccountKeys, ListWorkloadIdentityPools | metrics — billed to host | 2026-09-14 |
| 8 | `iamcredentials.googleapis.com` | impersonation, every check | GetServiceAccountAllowedLocations | metrics — billed to host | 2026-09-14 |
| 9 | `logging.googleapis.com` | V40 V96 V104 V125 V126 V128 V129 V130 V131 V136 V138 V139 V140 V146 V164 V180 | ListSinks, ListBuckets, ListLogMetrics, ListLogEntries | metrics — billed to host | 2026-09-14 |
| 10 | `monitoring.googleapis.com` | V136 V141 V158 V185 V187 | ListAlertPolicies, ListNotificationChannels | metrics — billed to host | 2026-09-14 |
| 11 | `orgpolicy.googleapis.com` | V23 V28 V32 V35 V43 V44 V47 V52 V60 V65 V70 V71 V73 V74 V79 V90 | ListPolicies, GetPolicy, GetEffectivePolicy | metrics — billed to host | 2026-09-14 |
| 12 | `policyanalyzer.googleapis.com` | V92 V97 | QueryActivity | metrics — billed to host | 2026-09-14 |
| 13 | `recommender.googleapis.com` | V103 | ListRecommendations | metrics — billed to host | 2026-09-14 |
| 14 | `securitycenter.googleapis.com` | V115 (V186 via cross-reference) | ListNotificationConfigs | **error** runs 3–6: `API DISABLED IN AUDIT HOST PROJECT: securitycenter.googleapis.com on simplifymycloud-dev`; enabled → OK in run 8 | 2026-09-13 |
| 15 | `serviceusage.googleapis.com` | V24 V45 V122 | ListServices | metrics — billed to host | 2026-09-14 |
| 16 | `spanner.googleapis.com` | V157 | ListInstances | **error** run 7: `API [spanner.googleapis.com] not enabled on project [288261943767]`; enabled → OK in run 8 | 2026-09-13 |
| 17 | `sqladmin.googleapis.com` | V14 V18 V39 V84 V121 V153 | List | metrics — billed to host, though the instance is in the audited project | 2026-09-14 |

- Enabling `securitycenter.googleapis.com` also turned on its dependency
  `securitycentermanagement.googleapis.com`.
- `storage.googleapis.com` (V30 V37 V42 V131 V139 V140 V152 V155 V159 V161–V163 V165 V168) doesn't
  report to the request-count metric, so its billing couldn't be observed. It is on by default in
  every project.
- `cloudkms.googleapis.com` is **no longer needed**. V160 now reads key IAM through Cloud Asset
  Inventory (BUG-025).

**The documented list was incomplete.** The runbook and run sheet enabled six APIs:
`cloudasset essentialcontacts accesscontextmanager recommender policyanalyzer osconfig`. Eleven of
the seventeen above were missing. `osconfig` is billed to the audited project, so enabling it in the
host did nothing. Both documents are corrected (BUG-028).

**Evidence basis.** The host project already had about 110 APIs enabled. So apart from rows 14 and
16, the evidence is "the audit called it, billed here" rather than an observed failure on a clean
project. Decision (2026-09-14): no clean-project confirmation run. Any project the audit runs from is
assumed to have these prerequisite APIs enabled.

### How the metrics were read

Cloud Monitoring, in each project, for the time window of run 8 only:

```
metric.type = "serviceruntime.googleapis.com/api/request_count"
resource.type = "consumed_api"
resource.labels.credential_id = "serviceaccount:<audit SA uniqueId>"
group by: resource.labels.service, resource.labels.method, metric.labels.response_code
```

## 2. Billed to the audited project — nothing to enable; N/A when off

These were observed billed to `iq9-gcp-dev-yamato` in run 8. When one is disabled in an audited
project, the pack's "Not applicable" table names it.

| API | Checks | Methods called in run 8 | Evidence |
|---|---|---|---|
| `compute.googleapis.com` | V6 V9 V10 V19 V49 V57 V58 V64 V67 V72 V106 V107 V111 V133 V135 V143 V147 V170 V171 V172 V174 | List, AggregatedList, Get, ListXpnHosts | metrics |
| `artifactregistry.googleapis.com` | V11 | ListLocations, ListRepositories | metrics |
| `run.googleapis.com` | V13 | ListServices | metrics |
| `cloudfunctions.googleapis.com` | V13 V17 | ListFunctions, ListLocations | metrics; run 2 error named the audited project |
| `secretmanager.googleapis.com` | V95 | ListSecrets | metrics |
| `iap.googleapis.com` | V54 | GetIapSettings | metrics |
| `container.googleapis.com` | V12 V16 V63 V75 V83 V120 V134 V156 | ListClusters | metrics; run 2 error named the audited project |
| `binaryauthorization.googleapis.com` | V20 V21 | GetPolicy | metrics; run 2 error named the audited project |
| `dns.googleapis.com` | V133 V142 V145 | List | metrics; run 2 error named the audited project |
| `osconfig.googleapis.com` | V9 V116 V118 V147 | ListPatchJobs, ListPatchDeployments | metrics; run 2 error named the audited project |
| `firestore.googleapis.com` | V157 | ListBackups | metrics |
| `websecurityscanner.googleapis.com` | V124 | ListScanConfigs | metrics; run 3 error named the audited project |
| `gkebackup.googleapis.com` | V156 | not called (no GKE clusters) | not observed |

To exercise these checks, the test enabled them in the disposable target project. The list is in
`scratch/teardown/test-infra/yamato-apis-enabled-by-test.txt` for teardown. In a customer audit, don't enable
them.

In run 8, V116 briefly came back N/A with `osconfig.googleapis.com not enabled` about 20 minutes
after the API was enabled. The same call succeeded on retry: this was propagation delay, not a
script fault.

## 3. IAM roles and permissions

**No permission gaps were found.** With the Terraform module's roles (30 predefined + 3 custom,
`enable_securitycenter = false`, `enable_billing_viewer = false`), the final run returned 0 DENIED
across 189 checks.

| # | Role / permission | Finding | Evidence | Change | Date |
|---|---|---|---|---|---|
| 1 | `roles/billing.viewer` | **Not needed.** V7 now reads the project's own billing info, which `resourcemanager.projects.get` allows. The old V7 listed billing accounts, which silently returned nothing without this role | run 3: all 18 projects falsely "NO BILLING ACCOUNT"; after the fix, correct with no billing role | `variables.tf` and module readme updated (BUG-015) | 2026-09-13 |
| 2 | `roles/securitycenter.adminViewer` | Not needed **where SCC is not activated**: V115 returned cleanly without it once the API was on. Untested on an organization with SCC activated | run 8, `enable_securitycenter = false` | none | 2026-09-14 |
| 3 | Custom roles `cisIg1Audit*` | Not a permission gap: role IDs from an earlier teardown were still reserved, so apply failed | `terraform apply` error | `custom_role_prefix` exposed in tfvars (BUG-001) | 2026-09-13 |

## 4. Local tooling the checks need

| Tool | Checks | Note |
|---|---|---|
| `jq` | 42 checks | if missing, now ERROR (`command not found`) rather than a silent PASS |
| `bq` | V38 | ships with the Cloud SDK |
| gcloud `alpha` component | V124 V136 V141 V158 V185 V187 | installed on the test machine, so not failure-tested |
| gcloud `beta` component | V156 | as above |
| `kubectl` | — | **no longer needed**: V22 uses Cloud Asset Inventory (BUG-011) |
