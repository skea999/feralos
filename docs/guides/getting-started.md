# Getting Started

## Prerequisites

- [opencode](https://opencode.ai) CLI
- Node.js 18+
- Git

## Setup

```bash
# 1. Load session memory
.agents/skills/session-memory/scripts/memory.sh context

# 2. Install the stack you chose (see AGENTS.md#Stack)
#    and update AGENTS.md#Stack accordingly

# 3. Install dependencies
# npm install   # or bun install / yarn

# 4. Start coding
# Use the workflow in AGENTS.md — Define → Plan → Code → Test → Review → Commit → Ship
```

## Workflow

See `AGENTS.md` for the complete agent-skills workflow.

## Session Memory

**MANDATORY** every session:

```bash
# Start
.agents/skills/session-memory/scripts/memory.sh context

# During work
.agents/skills/session-memory/scripts/memory.sh save "decision" "..." "tag1" "tag2"

# End
.agents/skills/session-memory/scripts/memory.sh save "summary" "Today: X, next: Z"
```
