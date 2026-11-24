# Phase 1 — API audit

Automated. 188 checks run by `gcloud`; each returns PASS, FAIL, N/A, REVIEW, SKIP, ERROR or DENIED.

Prerequisite: [Phase 0](01-setup.md) complete, tokeninfo verified **this session**.

---

## Pass sizes

| Pass | Checks | Run |
|---|---|---|
| Organization | 86 | Once per engagement |
| Project | 103 | Once per project |

V112 appears in both. Each report lists all 188 in order; the other pass's checks show as `ORG` or `PROJECT`.

---

## 1. Organization pass

```bash
./run-audit.sh --gcp-org
```

Output lands in `audits/gcp-org/`. **With no arguments the script prints its options and exits** — it audits nothing by accident.

Abort conditions, printed in the header:

| Header line | Action |
|---|---|
| `identity  <you> (NOT impersonating)` | Ctrl-C. Repeat Phase 0 step 3 |
| `identity  <wrong SA>` | Ctrl-C. Fix `SA_EMAIL` |
| `organization  <wrong ID>` | Ctrl-C. Fix `ORG_ID` |

At the end:

| | Action |
|---|---|
| `RUN STATUS: OK` | Proceed |
| `OK with errors` | Proceed; errors are decidable in Phase 2 |
| `DEGRADED` / `UNRELIABLE` | **Discard the run.** Results are not findings |

The `Next:` line names the review command.

---

## 2. Project passes

One project:

```bash
./run-audit.sh --project PROJECT_ID
```

All projects in the estate list:

```bash
./run-audit.sh --projects config/projects.txt
```

A batch:

```bash
split -l 9 config/projects.txt config/batch- && ./run-audit.sh --projects config/batch-aa
```

Nothing runs unless you name it: `--gcp-org`, `--project`, `--projects` or `--all`. There is no default, so a project run cannot drag in a second organization pass.

---

## 3. Long runs

Cloud Shell terminates on session end and reclaims the VM. `nohup`, `disown` and `tmux` do not survive it.

```bash
tmux new -s audit
```

Start the run inside tmux — survives an SSH drop, not a session end. Keep the workstation awake. Batch so each run completes in a sitting.

**Results are filed only after the last project.** A run stages its output in a temporary directory and files it per target at the end, so Ctrl-C mid-run loses the lot. Batch long runs rather than interrupting them.

---

## 4. Commit

```bash
git pull && git add audits && git commit -m "API audit: <target>" && git push
```

Name the path. Never `git add -A`.

---

## Output

One directory per target, named for what was audited. A re-audit replaces it: the newest result is the only one that counts, and git holds what came before.

```
audits/
  gcp-org/
    report/02-organization/01-automated-results.md
    report/02-organization/02-manual-gcp-tasks.md
    report/02-organization/03-manual-process.md
    evidence/results/organization.json          read by Phase 2
    evidence/run.log  audit.env  targets.txt  iam-inventory.txt
  gcp-projects/<project-id>/
    report/03-projects/<project-id>.md
    evidence/results/<project-id>.json          read by Phase 2
    evidence/run.log  audit.env  targets.txt
```

`--compile` builds a report set per side — `reports/gcp-org/` and `reports/gcp-projects/` — see [Phase 3](04-report-rollup.md).

Every pass finishes below 100% completion. REVIEW checks are undecided by design — that is [Phase 2](03-review-audit.md). **A target below 100% appears in no report.**

---

## Options

| Flag | Use |
|---|---|
| `--gcp-org` | The organization pass, 86 checks |
| `--project ID` | One project. Repeatable |
| `--projects FILE` | One ID per line |
| `--all` | Every ACTIVE project in the org |
| `--skip V96` | Omit a check that hangs. Reported as SKIP |
| `--parallel N` | Checks at once. Default 8 |
| `--out DIR` | Default `./audits`. Org goes to `<out>/gcp-org/`, projects to `<out>/gcp-projects/<id>/` |
