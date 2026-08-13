import { runNudge } from "./hooks.js";

try {
  const output = runNudge(process.env.CLAUDE_PROJECT_DIR);
  if (output !== "") process.stdout.write(output);
} catch {
  // A user-prompt hook must fail silent when its best-effort status scan cannot run.
}
