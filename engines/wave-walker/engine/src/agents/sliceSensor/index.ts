// SLICE SENSOR — one call per scheduled producer/consumer/ai job: a PURE EXTRACTOR, zero judgment
// (source lines 305-331, 451-468). Escalates to CONFIG.SENSOR_ESCALATE on a dead respawn (see run.ts).
import { CONFIG } from '../../config.js';
import { buildSliceSensor } from './prompts.js';
import type { Agent, Schema, SliceSensorArgs } from '../../types/index.js';

export const SLICE_PROPS: Record<string, Schema> = {
  fieldId: { type: 'string' },
  producer: {
    type: 'object',
    properties: {
      anchor: { type: 'string' },
      writer: { type: 'string' },
      typeToken: { type: 'string' },
      encoding: { type: 'string' },
      valueLiterals: { type: 'array', items: { type: 'string' } },
    },
  },
  dbColumn: {
    type: 'object',
    properties: {
      anchor: { type: 'string' },
      columnName: { type: 'string' },
      columnType: { type: 'string' },
      checkLiterals: { type: 'array', items: { type: 'string' } },
    },
  },
  resolver: { type: 'object', properties: { anchor: { type: 'string' } } },
  feSelection: {
    type: 'object',
    properties: { anchor: { type: 'string' }, queryName: { type: 'string' } },
  },
  feTypes: {
    type: 'array',
    items: {
      type: 'object',
      properties: {
        anchor: { type: 'string' },
        typeToken: { type: 'string' },
        kind: { type: 'string', description: 'generated | hand' },
      },
    },
  },
  consumers: {
    type: 'array',
    items: {
      type: 'object',
      properties: {
        anchor: { type: 'string' },
        name: { type: 'string' },
        decode: { type: 'string' },
        decodeExpr: { type: 'string', description: 'VERBATIM read/parse expression, <=80 chars' },
        context: { type: 'string', description: 'production | test | generated | story' },
        comparedLiterals: { type: 'array', items: { type: 'string' } },
        aliasChain: { type: 'array', items: { type: 'string' } },
      },
      required: ['anchor'],
    },
  },
  danglingRefs: {
    type: 'array',
    items: { type: 'object', properties: { ref: { type: 'string' }, anchor: { type: 'string' } } },
  },
  notes: { type: 'string' },
};

export const SLICES: Schema = {
  type: 'object',
  properties: {
    jobId: { type: 'string' },
    slices: {
      type: 'array',
      items: { type: 'object', properties: SLICE_PROPS, required: ['fieldId'] },
    },
    undeclaredReads: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          side: { type: 'string' },
          property: { type: 'string' },
          anchor: { type: 'string' },
          expr: { type: 'string' },
        },
        required: ['property', 'anchor'],
      },
    },
  },
  required: ['jobId', 'slices'],
};

export const sliceSensor: Agent<SliceSensorArgs> = {
  tier: CONFIG.TIER.sliceSensor,
  effort: CONFIG.EFFORT.sliceSensor,
  schema: SLICES,
  buildPrompt: buildSliceSensor,
};
