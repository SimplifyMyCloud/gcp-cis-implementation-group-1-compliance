# Audit service account — gcloud

Creates the read-only CIS IG1 audit service account without Terraform.

Identical to [`../terraform/audit-service-account/`](../terraform/audit-service-account/readme.md) — same 35 roles, same three custom roles, same impersonation-only authentication. Use this where Terraform is unavailable.

| | |
|---|---|
| [`manual-steps.md`](manual-steps.md) | Every command as copy-paste, no scripts — create and delete |
| `create.sh` | Creates the account, roles and bindings |
| `verify.sh` | Confirms what exists — run after create *and* after destroy |
| `destroy.sh` | Removes everything |

## Create

```bash
./create.sh \
  --org-id 123456789012 \
  --project my-audit-project \
  --auditor user:alex@example.com
```

Add `--dry-run` first to see exactly what it will do without changing anything.

| Option | |
|---|---|
| `--auditor` | Repeatable — one per person or group who may impersonate |
| `--enable-scc` | Adds `securitycenter.adminViewer`. Off by default; the binding fails where SCC is not licensed |
| `--enable-billing` | Adds `billing.viewer` at the org node. Optional — no check needs it |
| `--sa-name` / `--role-prefix` | Change the account ID or custom role prefix — needed when re-auditing within ~37 days of a teardown |
| `--dry-run` | Change nothing |

### It writes a record — keep it

`create.sh` writes `audit-sa-record.txt` listing every role granted, the account, and the organization.

**Without Terraform state, that file is the only reliable teardown input.** `destroy.sh` reads it. Lose it and you are reconstructing 35 bindings by hand.

Keep it until teardown is complete and verified. It contains no credentials.

## Use

```bash
gcloud config set auth/impersonate_service_account \
  cis-ig1-auditor@my-audit-project.iam.gserviceaccount.com

gcloud config get-value auth/impersonate_service_account
```

That must print the service account. That confirms the service account works. To set up a shell for the audit itself — branch, token check, output directory, config, and the reusable Cloud Shell setup — follow [auditor setup](../docs/cis-ig1-auditor-setup.md).

> **`gcloud auth list` will still show your own address, and that is correct.** Impersonation does not switch accounts — you stay authenticated as yourself and gcloud exchanges that credential for a short-lived service account token on each call. That is exactly why audit logs record both identities.

**No key is created.** A service account key would breach CIS safeguard 5.2, which this audit tests, and would put a long-lived credential in a file. Impersonation issues a token that expires within the hour.

## Verify

```bash
./verify.sh
```

Six checks: the account exists, **no keys exist**, binding count matches the record, no role carries a write verb, the custom roles hold only their stated permissions, and who may impersonate.

Run it again after teardown — it reports the inverse, confirming nothing remains.

## Destroy

```bash
gcloud config unset auth/impersonate_service_account   # FIRST
./destroy.sh
```

**Unset impersonation first.** The audit identity has no permission to revoke its own bindings, so every command fails otherwise. `destroy.sh` detects this and offers to unset it for you.

Then disable only the APIs the audit enabled — from the snapshot taken before it began, never the full list, since some were already on and in use.

### Do not skip it

The audit identity holds organization-wide read. Left in place it fails safeguards **5.1** (account inventory), **5.4** (restrict administrator privileges) and **6.2** (access revoking) — three of the controls this audit measures.

## What is granted

35 roles, every one read-only. Full table with what each permits and which IG1 controls it serves: [`../terraform/audit-service-account/readme.md`](../terraform/audit-service-account/readme.md#exactly-what-is-granted).

Three are custom roles, each replacing a predefined role that carries a write verb:

| Custom role | Replaces | Because the predefined role can |
|---|---|---|
| `StorageReader` | `roles/storage.admin` | **delete** every bucket and object |
| `KeyReader` | `roles/iam.serviceAccountKeyAdmin` | **create and delete** service account keys |
| `IapReader` | `roles/iap.settingsAdmin` | **modify** IAP settings |

`StorageReader` has no object permission at all — it cannot read file contents. `secretmanager.viewer` excludes `versions.access`, so secret values cannot be read. `cloudkms.viewer` excludes encrypt and decrypt.

## Known edges

**Custom role IDs outlive the roles.** Within 7 days of a teardown, `create.sh` undeletes them. After that they can't be undeleted, but the IDs stay reserved for up to ~37 days — `create.sh` stops with "marked for deletion" and tells you to re-run with `--role-prefix`.

**New grants take a minute or two.** Straight after `create.sh`, impersonation can fail with `Failed to impersonate`. Wait and retry.

**`securitycenter.adminViewer` fails without an SCC licence.** Expected — leave `--enable-scc` off.

**Re-running `create.sh` is safe.** It reuses an existing account, undeletes recently deleted roles, and IAM bindings are idempotent. `--dry-run` changes nothing, not even an undelete.

**Plain macOS bash works.** The scripts run on bash 3.2, which is what macOS ships.

## Keeping this in step with the Terraform

The role list in `create.sh` is copied from `terraform/audit-service-account/main.tf`. If one changes, change both. To compare:

```bash
grep -oE '"roles/[a-zA-Z.]+"' ../terraform/audit-service-account/main.tf \
  | tr -d '"' | sort > /tmp/tf-roles.txt
grep -oE '^  roles/[a-zA-Z.]+' create.sh | tr -d ' ' | sort > /tmp/sh-roles.txt
diff /tmp/tf-roles.txt /tmp/sh-roles.txt
```

Expect exactly three differences, all correct:

- `roles/iam.serviceAccountTokenCreator` — the impersonation grant on the service account, not an organization binding
- `roles/securitycenter.adminViewer` — added conditionally by `--enable-scc`
- `roles/billing.viewer` — added conditionally by `--enable-billing`

Anything else means the two have drifted.
