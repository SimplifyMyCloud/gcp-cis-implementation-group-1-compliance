# Delivery bundle

Everything needed to pitch, scope and hand over a CIS Controls v8.1 IG1 audit of a Google Cloud estate.

Two audiences, two bundles. Each has a one-pager and a deck, and each deck ships as **markdown** (read it, paste it, diff it) and **`.pptx`** (present it).

| | Reach for it when |
|---|---|
| [customer/one-pager.md](customer/one-pager.md) | Leave-behind for a CISO or their staff. What IG1 is, what the audit does, what they get, what it costs them |
| [customer/deck.md](customer/deck.md) · [customer/deck.pptx](customer/deck.pptx) | 20-minute customer briefing. 12 slides, benefits-focused, ends on the one ask: a read-only service account |
| [architects/one-pager.md](architects/one-pager.md) | Internal. Who to pitch, trigger events, what to promise and what never to promise, engagement shape, objection handling |
| [architects/deck.md](architects/deck.md) · [architects/deck.pptx](architects/deck.pptx) | 25-minute internal enablement session for fellow Resident Architects. How to sell it and how to scope it honestly |
| [verify-numbers.sh](verify-numbers.sh) | Re-derives every number the bundle claims. Run it before a meeting if the kit has changed |

The markdown and the `.pptx` of a deck hold the same content, slide for slide, with the speaker notes in both. The markdown is the source of record — read it, diff it, paste it into mail. The `.pptx` is an ordinary PowerPoint file: edit it directly for a specific customer, but push anything worth keeping back into the markdown so the next architect inherits it.

## Before you present either deck

Both decks state numbers that come from this repo, not from a brochure:

| Claim | Where it comes from |
|---|---|
| 56 IG1 safeguards; 44 GCP-actionable, 12 off-platform | [docs/cis-ig1-gcp-checklist.md](../docs/cis-ig1-gcp-checklist.md) |
| 290 requirements; 188 automated, 102 manual | [docs/cis-ig1-cli-validation.md](../docs/cis-ig1-cli-validation.md) |
| 86 checks per organization pass, 103 per project pass | `./run-audit.sh --gcp-org`, `--project` |
| 114 interview questions | `./run-audit.sh --interview` |
| Only 5 of 44 safeguards fully automatable | [docs/cis-ig1-gcp-checklist.md](../docs/cis-ig1-gcp-checklist.md) |

If the kit changes, re-check these before the meeting:

```bash
./delivery/verify-numbers.sh
```

It re-derives all seventeen claims from the documents and the tools that produce them, prints which matched, and exits non-zero if any no longer hold. Read-only — it runs no `gcloud` and touches no estate. A number you cannot source is worse than no number.

## The three things not to promise

1. **A certification.** CIS sets no pass mark for IG1. A percentage is progress against "implement every safeguard"
2. **A score without the interview.** 102 of 290 requirements are process questions; without them the denominator is dishonest
3. **Remediation.** The audit measures. Scope the fixing separately

## Running a pilot

Read [docs/01-setup.md](../docs/01-setup.md), point the kit at a sandbox organization, and run all four phases end to end before you pitch it to anyone. The deck is far easier to present once you have seen the output.

---

*CIS Controls® is a registered trademark of the Center for Internet Security. This kit is an independent implementation and is not a CIS certification.*
