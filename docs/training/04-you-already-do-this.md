# 4. You already do this

---

## The exercise

For each of the following: you know the engineering. Learn the number.

Read left to right. Left is what you would say in a design review. Right is what goes in the report.

> **Speaker note:** Consider covering the right column and asking the room to guess whether each one is even *in* IG1. They will get most of them. That is the point — make them discover it rather than telling them.

---

## Identity — the half you argue about most

| What you already say | Safeguard |
|---|---|
| "Nobody should have Owner on the org" | **5.4** Restrict administrator privileges to dedicated admin accounts |
| "Stop exporting service account keys, use Workload Identity" | **5.2** Use unique passwords *(static credentials)* |
| "That service account hasn't authenticated in a year" | **5.3** Disable dormant accounts |
| "Admins need hardware keys, not SMS" | **6.5** Require MFA for administrative access |
| "Use IAP, don't put a bastion on a public IP" | **6.4** Require MFA for remote network access |
| "Grant to groups, not to people" | **6.1 / 6.2** Access granting and revoking processes |

**5.2 is the one that surprises people.** The safeguard says "use unique passwords" and reads like a helpdesk policy. In GCP it means: eliminate exported service account keys, because a key file *is* a shared secret.

---

## Data — the highest-severity findings live here

| What you already say | Safeguard |
|---|---|
| "That bucket is world-readable" | **3.3** Configure data access control lists |
| "Turn on uniform bucket-level access, kill the legacy ACLs" | **3.3** *(same safeguard)* |
| "Set a lifecycle rule, we're paying to store 2019 logs" | **3.4** Enforce data retention |
| "Where does the PII actually live?" | **3.2** Establish and maintain a data inventory |

---

## Compute and network — the estate you touch daily

| What you already say | Safeguard |
|---|---|
| "Delete the default VPC" | **4.2** Secure configuration for network infrastructure |
| "Why is 22 open to `0.0.0.0/0`?" | **4.4** Implement and manage a firewall on servers |
| "Use OS Login, not project-wide SSH keys" | **4.6** Securely manage enterprise assets |
| "Shielded VM should be on" | **4.6** *(same safeguard)* |
| "The default compute SA still has Editor" | **4.7** Manage default accounts |
| "Set a minimum TLS 1.2 SSL policy on the LB" | **12.1** Ensure network infrastructure is up to date |

---

## Operations — the boring ones that save you at 3am

| What you already say | Safeguard |
|---|---|
| "Data Access logs are off, we can't investigate anything" | **8.2** Collect audit logs |
| "We need an org-level sink, not per-project" | **8.2** *(same safeguard)* |
| "Log bucket needs a retention policy and a lock" | **8.3** Ensure adequate audit log storage |
| "Put VM Manager on a patch schedule" | **7.3** Automated OS patch management |
| "Rebake the image, don't patch in place" | **7.3** *(same safeguard)* |
| "Scan the images in Artifact Registry" | **7.4** Automated application patch management |
| "Pin GKE to a release channel" | **2.2** Ensure authorized software is currently supported |
| "Turn on Cloud Asset Inventory at org scope" | **1.1** Enterprise asset inventory |

---

## Resilience — where the framework is sharper than we usually are

| What you already say | Safeguard |
|---|---|
| "Cloud SQL needs PITR and a real retention window" | **11.2** Perform automated backups |
| "Attach the snapshot schedule to the disk" | **11.2** *(a policy attached to nothing backs up nothing)* |
| "Backups go in a different project" | **11.4** Isolated instance of recovery data |
| "Bucket Lock the backup bucket" | **11.3** Protect recovery data |

**11.4 is stricter than most of us are.** A second copy reachable with the same credentials that encrypted production is not a backup. The safeguard requires *isolation*, and it is the one most orgs believe they pass and do not.

---

## The one nobody guesses

| What you already say | Safeguard |
|---|---|
| *(usually nothing)* | **17.2** Establish and maintain contact information for reporting security incidents |

Essential Contacts. If it is unset, Google's security notifications about *your* organization go to whoever created it — often someone who left years ago.

Two minutes of work. Closes most of a safeguard. Nobody ever does it.

> **Speaker note:** Good place to land the section. It is the cheapest fix in the whole framework and it is almost always missing.

---

## So what is IG1 actually for?

Everything above, you already believed.

What IG1 adds is that **it is external, fixed, and numbered.**

- You cannot be argued out of a numbered safeguard in sprint planning
- "Best practice" is an opinion; "Safeguard 3.3" is a requirement
- It converts security debt from a preference into a gap with a name

That is the whole value. Not the content — the *leverage*.
