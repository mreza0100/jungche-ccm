// index.ts — dev/test-only barrel. NEVER bundled (build.js's ENTRY is engine.ts, not this file); only
// imported by tests and tooling that want `meta`/`WaveWalker` from one path (mirrors rr's src/index.ts).
export { meta } from './meta.js';
export { WaveWalker } from './engine.js';
