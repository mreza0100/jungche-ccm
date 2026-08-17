// batching.ts — the E2 verify-panel batching reducers, pure functions over plain claim lists (same
// discipline as rules.ts/ledger.ts). Ported from the PROVEN reference variant
// (tmp/walker-measure/variants/wave-walker.v2-manifest-scale.js) with its haiku existence/behavior
// tiering DELIBERATELY dropped — every batch rides the verifier tier (sonnet/xhigh); measured haiku
// verifiers did 2.5× the tool calls (+21% tokens, +235% latency) and were rejected.
//
// Clustering rule: claims sharing a file-path prefix of depth ≥2 (e.g. 'apps/backend/src') land in one
// cluster; a claim with NO files opens its own cluster (never merged — no affinity evidence). Batches
// are greedy ≤4-claim slices of each cluster, cluster order preserved.
import type { ClaimIn } from './types/index.js';

export function prefixDepth2(f: string | undefined | null): string {
  const parts = String(f || '').split('/');
  return parts.slice(0, 2).join('/');
}

interface Cluster {
  prefixes: string[];
  claims: ClaimIn[];
}

export function clusterClaims(list: ClaimIn[]): ClaimIn[][] {
  const clusters: Cluster[] = [];
  for (const c of list) {
    const prefixes = (c.files || []).map(prefixDepth2).filter(Boolean);
    let cluster = prefixes.length
      ? clusters.find((cl) => cl.prefixes.some((p) => prefixes.includes(p)))
      : null;
    if (!cluster) {
      cluster = { prefixes: [], claims: [] };
      clusters.push(cluster);
    }
    cluster.claims.push(c);
    for (const p of prefixes) if (!cluster.prefixes.includes(p)) cluster.prefixes.push(p);
  }
  return clusters.map((cl) => cl.claims);
}

export function batchClaims(list: ClaimIn[]): ClaimIn[][] {
  const out: ClaimIn[][] = [];
  for (const cluster of clusterClaims(list))
    for (let i = 0; i < cluster.length; i += 4) out.push(cluster.slice(i, i + 4));
  return out;
}
