export class DreamerError extends Error {
  public constructor(message: string) {
    super(message);
    this.name = "DreamerError";
  }
}

export function fail(message: string): never {
  throw new DreamerError(message);
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
