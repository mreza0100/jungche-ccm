// source-schemas.ts — every StructuredOutput schema object TRANSCRIBED VERBATIM from the 730-line source
// (the authoritative generated dist/workflow.js), independent of each modular schema. schemas.test.ts deep-equals each ported
// seat schema against its entry here — any drift between the port and the source fails loudly.
/* eslint-disable */

// SLICE_PROPS (source lines 305-324)
export const SLICE_PROPS = {
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
// SLICES (source lines 325-331)
export const SLICES = {
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
// SCOUT (source lines 332-348) — carries the WAVE-WALKER INVARIANT-REGISTRY FEATURE's `armedInvariants`
// field (tmp/wave-walker-investigation.md § 2.1), optional and absent from the original bundle, and the
// D1 `changedFileCount` reconciliation field (audit 2026-07-28), REQUIRED — a scout can no longer return
// an enumerated list without its separately-executed count.
export const SCOUT = {
  type: 'object',
  properties: {
    headSha: { type: 'string' },
    territories: { type: 'array', items: { type: 'string' } },
    changedFiles: { type: 'array', items: { type: 'string' } },
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
// WALK (source lines 349-357) — carries the WAVE-WALKER INVARIANT-REGISTRY FEATURE's `failureScenario`
// field on defects (§ 2.5), optional and absent from the original bundle.
export const WALK = {
  type: 'object',
  properties: {
    threadId: { type: 'string' },
    name: { type: 'string' },
    type: { type: 'string' },
    flow: { type: 'string', enum: ['INTACT', 'AT-RISK', 'BROKEN', 'N/A'] },
    trace: { type: 'string' },
    defects: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          location: { type: 'string' },
          jc: { type: 'string' },
          failureScenario: {
            type: 'string',
            description: 'the concrete input/state under which this step corrupts, aborts, or lies',
          },
        },
        required: ['what', 'location'],
      },
    },
    hygiene: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          kind: { type: 'string' },
          where: { type: 'string' },
          detail: { type: 'string' },
          jc: { type: 'string' },
        },
        required: ['kind', 'where', 'detail'],
      },
    },
    notes: { type: 'string' },
  },
  required: ['threadId', 'flow', 'trace', 'defects', 'hygiene'],
};
// GATE_SWEEP (source lines 358-363)
export const GATE_SWEEP = {
  type: 'object',
  properties: {
    file: { type: 'string' },
    gates: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string' },
          resource: { type: 'string' },
          anchor: { type: 'string' },
          idArgs: { type: 'array', items: { type: 'string' } },
          rolesAllowed: { type: 'array', items: { type: 'string' } },
          chain: { type: 'array', items: { type: 'string' } },
          orgFence: { type: 'boolean' },
          ownershipFence: { type: 'boolean' },
          notes: { type: 'string' },
        },
        required: ['id', 'anchor'],
      },
    },
  },
  required: ['file', 'gates'],
};
// JUDGE (source lines 364-368)
export const JUDGE = {
  type: 'object',
  properties: {
    verdicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          anomalyId: { type: 'string' },
          verdict: { type: 'string', enum: ['CONFIRMED', 'FALSE', 'UNPROVEN'] },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          what: { type: 'string' },
          location: { type: 'string' },
          fix: { type: 'string' },
          why: { type: 'string' },
        },
        required: ['anomalyId', 'verdict', 'severity', 'what'],
      },
    },
  },
  required: ['verdicts'],
};
// DIGEST (source lines 369-375)
export const DIGEST = {
  type: 'object',
  properties: {
    territory: { type: 'string' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          lens: { type: 'string' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high'] },
          what: { type: 'string' },
          location: { type: 'string' },
          fix: { type: 'string' },
        },
        required: ['lens', 'severity', 'what', 'location'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['territory', 'findings', 'summary'],
};
// FOLD (source lines 376-382)
export const FOLD = {
  type: 'object',
  properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    actionItems: { type: 'array', items: { type: 'string' } },
    review: { type: 'string' },
  },
  required: ['verdict', 'actionItems', 'review'],
};
// SECURITY (source lines 479-485)
// D2 SECURITY FAN-OUT (audit 2026-07-28) — filesOpened (REQUIRED) + filesSkipped: the sweep's own
// denominator; a sweep can no longer return without its coverage.
export const SECURITY = {
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
// FINAL (source lines 665-672)
export const FINAL = {
  type: 'object',
  properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    reinstated: {
      type: 'array',
      items: {
        type: 'object',
        properties: { anomalyId: { type: 'string' }, why: { type: 'string' } },
        required: ['anomalyId', 'why'],
      },
    },
    missedRisks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          where: { type: 'string' },
          severity: { type: 'string' },
          jc: { type: 'string' },
        },
        required: ['what', 'where'],
      },
    },
    rationale: { type: 'string' },
  },
  required: ['verdict', 'missedRisks'],
};
// EXTRACT (source lines 68-73)
export const EXTRACT = {
  type: 'object',
  properties: {
    claims: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          taskId: { type: 'string' },
          kind: { type: 'string', description: 'existence | behavior | contract | dep' },
          statement: { type: 'string', description: 'self-contained, refutable, <=200 chars' },
          files: { type: 'array', items: { type: 'string' } },
          context: { type: 'string' },
        },
        required: ['id', 'statement'],
      },
    },
    conflictChecks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          tasks: { type: 'array', items: { type: 'string' } },
          what: { type: 'string' },
        },
        required: ['id', 'what'],
      },
    },
  },
  required: ['claims', 'conflictChecks'],
};
// VERIFY (source lines 86-93)
export const VERIFY = {
  type: 'object',
  properties: {
    claimId: { type: 'string' },
    verdict: { type: 'string', enum: ['CONFIRMED', 'REFUTED', 'PARTIAL', 'UNPROVEN'] },
    evidence: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          anchor: { type: 'string' },
          quote: { type: 'string', description: 'VERBATIM, <=120 chars' },
        },
        required: ['anchor'],
      },
    },
    reasoning: { type: 'string' },
  },
  required: ['claimId', 'verdict', 'reasoning'],
};
// CONFLICT (source lines 118-123)
export const CONFLICT = {
  type: 'object',
  properties: {
    conflicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string', description: 'conflict | refuted-premise | freeloader' },
          tasks: { type: 'array', items: { type: 'string' } },
          what: { type: 'string' },
          evidence: { type: 'string' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          fix: { type: 'string', description: 'concrete manifest correction' },
        },
        required: ['id', 'kind', 'what', 'severity'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['conflicts', 'summary'],
};
// PROBE (source lines 155-162)
export const PROBE = {
  type: 'object',
  properties: {
    laneId: { type: 'string' },
    claims: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          statement: { type: 'string', description: 'self-contained fact, <=200 chars' },
          kind: { type: 'string', description: 'support | counter' },
          targets: {
            type: 'array',
            items: { type: 'string' },
            description: 'counter only: attacked claim ids',
          },
          anchors: {
            type: 'array',
            items: {
              type: 'object',
              properties: {
                anchor: { type: 'string' },
                quote: { type: 'string', description: 'VERBATIM, <=120 chars' },
              },
              required: ['anchor', 'quote'],
            },
          },
        },
        required: ['statement', 'anchors'],
      },
    },
    leads: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          files: { type: 'array', items: { type: 'string' } },
        },
        required: ['what'],
      },
    },
    nothingFound: { type: 'boolean' },
  },
  required: ['laneId', 'claims', 'leads'],
};
// COORD (source lines 163-171)
export const COORD = {
  type: 'object',
  properties: {
    resultSoFar: { type: 'string', description: 'best current answer, <=1200 chars' },
    keyClaimIds: {
      type: 'array',
      items: { type: 'string' },
      description: 'the LOAD-BEARING ledger ids the answer rests on',
    },
    lanes: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string', description: 'pursue | attack' },
          question: { type: 'string' },
          files: { type: 'array', items: { type: 'string' } },
          targets: {
            type: 'array',
            items: { type: 'string' },
            description: 'attack only: claim ids to challenge',
          },
          note: { type: 'string' },
        },
        required: ['id', 'kind', 'question'],
      },
    },
    dropLeads: { type: 'array', items: { type: 'string' } },
    stop: {
      type: 'object',
      properties: { done: { type: 'boolean' }, reason: { type: 'string' } },
      required: ['done'],
    },
  },
  required: ['resultSoFar', 'keyClaimIds', 'lanes', 'stop'],
};
// AUDITS (source line 172)
export const AUDITS = {
  type: 'object',
  properties: {
    audits: {
      type: 'array',
      items: {
        type: 'object',
        properties: { id: { type: 'string' }, result: { type: 'string', enum: ['pass', 'fail'] } },
        required: ['id', 'result'],
      },
    },
  },
  required: ['audits'],
};
// SYNTH carries INTENTIONAL DIVERGENCE #4: answer.minLength 80 (anti-degenerate guard) — the one field-level departure from the source line 258 literal.
// SYNTH (source line 258)
export const SYNTH = {
  type: 'object',
  properties: {
    answer: { type: 'string', minLength: 80 },
    confidence: { type: 'string', enum: ['low', 'medium', 'high'] },
    report: { type: 'string' },
  },
  required: ['answer', 'confidence', 'report'],
};
