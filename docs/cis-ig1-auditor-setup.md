# Auditor command-line setup

How to get a shell ready to run the CIS IG1 audit: signed in, on your own branch of the engagement repository, running as the audit service account, with an output directory and config in place.

- **[Part 1](#part-1--every-audit)** is the sequence for every audit session, start to finish.
- **[Part 2](#part-2--cloud-shell-one-time-setup)** is a one-time Cloud Shell setup that turns Part 1's identity steps into one command: `audit-on`.

The service account already exists and holds its roles. Creating it is covered in [`terraform/audit-service-account/`](../terraform/audit-service-account/) and is not repeated here.

You need three values from whoever set up the engagement:

| Value | Example | Where it comes from |
|---|---|---|
| Organization ID | `123456789012` | `gcloud organizations list` |
| Audit host project | `acme-cis-audit` | The project the service account and APIs live in |
| Engagement repository | `git@github.com:acme/cis-ig1-audit.git` | The customer's private repo, which carries the kit |

---

## Part 1 — Every audit

### 1. Open a shell

**Cloud Shell** is the recommended shell. Open [shell.cloud.google.com](https://shell.cloud.google.com) signed in as your own auditor account. It already has `gcloud` with the `alpha` and `beta` components, `jq`, `git` and Go.

Working locally instead, confirm the tooling:

```bash
gcloud version | head -1 && jq --version && go version && git --version
```

```bash
gcloud components list --filter="id:(alpha beta)" --format="value(id,state.name)"
```

Neither `alpha` nor `beta` may say `Not Installed` — seven checks need them. Install with `gcloud components install alpha beta`.

### 2. Sign in as yourself

```bash
gcloud auth login
```

Cloud Shell signs you in automatically, so you only need this when the session has expired. It does expire, on the Workspace session timeout, and often mid-run:

```
Reauthentication failed. cannot prompt during non-interactive execution.
```

When you see that, run `gcloud auth login` and start the interrupted pass again. Nothing about the audit setup has changed; only your sign-in has lapsed.

Application Default Credentials (`gcloud auth application-default login`) are **not** needed. The audit calls `gcloud` and nothing else.

### 3. Be on your branch

You already have the repository: cloning it is a one-time job per machine, folded away below. Cloud Shell keeps your home directory between sessions, so the clone, your SSH key and your `gh` credentials all survive there too.

Move to your own branch and pick up whatever landed on `main` since you were last here:

```bash
git switch "$(whoami)/cis-ig1-audit" && git pull && git merge main
```

Merge rather than rebase. The branch holds committed results, and a rebase rewrites commits that may already be pushed.

<details>
<summary><strong>First time on this machine</strong> — credentials, clone, branch</summary>

Cloud Shell needs GitHub credentials to clone a private repository. Use the GitHub CLI if it is present:

```bash
gh --version && gh auth login
```

If `gh` is not installed, create an SSH key and add the public half to your GitHub account at **Settings → SSH and GPG keys**:

```bash
ssh-keygen -t ed25519 -C "$(gcloud config get-value account)"
```

```bash
cat ~/.ssh/id_ed25519.pub
```

Clone, then branch from an up-to-date `main`. Name the branch after yourself:

```bash
git clone git@github.com:CUSTOMER_ORG/CUSTOMER_REPO.git && cd CUSTOMER_REPO
```

```bash
git switch main && git pull && git switch -c "$(whoami)/cis-ig1-audit"
```

`whoami` in Cloud Shell is your account name without the domain, so `dana@acme.com` gets `dana/cis-ig1-audit`. Type the name yourself if you work locally under a different user.

</details>

### 4. Set the variables

```bash
export ORG_ID="REPLACE_ORG_ID"
export AUDIT_PROJECT="REPLACE_AUDIT_PROJECT"
export SA_EMAIL="cis-ig1-auditor@${AUDIT_PROJECT}.iam.gserviceaccount.com"
```

`run-audit.sh` reads `ORG_ID`. `SA_EMAIL` is used in the next step.

These last only as long as the shell. A new tab, or a Cloud Shell reconnect, starts without them. Part 2 makes them permanent.

### 5. Switch to the audit service account

```bash
gcloud config set project "$AUDIT_PROJECT"
```

```bash
gcloud config set auth/impersonate_service_account "$SA_EMAIL"
```

Every `gcloud` command from here prints a line starting `WARNING: This command is using service account impersonation`. That line is expected; the audit strips it from its output.

`gcloud auth list` still shows **your** address. That is correct. Impersonation does not change who is signed in — gcloud trades your credential for a short-lived service account token on each call, and that is why the audit log records both identities.

### 6. Prove it took effect

Ask Google whose token this is. The call only reads:

```bash
curl -s "https://oauth2.googleapis.com/tokeninfo?access_token=$(gcloud auth print-access-token)" | jq -r .email
```

| Output | Meaning |
|---|---|
| `cis-ig1-auditor@…` | Correct. Every audit command now runs as the auditor. |
| Your own address | Impersonation is not on. Repeat step 5. |
| `Failed to impersonate` | Your grant on the service account has not propagated. Wait a minute and retry. |

**Do not test this by creating something.** A denied write proves nothing, because you are denied whether or not impersonation is on. A write that succeeds has put a real resource in the customer's project, and the attempt is in their Admin Activity log either way.

### 7. Settings, then the output directory

Settings live in `config/`, output goes under `audit-state/`. They are kept apart on purpose: clearing out old runs must never cost you your configuration.

Copy the example — only if you have not already, since this is the file you filled in on an earlier day:

```bash
[ -f config/audit.env ] || cp config/audit.env.example config/audit.env
```

Open it and fill it in:

```bash
cloudshell edit config/audit.env
```

Locally, use `vi config/audit.env` or your editor.

| Value | Set it to |
|---|---|
| `ORG_ID` | The numeric organization ID from `gcloud organizations list`. Set it here and no command needs `--org` again. |
| `AUDIT_PROJECT` | The project that owns the audit service account — the one from step 4. |
| `SA_EMAIL` | The service account you impersonate, from step 4. |
| `APPROVED_REGISTRIES` | The registry prefixes images may come from, comma-separated with no spaces. **Ask the customer.** It is policy, not what happens to be in use. Example: `us-docker.pkg.dev/acme,gcr.io/acme,gke.gcr.io`. If they have no list, write `none`, not blank: blank skips the two checks, `none` fails them, which is the truth. |
| `ALLOWED_LOCATIONS` | Leave commented out. It defaults to the continental United States. Set it only for data that legitimately lives elsewhere, and give the whole list, because the value replaces the default rather than extending it. |
| `EXCLUDE_PROJECTS` | Leave at `^sys-`, which skips the projects Apps Script creates. |

`config/audit.env` is gitignored: it holds the customer's naming, registries and residency policy, which are theirs to publish rather than yours. [`config/readme.md`](../config/readme.md) has the full reference.

Then the output directory, which **is** committed on your branch:

```bash
mkdir -p ./audit-state/runs ./audit-state/projects
```

| Path | Holds |
|---|---|
| `audit-state/runs/<timestamp>/` | One directory per `run-audit.sh` run — report and evidence |
| `audit-state/projects/` | Output from project passes run by hand with `audit-run.go` |

If the customer's estate is large enough that you will work through it project by project, write the full project list now — the organization score divides by it:

```bash
gcloud projects list --filter='lifecycleState:ACTIVE' --format='value(projectId)' > config/projects.txt
```

### 8. Run and commit

The CLI is ready. Run the audit with output pointed at `audit-state/runs`:

```bash
./run-audit.sh --out ./audit-state/runs --all
```

Use `--project ID` (repeatable) or `--projects FILE` in place of `--all` to audit a subset. See [cis-ig1-scripted-audit.md](cis-ig1-scripted-audit.md) for the options.

The run ends by printing a `--review` command. Run it to decide each REVIEW check PASS or FAIL. Completion reaches 100% when the last one is decided — see [the score and `--review`](../readme.md#the-score-and---review):

```bash
./run-audit.sh --review ./audit-state/runs/<timestamp>
```

Commit the results to your branch and push.

**First, on the engagement repository only, allow it.** The kit ships with
`/audit-state/` gitignored, because the kit's own repository is public and
audit output names real projects, buckets and IAM principals. In the
customer's private repository the results are the deliverable, so delete that
line from `.gitignore` once, on your first run:

```bash
sed -i.bak '\|^/audit-state/$|d' .gitignore && rm -f .gitignore.bak
```

Then:

```bash
git add audit-state && git commit -m "CIS IG1 audit run $(date +%Y-%m-%d)"
```

Name the paths you mean. `git add -A` sweeps in whatever else is lying
around — it is how a live IAM inventory reached a public repository twice.

```bash
git push -u origin "$(whoami)/cis-ig1-audit"
```

Stage `audit-state` by name rather than `git add -A`, so nothing else in the working tree goes with it.

### 9. When you are finished

```bash
gcloud config unset auth/impersonate_service_account
```

If you leave impersonation set, every later `gcloud` command in this account still runs as the auditor, in this tab and every other one, including the commands you mean to run as yourself. Cloud Shell keeps `gcloud` settings between sessions, so the setting is still there next week.

Part 2 removes this step. Impersonation then lives in a separate configuration that is only active in tabs where you ask for it.

---

## Part 2 — Cloud Shell one-time setup

Cloud Shell keeps your home directory between sessions, and every new tab runs `~/.bashrc`. This setup stores the engagement values there and adds three commands:

| Command | Does |
|---|---|
| `audit-on` | This tab runs as the auditor: switches configuration, goes to the repo, confirms identity, marks the prompt `[AUDIT]` |
| `audit-off` | This tab runs as you again |
| `audit-whoami` | Prints whose token this tab is using |

You are signed in automatically, the values are set automatically, and **nothing runs as the auditor until you type `audit-on`**.

**Why not impersonate in every tab automatically?** An auditor who also works in the same Google account would then run every `gcloud` command as the audit service account without meaning to. The auditor is read-only, so the risk is not damage. It is confusion: a command fails with "permission denied" and nothing says why. It also muddies the audit trail, which should show the auditor's activity and nothing else.

### How it works

The impersonation setting lives in a **named gcloud configuration**, `cis-audit`, separate from your `default` one. `audit-on` sets `CLOUDSDK_ACTIVE_CONFIG_NAME=cis-audit` for the current tab only. Every command started from that tab — `gcloud`, `run-audit.sh`, `audit-run.go` — inherits the setting. Other tabs, and your default configuration, are untouched.

That is the difference from step 5 of Part 1. `gcloud config set` changes the configuration **every** tab uses. The environment variable changes one tab.

### A. Create the `cis-audit` configuration

Fill in the two values, then paste the block:

```bash
AUDIT_PROJECT="REPLACE_AUDIT_PROJECT"
SA_EMAIL="cis-ig1-auditor@${AUDIT_PROJECT}.iam.gserviceaccount.com"
ME="$(gcloud config get-value account 2>/dev/null)"

gcloud config configurations create cis-audit --no-activate
gcloud config set account "$ME" --configuration=cis-audit
gcloud config set project "$AUDIT_PROJECT" --configuration=cis-audit
gcloud config set auth/impersonate_service_account "$SA_EMAIL" --configuration=cis-audit
```

`--no-activate` leaves `default` as the active configuration, so your other tabs are not affected. `account` is set explicitly because a new configuration starts without one.

Check that `default` is still the active one:

```bash
gcloud config configurations list
```

### B. Add the block to `~/.bashrc`

Fill in the three values at the top, then paste the whole block. It appends to `~/.bashrc` once. Pasting it again changes nothing.

```bash
AUDIT_ORG_ID="REPLACE_ORG_ID"
AUDIT_PROJECT="REPLACE_AUDIT_PROJECT"
AUDIT_REPO="$HOME/REPLACE_CUSTOMER_REPO"

grep -q '# >>> cis-ig1-audit >>>' ~/.bashrc || {
cat >> ~/.bashrc <<EOF

# >>> cis-ig1-audit >>>
export ORG_ID="$AUDIT_ORG_ID"
export AUDIT_PROJECT="$AUDIT_PROJECT"
export SA_EMAIL="cis-ig1-auditor@\${AUDIT_PROJECT}.iam.gserviceaccount.com"
export AUDIT_REPO="$AUDIT_REPO"
EOF
cat >> ~/.bashrc <<'EOF'

audit-whoami() {
  curl -s "https://oauth2.googleapis.com/tokeninfo?access_token=$(gcloud auth print-access-token 2>/dev/null)" | jq -r '.email // "no valid token — run: gcloud auth login"'
}

audit-on() {
  export CLOUDSDK_ACTIVE_CONFIG_NAME=cis-audit
  [ -n "$_AUDIT_PS1" ] || _AUDIT_PS1="$PS1"
  PS1="[AUDIT] $_AUDIT_PS1"
  [ -d "$AUDIT_REPO" ] && cd "$AUDIT_REPO"
  echo "running as: $(audit-whoami)"
}

audit-off() {
  unset CLOUDSDK_ACTIVE_CONFIG_NAME
  [ -n "$_AUDIT_PS1" ] && PS1="$_AUDIT_PS1"
  unset _AUDIT_PS1
  echo "running as: $(audit-whoami)"
}
# <<< cis-ig1-audit <<<
EOF
}
```

The first half is written with the values expanded, so `~/.bashrc` holds your organization and project. The second half is written exactly as shown, so the functions run fresh each time you call them.

### C. Try it

```bash
source ~/.bashrc
```

```bash
audit-on
```

Expected output: `running as: cis-ig1-auditor@…`, the prompt starts with `[AUDIT]`, and you are in the repository.

Open a second tab and run `audit-whoami` there. It should print **your own** address. That confirms the audit identity stayed in the first tab.

```bash
audit-off
```

Expected output: `running as:` followed by your own address.

### Every audit after this

Part 1 shrinks to:

```bash
audit-on
```

```bash
git switch main && git pull && git switch "$(whoami)/cis-ig1-audit" && git merge main
```

```bash
./run-audit.sh --out ./audit-state/runs --all
```

Then commit as in [step 8](#8-run-and-commit), and `audit-off` or close the tab. Step 7 is needed only on the first run of an engagement, and step 2 only when the session has expired.

### Things to know about Cloud Shell

- **Your home directory is deleted after 120 days without use.** The repository is safe once pushed. The `~/.bashrc` block and the `cis-audit` configuration are not — if they have gone, repeat A and B.
- **Ephemeral mode keeps nothing.** If Cloud Shell was opened in ephemeral mode (it says so in the terminal banner), none of this setup survives the session. Open a normal session.
- **Sign-in still expires.** `audit-whoami` prints `no valid token — run: gcloud auth login` when that happens. Signing in again restores `audit-on` in every tab, and you do not need to redo A or B.

### Removing it at the end of the engagement

```bash
sed -i '/# >>> cis-ig1-audit >>>/,/# <<< cis-ig1-audit <<</d' ~/.bashrc
```

```bash
gcloud config configurations delete cis-audit
```

Then open a new tab so no shell still holds the old functions.
