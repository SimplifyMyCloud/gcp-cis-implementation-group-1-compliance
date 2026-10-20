# Phase 3 — Report rollup

Compiles every run into one set of reports. Reads what is on disk; calls nothing.

---

## Preflight

```bash
./run-audit.sh --incomplete
```

**Anything listed there is in no report** — not its score, not its findings. Clear it with `--review` first, or accept that it is absent.

```bash
test -f audits/manual-answers.json && echo "interview done"
```

Without the interview only 5 of 44 safeguards can be scored.

---

## Compile

```bash
./run-audit.sh --compile
```

---

## Output

Six files in `reports/`, beside `audits/`. The audits are the evidence; the reports are what you hand over.

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
| PROJECTS audited | Targets present in `audits/` at 100% | — |
| PROJECTS passing | Targets present in `audits/` at 100% with zero FAIL | — |
| ORGANIZATION | 27 safeguards the org pass judges. **Absent if the org pass is below 100%** | No |
| PROJECTS BY SAFEGUARD | 35 safeguards a project pass judges, averaged over finished audits | No |

Each target is scored against the safeguards **its own pass judges**. A project pass reaches 35 of 56, the org pass 27. Seven safeguards have no project-scope check; twelve have no GCP surface. Do not score a target out of 56.

---

## Reading the safeguard report

```bash
sed -n '/## By target/,/^Mean/p' reports/safeguards.md
```

```bash
sed -n '/## What is blocking/,$p' reports/safeguards.md
```

Blocking entries name the projects:

```
**4.6 Securely Manage Enterprise Assets** — FAIL
- project-wide SSH keys removed · `V67` — fails in **acme-dev-web**
- IAP TCP forwarding used for SSH/RDP · `V72` — fails in **acme-dev-web**, **acme-prod-web**
```

Dev projects only → fix in flight. Production named → live exposure. Both → mid-promotion.

---

## Coverage against a wider estate

`--compile` counts the targets in `audits/`, so the project percentage is of what you have audited. To report against the whole estate instead — "16 of 44" — pass the list by hand:

```bash
go run compliance-report.go -runs audits -answers audits/manual-answers.json -projects config/projects.txt -safeguards-md reports/safeguards.md -safeguards-json reports/safeguards.json
```

Keep that list complete. A stale one divides audited projects by the wrong total and reports percentages above 100%.

## Production-only figures

Third denominator, same audits:

```bash
go run compliance-report.go -runs audits -answers audits/manual-answers.json -projects config/production.txt -safeguards-md reports/safeguards-production.md -safeguards-json reports/safeguards-production.json
```

---

## Commit

```bash
git pull && git add audits reports config && git commit -m "Rollup $(date +%Y-%m-%d)" && git push
```

---

## Reporting rules

- **CIS sets no pass mark.** IG1 is "implement every safeguard". A percentage is progress, not a compliance claim.
- **56 is the honest denominator** only because the 12 off-platform safeguards are answered in the interview. The 44 GCP-actionable subset is also reported.
- **Audited ≠ passing.** Audited is coverage; passing is audited with zero FAIL.
- **Unanswered is not failing.** Unscoreable safeguards are listed by name.
- **An unfinished audit is not a result.** A target below 100% is excluded from every report, and named at the top of each one so a number cannot be quoted without knowing what was left out.
- **Label the two 44s.** 44 projects and 44 GCP-actionable safeguards are different numbers.
