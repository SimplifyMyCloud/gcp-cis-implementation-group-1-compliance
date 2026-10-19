# terraform

Infrastructure for the audit itself. Nothing here audits anything.

| | |
|---|---|
| [`audit-service-account/`](audit-service-account/readme.md) | The read-only audit identity: service account, custom roles, org bindings |

Run once per engagement, by someone with organization IAM admin. Destroy at the end — a standing org-wide read identity fails safeguards 5.1, 5.4 and 6.2.

State is gitignored. It is the teardown record; keep it somewhere you will still have at the end of the engagement.

Without Terraform, [`gcloud/`](../gcloud/readme.md) creates the same identity by shell script.
