# Agents — Quick Reference

## How to start a task

In opencode CLI press `Tab` and select an agent. These are the 6 you can pick:

| Agent | Mode | What it does |
|-------|------|-------------|
| `plan` | read-only | Plan and analyze. Standard opencode. |
| `build` | read+write | Implement code. Standard opencode. |
| `codebase` | read+write | Like `build` but auto-detects project stack and loads matching skills. |
| `planner` | read-only | Like `plan` but produces detailed breakdowns with deps, risks, files. |
| `legal-advisor` | read+report | Legal research, license compliance, GDPR. |
| `researcher` | read-only | Research APIs/libs via Context7 docs, cross-refs ≥2 sources. |

---

## Daily workflow

```text
Small task:    select build or @codebase
Need analysis: select plan or @planner first
Legal check:   select @legal-advisor
Everything else: @name when really needed
```

### Example

```text
1. Tab → select @planner
   "plan a checkout feature"

2. Tab → select @codebase
   "implement step 1 from the plan"

3. Tab → select build
   "fix the button alignment"
```

---

## All agents (invocable with `@name`)

| Agent | What it does |
|-------|-------------|
| `@codebase` | Multi-stack dev. Auto-detects .NET, Python, TS, Flutter, Go, Java, Node, React, Ruby, Rust. Presents plan, awaits approval, implements step-by-step with validation. |
| `@planner` | Read-only. Produces detailed task breakdown: steps, dependencies, risks, files. |
| `@legal-advisor` | Legal research + compliance. IRAC framework. Saves report to file. Never modifies code. |
| `@researcher` | Researches APIs/libs via Context7 docs, cross-refs ≥2 sources. |
| `@review` | PR/quality review. Delegates technical review to `@code-reviewer`. |
| `@code-reviewer` | Code correctness, readability, architecture review. Read-only. |
| `@debugger` | Root-cause debugging. Reproduce → hypothesize → isolate → minimal fix → regression test. |
| `@docs` | Writes README, API docs, ADRs, guides. |
| `@orchestrator` | Complex multi-phase projects. Plans then delegates to specialist agents. |
| `@test-automator` | Implements test suites, coverage, CI test integration. |
| `@performance-engineer` | Profiling, bottleneck analysis, latency/throughput optimization. |
| `@refactoring-specialist` | Safe cross-module refactoring. Preserves behavior. |
| `@security-auditor` | Security assessment, threat modeling, compliance review. |
| `@penetration-tester` | Simulated attacks, API security testing. |
| `@documentation-engineer` | Doc infrastructure: SSG, search, versioning, CI/CD for docs. |
| `@blogger` | Blog posts, podcast ideas, YouTube scripts. |
| `@em-advisor` | Engineering management advice: 1-on-1s, performance reviews, team dynamics. |
| `@information-scan` | Pre-commit secrets/PII scanner. Blocks commit if found. Also runs via git hook. |
| `@vulnerability-scan` | Tool-based vulnerability scanning (Docker, CI, app code). |
| `@brutal-critic` | Content review. Called by `@blogger`. Hidden from agent list. |

---

## Internal tools

Used by other agents internally. You don't call them.

| Tool | Called by | What it does |
|------|-----------|-------------|
| `@explore` | `@orchestrator`, `@codebase`, others | Fast file/content search. Read-only. |
| `@general` | `@orchestrator` | Generic multi-step research tasks. |
