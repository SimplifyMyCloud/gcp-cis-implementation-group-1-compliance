#!/usr/bin/env bash
#
# Creates the read-only CIS IG1 audit service account.
#
# Equivalent to terraform/audit-service-account/. Use this where Terraform is
# unavailable; the role set is identical and kept in step with the module.
#
# Everything created is recorded in ./audit-sa-record.txt, which destroy.sh
# reads. That file is this script's equivalent of Terraform state — keep it
# until teardown is complete and verified.
#
#   ./create.sh --org-id 123456789012 \
#               --project my-audit-project \
#               --auditor user:alex@example.com
#
set -euo pipefail

ORG_ID=""
PROJECT=""
AUDITORS=()
SA_NAME="cis-ig1-auditor"
ROLE_PREFIX="cisIg1Audit"
ENABLE_SCC=false
ENABLE_BILLING=false
DRY_RUN=false
RECORD="./audit-sa-record.txt"

usage() {
  cat <<'USAGE'
Usage: ./create.sh --org-id ORG --project PROJECT --auditor PRINCIPAL [options]

Required
  --org-id ORG_ID           Numeric organization ID being audited
  --project PROJECT_ID      Project that will own the service account
  --auditor PRINCIPAL       Who may impersonate it. Repeatable.
                            Format: user:a@b.com | group:g@b.com

Options
  --sa-name NAME            Service account ID          (default cis-ig1-auditor)
  --role-prefix PREFIX      Custom role ID prefix       (default cisIg1Audit)
  --enable-scc              Also grant securitycenter.adminViewer
                            Fails where SCC is not licensed, so off by default
  --enable-billing          Also grant billing.viewer at the org node
                            Only useful if the billing account is in this org
  --record FILE             Where to write the teardown record
                            (default ./audit-sa-record.txt)
  --dry-run                 Print what would happen, change nothing
  -h, --help                This message
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --org-id)        ORG_ID="$2"; shift 2 ;;
    --project)       PROJECT="$2"; shift 2 ;;
    --auditor)       AUDITORS+=("$2"); shift 2 ;;
    --sa-name)       SA_NAME="$2"; shift 2 ;;
    --role-prefix)   ROLE_PREFIX="$2"; shift 2 ;;
    --enable-scc)    ENABLE_SCC=true; shift ;;
    --enable-billing) ENABLE_BILLING=true; shift ;;
    --record)        RECORD="$2"; shift 2 ;;
    --dry-run)       DRY_RUN=true; shift ;;
    -h|--help)       usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 2 ;;
  esac
done

[[ -n "$ORG_ID"  ]] || { echo "ERROR: --org-id is required"  >&2; usage; exit 2; }
[[ -n "$PROJECT" ]] || { echo "ERROR: --project is required" >&2; usage; exit 2; }
[[ ${#AUDITORS[@]} -gt 0 ]] || { echo "ERROR: at least one --auditor is required" >&2; usage; exit 2; }

[[ "$ORG_ID" =~ ^[0-9]+$ ]] || { echo "ERROR: --org-id must be numeric" >&2; exit 2; }
for a in "${AUDITORS[@]}"; do
  [[ "$a" =~ ^(user|group|serviceAccount): ]] || {
    echo "ERROR: auditor '$a' must start with user:, group: or serviceAccount:" >&2; exit 2; }
done

SA_EMAIL="${SA_NAME}@${PROJECT}.iam.gserviceaccount.com"
SA_MEMBER="serviceAccount:${SA_EMAIL}"

# ---------------------------------------------------------------------------
# Read-only roles. Identical to terraform/audit-service-account/main.tf.
# Every one is a get, list or search — no write, update, delete or actuation.
# ---------------------------------------------------------------------------
ROLES=(
  # Resource hierarchy and posture
  roles/browser
  roles/orgpolicy.policyViewer
  roles/cloudasset.viewer
  roles/serviceusage.serviceUsageViewer

  # Identity
  roles/iam.securityReviewer
  roles/iam.serviceAccountViewer
  roles/recommender.iamViewer
  roles/privilegedaccessmanager.viewer
  roles/policyanalyzer.activityAnalysisViewer

  # Logging and monitoring
  roles/logging.viewer
  roles/logging.privateLogViewer     # Data Access log ENTRIES — separate grant
  roles/monitoring.viewer

  # Compute and network
  roles/compute.viewer
  roles/osconfig.inventoryViewer
  roles/container.viewer
  roles/gkebackup.viewer
  roles/dns.reader

  # Data services
  roles/cloudsql.viewer
  roles/datastore.viewer
  roles/spanner.viewer
  roles/bigquery.metadataViewer

  # Application and supply chain
  roles/run.viewer
  roles/cloudfunctions.viewer
  roles/artifactregistry.reader
  roles/binaryauthorization.policyViewer
  roles/cloudsecurityscanner.viewer

  # Secrets and keys — metadata only. secretmanager.viewer excludes
  # versions.access, so secret VALUES cannot be read. cloudkms.viewer
  # excludes encrypt and decrypt.
  roles/secretmanager.viewer
  roles/cloudkms.viewer

  # Governance
  roles/essentialcontacts.viewer
  roles/accesscontextmanager.policyReader
)
$ENABLE_SCC     && ROLES+=(roles/securitycenter.adminViewer)
$ENABLE_BILLING && ROLES+=(roles/billing.viewer)

# ---------------------------------------------------------------------------
# Custom roles. Each replaces a predefined role that carries a write verb.
# ---------------------------------------------------------------------------
CUSTOM_IDS=("${ROLE_PREFIX}StorageReader" "${ROLE_PREFIX}KeyReader" "${ROLE_PREFIX}IapReader")
CUSTOM_TITLES=("CIS IG1 Audit — Storage Reader" "CIS IG1 Audit — Service Account Key Reader" "CIS IG1 Audit — IAP Settings Reader")
# storage.admin can DELETE every bucket and object. This has no object permission at all.
# iam.serviceAccountKeyAdmin can CREATE and DELETE keys — granting it would breach safeguard 5.2.
# iap.settingsAdmin can MODIFY settings.
CUSTOM_PERMS=(
  "storage.buckets.get,storage.buckets.getIamPolicy,storage.buckets.list"
  "iam.serviceAccountKeys.list"
  "iap.web.getSettings"
)

run() {
  if $DRY_RUN; then printf '  [dry-run] %s\n' "$*"; else "$@"; fi
}

echo
echo "CIS IG1 audit service account"
echo "  organization : $ORG_ID"
echo "  project      : $PROJECT"
echo "  account      : $SA_EMAIL"
echo "  auditors     : ${AUDITORS[*]}"
echo "  roles        : ${#ROLES[@]} predefined + ${#CUSTOM_IDS[@]} custom, all read-only"
$DRY_RUN && echo "  MODE         : dry run, nothing will change"
echo

if ! $DRY_RUN; then
  read -r -p "Proceed? [y/N] " ans
  [[ "$ans" =~ ^[Yy]$ ]] || { echo "aborted"; exit 1; }
fi

# ---------------------------------------------------------------------------
# Record file. Without Terraform state this is the only reliable teardown
# input, so it is written before anything is granted, not after.
# ---------------------------------------------------------------------------
if ! $DRY_RUN; then
  {
    echo "# CIS IG1 audit service account — created $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    echo "# Read by destroy.sh. Keep until teardown is verified."
    echo "ORG_ID=$ORG_ID"
    echo "PROJECT=$PROJECT"
    echo "SA_EMAIL=$SA_EMAIL"
    echo "ROLE_PREFIX=$ROLE_PREFIX"
    for a in "${AUDITORS[@]}"; do echo "AUDITOR=$a"; done
    for r in "${ROLES[@]}";    do echo "ROLE=$r";    done
    for c in "${CUSTOM_IDS[@]}"; do echo "CUSTOM_ROLE=$c"; done
  } > "$RECORD"
  echo "→ record written to $RECORD"
  echo
fi

# ---------------------------------------------------------------------------
echo "1/4  service account"
if gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT" >/dev/null 2>&1; then
  echo "  exists already — reusing"
else
  run gcloud iam service-accounts create "$SA_NAME" \
    --project="$PROJECT" \
    --display-name="CIS IG1 auditor (read-only)" \
    --description="Read-only CIS IG1 audit of organization ${ORG_ID}. Impersonated, never keyed. Delete at teardown."
  $DRY_RUN || echo "  created $SA_EMAIL"
fi

# ---------------------------------------------------------------------------
echo
echo "2/4  custom roles"
for i in "${!CUSTOM_IDS[@]}"; do
  id="${CUSTOM_IDS[$i]}"
  if gcloud iam roles describe "$id" --organization="$ORG_ID" >/dev/null 2>&1; then
    echo "  exists already — $id"
  elif $DRY_RUN; then
    echo "  [dry-run] would undelete $id if soft-deleted, otherwise create it"
  else
    # A role deleted in the last 7 days is soft-deleted and can be undeleted.
    # After that it can't — yet its ID stays reserved until Google purges it
    # (up to ~37 days), so create fails too. describe returns NOT_FOUND for
    # both, which is why this tries undelete, then create, then explains.
    if gcloud iam roles undelete "$id" --organization="$ORG_ID" >/dev/null 2>&1; then
      echo "  undeleted    — $id (was soft-deleted from a previous audit)"
    elif err=$(gcloud iam roles create "$id" \
        --organization="$ORG_ID" \
        --title="${CUSTOM_TITLES[$i]}" \
        --description="Read-only. Created for a CIS IG1 audit; removed at teardown." \
        --permissions="${CUSTOM_PERMS[$i]}" \
        --stage=GA 2>&1 >/dev/null); then
      echo "  created      — $id"
    else
      echo "  FAILED       — $id" >&2
      echo "$err" | sed 's/^/                 /' >&2
      if [[ "$err" == *"marked for deletion"* ]]; then
        echo "  This role ID is still reserved from an earlier teardown and can no longer be" >&2
        echo "  undeleted. Re-run with a new prefix, e.g. --role-prefix ${ROLE_PREFIX}2" >&2
      fi
      exit 1
    fi
  fi
done

# ---------------------------------------------------------------------------
echo
echo "3/4  organization role bindings (${#ROLES[@]} predefined + ${#CUSTOM_IDS[@]} custom)"
FAILED=()
bind() {
  local role="$1"
  # add-iam-policy-binding is additive and idempotent — it adds one member to
  # one role and leaves every other binding in the organization untouched.
  if $DRY_RUN; then
    echo "  [dry-run] would grant $role"
  elif gcloud organizations add-iam-policy-binding "$ORG_ID" \
       --member="$SA_MEMBER" --role="$role" \
       --condition=None --quiet >/dev/null 2>&1; then
    echo "  ok      $role"
  else
    echo "  FAILED  $role"
    FAILED+=("$role")
  fi
}
for r in "${ROLES[@]}"; do bind "$r"; done
for c in "${CUSTOM_IDS[@]}"; do bind "organizations/${ORG_ID}/roles/${c}"; done

# ---------------------------------------------------------------------------
echo
echo "4/4  impersonation"
for a in "${AUDITORS[@]}"; do
  if $DRY_RUN; then
    echo "  [dry-run] would let $a impersonate $SA_EMAIL"
  elif gcloud iam service-accounts add-iam-policy-binding "$SA_EMAIL" \
       --project="$PROJECT" \
       --member="$a" \
       --role="roles/iam.serviceAccountTokenCreator" \
       --quiet >/dev/null 2>&1; then
    echo "  ok      $a"
  else
    echo "  FAILED  $a"
    FAILED+=("tokenCreator:$a")
  fi
done

# ---------------------------------------------------------------------------
echo
if [[ ${#FAILED[@]} -gt 0 ]]; then
  echo "⚠️  ${#FAILED[@]} grant(s) failed:"
  printf '     %s\n' "${FAILED[@]}"
  echo
  echo "   securitycenter.adminViewer fails where SCC is not licensed — expected."
  echo "   Anything else means the account running this lacks organization admin."
  echo
fi

cat <<EOF
Done.

  No service account key was created. A key would breach CIS safeguard 5.2,
  which this audit tests. Authentication is by impersonation only.

Next:

  gcloud config set auth/impersonate_service_account $SA_EMAIL
  gcloud config get-value auth/impersonate_service_account

  That must print the service account. 'gcloud auth list' will still show your
  own address — impersonation layers a short-lived token over your credential
  rather than switching accounts, which is why audit logs record both.

Verify:   ./verify.sh --record $RECORD
Teardown: ./destroy.sh --record $RECORD
EOF
