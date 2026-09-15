# IG1 Reference Card

*Handout, not slides. Send after the class.*

---

## SRE-speak → IG1

| You say | Safeguard |
|---|---|
| Public bucket / legacy ACLs | **3.3** Data access control lists |
| No lifecycle rule on the bucket | **3.4** Enforce data retention |
| Where does the PII live | **3.2** Data inventory |
| Default VPC still exists | **4.2** Secure network configuration |
| 22 open to `0.0.0.0/0` | **4.4** Firewall on servers |
| Cloud SQL has a public IP | **4.4** *(same)* |
| OS Login off / project-wide SSH keys | **4.6** Securely manage assets |
| Shielded VM off | **4.6** *(same)* |
| Default compute SA holds Editor | **4.7** Manage default accounts |
| Who has access to what | **5.1** Account inventory |
| Exported service account keys | **5.2** Static credentials |
| Dormant service accounts | **5.3** Disable dormant accounts |
| Owner/Editor on humans | **5.4** Restrict admin privileges |
| Group-based access, joiner/leaver | **6.1 / 6.2** Granting and revoking |
| IAP instead of a public bastion | **6.4** MFA for remote access |
| 2SV enforced on admins | **6.5** MFA for admin access |
| VM Manager patch schedule / image rebake | **7.3** OS patch management |
| Artifact Registry scanning | **7.4** Application patch management |
| Data Access logs off | **8.2** Collect audit logs |
| Org-level sink with `includeChildren` | **8.2** *(same)* |
| Log retention and Bucket Lock | **8.3** Audit log storage |
| GKE release channel | **2.2** Supported software |
| Binary Authorization | **2.3** Address unauthorized software |
| Cloud Asset Inventory at org scope | **1.1** Asset inventory |
| Cloud SQL PITR, snapshot schedules | **11.2** Automated backups |
| CMEK and Bucket Lock on backups | **11.3** Protect recovery data |
| Backups in a separate project | **11.4** Isolated recovery data |
| TLS 1.2 minimum SSL policy | **12.1** Network infrastructure up to date |
| Essential Contacts unset | **17.2** Incident contact information |

---

## Numbers

| | |
|---|---|
| IG1 safeguards | 56 |
| GCP-actionable | 44 |
| No GCP surface | 12 |
| Checklist requirements | 290 |
| With a CLI check | 188 |
| Auto-scored | 109 |
| Manual | 102 |

---

## Three things that catch people out

**Org policy is not retroactive.** The constraint stops new violations. Existing ones stay until someone removes them.

**One resource fails, the safeguard fails.** No partial credit.

**`[x]` means verified in the estate.** Not merged, not applied — verified.

---

## Commands

```bash
# Organization pass
go run audit-run.go -scope=org -org=$ORG_ID -pack ./audit-state/org

# Per project
go run audit-run.go -scope=project -org=$ORG_ID \
  -project=$PROJECT -pack ./audit-state/projects/$PROJECT

# Score the checklist
go run compliance-report.go
```
