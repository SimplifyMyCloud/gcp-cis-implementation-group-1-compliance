# config

Every setting the audit needs. Nothing else lives here.

| | |
|---|---|
| `audit.env` | The organization, the audit identity, the check inputs |
| `projects.txt` | The estate — the coverage denominator |
| `*.example` | Starting points. Committed |

```bash
cp config/audit.env.example config/audit.env
cp config/projects.txt.example config/projects.txt
```

Both filled-in files are committed: one shared settings file is what makes several auditors' results comparable. Neither holds a credential.

## audit.env

| Key | Read by | Notes |
|---|---|---|
| `ORG_ID` | `run-audit.sh`, `audit-run.go` | Numeric, not the domain |
| `AUDIT_PROJECT` | Setup steps | **The project with the 17 APIs enabled.** Not necessarily the SA's own project |
| `SA_EMAIL` | Setup steps | The identity impersonated |
| `AUDIT_BRANCH` | Setup steps | `<customer>-gcp-ig1-audit`. Per engagement, not per auditor |
| `APPROVED_REGISTRIES` | V20, V22 | **Required.** Blank → both SKIP → the pass cannot reach 100% |
| `EXCLUDE_PROJECTS` | Target resolution | Regex on project ID. Default `^sys-`. `none` disables |
| `ALLOWED_LOCATIONS` | Residency checks | Optional. Defaults to continental US; setting it **replaces** that list |

`--org` beats `$ORG_ID` beats `ORG_ID` in the file.

`AUDIT_PROJECT` is read by the setup steps, **not by the audit**. The audit uses gcloud's active project. Set both.

## projects.txt

One ID per line, `#` comments ignored. It selects what to audit:

```bash
./run-audit.sh --no-org --projects config/projects.txt
```

`--compile` does **not** read it. The reports count the targets present in
`audits/`, so a stale list cannot skew a percentage. To report coverage against
the whole estate, pass it to the Go tools by hand — see
[Phase 3](../docs/04-report-rollup.md).

## none is an answer

A resource that does not exist cannot meet a requirement. `none` makes the dependent checks FAIL — a result. Blank makes them SKIP, which holds the pass below 100%.

Never put a credential here. The audit runs by impersonation precisely so no key exists.
