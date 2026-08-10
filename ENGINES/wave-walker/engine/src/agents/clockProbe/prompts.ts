// clockProbe prompt — the walk's ONLY access to wall-clock time. The Workflow runtime forbids every
// in-script clock read, so elapsed time is unmeasurable from the graph itself; a seat with a shell is
// the one instrument that can answer "what time is it". Deliberately the smallest prompt in the engine:
// one command, one integer, no reasoning.
export const buildClockProbe = (): string =>
  'Run exactly this command and nothing else: `date +%s`\n' +
  'Return its integer output as `epochSeconds`. Do not explain, do not inspect the repository, ' +
  'do not run any other command. If the command fails, return `epochSeconds: 0`.';
