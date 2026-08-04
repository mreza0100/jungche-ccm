// Schema — a single StructuredOutput JSON-schema node. Every agent's output contract and every shared
// schema brick is one of these literals. Recursive: `properties` maps field name → nested Schema,
// `items` is the element Schema of an array. All but `type` are optional so a leaf like
// `{ type: 'string' }` and a deep object both satisfy it.
export interface Schema {
  type: 'object' | 'array' | 'string' | 'number' | 'integer' | 'boolean';
  properties?: Record<string, Schema>;
  items?: Schema;
  required?: string[];
  description?: string;
  enum?: string[];
  maxItems?: number;
  minLength?: number;
}
