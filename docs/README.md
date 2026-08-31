# Documentation

CIS Controls v8.1 Implementation Group 1, applied to a Google Cloud Organization.

Six documents. **Start with the runbook** — it sequences the rest.

---

## Which document do I need?

| I want to… | Go to |
|---|---|
| **Run an audit end to end** | **[Runbook](cis-ig1-audit-runbook.md)** |
| Understand what IG1 is and why each Control exists | [Overview](cis-ig1-overview.md) |
| Assess the organization and record findings | [Checklist](cis-ig1-gcp-checklist.md) |
| Run the `gcloud` command that proves an item compliant | [CLI Validation](cis-ig1-cli-validation.md) |
| Fix something that came up non-compliant | [Remediation Reference](cis-ig1-remediation-reference.md) |
| See what benchmark work contributes toward IG1 | [Benchmark Contribution](cis-ig1-benchmark-overlap.md) |
| Find out how much is already done | [`../README.md`](../README.md#scoring) |

---

## Quick start

Follow [`cis-ig1-audit-runbook.md`](cis-ig1-audit-runbook.md) — ten phases from prerequisites to reporting, each gating the next. The short version:

1. Read [About this document](cis-ig1-overview.md#about-this-document) and the [permissive-default vs secure-by-default](cis-ig1-overview.md#permissive-default-vs-secure-by-default-organizations) section — five minutes, and it determines the size of the job.
2. Run [Step 0](cis-ig1-gcp-checklist.md#step-0--establish-your-starting-position) in the checklist. One `gcloud` command.
3. Work down the checklist, ticking requirements.
4. For each failure, look up the safeguard ID in the [Remediation Reference](cis-ig1-remediation-reference.md#contents).
5. Or skip the clicking: `go run audit-run.go` runs every check and prints one report to tick from.
6. Score progress with `go run compliance-report.go`.

---

## 0. Runbook

**[`cis-ig1-audit-runbook.md`](cis-ig1-audit-runbook.md)** — the operational sequence: prerequisites and API enablement, permission smoke test, positive controls for the `jq` checks, full run, triage order, and how to report the result without it being misread.

Read this first if you are running an audit rather than reading about one.

---

## 1. Overview

**[`cis-ig1-overview.md`](cis-ig1-overview.md)** — why each Control exists, what it achieves, and what it touches in a GCP estate. All 18 Controls and all 56 IG1 safeguards, including the 12 with no GCP surface.

[About this document](cis-ig1-overview.md#about-this-document) · [Summary](cis-ig1-overview.md#summary)

| | Control | |
|---|---|---|
| 01 | [Inventory and Control of Enterprise Assets](cis-ig1-overview.md#control-01--inventory-and-control-of-enterprise-assets) | 2 safeguards |
| 02 | [Inventory and Control of Software Assets](cis-ig1-overview.md#control-02--inventory-and-control-of-software-assets) | 3 |
| 03 | [Data Protection](cis-ig1-overview.md#control-03--data-protection) | 6 |
| 04 | [Secure Configuration of Enterprise Assets and Software](cis-ig1-overview.md#control-04--secure-configuration-of-enterprise-assets-and-software) | 7 |
| 05 | [Account Management](cis-ig1-overview.md#control-05--account-management) | 4 |
| 06 | [Access Control Management](cis-ig1-overview.md#control-06--access-control-management) | 5 |
| 07 | [Continuous Vulnerability Management](cis-ig1-overview.md#control-07--continuous-vulnerability-management) | 4 |
| 08 | [Audit Log Management](cis-ig1-overview.md#control-08--audit-log-management) | 3 |
| 09 | [Email and Web Browser Protections](cis-ig1-overview.md#control-09--email-and-web-browser-protections) | 2 |
| 10 | [Malware Defenses](cis-ig1-overview.md#control-10--malware-defenses) | 3 |
| 11 | [Data Recovery](cis-ig1-overview.md#control-11--data-recovery) | 4 |
| 12 | [Network Infrastructure Management](cis-ig1-overview.md#control-12--network-infrastructure-management) | 1 |
| 13 | [Network Monitoring and Defense](cis-ig1-overview.md#control-13--network-monitoring-and-defense) | 0 at IG1 |
| 14 | [Security Awareness and Skills Training](cis-ig1-overview.md#control-14--security-awareness-and-skills-training) | 8 — no GCP surface |
| 15 | [Service Provider Management](cis-ig1-overview.md#control-15--service-provider-management) | 1 |
| 16 | [Application Software Security](cis-ig1-overview.md#control-16--application-software-security) | 0 at IG1 |
| 17 | [Incident Response Management](cis-ig1-overview.md#control-17--incident-response-management) | 3 |
| 18 | [Penetration Testing](cis-ig1-overview.md#control-18--penetration-testing) | 0 at IG1 |

---

## 2. Checklist

**[`cis-ig1-gcp-checklist.md`](cis-ig1-gcp-checklist.md)** — the working document. 44 GCP-actionable safeguards with per-requirement checkboxes.

[How to use this checklist](cis-ig1-gcp-checklist.md#how-to-use-this-checklist) · [**Step 0 — start here**](cis-ig1-gcp-checklist.md#step-0--establish-your-starting-position)

[Control 01](cis-ig1-gcp-checklist.md#control-01--inventory-and-control-of-enterprise-assets) ·
[02](cis-ig1-gcp-checklist.md#control-02--inventory-and-control-of-software-assets) ·
[03](cis-ig1-gcp-checklist.md#control-03--data-protection) ·
[04](cis-ig1-gcp-checklist.md#control-04--secure-configuration-of-enterprise-assets-and-software) ·
[05](cis-ig1-gcp-checklist.md#control-05--account-management) ·
[06](cis-ig1-gcp-checklist.md#control-06--access-control-management) ·
[07](cis-ig1-gcp-checklist.md#control-07--continuous-vulnerability-management) ·
[08](cis-ig1-gcp-checklist.md#control-08--audit-log-management) ·
[09](cis-ig1-gcp-checklist.md#control-09--email-and-web-browser-protections) ·
[10](cis-ig1-gcp-checklist.md#control-10--malware-defenses) ·
[11](cis-ig1-gcp-checklist.md#control-11--data-recovery) ·
[12](cis-ig1-gcp-checklist.md#control-12--network-infrastructure-management) ·
[15](cis-ig1-gcp-checklist.md#control-15--service-provider-management) ·
[17](cis-ig1-gcp-checklist.md#control-17--incident-response-management)

[Appendix A — excluded safeguards](cis-ig1-gcp-checklist.md#appendix-a--safeguards-excluded-from-this-checklist) ·
[Appendix B — assessment summary](cis-ig1-gcp-checklist.md#appendix-b--assessment-summary)

### Marking state

| Markup | State | Owner |
|---|---|---|
| `- [ ] item` | Not started | SRE |
| ``- [ ] item `PR #123` `` | PR submitted, awaiting approval | **Management** |
| `- [x] item` | Compliant, verified in the live estate | Done |

`[x]` means verified in the estate — not "merged". Date PRs as `` `PR #123 2026-07-02` `` so the report can age them.

---

## 3. Remediation Reference

**[`cis-ig1-remediation-reference.md`](cis-ig1-remediation-reference.md)** — for each failed safeguard: a `gcloud` command to confirm the finding, and example Terraform to fix it.

> ### ⚠️ [Read this first — Organization Policy is not retroactive](cis-ig1-remediation-reference.md#-read-this-first--organization-policy-is-not-retroactive)
>
> A constraint blocks *future* non-conforming operations and does nothing to what already exists. Applying it while the estate is still full of violations produces a green console and a false pass. Every org policy fix has a discovery command for the existing estate — run it first.

[Contents](cis-ig1-remediation-reference.md#contents) · [The org policy pattern](cis-ig1-remediation-reference.md#the-org-policy-pattern) · [Find everything at once](cis-ig1-remediation-reference.md#find-everything-at-once) · [Process safeguards](cis-ig1-remediation-reference.md#process-safeguards)

| Control | Safeguards |
|---|---|
| 01 | [1.1 Enterprise asset inventory](cis-ig1-remediation-reference.md#11--enterprise-asset-inventory) · [1.2 Unauthorized assets](cis-ig1-remediation-reference.md#12--address-unauthorized-assets) |
| 02 | [2.1 Software inventory](cis-ig1-remediation-reference.md#21--software-inventory) · [2.2 Supported software](cis-ig1-remediation-reference.md#22--supported-software) · [2.3 Unauthorized software](cis-ig1-remediation-reference.md#23--address-unauthorized-software) |
| 03 | [3.2 Data inventory](cis-ig1-remediation-reference.md#32--data-inventory) · [3.3 Data access control lists](cis-ig1-remediation-reference.md#33--data-access-control-lists) · [3.4 Data retention](cis-ig1-remediation-reference.md#34--enforce-data-retention) |
| 04 | [4.2 Secure network config](cis-ig1-remediation-reference.md#42--secure-network-configuration) · [4.3 Session locking](cis-ig1-remediation-reference.md#43--session-locking) · [4.4 Firewall on servers](cis-ig1-remediation-reference.md#44--firewall-on-servers) · [4.6 Securely manage assets](cis-ig1-remediation-reference.md#46--securely-manage-assets) · [4.7 Default accounts](cis-ig1-remediation-reference.md#47--manage-default-accounts) |
| 05 | [5.1 Account inventory](cis-ig1-remediation-reference.md#51--account-inventory) · [5.2 Static credentials](cis-ig1-remediation-reference.md#52--eliminate-static-credentials) · [5.3 Dormant accounts](cis-ig1-remediation-reference.md#53--dormant-accounts) · [5.4 Admin privileges](cis-ig1-remediation-reference.md#54--restrict-admin-privileges) |
| 06 | [6.3 MFA, external apps](cis-ig1-remediation-reference.md#63--mfa-for-externally-exposed-apps) · [6.4 MFA, remote access](cis-ig1-remediation-reference.md#64--mfa-for-remote-access) · [6.5 MFA, admin access](cis-ig1-remediation-reference.md#65--mfa-for-admin-access) |
| 07 | [7.3 OS patching](cis-ig1-remediation-reference.md#73--os-patch-management) · [7.4 Application patching](cis-ig1-remediation-reference.md#74--application-patch-management) |
| 08 | [8.2 Collect audit logs](cis-ig1-remediation-reference.md#82--collect-audit-logs) · [8.3 Audit log storage](cis-ig1-remediation-reference.md#83--audit-log-storage) |
| 09 | [9.2 DNS filtering](cis-ig1-remediation-reference.md#92--dns-filtering) |
| 10 | [10.1 Anti-malware](cis-ig1-remediation-reference.md#101--anti-malware) · [10.2 Signature updates](cis-ig1-remediation-reference.md#102--signature-updates) |
| 11 | [11.2 Automated backups](cis-ig1-remediation-reference.md#112--automated-backups) · [11.3 Protect recovery data](cis-ig1-remediation-reference.md#113--protect-recovery-data) · [11.4 Isolated recovery data](cis-ig1-remediation-reference.md#114--isolated-recovery-data) |
| 12 | [12.1 Network currency](cis-ig1-remediation-reference.md#121--network-infrastructure-currency) |
| 15 | [15.1 Service provider inventory](cis-ig1-remediation-reference.md#151--service-provider-inventory) |
| 17 | [17.2 Incident contacts](cis-ig1-remediation-reference.md#172--incident-contact-information) · [17.3 Incident reporting](cis-ig1-remediation-reference.md#173--incident-reporting) |

---

## 4. CLI Validation

**[`cis-ig1-cli-validation.md`](cis-ig1-cli-validation.md)** — 188 read-only `gcloud` checks covering 188 of the 290 checklist requirements, each with pass criteria. Canonical home for every validation command; the checklist links here by V-number.

[Before you start](cis-ig1-cli-validation.md#before-you-start) — required roles, Cloud Asset Inventory setup, and a caution about the audit identity.

The other 102 requirements are process, documentation, or Admin Console settings with no API surface. They appear under each safeguard as *manual* rather than being quietly omitted.

Most checks are written so **empty output means compliant**.

---

## 5. Benchmark Contribution

**[`cis-ig1-benchmark-overlap.md`](cis-ig1-benchmark-overlap.md)** — only when the CIS GCP Foundation Benchmark v5.0.0 is also in scope. One direction: what benchmark-driven work contributes toward IG1. 19 shared items closing 62 of 290 IG1 requirements.

[Shared work items](cis-ig1-benchmark-overlap.md#shared-work-items) ·
[What it buys you](cis-ig1-benchmark-overlap.md#what-benchmark-work-buys-you-against-ig1) ·
[What it will not touch](cis-ig1-benchmark-overlap.md#what-benchmark-work-will-not-touch) ·
[Handoff](cis-ig1-benchmark-overlap.md#handoff-to-the-benchmark-repository)

The **Rec #** column is intentionally blank — benchmark numbering shifts between major versions, so it must be filled from your copy of the PDF rather than from memory. Until then the counts are indicative for sizing, not a compliance claim.

---

## Training

**[`training/`](training/)** — source material for a 20-minute class on NIST 800 and CIS IG1, aimed at SREs who will run the audit. Nine markdown files, one per slide section, plus a reference card to hand out.

The thesis: IG1 is not new work, it is a naming convention for GCP hygiene engineers already argue for. What they need is the vocabulary, not the concepts.

Also there: [`09-process-interview.md`](training/09-process-interview.md), a worksheet covering the 72 process-and-people requirements — for a meeting with people who know the account rather than a terminal.

---

## Effort tracker

**[`cis-ig1-tracker.xlsx`](cis-ig1-tracker.xlsx)** — all 290 requirements as a spreadsheet, for tracking engineering effort. Import straight into Google Sheets (File → Import → Upload).

Three tabs: **Tracker** (one row per requirement, shaded columns are the ones you fill in), **Summary** (formulas over the tracker — by group, by scope, by safeguard, and where the work is sitting), **How to use** (legend).

The Status column is a dropdown: Not started · In progress · PR submitted · Compliant · N/A. `PR submitted` is the one that matters — it moves an item from engineering's queue to awaiting management approval.

---

## Scope note

IG1 is enterprise-wide essential cyber hygiene — it assumes employees, laptops, email, and vendors, not just cloud infrastructure. Of its 56 safeguards, **44** are GCP-actionable and appear in the checklist; **12** have no GCP surface and are listed in [Appendix A](cis-ig1-gcp-checklist.md#appendix-a--safeguards-excluded-from-this-checklist) with their owners.

A completed checklist evidences the GCP half of IG1, not IG1.

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This is an implementation aid, not affiliated with or endorsed by CIS.*
