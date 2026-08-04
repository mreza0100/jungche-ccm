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
const COMMITTED_BUNDLE = join(ENGINE_DIR, 'dist', 'workflow.js');
const VALIDATE_SCRIPT = join(ENGINE_DIR, 'validate-bundle.js');

const GENERATED_BANNER =
  '// ⚠ GENERATED FILE — DO NOT EDIT. Built from ~/.professor/ENGINES/wave-walker/engine (npm run build). Manual edits are overwritten by the next build; changes go in src/ with tests.';

// Built by string concatenation, not spelled literally — this repo's own leak gate scans this test file,
// and these are the exact terms validate-bundle.js's LEAK_TERMS denylist guards against (see that file's
// matching comment). Concatenation keeps the planted-term assertions below exercising the real strings the
// guard checks, without either literal appearing in this source.
const LEAK_TERM = ['int', 'uita'].join('');
const LEAK_HOME_PATH = '/home/' + ['re', 'za'].join('');

describe('build.js — GENERATED banner (d)', () => {
  it('the built bundle BEGINS with `export const meta` and carries the banner as the line right after the meta object closes', () => {
    const scratchDir = mkdtempSync(join(tmpdir(), 'wave-walker-build-test-'));
    try {
      const bundle = runBuild(join(scratchDir, 'workflow.js'));
      expect(bundle.startsWith('export const meta = {')).toBe(true);
      const lines = bundle.split('\n');
      expect(lines[0]).toBe('export const meta = {');
      // the meta object's closing line is bare `};` (stripModule leaves meta.ts's own formatting intact)
      const closeIdx = lines.findIndex((l, i) => i > 0 && l === '};');
      expect(closeIdx).toBeGreaterThan(0);
      expect(lines[closeIdx + 1]).toBe(GENERATED_BANNER);
      // deterministic — no timestamp/hash: two builds in a row produce byte-identical output
      const bundle2 = runBuild(join(scratchDir, 'workflow2.js'));
      expect(bundle2).toBe(bundle);
    } finally {
      rmSync(scratchDir, { recursive: true, force: true });
    }
  });

  it('the COMMITTED dist/workflow.js (what npm run build actually produced) carries the same banner', () => {
    const committed = readFileSync(COMMITTED_BUNDLE, 'utf8');
    expect(committed.startsWith('export const meta = {')).toBe(true);
    expect(committed).toContain('};\n' + GENERATED_BANNER + '\n');
  });
});

describe('validate-bundle.js — permanent project-leak guard (e)', () => {
  it(`planting "${LEAK_TERM}" into the bundle makes validate-bundle.js FAIL — proving the guard actually trips, not just that its source reads right`, () => {
    const original = readFileSync(COMMITTED_BUNDLE, 'utf8');
    try {
      // plant the leak term in a harmless spot — end of file, after the harness TAIL. The file must stay
      // syntactically valid JS (checks 1/1b run before the leak check) — a line comment satisfies that.
      writeFileSync(COMMITTED_BUNDLE, original + '\n// ' + LEAK_TERM + '\n');
      let threw = false;
      let output = '';
      try {
        execFileSync('node', [VALIDATE_SCRIPT], { stdio: 'pipe' });
      } catch (e) {
        threw = true;
        const err = e as { stderr?: Buffer; stdout?: Buffer };
        output = String(err.stderr || err.stdout || '');
      }
      expect(threw).toBe(true);
      expect(output).toContain('bundle leaks project-specific term "' + LEAK_TERM + '"');
    } finally {
      // clean up — restore the committed bundle byte-for-byte, whether the assertions above passed or not
      writeFileSync(COMMITTED_BUNDLE, original);
    }
    // post-cleanup sanity: the guard passes again on the restored bundle
    expect(() => execFileSync('node', [VALIDATE_SCRIPT], { stdio: 'pipe' })).not.toThrow();
  });

  it('the guard covers every leak term the mission enumerates, case-insensitively', () => {
    const original = readFileSync(COMMITTED_BUNDLE, 'utf8');
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
    try {
      for (const term of terms) {
        writeFileSync(COMMITTED_BUNDLE, original + '\n// ' + term + '\n');
        expect(
          () => execFileSync('node', [VALIDATE_SCRIPT], { stdio: 'pipe' }),
          'expected the guard to fail on "' + term + '"',
        ).toThrow();
      }
    } finally {
      writeFileSync(COMMITTED_BUNDLE, original);
    }
  });
});
