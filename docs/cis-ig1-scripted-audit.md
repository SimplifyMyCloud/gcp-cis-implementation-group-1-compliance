# Running the scripted audit

Everything the scripts do, start to finish. Copy-pasteable.

This covers only the automated half — 188 of the 290 requirements. The other 102 are manual and live in the [runbook](cis-ig1-audit-runbook.md) (phases 8 and 9) and the [process worksheet](training/09-process-interview.md).

For the full engagement including the manual work, teardown, and reporting, follow the [runbook](cis-ig1-audit-runbook.md) instead. This page is the subset you run at a terminal.

---

## 0. Prerequisites

```bash
go version        # 1.21+
jq --version      # required by ~45 checks
gcloud version
```

`jq` is not optional. Roughly a quarter of the checks pipe through it, and without it they fail rather than degrade.

```bash
export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)
export AUDIT_PROJECT=<project that carries API quota>
echo "org=$ORG_ID  project=$AUDIT_PROJECT"
```

Enable the APIs the checks need, **as yourself** — a brand-new service account cannot enable services:

```bash
mkdir -p ./audit-state
gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-before.txt

gcloud services enable \
  cloudasset.googleapis.com essentialcontacts.googleapis.com \
  accesscontextmanager.googleapis.com recommender.googleapis.com \
  policyanalyzer.googleapis.com osconfig.googleapis.com \
  --project="$AUDIT_PROJECT"

gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-after.txt

comm -13 ./audit-state/apis-before.txt ./audit-state/apis-after.txt \
  | tee ./audit-state/apis-enabled-by-audit.txt
```

That last file is the **only** safe input to teardown — disabling anything else risks turning off an API that was already in use.

---

## 1. Create the audit service account

```bash
cd terraform/audit-service-account

# Terraform uses Application Default Credentials, separate from `gcloud auth login`
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

```bash
terraform init
terraform plan       # review the role list — nothing is created yet
terraform apply
```

Every permission is read-only. [What each role grants](../terraform/audit-service-account/readme.md#exactly-what-is-granted).

---

## 2. Impersonate it

```bash
eval "$(terraform output -raw impersonate_command)"
gcloud config get-value auth/impersonate_service_account
```

That must print `cis-ig1-auditor@…`.

> **`gcloud auth list` will still show your own address, and that is correct.**
>
> Impersonation does not switch accounts. You stay authenticated as yourself, and gcloud exchanges that credential for a short-lived service account token on every call. `auth list` reports the authenticated account, so it never changes — which is precisely why audit logs can record both identities in the delegation chain.
>
> The config value above is the check that matters.

Every `gcloud` command from here runs with the service account's read-only permissions. Your own access is not additive to it — if the service account cannot read something, the command fails regardless of what you personally hold.

**IAM propagation takes a minute or two.** If the next step denies immediately after `terraform apply`, wait and retry before assuming a grant failed.

---

## 3. Smoke-test the permissions

Six checks, one per permission family. Much cheaper to fail here than 188 checks later.

```bash
cd ../..
go run audit-run.go -only V43,V27,V86,V91,V125,V181 -org="$ORG_ID"
```

| Check | Proves |
|---|---|
| `V43` | Organization Policy read |
| `V27` | Cloud Asset Inventory search |
| `V86` | IAM policy search at scale |
| `V91` | Service account **key** listing — a separate permission from listing accounts, via a custom role |
| `V125` | Logging read |
| `V181` | Essential Contacts read |

Any `DENIED` is a missing grant on the service account. **Fix and rerun before going further** — on some checks a permission gap converts silently into a false `PASS`, which is worse than an error.

---

## 4. Supply the two values that cannot be discovered

Checks discover their own resources — every project, every Cloud SQL instance, every bucket, every KMS key. Two values depend on your naming rather than anything queryable:

```bash
go run audit-run.go -init-config ./audit-state/audit.env
```

| Value | What it is |
|---|---|
| `BACKUP_BUCKET` | The bucket holding backups (6 checks) |
| `TFSTATE_BUCKET` | The bucket holding Terraform state (1 check) |

To find them:

```bash
gcloud projects list --format="value(projectId)" | while read -r p; do
  gcloud storage buckets list --project="$p" --format="value(name)" 2>/dev/null | sed "s|^|$p / |"
done
```

**If either does not exist, write `none`, not blank.**

Blank produces `SKIP`, which reads as "we could not check". `none` produces `FAIL`, which is the truth — an organization with no backup bucket has not skipped safeguard 11.3, it has failed it. Blank values quietly turn non-compliance into missing data.

---

## 5. Organization pass

Always first. Its findings explain the project results that follow — a missing org policy constraint is *why* fifty projects each have a default network.

```bash
go run audit-run.go -scope=org -org="$ORG_ID" \
  -config ./audit-state/audit.env \
  -pack ./audit-state/org \
  2>&1 | tee ./audit-state/org-run.log
```

**67 checks.** Results stream as they land:

```
  [  1/ 67] V43    REVIEW  4.1#2   Baseline enforced through org policy constraints
  [  2/ 67] V27    PASS    3.3#1   No publicly accessible Cloud Storage buckets
  [  3/ 67] V127   FAIL    8.2#2   Data Access audit logs enabled
```

Read `./audit-state/org/01-automated-results.md` before starting the project passes. An organization-level `DENIED` makes the project results unreliable too.

---

## 6. Project passes

One project per invocation, and every resource of the relevant kind inside it — every Cloud SQL instance, every node pool, every bucket. A requirement is met only when *every* resource meets it; one non-compliant instance out of ten fails the check, and the output names which one.

```bash
mkdir -p ./audit-state/projects
gcloud projects list --format="value(projectId)" | sort > ./audit-state/projects.txt
cat ./audit-state/projects.txt
```

Pick one and run it:

```bash
export PROJECT=<paste-a-projectId>

gcloud projects describe "$PROJECT" --format="value(projectId,lifecycleState)"

go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
  -config ./audit-state/audit.env \
  -pack "./audit-state/projects/$PROJECT" \
  2>&1 | tee "./audit-state/projects/$PROJECT.log"
```

**91 checks per project.** Repeat for each entry in `projects.txt`.

Once you trust the output, work the whole list:

```bash
while read -r PROJECT; do
  echo "=== $PROJECT"
  go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
    -config ./audit-state/audit.env \
    -pack "./audit-state/projects/$PROJECT" \
    -quiet 2>&1 | tail -12
done < ./audit-state/projects.txt
```

What is still outstanding:

```bash
comm -23 ./audit-state/projects.txt \
  <(ls ./audit-state/projects/ | grep -v '\.log$' | sort)
```

---

## 7. What comes out

Each pack is three files:

| File | Contents | Worked by |
|---|---|---|
| `01-automated-results.md` | Every check as one table, with findings and their output | You, reading |
| `02-manual-cli.md` | 30 GCP requirements with no CLI check | You, at a console |
| `03-manual-process.md` | 72 process requirements | Whoever owns the process — still yours, just written rather than configured |

Add `-review review.md` for the full untruncated output of every `REVIEW` check with its pass criteria and a tick-box.

### Verdicts

| | Meaning |
|---|---|
| `PASS` | Compliant |
| `FAIL` | A finding — the output names the offending resources |
| `REVIEW` | Ran clean; a human judges the result |
| `SKIP` | A value was not supplied — should be zero on a prepared run |
| `N/A` | API or product absent — not a failure |
| `DENIED` | Missing permission — fix before trusting anything |
| `ERROR` | The command failed or timed out |

Only **41 of 188** are auto-scored. The rest are `REVIEW` with their output saved. That ratio is deliberate — no machine can tell you whether your org policy list matches your intended baseline, and a confident wrong verdict is worse in an audit than an honest "you decide".

### Useful flags

```bash
-list                    # every check and how it is classified; runs nothing
-only V27,V91            # a subset
-parallel 4              # fewer concurrent calls, if you hit rate limits
-timeout 10m             # longer, if slow per-project loops get cut short
-quiet                   # suppress the live stream
-format=md -out f.md     # markdown to a file
```

---

## 8. Stop impersonating

When the scripted passes are finished:

```bash
gcloud config unset auth/impersonate_service_account
gcloud config get-value auth/impersonate_service_account   # should be empty
```

**Do this before any teardown.** The service account cannot revoke its own bindings or delete itself, so `terraform destroy` fails partway if you are still impersonating.

Teardown itself is [runbook phase 11](cis-ig1-audit-runbook.md#phase-11--tear-down-the-audit-access) — do not skip it. The audit identity holds organization-wide read, and leaving it in place fails safeguards 5.1, 5.4 and 6.2, which are among the controls you just measured.

---

## Next

- Triage the results — [runbook phase 7](cis-ig1-audit-runbook.md#phase-7--triage-in-this-order)
- Look up how to fix a finding — [remediation reference](cis-ig1-remediation-reference.md)
- Work the 102 manual requirements — [runbook phase 9](cis-ig1-audit-runbook.md#phase-9--the-102-manual-requirements)
- Score it — `go run compliance-report.go`
