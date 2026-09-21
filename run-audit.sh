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
#       04-compliance-score.md          the three numbers, for people
#       04-compliance-score.json        the same, for the dashboard
#       remediation-plan.csv          import into the tracker
#     evidence/                       the audit trail behind the report
#       results/                      each pass's results and review decisions (read by --review)
#       audit.env                     placeholder values used
#       targets.txt                   projects audited
#       run.log                       full console output
#       iam-inventory.txt             V86
#       excluded.txt                  projects skipped by EXCLUDE_PROJECTS (default ^sys-)
#
#   ./run-audit.sh                                # organization only
#   ./run-audit.sh --project my-proj              # org + one project (repeatable)
#   ./run-audit.sh --no-org --project my-proj     # that project alone
#   ./run-audit.sh --projects config/projects.txt # org + a list, one ID per line
#   ./run-audit.sh --all                          # org + every ACTIVE project
#
# Settings come from config/audit.env — the organization, the service account
# and the check inputs. Copy config/audit.env.example to start one.
#   ./run-audit.sh --review scratch/runs/2026-09-14_12-58-20   # decide every REVIEW check
#
# A run leaves the REVIEW checks undecided, so its Completion is below 100%.
# --review puts each one to the auditor, one at a time, for PASS or FAIL, then
# rebuilds the reports and the remediation plan. Answers are saved as they are
# given; quit at any point and run --review again to carry on.
#
# Runs as whatever gcloud is authenticated as — impersonate the audit service
# account first. Written for bash 3.2 (the macOS default); no dependencies
# beyond what the audit itself needs.
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: ./run-audit.sh [--project ID ...|--projects FILE|--all] [options]
       ./run-audit.sh --review RUN_DIR

Settings come from config/audit.env unless --config says otherwise: ORG_ID,
AUDIT_PROJECT, SA_EMAIL, APPROVED_REGISTRIES, ALLOWED_LOCATIONS,
EXCLUDE_PROJECTS. Start one by copying config/audit.env.example, or write a
fresh template with: go run audit-run.go -init-config config/audit.env

Projects (default: organization pass only)
  --project ID       Audit this project. Repeatable.
  --projects FILE    Audit every project ID in FILE, one per line; # comments allowed.
  --all              Audit every ACTIVE project in the organization.

Options
  --skip V96,V44     Checks NOT to run (reported as SKIP). Use for a check that hangs.
  --parallel N       Checks run at once (default 8). --parallel 1 runs them in order.
  --config FILE      Settings file (default: config/audit.env)
  --org ID           Organization ID (default: ORG_ID from the settings file, or $ORG_ID)
  --out DIR          Where run directories are created (default: ./scratch/runs)
  --no-org           Skip the organization pass (project passes only)
  -h, --help         This message

Review
  --review RUN_DIR   After a run: put each REVIEW check to you for PASS or FAIL,
                     organization first, then each project. Saved as you go; q
                     quits and --review again resumes. Rebuilds the reports and
                     the remediation plan. Needs nothing else — no --config.

Projects whose ID matches EXCLUDE_PROJECTS in the config are never audited
(default ^sys-, the projects Apps Script creates; set EXCLUDE_PROJECTS=none to
audit everything). They are listed in evidence/excluded.txt.
USAGE
}

CONFIG="" ORG="${ORG_ID:-}" OUT="./scratch/runs" PROJECTS_FILE="" ALL=false DO_ORG=true SKIP="" PARALLEL=8 REVIEW_DIR=""
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
    --skip)     SKIP="$2"; shift 2 ;;
    --parallel) PARALLEL="$2"; shift 2 ;;
    --review)   REVIEW_DIR="$2"; shift 2 ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 2 ;;
  esac
done

die() { echo "run-audit: $*" >&2; exit 2; }

# ---------------------------------------------------------------------------
# --review: decide the REVIEW checks of a finished run. Nothing is re-run.
# ---------------------------------------------------------------------------
# rollup.go recognises packs by their content, so it reads report/ directly.
rebuild_plan() {  # $1 run directory (absolute)
  local run="$1"
  go run rollup.go -in "$run/report" -out "$run/report/01-remediation-plan.md" \
    -csv "$run/report/remediation-plan.csv" \
    -score-md "$run/report/04-compliance-score.md" \
    -score-json "$run/report/04-compliance-score.json"
}

report_for() {  # $1 run directory, $2 pass name — the report file a pass is filed as
  if [[ "$2" == "organization" ]]; then echo "$1/report/02-organization/01-automated-results.md"
  else echo "$1/report/03-projects/$2.md"; fi
}

if [[ -n "$REVIEW_DIR" ]]; then
  [[ -d "$REVIEW_DIR/evidence/results" ]] || die "no saved results in $REVIEW_DIR/evidence/results — --review needs a run made by this version of run-audit.sh"
  RUN=$(cd "$REVIEW_DIR" && pwd)
  cd "$(dirname "$0")"
  STATES=()
  [[ -f "$RUN/evidence/results/organization.json" ]] && STATES+=("$RUN/evidence/results/organization.json")
  for f in "$RUN"/evidence/results/*.json; do
    [[ "$(basename "$f")" == "organization.json" ]] || STATES+=("$f")
  done
  STOPPED=false
  for state in "${STATES[@]}"; do
    name=$(basename "$state" .json)
    rm -f "$state.quit"
    go run audit-run.go -decide "$state" -md "$(report_for "$RUN" "$name")" \
      || die "review of $name failed"
    # audit-run leaves this marker when the auditor quits (q): stop the
    # session here rather than moving on to the next pass.
    if [[ -f "$state.quit" ]]; then rm -f "$state.quit"; STOPPED=true; break; fi
  done
  echo
  echo "=== Remediation plan"
  rebuild_plan "$RUN"
  echo
  echo "================================================================"
  echo "CIS IG1 audit review  $(basename "$RUN")"
  for state in "${STATES[@]}"; do
    name=$(basename "$state" .json)
    printf '  %-32s %s\n' "$name" "$(grep -m1 -o 'SCORE: [^*]*' "$(report_for "$RUN" "$name")" || echo 'SCORE: unknown')"
  done
  if $STOPPED; then echo "  stopped — run --review again to carry on"; fi
  echo "================================================================"
  exit 0
fi
# One settings file, in one place. Clearing an output directory must never
# cost the auditor their configuration, so nothing lives beside the runs.
DEFAULT_CONFIG="$(dirname "$0")/config/audit.env"
[[ -n "$CONFIG" ]] || CONFIG="$DEFAULT_CONFIG"
[[ -f "$CONFIG" ]] || die "no settings file at $CONFIG
  Copy the example:  cp config/audit.env.example config/audit.env
  Or write a fresh one:  go run audit-run.go -init-config config/audit.env"

# --org wins, then $ORG_ID, then the settings file.
if [[ -z "$ORG" ]]; then
  ORG=$(grep -E '^[[:space:]]*ORG_ID=' "$CONFIG" | tail -1 | cut -d= -f2- | tr -d "\"'" | sed 's/[[:space:]]*$//' || true)
fi
[[ -n "$ORG" ]]    || die "no organization — set ORG_ID in $CONFIG, pass --org, or export ORG_ID"
[[ "$ORG" =~ ^[0-9]+$ ]] || die "the organization must be the numeric ID, not a domain — got: $ORG"
[[ -z "$PROJECTS_FILE" || -f "$PROJECTS_FILE" ]] || die "projects file not found: $PROJECTS_FILE"
[[ "$PARALLEL" =~ ^[1-9][0-9]*$ ]] || die "--parallel must be a positive number"

# Same default and meaning as audit-run.go: ^sys- unless the config says otherwise.
EXCLUDE=$(grep -E '^[[:space:]]*EXCLUDE_PROJECTS=' "$CONFIG" | tail -1 | cut -d= -f2- | tr -d "\"'" | sed 's/[[:space:]]*$//' || true)
EXCLUDE="${EXCLUDE:-^sys-}"

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
: > "$EVIDENCE/excluded.txt"
if [[ ${#PROJECTS[@]} -gt 0 ]]; then
  printf '%s\n' "${PROJECTS[@]}" | sort -u > "$RUN/.candidates"
  if [[ "$EXCLUDE" == [Nn][Oo][Nn][Ee] ]]; then
    cp "$RUN/.candidates" "$EVIDENCE/targets.txt"
  else
    grep -vE -- "$EXCLUDE" "$RUN/.candidates" > "$EVIDENCE/targets.txt" || true
    # awk via ENVIRON, not sed: an alternation regex (^sys-|^tmp-) contains the
    # sed delimiter, which breaks the substitution and silently leaves this file
    # empty — so the run reports "0 excluded" having excluded everything.
    grep -E -- "$EXCLUDE" "$RUN/.candidates" \
      | EXCL="$EXCLUDE" awk '{print $0 "  (matches EXCLUDE_PROJECTS " ENVIRON["EXCL"] ")"}' \
      > "$EVIDENCE/excluded.txt" || true
  fi
  rm -f "$RUN/.candidates"
fi
EXCLUDED_COUNT=$(grep -c . "$EVIDENCE/excluded.txt" || true)
TARGET_COUNT=$(grep -c . "$EVIDENCE/targets.txt" || true)

IMPERSONATING=$(gcloud config get-value auth/impersonate_service_account 2>/dev/null || true)
echo "CIS IG1 audit  run $RUN_ID"
echo "  organization  $ORG"
echo "  identity      ${IMPERSONATING:-$(gcloud config get-value account 2>/dev/null) (NOT impersonating)}"
echo "  projects      $TARGET_COUNT audited, $EXCLUDED_COUNT excluded (EXCLUDE_PROJECTS $EXCLUDE)"
[[ -n "$SKIP" ]] && echo "  skipping      $SKIP"
echo "  parallel      $PARALLEL"
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
  local f="$2" status counts score
  if [[ ! -f "$f" ]]; then
    STATUS_LINES+=("$(printf '  %-32s NO PACK WRITTEN' "$1")"); BAD=$((BAD + 1)); return
  fi
  status=$(grep -m1 -o 'RUN STATUS: [^*]*' "$f" | sed 's/ *—.*//; s/ *$//')
  counts=$(awk -F'|' '/^\| (PASS|FAIL|REVIEW|N\/A|ERROR|DENIED) \|/ {gsub(/ /,"",$2); gsub(/ /,"",$3); printf "%s %s  ", $2, $3}' "$f")
  score=$(grep -m1 -o 'SCORE: [^*]*' "$f" | sed 's/^SCORE: //; s/ *$//' || true)
  STATUS_LINES+=("$(printf '  %-32s %-32s %s' "$1" "$status" "$counts")")
  [[ -n "$score" ]] && STATUS_LINES+=("$(printf '  %-32s %s' "" "$score")")
  case "$status" in *UNRELIABLE*|*DEGRADED*) BAD=$((BAD + 1)) ;; esac
}

run_pass() {  # args passed to audit-run.go
  set +e
  local extra=(-parallel "$PARALLEL")
  [[ -n "$SKIP" ]] && extra+=(-skip "$SKIP")
  go run audit-run.go -org="$ORG" -config "$CONFIG" "${extra[@]}" "$@"
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
go run rollup.go -in "$PACKS" -out "$REPORT/01-remediation-plan.md" -csv "$REPORT/remediation-plan.csv" \
  -score-md "$REPORT/04-compliance-score.md" -score-json "$REPORT/04-compliance-score.json"

# ---------------------------------------------------------------------------
# File the packs into report/ and evidence/
# ---------------------------------------------------------------------------
if [[ -d "$PACKS/org" ]]; then
  mv "$PACKS/org/01-automated-results.md" "$REPORT/02-organization/01-automated-results.md"
  [[ -f "$PACKS/org/02-manual-cli.md" ]]     && mv "$PACKS/org/02-manual-cli.md"     "$REPORT/02-organization/02-manual-gcp-tasks.md"
  [[ -f "$PACKS/org/03-manual-process.md" ]] && mv "$PACKS/org/03-manual-process.md" "$REPORT/02-organization/03-manual-process.md"
  [[ -f "$PACKS/org/iam-inventory.txt" ]]    && mv "$PACKS/org/iam-inventory.txt"    "$EVIDENCE/iam-inventory.txt"
  if [[ -f "$PACKS/org/results.json" ]]; then
    mkdir -p "$EVIDENCE/results"; mv "$PACKS/org/results.json" "$EVIDENCE/results/organization.json"
  fi
fi
if [[ -d "$PACKS/projects" ]]; then
  for d in "$PACKS"/projects/*/; do
    p=$(basename "$d")
    [[ -f "$d/01-automated-results.md" ]] && mv "$d/01-automated-results.md" "$REPORT/03-projects/$p.md"
    if [[ -f "$d/results.json" ]]; then
      mkdir -p "$EVIDENCE/results"; mv "$d/results.json" "$EVIDENCE/results/$p.json"
    fi
  done
fi
rm -rf "$PACKS"
rmdir "$REPORT/02-organization" "$REPORT/03-projects" 2>/dev/null || true
[[ -s "$EVIDENCE/excluded.txt" ]] || rm -f "$EVIDENCE/excluded.txt"
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
[[ $EXCLUDED_COUNT -gt 0 ]] && echo "  excluded          $EXCLUDED_COUNT project(s) matching $EXCLUDE — see evidence/excluded.txt"
[[ -n "$SKIP" ]] && echo "  checks skipped    $SKIP (reported as SKIP)"
[[ ${#NOT_FOUND[@]} -gt 0 ]] && echo "  not found         ${NOT_FOUND[*]}"
echo
echo "Read:  $REPORT/"
if grep -q 'REVIEW\*\* await the auditor' "$REPORT"/02-organization/01-automated-results.md "$REPORT"/03-projects/*.md 2>/dev/null; then
  echo "Next:  ./run-audit.sh --review $RUN"
  echo "       Completion stays below 100% until every REVIEW check is decided PASS or FAIL."
fi
echo "================================================================"

[[ $BAD -eq 0 ]] || { echo "run-audit: $BAD pass(es) UNRELIABLE, DEGRADED or missing — re-run before using these results" >&2; exit 1; }
