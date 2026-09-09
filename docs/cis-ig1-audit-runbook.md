# Audit Runbook

> Running it right now? [`cis-ig1-run-sheet.md`](cis-ig1-run-sheet.md) is the same sequence as bare commands with no explanation. This page is the reasoning behind them.

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

```bash
mkdir -p ./audit-state          # everything this audit records lives here
gcloud auth login
export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)
export AUDIT_PROJECT=<project carrying API quota>
gcloud config set project $AUDIT_PROJECT
which jq                        # required by ~45 checks, not optional
```

### Enable the APIs

**Snapshot first** — at teardown you must disable only the APIs *you* turned on.

```bash
gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-before.txt

gcloud services enable \
  cloudasset.googleapis.com essentialcontacts.googleapis.com \
  accesscontextmanager.googleapis.com recommender.googleapis.com \
  policyanalyzer.googleapis.com osconfig.googleapis.com \
  --project=$AUDIT_PROJECT

gcloud services list --enabled --project="$AUDIT_PROJECT" \
  --format="value(config.name)" | sort > ./audit-state/apis-after.txt

comm -13 ./audit-state/apis-before.txt ./audit-state/apis-after.txt \
  | tee ./audit-state/apis-enabled-by-audit.txt
```

### Create the audit service account

The audit runs as a dedicated read-only service account you impersonate — never as your own user. Teardown can then revoke unconditionally without stripping bindings you already had, every action is attributable to one identity in the logs, and a human holding standing org-wide read is the pattern safeguard 5.4 flags.

```bash
cd terraform/audit-service-account
gcloud auth application-default login    # ADC — separate from `gcloud auth login`
# edit terraform.tfvars
terraform init && terraform plan && terraform apply

eval "$(terraform output -raw impersonate_command)"
gcloud config get-value auth/impersonate_service_account
```

That must print the auditor service account. `gcloud auth list` will still show *your* address — impersonation layers a token over your credential rather than switching accounts, which is why audit logs record both identities.

Every permission is read-only, including custom roles replacing predefined ones that carry write verbs. Full detail: [`terraform/readme.md`](../terraform/readme.md).

**Enable the APIs before switching**, or as yourself — a new service account cannot enable services.

- [ ] `ORG_ID` and `AUDIT_PROJECT` exported
- [ ] `jq` present
- [ ] `apis-enabled-by-audit.txt` written
- [ ] Service account created and impersonation active
- [ ] No service account key created

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

## Phase 6b — Compile the plan

Once every project pass is done, turn 105 packs into one work list:

```bash
go run rollup.go -in ./audit-state \
  -out ./audit-state/remediation-plan.md \
  -csv ./audit-state/remediation-plan.csv
```

Pivoted on the **finding**, not the project — one org policy change that fixes 40 projects is one work item, not forty. Ranked by how many projects each affects.

**Scope** on each row says where the work happens: `org` once, `project × N` repeated, or `org + project × N` for both — apply the constraint *and* clean up what already violates it, because org policy is not retroactive.

Packs whose run was `UNRELIABLE` or `DEGRADED` are **excluded and listed separately**. A permission failure is not a compliance finding, and counting it would invent work.

Import the CSV into the tracker as a new tab: **File → Import → Upload → Insert new sheet**.

- [ ] `remediation-plan.md` produced
- [ ] Excluded packs re-run
- [ ] CSV imported into the tracker

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

**Do this as soon as the run finishes.** The audit identity holds org-wide read; left in place it fails safeguards 5.1, 5.4 and 6.2 — controls this audit just measured.

```bash
gcloud config unset auth/impersonate_service_account   # FIRST — the SA cannot delete itself

cd terraform/audit-service-account
terraform destroy
```

Then disable only the APIs this audit enabled:

```bash
while read -r API; do
  gcloud services disable "$API" --project="$AUDIT_PROJECT" --force --quiet
done < ./audit-state/apis-enabled-by-audit.txt
```

Never disable from the full Phase 1 list — some were already on and in use.

Verify:

```bash
eval "$(terraform output -raw teardown_verification)"
gcloud iam roles list --organization="$ORG_ID" --filter="name~cisIg1Audit"
```

Both empty means no trace remains. Console checks: [`terraform/readme.md`](../terraform/readme.md#verify-by-hand-in-the-console).

- [ ] Impersonation cleared **before** destroy
- [ ] `terraform destroy` clean
- [ ] Only audit-enabled APIs disabled
- [ ] Both verification commands empty
- [ ] Teardown date recorded with the findings

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
