# GCP Benchmark → IG1 Contribution

**One direction only:** what work driven by the CIS Google Cloud Platform Foundation Benchmark **v5.0.0** contributes toward CIS Controls v8.1 **IG1** compliance.

This repository targets 100% IG1. If the benchmark is also in scope, some of that work is shared — this document identifies which, so it is scheduled once rather than twice.

**The reverse question is out of scope here.** What percentage of the benchmark your post-IG1 infrastructure satisfies is a separate deliverable in its own repository, because it needs the benchmark's own recommendation inventory as its denominator.

---

## How to read this

**The unit is a work item, not a control.** Nobody schedules a control; they schedule an action. One action — *"enforce uniform bucket-level access org-wide, then remediate existing buckets"* — contributes to IG1 3.3 and to several benchmark recommendations at once. That action is the row.

**Contribution is partial by default.** An item here is doing work IG1 needs, not closing an IG1 safeguard. The **Covers** column counts the requirement checkboxes in [`cis-ig1-gcp-checklist.md`](cis-ig1-gcp-checklist.md) that the item closes, against that safeguard's total. Counts derive from the checklist itself and can be re-verified against it.

**The Rec # column is blank on purpose.** Benchmark numbering changes between major versions, and filling it from memory would put plausible but wrong references into a plan. Populate it from your copy of v5.0.0 — each recommendation lists its own CIS Controls v8 mapping, which is authoritative and overrides anything here that disagrees.

**Items marked ⚠️ are two pieces of work.** Applying an org policy constraint is a one-line change; remediating what already violates it is usually the larger half and scales with estate size. Org policy is not retroactive.

---

## Shared work items

| # | Work item | IG1 | Covers | % of safeguard | Benchmark area | Rec # |
|---|---|---|---|---|---|---|
| 1 | Enable Data Access audit logs org-wide | 8.2 | 4 / 11 | 36% | Logging and Monitoring | |
| 2 | Org-level aggregated sink, verified writer identity | 8.2<br>8.3 | 2 / 11<br>1 / 6 | 18%<br>17% | Logging and Monitoring | |
| 3 | Log retention, dedicated logging project, Bucket Lock | 8.3 | 5 / 6 | 83% | Logging and Monitoring | |
| 4 | Log-based metrics and alerts on control-plane changes | 8.2<br>5.4 | 1 / 11<br>1 / 8 | 9%<br>13% | Logging and Monitoring | |
| 5 | Block service account key creation and upload ⚠️ | 5.2 | 1 / 8 | 13% | Identity and Access Management | |
| 6 | Inventory and delete existing user-managed SA keys | 5.2 | 4 / 8 | 50% | Identity and Access Management | |
| 7 | Remove basic roles from individual users | 5.4 | 3 / 8 | 38% | Identity and Access Management | |
| 8 | Strip `roles/editor` from default service accounts ⚠️ | 4.7 | 7 / 9 | 78% | IAM / Virtual Machines | |
| 9 | Enforce MFA / 2SV, corporate credentials only | 6.5 | 5 / 6 | 83% | Identity and Access Management | |
| 10 | Remediate public buckets; uniform access + public access prevention ⚠️ | 3.3 | 5 / 11 | 45% | Cloud Storage | |
| 11 | BigQuery datasets not publicly accessible | 3.3 | 1 / 11 | 9% | BigQuery | |
| 12 | Cloud SQL: no public IP, no `0.0.0.0/0`, require SSL ⚠️ | 4.4 | 2 / 10 | 20% | Cloud SQL Database Services | |
| 13 | Cloud SQL automated backups and PITR | 11.2 | 1 / 7 | 14% | Cloud SQL Database Services | |
| 14 | Remove firewall rules allowing `0.0.0.0/0` to 22 / 3389 ⚠️ | 4.4 | 3 / 10 | 30% | Networking | |
| 15 | Delete default networks and permissive rules ⚠️ | 4.2 | 4 / 9 | 44% | Networking | |
| 16 | Enable VPC Flow Logs on subnets | 8.2 | 1 / 11 | 9% | Networking | |
| 17 | Enforce OS Login; retire project-wide SSH keys ⚠️ | 4.6 | 3 / 13 | 23% | Virtual Machines | |
| 18 | VM hardening: serial port, IP forwarding, Shielded VM ⚠️ | 4.6 | 4 / 13 | 31% | Virtual Machines | |
| 19 | Essential Contacts on corporate domains | 17.2 | 4 / 7 | 57% | Identity and Access Management | |

Where the two frameworks overlap, the benchmark is almost always the more prescriptive — it names the exact setting where IG1 names the outcome. **Build to the benchmark's specificity and the IG1 requirement is satisfied as a by-product.** Item 4 is the clearest case: IG1 asks that audit logs be collected; the benchmark enumerates the specific metric filters and alerts required.

---

## What benchmark work buys you against IG1

Completing all 19 shared items:

| IG1 safeguard | Requirements closed | |
|---|---|---|
| 8.3 Audit log storage | 6 / 6 | **100% — fully closed** |
| 6.5 MFA for admin access | 5 / 6 | 83% |
| 4.7 Manage default accounts | 7 / 9 | 78% |
| 8.2 Collect audit logs | 8 / 11 | 73% |
| 5.2 Eliminate static credentials | 5 / 8 | 63% |
| 17.2 Incident contact information | 4 / 7 | 57% |
| 3.3 Data access control lists | 6 / 11 | 55% |
| 4.6 Securely manage assets | 7 / 13 | 54% |
| 4.4 Firewall on servers | 5 / 10 | 50% |
| 5.4 Restrict admin privileges | 4 / 8 | 50% |
| 4.2 Secure network configuration | 4 / 9 | 44% |
| 11.2 Automated backups | 1 / 7 | 14% |

### The three numbers that matter

**62 of 290 IG1 requirements — 21%.**

**12 of 44 safeguards touched. 32 receive nothing at all.**

**1 safeguard fully closed** (8.3). Every other safeguard above still needs its remaining requirements finished before it can be marked compliant.

That third figure is the one to carry into planning, because it inverts the intuition. Benchmark-driven work touches a quarter of IG1's requirements while finishing almost none of its safeguards — 4.7 reaches 78% and 6.5 reaches 83%, and neither is done. Treating a safeguard as complete because the benchmark work landed will leave the final stretch of eleven safeguards unbudgeted.

---

## What benchmark work will not touch

The other 228 IG1 requirements, across 32 safeguards, get no contribution. Broadly:

- **Inventory** — asset, software, and data inventory (1.1, 1.2, 2.1, 2.2, 2.3, 3.2). The benchmark assesses configuration, not whether you know what you have.
- **Operational** — data retention (3.4), dormant account review (5.3), patch management (7.3, 7.4), anti-malware (10.1, 10.2), backup protection and isolation (11.3, 11.4), network currency (12.1), DNS filtering (9.2).
- **Access process** — granting and revoking (6.1, 6.2), MFA for external apps and remote access (6.3, 6.4).
- **Process and documentation** — 3.1, 3.5, 4.1, 7.1, 7.2, 8.1, 11.1, 15.1, 17.1, 17.3.
- **Non-GCP entirely** — the 12 workforce and endpoint safeguards. See the [overview](cis-ig1-overview.md#control-14--security-awareness-and-skills-training).

The [checklist](cis-ig1-gcp-checklist.md) covers all of it. This section exists only to make clear that a benchmark programme does not quietly deliver IG1 alongside it.

---

## Filling in the Rec # column

1. Open the v5.0.0 PDF. Each recommendation lists its own CIS Controls v8 safeguard mapping.
2. Populate **Rec #**, and confirm the IG1 column matches what CIS itself asserts.
3. Where CIS disagrees with this document, **CIS wins** — this is an unofficial planning aid.
4. Any benchmark recommendation whose stated mapping hits an IG1 safeguard not listed above is a missed contribution. Add it.
5. Re-verify on each benchmark release.

---

## Handoff to the benchmark repository

Two things belong in that repository rather than this one:

**The reverse measurement.** What percentage of v5.0.0 your post-IG1 infrastructure satisfies, which needs the benchmark's recommendation inventory as a denominator. Expect the per-item contribution to be higher than the figures above — the benchmark names exact settings, so an item tends to close a recommendation outright rather than partially — but the overall percentage to be lower, since most benchmark recommendations sit outside IG1's scope entirely.

**Benchmark-only scope.** KMS rotation intervals, DNSSEC signing algorithms, per-engine Cloud SQL database flags, API key restrictions, Confidential Computing, CMEK across data services, separation of duties for service account and KMS admins. None of it appears in IG1.

Also worth settling there: **GKE is a separate benchmark.** Cluster hardening lives in the CIS Google Kubernetes Engine Benchmark, so if clusters are in scope that is a third framework.

---

*CIS Controls® and CIS Benchmarks® are registered trademarks of the Center for Internet Security, Inc. This is an unofficial planning aid, not affiliated with or endorsed by CIS. Verify all mappings against the published benchmark before relying on them.*
