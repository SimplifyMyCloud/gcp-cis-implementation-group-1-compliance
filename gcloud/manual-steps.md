# Manual gcloud steps

Every command, in order, with nothing wrapped in a script. Paste one block at a time.

Same result as `create.sh` and `destroy.sh` in this directory, and as the Terraform module. Use this when you want to see exactly what happens, or when running the scripts is not an option.

---

## 1. Set your values

Everything below reads these, so this is the only place you type them.

```bash
export ORG_ID="123456789012"                    # gcloud organizations list
export PROJECT="my-audit-project"               # will own the service account
export AUDITOR="user:alex@example.com"          # who may impersonate it

export SA_NAME="cis-ig1-auditor"
export SA_EMAIL="${SA_NAME}@${PROJECT}.iam.gserviceaccount.com"
export SA_MEMBER="serviceAccount:${SA_EMAIL}"
export PREFIX="cisIg1Audit"

echo "org=$ORG_ID  sa=$SA_EMAIL  auditor=$AUDITOR"
```

---

## 2. Create the service account

```bash
gcloud iam service-accounts create "$SA_NAME" \
  --project="$PROJECT" \
  --display-name="CIS IG1 auditor (read-only)" \
  --description="Read-only CIS IG1 audit of organization ${ORG_ID}. Impersonated, never keyed. Delete at teardown."
```

**Do not create a key.** A service account key is a long-lived credential in a file, and creating one would breach CIS safeguard 5.2 — a safeguard this audit tests. Authentication is by impersonation, set up in step 5.

---

## 3. Create the three custom roles

Each replaces a predefined role that carries a write verb.

**Storage** — `roles/storage.admin` would allow **deleting** every bucket and object. This has no object permission at all.

```bash
gcloud iam roles create "${PREFIX}StorageReader" \
  --organization="$ORG_ID" \
  --title="CIS IG1 Audit — Storage Reader" \
  --description="Read-only. Created for a CIS IG1 audit; removed at teardown." \
  --permissions="storage.buckets.get,storage.buckets.getIamPolicy,storage.buckets.list" \
  --stage=GA
```

**Service account keys** — `roles/iam.serviceAccountKeyAdmin` would allow **creating and deleting** keys.

```bash
gcloud iam roles create "${PREFIX}KeyReader" \
  --organization="$ORG_ID" \
  --title="CIS IG1 Audit — Service Account Key Reader" \
  --description="Read-only. Created for a CIS IG1 audit; removed at teardown." \
  --permissions="iam.serviceAccountKeys.list" \
  --stage=GA
```

**IAP** — `roles/iap.settingsAdmin` would allow **modifying** settings.

```bash
gcloud iam roles create "${PREFIX}IapReader" \
  --organization="$ORG_ID" \
  --title="CIS IG1 Audit — IAP Settings Reader" \
  --description="Read-only. Created for a CIS IG1 audit; removed at teardown." \
  --permissions="iap.web.getSettings" \
  --stage=GA
```

> If a role from a previous audit was deleted **within the last 7 days**, `create` fails — use `gcloud iam roles undelete ROLE_ID --organization=$ORG_ID` instead. **After 7 days** it can't be undeleted, but the ID stays reserved for up to ~37 days (`describe` returns NOT_FOUND, `create` fails with "marked for deletion"): change `PREFIX` in step 1.

---

## 4. Grant the read-only roles

Thirty predefined roles, every one a get, list or search. The loop is one paste; each iteration is a plain `add-iam-policy-binding`.

`add-iam-policy-binding` is **additive** — it adds one member to one role and leaves every other binding in the organization untouched.

```bash
for ROLE in \
  roles/browser \
  roles/orgpolicy.policyViewer \
  roles/cloudasset.viewer \
  roles/serviceusage.serviceUsageViewer \
  roles/iam.securityReviewer \
  roles/iam.serviceAccountViewer \
  roles/recommender.iamViewer \
  roles/privilegedaccessmanager.viewer \
  roles/policyanalyzer.activityAnalysisViewer \
  roles/logging.viewer \
  roles/logging.privateLogViewer \
  roles/monitoring.viewer \
  roles/compute.viewer \
  roles/osconfig.inventoryViewer \
  roles/container.viewer \
  roles/gkebackup.viewer \
  roles/dns.reader \
  roles/cloudsql.viewer \
  roles/datastore.viewer \
  roles/spanner.viewer \
  roles/bigquery.metadataViewer \
  roles/run.viewer \
  roles/cloudfunctions.viewer \
  roles/artifactregistry.reader \
  roles/binaryauthorization.policyViewer \
  roles/cloudsecurityscanner.viewer \
  roles/secretmanager.viewer \
  roles/cloudkms.viewer \
  roles/essentialcontacts.viewer \
  roles/accesscontextmanager.policyReader
do
  if gcloud organizations add-iam-policy-binding "$ORG_ID" \
       --member="$SA_MEMBER" --role="$ROLE" \
       --condition=None --quiet >/dev/null 2>&1; then
    echo "  ok      $ROLE"
  else
    echo "  FAILED  $ROLE"
  fi
done
```

Then the three custom roles:

```bash
for ROLE in "${PREFIX}StorageReader" "${PREFIX}KeyReader" "${PREFIX}IapReader"; do
  gcloud organizations add-iam-policy-binding "$ORG_ID" \
    --member="$SA_MEMBER" \
    --role="organizations/${ORG_ID}/roles/${ROLE}" \
    --condition=None --quiet >/dev/null && echo "  ok      $ROLE"
done
```

### Two optional roles

**Security Command Center** — only where SCC is licensed. The binding fails otherwise.

```bash
gcloud organizations add-iam-policy-binding "$ORG_ID" \
  --member="$SA_MEMBER" --role="roles/securitycenter.adminViewer" \
  --condition=None --quiet
```

**Billing** — optional. No check needs it: V7 reads each project's own billing info. Grant it only if you want billing-account visibility, and only if the billing account lives inside this organization.

```bash
gcloud organizations add-iam-policy-binding "$ORG_ID" \
  --member="$SA_MEMBER" --role="roles/billing.viewer" \
  --condition=None --quiet
```

If the billing account sits outside the organization, grant it on the account instead:

```bash
gcloud billing accounts add-iam-policy-binding BILLING_ACCOUNT_ID \
  --member="$SA_MEMBER" --role="roles/billing.viewer"
```

That binding is easy to forget at teardown — write it down.

---

## 5. Allow yourself to impersonate it

This is the **only** permission a human gets, and it is on the service account, not on the organization.

```bash
gcloud iam service-accounts add-iam-policy-binding "$SA_EMAIL" \
  --project="$PROJECT" \
  --member="$AUDITOR" \
  --role="roles/iam.serviceAccountTokenCreator"
```

Repeat with a different `--member` for each additional auditor.

---

## 6. Verify before using it

**Bindings — expect 33, or 34–35 with the optional roles:**

```bash
gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten="bindings[].members" \
  --filter="bindings.members:${SA_EMAIL}" \
  --format="value(bindings.role)" | sort
```

**No keys — this must return nothing:**

```bash
gcloud iam service-accounts keys list \
  --iam-account="$SA_EMAIL" --managed-by=user --format="value(name)"
```

**Custom role permissions:**

```bash
for ROLE in "${PREFIX}StorageReader" "${PREFIX}KeyReader" "${PREFIX}IapReader"; do
  echo "== $ROLE"
  gcloud iam roles describe "$ROLE" --organization="$ORG_ID" \
    --format="value(includedPermissions)"
done
```

**Who may impersonate:**

```bash
gcloud iam service-accounts get-iam-policy "$SA_EMAIL" --project="$PROJECT" \
  --flatten="bindings[].members" \
  --filter="bindings.role:roles/iam.serviceAccountTokenCreator" \
  --format="value(bindings.members)"
```

---

## 7. Use it

```bash
gcloud config set auth/impersonate_service_account "$SA_EMAIL"
gcloud config get-value auth/impersonate_service_account
```

That must print the service account.

> **`gcloud auth list` will still show your own address, and that is correct.** Impersonation does not switch accounts — you stay authenticated as yourself, and gcloud exchanges that credential for a short-lived token on every call. That is exactly why audit logs record both identities: the service account in `principalEmail`, and you in `serviceAccountDelegationInfo`.

Prove the identity is in force. This reads and changes nothing:

```bash
curl -s "https://oauth2.googleapis.com/tokeninfo?access_token=$(gcloud auth print-access-token)" | jq -r .email
```

Must print `cis-ig1-auditor@…`. Your own address means impersonation is not active — check step 7.

Do not test this with a write instead. A denied write proves nothing on its own, since an operator lacking the permission is denied whether or not impersonation is active, and a successful one leaves a real service account behind in the project.

---

## 8. Delete everything

### Stop impersonating first

```bash
gcloud config unset auth/impersonate_service_account
gcloud config get-value account
```

**This must come first.** The audit identity cannot revoke its own bindings, so every command below fails while impersonation is active.

### Revoke the roles

```bash
for ROLE in \
  roles/browser roles/orgpolicy.policyViewer roles/cloudasset.viewer \
  roles/serviceusage.serviceUsageViewer roles/iam.securityReviewer \
  roles/iam.serviceAccountViewer roles/recommender.iamViewer \
  roles/privilegedaccessmanager.viewer roles/policyanalyzer.activityAnalysisViewer \
  roles/logging.viewer roles/logging.privateLogViewer roles/monitoring.viewer \
  roles/compute.viewer roles/osconfig.inventoryViewer roles/container.viewer \
  roles/gkebackup.viewer roles/dns.reader roles/cloudsql.viewer \
  roles/datastore.viewer roles/spanner.viewer roles/bigquery.metadataViewer \
  roles/run.viewer roles/cloudfunctions.viewer roles/artifactregistry.reader \
  roles/binaryauthorization.policyViewer roles/cloudsecurityscanner.viewer \
  roles/secretmanager.viewer roles/cloudkms.viewer \
  roles/essentialcontacts.viewer roles/accesscontextmanager.policyReader \
  roles/securitycenter.adminViewer roles/billing.viewer
do
  gcloud organizations remove-iam-policy-binding "$ORG_ID" \
    --member="$SA_MEMBER" --role="$ROLE" \
    --condition=None --quiet >/dev/null 2>&1 \
    && echo "  revoked  $ROLE" || echo "  skipped  $ROLE"
done
```

The two optional roles are included — "skipped" just means they were never granted.

### Revoke and delete the custom roles

```bash
for ROLE in "${PREFIX}StorageReader" "${PREFIX}KeyReader" "${PREFIX}IapReader"; do
  gcloud organizations remove-iam-policy-binding "$ORG_ID" \
    --member="$SA_MEMBER" \
    --role="organizations/${ORG_ID}/roles/${ROLE}" \
    --condition=None --quiet >/dev/null 2>&1
  gcloud iam roles delete "$ROLE" --organization="$ORG_ID" --quiet >/dev/null \
    && echo "  deleted  $ROLE"
done
```

Deleted custom roles can be undeleted for 7 days, and their IDs stay reserved for up to ~37. That is normal — they purge themselves. A new audit inside that window needs a different `PREFIX`.

### Delete the service account

```bash
gcloud iam service-accounts delete "$SA_EMAIL" --project="$PROJECT" --quiet
```

This also removes the impersonation grants on it.

---

## 9. Verify nothing remains

**No bindings — must be empty:**

```bash
gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten="bindings[].members" \
  --filter="bindings.members:${SA_EMAIL}" \
  --format="value(bindings.role)"
```

**No custom roles — must be empty, or show them as DELETED:**

```bash
gcloud iam roles list --organization="$ORG_ID" --filter="name~${PREFIX}"
```

**Account gone — must return NOT_FOUND:**

```bash
gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT"
```

**And one thing that should still be there** — the record of what the auditor did:

```bash
gcloud logging read \
  'protoPayload.methodName="GenerateAccessToken"' \
  --organization="$ORG_ID" --freshness=7d \
  --format="table(timestamp, protoPayload.authenticationInfo.principalEmail)"
```

The evidence outlives the auditor's ability to act. That is the point.

---

## Do not skip the teardown

The audit identity holds organization-wide read. Left in place it fails safeguards **5.1** (account inventory), **5.4** (restrict administrator privileges) and **6.2** (access revoking process) — three of the controls this audit exists to measure.
