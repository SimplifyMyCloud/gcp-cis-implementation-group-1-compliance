# Settings

Everything the audit needs you to decide lives here, and nothing else does.

Output goes elsewhere — `scratch/runs/` by default — so clearing a directory
full of old runs never costs you your configuration. That separation is the
point of this directory.

## Start an engagement

```bash
cp config/audit.env.example config/audit.env
cp config/projects.txt.example config/projects.txt   # optional
```

Then open `config/audit.env` and fill it in. That is the whole setup:

```bash
./run-audit.sh --project some-project-id
```

No `--config`, no `--org`, no exported variables. `run-audit.sh` reads
`config/audit.env` unless you point `--config` somewhere else.

## What goes in `audit.env`

| Setting | Read by | Notes |
|---|---|---|
| `ORG_ID` | `run-audit.sh`, `audit-run.go` | The numeric ID, not the domain |
| `AUDIT_PROJECT` | The setup steps | Owns the service account, has the audit APIs enabled |
| `SA_EMAIL` | The setup steps | The identity the auditor impersonates |
| `APPROVED_REGISTRIES` | V20, V22 | **Required.** Empty means both SKIP, which holds the audit below 100% complete |
| `EXCLUDE_PROJECTS` | Target resolution | Regex on the project ID; defaults to `^sys-` |
| `ALLOWED_LOCATIONS` | The residency checks | Optional; defaults to the continental US, and setting it **replaces** that list rather than extending it |

`--org` beats `$ORG_ID`, which beats `ORG_ID` in this file.

Two rules worth stating plainly:

- **`none` is an answer.** If the customer has no approved registry list, no
  backup project, no anything — write `none`. The dependent checks become
  findings rather than skips, because a resource that does not exist cannot
  meet the requirement, and a SKIP holds the audit below 100%.
- **These are policy answers, not observations.** Ask the customer what their
  approved registries are; do not read them off what happens to be deployed.
  Inferring policy from the estate makes every estate compliant with itself.

## What goes in `projects.txt`

One project ID per line; `#` comments and blank lines ignored. It serves two
purposes, and the second one catches people out:

```bash
./run-audit.sh --projects config/projects.txt      # audit every one
go run rollup.go -projects config/projects.txt     # the coverage denominator
```

For the organization score this list is the **estate**, not the backlog — it
is what "12 of 40 projects audited" divides by. Keep it complete even while
you work through it one project at a time. Generate it from the live
organization:

```bash
gcloud projects list --filter='lifecycleState:ACTIVE' --format='value(projectId)' > config/projects.txt
```

## What is committed, and what is not

Only the `.example` files and this readme. `config/audit.env` and
`config/projects.txt` are gitignored: they hold the customer's project names,
registry paths and residency policy, which are theirs to publish rather than
yours.

Nothing in this repository should ever hold a credential. The audit runs by
impersonation precisely so that no service account key exists — committing one
would break safeguard 5.2, which this repository exists to test.
