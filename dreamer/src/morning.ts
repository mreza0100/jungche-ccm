import { basename, dirname, join } from "node:path";
import { existsSync } from "node:fs";
import { command, listFiles, readLines, requireFile } from "./io.js";

interface MorningLane {
  readonly repo: string;
  readonly agent: string;
}

export function parseRepos(text: string): MorningLane[] {
  const lanes: MorningLane[] = [];
  for (const raw of text.split("\n")) {
    const line = raw.replace(/\s*#.*$/, "").trim();
    if (line === "") continue;
    const fields = line.split(/\s+/);
    lanes.push({ repo: fields[0] ?? "", agent: fields[1] ?? "Explore" });
  }
  return lanes;
}

export function morningMain(args: readonly string[]): number {
  if (args.length > 0) {
    process.stderr.write("dreamer-morning: FAIL accepts no arguments\n");
    return 1;
  }
  const engine = dirname(process.argv[1] ?? "");
  const runner = join(engine, "dreamer-night");
  const reposPath = join(engine, "repos.list");
  if (!existsSync(runner)) {
    process.stderr.write(`dreamer-morning: FAIL missing runner: ${runner}\n`);
    return 1;
  }
  try {
    requireFile(reposPath);
  } catch {
    process.stderr.write(`dreamer-morning: FAIL missing repository list: ${reposPath}\n`);
    return 1;
  }
  const configured = parseRepos(readLines(reposPath).join("\n"));
  if (configured.length === 0) {
    process.stderr.write("dreamer-morning: FAIL repository list is empty\n");
    return 1;
  }
  let overall = 0;
  const runLane = (repo: string, agent: string): void => {
    process.stdout.write(`dreamer-morning: BEGIN repo=${repo} agent=${agent}\n`);
    const runnerArgs = agent === "Explore" ? ["--repo", repo] : ["--repo", repo, "--agent", agent];
    const result = command(runner, runnerArgs);
    process.stdout.write(result.stdout);
    process.stderr.write(result.stderr);
    if (result.status === 0) process.stdout.write(`dreamer-morning: PASS repo=${repo} agent=${agent}\n`);
    else {
      overall = 1;
      process.stderr.write(`dreamer-morning: FAIL repo=${repo} agent=${agent} rc=${result.status}\n`);
    }
    process.stdout.write(`dreamer-morning: END repo=${repo} agent=${agent}\n`);
  };
  for (const row of configured) {
    runLane(row.repo, row.agent);
    const profiles = join(row.repo, ".professor", "stm", "lanes");
    if (!existsSync(profiles)) continue;
    for (const profile of listFiles(profiles, ".md")) {
      const lane = basename(profile, ".md");
      if (lane !== "explorer" && lane !== row.agent) runLane(row.repo, lane);
    }
  }
  return overall;
}
