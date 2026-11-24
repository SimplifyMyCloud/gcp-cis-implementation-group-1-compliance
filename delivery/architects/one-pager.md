# IG1 GCP Audit — Resident Architect Sell Sheet

**Internal. For Google Public Sector Resident Architects.** How to position this, what to promise, and what the engagement actually looks like.

## Why this lands

Public sector customers are asked for a security baseline and have nothing to point at. CIS IG1 is the easiest honest answer: recognised, NIST CSF 2.0 mapped, and only 56 safeguards — small enough that a customer can believe they will finish it.

The pitch is not "let us assess you." It is **"you already have the data; let's measure it."** 188 of the 290 requirements are answerable by `gcloud` against their own estate, today, read-only. That turns a six-week consulting assessment into a two-day audit plus adjudication.

## Who to pitch it to

| Buyer | The line that works |
|---|---|
| **CISO / CISO staff** | "A defensible number for your board, from your own control plane, not a questionnaire" |
| **Cloud platform lead** | "A remediation list pivoted on the fix, so one org policy closes forty projects as one work item" |
| **Compliance / ATO team** | "Evidence per requirement, per project, with the command that produced it recorded" |
| **Program exec under an audit finding** | "Baseline in a week, progress measurable every re-run" |

Strongest trigger events: a new ATO package, a FedRAMP or StateRAMP gap, a fresh CISO, a landing-zone build, or an audit finding they need to show movement against.

## What to promise — and what not to

**Promise:**
- Read-only. No agent, no key, no change window. This clears security review fast, and saying it early is what gets you the service account
- Three numbers per project and per org: completion, pass, fail
- A ranked remediation plan pivoted on the finding
- A stable JSON feed their own dashboard can ingest
- Repeatable: re-run after remediation and the delta is the proof of work

**Do not promise:**
- A certification. CIS sets **no pass mark** for IG1. Say this in the first meeting — it builds credibility and it is the one thing that will bite you later if you skip it
- A number without the interview. 102 of 290 requirements are process questions. Without them the denominator is dishonest, and the kit will tell you so
- Remediation. Scope that separately. Conflating them is how a two-week audit becomes a six-month project with no baseline

## Engagement shape

| Phase | What happens | Who | Effort |
|---|---|---|---|
| **0. Access** | Customer creates a read-only service account, grants it, enables 17 APIs in one project | Their cloud engineer | ~1 hour |
| **1. Automated passes** | Org pass (86 checks), then one pass per project (103 checks). Parallelised | You, from Cloud Shell | Hours, unattended |
| **2. Interview** | 114 structured questions covering the 102 process requirements. Answered once for the org | You + their security lead | Half a day |
| **3. Review** | Adjudicate every `REVIEW` verdict to PASS/FAIL/N\/A. This is the real work | You | The long pole |
| **4. Compile** | Rollup, score, remediation plan, JSON feed | You | Minutes |

**Scale reality check:** only **5 of the 44 GCP safeguards** are fully automatable end to end. The other 39 need a human to look at what the command returned. Scope phase 3 honestly — on a 35-project estate it is the bulk of the engagement.

## Objection handling

**"We already have Security Command Center."**
SCC finds misconfigurations. It does not tell you which of 56 safeguards you meet. Different question, and the two compose — SCC findings often *are* the evidence for a safeguard.

**"Will this touch production?"**
Every check reads. Show them the command list; it is in the repo and parsed at runtime, so what you show them is literally what runs.

**"Why not just use the CIS GCP Benchmark?"**
The Benchmark hardens GCP. IG1 covers the *enterprise* — inventory, access, training, incident response. A customer can pass the Benchmark and still fail IG1 badly, and IG1 is what their board and their auditor ask about.

**"Who keeps the service account?"**
They do, and we ask them to delete it when we finish. A standing org-wide read role is itself an IG1 finding (5.1, 5.4, 6.2), so leaving it in place costs them points. Say that out loud — it is the single most credibility-building line in the deck.

**"Can we run it ourselves next quarter?"**
Yes, and you should want that. It is Go stdlib and bash, no dependencies, no build step, no license. Leave them the repo. The re-run is where they see progress, and that is what brings you back.

## What you are handing over

The whole kit is this repo: one entry point (`run-audit.sh`), the checks as parsed markdown, the scoring in Go with no third-party code. Nothing to install, nothing to buy, nothing that expires. Clone it, read [docs/01-setup.md](../../docs/01-setup.md), and you can run a pilot this week.

---

*CIS Controls® is a registered trademark of the Center for Internet Security.*
