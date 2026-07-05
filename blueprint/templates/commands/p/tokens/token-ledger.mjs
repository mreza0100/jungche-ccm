#!/usr/bin/env node
// token-ledger — per-agent / per-operation token attribution for Claude Code sessions.
// Zero dependencies (node: builtins only). READ-ONLY over transcripts. No network.
//
// Truth source: sub-agent transcript JSONL written by Claude Code at
//   {root}/projects/{projectSlug}/{conversationId}/subagents/agent-{agentId}.jsonl
//   ...and nested workflow runs under .../subagents/workflows/wf_*/agent-*.jsonl
// The main loop is {root}/projects/{projectSlug}/{conversationId}.jsonl (sibling of the dir).
// Each assistant line carries message.usage + message.model; streaming writes multiple
// lines per API call, so we dedup before summing (see DEDUP note below).

import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import readline from "node:readline";

// ─── EDITABLE PRICING (USD per 1M tokens) ──────────────────────────────────────
// Match by substring on the model id (lowercased). cache-write = 1.25x input,
// cache-read = 0.1x input — the standard Anthropic prompt-caching multipliers.
// Update these as prices change. Unknown model → cost 0 + a warning, never a crash.
const PRICING = [
  // [substring, inputPerMTok, outputPerMTok]
  ["opus", 15.0, 75.0],
  ["sonnet", 3.0, 15.0],
  ["haiku", 0.8, 4.0],
  ["fable", 3.0, 15.0], // Fable 5 — priced as a Sonnet-class model; adjust if it diverges
  ["mythos", 15.0, 75.0], // Mythos — priced as an Opus-class model; adjust if it diverges
];
const CACHE_WRITE_MULT = 1.25;
const CACHE_READ_MULT = 0.1;

function priceFor(model) {
  const id = String(model || "").toLowerCase();
  const row = PRICING.find(([sub]) => id.includes(sub));
  return row ? { in: row[1], out: row[2], found: true } : { in: 0, out: 0, found: false };
}

const UNKNOWN_MODELS = new Set();
function costUSD(model, u) {
  const p = priceFor(model);
  if (!p.found && model && !UNKNOWN_MODELS.has(model)) {
    UNKNOWN_MODELS.add(model);
    warn(`unknown model "${model}" — cost counted as $0; add it to PRICING`);
  }
  const inRate = p.in / 1e6;
  return (
    u.in * inRate +
    u.out * (p.out / 1e6) +
    u.cw * inRate * CACHE_WRITE_MULT +
    u.cr * inRate * CACHE_READ_MULT
  );
}

// ─── ARG PARSING ───────────────────────────────────────────────────────────────
function parseArgs(argv) {
  const a = {
    roots: [],
    session: null,
    all: false,
    project: null,
    detail: null,
    json: false,
    byWorkflow: false,
    filter: null,
  };
  for (let i = 0; i < argv.length; i++) {
    const v = argv[i];
    if (v === "--all") a.all = true;
    else if (v === "--json") a.json = true;
    else if (v === "--session") a.session = argv[++i];
    else if (v === "--root") a.roots.push(argv[++i]);
    else if (v === "--project") a.project = argv[++i];
    else if (v === "--detail") a.detail = argv[++i];
    else if (v === "--by-workflow") a.byWorkflow = true;
    else if (v === "--filter") a.filter = argv[++i];
    else if (v === "-h" || v === "--help") a.help = true;
    else warn(`unknown arg ignored: ${v}`);
  }
  return a;
}

const WARNINGS = [];
function warn(m) {
  WARNINGS.push(m);
}

// ─── DISCOVERY ──────────────────────────────────────────────────────────────────
// Claude Code slugifies the cwd by replacing every "/" with "-" → the project dir name.
function projectSlug(cwd) {
  return cwd.replace(/\//g, "-");
}

function defaultRoots() {
  const home = os.homedir();
  return [
    path.join(home, ".claude"),
    path.join(home, ".claude-sessions"),
  ].filter((p) => fs.existsSync(p));
}

// A "project dir" is any .../projects/{slug} directory across all roots (including an
// optional multi-account ~/.claude-sessions/sNNN/projects/{slug} layout — we glob for it).
function findProjectDirs(roots, slug) {
  const dirs = [];
  for (const root of roots) {
    // root may itself be a projects/ parent, OR a sessions parent holding many sNNN/projects.
    const candidates = [path.join(root, "projects", slug)];
    const sessionsBase = root; // e.g. ~/.claude-sessions
    if (fs.existsSync(sessionsBase) && fs.statSync(sessionsBase).isDirectory()) {
      for (const ent of safeReaddir(sessionsBase)) {
        candidates.push(path.join(sessionsBase, ent, "projects", slug));
      }
    }
    for (const c of candidates) {
      if (fs.existsSync(c) && fs.statSync(c).isDirectory()) dirs.push(c);
    }
  }
  return [...new Set(dirs)];
}

function safeReaddir(p) {
  try {
    return fs.readdirSync(p);
  } catch {
    return [];
  }
}

// A "session" here = one main-conversation JSONL + its sibling {id}/subagents tree.
// Returns {conversationId, mainFile, subagentsDir, mtime} per conversation found.
function listSessions(projectDirs) {
  const sessions = [];
  for (const dir of projectDirs) {
    for (const ent of safeReaddir(dir)) {
      if (!ent.endsWith(".jsonl")) continue;
      const id = ent.slice(0, -6);
      const mainFile = path.join(dir, ent);
      const subDir = path.join(dir, id, "subagents");
      let mtime = 0;
      try {
        mtime = fs.statSync(mainFile).mtimeMs;
      } catch {}
      sessions.push({
        conversationId: id,
        mainFile,
        subagentsDir: fs.existsSync(subDir) ? subDir : null,
        mtime,
      });
    }
  }
  return sessions;
}

// All agent-*.jsonl under a subagents dir, including nested workflows/wf_*/.
function findAgentFiles(subagentsDir) {
  const out = [];
  if (!subagentsDir) return out;
  const stack = [subagentsDir];
  while (stack.length) {
    const d = stack.pop();
    for (const ent of safeReaddir(d)) {
      const full = path.join(d, ent);
      let st;
      try {
        st = fs.statSync(full);
      } catch {
        continue;
      }
      if (st.isDirectory()) stack.push(full);
      else if (ent.startsWith("agent-") && ent.endsWith(".jsonl")) out.push(full);
    }
  }
  return out;
}

// ─── PARSING ────────────────────────────────────────────────────────────────────
// DEDUP: streaming writes multiple assistant lines per API call sharing the same
// message.id. We key on (message.id, requestId) and keep the LAST occurrence — the
// final line carries the complete cumulative usage for that call. Verified on real
// files: 43 assistant lines collapsed to 18 distinct calls. Summing raw lines would
// overcount 2-3x. (requestId rarely splits a message.id but is kept for safety.)
async function parseTranscript(file) {
  const calls = new Map(); // key -> { usage, model, ts, hint }
  let malformed = 0;
  let stream;
  try {
    stream = fs.createReadStream(file, { encoding: "utf8" });
  } catch (e) {
    warn(`cannot open ${file}: ${e.message}`);
    return { calls, malformed, meta: null };
  }
  const rl = readline.createInterface({ input: stream, crlfDelay: Infinity });
  for await (const line of rl) {
    if (!line.trim()) continue;
    let d;
    try {
      d = JSON.parse(line);
    } catch {
      malformed++;
      continue;
    }
    if (d.type !== "assistant") continue;
    const m = d.message || {};
    const u = m.usage;
    if (!u) continue;
    const key = `${m.id || ""}|${d.requestId || ""}`;
    calls.set(key, {
      usage: {
        in: u.input_tokens || 0,
        out: u.output_tokens || 0,
        cw: u.cache_creation_input_tokens || 0,
        cr: u.cache_read_input_tokens || 0,
      },
      model: m.model || "unknown",
      ts: d.timestamp || null,
      hint: contentHint(m.content),
      attributionAgent: d.attributionAgent || null,
      attributionSkill: d.attributionSkill || null,
    });
  }
  return { calls, malformed };
}

// Short operation hint: first ~80 chars of assistant text, or the tool being called.
function contentHint(content) {
  if (typeof content === "string") return trunc(content);
  if (!Array.isArray(content)) return "";
  for (const b of content) {
    if (b && b.type === "text" && b.text) return trunc(b.text);
  }
  for (const b of content) {
    if (b && b.type === "tool_use") {
      const t = b.name || "tool";
      const inp = b.input || {};
      const arg = inp.file_path || inp.path || inp.command || inp.pattern || inp.description || "";
      return trunc(`[${t}] ${typeof arg === "string" ? arg : ""}`.trim());
    }
  }
  return "";
}

function trunc(s) {
  s = String(s).replace(/\s+/g, " ").trim();
  return s.length > 80 ? s.slice(0, 80) + "…" : s;
}

// The first user line of an agent transcript holds its task prompt — the best
// human label when meta.json is generic. Returns ~80-char snippet.
function firstUserSnippet(file) {
  try {
    const fd = fs.openSync(file, "r");
    const buf = Buffer.alloc(4096);
    const n = fs.readSync(fd, buf, 0, 4096, 0);
    fs.closeSync(fd);
    const firstLine = buf.toString("utf8", 0, n).split("\n")[0];
    const d = JSON.parse(firstLine);
    const c = d.message && d.message.content;
    if (typeof c === "string") return trunc(c);
    if (Array.isArray(c)) {
      for (const b of c) if (b.type === "text") return trunc(b.text);
    }
  } catch {}
  return "";
}

// meta.json sidecar (agent-{id}.meta.json) carries {agentType, description?}.
// description is the richest label for /wave:builder sub-agents ("BE developer", "gitter SETUP").
function readMeta(agentFile) {
  const metaFile = agentFile.replace(/\.jsonl$/, ".meta.json");
  try {
    return JSON.parse(fs.readFileSync(metaFile, "utf8"));
  } catch {
    return null;
  }
}

function agentIdOf(file) {
  const m = path.basename(file).match(/^agent-([^.]+)\.jsonl$/);
  return m ? m[1] : path.basename(file);
}

// Label priority: meta.description → meta.agentType → attributionAgent → prompt snippet → id.
function labelFor(agentFile, meta, sampleCall, snippet) {
  if (meta && meta.description) return meta.description;
  if (meta && meta.agentType) return meta.agentType;
  if (sampleCall && sampleCall.attributionAgent) return sampleCall.attributionAgent;
  if (snippet) return snippet.slice(0, 50);
  return agentIdOf(agentFile);
}

// ─── AGGREGATION ─────────────────────────────────────────────────────────────────
function emptyAgg() {
  return { calls: 0, in: 0, out: 0, cw: 0, cr: 0, cost: 0, models: new Set() };
}
function foldCall(agg, c) {
  agg.calls++;
  agg.in += c.usage.in;
  agg.out += c.usage.out;
  agg.cw += c.usage.cw;
  agg.cr += c.usage.cr;
  agg.cost += costUSD(c.model, c.usage);
  agg.models.add(c.model);
}
// Fold one row's aggregate into another (for grouping rows into runs/totals).
function mergeAgg(into, agg) {
  into.calls += agg.calls;
  into.in += agg.in;
  into.out += agg.out;
  into.cw += agg.cw;
  into.cr += agg.cr;
  into.cost += agg.cost;
  for (const m of agg.models) into.models.add(m);
}

// Extract the wf_* run id from an agent file path, or null if not under one.
function workflowIdOf(file) {
  const m = file.match(/\/workflows\/(wf_[^/]+)\//);
  return m ? m[1] : null;
}

async function buildLedger(session) {
  const rows = [];
  let totalMalformed = 0;

  // Main conversation loop as its own row.
  if (fs.existsSync(session.mainFile)) {
    const { calls, malformed } = await parseTranscript(session.mainFile);
    totalMalformed += malformed;
    const agg = emptyAgg();
    const ordered = [...calls.values()];
    for (const c of ordered) foldCall(agg, c);
    if (agg.calls > 0) {
      rows.push({
        id: session.conversationId.slice(0, 8),
        label: "MAIN (conversation loop)",
        agg,
        detail: ordered,
        wf: null,
        conv: session.conversationId,
        mtime: session.mtime,
      });
    }
  }

  // Each sub-agent file → one row.
  const agentFiles = findAgentFiles(session.subagentsDir);
  for (const file of agentFiles) {
    const { calls, malformed } = await parseTranscript(file);
    totalMalformed += malformed;
    const ordered = [...calls.values()];
    if (ordered.length === 0) continue;
    const meta = readMeta(file);
    const snippet = meta && meta.description ? "" : firstUserSnippet(file);
    const agg = emptyAgg();
    for (const c of ordered) foldCall(agg, c);
    let mtime = 0;
    try {
      mtime = fs.statSync(file).mtimeMs;
    } catch {}
    rows.push({
      id: agentIdOf(file),
      label: labelFor(file, meta, ordered[0], snippet),
      agg,
      detail: ordered,
      file,
      wf: workflowIdOf(file),
      conv: session.conversationId,
      mtime,
    });
  }
  return { rows, totalMalformed };
}

// ─── OUTPUT ──────────────────────────────────────────────────────────────────────
function fmtInt(n) {
  return n.toLocaleString("en-US");
}
function fmtUSD(n) {
  return "$" + n.toFixed(4);
}
function modelShort(models) {
  return [...models].map((m) => m.replace(/^claude-/, "")).join(",") || "—";
}

// Render an aligned text grid. `leftCols` = set of column indices left-aligned
// (the rest right-align); `sepBefore` = set of row indices to print a separator above.
function renderGrid(headers, data, leftCols, sepBefore = new Set()) {
  const widths = headers.map((h, i) =>
    Math.max(h.length, ...data.map((r) => String(r[i] || "").length))
  );
  const pad = (s, i) =>
    leftCols.has(i) ? String(s).padEnd(widths[i]) : String(s).padStart(widths[i]);
  const sep = widths.map((w) => "─".repeat(w)).join("─┼─");
  console.log(headers.map((h, i) => pad(h, i)).join(" │ "));
  console.log(sep);
  data.forEach((row, ri) => {
    if (sepBefore.has(ri)) console.log(sep);
    console.log(row.map((c, i) => pad(c, i)).join(" │ "));
  });
}

function printTable(rows) {
  rows.sort((a, b) => b.agg.cost - a.agg.cost);
  const total = emptyAgg();
  for (const r of rows) {
    total.calls += r.agg.calls;
    total.in += r.agg.in;
    total.out += r.agg.out;
    total.cw += r.agg.cw;
    total.cr += r.agg.cr;
    total.cost += r.agg.cost;
  }
  const H = ["AGENT / OPERATION", "MODEL", "CALLS", "IN", "OUT", "CACHE-W", "CACHE-R", "EST USD"];
  const data = rows.map((r) => [
    r.label.length > 38 ? r.label.slice(0, 37) + "…" : r.label,
    modelShort(r.agg.models),
    String(r.agg.calls),
    fmtInt(r.agg.in),
    fmtInt(r.agg.out),
    fmtInt(r.agg.cw),
    fmtInt(r.agg.cr),
    fmtUSD(r.agg.cost),
  ]);
  data.push([
    "TOTAL",
    "",
    String(total.calls),
    fmtInt(total.in),
    fmtInt(total.out),
    fmtInt(total.cw),
    fmtInt(total.cr),
    fmtUSD(total.cost),
  ]);
  renderGrid(H, data, new Set([0, 1]), new Set([data.length - 1]));
  // Calibration line: the four token definitions.
  const fresh = total.in + total.out + total.cw;
  console.log(
    `\nToken definitions  —  output-only: ${fmtInt(total.out)}` +
      `  ·  in+out (no cache): ${fmtInt(total.in + total.out)}` +
      `  ·  in+out+cache-write (fresh/billed-new): ${fmtInt(fresh)}` +
      `  ·  +cache-read (grand total): ${fmtInt(fresh + total.cr)}`
  );
}

function printDetail(rows, query) {
  const q = query.toLowerCase();
  const hit = rows.find(
    (r) => r.id.toLowerCase().includes(q) || r.label.toLowerCase().includes(q)
  );
  if (!hit) {
    console.log(`No agent matched "${query}".`);
    return;
  }
  console.log(`Detail for: ${hit.label}  [${hit.id}]  (${hit.agg.calls} calls)\n`);
  const H = ["#", "TIMESTAMP", "MODEL", "IN", "OUT", "C-W", "C-R", "USD", "HINT"];
  const data = hit.detail.map((c, i) => [
    String(i + 1),
    (c.ts || "").replace("T", " ").replace(/\.\d+Z$/, ""),
    (c.model || "").replace(/^claude-/, ""),
    fmtInt(c.usage.in),
    fmtInt(c.usage.out),
    fmtInt(c.usage.cw),
    fmtInt(c.usage.cr),
    fmtUSD(costUSD(c.model, c.usage)),
    c.hint || "",
  ]);
  // left-align timestamp(1), model(2), hint(8); right-align the numeric columns
  renderGrid(H, data, new Set([1, 2, 8]));
}

// Group rows by their wf_* run dir → one row per workflow run. Rows not under a
// wf_* dir (session-level /wave:builder agents, MAIN loops) fold into one trailing summary
// row labeled "(non-workflow agents)" — never silently dropped.
function printByWorkflow(rows) {
  const groups = new Map(); // wf id -> { wf, conv, agentCount, agg, mtime }
  const nonWf = { agg: emptyAgg(), agentCount: 0, mtime: 0 };
  for (const r of rows) {
    if (r.wf) {
      let g = groups.get(r.wf);
      if (!g) {
        g = { wf: r.wf, conv: r.conv, agentCount: 0, agg: emptyAgg(), mtime: 0 };
        groups.set(r.wf, g);
      }
      g.agentCount++;
      g.mtime = Math.max(g.mtime, r.mtime || 0);
      mergeAgg(g.agg, r.agg);
    } else {
      nonWf.agentCount++;
      nonWf.mtime = Math.max(nonWf.mtime, r.mtime || 0);
      mergeAgg(nonWf.agg, r.agg);
    }
  }
  const list = [...groups.values()].sort((a, b) => b.agg.cost - a.agg.cost);
  const total = emptyAgg();
  const fmtDate = (ms) => (ms ? new Date(ms).toISOString().slice(0, 10) : "—");
  const mkRow = (date, wf, conv, count, agg) => {
    mergeAgg(total, agg);
    const fresh = agg.in + agg.out + agg.cw;
    return [date, wf, conv, String(count), fmtInt(fresh), fmtInt(fresh + agg.cr), fmtUSD(agg.cost)];
  };
  const H = ["DATE", "WORKFLOW RUN", "PARENT CONV", "AGENTS", "FRESH", "GRAND TOTAL", "EST USD"];
  const data = list.map((g) =>
    mkRow(fmtDate(g.mtime), g.wf, g.conv.slice(0, 8), g.agentCount, g.agg)
  );
  if (nonWf.agentCount > 0) {
    data.push(mkRow(fmtDate(nonWf.mtime), "(non-workflow agents)", "—", nonWf.agentCount, nonWf.agg));
  }
  const fresh = total.in + total.out + total.cw;
  data.push(["TOTAL", "", "", "", fmtInt(fresh), fmtInt(fresh + total.cr), fmtUSD(total.cost)]);
  renderGrid(H, data, new Set([0, 1, 2]), new Set([data.length - 1]));
  console.log(
    "\nFRESH = in+out+cache-write (the harness's subagent_tokens definition); GRAND TOTAL adds cache-read." +
      "\nNote: a /wave:live's per-feature /wave:builder is NOT a wf_* run — /wave:live runs /wave:builder in the main session, so its" +
      "\nagents land in the non-workflow row. Total a /wave:builder or /wave:live feature with --filter <label>, not" +
      "\n--by-workflow. --by-workflow captures Workflow-engine runs (e.g. /rr) exactly."
  );
}

function toJSON(rows) {
  return rows
    .map((r) => ({
      id: r.id,
      label: r.label,
      model: modelShort(r.agg.models),
      calls: r.agg.calls,
      input_tokens: r.agg.in,
      output_tokens: r.agg.out,
      cache_creation: r.agg.cw,
      cache_read: r.agg.cr,
      est_cost_usd: Number(r.agg.cost.toFixed(6)),
    }))
    .sort((a, b) => b.est_cost_usd - a.est_cost_usd);
}

// ─── MAIN ────────────────────────────────────────────────────────────────────────
const HELP = `token-ledger — per-agent token attribution from Claude Code JSONL transcripts

Usage: node token-ledger.mjs [options]
  (default)              most recent session for the current project (cwd-derived)
  --all                  every session for the current project
  --session <id|path>    a specific conversationId or a path to a project dir / main .jsonl
  --project <slug>       project slug override (default: slugified cwd)
  --root <dir>           extra transcript root (repeatable)
  --detail <id|substr>   list one agent's individual API calls in order
  --by-workflow          group by workflow run (wf_*) instead of by agent — one row
                         per run + a "(non-workflow agents)" summary row + TOTAL.
                         Captures Workflow-engine runs (e.g. /rr) exactly. NOTE: a
                         /wave:live's per-feature /wave:builder is NOT a wf_* run (it runs in the
                         main session); total a /wave:builder or /wave:live feature with --filter.
  --filter <substr>      restrict the per-agent table + totals to rows whose label or
                         model id contains <substr> (case-insensitive); prints match
                         count. Composes with --all / --session / --json.
  --json                 machine-readable output
  -h, --help             this help

Discovery roots (auto): ~/.claude  and  ~/.claude-sessions/*/`;

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    console.log(HELP);
    return;
  }

  const roots = [...defaultRoots(), ...args.roots];
  const slug = args.project || projectSlug(process.cwd());
  const projectDirs = findProjectDirs(roots, slug);

  if (projectDirs.length === 0) {
    console.error(`No transcript dirs found for project slug "${slug}" under roots:\n  ${roots.join("\n  ")}`);
    process.exit(1);
  }

  let sessions = listSessions(projectDirs);

  // DEDUP across roots: ~/.claude and ~/.claude-sessions may be HARDLINKS to the same
  // inodes for shared conversations. Collapse by real inode of the main file so we
  // never double-count the same session discovered under two roots.
  const byInode = new Map();
  for (const s of sessions) {
    let key = s.mainFile;
    try {
      const st = fs.statSync(s.mainFile);
      key = `${st.dev}:${st.ino}`;
    } catch {}
    // keep the one whose subagents dir actually exists, else the first seen
    const prev = byInode.get(key);
    if (!prev || (!prev.subagentsDir && s.subagentsDir)) byInode.set(key, s);
  }
  sessions = [...byInode.values()];

  // Scope selection.
  let scope = [];
  if (args.session) {
    const sel = args.session;
    scope = sessions.filter(
      (s) => s.conversationId === sel || s.mainFile.includes(sel)
    );
    if (scope.length === 0) {
      // Maybe a direct path to a project dir or main file.
      if (fs.existsSync(sel)) {
        let dir = sel,
          id = null;
        if (sel.endsWith(".jsonl")) {
          id = path.basename(sel, ".jsonl");
          dir = path.dirname(sel);
        }
        scope = listSessions([dir]).filter((s) => !id || s.conversationId === id);
      }
    }
    if (scope.length === 0) {
      console.error(`No session matched "${sel}".`);
      process.exit(1);
    }
  } else if (args.all) {
    scope = sessions;
  } else {
    // Default: the single most recent session that actually has sub-agents.
    const withAgents = sessions.filter((s) => s.subagentsDir);
    const pool = withAgents.length ? withAgents : sessions;
    pool.sort((a, b) => b.mtime - a.mtime);
    scope = pool.slice(0, 1);
  }

  // Build ledger across scope, merging rows by id when --all spans sessions.
  const allRows = [];
  let malformed = 0;
  for (const s of scope) {
    const { rows, totalMalformed } = await buildLedger(s);
    malformed += totalMalformed;
    allRows.push(...rows);
  }

  // --filter restricts the per-agent table + its totals (and json/detail) to rows
  // whose label OR model id contains the substring (case-insensitive).
  let viewRows = allRows;
  let matched = null;
  if (args.filter) {
    const q = args.filter.toLowerCase();
    viewRows = allRows.filter(
      (r) =>
        r.label.toLowerCase().includes(q) ||
        [...r.agg.models].some((m) => String(m).toLowerCase().includes(q))
    );
    matched = viewRows.length;
  }

  if (args.byWorkflow) {
    const scopeDesc = args.all ? `${scope.length} sessions` : `${scope.length} session(s)`;
    console.log(`token-ledger · by-workflow · project "${slug}" · scope: ${scopeDesc}\n`);
    printByWorkflow(viewRows);
  } else if (args.detail) {
    printDetail(viewRows, args.detail);
  } else if (args.json) {
    console.log(JSON.stringify({ rows: toJSON(viewRows), malformed_lines: malformed }, null, 2));
  } else {
    const scopeDesc = args.all
      ? `${scope.length} sessions`
      : args.session
        ? `session ${scope.map((s) => s.conversationId.slice(0, 8)).join(", ")}`
        : `latest session ${scope[0]?.conversationId.slice(0, 8) || "?"}`;
    const filterDesc = matched !== null ? ` · filter "${args.filter}" matched ${matched} row(s)` : "";
    console.log(`token-ledger · project "${slug}" · scope: ${scopeDesc} · ${viewRows.length} rows${filterDesc}\n`);
    printTable(viewRows);
  }

  if (WARNINGS.length) {
    const unknownModels = WARNINGS.filter((w) => w.includes("unknown model"));
    console.error(`\n[${WARNINGS.length} warning(s)]`);
    for (const w of [...new Set(WARNINGS)].slice(0, 10)) console.error("  " + w);
  }
  if (malformed && !args.json) console.error(`\n[skipped ${malformed} malformed line(s)]`);
}

main().catch((e) => {
  console.error("token-ledger fatal:", e.stack || e.message);
  process.exit(1);
});
