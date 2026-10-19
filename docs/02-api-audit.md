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
./run-audit.sh
```

No arguments. Output lands in `audit-state/runs/<timestamp>/`.

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

Note the run directory from the `Next:` line.

---

## 2. Project passes

One project:

```bash
./run-audit.sh --no-org --project PROJECT_ID
```

All projects in the estate list:

```bash
./run-audit.sh --no-org --projects config/projects.txt
```

A batch:

```bash
split -l 9 config/projects.txt config/batch- && ./run-audit.sh --no-org --projects config/batch-aa
```

**`--no-org` is mandatory after the organization pass.** Without it you get a second org pass and a second set of REVIEW decisions for questions already answered, with nothing to say which is authoritative.

---

## 3. Long runs

Cloud Shell terminates on session end and reclaims the VM. `nohup`, `disown` and `tmux` do not survive it.

```bash
tmux new -s audit
```

Start the run inside tmux — survives an SSH drop, not a session end. Keep the workstation awake. Batch so each run completes in a sitting.

**Results are filed into `report/` only after the last project.** Ctrl-C mid-run leaves them unfiled under `<run>/.packs/`:

```bash
RUN=audit-state/runs/<timestamp>
mkdir -p "$RUN/report/03-projects" "$RUN/evidence/results"
for d in "$RUN"/.packs/projects/*/; do p=$(basename "$d"); [ -f "$d/01-automated-results.md" ] && mv "$d/01-automated-results.md" "$RUN/report/03-projects/$p.md"; [ -f "$d/results.json" ] && mv "$d/results.json" "$RUN/evidence/results/$p.json"; done
```

---

## 4. Commit

```bash
git pull && git add audit-state/runs/<timestamp> && git commit -m "API audit: <target>" && git push
```

Name the path. Never `git add -A`.

---

## Output

```
audit-state/runs/<timestamp>/
  report/
    01-remediation-plan.md
    02-organization/01-automated-results.md
    03-projects/<project-id>.md
    04-compliance-score.md  04-compliance-score.json
    remediation-plan.csv
  evidence/
    results/<pass>.json          read by Phase 2
    audit.env  targets.txt  run.log  iam-inventory.txt  excluded.txt
```

Every pass finishes below 100% completion. REVIEW checks are undecided by design — that is [Phase 2](03-review-audit.md).

---

## Options

| Flag | Use |
|---|---|
| `--no-org` | Skip the organization pass |
| `--project ID` | One project. Repeatable |
| `--projects FILE` | One ID per line |
| `--all` | Every ACTIVE project in the org |
| `--skip V96` | Omit a check that hangs. Reported as SKIP |
| `--parallel N` | Checks at once. Default 8 |
| `--out DIR` | Default `./audit-state/runs` |
