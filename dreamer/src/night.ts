import {
  chmodSync,
  closeSync,
  copyFileSync,
  existsSync,
  lstatSync,
  mkdirSync,
  mkdtempSync,
  openSync,
  readdirSync,
  realpathSync,
  renameSync,
  rmSync,
  statSync,
} from "node:fs";
import { basename, dirname, isAbsolute, join, relative, resolve, sep } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { DreamerError, errorMessage, fail } from "./errors.js";
import {
  anchorLookupPath,
  gateAnchorMaps,
  legacyAnchorPattern,
  normalizeVerdicts,
  parseAnchorRow,
  parseCoverage,
  parseVerdicts,
  renderCensus,
  renderExpandedCoverage,
  renderNormalizedVerdicts,
} from "./artifacts.js";
import {
  appendPrivate,
  canonicalDirectory,
  checkedCommand,
  command,
  copyDirectoryContents,
  countLines,
  dateToday,
  ensureDirectory,
  git,
  gitResult,
  isoNow,
  linesOf,
  listFiles,
  listFilesRecursive,
  mapFingerprint,
  readLines,
  readText,
  requireDirectory,
  requireFile,
  sha256File,
  uniqueSorted,
  writePrivate,
} from "./io.js";
import { cachedTitles, renderSurfaces, writeRenderedSurfaces } from "./surfaces.js";
import { laneSlug } from "./hooks.js";
import type {
  Census,
  HoldState,
  LaneContext,
  LaneProfile,
  NormalizedVerdict,
  RepoContext,
  StageLayout,
} from "./types.js";

const DEFAULT_REPO_ROOT = "/home/user/work/proja";
const DEFAULT_REGISTRY_BASE = "/home/user/.claude/projects";
export const DISTILL_MODEL = "gpt-5.6-luna";
export const REFINER_MODEL = "gpt-5.6-luna";
const DISTILL_EFFORT = "xhigh";
const REFINER_EFFORT = "xhigh";
const SEAT_TIMEOUT_MILLISECONDS = 2_700_000;
const SEAT_KILL_GRACE_MILLISECONDS = 30_000;

interface GlobalOptions {
  readonly repo: string;
  readonly agent: string;
  readonly bootstrapCount?: number;
  readonly corpusFile?: string;
  readonly command: string;
  readonly commandArguments: readonly string[];
}

interface RuntimeContext {
  readonly engine: string;
  readonly repo: RepoContext;
  readonly lane: LaneContext;
  readonly profile?: LaneProfile;
  readonly bootstrapCount?: number;
  readonly corpusFile?: string;
}

interface SweepCandidate {
  readonly path: string;
  readonly date: string;
  readonly sequence: number;
}

interface CorpusEnumeration {
  readonly paths: readonly string[];
  readonly census: Census;
  readonly cutoffDescription: string;
}

interface SeatResult {
  readonly status: number;
  readonly durationMilliseconds: number;
  readonly timedOut: boolean;
}

interface StructuredEvent {
  readonly phase: string;
  readonly timestamp: string;
  readonly [key: string]: unknown;
}

class RunLogger {
  public constructor(
    private readonly humanPath: string,
    private readonly structuredPath: string,
  ) {}

  public stdout(line: string): void {
    process.stdout.write(`${line}\n`);
    appendPrivate(this.humanPath, `${line}\n`);
  }

  public stderr(line: string): void {
    process.stderr.write(`${line}\n`);
    appendPrivate(this.humanPath, `${line}\n`);
  }

  public event(phase: string, details: Readonly<Record<string, unknown>> = {}): void {
    const event: StructuredEvent = { phase, timestamp: isoNow(), ...details };
    appendPrivate(this.structuredPath, `${JSON.stringify(event)}\n`);
  }
}

function parsePositiveInteger(value: string | undefined, message: string): number {
  if (value === undefined || !/^[1-9][0-9]*$/.test(value)) fail(message);
  return Number(value);
}

export function parseArguments(argumentsList: readonly string[]): GlobalOptions {
  let repo = DEFAULT_REPO_ROOT;
  let agent = "Explore";
  let bootstrapCount: number | undefined;
  let corpusFile: string | undefined;
  let index = 0;
  while (index < argumentsList.length) {
    const argument = argumentsList[index];
    if (argument === "--repo") {
      const value = argumentsList[index + 1];
      if (value === undefined) fail("--repo requires one absolute root");
      repo = value;
      index += 2;
    } else if (argument === "--agent") {
      const value = argumentsList[index + 1];
      if (value === undefined) fail("--agent requires one subagent type");
      agent = value;
      index += 2;
    } else if (argument === "--bootstrap-count") {
      bootstrapCount = parsePositiveInteger(argumentsList[index + 1], "--bootstrap-count requires one positive integer");
      index += 2;
    } else if (argument === "--corpus-file") {
      const value = argumentsList[index + 1];
      if (value === undefined) fail("--corpus-file requires one path");
      if (!isAbsolute(value)) fail("--corpus-file requires an absolute path");
      requireFile(value);
      corpusFile = value;
      index += 2;
    } else break;
  }
  if (bootstrapCount !== undefined && corpusFile !== undefined) fail("--bootstrap-count and --corpus-file are mutually exclusive");
  const commandName = argumentsList[index] ?? "run";
  const commandArguments = argumentsList.slice(index + 1);
  if ((bootstrapCount !== undefined || corpusFile !== undefined) && commandName !== "run" && commandName !== "supervise") {
    fail("--bootstrap-count and --corpus-file are valid only for run or supervise");
  }
  const result: {
    repo: string;
    agent: string;
    bootstrapCount?: number;
    corpusFile?: string;
    command: string;
    commandArguments: readonly string[];
  } = { repo, agent, command: commandName, commandArguments };
  if (bootstrapCount !== undefined) result.bootstrapCount = bootstrapCount;
  if (corpusFile !== undefined) result.corpusFile = corpusFile;
  return result;
}

function configureRepository(requested: string, registryBase = process.env.DREAMER_REGISTRY_BASE ?? DEFAULT_REGISTRY_BASE): RepoContext {
  if (!isAbsolute(requested)) fail(`repository root must be absolute: ${requested}`);
  let resolved: string;
  try {
    resolved = realpathSync(requested);
  } catch {
    fail(`repository root does not resolve: ${requested}`);
  }
  if (resolved !== requested) fail(`repository root must be canonical: ${requested}`);
  if (lstatSync(requested).isSymbolicLink()) fail(`repository root is a symlink: ${requested}`);
  return {
    repoRoot: requested,
    organ: join(requested, ".professor", "stm"),
    registry: join(registryBase, requested.replaceAll("/", "-")),
  };
}

function configureLane(requested: string): LaneContext {
  if (requested === "") fail("agent type must not be empty");
  if (/\s/.test(requested)) fail(`agent type must not contain whitespace: ${requested}`);
  const lane = laneSlug(requested);
  if (lane === undefined) fail(`agent type does not yield a lane slug: ${requested}`);
  return { agentType: requested, lane };
}

export function requireSeatLaw(): void {
  if (DISTILL_MODEL !== "gpt-5.6-luna") fail(`the dreamer distills on luna only; refusing DISTILL_MODEL=${DISTILL_MODEL}`);
  if (REFINER_MODEL !== "gpt-5.6-luna") fail(`the dreamer refines on luna only; refusing REFINER_MODEL=${REFINER_MODEL}`);
}

function requireCommands(includeCodex: boolean): void {
  const required = includeCodex ? ["codex", "find", "flock", "git"] : ["find", "flock", "git"];
  for (const name of required) {
    if (command("which", [name]).status !== 0) fail(`required command unavailable: ${name}`);
  }
}

function requireRepoContext(repo: RepoContext): void {
  requireDirectory(repo.repoRoot);
  const head = gitResult(repo.repoRoot, ["rev-parse", "--verify", "HEAD"]);
  if (head.status !== 0) fail(`repository has no HEAD: ${repo.repoRoot}`);
  const top = git(repo.repoRoot, ["rev-parse", "--show-toplevel"]);
  if (top !== repo.repoRoot) fail(`repository root is not the Git top level: ${repo.repoRoot}`);
  requireDirectory(repo.organ);
  requireDirectory(join(repo.organ, "maps"));
  requireDirectory(join(repo.organ, "dreamer"));
  requireDirectory(join(repo.organ, "archive"));
  requireFile(join(repo.organ, "stm.md"));
  const organTopResult = gitResult(repo.organ, ["rev-parse", "--show-toplevel"]);
  if (organTopResult.status !== 0) fail(`organ is not inside a Git repository: ${repo.organ}`);
  const organTop = organTopResult.stdout.trim();
  const rel = relative(organTop, repo.organ);
  if (rel.startsWith(`..${sep}`) || rel === ".." || isAbsolute(rel)) fail(`organ escapes its repository root: ${repo.organ}`);
  requireDirectory(repo.registry);
}

function resolveLaneProfile(context: RuntimeContext): LaneProfile {
  const local = join(context.repo.organ, "lanes", `${context.lane.lane}.md`);
  const global = join(context.engine, "lanes", `${context.lane.lane}.md`);
  const path = existsSync(local) ? local : existsSync(global) ? global : undefined;
  if (path === undefined) fail(`lane has no profile: expected ${local} or ${global}`);
  return { agentType: context.lane.agentType, lane: context.lane.lane, path, body: readText(path) };
}

function runDate(format: string): string {
  const result = command("date", [`+${format}`]);
  if (result.status !== 0) fail("date command failed");
  return result.stdout.trim();
}

function newStage(context: RuntimeContext): StageLayout {
  const stagingRoot = join(context.repo.organ, "dreamer", "staging");
  const logsRoot = join(context.repo.organ, "dreamer", "logs");
  ensureDirectory(stagingRoot);
  ensureDirectory(logsRoot);
  const stamp = runDate("%Y%m%dT%H%M%S");
  const root = mkdtempSync(join(stagingRoot, `${context.lane.lane}-${stamp}.`));
  chmodSync(root, 0o700);
  const maps = join(root, "maps");
  const meta = join(root, "meta");
  ensureDirectory(maps);
  ensureDirectory(meta);
  const logStem = basename(root);
  const humanLog = join(logsRoot, `${logStem}.log`);
  const structuredLog = join(logsRoot, `${logStem}.jsonl`);
  writePrivate(humanLog, "");
  writePrivate(structuredLog, "");
  return {
    root,
    maps,
    meta,
    paths: join(root, "paths.txt"),
    pin: join(root, "paths.sha256"),
    coverage: join(root, "coverage.md"),
    verdicts: join(root, "verdicts.md"),
    normalizedVerdicts: join(root, "verdicts-normalized.tsv"),
    structuredLog,
    humanLog,
  };
}

function validateStagePath(context: RuntimeContext, stage: string): void {
  let resolved: string;
  try {
    resolved = realpathSync(stage);
  } catch {
    fail(`staging directory does not resolve: ${stage}`);
  }
  if (resolved !== stage) fail(`staging path must be canonical: ${stage}`);
  const root = join(context.repo.organ, "dreamer", "staging");
  const rel = relative(root, stage);
  if (rel === "" || rel.startsWith(`..${sep}`) || rel === ".." || isAbsolute(rel)) fail(`staging path is outside ${root}: ${stage}`);
  if (lstatSync(stage).isSymbolicLink()) fail(`staging directory is a symlink: ${stage}`);
  const stats = statSync(stage);
  if (process.getuid !== undefined && stats.uid !== process.getuid()) fail(`staging directory has the wrong owner: ${stage}`);
  if ((stats.mode & 0o777) !== 0o700) fail(`staging directory mode is not 0700: ${stage}`);
}

export function gatePin(pathsPath: string, pinPath: string): string {
  requireFile(pathsPath);
  requireFile(pinPath);
  const pinLines = readLines(pinPath);
  const expected = pinLines[0] ?? "";
  if (!/^[0-9a-f]{64}$/.test(expected)) fail(`path pin is not one SHA-256: ${pinPath}`);
  if (pinLines.length !== 1) fail(`path pin has extra lines: ${pinPath}`);
  const paths = readLines(pathsPath);
  if (paths.some((path) => !/^\/[^\u0000-\u001f\u007f]+$/.test(path))) fail(`paths file contains a blank, relative, or control-character path: ${pathsPath}`);
  const sorted = uniqueSorted(paths);
  if (paths.length !== sorted.length || paths.some((path, index) => path !== sorted[index])) fail(`paths file is not sorted and unique: ${pathsPath}`);
  const actual = sha256File(pathsPath);
  if (actual !== expected) fail(`path pin mismatch: expected ${expected}, got ${actual}`);
  return `PIN PASS ${actual} ${pathsPath}`;
}

export function gateCoverage(pathsPath: string, coveragePath: string, pinPath?: string): string {
  requireFile(pathsPath);
  requireFile(coveragePath);
  if (pinPath !== undefined) gatePin(pathsPath, pinPath);
  const paths = readLines(pathsPath);
  const parsed = parseCoverage(readText(coveragePath), paths.length);
  if (!parsed.ok) {
    process.stderr.write(`COVERAGE FAIL ${coveragePath}\n`);
    for (const error of parsed.errors) process.stderr.write(`${error}\n`);
    throw new DreamerError(`coverage gate rejected ${coveragePath}`);
  }
  writePrivate(`${coveragePath}.expanded`, renderExpandedCoverage(parsed.value, paths));
  return `COVERAGE PASS ${paths.length} paths`;
}

export function gateAnchors(repo: string, maps: string, results: string, survivors: string): string {
  const result = gateAnchorMaps(repo, maps, results, survivors);
  return `ANCHORS PASS accepted=${result.accepted.length} rejected=${result.rejected.length}`;
}

export function gateVerdicts(survivorsPath: string, verdictsPath: string, normalizedPath: string): string {
  requireFile(survivorsPath);
  requireFile(verdictsPath);
  const survivors = uniqueSorted(readLines(survivorsPath));
  const parsed = parseVerdicts(readText(verdictsPath));
  if (!parsed.ok) {
    process.stderr.write(`VERDICTS FAIL ${verdictsPath}\nMALFORMED:\n${parsed.errors.join("\n")}\n`);
    throw new DreamerError(`verdict gate rejected ${verdictsPath}`);
  }
  const normalized = normalizeVerdicts(survivors, parsed.value);
  if (!normalized.ok) {
    process.stderr.write(`VERDICTS FAIL ${verdictsPath}\n${normalized.errors.join("\n")}\n`);
    throw new DreamerError(`verdict gate rejected ${verdictsPath}`);
  }
  writePrivate(normalizedPath, renderNormalizedVerdicts(normalized.value));
  const unruled = normalized.value.filter((row) => row.verdict === "UNRULED").length;
  return `VERDICTS PASS ruled=${normalized.value.length - unruled} unruled=${unruled}`;
}

function sweepCandidates(organ: string): SweepCandidate[] {
  return listFiles(join(organ, "dreamer"), ".md").flatMap((path) => {
    const match = /^([0-9]{4}-[0-9]{2}-[0-9]{2})(?:-([0-9]+))?\.md$/.exec(basename(path));
    if (match === null || !readLines(path).includes("END-OF-SWEEP")) return [];
    return [{ path, date: match[1] ?? "", sequence: Number(match[2] ?? "1") }];
  });
}

function latestAppliedSweep(organ: string, lane: string): string | undefined {
  const candidates = sweepCandidates(organ).filter((candidate) => {
    const laneRow = readLines(candidate.path).find((row) => row.startsWith("lane\t"));
    const sweepLane = laneRow?.split("\t")[1] ?? "explorer";
    return sweepLane === lane;
  }).sort((left, right) => left.date.localeCompare(right.date) || left.sequence - right.sequence);
  return candidates[candidates.length - 1]?.path;
}

function validDate(value: string): boolean {
  return value !== "" && Number.isFinite(Date.parse(value));
}

function cutoffForLane(context: RuntimeContext): { cutoff: string; epoch: number; sourceRows: string[] } {
  const latest = latestAppliedSweep(context.repo.organ, context.lane.lane);
  if (latest === undefined) {
    return {
      cutoff: "7 days ago",
      epoch: Date.now() - 7 * 86_400_000,
      sourceRows: ["window-mode\tsweep-cutoff", "newest-applied-sweep\tNONE", "cutoff-source\tbootstrap"],
    };
  }
  const rows = readLines(latest);
  let cutoff = rows.filter((row) => row.startsWith("enumerated-at\t")).map((row) => row.slice("enumerated-at\t".length)).at(-1) ?? "";
  let source = "enumerated-at";
  if (!validDate(cutoff)) {
    cutoff = rows.filter((row) => row.startsWith("Applied: ")).map((row) => row.slice("Applied: ".length)).at(-1) ?? "";
    source = "Applied";
  }
  if (!validDate(cutoff)) {
    cutoff = `${basename(latest).slice(0, 10)} 00:00:00`;
    source = "filename-date";
  }
  const epoch = Date.parse(cutoff);
  if (!Number.isFinite(epoch)) fail(`cannot derive valid cutoff from sweep: ${latest}`);
  return {
    cutoff,
    epoch,
    sourceRows: ["window-mode\tsweep-cutoff", `newest-applied-sweep\t${basename(latest)}`, `cutoff-source\t${source}`],
  };
}

function parseAgentType(metaPath: string): string | undefined {
  let parsed: unknown;
  try {
    parsed = JSON.parse(readText(metaPath));
  } catch {
    return undefined;
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return undefined;
  const record = Object.fromEntries(Object.entries(parsed));
  return typeof record.agentType === "string" ? record.agentType : undefined;
}

function writeWindow(stage: StageLayout, rows: readonly string[]): void {
  writePrivate(join(stage.meta, "window.tsv"), rows.map((row) => `${row}\n`).join(""));
}

function enumerateCorpus(context: RuntimeContext, stage: StageLayout): CorpusEnumeration {
  const enumeratedAt = isoNow();
  if (context.corpusFile !== undefined) {
    const sourceRows = readLines(context.corpusFile);
    const listed = sourceRows.filter((row) => row !== "" && !row.startsWith("#"));
    for (const path of listed) {
      if (!isAbsolute(path)) fail(`corpus path is not absolute: ${path}`);
      if (!existsSync(path) || !statSync(path).isFile()) fail(`corpus path is not a readable file: ${path}`);
    }
    const paths = uniqueSorted(listed);
    writePrivate(stage.paths, paths.map((path) => `${path}\n`).join(""));
    writePrivate(join(stage.root, "gaps.tsv"), "");
    copyFileSync(context.corpusFile, join(stage.meta, "corpus-file.txt"));
    const census: Census = {
      windowMetaCount: listed.length,
      agentMetaCount: listed.length,
      pairedTranscriptCount: listed.length,
      selectedPairedTranscriptCount: paths.length,
      omittedPairedTranscriptCount: listed.length - paths.length,
      coverageGapCount: 0,
      excludedOtherAgentOrInvalidCount: 0,
      invalidMetaCount: 0,
    };
    writeWindow(stage, [
      "window-mode\tcorpus-file",
      `corpus-file\t${context.corpusFile}`,
      `corpus-file-sha256\t${sha256File(context.corpusFile)}`,
      `agent-type\t${context.lane.agentType}`,
      `lane\t${context.lane.lane}`,
      "cutoff-exclusive\tNONE",
      `enumerated-at\t${enumeratedAt}`,
    ]);
    writePrivate(join(stage.root, "census.tsv"), renderCensus(census));
    writePrivate(stage.pin, `${sha256File(stage.paths)}\n`);
    return { paths, census, cutoffDescription: `corpus-file ${context.corpusFile}` };
  }

  const bootstrap = context.bootstrapCount !== undefined;
  const cutoff = bootstrap
    ? { cutoff: "NONE", epoch: Number.NEGATIVE_INFINITY, sourceRows: ["window-mode\tbootstrap-count", `bootstrap-count\t${context.bootstrapCount ?? 0}`] }
    : cutoffForLane(context);
  writeWindow(stage, [
    ...cutoff.sourceRows,
    `agent-type\t${context.lane.agentType}`,
    `lane\t${context.lane.lane}`,
    `cutoff-exclusive\t${cutoff.cutoff}`,
    `enumerated-at\t${enumeratedAt}`,
  ]);

  const allMetas = listFilesRecursive(context.repo.registry, ".meta.json")
    .filter((path) => basename(path).startsWith("agent-"))
    .filter((path) => bootstrap || statSync(path).mtimeMs > cutoff.epoch);
  let agentMetaCount = 0;
  let pairedTranscriptCount = 0;
  let excluded = 0;
  let invalid = 0;
  const gaps: string[] = [];
  const pairs: { meta: string; transcript: string; mtime: number }[] = [];
  for (const meta of allMetas) {
    const agentType = parseAgentType(meta);
    if (agentType === undefined) {
      invalid += 1;
      excluded += 1;
      continue;
    }
    if (agentType !== context.lane.agentType) {
      excluded += 1;
      continue;
    }
    agentMetaCount += 1;
    const transcript = meta.slice(0, -".meta.json".length) + ".jsonl";
    if (existsSync(transcript) && statSync(transcript).isFile()) {
      pairedTranscriptCount += 1;
      if (meta.includes("\t") || meta.includes("\n") || transcript.includes("\t") || transcript.includes("\n")) {
        fail(`registry path cannot be represented safely: ${meta}`);
      }
      pairs.push({ meta, transcript, mtime: statSync(meta).mtimeMs });
    } else gaps.push(`META-PRESENT-TRANSCRIPT-MISSING\t${meta}\t${transcript}`);
  }
  const selectedPairs = bootstrap
    ? [...pairs].sort((left, right) => right.mtime - left.mtime || left.meta.localeCompare(right.meta)).slice(0, context.bootstrapCount)
    : pairs;
  if (bootstrap) {
    writePrivate(join(stage.meta, "bootstrap-selection.tsv"), selectedPairs.map((row) => `${row.mtime}\t${row.meta}\t${row.transcript}\n`).join(""));
  }
  const paths = uniqueSorted(selectedPairs.map((row) => row.transcript));
  const census: Census = {
    windowMetaCount: allMetas.length,
    agentMetaCount,
    pairedTranscriptCount,
    selectedPairedTranscriptCount: paths.length,
    omittedPairedTranscriptCount: pairedTranscriptCount - paths.length,
    coverageGapCount: uniqueSorted(gaps).length,
    excludedOtherAgentOrInvalidCount: excluded,
    invalidMetaCount: invalid,
  };
  writePrivate(stage.paths, paths.map((path) => `${path}\n`).join(""));
  writePrivate(join(stage.root, "gaps.tsv"), uniqueSorted(gaps).map((row) => `${row}\n`).join(""));
  writePrivate(join(stage.root, "census.tsv"), renderCensus(census));
  writePrivate(stage.pin, `${sha256File(stage.paths)}\n`);
  return {
    paths,
    census,
    cutoffDescription: bootstrap ? `bootstrap-count ${context.bootstrapCount ?? 0}` : cutoff.cutoff,
  };
}

function buildDistillBrief(context: RuntimeContext, stage: StageLayout, today: string, repoHead: string): void {
  if (context.profile === undefined) fail("lane profile not resolved");
  const template = readText(join(context.engine, "dreamer-distill.prompt.md"));
  const cached = readLines(join(stage.root, "cached-titles.txt"));
  const provenance = context.corpusFile === undefined
    ? ""
    : readLines(context.corpusFile).filter((row) => row.startsWith("#")).map((row) => row.replace(/^#\s?/, "")).join("\n");
  const indexedPaths = readLines(stage.paths).map((path, index) => `${index + 1}. ${path}`).join("\n") || "(none)";
  const body = [
    template.trimEnd(),
    "\n## Lane\n",
    context.profile.body.trimEnd(),
    "\n## Run context\n",
    `Agent type: \`${context.lane.agentType}\` (lane \`${context.lane.lane}\`)`,
    `Repository root: \`${context.repo.repoRoot}\``,
    `Repository tree: \`${repoHead}\``,
    `Staging root: \`${stage.root}\``,
    `Map output directory: \`${stage.maps}\``,
    `Coverage output: \`${stage.coverage}\``,
    `Run date: \`${today}\``,
    "\n### Cached map titles\n",
    cached.length === 0 ? "(none)" : cached.map((row) => `- ${row}`).join("\n"),
    provenance === "" ? "" : `\n### Corpus provenance\n\n${provenance}`,
    `\n### Transcript paths (coverage indices)\n\n${indexedPaths}`,
    `\nWrite only \`${stage.maps}/*.md\` and \`${stage.coverage}\`; finish coverage with \`END-OF-RUN\`.\n`,
  ].filter((part) => part !== "").join("\n");
  writePrivate(join(stage.root, "distill-brief.md"), body);
}

function buildRefinerBrief(context: RuntimeContext, stage: StageLayout, repoHead: string): void {
  if (context.profile === undefined) fail("lane profile not resolved");
  const template = readText(join(context.engine, "dreamer-refiner.prompt.md"));
  const cached = readLines(join(stage.root, "cached-titles.txt"));
  const survivors = readLines(join(stage.root, "anchor-survivors.txt"));
  const body = [
    template.trimEnd(),
    "\n## Lane\n",
    context.profile.body.trimEnd(),
    "\n## Run context\n",
    `Agent type: \`${context.lane.agentType}\` (lane \`${context.lane.lane}\`)`,
    `Repository root: \`${context.repo.repoRoot}\``,
    `Repository tree: \`${repoHead}\``,
    `Staging root: \`${stage.root}\``,
    `Verdict output: \`${stage.verdicts}\``,
    "\n### Existing map titles\n",
    cached.length === 0 ? "(none)" : cached.map((row) => `- ${row}`).join("\n"),
    "\n### Staged maps to rule\n",
    survivors.length === 0 ? "(none)" : survivors.map((row) => `- ${stage.root}/${row}`).join("\n"),
    `\nWrite only \`${stage.verdicts}\` and AMEND edits to the listed staged maps. Rule every listed map or leave it mechanically UNRULED.\n`,
  ].join("\n");
  writePrivate(join(stage.root, "refiner-brief.md"), body);
}

function chmodFilesRecursive(path: string): void {
  for (const entry of readdirSync(path, { withFileTypes: true })) {
    const target = join(path, entry.name);
    if (entry.isDirectory()) chmodFilesRecursive(target);
    else if (entry.isFile()) chmodSync(target, 0o600);
  }
}

async function runSeat(stage: StageLayout, model: string, effort: string, briefPath: string, logPath: string, lastMessagePath: string): Promise<SeatResult> {
  const inputFd = openSync(briefPath, "r");
  const logFd = openSync(logPath, "w", 0o600);
  const started = Date.now();
  let timedOut = false;
  const child = spawn("codex", [
    "exec",
    "--ignore-user-config",
    "--ephemeral",
    "--skip-git-repo-check",
    "--sandbox", "workspace-write",
    "--cd", stage.root,
    "--model", model,
    "--config", `model_reasoning_effort=\"${effort}\"`,
    "--output-last-message", lastMessagePath,
    "-",
  ], {
    cwd: stage.root,
    env: process.env,
    detached: true,
    stdio: [inputFd, logFd, logFd],
  });
  const terminateGroup = (signal: string): void => {
    if (child.pid === undefined) return;
    try {
      process.kill(-child.pid, signal);
    } catch {
      // The group may have already exited between the status check and signal.
    }
  };
  const onParentSignal = (): void => terminateGroup("SIGTERM");
  process.on("SIGINT", onParentSignal);
  process.on("SIGTERM", onParentSignal);
  const timeout = setTimeout(() => {
    timedOut = true;
    terminateGroup("SIGTERM");
  }, SEAT_TIMEOUT_MILLISECONDS);
  let hardKill: number | undefined;
  const result = await new Promise<SeatResult>((resolveSeat) => {
    child.once("error", () => resolveSeat({ status: 1, durationMilliseconds: Date.now() - started, timedOut }));
    child.once("exit", (code) => resolveSeat({ status: code ?? 1, durationMilliseconds: Date.now() - started, timedOut }));
    hardKill = setTimeout(() => {
      if (timedOut) terminateGroup("SIGKILL");
    }, SEAT_TIMEOUT_MILLISECONDS + SEAT_KILL_GRACE_MILLISECONDS);
  });
  clearTimeout(timeout);
  if (hardKill !== undefined) clearTimeout(hardKill);
  process.off("SIGINT", onParentSignal);
  process.off("SIGTERM", onParentSignal);
  closeSync(inputFd);
  closeSync(logFd);
  chmodSync(logPath, 0o600);
  return result;
}

function writeLaneMembership(context: RuntimeContext, mapsDirectory: string, existingPath: string, outputPath: string): void {
  const existingPresent = existsSync(existingPath);
  const existing = new Map<string, string>();
  if (existingPresent) {
    readLines(existingPath).forEach((row, offset) => {
      const fields = row.split("\t");
      const map = fields[0] ?? "";
      const lane = fields[1] ?? "";
      if (fields.length !== 2 || !/^[a-z0-9][a-z0-9-]*\.md$/.test(map) || !/^[a-z0-9][a-z0-9-]*$/.test(lane)) {
        fail(`invalid lane row ${offset + 1}: ${existingPath}`);
      }
      if (existing.has(map)) fail(`duplicate lane rows: ${existingPath}`);
      existing.set(map, lane);
    });
  }
  const rows: string[] = [];
  for (const mapFile of listFiles(mapsDirectory, ".md")) {
    const slug = basename(mapFile);
    let lane = existing.get(slug);
    if (lane === undefined) {
      if (existsSync(join(context.repo.organ, "maps", slug))) {
        if (existingPresent) fail(`pre-existing map carries no lane row: ${slug}`);
        lane = "explorer";
      } else lane = context.lane.lane;
    }
    if (!/^[a-z0-9][a-z0-9-]*$/.test(lane)) fail(`invalid lane for map: ${slug} -> ${lane}`);
    rows.push(`${slug}\t${lane}`);
  }
  writePrivate(outputPath, uniqueSorted(rows).map((row) => `${row}\n`).join(""));
}

function surfaceByteStability(organ: string): string {
  requireDirectory(join(organ, "maps"));
  requireFile(join(organ, "stm.md"));
  const root = join(organ, "dreamer", "staging");
  ensureDirectory(root);
  const testStage = mkdtempSync(join(root, "surface-test."));
  chmodSync(testStage, 0o700);
  const one = join(testStage, "one");
  const two = join(testStage, "two");
  ensureDirectory(one);
  ensureDirectory(two);
  ensureDirectory(join(one, "agents"));
  ensureDirectory(join(two, "agents"));
  const lanes = existsSync(join(organ, "lanes.tsv")) ? join(organ, "lanes.tsv") : undefined;
  const first = renderSurfaces(join(organ, "maps"), join(organ, "stm.md"), lanes);
  const second = renderSurfaces(join(organ, "maps"), join(organ, "stm.md"), lanes);
  writeRenderedSurfaces(first, join(one, "stm.md"), join(one, "agents"));
  writeRenderedSurfaces(second, join(two, "stm.md"), join(two, "agents"));
  if (first.stm !== second.stm || JSON.stringify([...first.agents]) !== JSON.stringify([...second.agents])) {
    fail("lane surfaces are not byte-stable");
  }
  return `SURFACES PASS byte-stable maps=${listFiles(join(organ, "maps"), ".md").length} lanes=${first.agents.size} artifacts=${testStage}`;
}

function restampMap(repoRoot: string, source: string, destination: string, today: string): void {
  let provenanceCount = 0;
  const rows = readLines(source).map((row) => {
    const anchor = parseAnchorRow(row);
    const legacy = legacyAnchorPattern.exec(row);
    if (anchor.ok) {
      const currentHash = git(repoRoot, ["rev-parse", "--verify", `HEAD:${anchor.value.lookupPath}`]);
      const currentType = git(repoRoot, ["cat-file", "-t", currentHash]);
      return `- \`${anchor.value.displayPath}\` — ${currentType} \`${currentHash.slice(0, 12)}\``;
    }
    if (legacy !== null) {
      const displayPath = legacy[1] ?? "";
      const currentHash = git(repoRoot, ["rev-parse", "--verify", `HEAD:${anchorLookupPath(displayPath)}`]);
      const currentType = git(repoRoot, ["cat-file", "-t", currentHash]);
      return `- \`${displayPath}\` — ${currentType} \`${currentHash.slice(0, 12)}\``;
    }
    const provenance = /^Provenance:\s*[0-9]{4}-[0-9]{2}-[0-9]{2}\s*·\s*sid\s*([0-9a-f]{8})$/.exec(row);
    if (provenance !== null) {
      provenanceCount += 1;
      return `Provenance: ${today} · sid ${provenance[1] ?? ""}`;
    }
    return row;
  });
  if (provenanceCount !== 1) fail(`restamp found ${provenanceCount} Provenance lines: ${source}`);
  writePrivate(destination, `${rows.join("\n")}\n`);
}

function migrateAnchors(organ: string): string {
  requireDirectory(join(organ, "maps"));
  let touched = 0;
  let translated = 0;
  const maps = listFiles(join(organ, "maps"), ".md");
  for (const map of maps) {
    let rowsTranslated = 0;
    const rows = readLines(map).map((row) => {
      const match = legacyAnchorPattern.exec(row);
      if (match === null) return row;
      rowsTranslated += 1;
      return `- \`${match[1] ?? ""}\` — ${match[4] ?? ""} \`${match[5] ?? ""}\``;
    });
    if (rowsTranslated > 0) {
      writePrivate(map, `${rows.join("\n")}\n`);
      touched += 1;
      translated += rowsTranslated;
    }
  }
  if (maps.some((map) => readText(map).includes("git log -1"))) fail(`legacy anchor rows survive migration in ${join(organ, "maps")}`);
  return `MIGRATE PASS organ=${organ} maps=${maps.length} rewritten=${touched} rows=${translated}`;
}

function archiveName(archiveDirectory: string, base: string, today: string): string {
  let candidate = `${today}-${base}`;
  let counter = 2;
  while (existsSync(join(archiveDirectory, candidate))) {
    candidate = `${today}-${counter}-${base}`;
    counter += 1;
  }
  return candidate;
}

function sweepName(dreamerDirectory: string, today: string): string {
  let candidate = `${today}.md`;
  let counter = 2;
  while (existsSync(join(dreamerDirectory, candidate))) {
    candidate = `${today}-${counter}.md`;
    counter += 1;
  }
  return candidate;
}

function normalizedVerdictsFromRaw(stage: StageLayout): readonly NormalizedVerdict[] {
  gateVerdicts(join(stage.root, "anchor-survivors.txt"), stage.verdicts, stage.normalizedVerdicts);
  const parsed = parseVerdicts(readText(stage.verdicts));
  if (!parsed.ok) fail(`verdicts changed during normalization: ${parsed.errors.join("; ")}`);
  const normalized = normalizeVerdicts(uniqueSorted(readLines(join(stage.root, "anchor-survivors.txt"))), parsed.value);
  if (!normalized.ok) fail(`verdict normalization failed: ${normalized.errors.join("; ")}`);
  return normalized.value;
}

interface ApplyPreparation {
  readonly root: string;
  readonly sweepTarget: string;
  readonly explorerArchive: string;
}

function prepareApply(context: RuntimeContext, stage: StageLayout): ApplyPreparation {
  validateStagePath(context, stage.root);
  requireFile(join(stage.root, "READY-FOR-APPLY"));
  requireFile(join(stage.meta, "repo-head.txt"));
  requireFile(join(stage.meta, "maps.sha256"));
  if (readLines(join(stage.meta, "repo-root.txt"))[0] !== context.repo.repoRoot) fail("staged repository root mismatch");
  if (readLines(join(stage.meta, "organ.txt"))[0] !== context.repo.organ) fail("staged organ mismatch");
  const stagedLane = readLines(join(stage.meta, "lane.txt"))[0];
  if (stagedLane !== context.lane.lane) fail(`staged lane mismatch: stage=${stagedLane ?? ""} invocation=${context.lane.lane}`);
  const today = readLines(join(stage.meta, "run-date.txt"))[0] ?? "";
  if (!/^[0-9]{4}-[0-9]{2}-[0-9]{2}$/.test(today)) fail("staged run date is invalid");
  gatePin(stage.paths, stage.pin);
  gateCoverage(stage.paths, stage.coverage, stage.pin);
  gateAnchors(context.repo.repoRoot, stage.maps, join(stage.root, "anchor-postrefine.tsv"), join(stage.root, "anchor-postrefine-survivors.txt"));
  const verdictRows = normalizedVerdictsFromRaw(stage);
  const repoHead = git(context.repo.repoRoot, ["rev-parse", "HEAD"]);
  const recordedHead = readLines(join(stage.meta, "repo-head.txt"))[0] ?? "";
  if (repoHead !== recordedHead) fail(`repository HEAD moved since verification: ${recordedHead} -> ${repoHead}`);
  const recordedMaps = readLines(join(stage.meta, "maps.sha256"))[0] ?? "";
  if (mapFingerprint(join(context.repo.organ, "maps")) !== recordedMaps) fail("organ maps changed since preflight; rerun the night");

  const prep = join(stage.root, "apply");
  if (existsSync(prep)) fail(`apply preparation already exists: ${prep}`);
  ensureDirectory(prep);
  for (const directory of ["maps", "refuted", "surfaces", "surfaces-second"]) ensureDirectory(join(prep, directory));
  copyDirectoryContents(join(context.repo.organ, "maps"), join(prep, "maps"));
  const applyCandidates: string[] = [];
  const archivePlan: string[] = [];
  const ops: string[] = [];
  const postSurvivors = new Set(readLines(join(stage.root, "anchor-postrefine-survivors.txt")));
  for (const row of verdictRows) {
    const source = join(stage.root, row.mapPath);
    if (row.verdict === "CONFIRM" || row.verdict === "AMEND") {
      if (!postSurvivors.has(row.mapPath)) {
        ops.push(`NOT-APPLIED\t${row.mapPath}\tpost-refine anchor rejection`);
        continue;
      }
      const target = join(context.repo.organ, row.mapPath);
      if (existsSync(target)) fail(`map target collision: ${target}`);
      restampMap(context.repo.repoRoot, source, join(prep, "maps", basename(row.mapPath)), today);
      applyCandidates.push(`${row.verdict}\t${row.mapPath}\t${row.evidence}`);
      ops.push(`APPLY-${row.verdict}\t${row.mapPath}\t${row.evidence}`);
    } else if (row.verdict === "REFUTE") {
      const archiveTarget = archiveName(join(context.repo.organ, "archive"), basename(row.mapPath), today);
      const body = `${readText(source).trimEnd()}\n\nVerdict: REFUTE — ${row.evidence}\n`;
      writePrivate(join(prep, "refuted", archiveTarget), body);
      archivePlan.push(`${archiveTarget}\t${row.mapPath}`);
      ops.push(`ARCHIVE-REFUTE\t${row.mapPath}\tarchive/${archiveTarget}`);
    } else ops.push(`NOT-APPLIED\t${row.mapPath}\tunruled`);
  }
  writePrivate(join(prep, "apply-candidates.tsv"), applyCandidates.map((row) => `${row}\n`).join(""));
  writePrivate(join(prep, "archive-plan.tsv"), archivePlan.map((row) => `${row}\n`).join(""));

  const lanesPath = join(prep, "lanes.tsv");
  writeLaneMembership(context, join(prep, "maps"), join(context.repo.organ, "lanes.tsv"), lanesPath);
  ensureDirectory(join(prep, "surfaces", "agents"));
  ensureDirectory(join(prep, "surfaces-second", "agents"));
  const first = renderSurfaces(join(prep, "maps"), join(context.repo.organ, "stm.md"), lanesPath);
  const second = renderSurfaces(join(prep, "maps"), join(context.repo.organ, "stm.md"), lanesPath);
  writeRenderedSurfaces(first, join(prep, "surfaces", "stm.md"), join(prep, "surfaces", "agents"));
  writeRenderedSurfaces(second, join(prep, "surfaces-second", "stm.md"), join(prep, "surfaces-second", "agents"));
  if (first.stm !== second.stm || JSON.stringify([...first.agents]) !== JSON.stringify([...second.agents])) fail("lane surfaces are not byte-stable");
  for (const [lane, surface] of first.agents) ops.push(`SURFACE\tagents/${lane}.md\t${linesOf(surface).length} map rows`);
  let explorerArchive = "NONE";
  if (existsSync(join(context.repo.organ, "explorer-index.md"))) {
    explorerArchive = archiveName(join(context.repo.organ, "archive"), "explorer-index.md", today);
    ops.push(`MIGRATE-SURFACE\texplorer-index.md\tarchive/${explorerArchive}`);
  }
  writePrivate(join(prep, "ops.tsv"), ops.map((row) => `${row}\n`).join(""));
  const sweepTarget = sweepName(join(context.repo.organ, "dreamer"), today);
  const sweep = [
    `# Dreamer sweep — ${today}\n`,
    "## Coverage\n",
    readText(join(stage.meta, "window.tsv")).trimEnd(),
    `paths-sha256\t${readLines(stage.pin)[0] ?? ""}`,
    readText(join(stage.root, "census.tsv")).trimEnd(),
    "\n### Paths\n\n```text",
    readText(stage.paths).trimEnd(),
    "```\n\n### Typed gaps\n\n```text",
    readText(join(stage.root, "gaps.tsv")).trimEnd(),
    "```\n\n### Coverage\n\n```text",
    readText(`${stage.coverage}.expanded`).trimEnd(),
    "```\n\n## Gate results\n\n### Distill anchor gate\n\n```text",
    readText(join(stage.root, "anchor-results.tsv")).trimEnd(),
    "```\n\n### Post-verify anchor gate\n\n```text",
    readText(join(stage.root, "anchor-postrefine.tsv")).trimEnd(),
    "```\n\n### Lane membership\n\n```text",
    readText(lanesPath).trimEnd(),
    "```\n\n## Verdicts\n\n```text",
    readText(stage.normalizedVerdicts).trimEnd(),
    "```\n\n## Ops\n\n```text",
    readText(join(prep, "ops.tsv")).trimEnd(),
    `\`\`\`\n\nEND-OF-SWEEP\nApplied: ${isoNow()}\n`,
  ].join("\n");
  writePrivate(join(prep, "sweep.md"), sweep);
  writePrivate(join(prep, "explorer-archive.txt"), `${explorerArchive}\n`);
  writePrivate(join(prep, "sweep-target.txt"), `${sweepTarget}\n`);
  return { root: prep, sweepTarget, explorerArchive };
}

function stageLayoutFromRoot(context: RuntimeContext, root: string): StageLayout {
  const logs = join(context.repo.organ, "dreamer", "logs");
  const stem = basename(root);
  return {
    root,
    maps: join(root, "maps"),
    meta: join(root, "meta"),
    paths: join(root, "paths.txt"),
    pin: join(root, "paths.sha256"),
    coverage: join(root, "coverage.md"),
    verdicts: join(root, "verdicts.md"),
    normalizedVerdicts: join(root, "verdicts-normalized.tsv"),
    humanLog: join(logs, `${stem}.log`),
    structuredLog: join(logs, `${stem}.jsonl`),
  };
}

function atomicSurfaceCopy(source: string, destination: string): void {
  const temporary = join(dirname(destination), `.${basename(destination)}.dreamer-night`);
  copyFileSync(source, temporary);
  chmodSync(temporary, 0o600);
  renameSync(temporary, destination);
}

function applyStage(context: RuntimeContext, root: string): string {
  const stage = stageLayoutFromRoot(context, root);
  const prep = prepareApply(context, stage);
  for (const directory of ["agents", "archive", "dreamer", "maps"]) ensureDirectory(join(context.repo.organ, directory));
  for (const map of listFiles(join(prep.root, "maps"), ".md")) {
    const destination = join(context.repo.organ, "maps", basename(map));
    if (!existsSync(destination)) copyFileSync(map, destination);
  }
  for (const refuted of listFiles(join(prep.root, "refuted"), ".md")) {
    copyFileSync(refuted, join(context.repo.organ, "archive", basename(refuted)));
  }
  if (prep.explorerArchive !== "NONE" && existsSync(join(context.repo.organ, "explorer-index.md"))) {
    renameSync(join(context.repo.organ, "explorer-index.md"), join(context.repo.organ, "archive", prep.explorerArchive));
  }
  for (const surface of listFiles(join(prep.root, "surfaces", "agents"), ".md")) {
    atomicSurfaceCopy(surface, join(context.repo.organ, "agents", basename(surface)));
  }
  atomicSurfaceCopy(join(prep.root, "lanes.tsv"), join(context.repo.organ, "lanes.tsv"));
  atomicSurfaceCopy(join(prep.root, "surfaces", "stm.md"), join(context.repo.organ, "stm.md"));
  copyFileSync(join(prep.root, "sweep.md"), join(context.repo.organ, "dreamer", prep.sweepTarget));
  chmodSync(join(context.repo.organ, "dreamer", prep.sweepTarget), 0o600);
  writePrivate(join(stage.root, "APPLIED"), `APPLIED\t${isoNow()}\n`);
  return `dreamer-night: APPLIED stage=${stage.root} sweep=${prep.sweepTarget} — organ files written, uncommitted by design`;
}

function stageMetadata(context: RuntimeContext, stage: StageLayout, today: string, repoHead: string): void {
  writePrivate(join(stage.meta, "repo-root.txt"), `${context.repo.repoRoot}\n`);
  writePrivate(join(stage.meta, "organ.txt"), `${context.repo.organ}\n`);
  writePrivate(join(stage.meta, "agent-type.txt"), `${context.lane.agentType}\n`);
  writePrivate(join(stage.meta, "lane.txt"), `${context.lane.lane}\n`);
  writePrivate(join(stage.meta, "repo-head.txt"), `${repoHead}\n`);
  writePrivate(join(stage.meta, "run-date.txt"), `${today}\n`);
  writePrivate(join(stage.meta, "maps.sha256"), `${mapFingerprint(join(context.repo.organ, "maps"))}\n`);
  writePrivate(join(stage.meta, "human-log.txt"), `${stage.humanLog}\n`);
  writePrivate(join(stage.meta, "structured-log.txt"), `${stage.structuredLog}\n`);
}

async function executeNight(contextBase: RuntimeContext, mode: "autonomous" | "supervise"): Promise<number> {
  requireSeatLaw();
  requireCommands(true);
  requireRepoContext(contextBase.repo);
  requireFile(join(contextBase.engine, "dreamer-distill.prompt.md"));
  requireFile(join(contextBase.engine, "dreamer-refiner.prompt.md"));
  const profile = resolveLaneProfile(contextBase);
  const context: RuntimeContext = { ...contextBase, profile };
  const stage = newStage(context);
  const logger = new RunLogger(stage.humanLog, stage.structuredLog);
  logger.event("start", { mode, repo: context.repo.repoRoot, lane: context.lane.lane, stage: stage.root });
  try {
    const today = dateToday();
    const repoHead = git(context.repo.repoRoot, ["rev-parse", "HEAD"]);
    stageMetadata(context, stage, today, repoHead);
    writePrivate(join(stage.root, "cached-titles.txt"), cachedTitles(join(context.repo.organ, "maps")).map((row) => `${row}\n`).join(""));
    const corpus = enumerateCorpus(context, stage);
    logger.event("corpus", { paths: corpus.paths.length, census: corpus.census });
    if (corpus.paths.length === 0) {
      validateStagePath(context, stage.root);
      rmSync(stage.root, { recursive: true, force: false });
      const line = `dreamer-night: EMPTY-WINDOW stage=${stage.root} (no ${context.lane.agentType} transcripts since ${corpus.cutoffDescription})`;
      process.stdout.write(`${line}\n`);
      appendPrivate(stage.humanLog, `${line}\n`);
      logger.event("exit", { reason: "EMPTY-WINDOW" });
      return 0;
    }
    const pinResult = gatePin(stage.paths, stage.pin);
    writePrivate(join(stage.root, "gate-pin.log"), `${pinResult}\n`);
    logger.event("gate", { gate: "PIN", verdict: "PASS", phaseAfter: "preflight" });
    buildDistillBrief(context, stage, today, repoHead);
    logger.stdout(`dreamer-night: PREFLIGHT stage=${stage.root} paths=${corpus.paths.length} gaps=${corpus.census.coverageGapCount}`);
    logger.event("seat-start", { seat: "distill", model: DISTILL_MODEL, effort: DISTILL_EFFORT });
    const distill = await runSeat(stage, DISTILL_MODEL, DISTILL_EFFORT, join(stage.root, "distill-brief.md"), join(stage.root, "distill-seat.log"), join(stage.root, "distill-last-message.txt"));
    logger.event("seat-end", { seat: "distill", status: distill.status, durationMilliseconds: distill.durationMilliseconds, timedOut: distill.timedOut });
    if (distill.status !== 0) {
      writePrivate(join(stage.root, "FAILED"), `DISTILL-FAILED\t${distill.status}\n`);
      fail(`distill seat failed once; artifacts preserved at ${stage.root}`);
    }
    chmodFilesRecursive(stage.root);
    const postDistillPin = gatePin(stage.paths, stage.pin);
    writePrivate(join(stage.root, "gate-pin-post-distill.log"), `${postDistillPin}\n`);
    logger.stdout(postDistillPin);
    logger.event("gate", { gate: "PIN", verdict: "PASS", phaseAfter: "distill" });
    const coverageResult = gateCoverage(stage.paths, stage.coverage, stage.pin);
    writePrivate(join(stage.root, "gate-coverage.log"), `${coverageResult}\n`);
    logger.stdout(coverageResult);
    logger.event("gate", { gate: "COVERAGE", verdict: "PASS" });
    const anchorsResult = gateAnchors(context.repo.repoRoot, stage.maps, join(stage.root, "anchor-results.tsv"), join(stage.root, "anchor-survivors.txt"));
    writePrivate(join(stage.root, "gate-anchors.log"), `${anchorsResult}\n`);
    logger.stdout(anchorsResult);
    logger.event("gate", { gate: "ANCHORS", verdict: "PASS", phaseAfter: "distill" });
    buildRefinerBrief(context, stage, repoHead);
    const distillSurvivors = readLines(join(stage.root, "anchor-survivors.txt"));
    if (distillSurvivors.length > 0) {
      logger.event("seat-start", { seat: "refiner", model: REFINER_MODEL, effort: REFINER_EFFORT });
      const refiner = await runSeat(stage, REFINER_MODEL, REFINER_EFFORT, join(stage.root, "refiner-brief.md"), join(stage.root, "refiner-seat.log"), join(stage.root, "verify-last-message.txt"));
      logger.event("seat-end", { seat: "refiner", status: refiner.status, durationMilliseconds: refiner.durationMilliseconds, timedOut: refiner.timedOut });
      if (refiner.status !== 0) {
        writePrivate(join(stage.root, "FAILED"), `VERIFY-FAILED\t${refiner.status}\n`);
        fail(`verify seat failed once; artifacts preserved at ${stage.root}`);
      }
    } else {
      writePrivate(stage.verdicts, "");
      writePrivate(join(stage.root, "refiner-seat.log"), "VERIFY SKIP zero anchor-valid staged maps\n");
      logger.event("seat-skip", { seat: "refiner", reason: "zero anchor-valid staged maps" });
    }
    chmodFilesRecursive(stage.root);
    const postRefinePin = gatePin(stage.paths, stage.pin);
    writePrivate(join(stage.root, "gate-pin-post-refine.log"), `${postRefinePin}\n`);
    logger.stdout(postRefinePin);
    logger.event("gate", { gate: "PIN", verdict: "PASS", phaseAfter: "refiner" });
    const postAnchors = gateAnchors(context.repo.repoRoot, stage.maps, join(stage.root, "anchor-postrefine.tsv"), join(stage.root, "anchor-postrefine-survivors.txt"));
    writePrivate(join(stage.root, "gate-anchors-postrefine.log"), `${postAnchors}\n`);
    logger.stdout(postAnchors);
    logger.event("gate", { gate: "ANCHORS", verdict: "PASS", phaseAfter: "refiner" });
    const verdictResult = gateVerdicts(join(stage.root, "anchor-survivors.txt"), stage.verdicts, stage.normalizedVerdicts);
    writePrivate(join(stage.root, "gate-verdicts.log"), `${verdictResult}\n`);
    logger.stdout(verdictResult);
    logger.event("gate", { gate: "VERDICTS", verdict: "PASS" });
    const postSurvivors = new Set(readLines(join(stage.root, "anchor-postrefine-survivors.txt")));
    const normalized = normalizedVerdictsFromRaw(stage);
    const yieldCount = normalized.filter((row) => (row.verdict === "CONFIRM" || row.verdict === "AMEND") && postSurvivors.has(row.mapPath)).length;
    const holdState: HoldState = postSurvivors.size === 0 ? "ZERO-SURVIVORS" : yieldCount === 0 ? "ZERO-YIELD" : "READY";
    writePrivate(join(stage.meta, "apply-yield.txt"), `${yieldCount}\n`);
    writePrivate(join(stage.root, "READY-FOR-APPLY"), `${holdState}\t${isoNow()}\n`);
    logger.event("hold", { state: holdState, survivors: postSurvivors.size, yield: yieldCount });
    if (mode === "supervise") {
      if (holdState === "ZERO-SURVIVORS") {
        logger.stdout(`dreamer-night: HOLD-BEFORE-APPLY ZERO-SURVIVORS stage=${stage.root}`);
        logger.stdout("dreamer-night: no signed apply command: zero anchor-valid staged maps");
      } else {
        const stateSegment = holdState === "ZERO-YIELD" ? " ZERO-YIELD" : "";
        logger.stdout(`dreamer-night: HOLD-BEFORE-APPLY${stateSegment} stage=${stage.root}`);
        logger.stdout(`dreamer-night: signed apply command: ${process.argv[1] ?? "dreamer-night"} --repo ${context.repo.repoRoot} --agent ${context.lane.agentType} apply ${stage.root}`);
      }
      logger.event("exit", { reason: holdState });
    } else {
      logger.stdout(applyStage(context, stage.root));
      logger.event("exit", { reason: "APPLIED" });
    }
    return 0;
  } catch (error) {
    if (existsSync(stage.root) && !existsSync(join(stage.root, "FAILED"))) writePrivate(join(stage.root, "FAILED"), `ENGINE-FAILED\t${errorMessage(error)}\n`);
    logger.event("exit", { reason: "FAILED", error: errorMessage(error) });
    throw error;
  }
}

function usage(): string {
  return `Usage:
  dreamer-night [--repo ROOT] [--agent TYPE] [--bootstrap-count N | --corpus-file FILE] [supervise]
  dreamer-night [--repo ROOT] [--agent TYPE] apply STAGE
  dreamer-night [--repo ROOT] [--agent TYPE] inspect-repo

  --agent TYPE selects the subagent type to harvest (default Explore) and its
  lane; the lane needs a profile at lanes/{lane}.md and owns
  agents/{lane}.md plus its own sweep window.
  dreamer-night gate-pin PATHS SHA256
  dreamer-night gate-coverage PATHS COVERAGE [SHA256]
  dreamer-night [--repo ROOT] gate-anchors MAPS RESULTS SURVIVORS
  dreamer-night gate-anchors REPO MAPS RESULTS SURVIVORS
  dreamer-night gate-verdicts SURVIVORS VERDICTS NORMALIZED
  dreamer-night test-surfaces ORGAN
  dreamer-night migrate-anchors ORGAN
  dreamer-night [--repo ROOT] [--agent TYPE] lane-membership MAPS EXISTING OUT
`;
}

function needsRunnerLock(commandName: string): boolean {
  return commandName === "run" || commandName === "supervise" || commandName === "apply";
}

function acquireRunnerLock(argumentsList: readonly string[]): number {
  const entry = process.argv[1] ?? fail("runner path unavailable");
  const env = { ...process.env, DREAMER_LOCK_HELD: "1" };
  const result = spawnSync("flock", ["-n", entry, process.execPath, entry, ...argumentsList], { env, stdio: "inherit" });
  if (result.error !== undefined) fail(`required command unavailable: flock`);
  if ((result.status ?? 1) !== 0 && (result.status ?? 1) === 1) process.stderr.write("dreamer-night: FAIL: another dreamer-night process holds the runner lock\n");
  return result.status ?? 1;
}

export async function nightMain(argumentsList: readonly string[]): Promise<number> {
  const options = parseArguments(argumentsList);
  if ((options.command === "run" || options.command === "supervise") && process.env.DREAMER_LOCK_HELD !== "1") requireSeatLaw();
  if (needsRunnerLock(options.command) && process.env.DREAMER_LOCK_HELD !== "1") return acquireRunnerLock(argumentsList);
  const engine = dirname(process.argv[1] ?? "");
  const repo = configureRepository(options.repo);
  const lane = configureLane(options.agent);
  const contextBase: {
    engine: string;
    repo: RepoContext;
    lane: LaneContext;
    bootstrapCount?: number;
    corpusFile?: string;
  } = { engine, repo, lane };
  if (options.bootstrapCount !== undefined) contextBase.bootstrapCount = options.bootstrapCount;
  if (options.corpusFile !== undefined) contextBase.corpusFile = options.corpusFile;
  const context: RuntimeContext = contextBase;
  switch (options.command) {
    case "run":
      if (options.commandArguments.length !== 0) fail("run accepts only global flags");
      return executeNight(context, "autonomous");
    case "supervise":
      if (options.commandArguments.length !== 0) fail("supervise accepts only global flags");
      return executeNight(context, "supervise");
    case "apply":
      if (options.commandArguments.length !== 1) fail("apply requires one staging path");
      requireCommands(false);
      requireRepoContext(repo);
      process.stdout.write(`${applyStage(context, options.commandArguments[0] ?? "")}\n`);
      return 0;
    case "inspect-repo":
      if (options.commandArguments.length !== 0) fail("inspect-repo accepts only global flags");
      requireCommands(false);
      requireRepoContext(repo);
      process.stdout.write(`REPO\t${repo.repoRoot}\nORGAN\t${repo.organ}\nREGISTRY\t${repo.registry}\n`);
      return 0;
    case "gate-pin":
      if (options.commandArguments.length !== 2) fail("gate-pin requires PATHS SHA256");
      process.stdout.write(`${gatePin(options.commandArguments[0] ?? "", options.commandArguments[1] ?? "")}\n`);
      return 0;
    case "gate-coverage":
      if (options.commandArguments.length !== 2 && options.commandArguments.length !== 3) fail("gate-coverage requires PATHS COVERAGE [SHA256]");
      process.stdout.write(`${gateCoverage(options.commandArguments[0] ?? "", options.commandArguments[1] ?? "", options.commandArguments[2])}\n`);
      return 0;
    case "gate-anchors":
      if (options.commandArguments.length === 3) {
        process.stdout.write(`${gateAnchors(repo.repoRoot, options.commandArguments[0] ?? "", options.commandArguments[1] ?? "", options.commandArguments[2] ?? "")}\n`);
      } else if (options.commandArguments.length === 4) {
        process.stdout.write(`${gateAnchors(options.commandArguments[0] ?? "", options.commandArguments[1] ?? "", options.commandArguments[2] ?? "", options.commandArguments[3] ?? "")}\n`);
      } else fail("gate-anchors requires MAPS RESULTS SURVIVORS (or legacy REPO MAPS RESULTS SURVIVORS)");
      return 0;
    case "gate-verdicts":
      if (options.commandArguments.length !== 3) fail("gate-verdicts requires SURVIVORS VERDICTS NORMALIZED");
      process.stdout.write(`${gateVerdicts(options.commandArguments[0] ?? "", options.commandArguments[1] ?? "", options.commandArguments[2] ?? "")}\n`);
      return 0;
    case "test-surfaces":
      if (options.commandArguments.length !== 1) fail("test-surfaces requires ORGAN");
      process.stdout.write(`${surfaceByteStability(options.commandArguments[0] ?? "")}\n`);
      return 0;
    case "migrate-anchors":
      if (options.commandArguments.length !== 1) fail("migrate-anchors requires ORGAN");
      process.stdout.write(`${migrateAnchors(options.commandArguments[0] ?? "")}\n`);
      return 0;
    case "lane-membership":
      if (options.commandArguments.length !== 3) fail("lane-membership requires MAPS EXISTING OUT");
      requireRepoContext(repo);
      writeLaneMembership(context, options.commandArguments[0] ?? "", options.commandArguments[1] ?? "", options.commandArguments[2] ?? "");
      process.stdout.write(`LANES PASS ${options.commandArguments[2] ?? ""} rows=${countLines(options.commandArguments[2] ?? "")}\n`);
      return 0;
    case "-h":
    case "--help":
    case "help":
      process.stdout.write(usage());
      return 0;
    default:
      process.stderr.write(usage());
      fail(`unknown command: ${options.command}`);
  }
}
