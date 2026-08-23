# Global Setup — OpenCode AI Environment

Guide for an AI on how to install and configure the global opencode environment on a new machine.

## Prerequisites

- OpenCode CLI installed (`winget install OpenCode` on Windows, `nix shell nixpkgs#opencode` on Linux, or from opencode.ai)
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
| Plugin | `~/.config/opencode/plugins/caveman/` | `caveman-config.cjs`, `plugin.js` |

On Windows, commands go in `%APPDATA%\opencode\commands\`, everything else in `~/.config/opencode/`.

Verify:

```bash
ls ~/.config/opencode/commands/
ls ~/.config/opencode/skills/
ls ~/.config/opencode/agents/
ls ~/.config/opencode/plugins/caveman/
```

### Caveat: opencode ≥1.15 — agent `tools` frontmatter format

opencode v1.15.13 requires `tools` in agent YAML frontmatter to be an **object** (`tool-name: true`), not an **array** (`[Tool1, Tool2]`).

The upstream cavecrew agent files still use the array format, which causes `ConfigInvalidError` on startup:

```
Error: Configuration is invalid at ~/.config/opencode/agents/cavecrew-reviewer.md
↳ Expected object | undefined, got ["Read","Grep","Bash"] tools
```

**Fix** — convert to object for all 3 agent files:

```bash
sed -i 's/tools: \[Read, Grep, Bash\]/tools:\n  read: true\n  grep: true\n  bash: true/' ~/.config/opencode/agents/cavecrew-reviewer.md

sed -i 's/tools: \[Read, Grep, Glob, Bash\]/tools:\n  read: true\n  grep: true\n  glob: true\n  bash: true/' ~/.config/opencode/agents/cavecrew-investigator.md

sed -i 's/tools: \[Read, Edit, Write, Grep, Glob\]/tools:\n  read: true\n  edit: true\n  write: true\n  grep: true\n  glob: true/' ~/.config/opencode/agents/cavecrew-builder.md
```

Verify with:

```bash
opencode debug config
```

### Caveat: agents visible in tab mode

Cavecrew agents are subagents (invoked via `task` tool), not primary agents. Without `hidden: true`, they appear in Tab switching. Add to frontmatter of all 3 agent files:

```bash
for f in ~/.config/opencode/agents/cavecrew-*.md; do
  if ! grep -q '^hidden:' "$f"; then
    sed -i '/^---$/a hidden: true' "$f"
  fi
done
```

Or verify each file has `hidden: true` after `model:` or `tools:` block.

## 2. opencode-rules Plugin

Add to global `opencode.json` (`~/.config/opencode/opencode.json`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": ["opencode-rules@latest"]
}
```

Plugin auto-downloaded by Bun on first launch. Auto-updates via `@latest` tag.

Current full config:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "opencode-rules@latest"
  ],
  "compaction": {
    "auto": true
  },
  "mcp": {
    "context7": {
      "type": "remote",
      "url": "https://mcp.context7.com/mcp",
      "enabled": true,
      "headers": {
        "CONTEXT7_API_KEY": "<api-key>"
      }
    }
  }
}
```

## 3. Global Rule (`rules/general.md`)

`~/.config/opencode/rules/general.md`:

```markdown
- Caveman ON. Full.
- English ONLY.
- Context7 MCP for docs.
```

## 4. Global AGENTS.md

`~/.config/opencode/AGENTS.md`:

```markdown
- Caveman ON. Full.
- English ONLY.
- Context7 MCP for docs.
```

## 5. Context7 MCP

Configured in `opencode.json` (`mcp.context7` section). Requires a valid API key from https://context7.com.

## 6. Permission Configuration

The global `opencode.json` has no explicit `permission` section. Permissions are set per-project in `opencode.json`:

```json
{
  "permission": {
    "external_directory": "deny",
    "doom_loop": "deny"
  }
}
```

Currently active merged rules (`external_directory` is denied by default, but specific paths are allowlisted for skills, tools, etc.):

- `"*" "allow" "*"` — all operations allowed on project files
- `"external_directory" "deny" "*"` — external directories blocked by default
- Allowlisted: `~/.config/opencode/skills/cavecrew/`, `~/.agents/skills/*/`, project `.agents/skills/*/`, project `.opencode/skills/*/`, tool output, temp dirs

See actual effective rules at runtime by inspecting the permission blocks in agent responses.

## Verify Complete Setup

```bash
# Config
cat ~/.config/opencode/opencode.json

# Rules
cat ~/.config/opencode/rules/general.md

# AGENTS.md
cat ~/.config/opencode/AGENTS.md

# Commands
ls ~/.config/opencode/commands/

# Skills
ls ~/.config/opencode/skills/

# Agents
ls ~/.config/opencode/agents/

# Plugin
ls ~/.config/opencode/plugins/caveman/
```
