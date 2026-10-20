#!/usr/bin/env bash
#
# Runs a complete CIS IG1 audit and files the result by TARGET, not by date.
# One directory per thing audited; a re-audit replaces it, because the newest
# result is the only one that counts and git holds what came before.
#
#   audits/
#     gcp-org/
#       report/02-organization/01-automated-results.md
#       report/02-organization/02-manual-gcp-tasks.md
#       report/02-organization/03-manual-process.md
#       evidence/results/organization.json   read by --review
#       evidence/run.log  audit.env  targets.txt  iam-inventory.txt
#     <project-id>/
#       report/03-projects/<project-id>.md
#       evidence/results/<project-id>.json
#       evidence/run.log  audit.env  targets.txt
#
#   reports/                               built by --compile
#     remediation-plan.md  remediation-plan.csv
#     compliance-score.md  compliance-score.json
#     safeguards.md        safeguards.json
#
# audits/ is the evidence, reports/ is what you hand over. A target below
# 100% completion appears in no report; --incomplete lists those.
#
#   ./run-audit.sh                                # organization only
#   ./run-audit.sh --project my-proj              # org + one project (repeatable)
#   ./run-audit.sh --no-org --project my-proj     # that project alone
#   ./run-audit.sh --projects config/projects.txt # org + a list, one ID per line
#   ./run-audit.sh --all                          # org + every ACTIVE project
#
# Settings come from config/audit.env — the organization, the service account
# and the check inputs. Copy config/audit.env.example to start one.
#   ./run-audit.sh --review audit-state/runs/2026-09-14_12-58-20  # decide every REVIEW check
#   ./run-audit.sh --interview                    # the requirements no command can answer
#   ./run-audit.sh --compile                      # every report, across every run
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
       ./run-audit.sh --review [TARGET]
       ./run-audit.sh --incomplete
       ./run-audit.sh --interview
       ./run-audit.sh --compile

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
  --interview        Put every requirement no command can answer to the auditor.
                     114 questions, saved as you go, resumable.
  --incomplete       List every target below 100%, what is outstanding, and the
                     command to clear it. Those targets appear in no report.
  --compile          Build every report into reports/: the remediation plan,
                     the compliance score and the safeguard status. Counts the
                     targets present in audits/; for coverage against a wider
                     estate pass -projects to the Go tools by hand.
  --config FILE      Settings file (default: config/audit.env)
  --org ID           Organization ID (default: ORG_ID from the settings file, or $ORG_ID)
  --out DIR          Where results are written (default: ./audits)
  --no-org           Skip the organization pass (project passes only)
  -h, --help         This message

Review
  --review [TARGET]  After a run: put each REVIEW check to you for PASS or FAIL,
                     organization first, then each project. Saved as you go; q
                     quits and --review again resumes. Rebuilds the reports and
                     the remediation plan. Needs nothing else — no --config.

Projects whose ID matches EXCLUDE_PROJECTS in the config are never audited
(default ^sys-, the projects Apps Script creates; set EXCLUDE_PROJECTS=none to
audit everything). They are listed in evidence/excluded.txt.
USAGE
}

CONFIG="" ORG="${ORG_ID:-}" OUT="./audits" PROJECTS_FILE="" ALL=false DO_ORG=true SKIP="" PARALLEL=8
DO_REVIEW=false REVIEW_TARGET="" DO_COMPILE=false DO_INTERVIEW=false DO_INCOMPLETE=false
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
    --review)   DO_REVIEW=true
                if [[ $# -gt 1 && "$2" != -* ]]; then REVIEW_TARGET="$2"; shift 2; else shift; fi ;;
    --compile)  DO_COMPILE=true; shift ;;
    --interview) DO_INTERVIEW=true; shift ;;
    --incomplete) DO_INCOMPLETE=true; shift ;;
    -h|--help)  usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 2 ;;
  esac
done

die() { echo "run-audit: $*" >&2; exit 2; }

# ---------------------------------------------------------------------------
# Target directories
# ---------------------------------------------------------------------------
# Results are organised by WHAT was audited, not when. One directory per
# target, overwritten by a re-audit — the newest result is the only one that
# matters, and git holds the history.
#
#   audits/gcp-org/            the organization pass
#   audits/<project-id>/       one per project
#
# A directory is self-contained: its report, its results file, the run log and
# the settings that produced it.

report_in() {  # $1 target directory — the automated results file inside it
  if [[ -f "$1/report/02-organization/01-automated-results.md" ]]; then
    echo "$1/report/02-organization/01-automated-results.md"
  else
    local f; f=$(ls "$1"/report/03-projects/*.md 2>/dev/null | head -1 || true)
    echo "$f"
  fi
}

state_in() {  # $1 target directory — the results JSON inside it
  ls "$1"/evidence/results/*.json 2>/dev/null | head -1 || true
}

completion_of() {  # $1 report file — the completion percentage, or empty
  [[ -f "$1" ]] || return 0
  grep -m1 -oE 'completion [0-9]+%' "$1" 2>/dev/null | grep -oE '[0-9]+' || true
}

outstanding_in() {  # $1 report file — "22 REVIEW, 2 ERROR" or empty
  [[ -f "$1" ]] || return 0
  awk -F'|' '/^\| (REVIEW|SKIP|ERROR|DENIED) \|/ {
      gsub(/ /,"",$2); gsub(/ /,"",$3); if ($3+0 > 0) printf "%s%s %s", sep, $3, $2; sep=", " }' "$1"
}

# Every target directory under $OUT, organization first.
targets_under() {  # $1 root
  [[ -d "$1" ]] || return 0
  [[ -d "$1/gcp-org" ]] && echo "$1/gcp-org"
  for d in "$1"/*/; do
    d="${d%/}"
    [[ "$(basename "$d")" == "gcp-org" ]] && continue
    [[ -d "$d/evidence/results" || -d "$d/report" ]] || continue
    echo "$d"
  done
}

# ---------------------------------------------------------------------------
# --incomplete: what is below 100%, and therefore in no report
# ---------------------------------------------------------------------------
# --interview and --compile: everything after the runs are done.
# ---------------------------------------------------------------------------
# Neither touches GCP. The interview asks the auditor; the compile reads what
# is already on disk. Both work on the whole engagement rather than one run,
# so they take no run directory.
# Evidence and deliverables are kept apart: audits/ holds one directory per
# target, reports/ holds what you hand over. Keeping the generated reports out
# of the tree that is scanned also means a report can never be mistaken for a
# pack.
STATE="$OUT"                              # where the target directories are
REPORTS="$(dirname "$OUT")/reports"       # where the compiled reports go
ANSWERS="$STATE/manual-answers.json"
HERE=$(cd "$(dirname "$0")" && pwd)

if $DO_INTERVIEW; then
  cd "$HERE"
  mkdir -p "$STATE"
  exec go run compliance-report.go -interview "$ANSWERS"
fi

if $DO_COMPILE; then
  cd "$HERE"
  [[ -d "$OUT" ]] || die "no results found in $OUT — audit something first"

  # config/projects.txt selects what to AUDIT. The reports count what WAS
  # audited: the target directories present. Auto-passing the list as a
  # coverage denominator divided 16 audited projects by a stale 3-line file
  # and reported 533% — so it is not passed. For coverage against a wider
  # estate, call rollup.go or compliance-report.go with -projects by hand.
  proj=()
  ans=()
  if [[ -f "$ANSWERS" ]]; then
    ans=(-answers "$ANSWERS")
  else
    echo "note: no manual answers yet — run ./run-audit.sh --interview, or only the"
    echo "      5 fully automated safeguards can be scored"
  fi

  mkdir -p "$REPORTS"
  echo
  echo "=== Remediation plan and compliance score"
  go run rollup.go -in "$OUT" \
    -out        "$REPORTS/remediation-plan.md" \
    -csv        "$REPORTS/remediation-plan.csv" \
    -score-md   "$REPORTS/compliance-score.md" \
    -score-json "$REPORTS/compliance-score.json" \
    ${proj[@]+"${proj[@]}"}

  echo
  echo "=== Safeguard status"
  go run compliance-report.go -runs "$OUT" \
    -safeguards-md   "$REPORTS/safeguards.md" \
    -safeguards-json "$REPORTS/safeguards.json" \
    ${ans[@]+"${ans[@]}"} ${proj[@]+"${proj[@]}"}

  echo
  echo "================================================================"
  grep -m1 -o 'SCORE: [^*]*' "$REPORTS/compliance-score.md" 2>/dev/null || true
  grep -m1 -o 'SAFEGUARDS: [^*]*' "$REPORTS/safeguards.md"  2>/dev/null || true
  grep -m1 -o 'PROJECTS: [^*]*'   "$REPORTS/safeguards.md"  2>/dev/null || true
  grep -m1 -o 'ORGANIZATION: [^*]*' "$REPORTS/safeguards.md" 2>/dev/null || true
  grep -m1 -o 'PROJECTS, BY SAFEGUARD: [^*]*' "$REPORTS/safeguards.md" 2>/dev/null || true
  echo "================================================================"
  echo "  $REPORTS/remediation-plan.md    the work list, pivoted by finding"
  echo "  $REPORTS/remediation-plan.csv   the same, for the tracker"
  echo "  $REPORTS/compliance-score.md    coverage and the three numbers"
  echo "  $REPORTS/compliance-score.json  the same, for the dashboard"
  echo "  $REPORTS/safeguards.md          which of the 56 pass, and what blocks the rest"
  echo "  $REPORTS/safeguards.json        the same, for the dashboard"
  # Excluded targets are in no report. Say how many, and how to see them.
  part=0
  while IFS= read -r d; do
    [[ -n "$d" ]] || continue
    [[ "$(completion_of "$(report_in "$d")")" == 100 ]] || part=$((part + 1))
  done < <(targets_under "$OUT")
  if [[ "$part" -gt 0 ]]; then
    echo
    echo "  $part target(s) below 100% and excluded — ./run-audit.sh --incomplete"
  fi
  exit 0
fi

# ---------------------------------------------------------------------------
if $DO_INCOMPLETE; then
  cd "$(dirname "$0")"
  [[ -d "$OUT" ]] || die "no results in $OUT — audit something first"
  total=0; partial=0; rows=""
  while IFS= read -r d; do
    [[ -n "$d" ]] || continue
    total=$((total + 1))
    rep=$(report_in "$d"); pct=$(completion_of "$rep")
    [[ "$pct" == 100 ]] && continue
    partial=$((partial + 1))
    rows+=$(printf '%-30s %-11s %-24s %s\n' \
      "$(basename "$d")" "${pct:-?}%" "$(outstanding_in "$rep")" \
      "./run-audit.sh --review $(basename "$d")")$'\n'
  done < <(targets_under "$OUT")

  if [[ "$partial" == 0 ]]; then
    echo "All $total target(s) at 100%. Nothing excluded from the reports."
    exit 0
  fi
  echo "$partial of $total target(s) below 100% — excluded from all reports"
  echo
  printf '%-30s %-11s %-24s %s\n' TARGET COMPLETION OUTSTANDING 'REVIEW WITH'
  printf '%s' "$rows"
  exit 0
fi

# ---------------------------------------------------------------------------
# --review: decide what the machine could not. Nothing is re-run.
# ---------------------------------------------------------------------------
if $DO_REVIEW; then
  cd "$(dirname "$0")"
  DIRS=()
  if [[ -n "$REVIEW_TARGET" ]]; then
    # A target name, or a directory path — a run directory from an older
    # layout still works, so existing results stay reviewable.
    if [[ -d "$REVIEW_TARGET/evidence/results" ]]; then DIRS=("$REVIEW_TARGET")
    elif [[ -d "$OUT/$REVIEW_TARGET/evidence/results" ]]; then DIRS=("$OUT/$REVIEW_TARGET")
    else die "no results for '$REVIEW_TARGET' — try ./run-audit.sh --incomplete"; fi
  else
    while IFS= read -r d; do
      [[ -n "$d" ]] || continue
      rep=$(report_in "$d")
      [[ "$(completion_of "$rep")" == 100 ]] || DIRS+=("$d")
    done < <(targets_under "$OUT")
    [[ ${#DIRS[@]} -gt 0 ]] || { echo "Nothing outstanding — every target is at 100%."; exit 0; }
  fi

  STOPPED=false
  for d in "${DIRS[@]}"; do
    state=$(state_in "$d"); rep=$(report_in "$d")
    [[ -n "$state" && -n "$rep" ]] || { echo "skipping $(basename "$d") — no results file"; continue; }
    rm -f "$state.quit"
    go run audit-run.go -decide "$state" -md "$rep" || die "review of $(basename "$d") failed"
    # audit-run leaves this marker when the auditor quits (q): stop here
    # rather than moving on to the next target.
    if [[ -f "$state.quit" ]]; then rm -f "$state.quit"; STOPPED=true; break; fi
  done

  echo
  echo "================================================================"
  for d in "${DIRS[@]}"; do
    rep=$(report_in "$d")
    printf '  %-30s %s\n' "$(basename "$d")" \
      "$(grep -m1 -o 'SCORE: [^*]*' "$rep" 2>/dev/null || echo 'SCORE: unknown')"
  done
  $STOPPED && echo "  stopped — run --review again to carry on"
  echo "================================================================"
  echo "Then: ./run-audit.sh --compile"
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

# Everything a run produces is staged here, then filed into one directory per
# target. Staging keeps a half-finished run out of the results tree, and lets
# the per-run evidence — the log, the settings, the target list — be copied
# into every target directory so each one stands alone.
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT
EVIDENCE="$STAGE/evidence"
PACKS="$STAGE/.packs"
RUN="$STAGE"
mkdir -p "$EVIDENCE" "$PACKS"

# Everything from here is shown on screen and kept in each target's run.log.
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
echo "CIS IG1 audit"
echo "  organization  $ORG"
echo "  identity      ${IMPERSONATING:-$(gcloud config get-value account 2>/dev/null) (NOT impersonating)}"
echo "  projects      $TARGET_COUNT audited, $EXCLUDED_COUNT excluded (EXCLUDE_PROJECTS $EXCLUDE)"
[[ -n "$SKIP" ]] && echo "  skipping      $SKIP"
echo "  parallel      $PARALLEL"
echo "  output        $OUT/<target>/"
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
# File each pass into its own target directory
# ---------------------------------------------------------------------------
# A re-audit replaces the directory: the newest result is the only one that
# counts, and git holds what came before. Aggregate reports are built by
# --compile across every target, not per run.

file_evidence() {  # $1 target directory
  mkdir -p "$1/evidence"
  cp "$EVIDENCE/run.log"      "$1/evidence/run.log"      2>/dev/null || true
  cp "$EVIDENCE/audit.env"    "$1/evidence/audit.env"    2>/dev/null || true
  cp "$EVIDENCE/targets.txt"  "$1/evidence/targets.txt"  2>/dev/null || true
  [[ -s "$EVIDENCE/excluded.txt" ]] && cp "$EVIDENCE/excluded.txt" "$1/evidence/excluded.txt"
  return 0
}

FILED=()
if [[ -d "$PACKS/org" ]]; then
  d="$OUT/gcp-org"
  rm -rf "$d"
  mkdir -p "$d/report/02-organization" "$d/evidence/results"
  mv "$PACKS/org/01-automated-results.md" "$d/report/02-organization/01-automated-results.md"
  [[ -f "$PACKS/org/02-manual-cli.md" ]]     && mv "$PACKS/org/02-manual-cli.md"     "$d/report/02-organization/02-manual-gcp-tasks.md"
  [[ -f "$PACKS/org/03-manual-process.md" ]] && mv "$PACKS/org/03-manual-process.md" "$d/report/02-organization/03-manual-process.md"
  [[ -f "$PACKS/org/iam-inventory.txt" ]]    && mv "$PACKS/org/iam-inventory.txt"    "$d/evidence/iam-inventory.txt"
  [[ -f "$PACKS/org/results.json" ]]         && mv "$PACKS/org/results.json"         "$d/evidence/results/organization.json"
  file_evidence "$d"
  FILED+=("gcp-org")
fi
if [[ -d "$PACKS/projects" ]]; then
  for s in "$PACKS"/projects/*/; do
    p=$(basename "$s")
    d="$OUT/$p"
    rm -rf "$d"
    mkdir -p "$d/report/03-projects" "$d/evidence/results"
    [[ -f "$s/01-automated-results.md" ]] && mv "$s/01-automated-results.md" "$d/report/03-projects/$p.md"
    [[ -f "$s/results.json" ]]            && mv "$s/results.json"            "$d/evidence/results/$p.json"
    file_evidence "$d"
    FILED+=("$p")
  done
fi
rm -rf "$PACKS"
if [[ ${#NOT_FOUND[@]} -gt 0 ]]; then
  for d in "${FILED[@]}"; do printf '%s\n' "${NOT_FOUND[@]}" > "$OUT/$d/evidence/not-found.txt"; done
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo "================================================================"
echo "CIS IG1 audit"
[[ ${#STATUS_LINES[@]} -gt 0 ]] && printf '%s\n' "${STATUS_LINES[@]}"
[[ $EXCLUDED_COUNT -gt 0 ]] && echo "  excluded          $EXCLUDED_COUNT project(s) matching $EXCLUDE — see evidence/excluded.txt"
[[ -n "$SKIP" ]] && echo "  checks skipped    $SKIP (reported as SKIP)"
[[ ${#NOT_FOUND[@]} -gt 0 ]] && echo "  not found         ${NOT_FOUND[*]}"
echo
for d in "${FILED[@]}"; do echo "  $OUT/$d/"; done
echo
PARTIAL=()
for d in "${FILED[@]}"; do
  [[ "$(completion_of "$(report_in "$OUT/$d")")" == 100 ]] || PARTIAL+=("$d")
done
if [[ ${#PARTIAL[@]} -gt 0 ]]; then
  echo "Next:  ./run-audit.sh --review ${PARTIAL[0]}"
  echo "       ${#PARTIAL[@]} target(s) below 100%: ${PARTIAL[*]}"
  echo "       A target below 100% appears in no report. ./run-audit.sh --incomplete"
else
  echo "Next:  ./run-audit.sh --compile"
fi
echo "================================================================"

[[ $BAD -eq 0 ]] || { echo "run-audit: $BAD pass(es) UNRELIABLE, DEGRADED or missing — re-run before using these results" >&2; exit 1; }
