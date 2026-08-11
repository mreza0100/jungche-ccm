// @ts-nocheck — production-caller.js is the Node-only headless seam around the generated bundle.
import { spawnSync } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import { executeHarnessBundle } from 'cross-workflow';
import {
  CODE_READ_AGENT_TOOLS,
  CODE_READ_ALLOWED_TOOLS,
  CODE_READ_BUILTIN_TOOLS,
  CODE_READ_DENIED_TOOLS,
  CODE_READ_FENCE_SCRIPT,
  assertCodeReadObservation,
  assertFaithfulCodeReadPolicy,
  claudeCodeReadOnlyArguments,
  matchesDeclaredReadGrammar,
  parseGeneratedBundleTerminal,
  runGeneratedBundleWithWindow,
} from '../production-caller.js';

function globals(agent) {
  return {
    agent,
    parallel(thunks) {
      return Promise.all(thunks.map((thunk) => thunk()));
    },
    async pipeline(steps, initial) {
      let value = initial;
      for (const step of steps) value = await step(value);
      return value;
    },
    log() {},
    phase() {},
    workflow: {},
    budget: { total: null, spent: () => 0, remaining: () => Number.MAX_SAFE_INTEGER },
  };
}

function optionValue(args, name) {
  const index = args.indexOf(name);
  expect(index).toBeGreaterThanOrEqual(0);
  return args[index + 1];
}

describe('production caller seams over the real generated Claude bundle', () => {
  it('A1 records the caller policy and rejects connectors or Bash outside the declared grammar', async () => {
    const bundle = await readFile(resolve('dist/cross-workflow/claude/workflow.js'), 'utf8');
    const cliArgs = claudeCodeReadOnlyArguments();
    expect(optionValue(cliArgs, '--tools')).toBe(CODE_READ_BUILTIN_TOOLS.join(','));
    expect(optionValue(cliArgs, '--allowedTools')).toBe(CODE_READ_ALLOWED_TOOLS.join(','));
    expect(optionValue(cliArgs, '--disallowedTools')).toBe(CODE_READ_DENIED_TOOLS.join(','));
    expect(optionValue(cliArgs, '--permission-mode')).toBe('dontAsk');
    expect(cliArgs).toContain('--strict-mcp-config');
    expect(CODE_READ_DENIED_TOOLS).toContain('mcp__*');
    // The fence is now WIRED, so the policy assertion passes — and it passes because the argv actually
    // binds the pre-execution hook to Bash, not because the check was relaxed.
    expect(() => assertFaithfulCodeReadPolicy()).not.toThrow();
    const settings = JSON.parse(optionValue(cliArgs, '--settings'));
    expect(settings.hooks.PreToolUse[0].matcher).toBe('Bash');
    expect(settings.hooks.PreToolUse[0].hooks[0].command).toBe(CODE_READ_FENCE_SCRIPT);

    let calls = 0;
    const result = await executeHarnessBundle(
      bundle,
      {
        claims: [
          {
            id: 'a1-tool-policy',
            statement: 'The generated bundle reaches its verifier agent.',
            files: ['AGENTS.md'],
          },
        ],
        votes: 1,
        project: { repoRoot: '.' },
      },
      globals(async (_prompt, options) => {
        calls += 1;
        expect(options.label).toMatch(/^verify /u);
        return {
          claimId: 'a1-tool-policy',
          verdict: 'CONFIRMED',
          evidence: [{ anchor: 'AGENTS.md:1', quote: 'keeper' }],
          reasoning: 'The real generated bundle dispatched the verifier path.',
        };
      }),
    );
    expect(calls).toBe(1);
    expect(result.status).toBe('DONE');
    expect(matchesDeclaredReadGrammar('git rev-parse HEAD')).toBe(true);
    expect(matchesDeclaredReadGrammar('git diff --output=/tmp/x HEAD')).toBe(false);
    expect(matchesDeclaredReadGrammar('git diff --output\\=/tmp/x HEAD')).toBe(false);
    expect(matchesDeclaredReadGrammar("rg --pre 'touch /tmp/x' pattern .")).toBe(false);
    expect(matchesDeclaredReadGrammar("rg --p're' touch pattern .")).toBe(false);
    expect(matchesDeclaredReadGrammar('git show HEAD:file > /tmp/file')).toBe(false);

    expect(() =>
      assertCodeReadObservation({
        started: 1,
        returned: 1,
        incomplete: 0,
        toolCounts: Object.fromEntries(CODE_READ_AGENT_TOOLS.map((name) => [name, 1])),
        bashCommands: [{ agentId: 'test', command: 'git rev-parse HEAD' }],
      }),
    ).not.toThrow();
    expect(() =>
      assertCodeReadObservation({
        started: 1,
        returned: 1,
        incomplete: 0,
        toolCounts: {
          ...Object.fromEntries(CODE_READ_AGENT_TOOLS.map((name) => [name, 1])),
          mcp__harvester__fetch: 1,
        },
        bashCommands: [],
      }),
    ).toThrow(/connector set is not empty/u);
    expect(() =>
      assertCodeReadObservation({
        started: 1,
        returned: 1,
        incomplete: 0,
        toolCounts: { Bash: 1 },
        bashCommands: [{ agentId: 'test', command: 'git show HEAD:file > /tmp/file' }],
      }),
    ).toThrow(/outside the declared read grammar/u);
  });

  it('the pre-execution fence denies every Bash payload outside the declared grammar', () => {
    // Executes the REAL hook script the caller wires, over the exact payload shape Claude Code sends.
    // A mock here would prove only that the author agreed with themselves.
    const fence = (payload: string) =>
      spawnSync(process.execPath, [CODE_READ_FENCE_SCRIPT], { input: payload, encoding: 'utf8' });
    const bash = (command: string) => JSON.stringify({ tool_name: 'Bash', tool_input: { command } });

    // The two legitimate read shapes run.
    expect(fence(bash('git rev-parse HEAD')).status).toBe(0);
    expect(fence(bash('rg pattern .')).status).toBe(0);

    // The exact leak from the failed production walk: a redirect into global /tmp.
    const redirect = fence(bash('git show HEAD:file > /tmp/file'));
    expect(redirect.status).toBe(2);
    expect(JSON.parse(redirect.stdout).hookSpecificOutput.permissionDecision).toBe('deny');
    // The denial names the rule, never the command text — a reason string is a log line.
    expect(redirect.stdout).not.toContain('/tmp/file');

    expect(fence(bash('cat /etc/passwd')).status).toBe(2);
    expect(fence(bash('git diff --output=/tmp/x HEAD')).status).toBe(2);
    expect(fence(bash('git log ; rm -rf /tmp/x')).status).toBe(2);

    // Non-Bash tools are not this fence's business.
    expect(fence(JSON.stringify({ tool_name: 'Read', tool_input: { file_path: '/etc/passwd' } })).status).toBe(0);

    // Unparseable input is NOT permission to run — an error must never render as an allow.
    expect(fence('not json').status).toBe(2);
  });

  it('A2 turns a partial generated-bundle walk window into explicit incomplete-agent accounting', async () => {
    const bundle = await readFile(resolve('dist/cross-workflow/claude/workflow.js'), 'utf8');
    const scout = {
      headSha: '1976348c63a3dc2e4c122c5f33c7f6cb2b5fa635',
      territories: ['BE'],
      changedFiles: ['projb-be/src/api.ts'],
      changedFileCount: 1,
      mergeShas: ['1976348c63a3dc2e4c122c5f33c7f6cb2b5fa635'],
      threads: [
        {
          id: 'T1',
          type: 'feature-flow',
          name: 'partial completion pin',
          verify: 'follow the API entry point to its terminal',
        },
      ],
      operations: [],
      fields: [],
      jobs: [],
      gateFiles: [],
      authRule: '',
      armedInvariants: [],
    };
    const result = await runGeneratedBundleWithWindow({
      executeBundle: executeHarnessBundle,
      bundle,
      args: {
        reportPath: 'tmp/cross-workflow-phase2/report.md',
        branch: scout.headSha,
        debug: true,
        project: { repoRoot: '.' },
      },
      globals: globals(async (_prompt, options) => {
        // the TIME CHECKPOINT's t0 reading is the walk's first agent call, before the scout
        if (options.label.startsWith('clock · ')) return { epochSeconds: 1_000_000 };
        if (options.label === 'scout') return scout;
        return await new Promise(() => {});
      }),
      windowMs: 30,
    });
    expect(result.status).toBe('ERROR');
    expect(result.error.code).toBe('phase2_incomplete_agents');
    expect(result.error.reason).toBe('runtime_window_exhausted');
    expect(result.agents.started).toBeGreaterThan(result.agents.returned);
    // two seats complete before the window closes: the t0 clock reading and the scout
    expect(result.agents.returned).toBe(2);
    expect(result.agents.incomplete).toBe(result.agents.started - result.agents.returned);
  });

  it('A3 parses native generated-bundle results and makes malformed shapes explicit', async () => {
    const bundle = await readFile(resolve('dist/cross-workflow/claude/workflow.js'), 'utf8');
    const nativeResult = await executeHarnessBundle(
      bundle,
      {
        claims: [
          {
            id: 'a3-native-terminal',
            statement: 'The generated bundle returns the native Wave result shape.',
            files: ['AGENTS.md'],
          },
        ],
        votes: 1,
        project: { repoRoot: '.' },
      },
      globals(async () => ({
        claimId: 'a3-native-terminal',
        verdict: 'CONFIRMED',
        evidence: [{ anchor: 'AGENTS.md:1', quote: 'keeper' }],
        reasoning: 'The native bundle result reached its terminal parser.',
      })),
    );
    const parsed = parseGeneratedBundleTerminal({ result: nativeResult });
    expect(parsed.status).toBe('DONE');
    expect(parsed.mode).toBe('verify');

    const malformed = parseGeneratedBundleTerminal({
      result: { ...nativeResult, status: 'UNKNOWN' },
    });
    expect(malformed).toEqual({
      status: 'ERROR',
      error: {
        code: 'generated_bundle_result_unparseable',
        message: 'Generated Workflow output did not carry a recognized terminal result.',
        discriminator: 'object:status=UNKNOWN',
      },
    });
  });
});
