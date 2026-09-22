# CIS Controls v8.1 IG1 — GCP Audit Kit

Audit a Google Cloud Organization against CIS IG1. Read-only, scripted where possible, honest about what isn't.

## Start here

Two people, two jobs. They can be the same person.

| Role | Does | When | Needs |
|---|---|---|---|
| **Engagement admin** | Enables the APIs and creates the read-only audit service account | Once per customer, before the first audit | Rights to grant org-level IAM; Terraform, or bash for the `gcloud/` scripts |
| **Auditor** | Sets up a shell, runs the audit, reads and commits the results | Every audit session | The right to impersonate the audit service account — nothing else on the organization |

The audit runs as a read-only service account, impersonated and never keyed — a key would breach safeguard 5.2, which this audit tests.

## The order of work

| # | Step | Who | How |
|---|---|---|---|
| 1 | **Enable the 17 APIs** in the audit host project. Snapshot first, so teardown disables only what you enabled | Admin, once | [Run sheet A2](docs/cis-ig1-run-sheet.md#a2-snapshot-enabled-apis-then-enable) · [the list and why](docs/cis-ig1-required-apis.md) |
| 2 | **Create the audit service account** and grant auditors the right to impersonate it | Admin, once | [`terraform/`](terraform/readme.md), or [`gcloud/create.sh`](gcloud/readme.md) then `verify.sh` |
| 3 | **Set up the auditor shell** — sign in, clone the repo onto your branch, impersonate, prove it, fill in `config/audit.env`, create `audit-state/` | Auditor, every session | **[Auditor setup](docs/cis-ig1-auditor-setup.md)**, and [`config/readme.md`](config/readme.md) for the settings. In Cloud Shell, set it up once and then it is `audit-on` |
| 4 | **Smoke test** — five checks, one per permission family. Any `DENIED` or `ERROR`: stop and fix | Auditor | `go run audit-run.go -scope=org -only V27,V43,V86,V125,V181 -no-prompt` |
| 5 | **Run the audit** | Auditor | [`run-audit.sh`](#run-auditsh--the-whole-audit-in-one-command), below |
| 6 | **Review.** Confirm every pass says `RUN STATUS: OK`, then decide each REVIEW check PASS or FAIL, one at a time. Completion reaches 100% when the last one is decided | Auditor | `./run-audit.sh --review audit-state/runs/<timestamp>` — [below](#the-score-and---review) |
| 7 | **Do the manual half** — 102 requirements no API can answer: 30 console tasks, 72 process questions | Auditor, with the customer | [Runbook phases 8–9](docs/cis-ig1-audit-runbook.md) · [process interview](docs/training/09-process-interview.md) |
| 8 | **Commit the results** to your branch and push | Auditor | `git add audit-state && git commit && git push` — [setup step 8](docs/cis-ig1-auditor-setup.md#8-run-and-commit) |
| 9 | **Tear down** — stop impersonating, destroy the service account, disable only the APIs step 1 enabled | Admin, at the end | [Run sheet Part C](docs/cis-ig1-run-sheet.md#part-c--compile-and-tear-down) |

Audit one project at a time. The first run includes the organization pass; add `--no-org` for each project after that, so the organization's REVIEW checks are decided once rather than per project.

Auditing with two or three people: share one branch, one settings file, and one organization pass — [Running with a team](docs/cis-ig1-audit-runbook.md#running-with-a-team).

To run the passes one at a time instead of step 5, follow the [run sheet](docs/cis-ig1-run-sheet.md) from A8.

Step 9 is not optional. A standing org-wide read identity fails safeguards 5.1, 5.4 and 6.2 — the controls this audit just measured.

## Settings

Everything you have to decide lives in [`config/`](config/readme.md), and nothing else does. Output goes to `audit-state/runs/`, kept apart on purpose: clearing out old runs must never cost you your configuration.

```bash
cp config/audit.env.example config/audit.env     # then fill it in
cp config/projects.txt.example config/projects.txt   # optional
```

`config/audit.env` holds the organization (`ORG_ID`, `AUDIT_PROJECT`, `SA_EMAIL`) and the check inputs (`APPROVED_REGISTRIES`, `EXCLUDE_PROJECTS`, and `ALLOWED_LOCATIONS` if data lives outside the continental US). Fill it in once and no command needs a path or an ID again:

```bash
./run-audit.sh --project some-project-id
```

Commit the filled-in file. The repository assumes it is private, and one shared settings file is what keeps a team of auditors scoring against identical criteria — see [Running with a team](docs/cis-ig1-audit-runbook.md#running-with-a-team). [`config/readme.md`](config/readme.md) is the full reference, including why `none` is an answer and blank is not.

## run-audit.sh — the whole audit in one command

Runs every automated check for the organization and the projects you name, then files the results into one dated directory. Run it once steps 1–4 are done:

```bash
./run-audit.sh --all                            # every ACTIVE project
./run-audit.sh --project PROJECT                # repeatable
./run-audit.sh --projects config/projects.txt   # one ID per line
```

No `--config` and no `--org`: both come from `config/audit.env`. Pass `--config FILE` only to use a different settings file.

It reads the organization from `$ORG_ID` (or `--org`). What it does, in order:

1. **Resolves the targets** from `--all`, `--project` or `--projects`, and drops any project matching `EXCLUDE_PROJECTS` in `config/audit.env` — default `^sys-`, the projects Apps Script creates. Both lists go into `evidence/`.
2. **Prints who it is running as.** If that line says `NOT impersonating`, stop with Ctrl-C — the run is measuring your own access, not the auditor's.
3. **Runs the organization pass** — `audit-run.go -scope=org`, 86 checks.
4. **Runs one project pass per target** — `audit-run.go -scope=project`, 103 checks each. A project that does not exist, or that the auditor cannot see, is recorded as `not found` and skipped.
5. **Builds the remediation plan** with `rollup.go`: one row per finding, with the projects it affects.
6. **Scores the run**, also with `rollup.go`: the three numbers, written twice — `04-compliance-score.md` for people and `04-compliance-score.json` for the compliance dashboard.
7. **Files everything** and prints a summary line per pass, with its score. Each pass's results are saved to `evidence/results/` for `--review`.

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
    04-compliance-score.md       the three numbers, for people
    04-compliance-score.json     the same, for the dashboard
    remediation-plan.csv         import into the tracker
  evidence/
    results/                     each pass's results and review decisions
    audit.env  targets.txt  run.log  iam-inventory.txt  excluded.txt  not-found.txt
```

**It exits non-zero only if a pass is `UNRELIABLE`, `DEGRADED` or wrote nothing.** Failed checks are findings, not a broken run. `--skip V96` leaves out a check that hangs (reported as SKIP), and `--parallel 1` runs checks one at a time, in order. `--out DIR` puts the run somewhere other than `./audit-state/runs`.

### The score and `--review`

Every report opens with three numbers, for the organization and for each project:

| | Means |
|---|---|
| **Completion** | The share of checks with a final PASS or FAIL. **100% means the audit of that project is complete.** Below 100% means REVIEW checks are still undecided, or checks were blocked (SKIP, ERROR, DENIED) |
| **Pass** | PASS, plus N/A — the product is not enabled, so there is nothing in it to fail — plus REVIEW checks the auditor judged PASS |
| **Fail** | FAIL, plus REVIEW checks the auditor judged FAIL |

A run can't decide REVIEW checks itself, so it finishes below 100% and prints the next command:

```bash
./run-audit.sh --review audit-state/runs/2026-09-14_12-58-20
```

That shows each REVIEW check in turn — its pass criterion and the output — and waits for `p` (PASS) or `f` (FAIL). `s` leaves one for later, `v` pages through long output, `q` stops. Each answer is saved as it is given, with who decided and when, so `--review` again carries on where you stopped. Nothing is re-run. When it finishes it rebuilds the reports, the score and the remediation plan; a check decided FAIL becomes a finding like any other.

CIS sets no pass mark: IG1 is "implement every safeguard". A threshold such as 85% is the engagement's own, and should be reported as such.

### The compliance score files

Every run writes its score twice, into `report/`:

| File | For |
|---|---|
| `04-compliance-score.md` | Whoever opens the run directory |
| `04-compliance-score.json` | The compliance dashboard |

The score reflects where it sits. A run holding one project scores that project. Point `rollup.go` at the directory holding every run and it scores the **organization** instead: Pass and Fail compiled across every counted target, and Completion as estate coverage — projects with a finished audit over projects in the organization.

```bash
go run rollup.go -in audit-state/runs -out plan.md -csv plan.csv \
  -score-md score.md -score-json score.json -projects audit-state/projects.txt
```

Two rules keep the totals honest:

- **Only a complete audit counts.** A pass still holding REVIEW checks has not been reconciled, and one holding SKIP, ERROR or DENIED checks did not finish. Neither is a result, so neither contributes to a total; both are listed as `counted: false` with the reason.
- **The newest complete pass wins.** Auditing one project at a time leaves the same project in several run directories. Completeness beats recency: a finished audit from Tuesday counts over a half-reviewed one from Wednesday.

The JSON carries raw counts beside every percentage, so the dashboard never re-derives a rounded figure:

```json
{
  "schema": "cis-ig1-score/1",
  "scope": "project",
  "target": "acme-prod-web",
  "complete": true,
  "checks": { "total": 103, "pass": 89, "fail": 14, "pass_ran": 70, "pass_na": 19,
              "review": 0, "skip": 0, "denied": 0, "error": 0 },
  "score":  { "completion": 100, "pass": 86, "fail": 14 },
  "targets": [ { "name": "acme-prod-web", "counted": true, "...": "per-target detail" } ]
}
```

At organization scope the document also carries `coverage`, and `score.completion` is `-1` when the organization's project total was not supplied with `-projects` — reported rather than guessed, because a dashboard cannot tell a guessed 100% from a real one.

Do not edit `run-audit.sh` while it is running. Bash reads a script as it goes, so the change corrupts the run in progress.

## The scripts

| Script | Role | Run by |
|---|---|---|
| [`run-audit.sh`](run-audit.sh) | The whole audit in one command: org pass, project passes, rollup, score, filed into one dated directory. Reads [`config/audit.env`](config/readme.md) for the organization and the check inputs, so it needs no flags but a target | Auditor |
| [`audit-run.go`](audit-run.go) | The check engine. Reads the checks straight from [`docs/cis-ig1-cli-validation.md`](docs/cis-ig1-cli-validation.md) — the document is the single source, with no second copy to drift — runs them in parallel, scores each one, and writes a pack of results. `-list` shows what would run, `-only` runs a subset, `-init-config` writes a fresh `config/audit.env` | Auditor, directly or through `run-audit.sh` |
| [`rollup.go`](rollup.go) | Reads every pack and pivots on the finding rather than the project: one org policy fix is one row, not 105. Also writes the compliance score, as markdown and as JSON. Point `-in` at one run directory for that project's score, or at the directory holding every run for the organization's | `run-audit.sh`, or by hand |
| [`compliance-report.go`](compliance-report.go) | Scores the **checklist**, not a run: not started, PR submitted (and how long it has waited for approval), compliant. `--update` syncs Status lines to the checkboxes. Used while remediation is under way, after the audit | Auditor, as remediation progresses |
| [`gcloud/create.sh`](gcloud/create.sh) | Creates the audit service account, its three custom roles and its org bindings without Terraform. Writes `audit-sa-record.txt`, the record teardown needs | Admin |
| [`gcloud/verify.sh`](gcloud/verify.sh) | Confirms the identity is exactly as intended — no keys, no write verbs, the expected bindings — and after teardown, that nothing remains | Admin |
| [`gcloud/destroy.sh`](gcloud/destroy.sh) | Removes everything `create.sh` made, from its record | Admin |
| [`terraform/audit-service-account/`](terraform/audit-service-account/readme.md) | The Terraform module for the same service account: `apply` to create it, `destroy` to remove every trace | Admin |

## Documents

| | |
|---|---|
| [`docs/cis-ig1-auditor-setup.md`](docs/cis-ig1-auditor-setup.md) | **First.** Sign in, branch, impersonate, output directory; one-time Cloud Shell setup. |
| [`config/readme.md`](config/readme.md) | The settings: what goes in `audit.env` and `projects.txt`, and what is committed. |
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
| [`docs/cis-ig1-required-apis.md`](docs/cis-ig1-required-apis.md) | The 17 APIs the audit host project needs, and why each one. |

## The numbers

| | |
|---|---|
| IG1 safeguards | 56 |
| GCP-actionable | 44 (the other 12 are training, endpoints, removable media) |
| Checklist requirements | 290 |
| With a CLI check | 188 · **96 auto-scored**, 92 need a human to read the output |
| Manual | 102 · 30 console tasks, 72 process and documentation |

100% here is the GCP half of IG1, not IG1. Say so when reporting.

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

Each file carries `//go:build ignore` so several `package main` files can share a directory without colliding under `go vet ./...`.

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This is an implementation aid, not affiliated with or endorsed by CIS.*
