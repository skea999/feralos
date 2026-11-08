# aports source pinning

> **Double documentation.** Condensed feralos-side copy of the policy — full,
> canonical text lives in `feralos-aports` `README.md` → **"Source pinning
> policy"**. AGENTS.md carries the live fact short version under
> "aports Source Pinning".

## Rule (2026-11-08, user decision)

Every GitHub source in `feralos-aports` pins to a **TAG archive**
(`https://github.com/<org>/<repo>/archive/refs/tags/<tag>/...`) — **NEVER a
commit archive** (`archive/<sha>...`).

- The **sha512sum is the integrity guard**; the **tag is the version contract**.
- If upstream re-publishes a tag to fix a bug/CVE, the checksum FAILS = the
  signal to review the new content and port the fix. Commit pins would
  silently keep old bytes forever; tags give version semantics + fail-loud.

## Checksum-failure workflow

1. Fetch the new tag archive.
2. Extract both old and new archives, `diff -r` after normalizing the top
   dir (the only expected change is the directory name).
3. Legit fix → update `sha512sums`, bump `pkgrel`, note the ported fix in
   the commit message.
4. Unexpected/hostile diff → STOP and report; investigate upstream before
   shipping anything.

## Non-GitHub URLs + our own repos

- Non-GitHub URLs (Alpine mirrors, upstream hosts, GitLab): versioned
  release artifacts are fine; moving targets (`main`/`master`/HEAD) must be
  fixed to a pinned versioned artifact.
- Our own repos (`skea999/feralos`, `skea999/ursine`,
  `skea999/dms-window-buttons`): cut a tag for each package-relevant source
  state instead of commit pins (e.g. tag `dbus-audit-0.1.0` at the feralos
  commit the `feralos-dbus-audit` APKBUILD is cut from).

## Inventory 2026-11-08

17 archive sources across 16 packages, ALL tag archives; no release-asset
URLs, no non-GitHub sources, no moving targets. Commit-pin reverts landed
this day: `dms-shell` → `v1.6.2` (+ `dank-qml-common` → `v1.6.0` — commit
`26396ce…` is 1 commit behind the tag tip, the delta being a trivial `VERSION`
bump) and `feralos-dbus-audit` → feralos tag `dbus-audit-0.1.0`. The dux-shell
`v1.6.2` re-fetch sum equals the originally validated `7ccb2361…`; the tag
archive content is byte-identical to the previously pinned commit archive.
