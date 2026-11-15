'use strict';
const real = require('./real-app-binding.cjs');
exports.open = async function () {
  const f = await real.open();
  const variant = process.env.QUALITY_TYPED_NEGATIVE;
  const original = { answer:f.answer, result:f.result, snapshot:f.snapshot, settlement:f.settlement, pending:f.pending };
  let consumed = false;
  f.answer = async (key, raw) => {
    const response = await original.answer(key, raw);
    if (variant === 'wrong-unicode-count' && raw.includes('😀😀😀😀')) return {...response, ok:false, error:'altered captured scalar count'};
    if (variant === 'consumed-invalid-answer' && !response.ok) consumed = true;
    return response;
  };
  f.result = async key => {
    const result = await original.result(key);
    if (variant === 'reordered-selection' && result.answer?.optionIds?.length > 1) result.answer.optionIds.reverse();
    return result;
  };
  f.snapshot = async () => {
    const snapshot = await original.snapshot();
    if (variant === 'consumed-invalid-answer' && consumed) snapshot.pending = [];
    return snapshot;
  };
  f.settlement = async key => {
    const settlement = await original.settlement(key);
    if (variant === 'duplicate-settlement') settlement.durableResults.push(structuredClone(settlement.durableResults[0]));
    return settlement;
  };
  f.pending = async view => variant === 'erased-question-obligation' ? [] : original.pending(view);
  return f;
};
