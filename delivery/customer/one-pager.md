# CIS Controls IG1 — GCP Compliance Audit

**A defensible baseline for your Google Cloud estate, measured rather than asserted.**

## What IG1 is

CIS Controls v8.1 Implementation Group 1 is the floor: 56 safeguards CIS defines as *essential cyber hygiene* for any enterprise. Not a maturity model, not aspirational — the minimum a reasonable organisation is expected to have in place.

It is recognised, mapped to NIST CSF 2.0, and small enough to finish. That combination is why it works as a first compliance baseline.

## What the audit does

Of the 290 GCP-applicable requirements under those safeguards, **188 have a `gcloud` command that answers them**. We run all 188 against your organization and every project in scope, record the result of each, and put the ones a machine cannot judge to a human.

| | |
|---|---|
| **Read-only** | Every check reads. Nothing is created, changed or deleted |
| **No keys** | The audit runs as a service account your team creates, impersonated per call. No key is ever generated, so none can leak |
| **No agents** | Nothing is installed in your estate. No VM, no daemon, no sidecar |
| **Your identity, your control** | You create the service account, you grant it, you destroy it. We ask you to remove it when we finish |

## What you get

**Three numbers per project, and per the organization.**

- **Completion** — the share of checks that reached a final verdict. 100% means the audit of that project is finished
- **Pass** — safeguards met
- **Fail** — safeguards not met

**A remediation plan pivoted on the finding, not the project.** One organization policy change that fixes forty projects appears as one work item, not forty. On a recent engagement that turned 300-odd raw findings into roughly 25 pieces of work, ranked by how many projects each clears.

**Per-safeguard status, with the blocker named.** Not "safeguard 4.6 failed" but *which requirement* failed and *in which projects* — so your team knows what to change and where.

**A machine-readable feed.** JSON, stable schema, so the results land in your own dashboard rather than a slide deck nobody updates.

## What it costs you

| | |
|---|---|
| **Setup** | A read-only service account and 17 Google APIs enabled in one project. An hour of a cloud engineer's time |
| **Your input** | 102 of the 290 requirements have no technical check — *is there a documented process, is the inventory recertified, is there security awareness training*. We put those to you as a structured interview, answered once for the organization |
| **Elapsed time** | Driven by estate size. The automated passes are hours; adjudicating the results is the long pole |
| **Disruption** | None. No change window, no downtime, no agent |

## What we will not claim

CIS sets **no pass mark** for IG1. The standard is "implement every safeguard", so a percentage is progress against that — not a certification, and we will not present it as one.

An audit measures. It does not remediate. We will hand you a ranked work list and the example Terraform to close the common findings, but the changes are yours to make and test.

---

*CIS Controls® is a registered trademark of the Center for Internet Security. This audit is an independent implementation and is not a CIS certification.*
