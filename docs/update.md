# Update Reference

Run these from the project root. Harness targets **OpenCode v2 only** — no Claude Code, no Continue.

Node version matters: the `skills` CLI needs Node ≥ 22.20, `agents-opencode` ≥ 4.3.5 needs Node 24.
System node is 18 → **always wrap npx calls**: `nix-shell -p nodejs_24 --run "npx -y <pkg> ..."`
(under node 18, `npx agents-opencode` silently resolves the old V1-era pack 2.1.1).

## OpenCode CLI

Installed via nix. Upgrade with:

```bash
nix profile upgrade 'nixpkgs#opencode'
```

V2 runs a background service (`opencode service status`); config changes hot-reload, no restart needed.

## agents-opencode pack (`.opencode/`) — FIRST, before skills

```bash
nix-shell -p nodejs_24 --run "npx -y agents-opencode --update"
```

The pack targets v2 when it detects OpenCode major 2 (check log: `Install target: v2`).
If `.opencode/package-lock.json` gets pruned, regenerate it:

```bash
cd .opencode && nix-shell -p nodejs_24 --run "npm install --no-audit --no-fund" && cd ..
```

**Post-update conversion (pack ships V1 frontmatter on every release):**

1. Agent frontmatter — per file, all at once (V1 and V2 fields must not mix in one file):
   - `permission:` map → `permissions:` ordered list `{action, resource, effect}`;
     order = broad `"*": <effect>` rules FIRST, specific exceptions LAST
     (V2 is last-match-wins — official permissions doc);
     rename actions `bash`→`shell`, `task`→`subagent`, `write`/`patch`→`edit` (dedupe)
   - `temperature:` → drop (no V2 effect on agents)
   - `disable:` → `disabled:`; `tools:` maps → deny/allow permission rules
2. Command frontmatter: `subtask:` → `subagent:`
3. Quote `description:` values containing `: ` (YAML rejects unquoted multi-colon scalars)
4. Quote `resource:`/`action:` values equal to `*` or starting with `*` (`*.env`, `*`) —
   a bare leading `*` is a YAML alias token; always emit them as `"*"` / `"*.env"`

Converter script pattern: `/tmp/opencode/migrate_frontmatter.py` (python3 + pyyaml).

**Project `opencode.json`**: the installer refuses `permissions` merges while the V1
`permission` key exists (v1/v2 invariant). Keep the file V2-native: `permissions` array,
`agents`, `plugins` — no singular keys.

## Universal skills (`.agents/skills/`) — AFTER pack update

`skills-lock.json` tracks them via the skills.sh CLI (Vercel Labs).

```bash
nix-shell -p nodejs_24 --run "npx skills update -y"
```

**Known side effect**: `skills update` has NO agent scoping — it re-links skills into
every agent it detects on the machine (`.claude/skills/`, `.continue/skills/`, …).
This repo is opencode-only. After every update:

```bash
rm -rf .claude .continue
```

Both are gitignored, so leftovers never reach a commit. To restore skills from the lock
with explicit scoping (no cross-links), use:

```bash
nix-shell -p nodejs_24 --run "npx skills experimental_install -a opencode"
```

(`-a opencode` maps to `.agents/skills/` — the OpenCode project path per skills.sh.)

## Caveman + Cavecrew (GLOBAL only)

No project-local caveman install. The installer writes to `~/.config/opencode` (XDG) only;
its skills go to `~/.config/opencode/skills/` with `-g`. Re-run to overwrite with latest:

```bash
# Step 1 — plugin + commands + AGENTS.md (global opencode only)
nix-shell -p nodejs_24 --run "npx -y github:JuliusBrussee/caveman -- --force --only opencode"

# Step 2 — caveman + cavecrew skills, GLOBAL scope (-g, never project: repo stays clean)
nix-shell -p nodejs_24 --run "npx skills add JuliusBrussee/caveman -a opencode -g -y"

# Step 3 — cavecrew agents
for a in cavecrew-investigator cavecrew-builder cavecrew-reviewer; do
  curl -fsSL "https://raw.githubusercontent.com/JuliusBrussee/caveman/main/agents/$a.md" \
    -o ~/.config/opencode/agents/$a.md
done
```

**Post-install fixes (global `~/.config/opencode/`, outside the repo):**

1. `plugins/caveman/index.js` shim — v2 plugin discovery ignores `package.json` `main`,
   so re-export the v2 entry: `export { default } from './server.js';`
   (upstream `server.js` already carries the v2 `Plugin.define` adapter)
2. `opencode.json`: keep only the v2 `plugins` array; delete any legacy `plugin` key
   (duplicate load + "configured plugin path must be a directory" warnings)

## Template agents (dmicheneau) — DO NOT RUN

The agents-opencode pack ≥ 4.3.5 already ships every agent that pack installed
(flat in `.opencode/agents/`). Running the template installer creates duplicate
definitions in `.opencode/agents/devtools/` and `.opencode/agents/security/`.
Removed from this procedure on purpose.

## Project plugins (opencode.json)

- `opencode-rules@latest` — v2-native, auto-updates on OpenCode startup
- `agents-opencode` — loaded from `.opencode/plugins/agents-opencode.ts` (pack), updated above

## Global rules / AGENTS.md

`~/.config/opencode/rules/general.md` and `~/.config/opencode/AGENTS.md` are static.
Review and edit manually if the global policy changes.

## Context7 MCP API key

If the key in `~/.config/opencode/opencode.json` expires, replace it from https://context7.com.

## Verify after updating

```bash
opencode reload
grep -E 'loading plugin|failed to load' ~/.local/share/opencode/log/opencode.log | tail -10
```

Expect all three plugins (`caveman`, `agents-opencode.ts`, `opencode-rules`) in
`loading plugin` lines and zero `failed to load`. Then one live run:

```bash
timeout 120 opencode run "reply with exactly: OK"
```

## Notes

- User-created agents are safe from overwrites (no manifest entry).
- User-modified skills (`session-memory`, `using-agent-skills`) are excluded from
  `skills-lock.json` — `npx skills update` skips them.
- Caveman step 1 is idempotent; it may reset the global `opencode.json` plugin key —
  re-apply the post-install fixes above if it does.
