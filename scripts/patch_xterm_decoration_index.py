"""Apply or verify the pinned xterm 6.0.0 decoration-index correction.

The generated browser asset is included in source releases, so normal Go
builds do not require Python. This script reproduces the exact small change
from the original npm bundle and rejects every other upstream input.
"""
import argparse
import hashlib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
UPSTREAM_SHA256 = "14903579ff54664cd72f8e8699e6961a6272c21863ec1c3b118cdc8af5d4a972"
OLD_DELETE = (
    'delete(e){if(this._flushCleanupInserted(),0===this._array.length)return!1;'
    'const t=this._getKey(e);if(void 0===t)return!1;if(r=this._search(t),-1===r)'
    'return!1;if(this._getKey(this._array[r])!==t)return!1;do{if(this._array[r]===e)'
    'return 0===this._deletedIndices.length&&this._flushDeletedTask.enqueue('
    '(()=>this._flushDeleted())),this._deletedIndices.push(r),!0}while('
    '++r<this._array.length&&this._getKey(this._array[r])===t);return!1}'
).encode()
NEW_DELETE = b'''delete(e){
// Network Toolkit: marker.line changes during disposal. Locate the exact
// object instead of binary-searching its already-mutated sort key. Keep
// batched cleanup; insert() flushes pending deletes before indices can move.
this._flushCleanupInserted();
const index=this._array.indexOf(e);
if(index<0||this._deletedIndices.includes(index))return false;
if(this._deletedIndices.length===0)this._flushDeletedTask.enqueue(()=>this._flushDeleted());
this._deletedIndices.push(index);
return true;
}'''


def patched_bundle(data):
    if hashlib.sha256(data).hexdigest() == UPSTREAM_SHA256:
        if data.count(OLD_DELETE) != 1:
            raise ValueError("Pinned xterm deletion method was not found exactly once")
        return data.replace(OLD_DELETE, NEW_DELETE, 1)
    if data.count(NEW_DELETE) == 1:
        original = data.replace(NEW_DELETE, OLD_DELETE, 1)
        if hashlib.sha256(original).hexdigest() == UPSTREAM_SHA256:
            return data
    raise ValueError("Unknown xterm bundle; no asset was changed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", type=Path, default=ROOT / "web/assets/xterm.js")
    parser.add_argument("--check", action="store_true", help="verify without changing files")
    args = parser.parse_args()
    data = args.bundle.read_bytes()
    result = patched_bundle(data)
    if args.check and data != result:
        raise ValueError("The pinned xterm correction has not been applied")
    if data != result:
        args.bundle.write_bytes(result)
    print("xterm decoration index verified: " + hashlib.sha256(result).hexdigest())


if __name__ == "__main__":
    main()
