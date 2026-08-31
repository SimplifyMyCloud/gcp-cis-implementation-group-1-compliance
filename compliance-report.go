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
	"flag"
	"fmt"
	"os"
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
}

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
)

func main() {
	var (
		path   = flag.String("file", "docs/cis-ig1-gcp-checklist.md", "checklist to score")
		update = flag.Bool("update", false, "rewrite Status lines to match requirement boxes")
		format = flag.String("format", "text", "text | md")
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
			r := requirement{text: strings.TrimSpace(m[2])}
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
