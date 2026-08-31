# 5. Four things that surprise engineers

---

## 1. Org policy is not retroactive

This is the single most misunderstood thing about GCP org policies, and the most common way an audit produces a false pass.

**A constraint blocks future non-conforming operations. It does nothing to what already exists.**

Enforce `storage.publicAccessPrevention` and the console shows the policy as enforced — while every bucket that was already public stays public.

Enforce `iam.disableServiceAccountKeyCreation` and every key already sitting in a CI system keeps working forever.

> **Every constraint-based fix is half a fix.** The other half is finding and remediating what is already there. Eleven safeguards are affected.

> **Speaker note:** If they remember one technical thing from this class, make it this one.

---

## 2. A third of it is not infrastructure — but it is still ours

Of 290 requirement-level checks, **102 have no CLI validation**:

- **30** are GCP tasks with no API — Admin Console 2SV enforcement, session length, image build properties
- **72** are process and documentation — the written data management process, the named incident handler, the review cadence

**Every one of those 102 is about the GCP estate, so every one is ours.** "Written audit log management process" means *for our GCP logs*. "Documented network baseline" means *our VPCs*. Not automatable is not the same as not our problem.

No `terraform apply` produces a named incident handler. Someone on this team still has to write it down and date it.

**The most common failure in that pile is a missing date, not a missing document.** A process nobody has reviewed cannot be shown to be current.

---

## 3. Twelve safeguards are genuinely outside our scope

**We are responsible for Google Cloud. Nothing else.** Anything outside GCP we reference only where it touches GCP.

Twelve IG1 safeguards have no GCP surface at all:

- Security awareness training — 8 safeguards, Control 14, the largest single block in IG1
- End-user device encryption and endpoint firewalls
- Supported browsers and email clients
- Removable media autorun

We do not assess these, do not report on them, and do not own them.

**Where the boundary gets interesting:** 2SV enforcement lives in the Cloud Identity Admin Console, not in GCP — but it gates GCP access, so we assess it. That is the test. *Does it engage with GCP?* If yes, it is ours even when the setting lives elsewhere.

**If nobody outside this team owns those twelve, a 100% IG1 mandate is unachievable regardless of how our GCP work goes.** Flag it early — it is an escalation, not an engineering problem.

---

## 4. "Not compliant" hides two very different situations

Because remediation here needs management approval, an unmet requirement means one of two things:

| State | Owner |
|---|---|
| No fix written | **Us** |
| Fix written, PR open, awaiting approval | **Management** |

Our checklist tracks these separately, and the report ages the pending ones.

A fix sitting unapproved for 60 days is a management decision, not an SRE backlog item — and the report says so in those words.

> **Speaker note:** This usually gets a reaction. It is the part of the tooling built specifically for how our approval chain works.
