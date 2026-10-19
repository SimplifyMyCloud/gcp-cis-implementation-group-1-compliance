# gcloud

The audit identity without Terraform, for environments where it is unavailable.

| | |
|---|---|
| `create.sh` | Service account, three custom roles, org bindings. Writes `audit-sa-record.txt` |
| `verify.sh` | Confirms the identity is as intended — no keys, no write verbs, expected bindings |
| `destroy.sh` | Removes everything `create.sh` made, from its record |
| [`manual-steps.md`](manual-steps.md) | The same, command by command, for a change-controlled environment |

## Create

```bash
./create.sh ORG_ID HOST_PROJECT_ID AUDITOR_EMAIL
```

```bash
./verify.sh
```

`audit-sa-record.txt` is the teardown record. Keep it; `destroy.sh` reads it.

## Destroy

```bash
./destroy.sh
```

```bash
./verify.sh
```

Expect it to report nothing remaining.

## Notes

bash 3.2 compatible — the macOS default. Requires organization IAM admin.

Custom role IDs stay reserved for roughly 37 days after deletion and can only be undeleted in the first 7. Re-create in between with a new prefix.

Prefer [`terraform/audit-service-account/`](../terraform/audit-service-account/readme.md) where Terraform is available: the state is a better teardown record than a text file.
