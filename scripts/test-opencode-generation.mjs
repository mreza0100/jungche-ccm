// Exercise the installed symlink layout in an isolated HOME; fail loudly when
// a command directory is skipped or a recursive link cannot be inspected.
import { strict as assert } from "node:assert";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
const root = mkdtempSync(join(tmpdir(), "pfm-opencode-"));
try {
  const script = join(root, "repo/.claude/scripts/build-opencode.mjs");
  mkdirSync(dirname(script), { recursive: true });
  copyFileSync(resolve(".claude/scripts/build-opencode.mjs"), script);
  const home = join(root, "home");
  const commands = join(home, ".claude/commands");
  const source = join(root, "sources/wave");
  mkdirSync(commands, { recursive: true });
  mkdirSync(source, { recursive: true });
  writeFileSync(
    join(source, "refine.md"),
    "---\ndescription: Refine a task.\n---\nRead the task, then execute /wave:review.\n",
  );
  writeFileSync(
    join(source, "review.md"),
    "---\ndescription: Review a task.\n---\nCheck the result.\n",
  );
  symlinkSync(source, join(commands, "wave"));
  const run = (mode) =>
    spawnSync(process.execPath, [script, mode], {
      env: { ...process.env, OPENCODE_BUILD_HOME: home },
      encoding: "utf8",
      timeout: 30000,
    });
  const skill = join(commands, "context-meter");
  mkdirSync(skill);
  writeFileSync(
    join(skill, "SKILL.md"),
    "---\ndescription: Audit context.\n---\nCount prompt tokens.\n",
  );
  const generated = run("generate");
  assert.equal(generated.status, 0, generated.stderr);
  assert.match(
    readFileSync(join(home, ".config/opencode/command/wave-refine.md"), "utf8"),
    /Read the task, then execute \/wave-review\./,
  );
  assert.match(
    readFileSync(
      join(home, ".config/opencode/command/context-meter.md"),
      "utf8",
    ),
    /Count prompt tokens/,
  );
  assert.equal(run("doctor").status, 0);
  symlinkSync(commands, join(source, "cycle"));
  const cyclic = run("check");
  assert.notEqual(cyclic.status, 0, "cyclic source was silently accepted");
  assert.match(cyclic.stderr, /cycle/i);
  console.log(
    "OpenCode symlinked command generation and cycle diagnostics pass",
  );
} finally {
  rmSync(root, { recursive: true, force: true });
}
