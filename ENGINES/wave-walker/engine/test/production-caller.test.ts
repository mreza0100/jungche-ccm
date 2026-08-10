// @ts-nocheck — production-caller.js is the Node-only headless seam around the generated bundle.
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import { executeHarnessBundle } from 'cross-workflow';
import {
  CODE_READ_AGENT_TOOLS,
  CODE_READ_ALLOWED_TOOLS,
  CODE_READ_BUILTIN_TOOLS,
  CODE_READ_DENIED_TOOLS,
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
    expect(() => assertFaithfulCodeReadPolicy()).toThrow(/claude_bash_not_pre_execution_fenced/u);

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
