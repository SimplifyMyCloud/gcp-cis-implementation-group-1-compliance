# Selling the IG1 GCP Audit
### Internal enablement — Resident Architect team

*Internal deck — 12 slides, ~25 minutes with discussion. The `.pptx` in this directory is the same content, built to present.*

---

## 1 — Title

**Selling the IG1 GCP Audit**
What it is, who buys it, and what not to promise

Google Public Sector · Resident Architect enablement
**Internal**

> Notes: Audience is fellow RAs. Assume they know GCP cold and have never read CIS.

---

## 2 — The one-sentence version

### Your customer already has the data. We measure it instead of asking them about it.

188 of the 290 IG1 requirements are answerable by `gcloud` against their own estate, read-only, today.

That turns a six-week questionnaire assessment into **two days of automated passes plus adjudication.**

> Notes: If they remember one slide, make it this one.

---

## 3 — What IG1 actually is

**CIS Controls v8.1, Implementation Group 1 — 56 safeguards.**

- CIS's definition of *essential cyber hygiene*. The floor
- Mapped to NIST CSF 2.0 — which is what public sector customers are already being measured against
- **44 are GCP-actionable.** 12 are off-platform: 4 endpoint management, 8 security awareness training
- 56 Safeguards, 18 Controls, unchanged in v8.1 (June 2024)

**Not the CIS GCP Benchmark.** Different document, different scope. Know the difference cold — you will be asked.

> Notes: v8.1 added the Govern function and a Documentation asset class. IG1 count did not move.

---

## 4 — Trigger events

**When this sells itself:**

- A new ATO package, or a FedRAMP / StateRAMP gap
- A fresh CISO in the first 90 days
- A landing-zone or org-restructure project
- An audit finding they need to show movement against
- A board question nobody could answer

**When to wait:** mid-incident, mid-reorg, or when they have no platform team to hand remediation to.

> Notes: The "fresh CISO" one is the highest-conversion trigger by a wide margin.

---

## 5 — Who to pitch, and the line

| Buyer | The line |
|---|---|
| **CISO** | A defensible number for your board, from your own control plane — not a questionnaire |
| **Platform lead** | A remediation list pivoted on the fix. One org policy closes forty projects as one item |
| **Compliance / ATO** | Evidence per requirement, per project, with the command that produced it recorded |
| **Program exec** | Baseline in a week, measurable delta on every re-run |

Pitch the CISO for the mandate. Pitch the platform lead for the service account.

> Notes: You need both. The CISO says yes; the platform lead is who actually unblocks you.

---

## 6 — Promise this

- **Read-only.** No agent, no key, no change window
- **Three numbers** per project and per org: completion, pass, fail
- **A ranked remediation plan** pivoted on the finding
- **A stable JSON feed** their own dashboard ingests
- **Repeatable.** Re-run after remediation; the delta is the proof

Lead with read-only. It is what clears their security review, and clearing security review fast is what gets you the service account in week one instead of week five.

> Notes: Every engagement that stalled, stalled on access. Front-load it.

---

## 7 — Never promise this

### Three things that will bite you

**A certification.** CIS sets **no pass mark** for IG1. Say this in meeting one. It builds credibility and costs you nothing.

**A number without the interview.** 102 of 290 requirements are process questions. Skip them and the denominator is dishonest — and the kit will refuse to score them.

**Remediation.** Scope it separately. Conflating audit and remediation is how a two-week engagement becomes six months with no baseline.

> Notes: All three of these are learned the hard way. Say them plainly.

---

## 8 — Engagement shape

| Phase | What | Who | Effort |
|---|---|---|---|
| **0 Access** | Read-only SA, 17 APIs in one project | Their engineer | ~1 hr |
| **1 Automated** | Org pass (86 checks), project passes (103 each), parallelised | You | Hours, unattended |
| **2 Interview** | 114 questions, once for the org | You + security lead | Half a day |
| **3 Review** | Adjudicate every REVIEW verdict | You | **The long pole** |
| **4 Compile** | Rollup, score, plan, JSON feed | You | Minutes |

> Notes: Walk the table. Then land slide 9, which is the thing people under-scope.

---

## 9 — Scope phase 3 honestly

### Only 5 of 44 GCP safeguards are fully automatable

The other 39 need a human to look at what the command returned.

- **96** requirements answered mechanically
- **92** requirements where a command gathers evidence and *you* judge
- **30** console-only
- **72** process

On a 35-project estate, adjudication **is** the engagement. Everything else is hours.

> Notes: This is the slide that keeps RAs from over-promising a one-week delivery.

---

## 10 — Objection handling

**"We have Security Command Center."**
SCC finds misconfigurations. It does not say which of 56 safeguards you meet. They compose — SCC findings are often the *evidence* for a safeguard.

**"Will it touch production?"**
Every check reads. Show them the command list — it is in the repo and parsed at runtime, so what you show is literally what runs.

**"Why not the CIS GCP Benchmark?"**
The Benchmark hardens GCP. IG1 covers the enterprise — inventory, training, IR. You can pass the Benchmark and fail IG1 badly.

**"Can we run it ourselves next quarter?"**
Yes — and want that. Leave them the repo. The re-run is where they see progress, and progress is what brings you back.

> Notes: Have all four of these ready verbatim. They come up every time.

---

## 11 — The teardown line

### Ask them to delete the service account when you finish.

A standing org-wide read role is itself an IG1 finding — it costs them points under 5.1, 5.4 and 6.2.

Saying this out loud is the single most credibility-building moment in the engagement.

It also means you never hold standing access to a customer estate. Good for them, good for you.

> Notes: Do not skip this. It is also just correct.

---

## 12 — What you are handing over

**The whole kit is one repo.**

- One entry point: `run-audit.sh`
- Checks live as markdown, parsed at runtime — readable by the customer's security team
- Scoring in Go, standard library only. No dependencies, no build step, no license, nothing that expires

**Run a pilot this week:** clone it, read `docs/01-setup.md`, point it at a sandbox org.

Then bring what you learn back to the team.

> Notes: Close by asking who wants to run the next pilot.

---

*CIS Controls® is a registered trademark of the Center for Internet Security.*
