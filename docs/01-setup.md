# Phase 0 — Setup

Cloud Shell. Run once per engagement, then once per session where marked.

---

## Prerequisites

| | |
|---|---|
| Audit service account exists, with org bindings | [`terraform/audit-service-account/`](../terraform/audit-service-account/readme.md) |
| 17 APIs enabled in the quota project | [`cis-ig1-required-apis.md`](cis-ig1-required-apis.md) |
| You hold `roles/iam.serviceAccountTokenCreator` on the SA | — |
| Repository cloned, engagement branch exists | — |

---

## 1. Session — every time

```bash
gcloud auth login
```

```bash
cd "$AUDIT_REPO" && git switch "$AUDIT_BRANCH" && git pull
```

---

## 2. Settings — once per engagement

```bash
cp config/audit.env.example config/audit.env
```

```bash
cloudshell edit config/audit.env
```

| Key | Value |
|---|---|
| `ORG_ID` | Numeric org ID. `gcloud organizations list` |
| `AUDIT_PROJECT` | **The project with the 17 APIs enabled.** Not necessarily the SA's own project |
| `SA_EMAIL` | The audit service account |
| `AUDIT_BRANCH` | `<customer>-gcp-ig1-audit` |
| `APPROVED_REGISTRIES` | Registry prefixes, comma-separated. No list → `none`, never blank |
| `EXCLUDE_PROJECTS` | Leave `^sys-` |
| `ALLOWED_LOCATIONS` | Leave commented. Defaults to continental US. Setting it **replaces** the default |

```bash
gcloud projects list --format='value(projectId)' | sort > config/projects.txt
```

Strip out-of-scope projects. This is the coverage denominator.

```bash
git add config/audit.env config/projects.txt && git commit -m "Engagement settings" && git push
```

---

## 3. Impersonate — every session

```bash
source config/audit.env
```

```bash
gcloud config set project "$AUDIT_PROJECT"
```

```bash
gcloud config set auth/impersonate_service_account "$SA_EMAIL"
```

---

## 4. Verify — every session, before auditing

```bash
curl -s "https://oauth2.googleapis.com/tokeninfo?access_token=$(gcloud auth print-access-token)" | jq -r .email
```

| Output | Action |
|---|---|
| `$SA_EMAIL` | Proceed |
| Your own address | Repeat step 3 |
| `Failed to impersonate` | You lack `serviceAccountTokenCreator` on the SA. Stop |

```bash
gcloud config list
```

Confirm all three: `account` is you, `project` is `$AUDIT_PROJECT`, `impersonate_service_account` is `$SA_EMAIL`.

```bash
gcloud asset search-all-iam-policies --scope=organizations/$ORG_ID --limit=1
```

| Result | Meaning |
|---|---|
| A policy row | Quota project and bindings are correct. Go to Phase 1 |
| `API [cloudasset...] not enabled` | `AUDIT_PROJECT` is wrong, or the API is not enabled there |
| `PERMISSION_DENIED` | The SA has no org bindings, or `ORG_ID` is wrong |
| `does not have serviceusage.services.use` | Grant the SA `roles/serviceusage.serviceUsageConsumer` on `AUDIT_PROJECT` |

Do not proceed on an error. An audit against a broken identity produces DENIED for everything.

---

## 5. Smoke test — first session of an engagement

```bash
go run audit-run.go -scope=org -only V27,V43,V86,V125,V181 -no-prompt
```

Expect PASS, FAIL or N/A. **Any DENIED or ERROR: stop and fix.** Do not start Phase 1.

---

## Teardown — end of engagement

```bash
gcloud config unset auth/impersonate_service_account
```

Destroy the service account and its org bindings. A standing org-wide read identity fails safeguards 5.1, 5.4 and 6.2.
