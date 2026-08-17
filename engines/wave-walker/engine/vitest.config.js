import { defineConfig } from 'vitest/config'

// Coverage is measured over the engine LOGIC modules. meta.js (pure data) and index.js (entry
// wiring) carry no testable logic; build/validate are build-time tooling. rules.ts (the R1-R8
// zero-token rule engine) and ledger.ts (the investigate-mode claim ledger reducers) are the
// pure-reducer modules the port adds real coverage for — near-100%, since they are the
// equivalence-proof surface. Everything else — config validation, schemas, utils, prompt
// builders, and the engine phases (exercised via mocked ambient globals) — clears the 90% bar.
export default defineConfig({
  test: {
    include: ['test/**/*.test.ts'],
    setupFiles: ['test/setup.ts'],
    // 15s (vitest default 5s) — a per-file `vi.resetModules()` + fresh dynamic import re-transforms
    // the WHOLE reachable module graph the FIRST time it's touched in a worker (esbuild transform is
    // cached by file path across resetModules calls, not paid again per test); every later fresh
    // import of the same graph is fast. The WALK TELEMETRY feature's module graph (engine.ts, config.ts,
    // runtime.ts, utils/index.ts, types/*) is large enough that this one-time cost occasionally exceeds
    // the 5s default on whichever test happens to run first — never a logic hang (every subsequent test
    // in the same file completes in tens-to-hundreds of ms).
    testTimeout: 15000,
    coverage: {
      provider: 'v8',
      include: ['src/config.ts', 'src/rules.ts', 'src/ledger.ts', 'src/batching.ts', 'src/engine.ts', 'src/runtime.ts', 'src/utils/**/*.ts', 'src/agents/**/*.ts'],
      exclude: ['src/meta.ts', 'src/index.ts', 'src/agents/index.ts', 'build.js', 'validate-bundle.js', 'vitest.config.js', 'test/**'],
      reporter: ['text', 'text-summary'],
      thresholds: {
        lines: 90, statements: 90, functions: 90, branches: 85,
        'src/rules.ts': { lines: 98, statements: 98, functions: 100, branches: 95 },
        'src/ledger.ts': { lines: 98, statements: 98, functions: 100, branches: 95 },
        'src/batching.ts': { lines: 98, statements: 98, functions: 100, branches: 95 },
      },
    },
  },
})
