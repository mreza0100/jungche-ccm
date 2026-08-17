import { readFile, readdir } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

// The PreToolUse fence that makes the read grammar a BOUNDARY rather than an observation. Derived from
// this module's own location — never hardcoded, so moving the engine cannot silently unwire it.
export const CODE_READ_FENCE_SCRIPT = fileURLToPath(new URL('./code-read-fence.mjs', import.meta.url));

// Where the harness writes transcripts and workflow run directories. NOT ~/.claude unconditionally: a
// chat running under another account sets CLAUDE_CONFIG_DIR, and a caller that assumes the default root
// looks for evidence in a directory the run never wrote to — then reports the run as unbindable rather
// than reporting its own wrong assumption. One home, so every instrument agrees on where to look.
export function claudeConfigRoot() {
  const configured = process.env.CLAUDE_CONFIG_DIR;
  return configured !== undefined && configured !== '' ? configured : join(homedir(), '.claude');
}

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
        uses.push({ id: block.id ?? null, name: block.name, input: block.input });
    }
  }
  return uses;
}

// Which tool calls the pre-execution fence REFUSED. An attempt outside the grammar is not a breach —
// the fence denies before the command runs, so a denial is the control WORKING. Without this the
// evidence cannot tell a blocked command from an executed one, and every attempt reads as a breach.
function deniedToolUseIds(rows) {
  const denied = new Set();
  for (const row of rows) {
    const content = row?.message?.content;
    if (!Array.isArray(content)) continue;
    for (const block of content) {
      if (block?.type !== 'tool_result') continue;
      const text =
        typeof block.content === 'string'
          ? block.content
          : Array.isArray(block.content)
            ? block.content.map((part) => (typeof part?.text === 'string' ? part.text : '')).join('')
            : '';
      if (text.includes(CODE_READ_FENCE_DENIAL_MARKER) && typeof block.tool_use_id === 'string')
        denied.add(block.tool_use_id);
    }
  }
  return denied;
}

// Only READERS. A command absent from this list cannot run at all, so every write vector (tee, cp, mv,
// rm, dd, truncate, install, sed -i) is excluded by OMISSION rather than by blacklist — a blacklist is
// only ever as good as the last thing somebody remembered to add to it.
const READ_COMMANDS = [
  /^git (?:-C \S+ )?(?:diff|show|rev-parse|log|cat-file|ls-files|ls-tree|branch|merge-base|status|blame|describe|for-each-ref|shortlog)(?:\s|$)/u,
  /^rg(?:\s|$)/u,
  /^grep(?:\s|$)/u,
  /^wc(?:\s|$)/u,
  /^cat(?:\s|$)/u,
  /^head(?:\s|$)/u,
  /^tail(?:\s|$)/u,
  /^ls(?:\s|$)/u,
  /^date(?:\s|$)/u,
];

// The only pipe sinks that can be permitted: bounded readers that cannot create a file. `tee`, `sh`,
// and every writer are absent, so a pipe cannot become a write.
const BOUNDED_PIPE_SINK = /^\s*(?:head|tail|wc|grep|rg)(?:\s+[^\s]+)*\s*$/u;

// Every fence refusal message begins with this. It is the ONE string that distinguishes a command the
// fence blocked from a command that ran.
export const CODE_READ_FENCE_DENIAL_MARKER = 'code_read_fence:';

// Evidence classifier only. This describes the command grammar observed after execution; the
// pre-execution security boundary is code-read-fence.mjs, which imports THIS function — one grammar,
// one home.
// A metacharacter is only dangerous UNQUOTED. `grep "a|b;c" file` is one bounded read, not a pipeline —
// judging the raw string treats regex syntax as a shell attack and starves the walk. So scan with quote
// state, split only on unquoted pipes, and fail closed on an unterminated quote (we cannot reason about
// a string whose quoting does not resolve).
function scanUnquoted(command) {
  let quote = null;
  const segments = [''];
  const metacharacters = new Set();
  // The DEQUOTED form is what the shell will actually execute: `--p're'` is `--pre`, and a flag check
  // against the raw string would never see it. Quote-splitting a forbidden flag is the evasion the old
  // blanket quote ban hid rather than caught.
  let dequoted = '';
  for (const character of command) {
    if (quote !== null) {
      if (character === quote) {
        quote = null;
      } else {
        dequoted += character;
      }
      segments[segments.length - 1] += character;
      continue;
    }
    if (character === "'" || character === '"') {
      quote = character;
      segments[segments.length - 1] += character;
      continue;
    }
    if (character === '|') {
      segments.push('');
      dequoted += character;
      continue;
    }
    if ('\n\r;&<>`$'.includes(character)) metacharacters.add(character);
    segments[segments.length - 1] += character;
    dequoted += character;
  }
  return { unterminatedQuote: quote !== null, metacharacters, segments, dequoted };
}

const FORBIDDEN_FLAGS = /(?:^|\s)--(?:output|ext-diff|textconv|filters|pre|pre-glob)(?:=|\s|$)/u;

// Command allow-listing alone is NOT a boundary: `cat`, `wc`, `head`, and `grep` read any file the
// process can reach, so an allow-list without a path constraint hands over /etc/passwd, ~/.ssh, and
// every .env on the box. The walk audits ONE repository; reads leave it over this line and no further.
// The root is an EXPLICIT parameter, never process.cwd(). The fence runs inside the audited repository
// while the evidence classifier runs inside the caller's own project, so an ambient cwd gave the same
// rule two different answers: the fence correctly ALLOWED a repo read that the classifier then reported
// as an escape. One rule with an implicit parameter is still one rule with two homes.
// Unset root => we cannot judge an absolute path at all, so any absolute path is refused. Fail closed.
function readsStayInsideRoot(command) {
  const root = process.env.WAVE_WALKER_READ_ROOT ?? null;
  if (/(?:^|[\s'"=:])\.\.(?:\/|$)/u.test(command)) return false;
  const absolutePaths = command.match(/(?:^|[\s'"=:])(\/[^\s'"]*)/gu) ?? [];
  if (root === null) return absolutePaths.length === 0;
  return absolutePaths.every((raw) => {
    const path = raw.replace(/^[\s'"=:]+/u, '');
    return path === root || path.startsWith(`${root}/`);
  });
}

export function matchesDeclaredReadGrammar(command) {
  if (typeof command !== 'string' || command.length === 0) return false;
  if (FORBIDDEN_FLAGS.test(command)) return false;
  if (!readsStayInsideRoot(command)) return false;
  // The two stderr redirections that cannot create a real file: /dev/null discards, and &1 merges into
  // an existing stream. Named literally and stripped before the scan — every OTHER redirection stays a
  // write vector and stays denied.
  const normalized = command
    .replace(/\s2>\/dev\/null(?=\s|$)/gu, '')
    .replace(/\s2>&1(?=\s|$)/gu, '');
  const { unterminatedQuote, metacharacters, segments, dequoted } = scanUnquoted(normalized);
  // Re-check the forbidden flags against what the shell will really run, after quote removal.
  if (FORBIDDEN_FLAGS.test(dequoted)) return false;
  // Redirection creates files; `;`/`&` chain a second command past this check; `$`/backtick substitute
  // arbitrary execution. Those are the write/egress/execution vectors, and only those.
  if (unterminatedQuote || metacharacters.size > 0) return false;
  if (segments.length > 2) return false;
  if (segments.length === 2 && !BOUNDED_PIPE_SINK.test(segments[1])) return false;
  return READ_COMMANDS.some((pattern) => pattern.test(segments[0].trim()));
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
    const denied = deniedToolUseIds(rows);
    const agentBashCommands = uses
      .filter((use) => use.name === 'Bash')
      .map((use) => {
        const command = typeof use.input?.command === 'string' ? use.input.command : null;
        const inGrammar = matchesDeclaredReadGrammar(command);
        const record = {
          agentId,
          command,
          declaredReadGrammar: inGrammar,
          // An out-of-grammar command that the fence DENIED never executed. Recording this is what lets
          // a later reader tell a working control from a breach — the two are opposite verdicts and the
          // old schema rendered them identically.
          fenceDenied: use.id !== null ? denied.has(use.id) : null,
          executed: inGrammar || (use.id !== null ? !denied.has(use.id) : null),
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
  // The failure is a command that ESCAPED, never a command that was refused. A denied attempt is the
  // fence doing its job; treating it as a breach is how a working control gets reported as a violation.
  // `fenceDenied === null` means the journal could not be paired to a verdict — unknown, so fail closed.
  const outsideDeclaredReadGrammar = observation.bashCommands.filter(
    (record) => !matchesDeclaredReadGrammar(record?.command),
  );
  const escaped = outsideDeclaredReadGrammar.filter((record) => record?.fenceDenied !== true);
  if (escaped.length > 0)
    throw new Error(
      `A1 observed ${escaped.length} Bash command(s) that escaped the declared read grammar ` +
        `(${outsideDeclaredReadGrammar.length - escaped.length} further attempt(s) were denied by the fence)`,
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
