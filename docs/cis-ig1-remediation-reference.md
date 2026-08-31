# CIS Controls v8.1 IG1 — GCP Remediation Reference

**Companion to `cis-ig1-gcp-checklist.md`.** When a checklist item comes up **Not Compliant**, look up the safeguard ID here.

Each entry gives:

- **Check** — how to confirm the finding
- **Fix** — example Terraform you can lift into your own repo

**Validation commands live in [`cis-ig1-cli-validation.md`](cis-ig1-cli-validation.md).** That document is the canonical home for every read-only `gcloud` command, indexed by V-number and cross-linked from each checklist requirement. The **Check** entries below reference those V-numbers rather than repeating the commands, so there is one place to maintain when a command changes.

The Terraform is illustrative. It is not applied from this repository and it is not a drop-in module — read it, adapt the names and scopes to your estate, and apply it through your own review process.

---

# ⚠️ READ THIS FIRST — Organization Policy is not retroactive

**This is the single most misunderstood thing about GCP org policies, and the most common way an audit produces a false pass.**

Applying a constraint blocks *future* non-conforming operations. It does **nothing** to what already exists.

Enforce `storage.publicAccessPrevention` and the console will proudly show the policy as enforced — while every bucket that was already public stays public. Enforce `iam.disableServiceAccountKeyCreation` and every key already exported to a CI system keeps working forever. Enforce `iam.automaticIamGrantsForDefaultServiceAccounts` and every default service account that already holds Editor keeps holding it.

**Every org policy fix in this document is half a fix.** The other half is finding and remediating what is already there.

Affected safeguards, each with a discovery command in its section:

| Safeguard | Constraint blocks new… | …but these already exist |
|---|---|---|
| [2.3](#23--address-unauthorized-software) | Unattested image admission | Pods already running unattested |
| [3.3](#33--data-access-control-lists) | Public buckets, legacy ACLs, external grants | Buckets already public, ACLs already legacy |
| [4.2](#42--secure-network-configuration) | Default network creation | Default networks in every existing project |
| [4.4](#44--firewall-on-servers) | Public Cloud SQL, open authorized networks | Instances already exposed |
| [4.6](#46--securely-manage-assets) | Non-OS-Login, external IP, non-Shielded VMs | Every VM built before the constraint |
| [4.7](#47--manage-default-accounts) | Automatic Editor grant at project creation | Default SAs already holding Editor |
| [5.2](#52--eliminate-static-credentials) | New service account keys | **Every key already exported** |
| [5.4](#54--restrict-admin-privileges) | — | Basic-role grants already made |
| [8.2](#82--collect-audit-logs) | — | History before you enabled logging is gone forever |
| [15.1](#151--service-provider-inventory) | New external IAM grants | External grants already in place |
| [17.2](#172--incident-contact-information) | Contacts outside your domain | Contacts already configured |

## Find everything at once

Discovery commands use Cloud Asset Inventory, which searches the whole organization in one call rather than looping projects. Enable it first:

```
gcloud services enable cloudasset.googleapis.com --project=PROJECT_ID
```

You need `roles/cloudasset.viewer` at organization scope, plus `roles/iam.securityReviewer` for the IAM queries. Several examples pipe through `jq`. Full prerequisites are in [`cis-ig1-cli-validation.md`](cis-ig1-cli-validation.md#before-you-start).

**Run the discovery sweep before applying any constraint.** It tells you how big the clean-up is, and the violation list is your remediation backlog. Applying constraints first only hides the problem behind a green console.

**Then apply in dry-run.** On a permissive-default estate, enforcing blind will break running workloads. Set `dry_run_spec` instead of `spec`, read violations in Policy Analyzer, remediate, then promote to enforcement.

---

## Contents

| | | |
|---|---|---|
| [1.1](#11--enterprise-asset-inventory) [1.2](#12--address-unauthorized-assets) | [2.1](#21--software-inventory) [2.2](#22--supported-software) [2.3](#23--address-unauthorized-software) | [3.2](#32--data-inventory) [3.3](#33--data-access-control-lists) [3.4](#34--enforce-data-retention) |
| [4.2](#42--secure-network-configuration) [4.3](#43--session-locking) [4.4](#44--firewall-on-servers) | [4.6](#46--securely-manage-assets) [4.7](#47--manage-default-accounts) | [5.1](#51--account-inventory) [5.2](#52--eliminate-static-credentials) [5.3](#53--dormant-accounts) [5.4](#54--restrict-admin-privileges) |
| [6.3](#63--mfa-for-externally-exposed-apps) [6.4](#64--mfa-for-remote-access) [6.5](#65--mfa-for-admin-access) | [7.3](#73--os-patch-management) [7.4](#74--application-patch-management) | [8.2](#82--collect-audit-logs) [8.3](#83--audit-log-storage) |
| [9.2](#92--dns-filtering) | [10.1](#101--anti-malware) [10.2](#102--signature-updates) | [11.2](#112--automated-backups) [11.3](#113--protect-recovery-data) [11.4](#114--isolated-recovery-data) |
| [12.1](#121--network-infrastructure-currency) | [15.1](#151--service-provider-inventory) | [17.2](#172--incident-contact-information) [17.3](#173--incident-reporting) |

Safeguards **3.1, 3.5, 4.1, 5.3 (partly), 6.1, 6.2, 7.1, 7.2, 8.1, 11.1, 17.1** are process and documentation. They have no Terraform fix — see [Process safeguards](#process-safeguards) at the end.

---

## The org policy pattern

Most of Controls 3, 4, 5, and 17 come down to Organization Policy constraints. Rather than repeating the resource block for every safeguard, here is the pattern once. Later entries just name the constraint.

```hcl
# Boolean constraint, enforced
resource "google_org_policy_policy" "require_os_login" {
  name   = "organizations/${var.org_id}/policies/compute.requireOsLogin"
  parent = "organizations/${var.org_id}"

  spec {
    rules {
      enforce = "TRUE"
    }
  }
}

# Same constraint in dry-run: reports violations, blocks nothing.
# Start here on a permissive-default organization.
resource "google_org_policy_policy" "require_os_login_dryrun" {
  name   = "organizations/${var.org_id}/policies/compute.requireOsLogin"
  parent = "organizations/${var.org_id}"

  dry_run_spec {
    rules {
      enforce = "TRUE"
    }
  }
}

# List constraint, deny everything
resource "google_org_policy_policy" "no_external_ips" {
  name   = "organizations/${var.org_id}/policies/compute.vmExternalIpAccess"
  parent = "organizations/${var.org_id}"

  spec {
    rules {
      deny_all = "TRUE"
    }
  }
}

# List constraint, explicit allowlist
resource "google_org_policy_policy" "allowed_domains" {
  name   = "organizations/${var.org_id}/policies/iam.allowedPolicyMemberDomains"
  parent = "organizations/${var.org_id}"

  spec {
    rules {
      values {
        allowed_values = ["is:C01abc234"] # Cloud Identity customer ID, NOT a domain name
      }
    }
  }
}
```

To see everything currently set:

```
gcloud org-policies list --organization=ORGANIZATION_ID
gcloud org-policies describe CONSTRAINT --organization=ORGANIZATION_ID --effective
```

---

## 1.1 — Enterprise asset inventory

**Check**

```
gcloud asset feeds list --organization=ORGANIZATION_ID
gcloud projects list --format="table(projectId,lifecycleState,createTime)"
```

**Fix**

```hcl
resource "google_bigquery_dataset" "asset_inventory" {
  dataset_id = "asset_inventory"
  project    = var.security_project_id
  location   = "US"
}

# Scheduled org-wide export. Run on a cron; the export is a point-in-time snapshot.
resource "google_cloud_asset_organization_feed" "org_assets" {
  billing_project = var.security_project_id
  org_id          = var.org_id
  feed_id         = "org-asset-changes"
  content_type    = "RESOURCE"

  asset_types = [
    "compute.googleapis.com/Instance",
    "compute.googleapis.com/Disk",
    "compute.googleapis.com/Address",
    "storage.googleapis.com/Bucket",
    "sqladmin.googleapis.com/Instance",
    "container.googleapis.com/Cluster",
    "run.googleapis.com/Service",
    "cloudfunctions.googleapis.com/CloudFunction",
  ]

  feed_output_config {
    pubsub_destination {
      topic = google_pubsub_topic.asset_changes.id
    }
  }
}

resource "google_pubsub_topic" "asset_changes" {
  name    = "asset-inventory-changes"
  project = var.security_project_id
}
```

Enforce ownership metadata so the inventory is attributable:

```hcl
resource "google_tags_tag_key" "owner" {
  parent      = "organizations/${var.org_id}"
  short_name  = "owner"
  description = "Owning team for this resource"
}
```

**Also required:** a scheduled full export (`gcloud asset export`) for point-in-time inventory, and a documented review cadence. The feed captures *changes*, not a baseline.

---

## 1.2 — Address unauthorized assets

**Check**

```
gcloud projects list --filter="lifecycleState:DELETE_REQUESTED"
gcloud compute disks list --filter="-users:*" --format="table(name,zone,sizeGb)"
gcloud compute addresses list --filter="status:RESERVED" --format="table(name,region)"
```

**Fix**

There is no Terraform that removes unauthorized assets — by definition they are not in your state. The remediation is process, backed by detection:

```hcl
resource "google_logging_metric" "untagged_resource_creation" {
  name    = "untagged-resource-creation"
  project = var.security_project_id
  filter  = <<-EOT
    protoPayload.methodName=~"compute.instances.insert|storage.buckets.create"
    AND NOT protoPayload.request.labels.owner:*
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
  }
}
```

Pair with a documented disposition process: unattributable resources get an owner within N days or are quarantined.

---

## 2.1 — Software inventory

**Check**

```
gcloud compute instances os-inventory list-instances
gcloud container clusters list --format="table(name,currentMasterVersion,currentNodeVersion)"
```

**Fix**

```hcl
# VM Manager. Without this, os-inventory returns nothing.
resource "google_compute_project_metadata_item" "os_config" {
  project = var.project_id
  key     = "enable-osconfig"
  value   = "TRUE"
}

# The OS Config agent must also be present in the image. Bake it in rather than
# installing at boot — an agent installed by startup script is an agent that is
# missing on every instance where the script failed silently.
```

**Also required:** the agent in your golden image, and instances recreated from that image. Enabling the metadata flag does not retrofit existing VMs.

---

## 2.2 — Supported software

**Check**

```
gcloud compute images list --filter="deprecated.state:DEPRECATED" --show-deprecated
gcloud container clusters list --format="value(name,currentMasterVersion)"
gcloud sql instances list --format="table(name,databaseVersion)"
```

**Fix**

```hcl
# Pin GKE to a release channel so version currency is automatic rather than
# a recurring manual decision.
resource "google_container_cluster" "primary" {
  name     = "primary"
  location = var.region

  release_channel {
    channel = "REGULAR"
  }

  # ... other hardening, see 4.6
}

resource "google_sql_database_instance" "main" {
  name             = "main"
  database_version = "POSTGRES_16"

  settings {
    maintenance_window {
      day  = 7
      hour = 3
    }
  }
}
```

---

## 2.3 — Address unauthorized software

**Check**

```
gcloud container binauthz policy export
gcloud artifacts repositories list
```

**Fix**

```hcl
resource "google_binary_authorization_policy" "policy" {
  project = var.project_id

  default_admission_rule {
    evaluation_mode  = "REQUIRE_ATTESTATION"
    enforcement_mode = "ENFORCED_BLOCK_AND_AUDIT_LOG"
    require_attestations_by = [google_binary_authorization_attestor.build.name]
  }

  # Google-managed system images still need to run.
  admission_whitelist_patterns {
    name_pattern = "gcr.io/google-containers/*"
  }
}

resource "google_binary_authorization_attestor" "build" {
  name    = "build-attestor"
  project = var.project_id

  attestation_authority_note {
    note_reference = google_container_analysis_note.build.name
    public_keys {
      ascii_armored_pgp_public_key = var.attestor_public_key
    }
  }
}

resource "google_container_analysis_note" "build" {
  name    = "build-attestor-note"
  project = var.project_id
  attestation_authority {
    hint { human_readable_name = "Build pipeline attestor" }
  }
}
```

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

Binary Authorization evaluates images at **admission**. Pods already running when the policy is applied are not re-evaluated and keep running.

**Find running workloads that would fail the policy**

```
gcloud container clusters list --format="value(name,location)" | while read c loc; do
  gcloud container clusters get-credentials "$c" --region="$loc" --quiet 2>/dev/null
  kubectl get pods --all-namespaces \
    -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.spec.containers[*].image}{"\n"}{end}' \
    | grep -v "gcr.io/google-containers" | sed "s|^|$c / |"
done
```

Roll affected workloads after applying the policy, or they will run unattested until their next deployment.

---

## 3.2 — Data inventory

**Check**

```
gcloud scc settings services describe --organization=ORGANIZATION_ID \
  --service=SENSITIVE_DATA_PROTECTION
```

**Fix**

```hcl
resource "google_data_loss_prevention_discovery_config" "org_discovery" {
  parent   = "projects/${var.security_project_id}/locations/global"
  location = "global"
  status   = "RUNNING"

  org_config {
    location { organization_id = var.org_id }
    project_id = var.security_project_id
  }

  targets {
    big_query_target {
      filter { other_tables {} }
    }
  }

  targets {
    cloud_storage_target {
      filter { others {} }
    }
  }
}
```

---

## 3.3 — Data access control lists

The highest-value finding in the whole checklist. Public buckets are the single most common serious cloud misconfiguration.

**Check**

```
# Any bucket readable by the internet
for b in $(gcloud storage buckets list --format="value(name)"); do
  gcloud storage buckets get-iam-policy gs://$b --format=json \
    | grep -q 'allUsers\|allAuthenticatedUsers' && echo "PUBLIC: $b"
done

gcloud org-policies describe storage.publicAccessPrevention \
  --organization=ORGANIZATION_ID --effective
```

**Fix**

```hcl
locals {
  data_constraints = [
    "storage.publicAccessPrevention",
    "storage.uniformBucketLevelAccess",
  ]
}

resource "google_org_policy_policy" "data_protection" {
  for_each = toset(local.data_constraints)

  name   = "organizations/${var.org_id}/policies/${each.value}"
  parent = "organizations/${var.org_id}"

  spec {
    rules { enforce = "TRUE" }
  }
}

# Restrict IAM grants to your own tenant.
# WARNING: the value is a Cloud Identity CUSTOMER ID (C0xxxxxxx), not a domain.
# A wrong value removes your own ability to grant IAM. Use dry_run_spec first.
resource "google_org_policy_policy" "allowed_domains" {
  name   = "organizations/${var.org_id}/policies/iam.allowedPolicyMemberDomains"
  parent = "organizations/${var.org_id}"

  dry_run_spec {
    rules {
      values { allowed_values = var.allowed_customer_ids }
    }
  }
}

# Per-bucket hardening for new buckets
resource "google_storage_bucket" "data" {
  name                        = var.bucket_name
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning { enabled = true }
}
```

**Also required:** remediate existing public buckets. The constraint blocks new exposure; it does not revoke what is already public.

```
gcloud storage buckets remove-iam-policy-binding gs://BUCKET \
  --member=allUsers --role=roles/storage.objectViewer
```

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**Find every publicly readable bucket and dataset in the organization**

```
gcloud asset search-all-iam-policies \
  --scope=organizations/ORGANIZATION_ID \
  --query='policy:("allUsers" OR "allAuthenticatedUsers")' \
  --format="table(resource,policy.bindings.role)"
```

**Find buckets still on legacy per-object ACLs**

```
gcloud projects list --format="value(projectId)" | while read p; do
  gcloud storage buckets list --project="$p" --format="value(name)" 2>/dev/null | while read b; do
    ubla=$(gcloud storage buckets describe "gs://$b" \
      --format="value(uniform_bucket_level_access)" 2>/dev/null)
    [ "$ubla" != "True" ] && echo "LEGACY ACLs: $p / $b"
  done
done
```

**Find IAM grants to identities outside your tenant**

```
gcloud asset search-all-iam-policies \
  --scope=organizations/ORGANIZATION_ID \
  --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]?.members[]?
           | select(startswith("user:") or startswith("group:"))
           | select(test("@YOURDOMAIN\\.com$") | not)
           | "\($r)\t\(.)"' | sort -u
```

**Remediate**

```
gcloud storage buckets remove-iam-policy-binding gs://BUCKET \
  --member=allUsers --role=roles/storage.objectViewer

gcloud storage buckets update gs://BUCKET --uniform-bucket-level-access
```

---

## 3.4 — Enforce data retention

**Check**

```
gcloud storage buckets describe gs://BUCKET --format="value(lifecycle)"
bq show --format=prettyjson PROJECT:DATASET | grep -i expiration
```

**Fix**

```hcl
resource "google_storage_bucket" "retained" {
  name     = var.bucket_name
  location = var.region

  lifecycle_rule {
    condition { age = var.retention_days }
    action    { type = "Delete" }
  }

  lifecycle_rule {
    condition { age = 90 }
    action {
      type          = "SetStorageClass"
      storage_class = "NEARLINE"
    }
  }
}

resource "google_bigquery_dataset" "retained" {
  dataset_id                      = var.dataset_id
  default_table_expiration_ms     = var.retention_days * 24 * 60 * 60 * 1000
  default_partition_expiration_ms = var.retention_days * 24 * 60 * 60 * 1000
}
```

---

## 4.2 — Secure network configuration

**Check**

```
gcloud compute networks list --format="table(name,x_gcloud_subnet_mode)"
gcloud compute firewall-rules list --filter="name~default-allow"
```

**Fix**

```hcl
resource "google_org_policy_policy" "skip_default_network" {
  name   = "organizations/${var.org_id}/policies/compute.skipDefaultNetworkCreation"
  parent = "organizations/${var.org_id}"

  spec {
    rules { enforce = "TRUE" }
  }
}

resource "google_compute_network" "vpc" {
  name                    = "primary"
  auto_create_subnetworks = false # custom mode
}

resource "google_compute_subnetwork" "private" {
  name                     = "private"
  network                  = google_compute_network.vpc.id
  ip_cidr_range            = "10.0.0.0/20"
  region                   = var.region
  private_ip_google_access = true

  log_config {
    aggregation_interval = "INTERVAL_5_SEC"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}
```

**Also required:** delete existing default networks project by project. The constraint only affects new projects.

```
gcloud compute networks delete default --project=PROJECT_ID
```

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**Find every default network still present**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=compute.googleapis.com/Network \
  --query='name:default' \
  --format="table(project,displayName)"
```

**Find the permissive default firewall rules that come with them**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=compute.googleapis.com/Firewall \
  --query='name:(default-allow-ssh OR default-allow-rdp OR default-allow-icmp OR default-allow-internal)' \
  --format="table(project,displayName)"
```

**Find legacy and auto-mode VPCs**

```
gcloud projects list --format="value(projectId)" | while read p; do
  gcloud compute networks list --project="$p" \
    --format="value(name,x_gcloud_subnet_mode)" 2>/dev/null \
    | grep -E "LEGACY|AUTO" | sed "s|^|$p / |"
done
```

**Remediate**

```
gcloud compute networks delete default --project=PROJECT_ID
```

Delete the firewall rules first if the network delete is refused.

---

## 4.3 — Session locking

**Check**

Google Admin Console → Security → Google Cloud session control. Not exposed via `gcloud`.

**Fix**

Not manageable in Terraform. Set the Google Cloud session length in the Admin Console (Security → Google Cloud session control) to a defined reauthentication interval rather than "never". For IAP-protected resources:

```hcl
resource "google_iap_settings" "web" {
  name = "projects/${var.project_number}/iap_web"

  access_settings {
    reauth_settings {
      method       = "SECURE_KEY"
      max_age      = "3600s"
      policy_type  = "MINIMUM"
    }
  }
}
```

---

## 4.4 — Firewall on servers

**Check**

```
gcloud compute firewall-rules list \
  --filter="sourceRanges:0.0.0.0/0 AND allowed.ports:(22 3389 3306 5432 1433)" \
  --format="table(name,network,sourceRanges.list(),allowed[].map().firewall_rule().list())"

gcloud sql instances list --format="table(name,settings.ipConfiguration.ipv4Enabled)"
```

**Fix**

```hcl
resource "google_org_policy_policy" "sql_no_public_ip" {
  for_each = toset(["sql.restrictPublicIp", "sql.restrictAuthorizedNetworks"])

  name   = "organizations/${var.org_id}/policies/${each.value}"
  parent = "organizations/${var.org_id}"

  spec {
    rules { enforce = "TRUE" }
  }
}

# SSH via IAP only. 35.235.240.0/20 is Google's IAP forwarding range —
# this replaces 0.0.0.0/0 rules rather than supplementing them.
resource "google_compute_firewall" "iap_ssh" {
  name      = "allow-ssh-from-iap"
  network   = google_compute_network.vpc.name
  direction = "INGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["iap-ssh"]
}

resource "google_compute_firewall" "deny_all_ingress" {
  name      = "deny-all-ingress"
  network   = google_compute_network.vpc.name
  direction = "INGRESS"
  priority  = 65534

  deny { protocol = "all" }
  source_ranges = ["0.0.0.0/0"]

  log_config { metadata = "INCLUDE_ALL_METADATA" }
}
```

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**Find every Cloud SQL instance with a public IP**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=sqladmin.googleapis.com/Instance \
  --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.settings.ipConfiguration.ipv4Enabled == true)
           | "PUBLIC IP: \(.name)"'
```

**Find instances allowing 0.0.0.0/0 in authorized networks**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=sqladmin.googleapis.com/Instance \
  --format=json \
  | jq -r '.[] | select([.versionedResources[]?.resource.settings.ipConfiguration.authorizedNetworks[]?.value]
           | index("0.0.0.0/0")) | "OPEN TO WORLD: \(.name)"'
```

**Find firewall rules exposing admin and database ports to the internet**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=compute.googleapis.com/Firewall \
  --format=json \
  | jq -r '.[] | select([.versionedResources[]?.resource.sourceRanges[]?] | index("0.0.0.0/0"))
           | select([.versionedResources[]?.resource.allowed[]?.ports[]?]
             | any(. == "22" or . == "3389" or . == "3306" or . == "5432" or . == "1433"))
           | "OPEN: \(.name)"'
```

---

## 4.6 — Securely manage assets

**Check**

```
for c in compute.requireOsLogin compute.disableSerialPortAccess \
         compute.requireShieldedVm compute.vmExternalIpAccess compute.vmCanIpForward; do
  echo "== $c"
  gcloud org-policies describe $c --organization=ORGANIZATION_ID --effective 2>&1 | head -5
done
```

**Fix**

```hcl
locals {
  # Safe to enforce immediately on most estates
  enforce_now = [
    "compute.requireOsLogin",
    "compute.disableSerialPortAccess",
    "compute.requireShieldedVm",
  ]

  # Will break existing workloads — dry-run, remediate, then promote.
  # compute.vmExternalIpAccess in particular cuts off admin access to any VM
  # you currently reach by public IP. Deploy IAP TCP forwarding FIRST.
  dry_run_first = [
    "compute.vmExternalIpAccess",
    "compute.vmCanIpForward",
  ]
}

resource "google_org_policy_policy" "compute_enforced" {
  for_each = toset(local.enforce_now)

  name   = "organizations/${var.org_id}/policies/${each.value}"
  parent = "organizations/${var.org_id}"

  spec {
    rules { enforce = "TRUE" }
  }
}

resource "google_org_policy_policy" "compute_dryrun" {
  for_each = toset(local.dry_run_first)

  name   = "organizations/${var.org_id}/policies/${each.value}"
  parent = "organizations/${var.org_id}"

  dry_run_spec {
    rules { deny_all = "TRUE" }
  }
}
```

GKE hardening — the legacy settings are the ones that matter on an older cluster:

```hcl
resource "google_container_cluster" "hardened" {
  name     = "hardened"
  location = var.region

  enable_legacy_abac = false

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = true
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  master_authorized_networks_config {
    cidr_blocks {
      cidr_block   = var.admin_cidr
      display_name = "admin"
    }
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  master_auth {
    client_certificate_config { issue_client_certificate = false }
  }

  node_config {
    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }
    # Blocks the legacy metadata endpoints
    metadata = { disable-legacy-endpoints = "true" }
  }
}
```

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**Find VMs without OS Login enabled**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=compute.googleapis.com/Instance \
  --format=json \
  | jq -r '.[] | select([.versionedResources[]?.resource.metadata.items[]?
           | select(.key == "enable-oslogin" and (.value | ascii_upcase) == "TRUE")] | length == 0)
           | "NO OS LOGIN: \(.name)"'
```

**Find projects still carrying project-wide SSH keys**

```
gcloud projects list --format="value(projectId)" | while read p; do
  gcloud compute project-info describe --project="$p" \
    --format="value(commonInstanceMetadata.items[].key)" 2>/dev/null \
    | grep -qw "sshKeys" && echo "PROJECT-WIDE SSH KEYS: $p"
done
```

**Find VMs with external IPs**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=compute.googleapis.com/Instance \
  --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.networkInterfaces[]?.accessConfigs != null)
           | "EXTERNAL IP: \(.name)"'
```

**Find non-Shielded VMs**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=compute.googleapis.com/Instance \
  --format=json \
  | jq -r '.[] | select(.versionedResources[]?.resource.shieldedInstanceConfig.enableSecureBoot != true)
           | "NOT SHIELDED: \(.name)"'
```

Shielded VM cannot be turned on for a running instance — it requires a stop, or replacement from a UEFI-compatible image. Budget for that before enforcing.

**Find GKE clusters with legacy settings**

```
gcloud projects list --format="value(projectId)" | while read p; do
  gcloud container clusters list --project="$p" \
    --format="value(name,enableLegacyAbac,privateClusterConfig.enablePrivateNodes)" 2>/dev/null \
    | sed "s|^|$p / |"
done
```

---

## 4.7 — Manage default accounts

**Check**

```
gcloud projects get-iam-policy PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members~compute@developer.gserviceaccount.com" \
  --format="value(bindings.role)"
```

If that returns `roles/editor`, the default service account is over-privileged.

**Fix**

```hcl
resource "google_org_policy_policy" "no_default_grants" {
  name   = "organizations/${var.org_id}/policies/iam.automaticIamGrantsForDefaultServiceAccounts"
  parent = "organizations/${var.org_id}"

  spec {
    rules { enforce = "TRUE" }
  }
}

# Purpose-built identity instead of the default
resource "google_service_account" "workload" {
  account_id   = "app-workload"
  display_name = "Application workload identity"
  project      = var.project_id
}

resource "google_project_iam_member" "workload" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.workload.email}"
}
```

**Also required:** strip `roles/editor` from default service accounts in existing projects. The constraint prevents the grant at creation; it does not remove grants already made.

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**Find every project where the default service account still holds Editor**

```
gcloud projects list --format="value(projectId,projectNumber)" | while read p num; do
  role=$(gcloud projects get-iam-policy "$p" \
    --flatten="bindings[].members" \
    --filter="bindings.members:${num}-compute@developer.gserviceaccount.com AND bindings.role:roles/editor" \
    --format="value(bindings.role)" 2>/dev/null)
  [ -n "$role" ] && echo "DEFAULT SA HAS EDITOR: $p"
done
```

**Find workloads still running as a default service account**

```
gcloud asset search-all-resources \
  --scope=organizations/ORGANIZATION_ID \
  --asset-types=compute.googleapis.com/Instance \
  --format=json \
  | jq -r '.[] | select([.versionedResources[]?.resource.serviceAccounts[]?.email]
           | any(test("developer\\.gserviceaccount\\.com$")))
           | "DEFAULT SA IN USE: \(.name)"'
```

Strip the grant only after confirming nothing depends on it — removing Editor from a service account a workload is actively using will break that workload.

```
gcloud projects remove-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:PROJECT_NUMBER-compute@developer.gserviceaccount.com" \
  --role="roles/editor"
```

---

## 5.1 — Account inventory

**Check**

```
gcloud organizations get-iam-policy ORGANIZATION_ID --format=json
gcloud iam service-accounts list --project=PROJECT_ID
gcloud asset search-all-iam-policies --scope=organizations/ORGANIZATION_ID
```

**Fix**

```hcl
# Continuous IAM inventory into BigQuery for review
resource "google_cloud_asset_organization_feed" "iam_changes" {
  billing_project = var.security_project_id
  org_id          = var.org_id
  feed_id         = "iam-policy-changes"
  content_type    = "IAM_POLICY"
  asset_types     = [".*"]

  feed_output_config {
    pubsub_destination { topic = google_pubsub_topic.iam_changes.id }
  }
}

resource "google_pubsub_topic" "iam_changes" {
  name    = "iam-policy-changes"
  project = var.security_project_id
}
```

---

## 5.2 — Eliminate static credentials

**Check**

```
# Any user-managed key is a finding
gcloud iam service-accounts keys list \
  --iam-account=SA_EMAIL --managed-by=user \
  --format="table(name,validAfterTime)"
```

**Fix**

```hcl
resource "google_org_policy_policy" "no_sa_keys" {
  for_each = toset([
    "iam.disableServiceAccountKeyCreation",
    "iam.disableServiceAccountKeyUpload",
  ])

  name   = "organizations/${var.org_id}/policies/${each.value}"
  parent = "organizations/${var.org_id}"

  # Dry-run first: CI systems holding exported keys will fail loudly
  # the moment key creation is blocked and a rotation is attempted.
  dry_run_spec {
    rules { enforce = "TRUE" }
  }
}

# The replacement: federation instead of exported key files
resource "google_iam_workload_identity_pool" "ci" {
  workload_identity_pool_id = "ci-pool"
  project                   = var.project_id
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.ci.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  project                            = var.project_id

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }

  # Without an attribute condition, ANY GitHub repository can assume this identity.
  attribute_condition = "assertion.repository_owner == '${var.github_org}'"

  oidc { issuer_uri = "https://token.actions.githubusercontent.com" }
}
```

**Also required:** inventory and delete existing keys. Blocking creation leaves every already-exported key valid indefinitely — that is the actual exposure.

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**This is the important one.** Blocking key creation leaves every already-exported key valid indefinitely. The keys sitting in CI systems and on laptops are the actual exposure, and the constraint does nothing about them.

**Find every user-managed service account key in the organization**

```
gcloud projects list --format="value(projectId)" | while read p; do
  gcloud iam service-accounts list --project="$p" --format="value(email)" 2>/dev/null | while read sa; do
    gcloud iam service-accounts keys list --iam-account="$sa" --managed-by=user \
      --format="value(name,validAfterTime)" 2>/dev/null | while read key created; do
        echo "USER KEY: $p / $sa / created $created"
    done
  done
done
```

**Find keys that have never been used, or not used recently**

```
gcloud policy-intelligence query-activity \
  --activity-type=serviceAccountKeyLastAuthentication \
  --project=PROJECT_ID \
  --format="table(activity.lastAuthenticatedTime,fullResourceName)"
```

Delete unused keys first — they carry the same risk with none of the breakage.

```
gcloud iam service-accounts keys delete KEY_ID --iam-account=SA_EMAIL
```

---

## 5.3 — Dormant accounts

**Check**

```
gcloud iam service-accounts list --project=PROJECT_ID --format="value(email)" \
  | while read sa; do
      echo "$sa: $(gcloud logging read \
        "protoPayload.authenticationInfo.principalEmail=$sa" \
        --limit=1 --freshness=90d --format='value(timestamp)')"
    done
```

Empty timestamp means no authentication in 90 days.

**Fix**

Mostly process. The Terraform contribution is detection and disabling:

```hcl
resource "google_service_account" "legacy" {
  account_id = "legacy-app"
  project    = var.project_id
  disabled   = true # disable before deleting — deletion is hard to reverse
}
```

**Also required:** a recurring review cadence and integration with your leaver process.

---

## 5.4 — Restrict admin privileges

**Check**

```
gcloud organizations get-iam-policy ORGANIZATION_ID \
  --flatten="bindings[].members" \
  --filter="bindings.role:(roles/owner OR roles/editor) AND bindings.members~^user:" \
  --format="table(bindings.role,bindings.members)"
```

Any individual user with Owner or Editor at org level is a finding.

**Fix**

```hcl
# Grant to groups, never to individuals — group membership is the
# joiner/mover/leaver integration point.
resource "google_organization_iam_member" "security_admins" {
  org_id = var.org_id
  role   = "roles/iam.securityAdmin"
  member = "group:gcp-security-admins@${var.domain}"
}

# Time-bound elevation instead of standing privilege
resource "google_project_iam_member" "temporary_admin" {
  project = var.project_id
  role    = "roles/compute.admin"
  member  = "group:gcp-oncall@${var.domain}"

  condition {
    title      = "expires_end_of_quarter"
    expression = "request.time < timestamp('2026-12-31T00:00:00Z')"
  }
}

# Least-privilege custom role
resource "google_organization_iam_custom_role" "auditor" {
  org_id      = var.org_id
  role_id     = "cisAuditor"
  title       = "CIS Auditor"
  permissions = [
    "resourcemanager.projects.get",
    "orgpolicy.policy.get",
    "storage.buckets.get",
    "logging.logEntries.list",
  ]
}
```

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**Find every basic-role grant to an individual user across the organization**

```
gcloud asset search-all-iam-policies \
  --scope=organizations/ORGANIZATION_ID \
  --query='policy:(roles/owner OR roles/editor)' \
  --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]?
           | select(.role == "roles/owner" or .role == "roles/editor")
           | .role as $role | .members[]?
           | select(startswith("user:"))
           | "\($r)\t\($role)\t\(.)"' | sort -u
```

**Find over-granted roles that have never been exercised**

```
gcloud recommender recommendations list \
  --project=PROJECT_ID \
  --location=global \
  --recommender=google.iam.policy.Recommender \
  --format="table(content.overview.member,content.overview.removedRole)"
```

---

## 6.3 — MFA for externally exposed apps

**Check**

```
gcloud iap web get-iam-policy --resource-type=backend-services --service=SERVICE
```

**Fix**

```hcl
resource "google_iap_brand" "brand" {
  support_email     = var.support_email
  application_title = "Internal Apps"
  project           = var.project_id
}

resource "google_iap_client" "client" {
  display_name = "internal-app"
  brand        = google_iap_brand.brand.name
}

resource "google_iap_web_backend_service_iam_member" "access" {
  project             = var.project_id
  web_backend_service = google_compute_backend_service.app.name
  role                = "roles/iap.httpsResourceAccessor"
  member              = "group:app-users@${var.domain}"
}
```

**Also required:** MFA enforcement happens at the identity provider, not in IAP. IAP checks *who*; 2SV enforcement in Cloud Identity determines *how strongly they proved it*. See 6.5.

---

## 6.4 — MFA for remote access

**Check**

```
gcloud compute instances list --format="table(name,networkInterfaces[0].accessConfigs[0].natIP)"
gcloud compute firewall-rules list --filter="allowed.ports:22 AND sourceRanges:0.0.0.0/0"
```

**Fix**

```hcl
# IAP TCP forwarding replaces public-IP bastions entirely
resource "google_project_iam_member" "iap_tunnel" {
  project = var.project_id
  role    = "roles/iap.tunnelResourceAccessor"
  member  = "group:gcp-engineers@${var.domain}"
}

resource "google_compute_firewall" "iap_tunnel" {
  name    = "allow-iap-tunnel"
  network = google_compute_network.vpc.name

  allow {
    protocol = "tcp"
    ports    = ["22", "3389"]
  }

  source_ranges = ["35.235.240.0/20"]
}

resource "google_compute_instance" "private" {
  name         = "app-01"
  machine_type = "e2-medium"
  zone         = var.zone

  network_interface {
    subnetwork = google_compute_subnetwork.private.id
    # No access_config block = no external IP
  }

  metadata = { enable-oslogin = "TRUE" }

  boot_disk {
    initialize_params { image = var.hardened_image }
  }
}
```

Connect with `gcloud compute ssh app-01 --tunnel-through-iap`.

---

## 6.5 — MFA for admin access

**Check**

Google Admin Console → Security → Authentication → 2-Step Verification. Confirm **Enforcement** is on, not merely "allow users to turn on".

**Fix**

Not manageable in Terraform — 2SV enforcement is a Cloud Identity / Workspace setting. In the Admin Console:

1. Security → Authentication → 2-Step Verification → **Enforcement: On**
2. Set method to **Only security key** for the org unit containing admins
3. Confirm no exception org units exclude privileged users

**Verify by report, not by policy.** The Admin Console 2SV enrolment report is the evidence — a policy that exists with a grace period still open is not enforcement.

---

## 7.3 — OS patch management

**Check**

```
gcloud compute os-config patch-deployments list
gcloud compute os-config patch-jobs list --limit=5
```

**Fix**

```hcl
resource "google_os_config_patch_deployment" "monthly" {
  patch_deployment_id = "monthly-patch"
  project             = var.project_id

  instance_filter {
    group_labels {
      labels = { patch-group = "standard" }
    }
  }

  patch_config {
    reboot_config = "DEFAULT"
    apt    { type = "DIST" }
    yum    { security = true, minimal = true }
    windows_update { classifications = ["CRITICAL", "SECURITY"] }
  }

  recurring_schedule {
    time_zone { id = "UTC" }
    time_of_day { hours = 3 }
    monthly { week_day_of_month { week_ordinal = 2, day_of_week = "TUESDAY" } }
  }

  rollout {
    mode = "ZONE_BY_ZONE"
    disruption_budget { percent = 25 }
  }
}
```

**Preferred alternative:** rebuild rather than patch in place. Bake a fresh image on a schedule, update the instance template, and roll the MIG. Patching long-lived instances leaves drift between what the image says and what is running; replacing them does not.

```hcl
resource "google_compute_instance_group_manager" "app" {
  name               = "app-mig"
  base_instance_name = "app"
  zone               = var.zone
  target_size        = 3

  version {
    instance_template = google_compute_instance_template.app.id
  }

  update_policy {
    type                  = "PROACTIVE"
    minimal_action        = "REPLACE"
    max_surge_fixed       = 1
    max_unavailable_fixed = 0
  }
}
```

---

## 7.4 — Application patch management

**Check**

```
gcloud artifacts docker images list REPO --include-tags --format=json \
  | jq '.[].package'
gcloud artifacts docker images describe IMAGE --show-package-vulnerability
```

**Fix**

```hcl
resource "google_artifact_registry_repository" "containers" {
  repository_id = "containers"
  location      = var.region
  format        = "DOCKER"
  project       = var.project_id
}

# Container Scanning API — findings appear in Artifact Analysis
resource "google_project_service" "container_scanning" {
  project = var.project_id
  service = "containerscanning.googleapis.com"
}

# Route findings somewhere a human reads
resource "google_scc_notification_config" "vulns" {
  config_id    = "vulnerability-findings"
  organization = var.org_id
  pubsub_topic = google_pubsub_topic.scc_findings.id

  streaming_config {
    filter = "category=\"OS_VULNERABILITY\" AND state=\"ACTIVE\""
  }
}
```

---

## 8.2 — Collect audit logs

The most common material gap. Data Access logs are **off by default** for almost every service.

**Check**

```
gcloud organizations get-iam-policy ORGANIZATION_ID --format=json | jq '.auditConfigs'
gcloud logging sinks list --organization=ORGANIZATION_ID
```

Empty `auditConfigs` means you are collecting admin activity only.

**Fix**

```hcl
resource "google_organization_iam_audit_config" "all_services" {
  org_id  = var.org_id
  service = "allServices"

  audit_log_config { log_type = "ADMIN_READ" }
  audit_log_config { log_type = "DATA_READ" }
  audit_log_config { log_type = "DATA_WRITE" }
}

# Org-level aggregated sink. include_children is the difference between
# capturing the whole org and capturing one project.
resource "google_logging_organization_sink" "audit" {
  name             = "org-audit-sink"
  org_id           = var.org_id
  include_children = true
  destination      = "storage.googleapis.com/${google_storage_bucket.audit_logs.name}"

  filter = <<-EOT
    logName:"logs/cloudaudit.googleapis.com"
  EOT
}

# The sink writes with its own identity — without this grant it silently
# writes nothing and reports no error.
resource "google_storage_bucket_iam_member" "sink_writer" {
  bucket = google_storage_bucket.audit_logs.name
  role   = "roles/storage.objectCreator"
  member = google_logging_organization_sink.audit.writer_identity
}
```

Alerting on the changes that matter:

```hcl
resource "google_logging_metric" "org_policy_change" {
  name    = "org-policy-changes"
  project = var.security_project_id
  filter  = <<-EOT
    protoPayload.methodName=~"SetOrgPolicy|DeleteOrgPolicy"
    OR protoPayload.methodName="SetIamPolicy"
    OR protoPayload.methodName="google.iam.admin.v1.CreateServiceAccountKey"
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
  }
}
```

**Cost warning:** enabling `DATA_READ` on `allServices` can be expensive on a high-traffic estate. Scope to services holding sensitive data if volume is a problem — but record the scoping decision, because an auditor will ask.

> ### ⚠️ NOT RETROACTIVE
>
> Enabling Data Access logs starts the record **from that moment**. It does not recover anything that happened before. There is no command that finds the missing history, because it was never written.
>
> Enable this first, before the rest of the programme — every day it stays off is a day you can never investigate.

**Find projects and services where Data Access logging is still off**

```
gcloud organizations get-iam-policy ORGANIZATION_ID \
  --format=json | jq '.auditConfigs // "NO ORG-LEVEL AUDIT CONFIG"'

gcloud projects list --format="value(projectId)" | while read p; do
  cfg=$(gcloud projects get-iam-policy "$p" --format=json 2>/dev/null | jq -r '.auditConfigs // empty')
  [ -z "$cfg" ] && echo "NO DATA ACCESS LOGGING: $p"
done
```

**Find sinks that are silently writing nothing**

```
gcloud logging sinks list --organization=ORGANIZATION_ID \
  --format="table(name,destination,writerIdentity,includeChildren)"
```

A sink whose `writerIdentity` lacks write permission on its destination reports no error and delivers no logs. Verify the grant on every destination.

---

## 8.3 — Audit log storage

**Check**

```
gcloud logging buckets describe _Default --location=global \
  --organization=ORGANIZATION_ID --format="value(retentionDays)"
gcloud storage buckets describe gs://AUDIT_BUCKET --format="value(retentionPolicy)"
```

`30` is the untouched default.

**Fix**

```hcl
# Logs live in their own project with separate IAM. A compromised workload
# project must not be able to delete the evidence of the compromise.
resource "google_storage_bucket" "audit_logs" {
  name                        = "org-audit-logs-${var.org_id}"
  project                     = var.logging_project_id
  location                    = "US"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  # WORM. Once locked this CANNOT be shortened or removed — the bucket
  # can only be deleted after every object has aged out. Test the duration
  # before locking.
  retention_policy {
    is_locked        = true
    retention_period = 365 * 24 * 60 * 60
  }

  lifecycle_rule {
    condition { age = 90 }
    action {
      type          = "SetStorageClass"
      storage_class = "COLDLINE"
    }
  }
}

resource "google_logging_organization_bucket_config" "default" {
  organization   = var.org_id
  location       = "global"
  bucket_id      = "_Default"
  retention_days = 400
}
```

---

## 9.2 — DNS filtering

**Check**

```
gcloud dns response-policies list
gcloud compute routers nats list --router=ROUTER --region=REGION
```

**Fix**

```hcl
resource "google_dns_response_policy" "blocklist" {
  response_policy_name = "malicious-domain-blocklist"
  project              = var.project_id

  networks {
    network_url = google_compute_network.vpc.id
  }
}

resource "google_dns_response_policy_rule" "block" {
  for_each = toset(var.blocked_domains)

  response_policy = google_dns_response_policy.blocklist.response_policy_name
  rule_name       = replace(each.value, ".", "-")
  dns_name        = "${each.value}."
  project         = var.project_id

  local_data {
    local_datas {
      name    = "${each.value}."
      type    = "A"
      ttl     = 300
      rrdatas = ["0.0.0.0"]
    }
  }
}

# Centralised, observable egress instead of per-instance external IPs
resource "google_compute_router_nat" "nat" {
  name                               = "nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ALL"
  }
}
```

---

## 10.1 — Anti-malware

**Check**

```
gcloud compute os-config inventories describe INSTANCE --zone=ZONE \
  --format=json | jq '.items.installedPackages[] | select(.name | test("clamav|falcon|defender"))'
```

**Fix**

Agent deployment belongs in the image, not in Terraform. Bake it with Packer so coverage is a property of the image rather than of a startup script that may have failed:

```hcl
# packer/hardened-base.pkr.hcl
source "googlecompute" "base" {
  project_id          = var.project_id
  source_image_family = "ubuntu-2404-lts-amd64"
  zone                = var.zone
  image_family        = "hardened-base"
  image_name          = "hardened-base-{{timestamp}}"

  # Label the image with what it satisfies, so Cloud Asset Inventory can tell
  # you which RUNNING VMs came from an image carrying these controls.
  image_labels = {
    cis-safeguards = "4-6_7-3_10-1_10-2"
    hardened       = "true"
  }
}

build {
  sources = ["source.googlecompute.base"]

  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y google-osconfig-agent clamav clamav-daemon",
      "systemctl enable clamav-freshclam",
      "apt-get upgrade -y",
    ]
  }
}
```

Platform-level anti-tamper, which needs no agent:

```hcl
resource "google_compute_instance" "shielded" {
  name         = "app-01"
  machine_type = "e2-medium"
  zone         = var.zone

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  boot_disk {
    initialize_params { image = var.hardened_image }
  }

  network_interface { subnetwork = google_compute_subnetwork.private.id }
}
```

---

## 10.2 — Signature updates

**Check**

```
gcloud compute ssh INSTANCE --tunnel-through-iap \
  --command="systemctl is-active clamav-freshclam && freshclam --version"
```

**Fix**

Enable the updater in the image (see 10.1). The failure mode worth testing for: **an instance with restricted egress cannot reach the signature mirror**, so the agent runs happily with definitions from the day the image was baked. If you have applied 9.2 or egress firewall rules, verify the update path explicitly:

```hcl
resource "google_compute_firewall" "allow_signature_updates" {
  name      = "allow-av-signature-egress"
  network   = google_compute_network.vpc.name
  direction = "EGRESS"
  priority  = 900

  allow {
    protocol = "tcp"
    ports    = ["443"]
  }

  destination_ranges = var.signature_mirror_cidrs
  target_tags        = ["av-agent"]
}
```

---

## 11.2 — Automated backups

**Check**

```
gcloud sql instances describe INSTANCE \
  --format="value(settings.backupConfiguration.enabled,settings.backupConfiguration.pointInTimeRecoveryEnabled)"
gcloud compute resource-policies list --filter="snapshotSchedulePolicy:*"
```

**Fix**

```hcl
resource "google_sql_database_instance" "main" {
  name             = "main"
  database_version = "POSTGRES_16"

  settings {
    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
      start_time                     = "03:00"
      transaction_log_retention_days = 7

      backup_retention_settings {
        retained_backups = 30
        retention_unit   = "COUNT"
      }
    }
  }

  deletion_protection = true
}

resource "google_compute_resource_policy" "daily_snapshot" {
  name   = "daily-snapshot"
  region = var.region

  snapshot_schedule_policy {
    schedule {
      daily_schedule {
        days_in_cycle = 1
        start_time    = "04:00"
      }
    }

    retention_policy {
      max_retention_days    = 30
      on_source_disk_delete = "KEEP_AUTO_SNAPSHOTS"
    }
  }
}

# A schedule policy attached to nothing backs up nothing.
resource "google_compute_disk_resource_policy_attachment" "data" {
  name = google_compute_resource_policy.daily_snapshot.name
  disk = google_compute_disk.data.name
  zone = var.zone
}
```

---

## 11.3 — Protect recovery data

**Check**

```
gcloud storage buckets describe gs://BACKUP_BUCKET \
  --format="value(retentionPolicy.isLocked,encryption.defaultKmsKeyName)"
```

**Fix**

```hcl
resource "google_kms_key_ring" "backup" {
  name     = "backup-keyring"
  location = var.region
  project  = var.backup_project_id
}

resource "google_kms_crypto_key" "backup" {
  name            = "backup-key"
  key_ring        = google_kms_key_ring.backup.id
  rotation_period = "7776000s" # 90 days
}

# Key access separated from production identities: a compromised workload
# identity cannot decrypt — or destroy — the backups.
resource "google_kms_crypto_key_iam_member" "backup_only" {
  crypto_key_id = google_kms_crypto_key.backup.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.backup.email}"
}

resource "google_storage_bucket" "backups" {
  name                        = "org-backups"
  project                     = var.backup_project_id
  location                    = "US"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  retention_policy {
    is_locked        = true
    retention_period = 90 * 24 * 60 * 60
  }

  encryption {
    default_kms_key_name = google_kms_crypto_key.backup.id
  }

  versioning { enabled = true }
}
```

---

## 11.4 — Isolated recovery data

The safeguard most often failed by organizations that believe they pass it. A second copy reachable with the same credentials as production is not isolation.

**Check**

```
gcloud storage buckets list --project=BACKUP_PROJECT
gcloud projects get-iam-policy BACKUP_PROJECT --format=json \
  | jq '.bindings[] | select(.members[] | contains("prod"))'
```

Any production identity appearing in the backup project's IAM policy is a finding.

**Fix**

```hcl
# Separate folder, separate inheritance path
resource "google_folder" "backup" {
  display_name = "backup"
  parent       = "organizations/${var.org_id}"
}

resource "google_project" "backup" {
  name            = "org-backup"
  project_id      = var.backup_project_id
  folder_id       = google_folder.backup.name
  billing_account = var.billing_account
}

# Only this identity writes backups. It has no production access,
# and no production identity has any role here.
resource "google_service_account" "backup" {
  account_id = "backup-writer"
  project    = var.backup_project_id
}

resource "google_storage_bucket_iam_member" "backup_writer" {
  bucket = google_storage_bucket.backups.name
  role   = "roles/storage.objectCreator" # create only — cannot delete
  member = "serviceAccount:${google_service_account.backup.email}"
}

# Cross-region copy
resource "google_storage_bucket" "backups_dr" {
  name                        = "org-backups-dr"
  project                     = var.backup_project_id
  location                    = "EU"
  uniform_bucket_level_access = true

  retention_policy {
    is_locked        = true
    retention_period = 90 * 24 * 60 * 60
  }
}
```

**Test the isolation.** Attempt to delete a backup object using a production identity and confirm you are denied. An untested isolation claim is an assumption.

---

## 12.1 — Network infrastructure currency

**Check**

```
gcloud compute ssl-policies list --format="table(name,minTlsVersion,profile)"
gcloud compute vpn-gateways list
gcloud compute networks list --format="table(name,x_gcloud_subnet_mode)"
```

`LEGACY` subnet mode or a missing SSL policy are both findings.

**Fix**

```hcl
resource "google_compute_ssl_policy" "modern" {
  name            = "modern-tls"
  profile         = "RESTRICTED"
  min_tls_version = "TLS_1_2"
}

resource "google_compute_target_https_proxy" "app" {
  name             = "app-https-proxy"
  url_map          = google_compute_url_map.app.id
  ssl_certificates = [google_compute_managed_ssl_certificate.app.id]
  ssl_policy       = google_compute_ssl_policy.modern.id
}

resource "google_compute_security_policy" "armor" {
  name = "app-armor-policy"

  rule {
    action   = "deny(403)"
    priority = 1000
    match {
      expr { expression = "evaluatePreconfiguredExpr('xss-stable')" }
    }
  }

  rule {
    action   = "allow"
    priority = 2147483647
    match {
      versioned_expr = "SRC_IPS_V1"
      config { src_ip_ranges = ["*"] }
    }
  }
}
```

---

## 15.1 — Service provider inventory

**Check**

```
gcloud asset search-all-iam-policies \
  --scope=organizations/ORGANIZATION_ID \
  --query="policy:(-domain:YOURDOMAIN.com)" \
  --format="table(resource,policy.bindings.members)"
gcloud iam workload-identity-pools list --location=global --project=PROJECT_ID
```

**Fix**

```hcl
# The enforcement point: no IAM grant outside your tenant
resource "google_org_policy_policy" "allowed_domains" {
  name   = "organizations/${var.org_id}/policies/iam.allowedPolicyMemberDomains"
  parent = "organizations/${var.org_id}"

  spec {
    rules {
      values { allowed_values = var.allowed_customer_ids }
    }
  }
}
```

**Also required:** the inventory itself is a document, not a resource. Record each provider, what data it touches, and its review date. The constraint prevents new undocumented grants; it does not enumerate existing ones.

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

**Find existing external grants the constraint will not remove**

```
gcloud asset search-all-iam-policies \
  --scope=organizations/ORGANIZATION_ID \
  --format=json \
  | jq -r '.[] | .resource as $r | .policy.bindings[]?.members[]?
           | select(test("@YOURDOMAIN\\.com$") | not)
           | select(startswith("user:") or startswith("group:") or startswith("serviceAccount:"))
           | "\($r)\t\(.)"' | sort -u
```

**Find external identity federation trusts**

```
gcloud projects list --format="value(projectId)" | while read p; do
  gcloud iam workload-identity-pools list --location=global --project="$p" \
    --format="value(name)" 2>/dev/null | sed "s|^|$p / |"
done
```

---

## 17.2 — Incident contact information

**Check**

```
gcloud essential-contacts list --organization=ORGANIZATION_ID
```

Empty output means Google's security notifications are going to whoever originally created the org.

**Fix**

```hcl
resource "google_essential_contacts_contact" "security" {
  parent                              = "organizations/${var.org_id}"
  email                               = "security@${var.domain}"
  language_tag                        = "en-GB"
  notification_category_subscriptions = ["SECURITY", "TECHNICAL"]
}

resource "google_essential_contacts_contact" "legal" {
  parent                              = "organizations/${var.org_id}"
  email                               = "legal@${var.domain}"
  language_tag                        = "en-GB"
  notification_category_subscriptions = ["LEGAL", "SUSPENSION"]
}

resource "google_org_policy_policy" "contact_domains" {
  name   = "organizations/${var.org_id}/policies/essentialcontacts.allowedContactDomains"
  parent = "organizations/${var.org_id}"

  spec {
    rules {
      values { allowed_values = ["@${var.domain}"] }
    }
  }
}
```

Use monitored group addresses, never individuals. Send a test message and confirm receipt — an address that bounces silently is the same as no address.

> ### ⚠️ NOT RETROACTIVE
>
> The constraint above blocks **future** non-conforming operations only. Everything already in the estate stays exactly as it is, and the console will show the policy as enforced while the violations sit there untouched.
>
> **Applying the constraint is half the fix. Find and remediate what already exists.**

The domain constraint blocks **new** contacts outside your domain. Contacts already configured — including personal addresses and departed employees — stay in place.

**Find every contact currently configured**

```
gcloud essential-contacts list --organization=ORGANIZATION_ID \
  --format="table(email,notificationCategorySubscriptions.list())"

gcloud projects list --format="value(projectId)" | while read p; do
  gcloud essential-contacts list --project="$p" --format="value(email)" 2>/dev/null \
    | sed "s|^|$p / |"
done
```

**Find monitoring channels pointing at stale addresses**

```
gcloud alpha monitoring channels list --project=PROJECT_ID \
  --format="table(displayName,type,labels.email_address,enabled)"
```

---

## 17.3 — Incident reporting

**Check**

```
gcloud scc notifications list --organization=ORGANIZATION_ID
gcloud alpha monitoring channels list --project=PROJECT_ID
```

**Fix**

```hcl
resource "google_scc_notification_config" "critical" {
  config_id    = "critical-findings"
  organization = var.org_id
  pubsub_topic = google_pubsub_topic.scc_findings.id

  streaming_config {
    filter = "state=\"ACTIVE\" AND severity=\"CRITICAL\""
  }
}

resource "google_monitoring_notification_channel" "oncall" {
  display_name = "Security on-call"
  type         = "pubsub"
  project      = var.security_project_id

  labels = {
    topic = google_pubsub_topic.alerts.id
  }
}

resource "google_monitoring_alert_policy" "org_policy_tampering" {
  display_name = "Org policy modified"
  project      = var.security_project_id
  combiner     = "OR"

  conditions {
    display_name = "Org policy change detected"
    condition_threshold {
      filter          = "metric.type=\"logging.googleapis.com/user/org-policy-changes\""
      duration        = "0s"
      comparison      = "COMPARISON_GT"
      threshold_value = 0
    }
  }

  notification_channels = [google_monitoring_notification_channel.oncall.id]
}
```

**Verify the channel works.** Alerting into an unmonitored mailbox is the most common way this safeguard fails in practice.

---

## Process safeguards

These have no Terraform fix. They are satisfied by a written, dated, owned document — and an auditor will ask to see it, not to see your infrastructure.

| Safeguard | What the evidence looks like |
|---|---|
| **3.1** Data management process | Written process covering classification, handling, retention, disposal. Reviewed annually with the date recorded. |
| **3.5** Secure data disposal | Documented disposal procedure per storage service, including snapshots and backups. Disposal actions logged. |
| **4.1** Secure configuration process | Written baseline for GCP resources, with the enforcement mechanism named. |
| **5.3** Dormant accounts *(partly)* | Defined dormancy threshold and a recurring review that runs on schedule, not on request. |
| **6.1** Access granting process | Documented request and approval flow, with approvals retained. |
| **6.2** Access revoking process | Revocation procedure covering IAM, groups, keys, and sessions, with an SLA and completion verified rather than assumed. |
| **7.1** Vulnerability management process | Named sources, named owners, annual review. |
| **7.2** Remediation process | SLAs by severity, risk acceptance with expiry dates, findings tracked to closure. |
| **8.1** Audit log management process | Retention periods per log type, log sources enumerated, exclusion filters justified. |
| **11.1** Data recovery process | RPO and RTO per workload tier, recovery roles named, dated restore test evidence. |
| **17.1** Incident handling personnel | A named individual and deputy, reviewed annually. |

The most common audit failure here is not the absence of a process but the absence of a **date**. A process document with no review date cannot be shown to be current.

---

*CIS Controls® is a registered trademark of the Center for Internet Security, Inc. This is an implementation aid and is not affiliated with or endorsed by CIS. Terraform examples are illustrative and unvalidated — test in a non-production environment before use.*
