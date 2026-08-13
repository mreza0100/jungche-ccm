import { basename, join } from "node:path";
import { chmodSync, existsSync } from "node:fs";
import { command, gitResult, linesOf, readLines, readText, requireDirectory, requireFile, uniqueSorted, writePrivate } from "./io.js";
import type {
  AnchorRow,
  Census,
  ConductKind,
  ConductLine,
  CoverageArtifact,
  CoverageLine,
  CoverageStatus,
  NormalizedVerdict,
  ParseResult,
  SeatVerdict,
  Verdict,
} from "./types.js";

const anchorPattern = /^- `([^`]+)` — (blob|tree) `([0-9a-f]{12})`$/;
export const legacyAnchorPattern = /^- `([^`]+)` — `git log -1`: `([0-9a-f]{12})` \(([0-9]{4}-[0-9]{2}-[0-9]{2})\); (blob|tree) `([0-9a-f]{12})`$/;
const mapPathPattern = /^maps\/[a-z0-9][a-z0-9-]*\.md$/;
const mapFilePattern = /^[a-z0-9][a-z0-9-]*\.md$/;
const lanePattern = /^[a-z0-9][a-z0-9-]*$/;

function failure<T>(...errors: readonly string[]): ParseResult<T> {
  return { ok: false, errors };
}

function coverageStatus(value: string): CoverageStatus | undefined {
  if (value === "READ" || value === "SKIP") return value;
  return undefined;
}

function conductKind(value: string): ConductKind | undefined {
  if (value === "technique" || value === "prior" || value === "baseline") return value;
  return undefined;
}

export function parseCoverage(text: string, transcriptCount: number): ParseResult<CoverageArtifact> {
  const rows = linesOf(text);
  const errors: string[] = [];
  if (transcriptCount <= 0) errors.push("coverage requires at least one supplied transcript");
  if (rows[rows.length - 1] !== "END-OF-RUN") errors.push("missing final END-OF-RUN");
  if (rows.filter((row) => row === "END-OF-RUN").length !== 1) errors.push("END-OF-RUN must occur exactly once");
  const coverage: CoverageLine[] = [];
  const conduct: ConductLine[] = [];
  rows.forEach((row, offset) => {
    const lineNumber = offset + 1;
    if (row === "END-OF-RUN") return;
    const fields = row.split("\t");
    if (fields[0] === "CONDUCT") {
      if (fields.length !== 4) {
        errors.push(`line ${lineNumber}: expected CONDUCT<TAB>kind<TAB>slug|NONE<TAB>reason`);
        return;
      }
      const kind = conductKind(fields[1] ?? "");
      if (kind === undefined) {
        errors.push(`line ${lineNumber}: conduct kind is not technique, prior, or baseline`);
        return;
      }
      const slug = fields[2] ?? "";
      const reason = fields[3] ?? "";
      if (slug === "" || reason === "") {
        errors.push(`line ${lineNumber}: conduct slug and reason are both required`);
        return;
      }
      if (slug !== "NONE" && !lanePattern.test(slug)) {
        errors.push(`line ${lineNumber}: conduct slug is not lowercase kebab or NONE`);
        return;
      }
      conduct.push({ kind: "conduct", conductKind: kind, slug, reason });
      return;
    }
    if (fields.length !== 3) {
      errors.push(`line ${lineNumber}: expected index<TAB>READ|SKIP<TAB>reason`);
      return;
    }
    const rawIndex = fields[0] ?? "";
    if (!/^[1-9][0-9]*$/.test(rawIndex)) {
      errors.push(`line ${lineNumber}: first field is not a transcript index`);
      return;
    }
    const index = Number(rawIndex);
    if (index > transcriptCount) {
      errors.push(`line ${lineNumber}: index ${index} exceeds the ${transcriptCount} supplied transcripts`);
      return;
    }
    const status = coverageStatus(fields[1] ?? "");
    if (status === undefined) {
      errors.push(`line ${lineNumber}: status is not READ or SKIP`);
      return;
    }
    const reason = fields[2] ?? "";
    if (reason === "") {
      errors.push(`line ${lineNumber}: reason is empty`);
      return;
    }
    coverage.push({ kind: "coverage", index, status, reason });
  });

  const byIndex = new Map<number, number>();
  for (const row of coverage) byIndex.set(row.index, (byIndex.get(row.index) ?? 0) + 1);
  const duplicates = [...byIndex.entries()].filter((entry) => entry[1] > 1).map((entry) => String(entry[0]));
  const missing: string[] = [];
  for (let index = 1; index <= transcriptCount; index += 1) {
    if (!byIndex.has(index)) missing.push(String(index));
  }
  if (duplicates.length > 0) errors.push(`DUPLICATE INDEXES:\n${duplicates.join("\n")}`);
  if (missing.length > 0) errors.push(`UNRULED INDEXES:\n${missing.join("\n")}`);

  const conductCounts = new Map<ConductKind, number>();
  for (const row of conduct) conductCounts.set(row.conductKind, (conductCounts.get(row.conductKind) ?? 0) + 1);
  for (const kind of ["technique", "prior", "baseline"] as const) {
    const count = conductCounts.get(kind) ?? 0;
    if (count === 0) errors.push(`missing CONDUCT accounting for: ${kind}`);
    else if (count !== 1) errors.push(`CONDUCT accounting must occur exactly once for: ${kind}`);
  }
  return errors.length > 0 ? failure(...errors) : { ok: true, value: { coverage, conduct } };
}

export function renderExpandedCoverage(artifact: CoverageArtifact, paths: readonly string[]): string {
  return [...artifact.coverage]
    .sort((left, right) => left.index - right.index)
    .map((row) => `${paths[row.index - 1] ?? ""}\t${row.status}\t${row.reason}\n`)
    .join("");
}

export function anchorLookupPath(displayPath: string): string {
  const match = /^(.+):([0-9]+(?:-[0-9]+)?)$/.exec(displayPath);
  return match?.[1] ?? displayPath;
}

export function parseAnchorRow(row: string): ParseResult<AnchorRow> {
  const match = anchorPattern.exec(row);
  if (match === null) return failure("anchor row grammar mismatch");
  const displayPath = match[1] ?? "";
  const objectType = match[2];
  const hash = match[3] ?? "";
  if (objectType !== "blob" && objectType !== "tree") return failure("anchor object type grammar mismatch");
  if (displayPath.startsWith("/") || displayPath.startsWith("../") || displayPath.includes("/../") || displayPath.startsWith(".git/")) {
    return failure(`unsafe anchor path: ${displayPath}`);
  }
  return { ok: true, value: { displayPath, lookupPath: anchorLookupPath(displayPath), objectType, hash } };
}

export interface MapValidation {
  readonly anchors: readonly AnchorRow[];
  readonly title: string;
}

export function parseAndValidateMap(repo: string, mapFile: string): ParseResult<MapValidation> {
  const rows = readLines(mapFile);
  const first = rows[0];
  if (first === undefined) return failure("empty file");
  if (!/^#\s+[^#\s].+$/.test(first)) return failure("missing clean H1");
  const title = first.slice(2);
  if (/^(C:|L:|M:|MAP[-_: ])/u.test(title)) return failure("legacy title prefix");
  const headings = ["## Question", "## Answer", "## Derivation trail", "## Anchors"];
  for (const heading of headings) {
    if (rows.filter((row) => row === heading).length !== 1) return failure(`${heading.slice(3)} heading count is not one`);
  }
  if (rows.some((row) => row.startsWith("## ") && !headings.includes(row))) return failure("unexpected section heading");
  const provenanceRows = rows.filter((row) => row.startsWith("Provenance:"));
  if (provenanceRows.length !== 1) return failure("Provenance line count is not one");
  if (!/^Provenance: [0-9]{4}-[0-9]{2}-[0-9]{2} · sid [0-9a-f]{8}$/.test(provenanceRows[0] ?? "")) {
    return failure("Provenance grammar mismatch");
  }
  const questionIndex = rows.indexOf("## Question");
  const answerIndex = rows.indexOf("## Answer");
  const trailIndex = rows.indexOf("## Derivation trail");
  const provenanceIndex = rows.findIndex((row) => row.startsWith("Provenance:"));
  const anchorsIndex = rows.indexOf("## Anchors");
  if (!(questionIndex < answerIndex && answerIndex < trailIndex && trailIndex < provenanceIndex && provenanceIndex < anchorsIndex)) {
    return failure("section order mismatch");
  }
  if (rows.slice(1, questionIndex).some((row) => row.trim() !== "")) return failure("legacy preamble before Question");
  const hasBody = (start: number, end: number): boolean => rows.slice(start + 1, end).some((row) => row.trim() !== "");
  if (!hasBody(questionIndex, answerIndex) || !hasBody(answerIndex, trailIndex) || !hasBody(trailIndex, provenanceIndex)) {
    return failure("Question, Answer, or Derivation trail is empty");
  }
  const anchorRows: AnchorRow[] = [];
  for (const row of rows.slice(anchorsIndex + 1)) {
    if (row === "") continue;
    const parsed = parseAnchorRow(row);
    if (!parsed.ok) return parsed;
    const lookup = gitResult(repo, ["rev-parse", "--verify", "-q", `HEAD:${parsed.value.lookupPath}`]);
    if (lookup.status !== 0 || lookup.stdout.trim() === "") return failure(`anchor path absent at HEAD: ${parsed.value.lookupPath}`);
    const currentHash = lookup.stdout.trim();
    if (!currentHash.startsWith(parsed.value.hash)) {
      return failure(`anchor hash mismatch: ${parsed.value.lookupPath} expected=${parsed.value.hash} actual=${currentHash}`);
    }
    const typeResult = gitResult(repo, ["cat-file", "-t", currentHash]);
    if (typeResult.status !== 0 || typeResult.stdout.trim() === "") return failure(`anchor object unreadable: ${parsed.value.lookupPath}`);
    if (typeResult.stdout.trim() !== parsed.value.objectType) {
      return failure(`anchor object type mismatch: ${parsed.value.lookupPath} expected=${parsed.value.objectType} actual=${typeResult.stdout.trim()}`);
    }
    anchorRows.push(parsed.value);
  }
  if (anchorRows.length < 2 || anchorRows.length > 8) return failure(`anchor count outside 2-8: ${anchorRows.length}`);
  return { ok: true, value: { anchors: anchorRows, title } };
}

export interface AnchorGateResult {
  readonly accepted: readonly string[];
  readonly rejected: readonly { readonly mapPath: string; readonly reason: string }[];
}

export function gateAnchorMaps(repo: string, mapsDirectory: string, resultsPath: string, survivorsPath: string): AnchorGateResult {
  requireDirectory(repo);
  requireDirectory(mapsDirectory);
  const head = gitResult(repo, ["rev-parse", "--verify", "HEAD"]);
  if (head.status !== 0) throw new Error(`repository has no HEAD: ${repo}`);
  const accepted: string[] = [];
  const rejected: { mapPath: string; reason: string }[] = [];
  for (const mapFile of readdirMapFiles(mapsDirectory)) {
    const fileName = basename(mapFile);
    const mapPath = `maps/${fileName}`;
    if (!mapFilePattern.test(fileName) || fileName.includes("--")) {
      rejected.push({ mapPath, reason: "invalid map filename" });
      continue;
    }
    const parsed = parseAndValidateMap(repo, mapFile);
    if (parsed.ok) accepted.push(mapPath);
    else rejected.push({ mapPath, reason: parsed.errors.join("; ") });
  }
  const sortedAccepted = uniqueSorted(accepted);
  const resultRows = [
    ...sortedAccepted.map((mapPath) => `ACCEPT\t${mapPath}\tcanonical map and live anchors\n`),
    ...rejected.map((row) => `REJECT\t${row.mapPath}\t${row.reason}\n`),
  ].join("");
  writePrivate(resultsPath, resultRows);
  writePrivate(survivorsPath, sortedAccepted.map((row) => `${row}\n`).join(""));
  return { accepted: sortedAccepted, rejected };
}

function readdirMapFiles(mapsDirectory: string): string[] {
  return command("find", [mapsDirectory, "-maxdepth", "1", "-type", "f", "-name", "*.md", "-print"]).stdout
    .split("\n")
    .filter((row) => row !== "")
    .sort((left, right) => left.localeCompare(right));
}

function seatVerdict(value: string): SeatVerdict | undefined {
  if (value === "CONFIRM" || value === "AMEND" || value === "REFUTE") return value;
  return undefined;
}

export function parseVerdicts(text: string): ParseResult<readonly Verdict[]> {
  const verdicts: Verdict[] = [];
  const errors: string[] = [];
  linesOf(text).forEach((row, offset) => {
    if (row === "") return;
    const fields = row.split("\t");
    const verdict = seatVerdict(fields[0] ?? "");
    const mapPath = fields[1] ?? "";
    const evidence = fields[2] ?? "";
    if (fields.length !== 3 || verdict === undefined || !mapPathPattern.test(mapPath) || evidence === "") {
      errors.push(`line ${offset + 1}: ${row}`);
      return;
    }
    verdicts.push({ verdict, mapPath, evidence });
  });
  return errors.length > 0 ? failure(...errors) : { ok: true, value: verdicts };
}

export function normalizeVerdicts(survivors: readonly string[], parsed: readonly Verdict[]): ParseResult<readonly NormalizedVerdict[]> {
  const expected = uniqueSorted(survivors);
  const expectedSet = new Set(expected);
  const counts = new Map<string, number>();
  for (const verdict of parsed) counts.set(verdict.mapPath, (counts.get(verdict.mapPath) ?? 0) + 1);
  const duplicates = [...counts.entries()].filter((entry) => entry[1] > 1).map((entry) => entry[0]);
  const unknown = parsed.map((row) => row.mapPath).filter((path) => !expectedSet.has(path));
  const errors: string[] = [];
  if (duplicates.length > 0) errors.push(`DUPLICATE MAPS:\n${duplicates.join("\n")}`);
  if (unknown.length > 0) errors.push(`UNKNOWN MAPS:\n${uniqueSorted(unknown).join("\n")}`);
  if (errors.length > 0) return failure(...errors);
  const byMap = new Map(parsed.map((row) => [row.mapPath, row]));
  const normalized = expected.map((mapPath): NormalizedVerdict => {
    const row = byMap.get(mapPath);
    return row === undefined
      ? { verdict: "UNRULED", mapPath, evidence: "no verifier verdict; not applied" }
      : row;
  });
  return { ok: true, value: normalized };
}

export function renderNormalizedVerdicts(rows: readonly NormalizedVerdict[]): string {
  return rows.map((row) => `${row.verdict}\t${row.mapPath}\t${row.evidence}\n`).join("");
}

const censusKeys = [
  "window-meta-count",
  "agent-meta-count",
  "paired-transcript-count",
  "selected-paired-transcript-count",
  "omitted-paired-transcript-count",
  "coverage-gap-count",
  "excluded-other-agent-or-invalid-count",
  "invalid-meta-count",
] as const;

export function parseCensus(text: string): ParseResult<Census> {
  const values = new Map<string, number>();
  const errors: string[] = [];
  linesOf(text).forEach((row, offset) => {
    const fields = row.split("\t");
    if (fields.length !== 2 || !/^(0|[1-9][0-9]*)$/.test(fields[1] ?? "")) {
      errors.push(`line ${offset + 1}: invalid census row`);
      return;
    }
    const key = fields[0] ?? "";
    if (!censusKeys.includes(key)) errors.push(`line ${offset + 1}: unknown census key ${key}`);
    else if (values.has(key)) errors.push(`line ${offset + 1}: duplicate census key ${key}`);
    else values.set(key, Number(fields[1]));
  });
  for (const key of censusKeys) if (!values.has(key)) errors.push(`missing census key: ${key}`);
  if (errors.length > 0) return failure(...errors);
  return {
    ok: true,
    value: {
      windowMetaCount: values.get("window-meta-count") ?? 0,
      agentMetaCount: values.get("agent-meta-count") ?? 0,
      pairedTranscriptCount: values.get("paired-transcript-count") ?? 0,
      selectedPairedTranscriptCount: values.get("selected-paired-transcript-count") ?? 0,
      omittedPairedTranscriptCount: values.get("omitted-paired-transcript-count") ?? 0,
      coverageGapCount: values.get("coverage-gap-count") ?? 0,
      excludedOtherAgentOrInvalidCount: values.get("excluded-other-agent-or-invalid-count") ?? 0,
      invalidMetaCount: values.get("invalid-meta-count") ?? 0,
    },
  };
}

export function renderCensus(census: Census): string {
  return [
    ["window-meta-count", census.windowMetaCount],
    ["agent-meta-count", census.agentMetaCount],
    ["paired-transcript-count", census.pairedTranscriptCount],
    ["selected-paired-transcript-count", census.selectedPairedTranscriptCount],
    ["omitted-paired-transcript-count", census.omittedPairedTranscriptCount],
    ["coverage-gap-count", census.coverageGapCount],
    ["excluded-other-agent-or-invalid-count", census.excludedOtherAgentOrInvalidCount],
    ["invalid-meta-count", census.invalidMetaCount],
  ].map(([key, value]) => `${key}\t${value}\n`).join("");
}

export function readLaneMembership(path: string): ParseResult<ReadonlyMap<string, string>> {
  if (existsSync(path)) {
    const rows = new Map<string, string>();
    const errors: string[] = [];
    readLines(path).forEach((line, offset) => {
      const fields = line.split("\t");
      const map = fields[0] ?? "";
      const lane = fields[1] ?? "";
      if (fields.length !== 2 || !mapFilePattern.test(map) || !lanePattern.test(lane)) errors.push(`line ${offset + 1}: invalid lane row`);
      else if (rows.has(map)) errors.push(`line ${offset + 1}: duplicate lane row for ${map}`);
      else rows.set(map, lane);
    });
    return errors.length > 0 ? failure(...errors) : { ok: true, value: rows };
  }
  return { ok: true, value: new Map() };
}

export function chmodArtifact(path: string): void {
  chmodSync(path, 0o600);
}
