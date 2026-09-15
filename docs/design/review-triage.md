# REVIEW triage — easy call vs manual

**Status:** adopted 2026-09-15. Next: automate every ⚙️ check and every A check that can be scored once the operator supplies a prerequisite value (e.g. approved container registries) — in progress.

The audit's checks fall into three buckets:

| Bucket | Meaning | Checks |
|---|---|---|
| Fully automated | API query → PASS/FAIL | 41 (plus 30 cross-references that inherit a verdict) |
| **Hard to automate** | API query → a human verifies, validates or interprets | **117** — the checks that always return REVIEW (116) plus V151, run by hand |
| Human | No GCP API; a person is interviewed about process | 102 manual requirements (30 GCP tasks, 72 process) |

This page splits the 117 "hard to automate" checks in two:

- **A — Easy call.** A reviewer can decide PASS/FAIL from the output alone, plus at most one reference value
  they'll have before starting (retention period, approved regions, approved registries, backup identity).
  Output is short enough to read per project.
- **B — Manual.** The decision needs something the output doesn't hold (owners, documentation, intent, which
  systems are "production"), a cross-reference to other checks or a console, or output that routinely runs to
  hundreds of lines. These should become manual procedures with API or console instructions.

Output sizes come from the 2026-09-14 run over the test organization (1 org pack, 18 project packs).

## Summary

| | Checks |
|---|---|
| **A — Easy call** | **73** |
| &nbsp;&nbsp;of which mechanical — could move to fully automated with a small change ⚙️ | 48 |
| **B — Manual** | **44** |

## A — Easy call (73)

⚙️ = the rule is mechanical (enforced true, a count, a True/False column, empty list): `audit-run.go` could score it and move it to the fully automated bucket.

| Check | Req | Scope | Title | Reviewer's rule |
|---|---|---|---|---|
| V1 ⚙️ | `1.1#1` | org | Cloud Asset Inventory feed at organization scope | PASS if at least one feed is listed. |
| V2 ⚙️ | `1.1#2` | org | Feed exports to a durable destination | PASS if every feed row has a TOPIC. |
| V3 ⚙️ | `1.1#3` | org | All projects enumerated, including those flat under the org node | PASS if the two numbers are equal. |
| V4 ⚙️ | `1.1#4` | org | Inventory covers all major compute and data services | PASS if the listed asset types include Instance, Bucket, SQL Instance, Cluster and Run Service. |
| V7 | `1.2#2` | project | Projects with no billing account or pending deletion dispositioned | Each line is a FAIL until a disposition (delete / keep, and why) is on record. |
| V9 ⚙️ | `2.1#1` | project | VM Manager OS inventory enabled and agent present | PASS if instances with OS inventory equals total instances. |
| V12 ⚙️ | `2.1#4` | project | GKE cluster and node pool versions recorded | Evidence capture: PASS once the cluster/version list is saved (the output is the inventory). |
| V13 ⚙️ | `2.1#5` | project | Serverless runtime versions recorded | Evidence capture: PASS once the functions/Run list is saved. |
| V14 ⚙️ | `2.1#6` | project | Cloud SQL engine and version recorded | Evidence capture: PASS once the Cloud SQL engine/version list is saved. |
| V17 ⚙️ | `2.2#3` | project | No decommissioned serverless runtimes | PASS if the output is "none found"; any runtime listed is a FAIL. |
| V18 ⚙️ | `2.2#4` | project | No unsupported Cloud SQL database versions | PASS if the output is "none found"; any version listed is a FAIL. |
| V20 | `2.3#3` | project | Container images sourced only from approved registries | Compare the allowlist patterns with your approved-registry list: any extra pattern is a FAIL. |
| V21 ⚙️ | `2.3#4` | project | Binary Authorization policy in place | FAIL if ALWAYS_ALLOW; PASS if REQUIRE_ATTESTATION + ENFORCED_BLOCK_AND_AUDIT_LOG. |
| V23 | `3.1#3` | org | Data residency requirements defined and mapped | Compare the allowed-locations values with your residency policy. |
| V28 ⚙️ | `3.3#2` | org | Public access prevention enforced at org level | PASS if the effective policy shows enforce: true. |
| V32 ⚙️ | `3.3#6` | org | Uniform bucket-level access enforced | PASS if the effective policy shows enforce: true. |
| V35 | `3.3#9` | org | Domain restriction constraint enforced | PASS if enforced (not dry-run) and the allowed values are your Cloud Identity customer IDs. |
| V39 | `3.4#4` | project | Cloud SQL backup retention set to the defined period | Compare each instance's retained backups with the policy number. |
| V40 | `3.4#5` | org | Log bucket retention set explicitly | Compare RETENTION_DAYS on each bucket with the policy number; 30 is a FAIL unless that is the policy. |
| V45 ⚙️ | `4.1#6` | project | Security Command Center enabled at org scope | PASS if securitycenter.googleapis.com is listed. |
| V47 ⚙️ | `4.2#2` | org | Default network creation constraint enforced | PASS if the effective policy shows enforce: true. |
| V52 | `4.2#8` | org | VPC peering and Shared VPC constraints applied | PASS if the constraint is set, or a decision not to set it is on record. |
| V54 ⚙️ | `4.3#3` | project | IAP session duration configured | PASS if a max age is shown. |
| V58 | `4.4#4` | project | Egress rules constrained rather than allow-all | Any EGRESS allow to 0.0.0.0/0 is a FAIL unless documented; normally empty. |
| V60 ⚙️ | `4.4#6` | org | Cloud SQL public IP disabled by constraint | PASS if both policies show enforce: true. |
| V63 ⚙️ | `4.4#9` | project | GKE control plane authorized networks and private clusters | PASS if both columns are True for every cluster. |
| V65 ⚙️ | `4.6#1` | org | OS Login enforced org-wide | PASS if the effective policy shows enforce: true. |
| V70 ⚙️ | `4.6#6` | org | Serial port access disabled | PASS if the effective policy shows enforce: true. |
| V71 | `4.6#7` | org | External IP assignment restricted | PASS if denyAll, or a short allowlist you can name; dry-run only is a FAIL. |
| V73 ⚙️ | `4.6#9` | org | Shielded VM enforced | PASS if the effective policy shows enforce: true. |
| V74 ⚙️ | `4.6#10` | org | IP forwarding restricted | PASS if the policy shows denyAll. |
| V75 ⚙️ | `4.6#11` | project | GKE hardening settings | PASS if every cluster shows ABAC=false, WI set, SHIELDED=true. |
| V79 ⚙️ | `4.7#3` | org | Automatic default grants constraint enforced | PASS if the effective policy shows enforce: true. |
| V83 ⚙️ | `4.7#7` | project | Default GKE node service account replaced or scoped | FAIL if any node pool shows "default". |
| V86 ⚙️ | `5.1#1` | org | Full IAM principal inventory | Evidence capture: PASS if the inventory file has lines. |
| V87 ⚙️ | `5.1#2` | org | Conditional IAM bindings included | Evidence capture: PASS once the conditional-binding list is saved. |
| V90 ⚙️ | `5.2#2` | org | Service account key creation constraint enforced | PASS if enforce: true and not dry-run. |
| V95 ⚙️ | `5.2#7` | project | Remaining static credentials in Secret Manager with rotation | FAIL for any secret with an empty NEXT_ROTATION_TIME. |
| V97 ⚙️ | `5.3#3` | project | Service account activity assessed | FAIL for any account whose last authentication is older than the dormancy threshold (e.g. 90 days). |
| V102 | `5.4#6` | org | Just-in-time elevation in use | PASS if PAM entitlements or time-bound conditions on privileged roles are listed. |
| V105 | `6.1#2` | org | Access granted through group membership | Compare the two numbers: users far above groups (64 vs 1 here) is a FAIL. |
| V115 ⚙️ | `7.2#2` | org | Findings routed to an owning team automatically | PASS if at least one notification config with a Pub/Sub topic is listed. |
| V116 | `7.3#1` | project | Patch deployments configured on a recurring schedule | PASS if a recurring patch deployment is listed. |
| V118 | `7.3#3` | project | Patch compliance reporting reviewed | PASS if recent patch jobs show SUCCEEDED. |
| V121 ⚙️ | `7.3#7` | project | Cloud SQL maintenance windows configured | FAIL for any instance with an empty DAY/HOUR. |
| V122 ⚙️ | `7.4#1` | project | Artifact Analysis scanning enabled | PASS if containerscanning.googleapis.com is listed. |
| V126 ⚙️ | `8.2#1` | org | Admin Activity logs flowing for all projects | PASS if recent entries are returned. |
| V127 ⚙️ | `8.2#2` | project | Data Access audit logs enabled | PASS if auditConfigs show DATA_READ and DATA_WRITE for allServices. |
| V128 ⚙️ | `8.2#3` | org | Logging gap start date recorded | Evidence capture: record the earliest timestamp shown. |
| V129 | `8.2#4` | org | System Event and Policy Denied logs captured | PASS if entries are returned, or absence is noted. |
| V130 ⚙️ | `8.2#5` | org | Aggregated org-level sink with includeChildren | PASS if any sink shows includeChildren True. |
| V133 ⚙️ | `8.2#8` | project | DNS, NAT and firewall logging enabled | FAIL for any row showing False. |
| V134 ⚙️ | `8.2#9` | project | GKE and Cloud SQL logs captured | PASS if loggingService is logging.googleapis.com/kubernetes for every cluster. |
| V138 | `8.3#3` | org | Logs in a dedicated logging project | PASS if the sink destination project is not a workload project. |
| V139 ⚙️ | `8.3#4` | org | Bucket Lock applied to the log destination | PASS if isLocked is True for every log bucket. |
| V142 ⚙️ | `9.2#1` | project | Cloud DNS response policies applied | PASS if at least one response policy is bound to a network. |
| V143 | `9.2#2` | project | Egress routed through Cloud NAT or a proxy | PASS if a NAT is configured (external-IP instances are already scored by V68). |
| V145 ⚙️ | `9.2#4` | project | Cloud DNS logging enabled | PASS if enableLogging is True. |
| V152 ⚙️ | `11.1#4` | project | Terraform state backend versioning in scope | PASS if versioning is True on the state bucket. |
| V153 ⚙️ | `11.2#1` | project | Cloud SQL automated backups with PITR | PASS if both columns are True for every instance. |
| V157 | `11.2#5` | project | Other data services backed up | PASS if backups are listed for each Firestore/Spanner database in use. |
| V159 | `11.3#1` | project | Backup data encrypted with CMEK where required | PASS if a KMS key is shown (where key control is required). |
| V161 | `11.3#3` | project | Backup storage IAM restricted to a dedicated role | PASS if only the named backup identity has write roles. |
| V162 | `11.3#4` | project | No production SA holds delete on backups | FAIL if any identity other than the backup identity holds admin/objectAdmin/owner. |
| V163 ⚙️ | `11.3#5` | project | Bucket Lock applied to backup buckets | PASS if isLocked is True. |
| V165 | `11.4#1` | project | Backup copy in a separate project | PASS if the backup bucket's project is not a production project. |
| V166 | `11.4#2` | project | Backup project under a separate folder | PASS if the backup project's parent folder is not a production folder. |
| V168 | `11.4#4` | project | Copy held in a different region or multi-region | PASS if the backup location differs from the production region. |
| V170 ⚙️ | `12.1#2` | project | Classic VPN migrated to HA VPN | PASS if the target-vpn-gateways list is empty. |
| V171 ⚙️ | `12.1#3` | project | Load balancer SSL policies at a modern TLS minimum | FAIL for any HTTPS proxy with no SSL policy or a policy below TLS_1_2. |
| V172 | `12.1#4` | project | Legacy load balancers migrated | FAIL if target pools exist, or an HTTP proxy is not a redirect. |
| V181 ⚙️ | `17.2#1` | org | Essential Contacts set for the Security category | PASS if a contact lists SECURITY. |
| V183 | `17.2#3` | org | Contacts point to monitored group addresses | FAIL for any address that is an individual rather than a group. |

## B — Manual (44)

Each needs a written procedure: what to gather (API or console), who to ask, and what counts as PASS.

| Check | Req | Scope | Title | Why it can't be called from the output |
|---|---|---|---|---|
| V6 | `1.1#6` | project | Shared VPC host and service project relationships mapped | Needs your network topology documentation to say whether each Shared VPC host is accounted for. |
| V10 | `2.1#2` | project | Custom image catalogue maintained | Needs build records to say whether each custom image came from a known pipeline. |
| V11 | `2.1#3` | project | Artifact Registry inventory captured | Needs an owner for every repository, which the API does not hold. |
| V19 | `2.2#5` | project | Deprecated images not referenced by templates or MIGs | Needs each template image checked against the deprecated-image list from another check. |
| V22 | `2.3#5` | project | Workloads already running unattested identified and rolled | Every running pod image across every cluster must be checked against approved registries; scales with workloads. |
| V24 | `3.2#1` | project | Sensitive Data Protection discovery configured | The API being on proves nothing; the discovery configuration has to be confirmed in the console. |
| V26 | `3.2#4` | org | Data location recorded per store | Every bucket, dataset and SQL instance location vs approved regions: 45 lines in the test org, hundreds in a large one. Automatable if an APPROVED_REGIONS value is supplied. |
| V36 | `3.3#10` | org | VPC Service Controls perimeters around regulated data | Needs to know which projects hold regulated data before perimeter coverage can be judged. |
| V42 | `3.5#2` | project | Soft delete and versioning retention accounted for | "Reflected in the disposal process" is a process-document question, not a setting. |
| V43 | `4.1#2` | org | Baseline enforced through org policy constraints | Needs the documented security baseline (itself a manual requirement, 4.1#1) to compare against. |
| V57 | `4.4#3` | project | Default-deny ingress posture | Judging whether each allow rule is "explicit and narrower" than a deny-all is interpretation, per project. |
| V72 | `4.6#8` | project | IAP TCP forwarding in use; no public-IP bastions | Needs to know which hosts are admin/bastion hosts, then cross-checks external-IP results (V68). |
| V84 | `4.7#8` | project | Cloud SQL default database users reviewed | Password rotation dates are not exposed by the API; needs the DBA's records. |
| V88 | `5.1#3` | project | Service account inventory with owner and purpose | "Meaningful name and recorded owner": owners are not in the API. |
| V92 | `5.2#4` | project | Unused keys deleted, in-use keys replaced with federation | Each remaining key needs a documented replacement plan. |
| V94 | `5.2#6` | project | Workload Identity Federation in use | Needs the list of CI/external workloads that used keys, to check each has a pool. |
| V96 | `5.3#2` | org | Human account activity assessed | Cross-references the IAM inventory and the Workspace login report; not decidable from this output. |
| V98 | `5.3#4` | project | Unused service accounts disabled before deletion | Which accounts are "pending deletion" is not recorded anywhere in the API. |
| V103 | `5.4#7` | project | IAM Recommender findings actioned | The output has no age column, so "older than the SLA" can't be judged. Adding lastRefreshTime would make it an easy call. |
| V104 | `5.4#8` | project | Alerting on org-level IAM policy changes | Shows metrics but not whether an alert policy is bound to them; needs a second look. |
| V106 | `6.3#1` | project | Externally exposed applications enumerated | "Known, documented application" needs an application register. |
| V107 | `6.3#2` | project | IAP fronting internet-reachable internal applications | Needs to know which backends are internal applications before IAP=false can be judged. |
| V108 | `6.3#5` | org | Access Context Manager levels applied | "Bound to administrative surfaces" needs knowledge of which resources are admin surfaces. |
| V111 | `6.4#4` | project | VPN and Interconnect paths documented and MFA-gated | Needs topology documentation and MFA configuration outside GCP. |
| V113 | `6.5#3` | project | Phishing-resistant methods for org and folder admins | Produces the admin list only; phishing-resistant MFA enrolment is confirmed in the Admin Console. |
| V124 | `7.4#5` | project | Web Security Scanner run against exposed applications | Cross-references externally exposed applications (V106) to scan coverage. |
| V125 | `8.1#4` | org | Log exclusion filters documented and justified | Judging whether an exclusion filter drops security-relevant events means reading log filter logic. |
| V131 | `8.2#6` | org | Sink writer identity has permission on the destination | Only Cloud Storage destinations are checked automatically; logging buckets, BigQuery and Pub/Sub (the usual case) print "manual". |
| V136 | `8.2#11` | project | Alerting on control-plane changes | Five event types must each map to a metric AND an enabled alert; the output lists names only. |
| V140 | `8.3#5` | org | No workload principals hold delete on log storage | Needs to know who the dedicated logging administrators are, per bucket IAM policy. |
| V141 | `8.3#6` | project | Storage capacity and cost monitored | Matches alert names containing "log"; whether one really alerts on ingestion volume needs the policy opened. |
| V146 | `9.2#5` | project | Blocked-resolution events surfaced | Metric listed by name only; the alert binding needs a second look. |
| V151 | `10.2#2` | project | Update path reachable from restricted-egress instances | Never run automatically: SSH to a representative instance by hand. |
| V155 | `11.2#3` | project | Bucket versioning and soft delete enabled | Lists every bucket without versioning; deciding which are "data-bearing" is per-bucket judgement across all projects. |
| V156 | `11.2#4` | project | Backup for GKE configured where needed | Needs to know which clusters hold persistent state. |
| V158 | `11.2#6` | project | Backup success and failure monitored | Matches alert names containing "backup"; whether one really alerts on failure needs the policy opened. |
| V160 | `11.3#2` | project | KMS key access separated from production identities | Needs to know which service accounts are production workloads. |
| V164 | `11.3#6` | project | Backup deletion events logged and alerted | Metric listed by name only; the alert binding and the bucket in its filter need a second look. |
| V174 | `12.1#6` | project | Cloud Armor rule sets current | "Current preconfigured expression sets" needs the latest Cloud Armor rule versions from Google's documentation. |
| V175 | `15.1#3` | org | Third-party service accounts inventoried | 136 lines in the test org: every external service account must be matched to a documented integration. |
| V177 | `15.1#6` | org | Cross-project and cross-org grants identified | 266 lines in the test org: every cross-project grant must be matched to documentation. |
| V180 | `17.1#3` | project | Break-glass procedure documented and alerted on | Metric listed by name only; break-glass account and alert binding need a second look. |
| V184 | `17.2#4` | project | Pre-existing contacts reviewed and replaced | "Departed employee" is HR knowledge, not in the output. |
| V185 | `17.2#6` | project | Monitoring notification channels verified | "Departed employee" is HR knowledge; the enabled column alone is easy. |

## Found while triaging (not yet fixed)

- **Org-level queries in the project pass.** V6 (Shared VPC hosts for the organization), V45 (SCC "at org scope"),
  V113 (org IAM admins) and V127 (org audit config) read organization settings but are `scope: project`.
  They run identically once per project — 18 identical outputs in the test run, 105 in a large org. Same class as BUG-023.
- **V17 / V18** print "none found" when compliant, which is why they are REVIEW. Dropping the `echo` makes them
  empty-means-PASS, fully automated, with no loss of information.
- **Alert checks (V104, V136, V141, V146, V158, V164, V180)** share one gap: they list log metrics or alert policies
  by name, never the link between them. One shared check that joins metric filters to alert-policy conditions would
  move most of B's alerting group to A.

## Decisions

1. **A/B line agreed as listed** (2026-09-15). Checks near the line move to fully automated by adding an audit
   **prerequisite**: a value the operator supplies in `audit.env` (e.g. approved container registries).
2. **⚙️ checks move to fully automated now**, together with any A or B check a prerequisite makes scorable.
3. *Open:* format for B's manual procedures — a section per check in `cis-ig1-cli-validation.md`, or a separate document.
