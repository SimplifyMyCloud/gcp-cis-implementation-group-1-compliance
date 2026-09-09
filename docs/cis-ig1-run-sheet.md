# Run sheet

Flat command list. Paste each block in order. Context and reasoning live in the [runbook](cis-ig1-audit-runbook.md) — this page is just the commands.

**Part A** runs once for the organization. **Part B** runs once per project.

---

# Part A — Organization

## A1. Set variables

```bash
export ORG_ID="REPLACE_ORG_ID"
export AUDIT_PROJECT="REPLACE_AUDIT_PROJECT"
export SA_EMAIL="cis-ig1-auditor@${AUDIT_PROJECT}.iam.gserviceaccount.com"

mkdir -p ./audit-state/projects
echo "org=$ORG_ID  project=$AUDIT_PROJECT"
```

## A2. Check tools

```bash
go version && jq --version && gcloud version | head -1
```

## A3. Snapshot enabled APIs, then enable

```bash
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

## A4. Create the audit service account

```bash
cd gcloud
./create.sh --org-id "$ORG_ID" --project "$AUDIT_PROJECT" \
  --auditor "user:$(gcloud config get-value account)"
cd ..
```

## A5. Impersonate

```bash
gcloud config set auth/impersonate_service_account "$SA_EMAIL"
gcloud config get-value auth/impersonate_service_account
```

Must print the service account. `gcloud auth list` will still show *your* address — that is correct.

## A6. Prove it is read-only

```bash
gcloud iam service-accounts create throwaway-check --project="$AUDIT_PROJECT"
```

`PERMISSION_DENIED` is the **correct** result. If it succeeds, impersonation is not active — delete the account and redo A5.

## A7. Smoke test

```bash
go run audit-run.go -only V43,V27,V86,V91,V125,V181 -org="$ORG_ID"
```

Any `DENIED` — stop, fix the grant, re-run. Do not continue.

## A8. Starting position

```bash
gcloud resource-manager org-policies list --organization="$ORG_ID"
```

Empty means permissive-default. Record it as Step 0 in the checklist.

## A9. Config values

```bash
go run audit-run.go -init-config ./audit-state/audit.env
```

Find the buckets:

```bash
gcloud projects list --format="value(projectId)" | while read -r p; do
  gcloud storage buckets list --project="$p" --format="value(name)" 2>/dev/null | sed "s|^|$p / |"
done
```

Edit `./audit-state/audit.env`. **If a bucket does not exist write `none`, not blank** — blank gives SKIP, `none` gives FAIL, which is the truth.

## A10. Organization pass

```bash
go run audit-run.go -scope=org -org="$ORG_ID" \
  -config ./audit-state/audit.env \
  -pack ./audit-state/org \
  2>&1 | tee ./audit-state/org-run.log
```

## A11. Read the run status

```bash
head -20 ./audit-state/org/01-automated-results.md
```

`UNRELIABLE` or `DEGRADED` means fix and re-run before doing any projects.

## A12. Project list

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
  -config ./audit-state/audit.env \
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
    -config ./audit-state/audit.env \
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
go run rollup.go -in ./audit-state \
  -out ./audit-state/remediation-plan.md \
  -csv ./audit-state/remediation-plan.csv

head -40 ./audit-state/remediation-plan.md
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

Never the full A3 list — some were already on and in use.

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

- [ ] A1 variables set
- [ ] A2 go, jq, gcloud present
- [ ] A3 `apis-enabled-by-audit.txt` written
- [ ] A4 service account created
- [ ] A5 impersonation active
- [ ] A6 write attempt denied
- [ ] A7 smoke test, no DENIED
- [ ] A8 starting position recorded
- [ ] A9 `audit.env` filled, non-existent buckets set to `none`
- [ ] A10 org pass complete
- [ ] A11 run status OK
- [ ] A12 project list captured

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
