//go:build ignore

// Standalone script. The build tag keeps it out of package builds so several
// `package main` files can share a directory. Run it directly:
//
//	go run rollup.go
//
// Command rollup reads every audit pack under a directory and produces one
// remediation plan and one compliance score.
//
// The pivot matters. A plan organised by project gives you 105 sections and
// makes a single org policy change look like 105 separate tasks. This pivots
// on the FINDING instead: one row per failing requirement, with the affected
// projects listed against it. Roughly 25 work items instead of 105 reports.
//
// Packs are found by their CONTENT, not their file name, so a run directory
// works as-is:
//
//	go run rollup.go -in scratch/runs/2026-09-21_09-47-01   # one run
//	go run rollup.go -in scratch/runs                       # the whole org
//
// Sweeping several runs, the newest COMPLETE pass wins for each target: a
// pass still holding REVIEW checks, or checks blocked by SKIP/ERROR/DENIED,
// is not finished and does not count towards any total.
//
//	go run rollup.go -in scratch/runs -out plan.md -csv plan.csv \
//	  -score-md score.md -score-json score.json
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type finding struct {
	check    string
	ref      string // "4.4#1"
	title    string
	projects []string
	org      bool // also failed at organization scope
}

type packInfo struct {
	name     string // "organization" or the project ID
	scope    string // "organization" or "project"
	status   string // OK | OK with errors | DEGRADED | UNRELIABLE
	stamp    time.Time
	org      string // the organization ID the pack was audited against
	path     string
	pass     int
	fail     int
	na       int
	review   int
	skipped  int
	denied   int
	errored  int
	findings []string // refs
}

// total is every check that ran in this pack's own pass. Rows carrying ORG or
// PROJECT belong to the other pass: the report lists them so the auditor sees
// all 188 safeguards in order, but they are not this pass's work.
func (p packInfo) total() int {
	return p.pass + p.fail + p.na + p.review + p.skipped + p.denied + p.errored
}

// passed counts N/A as a pass. A product that is not enabled cannot be
// misconfigured, so the safeguard is satisfied by default.
func (p packInfo) passed() int { return p.pass + p.na }

// complete reports whether the pass finished: every check ended PASS, FAIL or
// N/A. A REVIEW nobody reconciled, or a check blocked by SKIP, ERROR or
// DENIED, leaves it unfinished — and an unfinished pass is not a result. It
// does not count towards any total.
func (p packInfo) complete() bool {
	return p.total() > 0 && p.review == 0 && p.skipped == 0 && p.denied == 0 && p.errored == 0
}

// usable reports whether the pack's findings may be treated as findings at
// all. A run that hit permission or execution failures is excluded from the
// plan, because counting a permission failure as non-compliance invents work.
func (p packInfo) usable() bool {
	return !strings.HasPrefix(p.status, "UNRELIABLE") && !strings.HasPrefix(p.status, "DEGRADED")
}

// percents gives the three numbers, matching audit-run.go exactly: Completion
// and Pass round down so neither overstates, and Fail is the remainder of the
// completed share, so Pass + Fail equals Completion rather than drifting a
// point short.
func (p packInfo) percents() (completion, pass, fail int) {
	completion = pctOf(p.passed()+p.fail, p.total())
	pass = pctOf(p.passed(), p.total())
	return completion, pass, completion - pass
}

func pctOf(n, of int) int {
	if of == 0 {
		return 0
	}
	return n * 100 / of
}

var (
	// The verdict group takes "/" so that N/A matches; \w+ alone silently
	// dropped every N/A row.
	reRow    = regexp.MustCompile(`(?m)^\| (V\d+) \| ` + "`" + `([\d.#]+)` + "`" + ` \| \*\*([A-Z/]+)\*\* \| (.+?) \|$`)
	reStatus = regexp.MustCompile(`\*\*RUN STATUS: ([^*]+)\*\*`)
	reScope  = regexp.MustCompile(`scope: \*\*(.+?)\*\*`)
	reStamp  = regexp.MustCompile(`· (\d{4}-\d{2}-\d{2} \d{2}:\d{2})`)
	reOrg    = regexp.MustCompile("Organization `(\\d+)`")
)

func main() {
	var (
		in        = flag.String("in", "./audit-state", "directory holding the packs — a run directory, or one holding many")
		out       = flag.String("out", "remediation-plan.md", "markdown plan")
		csvOut    = flag.String("csv", "remediation-plan.csv", "CSV for the tracker")
		minProj   = flag.Int("min", 1, "only report findings affecting at least N projects")
		scoreMD   = flag.String("score-md", "", "write the compliance score here, for people to read")
		scoreJSON = flag.String("score-json", "", "write the compliance score here, for the dashboard to ingest")
		projList  = flag.String("projects", "", "the organization's project list, one ID per line — the denominator for organization-scope completion")
	)
	flag.Parse()

	packs, notes, err := loadPacks(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rollup: %v\n", err)
		os.Exit(2)
	}
	if len(packs) == 0 {
		fmt.Fprintf(os.Stderr, "rollup: no audit packs found under %s\n"+
			"  a pack is a results report — report/02-organization/01-automated-results.md\n"+
			"  or report/03-projects/<project-id>.md. Point -in at a run directory.\n", *in)
		os.Exit(2)
	}
	for _, n := range notes {
		fmt.Fprintf(os.Stderr, "superseded: %s\n", n)
	}

	findings, usable, excluded := aggregate(packs, *minProj)

	if err := writePlan(*out, findings, packs, usable, excluded); err != nil {
		fmt.Fprintf(os.Stderr, "rollup: %v\n", err)
		os.Exit(2)
	}
	if *scoreMD != "" || *scoreJSON != "" {
		total := 0
		if *projList != "" {
			n, perr := readProjectList(*projList)
			if perr != nil {
				fmt.Fprintf(os.Stderr, "rollup: %v\n", perr)
				os.Exit(2)
			}
			total = n
		}
		if err := writeScore(*scoreMD, *scoreJSON, packs, total, *in); err != nil {
			fmt.Fprintf(os.Stderr, "rollup: %v\n", err)
			os.Exit(2)
		}
	}
	if err := writeCSV(*csvOut, findings); err != nil {
		fmt.Fprintf(os.Stderr, "rollup: %v\n", err)
		os.Exit(2)
	}

	fmt.Fprintf(os.Stderr, "read %d pack(s), %d usable\n", len(packs), usable)
	if excluded > 0 {
		fmt.Fprintf(os.Stderr, "EXCLUDED %d unreliable pack(s) — see the plan\n", excluded)
	}
	for _, p := range packs {
		if !p.complete() {
			c, _, _ := p.percents()
			fmt.Fprintf(os.Stderr, "INCOMPLETE %s at %d%% — not counted in the score\n", p.name, c)
		}
	}
	fmt.Fprintf(os.Stderr, "%d distinct finding(s)\n  %s\n  %s\n", len(findings), *out, *csvOut)
}

// loadPacks finds every audit pack under root and returns one per target.
//
// Packs are recognised by CONTENT — a scope line plus at least one result row
// — not by file name. run-audit.sh files project packs as
// report/03-projects/<id>.md, so matching on 01-automated-results.md alone
// found nothing but the organization and produced a plan that looked whole
// while silently omitting every project.
//
// Where a target appears more than once, which happens as soon as the
// directory spans several runs, the newest COMPLETE pass wins. Completeness
// beats recency: a finished audit from Tuesday is a result, and a half-
// reviewed one from Wednesday is not.
func loadPacks(root string) ([]packInfo, []string, error) {
	best := map[string]packInfo{}
	supersededBy := map[string]string{}

	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(fi.Name(), ".md") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		text := string(b)

		m := reScope.FindStringSubmatch(text)
		if m == nil || !reRow.MatchString(text) {
			return nil
		}

		p := packInfo{path: path, name: strings.TrimPrefix(m[1], "project "), scope: "project"}
		if p.name == "organization" || p.name == "org" {
			p.name, p.scope = "organization", "organization"
		}
		if m := reStatus.FindStringSubmatch(text); m != nil {
			p.status = strings.TrimSpace(m[1])
		} else {
			p.status = "unknown"
		}
		if m := reOrg.FindStringSubmatch(text); m != nil {
			p.org = m[1]
		}
		p.stamp = fi.ModTime()
		if m := reStamp.FindStringSubmatch(text); m != nil {
			if t, perr := time.Parse("2006-01-02 15:04", m[1]); perr == nil {
				p.stamp = t
			}
		}

		for _, m := range reRow.FindAllStringSubmatch(text, -1) {
			switch m[3] {
			case "PASS":
				p.pass++
			case "FAIL":
				p.fail++
				p.findings = append(p.findings, m[1]+"|"+m[2]+"|"+m[4])
			case "N/A":
				p.na++
			case "REVIEW":
				p.review++
			case "SKIP":
				p.skipped++
			case "DENIED":
				p.denied++
			case "ERROR":
				p.errored++
			}
			// ORG and PROJECT belong to the other pass: listed, never counted.
		}

		prev, seen := best[p.name]
		if seen && !supersedes(p, prev) {
			supersededBy[p.path] = prev.path
			return nil
		}
		if seen {
			supersededBy[prev.path] = p.path
		}
		best[p.name] = p
		return nil
	})

	var packs []packInfo
	for _, p := range best {
		packs = append(packs, p)
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].name < packs[j].name })

	var notes []string
	for older, newer := range supersededBy {
		notes = append(notes, fmt.Sprintf("%s superseded by %s", older, newer))
	}
	sort.Strings(notes)
	return packs, notes, err
}

// supersedes decides which of two passes for the same target to count.
// A complete pass always beats an incomplete one, however old; between two
// of equal standing, the newer wins.
func supersedes(a, b packInfo) bool {
	if a.complete() != b.complete() {
		return a.complete()
	}
	return a.stamp.After(b.stamp)
}

// aggregate pivots per-pack findings into per-finding project lists, and
// excludes packs whose run was unreliable — counting a permission failure as
// a compliance finding would be worse than not reporting it.
func aggregate(packs []packInfo, min int) ([]finding, int, int) {
	byRef := map[string]*finding{}
	usable, excluded := 0, 0

	for _, p := range packs {
		if !p.usable() {
			excluded++
			continue
		}
		usable++

		for _, f := range p.findings {
			parts := strings.SplitN(f, "|", 3)
			if len(parts) != 3 {
				continue
			}
			check, ref, title := parts[0], parts[1], parts[2]
			e, ok := byRef[ref]
			if !ok {
				e = &finding{check: check, ref: ref, title: title}
				byRef[ref] = e
			}
			if p.name == "organization" || p.name == "org" {
				e.org = true
			} else {
				e.projects = append(e.projects, p.name)
			}
		}
	}

	var out []finding
	for _, f := range byRef {
		if len(f.projects) < min && !f.org {
			continue
		}
		sort.Strings(f.projects)
		out = append(out, *f)
	}
	// most-widespread first: that is where effort pays off
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].projects) != len(out[j].projects) {
			return len(out[i].projects) > len(out[j].projects)
		}
		return out[i].ref < out[j].ref
	})
	return out, usable, excluded
}

func safeguard(ref string) string { return strings.SplitN(ref, "#", 2)[0] }

func writePlan(path string, fs []finding, packs []packInfo, usable, excluded int) error {
	var b strings.Builder

	b.WriteString("# CIS IG1 — Remediation Plan\n\n")
	fmt.Fprintf(&b, "Compiled from %d audit pack(s), %d usable.\n\n", len(packs), usable)

	if excluded > 0 {
		fmt.Fprintf(&b, "> ⚠️ **%d pack(s) excluded as unreliable.** Their runs hit permission or "+
			"execution failures, so their results are not findings — counting them would "+
			"invent work that may not exist. Re-run those projects.\n\n", excluded)
		b.WriteString("| Excluded | Run status |\n|---|---|\n")
		for _, p := range packs {
			if !p.usable() {
				fmt.Fprintf(&b, "| `%s` | %s |\n", p.name, p.status)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## How to read this\n\n")
	b.WriteString("One row per **finding**, not per project. A single org policy change that fixes " +
		"40 projects is one work item here, not forty.\n\n")
	b.WriteString("**Scope** says where the work happens:\n\n")
	b.WriteString("- `org` — one change at the organization node\n")
	b.WriteString("- `project × N` — repeated per project\n")
	b.WriteString("- `org + project × N` — **both**: apply the constraint *and* clean up what already " +
		"violates it. Org policy is not retroactive, so these are two pieces of work and sizing them " +
		"as one is how a plan goes wrong.\n\n")

	b.WriteString("## Work items\n\n")
	b.WriteString("| # | Requirement | Finding | Scope | Projects |\n|---|---|---|---|---|\n")
	for i, f := range fs {
		scope := fmt.Sprintf("project × %d", len(f.projects))
		if f.org && len(f.projects) > 0 {
			scope = fmt.Sprintf("**org + project × %d**", len(f.projects))
		} else if f.org {
			scope = "org"
		}
		n := len(f.projects)
		list := strings.Join(f.projects, ", ")
		if n > 6 {
			list = strings.Join(f.projects[:6], ", ") + fmt.Sprintf(" +%d", n-6)
		}
		if n == 0 {
			list = "—"
		}
		fmt.Fprintf(&b, "| %d | `%s` | %s | %s | %s |\n", i+1, f.ref, f.title, scope, list)
	}
	b.WriteString("\n")

	// Links resolve relative to wherever the plan is written (the runbook puts
	// it in audit-state/), not to docs/. Run from the repository root.
	docs := "docs"
	absOut, err1 := filepath.Abs(filepath.Dir(path))
	absDocs, err2 := filepath.Abs("docs")
	if err1 == nil && err2 == nil {
		if rel, err := filepath.Rel(absOut, absDocs); err == nil {
			docs = filepath.ToSlash(rel)
		}
	}

	b.WriteString("## Detail\n\n")
	for i, f := range fs {
		fmt.Fprintf(&b, "### %d. `%s` — %s\n\n", i+1, f.ref, f.title)
		fmt.Fprintf(&b, "Check [%s](%s/cis-ig1-cli-validation.md#%s) · safeguard %s · "+
			"[how to fix](%s/cis-ig1-remediation-reference.md)\n\n",
			f.check, docs, strings.ToLower(f.check), safeguard(f.ref), docs)
		if f.org {
			b.WriteString("Fails at **organization** scope.\n\n")
		}
		if n := len(f.projects); n > 0 {
			fmt.Fprintf(&b, "Fails in **%d project(s)**:\n\n```\n%s\n```\n\n",
				n, strings.Join(f.projects, "\n"))
		}
	}

	b.WriteString("## Targets by finding count\n\n")
	b.WriteString("An outlier — one project failing far more than the median — usually means " +
		"something structural, such as a project created outside the folder hierarchy and " +
		"inheriting no policy.\n\n")
	b.WriteString("Pass counts N/A: a product that is not enabled cannot be misconfigured, so " +
		"the safeguard is met by default. Completion below 100% means the pass is unfinished " +
		"and does not count towards the compliance score.\n\n")
	sorted := append([]packInfo(nil), packs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].fail > sorted[j].fail })
	b.WriteString("| Target | Fail | Pass | Completion | Run |\n|---|---|---|---|---|\n")
	for _, p := range sorted {
		c, _, _ := p.percents()
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d%% | %s |\n", p.name, p.fail, p.passed(), c, p.status)
	}
	b.WriteString("\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeCSV emits one row per finding, shaped to paste into the tracker as a
// new tab. Columns match the tracker's own ordering where they overlap.
func writeCSV(path string, fs []finding) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{
		"Req ID", "Safeguard", "Check", "Finding", "Scope",
		"Project count", "Projects", "Status", "Owner", "PR / Ticket", "Notes",
	}); err != nil {
		return err
	}

	for _, x := range fs {
		scope := fmt.Sprintf("project x %d", len(x.projects))
		if x.org && len(x.projects) > 0 {
			scope = fmt.Sprintf("org + project x %d", len(x.projects))
		} else if x.org {
			scope = "org"
		}
		if err := w.Write([]string{
			x.ref, safeguard(x.ref), x.check, x.title, scope,
			fmt.Sprint(len(x.projects)), strings.Join(x.projects, " "),
			"Not started", "", "", "",
		}); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Compliance score
// ---------------------------------------------------------------------------
//
// Two readers, two files. The JSON is ingested by the compliance dashboard;
// the markdown is for whoever opens the run directory. Both carry the same
// numbers, and the JSON carries raw counts as well as percentages so the
// dashboard never has to re-derive a rounded figure.
//
// The score reflects where it sits. A run directory holding one project
// scores that project. A directory spanning the whole estate scores the
// organization: Pass and Fail are compiled across every counted target, and
// Completion is how much of the estate has been audited — projects finished
// over projects in the organization.
//
// Only a COMPLETE pass counts. An audit still holding REVIEW checks has not
// been reconciled, and one holding SKIP, ERROR or DENIED checks did not
// finish; neither is a result, so neither contributes to a total.

type scoreChecks struct {
	Total  int `json:"total"`
	Pass   int `json:"pass"` // PASS + N/A
	Fail   int `json:"fail"`
	Ran    int `json:"pass_ran"`
	NA     int `json:"pass_na"`
	Review int `json:"review"`
	Skip   int `json:"skip"`
	Denied int `json:"denied"`
	Error  int `json:"error"`
}

type scorePct struct {
	Completion int `json:"completion"`
	Pass       int `json:"pass"`
	Fail       int `json:"fail"`
}

type scoreCoverage struct {
	ProjectsAudited int  `json:"projects_audited"`
	ProjectsCounted int  `json:"projects_counted"`
	ProjectsTotal   int  `json:"projects_total"`
	TotalKnown      bool `json:"projects_total_known"`
}

type scoreTarget struct {
	Name      string      `json:"name"`
	Scope     string      `json:"scope"`
	Complete  bool        `json:"complete"`
	Counted   bool        `json:"counted"`
	RunStatus string      `json:"run_status"`
	Audited   string      `json:"audited"`
	Source    string      `json:"source"`
	Checks    scoreChecks `json:"checks"`
	Score     scorePct    `json:"score"`
}

type scoreDoc struct {
	Schema       string         `json:"schema"`
	Generated    string         `json:"generated"`
	Organization string         `json:"organization,omitempty"`
	Scope        string         `json:"scope"`
	Target       string         `json:"target"`
	Complete     bool           `json:"complete"`
	Checks       scoreChecks    `json:"checks"`
	Score        scorePct       `json:"score"`
	Coverage     *scoreCoverage `json:"coverage,omitempty"`
	Targets      []scoreTarget  `json:"targets"`
}

func checksOf(p packInfo) scoreChecks {
	return scoreChecks{
		Total: p.total(), Pass: p.passed(), Fail: p.fail,
		Ran: p.pass, NA: p.na, Review: p.review,
		Skip: p.skipped, Denied: p.denied, Error: p.errored,
	}
}

func (c scoreChecks) add(o scoreChecks) scoreChecks {
	return scoreChecks{
		Total: c.Total + o.Total, Pass: c.Pass + o.Pass, Fail: c.Fail + o.Fail,
		Ran: c.Ran + o.Ran, NA: c.NA + o.NA, Review: c.Review + o.Review,
		Skip: c.Skip + o.Skip, Denied: c.Denied + o.Denied, Error: c.Error + o.Error,
	}
}

// buildScore assembles the document. projectsTotal is the number of projects
// in the organization; 0 means nobody supplied it, and the organization's
// Completion is then not knowable and is reported as such rather than guessed
// from the projects that happen to be present.
func buildScore(packs []packInfo, projectsTotal int, root string) scoreDoc {
	doc := scoreDoc{Schema: "cis-ig1-score/1", Generated: time.Now().UTC().Format(time.RFC3339)}

	var projects, counted, countedProjects int
	var totals scoreChecks
	allComplete := true

	for _, p := range packs {
		c, pa, f := p.percents()
		t := scoreTarget{
			Name: p.name, Scope: p.scope, Complete: p.complete(),
			Counted:   p.complete() && p.usable(),
			RunStatus: p.status, Audited: p.stamp.Format(time.RFC3339),
			Source: relTo(root, p.path),
			Checks: checksOf(p),
			Score:  scorePct{Completion: c, Pass: pa, Fail: f},
		}
		doc.Targets = append(doc.Targets, t)
		if doc.Organization == "" {
			doc.Organization = p.org
		}
		if p.scope == "project" {
			projects++
		}
		if !t.Counted {
			allComplete = false
			continue
		}
		counted++
		if p.scope == "project" {
			countedProjects++
		}
		totals = totals.add(t.Checks)
	}

	// Scope follows the contents: one project on its own is a project score.
	if len(packs) == 1 && packs[0].scope == "project" {
		p := packs[0]
		c, pa, f := p.percents()
		doc.Scope, doc.Target = "project", p.name
		doc.Complete = p.complete() && p.usable()
		doc.Checks = checksOf(p)
		doc.Score = scorePct{Completion: c, Pass: pa, Fail: f}
		return doc
	}

	doc.Scope, doc.Target = "organization", doc.Organization
	doc.Checks = totals
	doc.Complete = allComplete && counted > 0

	// Pass and Fail are compiled from every counted target's checks.
	passPct := pctOf(totals.Pass, totals.Total)
	failPct := 0
	if totals.Total > 0 {
		// Every counted target finished, so Pass and Fail divide the whole of
		// it. Taking Fail as the remainder keeps them summing to 100 instead
		// of drifting a point short, exactly as audit-run.go does per pass.
		failPct = 100 - passPct
	}
	// Completion at organization scope is estate coverage, not check
	// coverage: how many projects have a finished audit behind them.
	cov := scoreCoverage{
		ProjectsAudited: projects,
		ProjectsCounted: countedProjects,
		ProjectsTotal:   projectsTotal,
		TotalKnown:      projectsTotal > 0,
	}
	doc.Coverage = &cov
	if cov.TotalKnown {
		doc.Score = scorePct{Completion: pctOf(countedProjects, projectsTotal), Pass: passPct, Fail: failPct}
	} else {
		doc.Score = scorePct{Completion: -1, Pass: passPct, Fail: failPct}
	}
	return doc
}

// writeScore emits the score as JSON, markdown, or both. A completion of -1
// at organization scope means the estate's project total was never supplied,
// so coverage is not knowable — reported rather than guessed, because a
// dashboard cannot tell a guessed 100% from a real one.
func writeScore(mdPath, jsonPath string, packs []packInfo, projectsTotal int, root string) error {
	doc := buildScore(packs, projectsTotal, root)

	if jsonPath != "" {
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(jsonPath, append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	if mdPath != "" {
		if err := os.WriteFile(mdPath, []byte(renderScore(doc)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func renderScore(doc scoreDoc) string {
	var b strings.Builder

	b.WriteString("# 4. Compliance Score\n\n")
	if doc.Organization != "" {
		fmt.Fprintf(&b, "Organization `%s` · ", doc.Organization)
	}
	fmt.Fprintf(&b, "scope **%s**", doc.Scope)
	if doc.Scope == "project" {
		fmt.Fprintf(&b, " `%s`", doc.Target)
	}
	fmt.Fprintf(&b, " · compiled %s\n\n", doc.Generated)

	c := doc.Checks
	if doc.Scope == "project" {
		fmt.Fprintf(&b, "> **SCORE: completion %d%% · pass %d%% · fail %d%%**\n\n",
			doc.Score.Completion, doc.Score.Pass, doc.Score.Fail)
		b.WriteString("| | | |\n|---|---|---|\n")
		fmt.Fprintf(&b, "| **Completion** | **%d%%** | %d of %d checks are final — PASS, FAIL or N/A |\n",
			doc.Score.Completion, c.Pass+c.Fail, c.Total)
		fmt.Fprintf(&b, "| **Pass** | **%d%%** | %d checks: %d PASS, %d N/A |\n", doc.Score.Pass, c.Pass, c.Ran, c.NA)
		fmt.Fprintf(&b, "| **Fail** | **%d%%** | %d checks |\n\n", doc.Score.Fail, c.Fail)
		if !doc.Complete {
			b.WriteString("> ⚠️ **This audit is not complete and does not count towards any total.** " +
				"Reconcile the REVIEW checks with `./run-audit.sh --review <run directory>`; " +
				"a check left SKIP, ERROR or DENIED must be re-run.\n\n")
		}
		return b.String()
	}

	// Organization scope.
	if doc.Coverage != nil && doc.Coverage.TotalKnown {
		fmt.Fprintf(&b, "> **SCORE: completion %d%% · pass %d%% · fail %d%%**\n\n",
			doc.Score.Completion, doc.Score.Pass, doc.Score.Fail)
	} else {
		fmt.Fprintf(&b, "> **SCORE: completion unknown · pass %d%% · fail %d%%**\n\n", doc.Score.Pass, doc.Score.Fail)
	}
	b.WriteString("| | | |\n|---|---|---|\n")
	if doc.Coverage != nil {
		cov := *doc.Coverage
		if cov.TotalKnown {
			fmt.Fprintf(&b, "| **Completion** | **%d%%** | %d of %d projects in the organization have a finished audit |\n",
				doc.Score.Completion, cov.ProjectsCounted, cov.ProjectsTotal)
		} else {
			fmt.Fprintf(&b, "| **Completion** | **unknown** | %d project(s) finished; the organization's project total was not supplied (`-projects FILE`) |\n",
				cov.ProjectsCounted)
		}
	}
	fmt.Fprintf(&b, "| **Pass** | **%d%%** | %d of %d checks across every counted target: %d PASS, %d N/A |\n",
		doc.Score.Pass, c.Pass, c.Total, c.Ran, c.NA)
	fmt.Fprintf(&b, "| **Fail** | **%d%%** | %d checks |\n\n", doc.Score.Fail, c.Fail)

	b.WriteString("Pass and Fail are compiled across every **counted** target — those whose audit " +
		"finished. Completion is estate coverage: audited projects over projects in the " +
		"organization. N/A counts as a pass: a product that is not enabled cannot be " +
		"misconfigured.\n\n")

	b.WriteString("## Targets\n\n")
	b.WriteString("| Target | Scope | Counted | Completion | Pass | Fail | Audited |\n|---|---|---|---|---|---|---|\n")
	for _, t := range doc.Targets {
		counted := "yes"
		if !t.Counted {
			counted = "**no**"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d%% | %d%% | %d%% | %s |\n",
			t.Name, t.Scope, counted, t.Score.Completion, t.Score.Pass, t.Score.Fail, t.Audited)
	}
	b.WriteString("\n")

	var pending []string
	for _, t := range doc.Targets {
		if !t.Counted {
			pending = append(pending, t.Name)
		}
	}
	if len(pending) > 0 {
		fmt.Fprintf(&b, "> ⚠️ **%d target(s) are not counted** because their audit is unfinished: `%s`. "+
			"An audit holding REVIEW checks has not been reconciled; one holding SKIP, ERROR or "+
			"DENIED checks did not finish.\n\n", len(pending), strings.Join(pending, "`, `"))
	}
	return b.String()
}

// readProjectList counts the projects in the organization from a list file in
// the format run-audit.sh --projects takes: one ID per line, # comments and
// blank lines ignored.
func readProjectList(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		n++
	}
	return n, nil
}

// relTo names a pack by where it sits under the directory scanned, so the
// score file carries "report/03-projects/x.md" rather than somebody's home
// directory.
func relTo(root, path string) string {
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}
