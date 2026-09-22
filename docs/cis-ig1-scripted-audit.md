# Running the scripted audit

The automated half — 188 of the 290 requirements. The other 102 are manual: see the [runbook](cis-ig1-audit-runbook.md) phases 8–9 and the [process worksheet](training/09-process-interview.md).

For the full engagement including teardown and reporting, follow the [runbook](cis-ig1-audit-runbook.md). This page is just the terminal work.

## Setup

Set up your shell with [auditor setup](cis-ig1-auditor-setup.md) steps 1–7: tools, sign-in, your branch, variables, impersonation, the token check, and `config/audit.env` with `audit-state/`. With the Cloud Shell setup done, that is `audit-on`.

The APIs and the service account must exist first. They are set up once per engagement: [runbook phase 1](cis-ig1-audit-runbook.md#phase-1--prerequisites).

## Smoke test

```bash
go run audit-run.go -scope=org -org="$ORG_ID" -only V27,V43,V86,V125,V181 -no-prompt
```

Five organization checks, one per permission family. Any `DENIED` is a missing grant — fix and rerun before continuing, because on some checks a permission gap becomes a false `PASS`.

## Config

Created in [setup step 7](cis-ig1-auditor-setup.md#7-settings-then-the-output-directory). One prerequisite value — `APPROVED_REGISTRIES`, the registry prefixes images may come from. `ALLOWED_LOCATIONS` defaults to the continental United States and only needs setting if data lives elsewhere. Everything else is discovered, or asked of a human at review time.

**If the customer has no approved-registry list, write `none`, not blank.** Blank gives SKIP and the two checks disappear from the report. `none` gives FAIL, which is the truth — with no list of approved sources, safeguard 2.3 is not met.

## Run

In one command — org pass, project passes, rollup and score, filed into `audit-state/runs/<date_time>/report/` and `evidence/`:

```bash
./run-audit.sh --projects config/projects.txt   # or --project ID / --all
```

The run finishes with REVIEW checks undecided. Decide them, one at a time, then read the three numbers at the top of each report — Completion, Pass, Fail:

```bash
./run-audit.sh --review ./audit-state/runs/<date_time>
```

Or step by step:

```bash
# Organization — 86 checks, once. Always first.
go run audit-run.go -scope=org -org="$ORG_ID" \
  -pack ./audit-state/org

# Project — 103 checks, once per project
gcloud projects list --format="value(projectId)" | sort > ./audit-state/projects.txt
export PROJECT=<pick one>
go run audit-run.go -scope=project -org="$ORG_ID" -project="$PROJECT" \
  -pack "./audit-state/projects/$PROJECT"
```

Organization first — its findings explain the project results. A missing org policy constraint is *why* fifty projects each have a default network.

Within a project, every resource of the relevant kind is checked. One non-compliant Cloud SQL instance out of ten fails the safeguard, and the output names it.

## Output

Each pack is three files: `01-automated-results.md` (the results table, then a **Detail** block for every check in V-number order — verdict, pass criterion, why it got that verdict, output, and the command), `02-manual-cli.md` (30 console tasks), `03-manual-process.md` (72 process requirements). Project packs have only the first.

The results file lists **all 188 checks, V1 to V188**, whichever pass it came from. Checks that belong to the other pass are marked **ORG** in a project report and **PROJECT** in the organization report: not run there, not counted in the verdicts, but shown with their pass criterion, their command, and where their result is. A project report therefore accounts for every check without a gap in the numbering.

| Verdict | Meaning |
|---|---|
| `PASS` | Compliant |
| `FAIL` | A finding — output names the resources |
| `REVIEW` | Ran clean; you judge the result |
| `SKIP` | Value not supplied — should be zero |
| `N/A` | Product absent, not a failure |
| `DENIED` | Missing permission — fix before trusting anything |

96 of 188 are auto-scored. The other 92 are `REVIEW` with output saved, because no machine can tell you whether your org policy list matches your intended baseline, or which of your buckets holds the backups.

Useful flags: `-list` · `-only V27,V91` · `-parallel 4` · `-timeout 10m` · `-review review.md` · `-format=md -out f.md`

## Stop

```bash
gcloud config unset auth/impersonate_service_account
```

Used the [Cloud Shell setup](cis-ig1-auditor-setup.md#part-2--cloud-shell-one-time-setup)? Impersonation lives in the `cis-audit` configuration instead: run `audit-off`, or open a new tab, then [remove the setup](cis-ig1-auditor-setup.md#removing-it-at-the-end-of-the-engagement) at the end of the engagement.

Before any teardown — the service account cannot delete itself. Teardown is [runbook phase 11](cis-ig1-audit-runbook.md#phase-11--tear-down-the-audit-access), and skipping it fails safeguards 5.1, 5.4 and 6.2.
