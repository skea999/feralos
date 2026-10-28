# Repository Audit & History Scrub

Reusable procedure to keep this repository **credential-free, anonymous
(no machine-local identifiers) and English-only** — working tree AND full git
history. Run it before every public push / release.

## What we audit

| Category | Rule |
|----------|------|
| Credentials | No private keys, tokens, API keys, passwords in tree or history. Test-fixture credentials (`Alpine.1`, QEMU fixtures) are the documented exception |
| Identity | The **repository identity** (`skea999`, org URLs, `dev@feralos.net`-style project contacts) is public by design and stays. **Machine-local identifiers are forbidden**: local usernames (`skem`, `~/projects`, `~/projects/Progetti/...`), hardcoded home paths of any user (`/home/<user>` in code — use config), personal emails other than the project contact |
| Language | English-only, working tree and history. Legacy Italian is scrubbed via history rewrite (below) |

## 1. Content audit (working tree)

```sh
# credentials (expect: no hits outside docs/CI.md secret NAMES)
git ls-files -co --exclude-standard | xargs grep -inE \
  'BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[bap]-|Authorization: Bearer [A-Za-z0-9]'

# machine-local identifiers (expect 0)
git ls-files -co --exclude-standard | xargs grep -n '~/projects' 2>/dev/null

# emails outside the project contact
git ls-files -co --exclude-standard | xargs grep -inE \
  '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}' | grep -v 'dev@feralos.org'

# Italian (English-only)
git ls-files '*.md' '*.go' '*.yaml' | xargs grep -lnE \
  'english-scrub-markers'
```

## 2. Artifact

Save the audit output under (already gitignored by the `tests/artifacts/**/*`
rule — structure kept, content never committed):

```text
tests/artifacts/audits/<YYYY-MM-DD>/report.md
```

Report format: findings per category (CREDENTIALS / NAMES / LANGUAGE), each
marked CLEAN, INTENTIONAL, or FIXED.

## 3. History scrub (anonymize + English-only)

Full recipe and script: `scripts/history-scrub.sh` + mappings in
`scripts/history-replacements.txt`. Requires `pip3 install git-filter-repo`.

```sh
# 0. commit any working-tree fixes first (English commit message)

# 1. list Italian lines still present anywhere in history
./scripts/history-scrub.sh extract
#    -> tests/artifacts/audits/history-italian-remaining.txt
#    for each REAL Italian line add a mapping to scripts/history-replacements.txt:
#        original text==>english text

# 2. rewrite ALL branches (deletes refs/replace/* grafts, applies mappings)
./scripts/history-scrub.sh rewrite

# 3. force push every branch (AUTHORIZED destructive operation)
./scripts/history-scrub.sh push

# 4. verify from a fresh clone (expect 0/0)
./scripts/history-scrub.sh verify
```

### Notes

- `git filter-repo` rewrites **all local branches** (`main`, `develop`,
  `developDinit`, `alpineInstallerGolang`) — the per-branch Italian fixes are
  covered by the mappings, no per-branch checkout needed.
- The repository accumulated hundreds of `refs/replace/*` grafts from earlier
  operations; the scrub deletes them (they alter what `git log` shows).
- Force push rewrites shared history: everyone must re-clone.
- After rewrite, re-clone and re-run section 1 — both must be clean.

## 4. Identity policy (summary)

| Allowed (public project identity) | Forbidden (machine/personal) |
|-----------------------------------|------------------------------|
| `skea999` handle, org URLs, repo owner in README/CONTRIBUTING | local usernames (`skem`), `~/projects` paths |
| project contact `skea999@cryptolab.net` | any other personal email |
| upstream maintainer attribution (e.g. Alpine packagers) in docs | hardcoded `/home/<user>` in code — use config |
