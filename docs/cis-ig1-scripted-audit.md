# Running the scripted audit

The automated half — 188 of the 290 requirements. The other 102 are manual: see the [runbook](cis-ig1-audit-runbook.md) phases 8–9 and the [process worksheet](training/09-process-interview.md).

For the full engagement including teardown and reporting, follow the [runbook](cis-ig1-audit-runbook.md). This page is just the terminal work.

## Setup

```bash
go version && jq --version          # jq is required, not optional
gcloud components list --filter="id:(alpha beta)" --format="value(id,state.name)"   # both installed
export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)
export AUDIT_PROJECT=<project carrying API quota>
```

Enable the APIs and create the service account — [runbook phase 1](cis-ig1-audit-runbook.md#phase-1--prerequisites). Then impersonate:

```bash
cd terraform/audit-service-account
eval "$(terraform output -raw impersonate_command)"
gcloud config get-value auth/impersonate_service_account   # must print the SA
```

> `gcloud auth list` will still show your own address. That is correct — impersonation layers a short-lived token over your credential rather than switching accounts, which is why audit logs record both identities.

## Smoke test

```bash
cd ../..
go run audit-run.go -scope=org -org="$ORG_ID" -only V27,V43,V86,V125,V181 -no-prompt
```

Five organization checks, one per permission family. Any `DENIED` is a missing grant — fix and rerun before continuing, because on some checks a permission gap becomes a false `PASS`.

## Config

```bash
go run audit-run.go -init-config ./audit-state/audit.env
```

Eleven prerequisite values — buckets, backup project, approved registries, allowed locations, retention and dormancy thresholds, backup identity, production projects and regions. The template names each with an example; the full table is in [CLI validation](cis-ig1-cli-validation.md). Everything else is discovered.

**If one does not exist, write `none`, not blank.** Blank gives `SKIP` ("could not check"); `none` gives `FAIL`, which is the truth — no backup bucket is safeguard 11.3 failing.

## Run

In one command — org pass, project passes, rollup and score, filed into `scratch/runs/<date_time>/report/` and `evidence/`:

```bash
./run-audit.sh --org "$ORG_ID" --config ./audit-state/audit.env --projects projects.txt   # or --project ID / --all
```

Or step by step:

```bash
# Organization — 71 checks, once. Always first.
go run audit-run.go -scope=org -org="$ORG_ID" \
  -config ./audit-state/audit.env -pack ./audit-state/org

# Project — 87 checks, once per project
gcloud projects list --format="value(projectId)" | sort > ./audit-state/projects.txt
export PROJECT=<pick one>
go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
  -config ./audit-state/audit.env -pack "./audit-state/projects/$PROJECT"
```

Organization first — its findings explain the project results. A missing org policy constraint is *why* fifty projects each have a default network.

Within a project, every resource of the relevant kind is checked. One non-compliant Cloud SQL instance out of ten fails the safeguard, and the output names it.

## Output

Each pack is three files: `01-automated-results.md` (the results table, then a **Detail** block for every check in V-number order — verdict, pass criterion, why it got that verdict, output, and the command), `02-manual-cli.md` (30 console tasks), `03-manual-process.md` (72 process requirements). Project packs have only the first.

| Verdict | Meaning |
|---|---|
| `PASS` | Compliant |
| `FAIL` | A finding — output names the resources |
| `REVIEW` | Ran clean; you judge the result |
| `SKIP` | Value not supplied — should be zero |
| `N/A` | Product absent, not a failure |
| `DENIED` | Missing permission — fix before trusting anything |

109 of 188 are auto-scored. The other 79 are `REVIEW` with output saved, because no machine can tell you whether your org policy list matches your intended baseline.

Useful flags: `-list` · `-only V27,V91` · `-parallel 4` · `-timeout 10m` · `-review review.md` · `-format=md -out f.md`

## Stop

```bash
gcloud config unset auth/impersonate_service_account
```

Before any teardown — the service account cannot delete itself. Teardown is [runbook phase 11](cis-ig1-audit-runbook.md#phase-11--tear-down-the-audit-access), and skipping it fails safeguards 5.1, 5.4 and 6.2.
