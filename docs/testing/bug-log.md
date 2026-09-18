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
(uncommitted Terraform, now in `scratch/teardown/test-infra/`) so checks had something to find: an auto-mode
VPC, an e2-micro VM (external IP, not shielded, default compute SA, unlabelled, no rule reaches
it), internet-open firewall rules for tcp:3306, **all protocols**, and tcp:**20-25** (all targeting
a tag nothing carries), an untargeted internal rule, a BigQuery dataset with no expiration, and a
project-wide `ssh-keys` metadata entry (dummy public key). The org policy
`compute.requireOsLogin` refused a VM with `enable-oslogin=FALSE`, so the VM inherits OS Login
from project metadata instead — which became a test in itself (BUG-018).

Added 2026-09-14: an **empty** bucket readable by `allUsers` (`cis-test-public-bucket-iq9-yamato`).
V27 and V29 FAIL, naming it; they PASSed before it existed. A user-managed SA key fixture was **not**
created: the org enforces `iam.disableServiceAccountKeyCreation`, and org policy stays untouched.
Instead the key checks ran read-only against `simplifymycloud-dev`, which has keys that predate the
constraint. V91 FAILs, listing both user-managed keys Asset Inventory reports. V92 lists key
last-authentication times. No fault in either.

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

---

## Documentation pass — 2026-09-14

Every relative link and anchor was checked, and every command block in the run sheet was executed.
The `gcloud/` scripts ran a real create → verify → destroy → verify cycle under macOS's stock
`/bin/bash` 3.2, with a throwaway account (`cis-ig1-docs-test`, prefix `cisIg1DocTest`) since
deleted. Every read-only command in the remediation reference ran against the test org (26 blocks).

## BUG-029 — Doc links break on GitHub: files committed as `README.md`, linked as `readme.md`

| | |
|---|---|
| Found | 2026-09-14, link check against exact tracked filenames |
| Component | `readme.md`, `docs/readme.md`, `docs/training/readme.md`, `terraform/audit-service-account/readme.md` |
| Severity | broken links — work on macOS (case-insensitive disk), 404 on GitHub and Linux |

**Error**

```
git ls-files: README.md  docs/README.md  docs/training/README.md  terraform/audit-service-account/README.md
              gcloud/readme.md  terraform/readme.md
gcloud/readme.md:5: ../terraform/audit-service-account/readme.md -> not a tracked path
```

**Cause** — Mixed case in git; every link uses lowercase.

**Fix** — The four files renamed to `readme.md` in git (GitHub still renders it as the folder page).
Link and anchor check: 0 problems.

## BUG-030 — Smoke test in the run sheet, runbook and scripted audit refuses to run

| | |
|---|---|
| Found | 2026-09-14, run sheet A7 executed |
| Component | `docs/cis-ig1-run-sheet.md` A7, `docs/cis-ig1-audit-runbook.md` Phase 3, `docs/cis-ig1-scripted-audit.md` |
| Severity | blocks run — the first `go run` in the procedure exits |

**Error**

```
$ go run audit-run.go -only V43,V27,V86,V91,V125,V181 -org="$ORG_ID"
audit-run: -scope is required.
```

**Cause** — `-scope` became mandatory; the smoke test predates it. It also mixed V91 (project
scope) into what can only be an org-scope run.

**Fix** — `go run audit-run.go -scope=org -org="$ORG_ID" -only V27,V43,V86,V125,V181 -no-prompt`
(five org checks). Verified: 5 checks, 0 DENIED. The training demo had the same mix-up (V127 in
an org run silently ran 3 of 4); V55 substituted and verified. Legacy
`gcloud resource-manager org-policies list` replaced by `gcloud org-policies list` in five places.
Check counts 67/91 → 68/90 and "two values" → three (adds `BACKUP_PROJECT`) throughout.

## BUG-031 — `create.sh --dry-run` is not dry, and reports grants it never made

| | |
|---|---|
| Found | 2026-09-14, `gcloud/create.sh --dry-run` |
| Component | `gcloud/create.sh` |
| Severity | safety — a dry run can undelete roles; its output claims success |

**Error**

```
2/4  custom roles
                       (nothing printed)
3/4  organization role bindings
  ok      roles/browser         (dry run — nothing was granted)
```

**Cause** — `gcloud iam roles undelete` ran directly, outside the `run` dry-run wrapper. The
wrapper's "[dry-run]" line was sent to `/dev/null` with the command's output, and the wrapper
returns success, so every binding printed `ok`. In a real run, role creation failures aborted
without explanation, including the 7–37-day "marked for deletion" case (BUG-001).

**Fix** — Dry run skips undelete/create/grant entirely and prints `[dry-run] would …`. Role
creation captures the error and, for "marked for deletion", tells the operator to re-run with
`--role-prefix`. Verified: dry run prints 33 would-grant lines and writes no record; real run
created the account, 3 roles and 33 bindings.

## BUG-032 — `destroy.sh` fails on macOS: `mapfile: command not found`

| | |
|---|---|
| Found | 2026-09-14, compatibility check under `/bin/bash` 3.2 |
| Component | `gcloud/destroy.sh` |
| Severity | blocks teardown — with `set -e` the script exits before revoking anything |

**Error**

```
$ /bin/bash -c 'mapfile -t X < <(echo a)'
/bin/bash: mapfile: command not found
```

**Cause** — `mapfile` is bash 4+. macOS ships bash 3.2, and this machine has no other bash.

**Fix** — Arrays filled with `while read` loops. Its closing note about role IDs corrected too.
Verified: under `/bin/bash` 3.2, destroy revoked 33 bindings, deleted 3 roles and the account;
"no organization bindings remain".

## BUG-033 — `verify.sh` after teardown reports deleted custom roles as present

| | |
|---|---|
| Found | 2026-09-14, `verify.sh` after `destroy.sh` |
| Component | `gcloud/verify.sh` section 5 |
| Severity | wrong result — teardown looks incomplete |

**Error**

```
5  custom role permissions
   cisIg1DocTestKeyReader
      iam.serviceAccountKeys.list        (role was deleted)
```

**Cause** — `gcloud iam roles describe` keeps returning a role for days after deletion, with
`deleted: True`; the script only treated "not found" as gone.

**Fix** — Reads `deleted` too and prints "soft-deleted (correct after teardown)". Verified after
destroy; a live role still prints its permissions.

## BUG-034 — `-init-config` fails on a fresh clone

| | |
|---|---|
| Found | 2026-09-14, following the quick start |
| Component | `audit-run.go` — `writeConfigTemplate()` |
| Severity | blocks run — documented first command fails where `./audit-state` doesn't exist |

**Error**

```
audit-run: open ./audit-state/audit.env: no such file or directory
```

**Cause** — The template writer didn't create its directory; only the run sheet has a `mkdir`.
Its closing hint also suggested a command with no `-scope` (which fails, see BUG-030).

**Fix** — Creates the parent directory; hint now reads
`go run audit-run.go -scope=org -org=$ORG_ID -config … -pack ./audit-state/org`. Verified into a
non-existent directory.

## BUG-035 — Remediation reference repeats the check bugs fixed earlier

| | |
|---|---|
| Found | 2026-09-14, remediation reference review and execution |
| Component | `docs/cis-ig1-remediation-reference.md` "find what already exists" queries |
| Severity | wrong result — the queries engineers use to scope a fix returned nothing |

**Error** — seven Asset Inventory queries without `--read-mask` (BUG-005), exact-port firewall
matching (BUG-017), instance-only OS Login (BUG-018), and a `kubectl` loop that rewrote the
operator's kubeconfig with `--region` (BUG-011). After rewriting, one query failed on execution:

```
jq: error: syntax error, unexpected INVALID_CHARACTER … line 6, column 110
```

**Cause** — Commands copied from the checks before those were fixed. The jq error was an extra
closing parenthesis in the rewrite.

**Fix** — Same fixes as the checks; parenthesis corrected. Verified: all 26 read-only command
blocks run without error. The open-ports query finds 14 rules, including the three fixtures.

## BUG-036 — Committed `terraform.tfvars` targeted the test organization

| | |
|---|---|
| Found | 2026-09-14, documentation pass |
| Component | `terraform/audit-service-account/terraform.tfvars` |
| Severity | safety — an unedited clone would apply the audit identity to the wrong organization |

**Cause** — The test run's real values (org `933250405420`, `simplifymycloud-dev`, prefix
`cisIg1Audit2`) were committed in BUG-001.

**Fix** — Restored `REPLACE_*` placeholders, with the prefix line commented. `terraform plan` now
stops with "organization_id must be the numeric ID only". Test-org values moved to
`scratch/teardown/test-org.tfvars`; the live test identity is still managed with
`terraform plan -var-file=../../scratch/teardown/test-org.tfvars` (No changes). Terraform readme counts
corrected: 33 roles and 38 resources by default, not "35" and "around 40".

## BUG-037 — Remediation plan links break when the output path is absolute

| | |
|---|---|
| Found | 2026-09-14, first `run-audit.sh` run |
| Component | `rollup.go` — `writePlan()` |
| Severity | cosmetic — every check link in the plan 404s |

**Error**

```
Check [V147](docs/cis-ig1-cli-validation.md#v147)      # plan at scratch/runs/<run>/report/, via an absolute -out path
```

**Cause** — The BUG-024 fix computed `filepath.Rel(dir-of-out, "docs")`. That errors when one path
is absolute and the other relative, and the fallback was the bare `docs`. `run-audit.sh` passes
absolute paths.

**Fix** — Both paths made absolute before `Rel`. Verified with an absolute and a relative `-out`:
both produce `../../docs/…` from two levels down.

---

## Customer-org findings — 2026-09-14

First run against a large organization (105 projects), during a live demo.

## BUG-038 — V96 hangs the organization pass: an unbounded 90-day log read

| | |
|---|---|
| Found | 2026-09-14, org pass stalled at 61/68 on a 105-project organization |
| Component | `docs/cis-ig1-cli-validation.md` — V96 |
| Severity | blocks run — the org pass never finishes |

**Error** — no error; the progress counter stopped. The last check to finish was V183, but the one
running was V96:

```
gcloud logging read 'protoPayload.authenticationInfo.principalEmail:"@$DOMAIN"' \
  --organization=$ORG_ID --freshness=90d --format=… | sort -u
```

**Cause** — No `--limit`, so gcloud pages through 90 days of the organization's audit log. That took
seconds in the test organization and hours in a large one. The filter was also in single quotes,
so `$DOMAIN` was never substituted: the read matched nothing and the check was useless even
when it finished.

**Fix** — Double-quoted filter so the domain is filled in; `--limit=5000` (most recent entries); pass
criterion now says to confirm dormancy in the Workspace login report. Verified in the test organization:
22 s, returns `chris@simplifymy.cloud` (previously empty). No other `logging read` check is unbounded.

## BUG-039 — The per-check timeout never stops a hung check

| | |
|---|---|
| Found | 2026-09-14, same stall — the 3-minute timeout didn't fire |
| Component | `audit-run.go` — `execute()` |
| Severity | blocks run — any slow gcloud call hangs the whole pass |

**Error** — reproduced with a check `sleep 600 \| sort -u` and `-timeout 5s`: before the fix the run
never returned.

**Cause** — `exec.CommandContext` kills only `bash` on timeout. Its child (`gcloud`, or here `sleep`)
keeps the output pipe open, and `cmd.Run()` waits for the pipe to close. First flagged by the static
review on 2026-09-13; not reproduced until a large organization.

**Fix** — Each check runs in its own process group (`Setpgid`). On timeout the whole group is
killed, and `WaitDelay` stops waiting on the pipes 5 s later. Verified: the check reports
`ERROR · timed out after 5s`, the run completes in 9 s, and no child process survives.
*Note:* process groups are Unix-only, so the scripts run on macOS, Linux and Cloud Shell, not native Windows.

## BUG-040 — No way to leave Apps Script `sys-` projects out, or to skip one hung check

| | |
|---|---|
| Found | 2026-09-14, customer organization |
| Component | `audit-run.go`, `run-audit.sh` |
| Severity | blocks run — `sys-` projects inflate `--all` and can't be excluded; a hung check can only be bypassed by editing the doc |

**Cause** — Apps Script creates a `sys-<number>` project per script, under `system-gsuite/apps-script`.
They are real projects in the organization, so `--all` and project lists included them. There was
also no skip option and no way to set parallelism through `run-audit.sh`.

**Fix**

- `EXCLUDE_PROJECTS` (regex; default `^sys-`; `none` disables), read from `-config`/`-set`/environment.
  - `run-audit.sh` filters `--all`, `--projects` and `--project` with it, writes `evidence/excluded.txt`, and shows the count.
  - `audit-run.go -scope=project` refuses an excluded project (exit 2).
  - `-init-config` writes the line.
- `audit-run.go -skip V96,…` / `run-audit.sh --skip`: those checks are reported as SKIP ("the operator excluded this check with -skip"), not silently dropped. The console summary says "N CHECKS NOT RUN — excluded with -skip" instead of asking for placeholder values.
- `run-audit.sh --parallel N` passes through to `-parallel`.

Verified under `/bin/bash` 3.2: a list of `iq9-gcp-dev-yamato` + `sys-12345678901234567890123456`
with `--skip V96 --parallel 4` gave 1 audited, 1 excluded (recorded with reason), V96 SKIP, both passes
`RUN STATUS: OK`, 51 findings. `audit-run.go -project=sys-…` is refused with the pattern named.

**Still open (awaiting decision)** — organization checks that query every project at once through
Cloud Asset Inventory (32 of 68) still include `sys-` projects in their results.

---

## Automation pass — 2026-09-15

68 REVIEW checks rewritten to score themselves (see [review-triage.md](../design/review-triage.md)). Found while
converting and testing them:

## BUG-041 — `gcloud storage buckets describe` field names never match: false FAIL on versioned or locked buckets

| | |
|---|---|
| Found | 2026-09-15, V165 returned an empty project number; traced to every bucket `describe` |
| Component | `docs/cis-ig1-cli-validation.md` — V37, V42, V139, V155 (existing); V152, V159, V163, V165, V168 (new) |
| Severity | wrong result — every bucket reported "NO VERSIONING" / "NO BUCKET LOCK" regardless of its settings |

**Error**

```
$ gcloud storage buckets describe gs://iq9-iac-ops-tf-state-bucket --format="value(versioning.enabled)"
                                   # empty — on a bucket with versioning ON
$ gcloud storage buckets describe gs://iq9-iac-ops-tf-state-bucket --format=json | jq -c keys
[..., "versioning_enabled"]        # its own snake_case names, and unset keys are omitted
$ gcloud storage buckets describe gs://iq9-iac-ops-tf-state-bucket --raw --format="value(versioning.enabled)"
True
```

**Cause** — Without `--raw`, `gcloud storage` prints its own field names (`versioning_enabled`,
`default_kms_key`, …) and omits unset keys. The checks used Cloud Storage JSON API names (`versioning.enabled`,
`retention_policy.isLocked`), so the value was always empty. V155 listed every bucket in every project as
unversioned since day one.

**Fix** — Every bucket `describe` uses `--raw` with JSON API fields: `versioning.enabled`,
`retentionPolicy.isLocked`, `encryption.defaultKmsKeyName`, `lifecycle.rule`,
`iamConfiguration.uniformBucketLevelAccess.enabled`, `projectNumber`, `softDeletePolicy.retentionDurationSeconds`.
Verified on real buckets: versioning `True` on `iq9-iac-ops-tf-state-bucket` (V152 PASS), UBLA `True`,
`projectNumber`, and `lifecycle.rule` on a new fixture bucket `cis-test-lifecycle-retention-iq9-yamato` (V37 no longer
flags it). Unlocked retention omits `isLocked`, correctly read as unlocked. *Not verifiable in this org:* a locked
retention policy (locking is irreversible) and a CMEK default key (none exists).

## BUG-042 — V17 flagged every modern Python runtime as decommissioned

| | |
|---|---|
| Found | 2026-09-15, rewriting V17 |
| Component | `docs/cis-ig1-cli-validation.md` — V17 |
| Severity | wrong result — false finding for `python310`, `python311`, `python312`… |

**Cause** — `grep -E "python3(7)?"` matches any runtime containing `python3`; the `(7)?` is optional.

**Fix** — The runtime field is matched exactly: `^(nodejs(8|10|12|14)|python37|go1(11|13)|ruby2[0-9])$`.

## BUG-043 — Auditing the host project reports its disabled product APIs as setup gaps

| | |
|---|---|
| Found | 2026-09-15, project pass of `simplifymycloud-dev` (which also hosts the audit SA) |
| Component | `audit-run.go` — `execute()` |
| Severity | wrong result — ERROR instead of N/A |

**Error**

```
V20 ERROR  API DISABLED IN AUDIT HOST PROJECT: binaryauthorization.googleapis.com on simplifymycloud-dev — enable it there
```

**Cause** — Binary Authorization is gated by the audited project. When that project is also the host, the
disabled-API project matches the host and was classified as a setup gap.

**Fix** — In a project pass of the host project itself, a disabled API is N/A. Verified: V20/V21 N/A with the reason.

## BUG-044 — V126 read a 24-hour window: a quiet day looked like logs not flowing

| | |
|---|---|
| Found | 2026-09-15 |
| Component | `docs/cis-ig1-cli-validation.md` — V126 |
| Severity | wrong result — false FAIL |

**Cause** — Organization-level Admin Activity entries appear only when org-level settings change; none in 24 hours is
normal. **Fix** — 30-day window. Verified: PASS (last entry the previous day).

## BUG-045 — Organization queries in the project pass (V6, V113, V127)

| | |
|---|---|
| Found | 2026-09-15, REVIEW triage |
| Component | `docs/cis-ig1-cli-validation.md` |
| Severity | wasted work — identical output once per project (105× on a large org), duplicated findings |

**Fix** — `scope: org` (same class as BUG-023). Passes are now 71 org / 87 project checks.

## BUG-046 — `excluded.txt` empty and "0 excluded" reported when EXCLUDE_PROJECTS is an alternation

| | |
|---|---|
| Found | 2026-09-17, pre-customer validation pass |
| Component | `run-audit.sh` |
| Severity | **audit integrity** — the evidence says nothing was excluded while projects were excluded |

**Error**

```
sed: 1: "s|$|  (matches EXCLUDE_ ...": bad flag in substitute command: 'e'
  projects      1 audited, 0 excluded (EXCLUDE_PROJECTS ^(smc-|gen-|iq9-b|iq9-l|iq9-o|simplify))
```

**Cause** — the excluded list was annotated with `sed "s|$|  (matches EXCLUDE_PROJECTS $EXCLUDE)|"`. Any alternation
regex contains `|`, which is the delimiter, so sed failed, `excluded.txt` was written empty, and `EXCLUDED_COUNT`
counted its zero lines. Filtering itself was correct — only the record of it was lost. The default `^sys-` has no
`|`, which is why this survived the 105-project customer-org run: the one case that works is the default.

**Why it matters** — a report claiming every project was assessed, when 17 were skipped, is the kind of finding an
auditor is supposed to catch rather than produce.

**Fix** — annotate with `awk` reading the value from `ENVIRON`, which has no delimiter to collide with and does not
interpret backslash escapes the way `awk -v` would.

**Verification** — same run, after the fix:

```
  projects      1 audited, 17 excluded (EXCLUDE_PROJECTS ^(smc-|gen-|iq9-b|iq9-l|iq9-o|simplify))
gen-lang-client-0690825234  (matches EXCLUDE_PROJECTS ^(smc-|gen-|iq9-b|iq9-l|iq9-o|simplify))
```

**Note for operators** — editing `run-audit.sh` while a run is in progress corrupts that run: bash reads a script
lazily by byte offset, so changing its length makes the running shell lose its place (`unexpected EOF`). The pack is
written but the rollup never runs. Found by doing exactly this during the validation pass.

## BUG-047 — CLI validation doc had no setup: a manual audit ran as the operator, not the auditor

| | |
|---|---|
| Found | 2026-09-17, user preparing a manual validation pass |
| Component | `docs/cis-ig1-cli-validation.md` — "Before you start" |
| Severity | **wrong result, undetectable** — an Owner passes checks the audit identity would fail |

**Cause** — the section gave two lines (`export ORG_ID`, `gcloud config set project`) and nothing else. It never said
to impersonate the audit service account, never exported `$PROJECT_ID`, and never mentioned Cloud Shell. Anyone
working the document by hand — which is exactly how the automation gets validated — ran every command under their own
credentials.

**Why it matters** — three separate ways to a clean, wrong audit:

- **Running as an Owner** passes checks the read-only auditor would fail, so the manual pass disagrees with the
  automation and the automation looks broken.
- **`$PROJECT_ID` unset** makes project-scope checks audit whatever `gcloud config` points at, without erroring — the
  same defect as BUG-002, which cost 52 checks the wrong answer.
- **Unsubstituted placeholders** (`APPROVED_REGISTRIES` and the other ten) produce commands that run and return
  nothing, which reads like a pass.

**Fix** — six numbered steps: open a shell (Cloud Shell or local, with the `alpha`/`beta` component check),
authenticate, set `ORG_ID`/`AUDIT_PROJECT`/`SA_EMAIL`/`PROJECT_ID`, impersonate, **prove impersonation took effect**
with the `service-accounts create` negative test, and substitute placeholders by hand. Plus `gcloud config unset
auth/impersonate_service_account` when finished, so a later command in the same shell does not silently run as the
auditor. Mirrors run sheet A1–A6 so the two documents cannot drift into disagreement.

**Verification** — parser unaffected: 86 org / 103 project checks, unchanged.

## BUG-048 — the impersonation proof was a write attempt, and proved nothing

| | |
|---|---|
| Found | 2026-09-17, user working the new setup steps |
| Component | `docs/cis-ig1-cli-validation.md` step 5, `docs/cis-ig1-run-sheet.md` A6, `gcloud/manual-steps.md` |
| Severity | **inconclusive test, and a write during a read-only audit** |

**Reported** — "running setup step 5 — prove it took effect — I run the CLI and get an error that I do not have
permissions to create, which is by design, this service account is totally read-only."

The error was the documented pass, which is the first problem: a step whose success looks like a failure stops the
operator every time.

**Cause** — the proof was `gcloud iam service-accounts create throwaway-check`, reading the denial as evidence that
impersonation had taken effect. Three defects:

1. **It does not prove what it claims.** An operator who lacks `iam.serviceAccounts.create` is denied whether or not
   impersonation is active. The denial is identical in shape; only the identity named in the message differs, and the
   instruction did not turn on reading that carefully.
2. **It writes.** If impersonation is *not* active and the operator holds Owner, it succeeds — creating a real service
   account in the customer's project, which the doc then asks them to delete. A read-only engagement should not open
   by creating a principal, and the attempt lands in the customer's Admin Activity log either way.
3. **Success-as-error** is a confusing instruction to put in front of someone who has not run the audit before.

**Fix** — ask Google who the token belongs to, which reads and names the identity outright:

```
curl -s "https://oauth2.googleapis.com/tokeninfo?access_token=$(gcloud auth print-access-token)" | jq -r .email
```

Returns the impersonated account when impersonation is active, the operator's own address when it is not. No
ambiguity, no write, nothing to clean up. Applied in all three documents, with a note in each saying why not to
substitute a write test.

**Verification** — run live against the test org with impersonation active:

```
cis-ig1-auditor@simplifymycloud-dev.iam.gserviceaccount.com
```

---

## BUG-049 — docs still described the eleven-prerequisite build and the pre-`9f62275` pass sizes

| | |
|---|---|
| Found | 2026-09-18, repo-wide review of setup instructions |
| Component | `docs/cis-ig1-audit-runbook.md`, `docs/cis-ig1-cli-validation.md`, `docs/cis-ig1-scripted-audit.md`, `docs/training/06-how-we-run-it.md`, `docs/cis-ig1-run-sheet.md`, `run-audit.sh --help` |
| Severity | **wrong instructions** — an auditor following them fills in values the tool no longer reads |

**Cause** — two check-set changes landed in code without every document following:

1. **Pass sizes.** `9f62275` moved V6/V113/V127 to org scope and later rewrites added checks; the passes are
   86 org / 103 project, but eleven places still said 71 / 87.
2. **Prerequisites.** `8dea1a5` cut `audit.env` from eleven values to one. The run sheet still told the auditor to
   fill in "the eleven prerequisite values" and confirm a backup bucket by name; three docs explained the `none` rule
   with a backup bucket, which no check reads any more; `run-audit.sh --help` listed `BACKUP_BUCKET`,
   `TFSTATE_BUCKET`, `BACKUP_PROJECT` as the config; training/06 showed `needs BACKUP_BUCKET` as a `-list` example.
   The run sheet's closing checklist also still said "A6 write attempt denied" — the proof BUG-048 replaced.

**Fix** — counts corrected everywhere. The `none` rule is still right, and `audit-run.go` still implements it
(`absent` → FAIL, "a resource that doesn't exist can't meet the requirement"); it now applies to the one value left,
so every doc explains it with `APPROVED_REGISTRIES`: no approved-registry list means safeguard 2.3 is not met, and
`none` says so where blank would SKIP both checks out of the report. The bucket-confirmation query is removed. The
run sheet's config step and checklist went in `c70b4e7`, where the setup steps were replaced with links to
`docs/cis-ig1-auditor-setup.md`.

**Verification** — `git grep` for `eleven`, `71 checks`, `87 checks`, `BACKUP_BUCKET`, `TFSTATE_BUCKET`,
`BACKUP_PROJECT` and `name:backup` outside `docs/testing/` and `docs/design/` returns only two unrelated uses of
"eleven" (overview, benchmark overlap); `bash -n run-audit.sh` clean and `--help` shows the new line.
