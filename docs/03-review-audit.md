# Phase 2 — Review audit

Human in the loop. Two separate sessions:

| | Decides | Count | Scope |
|---|---|---|---|
| `--review` | Checks the machine ran but cannot judge, and checks it could not answer | ~45 per pass | Per run |
| `--interview` | Requirements no command can answer | 114 | Once per engagement |

A pass reaches 100% completion only when `--review` leaves nothing undecided. **A pass below 100% counts towards no statistic.**

---

## 1. Review a run

```bash
./run-audit.sh --review audit-state/runs/<timestamp>
```

Walks every undecided check, organization pass first, then each project. Non-cross-reference checks in V order, then cross-references.

| Key | Action |
|---|---|
| `p` | PASS |
| `f` | FAIL |
| `s` | Defer for this session only |
| `v` | Page the full output |
| `q` | Save and exit |

Saved after every answer. Resumable — rerun the same command.

### Verdicts offered

| Machine verdict | Meaning | Before deciding |
|---|---|---|
| `REVIEW` | Ran clean, needs judgement | Read the output against `Pass if:` |
| `ERROR` / `DENIED` / `SKIP` | Could not answer | **Verify the requirement another way first** — console, a narrower command, the customer |

The prompt prints the failure above the criterion for blocked checks. The report records both verdicts:

```
**Why PASS:** decided by the auditor on review — you, timestamp.
The machine's verdict was ERROR: <stderr>
```

Cross-references resolve from their referents. Decide the referents and most resolve themselves.

### Confirm

```bash
grep -m1 -o 'SCORE: [^*]*' audit-state/runs/<timestamp>/report/03-projects/PROJECT_ID.md
```

`completion 100%` is the gate. Pass and fail are the result.

---

## 2. The interview

Once per engagement, organization-wide.

```bash
./run-audit.sh --interview
```

114 questions: 102 manual requirements from the checklist, then the 12 IG1 safeguards with no GCP surface (endpoint management, then Control 14 awareness training — grouped last, with their owner shown).

| Key | Action |
|---|---|
| `y` | Satisfied |
| `n` | Not satisfied |
| `s` | Defer for this session only |
| `q` | Save and exit |

Saved to `audit-state/manual-answers.json` after every answer. Resumable.

No evidence is captured — attach it to the engagement record separately.

**Without this, only 5 of 44 GCP-actionable safeguards can be scored.** The other 39 each hold at least one requirement no command can answer.

---

## 3. Commit

```bash
git pull && git add audit-state && git commit -m "Review: <target>" && git push
```

---

## Rules

- **Unanswered is not failing.** An undecided check leaves its safeguard unscoreable, named in the report, never counted against the customer.
- **A requirement failing in any project fails the safeguard** estate-wide. Per-project verdicts stay visible.
- **Only 100%-complete passes count** towards coverage, the safeguard numbers or the averages.
- **Re-audit after remediation.** A promoted fix moves nothing until that project's new pass reaches 100%.
