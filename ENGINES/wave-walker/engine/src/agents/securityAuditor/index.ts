// SECURITY AUDITOR — the diff-scoped wave-level security sweep, audit/security.md 8A-8K, fanned out
// one auditor per changed-file slice (D2, audit 2026-07-28).
import { CONFIG } from '../../config.js';
import { buildSecurityAuditor } from './prompts.js';
import type { Agent, Schema, SecurityAuditorArgs } from '../../types/index.js';

export const SECURITY: Schema = {
  type: 'object',
  properties: {
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          category: { type: 'string', description: '8A-8K per the audit doc' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          what: { type: 'string' },
          location: { type: 'string' },
          expected: { type: 'string' },
          got: { type: 'string' },
          fix: { type: 'string' },
        },
        required: ['id', 'category', 'severity', 'what', 'location'],
      },
    },
    categoriesSwept: { type: 'array', items: { type: 'string' } },
    // D2 SECURITY FAN-OUT — the sweep's own denominator: what was ACTUALLY read, and what was assigned
    // but skipped (with why). Required, so a sweep can never return without its coverage.
    filesOpened: { type: 'array', items: { type: 'string' } },
    filesSkipped: {
      type: 'array',
      items: {
        type: 'object',
        properties: { file: { type: 'string' }, why: { type: 'string' } },
        required: ['file'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['findings', 'categoriesSwept', 'filesOpened', 'summary'],
};

export const securityAuditor: Agent<SecurityAuditorArgs> = {
  tier: CONFIG.TIER.securityAuditor,
  effort: CONFIG.EFFORT.securityAuditor,
  schema: SECURITY,
  buildPrompt: buildSecurityAuditor,
};
