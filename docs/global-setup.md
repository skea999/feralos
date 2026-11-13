# Global Setup — OpenCode AI Environment

Guide for an AI on how to install and configure the global opencode environment on a new machine.

Last verified: 2026-11-13.

## Version snapshot

| Component | Version |
|-----------|---------|
| OpenCode CLI | 2.2.6 (V2, background service) |
| Node.js | 24.14.0 via nix-shell (system node 18 is too old: skills CLI needs ≥ 22.20, agents-opencode needs 24) |
| caveman + cavecrew | latest (installer re-run, idempotent) |
| agents-opencode (project pkg) | 4.3.5 (v2 target) |
| universal skills (project `.agents/skills/`) | 40 via skills-lock.json (`npx skills update`) |
| global skills (`~/.agents/skills/`) | caveman suite (23) + ast-grep, ast-grep-outline |
| context-mode | 2.1.10 (`opencode plugin add`) |
| opencode-rules | @latest (auto-updates) |
| ast-grep | apk (Alpine, lags upstream) / nixpkgs#ast-grep (current) |

## Prerequisites

- OpenCode CLI 2.x installed (see §0)
- Node.js ≥ 22.20 available for npx tool calls — wrap EVERY `npx` command in
  the node24 shell (`nix-shell -p nodejs_24`); under an older system node,
  `npx agents-opencode` silently resolves the old V1-era pack

## 0. OpenCode CLI

Two equivalent install paths — keep the one matching the distro.

### Alpine Linux

```bash
# Alpine: install via the official installer (static binary)
curl -fsSL https://opencode.ai/install | bash
# V2 replaces the V1 binary in place; no side-by-side install
```

### Any distro with Nix

```bash
nix profile install nixpkgs#opencode
nix profile upgrade 'nixpkgs#opencode'   # later upgrades
```

V2 runs a client-server architecture: the TUI talks to a background service
(`opencode service status`). Config changes hot-reload — no restart needed.

## 1. Caveman + Cavecrew

Installer re-run overwrites with latest — idempotent. Global only (no
project-local caveman install exists); wrap npx in the node24 shell:

```bash
# Step 1 — Plugin + commands + AGENTS.md
nix-shell -p nodejs_24 --run "npx -y github:JuliusBrussee/caveman -- --force --only opencode"

# Step 2 — Skills (caveman + cavecrew + more, 22 total, global scope)
nix-shell -p nodejs_24 --run "npx -y skills add JuliusBrussee/caveman -a opencode -g -y"

# Step 3 — Cavecrew agents (subagents; also installed by step 1 — redundant but harmless)
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/agents/cavecrew-investigator.md -o ~/.config/opencode/agents/cavecrew-investigator.md
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/agents/cavecrew-builder.md -o ~/.config/opencode/agents/cavecrew-builder.md
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/agents/cavecrew-reviewer.md -o ~/.config/opencode/agents/cavecrew-reviewer.md
```

This installs:

| What | Path | Files |
|------|------|-------|
| Plugin | `~/.config/opencode/plugins/caveman/` | `server.js` (v2 entry), `plugin.js` (v1 hooks), `caveman-config.cjs`, `caveman-parse.cjs` |
| Commands | `~/.config/opencode/commands/` | `caveman`, `ultracave`, `megacave`, `caveman-commit`, `caveman-review`, `caveman-compress`, `caveman-stats`, `caveman-help` |
| Skills | `~/.agents/skills/` + `~/.config/opencode/skills/` | 22: caveman suite, cavecrew, investigate-first, lean-build, migration, safe-refactor, surgical-patch, verify-and-stop |
| Agents | `~/.config/opencode/agents/` | `cavecrew-builder`, `cavecrew-investigator`, `cavecrew-reviewer` (mode: subagent) |
| Global AGENTS.md | `~/.config/opencode/AGENTS.md` | appends `<!-- caveman-begin -->` block, patches `opencode.json` |

### Post-install fixes required on V2

1. `index.js` shim — v2 plugin discovery ignores `package.json` `main`, so
   re-export the v2 definition from `server.js`:

   ```bash
   printf "// opencode 2.x: package.json main ignored\nexport { default } from './server.js';\n" \
     > ~/.config/opencode/plugins/caveman/index.js
   ```

   (`server.js` already carries the v2 `Plugin.define` adapter; a re-run of the
   installer deletes the shim — re-create it.)

2. Global config cleanup — the installer may re-introduce V1 keys. Keep only
   the v2 keys in `~/.config/opencode/opencode.json`: `plugins` (never a
   legacy `plugin` array) and `permissions` (never a `permission` map).

### Caveat: caveman ≥ current — no manual patching needed

Older cavecrew agent files shipped a `tools:` array (invalid in opencode ≥1.15) and needed `hidden: true`. Current upstream uses `mode: subagent` and no `tools` frontmatter — the installer output loads clean. If agent files report schema errors on reload, re-run the installer before hand-patching.

## 2. Global plugins (`~/.config/opencode/opencode.json`)

Current full global config (v2 keys only — mixing v1 and v2 keys breaks the
v1/v2 key invariant and blocks installer merges):

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "compaction": { "auto": true },
  "permissions": [
    { "action": "shell", "resource": "sudo", "effect": "deny" },
    { "action": "shell", "resource": "sudo *", "effect": "deny" },
    { "action": "shell", "resource": "doas", "effect": "deny" },
    { "action": "shell", "resource": "doas *", "effect": "deny" },
    { "action": "shell", "resource": "*", "effect": "allow" },
    { "action": "*", "resource": "*", "effect": "allow" }
  ],
  "providers": {
    "ollama": {
      "package": "aisdk:@ai-sdk/openai-compatible",
      "settings": { "baseURL": "http://127.0.0.1:11434/v1" },
      "models": { "...": { "name": "..." } }
    }
  },
  "mcp": {
    "servers": {
      "context7": {
        "type": "remote",
        "url": "https://mcp.context7.com/mcp",
        "disabled": false,
        "headers": { "CONTEXT7_API_KEY": "<api-key>" }
      }
    }
  },
  "plugins": [
    "opencode-rules@latest",
    { "package": "/home/<user>/.config/opencode/plugins/caveman" },
    "context-mode"
  ]
}
```

Plugins auto-download on first launch. `opencode-rules` uses `@latest` and auto-updates at startup. The caveman entry must be the absolute directory path (relative global paths are not resolved).

## 3. context-mode

Context-window saver: keeps raw tool output out of the context window. 9 `ctx_*` MCP tools (sandbox `ctx_execute`, `ctx_batch`, `ctx_index` + FTS5 `ctx_search`, `ctx_resume`, `ctx_stats`, `ctx_doctor`, `ctx_upgrade`, `ctx_purge`), enforced by hooks: a Read or shell dump over 16 KB is denied and redirected to the matching `ctx_*` call; hook errors fall back to exit 0 so the agent keeps working. Local, zero telemetry: one `context.db` per project under `~/.context-mode`. Verified on OpenCode V2 (`context-mode@2.1.10` ships an OpenCode 2 adapter; CI covers the plugin path). Requires Node ≥ 24.15.0 — use the `nix-shell -p nodejs_24` wrapper (check the channel version before install).

Install (native, no manual config):

```bash
opencode plugin add context-mode
opencode mcp list --standalone   # check on OpenCode 2
```

Update / remove:

```bash
opencode plugin update
opencode plugin remove context-mode
```

Hard rule: install via the native command ONLY. NEVER add a
`mcp.servers.context-mode` entry next to the plugin — plugin + legacy MCP
entry together register ZERO `ctx_*` tools.

# Verify inside a session: `Call ctx_stats and show me the report.` and
`/ctx-doctor` (every check must print PASS). Complements the native v2
compaction (compaction summarizes the tail; context-mode keeps raw output
out from the start and adds a searchable archive + session resume).
Caveat: `ctx_doctor`'s plugin-registration check looks for the v1 `plugin`
key name — against a v2 `plugins` config it reports FAIL/WARN falsely.
Treat the plugin load line in the opencode log plus a live `ctx_stats`
call as the authoritative check. TypeScript scripts in the sandbox need a
TS runtime (bun, tsx or ts-node) — JS/shell/python run without one.

Routing awareness: global `~/.config/opencode/AGENTS.md` carries a
`<!-- context-mode-begin -->` block explaining the `ctx_*` routing and the
plugin/MCP conflict rule (see section 6). Per-project routing files are
optional — add only if a project needs custom routing.

## 4. ast-grep

Structural code search/lint/rewrite over tree-sitter ASTs. Two install paths —
pick per distro:

### Alpine Linux

No musl build exists upstream: npm ships `gnu`-only platform packages, GitHub releases too. On Alpine, the apk IS the musl build.

```bash
doas apk add ast-grep          # Alpine pkg lags upstream — upgrade via apk upgrade
```

### Any distro with Nix

```bash
nix profile install nixpkgs#ast-grep
```

### Skills (both paths)

```bash
nix-shell -p nodejs_24 --run "npx -y skills add ast-grep/agent-skill -g -y -a opencode"
# skills → ~/.agents/skills/ (ast-grep + ast-grep-outline)
```

- opencode loads `~/.agents/skills/` natively — no copy into `~/.config/opencode/skills/` needed.
- Search scope = the path argument: `ast-grep run -p '<pattern>' .` from a repo root searches that repo only.
- Per-project custom rules: `sgconfig.yml` at the project root + `ast-grep scan`.
- Preference rules live in global `AGENTS.md` (`<!-- ast-grep-begin -->` block): structural queries → ast-grep, plain text → grep/rg.

## 5. Global Rule (`rules/general.md`)

`~/.config/opencode/rules/general.md`:

```markdown
- Caveman ON. Full.
- English ONLY.
- Context7 MCP for docs.
```

## 6. Global AGENTS.md

`~/.config/opencode/AGENTS.md` holds header rules plus three marker blocks:

```markdown
- Caveman ON. Full.
- English ONLY.
- Context7 MCP for docs.

<!-- caveman-begin -->
...caveman rules (installer-managed)...
<!-- caveman-end -->

<!-- context-mode-begin -->
Context-mode plugin global: plugins array ONLY (+mcp entry kills all ctx tools).
Over ~20 lines of data (logs, dumps, JSON/CSV, big files, web): ctx_execute,
print answer only. ctx_batch: commands+queries one call. ctx_index stores
docs/large files; ctx_search finds instead of re-read. ctx_doctor health.
ctx_purge deletes (session|source|project).
<!-- context-mode-end -->

<!-- ast-grep-begin -->
Structural queries: ast-grep (`ast-grep run -p '<pattern>' .`, `ast-grep scan`
with sgconfig.yml). Plain text: grep/rg. Skills: ast-grep (rules),
ast-grep-outline (structure map).
<!-- ast-grep-end -->
```

Manual blocks (context-mode, ast-grep) are paste-ready as shown above; the
caveman block is installer-managed — never hand-edit it.

## 7. Context7 MCP

Configured in the global `mcp.servers.context7` section (section 2). Requires a valid API key from https://context7.com.

## 8. Permission Configuration

Global config carries one hard rule: **no privilege escalation** — `sudo` and `doas` (Alpine's sudo-equivalent) are denied as shell patterns; everything else is allowed. V2 uses one ordered `permissions` array; per the official V2 permissions doc the last matching rule wins, and any matched `deny` hard-blocks the operation — so the sudo/doas deny patterns stay first and the general allow rules follow. Per-project configs mirror the same shape and may add project-specific restrictions.

```jsonc
"permissions": [
  { "action": "shell", "resource": "sudo", "effect": "deny" },
  { "action": "shell", "resource": "sudo *", "effect": "deny" },
  { "action": "shell", "resource": "doas", "effect": "deny" },
  { "action": "shell", "resource": "doas *", "effect": "deny" },
  { "action": "shell", "resource": "*", "effect": "allow" },
  { "action": "*", "resource": "*", "effect": "allow" }
]
```

Note: this blocks the AGENT's shell calls only — the user still runs `doas apk ...` in their own terminal.

## 9. Parked / evaluated (not installed)

- **agentmemory (rohitg00) + launcher** — rejected for now: launcher v4 requires OpenCode **V2** (now satisfied — we run 2.2.6) but is tested on Windows only. The `@agentmemory/mcp` shim was not evaluated further.
- **opencode-memory-pro** (LanceDB, was V1-compatible) — candidate for per-project long-term memory (solutions/decisions/gotchas, semantic recall). Parked pending a V2 compatibility check. If adopted: global plugin install, per-project `dbPath` override in each project's `opencode.json`.
- **@mathew-cf/opencode-memory** — rejected: global `~/opencode-memory/` store by design; we want per-project memory only.

## Verify Complete Setup

```bash
# Config
cat ~/.config/opencode/opencode.json

# Rules
cat ~/.config/opencode/rules/general.md

# AGENTS.md
cat ~/.config/opencode/AGENTS.md

# Commands / Skills / Agents / Plugin
ls ~/.config/opencode/commands/
ls ~/.config/opencode/skills/
ls ~/.config/opencode/agents/
ls ~/.config/opencode/plugins/caveman/

# Global skills (opencode loads ~/.agents/skills/ natively)
ls ~/.agents/skills/

# ast-grep
ast-grep --version

# Service + plugins (v2: service owns sessions, plugins, permissions)
opencode service status
opencode reload
grep -E 'loading plugin|failed to load' ~/.local/share/opencode/log/opencode.log | tail -10

# context-mode live check (only if adopted; inside an opencode session)
# type: ctx stats
```
