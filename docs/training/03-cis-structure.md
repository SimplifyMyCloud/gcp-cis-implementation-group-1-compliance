# 3. How CIS is structured

---

## Three levels of noun

**Control** → a theme. 18 of them. "Data Protection", "Account Management".

**Safeguard** → a specific requirement inside a control. 153 total. Written `3.3`, `5.2`, `8.2`.

**Implementation Group** → which safeguards apply to you.

| IG | Safeguards | Who |
|---|---|---|
| **IG1** | 56 | Everyone. "Essential cyber hygiene." |
| IG2 | +74 | Orgs with a security function |
| IG3 | +23 | Orgs facing targeted attackers |

IG1 is cumulative from the bottom. IG2 includes IG1. We are doing IG1 only.

---

## What IG1 is not

**Not GCP-specific.** IG1 assumes an enterprise: laptops, email, staff, vendors. Twelve of the 56 safeguards have no cloud surface at all.

**Not a maturity score.** A safeguard is met or it is not. There is no partial credit — a safeguard with nine of ten requirements done is *not met*.

**Not optional per-safeguard.** IG1 is a floor, not a menu.

---

## The numbers you will be asked about

| | |
|---|---|
| IG1 safeguards total | **56** |
| GCP-actionable | **44** |
| No GCP surface (training, endpoints, removable media) | **12** |
| Requirement-level checks in our checklist | **290** |
| With a CLI validation | **188** |
| Process or console-only, done by a human | **102** |

> **Speaker note:** The 44/12 split is the one to emphasise. A green checklist is *the GCP half* of IG1, and saying so protects us from a claim we cannot support.
