# Documentation

| I want to… | Go to |
|---|---|
| Run an audit end to end | [Runbook](cis-ig1-audit-runbook.md) |
| Just run the scripts | [Scripted audit](cis-ig1-scripted-audit.md) |
| Understand what IG1 is | [Overview](cis-ig1-overview.md) |
| Record findings | [Checklist](cis-ig1-gcp-checklist.md) |
| Run the check that proves an item | [CLI validation](cis-ig1-cli-validation.md) |
| Fix something that failed | [Remediation reference](cis-ig1-remediation-reference.md) |
| Track the engineering effort | [Tracker](cis-ig1-tracker.xlsx) |
| Teach the team | [Training](training/) |
| See what benchmark work counts toward IG1 | [Benchmark contribution](cis-ig1-benchmark-overlap.md) |

## How the checklist is marked

Every one of the 290 requirements carries a marker:

| | Group | Count | How |
|---|---|---|---|
| ⚙️ | Automated | 41 | `audit-run.go` scores it |
| 🔍 | CLI + human | 147 | Script runs it, you judge the output |
| 🖥️ | GCP manual | 30 | Console, image build, or a test you perform |
| 👥 | Process & people | 72 | A conversation and a document |

The last two are the 102 no script touches. 👥 is the group to take into a room with people who know the account — [`training/09-process-interview.md`](training/09-process-interview.md) turns them into questions.

## Scope

IG1 is enterprise-wide. Of its 56 safeguards, **44** are GCP-actionable and appear here; **12** have no GCP surface (training, end-user devices, removable media) and are owned elsewhere — [Appendix A](cis-ig1-gcp-checklist.md#appendix-a--safeguards-excluded-from-this-checklist).

All 290 requirements here are yours, including the 102 manual ones — each concerns the GCP estate.
