# Bug log — CIS IG1 audit kit

Bugs that stopped the audit from completing against the test organization,
found by running it end to end. Each entry records the error as observed, the
cause, and the fix that was patched in.

Environment: org `933250405420`, host project `simplifymycloud-dev`, target
project `iq9-gcp-dev-yamato`, Go 1.26.2 (darwin/arm64), Terraform, gcloud (Homebrew).
Test runs began 2026-09-13.

Audit output from these runs is **not committed**; it is written under
`scratch/` (git-ignored). Relevant excerpts are quoted here instead.

---

<!-- Template
## BUG-NNN — short title

| | |
|---|---|
| Found | YYYY-MM-DD, phase (terraform apply / org pass / project pass / rollup / report) |
| Component | file:line |
| Check | V-number, if any |
| Severity | blocks run / wrong result / cosmetic |

**Error**

```
exact output
```

**Cause** — why it happened.

**Fix** — what changed, and how it was verified.
-->

## BUG-001 — Terraform apply fails re-creating custom roles; documented remedy doesn't work

| | |
|---|---|
| Found | 2026-09-13, `terraform apply` (audit service account) |
| Component | `terraform/audit-service-account/terraform.tfvars`, `terraform/readme.md`, `terraform/audit-service-account/readme.md` |
| Check | — (setup) |
| Severity | blocks run — audit SA created without its 3 custom roles (storage, SA keys, IAP) |

**Error**

```
Error: Error creating the custom organization role CIS IG1 Audit — IAP Settings Reader: googleapi:
Error 400: You can't create a role_id (cisIg1AuditIapReader) which has been marked for deletion., failedPrecondition
(same for cisIg1AuditKeyReader, cisIg1AuditStorageReader)
```

**Cause** — A previous audit's roles were destroyed more than 7 days ago. After day 7 the
role can no longer be undeleted: `gcloud iam roles describe`/`undelete` return `NOT_FOUND`
and `gcloud iam roles list --show-deleted` lists nothing, yet the role ID is still reserved
until Google purges it (up to ~37 days). The readmes said "IDs reserved for 30 days — either
`gcloud iam roles undelete`, or change `custom_role_prefix`", but undelete is impossible in
that window and `custom_role_prefix` wasn't in `terraform.tfvars`, so the operator had to find
it in `variables.tf`.

**Fix** — Added `custom_role_prefix` (with the error text and explanation) to
`terraform.tfvars`, set to `cisIg1Audit2` for this run. Rewrote the "known edges" paragraph in
both Terraform readmes to explain the 7-day/~37-day split and when each remedy applies.
Verified: re-plan showed 6 to add (3 roles + 3 bindings); apply completed, `role_count = 33`.

> Also affects `gcloud/create.sh` (lines 205–222): it tries `undelete`, then `create`,
> which will hit the same error after day 7. Not exercised in this test cycle (Terraform path chosen).

## BUG-002 — Project pass never sets `$PROJECT_ID` or gcloud's project; 37 checks ERROR, others audit the wrong project

| | |
|---|---|
| Found | 2026-09-13, project pass (run 1, `-project=iq9-gcp-dev-yamato`) |
| Component | `audit-run.go` — `main()`, `-scope=project` branch |
| Check | 39 checks use `"$PROJECT_ID"`; 52 project checks have no `--project` at all |
| Severity | blocks run — pack UNRELIABLE, 37 of 121 checks ERROR |

**Error**

```
V24  ERROR  ERROR: (gcloud.services.list) The project property is set to the empty string, which is invalid.
V57  ERROR  ERROR: (gcloud.compute.firewall-rules.list) The project property is set to the empty string, which is invalid.
V95  ERROR  ERROR: (gcloud.secrets.list) The project property is set to the empty string, which is invalid.
… (37 ERROR in total)
```

**Cause** — `-project` was stored only as the `PROJECT_ID` *placeholder* substitution, but
the placeholder regex deliberately skips `="$PROJECT_ID"` (treated as a shell variable the
caller exported). Nothing exported it, so gcloud received `--project=""`. The 52 commands
with no `--project` flag fell back to gcloud's `core/project` — the operator's own default
(`simplifymycloud-dev`), not the project under audit — and produced plausible-looking but
wrong results.

**Fix** — In the `-scope=project` branch, `os.Setenv("PROJECT_ID", project)` and
`os.Setenv("CLOUDSDK_CORE_PROJECT", project)` before any check runs; child `bash -c`
processes inherit both. Verified by re-running the project pass (see run 2).

## BUG-003 — Disabled API reported as DENIED, marking the whole pack UNRELIABLE

| | |
|---|---|
| Found | 2026-09-13, org pass (run 1) |
| Component | `audit-run.go` — `execute()` verdict switch |
| Check | V115 (`gcloud scc notifications list`) |
| Severity | blocks run — any disabled API makes the pack UNRELIABLE, and `rollup.go` then drops it |

**Error**

```
RUN STATUS: UNRELIABLE — 1 check(s) returned DENIED.
V115  DENIED
ERROR: (gcloud.scc.notifications.list) [cis-ig1-auditor@…] does not have permission to access organizations
instance [933250405420] (or it may not exist): Security Command Center API has not been used in project
288261943767 before or it is disabled. …  reason: SERVICE_DISABLED
```

**Cause** — Google reports a disabled API as a permission error ("does not have permission …
reason: SERVICE_DISABLED"). The switch tested permission phrases before disabled-API phrases,
so every disabled API became DENIED. The disabled-API branch was unreachable for this error.

**Fix** — Disabled-API test moved ahead of the permission test. It also now names the API and
project, and distinguishes the two cases: disabled in the **audit host project** (the project
owning the impersonated SA, looked up once at start) → ERROR "API DISABLED IN AUDIT HOST
PROJECT: x — enable it there"; disabled in an audited project → N/A (product not in use).
Verified: V115 re-run shows `ERROR — API DISABLED IN AUDIT HOST PROJECT: securitycenter.googleapis.com`.

## BUG-004 — Problems table shows gcloud's impersonation banner instead of the actual error

| | |
|---|---|
| Found | 2026-09-13, org pass and project pass (run 1) |
| Component | `audit-run.go` — `execute()` (stderr capture) |
| Check | every DENIED/ERROR under impersonation, e.g. V20, V21, V44, V72, V115, V166, V174 |
| Severity | wrong result (diagnostics) — failures can't be triaged from the pack |

**Error**

```
| V115 | DENIED | `WARNING: This command is using service account impersonation. All API calls will be executed as [cis-ig1-auditor@…].
    … 13 more lines` |
```

**Cause** — Every impersonated gcloud call prints that WARNING first on stderr. The Problems
table shows only the first stderr line, so the real error was always hidden. The kit's
documented auth model (impersonation only) guarantees this on every run.

**Fix** — The banner line is stripped from stderr before it is stored.

## BUG-005 — `search-all-resources` checks read `versionedResources` without `--read-mask`: false PASS

| | |
|---|---|
| Found | 2026-09-13, org pass (run 1) |
| Component | `docs/cis-ig1-cli-validation.md` |
| Check | V8 V15 V53 V55 V56 V59 V61 V66 V68 V69 V76 V81 V132 V150 V154 |
| Severity | wrong result — security findings silently reported as PASS |

**Error** — no error; wrong verdicts. Run 1: V55 "No firewall rules allowing the internet to SSH
or RDP" = **PASS**, while V50 in the same run listed `default-allow-ssh`/`default-allow-rdp`
in five projects.

```
$ gcloud asset search-all-resources --asset-types=compute.googleapis.com/Firewall --format=json | jq '[.[]|select(.versionedResources)]|length'
0          # of 27 firewall rules
$ … --read-mask='*' …
27
```

**Cause** — Cloud Asset `searchAllResources` omits `versionedResources` unless requested
with a read mask. Every jq filter on `.versionedResources[]?` saw nothing and printed nothing;
empty output is these checks' PASS criterion.

**Fix** — Added `--read-mask='*'` to all 15 commands. Verified: V55 now FAIL listing 11 rules
(including `simplifymycloud-dev/smc-fw-public-22`); V53, V59, V132 moved PASS → FAIL.
V15/V66/V68/V69/V150 still PASS because the org currently has **no VMs** — to be exercised
with test infrastructure.

## BUG-006 — V31 reports every member of the organization's own domain as "external"

| | |
|---|---|
| Found | 2026-09-13, org pass (run 1) |
| Component | `docs/cis-ig1-cli-validation.md` — V31 |
| Check | V31 (and V178, which inherits it) |
| Severity | wrong result — false FAIL; the one real external grant was buried in 27 false ones |

**Error**

```
V31 FAIL
//cloudresourcemanager.googleapis.com/organizations/933250405420	user:chris@simplifymy.cloud
//cloudresourcemanager.googleapis.com/projects/iq9-bootstrap	user:chris@simplifymy.cloud
… 27 rows, all @simplifymy.cloud, plus group:cloud-cluster-analytics-export@google.com
```

**Cause** — The domain was built with bash expansion `${DOMAIN//./\\.}` *inside* the jq program's
single quotes, so bash never expanded it; jq compared against the literal text `${DOMAIN…}` and
nothing matched.

**Fix** — Domain passed with `jq --arg domain "$DOMAIN"` and compared with
`ascii_downcase | endswith("@" + $domain)`. Verified run 3: V31 lists only
`group:cloud-cluster-analytics-export@google.com`.

## BUG-007 — Loop checks exit 1 when the resource is compliant (`[ … ] && echo`), reported ERROR

| | |
|---|---|
| Found | 2026-09-13, org and project passes (run 1–2) |
| Component | `docs/cis-ig1-cli-validation.md` |
| Check | V44, V49, V67, V78 observed ERROR; same pattern fixed in V30, V37, V38, V139, V155 |
| Severity | wrong result — compliant state reported as ERROR (and ERRORs degrade the pack) |

**Error**

```
V44  ERROR  (no stderr)
V49  ERROR  ``
V67  ERROR  ``
```

**Cause** — A script's exit status is its last command. `[ cond ] && echo …` as the last
statement of a loop body, or a trailing `grep`/`grep -q … && echo`, returns 1 when the
condition is false — i.e. when the last item is compliant. With empty output and non-zero
exit, `audit-run.go` records ERROR.

**Fix** — Rewritten as `if [ cond ]; then echo …; fi`, and trailing `grep` pipelines end in
`|| true`. Also, `audit-run.go` now writes "exit status N, no output and nothing on stderr" when
a check fails silently, instead of an empty Problems cell. Verified run 3: V44, V49, V67, V78 PASS.

## BUG-008 — Undefined `$p` in project checks (copied from an older all-projects loop)

| | |
|---|---|
| Found | 2026-09-13, project pass (run 2) |
| Component | `docs/cis-ig1-cli-validation.md` |
| Check | V78 (always failed); V30, V37, V67, V155 (blank project in output); V77 (wrong scope) |
| Severity | wrong result |

**Error**

```
V78  ERROR  (stderr hidden by 2>/dev/null; command was: gcloud projects get-iam-policy "" …)
V30  output: "LEGACY ACLs:  / bucket"
```

**Cause** — These checks were converted from "loop over every project with `$p`" to
single-project `$PROJECT_ID`, but `$p` references remained. V78 ran `get-iam-policy ""`.
V77 still looped over **every project in the organization** during a single-project pass.

**Fix** — `$p` → `$PROJECT_ID` throughout; V77 now checks only `$PROJECT_ID`. Verified run 3:
V78 PASS; V30 `LEGACY ACLs: iq9-gcp-dev-yamato / iq9-gcp-dev-yamato_cloudbuild` (confirmed
`uniform_bucket_level_access=False` directly); V77 `DEFAULT SA HAS EDITOR: iq9-gcp-dev-yamato`
(confirmed `168357744801-compute@` holds `roles/editor`).

## BUG-009 — Invalid gcloud syntax in individual checks

| | |
|---|---|
| Found | 2026-09-13, project pass (run 1–2) |
| Component | `docs/cis-ig1-cli-validation.md` |
| Check | V72, V124, V157 |
| Severity | blocks check — ERROR every run |

**Error**

```
V72   ERROR: (gcloud.compute.firewall-rules.list) Some requests did not succeed:
       - Invalid value for field 'filter': 'sourceRanges eq ".*\b35\.235\.240\.0/20\b.*"'. Invalid list filter expression.
V124  ERROR: (gcloud) Invalid choice: 'web-security-scanner'.
V157  bash: -c: line 4: syntax error: unexpected end of file
```

**Cause** — V72: `--filter="sourceRanges:35.235.240.0/20"` is translated to a server-side regex
the Compute API rejects. V124: `web-security-scanner` exists only as `gcloud alpha`. V157: a
trailing `\` line-continuation before `done`.

**Fix** — V72 `--filter="sourceRanges:(35.235.240.0/20)"` (tested: returns the IAP rule);
V124 `gcloud alpha web-security-scanner`; V157 continuation removed. Verified run 3: V72 REVIEW
with output, V124 N/A (API not enabled in project — correct), V157 REVIEW.

## BUG-010 — Bare placeholders never resolved (`BACKUP_PROJECT`, `POLICY`), commands run literally

| | |
|---|---|
| Found | 2026-09-13, project pass (run 2) |
| Component | `docs/cis-ig1-cli-validation.md` |
| Check | V166, V167, V174 |
| Severity | blocks check — ERROR (and earlier DENIED, marking the pack UNRELIABLE) |

**Error**

```
V166  ERROR: (gcloud.projects.describe) INVALID_ARGUMENT: Request contains an invalid argument.
V167  ERROR: (gcloud.projects.get-iam-policy) INVALID_ARGUMENT: Request contains an invalid argument.
```

**Cause** — `audit-run.go` only recognizes placeholders after `=` or `gs://`. These were bare
positional arguments (`gcloud projects describe BACKUP_PROJECT`), so they were never prompted
for, never substituted, and sent to the API literally. V174 used a literal `POLICY`.

**Fix** — V166/V167 assign `backup_project=BACKUP_PROJECT` first, so the placeholder is
recognized, prompted for, and can be answered `none`. V174 now loops over every security policy
rather than needing one named. Verified run 3: V166/V167 SKIP with "no value supplied for:
BACKUP_PROJECT" (correct until a value is given); V174 REVIEW.

## BUG-011 — V22 queried whatever cluster the operator's local kubectl pointed at

| | |
|---|---|
| Found | 2026-09-13, project pass (run 1–2) |
| Component | `docs/cis-ig1-cli-validation.md` — V22 |
| Check | V22 |
| Severity | wrong target — outside the audit identity and the project under audit |

**Error**

```
V22  ERROR  E0913 22:56:36 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list:
            Get \"https://136.118.234.188/api?timeout=32s\": dial tcp 136.118.234.188:443: i/o timeout"
```

**Cause** — `kubectl get pods` uses the operator's current kubeconfig context and the operator's
own credentials — not the impersonated audit SA, and not the project being audited. Here it hit an
unrelated, unreachable cluster.

**Fix** — Replaced with Cloud Asset Inventory: `gcloud asset search-all-resources
--scope=projects/$PROJECT_ID --asset-types=k8s.io/Pod --read-mask='*'`, listing namespace and
images. Read-only, runs as the audit SA, covers every cluster in the project, no kubeconfig.
Verified run 3: V22 REVIEW (no GKE in this project). "needs `kubectl`" → "needs `jq`".

## BUG-012 — V151 runs `gcloud compute ssh` inside a read-only audit

| | |
|---|---|
| Found | 2026-09-13, project pass (run 2) |
| Component | `docs/cis-ig1-cli-validation.md` — V151; `audit-run.go` parser |
| Check | V151 |
| Severity | safety — the audit would attempt to log in to (and push SSH keys to) an instance |

**Error**

```
V151  ERROR: (gcloud.compute.ssh) Underspecified resource [INSTANCE]. Specify the [--zone] flag.
```

**Cause** — The check is a by-hand command (SSH to a representative instance). The only thing
that stopped it executing was the unresolved `INSTANCE`; supplying a value would have run
`compute ssh` as the audit identity.

**Fix** — The command is now in a ```` ```sh ```` block. `audit-run.go` shows `sh` blocks but never
executes them: the check is reported REVIEW with "NOT RUN by audit-run — this command must be run
by hand", and `-list` shows it as `by-hand`. Verified run 3.

## BUG-013 — One disabled API in a multi-command check discarded the other command's results

| | |
|---|---|
| Found | 2026-09-13, project pass (run 2) |
| Component | `audit-run.go` — `execute()` |
| Check | V13 (`gcloud functions list` then `gcloud run services list`) |
| Severity | wrong result — Cloud Run services hidden behind N/A |

**Error**

```
V13  N/A
stdout: iq9-run-dev-yamato  us-west1 / iq9-run-dev-yamato-wiki  us-west1
stderr: Cloud Functions API has not been used in project iq9-gcp-dev-yamato before or it is disabled.
```

**Cause** — Any disabled-API text in stderr made the whole check N/A, regardless of stdout.

**Fix** — N/A only when there is no output. With output, the check is judged on it (REVIEW, or
FAIL for empty-means-pass checks) and the disabled API is noted. Verified run 3: V13 REVIEW.

## BUG-014 — Pack tables broken by multi-line cells; N/A had no reason

| | |
|---|---|
| Found | 2026-09-13, org and project packs (run 1–2) |
| Component | `audit-run.go` — `writePack()` |
| Check | Problems table (all ERROR/DENIED); N/A verdicts |
| Severity | diagnostics — Problems table rendered as broken markdown; N/A couldn't be told from a setup gap |

**Error**

```
| V20 | DENIED | `WARNING: This command is using service account impersonation. …
    … 15 more lines` |
```

**Cause** — The cell used `trim(errText, 1)`, which appends a newline and "… N more lines",
splitting the table row. N/A results carried their reason in `errText` but it was never printed.

**Fix** — New `cell()` renders the first line only, escaping `|` and backticks. The pack gains a
"Not applicable" table naming the disabled API and project for each N/A; cross-references carry
"inherited from …". Verified run 3.

## BUG-015 — V7 reports every project in the organization as "NO BILLING ACCOUNT"

| | |
|---|---|
| Found | 2026-09-13, project pass (run 3) |
| Component | `docs/cis-ig1-cli-validation.md` — V7; Terraform `enable_billing_viewer` docs |
| Check | V7 |
| Severity | wrong result — 18 false findings, including the project under audit |

**Error**

```
V7  NO BILLING ACCOUNT: gen-lang-client-0690825234
    NO BILLING ACCOUNT: iq9-gcp-dev-yamato
    … all 18 projects
$ gcloud billing projects describe iq9-gcp-dev-yamato   (as the audit SA)
billingAccountName: billingAccounts/000000-000002-6D3BF8
billingEnabled: true
```

**Cause** — The check listed billing accounts, then projects per account, and subtracted. The
audit SA can't list billing accounts (the billing account isn't in the organization, and
`billing.viewer` is off by default), and `2>/dev/null` hid that — so "billed projects" was always
empty and every project was reported unbilled. It also enumerated the whole org inside a
single-project pass.

**Fix** — V7 now runs `gcloud billing projects describe "$PROJECT_ID"
--format="value(billingEnabled)"`, which needs only `resourcemanager.projects.get` (already
granted), and prints `BILLING STATUS UNREADABLE` rather than a false finding if the read fails.
`roles/billing.viewer` is therefore needed by no check; `variables.tf` and the module readme say so.
Verified: V7 on `iq9-gcp-dev-yamato` returns no billing finding.

## BUG-016 — V86 writes the org's full IAM inventory into the current directory, outside the pack

| | |
|---|---|
| Found | 2026-09-13, org pass (run 3) |
| Component | `docs/cis-ig1-cli-validation.md` — V86, V175; `audit-run.go` |
| Check | V86 (also V175 `/tmp` collision) |
| Severity | evidence handling — a 24 KB org-wide IAM inventory appeared untracked at the repo root |

**Error**

```
$ git status --short
?? audit-state/
$ ls audit-state
iam-inventory.txt      # run used -pack ./scratch/audit-state/org
```

**Cause** — V86 hard-coded `./audit-state/iam-inventory.txt`, relative to wherever the operator
ran from, ignoring `-pack`. V175 (and old V7) wrote fixed names in `/tmp`, so two passes running
at once overwrite each other's project list.

**Fix** — `audit-run.go` creates the `-pack` directory up front and exports `AUDIT_PACK_DIR`;
V86 writes to `${AUDIT_PACK_DIR:-./audit-state}`. V175 uses `mktemp`. Verified: org run with
`-pack scratch/tmp-pack-o` puts `iam-inventory.txt` in the pack; nothing written at repo root.

---

## Test fixtures

From here, deliberately non-compliant resources were deployed to `iq9-gcp-dev-yamato`
(uncommitted Terraform in `scratch/test-infra/`) so checks had something to find: an auto-mode
VPC, an e2-micro VM (external IP, not shielded, default compute SA, unlabelled, no rule reaches
it), internet-open firewall rules for tcp:3306, **all protocols**, and tcp:**20-25** (all targeting
a tag nothing carries), an untargeted internal rule, a BigQuery dataset with no expiration, and a
project-wide `ssh-keys` metadata entry (dummy public key). The org policy
`compute.requireOsLogin` refused a VM with `enable-oslogin=FALSE`, so the VM inherits OS Login
from project metadata instead — which became a test in itself (BUG-018).

## BUG-017 — V55/V56 miss internet-open rules that allow all protocols or a port range

| | |
|---|---|
| Found | 2026-09-13, org pass against fixtures |
| Component | `docs/cis-ig1-cli-validation.md` — V55, V56 |
| Check | V55 (SSH/RDP), V56 (database ports) |
| Severity | wrong result — false PASS on the highest-risk firewall rules |

**Error** — no error. With fixtures `cis-test-open-all-protocols` (`allow { protocol = "all" }`) and
`cis-test-open-port-range` (tcp `20-25`), both from `0.0.0.0/0`:

```
V55 output: default-allow-ssh/rdp rules only — neither fixture listed
V56 output: OPEN DB PORT: …/cis-test-open-mysql            — all-protocols rule not listed
```

**Cause** — The jq matched only a literal port string equal to `"22"`, `"3389"`, etc. A rule
with `IPProtocol: "all"`, a tcp rule with no `ports` (every port), or a range like `"20-25"` or
`"0-65535"` never matched. IPv6 `::/0` was also ignored.

**Fix** — A `covers($n)` jq function tests each port or range. A rule is flagged if it is open to
`0.0.0.0/0` or `::/0` and allows protocol `all`, or tcp with no port list, or a port/range covering
a watched port. Verified: V55 lists both fixtures plus all 11 previously-found rules (13 total);
V56 lists `cis-test-open-mysql` and `cis-test-open-all-protocols`.

## BUG-018 — V66 ignores project-level OS Login: false FAIL on every VM that inherits it

| | |
|---|---|
| Found | 2026-09-13, org pass against fixtures |
| Component | `docs/cis-ig1-cli-validation.md` — V66 |
| Check | V66 |
| Severity | wrong result — false FAIL (OS Login is normally set at project level) |

**Error**

```
V66 FAIL
NO OS LOGIN: //compute.googleapis.com/projects/iq9-gcp-dev-yamato/zones/us-west1-a/instances/cis-test-noncompliant-vm
$ gcloud compute project-info describe --project=iq9-gcp-dev-yamato  →  enable-oslogin = true
```

**Cause** — Only instance metadata was read. OS Login is effective if the instance sets
`enable-oslogin`, **or, when it doesn't, its project does** — the usual configuration.

**Fix** — Also reads project metadata (Asset type `compute.googleapis.com/Project`, via
`jq --slurpfile`), and computes the effective value as instance value, else project value, else
off. The org policy is deliberately not treated as proof (it isn't retroactive). Verified: V66 PASS
for the fixture VM.

## BUG-019 — V67 looks only for the deprecated `sshKeys` key: false PASS

| | |
|---|---|
| Found | 2026-09-13, project pass against fixtures |
| Component | `docs/cis-ig1-cli-validation.md` — V67 |
| Check | V67 |
| Severity | wrong result — false PASS with project-wide SSH keys present |

**Error**

```
V67 PASS
$ gcloud compute project-info describe --project=iq9-gcp-dev-yamato --format="value(commonInstanceMetadata.items[].key)"
enable-oslogin;ssh-keys
```

**Cause** — `grep -qw "sshKeys"` matches only the legacy key. Current tooling (console, gcloud,
Terraform) writes `ssh-keys`.

**Fix** — Key list split on `;` and matched exactly against `ssh-keys|sshKeys`. Verified:
`PROJECT-WIDE SSH KEYS: iq9-gcp-dev-yamato ssh-keys`, V67 FAIL.

## BUG-020 — V147 skips exactly the VMs whose agent can't be verified: false PASS

| | |
|---|---|
| Found | 2026-09-13, project pass against fixtures |
| Component | `docs/cis-ig1-cli-validation.md` — V147 |
| Check | V147 |
| Severity | wrong result — false PASS |

**Error**

```
V147 PASS   (project has a running VM; OS Config API is not enabled in the project)
```

**Cause** — The loop started from `gcloud compute instances os-inventory list-instances`, which
returns only VMs that already report OS inventory. A VM with no OS Config agent — or in a project
without the OS Config API — never entered the loop. Errors from `describe` were also discarded
(`2>/dev/null`), and the jq path `.items.installedPackages` didn't match gcloud's output shape.

**Fix** — Loops over `gcloud compute instances list`. A VM whose inventory can't be read reports
`NO OS INVENTORY (agent presence unverifiable)`; otherwise the whole inventory document is searched
for AV package names, independent of its layout. Verified: V147 FAIL
`NO OS INVENTORY (agent presence unverifiable): cis-test-noncompliant-vm`.

## BUG-021 — 27 cross-references point at the wrong check; requirements scored from unrelated results

| | |
|---|---|
| Found | 2026-09-13, rollup of run 4 |
| Component | `docs/cis-ig1-cli-validation.md` — `See Vn` lines and `**Pass:**` lines of xref entries |
| Check | V41 V48 V51 V62 V80 V82 V85 V89 V93 V100 V101 V109 V110 V112 V117 V119 V123 V144 V148 V149 V169 V173 V176 V178 V179 V182 V186 |
| Severity | wrong result — an xref inherits its target's verdict, so these requirements were ticked or failed on evidence about something else |

**Error** — seen in the remediation plan:

```
| 9 | `17.2#2` | Contacts set for Legal, Suspension and Technical | project × 1 | iq9-gcp-dev-yamato |
V182 "Contacts set for Legal, Suspension and Technical" → See V163 "Bucket Lock applied to backup buckets"
```

A full listing showed the pattern: most targets were off by one or two positions (e.g. V51
"No auto-mode networks remain" → V50 "Default firewall rules deleted"; V110 "No VMs reachable on
22/3389" → V57 "Default-deny ingress posture"), consistent with checks being renumbered after
the cross-references were written.

**Cause** — Cross-reference targets not updated when checks were renumbered.

**Fix** — Each target re-chosen by matching the requirement to the check that measures it:

| Xref | Requirement | Was | Now |
|---|---|---|---|
| V41 | Buckets and datasets with no retention rule | V38, V39 | V37, V38 |
| V48 | Default network deleted from every project | V47 | V46 |
| V51 | No auto-mode or legacy networks remain | V50 | V49 |
| V62 | Existing open firewall rules removed | V57, V58 | V55, V56 |
| V80 | Editor stripped from default SAs | V71, V72 | V77, V78 |
| V82 | Workloads run as purpose-built SAs | V75 | V81 |
| V85 | Default network and firewall rules removed | V47, V51 | V46, V50 |
| V89 | External principals identified | V33 | V31 |
| V93 | Existing keys inventoried, aged, eliminated | V83 | V91, V92 |
| V100 | Pre-existing basic-role grants replaced | V32 | V34, V99 |
| V101 | Basic roles replaced throughout | V32 | V34 |
| V109 | SSH/RDP through IAP TCP forwarding | V67 | V72 |
| V110 | No VMs reachable on 22/3389 from internet | V57 | V55 |
| V112 | Bastion hosts removed or behind IAP | V66 | V68, V72 |
| V117 | OS Config agent coverage complete | V10 | V9 |
| V119 | Instance templates reference current images | V17 | V19 |
| V123 | Serverless on supported runtimes | V15 | V17 |
| V144 | Egress rules constrain destinations | V60 | V58 |
| V148 | Container image malware scanning | V116 | V122 |
| V149 | Binary Authorization preventing unattested images | V24 | V21 |
| V169 | GKE versions within supported window | V14 | V16 |
| V173 | Legacy networks eliminated | V50 | V49 |
| V176 | Workload Identity Federation trusts documented | V86 | V94 |
| V178 | Domain restriction constraint enforced | V31 | V35 |
| V179 | External grants predating the constraint | V33 | V31 |
| V182 | Contacts for Legal, Suspension, Technical | V163 | V181 |
| V186 | SCC findings routed to a monitored destination | V113 | V115 |

Matching `**Pass:**` text updated where it named the old check. **These mappings are a judgement
call and should be reviewed by the kit's author.** Verified run 5: V182 inherits V181, V110
inherits V55 (FAIL), V186 inherits V115.

## BUG-022 — Cross-references ran in both passes, unresolved in one; partial evidence could PASS

| | |
|---|---|
| Found | 2026-09-13, runs 1–4 |
| Component | `audit-run.go` — scope filter in `main()`, `resolveXrefs()` |
| Check | all 30 xrefs, e.g. V112 (needs org-scope V68 and project-scope V72) |
| Severity | wrong result — XREF noise in every pass; a two-target xref could PASS on half its evidence |

**Error**

```
org pack:     97 checks (67 org + all 30 xrefs)   — xrefs to project checks left as XREF
project pack: 121 checks (91 project + all 30 xrefs)
```

**Cause** — Every xref was added to both passes regardless of where its targets run.
`resolveXrefs` silently ignored targets missing from the pass and took the worst of the rest.

**Fix** — New `inPass()` includes an xref only in a pass that runs at least one of its targets
(org 82, project 108 checks). When some targets run in the other pass and the found ones would give
PASS/REVIEW, the xref is REVIEW with "also requires Vn, which runs in the other pass". Verified
run 5: V112 org FAIL (V68), project REVIEW (V72 + note).

## BUG-023 — V99 (org/folder IAM) tagged project scope: repeated in every project pass

| | |
|---|---|
| Found | 2026-09-13, rollup of run 4 |
| Component | `docs/cis-ig1-cli-validation.md` — V99 |
| Check | V99 |
| Severity | wrong result — one org finding becomes N identical per-project work items |

**Error**

```
| 17 | `5.4#1` | No basic roles held by individuals at org or folder level | project × 1 | iq9-gcp-dev-yamato |
```

**Cause** — The command reads `gcloud organizations get-iam-policy $ORG_ID` but the entry is
`scope: project`. On a 105-project estate the rollup would show "project × 105" for one org binding.

**Fix** — `scope: org`; the scope summary table updated to 68 org / 90 project. Verified run 5:
V99 appears in the org pack only.

## BUG-024 — Remediation plan links to the checks are broken

| | |
|---|---|
| Found | 2026-09-13, rollup of run 4 |
| Component | `rollup.go` — `writePlan()` |
| Check | — |
| Severity | cosmetic — every "Check Vn" / "how to fix" link 404s |

**Error**

```
Check [V147](cis-ig1-cli-validation.md#v147) · … [how to fix](cis-ig1-remediation-reference.md)
# plan written to ./audit-state/remediation-plan.md (runbook) or ./remediation-plan.md (default)
```

**Cause** — Links were hard-coded relative to `docs/`, but the plan is never written there.

**Fix** — Link prefix computed with `filepath.Rel` from the plan's directory to `docs/`. Verified:
`-out scratch/remediation-plan.md` produces `../docs/cis-ig1-cli-validation.md#v147`.

## BUG-025 — Project checks loop over every project in the org but query the same project N times

| | |
|---|---|
| Found | 2026-09-13, API request metrics for run 4 |
| Component | `docs/cis-ig1-cli-validation.md` — V42, V83, V84, V160 |
| Check | V42, V83, V84, V160 |
| Severity | wrong result + cost — duplicated output ×N projects, N× API calls, errors hidden; V83 wrong flag, V160 missed regional keys |

**Error** — API metrics for the audit SA during one project pass of `iq9-gcp-dev-yamato`:

```
sqladmin.googleapis.com   calls=41  List
cloudkms.googleapis.com   calls=18  403 ListKeyRings
$ gcloud kms keyrings list --location=global --project=iq9-gcp-dev-yamato
ERROR: … Google Cloud KMS API has not been used in project 168357744801 before or it is disabled.
```

**Cause** — Each check wrapped a `$PROJECT_ID` query in `gcloud projects list | while read -r p`,
left over from an all-projects version. With 18 projects, every call ran 18 times against the
same project, and V42/V84 printed each result 18 times. `2>/dev/null` everywhere turned the
disabled KMS API into empty "clean" output (REVIEW) instead of N/A. V83 used `--region` for
clusters whose location may be a zone. V160 only listed key rings in `global`.

**Fix** — Outer loops removed; `2>/dev/null` removed so errors reach the verdict logic. V83 uses
`--location`. V42 reads both values in one `describe` with an explicit `|` separator (a first
attempt with tab-splitting shifted soft-delete into the versioning column when versioning was
unset — caught in verification). V160 now uses one Asset Inventory call,
`search-all-iam-policies --scope=projects/$PROJECT_ID --asset-types=cloudkms.googleapis.com/CryptoKey`,
covering every location. Verified: V42 `versioning=off soft_delete=604800` (matches
`gcloud storage buckets describe`); V84 lists `yamato-dev` users once; V160 empty (no keys); V83 N/A (GKE API off).

## BUG-026 — `2>/dev/null` turns a failed command into a PASS

| | |
|---|---|
| Found | 2026-09-13, project pass against a non-existent project (deliberate negative test) |
| Component | `docs/cis-ig1-cli-validation.md` — V30 V37 V49 V91 (auto-scored); V7 V88 V94 V133 V143 V155 V156 V157 V184 (review) |
| Check | as listed |
| Severity | wrong result — false PASS whenever the audit identity can't read the project |

**Error**

```
$ go run audit-run.go -scope=project -project=cis-test-no-such-project-4242 -only V30,V37,V49,V91
V30 PASS   V37 PASS   V49 PASS   V91 PASS
```

**Cause** — The listing command's stderr was discarded. When it failed (no such project, no
permission, API disabled) it printed nothing; empty output is these checks' PASS criterion, and
`audit-run.go` never saw the error. Review checks showed an empty result a human would read as
"none found". V91 also still echoed the undefined `$p`. V7 listed `DELETE_REQUESTED` projects for
the whole org inside a single-project pass.

**Fix** — `2>/dev/null` removed from those project checks (org checks where a denial is expected
— sink destinations outside the org — left as they are). V91 `$p` → `$PROJECT_ID`. V7 reports only
the audited project's `lifecycleState`. Verified: against the non-existent project V30/V37/V49/V88/V91
now ERROR and V7/V94/V184 DENIED; against `iq9-gcp-dev-yamato` verdicts unchanged (V30/V37/V49 FAIL,
V91 PASS), V156/V157 N/A with the disabled API named.

## BUG-027 — V133 never ran: two commands joined by a stray line continuation

| | |
|---|---|
| Found | 2026-09-13, project pass |
| Component | `docs/cis-ig1-cli-validation.md` — V133 |
| Check | V133 |
| Severity | wrong result — empty REVIEW every run; the firewall part of "DNS, NAT and firewall logging" was never checked |

**Error**

```
ERROR: (gcloud.dns.policies.list) unrecognized arguments:
  gcloud
  compute
  routers
  list
```

**Cause** — A trailing `\` after `gcloud dns policies list …` appended the next line
(`gcloud compute routers list …`) as arguments. The failure was hidden by `2>/dev/null`, and bash
exited 0 through the rest of the pipeline.

**Fix** — Continuation removed; output sectioned into DNS policies, firewall rules
(`logConfig.enable`, which the check title promised but never queried), and Cloud NAT. Verified:
lists 5 firewall rules and `iq9-nat-bakery-us-we1 True`; DNS section notes the API is disabled.

## BUG-028 — Runbook enables 6 APIs; the audit needs 17 in the host project

| | |
|---|---|
| Found | 2026-09-13/14, runs 3–8 (errors) and API request metrics (run 8) |
| Component | `docs/cis-ig1-audit-runbook.md` Phase 1, `docs/cis-ig1-run-sheet.md` A3 |
| Check | V115, V157 observed ERROR; the rest measured (see `required-apis.md`) |
| Severity | blocks run — on a host project without the missing APIs, checks ERROR |

**Error**

```
V115  ERROR  API DISABLED IN AUDIT HOST PROJECT: securitycenter.googleapis.com on simplifymycloud-dev — enable it there and re-run
V157  API [spanner.googleapis.com] not enabled on project [288261943767]
```

**Cause** — The documented list (`cloudasset essentialcontacts accesscontextmanager recommender
policyanalyzer osconfig`) omitted 11 APIs that the audit's calls are billed to the host project for:
`bigquery cloudbilling cloudresourcemanager iam iamcredentials logging monitoring orgpolicy
securitycenter serviceusage spanner sqladmin`, less the ones already listed. It also included
`osconfig`, whose calls bill to the audited project, so enabling it in the host did nothing.
The test host project already had most of these enabled, which is how the gap stayed hidden.

**Fix** — Both documents now enable the 17 measured APIs and explain the host/audited-project
split. Verified: run 8 — both passes `RUN STATUS: OK`, 0 ERROR, 0 DENIED. Full evidence per API is in
[`required-apis.md`](required-apis.md). Recommended: one confirmation run from an empty host project.
