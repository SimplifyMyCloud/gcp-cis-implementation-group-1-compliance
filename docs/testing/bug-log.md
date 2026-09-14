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
