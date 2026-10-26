# Development

Run commands from the repository root. Read [AGENTS.md](../AGENTS.md) and the
[handoff](DEVELOPMENT_HANDOFF.md) before changing working features.

## Layout

- `cmd/network-toolkit/`: intact application `main` package, Go tests and Windows resource object.
- `internal/`: shared implementation packages.
- `resources.go`: embedded interface files with stable `web/` and `assets/` paths.
- `web/`, `assets/`, `themes/`: interface, icon/resource inputs and theme presets.
- `scripts/`: JavaScript/browser tests, release tools and resource generation.
- `docs/`: guides and historical development records; `docs/releases/` archives older release notes.
- Root `RELEASE_NOTES_<current>.txt`: current release note required by packaging.

## Build and test

Use the Go version in `go.mod`. Windows build wrappers keep their original
entry points and output: `BUILD_WINDOWS.cmd` or `sh build.sh` produces
`build/Network_Toolkit.exe`. The executable does not need the source folders.

```sh
python3 scripts/github_release.py validate
python3 -m unittest discover -s scripts -p '*_test.py' -v
node --test scripts/*.test.cjs
go test -mod=vendor ./cmd/network-toolkit ./internal/terminal ./internal/desktop ./internal/diag ./internal/deviceinfo
python3 scripts/build_windows_resources.py
sh build.sh
```

For a Linux development executable:

```sh
CGO_ENABLED=0 go build -mod=vendor -o build/NetworkToolkit-linux ./cmd/network-toolkit
npm run dev
```

`go build .` now targets the embedded-resource library, not the application.
Use `./cmd/network-toolkit` in direct application build/run/test commands.
The [Checks workflow](../.github/workflows/checks.yml) also runs native Windows
and browser checks. Cross-compilation is not native runtime acceptance.

## Packaging and publication

Stage/commit all intended source paths before packaging: the packager uses
`git ls-files`, so unstaged newly added files are not part of its source list.
Create local review artifacts without publishing:

```sh
python3 scripts/github_release.py package --tag v1.17.89 --exe build/Network_Toolkit.exe
python3 scripts/github_release.py verify --tag v1.17.89
```

Replace the example tag only for an authorized version change. Portable bundles
retain top-level `README.txt`, `VALIDATION.txt` and the current release note;
their `source/` tree mirrors the repository. The binary-only update bundle keeps
its original four-member allowlist, taking its guides from `docs/`.

Publication needs explicit authorization. The existing release guard skips an
already-published version on main pushes. Do not change versions/tags/assets or
use revision requests for a source-layout cleanup. Follow root AGENTS.md to
archive the previous release note on future authorized builds.
