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
`build/Network_Toolkit.exe`, and beside it the separate Text Editor and
Terminal programs `build/Network_Toolkit_TextEditor.exe` and
`build/Network_Toolkit_Terminal.exe` ([separate programs](SEPARATE_EXECUTABLES.md)).
They are the same code built with the `ntk_editor` and `ntk_terminal` tags and an
overlay that leaves the main program's resource object out; a plain
`go build -tags ntk_terminal` for Windows stops with "too many .rsrc sections".
The executables do not need the source folders.

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

## Release note writing

Follow [the release notes standard](RELEASE_NOTES_STYLE.md) for each new
version. The in-app "What's New" panel is a short, user-focused summary of
the effects of the changes. The expandable "Additional Details" section is
for technical behavior, safeguards, limitations and compatibility.
Use the exact headings "## What's New" and "## Additional Details" in the
new root release note, and verify the rendered preview and disclosure.
Keep detailed CI results, implementation traces and source identifiers in
release handoffs rather than the user-facing summary. Preserve previous
published notes without rewriting them.

## Packaging and publication

Stage/commit all intended source paths before packaging: the packager uses
`git ls-files`, so unstaged newly added files are not part of its source list.
Create local review artifacts without publishing. Install the NSIS compiler and
make `makensis` available on `PATH` to include the Windows setup executable.

```sh
python3 scripts/github_release.py package --exe build/Network_Toolkit.exe \
  --editor-exe build/Network_Toolkit_TextEditor.exe --terminal-exe build/Network_Toolkit_Terminal.exe \
  --linux-x86-64 build/linux-amd64/network-toolkit --linux-aarch64 build/linux-arm64/network-toolkit
python3 scripts/github_release.py verify
```

Packaging uses the current version from source. Portable bundles retain
top-level `README.txt`, `VALIDATION.txt` and the current release note, and hold
the three programs; their `source/` tree mirrors the repository. The
binary-only update bundle keeps the four members released updaters read and
adds the two separate programs (six members), taking its guides from `docs/`.
The release assets stay the same seven.

Checks installs and removes the setup build on an isolated Windows runner.
Release repeats that validation against the exact installer before publishing.

Publication needs explicit authorization. The existing release guard skips an
already-published version on main pushes. Do not change versions/tags/assets or
use revision requests for a source-layout cleanup. Follow root AGENTS.md to
archive the previous release note on future authorized builds.
