// agents-opencode — opencode plugin (V2)
//
// Ported from the V1 hook API to the OpenCode V2 plugin API.
//
// V1 -> V2 mapping:
//   client.app.log                        -> console.log at setup
//   experimental.session.compacting       -> session.hook("compaction")
//   tool.execute.before                   -> tool.hook("execute.before")
//   shell.env                             -> shell.hook("create.before")

import { Plugin } from '@opencode/plugin';

const PACKAGE_VERSION = '2.1.1';

const COMPACTION_CONTEXT = `## Agents Opencode Context

You are operating with the agents-opencode v${PACKAGE_VERSION} agent pack.

Available agents (invoke via @mention):
- @codebase — Multi-language development with profile detection
- @orchestrator — Strategic planning and complex workflow coordination
- @planner — Read-only analysis and implementation planning
- @review — Code review for security, performance, and best practices
- @docs — Documentation creation and maintenance
- @blogger — Content creation for blogging, podcasting, YouTube
- @brutal-critic — Content quality review with framework-based scoring
- @em-advisor — Engineering management guidance
- @legal-advisor — License auditing, compliance, and regulatory guidance

Active skills: 19 language/domain/utility skill packs loadable via skill tool.
Active commands: 14 slash commands (type / to see autocomplete).

Memory: state/session_state.json and handoff/latest.md preserve working state.
Context persistence: AGENTS.md tracks project milestones across sessions.

## Session State Reminder
- Current phase: Read state/session_state.json for working memory
- Check handoff/latest.md for continuation context
- Review AGENTS.md for recent milestones and patterns`;

const BLOCKED_BASENAMES = [
  '.env',
  'credentials.json',
  'secrets.yaml',
  'id_rsa',
  'id_ed25519',
  '.pem',
];

export default Plugin.define({
  id: 'agents-opencode',
  async setup(ctx) {
    console.log(
      `[agents-opencode] v${PACKAGE_VERSION} loaded — 9 agents, 19 skills, 14 commands available`
    );

    // Inject agent-specific state into compaction so critical context
    // survives context window truncation. The compaction hook receives the
    // transcript being summarized; appending a message adds it to the
    // summary input (OpenCode appends its own summary prompt after hooks).
    await ctx.session.hook('compaction', (event) => {
      if (Array.isArray(event.messages)) {
        event.messages.push({
          role: 'user',
          content: [{ type: 'text', text: COMPACTION_CONTEXT }],
        });
      }
    });

    // Safety hook: block agents from reading sensitive files.
    await ctx.tool.hook('execute.before', (event) => {
      if (event.tool !== 'read') return;
      const filePath = event.input && event.input.filePath;
      if (typeof filePath !== 'string') return;
      const basename = filePath.split(/[/\\]/).pop().toLowerCase() || '';
      for (const pattern of BLOCKED_BASENAMES) {
        if (basename === pattern || basename.endsWith(pattern)) {
          throw new Error(
            `[agents-opencode] Blocked reading sensitive file: ${filePath}. Do not read credential or secret files.`
          );
        }
      }
    });

    // Inject package version into shell environment for script awareness.
    await ctx.shell.hook('create.before', (event) => {
      event.env.AGENTS_OPENCODE_VERSION = PACKAGE_VERSION;
    });
  },
});
