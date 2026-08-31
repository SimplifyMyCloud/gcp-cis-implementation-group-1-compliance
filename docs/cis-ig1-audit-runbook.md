# Audit Runbook

How to validate a GCP Organization against CIS IG1 using the documents and scripts in this repository.

## What gets automated, and what does not

| | Category | Count | Who |
|---|---|---|---|
| **1** | CLI one-liner, unambiguous pass/fail | 39 | `audit-run.go` scores it |
| **2** | CLI-verifiable, output needs judgement | 149 | `audit-run.go` runs it and saves the output; marked **REVIEW** for a human |
| **3** | A GCP task with no CLI surface — Admin Console, image build, a test that must be performed | 30 | Human, step by step |
| **4** | Process, policy or documentation | 72 | Human, evidence-based |

100% automation was never the goal. The machine scores only what it can defend and hands everything else to a person with the output already gathered.

## Two passes, in this order

1. **Organization** — 67 checks, run once. The posture every project inherits.
2. **Project** — 91 checks, run **once per project**, targeted explicitly.

The organization pass goes first because its findings explain the project results. Within a project pass, every resource of the relevant kind is checked, and one non-compliant resource fails the requirement.

## This audit reads; it does not remediate

Every one of the 188 checks is read-only. The audit measures the current state of the organization and records what does not satisfy IG1 — it does not change anything to make a check pass. A requirement that is not met is a **FAIL for a human to follow up**, not something to fix mid-run.

Three exceptions, all prerequisites rather than remediation, and all reversed in [Phase 11](#phase-11--tear-down-the-audit-access):

- A **service account** (`cis-auditor`) is created to run the audit
- **APIs** are enabled on the audit project so the checks can run
- **IAM roles** are granted to that service account so it can read

The audit runs as that service account via impersonation — **no key is ever created**, because a service account key would violate safeguard 5.2, which this audit tests. Your own user account is never granted anything, so nothing you legitimately hold is at risk when the teardown revokes everything.

Both are changes to the organization. Both are recorded in `./audit-state/` as they are made, and both are torn down when the audit finishes — leaving a standing 22-role audit identity in place would itself fail safeguards 5.1, 5.4 and 6.2.

**Each phase gates the next.** A permissions problem discovered at Phase 5 invalidates everything above it, so do not skip ahead.

---

## Phase 1 — Prerequisites

Everything this audit records, changes, or produces lives in one directory:

```
audit-state/
  audit.env                     values the checks need
  apis-before.txt               enabled APIs before we touched anything
  apis-enabled-by-audit.txt     the diff — the only safe teardown list
  roles-granted.txt             roles given to the audit identity
  audit-member.txt              which identity ran it
  projects.txt                  every project in the org
  iam-inventory.txt             full IAM inventory (V86)
  org/                          the organization pass results
  projects/<project-id>/        one directory per project pass
```

```bash
mkdir -p ./audit-state

gcloud auth login
export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)
export AUDIT_PROJECT=<project that will carry API quota>
gcloud config set project $AUDIT_PROJECT

which jq kubectl bq     # jq is required by ~45 checks and is not optional
```

Enable the APIs deliberately. Each is a billable-service change on the organization, and the alternative is approving them at an interactive `y/N` prompt in the middle of the run:

**Snapshot what is already enabled first.** At teardown you must disable only the APIs *you* turned on — disabling one that was already in use will break whatever depends on it.

```bash
mkdir -p ./audit-state
gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-before.txt

gcloud services enable \
  cloudasset.googleapis.com \
  essentialcontacts.googleapis.com \
  accesscontextmanager.googleapis.com \
  recommender.googleapis.com \
  policyanalyzer.googleapis.com \
  osconfig.googleapis.com \
  --project=$AUDIT_PROJECT

gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-after.txt

# Exactly what this audit turned on — the teardown list
comm -13 ./audit-state/apis-before.txt ./audit-state/apis-after.txt \
  | tee ./audit-state/apis-enabled-by-audit.txt
```

Add `securitycenter.googleapis.com` only where SCC is licensed.

### Create the audit service account

**The audit runs as a dedicated service account you impersonate, not as your own user.** Three reasons, and the first is the one that bites:

**Revoking is clean.** Your user account probably already holds some of these roles, or inherits them from a group. Revoking unconditionally at teardown would strip bindings you had before the audit and legitimately need. A service account starts from nothing, so revoke-everything is exactly right.

**Attribution.** Every check writes audit log entries. Run as your user and they are indistinguishable from your normal admin activity; run as the auditor and the entire audit is one filterable block in the log.

**It is the honest answer to safeguard 5.4.** A human account holding standing organization-wide read is precisely the pattern this audit flags.

#### Use the Terraform module

```bash
# Application Default Credentials — separate from `gcloud auth login`
gcloud auth application-default login

cd terraform/audit-service-account
cp terraform.tfvars.example terraform.tfvars    # then edit
terraform init
terraform plan                                  # review with the customer
terraform apply
```

`terraform plan` is worth showing the customer before you apply — it is an exact, reviewable statement of what the audit will be able to read.

Every permission is read-only. Where the only predefined role carried a write verb, the module substitutes a custom role with an explicit permission list — including replacing `roles/storage.admin`, which can delete buckets, with a three-permission reader. See the [module readme](../terraform/audit-service-account/readme.md).

Then switch to the audit identity:

```bash
eval "$(terraform output -raw impersonate_command)"
gcloud config get-value auth/impersonate_service_account
```

That must print the service account. Note `gcloud auth list` will still show *your* address — impersonation layers on top of your credential rather than replacing it, which is why audit logs record both identities.

> **Enable the APIs before switching**, or as your own user — a brand-new service account has no rights yet, including the right to enable services. To step back briefly:
> ```bash
> gcloud config unset auth/impersonate_service_account
> ```

Impersonation propagates within a minute or so. If Phase 3 reports `PERMISSION_DENIED` immediately, wait and retry before assuming a grant failed.

### Record what was granted, for teardown

```bash
mkdir -p ./audit-state
printf '%s\n' browser orgpolicy.policyViewer cloudasset.viewer iam.securityReviewer \
  iam.serviceAccountViewer logging.viewer logging.privateLogViewer \
  essentialcontacts.viewer compute.viewer container.viewer cloudsql.viewer \
  storage.admin monitoring.viewer osconfig.inventoryViewer recommender.iamViewer \
  secretmanager.viewer artifactregistry.reader binaryauthorization.policyViewer \
  dns.reader cloudkms.viewer accesscontextmanager.policyReader \
  securitycenter.adminViewer > ./audit-state/roles-granted.txt

echo "$AUDIT_MEMBER" > ./audit-state/audit-member.txt
```

### Confirm the grants landed

```bash
gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten="bindings[].members" \
  --filter="bindings.members:${AUDIT_MEMBER#*:}" \
  --format="value(bindings.role)" | sort
```

Expect 22 or 23 roles. IAM propagation can take a minute or two — if Phase 3 reports `DENIED`, wait and rerun before assuming a grant failed.

> **The audit identity is itself a sensitive asset.** These roles let one principal enumerate every public bucket, over-permissioned account, and open firewall rule in the organization. That is why it is impersonated rather than keyed, and deleted when the audit ends.
>
> These bindings are torn down in [Phase 11](#phase-11--tear-down-the-audit-access). Do not skip it.

- [ ] `ORG_ID` and `AUDIT_PROJECT` exported
- [ ] `jq` present
- [ ] `./audit-state/apis-enabled-by-audit.txt` written
- [ ] APIs enabled
- [ ] `cis-auditor` service account created
- [ ] Impersonation active — `gcloud auth list` shows the service account
- [ ] No service account key created (impersonation only)
- [ ] Audit roles granted at **organization** scope
- [ ] Custom key-reader role created and bound
- [ ] Grants confirmed with `get-iam-policy`

---

## Phase 2 — Establish the starting position

```bash
gcloud resource-manager org-policies list --organization=$ORG_ID
```

Empty or near-empty output means a **permissive-default** organization. Constraints you did not apply came from Google's security baseline, meaning **secure-by-default**.

Record it as Step 0 in the checklist. It determines how much of Controls 3, 4, 5, 15 and 17 you are starting from — see [the overview](cis-ig1-overview.md#permissive-default-vs-secure-by-default-organizations).

- [ ] Starting position recorded in the checklist

---

## Phase 3 — Smoke-test permissions

Six checks, one per permission family. Cheaper to fail here than 188 checks later.

Confirm impersonation is active:

```bash
gcloud config get-value auth/impersonate_service_account
```

That must print the auditor service account. If it prints nothing, impersonation is not set and the smoke test proves nothing about the identity that will do the audit.

```bash
go run audit-run.go -only V43,V27,V86,V91,V125,V181 -org="$ORG_ID"
```

Any `DENIED` is a missing grant on the **service account**. **Fix it and rerun before continuing** — on some checks a permission gap converts silently into a false `PASS`.

- [ ] All six return something other than `DENIED`

---

## Phase 4 — Gather the two values that cannot be discovered

Checks discover their own resources. Where a safeguard concerns projects, Cloud SQL instances, GKE clusters, buckets, KMS keys or Cloud Routers, the command enumerates **every** one — because a requirement is met only when every resource meets it. One non-compliant instance out of ten fails the check, and the output names which one.

Two values remain, because they depend on your naming rather than on anything queryable:

```bash
go run audit-run.go -init-config ./audit-state/audit.env
```

| Value | What it is |
|---|---|
| `BACKUP_BUCKET` | The bucket holding backups (6 checks) |
| `TFSTATE_BUCKET` | The bucket holding Terraform state (1 check) |

**If either does not exist, write `none`, not blank.**

Blank produces `SKIP`, which reads as "we could not check." `none` produces `FAIL`, which is the truth: an organization with no backup bucket has not skipped safeguard 11.3, it has failed it. Blank values quietly turn non-compliance into missing data, and that is how a finding disappears from a report.

To find them:

```bash
gcloud projects list --format="value(projectId)" | while read -r p; do
  gcloud storage buckets list --project="$p" --format="value(name)" 2>/dev/null | sed "s|^|$p / |"
done
```

- [ ] `./audit-state/audit.env` filled in
- [ ] Anything non-existent recorded as `none`, not left blank

---

## Phase 5 — Run the organization pass

**Always first.** The organization pass establishes the posture every project inherits, and several of its findings explain the project results that follow — a missing org policy constraint is *why* fifty projects each have a default network.

```bash
go run audit-run.go -scope=org -org="$ORG_ID" \
  -config ./audit-state/audit.env \
  -pack ./audit-state/org \
  2>&1 | tee ./audit-state/org/run.log
```

67 checks. Read `01-automated-results.md` before starting the project passes — an org-level `DENIED` means the project passes will be unreliable too.

- [ ] Organization pack produced
- [ ] No `DENIED` remaining

---

## Phase 6 — Run one pass per project

The project pass targets **one project per invocation**. Within it, every resource of the relevant kind is enumerated — every Cloud SQL instance, every node pool, every KMS key, every bucket. A requirement is met only when every resource meets it; one non-compliant instance out of ten fails the check and the output names it.

### List the projects

```bash
mkdir -p ./audit-state/projects
gcloud projects list --format="value(projectId)" | sort > ./audit-state/projects.txt

echo "$(wc -l < ./audit-state/projects.txt) project(s):"
cat ./audit-state/projects.txt
```

### Set the project for this run

Pick one from that list and export it. Everything below reads `$PROJECT`, so this is the only place you type a project name:

```bash
export PROJECT=<paste-a-projectId-from-the-list-above>
```

Confirm it resolves before running 91 checks against a typo:

```bash
gcloud projects describe "$PROJECT" --format="value(projectId,name,lifecycleState)"
```

A `NOT_FOUND` here means the ID is wrong, or your audit identity cannot see that project — either way, fix it before continuing.

### Run the pass

```bash
go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
  -config ./audit-state/audit.env \
  -pack "./audit-state/projects/$PROJECT" \
  2>&1 | tee "./audit-state/projects/$PROJECT.log"
```

91 checks. Repeat from **Set the project** for each entry in `projects.txt`.

To see which projects you have already covered:

```bash
ls ./audit-state/projects/
comm -23 ./audit-state/projects.txt <(ls ./audit-state/projects/ | grep -v '\.log$' | sort)
```

The second command lists what is still outstanding.

To work the whole list unattended once you trust the output:

```bash
mkdir -p ./audit-state/projects
while read -r PROJECT; do
  echo "=== $PROJECT"
  go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
    -config ./audit-state/audit.env \
    -pack "./audit-state/projects/$PROJECT" \
    -quiet 2>&1 | tail -12
done < ./audit-state/projects.txt
```

Each pack is self-identifying — `01-automated-results.md` names the project it covers.

- [ ] `projects.txt` written
- [ ] A pack produced for every project
- [ ] Projects with `DENIED` or `ERROR` noted for a second look

---

## Phase 7 — Triage, in this order

> ### The audit will find the auditor
>
> `cis-auditor` holds 22 organization-level roles while the audit is running, so it appears in the results — typically in **V86** (IAM principal inventory), **V88** (conditional bindings), and the administrator-privilege checks under **5.4**.
>
> These are artifacts of the audit being in progress, not findings. The exception is deliberate and time-boxed: the auditor needs the vault open to count the bars. Note them as such, confirm they are gone after [Phase 11](#phase-11--tear-down-the-audit-access), and do not chase them.
>
> If `cis-auditor` still appears in a re-run *after* teardown, that is a genuine finding — the teardown did not complete.

| Verdict | Action |
|---|---|
| `DENIED` | Permission gap. Fix, rerun. **Everything else is suspect until this is clean.** |
| `ERROR` | Command failed. Capture the text — likely a doc bug. |
| `N/A` | Product or API absent. Mark not applicable; do not chase. |
| `SKIP` | Placeholder missing. Supply it, or handle the requirement manually. |
| `FAIL` | A finding. Output names the offending resources. |
| `REVIEW` | ~73 of these. Read the output against the stated pass criteria and decide. |
| `PASS` | Tick it. |

`N/A` and `DENIED` are separated deliberately: "product absent" and "missing permission" look identical in raw output, and chasing a nonexistent SCC finding in front of a customer is time you do not get back.

- [ ] No `DENIED` remaining
- [ ] All `ERROR` entries captured
- [ ] Every `REVIEW` adjudicated

---

## Phase 8 — Tick the checklist

Work through [`cis-ig1-gcp-checklist.md`](cis-ig1-gcp-checklist.md) with `findings.md` open beside it. Each requirement carrying a V-number maps directly to a line in the report.

Mark `- [x]` **only where verified in the estate**. A merged PR that has not been applied and confirmed is not compliance.

For each finding, raise the fix and record it:

```
- [ ] All currently public buckets remediated `PR #123 2026-08-20`
```

That middle state moves the item from *outstanding with SRE* to *awaiting management approval*, and the aging shows up in the score.

Remediation for every failure is in [`cis-ig1-remediation-reference.md`](cis-ig1-remediation-reference.md), looked up by safeguard ID.

- [ ] Every automated result reflected in the checklist
- [ ] Every finding has a PR reference or is recorded as outstanding

---

## Phase 9 — The 102 manual requirements

No script covers these. They appear under each safeguard in the validation document as *manual*, and they are mostly documentation: written processes, named owners, review cadences.

The most common audit failure here is a missing **date**, not a missing document. A process with no review date cannot be shown to be current.

- [ ] All 102 manual requirements assessed

---

## Phase 10 — Score

```bash
go run compliance-report.go              # report
go run compliance-report.go --update     # sync Status lines to the checkboxes
```

- [ ] Score recorded
- [ ] `docs/cis-ig1-gcp-checklist.md` committed

---

## Phase 11 — Tear down the audit access

**Do this as soon as the run is finished.** The audit identity holds 22 org-level roles including `storage.admin` and `logging.privateLogViewer`. Left in place it is a standing privileged account with no business owner — which fails safeguards **5.1** (account inventory), **5.4** (restrict administrator privileges) and **6.2** (access revoking process), the very controls this audit just measured.

### Stop impersonating first

**Order matters.** The service account does not hold `resourcemanager.organizations.setIamPolicy`, so it cannot revoke its own bindings. Revoke while still impersonating and every command fails.

```bash
gcloud config unset auth/impersonate_service_account

# Confirm you are back to your own account
gcloud config get-value auth/impersonate_service_account
```

### Destroy the audit identity

```bash
cd terraform/audit-service-account
terraform destroy
```

State records exactly what was created, so this removes exactly that — the service account, every organization binding, the three custom roles, and the impersonation grants. Nothing to reconcile by hand.

Verify:

```bash
eval "$(terraform output -raw teardown_verification)"
gcloud iam roles list --organization="$ORG_ID" --filter="name~cisIg1Audit"
```

Both empty means no trace remains.

If the audit was provisioned with `gcloud` rather than Terraform, use the revoke loop against `./audit-state/roles-granted.txt` instead, then delete the service account and the custom roles by hand.

### Disable only the APIs this audit enabled

```bash
if [ -s ./audit-state/apis-enabled-by-audit.txt ]; then
  while read -r API; do
    echo "  disabling $API"
    gcloud services disable "$API" --project="$AUDIT_PROJECT" --force --quiet
  done < ./audit-state/apis-enabled-by-audit.txt
else
  echo "  no APIs were enabled by this audit — nothing to disable"
fi
```

Never disable from the full list in Phase 1 — some were likely already on and in use. `apis-enabled-by-audit.txt` is the difference between the two snapshots and is the only safe input here.

### Delete the service account

This removes the token-creator binding along with it:

```bash
gcloud iam service-accounts delete "$AUDIT_SA" --project="$AUDIT_PROJECT" --quiet
```

If `AUDIT_SA` is no longer set in this shell:

```bash
export AUDIT_SA=$(sed 's|^serviceAccount:||' ./audit-state/audit-member.txt)
```

### Verify nothing is left behind

```bash
# No org-level bindings for the audit identity
gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten="bindings[].members" \
  --filter="bindings.members:${AUDIT_MEMBER#*:}" \
  --format="value(bindings.role)"

# Service account gone
gcloud iam service-accounts describe "$AUDIT_SA" --project="$AUDIT_PROJECT" 2>&1 | tail -1

# Impersonation cleared
gcloud config get-value auth/impersonate_service_account
```

Expected: nothing, `NOT_FOUND`, and an unset value.

- [ ] Impersonation cleared **before** revoking
- [ ] All 22 role bindings revoked
- [ ] Custom key-reader role unbound and deleted
- [ ] Only audit-enabled APIs disabled
- [ ] Service account deleted
- [ ] All three verification commands return the expected empty/NOT_FOUND
- [ ] Teardown date recorded alongside the findings

Keep `./audit-state/` with the findings. It is the record of what the audit changed and that it was reversed.

---

## Reporting the result

**Quote both numbers.** They say different things:

- **Requirement level (of 290)** — shows progress, moves steadily
- **Safeguard level (of 44)** — the audit-facing figure, moves far more slowly, because one unticked requirement keeps a whole safeguard open

Expect a large gap between them early on. That gap is real, not a reporting artefact.

**State the scope boundary explicitly.** We are responsible for Google Cloud and nothing else; anything outside GCP is referenced only where it engages with GCP — 2SV enforcement lives in the Cloud Identity Admin Console, but it gates GCP access, so we assess it.

These 290 requirements are the GCP-actionable half of IG1, and **all 290 are ours** — including the 102 that are process and documentation rather than infrastructure, because each concerns the GCP estate. The 12 workforce and endpoint safeguards — security awareness training, end-user device encryption and firewalls, removable media — have no GCP surface, are out of scope here, and are owned elsewhere. See [Appendix A](cis-ig1-gcp-checklist.md#appendix-a--safeguards-excluded-from-this-checklist).

Without that caveat, 100% on this checklist will be read as 100% IG1 compliance. It is not, and the difference is twelve safeguards nobody on the platform team can close.

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This is an implementation aid, not affiliated with or endorsed by CIS.*
