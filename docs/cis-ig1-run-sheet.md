# Run sheet

Flat command list. Paste each block in order. Context and reasoning live in the [runbook](cis-ig1-audit-runbook.md) — this page is just the commands.

**Part A** runs once for the organization. **Part B** runs once per project.

---

# Part A — Organization

## A1. Set up your shell

Follow [auditor setup](cis-ig1-auditor-setup.md) **steps 1–4**: open the shell and check tools, sign in, clone the repository onto your branch, set `ORG_ID`, `AUDIT_PROJECT` and `SA_EMAIL`.

With the [Cloud Shell setup](cis-ig1-auditor-setup.md#part-2--cloud-shell-one-time-setup) done and the service account already in place, `audit-on` replaces the sign-in, variables and impersonation in A1 and A4. You still need setup step 7 on the first run of an engagement, to create `config/audit.env` and `audit-state/`.

## A2. Snapshot enabled APIs, then enable

Once per engagement, as yourself.

```bash
mkdir -p ./audit-state
gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-before.txt

gcloud services enable \
  accesscontextmanager.googleapis.com bigquery.googleapis.com \
  cloudasset.googleapis.com cloudbilling.googleapis.com \
  cloudresourcemanager.googleapis.com essentialcontacts.googleapis.com \
  iam.googleapis.com iamcredentials.googleapis.com \
  logging.googleapis.com monitoring.googleapis.com \
  orgpolicy.googleapis.com policyanalyzer.googleapis.com \
  recommender.googleapis.com securitycenter.googleapis.com \
  serviceusage.googleapis.com spanner.googleapis.com \
  sqladmin.googleapis.com \
  --project="$AUDIT_PROJECT"
```

These are the APIs the audit's calls are **billed to the host project** for — measured from request metrics in a live run ([`docs/cis-ig1-required-apis.md`](cis-ig1-required-apis.md)). Product APIs such as Compute, GKE, DNS and OS Config bill to the *audited* project instead; enabling them here does nothing, and where they are off in an audited project the check is correctly N/A.

```bash
gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-after.txt

comm -13 ./audit-state/apis-before.txt ./audit-state/apis-after.txt \
  | tee ./audit-state/apis-enabled-by-audit.txt
```

## A3. Create the audit service account

Once per engagement, as yourself — impersonation is not on yet. Add `--dry-run` first to see every grant without changing anything. Terraform does the same job: [`terraform/`](../terraform/readme.md).

```bash
cd gcloud
./create.sh --org-id "$ORG_ID" --project "$AUDIT_PROJECT" \
  --auditor "user:$(gcloud config get-value account)"
cd ..
```

## A4. Impersonate, prove it, create the output directory and config

Follow [auditor setup](cis-ig1-auditor-setup.md) **steps 5–7**. At the end of them:

- the token check prints `cis-ig1-auditor@…`, not your own address
- `./audit-state/runs` and `./audit-state/projects` exist
- `config/audit.env` has `APPROVED_REGISTRIES` set

A `Failed to impersonate` straight after A3 means the new grant has not propagated. Wait a minute and retry.

## A5. Smoke test

```bash
go run audit-run.go -scope=org -org="$ORG_ID" -only V27,V43,V86,V125,V181 -no-prompt
```

Five organization checks, one per permission family. Any `DENIED` or `ERROR` — stop, fix, re-run. Do not continue. (V86 writes `./audit-state/iam-inventory.txt`.)

## A6. Starting position

```bash
gcloud org-policies list --organization="$ORG_ID"
```

Empty means permissive-default. Record it as Step 0 in the checklist.

## A7. Shortcut — the whole audit in one command

Everything from A8 to C2 in one step, filed into a dated run directory under `audit-state/runs/`. Skip to C3 afterwards.

```bash
gcloud projects list --format="value(projectId)" | sort > ./audit-state/projects.txt   # edit to taste
./run-audit.sh \
  --projects ./audit-state/projects.txt
```

```
audit-state/runs/2026-09-14_12-58-20/
  report/                        what the auditor reads, in reading order
    01-remediation-plan.md
    02-organization/
      01-automated-results.md
      02-manual-gcp-tasks.md
      03-manual-process.md
    03-projects/
      <project-id>.md            one per project
    04-compliance-score.md
    04-compliance-score.json
    remediation-plan.csv         import into the tracker
  evidence/
    results/                     each pass's results and review decisions
    audit.env  targets.txt  run.log  iam-inventory.txt  excluded.txt  not-found.txt
```

Then decide every REVIEW check PASS or FAIL. Completion in each report reaches 100% when the last is decided:

```bash
./run-audit.sh --review ./audit-state/runs/<timestamp>
```

`--project ID` (repeatable) or `--all` instead of `--projects`. A check hanging? `--skip V96` leaves it out (reported as SKIP); `--parallel 1` runs checks one at a time so the stuck one is obvious. Every check also has a 3-minute timeout (`-timeout` on `audit-run.go`). Projects that don't exist or can't be seen are listed as `not found` and skipped. The summary at the end shows every pass's `RUN STATUS`.

## A8. Organization pass

```bash
go run audit-run.go -scope=org -org="$ORG_ID" \
  -pack ./audit-state/org \
  2>&1 | tee ./audit-state/org-run.log
```

The command exits non-zero whenever any check FAILs — that is findings, not a broken run. The run's health is the status line below.

## A9. Read the run status

```bash
head -20 ./audit-state/org/01-automated-results.md
```

`UNRELIABLE` or `DEGRADED` means fix and re-run before doing any projects.

## A10. Project list

```bash
gcloud projects list --format="value(projectId)" | sort > ./audit-state/projects.txt
wc -l < ./audit-state/projects.txt
cat ./audit-state/projects.txt
```

---

# Part B — Per project

Repeat B1–B3 for each project in `projects.txt`.

## B1. Set the project

```bash
export PROJECT="REPLACE_PROJECT_ID"
gcloud projects describe "$PROJECT" --format="value(projectId,lifecycleState)"
```

`NOT_FOUND` means a wrong ID, or the audit identity cannot see it.

## B2. Run

```bash
go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
  -pack "./audit-state/projects/$PROJECT" \
  2>&1 | tee "./audit-state/projects/${PROJECT}.log"
```

## B3. Check it was a good run

```bash
grep -m1 "RUN STATUS" "./audit-state/projects/$PROJECT/01-automated-results.md"
```

Anything other than `OK` — re-run before moving on.

## B4. Progress

```bash
comm -23 ./audit-state/projects.txt \
  <(ls ./audit-state/projects/ | grep -v '\.log$' | sort)
```

Lists what is still outstanding.

## B5. Unattended, once you trust the output

```bash
while read -r PROJECT; do
  echo "=== $PROJECT"
  go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
      -pack "./audit-state/projects/$PROJECT" \
    -quiet 2>&1 | tail -12
done < ./audit-state/projects.txt
```

Then find any bad runs:

```bash
grep -L "RUN STATUS: OK" ./audit-state/projects/*/01-automated-results.md
```

---

# Part C — Compile and tear down

## C1. Remediation plan

```bash
go run rollup.go -in ./audit-state/runs \
  -out       ./audit-state/remediation-plan.md \
  -csv       ./audit-state/remediation-plan.csv \
  -score-md  ./audit-state/compliance-score.md \
  -score-json ./audit-state/compliance-score.json \
  -projects  ./config/projects.txt

head -40 ./audit-state/remediation-plan.md
grep -m1 SCORE ./audit-state/compliance-score.md
```

Import the CSV into the tracker: **File → Import → Upload → Insert new sheet**.

## C2. Score

```bash
go run compliance-report.go
```

## C3. Stop impersonating

```bash
gcloud config unset auth/impersonate_service_account
gcloud config get-value account
```

**Before teardown.** The audit identity cannot delete itself.

Used the [Cloud Shell setup](cis-ig1-auditor-setup.md#part-2--cloud-shell-one-time-setup)? Impersonation lives in the `cis-audit` configuration instead: run `audit-off`, or open a new tab, then [remove the setup](cis-ig1-auditor-setup.md#removing-it-at-the-end-of-the-engagement) at the end of the engagement.

## C4. Destroy the audit identity

```bash
cd gcloud
./destroy.sh --record ./audit-sa-record.txt
./verify.sh --record ./audit-sa-record.txt
cd ..
```

## C5. Disable only the APIs this audit enabled

```bash
while read -r API; do
  gcloud services disable "$API" --project="$AUDIT_PROJECT" --force --quiet
done < ./audit-state/apis-enabled-by-audit.txt
```

Never the full A2 list — some were already on and in use.

## C6. Confirm nothing remains

```bash
gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten="bindings[].members" \
  --filter="bindings.members:${SA_EMAIL}" \
  --format="value(bindings.role)"

gcloud iam roles list --organization="$ORG_ID" --filter="name~cisIg1Audit"

gcloud iam service-accounts describe "$SA_EMAIL" --project="$AUDIT_PROJECT" 2>&1 | tail -1
```

Expected: empty, empty, `NOT_FOUND`.

---

## Checklist

**Organization**

- [ ] A1 shell set up, on your branch, variables set (setup steps 1–4)
- [ ] A2 `apis-enabled-by-audit.txt` written
- [ ] A3 service account created
- [ ] A4 token check prints the auditor; `config/audit.env` and `audit-state/` in place (setup steps 5–7)
- [ ] A5 smoke test, no DENIED
- [ ] A6 starting position recorded
- [ ] A8 org pass complete (or A7 in place of A8 to C2)
- [ ] A9 run status OK
- [ ] A10 project list captured

**Projects**

- [ ] Every project in `projects.txt` has a pack
- [ ] No pack reports anything other than `RUN STATUS: OK`

**Close out**

- [ ] C1 remediation plan produced, CSV imported
- [ ] C2 score recorded
- [ ] C3 impersonation unset
- [ ] C4 identity destroyed and verified
- [ ] C5 only audit-enabled APIs disabled
- [ ] C6 all three checks return empty / NOT_FOUND
