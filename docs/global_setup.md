# Global Setup — OpenCode AI Environment

Guide for an AI on how to install and configure the global opencode environment on a new machine.

**OpenCode version: 2.x** (migrated from V1 on 2026-11-13 — see the migration section at the bottom).

## Prerequisites

- OpenCode CLI 2.x installed. On this machine: Nix profile (`nix profile install nixpkgs#opencode`) with the **flake:nixpkgs registry pointing at `nixpkgs-unstable`** — the stable channel still ships 1.x. Binary wrapper at `~/.local/bin/opencode` execs `~/.nix-profile/bin/opencode` (LD_LIBRARY_PATH shim for the Nix gcc lib).
- Node.js 18+

## 1. Caveman + Cavecrew

Three steps to install all caveman components:

```bash
# Step 1 — Plugin + commands + AGENTS.md
npx -y github:JuliusBrussee/caveman -- --force --only opencode

# Step 2 — Skills (caveman + cavecrew)
npx skills add JuliusBrussee/caveman -a opencode

# Step 3 — Cavecrew agents
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/agents/cavecrew-investigator.md -o ~/.config/opencode/agents/cavecrew-investigator.md
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/agents/cavecrew-builder.md -o ~/.config/opencode/agents/cavecrew-builder.md
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/agents/cavecrew-reviewer.md -o ~/.config/opencode/agents/cavecrew-reviewer.md
```

This installs:

| What | Path | Files |
|------|------|-------|
| Commands | `~/.config/opencode/commands/` | `caveman`, `caveman-commit`, `caveman-compress`, `caveman-help`, `caveman-review`, `caveman-stats` |
| Skills | `~/.config/opencode/skills/` | `cavecrew` |
| Agents | `~/.config/opencode/agents/` | `cavecrew-builder`, `cavecrew-investigator`, `cavecrew-reviewer` |
| Plugin | `~/.config/opencode/plugins/caveman/` | `caveman-config.cjs`, `caveman-parse.cjs`, `plugin.js` (V1 hooks), `server.js` (V2 entry), `index.js` (entrypoint shim) |

On Windows, commands go in `%APPDATA%\opencode\commands\`, everything else in `~/.config/opencode/`.

**Mode names (upstream 2026-11-12 redesign)**: the six legacy intensity levels are gone — three skills now: `caveman` (was lite/full), `ultracave` (was ultra), `megacave` (was wenyan-*). Legacy values map forward on read (`LEGACY_MODES` in `caveman-config.cjs`); the `.caveman-active` flag now stores `caveman`/`ultracave`/`megacave`/`off` + one-shot modes.

### OpenCode 2.x post-install fixes (applied 2026-11-13 — redo after a fresh caveman install)

1. **Plugin entrypoint**: V2 loads local directory plugins from an `index.js` entrypoint only (package.json `main` is ignored). Add:
   ```bash
   printf "// caveman plugin entry (V2 entrypoint convention: index.js)\nexport { default } from './server.js';\n" > ~/.config/opencode/plugins/caveman/index.js
   ```
2. **Config entry**: reference the plugin by absolute directory path in `opencode.json` (`{"package": "/home/skem/.config/opencode/plugins/caveman"}`) — relative paths in the global config are not resolved against the config dir.
3. **Agent frontmatter**: V2 ignores `name` in agent files (id = filename) — drop `name:` lines. Bare model aliases (`model: haiku`) are not valid V2 `provider/model` refs — drop them. `tools:` maps are legacy — replace with `permissions` rules. The upstream cavecrew agents are now V2-clean except the `name:` line.

Verify:

```bash
ls ~/.config/opencode/commands/
ls ~/.config/opencode/skills/
ls ~/.config/opencode/agents/
ls ~/.config/opencode/plugins/caveman/
```

## 2. opencode-rules Plugin

Configured in the global `opencode.json` under `plugins` (V2 plural, array form). Plugin auto-downloaded by Bun on first launch; auto-updates via `@latest`.

Current full config (`~/.config/opencode/opencode.json`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permissions": [
    { "action": "*", "resource": "*", "effect": "allow" },
    { "action": "external_directory", "resource": "*", "effect": "allow" }
  ],
  "providers": {
    "ollama": {
      "name": "Ollama",
      "package": "aisdk:@ai-sdk/openai-compatible",
      "settings": { "baseURL": "http://127.0.0.1:11434/v1" },
      "models": { "qwen3.5:9b": { "name": "qwen3.5:9b" } }
    }
  },
  "compaction": { "auto": true },
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
    { "package": "/home/skem/.config/opencode/plugins/caveman" }
  ]
}
```

(the `models` map above is trimmed; the real file lists all 7 ollama models)

## 3. Global Rule (`rules/general.md`)

`~/.config/opencode/rules/general.md`:

```markdown
- Caveman ON. Full.
- English ONLY.
- Context7 MCP for docs.
```

Injected into sessions by the `opencode-rules` plugin as `<rule>` system reminders.

## 4. Global AGENTS.md

`~/.config/opencode/AGENTS.md`:

```markdown
- Caveman ON. Full.
- English ONLY.
- Context7 MCP for docs.
```

## 5. Context7 MCP

Configured in `opencode.json` under `mcp.servers.context7` (V2: flat `mcp.<name>` moved under `mcp.servers`, `enabled` inverted to `disabled`). Requires a valid API key from <https://context7.com>. Consider `{env:CONTEXT7_API_KEY}` to keep the key out of the file.

## 6. Permission Configuration (V2 ordered rules)

V2 replaces the V1 grouped `permission` map with one ordered `permissions` array — last matching rule wins, actions renamed (`bash`→`shell`, `task`→`subagent`, `write`/`patch`→`edit`):

```json
{
  "permissions": [
    { "action": "*", "resource": "*", "effect": "allow" },
    { "action": "external_directory", "resource": "*", "effect": "allow" }
  ]
}
```

Note: V2 allows `external_directory` by default; add `{action: external_directory, effect: ask}` to restore the V1 prompt-before-external-access behavior (this machine keeps it `allow`).

## 7. Terminal client settings (`cli.json`)

V2 replaces layered `tui.json` files with one global `~/.config/opencode/cli.json` (auto-migrated on first V2 start; the old `tui.json` is left in place, harmless). Holds theme/diffs/session sidebar/animations + TUI-side plugins.

## Verify Complete Setup

```bash
cat ~/.config/opencode/opencode.json   # V2 config
cat ~/.config/opencode/cli.json        # V2 terminal settings
cat ~/.config/opencode/rules/general.md
cat ~/.config/opencode/AGENTS.md
ls ~/.config/opencode/commands/
ls ~/.config/opencode/skills/
ls ~/.config/opencode/agents/
ls ~/.config/opencode/plugins/caveman/
opencode debug agents                  # agents load without warnings
opencode mcp list                      # context7 connected
grep -i "loading plugin" ~/.local/share/opencode/log/opencode.log | tail
```

## OpenCode V1 → V2 Migration (2026-11-13)

What was migrated on this machine (backups: `opencode.json.v1bak`, `/tmp/opencode/migration/*`):

- **Binary**: Nix profile opencode 1.18.34 → 2.2.5. Source = global flake registry `flake:nixpkgs` → `nixpkgs-unstable` (26.11pre snapshot). The stable channel still ships 1.x. V1 used the `opencode-stable.db` database (nixpkgs' old `OPENCODE_CHANNEL=stable`); V2 uses `opencode.db`. The 4.9 GB V1 DB is kept untouched as archive.
- **Sessions**: V2 does NOT auto-import V1 sessions. Only the last V1 session was migrated on request (`ses_e43a8814affeJxOQxiq8z4deUa`, "Controllo attività in sospeso dalla memoria", + its 58 subagent children, 1421 messages). Method: V1 export (`XDG_DATA_HOME=<isolated copy of the V1 data dir> opencode-1.18.34 export <id>`) → Python converter to the V2 `SessionTransfer` shape (V2 session info requires `location:{directory}`; assistant messages carry inline `content` arrays instead of separate parts; tool parts need `time.created`) → POST `/api/experimental/session/import` with request-level `location` + Basic auth from `~/.config/opencode/service.json`. Import parents before children.
- **Config**: V1 `permission`/`agent`/`plugin`/`provider`/`mcp` maps → V2 `permissions` array / `agents` / `plugins` / `providers` (aisdk: package prefix, `settings.baseURL`) / `mcp.servers`.
- **Plugins**: caveman + agents-opencode ported to the V2 plugin API (`Plugin.define({id, setup})`, `ctx.session.hook("prompt"/"context"/"compaction")`, `ctx.tool.hook("execute.before")`, `ctx.shell.hook("create.before")`). V1 plugin implementations do NOT run on V2. Local plugin dirs need an `index.js` entrypoint; configured paths must be directories (absolute path for the global one).
- **Agents/commands frontmatter**: `permission` maps → `permissions` ordered lists; `temperature` dropped (no V2 effect); `subtask` → `subagent` in command files; legacy `tools` maps → permissions rules.
- **Terminal settings**: `tui.json` → `cli.json` (auto-migrated by V2 on first start).
- **Ecosystem update (same day)**: agents-opencode pack 2.1.1 → 4.3.5 (`nix shell nixpkgs#nodejs_24 --command npx agents-opencode@4.3.5 --update --all --target auto` — pack needs Node ≥24). The installer re-ships V1 frontmatter in its agent/command files on every update → re-apply the V2 frontmatter conversion after each update. Its plugin (`plugins/agents-opencode.ts` + `lib/guards.mjs`) is dual-entrypoint and loads natively on V2 (loose `.ts` files in `plugins/` are discovered). caveman upstream redesigned its modes (caveman/ultracave/megacave) and ships `server.js` as its V2 entry. opencode-rules 2.0.0 verified V2-native (`ctx.tool.hook` + `ctx.session.hook` + `ctx.event.subscribe`).
- **Stale-copy cleanup**: duplicate pre-redesign caveman skills in `~/.agents/skills/` (source of "duplicate skill name" warnings) removed; `~/.config/opencode/skills/` is authoritative. Orphaned global leftovers removed: `node_modules`, `package-lock.json`, `tui.json`, `settings.json`, empty `command/` + `hooks/` dirs.
- **CLI binary**: upgraded 2.2.5 → 2.2.6 via `nix profile upgrade opencode` (unstable snapshot). The background service picks up the new binary at its next start (`opencode service restart`, or just after closing all opencode instances).
