# Known issues

Things the kit gets wrong, found in live engagements, with the workaround that
gets an audit finished in the meantime.

## EXCLUDE_PROJECTS does not apply to the organization pass

**Found:** 2026-09-23, customer organization with 2,500+ Apps Script `sys-*` projects.

**Symptom:** the organization pass returns ERROR on the checks that enumerate
the estate — quota exhaustion. Setting `EXCLUDE_PROJECTS=^sys-` does not help.

**Cause:** `excludePattern()` is consulted in exactly one place, the guard that
rejects a single `-project` target. No organization-scope check filters by it.
The setting stops an excluded project getting its own project pass; it does
nothing about the org-wide queries, which see every project in the estate.

**Scale of the problem:** seven organization checks issue the same Cloud Asset
Inventory query independently —

```
gcloud asset search-all-resources --scope=organizations/$ORG_ID \
  --asset-types=compute.googleapis.com/Instance --read-mask='*' --format=json
```

V15, V66, V68, V69, V76, V81 and V150. V66 issues it twice. Each pulls every
VM in the organization with the full resource body, and gcloud pages through
the result. Against 2,500 projects that is enough to exhaust the Asset
Inventory search quota, and two cross-references (V82, V112) inherit the
resulting ERROR.

**Workaround:** run the query once, save the JSON, and run each check's `jq`
against the saved file. Same data and same logic, one API call rather than
eight.

**Fix, when there is time:** two parts, and they are separable.

1. Filter server-side in the Asset Inventory query so excluded projects never
   enter the result set. This is the one that addresses quota rather than
   hiding it. Verify the predicate against a live organization first.
2. Make `EXCLUDE_PROJECTS` apply to organization-scope commands, so the
   setting means what its name says.

Leave V3 enumerating everything — catching projects nobody knew about is its
entire purpose — but report excluded projects separately rather than as
findings.

## An ERROR cannot be resolved by the auditor

**Found:** 2026-09-23, same engagement.

**Symptom:** a check that ends ERROR holds completion below 100% and there is
no way to record a verdict for it, even when the auditor has verified the
requirement by hand. `--review` offers only checks whose verdict is REVIEW
(`nextUndecided` filters on `vReview`), so ERROR, SKIP and DENIED are
unreachable.

**Consequence:** an audit with any ERROR in it cannot be completed without
re-running the pass successfully. Where the cause is environmental — a quota,
a propagation delay, an API enabled mid-run — that may not be possible on the
day.

**Fix, when there is time:** let `--review` offer blocked checks too, recording
the verdict as the auditor's with the machine's ERROR text preserved beneath
it, exactly as a decided REVIEW already does. The distinction that matters in
the report is *who* decided, and that is already captured.
