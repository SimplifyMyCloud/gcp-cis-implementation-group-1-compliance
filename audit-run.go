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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	emptyPass  bool
	rawCommand string   // pre-substitution, so answers can be re-applied
	missing    []string // unresolved placeholders
	absent     []string // placeholders the operator declared non-existent
	refs       []string // for xrefs: the checks this one defers to
	byHand     string   // command the auditor runs manually (```sh block); never executed
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
}

var (
	reCheck    = regexp.MustCompile(`(?m)^#### (V\d+)\s*$`)
	reTitle    = regexp.MustCompile(`\*\*(.+?)\*\* · checklist ` + "`" + `([\d]+\.[\d]+#[\d]+)` + "`")
	reScope    = regexp.MustCompile(`· scope: (org|project|xref)`)
	reBash     = regexp.MustCompile("(?s)```bash\n(.*?)\n```")
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
var notPlaceholders = map[string]bool{
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
		list      = flag.Bool("list", false, "list checks without running them")
		parallel  = flag.Int("parallel", 8, "concurrent gcloud invocations")
		timeout   = flag.Duration("timeout", 3*time.Minute, "per-check timeout")
		format    = flag.String("format", "text", "text | md")
		outPath   = flag.String("out", "", "write report to a file instead of stdout")
		showAll   = flag.Bool("all", false, "include PASS entries in detailed output")
		quiet     = flag.Bool("quiet", false, "suppress the live per-check stream")
		reviewOut = flag.String("review", "", "write the REVIEW worksheet to a file (full output, nothing truncated)")
		pack      = flag.String("pack", "", "write the full audit pack (automated table + both manual worksheets) to a directory")
		config    = flag.String("config", "", "read placeholder values from a KEY=VALUE file")
		scope     = flag.String("scope", "", "which pass to run: org | project (required)")
		org       = flag.String("org", "", "organization ID to audit (defaults to $ORG_ID)")
		project   = flag.String("project", "", "the project to audit, for -scope=project")
		initCfg   = flag.String("init-config", "", "write a starter config listing every placeholder this run needs, then exit")
		noPrompt  = flag.Bool("no-prompt", false, "never prompt; leave unresolved placeholders as SKIP (for CI)")
	)
	var sets multiFlag
	flag.Var(&sets, "set", "placeholder substitution, repeatable (-set PROJECT_ID=foo)")
	flag.Parse()

	subs := map[string]string{}
	for _, kv := range sets {
		if k, v, ok := strings.Cut(kv, "="); ok {
			subs[k] = v
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

	// -org wins over the environment; the commands themselves read $ORG_ID,
	// so set it either way. A flag beside -project is less surprising than an
	// environment variable for one and a flag for the other.
	if *org != "" {
		os.Setenv("ORG_ID", *org)
	}
	if os.Getenv("ORG_ID") == "" {
		fmt.Fprintln(os.Stderr, "audit-run: no organization specified.\n\n"+
			"  Pass it:      -org=123456789012\n"+
			"  Or export it: export ORG_ID=$(gcloud organizations list --format='value(ID)' | head -1)")
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
			if c.scope == "org" || c.scope == "xref" {
				f = append(f, c)
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
			if c.scope == "project" || c.scope == "xref" {
				f = append(f, c)
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
			case !c.emptyPass:
				state = "review"
			}
			fmt.Printf("%-6s %-9s %-8s %s\n", c.id, c.ref, state, c.title)
		}
		fmt.Fprintf(os.Stderr, "\n%d checks\n", len(checks))
		return
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
		var from []string
		for _, id := range results[i].refs {
			ref, ok := byID[id]
			if !ok || ref.v == vXref {
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
		if ok && proj != "" && (proj == hostProjectNumber || proj == hostProjectID) {
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

func writePack(dir string, rs []result, manual []manualItem) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := time.Now().Format("2006-01-02 15:04")
	org := os.Getenv("ORG_ID")
	target := auditTarget

	// ---------- 01 automated ----------
	var a strings.Builder
	t := tally(rs)
	fmt.Fprintf(&a, "# 1. Automated Results\n\nOrganization `%s` · scope: **%s** · %s\n\n", org, target, stamp)
	fmt.Fprintf(&a, "%d checks run by `audit-run.go`.\n\n", len(rs))

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
	fmt.Fprintf(&a, "| Verdict | Count | Meaning |\n|---|---|---|\n")
	fmt.Fprintf(&a, "| PASS | %d | Compliant — tick the checklist |\n", t[vPass])
	fmt.Fprintf(&a, "| FAIL | %d | A finding — output below |\n", t[vFail])
	fmt.Fprintf(&a, "| REVIEW | %d | Ran clean; a human must judge the output |\n", t[vReview])
	fmt.Fprintf(&a, "| SKIP | %d | Placeholder not supplied |\n", t[vSkip])
	fmt.Fprintf(&a, "| N/A | %d | API or product absent |\n", t[vNA])
	fmt.Fprintf(&a, "| DENIED | %d | Missing permission |\n", t[vDenied])
	fmt.Fprintf(&a, "| ERROR | %d | Command failed |\n\n", t[vError])

	a.WriteString("## Results\n\n| Check | Requirement | Verdict | Title |\n|---|---|---|---|\n")
	for _, r := range rs {
		fmt.Fprintf(&a, "| %s | `%s` | **%s** | %s |\n", r.id, r.ref, r.v.label(), r.title)
	}
	a.WriteString("\n")

	if t[vFail] > 0 {
		a.WriteString("## Findings\n\n")
		for _, r := range rs {
			if r.v == vFail {
				fmt.Fprintf(&a, "### %s — %s\n\n`%s` · pass if: %s\n\n```\n%s\n```\n\n",
					r.id, r.title, r.ref, r.criteria, r.output)
			}
		}
	}
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
	if err := os.WriteFile(dir+"/01-automated-results.md", []byte(a.String()), 0o644); err != nil {
		return err
	}

	// The manual worksheets are organization-level. Skip them on project runs.
	if auditScope == "project" {
		fmt.Fprintf(os.Stderr, "\naudit pack written to %s/\n", dir)
		fmt.Fprintf(os.Stderr, "  01-automated-results.md   %d checks\n", len(rs))
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
	fmt.Fprintf(os.Stderr, "  01-automated-results.md   %d checks\n", len(rs))
	fmt.Fprintf(os.Stderr, "  02-manual-cli.md          %d GCP tasks\n", len(gcp))
	fmt.Fprintf(os.Stderr, "  03-manual-process.md      %d process requirements\n", len(proc))
	return nil
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
var discoverHint = map[string]string{
	"SECURITY_PROJECT":   "project carrying audit API quota",
	"PROJECT_ID":         "main workload project — gcloud projects list",
	"DOMAIN":             "Cloud Identity primary domain, e.g. example.com",
	"LOG_BUCKET":         "GCS bucket holding exported audit logs",
	"BACKUP_BUCKET":      "GCS bucket holding backups",
	"TFSTATE_BUCKET":     "GCS bucket holding Terraform state",
	"BACKUP_PROJECT":     "project holding isolated backups",
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
	b.WriteString("# audit-run placeholder values\n")
	b.WriteString("# Fill these in before the run so nothing is SKIPped.\n#\n")
	b.WriteString("# If a resource genuinely does not exist in this organization, set it to\n")
	b.WriteString("#   none\n")
	b.WriteString("# and the checks that need it are recorded as FINDINGS, not skipped. A\n")
	b.WriteString("# missing log bucket or backup bucket is non-compliance, not missing data.\n\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "# %s (%d check(s))\n%s=\n\n", discoverHint[k], count[k], k)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s — %d placeholder(s)\n\nFill it in, then:\n  go run audit-run.go -config %s -pack ./audit\n",
		path, len(keys), path)
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
