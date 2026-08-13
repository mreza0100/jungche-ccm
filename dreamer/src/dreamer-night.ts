import { DreamerError, errorMessage } from "./errors.js";
import { nightMain } from "./night.js";

process.umask(0o077);
try {
  process.exitCode = await nightMain(process.argv.slice(2));
} catch (error) {
  process.stderr.write(`dreamer-night: FAIL: ${errorMessage(error)}\n`);
  process.exitCode = error instanceof DreamerError ? 1 : 1;
}
