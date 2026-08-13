import { basename, join } from "node:path";
import { existsSync } from "node:fs";
import { anchorLookupPath, parseAnchorRow } from "./artifacts.js";
import { command, gitResult, linesOf, listFiles, readLines, readText } from "./io.js";
import { validSurface } from "./surfaces.js";

interface ClaudeHookInput {
  readonly toolInput: Readonly<Record<string, unknown>>;
  readonly subagentType: string;
  readonly prompt: string;
}

interface CodexHookInput {
  readonly agentType: string;
  readonly cwd: string;
}

function objectValue(value: unknown): Readonly<Record<string, unknown>> | undefined {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return undefined;
  return Object.fromEntries(Object.entries(value));
}

function stringField(object: Readonly<Record<string, unknown>>, key: string): string | undefined {
  const value = object[key];
  return typeof value === "string" && value !== "" ? value : undefined;
}

export function parseClaudeHookInput(text: string): ClaudeHookInput | undefined {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return undefined;
  }
  const root = objectValue(parsed);
  const toolInput = root === undefined ? undefined : objectValue(root.tool_input);
  if (toolInput === undefined) return undefined;
  const subagentType = stringField(toolInput, "subagent_type");
  const prompt = stringField(toolInput, "prompt");
  if (subagentType === undefined || prompt === undefined) return undefined;
  return { toolInput, subagentType, prompt };
}

export function parseCodexHookInput(text: string): CodexHookInput | undefined {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return undefined;
  }
  const root = objectValue(parsed);
  if (root === undefined) return undefined;
  const agentType = stringField(root, "agent_type");
  const cwd = stringField(root, "cwd");
  if (agentType === undefined || cwd === undefined) return undefined;
  return { agentType, cwd };
}

export function laneSlug(agentType: string): string | undefined {
  if (/\s/.test(agentType) || agentType === "") return undefined;
  if (agentType === "Explore") return "explorer";
  const slug = agentType.toLowerCase().replace(/[^a-z0-9-]+/g, "-").replace(/-+$/, "");
  return /^[a-z0-9][a-z0-9-]*$/.test(slug) ? slug : undefined;
}

export function repositoryFromWorktree(path: string): string {
  const marker = "/.worktrees/";
  const index = path.indexOf(marker);
  return index < 0 ? path : path.slice(0, index);
}

function driftCounts(repo: string, organ: string): ReadonlyMap<string, { moved: number; total: number }> | undefined {
  if (gitResult(repo, ["rev-parse", "--verify", "-q", "HEAD"]).status !== 0) return undefined;
  const counts = new Map<string, { moved: number; total: number }>();
  for (const mapFile of listFiles(join(organ, "maps"), ".md")) {
    const rows = readLines(mapFile);
    const anchorHeading = rows.indexOf("## Anchors");
    if (anchorHeading < 0) return undefined;
    const rawAnchors = rows.slice(anchorHeading + 1).filter((row) => row !== "");
    if (rawAnchors.length === 0) return undefined;
    let moved = 0;
    for (const raw of rawAnchors) {
      const parsed = parseAnchorRow(raw);
      if (!parsed.ok) return undefined;
      const lookup = gitResult(repo, ["rev-parse", "--verify", "-q", `HEAD:${anchorLookupPath(parsed.value.displayPath)}`]);
      if (lookup.status === 1 || (lookup.status === 0 && !lookup.stdout.trim().startsWith(parsed.value.hash))) moved += 1;
      else if (lookup.status !== 0) return undefined;
    }
    counts.set(basename(mapFile, ".md"), { moved, total: rawAnchors.length });
  }
  return counts;
}

export function annotateSurface(repo: string, organ: string, surface: string): string {
  const counts = driftCounts(repo, organ);
  if (counts === undefined) return surface;
  return linesOf(surface).map((row) => {
    const pointer = row.slice(row.lastIndexOf(" -> ") + 4);
    const slug = basename(pointer, ".md");
    const count = counts.get(slug);
    return count !== undefined && count.moved > 0 ? `${row} ⚠ DRIFTED (${count.moved}/${count.total} anchors moved)` : row;
  }).join("\n");
}

export function runClaudeHook(inputText: string, projectDirectory: string | undefined): string {
  if (projectDirectory === undefined || projectDirectory === "") return "";
  const parsed = parseClaudeHookInput(inputText);
  if (parsed === undefined) return "";
  const repo = repositoryFromWorktree(projectDirectory);
  const organ = join(repo, ".professor", "stm");
  if (!existsSync(organ)) return "";
  const lane = laneSlug(parsed.subagentType);
  if (lane === undefined) return "";
  let index = join(organ, "agents", `${lane}.md`);
  if (lane === "explorer" && validSurface(index) === undefined) index = join(organ, "explorer-index.md");
  const surface = validSurface(index);
  if (surface === undefined) return "";
  const annotated = annotateSurface(repo, organ, surface);
  const updatedInput = { ...parsed.toolInput, prompt: `${parsed.prompt}\n\nCached maps for this repository (bodies under ${organ}/maps/). Consult a covering map before re-deriving its subject, cite it when used, and re-verify any row marked DRIFTED:\n${annotated}` };
  return JSON.stringify({ hookSpecificOutput: { hookEventName: "PreToolUse", updatedInput } });
}

export function runCodexHook(inputText: string): string {
  const parsed = parseCodexHookInput(inputText);
  if (parsed === undefined) return "";
  const lane = laneSlug(parsed.agentType);
  if (lane === undefined) return "";
  const repo = repositoryFromWorktree(parsed.cwd);
  const organ = join(repo, ".professor", "stm");
  const surface = validSurface(join(organ, "agents", `${lane}.md`));
  if (surface === undefined) return "";
  const header = `Cached maps for this repository (bodies under ${organ}/maps/). Consult a covering map before re-deriving its subject and cite it when used:`;
  return JSON.stringify({ hookSpecificOutput: { hookEventName: "SubagentStart", additionalContext: `${header}\n${surface}` } });
}

export function runNudge(projectDirectory: string | undefined, nowMilliseconds = Date.now()): string {
  if (projectDirectory === undefined || projectDirectory === "") return "";
  const repo = repositoryFromWorktree(projectDirectory);
  const organ = join(repo, ".professor", "stm");
  if (!existsSync(organ)) return "";
  const staging = join(organ, "dreamer", "staging");
  if (existsSync(staging)) {
    for (const stage of command("find", [staging, "-mindepth", "2", "-maxdepth", "2", "-type", "f", "-name", "FAILED", "-print"]).stdout.split("\n").filter(Boolean)) {
      const recordedRepoPath = join(stage.slice(0, -"/FAILED".length), "meta", "repo-root.txt");
      if (existsSync(recordedRepoPath) && readLines(recordedRepoPath)[0] === repo) {
        return `🌙 dreamer-night failed — inspect ${staging}/*/FAILED\n`;
      }
    }
  }
  const dreamer = join(organ, "dreamer");
  if (!existsSync(dreamer)) return "";
  const candidates = listFiles(dreamer, ".md").flatMap((file) => {
    const match = /^([0-9]{4}-[0-9]{2}-[0-9]{2})(?:-([0-9]+))?\.md$/.exec(basename(file));
    if (match === null || !readLines(file).includes("END-OF-SWEEP")) return [];
    return [{ file, date: match[1] ?? "", sequence: Number(match[2] ?? "1") }];
  }).sort((left, right) => left.date.localeCompare(right.date) || left.sequence - right.sequence);
  const latest = candidates[candidates.length - 1];
  if (latest === undefined) return "";
  const appliedRows = readLines(latest.file).filter((row) => row.startsWith("Applied: "));
  const rawApplied = appliedRows[appliedRows.length - 1]?.slice("Applied: ".length) ?? `${latest.date}T00:00:00`;
  const applied = Date.parse(rawApplied);
  if (!Number.isFinite(applied)) return "";
  const age = nowMilliseconds - applied;
  if (age <= 172_800_000) return "";
  return `🌙 dreamer-night stale — newest applied sweep is ${Math.floor(age / 86_400_000)}d old; run /dreamer\n`;
}

export async function readStdin(): Promise<string> {
  return new Promise((resolve) => {
    let body = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => { body += chunk; });
    process.stdin.on("end", () => resolve(body));
  });
}
