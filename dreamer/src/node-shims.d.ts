declare module "node:fs" {
  export interface Stats {
    isFile(): boolean;
    isDirectory(): boolean;
    isSymbolicLink(): boolean;
    mode: number;
    uid: number;
    mtimeMs: number;
  }
  export interface Dirent {
    name: string;
    isFile(): boolean;
    isDirectory(): boolean;
  }
  export const constants: { readonly X_OK: number };
  export function accessSync(path: string, mode?: number): void;
  export function appendFileSync(path: string, data: string, options?: { encoding?: string; mode?: number }): void;
  export function chmodSync(path: string, mode: number): void;
  export function closeSync(fd: number): void;
  export function copyFileSync(source: string, destination: string): void;
  export function cpSync(source: string, destination: string, options?: { recursive?: boolean; preserveTimestamps?: boolean }): void;
  export function existsSync(path: string): boolean;
  export function lstatSync(path: string): Stats;
  export function mkdirSync(path: string, options?: { recursive?: boolean; mode?: number }): string | undefined;
  export function mkdtempSync(prefix: string): string;
  export function openSync(path: string, flags: string, mode?: number): number;
  export function readFileSync(path: string, encoding: "utf8"): string;
  export function readdirSync(path: string, options: { withFileTypes: true }): Dirent[];
  export function realpathSync(path: string): string;
  export function renameSync(oldPath: string, newPath: string): void;
  export function rmSync(path: string, options?: { recursive?: boolean; force?: boolean }): void;
  export function statSync(path: string): Stats;
  export function unlinkSync(path: string): void;
  export function writeFileSync(path: string, data: string, options?: { encoding?: string; mode?: number; flag?: string }): void;
}

declare module "node:path" {
  export function basename(path: string, suffix?: string): string;
  export function dirname(path: string): string;
  export function isAbsolute(path: string): boolean;
  export function join(...paths: string[]): string;
  export function relative(from: string, to: string): string;
  export function resolve(...paths: string[]): string;
  export const sep: string;
}

declare module "node:crypto" {
  interface Hash {
    update(data: string | Uint8Array): Hash;
    digest(encoding: "hex"): string;
  }
  export function createHash(algorithm: string): Hash;
}

declare module "node:child_process" {
  export interface SpawnSyncOptions {
    cwd?: string;
    env?: Record<string, string | undefined>;
    encoding?: "utf8";
    input?: string;
    stdio?: "inherit" | ["inherit", "inherit", "inherit"];
    timeout?: number;
    killSignal?: string;
  }
  export interface SpawnSyncReturns {
    pid?: number;
    stdout: string | null;
    stderr: string | null;
    status: number | null;
    signal: string | null;
    error?: Error;
  }
  export function spawnSync(command: string, args?: readonly string[], options?: SpawnSyncOptions): SpawnSyncReturns;
  export interface ChildProcess {
    pid?: number;
    exitCode: number | null;
    signalCode: string | null;
    once(event: "exit", listener: (code: number | null, signal: string | null) => void): ChildProcess;
    once(event: "error", listener: (error: Error) => void): ChildProcess;
    kill(signal?: string): boolean;
  }
  export interface SpawnOptions {
    cwd?: string;
    env?: Record<string, string | undefined>;
    detached?: boolean;
    stdio?: [number | "ignore", number | "ignore", number | "ignore"];
  }
  export function spawn(command: string, args?: readonly string[], options?: SpawnOptions): ChildProcess;
}

declare module "node:assert/strict" {
  interface Assert {
    (value: unknown, message?: string): asserts value;
    equal(actual: unknown, expected: unknown, message?: string): void;
    deepEqual(actual: unknown, expected: unknown, message?: string): void;
    match(actual: string, expected: RegExp, message?: string): void;
    doesNotMatch(actual: string, expected: RegExp, message?: string): void;
  }
  const assert: Assert;
  export default assert;
}

declare module "node:test" {
  export interface TestContext {
    name: string;
  }
  export default function test(name: string, fn: (context: TestContext) => void | Promise<void>): void;
}

declare const process: {
  argv: string[];
  env: Record<string, string | undefined>;
  cwd(): string;
  umask(mask?: number): number;
  exitCode?: number;
  pid: number;
  execPath: string;
  getuid?: () => number;
  kill(pid: number, signal?: string): boolean;
  on(event: "SIGINT" | "SIGTERM", listener: () => void): void;
  off(event: "SIGINT" | "SIGTERM", listener: () => void): void;
  stdin: { setEncoding(encoding: string): void; on(event: "data", listener: (chunk: string) => void): void; on(event: "end", listener: () => void): void };
  stdout: { write(data: string): boolean };
  stderr: { write(data: string): boolean };
};

declare function setTimeout(handler: () => void, timeout: number): number;
declare function clearTimeout(handle: number): void;
declare const console: {
  log(...values: unknown[]): void;
  error(...values: unknown[]): void;
};
