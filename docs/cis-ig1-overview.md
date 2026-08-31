# CIS Controls v8.1 — Implementation Group 1 (IG1) Overview

**Target environment:** Google Cloud Platform Organization
**Framework:** CIS Critical Security Controls v8.1 — Implementation Group 1
**Scope of this document:** All 18 Controls and all 56 IG1 Safeguards
**Document type:** Orientation / programme reference

---

## About this document

This is the *orientation* document. It maps the full IG1 framework — every Control, including the ones a cloud platform team cannot satisfy — so that future readers understand the whole picture and can see exactly where the GCP boundary sits.

The companion document, `cis-ig1-gcp-checklist.md`, is the *working* document. It contains only the safeguards with GCP-actionable requirements.

### Why the full framework is documented here

IG1 is defined by CIS as *essential cyber hygiene for the enterprise*, not for a single platform. It assumes an organisation with employees, laptops, email, and vendors. Of the 56 IG1 Safeguards, roughly a fifth have no GCP surface at all — they are workforce training, end-user device, and removable-media controls. Documenting them here (rather than silently dropping them) makes the ownership boundary explicit and prevents a future auditor from reading the checklist as a claim of full IG1 coverage.

### Legend

| Marker | Meaning |
|---|---|
| **GCP-owned** | The Control's IG1 safeguards are substantially satisfiable within the GCP Organization |
| **Shared** | Partially satisfiable in GCP; remainder sits with endpoint, identity, or people functions |
| **Out of GCP scope** | No meaningful GCP surface; owned elsewhere in the enterprise |
| **No IG1 safeguards** | The Control exists in v8.1 but contributes zero safeguards at IG1 |

### Permissive-default vs secure-by-default organizations

Two organizations can run identical workloads and start this audit from completely different positions, depending on when the Organization resource was created. This document uses two terms throughout to distinguish them:

**Permissive-default** — an Organization created before Google applied constraints automatically. Nothing is enforced at the org node: default networks carry SSH open to the internet, default service accounts hold `roles/editor`, and any principal with sufficient IAM can make a bucket public. Every project inherits an unrestricted default until a policy is explicitly set.

**Secure-by-default** — an Organization created recently enough that Google applies a set of security baseline constraints at creation. Several of the controls you would otherwise implement from scratch are enforced from day one.

Google changed the default sometime around 2024. The exact date matters less than the actual enforcement state, because posture is determined by what is set on *your* organization today — a permissive-default organization that has already been hardened is in better shape than a secure-by-default organization whose baseline was subsequently deleted.

#### Which one are you?

```
gcloud resource-manager org-policies list --organization=ORGANIZATION_ID
```

Empty or near-empty output indicates a permissive-default posture. A set of `iam.*`, `storage.*`, and `essentialcontacts.*` constraints you did not apply yourself indicates the baseline is present and active. **Run this before anything else** — it changes the size of the audit.

#### The baseline a secure-by-default organization inherits

If these appear and you did not apply them, they came from Google:

| Constraint | Related IG1 safeguard |
|---|---|
| `iam.managed.disableServiceAccountKeyCreation` | 5.2 |
| `iam.managed.disableServiceAccountKeyUpload` | 5.2 |
| `iam.automaticIamGrantsForDefaultServiceAccounts` | 4.7 |
| `iam.allowedPolicyMemberDomains` | 3.3, 15.1 |
| `essentialcontacts.managed.allowedContactDomains` | 17.2 |
| `compute.managed.restrictProtocolForwardingCreationForTypes` | 4.6 |
| `storage.uniformBucketLevelAccess` | 3.3 |

A permissive-default organization has **none** of these unless they were applied deliberately. That single difference accounts for a real share of the remediation work in Controls 3, 4, 5, 15, and 17.

#### Conditions common to permissive-default organizations

Every Control section below flags conditions worth checking for. The older the org and the less consistently it has been governed, the more will apply:

- **No org policy constraints** at the org node, so every project inherits an unrestricted default.
- **Projects created outside the folder hierarchy**, sitting flat directly under the org and outside any policy inheritance path.
- **Basic roles** (`roles/owner`, `roles/editor`, `roles/viewer`) used as the normal grant, where predefined or custom roles would now be expected.
- **Default networks** auto-created per project, carrying `default-allow-ssh`, `default-allow-rdp`, `default-allow-icmp`, and `default-allow-internal`.
- **Default service accounts** auto-granted `roles/editor` on their project.
- **Uniform bucket-level access**, **public access prevention**, **VPC Service Controls**, **Security Command Center**, **Essential Contacts**, and **OS Login** never enabled, because they postdate the org's creation or were not defaults at the time.

Treat these as the assumed starting position until you have checked. A secure-by-default organization, or one already under active governance, can skip to verification.

**Secure-by-default is not compliant.** The baseline covers seven constraints. This checklist covers 44 safeguards. It removes work; it does not remove the requirement.

#### And the constraints are not retroactive

Worth internalising before reading any further, because it shapes the whole remediation programme:

**An Organization Policy constraint blocks future non-conforming operations. It does nothing to what already exists.**

Applying `storage.publicAccessPrevention` to a permissive-default organization does not make one existing public bucket private. Applying `iam.disableServiceAccountKeyCreation` does not invalidate one already-exported key. The console will show the policy as enforced, and the violations will sit there untouched.

Every constraint-based remediation is therefore two pieces of work: apply the constraint to stop the bleeding, then find and fix the existing estate. The second half is usually the larger one, and it is the half that gets skipped. `cis-ig1-remediation-reference.md` gives a discovery command for each affected safeguard.

---

## Control 01 — Inventory and Control of Enterprise Assets

**IG1 safeguards:** 1.1, 1.2 (2 of 5) — **GCP-owned**

### Why this Control exists

You cannot defend, patch, monitor, or decommission an asset you do not know about. This Control is first in the framework because every subsequent Control is scoped by it — vulnerability management, logging, and access control are all applied *to an inventory*, and any gap in that inventory becomes a permanent blind spot rather than a deferred task. In cloud environments the failure mode is inverted from on-premise: assets are trivially easy to create and therefore accumulate silently, and the risk is not unknown hardware smuggled onto the network but forgotten projects, orphaned VMs, and abandoned proof-of-concept workloads still holding IAM grants and public IPs.

### Outcomes

A single authoritative, automatically refreshed record of every compute, storage, network, and identity resource across every project in the Organization, with a defined process for what happens when something appears in the environment that is not in the record.

### GCP infrastructure focus areas

- Cloud Asset Inventory enabled at **organization** scope, not per project, with a feed exporting to BigQuery or Cloud Storage
- Complete project enumeration, including projects created flat under the org node before the folder hierarchy existed
- Projects in `DELETE_REQUESTED` state, and projects with no active billing account, identified and dispositioned
- Compute Engine instances, GKE node pools, Cloud SQL instances, Cloud Run services, Cloud Functions, and App Engine versions all in inventory scope
- Orphaned resources: unattached persistent disks, reserved-but-unused static external IPs, idle load balancer forwarding rules, stale custom images
- Shared VPC host/service project relationships mapped so network assets are attributed to an owner
- Ownership metadata (labels for owner, cost centre, environment, data classification) enforced as a creation requirement

### Common permissive-default conditions

Projects created outside the folder hierarchy; long-lived "sandbox" and "test" projects with no current owner; resources predating any labelling standard.

---

## Control 02 — Inventory and Control of Software Assets

**IG1 safeguards:** 2.1, 2.2, 2.3 (3 of 7) — **GCP-owned**

### Why this Control exists

Unsupported and unauthorised software is the raw material of most successful intrusions. Software that has passed end-of-life receives no security patches, so a known vulnerability in it is permanent rather than temporary — the exposure window never closes. This Control exists to ensure that everything executing in the environment is both known and still receiving vendor security support, and that anything unknown is removed rather than tolerated.

### Outcomes

An inventory of operating systems, runtimes, and container base images in use, each confirmed to be within its supported lifecycle, and a working process for removing or isolating anything unsupported or unsanctioned.

### GCP infrastructure focus areas

- Compute Engine OS inventory via VM Manager, covering guest OS and installed package versions
- Machine image and golden image lineage — which images are approved, which are deprecated, and what is still running from a deprecated family
- Public image families in use that have passed end-of-support (CentOS 7, Debian 9/10, Ubuntu releases past their LTS window)
- GKE cluster and node pool versions against the supported release channel window; clusters on static versions past end-of-life
- Artifact Registry and Container Registry contents — inventory of images, and identification of unscanned or unsigned images
- Cloud Functions and App Engine runtimes on deprecated or decommissioned runtime versions
- Cloud SQL database engine versions past their supported release
- Deprecated and unmaintained custom images still referenced by instance templates or MIGs
- Marketplace-deployed solutions and their patch status

### Common permissive-default conditions

Golden images built once and never rebuilt; long-running VMs never re-created from a current image; GKE clusters pinned to unsupported static versions; CentOS 7 and Debian 9 in active use.

---

## Control 03 — Data Protection

**IG1 safeguards:** 3.1, 3.2, 3.3, 3.4, 3.5, 3.6 (6 of 14) — **Shared**

### Why this Control exists

Data is the actual target of most attacks; the infrastructure is only the route to it. This Control exists because access, retention, and disposal decisions must be deliberate rather than emergent — data that is retained indefinitely by default expands the blast radius of every future breach, and data whose access controls were set once at creation drifts toward over-permissiveness as teams and projects change. In cloud storage specifically, the dominant real-world failure is not a sophisticated exfiltration but a bucket or dataset made readable to `allUsers` or `allAuthenticatedUsers` and left that way.

### Outcomes

A documented data management process, an inventory of where sensitive data lives, access control lists that reflect current need rather than historical convenience, enforced retention limits, and verified disposal.

### GCP infrastructure focus areas

- Cloud Storage buckets granting `allUsers` or `allAuthenticatedUsers` — the single highest-value finding in this Control
- Public Access Prevention enforced at org level via `constraints/storage.publicAccessPrevention`
- Uniform bucket-level access enabled, retiring per-object legacy ACLs (`constraints/storage.uniformBucketLevelAccess`)
- BigQuery datasets shared to `allAuthenticatedUsers`, and authorised views granting broader access than intended
- Sensitive Data Protection (Cloud DLP) discovery scans across Cloud Storage, BigQuery, and Cloud SQL to build the data inventory
- Object Lifecycle Management policies on buckets to enforce retention and deletion
- BigQuery table and partition expiration settings; default dataset expiration
- Cloud SQL automated backup retention windows and point-in-time recovery configuration
- CMEK via Cloud KMS where key control is required, with key rotation schedules and IAM separation on the key rings
- `constraints/gcp.resourceLocations` to enforce data residency boundaries
- Secret material in Secret Manager rather than in environment variables, instance metadata, or source repositories
- VPC Service Controls perimeters around projects holding regulated data

### Notes on IG1 boundary

Safeguard **3.6 (Encrypt Data on End-User Devices)** is a laptop and mobile device control. It has no GCP surface and is excluded from the checklist.

### Common permissive-default conditions

Legacy per-object ACLs still in use; buckets created before public access prevention existed; BigQuery datasets with no expiration; no DLP discovery ever run.

---

## Control 04 — Secure Configuration of Enterprise Assets and Software

**IG1 safeguards:** 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7 (7 of 12) — **Shared**

### Why this Control exists

Vendor default configurations optimise for ease of initial setup, not for security. Default accounts, default network rules, default permissive service identities, and open management interfaces are all present at day one and persist unless deliberately removed. This is the largest IG1 Control by safeguard count because misconfiguration — not exploitation of a novel vulnerability — is the most common root cause of cloud compromise, and because configuration drifts continuously unless it is enforced rather than merely documented.

### Outcomes

A defined and enforced secure baseline for compute, network, and platform services, applied automatically at provisioning time and continuously verified, with vendor defaults removed.

### GCP infrastructure focus areas

- **Organization Policy constraints applied at the org node** — this is the enforcement mechanism the whole Control depends on, and its absence is the defining gap in a permissive-default organization. Remember that applying them stops new violations only; the existing estate needs a separate discovery and clean-up pass
- Default VPC network and its `default-allow-ssh` / `default-allow-rdp` / `default-allow-icmp` rules removed; `constraints/compute.skipDefaultNetworkCreation` enforced
- Default Compute Engine and App Engine service accounts stripped of `roles/editor`; `constraints/iam.automaticIamGrantsForDefaultServiceAccounts` enforced
- Firewall rules permitting `0.0.0.0/0` to port 22, 3389, or database ports
- External IP addresses on VMs restricted via `constraints/compute.vmExternalIpAccess`
- OS Login enforced (`constraints/compute.requireOsLogin`), replacing project-wide and instance-level SSH keys
- Serial port access disabled (`constraints/compute.disableSerialPortAccess`)
- Shielded VM enforced (`constraints/compute.requireShieldedVm`); Confidential Computing where warranted
- IP forwarding restricted (`constraints/compute.vmCanIpForward`)
- Cloud SQL public IP disabled (`constraints/sql.restrictPublicIp`) and authorised networks not set to `0.0.0.0/0`
- GKE hardening: private clusters, authorised networks on the control plane, Workload Identity, legacy ABAC disabled, basic auth and client certificate auth disabled, legacy metadata endpoints disabled, Shielded GKE nodes
- Session length for Google Cloud console and gcloud configured (Control 4.3 equivalent for session locking)
- Image baseline hardening applied at bake time rather than at boot time, with the bake pipeline as the enforcement point
- Configuration enforced through infrastructure-as-code with drift detection, not applied by hand

### Notes on IG1 boundary

Safeguard **4.5 (Implement and Manage a Firewall on End-User Devices)** applies to workstations and is excluded from the checklist. Safeguard **4.3 (Automatic Session Locking)** is primarily an endpoint control but has a partial GCP surface in console and CLI session duration.

### Common permissive-default conditions

No org policies at all; default network present in most projects; default service accounts holding Editor; project-wide SSH keys in use; GKE clusters with legacy ABAC or basic authentication still enabled.

---

## Control 05 — Account Management

**IG1 safeguards:** 5.1, 5.2, 5.3, 5.4 (4 of 6) — **Shared**

### Why this Control exists

Accounts are the persistent foothold. Attackers overwhelmingly prefer valid credentials to exploits, because a valid credential generates no alert and survives patching. This Control exists to constrain how many accounts exist, how privileged they are, and how long they persist after they stop being needed — dormant accounts and standing administrative privilege are the two conditions that convert a minor credential leak into a full compromise.

### Outcomes

A current inventory of every human and non-human identity with access to the environment, no shared credentials, dormant accounts disabled, and administrative privilege confined to dedicated accounts used only for administrative work.

### GCP infrastructure focus areas

- Full IAM principal inventory across org, folder, project, and resource-level bindings, including conditional bindings
- Service account inventory — the non-human identity population is usually far larger than the human one and far less governed
- User-managed service account keys: existence, age, and last use; `constraints/iam.disableServiceAccountKeyCreation` enforced
- Workload Identity Federation and Workload Identity (GKE) replacing exported key files
- Dormant identity detection: users and service accounts with no authentication activity in the defined window, via IAM activity analyser and Cloud Audit Logs
- Super admin accounts in Cloud Identity or Workspace — count, purpose, and whether they are used for daily work
- Separation of daily-use accounts from privileged administrative accounts
- `roles/owner` and `roles/editor` grants at org and folder level, and whether any are held by individual users
- External identities: `constraints/iam.allowedPolicyMemberDomains` to prevent grants outside the trusted domain
- Break-glass account existence, storage, and monitoring

### Notes on IG1 boundary

Safeguard **5.2 (Use Unique Passwords)** is primarily an identity-provider control; the GCP surface is the absence of shared accounts and the elimination of static service account keys as a shared-secret equivalent.

### Common permissive-default conditions

Long-lived service account JSON keys distributed to developers and CI systems; basic role grants to individual users at project level; departed employees still holding bindings; personal Gmail accounts granted access before domain restriction existed.

---

## Control 06 — Access Control Management

**IG1 safeguards:** 6.1, 6.2, 6.3, 6.4, 6.5 (5 of 8) — **GCP-owned**

### Why this Control exists

Control 5 governs *whether an account exists*; Control 6 governs *what it can reach and how it proves identity*. Three of its five IG1 safeguards mandate multi-factor authentication, which reflects the empirical finding that MFA on administrative and remote access defeats the large majority of credential-based attacks regardless of how the credential was obtained. The remaining two exist because access granting is usually well-defined and access revocation almost never is — the asymmetry between the two is where standing over-privilege accumulates.

### Outcomes

A documented process for granting and revoking access that executes reliably on joiner, mover, and leaver events, with MFA enforced on all administrative access, all remote access, and all externally exposed applications.

### GCP infrastructure focus areas

- 2-Step Verification enforced for all Cloud Identity / Workspace users, with phishing-resistant methods (security keys, passkeys) mandated for super admins and project owners
- MFA enforcement on any federated IdP path into GCP, verified rather than assumed
- Identity-Aware Proxy fronting internal web applications, replacing IP-allowlist-based access
- IAP TCP forwarding for SSH and RDP, eliminating bastion hosts with public IPs and inbound `0.0.0.0/0` rules
- Access granting driven by group membership rather than individual bindings, with group membership as the joiner/mover/leaver integration point
- Documented revocation procedure covering IAM bindings, service account keys, and OAuth token/session invalidation
- IAM Recommender findings for over-granted roles, and IAM Conditions for time-bound or context-bound access
- Privileged Access Manager for just-in-time elevation instead of standing admin grants
- Access Context Manager / BeyondCorp access levels applied to administrative surfaces
- Periodic access review cadence with evidence retained

### Common permissive-default conditions

Bastion hosts with public IPs and permissive SSH rules; per-user IAM bindings that survive team changes; 2SV enabled but not enforced; no revocation runbook.

---

## Control 07 — Continuous Vulnerability Management

**IG1 safeguards:** 7.1, 7.2, 7.3, 7.4 (4 of 7) — **GCP-owned**

### Why this Control exists

The gap between a vulnerability being disclosed and being exploited in the wild is now measured in days. Manual patching cannot operate at that tempo, which is why two of the four IG1 safeguards specifically require *automated* patch management rather than merely a patching policy. This Control exists to make remediation a continuously running process with defined timelines, rather than a periodic project that competes for capacity with feature work.

### Outcomes

Vulnerability information is continuously ingested, findings are ranked and assigned remediation timelines, and OS and application patches are applied automatically on a defined schedule with verification.

### GCP infrastructure focus areas

- Security Command Center enabled at org scope — the tier determines whether Vulnerability Assessment and Rapid Vulnerability Detection findings are available at all
- VM Manager OS patch management: patch deployments, patch compliance reporting, and coverage gaps where the OS Config agent is absent
- Artifact Analysis / Container Analysis scanning on Artifact Registry, with findings routed to an owner rather than to a dashboard nobody reads
- GKE node auto-upgrade enabled and clusters enrolled in a release channel
- Cloud SQL maintenance windows configured and automatic minor version upgrades permitted
- Web Security Scanner against externally exposed App Engine, Compute Engine, and GKE endpoints
- Image rebuild cadence — patching by rebaking and redeploying images rather than by patching long-lived instances in place
- Instance template and MIG rollout process so that a new image actually reaches production
- Defined remediation SLAs by severity, with findings tracked to closure
- Deprecation and end-of-life tracking for GCP services and runtimes with announced shutdown dates

### Common permissive-default conditions

Pet VMs patched by hand or not at all; no OS Config agent on older instances; container images built years ago and never rescanned; GKE clusters with auto-upgrade disabled to avoid disruption.

---

## Control 08 — Audit Log Management

**IG1 safeguards:** 8.1, 8.2, 8.3 (3 of 12) — **GCP-owned**

### Why this Control exists

Logs are the only mechanism by which an intrusion can be reconstructed after the fact, and the only evidence base for determining scope during an incident. This Control exists because logging fails in two specific ways that are both invisible until the moment they matter: logs that were never collected, and logs that were collected but rotated away before anyone looked. At IG1 the requirement is deliberately modest — have a process, collect the logs, and make sure there is somewhere to keep them — because collection without retention is worthless and retention without collection is impossible.

### Outcomes

A defined audit logging process with stated retention periods, audit logs collected from every asset in scope, and storage provisioned so that logs survive long enough to be useful in an investigation.

### GCP infrastructure focus areas

- Admin Activity audit logs (on by default and non-disableable) confirmed to be flowing and exported
- **Data Access audit logs enabled** — these are off by default for most services and their absence is the most common material logging gap in an older org
- Aggregated log sink at **organization** level with `includeChildren` set, so new projects are captured without action
- Log sink destination: Cloud Storage with retention policy and Bucket Lock, BigQuery for query, or Pub/Sub for SIEM forwarding
- Log bucket retention periods set explicitly rather than left at the 30-day default for `_Default`
- Immutability and tamper resistance on the log destination, with the logging project's IAM separated from the projects being logged
- Log sink service account permissions verified — a misconfigured sink writes nothing and reports no error to anyone
- VPC Flow Logs, Cloud NAT logs, firewall rules logging, and Cloud DNS logging enabled where required
- GKE audit logging and Cloud SQL logging configured
- Log-based metrics and alerting on org policy changes, IAM policy changes, service account key creation, custom role changes, and audit config changes
- Log exclusion filters reviewed — exclusions added for cost reasons can silently remove security-relevant events

### Common permissive-default conditions

Data Access logs never enabled; per-project sinks configured ad hoc and missing newer projects; default 30-day retention; logs stored in the same project as the workload being logged.

---

## Control 09 — Email and Web Browser Protections

**IG1 safeguards:** 9.1, 9.2 (2 of 7) — **Shared**

### Why this Control exists

Browsers and email clients are the software most consistently exposed to untrusted content, which makes them the most common initial access vector. This Control exists to reduce that exposure at two points: keeping the client software itself supported and patched, and blocking connections to known-malicious destinations at the DNS layer before content is ever retrieved.

### Outcomes

Only fully supported browsers and email clients in use, and DNS-layer filtering that blocks resolution of known-malicious domains.

### GCP infrastructure focus areas

- Cloud DNS response policies applied to VPC networks to block or redirect known-bad domains for workload egress
- Secure Web Proxy or an egress proxy for outbound HTTP/S filtering from VPCs
- Cloud NAT configured so egress is centralised and observable rather than per-instance via external IPs
- Egress firewall rules constraining outbound destinations rather than defaulting to allow-all
- Cloud DNS logging enabled to make resolution behaviour visible

### Notes on IG1 boundary

Safeguard **9.1 (Fully Supported Browsers and Email Clients)** is a workstation control with no GCP surface and is excluded from the checklist. Safeguard **9.2 (DNS Filtering)** is included in a workload-egress interpretation — the corporate-workstation half remains with the endpoint team.

### Common permissive-default conditions

Default egress-allow-all firewall rules; VMs with external IPs performing their own uncontrolled egress; no DNS logging.

---

## Control 10 — Malware Defenses

**IG1 safeguards:** 10.1, 10.2, 10.3 (3 of 7) — **Shared**

### Why this Control exists

Malware remains the primary means of converting initial access into persistence, lateral movement, and ransom. The IG1 safeguards are deliberately basic — deploy anti-malware, keep its signatures current, and disable the removable-media autorun paths — because these are the minimum conditions under which commodity malware is stopped rather than merely observed.

### Outcomes

Anti-malware protection deployed on assets that can host it, signature updates automated, and automatic-execution vectors disabled.

### GCP infrastructure focus areas

- Anti-malware agent deployment on Compute Engine instances where the workload and OS warrant it, baked into the image rather than installed post-boot
- Agent presence verified through OS inventory, so coverage gaps are detectable
- Signature update mechanism confirmed to function on instances with restricted egress — a common silent failure when egress filtering is introduced
- Container image malware scanning in the registry as the container-native equivalent
- Binary Authorization to prevent unattested images from being deployed to GKE or Cloud Run
- Shielded VM secure boot and integrity monitoring as a platform-level anti-tamper control
- Container-Optimized OS for GKE nodes, reducing the attack surface rather than layering an agent on it
- Cloud Storage malware scanning for buckets that accept file uploads from untrusted sources

### Notes on IG1 boundary

Safeguard **10.3 (Disable Autorun and Autoplay for Removable Media)** has no GCP surface and is excluded from the checklist. Safeguards 10.1 and 10.2 are included in a workload-server interpretation only; the end-user device population remains with the endpoint team.

### Common permissive-default conditions

No anti-malware on Linux VMs on the assumption it is unnecessary; agents installed manually on some instances and not others; no Binary Authorization.

---

## Control 11 — Data Recovery

**IG1 safeguards:** 11.1, 11.2, 11.3, 11.4 (4 of 5) — **GCP-owned**

### Why this Control exists

Ransomware changed the purpose of backup from availability insurance to security control. This Control has an unusually high proportion of its safeguards at IG1 because backups are the last line of defence, and because modern ransomware specifically targets backup systems first — which is why safeguard 11.4 requires an *isolated* instance of recovery data, not merely a second copy. A backup reachable with the same credentials that encrypted production is not a backup.

### Outcomes

A documented recovery process, automated backups running on schedule, recovery data protected with access controls and encryption, and at least one isolated copy that is not reachable from the production environment.

### GCP infrastructure focus areas

- Backup and DR Service or documented per-service backup configuration across the estate
- Cloud SQL automated backups, retention window, and point-in-time recovery enabled
- Compute Engine persistent disk snapshot schedules attached to disks, with retention policy
- Cloud Storage: versioning, soft delete, and Object Lifecycle Management on data-bearing buckets
- **Bucket Lock** retention policies on backup buckets to make them WORM and defeat deletion by a compromised credential
- Backup data held in a **separate project** with distinct IAM, ideally under a different folder, so production compromise does not grant backup deletion
- Cross-region replication or multi-region storage class for the isolated copy
- CMEK on backup data with key access separated from production identities
- Firestore, Bigtable, Spanner, and Filestore backup configuration where those services are in use
- GKE cluster and persistent volume backup via Backup for GKE
- Terraform state backend versioning and protection — state loss is a recovery event
- Documented and dated restore test evidence

### Common permissive-default conditions

Snapshot schedules absent or applied to some disks only; backups stored in the same project as production; no Bucket Lock; no restore ever tested.

---

## Control 12 — Network Infrastructure Management

**IG1 safeguards:** 12.1 (1 of 8) — **GCP-owned**

### Why this Control exists

Network infrastructure sits at a chokepoint: compromising it yields visibility into or control over everything traversing it, and it is frequently the least-patched category of asset because it is treated as furniture rather than as software. Only one safeguard sits at IG1, and it is the simplest one — keep it current — because unsupported network equipment and firmware is the condition that makes the rest of the Control moot.

### Outcomes

All network infrastructure running supported, current versions with security updates applied.

### GCP infrastructure focus areas

- GKE control plane and node versions within the supported window
- Cloud VPN tunnels using current IKE versions and ciphers; Classic VPN migrated to HA VPN
- Load balancer SSL policies set to modern TLS minimums, retiring TLS 1.0/1.1 and weak cipher suites
- Legacy (non-Application) load balancers migrated
- Auto-mode VPC networks converted to custom-mode, and legacy networks (pre-VPC, non-subnet) identified and eliminated
- Cloud Router and Interconnect/Partner Interconnect configuration current
- Cloud Armor policies present on externally exposed load balancers, with rule sets current
- Deprecated Compute Engine API versions still referenced by tooling
- Network topology documented, including Shared VPC and VPC Peering relationships

### Common permissive-default conditions

Auto-mode or legacy networks; Classic VPN still in service; SSL policies at the permissive default; VPC peering relationships nobody can currently explain.

---

## Control 13 — Network Monitoring and Defense

**IG1 safeguards:** none (0 of 11) — **No IG1 safeguards**

### Why this Control exists

Detection and response at the network layer — intrusion detection, traffic filtering between segments, centralised alerting, and network-level threat hunting. CIS places all eleven safeguards at IG2 and IG3 because they presuppose a staffed security operations function, which IG1 does not assume.

### Outcomes at IG1

None required. This Control is documented here for completeness and for forward planning only.

### GCP relevance if scope later extends to IG2

Cloud IDS, Packet Mirroring, VPC Flow Logs analysis, Cloud Armor adaptive protection, Security Command Center threat detection (Event Threat Detection, Container Threat Detection), and SIEM integration via Pub/Sub or Chronicle. Several of these are enabled as a side effect of the Control 8 logging work, which makes them inexpensive to reach later.

**No checklist items are generated by this Control.**

---

## Control 14 — Security Awareness and Skills Training

**IG1 safeguards:** 14.1 through 14.8 (8 of 9) — **Out of GCP scope**

### Why this Control exists

This is the largest single block of IG1 safeguards, which reflects the consistent finding that workforce behaviour — not technology — determines the outcome of phishing, pretexting, and social engineering attempts. The eight safeguards cover the awareness programme itself and seven specific topics: social engineering, authentication practices, data handling, unintentional data exposure, incident recognition and reporting, identifying missing security updates, and the risks of insecure networks.

### Outcomes

A workforce that can recognise and report the attacks it is actually targeted by.

### GCP relevance

None. There is no infrastructure configuration that satisfies any of these eight safeguards.

**No checklist items are generated by this Control.** These eight safeguards are owned by the enterprise security awareness function. If a future auditor evaluates this GCP Organization against full IG1, this Control must be evidenced by that function, not by the platform team.

---

## Control 15 — Service Provider Management

**IG1 safeguards:** 15.1 (1 of 7) — **GCP-owned**

### Why this Control exists

Third parties process, store, and transmit enterprise data, and their compromise becomes the enterprise's compromise. At IG1 the requirement is the foundational one: know who the providers are. Everything else in this Control — classification, contractual security requirements, monitoring, decommissioning — sits at IG2 and above, because none of it is possible without the inventory first.

### Outcomes

A maintained inventory of service providers holding, processing, or transmitting enterprise data, or supporting critical IT processes, reviewed at least annually.

### GCP infrastructure focus areas

- Google Cloud itself recorded as a service provider, with the shared responsibility boundary documented
- Marketplace-deployed solutions and their vendors enumerated
- Third-party service accounts and identities granted access into the Organization from outside the trusted domain
- Workload Identity Federation trust relationships to external identity providers — each is a trust dependency
- OAuth applications authorised against the Cloud Identity / Workspace tenant with GCP API scopes
- SaaS platforms integrated via API into GCP projects (CI/CD, observability, security tooling)
- Cross-project and cross-organization IAM grants
- `constraints/iam.allowedPolicyMemberDomains` as the enforcement point preventing undocumented external grants
- Data residency and processing locations per provider

### Common permissive-default conditions

Third-party tools onboarded years ago with service account keys and never reviewed; OAuth grants nobody remembers approving; no domain restriction constraint.

---

## Control 16 — Application Software Security

**IG1 safeguards:** none (0 of 14) — **No IG1 safeguards**

### Why this Control exists

Secure development lifecycle, dependency management, code review, secure design, and vulnerability handling in developed software. All fourteen safeguards sit at IG2 and IG3, on the reasoning that IG1 enterprises are predominantly consumers rather than producers of software.

### Outcomes at IG1

None required. Documented for completeness.

### GCP relevance if scope later extends to IG2

Cloud Build with provenance attestation, Software Delivery Shield, Artifact Analysis for dependency vulnerabilities, Binary Authorization, Secret Manager, and Assured Open Source Software. Note that if this Organization hosts in-house developed applications, this Control becomes materially relevant at IG2 and should be scoped early.

**No checklist items are generated by this Control.**

---

## Control 17 — Incident Response Management

**IG1 safeguards:** 17.1, 17.2, 17.3 (3 of 8) — **Shared**

### Why this Control exists

Every other Control eventually fails; this one determines what that failure costs. The three IG1 safeguards are deliberately minimal and entirely procedural — name someone accountable, publish contact details, and give people a way to report — because the most expensive failures in incident response are not tactical mistakes but the absence of anyone to make the first decision and the absence of any route by which a report reaches them.

### Outcomes

A named person accountable for incident handling, published and current contact information for reporting incidents including third-party contacts, and a documented enterprise reporting process that the workforce knows how to use.

### GCP infrastructure focus areas

- **Essential Contacts configured at organization level** for the Security category — this is where Google sends security notifications, and if it is unset those notifications go to whoever originally created the org
- Essential Contacts populated for Legal, Suspension, and Technical categories as well
- Cloud Billing account contacts current
- Security Command Center findings routed to a monitored destination via Pub/Sub, not to an unmonitored console
- Cloud Monitoring alerting and notification channels current — verify no alerts route to departed employees' addresses
- Support case contacts and support plan level, which determines escalation options during an incident
- Org-level break-glass access procedure documented and testable
- Log retention sufficient to support an investigation timeline (dependency on Control 8)

### Notes on IG1 boundary

The workforce-facing half of safeguard 17.3 — how an employee reports a suspected incident — is an enterprise process. The GCP-actionable portion is ensuring platform-generated signals reach a monitored human.

### Common permissive-default conditions

Essential Contacts never configured; security notifications routing to a former employee or a shared mailbox nobody reads; alerting channels with stale addresses.

---

## Control 18 — Penetration Testing

**IG1 safeguards:** none (0 of 5) — **No IG1 safeguards**

### Why this Control exists

Penetration testing validates that the other Controls work in practice against an adversary rather than merely existing on paper. All five safeguards sit at IG2 and IG3 on the reasoning that testing defences that have not yet been built produces findings the enterprise cannot act on.

### Outcomes at IG1

None required. Documented for completeness.

### GCP relevance if scope later extends to IG2

Google Cloud permits customer penetration testing of the customer's own projects without prior approval, subject to the Acceptable Use Policy — no notification form is required. Scoping would cover external attack surface, IAM privilege escalation paths, and lateral movement between projects and VPCs.

**No checklist items are generated by this Control.**

---

## Summary

### IG1 safeguard distribution

| Control | IG1 safeguards | GCP position |
|---|---|---|
| 01 Inventory and Control of Enterprise Assets | 2 | GCP-owned |
| 02 Inventory and Control of Software Assets | 3 | GCP-owned |
| 03 Data Protection | 6 | Shared (3.6 out of scope) |
| 04 Secure Configuration | 7 | Shared (4.5 out of scope) |
| 05 Account Management | 4 | Shared |
| 06 Access Control Management | 5 | GCP-owned |
| 07 Continuous Vulnerability Management | 4 | GCP-owned |
| 08 Audit Log Management | 3 | GCP-owned |
| 09 Email and Web Browser Protections | 2 | Shared (9.1 out of scope) |
| 10 Malware Defenses | 3 | Shared (10.3 out of scope) |
| 11 Data Recovery | 4 | GCP-owned |
| 12 Network Infrastructure Management | 1 | GCP-owned |
| 13 Network Monitoring and Defense | 0 | No IG1 safeguards |
| 14 Security Awareness and Skills Training | 8 | Out of GCP scope |
| 15 Service Provider Management | 1 | GCP-owned |
| 16 Application Software Security | 0 | No IG1 safeguards |
| 17 Incident Response Management | 3 | Shared |
| 18 Penetration Testing | 0 | No IG1 safeguards |
| **Total** | **56** | **44 with GCP-actionable requirements** |

### Safeguards with no GCP surface

Twelve IG1 safeguards generate no checklist items and are owned outside the platform team:

- **3.6** Encrypt Data on End-User Devices
- **4.5** Implement and Manage a Firewall on End-User Devices
- **9.1** Ensure Use of Only Fully Supported Browsers and Email Clients
- **10.3** Disable Autorun and Autoplay for Removable Media
- **14.1–14.8** Security Awareness and Skills Training (all eight)

### The critical path for a permissive-default organization

If the remediation programme needs a sequence, this ordering front-loads the items that make every subsequent item cheaper:

1. **Control 8 — logging.** Enable Data Access logs and build the org-level aggregated sink first. Without it there is no evidence base for anything else, and the historical record starts accumulating from the day it is switched on rather than the day it is needed.
2. **Control 1 — inventory.** Cloud Asset Inventory at org scope. Every other Control is scoped by this.
3. **Control 4 — org policies.** Applying constraints at the org node stops the problem growing while the backlog is worked — but only stops it growing. Constraints are not retroactive, so run the discovery sweep first to size the existing backlog, then apply in dry-run, then promote to enforcement.
4. **Control 3 — public data exposure.** Bucket and dataset public access is the highest-severity finding class and is quick to remediate.
5. **Control 6 and 5 — identity.** MFA enforcement and service account key elimination.
6. **Control 11 — backup isolation.** Separate the backup project before it is needed.
7. Remaining controls in dependency order.

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This document is an implementation aid and is not affiliated with or endorsed by CIS. Safeguard titles and Implementation Group assignments are drawn from CIS Controls v8.1; refer to the official CIS publication for authoritative safeguard text.*
