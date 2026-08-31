# 1. What this is

We're going to be auditing our GCP Organization against CIS IG1 and reporting the result. This session covers what that framework actually is and how it maps onto work you already do.

No customer in the room, so we can be direct about how it really works.

---

## The setup

There's no external audit team. We measure our own org, we report the result, and we write the remediation as code. Management reviews and applies it.

So we're doing both halves — finding the gaps and fixing them — which is unusual and worth knowing going in.

---

## Our scope: Google Cloud

We're responsible for the GCP Organization and nothing else.

For anything that lives outside GCP, the test is whether it engages with GCP:

| | In scope? |
|---|---|
| A public Cloud Storage bucket | Yes — it's GCP |
| 2SV enforcement in the Cloud Identity Admin Console | Yes — not GCP, but it gates GCP access |
| Laptop disk encryption | No |
| Security awareness training | No |

Twelve of the 56 IG1 safeguards fall outside that line. We name them, note who owns them, and move on.

Everything else is ours — including the written processes about our GCP estate, which surprises people.

> **Speaker note:** Worth settling this early, otherwise the "do we own laptop encryption?" question comes up halfway through the mapping section.

---

## What's actually new here

Technically, not much. You've argued for most of IG1 in design reviews already — don't leave the bucket public, stop passing service account keys around, turn on Data Access logs before you need them, keep backups in a separate project.

What's new is the vocabulary. Same finding, two ways to say it:

| Design review | Audit report |
|---|---|
| "that bucket's world-readable" | "Safeguard 3.3 is not met" |
| "we're still shipping JSON keys to CI" | "Safeguard 5.2 is not met" |

The second version travels better — it's a numbered requirement from an external standard rather than an engineering opinion, which changes how it gets prioritised.

That's most of what this session is: learning to say the thing you already know in the form the report needs.
