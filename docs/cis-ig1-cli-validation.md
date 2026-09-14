# CLI Validation Reference

Read-only `gcloud` commands for auditing IG1 compliance. **Canonical home for every validation command in this repository** — the checklist links here by V-number, and the remediation reference points here rather than repeating commands.

Every IG1 requirement falls into one of four categories. This document covers the first two; the checklist marks the other two.

| | Category | Count | Who runs it |
|---|---|---|---|
| **1** | CLI one-liner, unambiguous pass/fail | 39 | `audit-run.go` scores it |
| **2** | CLI-verifiable, but the output needs judgement | 149 | `audit-run.go` runs it, saves the output, marks it **REVIEW** — a human decides |
| **3** | A GCP task with no CLI surface (Admin Console, image build, a test that must be performed) | 30 | Human, step by step |
| **4** | Process, policy or documentation — nothing to do with infrastructure state | 72 | Human, evidence-based |

Categories 1 and 2 total 188 numbered checks below. Categories 3 and 4 total 102 and appear under each safeguard as *manual*, split by which kind they are — a console click is still a GCP audit task, whereas a written process is not.

100% automation was never the goal. The split is deliberate: a machine scores what it can defend, and everything else is handed to a person with the output already gathered.

---

## Before you start

```
export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)
gcloud config set project SECURITY_PROJECT
```

Checks discover their own resources. Where a safeguard concerns projects, Cloud SQL instances, GKE clusters, buckets, KMS keys or Cloud Routers, the command enumerates **every** one of them rather than sampling a single named resource — because **a requirement is met only when every resource meets it**. One non-compliant instance out of ten fails the check, and the output names which one.

### Two passes: organization, then project

Checks are tagged **`scope: org`** or **`scope: project`**, and they run as two separate passes.

| Pass | Checks | Command |
|---|---|---|
| **Organization** | 68 | `go run audit-run.go -scope=org` |
| **Project** | 90 | `go run audit-run.go -scope=project -project=PROJECT_ID` — **once per project** |

Run the organization pass first. It establishes the posture every project inherits, and several of its findings explain project-level results — a missing org policy constraint is why fifty projects each have a default network.

The project pass targets one project per invocation. Within that project it enumerates **every** resource of the relevant kind: every Cloud SQL instance, every node pool, every KMS key, every bucket. A requirement is met only when every resource meets it — one non-compliant instance out of ten fails the check, and the output names which one.

Only three values cannot be discovered, because they depend on your naming rather than on anything queryable: `BACKUP_BUCKET`, `TFSTATE_BUCKET` and `BACKUP_PROJECT`. Supply them with `-config`, or set them to `none` if they do not exist — which is a finding, not a skip.

**Required roles.** Verified by running a representative check from each family against a live organization.

Grant at **organization** scope:

| Role | Covers |
|---|---|
| `roles/browser` | Project enumeration |
| `roles/orgpolicy.policyViewer` | All org policy constraint checks |
| `roles/cloudasset.viewer` | Cloud Asset Inventory — resource and IAM search, ~50 checks |
| `roles/iam.securityReviewer` | IAM policy reads at org and project level |
| `roles/logging.viewer` | Sinks, buckets, metrics, log reads |
| `roles/essentialcontacts.viewer` | Incident contact checks |
| `roles/compute.viewer` | Instances, firewalls, networks, load balancers, VPN |
| `roles/accesscontextmanager.policyReader` | VPC Service Controls |

Grant at **project** scope (or organization, to inherit):

| Role | Covers |
|---|---|
| `roles/iam.serviceAccountViewer` | Service account listing |
| `roles/iam.serviceAccountKeyAdmin` *(or a custom role with `iam.serviceAccountKeys.list`)* | Service account key inventory — a **separate** permission from listing accounts |
| `roles/osconfig.inventoryViewer` | VM Manager OS inventory and patch state |
| `roles/container.viewer` | GKE clusters and node pools |
| `roles/cloudsql.viewer` | Cloud SQL instances and users |
| `roles/storage.admin` *(or custom with `storage.buckets.get` + `getIamPolicy`)* | Bucket configuration and IAM |
| `roles/monitoring.viewer` | Alert policies and notification channels |
| `roles/recommender.iamViewer` | IAM Recommender findings |
| `roles/secretmanager.viewer` | Secret rotation configuration |
| `roles/artifactregistry.reader` | Registry inventory |
| `roles/binaryauthorization.policyViewer` | Admission policy |
| `roles/dns.reader` | DNS policies and response policies |
| `roles/cloudkms.viewer` | KMS key IAM |
| `roles/securitycenter.adminViewer` | SCC, where licensed |

Reading **Data Access** audit log entries additionally requires `roles/logging.privateLogViewer` — a distinct grant from `roles/logging.viewer`, and a common omission.

Grant the org-scope set in one pass:

```
for R in browser orgpolicy.policyViewer cloudasset.viewer iam.securityReviewer \
         logging.viewer essentialcontacts.viewer compute.viewer \
         accesscontextmanager.policyReader; do
  gcloud organizations add-iam-policy-binding $ORG_ID \
    --member="serviceAccount:AUDITOR_SA@PROJECT.iam.gserviceaccount.com" \
    --role="roles/$R" --condition=None
done
```

**Enable the required APIs first.** Verified against a live organization — each of these prompts interactively mid-audit if left disabled, which is a poor time to discover it. Enabling an API is a billable-service change, so do it deliberately up front:

```
gcloud services enable \
  cloudasset.googleapis.com \
  essentialcontacts.googleapis.com \
  accesscontextmanager.googleapis.com \
  recommender.googleapis.com \
  policyanalyzer.googleapis.com \
  osconfig.googleapis.com \
  --project=SECURITY_PROJECT
```

Enable `securitycenter.googleapis.com` as well only where SCC is actually licensed. APIs are enabled on the project providing quota, not on the organization — so if you switch audit projects, do this again.

> **The audit identity is itself sensitive.** These roles let a single principal enumerate every public bucket, over-permissioned account, and open firewall rule in the organization. Use Workload Identity Federation, never an exported key — the credential that proves safeguard 5.2 should not violate it.

**Reading results.** Most checks are written so that **empty output means compliant**. Where that is not the case the pass criteria says so explicitly.

---

# Control 01 — Inventory and Control of Enterprise Assets

## 1.1 Establish and Maintain Detailed Enterprise Asset Inventory

#### V1

**Cloud Asset Inventory feed at organization scope** · checklist `1.1#1` · scope: org

```bash
gcloud asset feeds list --organization=$ORG_ID
```

**Pass:** At least one feed listed. Empty output means no continuous inventory exists.

#### V2

**Feed exports to a durable destination** · checklist `1.1#2` · scope: org

```bash
gcloud asset feeds list --organization=$ORG_ID \
  --format="table(name,feedOutputConfig.pubsubDestination.topic)"
```

**Pass:** Every feed shows a destination. A feed with no output config delivers nothing.

#### V3

**All projects enumerated, including those flat under the org node** · checklist `1.1#3` · scope: org

```bash
gcloud projects list --format="value(projectId)" | wc -l
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=cloudresourcemanager.googleapis.com/Project \
  --format="value(name)" | wc -l
```

**Pass:** Counts match. A gap means projects exist outside your inventory scope.

#### V4

**Inventory covers all major compute and data services** · checklist `1.1#4` · scope: org

```bash
gcloud asset feeds list --organization=$ORG_ID --format="value(assetTypes)"
```

**Pass:** Asset types include compute Instance, storage Bucket, sqladmin Instance, container Cluster, run Service.

#### V5

**Required ownership labels present on resources** · checklist `1.1#5` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance,storage.googleapis.com/Bucket \
  --format=json | jq -r '.[] | select(.labels.owner == null) | "UNLABELLED: \(.name)"'
```

**Pass:** Empty output. Any result is a resource with no attributable owner.

#### V6

**Shared VPC host and service project relationships mapped** · checklist `1.1#6` · scope: project

```bash
gcloud compute shared-vpc organizations list-host-projects $ORG_ID
```

**Pass:** Every host project listed is accounted for in your topology documentation.

**Manual — GCP task, no CLI surface:**

- `1.1#7` Inventory reviewed and recertified at least every six months

---

## 1.2 Address Unauthorized Assets

#### V7

**Projects with no billing account or pending deletion dispositioned** · checklist `1.2#2` · scope: project

```bash
state=$(gcloud projects describe "$PROJECT_ID" --format="value(lifecycleState)")
if [ "$state" != "ACTIVE" ]; then echo "PROJECT NOT ACTIVE: $PROJECT_ID ($state)"; fi

# Billing attached to the project under audit? Reads the project's own billing
# info, so it needs no role on the billing account — which usually sits outside
# the organization, where listing billing accounts returns nothing.
enabled=$(gcloud billing projects describe "$PROJECT_ID" --format="value(billingEnabled)") \
  || { echo "BILLING STATUS UNREADABLE: $PROJECT_ID"; exit 0; }
if [ "$enabled" != "True" ]; then echo "NO BILLING ACCOUNT: $PROJECT_ID"; fi
```

**Pass:** First command empty, or every result has a recorded disposition.

#### V8

**Orphaned resources reconciled** · checklist `1.2#3` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Disk --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.users == null) | "UNATTACHED DISK: \(.name)"'
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Address --query="state:RESERVED" --format="value(name)"
```

**Pass:** Empty, or each result is on a documented disposal list.

**Manual — GCP task, no CLI surface:**

- `1.2#1` Documented process for unowned resources

- `1.2#4` Unattributable resources escalated within a defined window

- `1.2#5` Remediation actions recorded

---

# Control 02 — Inventory and Control of Software Assets

## 2.1 Establish and Maintain a Software Inventory

#### V9

**VM Manager OS inventory enabled and agent present** · checklist `2.1#1` · scope: project

```bash
gcloud compute instances os-inventory list-instances --format="table(name,zone)"
gcloud compute instances list --format="value(name)" | wc -l
```

**Pass:** Instance counts match. A shortfall is instances with no OS Config agent.

#### V10

**Custom image catalogue maintained** · checklist `2.1#2` · scope: project

```bash
gcloud compute images list --no-standard-images --format="table(name,family,creationTimestamp,deprecated.state)"
```

**Pass:** Every image is attributable to a known build. Unfamiliar images are a finding.

#### V11

**Artifact Registry inventory captured** · checklist `2.1#3` · scope: project

```bash
gcloud artifacts repositories list --format="table(name,format,location)"
```

**Pass:** All repositories known and owned.

#### V12

**GKE cluster and node pool versions recorded** · checklist `2.1#4` · scope: project

```bash
gcloud container clusters list --format="table(name,location,currentMasterVersion,currentNodeVersion,releaseChannel.channel)"
```

**Pass:** Every cluster listed with its version captured in the inventory.

#### V13

**Serverless runtime versions recorded** · checklist `2.1#5` · scope: project

```bash
gcloud functions list --format="table(name,runtime,state)"
gcloud run services list --format="table(SERVICE,REGION)"
```

**Pass:** All runtimes captured.

#### V14

**Cloud SQL engine and version recorded** · checklist `2.1#6` · scope: project

```bash
gcloud sql instances list --format="table(name,databaseVersion,region)"
```

**Pass:** All instances captured.

**Manual — process or documentation, not infrastructure:**

- `2.1#7` Inventory refreshed automatically

---

## 2.2 Ensure Authorized Software is Currently Supported

#### V15

**No end-of-life guest operating systems** · checklist `2.2#1` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r '.[] | .name as $n | .versionedResources[]?.resource.disks[]?.licenses[]?
    | select(test("centos-7|debian-9|debian-10|ubuntu-1604|ubuntu-1804")) | "EOL OS: \($n)"'
```

**Pass:** Empty output.

#### V16

**No GKE clusters past end-of-life; all in a release channel** · checklist `2.2#2` · scope: project

```bash
gcloud container clusters list --format="value(name,currentMasterVersion,releaseChannel.channel)" \
  | awk '$3=="" {print "NO RELEASE CHANNEL: "$1" "$2}'
```

**Pass:** Empty output. Static-version clusters age out of support silently.

#### V17

**No decommissioned serverless runtimes** · checklist `2.2#3` · scope: project

```bash
gcloud functions list --format="value(name,runtime)" \
  | grep -E "nodejs(8|10|12|14)|python3(7)?|go1(11|13)?|ruby2" || echo "none found"
```

**Pass:** "none found".

#### V18

**No unsupported Cloud SQL database versions** · checklist `2.2#4` · scope: project

```bash
gcloud sql instances list --format="value(name,databaseVersion)" \
  | grep -E "MYSQL_5_6|POSTGRES_9|POSTGRES_10|SQLSERVER_2017" || echo "none found"
```

**Pass:** "none found".

#### V19

**Deprecated images not referenced by templates or MIGs** · checklist `2.2#5` · scope: project · needs `jq`

```bash
gcloud compute instance-templates list --format=json \
  | jq -r '.[] | "\(.name)\t\(.properties.disks[0].initializeParams.sourceImage // "n/a")"'
```

**Pass:** Cross-check each image against the deprecated list from V13. No matches.

**Manual — process or documentation, not infrastructure:**

- `2.2#6` Exception register for unsupported software

---

## 2.3 Address Unauthorized Software

#### V20

**Container images sourced only from approved registries** · checklist `2.3#3` · scope: project

```bash
gcloud container binauthz policy export --format="value(admissionWhitelistPatterns[].namePattern)"
```

**Pass:** Allowlist patterns match your approved registries only.

#### V21

**Binary Authorization policy in place** · checklist `2.3#4` · scope: project

```bash
gcloud container binauthz policy export \
  --format="value(defaultAdmissionRule.evaluationMode,defaultAdmissionRule.enforcementMode)"
```

**Pass:** REQUIRE_ATTESTATION and ENFORCED_BLOCK_AND_AUDIT_LOG. ALWAYS_ALLOW is a fail.

#### V22

**Workloads already running unattested identified and rolled** · checklist `2.3#5` · scope: project · needs `jq`

```bash
gcloud asset search-all-resources --scope=projects/$PROJECT_ID \
  --asset-types=k8s.io/Pod --read-mask='*' --format=json \
  | jq -r '.[] | .versionedResources[]?.resource as $p
    | "\($p.metadata.namespace // "?")\t\([$p.spec.containers[]?.image] | join(","))"' \
  | grep -v "gcr.io/google-containers" || true
```

**Pass:** Every image listed matches an approved registry.

**Manual — process or documentation, not infrastructure:**

- `2.3#1` Documented process for removing unapproved software

- `2.3#2` Marketplace deployment restricted

- `2.3#6` Findings tracked to closure

---

# Control 03 — Data Protection

## 3.1 Establish and Maintain a Data Management Process

#### V23

**Data residency requirements defined and mapped** · checklist `3.1#3` · scope: org

```bash
gcloud org-policies describe gcp.resourceLocations --organization=$ORG_ID --effective
```

**Pass:** Constraint present with an allowed-values list matching your residency policy.

**Manual — process or documentation, not infrastructure:**

- `3.1#1` Written data management process

- `3.1#2` Process references GCP storage services in use

- `3.1#4` Reviewed annually with review date recorded

---

## 3.2 Establish and Maintain a Data Inventory

#### V24

**Sensitive Data Protection discovery configured** · checklist `3.2#1` · scope: project · partial — completes with a manual step

```bash
gcloud services list --enabled --filter="dlp.googleapis.com" --project="$PROJECT_ID"
```

**Pass:** API enabled. Discovery config itself must be confirmed in the console or via REST.

#### V25

**Sensitivity classification recorded as labels** · checklist `3.2#3` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=storage.googleapis.com/Bucket,bigquery.googleapis.com/Dataset --format=json \
  | jq -r '.[] | select(.labels.data_classification == null) | "UNCLASSIFIED: \(.name)"'
```

**Pass:** Empty output.

#### V26

**Data location recorded per store** · checklist `3.2#4` · scope: org

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=storage.googleapis.com/Bucket,bigquery.googleapis.com/Dataset,sqladmin.googleapis.com/Instance \
  --format="table(name,location)"
```

**Pass:** Every location is within your approved regions.

**Manual — GCP task, no CLI surface:**

- `3.2#2` Cloud SQL and other data stores in the inventory

- `3.2#5` Inventory refreshed on a defined schedule

---

## 3.3 Configure Data Access Control Lists

#### V27

**No publicly accessible Cloud Storage buckets** · checklist `3.3#1` · scope: org

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID \
  --query='policy:("allUsers" OR "allAuthenticatedUsers")' \
  --asset-types=storage.googleapis.com/Bucket --format="table(resource)"
```

**Pass:** Empty output. Any result is a live public-exposure finding.

#### V28

**Public access prevention enforced at org level** · checklist `3.3#2` · scope: org

```bash
gcloud org-policies describe storage.publicAccessPrevention --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true.

#### V29

**All currently public buckets and datasets remediated** · checklist `3.3#3` · scope: org

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID \
  --query='policy:("allUsers" OR "allAuthenticatedUsers")' --format="table(resource,policy.bindings.role)"
```

**Pass:** Empty output across ALL asset types, not just buckets.

#### V30

**All buckets migrated off legacy per-object ACLs** · checklist `3.3#4` · scope: project · loops all projects — slow on a large estate

```bash
gcloud storage buckets list --project="$PROJECT_ID" --format="value(name)" | while read b; do
  u=$(gcloud storage buckets describe "gs://$b" --format="value(uniform_bucket_level_access)")
  if [ "$u" != "True" ]; then echo "LEGACY ACLs: $PROJECT_ID / $b"; fi
done
```

**Pass:** Empty output.

#### V31

**Pre-existing external-domain IAM grants reviewed** · checklist `3.3#5` · scope: org · needs `jq`

```bash
DOMAIN=$(gcloud organizations describe $ORG_ID --format="value(displayName)")
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --format=json \
  | jq -r --arg domain "$DOMAIN" '.[] | .resource as $r | .policy.bindings[]?.members[]?
    | select(startswith("user:") or startswith("group:"))
    | select(ascii_downcase | endswith("@" + ($domain | ascii_downcase)) | not) | "\($r)\t\(.)"' | sort -u
```

**Pass:** Empty, or every result is a documented and approved external grant.

#### V32

**Uniform bucket-level access enforced** · checklist `3.3#6` · scope: org

```bash
gcloud org-policies describe storage.uniformBucketLevelAccess --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true.

#### V33

**No publicly shared BigQuery datasets** · checklist `3.3#7` · scope: org

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID \
  --query='policy:("allUsers" OR "allAuthenticatedUsers")' \
  --asset-types=bigquery.googleapis.com/Dataset --format="table(resource)"
```

**Pass:** Empty output.

#### V34

**Access granted via groups and non-basic roles** · checklist `3.3#8` · scope: org · needs `jq`

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID \
  --query='policy:(roles/owner OR roles/editor OR roles/viewer)' --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]? | .role as $ro
    | .members[]? | select(startswith("user:")) | "\($r)\t\($ro)\t\(.)"' | sort -u
```

**Pass:** Empty output. Basic roles held by individuals are a fail.

#### V35

**Domain restriction constraint enforced** · checklist `3.3#9` · scope: org

```bash
gcloud org-policies describe iam.allowedPolicyMemberDomains --organization=$ORG_ID --effective
```

**Pass:** listPolicy present with your Cloud Identity customer IDs. Note: dry-run only is NOT compliant.

#### V36

**VPC Service Controls perimeters around regulated data** · checklist `3.3#10` · scope: org

```bash
# The policy ID is not knowable in advance — discover it first.
gcloud access-context-manager policies list --organization=$ORG_ID \
  --format="table(name,title)"

POLICY_ID=$(gcloud access-context-manager policies list --organization=$ORG_ID \
  --format="value(name)" | head -1)

gcloud access-context-manager perimeters list --policy=$POLICY_ID \
  --format="table(name,status.resources)"
```

**Pass:** Perimeters exist covering every project holding regulated data. No access policy at all means VPC Service Controls has never been configured.

**Manual — process or documentation, not infrastructure:**

- `3.3#11` Access reviewed on a defined cadence

---

## 3.4 Enforce Data Retention

#### V37

**Lifecycle rules applied to Cloud Storage buckets** · checklist `3.4#2` · scope: project · loops all projects — slow on a large estate

```bash
gcloud storage buckets list --project="$PROJECT_ID" --format="value(name)" | while read b; do
  l=$(gcloud storage buckets describe "gs://$b" --format="value(lifecycle_config)")
  if [ -z "$l" ]; then echo "NO LIFECYCLE RULE: $PROJECT_ID / $b"; fi
done
```

**Pass:** Empty, or every result is a bucket with a documented indefinite-retention justification.

#### V38

**BigQuery expiration configured on datasets** · checklist `3.4#3` · scope: project · needs `jq` · needs `bq`

```bash
for d in $(bq ls --format=json | jq -r '.[].id'); do
  exp=$(bq show --format=json "$d" | jq -r '.defaultTableExpirationMs // "none"')
  if [ "$exp" = "none" ]; then echo "NO EXPIRATION: $d"; fi
done
```

**Pass:** Empty output.

#### V39

**Cloud SQL backup retention set to the defined period** · checklist `3.4#4` · scope: project

```bash
gcloud sql instances list --format="value(name,settings.backupConfiguration.backupRetentionSettings.retainedBackups)"
```

**Pass:** Every instance shows a retention count matching policy.

#### V40

**Log bucket retention set explicitly** · checklist `3.4#5` · scope: org

```bash
gcloud logging buckets list --organization=$ORG_ID --location=global \
  --format="table(name,retentionDays,locked)"
```

**Pass:** No bucket left at 30 days unless that is the documented policy.

#### V41

**Buckets and datasets with no retention rule remediated** · checklist `3.4#6` · scope: xref · cross-reference

See V37 and V38 — both must return empty.

**Pass:** Both prerequisite checks empty.

**Manual — process or documentation, not infrastructure:**

- `3.4#1` Retention periods defined per data class

---

## 3.5 Securely Dispose of Data

#### V42

**Soft delete and versioning retention accounted for** · checklist `3.5#2` · scope: project

```bash
# Every bucket in the project — one non-compliant bucket fails the requirement
gcloud storage buckets list --project="$PROJECT_ID" --format="value(name)" | while read -r b; do
  gcloud storage buckets describe "gs://$b" \
    --format="value[separator='|'](versioning.enabled,soft_delete_policy.retentionDurationSeconds)" \
    | while IFS='|' read -r v sd; do
        echo "$PROJECT_ID / $b  versioning=${v:-off}  soft_delete=${sd:-none}"
      done
done
```

**Pass:** Values known and reflected in the disposal process. Soft delete extends real deletion time.

**Manual — process or documentation, not infrastructure:**

- `3.5#1` Documented disposal process per storage service

- `3.5#3` CMEK destruction procedure documented

- `3.5#4` Snapshots, images and backups in disposal scope

- `3.5#5` Project deletion verification procedure

- `3.5#6` Disposal actions logged

---

# Control 04 — Secure Configuration of Enterprise Assets and Software

## 4.1 Establish and Maintain a Secure Configuration Process

#### V43

**Baseline enforced through org policy constraints** · checklist `4.1#2` · scope: org

```bash
gcloud org-policies list --organization=$ORG_ID --format="table(constraint,listPolicy,booleanPolicy)"
```

**Pass:** Constraint list matches your documented baseline. Empty output is a total fail.

#### V44

**Constraints applied in dry-run first, then enforced** · checklist `4.1#3` · scope: org

```bash
gcloud org-policies list --organization=$ORG_ID --format="value(constraint)" | while read c; do
  d=$(gcloud org-policies describe "$c" --organization=$ORG_ID --format="value(dryRunSpec)" 2>/dev/null)
  s=$(gcloud org-policies describe "$c" --organization=$ORG_ID --format="value(spec)" 2>/dev/null)
  if [ -n "$d" ] && [ -z "$s" ]; then echo "DRY-RUN ONLY (not enforcing): $c"; fi
done
```

**Pass:** Empty output at audit time. A dry-run constraint blocks nothing.

#### V45

**Security Command Center enabled at org scope** · checklist `4.1#6` · scope: project

```bash
gcloud services list --enabled --filter="securitycenter.googleapis.com" --project="$PROJECT_ID"
```

**Pass:** the API is listed. `Listed 0 items.` means SCC is not enabled — a fail for this safeguard, and it makes 7.1 and 7.2 unachievable as written.

Record the tier separately; it is not exposed on this command and determines whether Vulnerability Assessment findings exist at all.

Individual detector services can be inspected once SCC is provisioned, but note `--service` takes a *detector* name (`web-security-scanner`, `container-threat-detection`, `event-threat-detection`) — there is no value meaning "SCC overall", and this surface is alpha-only:

```bash
gcloud alpha scc settings services describe --organization=$ORG_ID \
  --service=web-security-scanner
```

**Manual — process or documentation, not infrastructure:**

- `4.1#1` Written secure configuration baseline

- `4.1#4` Infrastructure provisioned via IaC

- `4.1#5` Drift detection running

- `4.1#7` Baseline reviewed annually

---

## 4.2 Establish and Maintain a Secure Configuration Process for Network Infrastructure

#### V46

**Default VPC network removed from all projects** · checklist `4.2#1` · scope: org

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Network --query='name:default' \
  --format="table(project,displayName)"
```

**Pass:** Empty output.

#### V47

**Default network creation constraint enforced** · checklist `4.2#2` · scope: org

```bash
gcloud org-policies describe compute.skipDefaultNetworkCreation --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true.

#### V48

**Default network deleted from every existing project** · checklist `4.2#3` · scope: xref · cross-reference

See V46 — the constraint is not retroactive.

**Pass:** V46 returns empty.

#### V49

**Legacy and auto-mode VPCs identified and converted** · checklist `4.2#4` · scope: project · loops all projects — slow on a large estate

```bash
gcloud compute networks list --project="$PROJECT_ID" \
  --format="value(name,x_gcloud_subnet_mode)" \
  | grep -E "LEGACY|AUTO" || true
```

**Pass:** Empty output.

#### V50

**Default firewall rules deleted** · checklist `4.2#5` · scope: org

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Firewall \
  --query='name:(default-allow-ssh OR default-allow-rdp OR default-allow-icmp OR default-allow-internal)' \
  --format="table(project,displayName)"
```

**Pass:** Empty output.

#### V51

**No auto-mode or legacy networks remain** · checklist `4.2#6` · scope: xref · cross-reference

See V49.

**Pass:** V49 returns empty.

#### V52

**VPC peering and Shared VPC constraints applied** · checklist `4.2#8` · scope: org

```bash
gcloud org-policies describe compute.restrictVpcPeering --organization=$ORG_ID --effective 2>/dev/null || echo "NOT SET"
```

**Pass:** Constraint present, or a documented decision that it is not required.

#### V53

**Firewall rules logging enabled on sensitive paths** · checklist `4.2#9` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Firewall --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.logConfig.enable != true) | "NO LOGGING: \(.name)"'
```

**Pass:** Empty for rules governing sensitive paths.

**Manual — process or documentation, not infrastructure:**

- `4.2#7` Documented network baseline

---

## 4.3 Configure Automatic Session Locking on Enterprise Assets

#### V54

**IAP session duration configured** · checklist `4.3#3` · scope: project

```bash
gcloud iap settings get --resource-type=iap_web --project="$PROJECT_ID" \
  --format="value(accessSettings.reauthSettings.maxAge)"
```

**Pass:** A finite max age is set.

**Manual — GCP task, no CLI surface:**

- `4.3#1` Google Cloud session length configured

- `4.3#2` Reauthentication applied to CLI and API access

- `4.3#4` Shell idle timeout in the baked image

---

## 4.4 Implement and Manage a Firewall on Servers

#### V55

**No firewall rules allowing the internet to SSH or RDP** · checklist `4.4#1` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Firewall --read-mask='*' --format=json \
  | jq -r 'def covers($n): (split("-") | map(tonumber)) as $r | $r[0] <= $n and $n <= ($r[1] // $r[0]);
    .[] | .name as $name | .versionedResources[]?.resource as $fw
    | select([$fw.sourceRanges[]?] | any(. == "0.0.0.0/0" or . == "::/0"))
    # "all" protocols, or tcp with no ports (= every port), or a port/range covering a watched port
    | select([$fw.allowed[]? | select(.IPProtocol == "all"
        or ((.IPProtocol == "tcp" or .IPProtocol == "6")
            and ((.ports // []) | length == 0 or any(covers(22) or covers(3389)))))] | length > 0)
    | "OPEN ADMIN PORT: \($name)"' | sort -u
```

**Pass:** Empty output.

#### V56

**No firewall rules exposing database ports to the internet** · checklist `4.4#2` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Firewall --read-mask='*' --format=json \
  | jq -r 'def covers($n): (split("-") | map(tonumber)) as $r | $r[0] <= $n and $n <= ($r[1] // $r[0]);
    .[] | .name as $name | .versionedResources[]?.resource as $fw
    | select([$fw.sourceRanges[]?] | any(. == "0.0.0.0/0" or . == "::/0"))
    # "all" protocols, or tcp with no ports (= every port), or a port/range covering a watched port
    | select([$fw.allowed[]? | select(.IPProtocol == "all"
        or ((.IPProtocol == "tcp" or .IPProtocol == "6")
            and ((.ports // []) | length == 0 or any(covers(3306) or covers(5432) or covers(1433) or covers(27017) or covers(6379)))))] | length > 0)
    | "OPEN DB PORT: \($name)"' | sort -u
```

**Pass:** Empty output.

#### V57

**Default-deny ingress posture** · checklist `4.4#3` · scope: project

```bash
gcloud compute firewall-rules list --project="$PROJECT_ID" \
  --format="table(name,direction,priority,sourceRanges.list(),denied[].map().firewall_rule().list())" \
  --sort-by=priority
```

**Pass:** A low-priority deny-all ingress rule exists and allow rules are explicit and narrower.

#### V58

**Egress rules constrained rather than allow-all** · checklist `4.4#4` · scope: project

```bash
gcloud compute firewall-rules list --project="$PROJECT_ID" \
  --filter="direction=EGRESS" --format="table(name,priority,destinationRanges.list(),allowed[].map().firewall_rule().list())"
```

**Pass:** No unrestricted allow-all egress to 0.0.0.0/0 except where documented.

#### V59

**Rules scoped by tag or service account** · checklist `4.4#5` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Firewall --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.targetTags == null
    and .versionedResources[]?.resource.targetServiceAccounts == null) | "UNSCOPED: \(.name)"'
```

**Pass:** Empty, or each result is justified.

#### V60

**Cloud SQL public IP disabled by constraint** · checklist `4.4#6` · scope: org

```bash
gcloud org-policies describe sql.restrictPublicIp --organization=$ORG_ID --effective
gcloud org-policies describe sql.restrictAuthorizedNetworks --organization=$ORG_ID --effective
```

**Pass:** Both enforced true.

#### V61

**Existing Cloud SQL instances with public IP remediated** · checklist `4.4#7` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=sqladmin.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.settings.ipConfiguration.ipv4Enabled == true)
    | "PUBLIC IP: \(.name)"'
```

**Pass:** Empty output.

#### V62

**Existing open firewall rules removed** · checklist `4.4#8` · scope: xref · cross-reference

See V55 and V56.

**Pass:** Both return empty.

#### V63

**GKE control plane authorized networks and private clusters** · checklist `4.4#9` · scope: project

```bash
gcloud container clusters list \
  --format="table(name,privateClusterConfig.enablePrivateNodes,masterAuthorizedNetworksConfig.enabled)"
```

**Pass:** Both True for every cluster.

#### V64

**Cloud Armor applied to externally exposed load balancers** · checklist `4.4#10` · scope: project · needs `jq`

```bash
gcloud compute backend-services list --global --format=json \
  | jq -r '.[] | select(.securityPolicy == null) | "NO CLOUD ARMOR: \(.name)"'
```

**Pass:** Empty for internet-facing backend services.

---

## 4.6 Securely Manage Enterprise Assets and Software

#### V65

**OS Login enforced org-wide** · checklist `4.6#1` · scope: org

```bash
gcloud org-policies describe compute.requireOsLogin --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true.

#### V66

**Existing VMs without OS Login remediated** · checklist `4.6#2` · scope: org · needs `jq`

```bash
# Effective OS Login: the instance's enable-oslogin if set, otherwise its
# project's. The org policy is not retroactive, so it proves nothing about
# instances that already exist.
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r --slurpfile projects <(gcloud asset search-all-resources --scope=organizations/$ORG_ID \
      --asset-types=compute.googleapis.com/Project --read-mask='*' --format=json) '
    def oslogin: [.[]? | select(.key == "enable-oslogin") | .value | ascii_upcase] | first;
    ($projects[0] | map({key: .project,
      value: ([.versionedResources[]?.resource.commonInstanceMetadata.items] | first | oslogin)})
      | from_entries) as $proj
    | .[] | ([.versionedResources[]?.resource.metadata.items] | first | oslogin) as $inst
    | select(($inst // $proj[.project] // "FALSE") != "TRUE")
    | "NO OS LOGIN: \(.name)"'
```

**Pass:** Empty output.

#### V67

**Project-wide SSH keys removed** · checklist `4.6#3` · scope: project · loops all projects — slow on a large estate

```bash
# "ssh-keys" is the current key; "sshKeys" the deprecated one. Both grant access.
gcloud compute project-info describe --project="$PROJECT_ID" \
  --format="value(commonInstanceMetadata.items[].key)" \
  | tr ';' '\n' | grep -Ex "ssh-keys|sshKeys" | sed "s|^|PROJECT-WIDE SSH KEYS: $PROJECT_ID |"
```

**Pass:** Empty output.

#### V68

**Existing VMs with external IPs migrated behind IAP** · checklist `4.6#4` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.networkInterfaces[]?.accessConfigs != null)
    | "EXTERNAL IP: \(.name)"'
```

**Pass:** Empty, or each result is a documented internet-facing workload.

#### V69

**Existing non-Shielded VMs replaced** · checklist `4.6#5` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.shieldedInstanceConfig.enableSecureBoot != true)
    | "NOT SHIELDED: \(.name)"'
```

**Pass:** Empty output.

#### V70

**Serial port access disabled** · checklist `4.6#6` · scope: org

```bash
gcloud org-policies describe compute.disableSerialPortAccess --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true.

#### V71

**External IP assignment restricted** · checklist `4.6#7` · scope: org

```bash
gcloud org-policies describe compute.vmExternalIpAccess --organization=$ORG_ID --effective
```

**Pass:** listPolicy denyAll true, or a narrow documented allowlist. Dry-run only is NOT compliant.

#### V72

**IAP TCP forwarding in use; no public-IP bastions** · checklist `4.6#8` · scope: project

```bash
gcloud compute firewall-rules list \
  --filter="sourceRanges:(35.235.240.0/20)" --format="table(name,network,allowed[].map().firewall_rule().list())"
```

**Pass:** IAP range rules exist; combined with V66 returning empty for admin hosts.

#### V73

**Shielded VM enforced** · checklist `4.6#9` · scope: org

```bash
gcloud org-policies describe compute.requireShieldedVm --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true.

#### V74

**IP forwarding restricted** · checklist `4.6#10` · scope: org

```bash
gcloud org-policies describe compute.vmCanIpForward --organization=$ORG_ID --effective
```

**Pass:** listPolicy denyAll true.

#### V75

**GKE hardening settings** · checklist `4.6#11` · scope: project · needs `jq`

```bash
gcloud container clusters list --format=json \
  | jq -r '.[] | "\(.name)\tABAC=\(.legacyAbac.enabled // false)\tWI=\(.workloadIdentityConfig.workloadPool // "OFF")\tSHIELDED=\(.nodeConfig.shieldedInstanceConfig.enableSecureBoot // false)"'
```

**Pass:** ABAC=false, WI set, SHIELDED=true for every cluster.

#### V76

**Secrets held in Secret Manager, not in metadata** · checklist `4.6#13` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r '.[] | .name as $n | .versionedResources[]?.resource.metadata.items[]?
    | select(.key | test("(?i)secret|password|token|apikey|api_key")) | "SECRET IN METADATA: \($n) key=\(.key)"'
```

**Pass:** Empty output.

**Manual — process or documentation, not infrastructure:**

- `4.6#12` Administrative changes made through IaC

---

## 4.7 Manage Default Accounts on Enterprise Assets and Software

#### V77

**Default Compute Engine service account not holding Editor** · checklist `4.7#1` · scope: project · loops all projects — slow on a large estate

```bash
num=$(gcloud projects describe "$PROJECT_ID" --format="value(projectNumber)")
gcloud projects get-iam-policy "$PROJECT_ID" --flatten="bindings[].members" \
  --filter="bindings.members:${num}-compute@developer.gserviceaccount.com AND bindings.role:roles/editor" \
  --format="value(bindings.role)" | sed "s|^|DEFAULT SA HAS EDITOR: $PROJECT_ID |"
```

**Pass:** Empty output.

#### V78

**Default App Engine service account reduced** · checklist `4.7#2` · scope: project · loops all projects — slow on a large estate

```bash
gcloud projects get-iam-policy "$PROJECT_ID" --flatten="bindings[].members" \
  --filter="bindings.members:${PROJECT_ID}@appspot.gserviceaccount.com AND bindings.role:roles/editor" \
  --format="value(bindings.role)" | grep -q editor && echo "APPENGINE SA HAS EDITOR: $PROJECT_ID" || true
```

**Pass:** Empty output.

#### V79

**Automatic default grants constraint enforced** · checklist `4.7#3` · scope: org

```bash
gcloud org-policies describe iam.automaticIamGrantsForDefaultServiceAccounts \
  --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true.

#### V80

**Editor stripped from default SAs in existing projects** · checklist `4.7#4` · scope: xref · cross-reference

See V77 and V78 — the constraint is not retroactive.

**Pass:** Both return empty.

#### V81

**Workloads no longer running as a default service account** · checklist `4.7#5` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r '.[] | select([.versionedResources[]?.resource.serviceAccounts[]?.email]
    | any(test("developer\\.gserviceaccount\\.com$"))) | "DEFAULT SA IN USE: \(.name)"'
```

**Pass:** Empty output.

#### V82

**Workloads run as purpose-built service accounts** · checklist `4.7#6` · scope: xref · cross-reference

See V81.

**Pass:** V81 returns empty.

#### V83

**Default GKE node service account replaced or scoped** · checklist `4.7#7` · scope: project

```bash
# Every node pool in every cluster in the project (zonal and regional)
gcloud container clusters list --project="$PROJECT_ID" --format="value(name,location)" \
  | while read -r c loc; do
      gcloud container node-pools list --cluster="$c" --location="$loc" --project="$PROJECT_ID" \
        --format="value(name,config.serviceAccount)" \
        | awk -v c="$c" '$2=="default" {print "DEFAULT NODE SA: " c " / " $1}'
    done
```

**Pass:** No node pool shows "default".

#### V84

**Cloud SQL default database users reviewed** · checklist `4.7#8` · scope: project

```bash
# Every Cloud SQL instance in the project
gcloud sql instances list --project="$PROJECT_ID" --format="value(name)" | while read -r i; do
  echo "== $PROJECT_ID / $i"
  gcloud sql users list --instance="$i" --project="$PROJECT_ID" --format="table(name,host,type)"
done
```

**Pass:** Default users removed or passwords rotated with the date recorded.

#### V85

**Default network and firewall rules removed** · checklist `4.7#9` · scope: xref · cross-reference

See V46 and V50.

**Pass:** Both return empty.

---

# Control 05 — Account Management

## 5.1 Establish and Maintain an Inventory of Accounts

#### V86

**Full IAM principal inventory** · checklist `5.1#1` · scope: org

```bash
# Written into the audit pack (audit-run.go sets AUDIT_PACK_DIR from -pack),
# not the working directory: this file is the organization's complete IAM
# inventory and belongs with the rest of the evidence.
out="${AUDIT_PACK_DIR:-./audit-state}"
mkdir -p "$out"
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID \
  --format="table(resource,policy.bindings.role)" > "$out/iam-inventory.txt"
wc -l "$out/iam-inventory.txt"
```

**Pass:** Inventory produced and retained as evidence.

#### V87

**Conditional IAM bindings included** · checklist `5.1#2` · scope: org · needs `jq`

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]? | select(.condition != null)
    | "\($r)\t\(.role)\t\(.condition.title)"'
```

**Pass:** Every conditional binding is in the inventory.

#### V88

**Service account inventory with owner and purpose** · checklist `5.1#3` · scope: project · loops all projects — slow on a large estate

```bash
gcloud iam service-accounts list --project="$PROJECT_ID" \
  --format="value(email,displayName,disabled)"
```

**Pass:** Every account has a meaningful display name and a recorded owner.

#### V89

**External principals identified** · checklist `5.1#5` · scope: xref · cross-reference

See V31.

**Pass:** V31 output reviewed and documented.

**Manual — GCP task, no CLI surface:**

- `5.1#4` Super admin accounts enumerated

- `5.1#6` Inventory generated automatically and reviewed

---

## 5.2 Use Unique Passwords

#### V90

**Service account key creation constraint enforced** · checklist `5.2#2` · scope: org

```bash
gcloud org-policies describe iam.disableServiceAccountKeyCreation --organization=$ORG_ID --effective
```

**Pass:** booleanPolicy enforced true. Dry-run only is NOT compliant.

#### V91

**Every pre-existing user-managed key inventoried** · checklist `5.2#3` · scope: project · loops all projects — slow on a large estate

```bash
gcloud iam service-accounts list --project="$PROJECT_ID" --format="value(email)" | while read sa; do
  gcloud iam service-accounts keys list --iam-account="$sa" --managed-by=user \
    --format="value(name,validAfterTime)" | while read k t; do
      echo "USER KEY: $PROJECT_ID / $sa / created $t"
  done
done
```

**Pass:** Empty output. This is the real 5.2 exposure — the constraint does not touch it.

#### V92

**Unused keys deleted, in-use keys replaced with federation** · checklist `5.2#4` · scope: project

```bash
gcloud policy-intelligence query-activity \
  --activity-type=serviceAccountKeyLastAuthentication \
  --project="$PROJECT_ID" \
  --format="table(activity.lastAuthenticatedTime,fullResourceName)"
```

**Pass:** No keys remain, or every remaining key has a documented replacement plan.

#### V93

**Existing keys inventoried, aged and eliminated** · checklist `5.2#5` · scope: xref · cross-reference

See V91 and V92.

**Pass:** V91 and V92 pass.

#### V94

**Workload Identity Federation in use** · checklist `5.2#6` · scope: project · loops all projects — slow on a large estate

```bash
gcloud iam workload-identity-pools list --location=global --project="$PROJECT_ID" \
  --format="value(name,state)"
```

**Pass:** Pools exist for every CI or external workload previously using keys.

#### V95

**Remaining static credentials in Secret Manager with rotation** · checklist `5.2#7` · scope: project

```bash
gcloud secrets list --project="$PROJECT_ID" \
  --format="table(name,replication.automatic,rotation.nextRotationTime)"
```

**Pass:** Every secret shows a rotation schedule.

**Manual — process or documentation, not infrastructure:**

- `5.2#1` No shared or generic human accounts

- `5.2#8` Break-glass credentials sealed and alerted on

---

## 5.3 Disable Dormant Accounts

#### V96

**Human account activity assessed** · checklist `5.3#2` · scope: org

```bash
# Bounded: an unbounded 90-day read of a large organization's audit logs pages
# for hours. The most recent 5,000 org-level entries name the active humans;
# a complete 90-day picture comes from the Workspace / Cloud Identity login report.
DOMAIN=$(gcloud organizations describe $ORG_ID --format="value(displayName)")
gcloud logging read \
  "protoPayload.authenticationInfo.principalEmail:\"@${DOMAIN}\"" \
  --organization=$ORG_ID --freshness=90d --limit=5000 \
  --format="value(protoPayload.authenticationInfo.principalEmail)" | sort -u
```

**Pass:** Compare against your IAM inventory (V86). A principal absent here made no org-level call in the most recent 5,000 entries — confirm dormancy in the Workspace login report before acting.

#### V97

**Service account activity assessed** · checklist `5.3#3` · scope: project

```bash
gcloud policy-intelligence query-activity \
  --activity-type=serviceAccountLastAuthentication \
  --project="$PROJECT_ID" \
  --format="table(activity.lastAuthenticatedTime,fullResourceName)"
```

**Pass:** No account exceeds the dormancy threshold.

#### V98

**Unused service accounts disabled before deletion** · checklist `5.3#4` · scope: project

```bash
gcloud iam service-accounts list --project="$PROJECT_ID" \
  --format="table(email,disabled)"
```

**Pass:** Accounts pending deletion show disabled=True.

**Manual — GCP task, no CLI surface:**

- `5.3#1` Dormancy threshold defined

- `5.3#5` Dormant review runs on a schedule

- `5.3#6` Departed-employee bindings removed

---

## 5.4 Restrict Administrator Privileges to Dedicated Administrator Accounts

#### V99

**No basic roles held by individuals at org or folder level** · checklist `5.4#1` · scope: org

```bash
gcloud organizations get-iam-policy $ORG_ID --flatten="bindings[].members" \
  --filter="bindings.role:(roles/owner OR roles/editor) AND bindings.members~^user:" \
  --format="table(bindings.role,bindings.members)"
```

**Pass:** Empty output.

#### V100

**Pre-existing basic-role grants enumerated and replaced** · checklist `5.4#2` · scope: xref · cross-reference

See V34 and V99.

**Pass:** V34 and V99 return empty.

#### V101

**Basic roles replaced throughout** · checklist `5.4#3` · scope: xref · cross-reference

See V34 — checks all levels, not just org.

**Pass:** V34 returns empty.

#### V102

**Just-in-time elevation in use** · checklist `5.4#6` · scope: org · needs `jq`

```bash
gcloud pam entitlements list --location=global --organization=$ORG_ID 2>/dev/null \
  || gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --format=json \
     | jq -r '.[] | .policy.bindings[]? | select(.condition != null) | .role' | sort -u
```

**Pass:** PAM entitlements exist, or time-bound IAM conditions are in use for privileged roles.

#### V103

**IAM Recommender findings actioned** · checklist `5.4#7` · scope: project

```bash
gcloud recommender recommendations list --project="$PROJECT_ID" --location=global \
  --recommender=google.iam.policy.Recommender \
  --format="table(content.overview.member,content.overview.removedRole,stateInfo.state)"
```

**Pass:** No ACTIVE recommendations older than your remediation SLA.

#### V104

**Alerting on org-level IAM policy changes** · checklist `5.4#8` · scope: project

```bash
gcloud logging metrics list --project="$PROJECT_ID" --format="table(name,filter)"
```

**Pass:** A metric exists filtering SetIamPolicy, with an alert policy bound to it.

**Manual — process or documentation, not infrastructure:**

- `5.4#4` Privileged access on dedicated admin identities

- `5.4#5` Super admin accounts not used for routine work

---

# Control 06 — Access Control Management

## 6.1 Establish an Access Granting Process

#### V105

**Access granted through group membership** · checklist `6.1#2` · scope: org · needs `jq`

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --format=json \
  | jq -r '[.[] | .policy.bindings[]?.members[]?] |
    {user: map(select(startswith("user:"))) | length, group: map(select(startswith("group:"))) | length}'
```

**Pass:** Group count substantially exceeds user count. A high user ratio indicates per-person bindings.

**Manual — process or documentation, not infrastructure:**

- `6.1#1` Documented access granting process

- `6.1#3` Least privilege with a defined role catalogue

- `6.1#4` Requests and approvals retained

- `6.1#5` New starter provisioning integrated

---

## 6.2 Establish an Access Revoking Process

**Manual — process or documentation, not infrastructure:**

- `6.2#1` Documented revocation procedure

- `6.2#2` Procedure covers bindings, groups, keys and OAuth

- `6.2#3` Session and token invalidation included

- `6.2#4` Defined revocation SLA

- `6.2#5` Revocation verified, not assumed

- `6.2#6` Periodic reconciliation against HR leavers

---

## 6.3 Require MFA for Externally-Exposed Applications

#### V106

**Externally exposed applications enumerated** · checklist `6.3#1` · scope: project

```bash
gcloud compute forwarding-rules list --global \
  --format="table(name,IPAddress,target,loadBalancingScheme)" \
  --filter="loadBalancingScheme=EXTERNAL OR loadBalancingScheme=EXTERNAL_MANAGED"
```

**Pass:** Every result is a known, documented application.

#### V107

**IAP fronting internet-reachable internal applications** · checklist `6.3#2` · scope: project · needs `jq`

```bash
gcloud compute backend-services list --global --format=json \
  | jq -r '.[] | "\(.name)\tIAP=\(.iap.enabled // false)"'
```

**Pass:** IAP=true for every internal application reachable from the internet.

#### V108

**Access Context Manager levels applied** · checklist `6.3#5` · scope: org

```bash
POLICY=$(gcloud access-context-manager policies list --organization=$ORG_ID \
  --format="value(name)" 2>/dev/null | head -1)
if [ -z "$POLICY" ]; then
  echo "NO ACCESS POLICY: Access Context Manager has never been configured"
else
  gcloud access-context-manager levels list --policy="$POLICY" \
    --format="table(name,title,basic.conditions)"
fi
```

**Pass:** Levels exist and are bound to administrative surfaces.

**Manual — process or documentation, not infrastructure:**

- `6.3#3` MFA enforced at the identity provider

- `6.3#4` No application relying on IP allowlisting alone

---

## 6.4 Require MFA for Remote Network Access

#### V109

**SSH and RDP routed through IAP TCP forwarding** · checklist `6.4#1` · scope: xref · cross-reference

See V72.

**Pass:** IAP range firewall rules present.

#### V110

**No VMs reachable on 22 or 3389 from the internet** · checklist `6.4#2` · scope: xref · cross-reference

See V55.

**Pass:** V55 returns empty.

#### V111

**VPN and Interconnect paths documented and MFA-gated** · checklist `6.4#4` · scope: project · partial — completes with a manual step

```bash
gcloud compute vpn-tunnels list --format="table(name,region,status,peerIp)"
gcloud compute interconnects list --format="table(name,state)"
```

**Pass:** Every path is in your topology documentation.

#### V112

**Bastion hosts removed or behind IAP** · checklist `6.4#5` · scope: xref · cross-reference

See V68 and V72.

**Pass:** No bastion appears with an external IP.

**Manual — process or documentation, not infrastructure:**

- `6.4#3` MFA enforced on the IAP identity

---

## 6.5 Require MFA for Administrative Access

#### V113

**Phishing-resistant methods for org and folder admins** · checklist `6.5#3` · scope: project · partial — completes with a manual step

```bash
gcloud organizations get-iam-policy $ORG_ID --flatten="bindings[].members" \
  --filter="bindings.role~admin OR bindings.role:roles/owner" \
  --format="value(bindings.members)" | sort -u
```

**Pass:** Produces the list of principals whose enrolment must then be confirmed in the Admin Console.

**Manual — GCP task, no CLI surface:**

- `6.5#1` 2-Step Verification enforced for all users

- `6.5#2` Phishing-resistant methods for super admins

- `6.5#4` MFA verified on the federated IdP path

- `6.5#5` Enforcement confirmed by report

- `6.5#6` Break-glass MFA exception documented

---

# Control 07 — Continuous Vulnerability Management

## 7.1 Establish and Maintain a Vulnerability Management Process

#### V114

**Security Command Center enabled with tier recorded** · checklist `7.1#2` · scope: xref · cross-reference

See V45.

**Pass:** SCC enabled and tier recorded.

**Manual — process or documentation, not infrastructure:**

- `7.1#1` Documented vulnerability management process

- `7.1#3` Vulnerability sources defined

- `7.1#4` Roles and responsibilities named

- `7.1#5` Process reviewed annually

---

## 7.2 Establish and Maintain a Remediation Process

#### V115

**Findings routed to an owning team automatically** · checklist `7.2#2` · scope: org

```bash
gcloud scc notifications list --organization=$ORG_ID \
  --format="table(name,pubsubTopic,streamingConfig.filter)"
```

**Pass:** At least one notification config exists with a live Pub/Sub topic.

**Manual — process or documentation, not infrastructure:**

- `7.2#1` Remediation SLAs defined by severity

- `7.2#3` Risk acceptance process with expiry dates

- `7.2#4` Findings tracked to closure

- `7.2#5` Monthly review against SLA

---

## 7.3 Perform Automated Operating System Patch Management

#### V116

**Patch deployments configured on a recurring schedule** · checklist `7.3#1` · scope: project

```bash
gcloud compute os-config patch-deployments list \
  --format="table(name,recurringSchedule.frequency,lastExecuteTime)"
```

**Pass:** At least one recurring deployment covering all instance groups.

#### V117

**OS Config agent coverage complete** · checklist `7.3#2` · scope: xref · cross-reference

See V9.

**Pass:** Instance counts match.

#### V118

**Patch compliance reporting reviewed** · checklist `7.3#3` · scope: project

```bash
gcloud compute os-config patch-jobs list --limit=10 \
  --format="table(name,state,createTime,instanceDetailsSummary)"
```

**Pass:** Recent jobs present and succeeding.

#### V119

**Instance templates reference current images** · checklist `7.3#5` · scope: xref · cross-reference

See V19.

**Pass:** Templates reference non-deprecated images.

#### V120

**GKE node auto-upgrade enabled** · checklist `7.3#6` · scope: project · needs `jq`

```bash
gcloud container clusters list --format=json \
  | jq -r '.[] | .name as $c | .nodePools[]?
    | select(.management.autoUpgrade != true) | "AUTO-UPGRADE OFF: \($c)/\(.name)"'
```

**Pass:** Empty output.

#### V121

**Cloud SQL maintenance windows configured** · checklist `7.3#7` · scope: project

```bash
gcloud sql instances list \
  --format="table(name,settings.maintenanceWindow.day,settings.maintenanceWindow.hour)"
```

**Pass:** Every instance shows a configured window.

**Manual — GCP task, no CLI surface:**

- `7.3#4` Image rebake cadence defined

---

## 7.4 Perform Automated Application Patch Management

#### V122

**Artifact Analysis scanning enabled** · checklist `7.4#1` · scope: project

```bash
gcloud services list --enabled --filter="containerscanning.googleapis.com" --project="$PROJECT_ID"
```

**Pass:** API enabled.

#### V123

**Serverless workloads on supported runtimes** · checklist `7.4#4` · scope: xref · cross-reference

See V17.

**Pass:** No deprecated runtimes.

#### V124

**Web Security Scanner run against exposed applications** · checklist `7.4#5` · scope: project

```bash
gcloud alpha web-security-scanner scan-configs list --project="$PROJECT_ID" \
  --format="table(displayName,startingUrls,schedule)"
```

**Pass:** Scan configs exist covering every externally exposed application from V101.

**Manual — process or documentation, not infrastructure:**

- `7.4#2` Container base image update process automated

- `7.4#3` Dependency scanning in the build pipeline

- `7.4#6` Rebuild-and-redeploy is the patching mechanism

---

# Control 08 — Audit Log Management

## 8.1 Establish and Maintain an Audit Log Management Process

#### V125

**Log exclusion filters documented and justified** · checklist `8.1#4` · scope: org · needs `jq`

```bash
gcloud logging sinks list --organization=$ORG_ID --format=json \
  | jq -r '.[] | select(.exclusions != null) | "\(.name): \(.exclusions[].filter)"'
```

**Pass:** Every exclusion is documented and none removes security-relevant events.

**Manual — process or documentation, not infrastructure:**

- `8.1#1` Written audit log management process

- `8.1#2` Log sources in scope enumerated

- `8.1#3` Ownership of the logging pipeline assigned

- `8.1#5` Process reviewed annually

---

## 8.2 Collect Audit Logs

#### V126

**Admin Activity logs flowing for all projects** · checklist `8.2#1` · scope: org

```bash
gcloud logging read 'logName:"cloudaudit.googleapis.com%2Factivity"' \
  --organization=$ORG_ID --limit=5 --freshness=1d \
  --format="value(logName,timestamp)"
```

**Pass:** Recent entries returned.

#### V127

**Data Access audit logs enabled** · checklist `8.2#2` · scope: project · needs `jq`

```bash
gcloud organizations get-iam-policy $ORG_ID --format=json \
  | jq '.auditConfigs // "NO ORG-LEVEL AUDIT CONFIG"'
```

**Pass:** auditConfigs present with DATA_READ and DATA_WRITE for allServices. The most common material gap.

#### V128

**Logging gap start date recorded** · checklist `8.2#3` · scope: org

```bash
gcloud logging read 'logName:"cloudaudit.googleapis.com%2Fdata_access"' \
  --organization=$ORG_ID --limit=1 --order=asc --format="value(timestamp)"
```

**Pass:** Earliest timestamp recorded as the start of your investigable window.

#### V129

**System Event and Policy Denied logs captured** · checklist `8.2#4` · scope: org

```bash
gcloud logging read 'logName:"cloudaudit.googleapis.com%2Fpolicy"' \
  --organization=$ORG_ID --limit=5 --freshness=30d --format="value(logName,timestamp)"
```

**Pass:** Entries returned, or a documented absence of policy denials.

#### V130

**Aggregated org-level sink with includeChildren** · checklist `8.2#5` · scope: org

```bash
gcloud logging sinks list --organization=$ORG_ID \
  --format="table(name,destination,includeChildren,filter)"
```

**Pass:** At least one sink with includeChildren=True.

#### V131

**Sink writer identity has permission on the destination** · checklist `8.2#6` · scope: org · needs `jq`

```bash
# Discover the sink destinations, then confirm each writer identity can write there
gcloud logging sinks list --organization=$ORG_ID --format="value(name,destination,writerIdentity)" \
| while read -r name dest writer; do
    case "$dest" in
      storage.googleapis.com/*)
        b="${dest#storage.googleapis.com/}"
        if gcloud storage buckets get-iam-policy "gs://$b" --format=json 2>/dev/null \
             | grep -qF "$writer"; then
          echo "ok   $name -> gs://$b"
        else
          echo "SINK CANNOT WRITE: $name -> gs://$b (writer $writer absent from bucket IAM)"
        fi ;;
      *) echo "manual  $name -> $dest (non-GCS destination, verify by hand)" ;;
    esac
  done
```

**Pass:** Writer identity appears in the destination IAM policy. Absent means the sink writes nothing, silently.

#### V132

**VPC Flow Logs enabled on sensitive subnets** · checklist `8.2#7` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Subnetwork --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.logConfig.enable != true) | "NO FLOW LOGS: \(.name)"'
```

**Pass:** Empty for subnets carrying sensitive traffic.

#### V133

**DNS, NAT and firewall logging enabled** · checklist `8.2#8` · scope: project

```bash
echo "== DNS policies (name, enableLogging)"
gcloud dns policies list --project="$PROJECT_ID" --format="value(name,enableLogging)"
echo "== Firewall rules (name, logConfig.enable)"
gcloud compute firewall-rules list --project="$PROJECT_ID" --format="value(name,logConfig.enable)"
echo "== Cloud NAT (name, logConfig.enable)"
gcloud compute routers list --project="$PROJECT_ID" --format="value(name,region)" \
| while read -r r reg; do
    gcloud compute routers nats list --router="$r" --region="$reg" --project="$PROJECT_ID" \
      --format="value(name,logConfig.enable)"
  done
```

**Pass:** Logging enabled on all.

#### V134

**GKE and Cloud SQL logs captured** · checklist `8.2#9` · scope: project

```bash
gcloud container clusters list \
  --format="table(name,loggingService,monitoringService)"
```

**Pass:** loggingService set to logging.googleapis.com/kubernetes.

#### V135

**Load balancer and Cloud Armor request logging enabled** · checklist `8.2#10` · scope: project · needs `jq`

```bash
gcloud compute backend-services list --global --format=json \
  | jq -r '.[] | select(.logConfig.enable != true) | "NO LB LOGGING: \(.name)"'
```

**Pass:** Empty output.

#### V136

**Alerting on control-plane changes** · checklist `8.2#11` · scope: project

```bash
gcloud logging metrics list --project="$PROJECT_ID" --format="value(name)"
gcloud alpha monitoring policies list --project="$PROJECT_ID" --format="value(displayName,enabled)"
```

**Pass:** Metrics exist for org policy, IAM, SA key, custom role and audit config changes, each with an enabled alert policy.

---

## 8.3 Ensure Adequate Audit Log Storage

#### V137

**Log bucket retention set explicitly** · checklist `8.3#1` · scope: xref · cross-reference

See V40.

**Pass:** No bucket left at the 30-day default.

#### V138

**Logs in a dedicated logging project** · checklist `8.3#3` · scope: org

```bash
gcloud logging sinks list --organization=$ORG_ID --format="value(destination)"
```

**Pass:** Destination project is distinct from every workload project.

#### V139

**Bucket Lock applied to the log destination** · checklist `8.3#4` · scope: org

```bash
gcloud logging sinks list --organization=$ORG_ID --format="value(destination)" \
| grep '^storage.googleapis.com/' | sed 's|^storage.googleapis.com/||' | sort -u \
| while read -r b; do
    locked=$(gcloud storage buckets describe "gs://$b" \
      --format="value(retention_policy.isLocked)" 2>/dev/null)
    if [ "$locked" != "True" ]; then echo "NO BUCKET LOCK: gs://$b"; fi
  done
```

**Pass:** isLocked=True.

#### V140

**No workload principals hold delete on log storage** · checklist `8.3#5` · scope: org · needs `jq`

```bash
gcloud logging sinks list --organization=$ORG_ID --format="value(destination)" \
| grep '^storage.googleapis.com/' | sed 's|^storage.googleapis.com/||' | sort -u \
| while read -r b; do
    echo "== gs://$b"
    gcloud storage buckets get-iam-policy "gs://$b" --format=json 2>/dev/null \
      | jq -r '.bindings[]? | select(.role | test("admin|owner|objectAdmin")) | "\(.role)\t\(.members[])"'
  done
```

**Pass:** Only the dedicated logging administrators appear.

#### V141

**Storage capacity and cost monitored** · checklist `8.3#6` · scope: project

```bash
gcloud alpha monitoring policies list --project="$PROJECT_ID" \
  --filter="displayName~log" --format="value(displayName,enabled)"
```

**Pass:** An alert exists on log ingestion volume.

**Manual — process or documentation, not infrastructure:**

- `8.3#2` Sink destination sized and budgeted

---

# Control 09 — Email and Web Browser Protections

## 9.2 Use DNS Filtering Services

#### V142

**Cloud DNS response policies applied** · checklist `9.2#1` · scope: project

```bash
gcloud dns response-policies list --format="table(responsePolicyName,networks)"
```

**Pass:** At least one policy bound to your VPC networks.

#### V143

**Egress routed through Cloud NAT or a proxy** · checklist `9.2#2` · scope: project

```bash
gcloud compute routers list --project="$PROJECT_ID" --format="value(name,region)" \
| while read -r r reg; do
    gcloud compute routers nats list --router="$r" --region="$reg" --project="$PROJECT_ID" \
      --format="value(name,natIpAllocateOption)"
  done
```

**Pass:** NAT configured; cross-check V66 for instances bypassing it with external IPs.

#### V144

**Egress firewall rules constrain destinations** · checklist `9.2#3` · scope: xref · cross-reference

See V58.

**Pass:** Egress not unrestricted allow-all.

#### V145

**Cloud DNS logging enabled** · checklist `9.2#4` · scope: project

```bash
gcloud dns policies list --format="table(name,enableLogging)"
```

**Pass:** enableLogging=True on policies attached to your networks.

#### V146

**Blocked-resolution events surfaced** · checklist `9.2#5` · scope: project

```bash
gcloud logging metrics list --project="$PROJECT_ID" --filter="name~dns" --format="value(name,filter)"
```

**Pass:** A metric and alert exist on blocked resolutions.

---

# Control 10 — Malware Defenses

## 10.1 Deploy and Maintain Anti-Malware Software

#### V147

**Agent presence verifiable through OS inventory** · checklist `10.1#3` · scope: project · needs `jq` · loops all projects — slow on a large estate

```bash
# Every instance, not just those already reporting inventory — a VM with no
# OS Config agent is exactly the one whose AV agent can't be verified.
gcloud compute instances list --project="$PROJECT_ID" --format="value(name,zone.basename())" \
  | while read -r n z; do
      if ! inv=$(gcloud compute instances os-inventory describe "$n" --zone="$z" --project="$PROJECT_ID" --format=json); then
        echo "NO OS INVENTORY (agent presence unverifiable): $n"
      elif ! echo "$inv" | jq -e '[.. | strings | select(test("clamav|falcon|defender|sentinel"; "i"))] | length > 0' >/dev/null; then
        echo "NO AV AGENT: $n"
      fi
    done
```

**Pass:** Empty, or every result is a documented exception.

#### V148

**Container image malware scanning in place** · checklist `10.1#4` · scope: xref · cross-reference

See V122.

**Pass:** containerscanning API enabled.

#### V149

**Binary Authorization preventing unattested images** · checklist `10.1#5` · scope: xref · cross-reference

See V21.

**Pass:** Policy enforcing.

#### V150

**Shielded VM integrity monitoring enabled** · checklist `10.1#6` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.shieldedInstanceConfig.enableIntegrityMonitoring != true)
    | "NO INTEGRITY MONITORING: \(.name)"'
```

**Pass:** Empty output.

**Manual — GCP task, no CLI surface:**

- `10.1#1` Anti-malware decision documented per workload class

- `10.1#2` Agent baked into the golden image

- `10.1#7` Malware scanning on untrusted-upload buckets

---

## 10.2 Configure Automatic Anti-Malware Signature Updates

#### V151

**Update path reachable from restricted-egress instances** · checklist `10.2#2` · scope: project · run per representative instance

```sh
gcloud compute ssh INSTANCE --tunnel-through-iap --project="$PROJECT_ID" \
  --command="systemctl is-active clamav-freshclam; freshclam --version"
```

*Run by hand. `audit-run.go` does not execute `sh` blocks: this logs in to an instance, which a read-only audit identity must not do.*

**Pass:** Service active and version current. The common silent failure is egress filtering blocking the mirror.

**Manual — GCP task, no CLI surface:**

- `10.2#1` Automatic signature updates enabled

- `10.2#3` Signature currency monitored

- `10.2#4` Registry scanning definitions maintained

---

# Control 11 — Data Recovery

## 11.1 Establish and Maintain a Data Recovery Process

#### V152

**Terraform state backend versioning in scope** · checklist `11.1#4` · scope: project

```bash
gcloud storage buckets describe gs://TFSTATE_BUCKET --format="value(versioning.enabled)"
```

**Pass:** True. State loss is a recovery event.

**Manual — process or documentation, not infrastructure:**

- `11.1#1` Written recovery process

- `11.1#2` RPO and RTO defined per workload tier

- `11.1#3` Recovery roles and escalation named

- `11.1#5` Process reviewed annually and after events

---

## 11.2 Perform Automated Backups

#### V153

**Cloud SQL automated backups with PITR** · checklist `11.2#1` · scope: project

```bash
gcloud sql instances list \
  --format="table(name,settings.backupConfiguration.enabled,settings.backupConfiguration.pointInTimeRecoveryEnabled)"
```

**Pass:** Both True for every instance.

#### V154

**Snapshot schedules attached to data-bearing disks** · checklist `11.2#2` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Disk --read-mask='*' --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.resourcePolicies == null) | "NO SNAPSHOT SCHEDULE: \(.name)"'
```

**Pass:** Empty for data-bearing disks. A policy that exists but is unattached backs up nothing.

#### V155

**Bucket versioning and soft delete enabled** · checklist `11.2#3` · scope: project

```bash
gcloud storage buckets list --project="$PROJECT_ID" --format="value(name)" | while read -r b; do
  v=$(gcloud storage buckets describe "gs://$b" --format="value(versioning.enabled)")
  if [ "$v" != "True" ]; then echo "NO VERSIONING: $PROJECT_ID / $b"; fi
done
```

**Pass:** Versioning True on data-bearing buckets.

#### V156

**Backup for GKE configured where needed** · checklist `11.2#4` · scope: project

```bash
gcloud container clusters list --project="$PROJECT_ID" --format="value(location)" | sort -u \
| while read -r loc; do
    gcloud beta container backup-restore backup-plans list --location="$loc" --project="$PROJECT_ID" \
      --format="value(name,cluster,backupSchedule.cronSchedule)"
  done
```

**Pass:** Plans exist for every cluster holding persistent state.

#### V157

**Other data services backed up** · checklist `11.2#5` · scope: project

```bash
gcloud firestore backups list --project="$PROJECT_ID" --format="value(name)"
gcloud spanner instances list --project="$PROJECT_ID" --format="value(name)" | while read -r si; do
  gcloud spanner backups list --instance="$si" --project="$PROJECT_ID" --format="value(name)"
done
```

**Pass:** Backups present for every service in use.

#### V158

**Backup success and failure monitored** · checklist `11.2#6` · scope: project

```bash
gcloud alpha monitoring policies list --project="$PROJECT_ID" \
  --filter="displayName~backup" --format="value(displayName,enabled)"
```

**Pass:** An alert exists on backup failure. Silent failure is the common mode.

**Manual — process or documentation, not infrastructure:**

- `11.2#7` Backup frequency meets the defined RPO

---

## 11.3 Protect Recovery Data

#### V159

**Backup data encrypted with CMEK where required** · checklist `11.3#1` · scope: project

```bash
gcloud storage buckets describe gs://BACKUP_BUCKET \
  --format="value(default_kms_key)"
```

**Pass:** A KMS key is set where key control is required.

#### V160

**KMS key access separated from production identities** · checklist `11.3#2` · scope: project · needs `jq`

```bash
# Every KMS key in the project, in every location, and who holds which role on it.
# One Asset Inventory call instead of keyrings × locations × keys.
gcloud asset search-all-iam-policies --scope=projects/$PROJECT_ID \
  --asset-types=cloudkms.googleapis.com/CryptoKey --format=json \
  | jq -r '.[] | .resource as $k | .policy.bindings[]? | "\($k)\t\(.role)\t\(.members | join(","))"'
```

**Pass:** No production workload service account appears.

#### V161

**Backup storage IAM restricted to a dedicated role** · checklist `11.3#3` · scope: project · needs `jq`

```bash
gcloud storage buckets get-iam-policy gs://BACKUP_BUCKET --format=json \
  | jq -r '.bindings[]? | "\(.role)\t\(.members[])"'
```

**Pass:** Only the dedicated backup identity has write access.

#### V162

**No production SA holds delete on backups** · checklist `11.3#4` · scope: project · needs `jq`

```bash
gcloud storage buckets get-iam-policy gs://BACKUP_BUCKET --format=json \
  | jq -r '.bindings[]? | select(.role | test("admin|objectAdmin|owner")) | "\(.role)\t\(.members[])"'
```

**Pass:** No production identity appears. objectCreator without delete is the target state.

#### V163

**Bucket Lock applied to backup buckets** · checklist `11.3#5` · scope: project

```bash
gcloud storage buckets describe gs://BACKUP_BUCKET \
  --format="value(retention_policy.isLocked,retention_policy.retentionPeriod)"
```

**Pass:** isLocked=True.

#### V164

**Backup deletion events logged and alerted** · checklist `11.3#6` · scope: project

```bash
gcloud logging metrics list --project="$PROJECT_ID" --filter="name~backup" --format="value(name,filter)"
```

**Pass:** A metric exists on storage.objects.delete in the backup bucket, with an alert.

---

## 11.4 Establish and Maintain an Isolated Instance of Recovery Data

#### V165

**Backup copy in a separate project** · checklist `11.4#1` · scope: project

```bash
gcloud storage buckets describe gs://BACKUP_BUCKET --format="value(project_number)"
```

**Pass:** Project differs from every production workload project.

#### V166

**Backup project under a separate folder** · checklist `11.4#2` · scope: project

```bash
backup_project=BACKUP_PROJECT
gcloud projects describe "$backup_project" --format="value(parent.type,parent.id)"
```

**Pass:** Parent folder differs from production folders.

#### V167

**No shared credential reaches both production and backups** · checklist `11.4#3` · scope: project · needs `jq`

```bash
backup_project=BACKUP_PROJECT
gcloud projects get-iam-policy "$backup_project" --format=json \
  | jq -r '.bindings[]?.members[]? | select(test("prod|production"))'
```

**Pass:** Empty output.

#### V168

**Copy held in a different region or multi-region** · checklist `11.4#4` · scope: project

```bash
gcloud storage buckets describe gs://BACKUP_BUCKET --format="value(location,location_type)"
```

**Pass:** Location differs from the production data region.

**Manual — GCP task, no CLI surface:**

- `11.4#5` Isolation verified by test

- `11.4#6` Restore from the isolated copy tested and dated

---

# Control 12 — Network Infrastructure Management

## 12.1 Ensure Network Infrastructure is Up-to-Date

#### V169

**GKE control plane and nodes within the supported window** · checklist `12.1#1` · scope: xref · cross-reference

See V16.

**Pass:** All clusters in a release channel and current.

#### V170

**Classic VPN migrated to HA VPN** · checklist `12.1#2` · scope: project

```bash
gcloud compute vpn-gateways list --format="table(name,region)"
gcloud compute target-vpn-gateways list --format="table(name,region)"
```

**Pass:** Second command empty — target-vpn-gateways are Classic VPN.

#### V171

**Load balancer SSL policies at a modern TLS minimum** · checklist `12.1#3` · scope: project

```bash
gcloud compute ssl-policies list --format="table(name,profile,minTlsVersion)"
gcloud compute target-https-proxies list --format="table(name,sslPolicy)"
```

**Pass:** Every proxy references a policy with minTlsVersion TLS_1_2 or higher. A proxy with no policy uses permissive defaults.

#### V172

**Legacy load balancers migrated** · checklist `12.1#4` · scope: project

```bash
gcloud compute target-http-proxies list --format="table(name,urlMap)"
gcloud compute target-pools list --format="table(name,region)"
```

**Pass:** target-pools empty; HTTP proxies only where redirect-to-HTTPS is intended.

#### V173

**Legacy networks eliminated** · checklist `12.1#5` · scope: xref · cross-reference

See V49.

**Pass:** V49 returns empty.

#### V174

**Cloud Armor rule sets current** · checklist `12.1#6` · scope: project

```bash
gcloud compute security-policies list --format="table(name,rules.len())"
gcloud compute security-policies list --format="value(name)" | while read -r pol; do
  echo "== $pol"
  gcloud compute security-policies describe "$pol" --format="value(rules[].description)"
done
```

**Pass:** Policies exist with current preconfigured expression sets.

**Manual — process or documentation, not infrastructure:**

- `12.1#7` Deprecated API versions removed from tooling

- `12.1#8` Network topology diagram current

---

# Control 15 — Service Provider Management

## 15.1 Establish and Maintain an Inventory of Service Providers

#### V175

**Third-party service accounts inventoried** · checklist `15.1#3` · scope: org · needs `jq`

```bash
# Projects belonging to this organization
org_projects=$(mktemp)
gcloud projects list --format="value(projectId)" | sort -u > "$org_projects"

# Every service account holding a binding, resolved back to its owning project
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]?.members[]?
    | select(startswith("serviceAccount:")) | sub("^serviceAccount:"; "")
    | "\($r)\t\(.)"' \
  | sort -u \
  | while IFS=$'\t' read -r res sa; do
      domain="${sa##*@}"
      case "$domain" in
        *.iam.gserviceaccount.com) proj="${domain%.iam.gserviceaccount.com}" ;;
        developer.gserviceaccount.com|appspot.gserviceaccount.com) continue ;;  # default SAs
        *.gserviceaccount.com) continue ;;                                      # Google-managed
        *) echo "NON-GCP IDENTITY: $res  $sa"; continue ;;
      esac
      grep -qx "$proj" "$org_projects" || echo "EXTERNAL SA: $res  $sa"
    done
```

**Pass:** Every result is a documented third-party integration.

An earlier version of this check filtered on `gserviceaccount.com$`, which excluded **every** service account — all GCP service accounts share that domain — so it silently returned empty on any organization. Third-party identities are distinguished by their owning *project*, not their domain, which is what the version above resolves.

#### V176

**Workload Identity Federation trusts documented** · checklist `15.1#4` · scope: xref · cross-reference

See V94.

**Pass:** Every pool and provider is in the service provider register.

#### V177

**Cross-project and cross-org grants identified** · checklist `15.1#6` · scope: org · needs `jq`

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]?.members[]?
    | select(startswith("serviceAccount:")) | "\($r)\t\(.)"' \
  | awk -F'\t' '{split($2,a,"@"); split(a[2],b,"."); if (index($1,b[1])==0) print}'
```

**Pass:** Every cross-project grant is documented.

#### V178

**Domain restriction constraint enforced** · checklist `15.1#7` · scope: xref · cross-reference

See V35.

**Pass:** Constraint enforced, not dry-run.

#### V179

**External grants predating the constraint enumerated** · checklist `15.1#8` · scope: xref · cross-reference

See V31.

**Pass:** V31 reviewed and documented.

**Manual — process or documentation, not infrastructure:**

- `15.1#1` Google Cloud recorded as a service provider

- `15.1#2` Marketplace solutions and vendors enumerated

- `15.1#5` OAuth applications reviewed

- `15.1#9` Inventory reviewed at least annually

---

# Control 17 — Incident Response Management

## 17.1 Designate Personnel to Manage Incident Handling

#### V180

**Break-glass procedure documented and alerted on** · checklist `17.1#3` · scope: project · partial — completes with a manual step

```bash
gcloud logging metrics list --project="$PROJECT_ID" --filter="name~breakglass OR name~break_glass" --format="value(name,filter)"
```

**Pass:** A metric and alert exist on break-glass account use.

**Manual — process or documentation, not infrastructure:**

- `17.1#1` Named individual accountable for incident handling

- `17.1#2` Deputy or escalation path named

- `17.1#4` Support plan level recorded

- `17.1#5` Designation reviewed annually

---

## 17.2 Establish and Maintain Contact Information for Reporting Security Incidents

#### V181

**Essential Contacts set for the Security category** · checklist `17.2#1` · scope: org

```bash
gcloud essential-contacts list --organization=$ORG_ID \
  --format="table(email,notificationCategorySubscriptions.list())"
```

**Pass:** At least one contact subscribed to SECURITY. Empty output means Google’s security notices go nowhere useful.

#### V182

**Contacts set for Legal, Suspension and Technical** · checklist `17.2#2` · scope: xref · cross-reference

See V181.

**Pass:** All four categories covered.

#### V183

**Contacts point to monitored group addresses** · checklist `17.2#3` · scope: org

```bash
gcloud essential-contacts list --organization=$ORG_ID --format="value(email)"
```

**Pass:** Every address is a distribution group, not an individual mailbox.

#### V184

**Pre-existing contacts reviewed and replaced** · checklist `17.2#4` · scope: project · loops all projects — slow on a large estate

```bash
gcloud essential-contacts list --project="$PROJECT_ID" --format="value(email)"
```

**Pass:** No personal or departed-employee addresses remain.

#### V185

**Monitoring notification channels verified** · checklist `17.2#6` · scope: project

```bash
gcloud alpha monitoring channels list --project="$PROJECT_ID" \
  --format="table(displayName,type,labels.email_address,enabled)"
```

**Pass:** No departed-employee addresses; all channels enabled.

**Manual — GCP task, no CLI surface:**

- `17.2#5` Cloud Billing account contacts current

- `17.2#7` Contacts verified by test message

---

## 17.3 Establish and Maintain an Enterprise Process for Reporting Incidents

#### V186

**SCC findings routed to a monitored destination** · checklist `17.3#1` · scope: xref · cross-reference

See V115.

**Pass:** Notification config exists with a live topic.

#### V187

**Log-based alerts route to an on-call rotation** · checklist `17.3#2` · scope: project · needs `jq`

```bash
gcloud alpha monitoring policies list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.notificationChannels | length == 0) | "NO CHANNEL: \(.displayName)"'
```

**Pass:** Empty output. An alert with no channel fires into nothing.

#### V188

**Log retention sufficient for investigation** · checklist `17.3#5` · scope: xref · cross-reference

See V40.

**Pass:** Retention meets the investigation window.

**Manual — process or documentation, not infrastructure:**

- `17.3#3` Documented path to raise a suspected incident

- `17.3#4` Handoff to the enterprise incident function defined

- `17.3#6` Reporting path tested and dated

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This is an implementation aid, not affiliated with or endorsed by CIS. Commands are unverified against a live organization — test before relying on them for audit evidence.*
