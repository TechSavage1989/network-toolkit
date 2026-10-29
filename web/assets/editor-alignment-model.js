/* Pure comparison presentation geometry. No source text is inserted or changed. */
(function (root) {
'use strict';
const LIMITS = Object.freeze({lineCells: 40000, totalCells: 1000000, sourceLines: 30000, rows: 60000});
const isHigh = value => value >= 0xD800 && value <= 0xDBFF;
const isLow = value => value >= 0xDC00 && value <= 0xDFFF;
const splitsPair = (text, offset) => offset > 0 && offset < text.length && isHigh(text.charCodeAt(offset - 1)) && isLow(text.charCodeAt(offset));
function range(from, to) { return from < to ? [{from, to}] : []; }
function appendRange(ranges, from, to) {
 const last = ranges[ranges.length - 1];
 if (last && last.to === from) last.to = to;
 else if (from < to) ranges.push({from, to});
}

// Only the bounded changed middle uses LCS. Prefix/suffix work is linear in the
// input, and all offsets remain UTF-16 source offsets (never inside a pair).
// The shared mutable budget is charged the conservative UTF-16 cell product.
function intralineRanges(left, right, budget = {remaining: LIMITS.totalCells}) {
 if (typeof left !== 'string' || typeof right !== 'string') throw new TypeError('Intraline inputs must be strings.');
 if (!budget || typeof budget !== 'object' || !Number.isSafeInteger(budget.remaining) || budget.remaining < 0) throw new TypeError('A finite nonnegative integer comparison budget is required.');
 budget.remaining = Math.min(LIMITS.totalCells, budget.remaining);
 let prefix = 0;
 while (prefix < left.length && prefix < right.length && left[prefix] === right[prefix]) prefix++;
 if (splitsPair(left, prefix) || splitsPair(right, prefix)) prefix--;
 let suffix = 0;
 while (suffix < left.length - prefix && suffix < right.length - prefix && left[left.length - suffix - 1] === right[right.length - suffix - 1]) suffix++;
 if (splitsPair(left, left.length - suffix) || splitsPair(right, right.length - suffix)) suffix--;
 const leftEnd = left.length - suffix, rightEnd = right.length - suffix;
 const fallback = {left: range(prefix, leftEnd), right: range(prefix, rightEnd)};
 const cells = (leftEnd - prefix) * (rightEnd - prefix);
 if (!cells || cells > LIMITS.lineCells || cells > budget.remaining) return fallback;
 budget.remaining -= cells;
 const a = Array.from(left.slice(prefix, leftEnd)), b = Array.from(right.slice(prefix, rightEnd));
 const starts = values => { const offsets = [prefix]; for (const value of values) offsets.push(offsets[offsets.length - 1] + value.length); return offsets; };
 const aOffsets = starts(a), bOffsets = starts(b), width = b.length + 1;
 // With a 40,000-cell product, the shorter dimension is at most 200, so a
 // Uint16 LCS length cannot overflow. Degenerate zero dimensions returned above.
 const table = new Uint16Array((a.length + 1) * width);
 for (let i = a.length - 1; i >= 0; i--) {
  for (let j = b.length - 1; j >= 0; j--) {
   table[i * width + j] = a[i] === b[j] ? 1 + table[(i + 1) * width + j + 1] : Math.max(table[(i + 1) * width + j], table[i * width + j + 1]);
  }
 }
 const result = {left: [], right: []};
 let i = 0, j = 0;
 while (i < a.length || j < b.length) {
  if (i < a.length && j < b.length && a[i] === b[j]) { i++; j++; }
  else if (i < a.length && (j === b.length || table[(i + 1) * width + j] >= table[i * width + j + 1])) {
   appendRange(result.left, aOffsets[i], aOffsets[i + 1]); i++;
  } else { appendRange(result.right, bOffsets[j], bOffsets[j + 1]); j++; }
 }
 return result;
}

// Every real line appears exactly once and retains its source index. Missing
// rows and unequal wrapping become leading/after-line pixel spacers only.
function alignedGeometry(result, leftHeights, rightHeights) {
 if (!result || result.limited || !Array.isArray(result.rows) || result.rows.length > LIMITS.rows) throw new TypeError('A bounded line comparison is required.');
 const heights = {left: leftHeights, right: rightHeights};
 for (const side of ['left', 'right']) {
  const values = heights[side];
  if (!Array.isArray(values) || values.length > LIMITS.sourceLines || values.some(value => typeof value !== 'number' || !Number.isFinite(value) || value <= 0 || value > Number.MAX_SAFE_INTEGER)) throw new TypeError('Source line heights must be bounded positive finite numbers.');
  if (result.lineCounts && result.lineCounts[side] !== values.length) throw new TypeError('Source line count differs from measured heights.');
 }
 const rowOffsets = [0], sourceOffsets = {left: [], right: []}, next = {left: 0, right: 0};
 for (const row of result.rows) {
  if (!row || row.left === null && row.right === null) throw new TypeError('An aligned row must contain a real source line.');
  let height = 0;
  for (const side of ['left', 'right']) {
   const index = row[side];
   if (index === null) continue;
   if (!Number.isInteger(index) || index !== next[side] || index >= heights[side].length) throw new TypeError('Aligned source indices must be complete and ordered.');
   sourceOffsets[side].push(rowOffsets[rowOffsets.length - 1]);
   height = Math.max(height, heights[side][index]); next[side]++;
  }
  const total = rowOffsets[rowOffsets.length - 1] + height;
  if (!Number.isFinite(total) || total <= rowOffsets[rowOffsets.length - 1] || total > Number.MAX_SAFE_INTEGER) throw new RangeError('Aligned height exceeds the finite geometry bound.');
  rowOffsets.push(total);
 }
 const total = rowOffsets[rowOffsets.length - 1], leading = {}, paddingAfter = {left: [], right: []};
 for (const side of ['left', 'right']) {
  if (next[side] !== heights[side].length) throw new TypeError('Comparison omitted a measured source line.');
  leading[side] = sourceOffsets[side][0] ?? total;
  sourceOffsets[side].push(total);
  for (let i = 0; i < heights[side].length; i++) paddingAfter[side].push(Math.max(0, sourceOffsets[side][i + 1] - sourceOffsets[side][i] - heights[side][i]));
 }
 return {rowOffsets, sourceOffsets, paddingAfter, leading};
}
const api = Object.freeze({LIMITS, intralineRanges, alignedGeometry});
root.NTKEditorAlignmentModel = api;
if (typeof module === 'object' && module.exports) module.exports = api;
})(typeof window === 'undefined' ? globalThis : window);
