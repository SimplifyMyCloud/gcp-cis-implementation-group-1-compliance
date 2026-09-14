//go:build ignore

// Standalone script. The build tag keeps it out of package builds so several
// `package main` files can share a directory. Run it directly:
//
//	go run rollup.go
//
// Command rollup reads every audit pack under a directory and produces one
// remediation plan.
//
// The pivot matters. A plan organised by project gives you 105 sections and
// makes a single org policy change look like 105 separate tasks. This pivots
// on the FINDING instead: one row per failing requirement, with the affected
// projects listed against it. Roughly 25 work items instead of 105 reports.
//
//	go run rollup.go -in ./audit-state -out plan.md -csv plan.csv
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
	status   string // OK | OK with errors | DEGRADED | UNRELIABLE
	pass     int
	fail     int
	denied   int
	errored  int
	findings []string // refs
}

var (
	reRow    = regexp.MustCompile(`(?m)^\| (V\d+) \| ` + "`" + `([\d.#]+)` + "`" + ` \| \*\*(\w+)\*\* \| (.+?) \|$`)
	reStatus = regexp.MustCompile(`\*\*RUN STATUS: ([^*]+)\*\*`)
	reScope  = regexp.MustCompile(`scope: \*\*(.+?)\*\*`)
)

func main() {
	var (
		in      = flag.String("in", "./audit-state", "directory holding the packs")
		out     = flag.String("out", "remediation-plan.md", "markdown plan")
		csvOut  = flag.String("csv", "remediation-plan.csv", "CSV for the tracker")
		minProj = flag.Int("min", 1, "only report findings affecting at least N projects")
	)
	flag.Parse()

	packs, err := loadPacks(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rollup: %v\n", err)
		os.Exit(2)
	}
	if len(packs) == 0 {
		fmt.Fprintf(os.Stderr, "rollup: no packs found under %s\n"+
			"  expected 01-automated-results.md files, e.g. %s/org/ and %s/projects/<id>/\n",
			*in, *in, *in)
		os.Exit(2)
	}

	findings, usable, excluded := aggregate(packs, *minProj)

	if err := writePlan(*out, findings, packs, usable, excluded); err != nil {
		fmt.Fprintf(os.Stderr, "rollup: %v\n", err)
		os.Exit(2)
	}
	if err := writeCSV(*csvOut, findings); err != nil {
		fmt.Fprintf(os.Stderr, "rollup: %v\n", err)
		os.Exit(2)
	}

	fmt.Fprintf(os.Stderr, "read %d pack(s), %d usable\n", len(packs), usable)
	if excluded > 0 {
		fmt.Fprintf(os.Stderr, "EXCLUDED %d unreliable pack(s) — see the plan\n", excluded)
	}
	fmt.Fprintf(os.Stderr, "%d distinct finding(s)\n  %s\n  %s\n", len(findings), *out, *csvOut)
}

func loadPacks(root string) ([]packInfo, error) {
	var packs []packInfo

	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || fi.Name() != "01-automated-results.md" {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		text := string(b)

		p := packInfo{name: filepath.Base(filepath.Dir(path))}
		if m := reScope.FindStringSubmatch(text); m != nil {
			p.name = strings.TrimPrefix(m[1], "project ")
		}
		if m := reStatus.FindStringSubmatch(text); m != nil {
			p.status = strings.TrimSpace(m[1])
		} else {
			p.status = "unknown"
		}

		for _, m := range reRow.FindAllStringSubmatch(text, -1) {
			switch m[3] {
			case "PASS":
				p.pass++
			case "FAIL":
				p.fail++
				p.findings = append(p.findings, m[1]+"|"+m[2]+"|"+m[4])
			case "DENIED":
				p.denied++
			case "ERROR":
				p.errored++
			}
		}
		packs = append(packs, p)
		return nil
	})

	sort.Slice(packs, func(i, j int) bool { return packs[i].name < packs[j].name })
	return packs, err
}

// aggregate pivots per-pack findings into per-finding project lists, and
// excludes packs whose run was unreliable — counting a permission failure as
// a compliance finding would be worse than not reporting it.
func aggregate(packs []packInfo, min int) ([]finding, int, int) {
	byRef := map[string]*finding{}
	usable, excluded := 0, 0

	for _, p := range packs {
		if strings.HasPrefix(p.status, "UNRELIABLE") || strings.HasPrefix(p.status, "DEGRADED") {
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
			if strings.HasPrefix(p.status, "UNRELIABLE") || strings.HasPrefix(p.status, "DEGRADED") {
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

	b.WriteString("## Projects by finding count\n\n")
	b.WriteString("An outlier — one project failing far more than the median — usually means " +
		"something structural, such as a project created outside the folder hierarchy and " +
		"inheriting no policy.\n\n")
	sorted := append([]packInfo(nil), packs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].fail > sorted[j].fail })
	b.WriteString("| Project | Fail | Pass | Run |\n|---|---|---|---|\n")
	for _, p := range sorted {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %s |\n", p.name, p.fail, p.pass, p.status)
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
