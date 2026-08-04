// batching.test.ts — the E2 verify-panel batching reducers (src/batching.ts), hand-traced against the
// proven reference variant's clusterClaims/batchClaims logic (haiku tiering excluded by design).
import { describe, expect, it } from 'vitest';
import { batchClaims, clusterClaims, prefixDepth2 } from '../src/batching.js';
import type { ClaimIn } from '../src/types/index.js';

const claim = (id: string, files?: string[]): ClaimIn => ({ id, statement: 's-' + id, files });

describe('prefixDepth2', () => {
  it('returns the first two path segments', () => {
    expect(prefixDepth2('app-be/src/infrastructure/graphql/x.ts')).toBe('app-be/src');
    expect(prefixDepth2('a/b')).toBe('a/b');
  });
  it('a single-segment path stays itself; empty/undefined degrade to ""', () => {
    expect(prefixDepth2('README.md')).toBe('README.md');
    expect(prefixDepth2('')).toBe('');
    expect(prefixDepth2(undefined)).toBe('');
    expect(prefixDepth2(null)).toBe('');
  });
});

describe('clusterClaims — file-cluster affinity on depth-2 path prefixes', () => {
  it('claims sharing a depth-2 prefix land in ONE cluster', () => {
    const clusters = clusterClaims([
      claim('c1', ['app-be/src/a.ts']),
      claim('c2', ['app-fe/app/b.tsx']),
      claim('c3', ['app-be/src/deep/c.ts']),
    ]);
    expect(clusters.map((cl) => cl.map((c) => c.id))).toEqual([['c1', 'c3'], ['c2']]);
  });
  it('a claim with NO files opens its OWN cluster every time (no affinity evidence, never merged)', () => {
    const clusters = clusterClaims([claim('c1'), claim('c2'), claim('c3', ['x/y/z.ts'])]);
    expect(clusters.map((cl) => cl.map((c) => c.id))).toEqual([['c1'], ['c2'], ['c3']]);
  });
  it('a multi-file claim bridges clusters through ANY shared prefix, growing the cluster prefix set', () => {
    const clusters = clusterClaims([
      claim('c1', ['app-be/src/a.ts']),
      claim('c2', ['app-be/src/b.ts', 'app-fe/app/b.tsx']),
      claim('c3', ['app-fe/app/c.tsx']), // joins c1's cluster via the prefix c2 dragged in
    ]);
    expect(clusters.map((cl) => cl.map((c) => c.id))).toEqual([['c1', 'c2', 'c3']]);
  });
});

describe('batchClaims — greedy ≤4-claim slices per cluster, cluster order preserved', () => {
  it('splits a large cluster into ≤4-claim batches', () => {
    const nine = Array.from({ length: 9 }, (_, i) =>
      claim('c' + (i + 1), ['same/prefix/f' + i + '.ts']),
    );
    const batches = batchClaims(nine);
    expect(batches.map((b) => b.length)).toEqual([4, 4, 1]);
    expect(batches[0].map((c) => c.id)).toEqual(['c1', 'c2', 'c3', 'c4']);
  });
  it('never mixes clusters inside a batch even when both are under-filled', () => {
    const batches = batchClaims([
      claim('a1', ['be/x/a.ts']),
      claim('a2', ['be/x/b.ts']),
      claim('b1', ['fe/y/a.tsx']),
    ]);
    expect(batches.map((b) => b.map((c) => c.id))).toEqual([['a1', 'a2'], ['b1']]);
  });
  it('empty input → no batches', () => expect(batchClaims([])).toEqual([]));
});
