# docs

## Phases — run in order

| | | |
|---|---|---|
| 0 | [`01-setup.md`](01-setup.md) | Cloud Shell, settings, impersonation, smoke test |
| 1 | [`02-api-audit.md`](02-api-audit.md) | The 188 automated checks |
| 2 | [`03-review-audit.md`](03-review-audit.md) | `--review` per run, `--interview` once per engagement |
| 3 | [`04-report-rollup.md`](04-report-rollup.md) | Compile every run into the five headline numbers |

## Reference

| | |
|---|---|
| [`cis-ig1-cli-validation.md`](cis-ig1-cli-validation.md) | All 188 checks: command, pass criterion, requirement |
| [`cis-ig1-gcp-checklist.md`](cis-ig1-gcp-checklist.md) | All 290 requirements, grouped by safeguard |
| [`cis-ig1-remediation-reference.md`](cis-ig1-remediation-reference.md) | Example Terraform per finding |
| [`cis-ig1-required-apis.md`](cis-ig1-required-apis.md) | The 17 APIs the quota project needs, with evidence |
| [`known-issues.md`](known-issues.md) | Defects found in the field, with workarounds |
| `cis-ig1-tracker.xlsx` | Effort tracker. Imports into Sheets |

## Do not restructure these two

`audit-run.go` and `compliance-report.go` parse them at runtime:

```
audit-run.go          -doc    docs/cis-ig1-cli-validation.md
compliance-report.go  -file   docs/cis-ig1-gcp-checklist.md
```

Their heading levels, table shapes and `V`-number links are the parser's input. Edit the content; leave the structure alone, or the checks stop being found.
