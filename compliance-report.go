//go:build ignore

// Standalone script. The build tag keeps it out of package builds so that
// two `package main` files can sit in one directory without colliding —
// `go vet ./...` and editors would otherwise report "main redeclared".
// Run it directly: go run compliance-report.go

// Command compliance-report scores docs/cis-ig1-gcp-checklist.md and reports
// how much of CIS IG1 the organization actually satisfies.
//
// It distinguishes three states, because "not compliant" conflates two very
// different situations when remediation requires management approval:
//
//   - [ ] item              not started    — no fix written, owned by SRE
//   - [ ] item `PR #123`    PR submitted   — fix written, owned by management
//   - [x] item              compliant      — verified in the live estate
//
// The headline number most reports show ("% compliant") understates the work
// done and misattributes the blockage. This tool reports that number, but also
// reports remediation coverage (compliant + submitted) and, more usefully, how
// long approvals have been outstanding.
//
// Single-file script — no module required. Run it from the repository root:
//
//	go run compliance-report.go
//	go run compliance-report.go --update    # rewrite Status lines to match
//	go run compliance-report.go --format=md > report.md
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type state int

const (
	notStarted state = iota
	prSubmitted
	compliant
)

func (s state) String() string {
	switch s {
	case compliant:
		return "Compliant"
	case prSubmitted:
		return "PR Submitted"
	default:
		return "Not Compliant"
	}
}

type requirement struct {
	text    string
	state   state
	pr      string
	raised  time.Time
	hasDate bool
	// vnum is the automated check that answers this requirement, e.g. "V27",
	// taken from the link the checklist already carries. Empty means no
	// command can answer it: the auditor does, in the interview.
	vnum string
}

func (r requirement) manual() bool { return r.vnum == "" }

type safeguard struct {
	id           string
	title        string
	control      string
	requirements []requirement
	lineNo       int // index of the Status line, for --update
}

// rollup derives safeguard state from its requirements. A safeguard is
// compliant only when every requirement is verified — one outstanding item
// keeps the whole safeguard open. If nothing is outstanding without a PR,
// the safeguard is blocked on approval rather than on SRE.
func (s safeguard) rollup() state {
	if len(s.requirements) == 0 {
		return notStarted
	}
	allDone, anyPR := true, false
	for _, r := range s.requirements {
		if r.state != compliant {
			allDone = false
		}
		if r.state == prSubmitted {
			anyPR = true
		}
	}
	switch {
	case allDone:
		return compliant
	case anyPR && !s.hasUnstarted():
		return prSubmitted
	case anyPR:
		return prSubmitted // partial: some fixes raised, some not written
	default:
		return notStarted
	}
}

func (s safeguard) hasUnstarted() bool {
	for _, r := range s.requirements {
		if r.state == notStarted {
			return true
		}
	}
	return false
}

// unstartedCount is the number that actually belongs to SRE.
func (s safeguard) unstartedCount() int {
	n := 0
	for _, r := range s.requirements {
		if r.state == notStarted {
			n++
		}
	}
	return n
}

var (
	reSafeguard = regexp.MustCompile(`^### (\d+\.\d+) (.+)$`)
	reControl   = regexp.MustCompile(`^## Control (\d+) — (.+)$`)
	reStatus    = regexp.MustCompile(`^\*\*Status:\*\*`)
	reReq       = regexp.MustCompile(`^- \[([ xX])\] (.+)$`)
	rePR        = regexp.MustCompile("`(PR #[\\w.-]+)(?:\\s+(\\d{4}-\\d{2}-\\d{2}))?`")
	reVLink     = regexp.MustCompile(`\[(V\d+)\]\(`)
	reExcluded  = regexp.MustCompile(`^\| (\d+\.\d+) \| ([^|]+?) \| ([^|]+?) \|$`)
)

func main() {
	var (
		path      = flag.String("file", "docs/cis-ig1-gcp-checklist.md", "checklist to score")
		update    = flag.Bool("update", false, "rewrite Status lines to match requirement boxes")
		format    = flag.String("format", "text", "text | md")
		interview = flag.String("interview", "", "put every unanswered manual requirement to the auditor, saving answers here")
		runs      = flag.String("runs", "", "directory of audit runs, for the automated verdicts")
		answers   = flag.String("answers", "", "the manual answers file written by -interview")
		sgMD      = flag.String("safeguards-md", "", "write the safeguard compliance report here")
		sgJSON    = flag.String("safeguards-json", "", "write the same, as JSON for the dashboard")
		projList  = flag.String("projects", "", "the organization's project list — the denominator for project coverage")
	)
	flag.Parse()

	lines, err := readLines(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compliance-report: %v\n", err)
		os.Exit(2)
	}

	safeguards, warnings := parse(lines)
	if len(safeguards) == 0 {
		fmt.Fprintln(os.Stderr, "compliance-report: no safeguards found — is this the right file?")
		os.Exit(2)
	}
	excludedSGs := parseExcluded(lines)

	if *interview != "" {
		os.Exit(runInterview(*interview, safeguards, excludedSGs))
	}

	if *sgMD != "" || *sgJSON != "" {
		auto := map[string]verdict3{}
		autoBy := map[string]map[string]verdict3{}
		var projects []projectStatus
		if *runs != "" {
			a, bt, p, rerr := loadRuns(*runs)
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "compliance-report: %v\n", rerr)
				os.Exit(2)
			}
			auto, autoBy, projects = a, bt, p
		}
		af := answerFile{Answers: map[string]answer{}}
		if *answers != "" {
			loaded, aerr := loadAnswers(*answers)
			if aerr != nil {
				fmt.Fprintf(os.Stderr, "compliance-report: %v\n", aerr)
				os.Exit(2)
			}
			af = loaded
		}
		total := 0
		if *projList != "" {
			n, perr := countProjects(*projList)
			if perr != nil {
				fmt.Fprintf(os.Stderr, "compliance-report: %v\n", perr)
				os.Exit(2)
			}
			total = n
		}
		doc := buildSafeguards(safeguards, excludedSGs, auto, autoBy, af, projects, total)
		if *sgJSON != "" {
			b, jerr := json.MarshalIndent(doc, "", "  ")
			if jerr != nil {
				fmt.Fprintf(os.Stderr, "compliance-report: %v\n", jerr)
				os.Exit(2)
			}
			if werr := os.WriteFile(*sgJSON, append(b, '\n'), 0o644); werr != nil {
				fmt.Fprintf(os.Stderr, "compliance-report: %v\n", werr)
				os.Exit(2)
			}
		}
		if *sgMD != "" {
			if werr := os.WriteFile(*sgMD, []byte(renderSafeguards(doc)), 0o644); werr != nil {
				fmt.Fprintf(os.Stderr, "compliance-report: %v\n", werr)
				os.Exit(2)
			}
		}
		fmt.Fprintf(os.Stderr, "safeguards %d%% (%d/%d) · projects %d%% audited (%d/%d), %d%% passing (%d/%d)\n",
			doc.Safeguards.Pct, doc.Safeguards.Pass, doc.Safeguards.Total,
			doc.Projects.PctAudited, doc.Projects.Audited, doc.Projects.Total,
			doc.Projects.PctPassing, doc.Projects.Passing, doc.Projects.Total)
		return
	}

	if *update {
		n := applyStatus(lines, safeguards)
		if err := writeLines(*path, lines); err != nil {
			fmt.Fprintf(os.Stderr, "compliance-report: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "updated %d status lines in %s\n", n, *path)
	}

	if *format == "md" {
		renderMarkdown(os.Stdout, safeguards)
	} else {
		renderText(os.Stdout, safeguards)
	}

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
}

func parse(lines []string) ([]safeguard, []string) {
	var (
		out      []safeguard
		warnings []string
		control  string
		cur      *safeguard
		inScored bool
	)

	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}

	for i, ln := range lines {
		if m := reControl.FindStringSubmatch(ln); m != nil {
			flush()
			control = "Control " + m[1] + " — " + m[2]
			inScored = true
			continue
		}

		// Appendices and the how-to-use block contain checkboxes that are not
		// safeguard requirements. Only score between Control headings.
		if strings.HasPrefix(ln, "## Appendix") || strings.HasPrefix(ln, "## Step 0") {
			flush()
			inScored = false
			continue
		}

		if m := reSafeguard.FindStringSubmatch(ln); m != nil && inScored {
			flush()
			cur = &safeguard{id: m[1], title: m[2], control: control, lineNo: -1}
			continue
		}

		if cur == nil {
			continue
		}

		if reStatus.MatchString(ln) {
			cur.lineNo = i
			continue
		}

		if m := reReq.FindStringSubmatch(ln); m != nil {
			r := requirement{text: reqText(m[2])}
			if vm := reVLink.FindStringSubmatch(ln); vm != nil {
				r.vnum = vm[1]
			}
			checked := m[1] == "x" || m[1] == "X"

			if pm := rePR.FindStringSubmatch(ln); pm != nil {
				r.pr = pm[1]
				if pm[2] != "" {
					if t, err := time.Parse("2006-01-02", pm[2]); err == nil {
						r.raised, r.hasDate = t, true
					}
				}
			}

			switch {
			case checked:
				r.state = compliant
				if r.pr != "" {
					warnings = append(warnings, fmt.Sprintf(
						"%s: requirement is ticked but still carries %s — drop the PR marker once verified",
						cur.id, r.pr))
				}
			case r.pr != "":
				r.state = prSubmitted
			default:
				r.state = notStarted
			}
			cur.requirements = append(cur.requirements, r)
		}
	}
	flush()
	return out, warnings
}

// applyStatus rewrites each Status line to reflect the derived rollup, so the
// document never disagrees with its own checkboxes.
func applyStatus(lines []string, sgs []safeguard) int {
	mark := func(want, got state) string {
		if want == got {
			return "x"
		}
		return " "
	}
	n := 0
	for _, sg := range sgs {
		if sg.lineNo < 0 {
			continue
		}
		got := sg.rollup()
		lines[sg.lineNo] = fmt.Sprintf(
			"**Status:** `[%s] Compliant`  `[%s] PR Submitted`  `[%s] Not Compliant`",
			mark(compliant, got), mark(prSubmitted, got), mark(notStarted, got))
		n++
	}
	return n
}

type totals struct {
	compliant, submitted, notStarted, total int
}

func (t totals) pct(n int) float64 {
	if t.total == 0 {
		return 0
	}
	return float64(n) / float64(t.total) * 100
}

func tally(sgs []safeguard) (sg totals, req totals) {
	for _, s := range sgs {
		sg.total++
		switch s.rollup() {
		case compliant:
			sg.compliant++
		case prSubmitted:
			sg.submitted++
		default:
			sg.notStarted++
		}
		for _, r := range s.requirements {
			req.total++
			switch r.state {
			case compliant:
				req.compliant++
			case prSubmitted:
				req.submitted++
			default:
				req.notStarted++
			}
		}
	}
	return
}

type waiting struct {
	sg     string
	prs    []string
	oldest int
	dated  bool
	remain int // requirements in this safeguard still unwritten
}

// pending groups outstanding approvals by safeguard rather than listing every
// requirement. Management needs to see "safeguard 3.3 has five fixes waiting,
// oldest 65 days", not five near-identical rows.
func pending(sgs []safeguard) []waiting {
	var out []waiting
	now := time.Now()

	for _, s := range sgs {
		w := waiting{sg: s.id + " " + s.title, remain: s.unstartedCount()}
		seen := map[string]bool{}

		for _, r := range s.requirements {
			if r.state != prSubmitted {
				continue
			}
			if !seen[r.pr] {
				seen[r.pr] = true
				w.prs = append(w.prs, r.pr)
			}
			if r.hasDate {
				if d := int(now.Sub(r.raised).Hours() / 24); d > w.oldest {
					w.oldest, w.dated = d, true
				}
			}
		}

		if len(w.prs) > 0 {
			sort.Strings(w.prs)
			out = append(out, w)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].oldest > out[j].oldest })
	return out
}

// prList renders the PR references compactly.
func (w waiting) prList() string {
	if len(w.prs) <= 3 {
		return strings.Join(w.prs, ", ")
	}
	return fmt.Sprintf("%s +%d", strings.Join(w.prs[:2], ", "), len(w.prs)-2)
}

func bar(pctVal float64, width int) string {
	filled := int(pctVal / 100 * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func renderText(w *os.File, sgs []safeguard) {
	sg, req := tally(sgs)

	fmt.Fprintf(w, "\nCIS CONTROLS v8.1 IG1 — GCP COMPLIANCE REPORT\n")
	fmt.Fprintf(w, "Generated %s\n", time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintf(w, "%s\n\n", strings.Repeat("=", 64))

	fmt.Fprintf(w, "SAFEGUARDS (%d in scope)\n\n", sg.total)
	fmt.Fprintf(w, "  Compliant          %3d  %5.1f%%  %s\n", sg.compliant, sg.pct(sg.compliant), bar(sg.pct(sg.compliant), 24))
	fmt.Fprintf(w, "  PR submitted       %3d  %5.1f%%  %s\n", sg.submitted, sg.pct(sg.submitted), bar(sg.pct(sg.submitted), 24))
	fmt.Fprintf(w, "  Not started        %3d  %5.1f%%  %s\n\n", sg.notStarted, sg.pct(sg.notStarted), bar(sg.pct(sg.notStarted), 24))

	fmt.Fprintf(w, "REQUIREMENTS (%d in scope)\n\n", req.total)
	fmt.Fprintf(w, "  Compliant          %3d  %5.1f%%\n", req.compliant, req.pct(req.compliant))
	fmt.Fprintf(w, "  PR submitted       %3d  %5.1f%%\n", req.submitted, req.pct(req.submitted))
	fmt.Fprintf(w, "  Not started        %3d  %5.1f%%\n\n", req.notStarted, req.pct(req.notStarted))

	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 64))
	fmt.Fprintf(w, "ACCOUNTABILITY SPLIT\n\n")
	remediated := req.compliant + req.submitted
	fmt.Fprintf(w, "  Remediation written by SRE   %3d / %d  %5.1f%%\n",
		remediated, req.total, req.pct(remediated))
	fmt.Fprintf(w, "  Awaiting management approval %3d          %5.1f%%\n",
		req.submitted, req.pct(req.submitted))
	fmt.Fprintf(w, "  Outstanding with SRE         %3d          %5.1f%%\n\n",
		req.notStarted, req.pct(req.notStarted))

	pend := pending(sgs)
	if len(pend) > 0 {
		fmt.Fprintf(w, "%s\n", strings.Repeat("-", 64))
		fmt.Fprintf(w, "AWAITING MANAGEMENT APPROVAL (%d safeguards)\n\n", len(pend))
		fmt.Fprintf(w, "  %-38s  %-22s  %-9s %s\n", "SAFEGUARD", "PRs", "WAITING", "STILL UNWRITTEN")
		maxDays := 0
		for _, p := range pend {
			age := "undated"
			if p.dated {
				age = fmt.Sprintf("%d days", p.oldest)
				if p.oldest > maxDays {
					maxDays = p.oldest
				}
			}
			name := p.sg
			if len(name) > 38 {
				name = name[:35] + "..."
			}
			remain := "-"
			if p.remain > 0 {
				remain = fmt.Sprintf("%d", p.remain)
			}
			fmt.Fprintf(w, "  %-38s  %-22s  %-9s %s\n", name, p.prList(), age, remain)
		}
		if maxDays > 0 {
			fmt.Fprintf(w, "\n  Oldest fix waiting on approval: %d days\n", maxDays)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 64))
	fmt.Fprintf(w, "BY CONTROL\n\n")
	byControl := map[string][]safeguard{}
	var order []string
	for _, s := range sgs {
		if _, seen := byControl[s.control]; !seen {
			order = append(order, s.control)
		}
		byControl[s.control] = append(byControl[s.control], s)
	}
	for _, c := range order {
		group := byControl[c]
		var done, sub int
		for _, s := range group {
			switch s.rollup() {
			case compliant:
				done++
			case prSubmitted:
				sub++
			}
		}
		pctVal := float64(done) / float64(len(group)) * 100
		name := c
		if len(name) > 44 {
			name = name[:41] + "..."
		}
		flag := ""
		if sub > 0 {
			flag = fmt.Sprintf("  (%d awaiting approval)", sub)
		}
		fmt.Fprintf(w, "  %-44s %2d/%-2d %5.1f%%%s\n", name, done, len(group), pctVal, flag)
	}
	fmt.Fprintln(w)
}

func renderMarkdown(w *os.File, sgs []safeguard) {
	sg, req := tally(sgs)
	fmt.Fprintf(w, "# CIS IG1 GCP Compliance Report\n\n")
	fmt.Fprintf(w, "_Generated %s_\n\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(w, "| | Safeguards | | Requirements | |\n|---|---|---|---|---|\n")
	fmt.Fprintf(w, "| Compliant | %d/%d | %.1f%% | %d/%d | %.1f%% |\n",
		sg.compliant, sg.total, sg.pct(sg.compliant), req.compliant, req.total, req.pct(req.compliant))
	fmt.Fprintf(w, "| PR submitted | %d/%d | %.1f%% | %d/%d | %.1f%% |\n",
		sg.submitted, sg.total, sg.pct(sg.submitted), req.submitted, req.total, req.pct(req.submitted))
	fmt.Fprintf(w, "| Not started | %d/%d | %.1f%% | %d/%d | %.1f%% |\n\n",
		sg.notStarted, sg.total, sg.pct(sg.notStarted), req.notStarted, req.total, req.pct(req.notStarted))

	if pend := pending(sgs); len(pend) > 0 {
		fmt.Fprintf(w, "## Awaiting management approval\n\n")
		fmt.Fprintf(w, "| Safeguard | PRs | Waiting | Still unwritten |\n|---|---|---|---|\n")
		for _, p := range pend {
			age := "undated"
			if p.dated {
				age = fmt.Sprintf("%d days", p.oldest)
			}
			fmt.Fprintf(w, "| %s | %s | %s | %d |\n", p.sg, p.prList(), age, p.remain)
		}
		fmt.Fprintln(w)
	}
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}

func writeLines(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// ---------------------------------------------------------------------------
// Safeguard compliance — the two numbers the engagement reports
// ---------------------------------------------------------------------------
//
// A safeguard passes when every requirement under it is satisfied. 188 of the
// 290 requirements carry an automated check and are answered by a run; the
// other 102 have no command that can answer them and are answered by the
// auditor, once, in the interview. Twelve IG1 safeguards have no GCP surface
// at all — training, end-user devices, removable media — and are answered the
// same way, so the denominator can honestly be 56 rather than 44.
//
//	go run compliance-report.go -interview audit-state/manual-answers.json
//	go run compliance-report.go -runs audit-state/runs \
//	  -answers audit-state/manual-answers.json \
//	  -safeguards-md audit-state/safeguards.md \
//	  -safeguards-json audit-state/safeguards.json \
//	  -projects config/projects.txt

type verdict3 int

const (
	vcOpen verdict3 = iota // nobody has answered, or the machine could not
	vcPass
	vcFail
)

func (v verdict3) String() string {
	switch v {
	case vcPass:
		return "pass"
	case vcFail:
		return "fail"
	}
	return "open"
}

// worse keeps the more serious of two verdicts. A requirement answered PASS in
// five projects and FAIL in two is a FAIL: the estate does not satisfy it.
func worse(a, b verdict3) verdict3 {
	if a == vcFail || b == vcFail {
		return vcFail
	}
	if a == vcOpen || b == vcOpen {
		return vcOpen
	}
	return vcPass
}

// excluded is one of the twelve IG1 safeguards with no GCP surface.
type excluded struct {
	id, title, owner string
}

// parseExcluded reads Appendix A. These are part of the enterprise's IG1
// obligation whoever owns them, so they belong in the denominator.
func parseExcluded(lines []string) []excluded {
	var out []excluded
	in := false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## Appendix A") {
			in = true
			continue
		}
		if in && strings.HasPrefix(ln, "## Appendix") {
			break
		}
		if !in {
			continue
		}
		if m := reExcluded.FindStringSubmatch(ln); m != nil {
			id := strings.TrimSpace(m[1])
			if id == "Safeguard" {
				continue
			}
			out = append(out, excluded{id, strings.TrimSpace(m[2]), strings.TrimSpace(m[3])})
		}
	}
	return out
}

// reqKey identifies a requirement across runs of this tool. The checklist
// carries no requirement IDs, so the key is its safeguard plus a hash of its
// text: stable when requirements are reordered, and deliberately invalidated
// when the text itself is edited, because the answer was to the old wording.
func reqKey(sg, text string) string {
	sum := sha256.Sum256([]byte(strings.Join(strings.Fields(text), " ")))
	return fmt.Sprintf("%s/%x", sg, sum[:4])
}

type answer struct {
	Key       string `json:"key"`
	Safeguard string `json:"safeguard"`
	Text      string `json:"text"`
	Verdict   string `json:"verdict"` // pass | fail
	By        string `json:"by"`
	At        string `json:"at"`
}

type answerFile struct {
	Schema  string            `json:"schema"`
	Answers map[string]answer `json:"answers"`
}

func loadAnswers(path string) (answerFile, error) {
	af := answerFile{Schema: "cis-ig1-manual-answers/1", Answers: map[string]answer{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return af, nil
	}
	if err != nil {
		return af, err
	}
	if err := json.Unmarshal(raw, &af); err != nil {
		return af, err
	}
	if af.Answers == nil {
		af.Answers = map[string]answer{}
	}
	return af, nil
}

// saveAnswers writes via a temporary file and renames, so an interrupted save
// cannot leave a half-written file where the answers used to be.
func saveAnswers(path string, af answerFile) error {
	b, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------------------------------------------------------------------------
// Automated verdicts, read back from the runs
// ---------------------------------------------------------------------------

type savedRun struct {
	Target  string `json:"Target"`
	Stamp   string `json:"Stamp"`
	Scope   string `json:"Scope"`
	Results []struct {
		ID      string   `json:"ID"`
		Verdict string   `json:"Verdict"`
		Refs    []string `json:"Refs"`
	} `json:"Results"`
	Decisions map[string]struct {
		Verdict string `json:"Verdict"`
	} `json:"Decisions"`
}

type projectStatus struct {
	name     string
	complete bool // every check reached a final verdict
	passing  bool // complete, and nothing failed
}

// loadRuns walks a directory of runs and returns the worst verdict seen for
// each check, plus per-project status. The newest file for a target wins, the
// same rule rollup.go applies: a target audited twice is counted once.
func loadRuns(root string) (map[string]verdict3, map[string]map[string]verdict3, []projectStatus, error) {
	// Which pass wins when a target was audited more than once — and it will
	// be, because a fix is promoted dev → production and the project is
	// re-audited afterwards. The rule is rollup.go's: a COMPLETE pass beats an
	// incomplete one however old, and between two of equal standing the newer
	// wins. Taking the newest outright would let a re-run with one REVIEW
	// still outstanding supersede the finished audit it was meant to confirm.
	type found struct {
		path     string
		stamp    string
		complete bool
	}
	newest := map[string]found{}
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/evidence/results/") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		var run savedRun
		if json.Unmarshal(raw, &run) != nil {
			return nil
		}
		cand := found{path: path, stamp: run.Stamp, complete: runComplete(run)}
		if cand.stamp == "" {
			cand.stamp = fi.ModTime().UTC().Format("2006-01-02 15:04")
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		prev, ok := newest[name]
		if !ok || betterRun(cand.complete, cand.stamp, prev.complete, prev.stamp) {
			newest[name] = cand
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}

	verdicts := map[string]verdict3{}
	// byTarget is the same data undivided: which target reached which verdict
	// for each check, so a failing safeguard can name the projects holding it
	// there rather than only the requirement.
	byTarget := map[string]map[string]verdict3{}
	var projects []projectStatus
	names := make([]string, 0, len(newest))
	for n := range newest {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		raw, rerr := os.ReadFile(newest[name].path)
		if rerr != nil {
			continue
		}
		var run savedRun
		if json.Unmarshal(raw, &run) != nil {
			continue
		}
		st := projectStatus{name: name, complete: true, passing: true}
		for _, r := range run.Results {
			label := r.Verdict
			// An auditor's decision stands in for whatever the machine reached,
			// exactly as the report renders it.
			if d, ok := run.Decisions[r.ID]; ok && d.Verdict != "" {
				label = d.Verdict
			}
			if len(r.Refs) > 0 {
				continue // a cross-reference: its referents already count
			}
			var v verdict3
			switch strings.ToUpper(label) {
			case "PASS", "N/A", "NA":
				v = vcPass
			case "FAIL":
				v = vcFail
			case "XREF":
				continue
			default: // REVIEW, SKIP, ERROR, DENIED
				v = vcOpen
			}
			if v == vcFail {
				st.passing = false
			}
			if v == vcOpen {
				st.complete, st.passing = false, false
			}
			if prev, ok := verdicts[r.ID]; ok {
				verdicts[r.ID] = worse(prev, v)
			} else {
				verdicts[r.ID] = v
			}
			if byTarget[r.ID] == nil {
				byTarget[r.ID] = map[string]verdict3{}
			}
			byTarget[r.ID][name] = v
		}
		if run.Scope == "project" {
			projects = append(projects, st)
		}
	}
	return verdicts, byTarget, projects, nil
}

// ---------------------------------------------------------------------------
// The interview: every requirement no command can answer
// ---------------------------------------------------------------------------

// runInterview puts each unanswered manual requirement to the auditor, saving
// after every answer so the session can be abandoned and resumed. The twelve
// safeguards with no GCP surface come last, grouped, because they are a
// different conversation with a different owner.
func runInterview(path string, sgs []safeguard, exc []excluded) int {
	af, err := loadAnswers(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compliance-report: %v\n", err)
		return 2
	}
	who := auditor()

	type q struct {
		key, sg, title, text, owner string
	}
	var queue []q
	for _, s := range sgs {
		for _, r := range s.requirements {
			if !r.manual() {
				continue
			}
			k := reqKey(s.id, r.text)
			if _, done := af.Answers[k]; !done {
				queue = append(queue, q{k, s.id, s.title, r.text, ""})
			}
		}
	}
	for _, e := range exc {
		k := reqKey(e.id, e.title)
		if _, done := af.Answers[k]; !done {
			queue = append(queue, q{k, e.id, e.title, e.title, e.owner})
		}
	}

	if len(queue) == 0 {
		fmt.Printf("Nothing left to answer — %d recorded in %s\n", len(af.Answers), path)
		return 0
	}

	fmt.Printf("\nManual interview · %d to answer · %d already recorded\n", len(queue), len(af.Answers))
	fmt.Printf("Answered by %s. Saved as you go; q quits and running this again resumes.\n", who)
	fmt.Println("Evidence is not captured here — attach it to the engagement record yourself.")

	in := bufio.NewReader(os.Stdin)
	for i, item := range queue {
		fmt.Printf("\n%s\n", strings.Repeat("═", 78))
		fmt.Printf("%-6s %s      [%d of %d]\n", item.sg, item.title, i+1, len(queue))
		if item.owner != "" {
			fmt.Printf("Owner: %s — no GCP surface\n", item.owner)
		}
		fmt.Printf("%s\n%s\n%s\n", strings.Repeat("─", 78), item.text, strings.Repeat("─", 78))

		for {
			fmt.Print("[y]es satisfied  [n]o  [s]kip for now  [q]uit and save > ")
			line, rerr := in.ReadString('\n')
			if rerr != nil && line == "" {
				fmt.Println()
				return 0
			}
			a := strings.ToLower(strings.TrimSpace(line))
			if a == "" {
				continue
			}
			switch a[0] {
			case 'y', 'n':
				v := "pass"
				if a[0] == 'n' {
					v = "fail"
				}
				af.Answers[item.key] = answer{
					Key: item.key, Safeguard: item.sg, Text: item.text,
					Verdict: v, By: who, At: time.Now().Format("2006-01-02 15:04"),
				}
				if err := saveAnswers(path, af); err != nil {
					fmt.Fprintf(os.Stderr, "compliance-report: %v\n", err)
					return 2
				}
			case 's':
			case 'q':
				fmt.Printf("\n%d answered, %d left. Run the interview again to carry on.\n",
					len(af.Answers), len(queue)-i)
				return 0
			default:
				continue
			}
			break
		}
	}
	fmt.Printf("\nAll answered — %d recorded in %s\n", len(af.Answers), path)
	return 0
}

func auditor() string {
	if out, err := exec.Command("gcloud", "config", "get-value", "account").Output(); err == nil {
		if a := strings.TrimSpace(string(out)); a != "" && a != "(unset)" {
			return a
		}
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}

// ---------------------------------------------------------------------------
// The report
// ---------------------------------------------------------------------------

type sgResult struct {
	ID       string    `json:"safeguard"`
	Title    string    `json:"title"`
	Control  string    `json:"control"`
	State    string    `json:"state"` // pass | fail | open
	GCP      bool      `json:"gcp_actionable"`
	Owner    string    `json:"owner,omitempty"`
	Total    int       `json:"requirements"`
	Pass     int       `json:"pass"`
	Fail     int       `json:"fail"`
	Open     int       `json:"open"`
	Blocking []blocker `json:"blocking,omitempty"`
}

// blocker is one requirement standing between a safeguard and passing, and
// the targets responsible. Naming them turns the report into a promotion
// tracker: a safeguard waiting on a dev project is work in flight, the same
// safeguard waiting on production is live exposure.
type blocker struct {
	Requirement string   `json:"requirement"`
	Check       string   `json:"check,omitempty"` // V-number, empty when manual
	State       string   `json:"state"`           // fail | open
	Targets     []string `json:"targets,omitempty"`
}

type counts struct {
	Total int `json:"total"`
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
	Open  int `json:"open"`
	Pct   int `json:"pct_passing"`
}

type projCounts struct {
	Total      int `json:"total"`
	Audited    int `json:"audited"`
	Passing    int `json:"passing"`
	PctAudited int `json:"pct_audited"`
	PctPassing int `json:"pct_passing"`
}

// targetResult scores one target against the safeguards IT can judge. A
// project pass runs 103 of the 188 checks and reaches 37 of the 44
// GCP-actionable safeguards; the organization pass reaches 29. Seven
// safeguards have no project-scope check at all. Scoring every target out of
// 56 would mark each one down for 19 safeguards it was never asked about, so
// each is scored against its own denominator — equal between projects, since
// every project pass covers the same 37.
//
// These are automated checks only. The manual half is answered once for the
// organization, not per target, and lives in the headline figure instead.
type targetResult struct {
	Name       string `json:"name"`
	Scope      string `json:"scope"`
	Complete   bool   `json:"complete"`
	Counted    bool   `json:"counted"`
	Safeguards int    `json:"safeguards_judged"`
	Passing    int    `json:"passing"`
	Failing    int    `json:"failing"`
	Open       int    `json:"open"`
	Pct        int    `json:"pct_passing"`
}

type sgDoc struct {
	Schema     string         `json:"schema"`
	Generated  string         `json:"generated"`
	Safeguards counts         `json:"safeguards"`
	GCPOnly    counts         `json:"gcp_actionable"`
	Excluded   counts         `json:"no_gcp_surface"`
	Projects   projCounts     `json:"projects"`
	Org        *targetResult  `json:"organization,omitempty"`
	ProjectAvg int            `json:"projects_pct_passing_mean"`
	Targets    []targetResult `json:"targets"`
	Detail     []sgResult     `json:"detail"`
	Unanswered []string       `json:"unanswered_safeguards,omitempty"`
}

func pctOf(n, of int) int {
	if of == 0 {
		return 0
	}
	return n * 100 / of
}

func buildSafeguards(sgs []safeguard, exc []excluded, auto map[string]verdict3,
	autoBy map[string]map[string]verdict3, af answerFile,
	projects []projectStatus, projectTotal int) sgDoc {

	doc := sgDoc{Schema: "cis-ig1-safeguards/1", Generated: time.Now().UTC().Format(time.RFC3339)}

	add := func(c *counts, state verdict3) {
		c.Total++
		switch state {
		case vcPass:
			c.Pass++
		case vcFail:
			c.Fail++
		default:
			c.Open++
		}
	}

	for _, s := range sgs {
		r := sgResult{ID: s.id, Title: s.title, Control: s.control, GCP: true, Total: len(s.requirements)}
		state := vcPass
		for _, req := range s.requirements {
			var v verdict3
			var targets []string
			if !req.manual() {
				for target, tv := range autoBy[req.vnum] {
					if tv != vcPass {
						targets = append(targets, target)
					}
				}
				sort.Strings(targets)
			}
			if req.manual() {
				if a, ok := af.Answers[reqKey(s.id, req.text)]; ok {
					v = vcPass
					if a.Verdict == "fail" {
						v = vcFail
					}
				} else {
					v = vcOpen
				}
			} else if got, ok := auto[req.vnum]; ok {
				v = got
			} else {
				v = vcOpen // the check exists but no run has answered it
			}
			switch v {
			case vcPass:
				r.Pass++
			case vcFail:
				r.Fail++
				r.Blocking = append(r.Blocking, blocker{req.text, req.vnum, "fail", targets})
			default:
				r.Open++
				r.Blocking = append(r.Blocking, blocker{req.text, req.vnum, "open", targets})
			}
			state = worse(state, v)
		}
		r.State = state.String()
		doc.Detail = append(doc.Detail, r)
		add(&doc.Safeguards, state)
		add(&doc.GCPOnly, state)
		if state == vcOpen {
			doc.Unanswered = append(doc.Unanswered, s.id)
		}
	}

	for _, e := range exc {
		r := sgResult{ID: e.id, Title: e.title, Owner: e.owner, GCP: false, Total: 1}
		state := vcOpen
		if a, ok := af.Answers[reqKey(e.id, e.title)]; ok {
			state = vcPass
			if a.Verdict == "fail" {
				state = vcFail
			}
		}
		switch state {
		case vcPass:
			r.Pass = 1
		case vcFail:
			r.Fail = 1
			r.Blocking = []blocker{{e.title, "", "fail", []string{e.owner}}}
		default:
			r.Open = 1
			r.Blocking = []blocker{{e.title, "", "open", []string{e.owner}}}
		}
		r.State = state.String()
		doc.Detail = append(doc.Detail, r)
		add(&doc.Safeguards, state)
		add(&doc.Excluded, state)
		if state == vcOpen {
			doc.Unanswered = append(doc.Unanswered, e.id)
		}
	}

	doc.Safeguards.Pct = pctOf(doc.Safeguards.Pass, doc.Safeguards.Total)
	doc.GCPOnly.Pct = pctOf(doc.GCPOnly.Pass, doc.GCPOnly.Total)
	doc.Excluded.Pct = pctOf(doc.Excluded.Pass, doc.Excluded.Total)

	// Two different questions about projects: how much of the estate has been
	// audited at all, and how much of it passes. A project only counts as
	// audited when every check reached a final verdict.
	doc.Projects.Total = projectTotal
	for _, p := range projects {
		if p.complete {
			doc.Projects.Audited++
		}
		if p.passing {
			doc.Projects.Passing++
		}
	}
	if projectTotal == 0 {
		doc.Projects.Total = len(projects)
	}
	doc.Projects.PctAudited = pctOf(doc.Projects.Audited, doc.Projects.Total)
	doc.Projects.PctPassing = pctOf(doc.Projects.Passing, doc.Projects.Total)

	doc.Targets = scoreTargets(sgs, autoBy, projects)
	var sum, n int
	for i := range doc.Targets {
		tr := doc.Targets[i]
		if tr.Scope == "organization" {
			org := tr
			doc.Org = &org
			continue
		}
		// Only a finished audit contributes to the average. An unreconciled
		// pass has safeguards nobody has judged yet, and averaging those in
		// reads as non-compliance rather than as work outstanding.
		if tr.Counted {
			sum += tr.Pct
			n++
		}
	}
	doc.ProjectAvg = 0
	if n > 0 {
		doc.ProjectAvg = sum / n
	}
	return doc
}

// scoreTargets scores each target against the safeguards its own pass judges.
// A safeguard counts for a target when at least one of its requirements has a
// check that target ran, and passes when every such requirement passed there.
func scoreTargets(sgs []safeguard, autoBy map[string]map[string]verdict3,
	projects []projectStatus) []targetResult {

	status := map[string]projectStatus{}
	names := map[string]string{} // name -> scope
	for _, p := range projects {
		status[p.name] = p
		names[p.name] = "project"
	}
	for _, byTarget := range autoBy {
		for target := range byTarget {
			if _, ok := names[target]; !ok {
				names[target] = "organization"
			}
		}
	}

	var out []targetResult
	for target, scope := range names {
		tr := targetResult{Name: target, Scope: scope}
		if st, ok := status[target]; ok {
			tr.Complete, tr.Counted = st.complete, st.complete
		} else {
			tr.Complete, tr.Counted = true, true // the organization pass
		}
		for _, s := range sgs {
			judged, state := false, vcPass
			for _, req := range s.requirements {
				if req.manual() {
					continue
				}
				v, ran := autoBy[req.vnum][target]
				if !ran {
					continue
				}
				judged = true
				state = worse(state, v)
			}
			if !judged {
				continue
			}
			tr.Safeguards++
			switch state {
			case vcPass:
				tr.Passing++
			case vcFail:
				tr.Failing++
			default:
				tr.Open++
			}
		}
		tr.Pct = pctOf(tr.Passing, tr.Safeguards)
		out = append(out, tr)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Scope == "organization") != (out[j].Scope == "organization") {
			return out[i].Scope == "organization"
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func renderSafeguards(doc sgDoc) string {
	var b strings.Builder
	b.WriteString("# CIS IG1 — Safeguard Compliance\n\n")
	fmt.Fprintf(&b, "Compiled %s\n\n", doc.Generated)

	fmt.Fprintf(&b, "> **SAFEGUARDS: %d%% — %d of %d passing**\n",
		doc.Safeguards.Pct, doc.Safeguards.Pass, doc.Safeguards.Total)
	fmt.Fprintf(&b, "> **PROJECTS: %d%% audited (%d of %d) · %d%% passing (%d of %d)**\n",
		doc.Projects.PctAudited, doc.Projects.Audited, doc.Projects.Total,
		doc.Projects.PctPassing, doc.Projects.Passing, doc.Projects.Total)
	if doc.Org != nil {
		fmt.Fprintf(&b, "> **ORGANIZATION: %d%% compliant** — %d of %d safeguards the organization pass judges\n",
			doc.Org.Pct, doc.Org.Passing, doc.Org.Safeguards)
	}
	fmt.Fprintf(&b, "> **PROJECTS, BY SAFEGUARD: %d%% compliant on average** across finished audits\n\n", doc.ProjectAvg)

	b.WriteString("| | Passing | Failing | Unanswered | Total |\n|---|---|---|---|---|\n")
	fmt.Fprintf(&b, "| **All IG1 safeguards** | **%d** | %d | %d | **%d** |\n",
		doc.Safeguards.Pass, doc.Safeguards.Fail, doc.Safeguards.Open, doc.Safeguards.Total)
	fmt.Fprintf(&b, "| GCP-actionable | %d | %d | %d | %d |\n",
		doc.GCPOnly.Pass, doc.GCPOnly.Fail, doc.GCPOnly.Open, doc.GCPOnly.Total)
	fmt.Fprintf(&b, "| No GCP surface (owned elsewhere) | %d | %d | %d | %d |\n\n",
		doc.Excluded.Pass, doc.Excluded.Fail, doc.Excluded.Open, doc.Excluded.Total)

	b.WriteString("A safeguard passes when **every** requirement under it is satisfied. " +
		"CIS sets no partial credit: IG1 is \"implement every safeguard\", so the percentage is " +
		"progress against that, not a compliance claim in itself. **Unanswered** means a check has " +
		"not run, is still awaiting review, or a manual requirement has not been put to the auditor — " +
		"never that it failed.\n\n")

	b.WriteString("Projects are counted two ways. **Audited** is an estate-coverage number: the pass " +
		"reached a final verdict on every check. **Passing** is stricter — audited, and nothing failed.\n\n")

	if len(doc.Unanswered) > 0 {
		fmt.Fprintf(&b, "> ⚠️ **%d safeguard(s) cannot yet be scored**: `%s`. Run the interview for the "+
			"manual requirements, and `--review` for any check still outstanding.\n\n",
			len(doc.Unanswered), strings.Join(doc.Unanswered, "`, `"))
	}

	b.WriteString("## By target\n\n")
	b.WriteString("Each target is scored against the safeguards **its own pass judges**, not against all 56. " +
		"A project pass reaches 35 of them and the organization pass 27; seven safeguards have no " +
		"project-scope check at all, and the twelve with no GCP surface belong to neither. Scoring every " +
		"target out of 56 would mark it down for safeguards it was never asked about.\n\n")
	b.WriteString("Automated checks only — the manual requirements are answered once for the organization " +
		"and carried in the headline figure instead. Only a finished audit contributes to the average.\n\n")
	b.WriteString("| Target | Scope | Judged | Passing | Failing | Open | % | Counted |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, tr := range doc.Targets {
		counted := "yes"
		if !tr.Counted {
			counted = "**no**"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %d | %d | **%d%%** | %s |\n",
			tr.Name, tr.Scope, tr.Safeguards, tr.Passing, tr.Failing, tr.Open, tr.Pct, counted)
	}
	fmt.Fprintf(&b, "\nMean across finished project audits: **%d%%**.\n\n", doc.ProjectAvg)

	b.WriteString("## Safeguards\n\n")
	b.WriteString("| Safeguard | | State | Req | Pass | Fail | Open |\n|---|---|---|---|---|---|---|\n")
	for _, r := range doc.Detail {
		state := r.State
		switch r.State {
		case "pass":
			state = "**pass**"
		case "fail":
			state = "**FAIL**"
		}
		name := r.Title
		if !r.GCP {
			name += " _(" + r.Owner + ")_"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %d | %d | %d |\n",
			r.ID, name, state, r.Total, r.Pass, r.Fail, r.Open)
	}
	b.WriteString("\n## What is blocking each one\n\n")
	b.WriteString("Where a check names projects, those are the ones that have not satisfied it. " +
		"A safeguard waiting only on a development project is a fix in flight; the same safeguard " +
		"waiting on production is live exposure.\n\n")
	for _, r := range doc.Detail {
		if len(r.Blocking) == 0 {
			continue
		}
		fmt.Fprintf(&b, "**%s %s** — %s\n\n", r.ID, r.Title, strings.ToUpper(r.State))
		for _, bl := range r.Blocking {
			line := "- " + bl.Requirement
			if bl.Check != "" {
				line += " · `" + bl.Check + "`"
			}
			switch {
			case len(bl.Targets) > 0 && bl.State == "fail":
				line += " — fails in **" + strings.Join(bl.Targets, "**, **") + "**"
			case len(bl.Targets) > 0:
				line += " — outstanding in " + strings.Join(bl.Targets, ", ")
			case bl.Check == "":
				line += " — not yet put to the auditor"
			default:
				line += " — no run has answered it"
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// countProjects counts the estate from a project list, the same format
// run-audit.sh --projects takes: one ID per line, # comments ignored.
func countProjects(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, ln := range strings.Split(string(raw), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		n++
	}
	return n, nil
}

// reqText is the requirement as a person would read it aloud: without the
// category marker, and without the trailing link to its check. Both are
// presentation, and both would otherwise end up inside the hash that
// identifies the requirement across runs.
func reqText(s string) string {
	s = strings.TrimSpace(s)
	for _, marker := range []string{"\u2699\ufe0f", "\U0001f50d", "\U0001f465", "\U0001f5a5\ufe0f", "\u2699", "\U0001f5a5"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, marker))
	}
	if i := strings.Index(s, " \u2192 ["); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// runComplete reports whether every check in a pass reached a final verdict,
// counting an auditor's decision as final.
func runComplete(run savedRun) bool {
	for _, r := range run.Results {
		if len(r.Refs) > 0 {
			continue // resolved from its referents, never decided on its own
		}
		label := r.Verdict
		if d, ok := run.Decisions[r.ID]; ok && d.Verdict != "" {
			label = d.Verdict
		}
		switch strings.ToUpper(label) {
		case "PASS", "FAIL", "N/A", "NA", "XREF":
		default:
			return false
		}
	}
	return len(run.Results) > 0
}

// betterRun is rollup.go's supersedes rule: completeness first, then recency.
func betterRun(aComplete bool, aStamp string, bComplete bool, bStamp string) bool {
	if aComplete != bComplete {
		return aComplete
	}
	return aStamp > bStamp
}
