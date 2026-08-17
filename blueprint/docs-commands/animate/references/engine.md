# /animate — Engine Contract & Recipes

Read by the `/animate` build agent (Step 3) and consulted by the orchestrator for verification (Step 4) and recording (Step 5). Behaviors marked ⚓ are debugged invariants — visual design is free, these are not.

**Contents:** 1 Architecture · 2 Core skeleton · 3 UI chrome · 4 Study mode · 5 Educational layer · 6 Visual defaults · 7 Verification · 8 Recording · 9 Pitfalls

## 1. Architecture — pure function of time

One global clock `t` (seconds). Every visual is a deterministic function of `t` — zero `setTimeout`/`setInterval`, zero CSS-transition-driven state. A single `requestAnimationFrame` loop advances `t` by `dt × speed` while playing and calls `render(t)` only when dirty. This is what makes scrubbing, act jumps, study mode, and headless recording trivially correct.

Four primitives drive everything:

- **cues** — time-windowed tweens `{t0, t1, ease, f(progress)}`; `f` sets transforms/opacity from progress.
- **togs** — boolean class flips: element gains a class when `t ≥ t0`, loses it when scrubbed back.
- **layers** — one container per act with a fade window `{a, b}`; opacity from `layerAlpha(t)`, plus `live`/`gone` classes so hidden acts cost no paint.
- **discrete** — per-frame lookups: active caption window, act-nav active/done state, time readout, end-of-run handling.

⚓ **Idempotent render.** `render(t)` twice with the same `t` produces identical DOM. No render step may depend on the previous frame. Seek anywhere → render → correct frame.

## 2. Core skeleton (copy, then extend)

```js
'use strict';
const el = (id) => document.getElementById(id);
const clamp = (v, a, b) => (v < a ? a : v > b ? b : v);
const ramp = (t, a, b) => clamp((t - a) / (b - a), 0, 1);
const E = { out: (p) => 1 - Math.pow(1 - p, 4), inout: (p) => (p < 0.5 ? 4 * p * p * p : 1 - Math.pow(-2 * p + 2, 3) / 2), linear: (p) => p };

/* ===== CONFIG — generated from storyboard.md, single source of truth ===== */
const TOTAL = 105; // seconds at 1×
const ACT_RANGES = [
  [0, 33],
  [33, 49],
]; // [start, end) per act
const ACT_STARTS = [5, 33]; // nav jump targets (skip cold opens)
const ACT_LABELS = ['I — …', 'II — …'];
const BEATS = [{ t: 7.5, act: 0, label: 'R1', tip: 'R1 — … [F3]' }]; // spine ticks
const CAPTIONS = [[5.0, 6.6, 'ACT I — …']]; // [from, to, text]
const STOPS = [32.9, 48.9]; // study-mode slide points
const HOLDS = [[13.0, 14.0]]; // gate pauses — playhead heartbeat

/* ===== ENGINE ===== */
let t = 0,
  playing = false,
  speed = 1,
  lastNow = 0,
  dirty = true;
let study = false,
  stopIdx = 0,
  stopAcc = 0,
  wasAuto = false;
const cues = [],
  togs = [];
let LAYERS = [];
function cue(t0, dur, f, e) {
  cues.push({ t0, t1: t0 + dur, e: e || E.out, f });
}
function tog(t0, node, cls) {
  if (node) togs.push({ t0, el: node, cls: cls || 'on' });
}
function layerAlpha(tt, a, b) {
  const fi = a <= 0 ? 1 : ramp(tt, a, a + 0.35),
    fo = 1 - ramp(tt, b - 0.35, b);
  return tt < a - 0.01 || tt > b + 0.01 ? 0 : Math.min(fi, fo);
}
function render(tt) {
  for (const c of cues) c.f(c.e(clamp((tt - c.t0) / (c.t1 - c.t0), 0, 1)), tt);
  for (const g of togs) g.el.classList.toggle(g.cls, tt >= g.t0);
  for (const L of LAYERS) {
    const al = layerAlpha(tt, L.a, L.b);
    L.el.style.opacity = al.toFixed(3);
    L.el.classList.toggle('live', al > 0.5);
    L.el.classList.toggle('gone', al === 0);
  }
  discrete(tt);
}
function discrete(tt) {
  let txt = '',
    al = 0;
  for (const c of CAPTIONS)
    if (tt >= c[0] - 0.2 && tt <= c[1] + 0.2) {
      txt = c[2];
      al = Math.min(ramp(tt, c[0] - 0.2, c[0]), 1 - ramp(tt, c[1], c[1] + 0.2));
      break;
    }
  const cap = el('caption');
  if (cap) {
    if (txt && cap.textContent !== txt) cap.textContent = txt;
    cap.style.opacity = al.toFixed(2);
  }
  let ai = 0;
  for (let i = 0; i < ACT_RANGES.length; i++) if (tt >= ACT_RANGES[i][0]) ai = i;
  document.querySelectorAll('.actBtn').forEach((b, i) => {
    b.classList.toggle('active', i === ai);
    b.classList.toggle('done', tt >= ACT_RANGES[i][1] - 0.01);
  });
  const f = (s) => Math.floor(s / 60) + ':' + String(Math.floor(s % 60)).padStart(2, '0');
  const tr = el('timeRead');
  if (tr) tr.textContent = f(tt) + ' / ' + f(TOTAL);
}
function seek(v) {
  t = clamp(v, 0, TOTAL);
  dirty = true;
}
function setPlaying(p) {
  playing = p;
  document.body.classList.toggle('paused', !p);
}
function jumpAct(i) {
  if (study) slide(i);
  else {
    seek(ACT_STARTS[i]);
    setPlaying(true);
  }
}
function slide(i) {
  stopIdx = clamp(i, 0, STOPS.length - 1);
  stopAcc = 0;
  seek(STOPS[stopIdx]);
}
function setStudy(v) {
  study = v;
  document.body.classList.toggle('study', v);
  if (v) {
    setPlaying(false);
    slide(0);
  } else dirty = true;
}
function frame(now) {
  const dt = (now - lastNow) / 1000;
  lastNow = now;
  if (playing && !study) {
    t = clamp(t + dt * speed, 0, TOTAL);
    if (t >= TOTAL) setPlaying(false);
    dirty = true;
  }
  if (playing && study) {
    stopAcc += dt;
    if (stopAcc >= 5) {
      stopAcc = 0;
      slide(stopIdx + 1);
    }
  }
  if (dirty) {
    render(t);
    dirty = false;
  }
  requestAnimationFrame(frame);
}
```

`init()` builds the scenes (registers cues/togs per beat), fills `LAYERS`, wires controls, runs `render(t)` once, then `lastNow = performance.now(); requestAnimationFrame(frame); setPlaying(true);`.

## 3. Required UI chrome

- **Controls bar** — play/pause (icon reflects state), restart, speed pill toggling 1×/2× (`aria-pressed`), one button per act.
- **Timeline spine** — full-width scrubber: progress fill + playhead, hover ghost playhead with time tooltip, one tick per `BEATS` entry carrying `data-tip`, act labels along it; click seeks, tick hover shows its tip.
- **Caption HUD** — single line above the spine, driven by `CAPTIONS` windows.
- **Tooltip singleton** — one absolutely-positioned div; a delegated `mouseover` on `[data-tip]` fills and places it.
- **End card** — appears at `TOTAL`, replay button (`seek(0); setPlaying(true)`), plus the Sources list (§ 5).
- **Keyboard** — Space play/pause (ignore when target is a button), R restart, S speed, ←/→ ±5 s, digits 1–9 act jumps. Skip all handling when target is INPUT/TEXTAREA/SELECT.

⚓ **Act jumps route through `jumpAct(i)`** — `seek` + `setPlaying(true)` together. Seeking alone from a paused or ended state strands the viewer on a transition frame.
⚓ **Auto-pause on `visibilitychange`/`blur`, resume on focus** — track `wasAuto` so a manually paused viewer stays paused.
⚓ **`prefers-reduced-motion: reduce` forces study mode on** at init and on media-query change.

## 4. Study mode

The educational heart: the same animation becomes step-through slides. Each `STOPS` entry is a curated freeze-frame — one per teaching moment, typically just after a beat completes. Prev/next buttons (visible only in study mode) plus 5 s auto-advance while "playing". Layer transitions reduce to short opacity fades (`body.study .layer { transition: opacity .2s }`). A visible toggle in the controls bar; digits/act buttons jump between slides instead of times.

## 5. Educational layer

What separates a teaching animation from a screensaver:

- Every beat's `tip` states precisely **what** happens and ends with its fact ID(s) from `facts.md` — e.g. `"7 — QA adversarial testing → 6-bugs.md [F12, F14]"`.
- Every caption explains **why** the moment matters, not what is visibly happening.
- All identifiers verbatim from source — a viewer must be able to grep any label back to the code or doc.
- **Legend** — a fixed or toggleable key for every symbol/color class used (lanes, gates, artifacts, failure paths).
- **Sources card** — the end card lists the citations from `facts.md` so a learner can go read the truth.
- Distinct visual grammar for: normal flow, gates (user/QA approval), loops/retries, failure paths, artifacts (file chips with real filenames).

## 6. Visual defaults (override per subject)

Dark theme; fixed stage (e.g. 1480×860) wrapped in `#stageWrap`, scaled to viewport via `transform: scale(Math.min(innerWidth/W, (innerHeight-120)/H))` on resize (rAF-debounced); one color per lane/actor, declared once as CSS variables; SVG for paths and travel — for moving dots along a path, precompute a lookup table with `getPointAtLength` and index it by progress; easing palette from `E`.

## 7. Verification protocol (Step 4 § Behavior)

```bash
cd tmp/animate/{name} && python3 -m http.server 8923
```

Drive `http://localhost:8923/{name}.html` with the Playwright MCP browser:

1. Console messages: clean (favicon 404 exempt).
2. Pause mid-act → click a _different_ act button → assert playback resumed and the act's content rendered (the historical regression).
3. Watch the full run at 2× → end card + replay visible.
4. Toggle study mode → step every stop with next → toggle back → playback resumes.
5. Hover three random spine ticks → tooltip text matches the storyboard.
6. Screenshot mid-act for every act (these go to the user at delivery).

## 8. Recording

Headless capture — the MCP browser can't record video; use playwright-core directly:

```js
// record.mjs — run with: node record.mjs
import { createRequire } from 'module';
// anchor at any package.json whose node_modules has playwright-core (ESM resolves
// from the importing file, not cwd)
const require = createRequire('{abs path to a package.json that has playwright-core}');
const { chromium } = require('playwright-core');
const DIR = '{abs output dir}';
const browser = await chromium.launch({
  // if launch reports a missing browser: ls ~/Library/Caches/ms-playwright/ (macOS) or ~/.cache/ms-playwright/ (Linux) and
  // point executablePath at the chrome-headless-shell binary that exists
  executablePath: '{from the cache dir}',
});
const ctx = await browser.newContext({
  viewport: { width: 1200, height: 750 },
  recordVideo: { dir: DIR + '/rec', size: { width: 1200, height: 750 } },
});
const page = await ctx.newPage();
await page.goto('file://' + DIR + '/{name}.html'); // file: fine here — not the MCP browser
await page.click('#speedPill'); // 2×
await page.evaluate(() => {
  seek(0);
  setPlaying(true);
});
await page.waitForTimeout((TOTAL / 2 + 3) * 1000); // full run at 2× + end card
await ctx.close(); // flushes the video
await browser.close();
```

webm → GIF (≈5 MB at 900 px — GitHub READMEs animate GIFs inline, not HTML):

```bash
ffmpeg -i rec/*.webm -vf "fps=6,scale=900:-1:flags=lanczos,split[s0][s1];\
[s0]palettegen=max_colors=128[p];[s1][p]paletteuse=dither=bayer:bayer_scale=4" {name}.gif
```

## 9. Pitfalls

- The Playwright **MCP** browser refuses `file:` URLs — always verify over `http.server`. Headless playwright-core (§ 8) loads `file:` fine.
- `node script.mjs` resolves imports from the script's own directory — `createRequire` anchored at a package.json that owns the dependency is the fix.
- Tooltip text lives in `data-tip` attributes — the accuracy diff (Step 4) must extract attributes, not just visible text.
- Keep the config block (§ 2) as the only place beat times appear; scene-builder functions read `BEATS`/`CAPTIONS`, never hardcode times twice.
