import { readFile, readdir } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

// The PreToolUse fence that makes the read grammar a BOUNDARY rather than an observation. Derived from
// this module's own location — never hardcoded, so moving the engine cannot silently unwire it.
export const CODE_READ_FENCE_SCRIPT = fileURLToPath(new URL('./code-read-fence.mjs', import.meta.url));

export const CODE_READ_BUILTIN_TOOLS = Object.freeze([
  'Workflow',
  'Bash',
  'Read',
  'Glob',
  'Grep',
]);

export const CODE_READ_AGENT_TOOLS = Object.freeze([
  'Bash',
  'Read',
  'Glob',
  'Grep',
  'StructuredOutput',
]);

export const CODE_READ_ALLOWED_TOOLS = Object.freeze([
  'Workflow',
  'Read',
  'Glob',
  'Grep',
  'Bash(git diff *)',
  'Bash(git show *)',
  'Bash(git rev-parse *)',
  'Bash(git log *)',
  'Bash(git cat-file *)',
  'Bash(git ls-files *)',
  'Bash(rg *)',
]);

export const CODE_READ_DENIED_TOOLS = Object.freeze([
  'mcp__*',
  'Write',
  'Edit',
  'NotebookEdit',
]);

// Wiring for the pre-execution fence, passed inline so there is no settings FILE to drift out of sync
// with this module and no path to hardcode. `--settings` accepts a JSON string as well as a path.
export function codeReadFenceSettings() {
  return {
    hooks: {
      PreToolUse: [
        {
          matcher: 'Bash',
          hooks: [{ type: 'command', command: CODE_READ_FENCE_SCRIPT }],
        },
      ],
    },
  };
}

export function claudeCodeReadOnlyArguments() {
  return [
    '--tools',
    CODE_READ_BUILTIN_TOOLS.join(','),
    '--allowedTools',
    CODE_READ_ALLOWED_TOOLS.join(','),
    '--disallowedTools',
    CODE_READ_DENIED_TOOLS.join(','),
    '--permission-mode',
    'dontAsk',
    '--strict-mcp-config',
    '--no-chrome',
    '--disable-slash-commands',
    '--settings',
    JSON.stringify(codeReadFenceSettings()),
  ];
}

// Raw Bash IS granted — the walk needs git and rg reads, and there is no narrower builtin that provides
// them. What makes that safe is not the allow-list (which only PRE-APPROVES; it never removes a tool)
// but the PreToolUse fence, which runs BEFORE the command and denies anything outside the grammar. So
// this assertion verifies the FENCE, not the tool list: the script must exist on disk and the argv must
// actually wire it to Bash. A fence that is merely intended is the failure this whole check exists for.
export function assertFaithfulCodeReadPolicy() {
  if (!existsSync(CODE_READ_FENCE_SCRIPT)) {
    throw new Error(
      `claude_bash_not_pre_execution_fenced: the fence script is missing at ${CODE_READ_FENCE_SCRIPT}; raw Bash would run unchecked.`,
    );
  }
  const args = claudeCodeReadOnlyArguments();
  const index = args.indexOf('--settings');
  if (index < 0 || index + 1 >= args.length) {
    throw new Error('claude_bash_not_pre_execution_fenced: no --settings payload wires the pre-tool fence.');
  }
  let settings;
  try {
    settings = JSON.parse(args[index + 1]);
  } catch (caught) {
    throw new Error('claude_bash_not_pre_execution_fenced: the --settings payload is not valid JSON.', {
      cause: caught,
    });
  }
  const wired = (settings?.hooks?.PreToolUse ?? []).some(
    (entry) =>
      entry?.matcher === 'Bash' &&
      (entry?.hooks ?? []).some(
        (hook) => hook?.type === 'command' && hook?.command === CODE_READ_FENCE_SCRIPT,
      ),
  );
  if (!wired) {
    throw new Error(
      'claude_bash_not_pre_execution_fenced: the --settings payload does not bind the fence script to PreToolUse(Bash).',
    );
  }
}

function increment(counts, name) {
  counts[name] = (counts[name] ?? 0) + 1;
}

function toolUses(rows) {
  const uses = [];
  for (const row of rows) {
    const content = row?.message?.content;
    if (!Array.isArray(content)) continue;
    for (const block of content) {
      if (block?.type === 'tool_use' && typeof block.name === 'string')
        uses.push({ name: block.name, input: block.input });
    }
  }
  return uses;
}

// Evidence classifier only. This describes the narrow command grammar observed
// after execution; it is not a pre-execution security boundary.
export function matchesDeclaredReadGrammar(command) {
  if (typeof command !== 'string' || command.length === 0) return false;
  if (/[\n\r;&|><`$()\\'"]/u.test(command)) return false;
  if (
    /(?:^|\s)--(?:output|ext-diff|textconv|filters|pre|pre-glob)(?:=|\s|$)/u.test(command)
  )
    return false;
  return [
    /^git diff(?:\s|$)/u,
    /^git show(?:\s|$)/u,
    /^git rev-parse(?:\s|$)/u,
    /^git log(?:\s|$)/u,
    /^git cat-file(?:\s|$)/u,
    /^git ls-files(?:\s|$)/u,
    /^rg(?:\s|$)/u,
  ].some((pattern) => pattern.test(command));
}

async function jsonLines(path) {
  const source = await readFile(path, 'utf8');
  return source
    .split('\n')
    .filter(Boolean)
    .map((line, index) => {
      try {
        return JSON.parse(line);
      } catch (caught) {
        throw new Error(`${path}: row ${index + 1} is not JSON`, { cause: caught });
      }
    });
}

export async function inspectWorkflowRun(runDirectory) {
  const journal = await jsonLines(join(runDirectory, 'journal.jsonl'));
  const started = new Set();
  const returned = new Set();
  for (const row of journal) {
    if (typeof row?.agentId !== 'string') continue;
    if (row.type === 'started') started.add(row.agentId);
    if (row.type === 'result') returned.add(row.agentId);
  }

  const names = (await readdir(runDirectory))
    .filter((name) => /^agent-[a-z0-9]+\.jsonl$/u.test(name))
    .sort();
  const toolCounts = {};
  const perAgent = [];
  const bashCommands = [];
  for (const name of names) {
    const agentId = name.slice('agent-'.length, -'.jsonl'.length);
    const rows = await jsonLines(join(runDirectory, name));
    const uses = toolUses(rows);
    const toolNames = uses.map((use) => use.name);
    for (const tool of toolNames) increment(toolCounts, tool);
    const agentBashCommands = uses
      .filter((use) => use.name === 'Bash')
      .map((use) => {
        const command = typeof use.input?.command === 'string' ? use.input.command : null;
        const record = {
          agentId,
          command,
          declaredReadGrammar: matchesDeclaredReadGrammar(command),
        };
        bashCommands.push(record);
        return record;
      });
    perAgent.push({ agentId, returned: returned.has(agentId), toolCounts: toolNames.reduce((counts, tool) => {
      increment(counts, tool);
      return counts;
    }, {}), bashCommands: agentBashCommands });
  }

  const incompleteAgentIds = [...started].filter((agentId) => !returned.has(agentId)).sort();
  const outsideDeclaredReadGrammar = bashCommands.filter(
    (record) => record.declaredReadGrammar !== true,
  );
  return {
    started: started.size,
    returned: returned.size,
    incomplete: incompleteAgentIds.length,
    incompleteAgentIds,
    toolCounts,
    bashCommands,
    outsideDeclaredReadGrammar,
    perAgent,
  };
}

export function assertCodeReadObservation(observation, { requireEveryMechanic = true } = {}) {
  if (observation.started < 1) throw new Error('A1 expected at least one generated-bundle agent');
  const connectorTools = Object.keys(observation.toolCounts).filter((name) => name.startsWith('mcp__'));
  if (connectorTools.length > 0)
    throw new Error(`A1 connector set is not empty: ${connectorTools.join(', ')}`);
  const unexpectedTools = Object.keys(observation.toolCounts).filter(
    (name) => !CODE_READ_AGENT_TOOLS.includes(name),
  );
  if (unexpectedTools.length > 0)
    throw new Error(`A1 unexpected agent tools: ${unexpectedTools.join(', ')}`);
  if (!Array.isArray(observation.bashCommands))
    throw new Error('A1 Bash command payloads were not inspected');
  const outsideDeclaredReadGrammar = observation.bashCommands.filter(
    (record) => !matchesDeclaredReadGrammar(record?.command),
  );
  if (outsideDeclaredReadGrammar.length > 0)
    throw new Error(
      `A1 observed ${outsideDeclaredReadGrammar.length} Bash command(s) outside the declared read grammar`,
    );
  if (requireEveryMechanic) {
    for (const name of ['Read', 'Grep', 'Glob', 'Bash', 'StructuredOutput']) {
      if ((observation.toolCounts[name] ?? 0) < 1)
        throw new Error(`A1 required mechanic was not exercised: ${name}`);
    }
  }
  return observation;
}

export function incompleteAgentResult({ started, returned, reason }) {
  const incomplete = Math.max(0, started - returned);
  return {
    status: 'ERROR',
    error: {
      code: 'phase2_incomplete_agents',
      message: 'Wave Walker did not receive terminal results from every started agent.',
      reason,
    },
    agents: { started, returned, incomplete },
  };
}

function resultDiscriminator(value) {
  if (value === null) return 'null';
  if (Array.isArray(value)) return 'array';
  if (typeof value !== 'object') return typeof value;
  const status = typeof value.status === 'string' ? value.status : '<missing>';
  return `object:status=${status}`;
}

export function parseGeneratedBundleTerminal(output) {
  const raw = output?.result;
  if (raw?.status === 'DONE') return raw;
  if (raw?.status === 'ok' && raw.value?.status === 'DONE') return raw.value;
  return {
    status: 'ERROR',
    error: {
      code: 'generated_bundle_result_unparseable',
      message: 'Generated Workflow output did not carry a recognized terminal result.',
      discriminator: resultDiscriminator(raw),
    },
  };
}

export async function runGeneratedBundleWithWindow({
  executeBundle,
  bundle,
  args,
  globals,
  windowMs,
}) {
  if (!Number.isInteger(windowMs) || windowMs < 1)
    throw new Error('runtime window must be a positive integer');
  let started = 0;
  let returned = 0;
  const trackedGlobals = {
    ...globals,
    async agent(prompt, options) {
      started += 1;
      const value = await globals.agent(prompt, options);
      returned += 1;
      return value;
    },
  };
  let timer;
  const timeout = new Promise((resolve) => {
    timer = setTimeout(
      () => resolve(incompleteAgentResult({ started, returned, reason: 'runtime_window_exhausted' })),
      windowMs,
    );
  });
  try {
    return await Promise.race([executeBundle(bundle, args, trackedGlobals), timeout]);
  } finally {
    clearTimeout(timer);
  }
}
