# Process and People — interview worksheet

The 72 IG1 requirements that no script can answer. They are about **how the estate is run**, not how it is configured, and the fastest way to answer them is to ask people who know the account.

Take this into a room with whoever has history on the engagement. Most of these are a two-minute conversation each; a few will surface real gaps.

## How to use it

For each requirement, you are establishing one of three things:

- **Yes, and here is the evidence** — name the document, the ticket, the runbook. Record where it lives.
- **Yes, but nothing is written down** — the practice exists in people's heads. That is **not compliant**. IG1 asks for a documented process, and an undocumented one cannot survive the person leaving.
- **No** — a finding, and usually a quick one to close.

**The most common failure in this group is a missing date, not a missing document.** A process nobody has reviewed cannot be shown to be current. When someone produces a document, ask when it was last reviewed before you tick anything.

**Ask who owns it, not just whether it exists.** "Somebody does that" is the answer that turns into a finding six months later.

---


## Knowing what exists

### 2.1 — Establish and Maintain a Software Inventory
- [ ] **`2.1#7`** Inventory refreshed automatically, not by manual survey
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 2.2 — Ensure Authorized Software is Currently Supported
- [ ] **`2.2#6`** Exception register for unsupported software with documented compensating controls
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 2.3 — Address Unauthorized Software
- [ ] **`2.3#1`** Documented process for removing unapproved software from the environment
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`2.3#2`** Marketplace deployment restricted to approved solutions
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`2.3#6`** Findings tracked to closure with a defined remediation window
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Data handling

### 3.1 — Establish and Maintain a Data Management Process
- [ ] **`3.1#1`** Written data management process covering sensitivity, ownership, handling, retention, disposal
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`3.1#2`** Process references GCP storage services in use
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`3.1#4`** Reviewed annually with review date recorded
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 3.3 — Configure Data Access Control Lists
- [ ] **`3.3#11`** Access reviewed on a defined cadence with evidence retained
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 3.4 — Enforce Data Retention
- [ ] **`3.4#1`** Retention periods defined per data class and documented
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 3.5 — Securely Dispose of Data
- [ ] **`3.5#1`** Documented disposal process for each storage service in use
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`3.5#3`** CMEK key destruction procedure documented where crypto-shredding is the disposal method
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`3.5#4`** Snapshots, images, and backups included in disposal scope, not just live data
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`3.5#5`** Project deletion procedure includes verification that data is unrecoverable after the recovery window
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`3.5#6`** Disposal actions logged and evidenced
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## How configuration gets made and kept

### 4.1 — Establish and Maintain a Secure Configuration Process
- [ ] **`4.1#1`** Written secure configuration baseline for GCP resources
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`4.1#4`** Infrastructure provisioned via infrastructure-as-code, not console clickops
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`4.1#5`** Drift detection running against declared state
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`4.1#7`** Baseline reviewed annually
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 4.2 — Establish and Maintain a Secure Configuration Process for Network Infrastructure
- [ ] **`4.2#7`** Documented network baseline covering subnets, routes, peering, and firewall standards
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 4.6 — Securely Manage Enterprise Assets and Software
- [ ] **`4.6#12`** Administrative changes made through IaC pipelines with review, not ad hoc console access
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Accounts and access

### 5.2 — Use Unique Passwords
- [ ] **`5.2#1`** No shared or generic human accounts in Cloud Identity / Workspace
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`5.2#8`** Break-glass credentials uniquely held, sealed, and their use alerted on
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 5.4 — Restrict Administrator Privileges to Dedicated Administrator Accounts
- [ ] **`5.4#4`** Privileged access held on dedicated admin identities, separate from daily-use accounts
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`5.4#5`** Super admin accounts not used for routine work
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 6.1 — Establish an Access Granting Process
- [ ] **`6.1#1`** Documented process for granting access to GCP, with approval step recorded
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.1#3`** Role selection follows least privilege with a defined role catalogue
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.1#4`** Requests and approvals retained as evidence
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.1#5`** New starter provisioning integrated with the identity source
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 6.2 — Establish an Access Revoking Process
- [ ] **`6.2#1`** Documented revocation procedure covering leavers and role changes
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.2#2`** Procedure covers IAM bindings, group membership, service account keys, and OAuth grants
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.2#3`** Session and token invalidation included, not just binding removal
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.2#4`** Defined SLA from trigger event to completed revocation
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.2#5`** Revocation completion verified and evidenced, not assumed
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.2#6`** Periodic reconciliation between HR/IdP leavers and remaining GCP access
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 6.3 — Require MFA for Externally-Exposed Applications
- [ ] **`6.3#3`** MFA enforced at the identity provider for all IAP-protected access
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`6.3#4`** No application relying on IP allowlisting alone as its access control
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 6.4 — Require MFA for Remote Network Access
- [ ] **`6.4#3`** MFA enforced on the identity used for IAP access
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Patching and vulnerabilities

### 7.1 — Establish and Maintain a Vulnerability Management Process
- [ ] **`7.1#1`** Documented vulnerability management process covering the GCP estate
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.1#3`** Vulnerability sources defined (SCC, Artifact Analysis, VM Manager, vendor advisories)
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.1#4`** Roles and responsibilities named
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.1#5`** Process reviewed annually
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 7.2 — Establish and Maintain a Remediation Process
- [ ] **`7.2#1`** Remediation SLAs defined by severity
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.2#3`** Risk acceptance process with expiry dates for exceptions
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.2#4`** Findings tracked to closure with evidence
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.2#5`** Monthly review of open findings against SLA
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 7.4 — Perform Automated Application Patch Management
- [ ] **`7.4#2`** Container base image update process defined and automated
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.4#3`** Application dependency scanning in the build pipeline
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`7.4#6`** Rebuild-and-redeploy is the patching mechanism, not in-place modification of running workloads
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Logging

### 8.1 — Establish and Maintain an Audit Log Management Process
- [ ] **`8.1#1`** Written audit log management process with stated retention periods per log type
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`8.1#2`** Log sources in scope enumerated
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`8.1#3`** Ownership of the logging pipeline assigned
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`8.1#5`** Process reviewed annually
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 8.3 — Ensure Adequate Audit Log Storage
- [ ] **`8.3#2`** Sink destination sized and budgeted for the retention period
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Backup and recovery

### 11.1 — Establish and Maintain a Data Recovery Process
- [ ] **`11.1#1`** Written recovery process covering each data-bearing service in use
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`11.1#2`** RPO and RTO defined per workload tier
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`11.1#3`** Recovery roles and escalation path named
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`11.1#5`** Process reviewed annually and after any recovery event
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 11.2 — Perform Automated Backups
- [ ] **`11.2#7`** Backup frequency meets the defined RPO
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Network

### 12.1 — Ensure Network Infrastructure is Up-to-Date
- [ ] **`12.1#7`** Deprecated Compute Engine API versions removed from tooling and IaC
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`12.1#8`** Network topology diagram current, including Shared VPC and peering relationships
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Third parties

### 15.1 — Establish and Maintain an Inventory of Service Providers
- [ ] **`15.1#1`** Google Cloud recorded as a service provider with the shared responsibility boundary documented
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`15.1#2`** Marketplace-deployed solutions and their vendors enumerated
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`15.1#5`** OAuth applications authorised against the Cloud Identity / Workspace tenant reviewed
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`15.1#9`** Inventory reviewed at least annually
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

## Incidents

### 17.1 — Designate Personnel to Manage Incident Handling
- [ ] **`17.1#1`** Named individual accountable for incident handling covering the GCP Organization
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`17.1#2`** Deputy or escalation path named for coverage
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`17.1#4`** Google Cloud support plan level recorded and adequate for incident escalation
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`17.1#5`** Designation reviewed annually
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

### 17.3 — Establish and Maintain an Enterprise Process for Reporting Incidents
- [ ] **`17.3#3`** Documented path for a platform engineer to raise a suspected incident
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`17.3#4`** Handoff defined between the GCP platform team and the enterprise incident function
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________
- [ ] **`17.3#6`** Reporting path tested and dated
  - Owner: ____________  ·  Evidence: ____________  ·  Last reviewed: __________

---

## Before you leave the room

- [ ] Every item above has an owner named, or is recorded as unowned
- [ ] Every "yes" has evidence you could actually produce to an auditor
- [ ] Every document produced has a review date, not just an existence
- [ ] Anything answered "it's in people's heads" is marked **not compliant**

Transfer the results into `docs/cis-ig1-gcp-checklist.md` — these are the 👥 items.
