#!/usr/bin/env bash
#
# Runs a complete CIS IG1 audit and files it into one dated directory:
#
#   <out>/2026-09-14_12-48-05/
#     report/                         what the auditor reads, in reading order
#       01-remediation-plan.md
#       02-organization/
#         01-automated-results.md
#         02-manual-gcp-tasks.md
#         03-manual-process.md
#       03-projects/
#         <project-id>.md             one per project audited
#       04-compliance-score.txt
#       remediation-plan.csv          import into the tracker
#     evidence/                       the audit trail behind the report
#       audit.env                     placeholder values used
#       targets.txt                   projects audited
#       run.log                       full console output
#       iam-inventory.txt             V86
#
#   ./run-audit.sh --config audit.env                       # organization only
#   ./run-audit.sh --config audit.env --project my-proj     # org + one project (repeatable)
#   ./run-audit.sh --config audit.env --projects list.txt   # org + a list, one ID per line
#   ./run-audit.sh --config audit.env --all                 # org + every ACTIVE project
#
# Runs as whatever gcloud is authenticated as — impersonate the audit service
# account first. Written for bash 3.2 (the macOS default); no dependencies
# beyond what the audit itself needs.
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: ./run-audit.sh --config FILE [--project ID ...|--projects FILE|--all] [options]

Required
  --config FILE      Placeholder values (BACKUP_BUCKET, TFSTATE_BUCKET, BACKUP_PROJECT).
                     Create one with: go run audit-run.go -init-config FILE

Projects (default: organization pass only)
  --project ID       Audit this project. Repeatable.
  --projects FILE    Audit every project ID in FILE, one per line; # comments allowed.
  --all              Audit every ACTIVE project in the organization.

Options
  --org ID           Organization ID (default: $ORG_ID)
  --out DIR          Where run directories are created (default: ./scratch/runs)
  --no-org           Skip the organization pass (project passes only)
  -h, --help         This message
USAGE
}

CONFIG="" ORG="${ORG_ID:-}" OUT="./scratch/runs" PROJECTS_FILE="" ALL=false DO_ORG=true
PROJECTS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)   CONFIG="$2"; shift 2 ;;
    --project)  PROJECTS+=("$2"); shift 2 ;;
    --projects) PROJECTS_FILE="$2"; shift 2 ;;
    --all)      ALL=true; shift ;;
    --org)      ORG="$2"; shift 2 ;;
    --out)      OUT="$2"; shift 2 ;;
    --no-org)   DO_ORG=false; shift ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 2 ;;
  esac
done

die() { echo "run-audit: $*" >&2; exit 2; }
[[ -n "$ORG" ]]    || die "no organization — pass --org or export ORG_ID"
[[ "$ORG" =~ ^[0-9]+$ ]] || die "--org must be the numeric organization ID"
[[ -n "$CONFIG" ]] || die "--config is required (create one: go run audit-run.go -init-config audit.env)"
[[ -f "$CONFIG" ]] || die "config not found: $CONFIG"
[[ -z "$PROJECTS_FILE" || -f "$PROJECTS_FILE" ]] || die "projects file not found: $PROJECTS_FILE"

# Resolve the operator's relative paths before moving to the repository root,
# which audit-run.go needs to find docs/cis-ig1-cli-validation.md.
abspath() { (cd "$(dirname "$1")" && printf '%s/%s\n' "$(pwd)" "$(basename "$1")"); }
CONFIG=$(abspath "$CONFIG")
[[ -n "$PROJECTS_FILE" ]] && PROJECTS_FILE=$(abspath "$PROJECTS_FILE")
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd)
cd "$(dirname "$0")"

RUN_ID=$(date +%Y-%m-%d_%H-%M-%S)
RUN="$OUT/$RUN_ID"
REPORT="$RUN/report"
EVIDENCE="$RUN/evidence"
PACKS="$RUN/.packs"            # audit-run's raw pack layout; filed into report/ at the end
mkdir -p "$REPORT/02-organization" "$REPORT/03-projects" "$EVIDENCE" "$PACKS"

# Everything from here is shown on screen and kept in evidence/run.log.
exec > >(tee "$EVIDENCE/run.log") 2>&1

cp "$CONFIG" "$EVIDENCE/audit.env"

# ---------------------------------------------------------------------------
# Targets
# ---------------------------------------------------------------------------
if $ALL; then
  while IFS= read -r p; do PROJECTS+=("$p"); done < <(
    gcloud projects list --filter="lifecycleState:ACTIVE" --format="value(projectId)" | sort)
fi
if [[ -n "$PROJECTS_FILE" ]]; then
  while IFS= read -r p; do
    p="${p%%#*}"; p="${p//[[:space:]]/}"
    [[ -n "$p" ]] && PROJECTS+=("$p")
  done < "$PROJECTS_FILE"
fi
: > "$EVIDENCE/targets.txt"
if [[ ${#PROJECTS[@]} -gt 0 ]]; then
  printf '%s\n' "${PROJECTS[@]}" | sort -u > "$EVIDENCE/targets.txt"
fi
TARGET_COUNT=$(grep -c . "$EVIDENCE/targets.txt" || true)

IMPERSONATING=$(gcloud config get-value auth/impersonate_service_account 2>/dev/null || true)
echo "CIS IG1 audit  run $RUN_ID"
echo "  organization  $ORG"
echo "  identity      ${IMPERSONATING:-$(gcloud config get-value account 2>/dev/null) (NOT impersonating)}"
echo "  projects      $TARGET_COUNT"
echo "  output        $RUN/"
echo

# ---------------------------------------------------------------------------
# Passes. audit-run exits 1 whenever a check FAILs — that is findings, not a
# broken run — so the pack's RUN STATUS decides health, and only exit 2
# (usage or I/O) stops the run.
# ---------------------------------------------------------------------------
STATUS_LINES=()
BAD=0

pass_summary() {  # $1 label, $2 pack file
  local f="$2" status counts
  if [[ ! -f "$f" ]]; then
    STATUS_LINES+=("$(printf '  %-32s NO PACK WRITTEN' "$1")"); BAD=$((BAD + 1)); return
  fi
  status=$(grep -m1 -o 'RUN STATUS: [^*]*' "$f" | sed 's/ *—.*//; s/ *$//')
  counts=$(awk -F'|' '/^\| (PASS|FAIL|REVIEW|N\/A|ERROR|DENIED) \|/ {gsub(/ /,"",$2); gsub(/ /,"",$3); printf "%s %s  ", $2, $3}' "$f")
  STATUS_LINES+=("$(printf '  %-32s %-32s %s' "$1" "$status" "$counts")")
  case "$status" in *UNRELIABLE*|*DEGRADED*) BAD=$((BAD + 1)) ;; esac
}

run_pass() {  # args passed to audit-run.go
  set +e
  go run audit-run.go -org="$ORG" -config "$CONFIG" "$@"
  local rc=$?
  set -e
  [[ $rc -le 1 ]] || { echo "run-audit: audit-run exited $rc — stopping" >&2; exit $rc; }
}

if $DO_ORG; then
  echo "=== Organization pass"
  run_pass -scope=org -pack "$PACKS/org"
  pass_summary "organization" "$PACKS/org/01-automated-results.md"
  echo
fi

NOT_FOUND=()
i=0
if [[ $TARGET_COUNT -gt 0 ]]; then
  # fd 3, not stdin: gcloud and go run inside the loop would otherwise read
  # the rest of the target list as their own input.
  while IFS= read -r p <&3; do
    i=$((i + 1))
    echo "=== Project $i/$TARGET_COUNT: $p"
    if ! gcloud projects describe "$p" --format="value(projectId)" >/dev/null 2>&1; then
      echo "  NOT FOUND (wrong ID, or the audit identity can't see it) — skipped"
      NOT_FOUND+=("$p"); echo; continue
    fi
    run_pass -scope=project -project="$p" -pack "$PACKS/projects/$p"
    pass_summary "$p" "$PACKS/projects/$p/01-automated-results.md"
    echo
  done 3< "$EVIDENCE/targets.txt"
fi

# ---------------------------------------------------------------------------
# Rollup and score
# ---------------------------------------------------------------------------
echo "=== Remediation plan"
go run rollup.go -in "$PACKS" -out "$REPORT/01-remediation-plan.md" -csv "$REPORT/remediation-plan.csv"
echo
go run compliance-report.go > "$REPORT/04-compliance-score.txt"

# ---------------------------------------------------------------------------
# File the packs into report/ and evidence/
# ---------------------------------------------------------------------------
if [[ -d "$PACKS/org" ]]; then
  mv "$PACKS/org/01-automated-results.md" "$REPORT/02-organization/01-automated-results.md"
  [[ -f "$PACKS/org/02-manual-cli.md" ]]     && mv "$PACKS/org/02-manual-cli.md"     "$REPORT/02-organization/02-manual-gcp-tasks.md"
  [[ -f "$PACKS/org/03-manual-process.md" ]] && mv "$PACKS/org/03-manual-process.md" "$REPORT/02-organization/03-manual-process.md"
  [[ -f "$PACKS/org/iam-inventory.txt" ]]    && mv "$PACKS/org/iam-inventory.txt"    "$EVIDENCE/iam-inventory.txt"
fi
if [[ -d "$PACKS/projects" ]]; then
  for d in "$PACKS"/projects/*/; do
    p=$(basename "$d")
    [[ -f "$d/01-automated-results.md" ]] && mv "$d/01-automated-results.md" "$REPORT/03-projects/$p.md"
  done
fi
rm -rf "$PACKS"
rmdir "$REPORT/02-organization" "$REPORT/03-projects" 2>/dev/null || true
if [[ ${#NOT_FOUND[@]} -gt 0 ]]; then
  printf '%s\n' "${NOT_FOUND[@]}" > "$EVIDENCE/not-found.txt"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
FINDINGS=$(grep -c '^| [0-9]' "$REPORT/01-remediation-plan.md" || true)
echo "================================================================"
echo "CIS IG1 audit  run $RUN_ID"
[[ ${#STATUS_LINES[@]} -gt 0 ]] && printf '%s\n' "${STATUS_LINES[@]}"
echo "  remediation plan  $FINDINGS distinct finding(s)"
[[ ${#NOT_FOUND[@]} -gt 0 ]] && echo "  not found         ${NOT_FOUND[*]}"
echo
echo "Read:  $REPORT/"
echo "================================================================"

[[ $BAD -eq 0 ]] || { echo "run-audit: $BAD pass(es) UNRELIABLE, DEGRADED or missing — re-run before using these results" >&2; exit 1; }
