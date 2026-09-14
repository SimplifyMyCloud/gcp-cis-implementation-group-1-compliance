# Automated audit platform — specification

**Status:** spec, not built. Agreed 2026-09-14; build starts after the manual customer test run.

Runs the CIS IG1 audit once or twice a week, until the organization reaches 100%, from a dedicated
project in the customer's organization. Everything is Terraform. Between runs, the audit identity
holds **no organization roles** and the audit APIs are **off**.

## Decisions

| # | Decision | Chosen |
|---|---|---|
| D1 | Where it lives | A dedicated project **inside the customer's organization**. All IDs are variables |
| D2 | How a run is triggered | **A + B**: an operator runs one command. Org access is granted with a **time-limited IAM condition** and destroyed at the end; if teardown is missed, the grant expires on its own |
| D3 | Hosting | **Cloud Run Jobs** (batch, no endpoint) |
| D4 | Project scope | **All projects, or a supplied list** |
| D5 | Output | **GCS bucket**, plus a summary printed to the operator's terminal |
| D6 | Terraform state | The customer's existing state bucket by default; location configurable |
| D7 | Code changes | Approved: host-project override, run-status wrapper, API preflight |
| D8 | Hosting project | **Supplied by the customer.** Terraform uses it; never creates or deletes it |
| D9 | Screen output | **Live progress while the jobs run, plus the end-of-run summary** |
| D10 | Access scope | **Organization-level bindings, org pass then per-project passes** — the same model as today's manual audit |

Rejected: unattended scheduling. Something would have to hold org IAM admin permanently to grant and
revoke access, which is a worse standing privilege than the read-only auditor it replaces.

## Architecture

Two Terraform root modules, split by what our testing showed can and can't be recreated twice a week.

```
infra/
  platform/     PERMANENT — applied once, grants nothing by itself
  access/       PER RUN   — the privilege; applied at start, destroyed at end
  image/        Dockerfile + entrypoint
audit.sh        operator wrapper: apply access → run jobs → print summary → destroy access
```

### `infra/platform` — permanent

| Resource | Why permanent |
|---|---|
| Project — **supplied by the customer**, referenced by `project_id`; Terraform does not create it | Home for everything |
| APIs Terraform itself needs: `serviceusage`, `cloudresourcemanager`, `iam`, `run`, `artifactregistry`, `storage`, `logging` | Can't manage the rest without them |
| Service account `cis-ig1-auditor` — **no roles** | An identity without bindings can do nothing. Recreating it each run would change its unique ID for no benefit |
| 3 custom roles (`StorageReader`, `KeyReader`, `IapReader`) | **Can't be deleted and recreated weekly.** Role IDs stay reserved for up to ~37 days after deletion (BUG-001). A role definition grants nothing until it is bound |
| Artifact Registry repository + image | The job container |
| Results bucket — versioned, uniform access, public access prevention, retention (V8), readable only by `results_readers` | Findings name every public bucket and open firewall in the estate |
| 3 Cloud Run Jobs: `audit-org`, `audit-projects`, `audit-rollup` | Job definitions grant nothing; they run as the SA, which has no roles yet |
| `roles/run.invoker` on the jobs, `roles/logging.viewer` on the project, for `operators` | Who may start a run and watch it |

### `infra/access` — per run

| Resource | Detail |
|---|---|
| 30 predefined + 3 custom **organization** role bindings to the SA | `google_organization_iam_member` (additive, never `_binding`). Each carries `condition { expression = "request.time < timestamp(\"<expiry>\")" }` |
| Expiry | `time_offset` (hashicorp/time), `access_hours` from apply time (default **8**), keyed to `run_id` so every run gets a fresh window |
| The 17 audit APIs in the audit project | `google_project_service`, `disable_on_destroy = true`, `disable_dependent_services = false` |
| Optional: `securitycenter.adminViewer`, `billing.viewer` | Variables, default off (see [required-apis.md](../testing/required-apis.md) §3) |

The 17 APIs overlap the platform set (`cloudresourcemanager`, `iam`, `logging`, `serviceusage`).
Those are owned by `platform` and **left out of `access`**, so destroying access never switches off
what Terraform needs.

### Runtime flow

```
operator$ ./audit.sh [--projects projects.txt | --all]
  1. run_id = UTC timestamp
  2. terraform -chdir=infra/access apply -var run_id=… -auto-approve   (as the operator)
  3. preflight job step: wait until every audit API answers              (≤ 25 min, then fail)
  4. gcloud run jobs execute audit-org      --wait
  5. gcloud run jobs execute audit-projects --wait  --tasks=N  (sharded list)
  6. gcloud run jobs execute audit-rollup   --wait
     (steps 4–6 stream live progress: audit.sh tails the execution's logs)
  7. print summary to screen, from the bucket
  8. terraform -chdir=infra/access destroy -auto-approve                (trap: runs on failure and Ctrl-C too)
```

Step 8 runs from a shell `trap`, so an interrupted run still revokes access. If the operator's
machine dies mid-run, the IAM condition expires on its own after `access_hours`; the next `audit.sh`
run destroys the stale access state first.

## The container (`infra/image`)

| Layer | Content |
|---|---|
| Base | `gcr.io/google.com/cloudsdktool/google-cloud-cli:slim`, pinned by digest |
| Add | `jq`, `gcloud components install alpha beta` (7 checks) |
| Build stage | `golang` image, pinned: `go build audit-run.go`, `go build rollup.go`. Still standard library only |
| Copy | the binaries, `docs/cis-ig1-cli-validation.md` (the checks live there, `-doc`), `entrypoint.sh` |
| Identity | Cloud Run attaches the SA; gcloud reads credentials from the metadata server. No keys, no impersonation |

### `entrypoint.sh` modes

| Mode | Does |
|---|---|
| `org` | Preflight → `audit-run -scope=org -pack /work/org` → upload the pack to `gs://…/runs/<run_id>/org/` |
| `projects` | Reads the target list, takes the slice for this task (`CLOUD_RUN_TASK_INDEX` / `CLOUD_RUN_TASK_COUNT`), runs `audit-run -scope=project` for each, uploads each pack as it finishes |
| `rollup` | Downloads every pack for `run_id` → `rollup` → uploads `remediation-plan.md`, `remediation-plan.csv`, `summary.txt` |

Every mode:

- **Judges the run by `RUN STATUS`, not the exit code.** `audit-run` exits 1 whenever it finds problems (code change 2). A task fails only if a pack reports `UNRELIABLE`/`DEGRADED`, or `audit-run` exits 2 (usage/IO).
- **Streams progress to stdout**, so Cloud Logging carries the live `[ n/ N] V55 FAIL …` lines.
- **Passes placeholders as a config file.** `BACKUP_BUCKET`, `TFSTATE_BUCKET` and `BACKUP_PROJECT` come from Terraform variables (`none` allowed) as job environment variables. `entrypoint.sh` writes them to `/work/audit.env` and calls `audit-run -config`. Not environment variables straight through: `substitute()` honours `none` only from `-config`/`-set`, and an environment variable `none` would be substituted literally (`gs://none`).

## Targeting projects (D4)

| `audit.sh` flag | Behaviour |
|---|---|
| `--all` (default) | The `projects` job discovers `gcloud projects list --filter=lifecycleState:ACTIVE` at run time |
| `--projects FILE` | One project ID per line; `#` comments allowed. Uploaded to `gs://…/runs/<run_id>/targets.txt` |
| `--project ID` | Repeatable, for a quick single-project re-check |

- **Sharding:** `tasks = min(ceil(projects / projects_per_task), max_tasks)`, defaults 10 per task and 10 tasks, with task parallelism 10. 105 projects at about 2.5 minutes each is about 4.5 hours serial and about 30 minutes sharded.
- **Validation:** unknown IDs are reported as `NOT FOUND` in the summary, not silently dropped.

## Output (D5)

```
gs://<results_bucket>/runs/<run_id>/
  targets.txt
  org/01-automated-results.md, 02-manual-cli.md, 03-manual-process.md, iam-inventory.txt
  projects/<project>/01-automated-results.md
  remediation-plan.md
  remediation-plan.csv
  summary.txt
```

On screen at the end of `audit.sh`:

```
CIS IG1 audit  run 20260917T140000Z  org 1234…  105 projects
  org pass        RUN STATUS: OK        PASS 7  FAIL 29  REVIEW 47
  project passes  105 OK · 0 DEGRADED · 0 UNRELIABLE
  findings        51 distinct  (rollup)
  not found       0
  results         gs://…/runs/20260917T140000Z/
  access          revoked ✔  (expiry would have been 22:00Z)
```

It also offers the `gcloud storage cat` command for `remediation-plan.md`.

**Live progress (D9).** While each job runs, `audit.sh` polls Cloud Logging every 10 seconds for
that execution's stdout (`resource.type="cloud_run_job"`,
`labels."run.googleapis.com/execution_name"=<execution>`, newer than the last timestamp seen) and
prints the new `[ n/ N] V55 FAIL …` lines, prefixed with the task index for the sharded job. It
polls rather than using a streaming tail because `gcloud alpha logging tail` needs an extra gRPC
library on the operator's machine. Lines may arrive a few seconds late and out of order across
tasks; the final summary is authoritative. `--quiet` turns it off. Operators need
`roles/logging.viewer` on the audit project, which `platform` grants to `operators`.

## Terraform state (D6)

Each root module uses a **GCS backend with partial configuration**. Backends can't read variables, so
the location goes in a `backend.hcl` the customer controls:

```hcl
# infra/platform/backend.hcl  (and infra/access/backend.hcl)
bucket = "customer-tf-state"
prefix = "cis-ig1-audit/platform"      # access: cis-ig1-audit/access
```

`terraform init -backend-config=backend.hcl`. Separate prefixes per module, so destroying access
can never touch the platform state. `audit.sh` takes `--backend-config-dir` and defaults to `infra/`.

## Variables

| Variable | Module | Default | Notes |
|---|---|---|---|
| `organization_id` | both | — | numeric |
| `project_id` | both | — | the dedicated audit project |
| `region` | platform | `us-central1` | Cloud Run, Artifact Registry and bucket location |
| `operators` | platform | — | `user:`/`group:`; may run `audit.sh` and read results |
| `results_readers` | platform | `[]` | extra read-only access to the bucket |
| `results_retention_days` | platform | `365` | bucket retention |
| `role_prefix` | platform | `cisIg1Audit` | change if IDs are reserved (BUG-001) |
| `backup_bucket`, `tfstate_bucket`, `backup_project` | platform | `none` | become job environment variables |
| `projects_per_task`, `max_tasks`, `task_timeout` | platform | `10`, `10`, `6h` | sharding |
| `access_hours` | access | `8` | lifetime of the conditional grants |
| `run_id` | access | — | set by `audit.sh` |
| `enable_securitycenter`, `enable_billing_viewer` | access | `false` | optional roles |

## Code changes (D7, approved)

| # | Change | Where |
|---|---|---|
| C1 | `AUDIT_HOST_PROJECT` environment variable overrides host-project detection. On Cloud Run there's no impersonation setting to read, so disabled host APIs would otherwise report N/A instead of ERROR | `audit-run.go` `resolveHostProject()` |
| C2 | Judge the run by `RUN STATUS`, not exit code | `entrypoint.sh`. A flag, e.g. `-exit-on=unreliable`, is the cleaner option in `audit-run.go`; decide at build |
| C3 | API preflight: poll a cheap read per audit API until it answers, up to 25 minutes; fail with the list still pending | `entrypoint.sh` (bash + gcloud, no Go change) |

## Operator permissions

The person running `audit.sh` needs what the grant requires, **only during the run**:

- `roles/resourcemanager.organizationAdmin` or `roles/iam.securityAdmin` at the org, to create the conditional bindings.
- `roles/serviceusage.serviceUsageAdmin` on the audit project, for the APIs.
- `roles/run.invoker` on the jobs, from `operators`.
- Read on the results bucket, from `operators`.

## Risks to verify during the build

| Risk | Why it matters | Verification |
|---|---|---|
| R1 **Conditional org bindings don't grant access for every service** | A service that doesn't evaluate IAM conditions ignores the binding → DENIED checks → UNRELIABLE pack. **The most likely thing to force a design change** | Full org + project run with conditional grants in the test org must show 0 DENIED. Fallback: unconditional bindings for the affected roles, still destroyed at the end |
| R2 API enable/disable lag | Up to ~20 minutes observed (V116 run 8). Disabling then re-enabling 3 days later may also lag | Preflight C3; time it across two consecutive runs |
| R3 Consumer project on Cloud Run | Tested with impersonation: calls billed to the SA's project. Assumed the same with metadata credentials | Read the request-count metrics after the first Cloud Run run, as for required-apis.md |
| R4 Destroying access disables an API another workload uses | The audit project is dedicated; `platform` owns the shared ones | `terraform plan` on access shows only the 13 audit-only APIs |
| R5 A task outlives its timeout | A large project with many buckets (V30/V37/V155 loop per bucket) | Per-task timeout `6h`; `-timeout` per check stays 3 minutes |
| R6 Role list drift | Roles now defined in `terraform/audit-service-account`, `gcloud/create.sh` and `infra/access` | One JSON role list read by both Terraform trees; `create.sh` drift check extended |

## Acceptance

1. `terraform apply` on `platform` from a clean clone with only variables and `backend.hcl` edited.
2. `./audit.sh --project iq9-gcp-dev-yamato` completes in the test org: org pass and project pass `RUN STATUS: OK`, remediation plan in the bucket, summary on screen.
3. `./audit.sh --all` completes: every pack `OK`, one rollup.
4. After each run: no organization bindings for the SA (`teardown_verification`); the 13 audit-only APIs are disabled.
5. Kill `audit.sh` mid-run: the trap destroys access. Kill it with `-9`: bindings stop granting after `access_hours`, and the next run cleans up first.
6. Findings match a manual run of the same commit (same PASS/FAIL per check).
7. `terraform destroy` on both modules leaves the project empty, apart from what `platform` was told to keep.

## Resolved questions (2026-09-14)

- **V1** The customer supplies the project → D8.
- **V2** Both live progress and the end-of-run summary → D9.
- **V3** Organization-level bindings with an org pass and per-project passes, as today → D10.
