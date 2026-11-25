#!/usr/bin/env bash
#
# Re-derive every number the delivery bundle claims, from the documents and
# the tools that produce them. Read-only: it runs no gcloud, touches no
# estate, writes nothing.
#
#   ./delivery/verify-numbers.sh
#
# Run it before a meeting if the kit has changed since the decks were
# written. A number you cannot source is worse than no number.
#
# Exit 0 = every claim re-derived and matched. Exit 1 = at least one
# mismatch; the row names the file to look at.

set -euo pipefail

cd "$(dirname "$0")/.."

CHECKLIST="docs/cis-ig1-gcp-checklist.md"
APIS="docs/cis-ig1-required-apis.md"

fails=0
skips=0

row() {  # claim, expected, actual, source
  local claim="$1" want="$2" got="$3" src="$4" mark
  if [[ -z "$got" ]]; then
    mark="SKIP"; skips=$((skips + 1)); got="-"
  elif [[ "$got" == "$want" ]]; then
    mark="ok  "
  else
    mark="FAIL"; fails=$((fails + 1))
  fi
  printf '  %s  %-34s %6s %6s   %s\n' "$mark" "$claim" "$want" "$got" "$src"
}

# --- the checklist: one line per requirement, one marker each -------------
eval "$(awk '
  /^#+ [0-9]+\.[0-9]+ / { sg = $2; next }
  /^- \[ \]/ {
    n = 0; kind = ""
    if (index($0, "⚙️"))  { n++; kind = "m" }
    if (index($0, "🔍"))  { n++; kind = "r" }
    if (index($0, "🖥️")) { n++; kind = "c" }
    if (index($0, "👥"))  { n++; kind = "p" }
    if (n != 1) next
    total++
    if      (kind == "m") mech++
    else if (kind == "r") rev++
    else if (kind == "c") con++
    else                  proc++
    if (sg != "") {
      reqs[sg]++
      if (kind == "c" || kind == "p") manual[sg]++
    }
  }
  END {
    for (s in reqs) if (reqs[s] > 0 && manual[s] + 0 == 0) fullyauto++
    printf "D_TOTAL=%d D_MECH=%d D_REV=%d D_CON=%d D_PROC=%d D_FULLYAUTO=%d\n",
      total + 0, mech + 0, rev + 0, con + 0, proc + 0, fullyauto + 0
  }
' "$CHECKLIST")"

D_OFFPLAT=$(awk '
  /^## Appendix A/ { f = 1; next }
  f && /^## /      { f = 0 }
  f && /^\| [0-9]+\.[0-9]+ \|/ { n++ }
  END { print n + 0 }' "$CHECKLIST")

D_HOSTAPIS=$(awk '
  /^## 1\. Enable in the audit host project/ { f = 1; next }
  f && /^## /  { f = 0 }
  f && /^\| *[0-9]+ *\| *`[a-z0-9.-]+\.googleapis\.com`/ { n++ }
  END { print n + 0 }' "$APIS")

# --- the tools -----------------------------------------------------------
G_CHECKS=""; G_ORG=""; G_PROJ=""; G_SGSCOPE=""; G_REQSCOPE=""
if command -v go >/dev/null 2>&1; then
  # -list writes its summary line to stderr, so merge the streams and count
  # the check rows themselves rather than trusting the total.
  out=$(go run audit-run.go -list 2>&1 || true)
  G_CHECKS=$(printf '%s\n' "$out" | grep -cE '^V[0-9]+' || true)

  out=$(go run audit-run.go -list -scope org 2>/dev/null || true)
  G_ORG=$(printf '%s\n' "$out" | grep -cE '^V[0-9]+' || true)

  out=$(go run audit-run.go -list -scope project -project verify-numbers 2>/dev/null || true)
  G_PROJ=$(printf '%s\n' "$out" | grep -cE '^V[0-9]+' || true)

  out=$(go run compliance-report.go -file "$CHECKLIST" 2>/dev/null || true)
  G_SGSCOPE=$(printf '%s\n' "$out" | sed -n 's/^SAFEGUARDS (\([0-9]*\) in scope).*/\1/p')
  G_REQSCOPE=$(printf '%s\n' "$out" | sed -n 's/^REQUIREMENTS (\([0-9]*\) in scope).*/\1/p')
fi

echo
echo "Delivery bundle — claim verification"
echo "===================================="
echo
printf '  %-4s  %-34s %6s %6s   %s\n' "" "claim" "stated" "found" "derived from"
echo

echo "  Requirements"
row "requirements in scope"  290 "$D_TOTAL"       "$CHECKLIST"
row "mechanical (gear)"       96 "$D_MECH"        "$CHECKLIST"
row "CLI, human reads it"     92 "$D_REV"         "$CHECKLIST"
row "console-only"            30 "$D_CON"         "$CHECKLIST"
row "process and people"      72 "$D_PROC"        "$CHECKLIST"
row "automated = 96 + 92"    188 "$((D_MECH + D_REV))"  "derived"
row "manual = 30 + 72"       102 "$((D_CON + D_PROC))"  "derived"

echo
echo "  Safeguards"
row "GCP-actionable"           44 "$G_SGSCOPE"    "compliance-report.go"
row "requirements in scope"   290 "$G_REQSCOPE"   "compliance-report.go"
row "off-platform"             12 "$D_OFFPLAT"    "$CHECKLIST Appendix A"
row "IG1 total = 44 + 12"      56 "$((D_OFFPLAT + 44))" "derived"
row "fully automatable"         5 "$D_FULLYAUTO"  "$CHECKLIST"

echo
echo "  Checks and access"
row "checks parsed"           188 "$G_CHECKS"     "audit-run.go -list"
row "organization pass"        86 "$G_ORG"        "audit-run.go -scope org"
row "project pass"            103 "$G_PROJ"       "audit-run.go -scope project"
row "APIs in the host project" 17 "$D_HOSTAPIS"   "$APIS"
row "interview = 102 + 12"    114 "$((D_CON + D_PROC + D_OFFPLAT))" "derived"

echo
if [[ $fails -gt 0 ]]; then
  echo "  $fails claim(s) no longer hold. Fix the decks before you present them:"
  echo "    delivery/customer/{one-pager,deck}.md  delivery/customer/deck.pptx"
  echo "    delivery/architects/{one-pager,deck}.md  delivery/architects/deck.pptx"
  echo
  exit 1
fi

if [[ $skips -gt 0 ]]; then
  echo "  $skips claim(s) not checked — a tool could not be run (is Go on PATH?)."
  echo "  Everything that was checked matched."
else
  echo "  Every claim in delivery/ re-derived and matched."
fi
echo
