/* Pure comparison presentation geometry. No source text is inserted or changed. */
(function (root) {
'use strict';
const LIMITS = Object.freeze({lineCells: 40000, totalCells: 1000000, sourceLines: 30000, rows: 60000});
const isHigh = value => value >= 0xD800 && value <= 0xDBFF;
const isLow = value => value >= 0xDC00 && value <= 0xDFFF;
const splitsPair = (text, offset) => offset > 0 && offset < text.length && isHigh(text.charCodeAt(offset - 1)) && isLow(text.charCodeAt(offset));
const wordParts = /[\p{L}\p{M}\p{N}_]+(?:[.-][\p{L}\p{M}\p{N}_]+)*|\s+|./gsu;
function range(from, to) { return from < to ? [{from, to}] : []; }
function appendRange(ranges, from, to) {
 const last = ranges[ranges.length - 1];
 if (last && last.to === from) last.to = to;
 else if (from < to) ranges.push({from, to});
}

function commonEdges(left, right, leftStart, rightStart, leftEnd, rightEnd) {
 const max = Math.min(leftEnd - leftStart, rightEnd - rightStart);
 let prefix = 0, suffix = 0;
 while (prefix < max && left[leftStart + prefix] === right[rightStart + prefix]) prefix++;
 if (splitsPair(left, leftStart + prefix) || splitsPair(right, rightStart + prefix)) prefix--;
 while (suffix < max - prefix && left[leftEnd - suffix - 1] === right[rightEnd - suffix - 1]) suffix++;
 if (splitsPair(left, leftEnd - suffix) || splitsPair(right, rightEnd - suffix)) suffix--;
 return {prefix, suffix};
}

function compareParts(text, from, to) {
 return Array.from(text.slice(from, to).matchAll(wordParts), match => ({text:match[0], from:from + match.index}));
}

// Align complete words and identifiers inside the changed middle. Exact edges
// inside each replacement still expose small character edits, while a shared
// letter inside two unrelated words cannot interrupt the highlighted span.
// Bound the alignment work by the original UTF-16 cell product.
function intralineRanges(left, right, budget = {remaining: LIMITS.totalCells}) {
 if (typeof left !== 'string' || typeof right !== 'string') throw new TypeError('Intraline inputs must be strings.');
 if (!budget || typeof budget !== 'object' || !Number.isSafeInteger(budget.remaining) || budget.remaining < 0) throw new TypeError('A finite nonnegative integer comparison budget is required.');
 budget.remaining = Math.min(LIMITS.totalCells, budget.remaining);
 const {prefix, suffix} = commonEdges(left, right, 0, 0, left.length, right.length);
 const leftEnd = left.length - suffix, rightEnd = right.length - suffix;
 const fallback = {left: range(prefix, leftEnd), right: range(prefix, rightEnd)};
 const cells = (leftEnd - prefix) * (rightEnd - prefix);
 if (!cells || cells > LIMITS.lineCells || cells > budget.remaining) return fallback;
 budget.remaining -= cells;
 const a = compareParts(left, prefix, leftEnd), b = compareParts(right, prefix, rightEnd);
 const width = b.length + 1;
 // With a 40,000-cell product, the shorter dimension is at most 200, so a
 // Uint16 LCS length cannot overflow. Degenerate zero dimensions returned above.
 const table = new Uint16Array((a.length + 1) * width);
 for (let i = a.length - 1; i >= 0; i--) {
  for (let j = b.length - 1; j >= 0; j--) {
   table[i * width + j] = a[i].text === b[j].text
    ? a[i].text.length + table[(i + 1) * width + j + 1]
    : Math.max(table[(i + 1) * width + j], table[i * width + j + 1]);
  }
 }
 const result = {left: [], right: []};
 let leftStart = prefix, rightStart = prefix;
 function appendChanges(leftStop, rightStop) {
  const {prefix, suffix} = commonEdges(left, right, leftStart, rightStart, leftStop, rightStop);
  const leftFrom = leftStart + prefix, rightFrom = rightStart + prefix;
  const leftTo = leftStop - suffix, rightTo = rightStop - suffix;
  if (leftTo - leftFrom === rightTo - rightFrom && leftTo - leftFrom > 2) {
   const oldText = Array.from(left.slice(leftFrom, leftTo));
   const newText = Array.from(right.slice(rightFrom, rightTo));
   const same = oldText.length === newText.length && oldText.filter((value, index) => value === newText[index]).length;
   // Two-thirds positional agreement distinguishes several small edits from
   // a replacement that happens to share repeated characters or punctuation.
   if (same * 3 >= oldText.length * 2) {
    let leftOffset = leftFrom, rightOffset = rightFrom;
    for (let index = 0; index < oldText.length; index++) {
     const nextLeft = leftOffset + oldText[index].length, nextRight = rightOffset + newText[index].length;
     if (oldText[index] !== newText[index]) {
      appendRange(result.left, leftOffset, nextLeft);
      appendRange(result.right, rightOffset, nextRight);
     }
     leftOffset = nextLeft;
     rightOffset = nextRight;
    }
    return;
   }
  }
  appendRange(result.left, leftFrom, leftTo);
  appendRange(result.right, rightFrom, rightTo);
 }
 let i = 0, j = 0;
 while (i < a.length && j < b.length) {
  const leftPart = a[i], rightPart = b[j];
  if (leftPart.text === rightPart.text) {
   appendChanges(leftPart.from, rightPart.from);
   leftStart = leftPart.from + leftPart.text.length;
   rightStart = rightPart.from + rightPart.text.length;
   i++;
   j++;
  } else {
   const skipLeft = table[(i + 1) * width + j], skipRight = table[i * width + j + 1];
   // A tied alignment must choose the same spans if the panes are swapped.
   if (skipLeft > skipRight || (skipLeft === skipRight && leftPart.text < rightPart.text)) i++;
   else j++;
  }
 }
 appendChanges(leftEnd, rightEnd);
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
