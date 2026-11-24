# CIS Controls IG1 on Google Cloud
### Measuring your security baseline from your own control plane

*Customer briefing deck — 12 slides, ~20 minutes. The `.pptx` in this directory is the same content, built to present.*

---

## 1 — Title

**CIS Controls IG1 on Google Cloud**
Measuring your security baseline from your own control plane

Google Public Sector · Resident Architect team

> Notes: This is a 20-minute briefing. The ask at the end is one read-only service account.

---

## 2 — The problem

You are asked for a security baseline.

- The board asks "are we secure?"
- The auditor asks "against what standard?"
- The honest answer today is a spreadsheet somebody filled in from memory

**A baseline you asserted is not a baseline. It is a hope.**

> Notes: Don't linger. Everyone in the room already knows this.

---

## 3 — Why IG1

**56 safeguards. The floor, not the ceiling.**

| | |
|---|---|
| **Recognised** | CIS Controls v8.1, mapped to NIST CSF 2.0 |
| **Essential** | CIS defines IG1 as *essential cyber hygiene* — the minimum expected of any enterprise |
| **Finishable** | 56 safeguards, not 153. Small enough to actually complete |

Not a maturity model. Not aspirational. The things a reasonable organisation is expected to have.

> Notes: v8.1 shipped June 2024. It added the Govern function from CSF 2.0 and a Documentation asset class. IG1 is still 56 safeguards.

---

## 4 — The key insight

### 188 of 290 requirements are already answerable

Under those 56 safeguards sit **290 GCP-applicable requirements**.

**188 of them have a `gcloud` command that answers the question.**

Your control plane already knows whether logging is on, whether MFA is enforced, whether that bucket is public.

**We do not need to interview you about the things your API can tell us.**

> Notes: This is the whole pitch. Everything after this is mechanics.

---

## 5 — What runs

**Read-only. No agent. No key.**

| | |
|---|---|
| **Read-only** | Every check reads. Nothing is created, changed or deleted |
| **No keys** | Runs as a service account your team creates, impersonated per call. No key file is ever generated — so none can leak |
| **No agents** | Nothing installed in your estate. No VM, no daemon, no sidecar |
| **Your identity** | You create it, you grant it, you destroy it. We ask you to remove it when we finish |

**86 checks** against the organization · **103 checks** against each project

> Notes: The no-key point is what gets this through their security review. Lead with it.

---

## 6 — What a human still has to do

**Automation gets you most of the way, and we say where it stops.**

- **96 requirements** a command answers mechanically — on or off
- **92 requirements** a command gathers evidence for, and a human judges
- **30 requirements** that need a console no API covers
- **72 requirements** that are process questions — is it documented, is it reviewed, who owns it

Only **5 of 44** GCP safeguards are fully automatable end to end.

**We will not pretend a tool judged what a person judged.**

> Notes: Counter-intuitive slide to include, and the reason they will trust the number.

---

## 7 — The interview

**102 requirements have no technical check.**

*Is there a documented incident response process? Is the asset inventory recertified? Does everyone get security awareness training?*

- 114 structured questions
- Answered **once** for the organization, not per project
- Half a day with your security lead

Fold these in and the denominator is honestly **56 of 56** — not "the 44 we could automate."

> Notes: If they want to skip this, the score is dishonest and the kit says so.

---

## 8 — What you get: three numbers

### Per project, and per the organization

**Completion** — share of checks that reached a final verdict. 100% means finished

**Pass** — safeguards met

**Fail** — safeguards not met

Anything below 100% completion is **excluded from the report entirely.** A partial audit is not a result.

> Notes: The exclusion rule matters — it's why the number can be trusted.

---

## 9 — What you get: a plan, not a list

**Pivoted on the finding, not the project.**

One organization policy change that fixes forty projects is **one work item** — not forty.

On a recent engagement:

### ~300 raw findings → ~25 pieces of work

Ranked by how many projects each one clears.

> Notes: This is what the platform lead cares about. The CISO cares about slide 8; the engineer cares about this one.

---

## 10 — What you get: a feed, not a slide

**Stable JSON, so it lands in your dashboard.**

- Two feeds: organization and projects
- Versioned schema
- Per-safeguard status, with the failing requirement named and the projects listed
- Re-run after remediation; the delta is the proof of work

A number in a deck is stale the day after the meeting. A feed is not.

> Notes: On the last engagement this went straight into BigQuery and Looker.

---

## 11 — What it costs you

| | |
|---|---|
| **Setup** | Read-only service account, 17 APIs in one project. ~1 hour of a cloud engineer |
| **Your input** | Half a day of your security lead, for the interview |
| **Elapsed** | Driven by estate size. The automated passes are hours; adjudication is the long pole |
| **Disruption** | None. No change window, no downtime, no agent |

**What we will not claim:** CIS sets no pass mark for IG1. A percentage is progress, not a certification — and an audit measures, it does not remediate.

> Notes: Say the "no pass mark" line out loud. It is the most credibility-building sentence in the deck.

---

## 12 — The ask

### One read-only service account.

1. Your engineer creates it and enables 17 APIs — about an hour
2. We run the organization pass and the project passes
3. Half a day with your security lead for the interview
4. You get three numbers, a ranked plan, and a feed

**Then delete the service account.** A standing org-wide read role is itself an IG1 finding.

> Notes: End on the teardown. It proves you meant the read-only part.

---

*CIS Controls® is a registered trademark of the Center for Internet Security. This audit is an independent implementation and is not a CIS certification.*
