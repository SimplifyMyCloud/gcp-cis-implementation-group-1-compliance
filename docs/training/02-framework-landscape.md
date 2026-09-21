# 2. The framework landscape

---

## NIST and CIS are not competitors

They answer different questions.

| | Answers | Looks like |
|---|---|---|
| **NIST CSF 2.0** | *What outcomes* should we achieve? | Six functions: Govern, Identify, Protect, Detect, Respond, Recover |
| **NIST SP 800-53** | *What controls* exist, exhaustively? | 1,196 controls across 20 families |
| **CIS Controls v8.1** | *Which* of those, in *what order*? | 18 controls, 153 safeguards |
| **CIS IG1** | What is the *minimum* to start? | 56 safeguards |

NIST describes the destination. CIS gives you a prioritised route. IG1 is the first leg.

> **Speaker note:** If someone asks "why not just do 800-53" — 1,196 controls with no prioritisation is not an engineering plan, it is a library.

---

## The NIST 800 series, briefly

You will hear these names. What they actually are:

- **SP 800-53 Rev 5** — the master catalog. 1,196 controls, 20 families. Written for federal systems; everything else derives from it.
- **SP 800-171 Rev 3** (May 2024) — 97 requirements across 17 families, tailored from 800-53 for protecting Controlled Unclassified Information on *non*-federal systems. This is the one defence contractors mean.
- **CSF 2.0** (Feb 2024) — outcome-based, not a control list. Added **Govern** to the original five functions.

None of these are our mandate. They are the ecosystem CIS maps into, and the reason CIS carries weight.

> **Speaker note:** DoD still requires 800-171 **Rev 2** under DFARS, not Rev 3. If a contractor asks, that distinction matters to them.

---

## Where our work sits

```
NIST CSF 2.0          what outcome we want
      ↓
CIS Controls v8.1     which safeguards achieve it
      ↓
CIS IG1               which 56 to do first
      ↓
GCP Org Policy        how it is enforced in our estate
      ↓
Terraform             how it is applied and kept applied
```

We operate at the bottom two layers. This class is about reading upward.
