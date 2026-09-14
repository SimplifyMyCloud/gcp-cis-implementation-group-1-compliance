# NIST 800 & CIS IG1 for SREs — 20 minute class

One file per section. Each `##` is roughly one slide if you build a deck, and the whole thing reads as a document if you'd rather just walk the repo page.

**Audience:** our own SREs, who will run and report on IG1 compliance. Deep GCP knowledge, no compliance background. Internal session — no customer present.

**The thesis, and the only thing they must leave with:** IG1 is not new work. It is a naming convention for the GCP hygiene they already argue for in design reviews. What they lack is the vocabulary to say it in a language auditors and executives accept.

Do not teach compliance. Teach translation.

## Running order

| # | Section | Roughly |
|---|---|---|
| 1 | [What this is](01-why-this-matters.md) | 2 min |
| 2 | [The framework landscape](02-framework-landscape.md) | 3 min |
| 3 | [How CIS is structured](03-cis-structure.md) | 2 min |
| 4 | [**You already do this**](04-you-already-do-this.md) | **6 min** |
| 5 | [Four things that surprise engineers](05-what-surprises-engineers.md) | 3 min |
| 6 | [How we actually run it](06-how-we-run-it.md) | 3 min |
| 7 | [What we need from you](07-what-we-need-from-you.md) | 1 min |

Timings are here for planning only — they are not printed on the sections, so this reads as a document if you present the repo page directly rather than building slides.

[`08-reference-card.md`](08-reference-card.md) is a one-page handout, not slides. Send it after.

[`09-process-interview.md`](09-process-interview.md) is not part of the class. It is the 72 process-and-people requirements turned into a worksheet — take it into a room with whoever has history on the account and work through them out loud. Fastest way to close the group no script can touch.

## Delivery notes

**Section 4 is the class.** Everything before it is setup, everything after is logistics. If you are running long, cut from 2 and 6 — never from 4.

**Do not read the safeguard numbers aloud.** Nobody retains "3.3" from a slide. They retain "the public bucket one." The numbers are on the slide so they can find them later; your job is to make the *concept* land.

**Expect pushback of the form "this is just basic hygiene."** That is the correct reaction and you should agree with it enthusiastically. The value of IG1 is not that it is clever — it is that it is a fixed, externally defined list that stops security work being re-litigated in every sprint planning.

**Have the real numbers ready.** Someone will ask how big this is. 56 IG1 safeguards, 44 GCP-actionable, 290 requirement-level checks in our checklist, 188 with a CLI check.
