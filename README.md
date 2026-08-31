# CIS Controls v8.1 IG1 — GCP Audit Checklist

A documentation-only reference for auditing a Google Cloud Organization against CIS Controls v8.1 **Implementation Group 1**.

Built to be carried into an audit: tick the boxes, and when something fails, look up the fix.

**The documents are the product.** Three markdown files, no pipeline, no state. One single-file Go script is included to score the checklist — it reads the markdown and reports percentages. No module, no dependencies, nothing else to run.

## The three documents

| Document | When you use it |
|---|---|
| [`docs/cis-ig1-audit-runbook.md`](docs/cis-ig1-audit-runbook.md) | **Start here to run an audit.** Eleven phases, prerequisites through teardown. |
| [`docs/cis-ig1-scripted-audit.md`](docs/cis-ig1-scripted-audit.md) | **Just running the scripts.** Setup, impersonation, both passes, reading the output. |
| [`docs/cis-ig1-tracker.xlsx`](docs/cis-ig1-tracker.xlsx) | **Tracking the work.** All 290 requirements as a spreadsheet; imports into Google Sheets. |
| [`docs/cis-ig1-overview.md`](docs/cis-ig1-overview.md) | **Before the audit.** Why each of the 18 Controls exists, what it achieves, and what it touches in a GCP estate. Read once for context. |
| [`docs/cis-ig1-gcp-checklist.md`](docs/cis-ig1-gcp-checklist.md) | **During the audit.** 44 GCP-actionable safeguards, each with a Compliant / Not Compliant marker and per-requirement checkboxes. |
| [`docs/cis-ig1-cli-validation.md`](docs/cis-ig1-cli-validation.md) | **While auditing.** 188 read-only `gcloud` checks with pass criteria, linked from each checklist requirement by V-number. |
| [`docs/cis-ig1-remediation-reference.md`](docs/cis-ig1-remediation-reference.md) | **When something fails.** Look up the safeguard ID: a `gcloud` command to confirm the finding, and example Terraform to fix it. |
| [`docs/cis-ig1-benchmark-overlap.md`](docs/cis-ig1-benchmark-overlap.md) | **Only if the CIS GCP Foundation Benchmark is also in scope.** What benchmark-driven work contributes toward IG1 — 19 shared items, 62 of 290 requirements. Planning aid; recommendation numbers must be filled in from your copy of the benchmark. |

Start with **Step 0** in the checklist. One `gcloud` command tells you how much of the audit is already done.

## Running the audit

`audit-run.go` executes every CLI check and prints one report. Audit from that report rather than clicking through 188 checks one at a time.

```
go run audit-run.go -scope=org -org=123456789012 -pack ./audit-state/org

go run audit-run.go -scope=project -org=123456789012 \
  -project=my-project -pack ./audit-state/projects/my-project

go run audit-run.go -list                        # what would run
go run audit-run.go                              # run everything
go run audit-run.go -only V27,V30,V91            # a subset
go run audit-run.go -format=md -out findings.md  # for a ticket
```

`-org` may be omitted if `ORG_ID` is exported. `-scope` is required — the organization pass runs once, the project pass runs once per project.

The audit runs as a dedicated `cis-auditor` service account, impersonated rather than keyed — a service account key would violate safeguard 5.2, which the audit tests. Your own account is never granted the 22 audit roles, so the teardown can revoke everything unconditionally without stripping bindings you legitimately hold. See [Phase 1](docs/cis-ig1-audit-runbook.md#phase-1--prerequisites) and [Phase 11](docs/cis-ig1-audit-runbook.md#phase-11--tear-down-the-audit-access).

Audit results are committed to this repository, which is private. They name live findings — public buckets, over-permissioned accounts, open firewall rules — so repo access is access to the findings. Keep the access list matched to who should see them.

It **parses `docs/cis-ig1-cli-validation.md`** rather than embedding the commands, so the document stays the single source of truth — fix a command there and this picks it up.

Supply placeholders with `-set`, repeatable:

```
go run audit-run.go -set PROJECT_ID=prod-1 -set SECURITY_PROJECT=sec-1   -set DOMAIN=example.com -set LOG_BUCKET=org-audit-logs
```

### Verdicts

| | Meaning |
|---|---|
| `PASS` | Compliant |
| `FAIL` | A finding — output is shown |
| `REVIEW` | Ran clean, but the result needs your judgement |
| `SKIP` | No value supplied — should be zero on a properly prepared run |
| `N/A` | API or product absent — not a failure |
| `DENIED` | Missing permission; fix before trusting any result |
| `ERROR` | Command failed or timed out |
| `XREF` | Cross-reference whose target did not run — rare |

Results stream live as each check completes, so you see verdicts as they land rather than waiting for the report. `-quiet` suppresses it.

**Cross-references inherit.** Thirty checks have no command of their own — they say "See V57". Those adopt the verdict of the check they point at, taking the worst where they reference several. They only remain `XREF` if the target was excluded by `-only`.

**Checks discover their own resources.** Anything project-, instance-, cluster-, bucket-, key- or router-scoped is enumerated in full rather than sampled: a requirement is met only when **every** resource meets it, and the output names the ones that do not.

**Nothing should be SKIPped.** Only `BACKUP_BUCKET` and `TFSTATE_BUCKET` cannot be discovered — they depend on your naming. Gather them before the run:

```
go run audit-run.go -init-config ./audit-state/audit.env   # template of every value needed
# fill it in
go run audit-run.go -config ./audit-state/audit.env -pack ./audit-state/org
```

If a placeholder is still unresolved and the terminal is interactive, the runner asks for it up front rather than skipping mid-run.

**"Does not exist" is a finding, not a skip.** Set a value to `none` and the checks that need it are recorded as **FAIL**. That distinction matters: an organization with no log bucket has not "skipped" safeguard 8.3, it has failed it. Reporting that as SKIP quietly converts non-compliance into missing data.

Only **39 of 188** checks are auto-verdicted. A check qualifies only when its pass criteria opens by saying empty output means compliant; anything ambiguous becomes `REVIEW` with its output shown. That ratio is deliberate — a confident wrong verdict is worse in an audit than an honest "you decide".

`N/A` and `DENIED` are separated on purpose. During testing, "product absent" and "missing permission" looked identical, and chasing a nonexistent SCC finding on a customer call is time you do not get back.

Two things it handles that a shell script would not: `CLOUDSDK_CORE_DISABLE_PROMPTS=1` is set so a disabled API cannot hang the run on gcloud's interactive `y/N`, and identical commands are memoised so shared checks run once.

Exit code is non-zero if any check is `FAIL`, `DENIED`, or `ERROR`.

Both Go files carry `//go:build ignore`. They are standalone scripts, and without the tag two `package main` files in one directory collide — `go vet ./...` errors and editors report "main redeclared". The tag keeps them out of package builds while `go run <file>.go` works normally.

## The audit identity

`terraform/` provisions the read-only service account the audit runs as. [Run and teardown instructions](terraform/readme.md), including manual Console checks to confirm nothing is left behind.

```bash
cd terraform/audit-service-account
terraform init && terraform plan && terraform apply
eval "$(terraform output -raw impersonate_command)"
```

Read-only by construction — every permission is a get, list, or search, and where the only predefined role carried a write verb it is replaced by a custom role with an explicit permission list. `roles/storage.admin` in particular, which can delete buckets, is replaced by a three-permission reader.

No service account key is created. Impersonation only, because a key would breach safeguard 5.2.

`terraform destroy` removes every trace: the account, all organization bindings, the custom roles, the impersonation grants. That verifiable teardown is the main reason this is Terraform rather than a shell loop.

## Scoring

`compliance-report.go` reads the checklist markdown and reports how much of IG1 is satisfied. Single file, standard library only, no `go.mod` and no dependencies — Go 1.21 or later.

Run it from the repository root:

```
go run compliance-report.go
```

| Flag | Effect |
|---|---|
| *(none)* | Print the report to stdout |
| `--update` | Rewrite the `**Status:**` lines in the checklist to match the requirement boxes |
| `--format=md` | Emit markdown instead of plain text, for pasting into a ticket |
| `--file=PATH` | Score a different file (default `docs/cis-ig1-gcp-checklist.md`) |

Sample output:

```
SAFEGUARDS (44 in scope)

  Compliant            5   11.4%  ██░░░░░░░░░░░░░░░░░░░░░░
  PR submitted         3    6.8%  █░░░░░░░░░░░░░░░░░░░░░░░
  Not started         36   81.8%  ███████████████████░░░░░

----------------------------------------------------------------
ACCOUNTABILITY SPLIT

  Remediation written by SRE    69 / 290   23.8%
  Awaiting management approval  26            9.0%
  Outstanding with SRE         221          76.2%

----------------------------------------------------------------
AWAITING MANAGEMENT APPROVAL (3 safeguards)

  SAFEGUARD                               PRs                WAITING   STILL UNWRITTEN
  3.3 Configure Data Access Control L...  PR #101, PR #102 +9  65 days   -
  4.6 Securely Manage Enterprise Asse...  PR #101, PR #102 +11 65 days   -
  4.7 Manage Default Accounts on Ente...  PR #210              13 days   7

  Oldest fix waiting on approval: 65 days
```

The `**Status:**` lines are derived output, never input — the tool reads only the requirement checkboxes, so the document cannot contradict itself. It also warns if a requirement is ticked while still carrying a PR marker, since that combination silently inflates the numbers.

### Three states

Remediation here requires code that management approves and runs, so "not compliant" covers two very different situations. The checklist distinguishes them:

| Markup | State | Owner |
|---|---|---|
| `- [ ] item` | Not started | SRE |
| ``- [ ] item `PR #123` `` | PR submitted — fix written, awaiting approval | **Management** |
| `- [x] item` | Compliant — verified in the live estate | Done |

`[x]` means verified in the estate, not "merged". A merged PR that has not been applied and confirmed is not compliance.

The report separates *remediation written by SRE* from *awaiting management approval*, and ages every outstanding PR. That aging list is the point: a fix sitting unapproved for 60 days is a management decision, not an SRE backlog item, and the report says so in those terms.

Date PRs to enable aging:

```
- [ ] All currently public buckets remediated `PR #123 2026-07-02`
```

## Scope

IG1 is *enterprise-wide* essential cyber hygiene — it assumes employees, laptops, email, and vendors, not just cloud infrastructure. Of its 56 safeguards:

- **44** have GCP-actionable requirements and appear in the checklist
- **12** have no GCP surface at all (workforce training, end-user devices, removable media) and are listed in Appendix A of the checklist, with their owners

Both numbers are stated deliberately. A completed checklist evidences the GCP half of IG1, not IG1.

## Permissive-default vs secure-by-default organizations

Two organizations running identical workloads can start from very different positions depending on when the Organization resource was created.

- **Permissive-default** — created before Google applied constraints automatically. Nothing enforced at the org node: default networks with SSH open to the internet, default service accounts holding `roles/editor`.
- **Secure-by-default** — created recently enough that Google applies a set of security baseline constraints at creation, so several controls are enforced on day one.

Google changed the default sometime around 2024. The exact date matters less than the actual enforcement state on *your* organization:

```
gcloud resource-manager org-policies list --organization=ORGANIZATION_ID
```

Empty output indicates a permissive-default posture. A hardened permissive-default organization is in better shape than a secure-by-default one whose baseline was later deleted. The full baseline list and its safeguard mapping is in the overview.

## About the Terraform

The remediation reference contains example Terraform. It is illustrative, not a module — read it, adapt names and scopes to your estate, and apply it through your own review process.

### ⚠️ Organization Policy is not retroactive

The most misunderstood thing about GCP org policies, and the most common way an audit produces a false pass.

**A constraint blocks future non-conforming operations. It does nothing to what already exists.** Enforce `storage.publicAccessPrevention` and the console shows the policy as enforced while every already-public bucket stays public. Enforce `iam.disableServiceAccountKeyCreation` and every key already sitting in a CI system keeps working forever.

Every constraint-based fix is half a fix. The remediation reference opens with a table of affected safeguards and gives a `gcloud` discovery command for each, so you can find the existing violations rather than hiding them behind a green console. The checklist marks the clean-up items **↺ existing estate** — do not tick a safeguard Compliant on the constraint alone.

Run the discovery sweep **before** applying constraints. The violation list is your remediation backlog, and it tells you how big the job is.

**Then enforce carefully.** On a permissive-default estate, apply in dry-run, read the violations in Policy Analyzer, remediate, then promote to enforcement.

## Versioning

Written against **CIS Controls v8.1** (June 2024), the current release. Safeguard numbering differs from v8 and substantially from v7.1. Verify safeguard text against the official CIS publication before submitting evidence to an auditor.

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This is an implementation aid, not affiliated with or endorsed by CIS.*
