// build.js — dependency-derived concatenating bundler for the Wave Walker Workflow engine.
//
// Why not esbuild: the Workflow sandbox needs the output file to BEGIN with
// `export const meta = { …pure literal… }` AND END with a top-level `return await …run()`.
// esbuild (any --format) restructures exports to `var meta = …; export { meta }` at the
// bottom and REJECTS top-level return inside a module. Neither is fixable by flags. So we
// concatenate the ES modules into one flat script ourselves: strip cross-module import/export
// (everything shares one top-level scope), keep meta's `export const` verbatim at the very top,
// and append the harness return as a tail.
//
// TypeScript: the source is `.ts`, but the bundle must be plain JS. We type-strip each module with
// Node's built-in `stripTypeScriptTypes` (mode:'strip') BEFORE acorn parses it. Strip mode BLANKS type
// annotations in place (replacing them with whitespace) — it never reflows code, so meta stays
// byte-identical at the top, acorn parses the result as plain JS, and no helper globals are injected.
// (This needs the source to avoid emit-requiring TS — no enums / namespaces / parameter-properties;
// we use plain `type`/annotations only.)
//
// MODULE ORDER is auto-derived, not hand-maintained: a stable topological sort of the REAL import
// graph, walked from src/engine.ts (the true entry — everything load-bearing is reachable from it).
// src/meta.ts is pinned first (nothing imports it at RUNTIME — only the dev-only src/index.ts does,
// which is itself never bundled). A "pure barrel" — a file whose entire body is import/export-from
// statements and nothing else (src/agents/index.ts, the dev-only src/index.ts) — is walked THROUGH
// but never bundled itself; its re-exported symbols still land in the flat scope via their own real
// module, discovered when the barrel's own edges are followed. Each module's edges are resolved to
// absolute paths and SORTED before recursing, so the bundle's order is stable even if a developer
// reorders unrelated import statements within a file — determinism comes from the sort, not from
// incidental source layout. The dependency rules that used to be spelled out by hand in a comment
// (prompts before index, shared/utils before agents, run.ts after runtime.ts, engine.ts last) are now
// consequences of the real import graph, not asserted separately.
//
// Three loud, build-breaking guards replace the old ORDER-membership check (assertBundled), which only
// checked SET membership and never caught an ordering bug (the exact TDZ hole a flat concatenation is
// exposed to):
//   (a) reachability — every content-bearing file under src/ must be discovered from the entry;
//       an orphan (never imported, directly or through a barrel) fails the build, naming it.
//   (b) cycles — an import cycle (even one that loops back through a barrel) fails, naming the cycle.
//   (c) post-sort order — for every module in the final order, each of its resolved dependencies is
//       asserted to sit at an EARLIER position; this is the actual invariant the flat-concatenation
//       trick depends on, verified mechanically rather than assumed.
//
// CROSS-RUNTIME COMPILE: the flat program is canonical build input to the pinned cross-workflow
// library. Its native-contract Claude compiler preserves the established raw result shape and emits a
// side-by-side candidate at dist/cross-workflow/claude/workflow.js. The production legacy artifact at
// dist/workflow.js is never touched by this build. The Codex compiler emits the sibling
// dist/cross-workflow/codex/runner.mjs from that exact program and unwraps the portable harness envelope
// back to the same raw result shape. Both manifests are generated beside their artifacts. `npm run
// verify` rebuilds every candidate target in a disposable directory and byte-compares all four files,
// while separately pinning the legacy production artifact until an equivalence-gated pointer switch.
//
// build.js itself runs under Node at BUILD time — its fs/path/child_process use never enters the
// bundle (only the stripped module source does). Edit src/, run `npm run build`, commit both.

import { readFileSync, writeFileSync, existsSync, readdirSync, mkdirSync } from 'node:fs';
import { join, dirname, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execSync } from 'node:child_process';
import { stripTypeScriptTypes } from 'node:module';
import { parse } from 'acorn';
import {
  compileHarnessProgramClaudeNative,
  compileHarnessProgramCodexNative,
} from 'cross-workflow';
import { createWaveWalkerDefinition } from './cross-workflow.config.js';

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC_DIR = join(HERE, 'src');
const OUT = join(HERE, 'dist', 'cross-workflow', 'claude', 'workflow.js');

// the true entry (everything load-bearing is reachable from here) + the one pinned-first exception
// (meta.ts is never imported by any bundled module — only by the dev-only src/index.ts).
const ENTRY = join(SRC_DIR, 'engine.ts');
const META = join(SRC_DIR, 'meta.ts');

const TAIL =
  '\n// ── entry — the Workflow harness wraps this file in an async scope and awaits its return ──\n' +
  'const ww = new WaveWalker()\n' +
  'return await ww.run()\n';

// GENERATED BANNER — deterministic (no timestamp/hash), emitted as the first line AFTER meta's closing
// brace so the bundle still BEGINS with `export const meta` (the harness requirement validate-bundle.js
// check 3 enforces).
const GENERATED_BANNER =
  '// ⚠ GENERATED FILE — DO NOT EDIT. Built from ~/.professor/ENGINES/wave-walker/engine (npm run build). Manual edits are overwritten by the next build; changes go in src/ with tests.';

const fail = (msg) => {
  console.error('✗ build failed — ' + msg);
  process.exit(1);
};

const rel = (absPath) => relative(HERE, absPath);

// ── ANALYSIS (memoized) — strip TS types + acorn-parse each module ONCE for graph discovery; classifies
// every top-level statement as an import/re-export EDGE or real CONTENT. A "pure barrel" has edges but
// zero content (agents/index.ts, the dev-only index.ts); an ambient/type-only file has neither (never
// reachable via a runtime edge — `import type` is blanked to whitespace by the strip, same as a comment).
const analysisCache = new Map();
function analyze(absPath) {
  if (analysisCache.has(absPath)) return analysisCache.get(absPath);
  if (!existsSync(absPath)) fail(rel(absPath) + ' does not exist');
  const stripped = stripTypeScriptTypes(readFileSync(absPath, 'utf8'), { mode: 'strip' });
  let ast;
  try {
    ast = parse(stripped, { ecmaVersion: 'latest', sourceType: 'module' });
  } catch (e) {
    fail(rel(absPath) + ': failed to parse after TS-type-strip — ' + e.message);
  }
  const edges = [];
  let hasContent = false;
  for (const node of ast.body) {
    if (node.type === 'ImportDeclaration') edges.push(node.source.value);
    else if (node.type === 'ExportNamedDeclaration') {
      if (node.source)
        edges.push(node.source.value); // `export { X } from '…'` — a re-export edge
      else hasContent = true; // `export const/function X` or a local `export { X }` — real content
    } else if (node.type === 'ExportAllDeclaration') edges.push(node.source.value);
    else hasContent = true; // any other statement (incl. `export default …`) is real content
  }
  const info = { edges, hasContent, isBarrel: !hasContent && edges.length > 0 };
  analysisCache.set(absPath, info);
  return info;
}

// every import in a bundled module must resolve to a real file under src/ — a bare specifier (an npm
// package) or a broken relative path both fail loudly rather than leaving a dangling reference.
function resolveSpec(fromAbsPath, spec) {
  if (!spec.startsWith('.'))
    fail(
      rel(fromAbsPath) +
        ': imports external package "' +
        spec +
        '" — the Workflow sandbox has no module loader.\n' +
        '       Vendor it as pure JS under src/ (discoverable from engine.ts) or move the work to a compute agent.',
    );
  const target = join(dirname(fromAbsPath), spec).replace(/\.js$/, '.ts');
  if (!existsSync(target))
    fail(rel(fromAbsPath) + ': imports "' + spec + '" but ' + rel(target) + ' does not exist');
  return target;
}

// flatten a barrel's edges down to their ultimate CONTENT targets (recursing through a barrel-of-barrels,
// though none exists today) — used only by the post-sort belt-and-braces check below.
function flattenBarrel(absPath, seen = new Set()) {
  if (seen.has(absPath)) fail('barrel cycle at ' + rel(absPath));
  seen.add(absPath);
  const out = [];
  for (const spec of analyze(absPath).edges) {
    const target = resolveSpec(absPath, spec);
    out.push(...(analyze(target).isBarrel ? flattenBarrel(target, seen) : [target]));
  }
  return out;
}

// ── DISCOVERY — a stable DFS topological sort from ENTRY. Each module's own edges are resolved + SORTED
// (by absolute path) before recursing, so the output order never depends on incidental import-statement
// layout. A barrel is visited (so cycles through it are still caught) but never pushed to `order`. Colors
// are the classic white/gray/black cycle guard: gray = on the current DFS stack.
function discoverOrder() {
  const WHITE = 0,
    GRAY = 1,
    BLACK = 2;
  const color = new Map();
  const order = [];
  function visit(absPath, chain) {
    const c = color.get(absPath) || WHITE;
    if (c === BLACK) return;
    if (c === GRAY) {
      const start = chain.indexOf(absPath);
      fail('import cycle: ' + [...chain.slice(start), absPath].map(rel).join(' → '));
    }
    color.set(absPath, GRAY);
    const { edges, isBarrel } = analyze(absPath);
    const deps = [...new Set(edges.map((spec) => resolveSpec(absPath, spec)))].sort();
    for (const dep of deps) visit(dep, [...chain, absPath]);
    color.set(absPath, BLACK);
    if (!isBarrel) order.push(absPath);
  }
  visit(ENTRY, []);
  return [META, ...order]; // meta.ts pinned first — see header comment
}

// ── GUARD (a) — every content-bearing .ts file under src/ (excluding ambient `.d.ts`, which is
// compile-time-only and never imported at runtime) must have been discovered from ENTRY. An orphan is
// almost always a bug: a new module wired into nothing, or an old one nothing references anymore.
function collectAllTsFiles(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) collectAllTsFiles(p, out);
    else if (entry.isFile() && entry.name.endsWith('.ts') && !entry.name.endsWith('.d.ts'))
      out.push(p);
  }
  return out;
}
function assertReachability(finalOrder) {
  const reached = new Set(finalOrder);
  const orphans = collectAllTsFiles(SRC_DIR).filter(
    (f) => analyze(f).hasContent && !reached.has(f),
  );
  if (orphans.length)
    fail(
      'the following content-bearing src/ file(s) are UNREACHABLE from src/engine.ts (never imported,\n' +
        '       directly or through a re-export barrel) — wire them in or delete them:\n' +
        orphans.map((f) => '       - ' + rel(f)).join('\n'),
    );
}

// ── GUARD (c) — for every module in the final order, each of its resolved dependencies must sit at an
// EARLIER position. This is the actual correctness invariant a flat concatenation depends on (a later
// position would read an undefined/TDZ binding); verified mechanically instead of trusted from the sort.
function assertImportsPrecedeUse(finalOrder) {
  const pos = new Map(finalOrder.map((p, i) => [p, i]));
  for (const absPath of finalOrder) {
    const { edges } = analyze(absPath);
    for (const spec of edges) {
      const target = resolveSpec(absPath, spec);
      const deps = analyze(target).isBarrel ? flattenBarrel(target) : [target];
      for (const dep of deps) {
        if (!pos.has(dep))
          fail(rel(absPath) + ' depends on ' + rel(dep) + ', which never made it into the bundle');
        if (pos.get(dep) >= pos.get(absPath))
          fail(
            'module order violation: ' +
              rel(absPath) +
              ' (position ' +
              pos.get(absPath) +
              ') depends on ' +
              rel(dep) +
              ' (position ' +
              pos.get(dep) +
              '), which does not come earlier',
          );
      }
    }
  }
}

// strip a module down to its bare declarations for flat concatenation. AST-based (acorn) so it is
// provably correct: a real parser tells code from strings, so it can never mis-handle a multi-line
// import, a string literal that contains the word `import`, or an unusual import/export form.
function stripModule(absPath, isMeta) {
  let code = stripTypeScriptTypes(readFileSync(absPath, 'utf8'), { mode: 'strip' });
  const ast = parse(code, { ecmaVersion: 'latest', sourceType: 'module' });
  // collect byte-offset spans to delete, then apply right-to-left so earlier offsets stay valid
  const cuts = [];
  for (const node of ast.body) {
    if (node.type === 'ImportDeclaration') {
      resolveSpec(absPath, node.source.value); // defense-in-depth — discovery already validated every edge
      cuts.push([node.start, node.end]); // drop the whole import — its symbols live in the flat scope
    } else if (isMeta) {
      // meta.ts keeps `export const meta` verbatim — leave its export untouched
    } else if (node.type === 'ExportNamedDeclaration') {
      if (node.declaration) {
        cuts.push([node.start, node.declaration.start]); // `export const X` → strip just the `export ` keyword
      } else {
        if (node.source) resolveSpec(absPath, node.source.value);
        cuts.push([node.start, node.end]); // `export { … } [from '…']` re-export → drop the whole statement
      }
    } else if (node.type === 'ExportAllDeclaration') {
      if (node.source) resolveSpec(absPath, node.source.value);
      cuts.push([node.start, node.end]); // `export * from '…'` → drop the whole statement
    } else if (node.type === 'ExportDefaultDeclaration') {
      cuts.push([node.start, node.declaration.start]); // `export default <x>` → strip the `export default ` prefix
    }
  }
  cuts.sort((a, b) => b[0] - a[0]);
  for (const [start, end] of cuts) code = code.slice(0, start) + code.slice(end);
  return code;
}

// runBuild — the whole bundling pipeline, parameterized on the output path so verify.js can drive the
// IDENTICAL logic against a disposable temp path without disturbing the committed bundle. Returns the
// bundle text (verify.js compares it in-memory too; it still writes it, for a real on-disk diff target).
//
// dist/package.json — the bundle is neither valid ESM (a top-level `return`, the harness TAIL) nor valid
// CommonJS (`export const meta`, the harness contract) in ISOLATION; it only compiles as the body of the
// async function the harness wraps it in (validate-bundle.js check 1b). `node --check` (check 1, a cheap
// pre-check) only tolerates this combination when Node's module-type is left UNSPECIFIED — no "type" key
// anywhere in its package.json ancestry — which engages Node's syntax-detection heuristic instead of
// strict ESM/CJS parsing. rr's ../workflow.js gets this for free (it sits OUTSIDE engine/'s ancestry
// entirely, so package.json lookup finds nothing); our dist/workflow.js sits INSIDE engine/, which sets
// "type":"module" for build tooling (build.js/verify.js need ESM). A directory-scoped, TYPE-LESS
// package.json breaks that inheritance for everything under dist/ without touching engine/'s own type.
// Ensured on every build so a `rm -rf dist` self-heals; harmless (if unnecessary) for verify.js's
// disposable temp-dir build, which already sits outside any type:module ancestry.
function ensureDetectableModuleType(outPath) {
  const dir = dirname(outPath);
  if (!existsSync(dir)) return; // verify.js's mkdtempSync already created its scratch dir
  const pkgPath = join(dir, 'package.json');
  const pkg = '{ "private": true }\n';
  if (!existsSync(pkgPath) || readFileSync(pkgPath, 'utf8') !== pkg) writeFileSync(pkgPath, pkg);
}

const encodedManifest = (value) => JSON.stringify(value, null, 2) + '\n';

export async function runBuild(outPath, { emitAllTargets = false } = {}) {
  const finalOrder = discoverOrder();
  assertReachability(finalOrder);
  assertImportsPrecedeUse(finalOrder);
  mkdirSync(dirname(outPath), { recursive: true });
  ensureDetectableModuleType(outPath);

  let bundle = '';
  finalOrder.forEach((absPath, i) => {
    const modRel = rel(absPath);
    // the FIRST module (meta.ts) gets NO banner — the harness needs `export const meta` as the
    // literal first bytes of the bundle. Every later module keeps its `// ╔══ module:` banner.
    if (i > 0)
      bundle +=
        '// ╔══ module: ' + modRel + ' ' + '═'.repeat(Math.max(0, 60 - modRel.length)) + '\n';
    bundle += stripModule(absPath, absPath === META).replace(/\n+$/, '\n');
    // the GENERATED banner rides immediately after meta's closing brace — the first line after it.
    if (i === 0) bundle += GENERATED_BANNER + '\n';
  });
  bundle += TAIL;

  const definition = createWaveWalkerDefinition(bundle);
  const claudeArtifact = await compileHarnessProgramClaudeNative(definition);
  writeFileSync(outPath, claudeArtifact.code);

  if (emitAllTargets) {
    const claudeDir = dirname(outPath);
    const targetRoot = dirname(claudeDir);
    const codexDir = join(targetRoot, 'codex');
    mkdirSync(codexDir, { recursive: true });
    const codexArtifact = await compileHarnessProgramCodexNative(definition);
    writeFileSync(join(claudeDir, 'manifest.json'), encodedManifest(claudeArtifact.manifest));
    writeFileSync(join(codexDir, 'runner.mjs'), codexArtifact.code);
    writeFileSync(join(codexDir, 'manifest.json'), encodedManifest(codexArtifact.manifest));
  }
  return claudeArtifact.code;
}

// CLI entry — only when run directly (`node build.js`), not when imported by verify.js.
if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const bundle = await runBuild(OUT, { emitAllTargets: true });
  console.log('bundled → ' + OUT + ' · ' + bundle.split('\n').length + ' lines');
  console.log('compiled → ' + join(HERE, 'dist', 'cross-workflow', 'codex', 'runner.mjs'));
  // chain the validator — a non-zero exit here fails `npm run build` loudly
  execSync(
    'node ' + JSON.stringify(join(HERE, 'validate-bundle.js')) + ' ' + JSON.stringify(OUT),
    { stdio: 'inherit' },
  );
}
