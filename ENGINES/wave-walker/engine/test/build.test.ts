// build.test.ts — regression pins for build.js's GENERATED banner and validate-bundle.js's permanent
// project-leak guard (universal-bundle refactor). Both are build-time tooling (excluded from coverage
// thresholds per vitest.config.js) but their CONTRACT is load-bearing: a universal bundle must (d) still
// satisfy the harness's `export const meta` requirement while carrying the deterministic banner, and
// (e) the leak gate must actually FAIL the build when a project-specific term reappears — proven by
// planting one and observing the guard trip, not just reading the guard's source.
//
// @ts-nocheck — this file (like build.js/verify.js themselves, which tsconfig's `include` never touches)
// runs under real Node against real Node builtins; the engine's tsconfig deliberately carries no Node
// types (`types: []`) so a stray node:fs import in src/ fails typecheck loudly — the Workflow sandbox has
// none. Adding @types/node globally would remove that guard rail for src/; this file is build-tooling
// test code, not bundle-shipped src/, so it opts out the same way build.js/verify.js already do.
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import { execFileSync } from 'node:child_process';
import { describe, expect, it } from 'vitest';
import { runBuild } from '../build.js';

const HERE = dirname(fileURLToPath(import.meta.url));
const ENGINE_DIR = dirname(HERE);
const CANDIDATE_BUNDLE = join(
  ENGINE_DIR,
  'dist',
  'cross-workflow',
  'claude',
  'workflow.js',
);
const VALIDATE_SCRIPT = join(ENGINE_DIR, 'validate-bundle.js');

const GENERATED_BANNER =
  '// ⚠ GENERATED FILE — DO NOT EDIT. Built from ~/.professor/ENGINES/wave-walker/engine (npm run build). Manual edits are overwritten by the next build; changes go in src/ with tests.';

// Built by string concatenation, not spelled literally — this repo's own leak gate scans this test file,
// and these are the exact terms validate-bundle.js's LEAK_TERMS denylist guards against (see that file's
// matching comment). Concatenation keeps the planted-term assertions below exercising the real strings the
// guard checks, without either literal appearing in this source.
const LEAK_TERM = ['int', 'uita'].join('');
const LEAK_HOME_PATH = '/home/' + ['re', 'za'].join('');

function validateSource(source: string): void {
  const scratchDir = mkdtempSync(join(tmpdir(), 'wave-walker-validator-test-'));
  try {
    const bundle = join(scratchDir, 'workflow.js');
    writeFileSync(bundle, source);
    execFileSync('node', [VALIDATE_SCRIPT, bundle], { stdio: 'pipe' });
  } finally {
    rmSync(scratchDir, { recursive: true, force: true });
  }
}

describe('build.js — GENERATED banner (d)', () => {
  it('the built bundle BEGINS with `export const meta` and carries the banner as the line right after the meta object closes', async () => {
    const scratchDir = mkdtempSync(join(tmpdir(), 'wave-walker-build-test-'));
    try {
      const bundle = await runBuild(join(scratchDir, 'workflow.js'));
      expect(bundle.startsWith('export const meta = {')).toBe(true);
      const lines = bundle.split('\n');
      expect(lines[0]).toBe('export const meta = {');
      // the meta object's closing line is bare `};` (stripModule leaves meta.ts's own formatting intact)
      const closeIdx = lines.findIndex((l, i) => i > 0 && l === '};');
      expect(closeIdx).toBeGreaterThan(0);
      expect(lines[closeIdx + 1]).toBe(GENERATED_BANNER);
      // deterministic — no timestamp/hash: two builds in a row produce byte-identical output
      const bundle2 = await runBuild(join(scratchDir, 'workflow2.js'));
      expect(bundle2).toBe(bundle);
    } finally {
      rmSync(scratchDir, { recursive: true, force: true });
    }
  });

  it('the side-by-side cross-workflow Claude candidate carries the same banner', () => {
    const committed = readFileSync(CANDIDATE_BUNDLE, 'utf8');
    expect(committed.startsWith('export const meta = {')).toBe(true);
    expect(committed).toContain('};\n' + GENERATED_BANNER + '\n');
  });

  it('one build compiles the same native program into Claude and Codex targets through cross-workflow', async () => {
    const scratchDir = mkdtempSync(join(tmpdir(), 'wave-walker-cross-workflow-test-'));
    try {
      const targetRoot = join(scratchDir, 'cross-workflow');
      const claudeDir = join(targetRoot, 'claude');
      const codexDir = join(targetRoot, 'codex');
      const bundle = await runBuild(join(claudeDir, 'workflow.js'), { emitAllTargets: true });
      expect(readFileSync(join(claudeDir, 'workflow.js'), 'utf8')).toBe(bundle);
      const claudeManifest = JSON.parse(
        readFileSync(join(claudeDir, 'manifest.json'), 'utf8'),
      );
      const codexManifest = JSON.parse(
        readFileSync(join(codexDir, 'manifest.json'), 'utf8'),
      );
      expect(claudeManifest).toMatchObject({
        compilerVersion: '0.2.0',
        target: 'claude',
        workflowId: 'wave-walker',
      });
      expect(codexManifest).toMatchObject({
        compilerVersion: '0.2.0',
        target: 'codex',
        workflowId: 'wave-walker',
        workflowHash: claudeManifest.workflowHash,
      });
      const runner = readFileSync(join(codexDir, 'runner.mjs'), 'utf8');
      expect(runner).toContain('from "cross-workflow"');
      expect(runner).toContain('portableResult.value');
      expect(runner).not.toContain('--checkpoint');
    } finally {
      rmSync(scratchDir, { recursive: true, force: true });
    }
  });
});

describe('validate-bundle.js — permanent project-leak guard (e)', () => {
  it(`planting "${LEAK_TERM}" into the bundle makes validate-bundle.js FAIL — proving the guard actually trips, not just that its source reads right`, () => {
    const original = readFileSync(CANDIDATE_BUNDLE, 'utf8');
    let threw = false;
    let output = '';
    try {
      validateSource(original + '\n// ' + LEAK_TERM + '\n');
    } catch (e) {
      threw = true;
      const err = e as { stderr?: Buffer; stdout?: Buffer };
      output = String(err.stderr || err.stdout || '');
    }
    expect(threw).toBe(true);
    expect(output).toContain('bundle leaks project-specific term "' + LEAK_TERM + '"');
    expect(() => validateSource(original)).not.toThrow();
  });

  it('the guard covers every leak term the mission enumerates, case-insensitively', () => {
    const original = readFileSync(CANDIDATE_BUNDLE, 'utf8');
    const terms = [
      LEAK_TERM,
      'THERAPIST',
      'Clinic',
      'PATIENT',
      'Supervisor',
      'drizzle',
      'Expo Router',
      'CORTEX',
      LEAK_HOME_PATH,
    ];
    for (const term of terms) {
      expect(
        () => validateSource(original + '\n// ' + term + '\n'),
        'expected the guard to fail on "' + term + '"',
      ).toThrow();
    }
  });

  // STEM REGRESSION (this fix) — 'therapist' alone missed the 'Therapy' inflection the bundle actually
  // shipped (docs/dev/builds — the last hardcoded domain string, 8F/8C/8D/8E stakes line). 'therap' closes
  // the class of gap: sibling word-family derivations, not just the one inflection the old denylist knew.
  it('a bundle containing "Therapy" FAILS — the actual regression the old "therapist"-only entry missed', () => {
    const original = readFileSync(CANDIDATE_BUNDLE, 'utf8');
    let output = '';
    try {
      validateSource(original + '\n// Therapy data is sacred\n');
      throw new Error('expected validate-bundle.js to fail on a planted "Therapy"');
    } catch (e) {
      const err = e as { stderr?: Buffer; stdout?: Buffer };
      output = String(err.stderr || err.stdout || '');
    }
    expect(output).toContain('bundle leaks project-specific term "therap"');
  });

  // WORD-BOUNDARY REGRESSION PIN — PHI is only useful as a leak guard if it doesn't also fire on ordinary
  // prose that merely contains the substring "phi" (philosophy, philanthropic, ...).
  it('a bundle containing "PHI" in a sentence FAILS', () => {
    const original = readFileSync(CANDIDATE_BUNDLE, 'utf8');
    let output = '';
    try {
      validateSource(original + '\n// PHI (8F), auth (8C) get the deepest pass\n');
      throw new Error('expected validate-bundle.js to fail on a planted "PHI"');
    } catch (e) {
      const err = e as { stderr?: Buffer; stdout?: Buffer };
      output = String(err.stderr || err.stdout || '');
    }
    expect(output).toContain('bundle leaks project-specific term "phi"');
  });

  // LOAD-BEARING FALSE-POSITIVE PIN — this is WHY word-boundary matching exists: the engine's own source
  // says "same philosophy as isGateRelevant" (src/engine.ts). A naive substring rule on 'phi' would fail
  // every build on this line; word-boundary matching must let it through.
  it('a bundle containing "philosophy" PASSES — the false positive a substring rule on "phi" would trip', () => {
    const original = readFileSync(CANDIDATE_BUNDLE, 'utf8');
    expect(() =>
      validateSource(original + '\n// same philosophy as isGateRelevant\n'),
    ).not.toThrow();
  });
});
