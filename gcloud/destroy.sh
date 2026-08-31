#!/usr/bin/env bash
#
# Removes everything create.sh made, using the record file it wrote.
#
# Without Terraform state, that record IS the teardown input. If it is missing
# you are reconstructing 35 bindings by hand, so --record is required rather
# than guessed.
#
#   ./destroy.sh --record ./audit-sa-record.txt
#
set -euo pipefail

RECORD="./audit-sa-record.txt"
DRY_RUN=false
KEEP_SA=false

usage() {
  cat <<'USAGE'
Usage: ./destroy.sh [--record FILE] [--dry-run] [--keep-service-account]

  --record FILE            Record written by create.sh (default ./audit-sa-record.txt)
  --dry-run                Print what would happen, change nothing
  --keep-service-account   Revoke every role but leave the account in place
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --record) RECORD="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    --keep-service-account) KEEP_SA=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 2 ;;
  esac
done

[[ -f "$RECORD" ]] || {
  cat >&2 <<EOF
ERROR: record file not found: $RECORD

  Without it there is no reliable list of what to revoke. Find the file
  create.sh wrote, or reconstruct the bindings by hand with:

    gcloud organizations get-iam-policy ORG_ID \\
      --flatten="bindings[].members" \\
      --filter="bindings.members~cis-ig1-auditor" \\
      --format="value(bindings.role)"
EOF
  exit 2
}

ORG_ID=$(grep '^ORG_ID=' "$RECORD" | cut -d= -f2)
PROJECT=$(grep '^PROJECT=' "$RECORD" | cut -d= -f2)
SA_EMAIL=$(grep '^SA_EMAIL=' "$RECORD" | cut -d= -f2)
mapfile -t ROLES        < <(grep '^ROLE='        "$RECORD" | cut -d= -f2)
mapfile -t CUSTOM_ROLES < <(grep '^CUSTOM_ROLE=' "$RECORD" | cut -d= -f2)
SA_MEMBER="serviceAccount:${SA_EMAIL}"

echo
echo "Teardown"
echo "  organization : $ORG_ID"
echo "  account      : $SA_EMAIL"
echo "  revoking     : ${#ROLES[@]} predefined + ${#CUSTOM_ROLES[@]} custom roles"
$KEEP_SA && echo "  keeping the service account"
$DRY_RUN && echo "  MODE         : dry run, nothing will change"
echo

# Impersonation must be off first: the audit identity has no permission to
# revoke its own bindings, so every command below would fail.
CURRENT=$(gcloud config get-value auth/impersonate_service_account 2>/dev/null || true)
if [[ -n "$CURRENT" && "$CURRENT" != "(unset)" ]]; then
  echo "⚠️  Currently impersonating: $CURRENT"
  echo "   The audit identity cannot revoke its own bindings."
  echo
  read -r -p "   Unset impersonation now? [Y/n] " ans
  if [[ ! "$ans" =~ ^[Nn]$ ]]; then
    gcloud config unset auth/impersonate_service_account
    echo "   unset — continuing as $(gcloud config get-value account 2>/dev/null)"
    echo
  else
    echo "   aborted"; exit 1
  fi
fi

if ! $DRY_RUN; then
  read -r -p "Proceed? [y/N] " ans
  [[ "$ans" =~ ^[Yy]$ ]] || { echo "aborted"; exit 1; }
fi

run() { if $DRY_RUN; then printf '  [dry-run] %s\n' "$*"; else "$@"; fi; }

echo
echo "1/3  revoking organization bindings"
unbind() {
  local role="$1"
  if run gcloud organizations remove-iam-policy-binding "$ORG_ID" \
       --member="$SA_MEMBER" --role="$role" \
       --condition=None --quiet >/dev/null 2>&1; then
    echo "  revoked  $role"
  else
    echo "  skipped  $role (not granted)"
  fi
}
for r in "${ROLES[@]}"; do unbind "$r"; done
for c in "${CUSTOM_ROLES[@]}"; do unbind "organizations/${ORG_ID}/roles/${c}"; done

echo
echo "2/3  deleting custom roles"
for c in "${CUSTOM_ROLES[@]}"; do
  if run gcloud iam roles delete "$c" --organization="$ORG_ID" --quiet >/dev/null 2>&1; then
    echo "  deleted  $c"
  else
    echo "  skipped  $c (already gone)"
  fi
done
echo "  note: custom roles are soft-deleted for 7 days and their IDs stay"
echo "        reserved for 30. Re-running create.sh inside that window will"
echo "        undelete them rather than failing."

echo
echo "3/3  service account"
if $KEEP_SA; then
  echo "  kept, as requested — it now holds no organization roles"
else
  if run gcloud iam service-accounts delete "$SA_EMAIL" --project="$PROJECT" --quiet >/dev/null 2>&1; then
    echo "  deleted  $SA_EMAIL"
    echo "  (this also removes the impersonation grants on it)"
  else
    echo "  skipped  $SA_EMAIL (already gone)"
  fi
fi

echo
echo "Verifying nothing remains…"
LEFT=$(gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten="bindings[].members" \
  --filter="bindings.members:${SA_EMAIL}" \
  --format="value(bindings.role)" 2>/dev/null || true)

if [[ -z "$LEFT" ]]; then
  echo "  ✅ no organization bindings remain for $SA_EMAIL"
else
  echo "  ⚠️  still bound:"
  echo "$LEFT" | sed 's/^/     /'
fi

echo
echo "Also disable any APIs this audit enabled — but ONLY those, from the"
echo "snapshot taken before the audit began:"
echo
echo "  while read -r API; do"
echo "    gcloud services disable \"\$API\" --project=$PROJECT --force --quiet"
echo "  done < ./audit-state/apis-enabled-by-audit.txt"
echo
echo "The audit log record of what the auditor did remains, and should."
