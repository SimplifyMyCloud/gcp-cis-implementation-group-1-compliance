# CIS Controls v8.1 IG1 — GCP Compliance Checklist

**Target:** Google Cloud Platform Organization
**Scope:** GCP-actionable requirements only — 44 of 56 IG1 Safeguards
**Companion document:** `cis-ig1-overview.md`

---

## How to use this checklist

Each safeguard carries a **Status** line and a set of requirement checkboxes beneath it. Tick the requirements; the Status line is derived from them.

### How each requirement is verified

Every requirement carries a marker showing how it gets checked and who does it.

| | Group | Count | How |
|---|---|---|---|
| ⚙️ | **Automated** | 41 | `audit-run.go` runs it and scores it pass/fail |
| 🔍 | **CLI, human reads it** | 147 | `audit-run.go` runs it and saves the output; you judge the result |
| 🖥️ | **GCP, no CLI** | 30 | Admin Console, image build, or a test you perform by hand |
| 👥 | **Process and people** | 72 | Answered by a conversation and a document, not a command |

⚙️ and 🔍 both run from the script — the difference is whether a machine can defend the verdict. 🖥️ and 👥 are the 102 no script touches.

**👥 is the group to take into a room with people who know the account.** Those 72 are about how the estate is run rather than how it is configured, and the fastest way to answer them is usually to ask someone who has been on the engagement for years. [`training/09-process-interview.md`](training/09-process-interview.md) turns them into questions you can work through in a meeting.

### Three states, not two

A binary compliant/not-compliant model hides the distinction that matters here: work this team has finished versus work still sitting with this team. Requirements therefore carry one of three states.

| Markup | State | Meaning |
|---|---|---|
| `- [ ] item` | **Not started** | No fix written. Owned by SRE. |
| ``- [ ] item `PR #123` `` | **PR submitted** | Fix written and raised as code. **Owned by management** — awaiting approval and apply. |
| `- [x] item` | **Compliant** | Verified in the live estate. Done. |

The `[x]` box means **verified in the estate**, not "code merged". A merged PR that has not been applied, or has been applied without confirming the result, is not compliant. Verify, then tick.

Optionally date the PR so waiting time can be measured:

```
- [ ] All currently public buckets remediated `PR #123 2026-07-02`
```

### Why the middle state exists

Remediation here requires code that management approves and runs. That split means an unremediated finding has two very different causes, and reporting them as one number is misleading.

Once a fix is raised as a PR, SRE has discharged its part. The finding is still open, but it is open **pending approval**, and the report says so explicitly — including how many days it has been waiting. The metric to watch is not "how much is non-compliant" but "how much is non-compliant *and nobody has written the fix*".

### Scoring

```
go run compliance-report.go
```

Reports percentage compliant, percentage awaiting approval, and the aging list of PRs sitting unapproved. Add `--update` to rewrite the Status lines in this document to match the requirement boxes.

**Assessed by:** _______________
**Assessment date:** _______________
**Organization ID:** _______________

Twelve IG1 safeguards have no GCP surface and are excluded — see Appendix A.

When something fails, flip to **`cis-ig1-remediation-reference.md`** and look up the safeguard ID. It gives the `gcloud` command to confirm the finding and example Terraform to fix it.

> ### ⚠️ An applied org policy is not a passed safeguard
>
> Organization Policy constraints are **not retroactive**. They block future non-conforming operations and change nothing that already exists. A bucket that was public before the constraint stays public. A service account key already exported stays valid.
>
> Items below marked **↺ existing estate** are the clean-up half. **Do not mark a safeguard Compliant on the strength of the constraint alone** — that is how an audit produces a false pass. The remediation reference gives a discovery command for each one.

---

## Step 0 — Establish your starting position

Before working the checklist, determine whether this Organization is **permissive-default** or **secure-by-default** (see the overview document). It materially changes how much of Controls 3, 4, 5, 15, and 17 is already done.

```
gcloud org-policies list --organization=ORGANIZATION_ID
```

- [ ] Existing org policy constraints listed and recorded
- [ ] Starting position recorded below

**This organization is:** `[ ] Permissive-default (no baseline enforced)`  `[ ] Secure-by-default (baseline present)`

Constraints you did not apply yourself came from Google's security baseline and partially satisfy safeguards **3.3, 4.6, 4.7, 5.2, 15.1, and 17.2**. Check rather than assume — a baseline can be deleted after creation, and a permissive-default organization that has already been hardened may be in better shape than a secure-by-default one that has not been maintained.

A secure-by-default organization still has 44 safeguards to satisfy. The baseline removes work; it does not remove the requirement.

---

## Control 01 — Inventory and Control of Enterprise Assets

### 1.1 Establish and Maintain Detailed Enterprise Asset Inventory
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Cloud Asset Inventory feed configured at **organization** scope → [V1](cis-ig1-cli-validation.md#v1)
- [ ] 🔍 Feed exports to BigQuery or Cloud Storage on a scheduled basis → [V2](cis-ig1-cli-validation.md#v2)
- [ ] 🔍 All projects enumerated, including those created flat under the org node → [V3](cis-ig1-cli-validation.md#v3)
- [ ] 🔍 Inventory covers Compute Engine, GKE, Cloud SQL, Cloud Run, Cloud Functions, App Engine, Cloud Storage → [V4](cis-ig1-cli-validation.md#v4)
- [ ] ⚙️ Required labels enforced (owner, environment, cost centre, data classification) → [V5](cis-ig1-cli-validation.md#v5)
- [ ] 🔍 Shared VPC host and service project relationships mapped → [V6](cis-ig1-cli-validation.md#v6)
- [ ] 🖥️ Inventory reviewed and recertified at least every six months

### 1.2 Address Unauthorized Assets
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🖥️ Documented process for resources found without an owner or approval
- [ ] 🔍 Projects with no billing account or in `DELETE_REQUESTED` state dispositioned → [V7](cis-ig1-cli-validation.md#v7)
- [ ] ⚙️ Orphaned resources reconciled (unattached disks, unused static IPs, idle forwarding rules) → [V8](cis-ig1-cli-validation.md#v8)
- [ ] 🖥️ Unlabelled or unattributable resources escalated within a defined window
- [ ] 🖥️ Remediation actions recorded (removed, quarantined, or formally accepted)

---

## Control 02 — Inventory and Control of Software Assets

### 2.1 Establish and Maintain a Software Inventory
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 VM Manager OS inventory enabled; OS Config agent present on all Compute Engine instances → [V9](cis-ig1-cli-validation.md#v9)
- [ ] 🔍 Custom image and image family catalogue maintained with build provenance → [V10](cis-ig1-cli-validation.md#v10)
- [ ] 🔍 Artifact Registry / Container Registry image inventory captured → [V11](cis-ig1-cli-validation.md#v11)
- [ ] 🔍 GKE cluster and node pool versions recorded → [V12](cis-ig1-cli-validation.md#v12)
- [ ] 🔍 Cloud Functions, Cloud Run, and App Engine runtime versions recorded → [V13](cis-ig1-cli-validation.md#v13)
- [ ] 🔍 Cloud SQL engine and version recorded per instance → [V14](cis-ig1-cli-validation.md#v14)
- [ ] 👥 Inventory refreshed automatically, not by manual survey

### 2.2 Ensure Authorized Software is Currently Supported
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] ⚙️ No instances running end-of-life guest OS (CentOS 7, Debian 9/10, out-of-window Ubuntu) → [V15](cis-ig1-cli-validation.md#v15)
- [ ] ⚙️ No GKE clusters on versions past end-of-life; all enrolled in a release channel → [V16](cis-ig1-cli-validation.md#v16)
- [ ] 🔍 No Cloud Functions or App Engine services on decommissioned runtimes → [V17](cis-ig1-cli-validation.md#v17)
- [ ] 🔍 No Cloud SQL instances on unsupported database versions → [V18](cis-ig1-cli-validation.md#v18)
- [ ] 🔍 Deprecated custom images not referenced by any instance template or MIG → [V19](cis-ig1-cli-validation.md#v19)
- [ ] 👥 Exception register for unsupported software with documented compensating controls

### 2.3 Address Unauthorized Software
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Documented process for removing unapproved software from the environment
- [ ] 👥 Marketplace deployment restricted to approved solutions
- [ ] 🔍 Container images sourced only from approved registries → [V20](cis-ig1-cli-validation.md#v20)
- [ ] 🔍 Binary Authorization policy in place for GKE / Cloud Run (or a documented equivalent gate) → [V21](cis-ig1-cli-validation.md#v21)
- [ ] 🔍 **↺ existing estate:** workloads already running unattested identified and rolled → [V22](cis-ig1-cli-validation.md#v22)
- [ ] 👥 Findings tracked to closure with a defined remediation window

---

## Control 03 — Data Protection

### 3.1 Establish and Maintain a Data Management Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Written data management process covering sensitivity, ownership, handling, retention, disposal
- [ ] 👥 Process references GCP storage services in use
- [ ] 🔍 Data residency requirements defined and mapped to regions → [V23](cis-ig1-cli-validation.md#v23)
- [ ] 👥 Reviewed annually with review date recorded

### 3.2 Establish and Maintain a Data Inventory
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Sensitive Data Protection (Cloud DLP) discovery configured across Cloud Storage and BigQuery → [V24](cis-ig1-cli-validation.md#v24)
- [ ] 🖥️ Cloud SQL and other data stores included in the inventory
- [ ] ⚙️ Sensitivity classification recorded as resource labels or in a central register → [V25](cis-ig1-cli-validation.md#v25)
- [ ] 🔍 Data location recorded per store for residency verification → [V26](cis-ig1-cli-validation.md#v26)
- [ ] 🖥️ Inventory refreshed on a defined schedule

### 3.3 Configure Data Access Control Lists
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] ⚙️ No Cloud Storage buckets granting `allUsers` or `allAuthenticatedUsers` → [V27](cis-ig1-cli-validation.md#v27)
- [ ] 🔍 `constraints/storage.publicAccessPrevention` enforced at org level → [V28](cis-ig1-cli-validation.md#v28)
- [ ] ⚙️ **↺ existing estate:** all currently public buckets and datasets found and remediated → [V29](cis-ig1-cli-validation.md#v29)
- [ ] ⚙️ **↺ existing estate:** all buckets migrated off legacy per-object ACLs → [V30](cis-ig1-cli-validation.md#v30)
- [ ] ⚙️ **↺ existing estate:** all pre-existing external-domain IAM grants reviewed and removed → [V31](cis-ig1-cli-validation.md#v31)
- [ ] 🔍 `constraints/storage.uniformBucketLevelAccess` enforced; legacy object ACLs retired → [V32](cis-ig1-cli-validation.md#v32)
- [ ] ⚙️ No BigQuery datasets shared to `allUsers` or `allAuthenticatedUsers` → [V33](cis-ig1-cli-validation.md#v33)
- [ ] ⚙️ Access granted via groups and predefined/custom roles, not basic roles → [V34](cis-ig1-cli-validation.md#v34)
- [ ] 🔍 `constraints/iam.allowedPolicyMemberDomains` enforced at org level → [V35](cis-ig1-cli-validation.md#v35)
- [ ] 🔍 VPC Service Controls perimeters around projects holding regulated data → [V36](cis-ig1-cli-validation.md#v36)
- [ ] 👥 Access reviewed on a defined cadence with evidence retained

### 3.4 Enforce Data Retention
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Retention periods defined per data class and documented
- [ ] ⚙️ Object Lifecycle Management rules applied to Cloud Storage buckets → [V37](cis-ig1-cli-validation.md#v37)
- [ ] ⚙️ BigQuery default table and partition expiration configured on datasets → [V38](cis-ig1-cli-validation.md#v38)
- [ ] 🔍 Cloud SQL backup retention windows set to the defined period → [V39](cis-ig1-cli-validation.md#v39)
- [ ] 🔍 Log bucket retention set explicitly (not left at the `_Default` 30 days) → [V40](cis-ig1-cli-validation.md#v40)
- [ ] 🔍 Buckets and datasets with no retention rule identified and remediated → [V41](cis-ig1-cli-validation.md#v41)

### 3.5 Securely Dispose of Data
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Documented disposal process for each storage service in use
- [ ] 🔍 Cloud Storage soft delete / versioning retention understood and accounted for in disposal → [V42](cis-ig1-cli-validation.md#v42)
- [ ] 👥 CMEK key destruction procedure documented where crypto-shredding is the disposal method
- [ ] 👥 Snapshots, images, and backups included in disposal scope, not just live data
- [ ] 👥 Project deletion procedure includes verification that data is unrecoverable after the recovery window
- [ ] 👥 Disposal actions logged and evidenced

---

## Control 04 — Secure Configuration of Enterprise Assets and Software

### 4.1 Establish and Maintain a Secure Configuration Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Written secure configuration baseline for GCP resources
- [ ] 🔍 Baseline enforced through Organization Policy constraints at the **org node** → [V43](cis-ig1-cli-validation.md#v43)
- [ ] ⚙️ Constraints applied in dry-run first, violations triaged, then enforced → [V44](cis-ig1-cli-validation.md#v44)
- [ ] 👥 Infrastructure provisioned via infrastructure-as-code, not console clickops
- [ ] 👥 Drift detection running against declared state
- [ ] 🔍 Security Command Center enabled at org scope for continuous posture assessment → [V45](cis-ig1-cli-validation.md#v45)
- [ ] 👥 Baseline reviewed annually

### 4.2 Establish and Maintain a Secure Configuration Process for Network Infrastructure
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] ⚙️ Default VPC network removed from all projects → [V46](cis-ig1-cli-validation.md#v46)
- [ ] 🔍 `constraints/compute.skipDefaultNetworkCreation` enforced → [V47](cis-ig1-cli-validation.md#v47)
- [ ] 🔍 **↺ existing estate:** default network deleted from every existing project → [V48](cis-ig1-cli-validation.md#v48)
- [ ] ⚙️ **↺ existing estate:** legacy and auto-mode VPCs identified and converted → [V49](cis-ig1-cli-validation.md#v49)
- [ ] ⚙️ Default firewall rules deleted (`default-allow-ssh`, `default-allow-rdp`, `default-allow-icmp`, `default-allow-internal`) → [V50](cis-ig1-cli-validation.md#v50)
- [ ] 🔍 Auto-mode VPCs converted to custom-mode; no legacy networks remain → [V51](cis-ig1-cli-validation.md#v51)
- [ ] 👥 Documented network baseline covering subnets, routes, peering, and firewall standards
- [ ] 🔍 `constraints/compute.restrictVpcPeering` and Shared VPC constraints applied as required → [V52](cis-ig1-cli-validation.md#v52)
- [ ] ⚙️ Firewall rules logging enabled on rules governing sensitive paths → [V53](cis-ig1-cli-validation.md#v53)

### 4.3 Configure Automatic Session Locking on Enterprise Assets
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🖥️ Google Cloud session length configured in Admin Console (reauthentication frequency set, not "never")
- [ ] 🖥️ Reauthentication policy applied to Google Cloud CLI and API access
- [ ] 🔍 IAP session duration configured for IAP-protected resources → [V54](cis-ig1-cli-validation.md#v54)
- [ ] 🖥️ Shell idle timeout set in the baked VM image for interactive sessions

### 4.4 Implement and Manage a Firewall on Servers
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] ⚙️ No firewall rules permitting `0.0.0.0/0` to TCP 22 or TCP 3389 → [V55](cis-ig1-cli-validation.md#v55)
- [ ] ⚙️ No firewall rules permitting `0.0.0.0/0` to database ports (3306, 5432, 1433, 27017, 6379) → [V56](cis-ig1-cli-validation.md#v56)
- [ ] 🔍 Default-deny ingress posture with explicit allow rules → [V57](cis-ig1-cli-validation.md#v57)
- [ ] 🔍 Egress rules constrained rather than default allow-all → [V58](cis-ig1-cli-validation.md#v58)
- [ ] ⚙️ Rules scoped by network tag or service account rather than broad IP ranges → [V59](cis-ig1-cli-validation.md#v59)
- [ ] 🔍 Cloud SQL public IP disabled (`constraints/sql.restrictPublicIp`); authorized networks not `0.0.0.0/0` → [V60](cis-ig1-cli-validation.md#v60)
- [ ] ⚙️ **↺ existing estate:** existing Cloud SQL instances with public IP remediated → [V61](cis-ig1-cli-validation.md#v61)
- [ ] 🔍 **↺ existing estate:** existing firewall rules exposing 22/3389/DB ports to `0.0.0.0/0` removed → [V62](cis-ig1-cli-validation.md#v62)
- [ ] 🔍 GKE control plane authorized networks configured; private clusters in use → [V63](cis-ig1-cli-validation.md#v63)
- [ ] ⚙️ Cloud Armor policies applied to externally exposed load balancers → [V64](cis-ig1-cli-validation.md#v64)

### 4.6 Securely Manage Enterprise Assets and Software
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 OS Login enforced org-wide (`constraints/compute.requireOsLogin`); project-wide SSH keys retired → [V65](cis-ig1-cli-validation.md#v65)
- [ ] ⚙️ **↺ existing estate:** existing VMs without OS Login identified and remediated → [V66](cis-ig1-cli-validation.md#v66)
- [ ] ⚙️ **↺ existing estate:** project-wide SSH keys removed from every existing project → [V67](cis-ig1-cli-validation.md#v67)
- [ ] ⚙️ **↺ existing estate:** existing VMs with external IPs migrated behind IAP → [V68](cis-ig1-cli-validation.md#v68)
- [ ] ⚙️ **↺ existing estate:** existing non-Shielded VMs replaced (requires stop or rebuild) → [V69](cis-ig1-cli-validation.md#v69)
- [ ] 🔍 Serial port access disabled (`constraints/compute.disableSerialPortAccess`) → [V70](cis-ig1-cli-validation.md#v70)
- [ ] 🔍 External IPs restricted (`constraints/compute.vmExternalIpAccess`) → [V71](cis-ig1-cli-validation.md#v71)
- [ ] 🔍 IAP TCP forwarding used for SSH/RDP; no public-IP bastion hosts → [V72](cis-ig1-cli-validation.md#v72)
- [ ] 🔍 Shielded VM enforced (`constraints/compute.requireShieldedVm`) → [V73](cis-ig1-cli-validation.md#v73)
- [ ] 🔍 IP forwarding restricted (`constraints/compute.vmCanIpForward`) → [V74](cis-ig1-cli-validation.md#v74)
- [ ] 🔍 GKE hardening: Workload Identity on, legacy ABAC off, basic auth off, client cert auth off, legacy metadata endpoints off, Shielded GKE nodes on → [V75](cis-ig1-cli-validation.md#v75)
- [ ] 👥 Administrative changes made through IaC pipelines with review, not ad hoc console access
- [ ] ⚙️ Secrets held in Secret Manager, not in instance metadata, environment variables, or repositories → [V76](cis-ig1-cli-validation.md#v76)

### 4.7 Manage Default Accounts on Enterprise Assets and Software
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] ⚙️ Default Compute Engine service account stripped of `roles/editor` in every project → [V77](cis-ig1-cli-validation.md#v77)
- [ ] ⚙️ Default App Engine service account privileges reduced → [V78](cis-ig1-cli-validation.md#v78)
- [ ] 🔍 `constraints/iam.automaticIamGrantsForDefaultServiceAccounts` enforced → [V79](cis-ig1-cli-validation.md#v79)
- [ ] 🔍 **↺ existing estate:** `roles/editor` stripped from default service accounts in every existing project → [V80](cis-ig1-cli-validation.md#v80)
- [ ] ⚙️ **↺ existing estate:** workloads still running as a default service account migrated → [V81](cis-ig1-cli-validation.md#v81)
- [ ] 🔍 Workloads run as purpose-built service accounts, not the default → [V82](cis-ig1-cli-validation.md#v82)
- [ ] 🔍 Default GKE node service account replaced or scoped down → [V83](cis-ig1-cli-validation.md#v83)
- [ ] 🔍 Cloud SQL default database users reviewed and passwords rotated → [V84](cis-ig1-cli-validation.md#v84)
- [ ] 🔍 Default network and default firewall rules removed (cross-reference 4.2) → [V85](cis-ig1-cli-validation.md#v85)

---

## Control 05 — Account Management

### 5.1 Establish and Maintain an Inventory of Accounts
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Full IAM principal inventory across org, folder, project, and resource-level bindings → [V86](cis-ig1-cli-validation.md#v86)
- [ ] 🔍 Conditional IAM bindings included in the inventory → [V87](cis-ig1-cli-validation.md#v87)
- [ ] 🔍 Service account inventory maintained with owner and purpose per account → [V88](cis-ig1-cli-validation.md#v88)
- [ ] 🖥️ Super admin accounts in Cloud Identity / Workspace enumerated and justified
- [ ] 🔍 External (non-domain) principals with any binding identified → [V89](cis-ig1-cli-validation.md#v89)
- [ ] 🖥️ Inventory generated automatically and reviewed at least every six months

### 5.2 Use Unique Passwords
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 No shared or generic human accounts in Cloud Identity / Workspace
- [ ] 🔍 `constraints/iam.disableServiceAccountKeyCreation` enforced at org level → [V90](cis-ig1-cli-validation.md#v90)
- [ ] ⚙️ **↺ existing estate:** every pre-existing user-managed key inventoried — *this is the actual exposure, not new key creation* → [V91](cis-ig1-cli-validation.md#v91)
- [ ] 🔍 **↺ existing estate:** unused keys deleted; in-use keys replaced with federation and then deleted → [V92](cis-ig1-cli-validation.md#v92)
- [ ] 🔍 Existing user-managed service account keys inventoried, aged, and eliminated → [V93](cis-ig1-cli-validation.md#v93)
- [ ] 🔍 Workload Identity Federation / Workload Identity used in place of exported key files → [V94](cis-ig1-cli-validation.md#v94)
- [ ] 🔍 Any remaining static credentials stored in Secret Manager with rotation configured → [V95](cis-ig1-cli-validation.md#v95)
- [ ] 👥 Break-glass credentials uniquely held, sealed, and their use alerted on

### 5.3 Disable Dormant Accounts
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🖥️ Dormancy threshold defined (e.g. 45 days without authentication)
- [ ] 🔍 Human account activity assessed via Cloud Audit Logs / Admin Console reports → [V96](cis-ig1-cli-validation.md#v96)
- [ ] 🔍 Service account activity assessed via IAM activity analyser and authentication logs → [V97](cis-ig1-cli-validation.md#v97)
- [ ] 🔍 Unused service accounts disabled before deletion, then deleted → [V98](cis-ig1-cli-validation.md#v98)
- [ ] 🖥️ Dormant account review runs on a recurring schedule, not on request
- [ ] 🖥️ Departed-employee bindings removed as part of the leaver process

### 5.4 Restrict Administrator Privileges to Dedicated Administrator Accounts
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] ⚙️ No `roles/owner` or `roles/editor` granted to individual users at org or folder level → [V99](cis-ig1-cli-validation.md#v99)
- [ ] 🔍 **↺ existing estate:** all pre-existing basic-role grants to individuals enumerated and replaced → [V100](cis-ig1-cli-validation.md#v100)
- [ ] 🔍 Basic roles replaced with predefined or custom roles throughout → [V101](cis-ig1-cli-validation.md#v101)
- [ ] 👥 Privileged access held on dedicated admin identities, separate from daily-use accounts
- [ ] 👥 Super admin accounts not used for routine work
- [ ] 🔍 Privileged Access Manager or IAM Conditions used for just-in-time elevation → [V102](cis-ig1-cli-validation.md#v102)
- [ ] 🔍 IAM Recommender findings for over-granted roles reviewed and actioned → [V103](cis-ig1-cli-validation.md#v103)
- [ ] 🔍 Alerting configured on org-level IAM policy changes → [V104](cis-ig1-cli-validation.md#v104)

---

## Control 06 — Access Control Management

### 6.1 Establish an Access Granting Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Documented process for granting access to GCP, with approval step recorded
- [ ] 🔍 Access granted through group membership rather than individual bindings → [V105](cis-ig1-cli-validation.md#v105)
- [ ] 👥 Role selection follows least privilege with a defined role catalogue
- [ ] 👥 Requests and approvals retained as evidence
- [ ] 👥 New starter provisioning integrated with the identity source

### 6.2 Establish an Access Revoking Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Documented revocation procedure covering leavers and role changes
- [ ] 👥 Procedure covers IAM bindings, group membership, service account keys, and OAuth grants
- [ ] 👥 Session and token invalidation included, not just binding removal
- [ ] 👥 Defined SLA from trigger event to completed revocation
- [ ] 👥 Revocation completion verified and evidenced, not assumed
- [ ] 👥 Periodic reconciliation between HR/IdP leavers and remaining GCP access

### 6.3 Require MFA for Externally-Exposed Applications
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Externally exposed applications enumerated → [V106](cis-ig1-cli-validation.md#v106)
- [ ] 🔍 Identity-Aware Proxy fronting internal applications reachable from the internet → [V107](cis-ig1-cli-validation.md#v107)
- [ ] 👥 MFA enforced at the identity provider for all IAP-protected access
- [ ] 👥 No application relying on IP allowlisting alone as its access control
- [ ] 🔍 Access Context Manager levels applied where device or location assurance is required → [V108](cis-ig1-cli-validation.md#v108)

### 6.4 Require MFA for Remote Network Access
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 SSH and RDP to VMs routed through IAP TCP forwarding → [V109](cis-ig1-cli-validation.md#v109)
- [ ] 🔍 No VM instances reachable on port 22 or 3389 from `0.0.0.0/0` → [V110](cis-ig1-cli-validation.md#v110)
- [ ] 👥 MFA enforced on the identity used for IAP access
- [ ] 🔍 Cloud VPN / Interconnect access paths documented and MFA-gated at the identity layer → [V111](cis-ig1-cli-validation.md#v111)
- [ ] 🔍 Remaining bastion hosts removed or placed behind IAP → [V112](cis-ig1-cli-validation.md#v112)

### 6.5 Require MFA for Administrative Access
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🖥️ 2-Step Verification **enforced** (not merely available) for all Cloud Identity / Workspace users
- [ ] 🖥️ Phishing-resistant methods (security keys or passkeys) required for super admins
- [ ] 🔍 Phishing-resistant methods required for any principal holding org or folder-level admin roles → [V113](cis-ig1-cli-validation.md#v113)
- [ ] 🖥️ MFA verified on the federated IdP path if SSO is in use
- [ ] 🖥️ Enforcement confirmed by report, not by policy statement alone
- [ ] 🖥️ Break-glass accounts covered by a documented MFA exception with compensating monitoring

---

## Control 07 — Continuous Vulnerability Management

### 7.1 Establish and Maintain a Vulnerability Management Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Documented vulnerability management process covering the GCP estate
- [ ] 🔍 Security Command Center enabled at org scope with tier recorded → [V114](cis-ig1-cli-validation.md#v114)
- [ ] 👥 Vulnerability sources defined (SCC, Artifact Analysis, VM Manager, vendor advisories)
- [ ] 👥 Roles and responsibilities named
- [ ] 👥 Process reviewed annually

### 7.2 Establish and Maintain a Remediation Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Remediation SLAs defined by severity
- [ ] 🔍 Findings routed to an owning team automatically, not surfaced only in a console → [V115](cis-ig1-cli-validation.md#v115)
- [ ] 👥 Risk acceptance process with expiry dates for exceptions
- [ ] 👥 Findings tracked to closure with evidence
- [ ] 👥 Monthly review of open findings against SLA

### 7.3 Perform Automated Operating System Patch Management
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 VM Manager patch deployments configured on a recurring schedule → [V116](cis-ig1-cli-validation.md#v116)
- [ ] 🔍 OS Config agent present on all instances; coverage gaps identified and closed → [V117](cis-ig1-cli-validation.md#v117)
- [ ] 🔍 Patch compliance reporting reviewed → [V118](cis-ig1-cli-validation.md#v118)
- [ ] 🖥️ Image rebake cadence defined so new instances launch pre-patched
- [ ] 🔍 Instance templates and MIGs updated to reference current images → [V119](cis-ig1-cli-validation.md#v119)
- [ ] ⚙️ GKE node auto-upgrade enabled on all node pools → [V120](cis-ig1-cli-validation.md#v120)
- [ ] 🔍 Cloud SQL maintenance windows configured with automatic minor version updates → [V121](cis-ig1-cli-validation.md#v121)

### 7.4 Perform Automated Application Patch Management
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Artifact Analysis vulnerability scanning enabled on Artifact Registry → [V122](cis-ig1-cli-validation.md#v122)
- [ ] 👥 Container base image update process defined and automated
- [ ] 👥 Application dependency scanning in the build pipeline
- [ ] 🔍 Cloud Functions and Cloud Run redeployed onto supported runtimes on a schedule → [V123](cis-ig1-cli-validation.md#v123)
- [ ] 🔍 Web Security Scanner run against externally exposed applications → [V124](cis-ig1-cli-validation.md#v124)
- [ ] 👥 Rebuild-and-redeploy is the patching mechanism, not in-place modification of running workloads

---

## Control 08 — Audit Log Management

### 8.1 Establish and Maintain an Audit Log Management Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Written audit log management process with stated retention periods per log type
- [ ] 👥 Log sources in scope enumerated
- [ ] 👥 Ownership of the logging pipeline assigned
- [ ] 🔍 Log exclusion filters documented and justified — no security-relevant events excluded for cost → [V125](cis-ig1-cli-validation.md#v125)
- [ ] 👥 Process reviewed annually

### 8.2 Collect Audit Logs
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Admin Activity audit logs confirmed flowing for all projects → [V126](cis-ig1-cli-validation.md#v126)
- [ ] 🔍 **Data Access audit logs enabled** (`ADMIN_READ`, `DATA_READ`, `DATA_WRITE`) — off by default → [V127](cis-ig1-cli-validation.md#v127)
- [ ] 🔍 **↺ not retroactive:** enabling starts the record now; pre-existing history is unrecoverable — note the gap start date as an audit finding → [V128](cis-ig1-cli-validation.md#v128)
- [ ] 🔍 System Event and Policy Denied logs captured → [V129](cis-ig1-cli-validation.md#v129)
- [ ] 🔍 Aggregated log sink configured at **organization** level with `includeChildren = true` → [V130](cis-ig1-cli-validation.md#v130)
- [ ] 🔍 Sink service account permissions verified — confirm logs are arriving at the destination → [V131](cis-ig1-cli-validation.md#v131)
- [ ] ⚙️ VPC Flow Logs enabled on subnets carrying sensitive traffic → [V132](cis-ig1-cli-validation.md#v132)
- [ ] 🔍 Cloud DNS logging, Cloud NAT logging, and firewall rules logging enabled → [V133](cis-ig1-cli-validation.md#v133)
- [ ] 🔍 GKE audit logs and Cloud SQL logs captured → [V134](cis-ig1-cli-validation.md#v134)
- [ ] ⚙️ Load balancer and Cloud Armor request logging enabled → [V135](cis-ig1-cli-validation.md#v135)
- [ ] 🔍 Alerting on org policy changes, IAM changes, service account key creation, custom role changes, and audit config changes → [V136](cis-ig1-cli-validation.md#v136)

### 8.3 Ensure Adequate Audit Log Storage
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Log bucket retention set explicitly to the defined period (not the 30-day `_Default`) → [V137](cis-ig1-cli-validation.md#v137)
- [ ] 👥 Sink destination sized and budgeted for the retention period
- [ ] 🔍 Logs stored in a **dedicated logging project** with IAM separated from workload projects → [V138](cis-ig1-cli-validation.md#v138)
- [ ] 🔍 Bucket Lock retention policy applied to the log destination for tamper resistance → [V139](cis-ig1-cli-validation.md#v139)
- [ ] 🔍 No workload-project principals hold delete permission on log storage → [V140](cis-ig1-cli-validation.md#v140)
- [ ] 🔍 Storage capacity and cost monitored with alerting before ingestion is throttled or dropped → [V141](cis-ig1-cli-validation.md#v141)

---

## Control 09 — Email and Web Browser Protections

### 9.2 Use DNS Filtering Services
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Cloud DNS response policies applied to VPC networks to block known-malicious domains → [V142](cis-ig1-cli-validation.md#v142)
- [ ] 🔍 Workload egress routed through Cloud NAT or Secure Web Proxy rather than per-instance external IPs → [V143](cis-ig1-cli-validation.md#v143)
- [ ] 🔍 Egress firewall rules constrain outbound destinations → [V144](cis-ig1-cli-validation.md#v144)
- [ ] 🔍 Cloud DNS logging enabled for visibility into resolution behaviour → [V145](cis-ig1-cli-validation.md#v145)
- [ ] 🔍 Blocked-resolution events surfaced to a monitored destination → [V146](cis-ig1-cli-validation.md#v146)

*Safeguard 9.1 is out of GCP scope — see appendix.*

---

## Control 10 — Malware Defenses

### 10.1 Deploy and Maintain Anti-Malware Software
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🖥️ Anti-malware agent decision documented per workload class, with rationale where not deployed
- [ ] 🖥️ Agent baked into the golden image rather than installed post-boot
- [ ] ⚙️ Agent presence verifiable through OS inventory so coverage gaps are detectable → [V147](cis-ig1-cli-validation.md#v147)
- [ ] 🔍 Container image malware scanning in place for containerised workloads → [V148](cis-ig1-cli-validation.md#v148)
- [ ] 🔍 Binary Authorization preventing unattested images reaching GKE / Cloud Run → [V149](cis-ig1-cli-validation.md#v149)
- [ ] ⚙️ Shielded VM secure boot and integrity monitoring enabled → [V150](cis-ig1-cli-validation.md#v150)
- [ ] 🖥️ Malware scanning on Cloud Storage buckets accepting untrusted uploads

### 10.2 Configure Automatic Anti-Malware Signature Updates
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🖥️ Automatic signature updates enabled on deployed agents
- [ ] 🔍 Update path reachable from instances with restricted egress — verified, not assumed → [V151](cis-ig1-cli-validation.md#v151)
- [ ] 🖥️ Signature currency monitored, with alerting on stale definitions
- [ ] 🖥️ Container and registry scanning definitions maintained by the platform provider or updated on schedule

*Safeguard 10.3 is out of GCP scope — see appendix.*

---

## Control 11 — Data Recovery

### 11.1 Establish and Maintain a Data Recovery Process
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Written recovery process covering each data-bearing service in use
- [ ] 👥 RPO and RTO defined per workload tier
- [ ] 👥 Recovery roles and escalation path named
- [ ] 🔍 Terraform state backend versioning and recovery included in scope → [V152](cis-ig1-cli-validation.md#v152)
- [ ] 👥 Process reviewed annually and after any recovery event

### 11.2 Perform Automated Backups
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Cloud SQL automated backups enabled with point-in-time recovery → [V153](cis-ig1-cli-validation.md#v153)
- [ ] ⚙️ Persistent disk snapshot schedules attached to all data-bearing disks → [V154](cis-ig1-cli-validation.md#v154)
- [ ] 🔍 Cloud Storage versioning and soft delete enabled on data-bearing buckets → [V155](cis-ig1-cli-validation.md#v155)
- [ ] 🔍 Backup for GKE configured where GKE holds persistent state → [V156](cis-ig1-cli-validation.md#v156)
- [ ] 🔍 Firestore, Bigtable, Spanner, Filestore backups configured where in use → [V157](cis-ig1-cli-validation.md#v157)
- [ ] 🔍 Backup success and failure monitored with alerting — silent failure detected → [V158](cis-ig1-cli-validation.md#v158)
- [ ] 👥 Backup frequency meets the defined RPO

### 11.3 Protect Recovery Data
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Backup data encrypted, with CMEK where key control is required → [V159](cis-ig1-cli-validation.md#v159)
- [ ] 🔍 KMS key access separated from production workload identities → [V160](cis-ig1-cli-validation.md#v160)
- [ ] 🔍 IAM on backup storage restricted to a dedicated backup role → [V161](cis-ig1-cli-validation.md#v161)
- [ ] 🔍 No production workload service account holds delete permission on backups → [V162](cis-ig1-cli-validation.md#v162)
- [ ] 🔍 **Bucket Lock** retention policy applied to backup buckets (WORM) → [V163](cis-ig1-cli-validation.md#v163)
- [ ] 🔍 Backup deletion events logged and alerted on → [V164](cis-ig1-cli-validation.md#v164)

### 11.4 Establish and Maintain an Isolated Instance of Recovery Data
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 At least one backup copy held in a **separate project** from the production workload → [V165](cis-ig1-cli-validation.md#v165)
- [ ] 🔍 Backup project sits under a separate folder with distinct IAM inheritance → [V166](cis-ig1-cli-validation.md#v166)
- [ ] ⚙️ No shared credential can both access production and delete the isolated copy → [V167](cis-ig1-cli-validation.md#v167)
- [ ] 🔍 Copy held in a different region, or in multi-region storage → [V168](cis-ig1-cli-validation.md#v168)
- [ ] 🖥️ Isolation verified by test — attempt access with a production identity and confirm denial
- [ ] 🖥️ Restore from the isolated copy tested and dated

---

## Control 12 — Network Infrastructure Management

### 12.1 Ensure Network Infrastructure is Up-to-Date
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 GKE control plane and node versions within the supported window → [V169](cis-ig1-cli-validation.md#v169)
- [ ] 🔍 Classic VPN migrated to HA VPN; tunnels using current IKE version and ciphers → [V170](cis-ig1-cli-validation.md#v170)
- [ ] 🔍 Load balancer SSL policies set to a modern TLS minimum; TLS 1.0/1.1 and weak ciphers disabled → [V171](cis-ig1-cli-validation.md#v171)
- [ ] 🔍 Legacy (non-Application) load balancers migrated → [V172](cis-ig1-cli-validation.md#v172)
- [ ] 🔍 Legacy networks eliminated; auto-mode VPCs converted to custom-mode → [V173](cis-ig1-cli-validation.md#v173)
- [ ] 🔍 Cloud Armor rule sets current → [V174](cis-ig1-cli-validation.md#v174)
- [ ] 👥 Deprecated Compute Engine API versions removed from tooling and IaC
- [ ] 👥 Network topology diagram current, including Shared VPC and peering relationships

---

## Control 15 — Service Provider Management

### 15.1 Establish and Maintain an Inventory of Service Providers
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Google Cloud recorded as a service provider with the shared responsibility boundary documented
- [ ] 👥 Marketplace-deployed solutions and their vendors enumerated
- [ ] 🔍 Third-party service accounts with access into the Organization inventoried → [V175](cis-ig1-cli-validation.md#v175)
- [ ] 🔍 Workload Identity Federation trust relationships to external IdPs documented → [V176](cis-ig1-cli-validation.md#v176)
- [ ] 👥 OAuth applications authorised against the Cloud Identity / Workspace tenant reviewed
- [ ] 🔍 Cross-project and cross-organization IAM grants identified → [V177](cis-ig1-cli-validation.md#v177)
- [ ] 🔍 `constraints/iam.allowedPolicyMemberDomains` enforced to prevent undocumented external grants → [V178](cis-ig1-cli-validation.md#v178)
- [ ] 🔍 **↺ existing estate:** external grants predating the constraint enumerated and documented or removed → [V179](cis-ig1-cli-validation.md#v179)
- [ ] 👥 Inventory reviewed at least annually

---

## Control 17 — Incident Response Management

### 17.1 Designate Personnel to Manage Incident Handling
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 👥 Named individual accountable for incident handling covering the GCP Organization
- [ ] 👥 Deputy or escalation path named for coverage
- [ ] 🔍 Break-glass access procedure documented, tested, and alerted on → [V180](cis-ig1-cli-validation.md#v180)
- [ ] 👥 Google Cloud support plan level recorded and adequate for incident escalation
- [ ] 👥 Designation reviewed annually

### 17.2 Establish and Maintain Contact Information for Reporting Security Incidents
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 **Essential Contacts configured at organization level for the Security category** → [V181](cis-ig1-cli-validation.md#v181)
- [ ] 🔍 Essential Contacts configured for Legal, Suspension, and Technical categories → [V182](cis-ig1-cli-validation.md#v182)
- [ ] 🔍 Contacts point to monitored group addresses, not individuals → [V183](cis-ig1-cli-validation.md#v183)
- [ ] 🔍 **↺ existing estate:** pre-existing contacts (personal addresses, departed employees) reviewed and replaced → [V184](cis-ig1-cli-validation.md#v184)
- [ ] 🖥️ Cloud Billing account contacts current
- [ ] 🔍 Cloud Monitoring notification channels verified — no departed-employee addresses → [V185](cis-ig1-cli-validation.md#v185)
- [ ] 🖥️ Contacts verified by test message on a defined cadence

### 17.3 Establish and Maintain an Enterprise Process for Reporting Incidents
**Status:** `[ ] Compliant`  `[ ] PR Submitted`  `[ ] Not Compliant`

- [ ] 🔍 Security Command Center findings routed to a monitored destination via Pub/Sub → [V186](cis-ig1-cli-validation.md#v186)
- [ ] ⚙️ Log-based alerts route to an on-call rotation, not an unmonitored mailbox → [V187](cis-ig1-cli-validation.md#v187)
- [ ] 👥 Documented path for a platform engineer to raise a suspected incident
- [ ] 👥 Handoff defined between the GCP platform team and the enterprise incident function
- [ ] 🔍 Log retention sufficient to support investigation (dependency on 8.3) → [V188](cis-ig1-cli-validation.md#v188)
- [ ] 👥 Reporting path tested and dated

---

## Appendix A — Safeguards excluded from this checklist

These twelve IG1 safeguards have no GCP surface. They remain part of the enterprise's IG1 obligation and are owned outside the platform team. Recorded here so the exclusion is explicit and auditable.

| Safeguard | Title | Owner |
|---|---|---|
| 3.6 | Encrypt Data on End-User Devices | Endpoint management |
| 4.5 | Implement and Manage a Firewall on End-User Devices | Endpoint management |
| 9.1 | Ensure Use of Only Fully Supported Browsers and Email Clients | Endpoint management |
| 10.3 | Disable Autorun and Autoplay for Removable Media | Endpoint management |
| 14.1 | Establish and Maintain a Security Awareness Program | Security awareness function |
| 14.2 | Train Workforce Members to Recognize Social Engineering Attacks | Security awareness function |
| 14.3 | Train Workforce Members on Authentication Best Practices | Security awareness function |
| 14.4 | Train Workforce on Data Handling Best Practices | Security awareness function |
| 14.5 | Train Workforce Members on Causes of Unintentional Data Exposure | Security awareness function |
| 14.6 | Train Workforce Members on Recognizing and Reporting Security Incidents | Security awareness function |
| 14.7 | Train Workforce on How to Identify and Report Missing Security Updates | Security awareness function |
| 14.8 | Train Workforce on the Dangers of Insecure Networks | Security awareness function |

Controls **13** (Network Monitoring and Defense), **16** (Application Software Security), and **18** (Penetration Testing) contain no IG1 safeguards at all and therefore generate no items in either direction.

## Appendix B — Assessment summary

| Control | Safeguards assessed | Compliant | Not compliant |
|---|---|---|---|
| 01 Inventory and Control of Enterprise Assets | 2 | | |
| 02 Inventory and Control of Software Assets | 3 | | |
| 03 Data Protection | 5 | | |
| 04 Secure Configuration | 6 | | |
| 05 Account Management | 4 | | |
| 06 Access Control Management | 5 | | |
| 07 Continuous Vulnerability Management | 4 | | |
| 08 Audit Log Management | 3 | | |
| 09 Email and Web Browser Protections | 1 | | |
| 10 Malware Defenses | 2 | | |
| 11 Data Recovery | 4 | | |
| 12 Network Infrastructure Management | 1 | | |
| 15 Service Provider Management | 1 | | |
| 17 Incident Response Management | 3 | | |
| **Total** | **44** | | |

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This checklist is an implementation aid and is not affiliated with or endorsed by CIS. Refer to the official CIS Controls v8.1 publication for authoritative safeguard text.*
