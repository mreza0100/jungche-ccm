import { readStdin, runCodexHook } from "./hooks.js";

const output = runCodexHook(await readStdin());
if (output !== "") process.stdout.write(`${output}\n`);
