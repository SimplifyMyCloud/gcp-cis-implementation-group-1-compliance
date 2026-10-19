# Phase 3 — Report rollup

Compiles every run into one set of reports. Reads what is on disk; calls nothing.

---

## Preflight

| | Check |
|---|---|
| Every pass at 100% | `grep -o 'SCORE: [^*]*' audit-state/runs/*/report/*/*.md audit-state/runs/*/report/*.md 2>/dev/null \| grep -c 'completion 100%'` |
| Interview done | `test -f audit-state/manual-answers.json && echo yes` |
| Estate list current | `wc -l < config/projects.txt` |
| Results readable | `ls audit-state/runs/*/evidence/results/*.json \| wc -l` |

A pass below 100% is excluded from every total and named in the output. The interview is required for the safeguard figures.

---

## Compile

```bash
./run-audit.sh --compile
```

---

## Output

Six files in `audit-state/`:

| File | Contents |
|---|---|
| `remediation-plan.md` | One row per finding, with affected projects. Ranked by spread |
| `remediation-plan.csv` | Same, for the tracker |
| `compliance-score.md` | Per-target completion / pass / fail, and coverage |
| `compliance-score.json` | Same, for the dashboard |
| `safeguards.md` | Which of the 56 pass, and what blocks the rest |
| `safeguards.json` | Same, for the dashboard |

---

## The five numbers

Printed on completion:

```
SCORE: completion x% · pass y% · fail z%
SAFEGUARDS: x% — n of 56 passing
PROJECTS: x% audited (n of 44) · y% passing (n of 44)
ORGANIZATION: x% compliant
PROJECTS, BY SAFEGUARD: x% compliant on average
```

| Number | Denominator | Includes manual |
|---|---|---|
| SAFEGUARDS | 56 IG1 safeguards | Yes |
| PROJECTS audited | `config/projects.txt` — passes at 100% | — |
| PROJECTS passing | `config/projects.txt` — at 100% with zero FAIL | — |
| ORGANIZATION | 27 safeguards the org pass judges | No |
| PROJECTS BY SAFEGUARD | 35 safeguards a project pass judges, averaged over finished audits | No |

Each target is scored against the safeguards **its own pass judges**. A project pass reaches 35 of 56, the org pass 27. Seven safeguards have no project-scope check; twelve have no GCP surface. Do not score a target out of 56.

---

## Reading the safeguard report

```bash
sed -n '/## By target/,/^Mean/p' audit-state/safeguards.md
```

```bash
sed -n '/## What is blocking/,$p' audit-state/safeguards.md
```

Blocking entries name the projects:

```
**4.6 Securely Manage Enterprise Assets** — FAIL
- project-wide SSH keys removed · `V67` — fails in **acme-dev-web**
- IAP TCP forwarding used for SSH/RDP · `V72` — fails in **acme-dev-web**, **acme-prod-web**
```

Dev projects only → fix in flight. Production named → live exposure. Both → mid-promotion.

---

## Production-only figures

Second denominator, same runs:

```bash
go run compliance-report.go -runs audit-state/runs -answers audit-state/manual-answers.json -projects config/production.txt -safeguards-md audit-state/safeguards-production.md -safeguards-json audit-state/safeguards-production.json
```

---

## Commit

```bash
git pull && git add audit-state config && git commit -m "Rollup $(date +%Y-%m-%d)" && git push
```

---

## Reporting rules

- **CIS sets no pass mark.** IG1 is "implement every safeguard". A percentage is progress, not a compliance claim.
- **56 is the honest denominator** only because the 12 off-platform safeguards are answered in the interview. The 44 GCP-actionable subset is also reported.
- **Audited ≠ passing.** Audited is coverage; passing is audited with zero FAIL.
- **Unanswered is not failing.** Unscoreable safeguards are listed by name.
- **Label the two 44s.** 44 projects and 44 GCP-actionable safeguards are different numbers.
