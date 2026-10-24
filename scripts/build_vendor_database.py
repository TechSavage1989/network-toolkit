#!/usr/bin/env python3
"""Build the embedded vendor table from unmodified, downloaded IEEE CSVs.

This is a source-maintenance tool, not an application runtime dependency.
The source directory contains MA-L.csv, MA-M.csv, MA-S.csv, IAB.csv, and the
download metadata in sources.json. It makes no network requests.
"""

import argparse
import csv
import datetime
import gzip
import hashlib
import io
import json
from pathlib import Path
import re
import unicodedata


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-dir", type=Path, required=True)
    parser.add_argument("--date", required=True, help="UTC retrieval date, YYYY-MM-DD")
    args = parser.parse_args()
    date = datetime.date.fromisoformat(args.date).isoformat()
    sources = json.loads((args.source_dir / "sources.json").read_text(encoding="utf-8"))
    expected = {"MA-L": 6, "MA-M": 7, "MA-S": 9, "IAB": 9}
    rows = {}
    names = {}
    notices = [
        "Network Toolkit - bundled MAC assignment data",
        "",
        "Source: IEEE Registration Authority public assignment registries.",
        "https://standards.ieee.org/products-programs/regauth/",
        "Retrieved: " + date + " UTC",
        "",
        "The embedded table retains only assignment prefixes, registry names,",
        "and assigned organization names. It contains no mailing addresses.",
        "An assignment identifies its registered organization, not a device model.",
        "The original registry data belongs to its respective rights holders.",
        "",
    ]
    if {s["registry"] for s in sources} != set(expected):
        raise ValueError("sources.json must describe all four IEEE registries")
    for source in sources:
        registry = source["registry"]
        raw = (args.source_dir / (registry + ".csv")).read_bytes()
        digest = hashlib.sha256(raw).hexdigest()
        if digest != source["sha256"] or len(raw) != source["bytes"]:
            raise ValueError("download provenance mismatch: " + registry)
        count = 0
        reader = csv.DictReader(io.StringIO(raw.decode("utf-8-sig"), newline=""), strict=True)
        for row in reader:
            count += 1
            prefix = row["Assignment"].strip().upper()
            vendor = " ".join(row["Organization Name"].split())
            if row["Registry"] != registry or not re.fullmatch("[0-9A-F]{%d}" % expected[registry], prefix):
                raise ValueError("invalid assignment in " + registry)
            if not vendor or len(vendor.encode("utf-8")) > 1024 or any(unicodedata.category(c) == "Cc" for c in vendor):
                raise ValueError("invalid organization name in " + registry)
            item = {"p": prefix, "v": vendor, "r": registry}
            if prefix in rows and rows[prefix]["r"] != registry:
                raise ValueError("conflicting registries for assignment " + prefix)
            rows[prefix] = item
            names.setdefault(prefix, set()).add(vendor)
        if count != source["entries"]:
            raise ValueError("download row count mismatch: " + registry)
        notices.extend([
            registry + ": " + str(count) + " source rows",
            source["url"],
            "Last-Modified: " + source["lastModified"],
            "SHA256: " + digest,
            "",
        ])
    for prefix, organizations in names.items():
        rows[prefix]["v"] = "; ".join(sorted(organizations))
        if len(rows[prefix]["v"].encode("utf-8")) > 1024:
            raise ValueError("combined organization names too long: " + prefix)
    notices.extend(["Rare historical prefixes with multiple registered organizations retain",
                    "all listed organization names, separated by semicolons.", ""])
    payload = {
        "format": 1,
        "source": "IEEE public registries (MA-L, MA-M, MA-S, IAB)",
        "sourceDate": date,
        "updatedAt": date + "T00:00:00Z",
        "entries": [rows[key] for key in sorted(rows)],
    }
    target = Path(__file__).resolve().parents[1] / "internal" / "identity" / "data"
    target.mkdir(parents=True, exist_ok=True)
    plain = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    compressed = gzip.compress(plain, compresslevel=9, mtime=0)
    (target / "vendors.json.gz").write_bytes(compressed)
    notices.extend(["Embedded unique prefixes: " + str(len(rows)),
                    "Embedded SHA256: " + hashlib.sha256(compressed).hexdigest(), ""])
    (target / "SOURCE.txt").write_text("\n".join(notices), encoding="utf-8")
    print(json.dumps({"entries": len(rows), "compressedBytes": len(compressed), "expandedBytes": len(plain), "path": str(target / "vendors.json.gz")}))


if __name__ == "__main__":
    main()
