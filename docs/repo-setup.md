# Repo Setup

Using this template for a new project.

## 1. Clone

```bash
git clone <template-url> <new-project>
cd <new-project>
git remote remove origin
git remote add origin <new-repo-url>
```

## 2. Git hooks

```bash
git config core.hooksPath .githooks
```

## 3. Session memory

```bash
# Init session
.agents/skills/session-memory/scripts/memory.sh context

# First recall
.agents/skills/session-memory/scripts/memory.sh recall "project" --limit 10
```

## 4. Stack decision

Edit `AGENTS.md`:

- `Stack: <your-stack>`
- `Deferred` items can be resolved (lint, typecheck, test scripts in `package.json`)

