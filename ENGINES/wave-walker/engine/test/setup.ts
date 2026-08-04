// vitest setupFiles — set the ambient globals the engine reads at MODULE LOAD (`new Configs(args)`,
// `const _agent = agent` in runtime.ts) BEFORE any module imports. Config/engine tests that need a
// DIFFERENT args shape (verify/investigate mode, agents overrides) override `globalThis.args` per-test
// via `vi.resetModules()` + a fresh dynamic import; this is only the safe WALK-mode default.
globalThis.args = { reportPath: 'docs/dev/waves/test/report.md' };
globalThis.log = () => {};
globalThis.phase = () => {};
globalThis.agent = async () => ({});
globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
  Promise.all(thunks.map((t) => t()));
globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
