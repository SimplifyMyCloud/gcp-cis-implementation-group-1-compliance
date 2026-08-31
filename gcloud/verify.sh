#!/usr/bin/env bash
#
# Confirms the audit identity is exactly what it should be — and, after
# teardown, that nothing remains.
#
#   ./verify.sh --record ./audit-sa-record.txt
#
set -euo pipefail

RECORD="./audit-sa-record.txt"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --record) RECORD="$2"; shift 2 ;;
    -h|--help) echo "Usage: ./verify.sh [--record FILE]"; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

[[ -f "$RECORD" ]] || { echo "ERROR: record not found: $RECORD" >&2; exit 2; }

ORG_ID=$(grep '^ORG_ID=' "$RECORD" | cut -d= -f2)
PROJECT=$(grep '^PROJECT=' "$RECORD" | cut -d= -f2)
SA_EMAIL=$(grep '^SA_EMAIL=' "$RECORD" | cut -d= -f2)
EXPECTED=$(( $(grep -c '^ROLE=' "$RECORD") + $(grep -c '^CUSTOM_ROLE=' "$RECORD") ))

echo
echo "Verifying $SA_EMAIL in organization $ORG_ID"
echo

# --- 1. does it exist
echo "1  service account"
if gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT" >/dev/null 2>&1; then
  echo "   exists"
  SA_EXISTS=true
else
  echo "   absent — if you have run destroy.sh, that is correct"
  SA_EXISTS=false
fi

# --- 2. no keys, ever
echo
echo "2  keys (must be none — the audit is impersonated, never keyed)"
if $SA_EXISTS; then
  KEYS=$(gcloud iam service-accounts keys list --iam-account="$SA_EMAIL" \
         --managed-by=user --format="value(name)" 2>/dev/null || true)
  if [[ -z "$KEYS" ]]; then
    echo "   ✅ no user-managed keys"
  else
    echo "   ❌ KEYS EXIST — this breaches safeguard 5.2. Delete them:"
    echo "$KEYS" | sed 's|.*/|      gcloud iam service-accounts keys delete |'
  fi
else
  echo "   n/a"
fi

# --- 3. bindings match the record
echo
echo "3  organization bindings"
ACTUAL=$(gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten="bindings[].members" \
  --filter="bindings.members:${SA_EMAIL}" \
  --format="value(bindings.role)" 2>/dev/null | sort || true)
COUNT=$(printf '%s' "$ACTUAL" | grep -c . || true)

if $SA_EXISTS; then
  echo "   $COUNT held, $EXPECTED expected"
  [[ "$COUNT" -eq "$EXPECTED" ]] && echo "   ✅ matches the record" \
                                 || echo "   ⚠️  mismatch — compare below against $RECORD"
else
  [[ "$COUNT" -eq 0 ]] && echo "   ✅ none remain" \
                       || echo "   ❌ $COUNT binding(s) still present after teardown"
fi

# --- 4. every role is read-only
echo
echo "4  write permissions (must be none)"
SUSPECT=$(printf '%s\n' "$ACTUAL" | grep -viE 'viewer|reader|browser|securityReviewer|inventoryViewer|metadataViewer|policyReader|activityAnalysisViewer' | grep . || true)
if [[ -z "$SUSPECT" ]]; then
  echo "   ✅ every role is a viewer, reader, or custom read-only role"
else
  echo "   ⚠️  review these — they are not obviously read-only:"
  printf '%s\n' "$SUSPECT" | sed 's/^/      /'
  echo "      (custom roles named ${ROLE_PREFIX:-cisIg1Audit}* are expected here)"
fi

# --- 5. custom roles hold only what they should
echo
echo "5  custom role permissions"
while read -r c; do
  [[ -z "$c" ]] && continue
  PERMS=$(gcloud iam roles describe "$c" --organization="$ORG_ID" \
          --format="value(includedPermissions)" 2>/dev/null || echo "ABSENT")
  if [[ "$PERMS" == "ABSENT" ]]; then
    echo "   $c — absent (correct after teardown)"
  else
    echo "   $c"
    echo "      $PERMS"
  fi
done < <(grep '^CUSTOM_ROLE=' "$RECORD" | cut -d= -f2)

# --- 6. who can impersonate
echo
echo "6  who may impersonate"
if $SA_EXISTS; then
  gcloud iam service-accounts get-iam-policy "$SA_EMAIL" --project="$PROJECT" \
    --flatten="bindings[].members" \
    --filter="bindings.role:roles/iam.serviceAccountTokenCreator" \
    --format="value(bindings.members)" 2>/dev/null | sed 's/^/   /' || echo "   none"
else
  echo "   n/a"
fi

echo
echo "Full binding list:"
printf '%s\n' "$ACTUAL" | sed 's/^/   /'
echo
