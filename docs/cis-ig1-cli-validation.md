# CLI Validation Reference

Read-only `gcloud` commands for auditing IG1 compliance. **Canonical home for every validation command in this repository** — the checklist links here by V-number, and the remediation reference points here rather than repeating commands.

Every IG1 requirement falls into one of four categories. This document covers the first two; the checklist marks the other two.

| | Category | Count | Who runs it |
|---|---|---|---|
| **1** | CLI check, unambiguous pass/fail | 109 | `audit-run.go` scores it (103 PASS/FAIL, 6 evidence captures) |
| **2** | CLI-verifiable, but the output needs judgement | 79 | `audit-run.go` runs it, saves the output, marks it **REVIEW** — a human decides (48 checks, 30 cross-references, 1 by hand) |
| **3** | A GCP task with no CLI surface (Admin Console, image build, a test that must be performed) | 30 | Human, step by step |
| **4** | Process, policy or documentation — nothing to do with infrastructure state | 72 | Human, evidence-based |

Categories 1 and 2 total 188 numbered checks below. Categories 3 and 4 total 102 and appear under each safeguard as *manual*, split by which kind they are — a console click is still a GCP audit task, whereas a written process is not.

100% automation was never the goal. The split is deliberate: a machine scores what it can defend, and everything else is handed to a person with the output already gathered.

---

## Before you start

Run these commands as the **audit service account**, never as yourself. An operator with Owner passes checks the audit identity would fail, so a manual pass run under your own credentials measures your access rather than the estate's posture.

### 1. Open a shell

**Cloud Shell** is the quickest — it already has `gcloud`, `jq` and Go, and it authenticates as the account you signed into the Console with. Open [shell.cloud.google.com](https://shell.cloud.google.com), or the terminal icon in the Console toolbar.

Locally instead, confirm the tooling first:

```bash
gcloud version | head -1 && jq --version
gcloud components list --filter="id:(alpha beta)" --format="value(id,state.name)"
```

`alpha` and `beta` must not say `Not Installed` — seven checks need them (`gcloud components install alpha beta`).

### 2. Authenticate as yourself

```bash
gcloud auth login
```

Cloud Shell is already signed in, so this is only needed if the session has expired — which it does on a Workspace session timeout, mid-audit, with `Reauthentication failed. cannot prompt during non-interactive execution`.

### 3. Set the variables the checks read

```bash
export ORG_ID="REPLACE_ORG_ID"                  # gcloud organizations list
export AUDIT_PROJECT="REPLACE_AUDIT_PROJECT"    # the project the APIs are enabled in
export SA_EMAIL="cis-ig1-auditor@${AUDIT_PROJECT}.iam.gserviceaccount.com"

gcloud config set project "$AUDIT_PROJECT"
```

Project-scope checks additionally read `$PROJECT_ID` — **the project under audit, not the audit host**:

```bash
export PROJECT_ID="REPLACE_PROJECT_UNDER_AUDIT"
```

Export it again for every project you move to. Leave it unset and the commands silently audit whatever `gcloud config` points at, which reports the wrong project's posture without erroring — it is the single easiest way to produce a clean, wrong audit.

### 4. Impersonate the audit service account

```bash
gcloud config set auth/impersonate_service_account "$SA_EMAIL"
gcloud config get-value auth/impersonate_service_account
```

That must print `cis-ig1-auditor@…`. Every command afterwards carries a banner naming the impersonated account; that banner is expected, and the checks in this document strip it from their output.

`gcloud auth list` still shows **your** address, and that is correct. Impersonation does not change the authenticated account — gcloud exchanges your credential for a short-lived service account token on each call, which is exactly why the audit log records both identities.

A fresh grant takes a minute or two to propagate. Until it does, commands fail with `Failed to impersonate`.

### 5. Prove it took effect

Ask Google who the token belongs to. This reads; it changes nothing:

```bash
curl -s "https://oauth2.googleapis.com/tokeninfo?access_token=$(gcloud auth print-access-token)" | jq -r .email
```

Expected: `cis-ig1-auditor@…`. That is the identity every subsequent command runs as.

- **Your own address** — impersonation is not active. Redo step 4 and check `gcloud config get-value auth/impersonate_service_account` prints the account.
- `Failed to impersonate` — the grant has not propagated. Wait a minute and retry.

> **Do not test this by attempting to create something.** A write that fails proves nothing on its own — an operator without the permission is denied whether or not impersonation is active — and a write that *succeeds* means you have created a real resource in the customer's project and now have to remove it. An audit that promises to be read-only should not open with a write, and the attempt is recorded in their Admin Activity log either way.

### 6. Placeholder values

Eleven bare words in the commands below (`APPROVED_REGISTRIES`, `DORMANCY_DAYS`, `BACKUP_BUCKET` and the rest) are substituted by `audit-run.go` from its config file. Running by hand, **replace them yourself** — your shell will not, and an unsubstituted placeholder produces a command that runs and returns nothing, which reads like a pass.

If a resource genuinely does not exist, that is a **finding**, not a check to skip.

### When you are finished

```bash
gcloud config unset auth/impersonate_service_account
```

Leave it set and every later `gcloud` command in that shell still runs as the auditor — including ones you intend to run as yourself.

---

Checks discover their own resources. Where a safeguard concerns projects, Cloud SQL instances, GKE clusters, buckets, KMS keys or Cloud Routers, the command enumerates **every** one of them rather than sampling a single named resource — because **a requirement is met only when every resource meets it**. One non-compliant instance out of ten fails the check, and the output names which one.

### Two passes: organization, then project

Checks are tagged **`scope: org`** or **`scope: project`**, and they run as two separate passes.

| Pass | Checks | Command |
|---|---|---|
| **Organization** | 71 | `go run audit-run.go -scope=org` |
| **Project** | 87 | `go run audit-run.go -scope=project -project=PROJECT_ID` — **once per project** |

Run the organization pass first. It establishes the posture every project inherits, and several of its findings explain project-level results — a missing org policy constraint is why fifty projects each have a default network.

The project pass targets one project per invocation. Within that project it enumerates **every** resource of the relevant kind: every Cloud SQL instance, every node pool, every KMS key, every bucket. A requirement is met only when every resource meets it — one non-compliant instance out of ten fails the check, and the output names which one.

**Prerequisites.** Eleven values can't be discovered, because they depend on your naming or your policy rather than on anything queryable. Supply them in the `-config` file (`-init-config` writes a template), or set one to `none` if it does not exist — which is a finding, not a skip.

| Value | What it is | Example | Checks |
|---|---|---|---|
| `BACKUP_BUCKET` | The bucket holding backups | `acme-backups` | 6 |
| `TFSTATE_BUCKET` | The bucket holding Terraform state | `acme-tf-state` | 1 |
| `BACKUP_PROJECT` | The project holding isolated backup copies | `acme-backup` | 2 |
| `APPROVED_REGISTRIES` | Approved container registry prefixes | `us-docker.pkg.dev/acme,gke.gcr.io` | 2 |
| `ALLOWED_LOCATIONS` | Locations data may live in | `us,us-central1,us-west1` | 2 |
| `LOG_RETENTION_DAYS` | Minimum log bucket retention, days | `400` | 1 |
| `SQL_BACKUP_RETENTION` | Minimum automated backups per Cloud SQL instance | `30` | 1 |
| `DORMANCY_DAYS` | Days without authentication before a service account is dormant | `90` | 1 |
| `BACKUP_IDENTITY` | The one service account allowed to write backups | `backup-writer@acme-backup.iam.gserviceaccount.com` | 2 |
| `PRODUCTION_PROJECTS` | Regular expressions matching production project IDs | `^acme-prod-,^acme-pci-` | 3 |
| `PRODUCTION_REGIONS` | Regions production data lives in | `us-west1,us-east1` | 1 |

Lists are comma-separated with no spaces. Each value turns a judgement call into a PASS/FAIL, so these checks are scored automatically.

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
set -o pipefail
n=$(gcloud asset feeds list --organization=$ORG_ID --format=json \
  | jq '(if type == "array" then . else (.feeds // []) end) | length') || exit 1
if [ "$n" -eq 0 ]; then echo "NO ORGANIZATION ASSET FEED: no continuous inventory exists"; fi
```

**Pass:** Empty output. A line means no organization-level Cloud Asset Inventory feed exists.

#### V2

**Feed exports to a durable destination** · checklist `1.1#2` · scope: org

```bash
gcloud asset feeds list --organization=$ORG_ID --format=json \
  | jq -r '(if type == "array" then . else (.feeds // []) end)[]
    | select(.feedOutputConfig.pubsubDestination.topic == null)
    | "FEED WITHOUT DESTINATION: \(.name)"'
```

**Pass:** Empty output. Each line is a feed with no Pub/Sub destination, which delivers nothing.

#### V3

**All projects enumerated, including those flat under the org node** · checklist `1.1#3` · scope: org

```bash
set -o pipefail
a=$(gcloud projects list --format="value(projectId)" | wc -l | tr -d ' ') || exit 1
b=$(gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=cloudresourcemanager.googleapis.com/Project --format="value(name)" | wc -l | tr -d ' ') || exit 1
if [ "$a" != "$b" ]; then echo "PROJECT COUNT MISMATCH: gcloud projects list=$a, Asset Inventory=$b"; fi
```

**Pass:** Empty output. A line means the two counts differ — projects exist outside the inventory scope.

#### V4

**Inventory covers all major compute and data services** · checklist `1.1#4` · scope: org

```bash
gcloud asset feeds list --organization=$ORG_ID --format=json \
  | jq -r '[(if type == "array" then . else (.feeds // []) end)[] | .assetTypes[]?] as $have
    | ["compute.googleapis.com/Instance", "storage.googleapis.com/Bucket", "sqladmin.googleapis.com/Instance",
       "container.googleapis.com/Cluster", "run.googleapis.com/Service"][]
    | select(. as $t | $have | index($t) | not) | "NOT IN ANY FEED: \(.)"'
```

**Pass:** Empty output. Each line is a major asset type no feed covers.

#### V5

**Required ownership labels present on resources** · checklist `1.1#5` · scope: org · needs `jq`

```bash
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance,storage.googleapis.com/Bucket \
  --format=json | jq -r '.[] | select(.labels.owner == null) | "UNLABELLED: \(.name)"'
```

**Pass:** Empty output. Any result is a resource with no attributable owner.

#### V6

**Shared VPC host and service project relationships mapped** · checklist `1.1#6` · scope: org

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

**Pass:** Empty output. Each line is a project that is not ACTIVE or has no billing account — a finding until its disposition is recorded.

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
set -o pipefail
all=$(gcloud compute instances list --project="$PROJECT_ID" --format="value(name)" | sort) || exit 1
inv=$(gcloud compute instances os-inventory list-instances --project="$PROJECT_ID" --format="value(name)" | sort)
comm -23 <(printf '%s\n' "$all" | grep .) <(printf '%s\n' "$inv" | grep .) | sed 's/^/NO OS INVENTORY: /'
```

**Pass:** Empty output. Each line is an instance reporting no OS inventory — no OS Config agent, or the API is off.

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

**Pass:** Evidence: the GKE cluster and node pool versions, recorded as the inventory. PASS when the command runs.

#### V13

**Serverless runtime versions recorded** · checklist `2.1#5` · scope: project

```bash
gcloud functions list --format="table(name,runtime,state)"
gcloud run services list --format="table(SERVICE,REGION)"
```

**Pass:** Evidence: the serverless runtime versions, recorded as the inventory. PASS when the command runs.

#### V14

**Cloud SQL engine and version recorded** · checklist `2.1#6` · scope: project

```bash
gcloud sql instances list --format="table(name,databaseVersion,region)"
```

**Pass:** Evidence: the Cloud SQL engine versions, recorded as the inventory. PASS when the command runs.

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
gcloud functions list --project="$PROJECT_ID" --format="value(name.basename(),runtime)" \
  | awk '$2 ~ /^(nodejs(8|10|12|14)|python37|go1(11|13)|ruby2[0-9])$/ {print "DECOMMISSIONED RUNTIME: " $1 " (" $2 ")"}'
```

**Pass:** Empty output. Each line is a function on a decommissioned runtime.

#### V18

**No unsupported Cloud SQL database versions** · checklist `2.2#4` · scope: project

```bash
gcloud sql instances list --project="$PROJECT_ID" --format="value(name,databaseVersion)" \
  | awk '$2 ~ /^(MYSQL_5_6|POSTGRES_9|POSTGRES_10|SQLSERVER_2017)/ {print "UNSUPPORTED VERSION: " $1 " (" $2 ")"}'
```

**Pass:** Empty output. Each line is an instance on an unsupported database version.

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
approved=APPROVED_REGISTRIES
gcloud container binauthz policy export --project="$PROJECT_ID" --format=json \
  | jq -r --arg approved "$approved" '($approved | split(",")) as $ok
    | .admissionWhitelistPatterns[]?.namePattern
    | select(. as $p | $ok | map(. as $a | $p | startswith($a)) | any | not)
    | "UNAPPROVED ALLOWLIST PATTERN: \(.)"'
```

**Pass:** Empty output. Requires the APPROVED_REGISTRIES prerequisite. Each line is an allowlist pattern outside the approved registries.

#### V21

**Binary Authorization policy in place** · checklist `2.3#4` · scope: project

```bash
gcloud container binauthz policy export --project="$PROJECT_ID" --format=json \
  | jq -r '.defaultAdmissionRule as $d
    | select($d.evaluationMode != "REQUIRE_ATTESTATION" or $d.enforcementMode != "ENFORCED_BLOCK_AND_AUDIT_LOG")
    | "BINAUTHZ NOT ENFORCING: evaluationMode=\($d.evaluationMode) enforcementMode=\($d.enforcementMode)"'
```

**Pass:** Empty output. A line means the default rule does not require attestation with enforcement (ALWAYS_ALLOW is a fail).

#### V22

**Workloads already running unattested identified and rolled** · checklist `2.3#5` · scope: project · needs `jq`

```bash
approved=APPROVED_REGISTRIES
gcloud asset search-all-resources --scope=projects/$PROJECT_ID \
  --asset-types=k8s.io/Pod --read-mask='*' --format=json \
  | jq -r --arg approved "$approved" '($approved | split(",")) as $ok
    | .[] | .versionedResources[]?.resource as $p
    | select(($p.metadata.namespace // "") | test("^(kube-system|gke-|gmp-)") | not)
    | $p.spec.containers[]?.image
    | select(. as $i | $ok | map(. as $a | $i | startswith($a)) | any | not)
    | "UNAPPROVED IMAGE: \($p.metadata.namespace // "?")/\($p.metadata.name // "?") \(.)"' | sort -u
```

**Pass:** Empty output. Requires the APPROVED_REGISTRIES prerequisite. Each line is a running pod image from outside the approved registries (Google-managed system namespaces excluded).

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
allowed=ALLOWED_LOCATIONS
gcloud org-policies describe gcp.resourceLocations --organization=$ORG_ID --effective --format=json \
  | jq -r --arg allowed "$allowed" '($allowed | ascii_downcase | split(",")) as $ok
    | [.spec.rules[]?] as $r
    | if ($r | length) == 0 or ($r | map(.allowAll == true) | any)
      then "NOT RESTRICTED: gcp.resourceLocations allows every location"
      else ($r[].values.allowedValues[]?
        | select((ascii_downcase | sub("^in:"; "") | sub("-locations$"; "")) as $v | $ok | index($v) | not)
        | "ALLOWED BY POLICY BUT NOT IN ALLOWED_LOCATIONS: \(.)")
      end'
```

**Pass:** Empty output. Requires the ALLOWED_LOCATIONS prerequisite. A line means the constraint is unset, or allows a location outside ALLOWED_LOCATIONS.

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
allowed=ALLOWED_LOCATIONS
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=storage.googleapis.com/Bucket,bigquery.googleapis.com/Dataset,sqladmin.googleapis.com/Instance \
  --format=json \
  | jq -r --arg allowed "$allowed" '($allowed | ascii_downcase | split(",")) as $ok
    | .[] | select((.location // "" | ascii_downcase) as $l | $ok | index($l) | not)
    | "OUTSIDE ALLOWED_LOCATIONS: \(.name) (\(.location))"'
```

**Pass:** Empty output. Requires the ALLOWED_LOCATIONS prerequisite. Each line is a data store outside the allowed locations.

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
c=storage.publicAccessPrevention
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

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
  u=$(gcloud storage buckets describe "gs://$b" --raw --format="value(iamConfiguration.uniformBucketLevelAccess.enabled)")
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
c=storage.uniformBucketLevelAccess
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

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
  l=$(gcloud storage buckets describe "gs://$b" --raw --format="value(lifecycle.rule)")
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
min=SQL_BACKUP_RETENTION
gcloud sql instances list --project="$PROJECT_ID" --format=json \
  | jq -r --argjson min "$min" '.[] | select(.instanceType != "READ_REPLICA_INSTANCE")
    | (.settings.backupConfiguration.backupRetentionSettings.retainedBackups // 0 | tonumber) as $n
    | select($n < $min) | "BACKUP RETENTION BELOW \($min): \(.name) keeps \($n)"'
```

**Pass:** Empty output. Requires the SQL_BACKUP_RETENTION prerequisite. Each line is an instance keeping fewer backups than required.

#### V40

**Log bucket retention set explicitly** · checklist `3.4#5` · scope: org

```bash
min=LOG_RETENTION_DAYS
gcloud logging buckets list --organization=$ORG_ID --location=global --format=json \
  | jq -r --argjson min "$min" '.[] | select((.retentionDays // 30) < $min)
    | "RETENTION BELOW \($min) DAYS: \(.name) (\(.retentionDays // 30))"'
```

**Pass:** Empty output. Requires the LOG_RETENTION_DAYS prerequisite. Each line is a log bucket retained for less than the required period.

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
  gcloud storage buckets describe "gs://$b" --raw \
    --format="value[separator='|'](versioning.enabled,softDeletePolicy.retentionDurationSeconds)" \
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
c=compute.skipDefaultNetworkCreation
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

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
c=compute.restrictVpcPeering
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" '[.spec.rules[]?] as $r
    | if ($r | length) == 0 or ($r | map(.allowAll == true) | any) then "NOT SET: \($c) allows peering with any network" else empty end'
```

**Pass:** Empty output. A line means VPC peering is unrestricted; a decision not to restrict it is recorded as an exception.

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
set -o pipefail
iap=$(gcloud compute backend-services list --project="$PROJECT_ID" --format=json \
  | jq '[.[] | select(.iap.enabled == true)] | length') || exit 1
if [ "$iap" -gt 0 ]; then
  age=$(gcloud iap settings get --resource-type=iap_web --project="$PROJECT_ID" \
    --format="value(accessSettings.reauthSettings.maxAge)") || exit 1
  if [ -z "$age" ]; then echo "IAP IN USE WITHOUT A SESSION MAX AGE: $PROJECT_ID ($iap backend service(s))"; fi
fi
```

**Pass:** Empty output. A line means IAP fronts a backend service but no session max age is set.

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
set -o pipefail
rules=$(gcloud compute firewall-rules list --project="$PROJECT_ID" --format=json) || exit 1
gcloud compute networks list --project="$PROJECT_ID" --format="value(name)" | while read -r net; do
  printf '%s' "$rules" | jq -r --arg net "$net" '
    [.[] | select((.network | split("/") | last) == $net and .direction == "EGRESS" and .disabled != true)] as $eg
    | ($eg[] | select(.allowed != null and ([.destinationRanges[]?] | any(. == "0.0.0.0/0" or . == "::/0")))
        | "EGRESS ALLOWED TO THE INTERNET: \($net)/\(.name)"),
      (if ([$eg[] | select(.denied != null and ([.destinationRanges[]?] | index("0.0.0.0/0")))] | length) == 0
       then "NO EGRESS DENY RULE: \($net) relies on the implied allow-all egress" else empty end)'
done
```

**Pass:** Empty output. A line is an explicit internet egress allow, or a network with no egress deny rule (so the implied allow-all applies). Documented exceptions are recorded against the finding.

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
for c in sql.restrictPublicIp sql.restrictAuthorizedNetworks; do
  gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
    | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
done
```

**Pass:** Empty output. A line names a constraint that is not enforced.

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
gcloud container clusters list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | (.privateClusterConfig.enablePrivateNodes // .networkConfig.defaultEnablePrivateNodes // false) as $priv
    | (.masterAuthorizedNetworksConfig.enabled // .controlPlaneEndpointsConfig.ipEndpointsConfig.authorizedNetworksConfig.enabled // false) as $man
    | select($priv != true or $man != true)
    | "CLUSTER NOT PRIVATE OR AUTHORIZED: \(.name) privateNodes=\($priv) authorizedNetworks=\($man)"'
```

**Pass:** Empty output. Each line is a cluster without private nodes or control-plane authorized networks.

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
c=compute.requireOsLogin
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

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
c=compute.disableSerialPortAccess
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

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
c=compute.requireShieldedVm
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

#### V74

**IP forwarding restricted** · checklist `4.6#10` · scope: org

```bash
c=compute.vmCanIpForward
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.denyAll] | any) then empty else "NOT DENY-ALL: \($c)" end'
```

**Pass:** Empty output. A line means IP forwarding is not denied for all VMs.

#### V75

**GKE hardening settings** · checklist `4.6#11` · scope: project · needs `jq`

```bash
gcloud container clusters list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.legacyAbac.enabled == true or .workloadIdentityConfig.workloadPool == null
      or .nodeConfig.shieldedInstanceConfig.enableSecureBoot != true)
    | "GKE HARDENING GAP: \(.name) ABAC=\(.legacyAbac.enabled // false) WI=\(.workloadIdentityConfig.workloadPool // "OFF") SECURE_BOOT=\(.nodeConfig.shieldedInstanceConfig.enableSecureBoot // false)"'
```

**Pass:** Empty output. Each line is a cluster with legacy ABAC on, Workload Identity off, or Secure Boot off.

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
c=iam.automaticIamGrantsForDefaultServiceAccounts
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

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

**Pass:** Empty output. Each line is a node pool running as the default service account.

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

**Pass:** Evidence: the organization IAM inventory, written to the pack. PASS when the command runs.

#### V87

**Conditional IAM bindings included** · checklist `5.1#2` · scope: org · needs `jq`

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]? | select(.condition != null)
    | "\($r)\t\(.role)\t\(.condition.title)"'
```

**Pass:** Evidence: every conditional IAM binding in the organization. PASS when the command runs.

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
c=iam.disableServiceAccountKeyCreation
gcloud org-policies describe "$c" --organization=$ORG_ID --effective --format=json \
  | jq -r --arg c "$c" 'if ([.spec.rules[]?.enforce] | any) then empty else "NOT ENFORCED: \($c)" end'
```

**Pass:** Empty output. A line means the constraint is not enforced — unset, enforce false, or dry-run only.

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
gcloud secrets list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.rotation.nextRotationTime == null) | "NO ROTATION SCHEDULE: \(.name | split("/") | last)"'
```

**Pass:** Empty output. Each line is a secret with no rotation schedule.

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
set -o pipefail
days=DORMANCY_DAYS
sas=$(gcloud iam service-accounts list --project="$PROJECT_ID" --filter="disabled=false" --format="value(email)") || exit 1
act=$(gcloud policy-intelligence query-activity --activity-type=serviceAccountLastAuthentication \
  --project="$PROJECT_ID" --format=json) || exit 1
if [ -n "$sas" ]; then
  printf '%s\n' "$sas" | while read -r sa; do
    printf '%s' "$act" | jq -r --arg sa "$sa" --argjson days "$days" '(now - $days * 86400) as $cut
      | ([.[] | select(.activity.serviceAccount.fullResourceName | endswith("/" + $sa)) | .activity.lastAuthenticatedTime] | first) as $t
      | if $t == null then "DORMANT (no authentication recorded): \($sa)"
        elif ($t | fromdateiso8601) < $cut then "DORMANT (last authenticated \($t)): \($sa)"
        else empty end'
  done
fi
```

**Pass:** Empty output. Requires the DORMANCY_DAYS prerequisite. Each line is an enabled service account with no authentication within that many days.

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

**Phishing-resistant methods for org and folder admins** · checklist `6.5#3` · scope: org · partial — completes with a manual step

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
set -o pipefail
n=$(gcloud scc notifications list --organization=$ORG_ID --format=json | jq 'length') || exit 1
if [ "$n" -eq 0 ]; then echo "NO SCC NOTIFICATION CONFIG: findings are not routed anywhere"; fi
```

**Pass:** Empty output. A line means no Security Command Center notification config exists.

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
set -o pipefail
vms=$(gcloud compute instances list --project="$PROJECT_ID" --format="value(name)" | wc -l | tr -d ' ') || exit 1
if [ "$vms" -gt 0 ]; then
  n=$(gcloud compute os-config patch-deployments list --project="$PROJECT_ID" --format=json \
    | jq '[.[] | select(.recurringSchedule != null)] | length') \
    || { echo "NO RECURRING PATCH DEPLOYMENT: $PROJECT_ID has $vms instance(s) and OS Config is unavailable"; exit 0; }
  if [ "$n" -eq 0 ]; then echo "NO RECURRING PATCH DEPLOYMENT: $PROJECT_ID has $vms instance(s)"; fi
fi
```

**Pass:** Empty output. A line means the project has instances but no recurring patch deployment.

#### V117

**OS Config agent coverage complete** · checklist `7.3#2` · scope: xref · cross-reference

See V9.

**Pass:** Instance counts match.

#### V118

**Patch compliance reporting reviewed** · checklist `7.3#3` · scope: project

```bash
set -o pipefail
vms=$(gcloud compute instances list --project="$PROJECT_ID" --format="value(name)" | wc -l | tr -d ' ') || exit 1
if [ "$vms" -gt 0 ]; then
  jobs=$(gcloud compute os-config patch-jobs list --project="$PROJECT_ID" --limit=50 --format=json) \
    || { echo "NO PATCH JOBS: $PROJECT_ID has $vms instance(s) and OS Config is unavailable"; exit 0; }
  printf '%s' "$jobs" | jq -r --arg p "$PROJECT_ID" '(now - 30 * 86400) as $cut
    | [.[] | select((.createTime | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601) > $cut)] as $recent
    | if ($recent | length) == 0 then "NO PATCH JOB IN THE LAST 30 DAYS: \($p)"
      else ($recent[] | select(.state != "SUCCEEDED") | "PATCH JOB NOT SUCCEEDED: \(.name | split("/") | last) \(.state)") end'
fi
```

**Pass:** Empty output. A line means instances exist but no patch job ran in 30 days, or a recent job did not succeed.

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
gcloud sql instances list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.instanceType != "READ_REPLICA_INSTANCE" and .settings.maintenanceWindow.day == null)
    | "NO MAINTENANCE WINDOW: \(.name)"'
```

**Pass:** Empty output. Each line is an instance with no maintenance window configured.

**Manual — GCP task, no CLI surface:**

- `7.3#4` Image rebake cadence defined

---

## 7.4 Perform Automated Application Patch Management

#### V122

**Artifact Analysis scanning enabled** · checklist `7.4#1` · scope: project

```bash
set -o pipefail
repos=$(gcloud artifacts repositories list --project="$PROJECT_ID" --format=json \
  | jq '[.[] | select(.format == "DOCKER")] | length') || exit 1
if [ "$repos" -gt 0 ]; then
  on=$(gcloud services list --enabled --project="$PROJECT_ID" \
    --filter="config.name=containerscanning.googleapis.com" --format="value(config.name)") || exit 1
  if [ -z "$on" ]; then echo "CONTAINER SCANNING OFF: $PROJECT_ID has $repos Docker repository(ies)"; fi
fi
```

**Pass:** Empty output. A line means the project holds Docker repositories but Artifact Analysis scanning is off.

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
set -o pipefail
n=$(gcloud logging read 'logName:"cloudaudit.googleapis.com%2Factivity"' \
  --organization=$ORG_ID --limit=5 --freshness=30d --format="value(timestamp)" | wc -l | tr -d ' ') || exit 1
if [ "$n" -eq 0 ]; then echo "NO ADMIN ACTIVITY ENTRIES IN THE LAST 30 DAYS"; fi
```

**Pass:** Empty output. A line means no Admin Activity audit entries were written in 30 days — organization-level changes are infrequent, so a quiet day is not a gap.

#### V127

**Data Access audit logs enabled** · checklist `8.2#2` · scope: org · needs `jq`

```bash
gcloud organizations get-iam-policy $ORG_ID --format=json \
  | jq -r '[.auditConfigs[]? | select(.service == "allServices") | .auditLogConfigs[]?.logType] as $t
    | ["DATA_READ", "DATA_WRITE"][] | select(. as $x | $t | index($x) | not)
    | "DATA ACCESS LOG NOT ENABLED FOR allServices: \(.)"'
```

**Pass:** Empty output. Each line is a Data Access log type not enabled for allServices at the organization. The most common material gap.

#### V128

**Logging gap start date recorded** · checklist `8.2#3` · scope: org

```bash
gcloud logging read 'logName:"cloudaudit.googleapis.com%2Fdata_access"' \
  --organization=$ORG_ID --limit=1 --order=asc --format="value(timestamp)"
```

**Pass:** Evidence: the earliest Data Access log timestamp — the start of the investigable window. PASS when the command runs.

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
gcloud logging sinks list --organization=$ORG_ID --format=json \
  | jq -r 'if ([.[] | select(.includeChildren == true)] | length) == 0 then "NO AGGREGATED ORG SINK: no sink has includeChildren=true" else empty end'
```

**Pass:** Empty output. A line means no organization sink aggregates child-project logs.

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
gcloud dns policies list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.enableLogging != true) | "DNS POLICY LOGGING OFF: \(.name)"'
gcloud compute firewall-rules list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.logConfig.enable != true) | "FIREWALL LOGGING OFF: \(.name)"'
gcloud compute routers list --project="$PROJECT_ID" --format="value(name,region.basename())" \
| while read -r r reg; do
    gcloud compute routers nats list --router="$r" --region="$reg" --project="$PROJECT_ID" --format=json \
      | jq -r --arg r "$r" '.[] | select(.logConfig.enable != true) | "NAT LOGGING OFF: \($r)/\(.name)"'
  done
```

**Pass:** Empty output. Each line is a DNS policy, firewall rule or Cloud NAT with logging off.

#### V134

**GKE and Cloud SQL logs captured** · checklist `8.2#9` · scope: project

```bash
gcloud container clusters list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.loggingService != "logging.googleapis.com/kubernetes")
    | "GKE LOGGING NOT CLOUD LOGGING: \(.name) (\(.loggingService // "none"))"'
```

**Pass:** Empty output. Each line is a cluster not sending logs to Cloud Logging.

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
set -o pipefail
prod=PRODUCTION_PROJECTS
prod_re=${prod//,/|}
dests=$(gcloud logging sinks list --organization=$ORG_ID --format=json \
  | jq -r '.[] | select(.includeChildren == true) | .destination') || exit 1
found=0
while read -r d; do
  case "$d" in
    storage.googleapis.com/*)
      num=$(gcloud storage buckets describe "gs://${d#storage.googleapis.com/}" --raw --format="value(projectNumber)") || continue
      proj=$(gcloud projects list --filter="projectNumber=$num" --format="value(projectId)") || continue ;;
    */projects/*) proj=$(printf '%s' "$d" | sed -E 's|.*projects/([^/]+).*|\1|') ;;
    *) continue ;;
  esac
  found=1
  if printf '%s' "$proj" | grep -Eq -- "$prod_re"; then echo "LOGS ROUTED TO A PRODUCTION PROJECT: $proj ($d)"; fi
done <<< "$dests"
if [ "$found" -eq 0 ]; then echo "NO AGGREGATED SINK TO A LOGGING PROJECT"; fi
```

**Pass:** Empty output. Requires the PRODUCTION_PROJECTS prerequisite. A line means no aggregated sink routes to a project, or one routes to a production project.

#### V139

**Bucket Lock applied to the log destination** · checklist `8.3#4` · scope: org

```bash
set -o pipefail
dests=$(gcloud logging sinks list --organization=$ORG_ID --format="value(destination)" | sort -u) || exit 1
while read -r d; do
  case "$d" in
    storage.googleapis.com/*)
      b="${d#storage.googleapis.com/}"
      locked=$(gcloud storage buckets describe "gs://$b" --raw --format="value(retentionPolicy.isLocked)")
      if [ "$locked" != "True" ]; then echo "NO BUCKET LOCK: gs://$b"; fi ;;
    logging.googleapis.com/*)
      name="${d#logging.googleapis.com/}"
      parent=$(printf '%s' "$name" | cut -d/ -f1-2); loc=$(printf '%s' "$name" | cut -d/ -f4); id=$(printf '%s' "$name" | cut -d/ -f6)
      case "$parent" in
        organizations/*) flag="--organization=${parent#organizations/}" ;;
        folders/*)       flag="--folder=${parent#folders/}" ;;
        projects/*)      flag="--project=${parent#projects/}" ;;
        *) continue ;;
      esac
      locked=$(gcloud logging buckets describe "$id" --location="$loc" "$flag" --format="value(locked)")
      if [ "$locked" != "True" ]; then echo "LOG BUCKET NOT LOCKED: $name"; fi ;;
  esac
done <<< "$dests"
```

**Pass:** Empty output. Each line is a sink destination — Cloud Storage bucket or log bucket — without a lock.

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
set -o pipefail
nets=$(gcloud compute networks list --project="$PROJECT_ID" --format="value(name)" | wc -l | tr -d ' ') || exit 1
if [ "$nets" -gt 0 ]; then
  n=$(gcloud dns response-policies list --project="$PROJECT_ID" --format=json \
    | jq '[.[] | select((.networks // []) | length > 0)] | length') \
    || { echo "NO DNS RESPONSE POLICY: $PROJECT_ID has $nets VPC network(s) and Cloud DNS is unavailable"; exit 0; }
  if [ "$n" -eq 0 ]; then echo "NO DNS RESPONSE POLICY BOUND TO A NETWORK: $PROJECT_ID has $nets VPC network(s)"; fi
fi
```

**Pass:** Empty output. A line means the project has VPC networks but no response policy bound to them.

#### V143

**Egress routed through Cloud NAT or a proxy** · checklist `9.2#2` · scope: project

```bash
set -o pipefail
vms=$(gcloud compute instances list --project="$PROJECT_ID" --format="value(name)" | wc -l | tr -d ' ') || exit 1
if [ "$vms" -gt 0 ]; then
  nats=0
  while read -r r reg; do
    [ -n "$r" ] || continue
    c=$(gcloud compute routers nats list --router="$r" --region="$reg" --project="$PROJECT_ID" --format="value(name)" | wc -l | tr -d ' ')
    nats=$((nats + c))
  done <<< "$(gcloud compute routers list --project="$PROJECT_ID" --format="value(name,region.basename())")"
  if [ "$nats" -eq 0 ]; then echo "NO CLOUD NAT: $PROJECT_ID has $vms instance(s)"; fi
fi
```

**Pass:** Empty output. A line means instances exist with no Cloud NAT for egress. Instances with external IPs are scored separately by V68.

#### V144

**Egress firewall rules constrain destinations** · checklist `9.2#3` · scope: xref · cross-reference

See V58.

**Pass:** Egress not unrestricted allow-all.

#### V145

**Cloud DNS logging enabled** · checklist `9.2#4` · scope: project

```bash
set -o pipefail
nets=$(gcloud compute networks list --project="$PROJECT_ID" --format="value(name)" | wc -l | tr -d ' ') || exit 1
if [ "$nets" -gt 0 ]; then
  n=$(gcloud dns policies list --project="$PROJECT_ID" --format=json \
    | jq '[.[] | select(.enableLogging == true and ((.networks // []) | length > 0))] | length') \
    || { echo "DNS LOGGING OFF: $PROJECT_ID has $nets VPC network(s) and Cloud DNS is unavailable"; exit 0; }
  if [ "$n" -eq 0 ]; then echo "DNS LOGGING OFF: no logging DNS policy on $PROJECT_ID's $nets VPC network(s)"; fi
fi
```

**Pass:** Empty output. A line means the project has VPC networks with no DNS policy that logs queries.

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
v=$(gcloud storage buckets describe gs://TFSTATE_BUCKET --raw --format="value(versioning.enabled)") || exit 1
if [ "$v" != "True" ]; then echo "TERRAFORM STATE VERSIONING OFF: gs://TFSTATE_BUCKET"; fi
```

**Pass:** Empty output. A line means the Terraform state bucket is not versioned — state loss is a recovery event.

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
gcloud sql instances list --project="$PROJECT_ID" --format=json \
  | jq -r '.[] | select(.instanceType != "READ_REPLICA_INSTANCE")
    | .settings.backupConfiguration as $b
    | select($b.enabled != true or (($b.pointInTimeRecoveryEnabled == true) or ($b.binaryLogEnabled == true)) | not)
    | "BACKUP OR PITR OFF: \(.name) backups=\($b.enabled // false) pitr=\(($b.pointInTimeRecoveryEnabled // $b.binaryLogEnabled) // false)"'
```

**Pass:** Empty output. Each line is an instance without automated backups or point-in-time recovery.

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
  v=$(gcloud storage buckets describe "gs://$b" --raw --format="value(versioning.enabled)")
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
if dbs=$(gcloud firestore databases list --project="$PROJECT_ID" --format="value(name.basename())"); then
  for db in $dbs; do
    s=$(gcloud firestore backups schedules list --database="$db" --project="$PROJECT_ID" --format="value(name)")
    if [ -z "$s" ]; then echo "FIRESTORE DATABASE WITHOUT BACKUP SCHEDULE: $db"; fi
  done
fi
if insts=$(gcloud spanner instances list --project="$PROJECT_ID" --format="value(name.basename())"); then
  for i in $insts; do
    for d in $(gcloud spanner databases list --instance="$i" --project="$PROJECT_ID" --format="value(name.basename())"); do
      b=$(gcloud spanner backups list --instance="$i" --project="$PROJECT_ID" --filter="database:$d" --format="value(name)")
      if [ -z "$b" ]; then echo "SPANNER DATABASE WITHOUT BACKUP: $i/$d"; fi
    done
  done
fi
```

**Pass:** Empty output. Each line is a Firestore database with no backup schedule or a Spanner database with no backup.

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
k=$(gcloud storage buckets describe gs://BACKUP_BUCKET --raw --format="value(encryption.defaultKmsKeyName)") || exit 1
if [ -z "$k" ]; then echo "BACKUP BUCKET WITHOUT CMEK: gs://BACKUP_BUCKET"; fi
```

**Pass:** Empty output. A line means the backup bucket has no customer-managed encryption key.

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
id=BACKUP_IDENTITY
gcloud storage buckets get-iam-policy gs://BACKUP_BUCKET --format=json \
  | jq -r --arg id "$id" '.bindings[]?
    | select(.role | test("objectCreator|objectAdmin|objectUser|storage.admin|legacyBucketWriter|legacyBucketOwner|roles/owner|roles/editor"))
    | .role as $r | .members[] | select(sub("^(serviceAccount|user|group):"; "") != $id)
    | "WRITE ACCESS BESIDES THE BACKUP IDENTITY: \(.) (\($r))"'
```

**Pass:** Empty output. Requires the BACKUP_IDENTITY prerequisite. Each line is another principal with write access to the backup bucket.

#### V162

**No production SA holds delete on backups** · checklist `11.3#4` · scope: project · needs `jq`

```bash
id=BACKUP_IDENTITY
gcloud storage buckets get-iam-policy gs://BACKUP_BUCKET --format=json \
  | jq -r --arg id "$id" '.bindings[]?
    | select(.role | test("objectAdmin|objectUser|storage.admin|legacyBucketOwner|roles/owner|roles/editor"))
    | .role as $r | .members[]
    | if sub("^(serviceAccount|user|group):"; "") == $id then "BACKUP IDENTITY CAN DELETE BACKUPS: \(.) (\($r)) — objectCreator is the target"
      else "DELETE ACCESS TO BACKUPS: \(.) (\($r))" end'
```

**Pass:** Empty output. Requires the BACKUP_IDENTITY prerequisite. Each line is a principal able to delete backups, including the backup identity itself.

#### V163

**Bucket Lock applied to backup buckets** · checklist `11.3#5` · scope: project

```bash
l=$(gcloud storage buckets describe gs://BACKUP_BUCKET --raw --format="value(retentionPolicy.isLocked)") || exit 1
if [ "$l" != "True" ]; then echo "NO BUCKET LOCK ON BACKUPS: gs://BACKUP_BUCKET"; fi
```

**Pass:** Empty output. A line means the backup bucket has no locked retention policy.

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
prod=PRODUCTION_PROJECTS
num=$(gcloud storage buckets describe gs://BACKUP_BUCKET --raw --format="value(projectNumber)") || exit 1
proj=$(gcloud projects list --filter="projectNumber=$num" --format="value(projectId)") || exit 1
if printf '%s' "$proj" | grep -Eq -- "${prod//,/|}"; then echo "BACKUP BUCKET IN A PRODUCTION PROJECT: gs://BACKUP_BUCKET is in $proj"; fi
```

**Pass:** Empty output. Requires the PRODUCTION_PROJECTS prerequisite. A line means the backup bucket lives in a production project.

#### V166

**Backup project under a separate folder** · checklist `11.4#2` · scope: project

```bash
set -o pipefail
prod=PRODUCTION_PROJECTS
backup_project=BACKUP_PROJECT
bparent=$(gcloud projects describe "$backup_project" --format="value(parent.type,parent.id)" | tr '\t' ' ') || exit 1
gcloud projects list --format="value(projectId,parent.type,parent.id)" \
| while IFS=$'\t' read -r p type pid; do
    [ "$p" = "$backup_project" ] && continue
    if printf '%s' "$p" | grep -Eq -- "${prod//,/|}" && [ "$type $pid" = "$bparent" ]; then
      echo "BACKUP PROJECT SHARES A PARENT WITH PRODUCTION: $backup_project and $p under $bparent"
    fi
  done
```

**Pass:** Empty output. Requires the PRODUCTION_PROJECTS prerequisite. A line means the backup project sits under the same folder as a production project.

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
regions=PRODUCTION_REGIONS
loc=$(gcloud storage buckets describe gs://BACKUP_BUCKET --raw --format="value(location)") || exit 1
for r in ${regions//,/ }; do
  if [ "$(printf '%s' "$r" | tr '[:upper:]' '[:lower:]')" = "$(printf '%s' "$loc" | tr '[:upper:]' '[:lower:]')" ]; then
    echo "BACKUP COPY IN A PRODUCTION REGION: gs://BACKUP_BUCKET is in $loc"
  fi
done
```

**Pass:** Empty output. Requires the PRODUCTION_REGIONS prerequisite. A line means the backup bucket is in a production region.

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
gcloud compute target-vpn-gateways list --project="$PROJECT_ID" --format="value(name,region.basename())" \
  | sed 's/^/CLASSIC VPN GATEWAY: /'
```

**Pass:** Empty output. Each line is a Classic VPN gateway still to be migrated to HA VPN.

#### V171

**Load balancer SSL policies at a modern TLS minimum** · checklist `12.1#3` · scope: project

```bash
set -o pipefail
pols=$(gcloud compute ssl-policies list --project="$PROJECT_ID" --format=json) || exit 1
gcloud compute target-https-proxies list --project="$PROJECT_ID" --format=json \
  | jq -r --argjson pols "$pols" '($pols | map({key: .selfLink, value: (.minTlsVersion // "TLS_1_0")}) | from_entries) as $m
    | .[] | if .sslPolicy == null then "HTTPS PROXY WITHOUT SSL POLICY: \(.name)"
      elif ($m[.sslPolicy] // "TLS_1_0") == "TLS_1_0" or ($m[.sslPolicy] // "") == "TLS_1_1"
      then "HTTPS PROXY BELOW TLS 1.2: \(.name) (\($m[.sslPolicy]))" else empty end'
```

**Pass:** Empty output. Each line is an HTTPS proxy with no SSL policy (permissive defaults) or a minimum below TLS 1.2.

#### V172

**Legacy load balancers migrated** · checklist `12.1#4` · scope: project

```bash
set -o pipefail
gcloud compute target-pools list --project="$PROJECT_ID" --format="value(name,region.basename())" \
  | sed 's/^/LEGACY TARGET POOL: /'
maps=$(gcloud compute url-maps list --project="$PROJECT_ID" --format=json) || exit 1
gcloud compute target-http-proxies list --project="$PROJECT_ID" --format=json \
  | jq -r --argjson maps "$maps" '($maps | map({key: .selfLink,
        value: (.defaultUrlRedirect.httpsRedirect == true and ((.pathMatchers // []) | length) == 0)}) | from_entries) as $redir
    | .[] | select($redir[.urlMap] != true) | "HTTP PROXY NOT REDIRECTING TO HTTPS: \(.name)"'
```

**Pass:** Empty output. Each line is a legacy target pool, or an HTTP proxy that serves traffic rather than redirecting to HTTPS.

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
gcloud essential-contacts list --organization=$ORG_ID --format=json \
  | jq -r 'if ([.[] | .notificationCategorySubscriptions[]? | select(. == "SECURITY" or . == "ALL")] | length) == 0
    then "NO ESSENTIAL CONTACT FOR SECURITY: Google security notices go nowhere useful" else empty end'
```

**Pass:** Empty output. A line means no organization contact is subscribed to SECURITY.

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
