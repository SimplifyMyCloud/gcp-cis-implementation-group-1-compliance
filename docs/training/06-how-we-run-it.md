# 6. How we actually run it

---

## Two passes, in order

Organization first. Its findings explain the project results — a missing org policy constraint is *why* fifty projects each have a default network.

Everything below assumes the shell is already set up — signed in, on your branch, impersonating the auditor, with `audit-state/audit.env` in place. That is [auditor setup](../cis-ig1-auditor-setup.md), or `audit-on` once its Cloud Shell setup is done.

```bash
export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)
```

### Organization pass

```bash
go run audit-run.go -scope=org -org=$ORG_ID -pack ./audit-state/org
```

86 checks. Results stream as they land:

```
Running 86 checks against organization 123456789012
Audit host project my-audit-project (123456789)

  [  1/ 71] V43    REVIEW  4.1#2     Baseline enforced through org policy constraints
  [  2/ 71] V27    PASS    3.3#1     No publicly accessible Cloud Storage buckets
  [  3/ 71] V55    FAIL    4.4#1     No firewall rules allowing the internet to SSH or R…
  ...
```

### Project pass

```bash
gcloud projects list --format="value(projectId)"

export PROJECT=<pick-one>
go run audit-run.go -scope=project -org=$ORG_ID \
  -project=$PROJECT -pack ./audit-state/projects/$PROJECT
```

103 checks, against that project only. Repeat per project.

---

## Running it live

For a demo you do not want to wait for 86 checks. Pick four that hit different subsystems and finish in seconds:

```bash
go run audit-run.go -scope=org -org=$ORG_ID \
  -only V43,V27,V55,V181
```

| | What it proves |
|---|---|
| `V43` | Org policy constraints — the whole enforcement story |
| `V27` | Public buckets — the highest-severity finding class |
| `V55` | SSH and RDP open to the internet — the finding that gets attention |
| `V181` | Essential Contacts — the two-minute fix nobody does |

To show the shape without touching the org at all:

```bash
go run audit-run.go -list | head -20
```

That prints every check and how it is classified — `runnable`, `review`, `needs APPROVED_REGISTRIES`, `xref`, `by-hand` — and runs nothing.

---

## What comes out

| File | Contents |
|---|---|
| `01-automated-results.md` | Every check, as one table, with findings and their output |
| `02-manual-cli.md` | 30 GCP tasks with no CLI check — Admin Console, image build, tests to perform |
| `03-manual-process.md` | 72 process requirements — **still ours**, just written rather than configured |

---

## Verdicts

| | Meaning |
|---|---|
| `PASS` | Compliant |
| `FAIL` | A finding — output names the offending resources |
| `REVIEW` | Ran clean; **a human decides** |
| `N/A` | Product or API absent — not a failure |
| `DENIED` | Missing permission — fix before trusting anything |

**96 of 188 are auto-scored.** The other 92 are `REVIEW` with output saved verbatim.

That ratio is deliberate. No machine can tell you whether your org policy list matches your intended baseline. A confident wrong verdict is worse in an audit than an honest "you decide."

---

## Two rules the tool enforces

**Every resource, not a sample.** Ten Cloud SQL instances, one non-compliant, the safeguard fails — and the output names which one.

**"Doesn't exist" is a finding, not a skip.** No log bucket is not a check we couldn't run. It is safeguard 8.3 failing.

---

## We run as a throwaway service account

Created at Phase 1, impersonated — never keyed, because a service account key would violate 5.2, which we are testing.

Deleted at Phase 11, along with its 22 org-level roles.

**The audit will find the auditor** in the IAM inventory checks while it is running. That is expected. If it is still there *after* teardown, that is a real finding.

> **Speaker note:** The vault analogy lands well — the auditor has to open the vault to count the bars, even where the rule says the vault stays shut.
