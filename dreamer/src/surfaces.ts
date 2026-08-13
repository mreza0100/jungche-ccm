import { basename, join } from "node:path";
import { existsSync } from "node:fs";
import { fail } from "./errors.js";
import { listFiles, readLines, readText, requireDirectory, requireFile, uniqueSorted, writePrivate } from "./io.js";
import { readLaneMembership } from "./artifacts.js";

export interface RenderedSurfaces {
  readonly stm: string;
  readonly agents: ReadonlyMap<string, string>;
}

export function extractMapTitle(mapFile: string): string {
  const first = readLines(mapFile)[0];
  if (first === undefined || !first.startsWith("# ")) fail(`map lacks H1 during surface render: ${mapFile}`);
  const title = first.slice(2);
  if (title.includes("\t") || title.includes(" -> maps/")) fail(`unsafe map title during surface render: ${mapFile}`);
  return title;
}

export function extractMapQuestion(mapFile: string): string | undefined {
  const rows = readLines(mapFile);
  const heading = rows.indexOf("## Question");
  if (heading < 0) return undefined;
  return rows.slice(heading + 1).find((row) => row.trim() !== "")?.replaceAll("\t", " ").trim();
}

export function cachedTitles(mapsDirectory: string): string[] {
  return uniqueSorted(listFiles(mapsDirectory, ".md").map((mapFile) => {
    const title = extractMapTitle(mapFile);
    const question = extractMapQuestion(mapFile);
    return question === undefined ? title : `${title} — ${question}`;
  }));
}

export function renderSurfaces(mapsDirectory: string, oldStmPath: string, lanesPath: string | undefined): RenderedSurfaces {
  requireDirectory(mapsDirectory);
  const parsedMembership = lanesPath === undefined ? { ok: true as const, value: new Map<string, string>() } : readLaneMembership(lanesPath);
  if (!parsedMembership.ok) fail(`invalid lane membership: ${parsedMembership.errors.join("; ")}`);
  const legacy = lanesPath === undefined;
  const titleRows: { lane: string; title: string; surface: string }[] = [];
  for (const mapFile of listFiles(mapsDirectory, ".md")) {
    const slug = basename(mapFile);
    const lane = legacy ? "explorer" : parsedMembership.value.get(slug);
    if (lane === undefined) fail(`map carries no lane row: ${slug}`);
    const title = extractMapTitle(mapFile);
    titleRows.push({ lane, title, surface: `- ${title} -> maps/${slug}` });
  }
  const duplicateTitles = titleRows
    .map((row) => row.title)
    .sort((left, right) => left.localeCompare(right))
    .filter((title, index, rows) => index > 0 && title === rows[index - 1]);
  if (duplicateTitles.length > 0) fail("duplicate map titles prevent deterministic surface generation");

  const agents = new Map<string, string>();
  for (const lane of uniqueSorted(titleRows.map((row) => row.lane))) {
    const body = titleRows
      .filter((row) => row.lane === lane)
      .sort((left, right) => left.title.localeCompare(right.title))
      .map((row) => `${row.surface}\n`)
      .join("");
    agents.set(lane, body);
  }
  const retained = existsSync(oldStmPath)
    ? readLines(oldStmPath).filter((row) => row.startsWith("- ") && !/ -> maps\/[a-z0-9][a-z0-9-]*\.md$/.test(row))
    : [];
  const stmRows = titleRows
    .sort((left, right) => left.title.localeCompare(right.title))
    .map((row) => row.surface);
  const stm = ["# Index of maps/ — stale content: edit the map file directly.", ...stmRows, ...retained].join("\n") + "\n";
  return { stm, agents };
}

export function writeRenderedSurfaces(rendered: RenderedSurfaces, stmPath: string, agentsDirectory: string): void {
  requireDirectory(agentsDirectory);
  writePrivate(stmPath, rendered.stm);
  for (const [lane, body] of rendered.agents) writePrivate(join(agentsDirectory, `${lane}.md`), body);
}

export function validSurface(path: string): string | undefined {
  if (!existsSync(path)) return undefined;
  const body = readText(path);
  if (body === "") return undefined;
  const rows = readLines(path);
  if (rows.length === 0 || rows.some((row) => !/^- [^\s].* -> maps\/[a-z0-9][a-z0-9-]*\.md$/.test(row))) return undefined;
  return rows.join("\n");
}
