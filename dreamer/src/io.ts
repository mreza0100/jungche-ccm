import {
  appendFileSync,
  chmodSync,
  copyFileSync,
  cpSync,
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  realpathSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { basename, join } from "node:path";
import { fail } from "./errors.js";

export function requireFile(path: string): void {
  if (!existsSync(path) || !statSync(path).isFile()) fail(`missing file: ${path}`);
}

export function requireDirectory(path: string): void {
  if (!existsSync(path) || !statSync(path).isDirectory()) fail(`missing directory: ${path}`);
}

export function ensureDirectory(path: string, mode = 0o700): void {
  mkdirSync(path, { recursive: true, mode });
  chmodSync(path, mode);
}

export function readText(path: string): string {
  requireFile(path);
  return readFileSync(path, "utf8");
}

export function linesOf(text: string): string[] {
  if (text === "") return [];
  const lines = text.split("\n");
  if (lines[lines.length - 1] === "") lines.pop();
  return lines;
}

export function readLines(path: string): string[] {
  return linesOf(readText(path));
}

export function writePrivate(path: string, text: string): void {
  writeFileSync(path, text, { encoding: "utf8", mode: 0o600 });
  chmodSync(path, 0o600);
}

export function appendPrivate(path: string, text: string): void {
  appendFileSync(path, text, { encoding: "utf8", mode: 0o600 });
  chmodSync(path, 0o600);
}

export function sha256Text(text: string): string {
  return createHash("sha256").update(text).digest("hex");
}

export function sha256File(path: string): string {
  return sha256Text(readText(path));
}

export function listFiles(path: string, suffix?: string): string[] {
  requireDirectory(path);
  return readdirSync(path, { withFileTypes: true })
    .filter((entry) => entry.isFile() && (suffix === undefined || entry.name.endsWith(suffix)))
    .map((entry) => join(path, entry.name))
    .sort((left, right) => left.localeCompare(right));
}

export function listFilesRecursive(path: string, suffix: string): string[] {
  requireDirectory(path);
  const found: string[] = [];
  for (const entry of readdirSync(path, { withFileTypes: true })) {
    const candidate = join(path, entry.name);
    if (entry.isDirectory()) found.push(...listFilesRecursive(candidate, suffix));
    else if (entry.isFile() && entry.name.endsWith(suffix)) found.push(candidate);
  }
  return found.sort((left, right) => left.localeCompare(right));
}

export interface CommandResult {
  readonly status: number;
  readonly stdout: string;
  readonly stderr: string;
}

export function command(commandName: string, args: readonly string[], cwd?: string, input?: string): CommandResult {
  const options: {
    cwd?: string;
    encoding: "utf8";
    env: Record<string, string | undefined>;
    input?: string;
  } = { encoding: "utf8", env: process.env };
  if (cwd !== undefined) options.cwd = cwd;
  if (input !== undefined) options.input = input;
  const result = spawnSync(commandName, args, options);
  if (result.error !== undefined) fail(`required command unavailable: ${commandName}: ${result.error.message}`);
  return {
    status: result.status ?? 1,
    stdout: result.stdout ?? "",
    stderr: result.stderr ?? "",
  };
}

export function checkedCommand(commandName: string, args: readonly string[], cwd?: string, input?: string): string {
  const result = command(commandName, args, cwd, input);
  if (result.status !== 0) {
    const detail = result.stderr.trim() || result.stdout.trim() || `exit ${result.status}`;
    fail(`${commandName} failed: ${detail}`);
  }
  return result.stdout.trimEnd();
}

export function git(repo: string, args: readonly string[]): string {
  const allowed = new Set(["rev-parse", "cat-file", "show", "grep", "status"]);
  const operation = args[0];
  if (operation === undefined || !allowed.has(operation)) fail(`forbidden git operation: ${operation ?? "empty"}`);
  return checkedCommand("git", ["-C", repo, ...args]);
}

export function gitResult(repo: string, args: readonly string[]): CommandResult {
  const allowed = new Set(["rev-parse", "cat-file", "show", "grep", "status"]);
  const operation = args[0];
  if (operation === undefined || !allowed.has(operation)) fail(`forbidden git operation: ${operation ?? "empty"}`);
  return command("git", ["-C", repo, ...args]);
}

export function canonicalDirectory(path: string): string {
  requireDirectory(path);
  const resolved = realpathSync(path);
  if (resolved !== path) fail(`path must be canonical: ${path}`);
  if (lstatSync(path).isSymbolicLink()) fail(`path is a symlink: ${path}`);
  return resolved;
}

export function copyDirectoryContents(source: string, destination: string): void {
  requireDirectory(source);
  ensureDirectory(destination);
  for (const entry of readdirSync(source, { withFileTypes: true })) {
    const from = join(source, entry.name);
    const to = join(destination, entry.name);
    if (entry.isDirectory()) cpSync(from, to, { recursive: true, preserveTimestamps: true });
    else if (entry.isFile()) copyFileSync(from, to);
  }
}

export function mapFingerprint(mapsDirectory: string): string {
  const rows = listFiles(mapsDirectory, ".md").map((file) => `${sha256File(file)}  ${basename(file)}\n`).join("");
  return sha256Text(rows);
}

export function countLines(path: string): number {
  return readLines(path).length;
}

export function uniqueSorted(values: readonly string[]): string[] {
  return [...new Set(values)].sort((left, right) => left.localeCompare(right));
}

export function isoNow(): string {
  return new Date().toISOString();
}

export function dateToday(): string {
  return new Date().toISOString().slice(0, 10);
}
