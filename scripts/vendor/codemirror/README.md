# Rebuilding the offline CodeMirror core

This directory pins @codemirror/state6.7.6, @codemirror/view6.43.15 and build-only
esbuild0.28.2 from the official https://registry.npmjs.org registry. `REGISTRY.json`
records the primary registry versions, publication dates, tarball URLs and
integrities read on2026-10-28. `package-lock.json` pins every transitive dependency
and optional esbuild platform binary by version, official URL and SHA-512. No
package install scripts run. No global installation or application settings are
required. `node_modules` is ignored and must never be committed or shipped.

From this directory, with Node.js18+ and npm supporting lockfileVersion3:

```sh
npm ci --ignore-scripts --no-audit --no-fund --registry=https://registry.npmjs.org
npm run build
npm run verify
npm audit --audit-level=low --registry=https://registry.npmjs.org
npm audit --omit=dev --audit-level=low --registry=https://registry.npmjs.org
```

Initial build used Node24.22.1/npm11.9.0 on Linux x64. The esbuild API locates its
pinned platform binary without running package lifecycle scripts. `build` writes
the checked-in outputs. `verify` rebuilds in memory and byte-compares the bundle,
manifest, runtime license notices and build notices; it fails on any difference.
It does not update them. The builder itself makes no network requests; only the
explicit npm install/audit commands contact the registry. CI should use `verify`,
not silently regenerate changed outputs. Audit failure blocks release review.

Runtime outputs:
- `web/assets/vendor/codemirror/codemirror.js`: one minified ES2020 browser IIFE,
  global `NetworkToolkitCodeMirror`,15 explicitly enumerated core exports.
- `SOURCE.json`: exact runtime package identities, source tarball integrities,
  license hashes, entry/build/lock hashes and output SHA-256/size.
- `LICENSES.txt`: full MIT text for all six bundled runtime packages.

The bundle includes only state/view and their small transitive utilities. It
contains no merge engine, language pack, command/history package, source map,
external imports, CDN dependency, file/credential API or runtime package loader.
The application retains its existing diff algorithm, undo and file ownership.
`BUILD_LICENSES.txt` covers esbuild and all locked optional platform binaries;
those are not in the browser bundle. Preserve both runtime and build notices.
The release's external THIRD_PARTY_NOTICES.txt must also include runtime notices.

`AUDIT.json` is the historical initial full/runtime audit result (zero known
vulnerabilities at that time), not a claim of future or comprehensive security.
The repository test `scripts/codemirror-vendor.test.cjs` verifies integrity,
allowlisted exports, exact package pins, no runtime network/eval loaders and a
Unicode/source-position state-edit smoke test without launching a browser.
Actual rendered editing, IME, selection and pixel behavior require the separate
hosted comparison acceptance gate; a reproducible bundle is not that proof.
