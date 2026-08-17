// sliceSensor prompt — byte-identical to the source's inline construction (wave-walker.js lines 451-468).
import { RO } from '../shared.js';
import { ENC_VOCAB, DEC_VOCAB } from '../../constants.js';
import type { SliceSensorArgs } from '../../types/index.js';

export const buildSliceSensor = ({ jobId, kind, files, hint, assigned }: SliceSensorArgs): string =>
  'You are a PURE EXTRACTOR (scheduled sensor). NO judgment, NO bug-finding — extract and return, nothing else. ' +
  'Read ONLY these files (Grep to confirm an anchor is fine): ' +
  JSON.stringify(files) +
  '. Hint: ' +
  (hint || 'none') +
  '.\n' +
  'For EACH assigned field, extract its ' +
  kind.toUpperCase() +
  ' slice:\n' +
  (kind === 'producer'
    ? '· producer: where the value is mapped onto the result object — anchor, writer, typeToken, encoding (' +
      ENC_VOCAB +
      '), valueLiterals (EXACT, case-preserved).\n· dbColumn (if from a column): anchor, columnName, columnType, checkLiterals.\n· resolver (if a dedicated field/type resolver exists): anchor.\n'
    : kind === 'consumer'
      ? '· feSelection: where the query selects it — anchor, queryName (omit if never selected).\n· feTypes: generated type AND any hand-written interfaces — anchor, typeToken, kind (generated|hand).\n· consumers: EVERY read to the leaf render — anchor, name, decode (' +
        DEC_VOCAB +
        '), decodeExpr (VERBATIM, <=80 chars), context (production|test|generated|story), comparedLiterals (EXACT, case-preserved), aliasChain.\n· PARSE SITES ARE CONSUMERS: a screen that parses/transforms before drilling down (JSON.parse, mapping, memo) is its own consumer — its verbatim expression is the decodeExpr; a JSON.parse(JSON.stringify(x)) roundtrip MUST appear verbatim, never summarized; record each screen\'s parse separately.\n· undeclaredReads: any property read off the same result object NOT in your assigned field list (side:"fe"), INCLUDING reads in fallback chains (a ?? b, a || b, ternaries) and optional-chained access; plus any field the resolver returns beyond the declared set if a resolver file is listed (side:"be", expand spreads).\n'
      : '· producer (AI/engine writer): where this project\'s AI/compute layer computes/writes this value — anchor, writer:"ai", encoding, valueLiterals (EXACT). Grep the snake_case form if applicable.\n') +
  'A field with nothing to extract here gets a slice with just its fieldId. Every anchor grep-verified file:line. Keep strings SHORT (<=80 chars).\n' +
  'Assigned fields: ' +
  JSON.stringify(assigned) +
  RO +
  ' Structured output: jobId=' +
  jobId +
  ', slices, undeclaredReads.';
