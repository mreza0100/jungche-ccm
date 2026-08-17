// claimExtractor prompt — the source's inline construction (wave-walker.js lines 74-77) plus the E2
// breadth-first clause ("Target ~4-6 claims per task, covering EVERY task — breadth across tasks before
// depth within any task."), the port's ONE intended extractor-prompt change (proven: full 16/16-task
// coverage on wave.md vs depth-first's 12 dropped tasks under the old 24-claim cap).
import { RO } from '../shared.js';
import type { ClaimExtractorArgs } from '../../types/index.js';

export const buildClaimExtractor = ({ manifestPath, repoRoot }: ClaimExtractorArgs): string =>
  'You are the CLAIM EXTRACTOR of a manifest-verify panel. Read the wave manifest at ' +
  manifestPath +
  ' (repo root ' +
  repoRoot +
  ') and mine EVERY load-bearing factual claim a hallucination could hide in — a claim is load-bearing when refuting it would change a task\'s design or scope. Per task extract: existence claims (a named file/symbol/field/column/enum/env var/prompt the task assumes EXISTS or assumes ABSENT — incl. every Named anchor and File-plan path), behavior premises ("X currently does Y" statements the design rests on — the classic hallucination class), contract claims (SDL/SQS/WS shapes vs live code), dep claims (cross-task Depends and shared symbols). Each: id T{n}-C{k}, taskId, kind, a SELF-CONTAINED refutable statement, exact files to start probing, 1-line context. ORDER MOST LOAD-BEARING FIRST (a cap may drop the tail). Target ~4-6 claims per task, covering EVERY task — breadth across tasks before depth within any task. Also emit conflictChecks: task pairs/sets whose File plans, Contracts, or data models might collide (same file EDIT+DELETE, one field two shapes, duplicated work) — checks only, no verdicts.' +
  RO +
  ' Structured output: claims, conflictChecks.';
