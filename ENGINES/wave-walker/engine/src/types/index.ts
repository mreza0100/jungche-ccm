// Types barrel — the one import path for every wave-walker engine type. Consumers do
// `import type { … } from '<rel>/types/index.js'`. (Ambient Workflow globals live in ./globals.d.ts,
// auto-included via tsconfig — not re-exported here.)
export type { Schema } from './schema.js';
export type * from './domain.js';
export type * from './run.js';
export type * from './agents.js';
