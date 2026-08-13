import { readStdin, runClaudeHook } from "./hooks.js";

const output = runClaudeHook(await readStdin(), process.env.CLAUDE_PROJECT_DIR);
if (output !== "") process.stdout.write(`${output}\n`);
