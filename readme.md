# CIS Controls v8.1 IG1 — GCP audit kit

Audits a Google Cloud organization against **CIS Controls v8.1 Implementation Group 1**: 56 safeguards, of which 44 are GCP-actionable across 290 requirements.

188 of those requirements have a `gcloud` command that answers them. The kit runs all 188 read-only as an impersonated service account holding no keys, puts the ones a machine cannot judge to the auditor, takes the 102 that no command can answer as an interview, and compiles the result into a score per safeguard, per project and per organization.

```bash
./run-audit.sh --gcp-org               # organization pass, 86 checks
./run-audit.sh --project ID            # project pass, 103 checks
./run-audit.sh --incomplete            # what is below 100%, and in no report
./run-audit.sh --review TARGET         # decide what the machine could not
./run-audit.sh --interview             # the 114 a command cannot answer
./run-audit.sh --compile               # two report feeds: org and projects
```

Run it bare and it prints the options rather than auditing anything.

## Start here

| | |
|---|---|
| **Auditing an estate** | [docs/01-setup.md](docs/01-setup.md), then the three phases below. Allow an hour for access |
| **Pitching or scoping one** | [delivery/](delivery/readme.md) — one-pagers and decks for a customer CISO and for fellow architects |
| **Changing the kit** | [docs/readme.md](docs/readme.md). Two of those documents are parsed at runtime; do not restructure them |

You need Cloud Shell or any machine with `gcloud` and Go, and a read-only service account you can impersonate. Nothing else — no modules, no build, no license.

## How it works

The 188 checks are not stored in code. `audit-run.go` parses them out of [`docs/cis-ig1-cli-validation.md`](docs/cis-ig1-cli-validation.md) at runtime — the document is the single source, so there is no second copy to drift. Each check carries its command, its pass criterion and the requirement it answers.

A check ends in one of seven verdicts. PASS, FAIL and N/A are final. REVIEW means it ran clean and needs a human. SKIP, ERROR and DENIED mean it could not answer; those are decidable by the auditor too, with the failure recorded beneath the verdict.

A pass is complete when nothing is undecided. **A target below 100% appears in no report** — not its score, not its findings. An unfinished audit is work outstanding, not a result, and `--incomplete` is how you find it.

Results are filed by what was audited, not when: `audits/gcp-org/` and `audits/gcp-projects/<project-id>/`, one directory each, replaced by a re-audit. `--compile` builds a report set for each side into `reports/gcp-org/` and `reports/gcp-projects/` — two feeds a dashboard reads with one schema. `audits/` is the evidence, `reports/` is what you hand over.

A safeguard passes when every requirement under it passes, including the manual ones. A requirement failing in any audited project fails the safeguard estate-wide.

## The three phases

| | | |
|---|---|---|
| 0 | [Setup](docs/01-setup.md) | Cloud Shell, settings, impersonation, smoke test |
| 1 | [API audit](docs/02-api-audit.md) | The 188 automated checks |
| 2 | [Review audit](docs/03-review-audit.md) | `--review` per run, `--interview` once |
| 3 | [Report rollup](docs/04-report-rollup.md) | Five headline numbers, six files |

## Scripts

| | |
|---|---|
| [`run-audit.sh`](run-audit.sh) | Every phase. The only command an auditor needs |
| [`audit-run.go`](audit-run.go) | Check engine. Parses the validation doc, runs checks in parallel, scores them, drives `--review` |
| [`rollup.go`](rollup.go) | Pivots findings by requirement rather than project; writes the compliance score |
| [`compliance-report.go`](compliance-report.go) | The interview, and the safeguard roll-up across runs |
| [`terraform/audit-service-account/`](terraform/audit-service-account/readme.md) | The read-only audit identity |
| [`gcloud/`](gcloud/readme.md) | The same identity without Terraform |

Standard library only. No `go.mod`, no dependencies, no build step — `go run <file>.go` on any machine with Go. Deliberate: these run inside a customer's environment, where "install these modules first" is not a conversation worth having.

## Reference

| | |
|---|---|
| [`docs/cis-ig1-cli-validation.md`](docs/cis-ig1-cli-validation.md) | All 188 checks, commands and pass criteria. Parsed at runtime |
| [`docs/cis-ig1-gcp-checklist.md`](docs/cis-ig1-gcp-checklist.md) | All 290 requirements by safeguard. Parsed at runtime |
| [`docs/cis-ig1-remediation-reference.md`](docs/cis-ig1-remediation-reference.md) | Example Terraform per finding |
| [`docs/cis-ig1-required-apis.md`](docs/cis-ig1-required-apis.md) | The 17 APIs the quota project needs |
| [`docs/known-issues.md`](docs/known-issues.md) | Defects found in the field, with workarounds |
| [`docs/readme.md`](docs/readme.md) | What every document is for, and which two the parsers read |
| [`config/`](config/readme.md) | Every setting, and nothing else |
| [`delivery/`](delivery/readme.md) | One-pagers and decks for pitching and handing over an engagement |

## CIS

| | |
|---|---|
| [CIS Controls v8.1](https://www.cisecurity.org/controls/v8-1) | The framework |
| [Implementation Groups](https://www.cisecurity.org/controls/implementation-groups) | Why IG1 is the baseline |
| [CIS Benchmarks — GCP](https://www.cisecurity.org/benchmark/google_cloud_computing_platform) | Configuration benchmarks. Distinct from the Controls |

CIS sets no pass mark. IG1 is "implement every safeguard", so a percentage is progress against that, not a compliance claim.

## Teardown

Destroy the audit identity when the engagement ends. A standing org-wide read fails safeguards 5.1, 5.4 and 6.2 — the controls this kit measures.
