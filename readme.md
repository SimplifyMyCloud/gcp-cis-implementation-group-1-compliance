# CIS Controls v8.1 IG1 — GCP Audit Kit

Audit a Google Cloud Organization against CIS IG1. Read-only, scripted where possible, honest about what isn't.

## Documents

| | |
|---|---|
| [`docs/cis-ig1-run-sheet.md`](docs/cis-ig1-run-sheet.md) | **At the terminal.** Flat copy-paste commands, org then per project. |
| [`docs/cis-ig1-audit-runbook.md`](docs/cis-ig1-audit-runbook.md) | The same, with context and reasoning. |
| [`docs/cis-ig1-scripted-audit.md`](docs/cis-ig1-scripted-audit.md) | Just the terminal work. |
| [`docs/cis-ig1-overview.md`](docs/cis-ig1-overview.md) | Why each of the 18 Controls exists. Read once. |
| [`docs/cis-ig1-gcp-checklist.md`](docs/cis-ig1-gcp-checklist.md) | The working checklist — 290 requirements. |
| [`docs/cis-ig1-cli-validation.md`](docs/cis-ig1-cli-validation.md) | 188 `gcloud` checks with pass criteria. |
| [`docs/cis-ig1-remediation-reference.md`](docs/cis-ig1-remediation-reference.md) | Example Terraform to fix a finding. |
| [`docs/cis-ig1-tracker.xlsx`](docs/cis-ig1-tracker.xlsx) | Spreadsheet for tracking effort. Imports into Sheets. |
| [`docs/training/`](docs/training/) | 20-minute class for SREs. |
| [`terraform/`](terraform/) | The read-only audit service account. |
| [`gcloud/`](gcloud/) | Same account via shell scripts, where Terraform is unavailable. |
| [`docs/testing/required-apis.md`](docs/testing/required-apis.md) | The 17 APIs the audit project needs, with evidence from a live run. |
| [`docs/testing/bug-log.md`](docs/testing/bug-log.md) | What a live test run found and fixed. |

## The numbers

| | |
|---|---|
| IG1 safeguards | 56 |
| GCP-actionable | 44 (the other 12 are training, endpoints, removable media) |
| Checklist requirements | 290 |
| With a CLI check | 188 · **96 auto-scored**, 92 need a human to read the output |
| Manual | 102 · 30 console tasks, 72 process and documentation |

100% here is the GCP half of IG1, not IG1. Say so when reporting.

## Quick start

Prerequisites: `go`, `jq`, gcloud with the `alpha` and `beta` components, Terraform, and the [17 APIs](docs/testing/required-apis.md#1-enable-in-the-audit-host-project--17-apis) enabled in the audit project.

```bash
export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)

cd terraform/audit-service-account
gcloud auth application-default login
# edit terraform.tfvars — replace the REPLACE_* values
terraform init && terraform apply
eval "$(terraform output -raw impersonate_command)"

cd ../..
go run audit-run.go -scope=org -org=$ORG_ID -init-config ./audit-state/audit.env
# edit audit.env — APPROVED_REGISTRIES (ALLOWED_LOCATIONS defaults to the continental US)

go run audit-run.go -scope=org -org=$ORG_ID -config ./audit-state/audit.env -pack ./audit-state/org
go run audit-run.go -scope=project -org=$ORG_ID -project=PROJECT -config ./audit-state/audit.env -pack ./audit-state/projects/PROJECT

go run rollup.go -in ./audit-state -out ./audit-state/remediation-plan.md -csv ./audit-state/remediation-plan.csv
go run compliance-report.go        # score the checklist
```

**Or run the whole audit with one command.** Once impersonation is set and `audit.env` is filled in, `run-audit.sh` runs the org pass, the project passes, rollup and the score, and files everything for that run into one dated directory:

```bash
./run-audit.sh --org $ORG_ID --config ./audit-state/audit.env --project PROJECT     # repeatable
./run-audit.sh --org $ORG_ID --config ./audit-state/audit.env --projects list.txt  # one ID per line
./run-audit.sh --org $ORG_ID --config ./audit-state/audit.env --all                # every ACTIVE project
```

```
scratch/runs/2026-09-14_12-58-20/
  report/                        what the auditor reads, in reading order
    01-remediation-plan.md
    02-organization/
      01-automated-results.md
      02-manual-gcp-tasks.md
      03-manual-process.md
    03-projects/
      <project-id>.md            one per project
    04-compliance-score.txt
    remediation-plan.csv         import into the tracker
  evidence/
    audit.env  targets.txt  run.log  iam-inventory.txt  not-found.txt
```

`--skip V96` leaves out a check (reported as SKIP) and `--parallel 1` runs checks one at a time, in order. Projects whose ID matches `EXCLUDE_PROJECTS` in `audit.env` are never audited — default `^sys-`, the projects Apps Script creates; they are listed in `evidence/excluded.txt`.

Runs go to `./scratch/runs/` (git-ignored) by default; `--out ./audit-state/runs` keeps them in the repository. It exits non-zero only if a pass is `UNRELIABLE` or `DEGRADED`.

A healthy pass says `RUN STATUS: OK` at the top of `01-automated-results.md`. `audit-run.go` exits non-zero whenever a check FAILs — that means findings, not a broken run. Step-by-step: [run sheet](docs/cis-ig1-run-sheet.md).

The audit runs as a read-only service account, impersonated never keyed — a key would breach safeguard 5.2, which this audit tests. `terraform destroy` removes every trace.

## Three things that catch people out

**Org policy is not retroactive.** A constraint blocks new violations and does nothing to what already exists. Every constraint fix is half a fix.

**One resource fails, the safeguard fails.** No partial credit — ten Cloud SQL instances, one non-compliant, the safeguard is not met.

**`[x]` means verified in the estate.** Not merged, not applied — verified.

## Scoring

```bash
go run compliance-report.go              # report
go run compliance-report.go --update     # sync Status lines to the checkboxes
```

Requirements can be `Not started`, `In progress`, `PR submitted`, `Compliant`, or `N/A`. `PR submitted` moves an item from engineering's queue to awaiting management approval, and the report ages it.

### The Go scripts

`audit-run.go`, `rollup.go` and `compliance-report.go` are **standard library only**. No `go.mod`, no dependencies, no build step — `go run <file>.go` works on any machine with Go installed.

That is deliberate and worth protecting. These run inside a customer's environment, and "install these modules first" is a conversation you do not want to have there. Where a dependency would buy convenience — writing `.xlsx` directly rather than emitting CSV, for instance — **take the CSV and the three clicks.**

Each file carries `//go:build ignore` so several `package main` files can share a directory without colliding under `go vet ./...`. Audit results are committed to this repository, which is private.

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This is an implementation aid, not affiliated with or endorsed by CIS.*
