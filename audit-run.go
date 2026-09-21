//go:build ignore

// Standalone script. The build tag keeps it out of package builds so that
// two `package main` files can sit in one directory without colliding —
// `go vet ./...` and editors would otherwise report "main redeclared".
// Run it directly: go run audit-run.go

// Command audit-run executes every CLI validation check and prints a single
// report you can tick the checklist from.
//
// It parses docs/cis-ig1-cli-validation.md rather than embedding the commands,
// so the document stays the single source of truth. Edit a command there and
// this picks it up — there is no second copy to drift.
//
// Why this exists: auditing item-by-item means read, click through to the
// command, copy, run, interpret, navigate back, tick — seven actions, 188
// times. Running everything first and ticking from one results page removes
// the navigation entirely.
//
//	go run audit-run.go                      # run everything
//	go run audit-run.go -list                # show what would run
//	go run audit-run.go -only V27,V30,V91    # a subset
//	go run audit-run.go -format=md -out report.md
//	go run audit-run.go -set PROJECT_ID=my-proj -set LOG_BUCKET=my-logs
//
// $ORG_ID must be exported. Placeholders like PROJECT_ID are supplied with
// -set or from the environment; checks with unresolved placeholders are
// reported SKIP rather than run against a broken command.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type verdict int

const (
	vPass verdict = iota
	vFail
	vReview // ran fine, but a human decides
	vSkip   // unresolved placeholder
	vNA     // API not enabled / product absent
	vDenied // permission error
	vError  // anything else
	vXref   // no command; points at another check
)

func (v verdict) label() string {
	return [...]string{"PASS", "FAIL", "REVIEW", "SKIP", "N/A", "DENIED", "ERROR", "XREF"}[v]
}

type check struct {
	num       int
	id        string // "V27"
	title     string
	ref       string // "3.3#1"
	safeguard string
	scope     string // "org" | "project" | "xref"
	command   string
	criteria  string
	// emptyPass is true when the pass criteria says empty output means
	// compliant. Those checks can be auto-verdicted; the rest cannot, and
	// are surfaced as REVIEW rather than guessed at.
	emptyPass bool
	// evidence is true when the pass criteria opens "Evidence": the command's
	// output is the record the requirement asks for, so it passes once the
	// command runs cleanly, and the output is kept in the pack.
	evidence   bool
	rawCommand string   // pre-substitution, so answers can be re-applied
	missing    []string // unresolved placeholders
	absent     []string // placeholders the operator declared non-existent
	refs       []string // for xrefs: the checks this one defers to
	byHand     string   // command the auditor runs manually (```sh block); never executed
	skipped    bool     // excluded by the operator with -skip; reported, never run
}

// manualItem is a requirement with no CLI check: either a GCP task that must
// be done by hand (category 3) or a process/documentation requirement that has
// nothing to do with infrastructure state (category 4).
type manualItem struct {
	ref       string
	safeguard string
	scope     string // "org" | "project" | "xref"
	sgTitle   string
	title     string
	gcpTask   bool // true = category 3, false = category 4
}

type result struct {
	check
	v        verdict
	output   string
	errText  string
	duration time.Duration
	// Set when an auditor decided a REVIEW check with -decide. v then holds
	// the auditor's verdict; the machine's REVIEW is kept in the saved state.
	reviewedBy string
	reviewedAt string
}

var (
	reCheck = regexp.MustCompile(`(?m)^#### (V\d+)\s*$`)
	reTitle = regexp.MustCompile(`\*\*(.+?)\*\* · checklist ` + "`" + `([\d]+\.[\d]+#[\d]+)` + "`")
	reScope = regexp.MustCompile(`· scope: (org|project|xref)`)
	reBash  = regexp.MustCompile("(?s)```bash\n(.*?)\n```")
	// A ```sh block is shown to the auditor but never executed: it does
	// something a read-only audit identity must not (e.g. SSH to an instance).
	reManualSh = regexp.MustCompile("(?s)```sh\n(.*?)\n```")
	reCriteria = regexp.MustCompile(`(?m)^\*\*Pass:\*\* (.+)$`)
	// A placeholder only counts where a value is genuinely expected: after
	// "=" or "gs://". Matching bare UPPER_SNAKE anywhere produces false
	// positives on jq output labels ("UNATTACHED DISK:") and on format
	// column names ("table(SERVICE,REGION)"), neither of which needs
	// substituting. "=$VAR" is deliberately excluded — that is a shell
	// variable the caller has already exported.
	rePlaceholder = regexp.MustCompile(`(?:=|gs://)([A-Z][A-Z0-9_]{3,})\b`)
	reSeeRef      = regexp.MustCompile(`\bSee (V\d+)`)
	reSafeguardH  = regexp.MustCompile(`(?m)^## (\d+\.\d+) (.+)$`)
	reManualHead  = regexp.MustCompile(`\*\*Manual — (GCP task, no CLI surface|process or documentation, not infrastructure):\*\*`)
	reManualItem  = regexp.MustCompile("(?m)^- `(\\d+\\.\\d+#\\d+)` (.+)$")

	// gcloud prints this on every impersonated call. It is not an error, and
	// left in stderr it becomes the first — often the only visible — line of
	// every Problems entry, hiding the real message beneath it.
	reImpersonationNote = regexp.MustCompile(`(?m)^WARNING: This command is using service account impersonation\..*$\n?`)
	// Something in the pipeline failed: gcloud, bq, jq, or bash itself.
	reToolError = regexp.MustCompile(`(?m)^(ERROR: |BigQuery error|jq: error|bash: |.*syntax error|.*command not found)`)
	// Names the disabled API and the project it is disabled in, from either
	// gcloud's own "API [x] not enabled on project [n]" line or the activation
	// URL in the SERVICE_DISABLED error body.
	reDisabledAPI = regexp.MustCompile(`API \[([a-z0-9.-]+\.googleapis\.com)\] not enabled on project \[([^\]]+)\]` +
		`|apis/api/([a-z0-9.-]+\.googleapis\.com)/overview\?project=([a-z0-9-]+)`)
)

// hostProjectNumber and hostProjectID identify the project that owns the
// impersonated audit account, which is where gcloud bills the audit's API
// calls. An API disabled there is a setup gap, reported as ERROR rather than
// N/A. Both empty if they can't be determined.
var hostProjectNumber, hostProjectID string

// disabledAPI returns the API and project named in a SERVICE_DISABLED error.
func disabledAPI(errText string) (api, project string, ok bool) {
	m := reDisabledAPI.FindStringSubmatch(errText)
	switch {
	case m == nil:
		return "", "", false
	case m[1] != "":
		return m[1], m[2], true
	default:
		return m[3], m[4], true
	}
}

// resolveHostProject finds the audit host project from the impersonated
// service account's email — NAME@PROJECT.iam.gserviceaccount.com.
func resolveHostProject() {
	out, err := exec.Command("gcloud", "config", "get-value", "auth/impersonate_service_account").Output()
	if err != nil {
		return
	}
	sa := strings.TrimSpace(string(out))
	_, domain, ok := strings.Cut(sa, "@")
	if !ok || !strings.HasSuffix(domain, ".iam.gserviceaccount.com") {
		return
	}
	hostProjectID = strings.TrimSuffix(domain, ".iam.gserviceaccount.com")
	num, err := exec.Command("gcloud", "projects", "describe", hostProjectID, "--format=value(projectNumber)").Output()
	if err == nil {
		hostProjectNumber = strings.TrimSpace(string(num))
	}
}

// gcloud enum values that legitimately follow "=" and must not be treated
// as placeholders awaiting substitution.
// defaultConfig is where the settings live. Keeping them out of the output
// directories means clearing a scratch directory never costs anybody their
// configuration.
const defaultConfig = "config/audit.env"

var notPlaceholders = map[string]bool{
	"ORG_ID": true, "AUDIT_PROJECT": true, "SA_EMAIL": true,
	"TRUE": true, "FALSE": true, "NULL": true, "EGRESS": true, "INGRESS": true,
	"EXTERNAL": true, "EXTERNAL_MANAGED": true, "ADMIN_READ": true,
	"DATA_READ": true, "DATA_WRITE": true, "SECURITY": true, "TECHNICAL": true,
	"ACTIVE": true, "CRITICAL": true, "DELETE_REQUESTED": true,
	"REQUIRE_ATTESTATION": true, "SECURITY_CENTER_SERVICE": true,
}

func main() {
	var (
		docPath   = flag.String("doc", "docs/cis-ig1-cli-validation.md", "validation document to parse")
		only      = flag.String("only", "", "comma-separated V-numbers to run (e.g. V27,V30)")
		skip      = flag.String("skip", "", "comma-separated V-numbers NOT to run (e.g. V96); reported as SKIP")
		list      = flag.Bool("list", false, "list checks without running them")
		parallel  = flag.Int("parallel", 8, "concurrent gcloud invocations")
		timeout   = flag.Duration("timeout", 3*time.Minute, "per-check timeout")
		format    = flag.String("format", "text", "text | md")
		outPath   = flag.String("out", "", "write report to a file instead of stdout")
		showAll   = flag.Bool("all", false, "include PASS entries in detailed output")
		quiet     = flag.Bool("quiet", false, "suppress the live per-check stream")
		reviewOut = flag.String("review", "", "write the REVIEW worksheet to a file (full output, nothing truncated)")
		pack      = flag.String("pack", "", "write the full audit pack (automated table + both manual worksheets) to a directory")
		config    = flag.String("config", "", "settings file (default: config/audit.env when it exists)")
		scope     = flag.String("scope", "", "which pass to run: org | project (required)")
		org       = flag.String("org", "", "organization ID to audit (defaults to $ORG_ID)")
		project   = flag.String("project", "", "the project to audit, for -scope=project")
		initCfg   = flag.String("init-config", "", "write a starter settings file listing every placeholder this run needs, then exit")
		noPrompt  = flag.Bool("no-prompt", false, "never prompt; leave unresolved placeholders as SKIP (for CI)")
		decide    = flag.String("decide", "", "review a finished pass: put each REVIEW check to the auditor for PASS or FAIL (reads a pack's results.json)")
		decideMD  = flag.String("md", "", "with -decide: the report to rebuild (default: 01-automated-results.md beside the results.json)")
	)
	var sets multiFlag
	flag.Var(&sets, "set", "placeholder substitution, repeatable (-set PROJECT_ID=foo)")
	flag.Parse()

	if *decide != "" {
		os.Exit(runDecide(*decide, *decideMD))
	}

	subs := map[string]string{}
	for _, kv := range sets {
		if k, v, ok := strings.Cut(kv, "="); ok {
			subs[k] = v
		}
	}

	// One settings file, in one place, so the auditor never types a path and
	// nothing that matters lives in a directory anybody might clear out.
	if *config == "" {
		if _, err := os.Stat(defaultConfig); err == nil {
			*config = defaultConfig
		}
	}

	if *config != "" {
		loaded, err := loadConfig(*config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
			os.Exit(2)
		}
		for k, v := range loaded {
			if _, already := subs[k]; !already {
				subs[k] = v
			}
		}
	}

	// -org wins over the environment, and the environment over the settings
	// file; the commands themselves read $ORG_ID, so set it either way. A flag
	// beside -project is less surprising than an environment variable for one
	// and a flag for the other.
	if *org != "" {
		os.Setenv("ORG_ID", *org)
	}
	if os.Getenv("ORG_ID") == "" && subs["ORG_ID"] != "" {
		os.Setenv("ORG_ID", subs["ORG_ID"])
	}
	if os.Getenv("ORG_ID") == "" {
		fmt.Fprintf(os.Stderr, "audit-run: no organization specified.\n\n"+
			"  Set it once:  ORG_ID in %s\n"+
			"  Or pass it:   -org=123456789012\n"+
			"  Or export it: export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)\n", defaultConfig)
		os.Exit(2)
	}

	checks, err := parseDoc(*docPath, subs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-run: cannot read %s\n  %v\n\n"+
			"  The checks live in that document — run this from the repository root,\n"+
			"  or point at it with -doc.\n", *docPath, err)
		os.Exit(2)
	}
	if len(checks) == 0 {
		fmt.Fprintf(os.Stderr, "audit-run: no checks parsed from %s\n", *docPath)
		os.Exit(2)
	}

	if *initCfg != "" {
		if err := writeConfigTemplate(*initCfg, checks); err != nil {
			fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
			os.Exit(2)
		}
		return
	}

	switch *scope {
	case "org":
		var f []check
		for _, c := range checks {
			if inPass(c, "org", checks) {
				f = append(f, c)
			} else {
				otherPass = append(otherPass, c)
			}
		}
		checks = f
	case "project":
		auditTarget = "project " + *project
		auditScope = "project"
		if *project == "" {
			fmt.Fprintln(os.Stderr, "audit-run: -scope=project needs -project=PROJECT_ID\n"+
				"  The project pass targets one project per run. List them with:\n"+
				"    gcloud projects list --format=\"value(projectId)\"")
			os.Exit(2)
		}
		if pattern, re := excludePattern(subs); re != nil && re.MatchString(*project) {
			fmt.Fprintf(os.Stderr, "audit-run: project %s matches EXCLUDE_PROJECTS (%s) — not audited.\n"+
				"  Set EXCLUDE_PROJECTS=none in the config to audit it anyway.\n", *project, pattern)
			os.Exit(2)
		}
		subs["PROJECT_ID"] = *project
		// Most project checks read the project from the shell, not from a
		// placeholder: `--project="$PROJECT_ID"` is deliberately not substituted
		// (see rePlaceholder), and many commands carry no --project at all and
		// fall back to gcloud's core/project. Without both of these, the pass
		// silently audits an empty project or whatever gcloud was last pointed
		// at — typically the audit host project.
		os.Setenv("PROJECT_ID", *project)
		os.Setenv("CLOUDSDK_CORE_PROJECT", *project)
		var f []check
		for _, c := range checks {
			if inPass(c, "project", checks) {
				f = append(f, c)
			} else {
				otherPass = append(otherPass, c)
			}
		}
		checks = f
		// re-substitute now that PROJECT_ID is known
		for i := range checks {
			if checks[i].rawCommand != "" {
				checks[i].command, checks[i].missing, checks[i].absent = substitute(checks[i].rawCommand, subs)
			}
		}
	case "":
		if *initCfg == "" && !*list {
			fmt.Fprintln(os.Stderr, "audit-run: -scope is required.\n\n"+
				"  Run the organization pass first:\n"+
				"    go run audit-run.go -scope=org -org=$ORG_ID -pack ./audit-org\n\n"+
				"  Then one pass per project:\n"+
				"    go run audit-run.go -scope=project -org=$ORG_ID -project=PROJECT_ID \\\n"+
				"      -pack ./audit-PROJECT_ID")
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "audit-run: unknown -scope %q (want org or project)\n", *scope)
		os.Exit(2)
	}

	if *only != "" {
		want := map[string]bool{}
		for _, id := range strings.Split(*only, ",") {
			want[strings.ToUpper(strings.TrimSpace(id))] = true
		}
		var f []check
		for _, c := range checks {
			if want[c.id] {
				f = append(f, c)
			}
		}
		checks = f
	}
	if *skip != "" {
		// Skipped checks stay in the report as SKIP, so a reader sees what was
		// deliberately not run rather than finding it silently missing.
		for _, id := range strings.Split(*skip, ",") {
			id = strings.ToUpper(strings.TrimSpace(id))
			for i := range checks {
				if checks[i].id == id {
					checks[i].skipped = true
				}
			}
		}
	}

	// Resolve placeholders BEFORE running anything. A value the operator can
	// supply should never become a SKIP mid-run, and "this does not exist" is
	// a finding rather than an absence of information.
	if !*list && !*noPrompt {
		checks = promptForMissing(checks, subs)
	}

	if *list {
		for _, c := range checks {
			state := "runnable"
			switch {
			case c.byHand != "":
				state = "by-hand"
			case c.command == "":
				state = "xref"
			case len(c.missing) > 0:
				state = "needs " + strings.Join(c.missing, ",")
			case c.evidence:
				state = "evidence"
			case !c.emptyPass:
				state = "review"
			}
			fmt.Printf("%-6s %-9s %-8s %s\n", c.id, c.ref, state, c.title)
		}
		fmt.Fprintf(os.Stderr, "\n%d checks\n", len(checks))
		return
	}

	// Checks that write evidence files (V86's IAM inventory) put them in the
	// pack, not in whatever directory the operator happened to run from.
	if *pack != "" {
		if err := os.MkdirAll(*pack, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
			os.Exit(2)
		}
		os.Setenv("AUDIT_PACK_DIR", *pack)
	}

	resolveHostProject()
	if !*quiet {
		fmt.Fprintf(os.Stderr, "Running %d checks against organization %s\n", len(checks), os.Getenv("ORG_ID"))
		if hostProjectID != "" {
			fmt.Fprintf(os.Stderr, "Audit host project %s (%s)\n", hostProjectID, hostProjectNumber)
		}
		fmt.Fprintln(os.Stderr)
	}
	results := run(checks, *parallel, *timeout, !*quiet)
	if !*quiet {
		fmt.Fprintln(os.Stderr)
	}

	out := os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
			os.Exit(2)
		}
		defer f.Close()
		out = f
	}

	if *format == "md" {
		renderMD(out, results, *showAll)
	} else {
		renderText(out, results, *showAll)
	}

	if *pack != "" {
		manual, err := parseManual(*docPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
			os.Exit(2)
		}
		if err := writePack(*pack, results, manual); err != nil {
			fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
			os.Exit(2)
		}
	}

	if *reviewOut != "" {
		if err := writeReviewSheet(*reviewOut, results); err != nil {
			fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
			os.Exit(2)
		}
		n := 0
		for _, r := range results {
			if r.v == vReview {
				n++
			}
		}
		fmt.Fprintf(os.Stderr, "wrote %s — %d checks needing human review\n", *reviewOut, n)
	}

	for _, r := range results {
		if r.v == vFail || r.v == vDenied || r.v == vError {
			os.Exit(1)
		}
	}
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }

func parseDoc(path string, subs map[string]string) ([]check, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(raw)

	locs := reCheck.FindAllStringSubmatchIndex(text, -1)
	var checks []check

	for i, loc := range locs {
		id := text[loc[2]:loc[3]]
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := text[loc[1]:end]

		c := check{id: id}
		c.num, _ = strconv.Atoi(strings.TrimPrefix(id, "V"))

		if m := reTitle.FindStringSubmatch(body); m != nil {
			c.title = m[1]
			c.ref = m[2]
			c.safeguard = strings.Split(m[2], "#")[0]
		}
		if m := reScope.FindStringSubmatch(body); m != nil {
			c.scope = m[1]
		}
		if m := reBash.FindStringSubmatch(body); m != nil {
			c.command = strings.TrimSpace(m[1])
		} else if m := reManualSh.FindStringSubmatch(body); m != nil {
			c.byHand = strings.TrimSpace(m[1])
		}
		if m := reCriteria.FindStringSubmatch(body); m != nil {
			c.criteria = strings.TrimSpace(m[1])
		}

		// Auto-verdict only where the criteria *opens* by saying empty means
		// compliant. Merely mentioning "empty" is not enough — V43 reads
		// "Empty output is a total fail", where the polarity is reversed.
		// Anything ambiguous becomes REVIEW rather than a confident wrong
		// answer, which is the failure mode that matters in an audit.
		lc := strings.ToLower(c.criteria)
		c.emptyPass = strings.HasPrefix(lc, "empty") && !strings.Contains(lc, "is a total fail")
		c.evidence = strings.HasPrefix(lc, "evidence")

		if c.byHand != "" {
			// Run by hand; nothing to parse or substitute.
		} else if c.command == "" {
			// A cross-reference entry: no command, just "See V57 and V58."
			for _, m := range reSeeRef.FindAllStringSubmatch(body, -1) {
				c.refs = append(c.refs, m[1])
			}
			// "V57 and V58" — the second reference has no "See" prefix.
			for _, m := range regexp.MustCompile(`\band (V\d+)`).FindAllStringSubmatch(body, -1) {
				c.refs = append(c.refs, m[1])
			}
		} else {
			c.rawCommand = c.command
			c.command, c.missing, c.absent = substitute(c.command, subs)
		}
		checks = append(checks, c)
	}

	sort.Slice(checks, func(i, j int) bool { return checks[i].num < checks[j].num })
	return checks, nil
}

// substitute replaces known placeholders and reports any left unresolved.
// parseManual walks the document for the two manual blocks under each
// safeguard heading. They are the 102 requirements no script can answer.
func parseManual(path string) ([]manualItem, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")

	var out []manualItem
	var sg, sgTitle string
	gcpTask, inBlock := false, false

	for _, ln := range lines {
		if m := reSafeguardH.FindStringSubmatch(ln); m != nil {
			sg, sgTitle, inBlock = m[1], m[2], false
			continue
		}
		if m := reManualHead.FindStringSubmatch(ln); m != nil {
			gcpTask = strings.HasPrefix(m[1], "GCP task")
			inBlock = true
			continue
		}
		if inBlock {
			if m := reManualItem.FindStringSubmatch(ln); m != nil {
				out = append(out, manualItem{
					ref: m[1], safeguard: sg, sgTitle: sgTitle,
					title: m[2], gcpTask: gcpTask,
				})
				continue
			}
			if strings.TrimSpace(ln) != "" {
				inBlock = false
			}
		}
	}
	return out, nil
}

func substitute(cmd string, subs map[string]string) (string, []string, []string) {
	seen := map[string]bool{}
	var missing, absent []string
	seenAbsent := map[string]bool{}

	out := rePlaceholder.ReplaceAllStringFunc(cmd, func(match string) string {
		// match is "=TOKEN" or "gs://TOKEN"; keep the prefix intact.
		idx := strings.LastIndexAny(match, "=/")
		prefix, tok := match[:idx+1], match[idx+1:]

		if notPlaceholders[tok] {
			return match
		}
		if v, ok := subs[tok]; ok {
			// "none" means the operator confirmed this resource does not exist.
			// Running the command against a literal "none" would produce a
			// misleading error; the honest result is a finding.
			if strings.EqualFold(v, "none") {
				if !seenAbsent[tok] {
					seenAbsent[tok] = true
					absent = append(absent, tok)
				}
				return match
			}
			return prefix + v
		}
		if v := os.Getenv(tok); v != "" {
			return prefix + v
		}
		if v, ok := placeholderDefault[tok]; ok {
			return prefix + v
		}
		if !seen[tok] {
			seen[tok] = true
			missing = append(missing, tok)
		}
		return match
	})

	sort.Strings(missing)
	sort.Strings(absent)
	return out, missing, absent
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// resolveXrefs gives cross-reference entries the verdict of whatever they
// defer to. Without this they sit as XREF forever and read like an error.
// The worst verdict among the referenced checks wins — a cross-reference to
// two checks is only satisfied when both are.
// inPass reports whether a check belongs to the given pass. A cross-reference
// joins a pass only when at least one check it defers to runs there; otherwise
// it would sit unresolved as XREF in every pass.
func inPass(c check, pass string, all []check) bool {
	if c.scope == pass {
		return true
	}
	if c.scope != "xref" {
		return false
	}
	for _, id := range c.refs {
		for _, o := range all {
			if o.id == id && o.scope == pass {
				return true
			}
		}
	}
	return false
}

func resolveXrefs(results []result) {
	byID := map[string]*result{}
	for i := range results {
		byID[results[i].id] = &results[i]
	}
	rank := map[verdict]int{vPass: 0, vReview: 1, vNA: 2, vSkip: 3, vError: 4, vDenied: 5, vFail: 6}

	for i := range results {
		if results[i].v != vXref || len(results[i].refs) == 0 {
			continue
		}
		worst, found := vPass, false
		var from, elsewhere []string
		for _, id := range results[i].refs {
			ref, ok := byID[id]
			if !ok || ref.v == vXref {
				elsewhere = append(elsewhere, id)
				continue
			}
			found = true
			from = append(from, id+"="+ref.v.label())
			if rank[ref.v] > rank[worst] {
				worst = ref.v
			}
		}
		if found {
			results[i].v = worst
			results[i].output = "inherited from " + strings.Join(from, ", ")
			// Half the evidence is not a pass: the rest is checked in the
			// other pass (e.g. V112 needs org-scope V68 and project-scope V72).
			if len(elsewhere) > 0 && rank[worst] <= rank[vReview] {
				results[i].v = vReview
				results[i].output += "; also requires " + strings.Join(elsewhere, ", ") +
					", which runs in the other pass — check it there before ticking"
			}
			results[i].errText = results[i].output
		}
	}
}

func run(checks []check, parallel int, timeout time.Duration, stream bool) []result {
	results := make([]result, len(checks))
	jobs := make(chan int)
	var wg sync.WaitGroup

	// Identical commands are memoised — several checks legitimately share one.
	var mu sync.Mutex
	cache := map[string]result{}

	var done int
	total := 0
	for _, c := range checks {
		if c.command != "" && len(c.missing) == 0 {
			total++
		}
	}

	for w := 0; w < parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				c := checks[i]
				switch {
				case c.skipped:
					results[i] = result{check: c, v: vSkip, errText: "not run: excluded with -skip"}
					continue
				case c.byHand != "":
					results[i] = result{check: c, v: vReview,
						output: "NOT RUN by audit-run — this command must be run by hand:\n\n" + c.byHand}
					continue
				case c.command == "":
					results[i] = result{check: c, v: vXref, output: c.criteria}
					continue
				case len(c.absent) > 0:
					// The operator stated this resource does not exist. That is a
					// compliance finding, not missing information.
					results[i] = result{check: c, v: vFail,
						output: "Does not exist in this organization: " + strings.Join(c.absent, ", ") +
							"\nThe requirement cannot be satisfied by a resource that was never created."}
					continue
				case len(c.missing) > 0:
					results[i] = result{check: c, v: vSkip,
						errText: "no value supplied for: " + strings.Join(c.missing, ", ")}
					continue
				}

				mu.Lock()
				cached, hit := cache[c.command]
				mu.Unlock()
				if hit {
					cached.check = c
					results[i] = cached
					continue
				}

				r := execute(c, timeout)
				mu.Lock()
				cache[c.command] = r
				done++
				if stream {
					fmt.Fprintf(os.Stderr, "  [%3d/%3d] %-6s %-7s %-9s %s\n",
						done, total, r.id, r.v.label(), r.ref, truncate(r.title, 52))
				}
				mu.Unlock()
				results[i] = r
			}
		}()
	}

	for i := range checks {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	resolveXrefs(results)
	return results
}

func execute(c check, timeout time.Duration) result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "bash", "-c", c.command)
	// On timeout, killing bash alone leaves gcloud running with the output
	// pipe open, and Run() waits for it indefinitely. Run each check in its
	// own process group, kill the whole group, and stop waiting on the pipes
	// shortly after.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	// Without this, a check against a disabled API blocks forever on gcloud's
	// interactive "enable and retry? (y/N)" prompt.
	cmd.Env = append(os.Environ(), "CLOUDSDK_CORE_DISABLE_PROMPTS=1")

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	r := result{
		check:    c,
		output:   strings.TrimSpace(stdout.String()),
		errText:  strings.TrimSpace(reImpersonationNote.ReplaceAllString(stderr.String(), "")),
		duration: time.Since(start),
	}

	if ctx.Err() == context.DeadlineExceeded {
		r.v = vError
		r.errText = fmt.Sprintf("timed out after %s", timeout)
		return r
	}

	low := strings.ToLower(r.errText)

	// A disabled API is reported by Google as PERMISSION_DENIED with reason
	// SERVICE_DISABLED, so this must be tested before the permission case or
	// every disabled API reads as a missing grant.
	if strings.Contains(low, "service_disabled") || strings.Contains(low, "has not been used in project") ||
		strings.Contains(low, "not enabled") {
		api, proj, ok := disabledAPI(r.errText)
		// When the audited project is also the host project, the API is gated by
		// the project being audited: treat it as an absent product, not a gap.
		auditingHost := auditScope == "project" && (os.Getenv("PROJECT_ID") == hostProjectID)
		if ok && proj != "" && !auditingHost && (proj == hostProjectNumber || proj == hostProjectID) {
			// Disabled in the project that carries the audit's quota: that is
			// a setup gap in the audit, not an absent product in the estate.
			r.v = vError
			r.errText = fmt.Sprintf("API DISABLED IN AUDIT HOST PROJECT: %s on %s — enable it there and re-run\n%s",
				api, hostProjectID, r.errText)
			return r
		}
		if ok {
			r.errText = fmt.Sprintf("%s not enabled on project %s\n%s", api, proj, r.errText)
		}
		if r.output == "" {
			r.v = vNA
			return r
		}
		// Several checks run more than one command (functions, then Cloud Run).
		// One product being absent must not discard what the others returned.
		if c.emptyPass {
			r.v = vFail
		} else {
			r.v = vReview
		}
		return r
	}

	switch {
	case strings.Contains(low, "permission_denied"), strings.Contains(low, "permission denied"),
		strings.Contains(low, "does not have permission"), strings.Contains(low, "caller does not have"):
		r.v = vDenied
	case err != nil && r.output == "":
		r.v = vError
		if r.errText == "" {
			// Typically a trailing `grep` or `[ … ] &&` that matched nothing.
			r.errText = fmt.Sprintf("%v, no output and nothing on stderr", err)
		}
	// bash runs without pipefail, so `gcloud … | jq …` exits 0 with empty
	// output when gcloud fails. Empty output is only a PASS if nothing in the
	// pipeline reported an error.
	case c.emptyPass && r.output == "" && reToolError.MatchString(r.errText):
		r.v = vError
	case c.evidence:
		r.v = vPass
	case c.emptyPass:
		if r.output == "" {
			r.v = vPass
		} else {
			r.v = vFail
		}
	default:
		r.v = vReview
	}
	return r
}

// why explains, in one sentence, how a check reached its verdict.
func why(r result) string {
	firstErr, _, _ := strings.Cut(strings.TrimSpace(r.errText), "\n")
	lines := 0
	if r.output != "" {
		lines = strings.Count(r.output, "\n") + 1
	}
	plural := "s"
	if lines == 1 {
		plural = ""
	}

	// A cross-reference has no command of its own; its verdict is inherited.
	if r.command == "" && r.byHand == "" && r.v != vXref {
		return "this requirement has no command of its own — it takes the verdict of the check it refers to (" + r.output + ")."
	}

	switch r.v {
	case vPass:
		if r.evidence {
			return "the command ran cleanly and its output is the evidence this requirement asks for; it is kept below."
		}
		return "the command ran cleanly and returned no output, which is the pass criterion."
	case vFail:
		switch {
		case len(r.absent) > 0:
			return "you declared " + strings.Join(r.absent, ", ") + " as `none`. A resource that doesn't exist can't meet the requirement, so this is a finding rather than a skip."
		case firstErr != "":
			return fmt.Sprintf("empty output would pass, and the command returned %d line%s — each names something that doesn't meet the criterion above. Part of the command couldn't run (see stderr): %s", lines, plural, firstErr)
		default:
			return fmt.Sprintf("empty output would pass, and the command returned %d line%s — each names something that doesn't meet the criterion above.", lines, plural)
		}
	case vReview:
		switch {
		case r.byHand != "":
			return "this check is never run automatically — it would log in to an instance, which a read-only audit must not do. Run the command by hand and judge the result."
		case firstErr != "":
			return "the command ran, but a machine can't score this criterion — compare the output with it. Part of the command couldn't run (see stderr): " + firstErr
		case r.output == "":
			return "the command ran cleanly and returned nothing, but a machine can't score this criterion — decide whether no output meets it."
		default:
			return fmt.Sprintf("the command ran cleanly and returned %d line%s, but a machine can't score this criterion — compare the output with it.", lines, plural)
		}
	case vNA:
		return firstErr + " — the product isn't in use here, so the requirement doesn't apply."
	case vSkip:
		if r.skipped {
			return "the operator excluded this check with -skip, so it was not run. Its requirement is unverified until it runs."
		}
		return firstErr + ". Add it to the `-config` file (or set it to `none` if it doesn't exist) and re-run."
	case vDenied:
		return "the audit identity is missing a permission, so this result can't be trusted: " + firstErr
	case vError:
		if firstErr == "" {
			return "the command failed without saying why."
		}
		return "the command failed: " + firstErr
	case vXref:
		return "this requirement is scored by another check that didn't run in this pass — see " + strings.Join(r.refs, ", ") + " in the other pass."
	}
	return ""
}

// excludePattern returns the EXCLUDE_PROJECTS regular expression — from -config
// or -set, else the environment, else ^sys- (the projects Apps Script creates,
// which belong in no audit). "none" turns exclusion off.
func excludePattern(subs map[string]string) (string, *regexp.Regexp) {
	pattern := subs["EXCLUDE_PROJECTS"]
	if pattern == "" {
		pattern = os.Getenv("EXCLUDE_PROJECTS")
	}
	if pattern == "" {
		pattern = "^sys-"
	}
	if strings.EqualFold(pattern, "none") {
		return pattern, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-run: EXCLUDE_PROJECTS %q is not a valid regular expression: %v\n", pattern, err)
		os.Exit(2)
	}
	return pattern, re
}

// cell renders the first line of s as a single markdown table cell. A raw
// multi-line value breaks the row, and a "|" or backtick breaks the table.
func cell(s string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if first == "" {
		return "—"
	}
	first = strings.ReplaceAll(first, "`", "'")
	return "`" + strings.ReplaceAll(first, "|", `\|`) + "`"
}

func tally(rs []result) map[verdict]int {
	t := map[verdict]int{}
	for _, r := range rs {
		t[r.v]++
	}
	return t
}

func trim(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + fmt.Sprintf("\n    … %d more lines", len(lines)-n)
}

func renderText(w *os.File, rs []result, showAll bool) {
	t := tally(rs)
	fmt.Fprintf(w, "\nCIS IG1 GCP — AUDIT RUN\n%s\n", strings.Repeat("=", 72))
	fmt.Fprintf(w, "Organization %s · %s\n\n", os.Getenv("ORG_ID"), time.Now().Format("2006-01-02 15:04"))

	fmt.Fprintf(w, "  PASS    %3d   compliant — tick it\n", t[vPass])
	fmt.Fprintf(w, "  FAIL    %3d   a finding — output names the offending resources\n", t[vFail])
	fmt.Fprintf(w, "  REVIEW  %3d   NEEDS A HUMAN — output saved, you decide\n", t[vReview])
	fmt.Fprintf(w, "  SKIP    %3d   needs a -set value (see below)\n", t[vSkip])
	fmt.Fprintf(w, "  N/A     %3d   API or product absent — not a failure\n", t[vNA])
	fmt.Fprintf(w, "  DENIED  %3d   missing permission — fix before trusting anything\n", t[vDenied])
	fmt.Fprintf(w, "  ERROR   %3d   command failed or timed out\n", t[vError])
	if t[vXref] > 0 {
		fmt.Fprintf(w, "  XREF    %3d   defers to another check that did not run\n", t[vXref])
	}
	fmt.Fprintln(w)

	// Turning SKIPs into a copy-pasteable command is the difference between
	// "46 skipped" and a next action.
	if t[vSkip] > 0 {
		need := map[string]int{}
		for _, r := range rs {
			if r.v == vSkip {
				for _, m := range r.missing {
					need[m]++
				}
			}
		}
		var keys []string
		for k := range need {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if need[keys[i]] != need[keys[j]] {
				return need[keys[i]] > need[keys[j]]
			}
			return keys[i] < keys[j]
		})
		plural := "CHECKS"
		if t[vSkip] == 1 {
			plural = "CHECK"
		}
		// Checks excluded with -skip need no value; only list real placeholders.
		if len(keys) == 0 {
			fmt.Fprintf(w, "%s\n%d %s NOT RUN — excluded with -skip\n\n", strings.Repeat("-", 72), t[vSkip], plural)
		} else {
			fmt.Fprintf(w, "%s\nTO RESOLVE THE %d SKIPPED %s\n\n", strings.Repeat("-", 72), t[vSkip], plural)
			for _, k := range keys {
				unit := "checks"
				if need[k] == 1 {
					unit = "check"
				}
				fmt.Fprintf(w, "  %-22s unblocks %d %s\n", k, need[k], unit)
			}
			fmt.Fprintf(w, "\n  go run audit-run.go \\\n")
			for i, k := range keys {
				cont := " \\"
				if i == len(keys)-1 {
					cont = ""
				}
				fmt.Fprintf(w, "    -set %s=<value>%s\n", k, cont)
			}
			fmt.Fprintln(w)
		}
	}

	if t[vDenied] > 0 {
		fmt.Fprintf(w, "%s\nPERMISSION PROBLEMS — fix these before trusting any result\n\n", strings.Repeat("-", 72))
		for _, r := range rs {
			if r.v == vDenied {
				fmt.Fprintf(w, "  %-6s %-9s %s\n         %s\n", r.id, r.ref, r.title, trim(r.errText, 2))
			}
		}
		fmt.Fprintln(w)
	}

	if t[vFail] > 0 {
		fmt.Fprintf(w, "%s\nFINDINGS\n\n", strings.Repeat("-", 72))
		for _, r := range rs {
			if r.v == vFail {
				fmt.Fprintf(w, "  %-6s %-9s %s\n", r.id, r.ref, r.title)
				for _, ln := range strings.Split(trim(r.output, 8), "\n") {
					fmt.Fprintf(w, "         %s\n", ln)
				}
				fmt.Fprintln(w)
			}
		}
	}

	if t[vReview] > 0 {
		fmt.Fprintf(w, "%s\nNEEDS A HUMAN — %d checks\n\n", strings.Repeat("-", 72), t[vReview])
		fmt.Fprintf(w, "  These ran cleanly. Judging the output is not something a machine\n")
		fmt.Fprintf(w, "  can do, so it is saved verbatim for you to adjudicate.\n\n")
		fmt.Fprintf(w, "  Run with -review review.md for the full worksheet.\n\n")
		for _, r := range rs {
			if r.v == vReview {
				lines := 0
				if r.output != "" {
					lines = len(strings.Split(r.output, "\n"))
				}
				fmt.Fprintf(w, "  %-6s %-9s %-46s %d line(s)\n", r.id, r.ref, truncate(r.title, 46), lines)
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\nALL CHECKS\n\n", strings.Repeat("-", 72))
	sg := ""
	for _, r := range rs {
		if r.v == vPass && !showAll {
			// still list it, just without body
		}
		if r.safeguard != sg {
			sg = r.safeguard
			fmt.Fprintf(w, "\n  %s\n", sg)
		}
		fmt.Fprintf(w, "    %-6s %-7s %-9s %s\n", r.id, r.v.label(), r.ref, r.title)
	}
	fmt.Fprintln(w)
}

// writeReviewSheet emits the REVIEW checks with their full output, the command
// that produced it, and the pass criteria — everything needed to adjudicate
// without going back to the terminal or the validation document. Output is
// never truncated here; the summary report truncates, this does not.
// writePack emits the three deliverables of an audit run:
//
//	01-automated-results.md  every check the script ran, as one table
//	02-manual-cli.md         GCP tasks needing a human at a terminal or console
//	03-manual-process.md     process and documentation requirements
//
// Splitting them matters because they are three different jobs, often for
// three different people, and bundling them produces a document nobody owns.
// auditTarget describes what this run covered, so a pack is self-identifying
// once there are dozens of them.
var auditTarget = "organization"

// auditScope is "org" or "project". The manual worksheets are generated from
// the checklist rather than from the estate, so they are identical in every
// pack — writing them per project would produce 105 copies of the same 72
// process requirements and imply they need answering project by project when
// they are organization-level.
var auditScope = "org"

// otherPass holds the checks that belong to the other pass. They are not run,
// but the pack lists them in V-number order alongside the checks that were, so
// a report accounts for every check from V1 to the last — a reviewer reporting
// on one project sees the organization checks it depends on, and where to find
// their results, rather than a numbering with gaps.
var otherPass []check

// otherPassLabel is what an otherPass check shows in place of a verdict: the
// name of the pass it belongs to. Deliberately not a verdict — rollup.go and
// run-audit.sh count only verdicts, so these rows never become findings.
func otherPassLabel() string {
	if auditScope == "project" {
		return "ORG"
	}
	return "PROJECT"
}

// otherPassWhere says where an otherPass check's result lives.
func otherPassWhere() string {
	if auditScope == "project" {
		return "Organization-scope check — it runs once, in the organization pass, not per project. " +
			"Its result is in the organization report (`report/02-organization/01-automated-results.md` in a `run-audit.sh` run)."
	}
	return "Project-scope check — it runs once per project, in the project pass. " +
		"Its results are in each project's report (`report/03-projects/<project-id>.md` in a `run-audit.sh` run)."
}

func writePack(dir string, rs []result, manual []manualItem) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if runStamp == "" {
		runStamp = time.Now().Format("2006-01-02 15:04")
	}
	stamp := runStamp
	org := os.Getenv("ORG_ID")

	// ---------- 01 automated ----------
	if err := os.WriteFile(dir+"/01-automated-results.md", []byte(renderAutomated(rs)), 0o644); err != nil {
		return err
	}
	// The machine-readable record behind that file: what -decide reads to walk
	// the auditor through the REVIEW checks and rebuild the report afterwards.
	if err := saveState(dir+"/results.json", newState(rs)); err != nil {
		return err
	}

	// The manual worksheets are organization-level. Skip them on project runs.
	if auditScope == "project" {
		fmt.Fprintf(os.Stderr, "\naudit pack written to %s/\n", dir)
		fmt.Fprintf(os.Stderr, "  01-automated-results.md   %d checks (+%d organization checks listed)\n", len(rs), len(otherPass))
		fmt.Fprintf(os.Stderr, "  (manual worksheets are organization-level — see the org pack)\n")
		return nil
	}

	// ---------- 02 manual CLI / GCP ----------
	var b strings.Builder
	var gcp, proc []manualItem
	for _, m := range manual {
		if m.gcpTask {
			gcp = append(gcp, m)
		} else {
			proc = append(proc, m)
		}
	}

	fmt.Fprintf(&b, "# 2. Manual — GCP Tasks\n\nOrganization `%s` · %s\n\n", org, stamp)
	fmt.Fprintf(&b, "**%d requirements.** These concern GCP configuration but have no single-command CLI check: ", len(gcp))
	b.WriteString("Admin Console settings, image build properties, or a test that has to be performed rather than queried.\n\n")
	b.WriteString("Work them one at a time and record the result. Where a console is involved, note where you looked.\n\n---\n\n")
	writeManualTable(&b, gcp)
	if err := os.WriteFile(dir+"/02-manual-cli.md", []byte(b.String()), 0o644); err != nil {
		return err
	}

	// ---------- 03 manual process ----------
	var c strings.Builder
	fmt.Fprintf(&c, "# 3. Manual — Process and Documentation\n\nOrganization `%s` · %s\n\n", org, stamp)
	fmt.Fprintf(&c, "**%d requirements.** None concern infrastructure *state*, but every one concerns the GCP estate — ", len(proc))
	c.WriteString("the data management process for GCP data, the audit log process for GCP logs. ")
	c.WriteString("They are ours; they are satisfied by a written, owned, dated document rather than by configuration.\n\n")
	c.WriteString("> The most common audit failure here is a missing **review date**, not a missing document. ")
	c.WriteString("A process nobody has reviewed cannot be shown to be current.\n\n---\n\n")
	writeManualTable(&c, proc)
	if err := os.WriteFile(dir+"/03-manual-process.md", []byte(c.String()), 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\naudit pack written to %s/\n", dir)
	fmt.Fprintf(os.Stderr, "  01-automated-results.md   %d checks (+%d project checks listed)\n", len(rs), len(otherPass))
	fmt.Fprintf(os.Stderr, "  02-manual-cli.md          %d GCP tasks\n", len(gcp))
	fmt.Fprintf(os.Stderr, "  03-manual-process.md      %d process requirements\n", len(proc))
	return nil
}

// renderAutomated builds 01-automated-results.md. It is called once when the
// pass runs, and again by -decide each time the auditor's decisions change.
func renderAutomated(rs []result) string {
	org := os.Getenv("ORG_ID")
	target := auditTarget
	stamp := runStamp
	var a strings.Builder
	t := tally(rs)
	fmt.Fprintf(&a, "# 1. Automated Results\n\nOrganization `%s` · scope: **%s** · %s\n\n", org, target, stamp)
	fmt.Fprintf(&a, "%d checks run by `audit-run.go`.", len(rs))
	if len(otherPass) > 0 {
		pass := "organization"
		if auditScope == "org" {
			pass = "project"
		}
		fmt.Fprintf(&a, " The other %d belong to the %s pass: they are listed in order below as **%s**, "+
			"not run here and not counted in the verdicts.", len(otherPass), pass, otherPassLabel())
	}
	a.WriteString("\n\n")

	// A run that mostly errored produces a pack full of ERROR verdicts that
	// looks superficially like a set of findings. State the run's health up
	// front so a rollup — and a reader — can tell the two apart.
	broken := t[vDenied] + t[vError]
	pct := 0
	if len(rs) > 0 {
		pct = broken * 100 / len(rs)
	}
	switch {
	case t[vDenied] > 0:
		fmt.Fprintf(&a, "> **RUN STATUS: UNRELIABLE** — %d check(s) returned DENIED. "+
			"The audit identity is missing permissions, so passes cannot be trusted: "+
			"a permission gap converts silently into a false PASS on some checks. "+
			"Fix the grants and re-run before using these results.\n\n", t[vDenied])
	case pct >= 20:
		fmt.Fprintf(&a, "> **RUN STATUS: DEGRADED** — %d%% of checks errored. "+
			"Treat this pack as incomplete rather than as findings.\n\n", pct)
	case broken > 0:
		fmt.Fprintf(&a, "> **RUN STATUS: OK with %d error(s)** — see the Problems table.\n\n", broken)
	default:
		fmt.Fprintf(&a, "> **RUN STATUS: OK** — every check executed.\n\n")
	}
	writeScore(&a, rs)
	fmt.Fprintf(&a, "| Verdict | Count | Meaning |\n|---|---|---|\n")
	fmt.Fprintf(&a, "| PASS | %d | Compliant — tick the checklist |\n", t[vPass])
	fmt.Fprintf(&a, "| FAIL | %d | A finding — output below |\n", t[vFail])
	fmt.Fprintf(&a, "| REVIEW | %d | Ran clean; a human must judge the output |\n", t[vReview])
	fmt.Fprintf(&a, "| SKIP | %d | Placeholder not supplied |\n", t[vSkip])
	fmt.Fprintf(&a, "| N/A | %d | API or product absent |\n", t[vNA])
	fmt.Fprintf(&a, "| DENIED | %d | Missing permission |\n", t[vDenied])
	fmt.Fprintf(&a, "| ERROR | %d | Command failed |\n\n", t[vError])

	// Every check in V-number order: this pass's results interleaved with the
	// other pass's checks, so the numbering has no gaps.
	type entry struct {
		r   *result
		c   check
		ran bool
	}
	var all []entry
	for i := range rs {
		all = append(all, entry{r: &rs[i], c: rs[i].check, ran: true})
	}
	for _, c := range otherPass {
		all = append(all, entry{c: c})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].c.num < all[j].c.num })

	a.WriteString("## Results\n\n| Check | Requirement | Verdict | Title |\n|---|---|---|---|\n")
	for _, e := range all {
		label := otherPassLabel()
		if e.ran {
			label = e.r.v.label()
		}
		fmt.Fprintf(&a, "| %s | `%s` | **%s** | %s |\n", e.c.id, e.c.ref, label, e.c.title)
	}
	a.WriteString("\n")

	if t[vDenied]+t[vError] > 0 {
		a.WriteString("## Problems\n\n| Check | Verdict | Detail |\n|---|---|---|\n")
		for _, r := range rs {
			if r.v == vDenied || r.v == vError {
				fmt.Fprintf(&a, "| %s | %s | %s |\n", r.id, r.v.label(), cell(r.errText))
			}
		}
		a.WriteString("\n")
	}
	// N/A is "not a failure" only if the reason really is an unused product.
	// Without the reason, a reader can't tell that from a setup gap.
	if t[vNA] > 0 {
		a.WriteString("## Not applicable\n\n| Check | Reason |\n|---|---|\n")
		for _, r := range rs {
			if r.v == vNA {
				fmt.Fprintf(&a, "| %s | %s |\n", r.id, cell(r.errText))
			}
		}
		a.WriteString("\n")
	}

	// Every check, in V-number order, with its verdict and the reason for it,
	// so the pack reads as the evidence record rather than a list of failures.
	// Nothing here may look like a Results table row: rollup.go parses those.
	a.WriteString("## Detail\n\nEvery check in V-number order: its verdict, the pass criterion, why it got that verdict, and the output. ")
	if len(otherPass) > 0 {
		fmt.Fprintf(&a, "Checks marked **%s** belong to the other pass: they show what would be checked and where the result is.", otherPassLabel())
	}
	a.WriteString("\n\n")
	for _, e := range all {
		if !e.ran {
			c := e.c
			fmt.Fprintf(&a, "### %s — %s\n\n", c.id, c.title)
			fmt.Fprintf(&a, "**%s** · `%s`\n\n", otherPassLabel(), c.ref)
			if c.criteria != "" {
				fmt.Fprintf(&a, "**Pass if:** %s\n\n", c.criteria)
			}
			fmt.Fprintf(&a, "**Why %s:** %s\n\n", otherPassLabel(), otherPassWhere())
			switch {
			case c.byHand != "":
				fmt.Fprintf(&a, "<details><summary>command (run by hand)</summary>\n\n```bash\n%s\n```\n\n</details>\n\n", c.byHand)
			case c.command != "":
				fmt.Fprintf(&a, "<details><summary>command</summary>\n\n```bash\n%s\n```\n\n</details>\n\n", c.command)
			case len(c.refs) > 0:
				fmt.Fprintf(&a, "Cross-reference — takes its result from %s.\n\n", strings.Join(c.refs, ", "))
			}
			continue
		}
		r := *e.r
		fmt.Fprintf(&a, "### %s — %s\n\n", r.id, r.title)
		fmt.Fprintf(&a, "**%s** · `%s`\n\n", r.v.label(), r.ref)
		if r.criteria != "" {
			fmt.Fprintf(&a, "**Pass if:** %s\n\n", r.criteria)
		}
		if r.reviewedBy != "" {
			machine := r
			machine.v = vReview
			fmt.Fprintf(&a, "**Why %s:** decided by the auditor on review — %s, %s. The machine's verdict was REVIEW: %s\n\n",
				r.v.label(), r.reviewedBy, r.reviewedAt, why(machine))
		} else {
			fmt.Fprintf(&a, "**Why %s:** %s\n\n", r.v.label(), why(r))
		}
		body := r.output
		if body == "" {
			body = "(no output)"
		}
		fmt.Fprintf(&a, "```\n%s\n```\n\n", body)
		if r.errText != "" && r.errText != r.output {
			fmt.Fprintf(&a, "<details><summary>stderr</summary>\n\n```\n%s\n```\n\n</details>\n\n", r.errText)
		}
		if r.command != "" {
			fmt.Fprintf(&a, "<details><summary>command</summary>\n\n```bash\n%s\n```\n\n</details>\n\n", r.command)
		}
	}
	return a.String()
}

func writeManualTable(w *strings.Builder, items []manualItem) {
	sg := ""
	for _, m := range items {
		if m.safeguard != sg {
			if sg != "" {
				w.WriteString("\n")
			}
			sg = m.safeguard
			fmt.Fprintf(w, "## %s %s\n\n", m.safeguard, m.sgTitle)
			w.WriteString("| | Requirement | Result | Evidence / notes |\n|---|---|---|---|\n")
		}
		fmt.Fprintf(w, "| `%s` | %s | ☐ Compliant ☐ Not compliant | |\n", m.ref, m.title)
	}
	w.WriteString("\n")
}

func writeReviewSheet(path string, rs []result) error {
	var b strings.Builder

	var review []result
	for _, r := range rs {
		if r.v == vReview {
			review = append(review, r)
		}
	}

	b.WriteString("# Review Worksheet\n\n")
	fmt.Fprintf(&b, "Organization `%s` · %s\n\n", os.Getenv("ORG_ID"), time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "**%d checks ran cleanly but need a human to decide.** ", len(review))
	b.WriteString("The command executed, the output is below, and the pass criteria is stated. ")
	b.WriteString("No machine can judge these — that is why they are here rather than scored.\n\n")
	b.WriteString("Tick each box once adjudicated, then reflect the result in `docs/cis-ig1-gcp-checklist.md`.\n\n---\n\n")

	sg := ""
	for _, r := range review {
		if r.safeguard != sg {
			sg = r.safeguard
			fmt.Fprintf(&b, "## Safeguard %s\n\n", sg)
		}
		fmt.Fprintf(&b, "### %s — %s\n\n", r.id, r.title)
		fmt.Fprintf(&b, "- [ ] Adjudicated · checklist requirement `%s`\n\n", r.ref)
		fmt.Fprintf(&b, "**Pass if:** %s\n\n", r.criteria)

		body := r.output
		if body == "" {
			body = "(no output)"
		}
		fmt.Fprintf(&b, "```\n%s\n```\n\n", body)

		if r.errText != "" {
			fmt.Fprintf(&b, "<details><summary>stderr</summary>\n\n```\n%s\n```\n\n</details>\n\n", r.errText)
		}
		fmt.Fprintf(&b, "<details><summary>command</summary>\n\n```bash\n%s\n```\n\n</details>\n\n---\n\n", r.command)
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func renderMD(w *os.File, rs []result, showAll bool) {
	t := tally(rs)
	fmt.Fprintf(w, "# CIS IG1 GCP — Audit Run\n\n")
	fmt.Fprintf(w, "Organization `%s` · %s\n\n", os.Getenv("ORG_ID"), time.Now().Format("2006-01-02"))
	fmt.Fprintf(w, "| Verdict | Count |\n|---|---|\n")
	for _, v := range []verdict{vPass, vFail, vReview, vSkip, vNA, vDenied, vError, vXref} {
		fmt.Fprintf(w, "| %s | %d |\n", v.label(), t[v])
	}
	fmt.Fprintln(w)

	if t[vFail] > 0 {
		fmt.Fprintf(w, "## Findings\n\n")
		for _, r := range rs {
			if r.v == vFail {
				fmt.Fprintf(w, "### %s · `%s` %s\n\n```\n%s\n```\n\n", r.id, r.ref, r.title, trim(r.output, 12))
			}
		}
	}
	if t[vDenied] > 0 {
		fmt.Fprintf(w, "## Permission problems\n\n")
		for _, r := range rs {
			if r.v == vDenied {
				fmt.Fprintf(w, "- **%s** `%s` %s — `%s`\n", r.id, r.ref, r.title, trim(r.errText, 1))
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "## All checks\n\n| Check | Verdict | Requirement | Title |\n|---|---|---|---|\n")
	for _, r := range rs {
		fmt.Fprintf(w, "| %s | %s | `%s` | %s |\n", r.id, r.v.label(), r.ref, r.title)
	}
}

func loadConfig(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, ln := range strings.Split(string(raw), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if k, v, ok := strings.Cut(ln, "="); ok {
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			if v != "" {
				out[strings.TrimSpace(k)] = v
			}
		}
	}
	return out, nil
}

// discoverHint tells the operator what each placeholder is and, where useful,
// the command that finds it. Prompting for "POLICY_ID" with no context is not
// meaningfully better than skipping.
// placeholderDefault supplies a value for placeholders whose answer is the
// same for nearly every engagement. The operator can still override it in the
// config; leaving it out no longer costs the check. A default here must be the
// safe, common case — a wrong default produces confident findings, which is
// worse than a SKIP.
var placeholderDefault = map[string]string{
	// Continental United States: the regions plus the "US" multi-region, which
	// is what a bucket or dataset reports when it is not in a single region.
	"ALLOWED_LOCATIONS": "us,US,us-central1,us-east1,us-east4,us-east5,us-south1,us-west1,us-west2,us-west3,us-west4",
}

var discoverHint = map[string]string{
	"SECURITY_PROJECT":   "project carrying audit API quota",
	"PROJECT_ID":         "main workload project — gcloud projects list",
	"DOMAIN":             "Cloud Identity primary domain, e.g. example.com",
	"LOG_BUCKET":         "GCS bucket holding exported audit logs",
	"INSTANCE":           "a Cloud SQL instance — gcloud sql instances list",
	"CLUSTER":            "a GKE cluster — gcloud container clusters list",
	"REGION":             "region for the resource being checked",
	"ROUTER":             "Cloud Router — gcloud compute routers list",
	"POLICY_ID":          "Access Context Manager policy — gcloud access-context-manager policies list",
	"BILLING_ACCOUNT_ID": "gcloud billing accounts list",
	"RING":               "KMS key ring — gcloud kms keyrings list",
	"KEY":                "KMS key — gcloud kms keys list",
	"LOCATION":           "KMS key location",
	"BUCKET":             "any bucket to sample for configuration",

	// Audit prerequisites: values that turn a judgement call into a PASS/FAIL.
	// Comma-separated lists, no spaces.
	"APPROVED_REGISTRIES": "approved container registry prefixes, comma-separated — e.g. us-docker.pkg.dev/acme,gcr.io/acme,gke.gcr.io",
	"ALLOWED_LOCATIONS":   "locations data may live in, comma-separated — defaults to the continental US; set it only if data lives elsewhere",
}

func neededPlaceholders(checks []check) []string {
	need := map[string]int{}
	for _, c := range checks {
		for _, m := range c.missing {
			need[m]++
		}
	}
	var keys []string
	for k := range need {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if need[keys[i]] != need[keys[j]] {
			return need[keys[i]] > need[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func writeConfigTemplate(path string, checks []check) error {
	keys := neededPlaceholders(checks)
	count := map[string]int{}
	for _, c := range checks {
		for _, m := range c.missing {
			count[m]++
		}
	}

	var b strings.Builder
	b.WriteString("# CIS IG1 audit — settings\n")
	b.WriteString("# One file per engagement. Fill it in before the run so nothing is SKIPped.\n#\n")
	b.WriteString("# These are policy answers, not facts about the estate — ask the customer\n")
	b.WriteString("# rather than inferring them from what happens to be deployed. If a value\n")
	b.WriteString("# genuinely does not exist, set it to\n")
	b.WriteString("#   none\n")
	b.WriteString("# and the checks that need it are recorded as FINDINGS, not skipped.\n#\n")
	b.WriteString("# ALLOWED_LOCATIONS is not listed unless you need it: it defaults to the\n")
	b.WriteString("# continental United States. Set it only if data legitimately lives\n")
	b.WriteString("# elsewhere, and give the full list — the default is replaced, not extended.\n\n")
	b.WriteString("# --- The organization -------------------------------------------------\n")
	b.WriteString("# Set these once and no command needs a path or an ID again.\n\n")
	b.WriteString("# The organization to audit. Find it with:\n")
	b.WriteString("#   gcloud organizations list\n")
	fmt.Fprintf(&b, "ORG_ID=%s\n\n", os.Getenv("ORG_ID"))
	b.WriteString("# The project that owns the audit service account and has the audit APIs\n")
	b.WriteString("# enabled. Used by the setup steps, not by the audit itself.\n")
	b.WriteString("AUDIT_PROJECT=\n\n")
	b.WriteString("# The audit service account the auditor impersonates. Used by the setup\n")
	b.WriteString("# steps, not by the audit itself.\n")
	b.WriteString("SA_EMAIL=\n\n")
	b.WriteString("# --- Check inputs ------------------------------------------------------\n\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "# %s (%d check(s))\n%s=\n\n", discoverHint[k], count[k], k)
	}
	b.WriteString("# Projects never audited — a regular expression matched against the project ID.\n")
	b.WriteString("# ^sys- skips the projects Apps Script creates. Set to none to audit everything.\n")
	b.WriteString("EXCLUDE_PROJECTS=^sys-\n")
	// config/ does not exist on a fresh clone until somebody copies an example
	// into it.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s — %d placeholder(s)\n\nFill it in, then run the audit:\n"+
		"  ./run-audit.sh --project PROJECT_ID\n", path, len(keys))
	return nil
}

// promptForMissing asks for every unresolved placeholder up front, before any
// check runs. Answering "none" marks the dependent checks as findings rather
// than skips, because a resource that does not exist is a compliance result,
// not an absence of information.
func promptForMissing(checks []check, subs map[string]string) []check {
	keys := neededPlaceholders(checks)
	if len(keys) == 0 {
		return checks
	}

	fi, _ := os.Stdin.Stat()
	if fi != nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		return checks // not a terminal; leave them as SKIP
	}

	count := map[string]int{}
	for _, c := range checks {
		for _, m := range c.missing {
			count[m]++
		}
	}

	fmt.Fprintf(os.Stderr, "%d placeholder(s) are needed before the run.\n", len(keys))
	fmt.Fprintf(os.Stderr, "Enter a value, or 'none' if it does not exist in this organization.\n")
	fmt.Fprintf(os.Stderr, "('none' records the dependent checks as findings, not skips.)\n\n")

	sc := bufio.NewScanner(os.Stdin)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %s — %s\n  %s (%d check(s)) > ", k, discoverHint[k], k, count[k])
		if !sc.Scan() {
			break
		}
		v := strings.TrimSpace(sc.Text())
		if v == "" {
			fmt.Fprintf(os.Stderr, "    left unset — %d check(s) will be SKIP\n\n", count[k])
			continue
		}
		subs[k] = v
		fmt.Fprintln(os.Stderr)
	}
	fmt.Fprintln(os.Stderr)

	// Re-substitute with the answers now in hand.
	for i := range checks {
		if checks[i].command == "" {
			continue
		}
		checks[i].command, checks[i].missing, checks[i].absent = substitute(checks[i].rawCommand, subs)
	}
	return checks
}

// ---------------------------------------------------------------------------
// Score and review
// ---------------------------------------------------------------------------

// runStamp is when the pass ran. A report rebuilt by -decide keeps it, so the
// file still says when the evidence was gathered, not when it was judged.
var runStamp string

// scoreOf reduces a pass to the three numbers a project is reported on.
//
// Every check ends as pass, fail, or not yet final. N/A is a pass: the product
// is not enabled, so there is nothing in it to fail. REVIEW is not final until
// the auditor decides it, and SKIP, ERROR and DENIED are checks that did not
// get an answer at all. Completion is the share that is final; it reaches
// 100% only when every check is PASS or FAIL.
type score struct {
	total, pass, fail           int
	passRan, passNA, passReview int
	failRan, failReview         int
	undecided, blocked          int
}

func scoreOf(rs []result) score {
	var s score
	s.total = len(rs)
	for _, r := range rs {
		switch r.v {
		case vPass:
			s.pass++
			if r.reviewedBy != "" {
				s.passReview++
			} else {
				s.passRan++
			}
		case vNA:
			s.pass++
			s.passNA++
		case vFail:
			s.fail++
			if r.reviewedBy != "" {
				s.failReview++
			} else {
				s.failRan++
			}
		case vReview:
			s.undecided++
		default:
			s.blocked++
		}
	}
	return s
}

// pct rounds down, so a pass that is one check short never reads as 100%.
func pct(n, of int) int {
	if of == 0 {
		return 0
	}
	return n * 100 / of
}

// percents gives the three numbers. Completion and Pass round down — neither
// may overstate — and Fail is the remainder of the completed share, so Pass +
// Fail always equals Completion instead of drifting a point short.
func (s score) percents() (completion, pass, fail int) {
	completion = pct(s.pass+s.fail, s.total)
	pass = pct(s.pass, s.total)
	return completion, pass, completion - pass
}

func (s score) line() string {
	c, p, f := s.percents()
	return fmt.Sprintf("completion %d%% · pass %d%% · fail %d%%", c, p, f)
}

func writeScore(a *strings.Builder, rs []result) {
	s := scoreOf(rs)
	// run-audit.sh reads this line for its summary.
	fmt.Fprintf(a, "> **SCORE: %s**\n\n", s.line())
	a.WriteString("| | | |\n|---|---|---|\n")
	done := fmt.Sprintf("%d of %d checks are final — PASS or FAIL.", s.pass+s.fail, s.total)
	if s.undecided > 0 {
		done += fmt.Sprintf(" **%d REVIEW** await the auditor: `./run-audit.sh --review <run directory>`.", s.undecided)
	}
	if s.blocked > 0 {
		done += fmt.Sprintf(" **%d** did not run (SKIP, ERROR or DENIED) — fix and re-run.", s.blocked)
	}
	c, p, f := s.percents()
	fmt.Fprintf(a, "| **Completion** | **%d%%** | %s |\n", c, done)
	fmt.Fprintf(a, "| **Pass** | **%d%%** | %d checks: %d PASS, %d N/A (product not enabled, nothing to fail), %d judged PASS on review |\n",
		p, s.pass, s.passRan, s.passNA, s.passReview)
	fmt.Fprintf(a, "| **Fail** | **%d%%** | %d checks: %d FAIL, %d judged FAIL on review |\n\n",
		f, s.fail, s.failRan, s.failReview)
	a.WriteString("Percentages are of every check in this pass. Pass rounds down; Fail is the rest of the completed share, so Pass + Fail = Completion.\n\n")
}

// The saved state is everything the report is built from, so -decide can
// rebuild it without re-running a single check. JSON from the standard
// library, like everything else here.
type savedCheck struct {
	ID, Title, Ref, Safeguard, Scope, Command, Criteria, ByHand string
	Num                                                         int
	EmptyPass, Evidence, Skipped                                bool
	Refs, Missing, Absent                                       []string
}

type savedResult struct {
	savedCheck
	Verdict string // the machine's verdict; never overwritten by a decision
	Output  string
	ErrText string
}

type decision struct {
	Verdict string // PASS or FAIL
	By      string
	At      string
}

type runState struct {
	Org, Target, Scope, Stamp string
	Results                   []savedResult
	OtherPass                 []savedCheck
	Decisions                 map[string]decision
}

func toSaved(c check) savedCheck {
	return savedCheck{ID: c.id, Title: c.title, Ref: c.ref, Safeguard: c.safeguard, Scope: c.scope,
		Command: c.command, Criteria: c.criteria, ByHand: c.byHand, Num: c.num,
		EmptyPass: c.emptyPass, Evidence: c.evidence, Skipped: c.skipped,
		Refs: c.refs, Missing: c.missing, Absent: c.absent}
}

func fromSaved(c savedCheck) check {
	return check{id: c.ID, title: c.Title, ref: c.Ref, safeguard: c.Safeguard, scope: c.Scope,
		command: c.Command, criteria: c.Criteria, byHand: c.ByHand, num: c.Num,
		emptyPass: c.EmptyPass, evidence: c.Evidence, skipped: c.Skipped,
		refs: c.Refs, missing: c.Missing, absent: c.Absent}
}

func verdictFromLabel(l string) verdict {
	for v := vPass; v <= vXref; v++ {
		if v.label() == l {
			return v
		}
	}
	return vError
}

func newState(rs []result) runState {
	st := runState{Org: os.Getenv("ORG_ID"), Target: auditTarget, Scope: auditScope, Stamp: runStamp,
		Decisions: map[string]decision{}}
	for _, r := range rs {
		st.Results = append(st.Results, savedResult{savedCheck: toSaved(r.check),
			Verdict: r.v.label(), Output: r.output, ErrText: r.errText})
	}
	for _, c := range otherPass {
		st.OtherPass = append(st.OtherPass, toSaved(c))
	}
	return st
}

// saveState writes through a temporary file and a rename, so a session killed
// mid-write leaves the previous state rather than half a file.
func saveState(path string, st runState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadState(path string) (runState, error) {
	var st runState
	b, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, fmt.Errorf("%s: %v", path, err)
	}
	if st.Decisions == nil {
		st.Decisions = map[string]decision{}
	}
	return st, nil
}

func isXref(c check) bool { return c.command == "" && c.byHand == "" && len(c.refs) > 0 }

// applyDecisions turns the saved machine results plus the auditor's decisions
// into the results the report shows. A decision only ever replaces a REVIEW.
// Cross-references are re-derived from the decided checks they point at, so
// deciding V37 also settles V41; a cross-reference left REVIEW after that —
// one that needs a check from the other pass — is decided in its own right.
func applyDecisions(st runState) []result {
	var rs []result
	for _, sr := range st.Results {
		r := result{check: fromSaved(sr.savedCheck), v: verdictFromLabel(sr.Verdict),
			output: sr.Output, errText: sr.ErrText}
		if isXref(r.check) {
			r.v, r.output, r.errText = vXref, "", ""
		} else if d, ok := st.Decisions[r.id]; ok && r.v == vReview {
			r.v, r.reviewedBy, r.reviewedAt = verdictFromLabel(d.Verdict), d.By, d.At
		}
		rs = append(rs, r)
	}
	resolveXrefs(rs)
	for i := range rs {
		if d, ok := st.Decisions[rs[i].id]; ok && isXref(rs[i].check) && rs[i].v == vReview {
			rs[i].v, rs[i].reviewedBy, rs[i].reviewedAt = verdictFromLabel(d.Verdict), d.By, d.At
		}
	}
	return rs
}

// nextUndecided returns the next REVIEW to put to the auditor: checks with a
// command of their own first, in V order, then any cross-reference still
// REVIEW once those are settled.
func nextUndecided(rs []result, passed map[string]bool) *result {
	for _, xref := range []bool{false, true} {
		for i := range rs {
			if rs[i].v == vReview && isXref(rs[i].check) == xref && !passed[rs[i].id] {
				return &rs[i]
			}
		}
	}
	return nil
}

// auditorIdentity is the human making the call — the signed-in account, not
// the service account being impersonated.
func auditorIdentity() string {
	if out, err := exec.Command("gcloud", "config", "get-value", "account").Output(); err == nil {
		if a := strings.TrimSpace(string(out)); a != "" {
			return a
		}
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}

const reviewPreviewLines = 30

// runDecide walks the auditor through every REVIEW check in a saved pass,
// one at a time, waiting for PASS or FAIL. Each answer is saved the moment it
// is given, so quitting — or a lapsed sign-in, or a closed laptop — loses
// nothing; running it again resumes at the first undecided check.
func runDecide(statePath, mdPath string) int {
	st, err := loadState(statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
		return 2
	}
	os.Setenv("ORG_ID", st.Org)
	auditTarget, auditScope, runStamp = st.Target, st.Scope, st.Stamp
	otherPass = nil
	for _, c := range st.OtherPass {
		otherPass = append(otherPass, fromSaved(c))
	}
	if mdPath == "" {
		mdPath = filepath.Join(filepath.Dir(statePath), "01-automated-results.md")
	}

	who := auditorIdentity()
	passed := map[string]bool{} // skipped for now, this session only
	rs := applyDecisions(st)

	fmt.Printf("\nReview — %s · %d undecided\n", st.Target, scoreOf(rs).undecided)
	fmt.Printf("Decided by %s. Answers are saved as you go; q quits, and running this again resumes.\n", who)

	for {
		r := nextUndecided(rs, passed)
		if r == nil {
			break
		}
		fmt.Printf("\n%s\n", strings.Repeat("═", 78))
		fmt.Printf("%s  REVIEW  %s  %s      [%d left]\n", r.id, r.ref, r.title, scoreOf(rs).undecided-len(passed))
		fmt.Printf("Pass if: %s\n", r.criteria)
		fmt.Printf("%s\n", strings.Repeat("─", 78))
		body := r.output
		if body == "" {
			body = "(no output)"
		}
		lines := strings.Split(body, "\n")
		if len(lines) > reviewPreviewLines {
			fmt.Println(strings.Join(lines[:reviewPreviewLines], "\n"))
			fmt.Printf("… %d more lines — v to view all\n", len(lines)-reviewPreviewLines)
		} else {
			fmt.Println(body)
		}
		if r.byHand != "" {
			fmt.Printf("%s\nRun by hand:\n%s\n", strings.Repeat("─", 78), r.byHand)
		}
		fmt.Printf("%s\n", strings.Repeat("─", 78))

		for answered := false; !answered; {
			fmt.Print("[p]ass  [f]ail  [s]kip for now  [v]iew full output  [q]uit and save > ")
			line, rerr := readAnswer()
			choice := strings.ToLower(strings.TrimSpace(line))
			if rerr != nil && choice == "" {
				choice = "q" // end of input: stop cleanly rather than loop
			}
			switch choice {
			case "p", "f":
				v := "PASS"
				if choice == "f" {
					v = "FAIL"
				}
				st.Decisions[r.id] = decision{Verdict: v, By: who, At: time.Now().Format("2006-01-02 15:04")}
				if err := saveState(statePath, st); err != nil {
					fmt.Fprintf(os.Stderr, "audit-run: could not save the decision: %v\n", err)
					return 2
				}
				rs = applyDecisions(st)
				answered = true
			case "s":
				passed[r.id] = true
				answered = true
			case "v":
				showFull(body)
			case "q":
				// A quit ends the whole session, not just this pass. The exit
				// code can't say so — `go run` reports every non-zero exit as
				// 1 — so leave a marker beside the state for run-audit.sh.
				if rc := finishDecide(statePath, mdPath, rs); rc != 0 {
					return rc
				}
				if err := os.WriteFile(statePath+".quit", nil, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
					return 2
				}
				return 0
			}
		}
	}
	return finishDecide(statePath, mdPath, rs)
}

// readAnswer reads one line from stdin a byte at a time. A buffered reader
// would read ahead past this line, taking answers that belong to the next
// pass's review, which runs as a separate process on the same input.
func readAnswer() (string, error) {
	var b []byte
	one := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(one)
		if n == 1 {
			if one[0] == '\n' {
				return string(b), nil
			}
			b = append(b, one[0])
		}
		if err != nil {
			return string(b), err
		}
	}
}

// showFull pages long output through less where it exists, else prints it.
func showFull(body string) {
	if path, err := exec.LookPath("less"); err == nil {
		cmd := exec.Command(path, "-R")
		cmd.Stdin = strings.NewReader(body)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if cmd.Run() == nil {
			return
		}
	}
	fmt.Println(body)
}

func finishDecide(statePath, mdPath string, rs []result) int {
	if err := os.WriteFile(mdPath, []byte(renderAutomated(rs)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "audit-run: %v\n", err)
		return 2
	}
	s := scoreOf(rs)
	fmt.Printf("\n%s — %s\n", auditTarget, s.line())
	if s.undecided > 0 {
		fmt.Printf("%d REVIEW still undecided. Run the review again to continue.\n", s.undecided)
	}
	fmt.Printf("Report: %s\n", mdPath)
	return 0
}
