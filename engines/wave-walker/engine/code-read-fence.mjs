#!/usr/bin/env node
// PreToolUse(Bash) fence for the read-only Wave Walker caller.
//
// WHY THIS EXISTS: `--allowedTools` PRE-APPROVES patterns, it does not REMOVE raw Bash. A production
// walk proved it — 176 of 178 Bash payloads fell outside the declared read grammar and two commands
// redirected three files into global /tmp. A post-run classifier can only report that; it cannot stop
// it. This hook is the boundary: it runs BEFORE the command, and a command outside the grammar never
// executes.
//
// ONE GRAMMAR, ONE HOME: the predicate is imported from production-caller.js — the same function the
// post-run observer uses. A second copy here would drift, and the drift would be invisible until a
// walk leaked something the classifier then declared fine.
import { matchesDeclaredReadGrammar } from './production-caller.js';

function deny(reason) {
  process.stdout.write(
    JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'PreToolUse',
        permissionDecision: 'deny',
        permissionDecisionReason: reason,
      },
    }),
  );
  // JSON + exit 2: exit 2 blocks on the legacy path and the JSON carries the reason on the modern one.
  // Fail-closed under either contract — the property a security fence must not get wrong.
  process.exit(2);
}

const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);

let payload;
try {
  payload = JSON.parse(Buffer.concat(chunks).toString('utf8'));
} catch {
  // Unparseable hook input is not permission to run: an error must never render as an allow.
  deny('code_read_fence: hook input was not valid JSON, so the command could not be checked.');
}

if (payload?.tool_name !== 'Bash') process.exit(0);

const command = payload?.tool_input?.command;
if (matchesDeclaredReadGrammar(command)) process.exit(0);

// The reason names the RULE and the shape, never the command text — a denial message is a log line,
// and repository content does not belong in one.
deny(
  'code_read_fence: this command is outside the declared read grammar. The Wave Walker runs read-only: ' +
    'exactly one `git diff|show|rev-parse|log|cat-file|ls-files` or `rg` invocation, with no shell ' +
    'metacharacters, no redirection, no command chaining, and none of --output/--ext-diff/--textconv/' +
    '--filters/--pre/--pre-glob. Use Read/Grep/Glob for file inspection, or split the work into single ' +
    'bounded git/rg reads.',
);
