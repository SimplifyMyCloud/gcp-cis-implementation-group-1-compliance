# Auditing one project — checklist

For a project in the estate that has not been audited yet, in an engagement
already under way.

**Assumes:** you are signed in and impersonating the audit service account,
on the engagement branch and up to date, with `config/audit.env` filled in,
and the organization pass already run and reviewed. If any of that is not
true, start at [auditor setup](cis-ig1-auditor-setup.md).

---

## 1. Run it

```bash
./run-audit.sh --no-org --project THEIR_PROJECT
```

- [ ] `--no-org` is present. The organization pass is already done; running it
      again produces a second set of REVIEW decisions for identical questions,
      with nothing to say which is authoritative.
- [ ] `RUN STATUS: OK`. **DEGRADED** or **UNRELIABLE** means stop — a run that
      hit permission or execution failures is not a set of findings, and
      treating it as one invents work that may not exist.
- [ ] Note the run directory from the `Next:` line it prints.

## 2. Review to 100%

```bash
./run-audit.sh --review audit-state/runs/<timestamp>
```

- [ ] Every **REVIEW** decided `p` or `f`.
- [ ] Every **ERROR**, **DENIED** and **SKIP** decided too. Verify the
      requirement another way first — console, a narrower command, the
      customer — then record the verdict. The report keeps the machine's
      failure and its stderr underneath, so the evidence chain stays honest.
- [ ] The closing line reads **completion 100%**.

`s` defers one for this session, `q` saves and exits. Running it again resumes
where you stopped; nothing is re-run.

## 3. Confirm before committing

```bash
grep -m1 -o 'SCORE: [^*]*' audit-state/runs/<timestamp>/report/03-projects/THEIR_PROJECT.md
```

- [ ] `completion 100%`.

Pass and fail are the result. **Completion is the gate** — below 100% the
project counts towards nothing, because an unreconciled REVIEW or an
unresolved ERROR is not a result.

## 4. Commit

```bash
git pull
```

```bash
git add audit-state/runs/<timestamp> && git commit -m "Audit: THEIR_PROJECT" && git push
```

- [ ] Named the path. **Never `git add -A`** — that is how a live IAM inventory
      reached a public repository twice.
- [ ] Pulled first. Several auditors share this branch; each run writes its own
      directory, so there is nothing to merge, but pushes still race.

## 5. Recompile the reports

```bash
./run-audit.sh --compile
```

- [ ] Run after each project, or once at the end of a batch. The three headline
      numbers print when it finishes.

```bash
git add audit-state/*.md audit-state/*.csv audit-state/*.json && git commit -m "Recompile reports" && git push
```

---

## If the project is new to the estate

```bash
echo THEIR_PROJECT >> config/projects.txt && git add config/projects.txt && git commit -m "Estate: add THEIR_PROJECT" && git push
```

`config/projects.txt` is the denominator for coverage. A project missing from
it is invisible to the percentage even after you have audited it.

## Where a failure goes next

A violation is fixed in the development project, tested, promoted to
production, and both are re-audited — see
[Running with a team](cis-ig1-audit-runbook.md#running-with-a-team) and
[the remediation plan](cis-ig1-audit-runbook.md#phase-6b--compile-the-plan).

`audit-state/safeguards.md` names the projects holding each failing safeguard
back, so a safeguard waiting only on a development project is a fix in flight,
and the same safeguard waiting on production is live exposure.
