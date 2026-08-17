// SCOUT — the scout-scheduler seat: diffs the wave, emits the thread manifest AND the ledger schedule
// in one pass (source lines 332-348, 400-415).
import { CONFIG } from '../../config.js';
import { buildScout } from './prompts.js';
import type { Agent, Schema, ScoutArgs } from '../../types/index.js';

export const SCOUT: Schema = {
  type: 'object',
  properties: {
    headSha: { type: 'string' },
    territories: { type: 'array', items: { type: 'string' } },
    changedFiles: { type: 'array', items: { type: 'string' } },
    // D1 FILE-SET RECONCILIATION (audit 2026-07-28) — the separately-executed `wc -l` count; the engine
    // fails the walk when this disagrees with changedFiles.length (one corrective retry first).
    changedFileCount: {
      type: 'integer',
      description:
        'the separately-executed `wc -l` count of the name-only diff — NEVER the length of the enumerated list',
    },
    mergeShas: { type: 'array', items: { type: 'string' } },
    threads: {
      type: 'array',
      description:
        'the functional/hygiene walk manifest — feature flow | seam | field | schema/db | invariant | test-data | dead-code-ripple',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          type: { type: 'string' },
          name: { type: 'string' },
          scope: { type: 'string' },
          files: { type: 'array', items: { type: 'string' } },
          verify: { type: 'string' },
        },
        required: ['id', 'type', 'name', 'verify'],
      },
    },
    operations: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string' },
          anchor: { type: 'string' },
          resultType: { type: 'string' },
        },
        required: ['id', 'anchor'],
      },
    },
    fields: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          ownerType: { type: 'string' },
          field: { type: 'string' },
          apis: { type: 'array', items: { type: 'string' } },
          sdl: {
            type: 'object',
            properties: { anchor: { type: 'string' }, typeToken: { type: 'string' } },
          },
        },
        required: ['id', 'ownerType', 'field'],
      },
    },
    jobs: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          jobId: { type: 'string' },
          kind: { type: 'string', description: 'producer | consumer | ai' },
          files: { type: 'array', items: { type: 'string' } },
          fieldIds: { type: 'array', items: { type: 'string' } },
          hint: { type: 'string' },
        },
        required: ['jobId', 'kind', 'files', 'fieldIds'],
      },
    },
    gateFiles: {
      type: 'array',
      items: { type: 'string' },
      description:
        "EVERY file within this project's configured gate-resolver surface, repo-wide (args.project.gateResolverPattern) — fence-outlier context; [] when no surface is configured",
    },
    authRule: {
      type: 'string',
      description:
        "VERBATIM auth/role-fence rule extracted per this project's configured auth doc (args.project.authDoc); '' when none is configured",
    },
    // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — optional, absent when no
    // registry was supplied (CONFIG.INVARIANTS empty). engine.ts's computeArmedInvariants unions this
    // with a zero-token territory-glob fail-safe, so a scout omission can never silently disarm a hunt.
    armedInvariants: {
      type: 'array',
      description: 'registry entries whose triggers fired against this diff',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          matchedFiles: { type: 'array', items: { type: 'string' } },
          reason: { type: 'string' },
        },
        required: ['id'],
      },
    },
  },
  required: [
    'headSha',
    'changedFiles',
    'changedFileCount',
    'threads',
    'fields',
    'jobs',
    'gateFiles',
  ],
};

export const scout: Agent<ScoutArgs> = {
  tier: CONFIG.TIER.scout,
  effort: CONFIG.EFFORT.scout,
  schema: SCOUT,
  buildPrompt: buildScout,
};
