# Required APIs and IAM roles — CIS IG1 audit

Discovered by running the audit end to end against a test organization. Every
entry was **proven by a real error**, not inferred from the docs.

| | |
|---|---|
| Test organization | `933250405420` (simplifymy.cloud) |
| Audit host project | `simplifymycloud-dev` — owns the audit SA, APIs are enabled here |
| Project-scope target | `iq9-gcp-dev-yamato` |
| Audit identity | `cis-ig1-auditor@simplifymycloud-dev.iam.gserviceaccount.com` (impersonated) |
| Started | 2026-09-13 |

## APIs to enable in the audit host project

```bash
gcloud services enable --project=simplifymycloud-dev \
  # (filled in as APIs are confirmed)
```

| # | API | Needed by (phase / V-number) | Error that revealed it | Date |
|---|---|---|---|---|

## APIs that must be enabled in the audited project

Some calls are billed to, and gated by, the project that owns the resource
rather than the audit host project. Those are listed here separately.

| # | API | Needed by (V-number) | Error that revealed it | Date |
|---|---|---|---|---|

## IAM roles or permissions missing from the audit service account

| # | Role / permission | Needed by (V-number) | Error that revealed it | Fix applied | Date |
|---|---|---|---|---|---|
